package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// permissionsEqual checks if two permission slices contain the same elements (order-independent)
func permissionsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int)
	for _, perm := range a {
		counts[perm]++
	}
	for _, perm := range b {
		counts[perm]--
		if counts[perm] < 0 {
			return false
		}
	}
	return true
}

func TestAccessBits_String(t *testing.T) {
	tests := []struct {
		name string
		bits AccessBits
		want string
	}{
		{
			name: "read only",
			bits: AccessRead,
			want: "r",
		},
		{
			name: "write only",
			bits: AccessWrite,
			want: "w",
		},
		{
			name: "execute only",
			bits: AccessExecute,
			want: "x",
		},
		{
			name: "read write",
			bits: AccessRead | AccessWrite,
			want: "rw",
		},
		{
			name: "read execute",
			bits: AccessRead | AccessExecute,
			want: "rx",
		},
		{
			name: "write execute",
			bits: AccessWrite | AccessExecute,
			want: "wx",
		},
		{
			name: "all permissions",
			bits: AccessRead | AccessWrite | AccessExecute,
			want: "rwx",
		},
		{
			name: "no permissions",
			bits: 0,
			want: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bits.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAccessBits(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    AccessBits
		wantErr bool
	}{
		{
			name:    "parse r",
			input:   "r",
			want:    AccessRead,
			wantErr: false,
		},
		{
			name:    "parse w",
			input:   "w",
			want:    AccessWrite,
			wantErr: false,
		},
		{
			name:    "parse x",
			input:   "x",
			want:    AccessExecute,
			wantErr: false,
		},
		{
			name:    "parse rw",
			input:   "rw",
			want:    AccessRead | AccessWrite,
			wantErr: false,
		},
		{
			name:    "parse rx",
			input:   "rx",
			want:    AccessRead | AccessExecute,
			wantErr: false,
		},
		{
			name:    "parse wx",
			input:   "wx",
			want:    AccessWrite | AccessExecute,
			wantErr: false,
		},
		{
			name:    "parse rwx",
			input:   "rwx",
			want:    AccessRead | AccessWrite | AccessExecute,
			wantErr: false,
		},
		{
			name:    "parse order doesn't matter",
			input:   "xwr",
			want:    AccessRead | AccessWrite | AccessExecute,
			wantErr: false,
		},
		{
			name:    "parse duplicates",
			input:   "rrww",
			want:    AccessRead | AccessWrite,
			wantErr: false,
		},
		{
			name:    "invalid character",
			input:   "rz",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAccessBits(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAccessBits_Has(t *testing.T) {
	tests := []struct {
		name       string
		bits       AccessBits
		permission AccessBits
		want       bool
	}{
		{
			name:       "has read in rw",
			bits:       AccessRead | AccessWrite,
			permission: AccessRead,
			want:       true,
		},
		{
			name:       "has write in rw",
			bits:       AccessRead | AccessWrite,
			permission: AccessWrite,
			want:       true,
		},
		{
			name:       "doesn't have execute in rw",
			bits:       AccessRead | AccessWrite,
			permission: AccessExecute,
			want:       false,
		},
		{
			name:       "has rw in rwx",
			bits:       AccessRead | AccessWrite | AccessExecute,
			permission: AccessRead | AccessWrite,
			want:       true,
		},
		{
			name:       "doesn't have rw in read only",
			bits:       AccessRead,
			permission: AccessRead | AccessWrite,
			want:       false,
		},
		{
			name:       "zero permissions",
			bits:       0,
			permission: AccessRead,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bits.Has(tt.permission)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPermissionContext_Intersect(t *testing.T) {
	tests := []struct {
		name  string
		pc1   *PermissionContext
		pc2   *PermissionContext
		want  *PermissionContext
	}{
		{
			name: "same patterns, intersect permissions",
			pc1: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read", "write"}},
			}),
			pc2: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read"}},
			}),
			want: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read"}},
			}),
		},
		{
			name: "different patterns, no intersection",
			pc1: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read", "write"}},
			}),
			pc2: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///other/*", Permissions: []string{"read"}},
			}),
			want: NewPermissionContext([]PermissionEntry{}),
		},
		{
			name: "multiple patterns, partial intersection",
			pc1: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read", "write"}},
				{URIPattern: "http://api.example.com/*", Permissions: []string{"read"}},
			}),
			pc2: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read"}},
				{URIPattern: "fn://my-app/*", Permissions: []string{"execute"}},
			}),
			want: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read"}},
			}),
		},
		{
			name: "no common permissions, filtered out",
			pc1: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read"}},
			}),
			pc2: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"write"}},
			}),
			want: NewPermissionContext([]PermissionEntry{}),
		},
		{
			name: "empty contexts",
			pc1:  NewPermissionContext([]PermissionEntry{}),
			pc2:  NewPermissionContext([]PermissionEntry{}),
			want: NewPermissionContext([]PermissionEntry{}),
		},
		{
			name: "one empty context",
			pc1: NewPermissionContext([]PermissionEntry{
				{URIPattern: "file:///data/*", Permissions: []string{"read", "write"}},
			}),
			pc2:  NewPermissionContext([]PermissionEntry{}),
			want: NewPermissionContext([]PermissionEntry{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pc1.Intersect(tt.pc2)
			assert.Equal(t, len(tt.want.Entries), len(got.Entries))

			// Compare entries (order may vary, so check all expected entries are present)
			for _, wantEntry := range tt.want.Entries {
				found := false
				for _, gotEntry := range got.Entries {
					if gotEntry.URIPattern == wantEntry.URIPattern &&
					   len(gotEntry.Permissions) == len(wantEntry.Permissions) &&
					   permissionsEqual(gotEntry.Permissions, wantEntry.Permissions) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected entry not found: %+v", wantEntry)
			}
		})
	}
}

