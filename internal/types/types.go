package types

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AccessBits represents permissions as a bitfield.
// Expanded from uint8 to uint64 to support driver-specific permissions.
type AccessBits uint64

const (
	AccessRead    AccessBits = 1 << 0 // 0x01
	AccessWrite   AccessBits = 1 << 1 // 0x02
	AccessExecute AccessBits = 1 << 2 // 0x04
)

// String returns a string representation of access bits (e.g., "rw", "rx", "rwx").
func (a AccessBits) String() string {
	var result string
	if a&AccessRead != 0 {
		result += "r"
	}
	if a&AccessWrite != 0 {
		result += "w"
	}
	if a&AccessExecute != 0 {
		result += "x"
	}
	if result == "" {
		return "-"
	}
	return result
}

// ParseAccessBits parses a string like "rw", "rx", "rwx" into AccessBits.
func ParseAccessBits(s string) (AccessBits, error) {
	var bits AccessBits
	for _, c := range s {
		switch c {
		case 'r':
			bits |= AccessRead
		case 'w':
			bits |= AccessWrite
		case 'x':
			bits |= AccessExecute
		default:
			return 0, fmt.Errorf("invalid access character: %c", c)
		}
	}
	return bits, nil
}

// Has checks if the access bits include the specified permission.
func (a AccessBits) Has(permission AccessBits) bool {
	return a&permission == permission
}

// PermissionEntry represents a single URI-based access control entry.
type PermissionEntry struct {
	URIPattern string     // URI pattern with wildcard support (e.g., "file:///data/*")
	Access     AccessBits // Allowed access bits
}

// SetNamedPermissions sets permissions using driver-specific permission names.
// The driver class is inferred from the URI pattern scheme.
// Example: entry.SetNamedPermissions([]string{"GET", "POST"}) for "https://api.example.com/*"
func (e *PermissionEntry) SetNamedPermissions(names []string) error {
	// Extract driver class from URI pattern
	driverClass := inferDriverClass(e.URIPattern)
	if driverClass == "" {
		return fmt.Errorf("cannot infer driver class from URI pattern: %s", e.URIPattern)
	}

	schema, ok := GetPermissionSchema(driverClass)
	if !ok {
		// Fallback to standard rwx if no schema registered
		if len(names) == 0 {
			return fmt.Errorf("no permissions specified")
		}
		access, err := ParseAccessBits(names[0])
		if err != nil {
			return fmt.Errorf("unknown driver class %s and cannot parse as standard access: %w", driverClass, err)
		}
		e.Access = access
		return nil
	}

	access, err := schema.ParsePermissions(names)
	if err != nil {
		return err
	}

	e.Access = AccessBits(access)
	return nil
}

// HasNamedPermission checks if a specific named permission is granted.
func (e *PermissionEntry) HasNamedPermission(permissionName string) bool {
	driverClass := inferDriverClass(e.URIPattern)
	if driverClass == "" {
		return false
	}

	schema, ok := GetPermissionSchema(driverClass)
	if !ok {
		return false
	}

	return schema.HasPermission(uint64(e.Access), permissionName)
}

// GetPermissionNames returns the list of permission names granted by this entry.
func (e *PermissionEntry) GetPermissionNames() []string {
	driverClass := inferDriverClass(e.URIPattern)
	if driverClass == "" {
		return []string{}
	}

	schema, ok := GetPermissionSchema(driverClass)
	if !ok {
		// Fallback to standard rwx format
		return []string{e.Access.String()}
	}

	return schema.GetPermissionNames(uint64(e.Access))
}

