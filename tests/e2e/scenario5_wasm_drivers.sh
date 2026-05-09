#!/bin/bash
set -e
echo "Testing Scenario 5: WASM Drivers (Shell, HTTP, File)"

# Test 5.1: WASM Shell Driver
echo "  Test 5.1: WASM shell driver via native executor"
RESULT=$(cd ../../ && go run ./tests/e2e/helpers/test_wasm_shell_driver.go 2>&1)
if echo "$RESULT" | grep -q "SUCCESS"; then
    echo "    ✓ WASM shell driver works"
else
    echo "    ✗ WASM shell driver failed:"
    echo "$RESULT"
    exit 1
fi

# Test 5.2: WASM HTTP Driver
echo "  Test 5.2: WASM HTTP driver via native executor"
RESULT=$(cd ../../ && go run ./tests/e2e/helpers/test_wasm_http_driver.go 2>&1)
if echo "$RESULT" | grep -q "SUCCESS"; then
    echo "    ✓ WASM HTTP driver works"
else
    echo "    ✗ WASM HTTP driver failed:"
    echo "$RESULT"
    exit 1
fi

# Test 5.3: WASM File Driver
echo "  Test 5.3: WASM file driver via native executor"
RESULT=$(cd ../../ && go run ./tests/e2e/helpers/test_wasm_file_driver.go 2>&1)
if echo "$RESULT" | grep -q "SUCCESS"; then
    echo "    ✓ WASM file driver works"
else
    echo "    ✗ WASM file driver failed:"
    echo "$RESULT"
    exit 1
fi

echo "  ✓ Scenario 5 passed"
