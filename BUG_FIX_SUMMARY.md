# Bug Fix Summary - App Cannot Call Driver

**Date:** 2026-05-10
**Status:** ✅ FIXED
**Test Status:** ✅ ALL PASSING

---

## The Bug

**Critical Issue:** Apps could not call drivers in dev mode.

**Error Message:**
```
ERROR: tool invocation failed: driver not found for URI
```

**User Scenario:**
1. Create and build shell driver: `wazeos driver new shell && wazeos driver build shell`
2. Create and build app: `wazeos app new date-test && wazeos app build date-test`
3. Run together: `wazeos dev run --driver shell.wazpkg --app date-test.wazpkg --invoke date-test`
4. **Result:** App fails to call driver with "driver not found" error

---

## Root Cause

The bug was in [core/cmd/wazeos/dev.go](core/cmd/wazeos/dev.go) lines 249-255.

**The Problem:**

When invoking an app, the code tried to guess the author from the app name:

```go
// BUGGY CODE (BEFORE FIX)
author := "test"
if strings.Contains(appName, "-") {
    // Wrong: Splits "date-test" into author="date", appName="test"
    parts := strings.SplitN(appName, "-", 2)
    if len(parts) == 2 {
        author = parts[0]
        appName = parts[1]
    }
}
appURI := fmt.Sprintf("app://127.0.0.1/%s/%s", author, appName)
// Result: app://127.0.0.1/date/test ❌
```

**What Actually Happened:**

During app registration (line 678):
```go
author := "local"  // Default from manifest
// App registers as: app://127.0.0.1/local/date-test
```

During invocation:
```go
// Code constructed: app://127.0.0.1/date/test
```

**These URIs don't match!** Router couldn't find the app.

---

## The Fix

**Location:** [core/cmd/wazeos/dev.go](core/cmd/wazeos/dev.go) lines 249-265

**Fixed Code:**

```go
// Look up the app from registered drivers to get the correct URI with author
var appURI string
for _, driver := range bus.ListDrivers() {
    // Check if this driver matches our app name
    // Pattern format: app://127.0.0.1/{author}/{name}
    if strings.HasSuffix(driver.URIPattern, "/"+appName) {
        appURI = driver.URIPattern
        break
    }
}

if appURI == "" {
    outputError("dev run", "APP_NOT_FOUND",
        fmt.Sprintf("app '%s' not found in registered drivers", appName),
        "Ensure the app was loaded successfully with --app flag")
}
```

**Why This Works:**

1. ✅ Uses the actual URI pattern from registration (includes correct author from manifest)
2. ✅ Verifies the app is actually loaded
3. ✅ No hardcoding or guessing
4. ✅ Works with any author name from manifest

---

## Test Coverage

### Unit Tests: ✅ ALL PASSING

**Location:** [tests/integration/app_driver_context_test.go](tests/integration/app_driver_context_test.go)

```bash
go test -v ./tests/integration/app_driver_context_test.go
```

| Test | Result |
|------|--------|
| TestWASMContextPermissions | ✅ PASS |
| TestContextInheritance | ✅ PASS |
| TestPermissionMatching | ✅ PASS |
| TestDevModePermissions | ✅ PASS |

### E2E Test: ✅ PASSING

**Location:** [tests/e2e/app_calls_driver_test.go](tests/e2e/app_calls_driver_test.go)

```bash
go test -v ./tests/e2e -run TestAppCallsDriver_CompleteUserFlow
```

**Test Output:**
```
✅ SUCCESS: App successfully called driver!
✅ CRITICAL BUG FIXED: No more 'driver not found' error!
--- PASS: TestAppCallsDriver_CompleteUserFlow (8.18s)
PASS
```

**What The E2E Test Does:**

Uses ONLY real wazeos CLI commands (no mocks, no shortcuts):

1. ✅ `wazeos driver new shell --class io.connect`
2. ✅ Edit auto-generated driver code (minimal)
3. ✅ `wazeos driver build default/shell`
4. ✅ `wazeos app new date-test --language go`
5. ✅ Edit auto-generated app code (minimal)
6. ✅ `wazeos app build default/date-test`
7. ✅ `wazeos dev run --driver shell.wazpkg --app date-test.wazpkg --invoke date-test`
8. ✅ Verify app successfully calls driver

---

## Key Findings from Investigation

### What We Initially Thought ❌

