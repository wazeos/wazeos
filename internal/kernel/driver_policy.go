package kernel

import (
	"fmt"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// defaultPolicyRegistry implements types.DriverPolicyRegistry.
type defaultPolicyRegistry struct {
	mu       sync.RWMutex
	policies map[string]*types.DriverClassPolicy
}

// NewDriverPolicyRegistry creates a new driver policy registry with default policies.
func NewDriverPolicyRegistry() types.DriverPolicyRegistry {
	registry := &defaultPolicyRegistry{
		policies: make(map[string]*types.DriverClassPolicy),
	}

	// Register default policies for known driver classes
	defaultPolicies := []types.DriverClassPolicy{
		{
			Class:       "io.bus",
			Cardinality: types.CardinalityOne,
			Requirement: types.RequirementRequired,
			Description: "Core I/O routing bus - exactly one required for system operation",
		},
		{
			Class:       "io.resource",
			Cardinality: types.CardinalityMany,
			Requirement: types.RequirementOptional,
			Description: "Resource drivers for external I/O - multiple allowed, not required",
		},
		{
			Class:       "io.request",
			Cardinality: types.CardinalityMany,
			Requirement: types.RequirementRequired,
			Description: "Request drivers for inbound requests - at least one required",
		},
		{
			Class:       "runtime.exec",
			Cardinality: types.CardinalityOne,
			Requirement: types.RequirementRequired,
			Description: "Runtime execution engine - exactly one required",
		},
		{
			Class:       "pkg.install",
			Cardinality: types.CardinalityOne,
			Requirement: types.RequirementRequired,
			Description: "Package manager - exactly one required",
		},
		{
			Class:       "security.authz",
			Cardinality: types.CardinalityOne,
			Requirement: types.RequirementRequired,
			Description: "Authorization engine - exactly one required",
		},
	}

	for _, policy := range defaultPolicies {
		registry.RegisterPolicy(policy)
	}

	return registry
}

// RegisterPolicy registers a policy for a driver class.
func (r *defaultPolicyRegistry) RegisterPolicy(policy types.DriverClassPolicy) error {
	if policy.Class == "" {
		return fmt.Errorf("driver class cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.policies[policy.Class] = &policy
	return nil
}

// GetPolicy returns the policy for a driver class, or nil if not registered.
func (r *defaultPolicyRegistry) GetPolicy(class string) *types.DriverClassPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.policies[class]
}

// ValidateDriverCount checks if the current driver count satisfies the policy.
func (r *defaultPolicyRegistry) ValidateDriverCount(class string, count int) error {
	policy := r.GetPolicy(class)
	if policy == nil {
		// No policy registered - allow any count
		return nil
	}

	switch policy.Cardinality {
	case types.CardinalityOne:
		if count > 1 {
			return fmt.Errorf("driver class %q requires exactly one driver, but %d are registered", class, count)
		}
		if count == 0 && policy.Requirement == types.RequirementRequired {
			return fmt.Errorf("driver class %q is required but no drivers are registered", class)
		}
	case types.CardinalityMany:
		if count == 0 && policy.Requirement == types.RequirementRequired {
			return fmt.Errorf("driver class %q is required but no drivers are registered", class)
		}
	}

	return nil
}

// GetRequiredClasses returns all required driver classes.
func (r *defaultPolicyRegistry) GetRequiredClasses() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	required := make([]string, 0)
	for class, policy := range r.policies {
		if policy.Requirement == types.RequirementRequired {
			required = append(required, class)
		}
	}

	return required
}
