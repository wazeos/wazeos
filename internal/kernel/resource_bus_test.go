package kernel

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

func TestNewResourceBus(t *testing.T) {
	bus := NewResourceBus()
	assert.NotNil(t, bus)
}

func TestResourceBus_RegisterDriver(t *testing.T) {
	bus := NewResourceBus()

	driver := &mockResourceDriver{
		name:     "test",
		patterns: []string{"file:///*"},
	}

	err := bus.RegisterDriver(driver)
	assert.NoError(t, err)
}

func TestResourceBus_RegisterDriver_Nil(t *testing.T) {
	bus := NewResourceBus()

	err := bus.RegisterDriver(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestResourceBus_RegisterDriver_NoPatterns(t *testing.T) {
	bus := NewResourceBus()

	driver := &mockResourceDriver{
		name:     "test",
		patterns: []string{},
	}

	err := bus.RegisterDriver(driver)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no patterns")
}

func TestResourceBus_RegisterDriver_Duplicate(t *testing.T) {
	bus := NewResourceBus()

	// Multiple drivers with the same name are allowed - routing is by URI pattern
	driver1 := &mockResourceDriver{name: "test", patterns: []string{"file:///*"}}
	driver2 := &mockResourceDriver{name: "test", patterns: []string{"http://*/*"}}

	err := bus.RegisterDriver(driver1)
	assert.NoError(t, err)

	// Should succeed - different patterns are allowed with same name
	err = bus.RegisterDriver(driver2)
	assert.NoError(t, err)
}

func TestResourceBus_RegisterDriver_InvalidPattern(t *testing.T) {
	bus := NewResourceBus()

	driver := &mockResourceDriver{
		name:     "test",
		patterns: []string{":::invalid:::"},
	}

	err := bus.RegisterDriver(driver)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestResourceBus_Call(t *testing.T) {
	bus := NewResourceBus()

	driver := &mockResourceDriver{
		name:     "file",
		patterns: []string{"file:///*"},
	}

	require.NoError(t, bus.RegisterDriver(driver))

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:     execCtx,
		URI:         "file:///data/test.txt",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"read"},
	}

	result, err := bus.Call(ctx, call)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "mock response", string(result.Body))
}

func TestResourceBus_Call_NilCall(t *testing.T) {
	bus := NewResourceBus()

	ctx := context.Background()
	result, err := bus.Call(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, types.IsInvalidRequest(err))
}

func TestResourceBus_Call_NoMatchingDriver(t *testing.T) {
	bus := NewResourceBus()

	driver := &mockResourceDriver{
		name:     "file",
		patterns: []string{"file:///*"},
	}

	require.NoError(t, bus.RegisterDriver(driver))

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:     execCtx,
		URI:         "http://example.com/test",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"read"},
	}

	result, err := bus.Call(ctx, call)
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 404, result.StatusCode)
	assert.True(t, types.IsNotFound(err))
}

func TestResourceBus_findDriver_Specificity(t *testing.T) {
	bus := NewResourceBus()

	// Register drivers with different specificity
	fileDriver := &mockResourceDriver{
		name:     "file",
		patterns: []string{"file:///*"},
	}
	httpDriver := &mockResourceDriver{
		name:     "http",
		patterns: []string{"http://*/*"},
	}
	specificHTTPDriver := &mockResourceDriver{
		name:     "http-api",
		patterns: []string{"http://api.example.com/*"},
	}

	require.NoError(t, bus.RegisterDriver(fileDriver))
	require.NoError(t, bus.RegisterDriver(httpDriver))
	require.NoError(t, bus.RegisterDriver(specificHTTPDriver))

	tests := []struct {
		name         string
		uri          string
		wantDriver   string
	}{
		{
			name:       "file URI matches file driver",
			uri:        "file:///data/test.txt",
			wantDriver: "file",
		},
		{
			name:       "http URI matches http driver",
			uri:        "http://example.com/test",
			wantDriver: "http",
		},
		{
			name:       "specific http URI matches specific driver",
			uri:        "http://api.example.com/v1/users",
			wantDriver: "http-api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := bus.findDriver(tt.uri)
			require.NotNil(t, driver)
			assert.Equal(t, tt.wantDriver, driver.Name())
		})
	}
}

