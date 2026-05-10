# WazeOS Test Suite

## Test Philosophy

**IMPORTANT:** Tests must use only commands and tools that end users would run. No mocks, no manual driver creation, no shortcuts.

## Test Structure

```
tests/
├── e2e/                    # End-to-end tests using actual CLI
│   └── app_calls_driver_test.go
├── integration/            # Integration tests for components
│   └── app_driver_context_test.go
└── README.md              # This file
```

## Running Tests

### End-to-End Tests (Recommended)

These test the complete user flow using only wazeos CLI commands:

```bash
cd /Users/dcoady/dev/os
go test -v ./tests/e2e/... -timeout 10m
```

**What they test:**
- ✅ `wazeos driver new` command
- ✅ `wazeos driver build` command
- ✅ `wazeos app new` command
- ✅ `wazeos app build` command
- ✅ `wazeos dev run` command with driver+app
- ✅ App successfully calling driver (critical user scenario)

### Integration Tests

Test internal component interactions:

```bash
go test -v ./tests/integration/...
```

**What they test:**
- ✅ Context permission logic
- ✅ Permission inheritance
- ✅ Pattern matching
- ✅ Dev mode permissions

## Critical Test: App Calls Driver

The most important test reproduces the exact user feedback scenario:

```go
func TestAppCallsDriver_CompleteUserFlow(t *testing.T) {
    // Uses only actual wazeos commands
    // No mocks, no shortcuts
    // If this passes, the bug is fixed
    // If this fails, the bug is reproduced
}
```

**Run it:**
```bash
go test -v ./tests/e2e -run TestAppCallsDriver_CompleteUserFlow -timeout 10m
```

This test will either:
- ✅ **PASS** - App successfully calls driver (bug is fixed!)
- ❌ **FAIL** - Reproduces "driver not found for URI" error (bug confirmed)

## Test Requirements

**TinyGo must be installed:**
```bash
brew install tinygo  # macOS
# or see: https://tinygo.org/getting-started/install/
```

## Questions?

See:
- [TEST_PLAN.md](TEST_PLAN.md) - Overall test strategy
- [../core/TEST_RESULTS_AND_RECOMMENDATIONS.md](../core/TEST_RESULTS_AND_RECOMMENDATIONS.md) - Analysis

---

**Remember:** If a user can't do it, tests shouldn't do it either.
