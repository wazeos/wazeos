# Testing Summary - App Calls Driver Functionality

**Status:** ✅ Tests Complete - ❌ Bug Confirmed
**Date:** 2026-05-10

## What Was Accomplished

### 1. Unit Tests (✅ ALL PASSING)

**Location:** [tests/integration/app_driver_context_test.go](../integration/app_driver_context_test.go)

These tests verify the permission logic at the component level:

```bash
cd /Users/dcoady/dev/os
go test -v ./tests/integration/app_driver_context_test.go
```

| Test | Purpose | Status |
|------|---------|--------|
| `TestWASMContextPermissions` | Verifies execCtx has ** permissions | ✅ PASS |
| `TestContextInheritance` | Verifies identity preserved | ✅ PASS |
| `TestPermissionMatching` | Verifies pattern matching | ✅ PASS |
| `TestDevModePermissions` | Verifies dev mode access | ✅ PASS |

**Conclusion:** The permission logic in the fix works correctly in isolation.

### 2. End-to-End Test (✅ TEST WORKS - ❌ BUG REPRODUCED)

**Location:** [tests/e2e/app_calls_driver_test.go](./app_calls_driver_test.go)

This test reproduces the exact user scenario using ONLY real wazeos CLI commands:

```bash
cd /Users/dcoady/dev/os
go test -v ./tests/e2e -run TestAppCallsDriver_CompleteUserFlow -timeout 10m
```

**Test Steps:**
1. ✅ `wazeos driver new shell --class io.connect`
2. ✅ Edit auto-generated driver.go (add os/exec import, implement shell command executor)
3. ✅ `wazeos driver build default/shell`
4. ✅ `wazeos app new date-test --language go`
5. ✅ Edit auto-generated main.go (implement ctx.Call to shell driver)
6. ✅ `wazeos app build default/date-test`
7. ❌ `wazeos dev run --driver shell.wazpkg --app date-test.wazpkg --invoke ...`

**Error Reproduced:**
```
ERROR: tool invocation failed: driver not found for URI
```

**Conclusion:** The bug is REAL. Despite drivers loading successfully with pattern `shell://**`, apps cannot call them.

## Key Findings

### What Works ✅

1. **Fix is applied correctly** - drivers/runtime/wasm/driver.go:365-374
2. **Permission logic is correct** - Unit tests verify this
3. **Driver registration works** - Logs show driver registered with correct pattern
4. **App registration works** - Logs show app loaded successfully
5. **Compilation works** - Both driver and app WASM modules build successfully

### What's Broken ❌

1. **Driver Resolution** - When app calls `ctx.Call("shell://date", ...)`, router returns nil
2. **URI Matching** - Pattern `shell://**` not matching URI `shell://date` (or router not finding it)
3. **Integration** - The connection between app WASM context and driver registry is broken

## Root Cause Analysis

The bug is NOT a permission issue. The bug is in **driver routing**.

### Evidence

1. Driver logs show: `driver registered name=dev-shell-0 pattern=shell://** capabilities=[call]`
2. This proves the driver IS registered
3. But when app calls `shell://date`, error is "driver not found for URI"
4. This means `bus.router.Match(req.URI)` returns `nil`

### Possible Causes

#### Theory 1: Bus Instance Mismatch (MOST LIKELY)

The WASM app's execCtx may be using a different IOBus instance than the one where drivers are registered.

**Check in code:**
- drivers/runtime/wasm/driver.go:360: `bus := ctx.IOBus()`
- dev.go: Uses `iobus.GetDefaultBus()`

If these are different bus instances, that's the bug!

#### Theory 2: Pattern Matching Bug

The router's pattern matching may not handle `**` wildcards correctly for URIs like `shell://date`.

#### Theory 3: Timing Issue

Drivers may be registered in a state that isn't visible to the WASM runtime's IOBus.

## Test Philosophy Success ✅

The E2E test strictly follows: **"If a user can't do it, tests shouldn't do it either"**

- ✅ Uses only `wazeos` CLI commands
- ✅ Edits only auto-generated code
- ✅ Makes minimal edits
- ✅ Uses local module paths (`sdk/driver/wasm`)
- ✅ No mocks, no manual driver creation

## Next Actions

### Immediate (CRITICAL)

1. Add debug logging to verify bus instance theory
2. Add router state logging when driver not found
3. Test pattern matching in isolation

### Medium Priority

1. Improve error messages
2. Add verbose mode showing routing
3. Document the fix once found

## How to Run Tests

### Unit Tests (All Passing)
```bash
cd /Users/dcoady/dev/os
go test -v ./tests/integration/app_driver_context_test.go
```

### E2E Test (Reproduces Bug)
```bash
cd /Users/dcoady/dev/os
go test -v ./tests/e2e -run TestAppCallsDriver_CompleteUserFlow -timeout 10m
```

Expected: Test shows "❌ CRITICAL BUG REPRODUCED"

## Conclusion

✅ **Mission Accomplished:** Tests successfully:
1. Verify permission fix works in isolation
2. Reproduce the exact user bug
3. Follow strict testing principles
4. Provide clear evidence the bug is in driver routing, not permissions

**Next Step:** Debug driver routing to find why registered drivers aren't found by apps.
