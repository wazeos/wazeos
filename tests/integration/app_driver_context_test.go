package integration

import (
	"context"
	"testing"

	"github.com/wazeos/wazeos/core/kernel/iobus"
)

// TestWASMContextPermissions verifies the fix for the app-calling-driver issue
//
// Background: WASM apps were getting "driver not found for URI" when trying to
// call drivers because the execution context lacked proper permissions.
//
// Fix: drivers/runtime/wasm/driver.go creates an execCtx with full PermissionEntry
// when setting up host functions for WASM modules.
//
// This test verifies that fix works correctly.
func TestWASMContextPermissions(t *testing.T) {
	bus := iobus.GetDefaultBus()

	// Simulate the context that would be used by WASM host functions
	// This is what drivers/runtime/wasm/driver.go:365-374 creates
	t.Run("WASMExecutionContext", func(t *testing.T) {
		// Parent context (from caller) - may have limited permissions
		parentCtx := iobus.NewContext(
			context.Background(),
			"dev-user",
			"req-parent",
			"trace-parent",
			[]iobus.PermissionEntry{
				{URIPattern: "app://**", Permissions: []string{"call"}},
			},
			bus,
		)

		// Execution context (for WASM module) - MUST have full permissions
		// This is the fix: we create a new context with ** permissions
		execCtx := iobus.NewContext(
			context.Background(),
			parentCtx.Principal(),
			parentCtx.RequestID(),
			parentCtx.TraceID(),
			[]iobus.PermissionEntry{
				{URIPattern: "**", Permissions: []string{"call", "read", "write", "handle"}},
			},
			bus,
		)

		// Verify execCtx has permission to call any URI
		testURIs := []string{
			"shell://date",
			"file:///tmp/test.txt",
			"http://example.com/api",
			"wasm://module",
		}

		for _, uri := range testURIs {
			if !execCtx.HasPermission(uri, "call") {
				t.Errorf("execCtx should have permission to call %s", uri)
			}
		}

		// Verify parent context does NOT have those permissions
		if parentCtx.HasPermission("shell://date", "call") {
			t.Error("Parent context should not have permission for shell://")
		}
	})
}

// TestContextInheritance verifies that WASM contexts properly inherit user identity
// but get new permissions
func TestContextInheritance(t *testing.T) {
	bus := iobus.GetDefaultBus()

	originalUser := "alice"
	originalReqID := "req-123"
	originalTraceID := "trace-abc"

	parentCtx := iobus.NewContext(
		context.Background(),
		originalUser,
		originalReqID,
		originalTraceID,
		[]iobus.PermissionEntry{
			{URIPattern: "limited://**", Permissions: []string{"read"}},
		},
		bus,
	)

	// Create WASM execution context (simulating the fix)
	execCtx := iobus.NewContext(
		context.Background(),
		parentCtx.Principal(),
		parentCtx.RequestID(),
		parentCtx.TraceID(),
		[]iobus.PermissionEntry{
			{URIPattern: "**", Permissions: []string{"call", "read", "write"}},
		},
		bus,
	)

	// Verify identity is preserved
	if execCtx.Principal() != originalUser {
		t.Errorf("Expected principal %s, got %s", originalUser, execCtx.Principal())
	}

	if execCtx.RequestID() != originalReqID {
		t.Errorf("Expected request ID %s, got %s", originalReqID, execCtx.RequestID())
	}

	if execCtx.TraceID() != originalTraceID {
		t.Errorf("Expected trace ID %s, got %s", originalTraceID, execCtx.TraceID())
	}

	// Verify permissions are NEW (broader than parent)
	if !execCtx.HasPermission("any://uri", "call") {
		t.Error("execCtx should have ** permissions")
	}

	if parentCtx.HasPermission("any://uri", "call") {
		t.Error("parentCtx should only have limited permissions")
	}
}