// inferDriverClass extracts the driver class from a URI pattern.
// Examples:
//   - "file:///data/*" -> "io.resource.file"
//   - "https://api.example.com/*" -> "io.resource.http"
//   - "fn://app-name/*" -> "io.resource.fn"
//   - "queue://topic/*" -> "kernel.ipc"
func inferDriverClass(uriPattern string) string {
	if strings.HasPrefix(uriPattern, "file://") {
		return "io.resource.file"
	}
	if strings.HasPrefix(uriPattern, "http://") || strings.HasPrefix(uriPattern, "https://") {
		return "io.resource.http"
	}
	if strings.HasPrefix(uriPattern, "fn://") {
		return "io.resource.fn"
	}
	if strings.HasPrefix(uriPattern, "queue://") || strings.HasPrefix(uriPattern, "ipc://") {
		return "kernel.ipc"
	}
	return ""
}

// PermissionContext represents the set of permissions for a principal.
type PermissionContext struct {
	Entries []PermissionEntry
}

// NewPermissionContext creates a new permission context with the given entries.
func NewPermissionContext(entries []PermissionEntry) *PermissionContext {
	return &PermissionContext{Entries: entries}
}

// Intersect returns a new PermissionContext with the intersection of permissions.
// Used when chaining fn:// calls to reduce permissions at each hop.
func (pc *PermissionContext) Intersect(other *PermissionContext) *PermissionContext {
	// Simple implementation: only include entries that exist in both contexts
	// For MVP, we'll use exact pattern matching
	result := make([]PermissionEntry, 0)

	for _, entry1 := range pc.Entries {
		for _, entry2 := range other.Entries {
			if entry1.URIPattern == entry2.URIPattern {
				// Intersection of access bits
				intersectedAccess := entry1.Access & entry2.Access
				if intersectedAccess != 0 {
					result = append(result, PermissionEntry{
						URIPattern: entry1.URIPattern,
						Access:     intersectedAccess,
					})
				}
			}
		}
	}

	return &PermissionContext{Entries: result}
}

// ExecutionContext carries execution state through the driver pipeline.
type ExecutionContext struct {
	RequestID         string            // Unique ID for this invocation
	ParentRequestID   *string           // ID of calling invocation (nil for top-level)
	TraceID           string            // Distributed trace ID
	Principal         string            // Authenticated principal (e.g., "user:alice")
	PermissionContext *PermissionContext // Resolved permissions for this call chain
	Timestamp         time.Time         // Request timestamp
	Metadata          map[string]string // Additional context (e.g., headers, environment)
}

// NewExecutionContext creates a new execution context for a top-level request.
func NewExecutionContext(requestID, traceID, principal string, permissions *PermissionContext) *ExecutionContext {
	return &ExecutionContext{
		RequestID:         requestID,
		ParentRequestID:   nil,
		TraceID:           traceID,
		Principal:         principal,
		PermissionContext: permissions,
		Timestamp:         time.Now(),
		Metadata:          make(map[string]string),
	}
}

// ChildContext creates a child execution context for a nested fn:// call.
func (ec *ExecutionContext) ChildContext(newRequestID string, newPermissions *PermissionContext) *ExecutionContext {
	parentID := ec.RequestID
	return &ExecutionContext{
		RequestID:         newRequestID,
		ParentRequestID:   &parentID,
		TraceID:           ec.TraceID,
		Principal:         ec.Principal,
		PermissionContext: ec.PermissionContext.Intersect(newPermissions),
		Timestamp:         time.Now(),
		Metadata:          make(map[string]string),
	}
}

// WazeroPermissions represents wazero-specific capabilities.
type WazeroPermissions struct {
	Network   []string `json:"network,omitempty"`   // Network access: ["connect", "listen"]
	FS        []string `json:"fs,omitempty"`        // Filesystem access paths
	Env       []string `json:"env,omitempty"`       // Environment variables allowed
	Sockets   bool     `json:"sockets,omitempty"`   // Socket access
	StdIO     bool     `json:"stdio,omitempty"`     // DEPRECATED: Stdio is always enabled for all modules
}

// DriverPrivileges represents privileges granted TO drivers by wazero.
// These are system capabilities the driver requests.
type DriverPrivileges struct {
	Wazero        WazeroPermissions `json:"wazero"`
	HostFunctions []string          `json:"hostFunctions"` // Allowed host function namespaces
}

