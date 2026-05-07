package types

// DriverCardinality defines how many drivers of a class can be registered.
type DriverCardinality string

const (
	// CardinalityOne requires exactly one driver of this class.
	CardinalityOne DriverCardinality = "one"

	// CardinalityMany allows multiple drivers of this class.
	CardinalityMany DriverCardinality = "many"
)

// DriverRequirement defines whether a driver class is required for startup.
type DriverRequirement string

const (
	// RequirementRequired means the driver class is required for base functionality.
	RequirementRequired DriverRequirement = "required"

	// RequirementOptional means the driver class is not required for base functionality.
	RequirementOptional DriverRequirement = "optional"
)

// DriverClassPolicy defines the startup policy for a driver class.
type DriverClassPolicy struct {
	// Class is the driver class name (e.g., "io.bus", "io.resource")
	Class string

	// Cardinality defines how many drivers of this class can exist.
	Cardinality DriverCardinality

	// Requirement defines whether this driver class is required.
	Requirement DriverRequirement

	// Description provides human-readable context about this driver class.
	Description string
}

// DriverPolicyRegistry manages policies for all driver classes.
type DriverPolicyRegistry interface {
	// RegisterPolicy registers a policy for a driver class.
	RegisterPolicy(policy DriverClassPolicy) error

	// GetPolicy returns the policy for a driver class, or nil if not registered.
	GetPolicy(class string) *DriverClassPolicy

	// ValidateDriverCount checks if the current driver count satisfies the policy.
	ValidateDriverCount(class string, count int) error

	// GetRequiredClasses returns all required driver classes.
	GetRequiredClasses() []string
}