func TestNewExecutionContext(t *testing.T) {
	permissions := NewPermissionContext([]PermissionEntry{
		{URIPattern: "file:///data/*", Permissions: []string{"read"}},
	})

	ctx := NewExecutionContext("req-123", "trace-456", "user:alice", permissions)

	assert.Equal(t, "req-123", ctx.RequestID)
	assert.Equal(t, "trace-456", ctx.TraceID)
	assert.Equal(t, "user:alice", ctx.Principal)
	assert.Nil(t, ctx.ParentRequestID)
	assert.Equal(t, permissions, ctx.PermissionContext)
	assert.NotZero(t, ctx.Timestamp)
	assert.NotNil(t, ctx.Metadata)
}

func TestExecutionContext_ChildContext(t *testing.T) {
	parentPermissions := NewPermissionContext([]PermissionEntry{
		{URIPattern: "file:///data/*", Permissions: []string{"read", "write"}},
		{URIPattern: "fn://my-app/*", Permissions: []string{"execute"}},
	})

	childPermissions := NewPermissionContext([]PermissionEntry{
		{URIPattern: "file:///data/*", Permissions: []string{"read"}},
		{URIPattern: "http://api.example.com/*", Permissions: []string{"read"}},
	})

	parent := NewExecutionContext("req-123", "trace-456", "user:alice", parentPermissions)

	// Wait a tiny bit to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)

	child := parent.ChildContext("req-124", childPermissions)

	// Check child properties
	assert.Equal(t, "req-124", child.RequestID)
	assert.Equal(t, "trace-456", child.TraceID)
	assert.Equal(t, "user:alice", child.Principal)
	require.NotNil(t, child.ParentRequestID)
	assert.Equal(t, "req-123", *child.ParentRequestID)

	// Check permission intersection
	assert.Equal(t, 1, len(child.PermissionContext.Entries))
	assert.Equal(t, "file:///data/*", child.PermissionContext.Entries[0].URIPattern)
	assert.Equal(t, []string{"read"}, child.PermissionContext.Entries[0].Permissions)

	// Timestamp should be after parent
	assert.True(t, child.Timestamp.After(parent.Timestamp))
}

