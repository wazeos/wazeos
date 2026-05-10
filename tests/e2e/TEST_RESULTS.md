# E2E Test Results - App Calls Driver

**Date:** 2026-05-10
**Test:** `TestAppCallsDriver_CompleteUserFlow`
**Status:** ✅ TEST WORKING - ❌ BUG REPRODUCED

## Executive Summary

The end-to-end test has been successfully implemented and **confirms the critical bug reported by the user**. The test uses ONLY real wazeos CLI commands (no mocks, no shortcuts) and reproduces the exact user scenario.

## Test Flow

| Step | Command | Result |
|------|---------|--------|
| 1. Create driver | `wazeos driver new shell --class io.connect` | ✅ PASS |
| 2. Implement driver | Edit auto-generated code minimally | ✅ PASS |
| 3. Build driver | `wazeos driver build default/shell` | ✅ PASS |
| 4. Create app | `wazeos app new date-test --language go` | ✅ PASS |
| 5. Implement app | Edit auto-generated code minimally | ✅ PASS |
| 6. Build app | `wazeos app build default/date-test` | ✅ PASS |
| 7. Run together | `wazeos dev run --driver ... --app ...` | ❌ **BUG REPRODUCED** |

## Critical Finding

```
Loading 1 driver(s)...
  [1/1] Loading: drivers/default/shell/build/shell.wazpkg
    ✓ Driver loaded
    → Registered: dev-shell-0
    → Pattern: shell://**
    → Capabilities: [call]

Loading 1 app(s)...
  [1/1] Loading: apps/default/date-test/build/date-test.wazpkg
    ✓ App loaded

✓ Environment ready

Invoking: date-test/test-date
ERROR: tool invocation failed: driver not found for URI
```

**Analysis:**
- Driver successfully registers with pattern `shell://**`
- App successfully loads
- When app calls `ctx.Call("shell://date", ...)` it fails with "driver not found for URI"
- This is the EXACT error from user feedback

## Why This Bug Exists

Despite the fix in [drivers/runtime/wasm/driver.go:365-374](../../drivers/runtime/wasm/driver.go#L365-L374) that creates `execCtx` with full permissions, the WASM app still cannot call the driver.

### Possible Root Causes

1. **Pattern Matching Issue**
   - Pattern `shell://**` should match URI `shell://date`
   - Router may not be properly matching the pattern

2. **Bus Instance Mismatch**
   - WASM runtime may be using a different IOBus instance
   - Driver registered in one bus, app trying to call from another

3. **execCtx Not Being Used**
   - The fix creates execCtx but it may not be passed to the right place
   - Host function `host_iobus_call` may not be using execCtx

4. **Timing Issue**
   - Driver loads after app, or vice versa
   - Router state not synchronized

## Next Steps

### Priority 1: Add Debug Logging

Add logging to these critical points to diagnose the issue:

1. **iobus.go:303** - Router matching
   ```go
   log.Printf("DEBUG: Looking for driver matching URI: %s", req.URI)
   driver := bus.router.Match(req.URI)
   if driver == nil {
       log.Printf("DEBUG: No driver found. Registered patterns: %v", bus.router.ListPatterns())
   }
   ```

2. **drivers/runtime/wasm/driver.go:381** - Host function call
   ```go
   log.Printf("DEBUG: host_iobus_call - URI: %s, Context permissions: %v", uri, execCtx.Permissions())
   ```

3. **dev.go - loadWASMDriver()** - Driver registration
   ```go
   log.Printf("DEBUG: Registering driver: %s with pattern: %s", spec.Name, spec.URIPattern)
   err := bus.Register(spec)
   log.Printf("DEBUG: Registration result for %s: %v", spec.Name, err)
   ```

### Priority 2: Verify Fix Location

The fix in driver.go creates execCtx, but we need to verify:
- Is execCtx actually passed to the host function?
- Does the host function use it for the Call operation?
- Is there a code path where the wrong context is used?

### Priority 3: Pattern Matching Test

Create a unit test that verifies:
```go
func TestPatternMatching_ShellDriver(t *testing.T) {
    pattern := "shell://**"
    uri := "shell://date"

    // Test that pattern matches URI
    matches := matchPattern(pattern, uri)
    assert.True(t, matches, "Pattern %s should match URI %s", pattern, uri)
}
```

## Test Philosophy Compliance

✅ This test follows the principle: **"If a user can't do it, tests shouldn't do it either"**

The test:
- Uses only `wazeos` CLI commands
- Edits only auto-generated code (minimally)
- No mocks or manual driver creation
- No GitHub imports or core SDK paths
- Uses only local module paths like `sdk/driver/wasm`

## Conclusion

**The E2E test is working correctly and proves the bug is real.**

The unit tests in [tests/integration/app_driver_context_test.go](../integration/app_driver_context_test.go) verify that the context permission logic works in isolation. However, this E2E test proves that in the real WASM runtime environment, apps still cannot call drivers.

The fix in driver.go may be correct in principle, but there's a missing piece in how it integrates with the actual driver resolution and routing.

## Running The Test

```bash
cd /Users/dcoady/dev/os
go test -v ./tests/e2e -run TestAppCallsDriver_CompleteUserFlow -timeout 10m
```

Expected result: Test reproduces the "driver not found for URI" error.

## Related Documents

- [TEST_PLAN.md](../TEST_PLAN.md) - Overall test strategy
- [TEST_RESULTS_AND_RECOMMENDATIONS.md](../../core/TEST_RESULTS_AND_RECOMMENDATIONS.md) - Analysis
- [WASM_APP_DRIVER_ISSUE.md](../../core/WASM_APP_DRIVER_ISSUE.md) - Root cause analysis
- [tests/integration/app_driver_context_test.go](../integration/app_driver_context_test.go) - Unit tests (passing)