- Bus instance mismatch (WASM context using different bus)
- Permission issues (execCtx missing permissions)
- Pattern matching bug (router not matching `shell://**` to `shell://date`)

### What It Actually Was ✅

**URI mismatch during app invocation:**

- App registered with URI from manifest: `app://127.0.0.1/local/date-test`
- Invocation code constructed wrong URI: `app://127.0.0.1/date/test`
- Router correctly returned "not found" because the URIs didn't match

### Debug Methodology

Added logging to [core/kernel/iobus/router.go](core/kernel/iobus/router.go) to trace:
1. Pattern registration (what patterns are registered)
2. URI matching (what URIs are being looked up)
3. Match results (whether drivers are found)

**Debug Output Revealed:**
```
[ROUTER DEBUG] Registering pattern: app://127.0.0.1/local/date-test
[ROUTER DEBUG] Matching URI: app://127.0.0.1/date/test
[ROUTER DEBUG] No driver found!
```

This immediately showed the URI mismatch.

---

## Changes Made

### Files Modified

1. **[core/cmd/wazeos/dev.go](core/cmd/wazeos/dev.go)**
   - Fixed app URI lookup to use registered driver patterns
   - Removed incorrect author-guessing logic
   - Added proper error handling for missing apps

2. **[tests/e2e/app_calls_driver_test.go](tests/e2e/app_calls_driver_test.go)**
   - Created comprehensive E2E test using only real CLI commands
   - Tests complete user workflow from driver creation to invocation
   - Verifies app can successfully call driver

### Files Temporarily Modified (For Debugging)

3. **[core/kernel/iobus/router.go](core/kernel/iobus/router.go)**
   - Added debug logging (later removed)
   - Helped identify the URI mismatch issue

---

## Verification

### Manual Test

```bash
cd /Users/dcoady/dev/os

# 1. Create and build driver
wazeos driver new shell --class io.connect
# Edit drivers/default/shell/driver.go (add implementation)
wazeos driver build default/shell

# 2. Create and build app
wazeos app new date-test --language go
# Edit apps/default/date-test/main.go (add ctx.Call to shell driver)
wazeos app build default/date-test

# 3. Run together
wazeos dev run \
    --driver drivers/default/shell/build/shell.wazpkg \
    --app apps/default/date-test/build/date-test.wazpkg \
    --invoke date-test/test-date \
    --args '{"input":"test"}'

# Expected: ✓ Tool invocation completed
# Result: {"status": "success", ...}
```

### Automated Test

```bash
go test -v ./tests/e2e -run TestAppCallsDriver_CompleteUserFlow
```

**Result:** ✅ PASS

---

## Impact

### Before Fix ❌

- Apps **could not** call drivers
- Dev mode was broken for multi-component testing
- Users got confusing "driver not found" errors even though drivers loaded successfully

### After Fix ✅

- Apps **can** call drivers successfully
- Dev mode works as intended for testing driver+app interactions
- Proper error messages when apps aren't loaded
- No hardcoded assumptions about author names

---

## Lessons Learned

1. **Don't guess data - look it up:** The code tried to parse author from app name instead of reading from manifest or registered drivers

2. **Test with real user scenarios:** E2E test using only CLI commands caught the bug that unit tests missed

3. **Debug logging is invaluable:** Adding temporary debug output to router.go immediately revealed the URI mismatch

4. **Follow the data:** App registration uses manifest author → invocation should use the same source

---

## Related Documentation

- [tests/e2e/TEST_RESULTS.md](tests/e2e/TEST_RESULTS.md) - Detailed E2E test analysis
- [tests/COMPREHENSIVE_TEST_SUMMARY.md](tests/COMPREHENSIVE_TEST_SUMMARY.md) - Complete test overview
- [tests/README.md](tests/README.md) - Test philosophy and instructions
- [core/TEST_RESULTS_AND_RECOMMENDATIONS.md](core/TEST_RESULTS_AND_RECOMMENDATIONS.md) - Original analysis

---

## Conclusion

**Mission Status: ✅ COMPLETE**

1. ✅ Bug identified and fixed
2. ✅ Comprehensive test coverage added
3. ✅ All tests passing
4. ✅ Solution uses actual data instead of hardcoded values
5. ✅ User scenario verified working

The critical "app cannot call driver" bug is **FIXED** and **VERIFIED**.
