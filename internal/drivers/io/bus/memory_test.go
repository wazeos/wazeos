package bus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

// mockResourceDriver is a test driver
type mockResourceDriver struct {
	name     string
	patterns []string
	called   bool
}

func (m *mockResourceDriver) Name() string {
	return m.name
}

func (m *mockResourceDriver) Patterns() []string {
	return m.patterns
}

func (m *mockResourceDriver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	m.called = true
	return &types.ResourceResult{
		StatusCode: 200,
		Headers:    make(map[string]string),
		Body:       []byte("success"),
	}, nil
}

func TestParseURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantScheme  string
		wantHost    string
		wantPath    string
		wantErr     bool
	}{
		{
			name:       "file URI with path",
			uri:        "file:///data/file.txt",
			wantScheme: "file",
			wantHost:   "",
			wantPath:   "/data/file.txt",
		},
		{
			name:       "https URI with host and path",
			uri:        "https://api.example.com/v1/users",
			wantScheme: "https",
			wantHost:   "api.example.com",
			wantPath:   "/v1/users",
		},
		{
			name:       "URI with just host",
			uri:        "https://example.com",
			wantScheme: "https",
			wantHost:   "example.com",
			wantPath:   "/",
		},
		{
			name:       "fn URI",
			uri:        "fn://my-app/invoke",
			wantScheme: "fn",
			wantHost:   "my-app",
			wantPath:   "/invoke",
		},
		{
			name:    "invalid URI no scheme",
			uri:     "not-a-uri",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantScheme, parsed.scheme)
			assert.Equal(t, tt.wantHost, parsed.host)
			assert.Equal(t, tt.wantPath, parsed.path)
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		pattern     string
		wantMatch   bool
		minScore    int // minimum expected score
	}{
		{
			name:      "exact file path match",
			uri:       "file:///data/file.txt",
			pattern:   "file:///data/file.txt",
			wantMatch: true,
			minScore:  100, // exact path: (2 segments * 10) + 15 chars + 100 = 135+
		},
		{
			name:      "file path wildcard match",
			uri:       "file:///data/file.txt",
			pattern:   "file:///data/*",
			wantMatch: true,
			minScore:  10, // wildcard: (1 segment * 10) + 6 chars = 16+
		},
		{
			name:      "file full wildcard match",
			uri:       "file:///data/file.txt",
			pattern:   "file://*/*",
			wantMatch: true,
			minScore:  0, // full wildcard: 0
		},
		{
			name:      "https exact host match",
			uri:       "https://api.example.com/v1/users",
			pattern:   "https://api.example.com/*",
			wantMatch: true,
			minScore:  100, // exact host: (3 segments * 15) + 15 chars + 50 + path = 110+
		},
		{
			name:      "https wildcard host match",
			uri:       "https://api.example.com/v1/users",
			pattern:   "https://*.example.com/*",
			wantMatch: true,
			minScore:  30, // wildcard host: (2 segments * 15) = 30
		},
		{
			name:      "https no match different host",
			uri:       "https://api.example.com/v1/users",
			pattern:   "https://other.com/*",
			wantMatch: false,
		},
		{
			name:      "different scheme no match",
			uri:       "https://example.com/path",
			pattern:   "http://example.com/path",
			wantMatch: false,
		},
		{
			name:      "path prefix no match",
			uri:       "file:///other/file.txt",
			pattern:   "file:///data/*",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uriParsed, err := parseURI(tt.uri)
			require.NoError(t, err)

			patternParsed, err := parseURI(tt.pattern)
			require.NoError(t, err)

			score := matchesPattern(uriParsed, patternParsed)

			if tt.wantMatch {
				assert.GreaterOrEqual(t, score, tt.minScore, "expected match with score >= %d, got %d", tt.minScore, score)
			} else {
				assert.Equal(t, -1, score, "expected no match (score -1)")
			}
		})
	}
}

func TestFindBestDriver_Specificity(t *testing.T) {
	bus := NewMemoryIOBus(nil)

	// Register drivers with different specificity levels
	genericDriver := &mockResourceDriver{
		name:     "generic",
		patterns: []string{"file://*/*"},
	}
	specificDriver := &mockResourceDriver{
		name:     "specific",
		patterns: []string{"file:///data/*"},
	}
	exactDriver := &mockResourceDriver{
		name:     "exact",
		patterns: []string{"file:///data/important.txt"},
	}

	require.NoError(t, bus.RegisterDriver(genericDriver))
	require.NoError(t, bus.RegisterDriver(specificDriver))
	require.NoError(t, bus.RegisterDriver(exactDriver))

	tests := []struct {
		name           string
		uri            string
		expectedDriver string
	}{
		{
			name:           "exact match wins",
			uri:            "file:///data/important.txt",
			expectedDriver: "exact",
		},
		{
			name:           "specific path wins",
			uri:            "file:///data/other.txt",
			expectedDriver: "specific",
		},
		{
			name:           "generic fallback",
			uri:            "file:///tmp/file.txt",
			expectedDriver: "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := bus.findBestDriver(tt.uri)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDriver, driver.Name())
		})
	}
}

