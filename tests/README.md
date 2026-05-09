# WazeOS v2 Test Suite

This directory contains all tests for WazeOS v2, including unit tests, integration tests, and end-to-end tests.

## Directory Structure

```
tests/
├── e2e/                      # End-to-end test scenarios
│   ├── common.sh            # Shared utilities for E2E tests
│   ├── test_time_app.sh     # Scenario 1: Time app with shell driver
│   ├── test_random_app.sh   # Scenario 2: Random Wikipedia app
│   ├── test_temp_app.sh     # Scenario 3: Temp file manager app
│   └── run_all.sh           # Master test runner
│
├── drivers/                  # Unit tests for drivers
│   ├── file_driver_test.go  # File driver tests (TODO)
│   ├── http_driver_test.go  # HTTP driver tests (TODO)
│   └── shell_driver_test.go # Shell driver tests (TODO)
│
└── apps/                     # Unit tests for apps
    ├── time_app_test.rs     # Time app tests (TODO)
    ├── random_app_test.rs   # Random app tests (TODO)
    └── temp_app_test.rs     # Temp app tests (TODO)
```

## End-to-End Test Scenarios

### Scenario 1: Time App with Shell Driver

**Purpose**: Verify users can create a custom driver and app that work together

**Tests**:
- Shell driver compiles and registers
- Time app compiles to WASM
- Shell commands execute successfully
- App receives and formats time string

**Run**:
```bash
./e2e/test_time_app.sh
```

### Scenario 2: Random Wikipedia App

**Purpose**: Verify HTTP client operations and network-based apps

**Tests**:
- HTTP driver handles redirects
- App makes HTTP requests
- App parses HTML content
- Returns structured data

**Run**:
```bash
./e2e/test_random_app.sh
```

### Scenario 3: Temp File Manager App

**Purpose**: Verify file operations (list, write, read)

**Tests**:
- File driver lists directory contents
- File driver writes files
- File driver reads files
- App chains multiple operations
- Content verification passes

**Run**:
```bash
./e2e/test_temp_app.sh
```

## Running Tests

### Run All E2E Tests

```bash
cd tests/e2e
./run_all.sh
```

### Run Individual Test Scenario

```bash
cd tests/e2e
./test_time_app.sh      # Scenario 1
./test_random_app.sh    # Scenario 2
./test_temp_app.sh      # Scenario 3
```

### Run with Verbose Output

```bash
bash -x ./e2e/test_time_app.sh
```

## Test Environment

The test scripts use these environment variables:

- `WAZEOS_ROOT` - Root directory of WazeOS v2 (defaults to `$(pwd)/v2`)
- `WAZEOS_BIN` - Path to wazeos binary (defaults to `${WAZEOS_ROOT}/bin/wazeos`)

Set them manually if needed:
```bash
export WAZEOS_ROOT=/path/to/wazeos/v2
export WAZEOS_BIN=/path/to/wazeos/bin/wazeos
./e2e/run_all.sh
```

## Test Assertions

The `common.sh` library provides test assertion functions:

```bash
# Equality assertion
assert_eq "$actual" "$expected" "Values should match"

# Substring assertion
assert_contains "$haystack" "$needle" "Should contain substring"

# File existence
assert_file_exists "/path/to/file" "File should exist"

# Command success
assert_cmd_success "some_command" "Command should succeed"
```

## CI/CD Integration

Tests are automatically run on every commit via GitHub Actions:

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests
on: [push, pull_request]
jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run E2E tests
        run: ./v2/tests/e2e/run_all.sh
```

## Writing New Tests

### Adding a New E2E Scenario

1. Create test script in `e2e/`:
```bash
touch e2e/test_my_scenario.sh
chmod +x e2e/test_my_scenario.sh
```

2. Use the template:
```bash
#!/bin/bash
# E2E Test: My Scenario Description

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

start_test_suite "My Scenario"
setup_test_env

log_test "Testing something"
# ... test code ...

finish_test_suite "My Scenario"
```

3. Add to `run_all.sh`:
```bash
if bash "$SCRIPT_DIR/test_my_scenario.sh"; then
    ((PASSED_SUITES++))
else
    ((FAILED_SUITES++))
fi
```

### Adding Unit Tests

#### Go Driver Tests

```go
// drivers/mydriver/driver_test.go
package mydriver

import (
    "testing"
    "github.com/wazeos/wazeos/v2/kernel/iobus"
)

func TestDriverCall(t *testing.T) {
    driver := &MyDriver{}
    ctx := createTestContext()

    req := iobus.Request{
        URI: "myscheme://test",
        Operation: iobus.OpCall,
    }

    resp, err := driver.Call(ctx, req)
    if err != nil {
        t.Fatalf("Call failed: %v", err)
    }

    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

#### Rust App Tests

```rust
// apps/myapp/tests/integration_test.rs
use wazeos_sdk::{Context, mcp_tool};

#[test]
fn test_app_logic() {
    let ctx = create_test_context();
    let result = my_tool(&ctx, input);
    assert!(result.is_ok());
}
```

## Test Coverage Goals

| Component | Target Coverage |
|-----------|----------------|
| Core kernel | 90%+ |
| Drivers | 80%+ |
| Apps | 70%+ |
| E2E scenarios | 100% pass |

## Troubleshooting

### Tests Fail with "command not found"

Ensure wazeos binary is built:
```bash
cd v2/kernel
go build -o ../bin/wazeos ./...
```

### Tests Fail with Permission Errors

Check file permissions:
```bash
chmod +x tests/e2e/*.sh
chmod -R 755 v2/bin/
```

### Mock Behavior

In early development, some tests use mocked responses. This is expected and indicated by warnings in test output.

As drivers are implemented, tests will switch from mocks to real implementations automatically.

## Performance Benchmarks

E2E test suite should complete in:
- **Target**: < 10 seconds total
- **Current**: ~15 seconds (includes build time)

Individual scenario targets:
- Time app: < 3 seconds
- Random app: < 4 seconds
- Temp app: < 3 seconds

## Contributing

When adding features:
1. ✅ Add unit tests for the feature
2. ✅ Update or add E2E scenario if needed
3. ✅ Ensure all tests pass
4. ✅ Update this README if new test categories added

---

**Last Updated**: 2026-05-08
**Test Coverage**: Unit tests TODO, E2E tests complete
