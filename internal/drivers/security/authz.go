package security

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// Authz implements types.SecurityAuthz with in-memory permission storage.
type Authz struct {
	mu          sync.RWMutex
	permissions map[string]*types.PermissionContext // principal -> permissions
}

// NewAuthz creates a new authorization driver.
func NewAuthz() *Authz {
	return &Authz{
		permissions: make(map[string]*types.PermissionContext),
	}
}

// Name returns the driver class.
func (a *Authz) Name() string {
	return "kernel.security.authz"
}

// GetPermissions returns the permission context for a principal.
func (a *Authz) GetPermissions(ctx context.Context, principal string) (*types.PermissionContext, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	permissions, exists := a.permissions[principal]
	if !exists {
		// Return empty permission context if no permissions set
		return types.NewPermissionContext([]types.PermissionEntry{}), nil
	}

	// Return a copy to prevent external modification
	entries := make([]types.PermissionEntry, len(permissions.Entries))
	copy(entries, permissions.Entries)

	return types.NewPermissionContext(entries), nil
}

// SetPermissions updates the permission context for a principal.
func (a *Authz) SetPermissions(ctx context.Context, principal string, permissions *types.PermissionContext) error {
	if principal == "" {
		return fmt.Errorf("principal cannot be empty")
	}

	if permissions == nil {
		return fmt.Errorf("permissions cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Store a copy to prevent external modification
	entries := make([]types.PermissionEntry, len(permissions.Entries))
	copy(entries, permissions.Entries)

	a.permissions[principal] = types.NewPermissionContext(entries)

	return nil
}

// CheckAccess validates if a URI access is allowed by the permission context.
func (a *Authz) CheckAccess(uri string, requiredPermissions []string, permissions *types.PermissionContext) error {
	if permissions == nil {
		return types.ErrPermissionDenied
	}

	// Check each permission entry
	for _, entry := range permissions.Entries {
		if matchURI(uri, entry.URIPattern) {
			// Check if all required permissions are present
			hasAll := true
			for _, required := range requiredPermissions {
				found := false
				for _, perm := range entry.Permissions {
					if perm == required {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if hasAll {
				return nil // Access granted
			}
		}
	}

	return types.ErrPermissionDenied
}

// matchURI checks if a URI matches a pattern with wildcard support.
// Patterns can use * as a wildcard that matches any characters.
func matchURI(uri, pattern string) bool {
	// Exact match
	if uri == pattern {
		return true
	}

	// Wildcard match all
	if pattern == "*" {
		return true
	}

	// No wildcards - must be exact match (already checked above)
	if !strings.Contains(pattern, "*") {
		return false
	}

	// Split pattern and URI by scheme
	uriScheme, uriRest := splitScheme(uri)
	patternScheme, patternRest := splitScheme(pattern)

	// Scheme must match exactly (if pattern specifies one)
	if patternScheme != "" && uriScheme != patternScheme {
		return false
	}

	// Match the rest using glob-style matching
	return matchGlob(uriRest, patternRest)
}

// splitScheme splits a URI into scheme and the rest.
// Returns ("scheme", "//host/path") or ("", "original") if no scheme.
func splitScheme(uri string) (string, string) {
	idx := strings.Index(uri, "://")
	if idx == -1 {
		return "", uri
	}
	return uri[:idx], uri[idx+3:]
}

// matchGlob performs glob-style wildcard matching.
// Supports * wildcard that matches any sequence of characters.
func matchGlob(s, pattern string) bool {
	// Fast path: exact match
	if s == pattern {
		return true
	}

	// Wildcard matches everything
	if pattern == "*" {
		return true
	}

	// No wildcards - must be exact match
	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	// Handle path-style matching for better specificity
	// If pattern looks like a path pattern, use path matching
	if strings.Contains(pattern, "/") {
		return matchPath(s, pattern)
	}

	// General glob matching
	return matchGlobGeneral(s, pattern)
}

// matchPath matches URI paths with wildcard support.
// Examples:
//   - "/data/*" matches "/data/file.txt", "/data/dir/file.txt"
//   - "/data/*/file" matches "/data/foo/file", "/data/bar/file"
func matchPath(uriPath, pattern string) bool {
	// Special case: pattern ends with /* means match anything under that path
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(uriPath, prefix)
	}

	// Split into segments
	uriSegments := strings.Split(strings.Trim(uriPath, "/"), "/")
	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")

	// Must have same number of segments unless pattern ends with *
	if len(uriSegments) != len(patternSegments) {
		// Check if pattern has trailing *
		if len(patternSegments) > 0 && patternSegments[len(patternSegments)-1] == "*" {
			// Pattern has fewer segments + wildcard, check prefix match
			if len(uriSegments) < len(patternSegments)-1 {
				return false
			}
			// Check segments up to the wildcard
			for i := 0; i < len(patternSegments)-1; i++ {
				if !matchGlobGeneral(uriSegments[i], patternSegments[i]) {
					return false
				}
			}
			return true
		}
		return false
	}

	// Match each segment
	for i := 0; i < len(patternSegments); i++ {
		if !matchGlobGeneral(uriSegments[i], patternSegments[i]) {
			return false
		}
	}

	return true
}

// matchGlobGeneral performs general glob matching with * wildcard.
func matchGlobGeneral(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	// Use path.Match for glob matching
	matched, err := path.Match(pattern, s)
	if err != nil {
		return false
	}

	return matched
}
