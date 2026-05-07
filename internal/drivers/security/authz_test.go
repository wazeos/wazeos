package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

func TestNewAuthz(t *testing.T) {
	authz := NewAuthz()
	assert.NotNil(t, authz)
	assert.NotNil(t, authz.permissions)
}

func TestAuthz_Name(t *testing.T) {
	authz := NewAuthz()
	assert.Equal(t, "kernel.security.authz", authz.Name())
}

func TestAuthz_SetPermissions(t *testing.T) {
	authz := NewAuthz()
	ctx := context.Background()

	permissions := types.NewPermissionContext([]types.PermissionEntry{
		{URIPattern: "file:///data/*", Access: types.AccessRead | types.AccessWrite},
		{URIPattern: "http://api.example.com/*", Access: types.AccessRead},
	})

	err := authz.SetPermissions(ctx, "user:alice", permissions)
	assert.NoError(t, err)

	// Verify permissions were stored
	retrieved, err := authz.GetPermissions(ctx, "user:alice")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(retrieved.Entries))
}

func TestAuthz_SetPermissions_EmptyPrincipal(t *testing.T) {
	authz := NewAuthz()
	ctx := context.Background()

	permissions := types.NewPermissionContext([]types.PermissionEntry{})

	err := authz.SetPermissions(ctx, "", permissions)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "principal cannot be empty")
}

func TestAuthz_SetPermissions_NilPermissions(t *testing.T) {
	authz := NewAuthz()
	ctx := context.Background()

	err := authz.SetPermissions(ctx, "user:alice", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permissions cannot be nil")
}

func TestAuthz_GetPermissions_NotFound(t *testing.T) {
	authz := NewAuthz()
	ctx := context.Background()

	permissions, err := authz.GetPermissions(ctx, "user:unknown")
	assert.NoError(t, err)
	assert.NotNil(t, permissions)
	assert.Equal(t, 0, len(permissions.Entries))
}

func TestAuthz_GetPermissions_ReturnsCopy(t *testing.T) {
	authz := NewAuthz()
	ctx := context.Background()

	original := types.NewPermissionContext([]types.PermissionEntry{
		{URIPattern: "file:///data/*", Access: types.AccessRead},
	})

	err := authz.SetPermissions(ctx, "user:alice", original)
	require.NoError(t, err)

	// Get permissions and modify
	retrieved, err := authz.GetPermissions(ctx, "user:alice")
	require.NoError(t, err)

	// Modify retrieved permissions
	retrieved.Entries[0].Access = types.AccessWrite

	// Get permissions again and verify original wasn't modified
	retrieved2, err := authz.GetPermissions(ctx, "user:alice")
	require.NoError(t, err)
	assert.Equal(t, types.AccessRead, retrieved2.Entries[0].Access)
}

func TestAuthz_CheckAccess_Allowed(t *testing.T) {
	authz := NewAuthz()

	permissions := types.NewPermissionContext([]types.PermissionEntry{
		{URIPattern: "file:///data/*", Access: types.AccessRead | types.AccessWrite},
		{URIPattern: "http://api.example.com/*", Access: types.AccessRead},
		{URIPattern: "fn://my-app/*", Access: types.AccessExecute},
	})

	tests := []struct {
		name string
		uri  string
		mode types.AccessBits
	}{
		{
			name: "file read allowed",
			uri:  "file:///data/test.txt",
			mode: types.AccessRead,
		},
		{
			name: "file write allowed",
			uri:  "file:///data/test.txt",
			mode: types.AccessWrite,
		},
		{
			name: "file read+write allowed",
			uri:  "file:///data/test.txt",
			mode: types.AccessRead | types.AccessWrite,
		},
		{
			name: "http read allowed",
			uri:  "http://api.example.com/v1/users",
			mode: types.AccessRead,
		},
		{
			name: "fn execute allowed",
			uri:  "fn://my-app/arg1",
			mode: types.AccessExecute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authz.CheckAccess(tt.uri, tt.mode, permissions)
			assert.NoError(t, err)
		})
	}
}

func TestAuthz_CheckAccess_Denied(t *testing.T) {
	authz := NewAuthz()

	permissions := types.NewPermissionContext([]types.PermissionEntry{
		{URIPattern: "file:///data/*", Access: types.AccessRead},
		{URIPattern: "http://api.example.com/*", Access: types.AccessRead},
	})

	tests := []struct {
		name string
		uri  string
		mode types.AccessBits
	}{
		{
			name: "file write denied",
			uri:  "file:///data/test.txt",
			mode: types.AccessWrite,
		},
		{
			name: "file execute denied",
			uri:  "file:///data/test.txt",
			mode: types.AccessExecute,
		},
		{
			name: "http write denied",
			uri:  "http://api.example.com/v1/users",
			mode: types.AccessWrite,
		},
		{
			name: "no matching pattern",
			uri:  "s3://bucket/object",
			mode: types.AccessRead,
		},
		{
			name: "different path",
			uri:  "file:///other/test.txt",
			mode: types.AccessRead,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authz.CheckAccess(tt.uri, tt.mode, permissions)
			assert.Error(t, err)
			assert.True(t, types.IsPermissionDenied(err))
		})
	}
}