// PermissionDefinitionMetadata represents a permission definition in driver metadata.
// These define the permissions that the driver EXPOSES for access control.
type PermissionDefinitionMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Bit         uint64 `json:"bit"`
}

// AppMetadata represents parsed metadata from an app package.
type AppMetadata struct {
	Name          string                          `json:"name"`
	Version       string                          `json:"version"`
	Author        string                          `json:"author"`
	Description   string                          `json:"description,omitempty"`
	Type          string                          `json:"type,omitempty"`          // "app" or "driver" (default: "app")
	DriverClass   string                          `json:"driverClass,omitempty"`   // Driver class if type="driver"
	URIPatterns   []string                        `json:"uriPatterns,omitempty"`   // URI patterns this driver handles (e.g., ["file://*/*"])
	Dependencies  []string                        `json:"dependencies,omitempty"`  // Required packages (apps or drivers) specified as "author/name:version"
	Entrypoint    string                          `json:"entrypoint,omitempty"`    // Wasm entrypoint (default: "_start")
	Prerequisites []string                        `json:"prerequisites,omitempty"` // Packages (apps or drivers) auto-installed before this package
	Privileges    *DriverPrivileges               `json:"privileges,omitempty"`    // System privileges for drivers (wazero capabilities)
	Permissions   []PermissionDefinitionMetadata  `json:"permissions,omitempty"`   // Access control permissions exposed by drivers
	InputSchema   *json.RawMessage                `json:"inputSchema,omitempty"`   // MCP tool schema (JSON Schema format)
}

// AppID returns the canonical app identifier.
func (m *AppMetadata) AppID() string {
	return fmt.Sprintf("%s/%s:%s", m.Author, m.Name, m.Version)
}

// IsDriver returns true if this is a driver package.
func (m *AppMetadata) IsDriver() bool {
	return m.Type == "driver"
}

// GetEntrypoint returns the entrypoint with default fallback.
func (m *AppMetadata) GetEntrypoint() string {
	if m.Entrypoint != "" {
		return m.Entrypoint
	}
	return "_start"
}

// HasPermission checks if a host function is allowed.
func (m *AppMetadata) HasPermission(hostFunc string) bool {
	if m.Privileges == nil {
		return false
	}
	for _, allowed := range m.Privileges.HostFunctions {
		if allowed == hostFunc || allowed == "*" {
			return true
		}
	}
	return false
}

// InvocationRequest represents a request to execute a wasm app.
type InvocationRequest struct {
	Context *ExecutionContext
	AppID   string   // Target app identifier
	Args    []string // Command-line arguments to pass to wasm
}

// InvocationResult represents the result of wasm app execution.
type InvocationResult struct {
	RequestID  string        // Matches InvocationRequest.Context.RequestID
	Stdout     []byte        // Captured stdout
	Stderr     []byte        // Captured stderr
	ExitCode   int           // Wasm exit code
	Duration   time.Duration // Execution time
	MemoryUsed int64         // Peak linear memory in bytes
	Error      error         // Non-nil if execution failed
}

// ResourceCall represents an IO call from a wasm app to a resource driver.
type ResourceCall struct {
	Context    *ExecutionContext
	URI        string            // Target URI (e.g., "file:///tmp/data.txt")
	Method     string            // Operation method (e.g., "GET", "POST", "READ", "WRITE")
	Headers    map[string]string // Protocol-specific headers
	Body       []byte            // Request payload
	AccessMode AccessBits        // Required access (Read, Write, Execute)
}

// ResourceResult represents the result of a resource call.
type ResourceResult struct {
	StatusCode int               // Protocol-specific status (HTTP status, 0 for success, etc.)
	Headers    map[string]string // Response headers
	Body       []byte            // Response payload
	Error      error             // Non-nil if call failed
}

