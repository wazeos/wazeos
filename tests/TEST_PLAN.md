# WazeOS Integration Test Plan

## Overview
This document outlines the test plan for verifying the critical "app-calling-driver" functionality based on user feedback from 2026-05-10.

## User-Reported Issue

**Status:** CRITICAL BUG
**Impact:** Apps cannot call drivers in dev mode
**Error:** "driver not found for URI"

### Scenario
1. User creates shell driver: `wazeos driver new shell --class io.connect`
2. User builds shell driver: `wazeos driver build wazeos/shell`
3. User creates date-test app: `wazeos app new date-test`
4. User builds date-test app: `wazeos app build wazeos/date-test`
5. User runs both together:
   ```bash
   wazeos dev run --driver drivers/wazeos/shell/build/shell.wazpkg \
                  --app apps/wazeos/date-test/build/date-test.wazpkg \
                  --invoke wazeos/date-test --args '{}'
   ```

### Expected Result
App successfully calls `shell://date` and receives response

### Actual Result
```
ERROR: tool invocation failed: driver not found for URI
```

## Root Cause Analysis

Documented in: [WASM_APP_DRIVER_ISSUE.md](../core/WASM_APP_DRIVER_ISSUE.md)

**Issue:** WASM modules were initialized with contexts that lacked proper IO Bus permissions.

**Fix Location:** `/drivers/runtime/wasm/driver.go:365-374`

**Fix:** Create execution context (`execCtx`) with full `PermissionEntry` array when setting up WASM host functions:
```go
execCtx := iobus.NewContext(
    context.Background(),
    ctx.Principal(),
    ctx.RequestID(),
    ctx.TraceID(),
    []iobus.PermissionEntry{
        {URIPattern: "**", Permissions: []string{"call", "read", "write", "handle"}},
    },
    bus,
)
```

## Test Coverage

### Unit Tests ✅

**File:** `/tests/integration/app_driver_context_test.go`

| Test | Purpose | Status |
|------|---------|--------|
| `TestWASMContextPermissions` | Verifies execCtx has full permissions | ✅ PASS |
| `TestContextInheritance` | Verifies identity preserved, permissions expanded | ✅ PASS |
| `TestPermissionMatching` | Verifies pattern matching logic | ✅ PASS |
| `TestDevModePermissions` | Verifies dev mode ** permissions work | ✅ PASS |

### Integration Tests ⚠️ NEEDED

**File:** `/tests/integration/app_driver_integration_test.go` (created but needs WASM modules)

| Scenario | Implementation Status |
|----------|----------------------|
| Direct driver call | ✅ Mockable |
| App→Driver call | ⚠️ Needs real WASM |
| Multiple drivers+apps | ⚠️ Needs real WASM |
| Permission isolation | ✅ Mockable |

### End-to-End Tests ❌ CRITICAL NEED

**Missing:** Real WASM-based integration test

**Required Steps:**
1. Build minimal shell driver WASM module
   - Accepts `command` parameter (array)
   - Executes command
   - Returns output

2. Build minimal test app WASM module
   - Calls `shell://date` with `{"command": ["date"]}`
   - Returns result

3. Test flow:
   ```go
   func TestRealWASMAppCallsDriver(t *testing.T) {
       bus := iobus.GetDefaultBus()

       // Load shell driver WASM
       shellSpec := iobus.DriverSpec{
           Name: "test-shell",
           Runtime: "wasm",
           Binary: "./testdata/shell_driver.wasm",
           URIPattern: "shell://**",
       }
       bus.Register(shellSpec)

       // Load test app WASM
       appSpec := iobus.DriverSpec{
           Name: "test-app",
           Runtime: "wasm",
           Binary: "./testdata/test_app.wasm",
           URIPattern: "app://test",
       }
       bus.Register(appSpec)

       // Invoke app (which calls shell driver internally)
       ctx := devModeContext()
       resp, err := bus.Call(ctx, iobus.Request{
           URI: "app://test",
           Operation: iobus.OpCall,
       })

       // Verify success
       assert.NoError(t, err)
       assert.Equal(t, 200, resp.StatusCode)
   }
   ```

## Test Data Needed

### 1. Shell Driver WASM (`testdata/shell_driver.wasm`)