func TestFindBestDriver_HTTPHosts(t *testing.T) {
	bus := NewMemoryIOBus(nil)

	// Register drivers with different host specificity
	wildcardDriver := &mockResourceDriver{
		name:     "wildcard",
		patterns: []string{"https://*.example.com/*"},
	}
	specificDriver := &mockResourceDriver{
		name:     "specific",
		patterns: []string{"https://api.example.com/*"},
	}

	require.NoError(t, bus.RegisterDriver(wildcardDriver))
	require.NoError(t, bus.RegisterDriver(specificDriver))

	tests := []struct {
		name           string
		uri            string
		expectedDriver string
	}{
		{
			name:           "exact host match wins",
			uri:            "https://api.example.com/v1/users",
			expectedDriver: "specific",
		},
		{
			name:           "wildcard host match",
			uri:            "https://other.example.com/path",
			expectedDriver: "wildcard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := bus.findBestDriver(tt.uri)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDriver, driver.Name())
		})
	}
}

func TestFindBestDriver_LengthGranularity(t *testing.T) {
	bus := NewMemoryIOBus(nil)

	// Register drivers with same segment count but different lengths
	shortPathDriver := &mockResourceDriver{
		name:     "short",
		patterns: []string{"file:///a/*"},
	}
	longPathDriver := &mockResourceDriver{
		name:     "long",
		patterns: []string{"file:///abc/*"},
	}

	require.NoError(t, bus.RegisterDriver(shortPathDriver))
	require.NoError(t, bus.RegisterDriver(longPathDriver))

	// Both patterns have 1 segment, but /abc/ is longer and should win
	driver, err := bus.findBestDriver("file:///abc/test.txt")
	require.NoError(t, err)
	assert.Equal(t, "long", driver.Name(), "longer prefix should win when segment count is equal")

	// For /a/ path, only short driver matches
	driver, err = bus.findBestDriver("file:///a/test.txt")
	require.NoError(t, err)
	assert.Equal(t, "short", driver.Name())
}

func TestFindBestDriver_WildcardHostGranularity(t *testing.T) {
	bus := NewMemoryIOBus(nil)

	// Register drivers with different wildcard host specificities
	shortWildcardDriver := &mockResourceDriver{
		name:     "short-wildcard",
		patterns: []string{"https://*.com/*"},
	}
	longWildcardDriver := &mockResourceDriver{
		name:     "long-wildcard",
		patterns: []string{"https://*.example.com/*"},
	}

	require.NoError(t, bus.RegisterDriver(shortWildcardDriver))
	require.NoError(t, bus.RegisterDriver(longWildcardDriver))

	// *.example.com has more segments (2) than *.com (1), so it should win
	driver, err := bus.findBestDriver("https://api.example.com/resource")
	require.NoError(t, err)
	assert.Equal(t, "long-wildcard", driver.Name(), "longer wildcard suffix should win")

	// For other.com, only short wildcard matches
	driver, err = bus.findBestDriver("https://other.com/resource")
	require.NoError(t, err)
	assert.Equal(t, "short-wildcard", driver.Name())
}

func TestCall_UsesPatternMatching(t *testing.T) {
	bus := NewMemoryIOBus(nil)

	driver := &mockResourceDriver{
		name:     "test-driver",
		patterns: []string{"file:///data/*"},
	}

	require.NoError(t, bus.RegisterDriver(driver))

	ctx := context.Background()
	call := &types.ResourceCall{
		URI:         "file:///data/test.txt",
		Permissions: []string{"read"},
		Headers:     make(map[string]string),
		Body:        []byte{},
	}

	result, err := bus.Call(ctx, call)
	require.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.True(t, driver.called, "driver should have been called")
}

func TestCall_NoMatchingDriver(t *testing.T) {
	bus := NewMemoryIOBus(nil)

	driver := &mockResourceDriver{
		name:     "test-driver",
		patterns: []string{"file:///data/*"},
	}

	require.NoError(t, bus.RegisterDriver(driver))

	ctx := context.Background()
	call := &types.ResourceCall{
		URI:         "file:///other/test.txt", // different path
		Permissions: []string{"read"},
		Headers:     make(map[string]string),
		Body:        []byte{},
	}

	result, err := bus.Call(ctx, call)
	require.NoError(t, err)
	assert.Equal(t, 404, result.StatusCode, "should return 404 for no matching driver")
	assert.False(t, driver.called, "driver should not have been called")
}