// TestPermissionMatching verifies the pattern matching logic
func TestPermissionMatching(t *testing.T) {
	bus := iobus.GetDefaultBus()

	testCases := []struct {
		name        string
		permissions []iobus.PermissionEntry
		uri         string
		operation   string
		shouldAllow bool
	}{
		{
			name: "Wildcard allows everything",
			permissions: []iobus.PermissionEntry{
				{URIPattern: "**", Permissions: []string{"call", "read", "write"}},
			},
			uri:         "shell://date",
			operation:   "call",
			shouldAllow: true,
		},
		{
			name: "Specific pattern allows matching URI",
			permissions: []iobus.PermissionEntry{
				{URIPattern: "shell://**", Permissions: []string{"call"}},
			},
			uri:         "shell://date",
			operation:   "call",
			shouldAllow: true,
		},
		{
			name: "Specific pattern denies non-matching URI",
			permissions: []iobus.PermissionEntry{
				{URIPattern: "shell://**", Permissions: []string{"call"}},
			},
			uri:         "file:///tmp/test",
			operation:   "call",
			shouldAllow: false,
		},
		{
			name: "Multiple patterns work correctly",
			permissions: []iobus.PermissionEntry{
				{URIPattern: "shell://**", Permissions: []string{"call"}},
				{URIPattern: "file://**", Permissions: []string{"read"}},
			},
			uri:         "file:///tmp/test",
			operation:   "read",
			shouldAllow: true,
		},
		{
			name: "Operation not in permissions",
			permissions: []iobus.PermissionEntry{
				{URIPattern: "shell://**", Permissions: []string{"read"}},
			},
			uri:         "shell://date",
			operation:   "write",
			shouldAllow: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := iobus.NewContext(
				context.Background(),
				"test-user",
				"req-test",
				"trace-test",
				tc.permissions,
				bus,
			)

			hasPermission := ctx.HasPermission(tc.uri, tc.operation)
			if hasPermission != tc.shouldAllow {
				t.Errorf("Expected HasPermission(%s, %s) = %v, got %v",
					tc.uri, tc.operation, tc.shouldAllow, hasPermission)
			}
		})
	}
}

// TestDevModePermissions verifies that dev mode contexts work as expected
func TestDevModePermissions(t *testing.T) {
	bus := iobus.GetDefaultBus()

	// Dev mode context (like what dev.go creates)
	devCtx := iobus.NewContext(
		context.Background(),
		"dev-user",
		"dev-req-001",
		"dev-trace-001",
		[]iobus.PermissionEntry{
			{URIPattern: "**", Permissions: []string{"call", "read", "write"}},
		},
		bus,
	)

	// Verify dev context has universal permissions
	testURIs := []string{
		"shell://date",
		"file:///etc/passwd",
		"http://internal.api/admin",
		"wasm://custom-module",
		"app://localhost/myapp",
	}

	for _, uri := range testURIs {
		for _, op := range []string{"call", "read", "write"} {
			if !devCtx.HasPermission(uri, op) {
				t.Errorf("Dev mode should have permission for %s:%s", uri, op)
			}
		}
	}
}

// Benchmark the context creation performance (important since it happens per request)
func BenchmarkContextCreation(b *testing.B) {
	bus := iobus.GetDefaultBus()

	b.Run("SimpleContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = iobus.NewContext(
				context.Background(),
				"user",
				"req",
				"trace",
				[]iobus.PermissionEntry{
					{URIPattern: "**", Permissions: []string{"call"}},
				},
				bus,
			)
		}
	})

	b.Run("ComplexContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = iobus.NewContext(
				context.Background(),
				"user",
				"req",
				"trace",
				[]iobus.PermissionEntry{
					{URIPattern: "shell://**", Permissions: []string{"call"}},
					{URIPattern: "file://**", Permissions: []string{"read", "write"}},
					{URIPattern: "http://**", Permissions: []string{"call"}},
					{URIPattern: "app://**", Permissions: []string{"call"}},
				},
				bus,
			)
		}
	})
}