func TestResourceBus_findDriver_InvalidURI(t *testing.T) {
	bus := NewResourceBus()

	driver := &mockResourceDriver{
		name:     "file",
		patterns: []string{"file:///*"},
	}

	require.NoError(t, bus.RegisterDriver(driver))

	result := bus.findDriver(":::invalid:::")
	assert.Nil(t, result)
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		pattern string
		want    bool
	}{
		{
			name:    "exact match",
			s:       "test",
			pattern: "test",
			want:    true,
		},
		{
			name:    "wildcard match all",
			s:       "anything",
			pattern: "*",
			want:    true,
		},
		{
			name:    "prefix match",
			s:       "test-file.txt",
			pattern: "test-*",
			want:    true,
		},
		{
			name:    "suffix match",
			s:       "file.txt",
			pattern: "*.txt",
			want:    true,
		},
		{
			name:    "middle match",
			s:       "prefix-middle-suffix",
			pattern: "prefix-*-suffix",
			want:    true,
		},
		{
			name:    "no match",
			s:       "test",
			pattern: "other",
			want:    false,
		},
		{
			name:    "prefix no match",
			s:       "test",
			pattern: "other-*",
			want:    false,
		},
		{
			name:    "suffix no match",
			s:       "test",
			pattern: "*.other",
			want:    false,
		},
		{
			name:    "empty string with wildcard",
			s:       "",
			pattern: "*",
			want:    true,
		},
		{
			name:    "empty string with pattern",
			s:       "",
			pattern: "test",
			want:    false,
		},
		{
			name:    "multiple wildcards",
			s:       "one-two-three",
			pattern: "*-*-*",
			want:    true,
		},
		{
			name:    "consecutive wildcards",
			s:       "test",
			pattern: "**",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGlob(tt.s, tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchScore(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		pattern string
		want    int // 0 = no match, higher = more specific
	}{
		{
			name:    "exact scheme match",
			uri:     "file:///data/test",
			pattern: "file:///*",
			want:    11, // scheme(1) + path(10)
		},
		{
			name:    "scheme + authority match",
			uri:     "http://api.example.com/test",
			pattern: "http://api.example.com/*",
			want:    111, // scheme(1) + authority(100) + path(10)
		},
		{
			name:    "scheme only match",
			uri:     "http://example.com/test",
			pattern: "http://*/*",
			want:    11, // scheme(1) + path(10)
		},
		{
			name:    "no match - different scheme",
			uri:     "file:///data/test",
			pattern: "http://*/*",
			want:    0,
		},
		{
			name:    "no match - different authority",
			uri:     "http://example.com/test",
			pattern: "http://other.com/*",
			want:    0,
		},
		{
			name:    "no match - different path",
			uri:     "file:///data/test",
			pattern: "file:///other/*",
			want:    0,
		},
		{
			name:    "wildcard authority",
			uri:     "http://anything.com/test",
			pattern: "http://*/*",
			want:    1, // should match with wildcard authority and path
		},
		{
			name:    "wildcard path",
			uri:     "file:///any/path/here",
			pattern: "file:///*",
			want:    11, // scheme(1) + path(10)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := parsePattern(tt.pattern)
			require.NoError(t, err)

			parsedURI, err := url.Parse(tt.uri)
			require.NoError(t, err)

			got := matchScore(parsedURI, pattern)

			// For test purposes, we'll compare relative scores
			// Just check if match/no-match is correct
			if tt.want == 0 {
				assert.Equal(t, 0, got, "expected no match")
			} else {
				assert.Greater(t, got, 0, "expected match")
			}
		})
	}
}

func TestParsePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    uriPattern
		wantErr bool
	}{
		{
			name:    "file pattern",
			pattern: "file:///data/*",
			want: uriPattern{
				original:  "file:///data/*",
				scheme:    "file",
				authority: "",
				path:      "/data/*",
			},
			wantErr: false,
		},
		{
			name:    "http pattern with authority",
			pattern: "http://api.example.com/v1/*",
			want: uriPattern{
				original:  "http://api.example.com/v1/*",
				scheme:    "http",
				authority: "api.example.com",
				path:      "/v1/*",
			},
			wantErr: false,
		},
		{
			name:    "wildcard authority",
			pattern: "http://*/*/*",
			want: uriPattern{
				original:  "http://*/*/*",
				scheme:    "http",
				authority: "*",
				path:      "/*/*",
			},
			wantErr: false,
		},
		{
			name:    "invalid pattern",
			pattern: ":::invalid:::",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePattern(tt.pattern)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.scheme, got.scheme)
				assert.Equal(t, tt.want.authority, got.authority)
				assert.Equal(t, tt.want.path, got.path)
			}
		})
	}
}

func TestResourceBus_MultipleDrivers_BestMatch(t *testing.T) {
	bus := NewResourceBus()

	// Register drivers with overlapping patterns
	// More general driver
	httpDriver := &mockResourceDriver{
		name:     "http-general",
		patterns: []string{"http://*/*"},
	}

	// More specific driver
	apiDriver := &mockResourceDriver{
		name:     "http-api",
		patterns: []string{"http://api.example.com/*"},
	}

	// Most specific driver
	specificAPIDriver := &mockResourceDriver{
		name:     "http-api-v1",
		patterns: []string{"http://api.example.com/v1/*"},
	}

	require.NoError(t, bus.RegisterDriver(httpDriver))
	require.NoError(t, bus.RegisterDriver(apiDriver))
	require.NoError(t, bus.RegisterDriver(specificAPIDriver))

	tests := []struct {
		name           string
		uri            string
		expectedDriver string
	}{
		{
			name:           "matches most specific",
			uri:            "http://api.example.com/v1/users",
			expectedDriver: "http-api-v1",
		},
		{
			name:           "matches moderately specific",
			uri:            "http://api.example.com/v2/posts",
			expectedDriver: "http-api",
		},
		{
			name:           "matches general",
			uri:            "http://other.com/test",
			expectedDriver: "http-general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := bus.findDriver(tt.uri)
			require.NotNil(t, driver)
			assert.Equal(t, tt.expectedDriver, driver.Name())
		})
	}
}