// AuthPayload represents authentication input from a request.
type AuthPayload struct {
	Headers map[string]string // HTTP headers (for Basic Auth, Bearer tokens, etc.)
	Body    []byte            // Request body (if needed)
}

// InvocationHandler is called by request drivers to dispatch work.
type InvocationHandler interface {
	// Invoke executes an app and returns the result.
	// Blocks until execution completes or times out.
	Invoke(ctx context.Context, req *InvocationRequest) (*InvocationResult, error)
}

// ResourceBus dispatches IO calls to registered resource drivers.
type ResourceBus interface {
	// Call routes a resource call to the appropriate driver.
	Call(ctx context.Context, call *ResourceCall) (*ResourceResult, error)

	// RegisterDriver adds a resource driver to the bus.
	RegisterDriver(driver ResourceDriver) error
}

// RequestDriver handles inbound MCP tool calls.
type RequestDriver interface {
	// Name returns the driver class (e.g., "io.request.http").
	Name() string

	// Patterns returns URI patterns this driver handles.
	Patterns() []string

	// Start begins listening for inbound requests.
	// Must not block - should spawn goroutines for background work.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the driver.
	Stop(ctx context.Context) error

	// SetInvoker provides the callback to dispatch invocations.
	SetInvoker(invoker InvocationHandler)
}

// ResourceDriver handles outbound IO calls from wasm apps.
type ResourceDriver interface {
	// Name returns the driver class (e.g., "io.resource.file").
	Name() string

	// Patterns returns URI patterns this driver handles.
	Patterns() []string

	// HandleCall processes an IO call from a wasm app.
	HandleCall(ctx context.Context, call *ResourceCall) (*ResourceResult, error)
}

// SecurityAuthn extracts principals from requests.
type SecurityAuthn interface {
	// Name returns the driver class (e.g., "kernel.security.authn.basic").
	Name() string

	// Authenticate attempts to extract a principal from the request payload.
	// Returns (principal, nil) on success.
	// Returns ("", ErrAbstain) if credentials not recognized.
	// Returns ("", error) if credentials recognized but invalid.
	Authenticate(ctx context.Context, payload *AuthPayload) (string, error)
}

// SecurityAuthz maps principals to permissions and enforces access control.
type SecurityAuthz interface {
	// Name returns "kernel.security.authz".
	Name() string

	// GetPermissions returns the permission context for a principal.
	GetPermissions(ctx context.Context, principal string) (*PermissionContext, error)

	// SetPermissions updates the permission context for a principal.
	SetPermissions(ctx context.Context, principal string, permissions *PermissionContext) error

	// CheckAccess validates if a URI access is allowed by the permission context.
	// Returns nil if allowed, error if denied.
	CheckAccess(uri string, mode AccessBits, permissions *PermissionContext) error
}

// PackageManager manages app installation and lifecycle.
type PackageManager interface {
	// Name returns "kernel.pkg".
	Name() string

	// Install installs an app from a zip file.
	Install(ctx context.Context, zipData []byte) (*AppMetadata, error)

	// Uninstall removes an installed app.
	Uninstall(ctx context.Context, appID string) error

	// List returns metadata for all installed apps.
	List(ctx context.Context) ([]*AppMetadata, error)

	// Get returns metadata for a specific app.
	Get(ctx context.Context, appID string) (*AppMetadata, error)

	// Resolve resolves an app name to a full app ID.
	// If version not specified, returns latest installed version.
	Resolve(ctx context.Context, appName string) (string, error)

	// GetWasmBinary returns the WASM binary for an app.
	GetWasmBinary(appID string) ([]byte, error)
}

// CompiledModule represents a compiled WASM module ready for instantiation
type CompiledModule interface {
	// Close releases resources associated with the compiled module
	Close(ctx context.Context) error

	// Name returns the module name (typically the appID)
	Name() string
}