func TestAppMetadata_AppID(t *testing.T) {
	tests := []struct {
		name     string
		metadata AppMetadata
		want     string
	}{
		{
			name: "standard app ID",
			metadata: AppMetadata{
				Name:    "my-app",
				Version: "1.0.0",
				Author:  "alice",
			},
			want: "alice/my-app:1.0.0",
		},
		{
			name: "complex names",
			metadata: AppMetadata{
				Name:    "data-processor",
				Version: "2.3.4",
				Author:  "bob-smith",
			},
			want: "bob-smith/data-processor:2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metadata.AppID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInvocationResult_Success(t *testing.T) {
	result := &InvocationResult{
		RequestID:  "req-123",
		Stdout:     []byte("hello world"),
		Stderr:     []byte(""),
		ExitCode:   0,
		Duration:   100 * time.Millisecond,
		MemoryUsed: 1024 * 1024, // 1MB

	}

	assert.Equal(t, "req-123", result.RequestID)
	assert.Equal(t, "hello world", string(result.Stdout))
	assert.Equal(t, 0, result.ExitCode)
	assert.Empty(t, result.Error)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.Greater(t, result.MemoryUsed, int64(0))
}

func TestResourceCall_Structure(t *testing.T) {
	ctx := NewExecutionContext("req-123", "trace-456", "user:alice", NewPermissionContext(nil))

	call := &ResourceCall{
		Context:     ctx,
		URI:         "file:///data/test.txt",
		Headers:     map[string]string{"Content-Type": "text/plain"},
		Body:        []byte("test data"),
		Permissions: []string{"read"},
	}

	assert.Equal(t, "file:///data/test.txt", call.URI)
	assert.Equal(t, []string{"read"}, call.Permissions)
	assert.Equal(t, "text/plain", call.Headers["Content-Type"])
	assert.Equal(t, "test data", string(call.Body))
}

func TestResourceResult_Success(t *testing.T) {
	result := &ResourceResult{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"status":"ok"}`),
		Error:      "",
	}

	assert.Equal(t, 200, result.StatusCode)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Headers, "Content-Type")
	assert.JSONEq(t, `{"status":"ok"}`, string(result.Body))
}

func TestResourceResult_Error(t *testing.T) {
	result := &ResourceResult{
		StatusCode: 403,
		Headers:    map[string]string{},
		Body:       []byte("permission denied"),
		Error:      assert.AnError.Error(),
	}

	assert.Equal(t, 403, result.StatusCode)
	assert.NotEmpty(t, result.Error)
	assert.Equal(t, "permission denied", string(result.Body))
}

func TestMetricsSnapshot_Structure(t *testing.T) {
	snapshot := &MetricsSnapshot{
		InvocationCounts: map[string]int64{
			"alice/my-app_1.0.0": 100,
			"bob/other-app_2.0.0": 50,
		},
		AverageDurations: map[string]float64{
			"alice/my-app_1.0.0": 0.123,
			"bob/other-app_2.0.0": 0.456,
		},
		MemoryUsage: map[string]int64{
			"alice/my-app_1.0.0": 1024 * 1024,
			"bob/other-app_2.0.0": 2048 * 1024,
		},
	}

	assert.Equal(t, int64(100), snapshot.InvocationCounts["alice/my-app_1.0.0"])
	assert.Equal(t, 0.123, snapshot.AverageDurations["alice/my-app_1.0.0"])
	assert.Equal(t, int64(1024*1024), snapshot.MemoryUsage["alice/my-app_1.0.0"])
}

func TestCacheEventType_Constants(t *testing.T) {
	assert.Equal(t, CacheEventType("hit"), CacheHit)
	assert.Equal(t, CacheEventType("miss"), CacheMiss)
	assert.Equal(t, CacheEventType("eviction"), CacheEviction)
}
