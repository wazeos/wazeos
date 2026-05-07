package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

func TestNewDriverPolicyRegistry(t *testing.T) {
	registry := NewDriverPolicyRegistry()
	require.NotNil(t, registry)

	// Verify default policies are registered
	policy := registry.GetPolicy("io.bus")
	require.NotNil(t, policy)
	assert.Equal(t, types.CardinalityOne, policy.Cardinality)
	assert.Equal(t, types.RequirementRequired, policy.Requirement)

	policy = registry.GetPolicy("io.resource")
	require.NotNil(t, policy)
	assert.Equal(t, types.CardinalityMany, policy.Cardinality)
	assert.Equal(t, types.RequirementOptional, policy.Requirement)
}

func TestDriverPolicyRegistry_RegisterPolicy(t *testing.T) {
	registry := NewDriverPolicyRegistry()

	policy := types.DriverClassPolicy{
		Class:       "test.driver",
		Cardinality: types.CardinalityOne,
		Requirement: types.RequirementOptional,
		Description: "Test driver",
	}

	err := registry.RegisterPolicy(policy)
	assert.NoError(t, err)

	retrieved := registry.GetPolicy("test.driver")
	require.NotNil(t, retrieved)
	assert.Equal(t, "test.driver", retrieved.Class)
	assert.Equal(t, types.CardinalityOne, retrieved.Cardinality)
}

func TestDriverPolicyRegistry_RegisterPolicy_EmptyClass(t *testing.T) {
	registry := NewDriverPolicyRegistry()

	policy := types.DriverClassPolicy{
		Cardinality: types.CardinalityOne,
		Requirement: types.RequirementOptional,
	}

	err := registry.RegisterPolicy(policy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "class cannot be empty")
}

func TestDriverPolicyRegistry_ValidateDriverCount_CardinalityOne(t *testing.T) {
	registry := NewDriverPolicyRegistry()

	policy := types.DriverClassPolicy{
		Class:       "test.one",
		Cardinality: types.CardinalityOne,
		Requirement: types.RequirementRequired,
	}
	registry.RegisterPolicy(policy)

	// Zero drivers - should fail for required
	err := registry.ValidateDriverCount("test.one", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required but no drivers")

	// One driver - should pass
	err = registry.ValidateDriverCount("test.one", 1)
	assert.NoError(t, err)

	// Two drivers - should fail (cardinality one)
	err = registry.ValidateDriverCount("test.one", 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one driver")
}

func TestDriverPolicyRegistry_ValidateDriverCount_CardinalityMany(t *testing.T) {
	registry := NewDriverPolicyRegistry()

	policy := types.DriverClassPolicy{
		Class:       "test.many",
		Cardinality: types.CardinalityMany,
		Requirement: types.RequirementRequired,
	}
	registry.RegisterPolicy(policy)

	// Zero drivers - should fail for required
	err := registry.ValidateDriverCount("test.many", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required but no drivers")

	// One driver - should pass
	err = registry.ValidateDriverCount("test.many", 1)
	assert.NoError(t, err)

	// Multiple drivers - should pass (cardinality many)
	err = registry.ValidateDriverCount("test.many", 5)
	assert.NoError(t, err)
}

func TestDriverPolicyRegistry_ValidateDriverCount_Optional(t *testing.T) {
	registry := NewDriverPolicyRegistry()

	policy := types.DriverClassPolicy{
		Class:       "test.optional",
		Cardinality: types.CardinalityMany,
		Requirement: types.RequirementOptional,
	}
	registry.RegisterPolicy(policy)

	// Zero drivers - should pass for optional
	err := registry.ValidateDriverCount("test.optional", 0)
	assert.NoError(t, err)

	// Multiple drivers - should pass
	err = registry.ValidateDriverCount("test.optional", 3)
	assert.NoError(t, err)
}

func TestDriverPolicyRegistry_ValidateDriverCount_NoPolicy(t *testing.T) {
	registry := NewDriverPolicyRegistry()

	// No policy registered - should allow any count
	err := registry.ValidateDriverCount("unknown.class", 0)
	assert.NoError(t, err)

	err = registry.ValidateDriverCount("unknown.class", 100)
	assert.NoError(t, err)
}

func TestDriverPolicyRegistry_GetRequiredClasses(t *testing.T) {
	registry := NewDriverPolicyRegistry()

	required := registry.GetRequiredClasses()
	require.NotEmpty(t, required)

	// Verify default required classes are present
	assert.Contains(t, required, "io.bus")
	assert.Contains(t, required, "io.request")
	assert.Contains(t, required, "kernel.runtime.exec")

	// Optional classes should not be in the list
	assert.NotContains(t, required, "io.resource")
	assert.NotContains(t, required, "kernel.ipc")
}