// LifecycleManager manages the lifecycle of compiled WASM modules
type LifecycleManager interface {
	// Name returns the lifecycle manager name (e.g., "kernel.runtime.lifecycle.lru")
	Name() string

	// Get retrieves a compiled module from cache, returns nil if not found
	Get(ctx context.Context, appID string) (CompiledModule, error)

	// Put stores a compiled module in the cache
	Put(ctx context.Context, appID string, module CompiledModule) error

	// Remove explicitly removes a module from cache
	Remove(ctx context.Context, appID string) error

	// Clear removes all cached modules
	Clear(ctx context.Context) error

	// Stats returns cache statistics
	Stats() LifecycleStats
}

// LifecycleStats contains cache statistics
type LifecycleStats struct {
	// Hits is the number of cache hits
	Hits uint64

	// Misses is the number of cache misses
	Misses uint64

	// Evictions is the number of evictions
	Evictions uint64

	// CurrentSize is the current number of cached modules
	CurrentSize int

	// MaxSize is the maximum number of modules (0 = unlimited)
	MaxSize int
}

// HostFunction represents a function callable from WASM.
type HostFunction func(ctx context.Context, params []byte) ([]byte, error)

// RuntimeExec manages wazero engine lifecycle and app execution.
type RuntimeExec interface {
	// Name returns "kernel.runtime.exec".
	Name() string

	// LoadApp compiles and prepares a wasm binary for execution.
	LoadApp(ctx context.Context, appID string, wasmBytes []byte) error

	// LoadDriver compiles and prepares a driver wasm binary with metadata.
	// Metadata is used for permission checking during execution.
	LoadDriver(ctx context.Context, appID string, wasmBytes []byte, metadata *AppMetadata) error

	// GetMetadata extracts metadata from a WASM binary by calling its wazeos_metadata() function.
	// This allows apps to be self-describing without external metadata files.
	GetMetadata(ctx context.Context, wasmBytes []byte) (*AppMetadata, error)

	// UnloadApp removes a loaded app from the runtime.
	UnloadApp(ctx context.Context, appID string) error

	// Execute runs a wasm app with the given arguments.
	// Blocks until execution completes or context is cancelled.
	Execute(ctx context.Context, req *InvocationRequest) (*InvocationResult, error)

	// RegisterHostFunction registers a host function for WASM drivers to call.
	RegisterHostFunction(namespace, name string, fn HostFunction) error

	// SetResourceBus provides access to the resource driver bus.
	SetResourceBus(bus ResourceBus)
}

// CacheEventType represents different cache event types.
type CacheEventType string

const (
	CacheHit      CacheEventType = "hit"
	CacheMiss     CacheEventType = "miss"
	CacheEviction CacheEventType = "eviction"
)

// MetricsSnapshot represents current metrics state.
type MetricsSnapshot struct {
	InvocationCounts map[string]int64   // appID → count
	AverageDurations map[string]float64 // appID → avg duration (seconds)
	MemoryUsage      map[string]int64   // appID → avg memory (bytes)
}

// RuntimeTelemetry collects and exports metrics.
type RuntimeTelemetry interface {
	// Name returns "kernel.runtime.telemetry".
	Name() string

	// RecordInvocation logs an invocation event.
	RecordInvocation(appID string, duration time.Duration, success bool)

	// RecordMemoryUsage logs peak memory for an invocation.
	RecordMemoryUsage(appID string, bytes int64)

	// RecordCacheEvent logs cache hit/miss/eviction.
	RecordCacheEvent(appID string, eventType CacheEventType)

	// GetMetrics returns current metrics snapshot.
	GetMetrics() *MetricsSnapshot
}

