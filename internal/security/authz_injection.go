package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/wazeos/wazeos/internal/types"
)

// AuthzInjectionLayer wraps a ResourceBus and injects credentials automatically
type AuthzInjectionLayer struct {
	bus     types.ResourceBus
	drivers map[string]string // scheme -> driver ID mapping
}

// NewAuthzInjectionLayer creates a new authz injection layer
func NewAuthzInjectionLayer(bus types.ResourceBus) *AuthzInjectionLayer {
	return &AuthzInjectionLayer{
		bus: bus,
		drivers: map[string]string{
			"s3":     "io.resource.s3",
			"secret": "kernel.security.secrets",
		},
	}
}

// Call intercepts resource calls and injects credentials
func (a *AuthzInjectionLayer) Call(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	fmt.Printf("[AUTHZ] Intercepted call: %s %s\n", call.Method, call.URI)

	// Parse URI to determine driver
	parsed, err := url.Parse(call.URI)
	if err != nil {
		fmt.Printf("[AUTHZ] ❌ Invalid URI: %v\n", err)
		return &types.ResourceResult{
			StatusCode: 400,
			Body:       []byte(fmt.Sprintf("invalid URI: %v", err)),
		}, nil
	}

	// Determine driver ID from scheme
	driverID, ok := a.drivers[parsed.Scheme]
	if !ok {
		fmt.Printf("[AUTHZ] ⚠️  No driver mapping for scheme '%s', passing through\n", parsed.Scheme)
		// No known driver, pass through without injection
		return a.bus.Call(ctx, call)
	}

	fmt.Printf("[AUTHZ] Resolved scheme '%s' → driver '%s'\n", parsed.Scheme, driverID)

	// Don't inject credentials for secret:// calls (would be recursive)
	if parsed.Scheme == "secret" {
		fmt.Printf("[AUTHZ] Skipping credential injection for secret:// (would be recursive)\n")
		return a.bus.Call(ctx, call)
	}

	// Query secrets for this driver
	fmt.Printf("[AUTHZ] Querying secrets with prefix: %s\n", driverID)
	credentials, err := a.queryCredentials(ctx, driverID, call.URI)
	if err != nil {
		// Log but don't fail - driver might not need credentials
		fmt.Printf("[AUTHZ] ⚠️  Failed to query credentials: %v\n", err)
	} else {
		fmt.Printf("[AUTHZ] Found %d credential parameters\n", len(credentials))
		for key := range credentials {
			fmt.Printf("[AUTHZ]   • %s: ****** (redacted)\n", key)
		}
	}

	// Inject credentials as header if we got any
	if len(credentials) > 0 {
		if call.Headers == nil {
			call.Headers = make(map[string]string)
		}
		credsJSON, _ := json.Marshal(credentials)
		call.Headers["X-WazeOS-Credentials"] = string(credsJSON)
		fmt.Printf("[AUTHZ] ✓ Injected %d bytes of credentials as X-WazeOS-Credentials header\n", len(credsJSON))
	}

	// Forward to driver
	fmt.Printf("[AUTHZ] Forwarding to resource bus...\n")
	result, err := a.bus.Call(ctx, call)
	if err != nil {
		fmt.Printf("[AUTHZ] ❌ Call failed: %v\n", err)
	} else {
		fmt.Printf("[AUTHZ] ✓ Call completed with status %d\n", result.StatusCode)
	}

	return result, err
}

// queryCredentials queries the secrets store for driver credentials
func (a *AuthzInjectionLayer) queryCredentials(ctx context.Context, driverID, targetURI string) (map[string]string, error) {
	// Build MATCH query URI
	prefix := fmt.Sprintf("driver.%s", driverID)
	matchURI := fmt.Sprintf("secret:///?prefix=%s", url.QueryEscape(prefix))

	// Call secrets driver
	matchCall := &types.ResourceCall{
		URI:    matchURI,
		Method: "MATCH",
	}

	result, err := a.bus.Call(ctx, matchCall)
	if err != nil {
		return nil, err
	}

	if result.StatusCode != 200 {
		return nil, fmt.Errorf("secrets query failed: %d", result.StatusCode)
	}

	// Parse response
	var response struct {
		Matches []struct {
			Key   string      `json:"key"`
			Value interface{} `json:"value"`
		} `json:"matches"`
	}

	if err := json.Unmarshal(result.Body, &response); err != nil {
		return nil, err
	}

	// Extract parameter names by stripping prefix
	credentials := make(map[string]string)
	prefixWithDot := prefix + "."

	for _, match := range response.Matches {
		// Strip "driver.io.resource.s3." to get parameter name
		paramName := strings.TrimPrefix(match.Key, prefixWithDot)

		// Convert value to string
		if valueStr, ok := match.Value.(string); ok {
			credentials[paramName] = valueStr
		}
	}

	return credentials, nil
}

// RegisterDriver registers a driver with the underlying bus
func (a *AuthzInjectionLayer) RegisterDriver(driver types.ResourceDriver) error {
	return a.bus.RegisterDriver(driver)
}

// RegisterScheme registers a URI scheme -> driver ID mapping
func (a *AuthzInjectionLayer) RegisterScheme(scheme, driverID string) {
	a.drivers[scheme] = driverID
}