**Source:** `testdata/shell_driver/main.go`
```go
//go:build wasip1

package main

import (
    "os/exec"
    sdk "github.com/wazeos/wazeos/core/sdk/go/driver/wasm"
)

func main() {
    params := sdk.DefineParams(
        sdk.Param{
            Name: "command",
            Type: sdk.Array,
            Required: true,
        },
    )

    sdk.Run(params, func(args *sdk.Args) sdk.Response {
        command := args.StringArray("command")
        if len(command) == 0 {
            return sdk.ErrorResponse(400, "command required")
        }

        cmd := exec.Command(command[0], command[1:]...)
        output, err := cmd.Output()
        if err != nil {
            return sdk.ErrorResponse(500, err.Error())
        }

        return sdk.Response{
            StatusCode: 200,
            Body: output,
        }
    })
}
```

**Build:** `tinygo build -o testdata/shell_driver.wasm -target=wasi main.go`

### 2. Test App WASM (`testdata/test_app.wasm`)

**Source:** `testdata/test_app/main.go`
```go
//go:build wasip1

package main

import (
    "encoding/json"
    "fmt"
    app "github.com/wazeos/wazeos/core/sdk/go/app"
)

func main() {
    app.HandleTool(func(ctx *app.AppContext, args json.RawMessage) (interface{}, error) {
        // Call shell driver
        resp, err := ctx.Call("shell://date", map[string]interface{}{
            "command": []interface{}{"date", "+%Y-%m-%d"},
        })
        if err != nil {
            return nil, fmt.Errorf("shell call failed: %w", err)
        }

        if resp.Error != "" {
            return nil, fmt.Errorf("shell error: %s", resp.Error)
        }

        return map[string]interface{}{
            "result": string(resp.Body),
            "status": "success",
        }, nil
    })
}
```

**Build:** `tinygo build -o testdata/test_app.wasm -target=wasi main.go`

## Verification Checklist

- [x] Fix applied in `drivers/runtime/wasm/driver.go`
- [x] Unit tests for context permissions
- [x] Integration tests for context inheritance
- [x] Permission pattern matching tests
- [ ] **CRITICAL:** End-to-end test with real WASM modules
- [ ] **CRITICAL:** Reproduce exact user scenario
- [ ] Performance benchmarks (context creation is per-request)
- [ ] Documentation updated with fix explanation

## Manual Testing Steps

To manually verify the fix works:

```bash
# 1. Build test WASM modules
cd tests/testdata/shell_driver && tinygo build -o shell_driver.wasm -target=wasi main.go
cd ../test_app && tinygo build -o test_app.wasm -target=wasi main.go

# 2. Create packages
wazeos driver package shell_driver.wasm
wazeos app package test_app.wasm

# 3. Run dev mode
wazeos dev run --driver shell_driver.wazpkg \
               --app test_app.wazpkg \
               --invoke test-app --args '{}'

# Expected: Success with date output
# If bug still exists: "driver not found for URI"
```

## Success Criteria

✅ **All tests pass**
✅ **Manual reproduction works**
✅ **Performance acceptable** (< 1ms context creation)
✅ **User scenario documented**
✅ **Regression tests in place**

## Open Questions

1. ❓ Why is user still seeing the error if fix is applied?
   - Possible causes:
     - Binary not rebuilt after fix
     - Different code path in their setup
     - Pattern matching issue (shell://** not matching shell://date)
     - Bus instance mismatch (different IOBus for WASM vs dev)

2. ❓ Are there other scenarios where context permissions might be insufficient?
   - Handle creation
   - Resource access
   - Nested driver calls

3. ❓ Should WASM execCtx always have ** permissions in dev mode?
   - Current: Yes (hardcoded)
   - Alternative: Load from DriverSpec.Permissions
   - Security: Need to document security model

## Next Steps

1. **IMMEDIATE:** Create testdata WASM modules
2. **IMMEDIATE:** Add end-to-end test with real WASM
3. **HIGH:** Reproduce exact user scenario
4. **HIGH:** Add debug logging to help diagnose issues
5. **MEDIUM:** Document security model
6. **MEDIUM:** Add integration tests for nested calls
7. **LOW:** Performance optimization if needed

## References

- [WASM_APP_DRIVER_ISSUE.md](../core/WASM_APP_DRIVER_ISSUE.md) - Original issue analysis
- [DX_IMPROVEMENTS.md](../core/DX_IMPROVEMENTS.md) - User experience improvements
- [User Feedback 2026-05-10] - Detailed bug report and scenario

---

**Last Updated:** 2026-05-10
**Status:** Fix applied, comprehensive tests needed
**Priority:** CRITICAL - blocks core functionality