func TestAuthz_CheckAccess_NilPermissions(t *testing.T) {
	authz := NewAuthz()

	err := authz.CheckAccess("file:///data/test.txt", types.AccessRead, nil)
	assert.Error(t, err)
	assert.True(t, types.IsPermissionDenied(err))
}

func TestMatchURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		pattern string
		want    bool
	}{
		{
			name:    "exact match",
			uri:     "file:///data/test.txt",
			pattern: "file:///data/test.txt",
			want:    true,
		},
		{
			name:    "wildcard match all",
			uri:     "anything",
			pattern: "*",
			want:    true,
		},
		{
			name:    "file pattern match",
			uri:     "file:///data/test.txt",
			pattern: "file:///data/*",
			want:    true,
		},
		{
			name:    "file pattern nested",
			uri:     "file:///data/dir/test.txt",
			pattern: "file:///data/*",
			want:    true,
		},
		{
			name:    "http pattern match",
			uri:     "http://api.example.com/v1/users",
			pattern: "http://api.example.com/*",
			want:    true,
		},
		{
			name:    "fn pattern match",
			uri:     "fn://my-app/arg1/arg2",
			pattern: "fn://my-app/*",
			want:    true,
		},
		{
			name:    "different scheme",
			uri:     "https://example.com/test",
			pattern: "http://example.com/*",
			want:    false,
		},
		{
			name:    "different path prefix",
			uri:     "file:///other/test.txt",
			pattern: "file:///data/*",
			want:    false,
		},
		{
			name:    "no wildcard no match",
			uri:     "file:///data/test.txt",
			pattern: "file:///data/other.txt",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchURI(tt.uri, tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{
			name:    "exact match",
			path:    "/data/test",
			pattern: "/data/test",
			want:    true,
		},
		{
			name:    "trailing wildcard",
			path:    "/data/file.txt",
			pattern: "/data/*",
			want:    true,
		},
		{
			name:    "trailing wildcard nested",
			path:    "/data/dir/file.txt",
			pattern: "/data/*",
			want:    true,
		},
		{
			name:    "segment wildcard",
			path:    "/data/anything/file",
			pattern: "/data/*/file",
			want:    true,
		},
		{
			name:    "multiple wildcards",
			path:    "/one/two/three",
			pattern: "/*/*/three",
			want:    true,
		},
		{
			name:    "different segment count",
			path:    "/data/test",
			pattern: "/data/test/extra",
			want:    false,
		},
		{
			name:    "no match",
			path:    "/data/file",
			pattern: "/other/*",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPath(tt.path, tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchGlobGeneral(t *testing.T) {
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
			name:    "wildcard all",
			s:       "anything",
			pattern: "*",
			want:    true,
		},
		{
			name:    "prefix wildcard",
			s:       "test.txt",
			pattern: "*.txt",
			want:    true,
		},
		{
			name:    "suffix wildcard",
			s:       "test.txt",
			pattern: "test.*",
			want:    true,
		},
		{
			name:    "middle wildcard",
			s:       "test-file.txt",
			pattern: "test-*.txt",
			want:    true,
		},
		{
			name:    "no match",
			s:       "test",
			pattern: "other",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGlobGeneral(tt.s, tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitScheme(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantScheme  string
		wantRest    string
	}{
		{
			name:       "file URI",
			uri:        "file:///data/test.txt",
			wantScheme: "file",
			wantRest:   "/data/test.txt",
		},
		{
			name:       "http URI",
			uri:        "http://example.com/path",
			wantScheme: "http",
			wantRest:   "example.com/path",
		},
		{
			name:       "no scheme",
			uri:        "/data/test.txt",
			wantScheme: "",
			wantRest:   "/data/test.txt",
		},
		{
			name:       "fn URI",
			uri:        "fn://my-app/arg1",
			wantScheme: "fn",
			wantRest:   "my-app/arg1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotScheme, gotRest := splitScheme(tt.uri)
			assert.Equal(t, tt.wantScheme, gotScheme)
			assert.Equal(t, tt.wantRest, gotRest)
		})
	}
}

func TestAuthz_ConcurrentAccess(t *testing.T) {
	authz := NewAuthz()
	ctx := context.Background()

	// Test concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			permissions := types.NewPermissionContext([]types.PermissionEntry{
				{URIPattern: "file:///data/*", Access: types.AccessRead},
			})
			_ = authz.SetPermissions(ctx, "user:test", permissions)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = authz.GetPermissions(ctx, "user:test")
		}
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done
}