// Kernel manages driver registration and lifecycle.
type Kernel interface {
	// RegisterRequestDriver adds an ingress driver.
	RegisterRequestDriver(driver RequestDriver) error

	// RegisterResourceDriver adds an egress driver.
	RegisterResourceDriver(driver ResourceDriver) error

	// RegisterSecurityAuthn adds an authentication driver.
	RegisterSecurityAuthn(driver SecurityAuthn) error

	// SetSecurityAuthz sets the authorization driver (only one).
	SetSecurityAuthz(authz SecurityAuthz) error

	// SetPackageManager sets the package manager (only one).
	SetPackageManager(pkg PackageManager) error

	// SetRuntimeExec sets the execution engine (only one).
	SetRuntimeExec(exec RuntimeExec) error

	// SetTelemetry sets the telemetry driver (only one).
	SetTelemetry(telemetry RuntimeTelemetry) error

	// Start initializes all drivers and begins processing.
	Start(ctx context.Context) error

	// Stop gracefully shuts down all drivers.
	Stop(ctx context.Context) error

	// RegisterAuditDriver adds an audit driver.
	RegisterAuditDriver(driver AuditDriver) error
}

// AuditEventType represents the type of audit event.
type AuditEventType string

const (
	AuditEventResourceCall AuditEventType = "resource_call"
	AuditEventAuthzCheck   AuditEventType = "authz_check"
	AuditEventAppInvoke    AuditEventType = "app_invoke"
)

// AuditEvent represents a security audit event.
type AuditEvent struct {
	ID        string            `json:"id"`        // Unique event ID
	Timestamp time.Time         `json:"timestamp"` // When the event occurred
	Type      AuditEventType    `json:"type"`      // Type of event
	Principal string            `json:"principal"` // Who performed the action
	RequestID string            `json:"requestId"` // Associated request ID
	TraceID   string            `json:"traceId"`   // Trace ID for correlation
	Success   bool              `json:"success"`   // Whether the operation succeeded
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"` // Event-specific data
}

// ResourceCallAuditEvent represents an audit event for a resource call.
type ResourceCallAuditEvent struct {
	AuditEvent
	URI        string `json:"uri"`
	Method     string `json:"method"`
	Driver     string `json:"driver"`     // Which driver handled it
	StatusCode int    `json:"statusCode"` // Result status code
	Duration   int64  `json:"duration"`   // Duration in nanoseconds
}

// AuthzCheckAuditEvent represents an audit event for an authorization check.
type AuthzCheckAuditEvent struct {
	AuditEvent
	URI        string     `json:"uri"`
	Access     AccessBits `json:"access"`
	Allowed    bool       `json:"allowed"`
	Reason     string     `json:"reason,omitempty"` // Why denied
	URIPattern string     `json:"uriPattern,omitempty"`
}

// AppInvokeAuditEvent represents an audit event for app invocation.
type AppInvokeAuditEvent struct {
	AuditEvent
	AppID    string `json:"appId"`
	ExitCode int    `json:"exitCode"`
	Duration int64  `json:"duration"` // Duration in nanoseconds
}

// AuditDriver is the interface for audit/logging drivers.
type AuditDriver interface {
	// Name returns the driver class (e.g., "kernel.security.audit.syslog").
	Name() string

	// RecordResourceCall logs a resource call event.
	RecordResourceCall(ctx context.Context, event *ResourceCallAuditEvent) error

	// RecordAuthzCheck logs an authorization check event.
	RecordAuthzCheck(ctx context.Context, event *AuthzCheckAuditEvent) error

	// RecordAppInvoke logs an app invocation event.
	RecordAppInvoke(ctx context.Context, event *AppInvokeAuditEvent) error

	// Query retrieves audit events (optional - can return ErrNotSupported).
	Query(ctx context.Context, filter AuditFilter) ([]*AuditEvent, error)
}

// AuditFilter represents criteria for querying audit events.
type AuditFilter struct {
	StartTime  time.Time        `json:"startTime,omitempty"`
	EndTime    time.Time        `json:"endTime,omitempty"`
	Principal  string           `json:"principal,omitempty"`
	EventTypes []AuditEventType `json:"eventTypes,omitempty"`
	Success    *bool            `json:"success,omitempty"` // nil = all, true = success only, false = failures only
	Limit      int              `json:"limit,omitempty"`   // Max results to return
}
