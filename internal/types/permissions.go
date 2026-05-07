package types

import (
	"fmt"
	"sync"
)

// PermissionDefinition defines a single permission for a driver.
type PermissionDefinition struct {
	Name        string `json:"name"`        // e.g., "read", "GET", "produce"
	Description string `json:"description"` // Human-readable description
	Bit         uint64 `json:"bit"`         // Bit position (0-63)
}

// PermissionSchema defines the permission model for a driver class.
type PermissionSchema struct {
	DriverClass string                 `json:"driverClass"` // e.g., "io.resource.http"
	Permissions []PermissionDefinition `json:"permissions"`
	nameToBit   map[string]uint64      // Internal lookup cache
}

// NewPermissionSchema creates a permission schema from definitions.
func NewPermissionSchema(driverClass string, permissions []PermissionDefinition) *PermissionSchema {
	schema := &PermissionSchema{
		DriverClass: driverClass,
		Permissions: permissions,
		nameToBit:   make(map[string]uint64),
	}

	for _, perm := range permissions {
		schema.nameToBit[perm.Name] = perm.Bit
	}

	return schema
}

// GetBit returns the bit value for a permission name.
func (s *PermissionSchema) GetBit(name string) (uint64, bool) {
	bit, ok := s.nameToBit[name]
	return bit, ok
}

// HasPermission checks if the given access bits contain the named permission.
func (s *PermissionSchema) HasPermission(access uint64, permissionName string) bool {
	bit, ok := s.nameToBit[permissionName]
	if !ok {
		return false
	}
	return (access & bit) != 0
}

// ParsePermissions converts permission names to an access bits value.
func (s *PermissionSchema) ParsePermissions(names []string) (uint64, error) {
	var access uint64
	for _, name := range names {
		bit, ok := s.nameToBit[name]
		if !ok {
			return 0, fmt.Errorf("unknown permission: %s for driver %s", name, s.DriverClass)
		}
		access |= bit
	}
	return access, nil
}

// GetPermissionNames returns the names of all permissions set in the access bits.
func (s *PermissionSchema) GetPermissionNames(access uint64) []string {
	names := make([]string, 0)
	for _, perm := range s.Permissions {
		if (access & perm.Bit) != 0 {
			names = append(names, perm.Name)
		}
	}
	return names
}

// PermissionRegistry manages permission schemas for all driver classes.
type PermissionRegistry struct {
	mu      sync.RWMutex
	schemas map[string]*PermissionSchema // driverClass -> schema
}

var (
	globalPermissionRegistry = &PermissionRegistry{
		schemas: make(map[string]*PermissionSchema),
	}
)

// RegisterPermissionSchema registers a permission schema for a driver class.
func RegisterPermissionSchema(schema *PermissionSchema) error {
	return globalPermissionRegistry.Register(schema)
}

// GetPermissionSchema retrieves the permission schema for a driver class.
func GetPermissionSchema(driverClass string) (*PermissionSchema, bool) {
	return globalPermissionRegistry.Get(driverClass)
}

// Register registers a permission schema.
func (r *PermissionRegistry) Register(schema *PermissionSchema) error {
	if schema == nil {
		return fmt.Errorf("schema cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.schemas[schema.DriverClass]; exists {
		return fmt.Errorf("permission schema for driver class %q already registered", schema.DriverClass)
	}

	r.schemas[schema.DriverClass] = schema
	return nil
}

// Get retrieves a permission schema by driver class.
func (r *PermissionRegistry) Get(driverClass string) (*PermissionSchema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schema, ok := r.schemas[driverClass]
	return schema, ok
}

// List returns all registered permission schemas.
func (r *PermissionRegistry) List() []*PermissionSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make([]*PermissionSchema, 0, len(r.schemas))
	for _, schema := range r.schemas {
		schemas = append(schemas, schema)
	}
	return schemas
}

// Standard permission definitions for common driver types.

// FilePermissions defines standard file system permissions.
var FilePermissions = NewPermissionSchema("io.resource.file", []PermissionDefinition{
	{Name: "read", Description: "Read file contents", Bit: 1 << 0},
	{Name: "write", Description: "Write/modify file contents", Bit: 1 << 1},
	{Name: "execute", Description: "Execute file", Bit: 1 << 2},
	{Name: "delete", Description: "Delete file", Bit: 1 << 3},
	{Name: "list", Description: "List directory contents", Bit: 1 << 4},
})

// HTTPPermissions defines HTTP method permissions.
var HTTPPermissions = NewPermissionSchema("io.resource.http", []PermissionDefinition{
	{Name: "GET", Description: "HTTP GET requests", Bit: 1 << 0},
	{Name: "POST", Description: "HTTP POST requests", Bit: 1 << 1},
	{Name: "PUT", Description: "HTTP PUT requests", Bit: 1 << 2},
	{Name: "DELETE", Description: "HTTP DELETE requests", Bit: 1 << 3},
	{Name: "PATCH", Description: "HTTP PATCH requests", Bit: 1 << 4},
	{Name: "OPTIONS", Description: "HTTP OPTIONS requests", Bit: 1 << 5},
	{Name: "HEAD", Description: "HTTP HEAD requests", Bit: 1 << 6},
})

// FnPermissions defines function call permissions.
var FnPermissions = NewPermissionSchema("io.resource.fn", []PermissionDefinition{
	{Name: "invoke", Description: "Invoke other apps", Bit: 1 << 0},
})

// IPCPermissions defines IPC queue permissions.
var IPCPermissions = NewPermissionSchema("kernel.ipc", []PermissionDefinition{
	{Name: "produce", Description: "Produce/send messages to queue", Bit: 1 << 0},
	{Name: "consume", Description: "Consume/receive messages from queue", Bit: 1 << 1},
	{Name: "create", Description: "Create new queues/topics", Bit: 1 << 2},
	{Name: "delete", Description: "Delete queues/topics", Bit: 1 << 3},
	{Name: "admin", Description: "Administrative operations", Bit: 1 << 4},
})

// AuditPermissions defines audit log permissions.
var AuditPermissions = NewPermissionSchema("kernel.security.audit", []PermissionDefinition{
	{Name: "read", Description: "Read audit logs", Bit: 1 << 0},
	{Name: "write", Description: "Write to audit log", Bit: 1 << 1},
	{Name: "export", Description: "Export audit logs", Bit: 1 << 2},
	{Name: "clear", Description: "Clear audit logs", Bit: 1 << 3},
	{Name: "admin", Description: "Administrative operations", Bit: 1 << 4},
})

func init() {
	// Register standard permission schemas
	RegisterPermissionSchema(FilePermissions)
	RegisterPermissionSchema(HTTPPermissions)
	RegisterPermissionSchema(FnPermissions)
	RegisterPermissionSchema(IPCPermissions)
	RegisterPermissionSchema(AuditPermissions)
}
