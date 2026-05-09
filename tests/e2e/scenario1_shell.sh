#!/bin/bash
set -e
echo "Testing Scenario 1: Shell Driver + Time App"
echo "  Test 1.1: Shell driver executes 'date' command"
RESULT=$(cd ../../ && go run ./tests/e2e/helpers/test_shell_driver.go 2>&1)
if echo "$RESULT" | grep -q "SUCCESS"; then
    echo "    ✓ Shell driver works"
else
    echo "    ✗ Shell driver failed: $RESULT"
    exit 1
fi
echo "  Test 1.2: Time app WASM exists"
if [ -f "../../examples/apps/time/target/wasm32-wasip1/release/time_app.wasm" ]; then
    echo "    ✓ Time app WASM found"
else
    echo "    ✗ Time app WASM not found"
    exit 1
fi
echo "  Test 1.3: WASM size is reasonable"
ls -lh "../../examples/apps/time/target/wasm32-wasip1/release/time_app.wasm" | grep -q "K"
if [ $? -eq 0 ]; then
    echo "    ✓ WASM size OK"
else
    echo "    ✗ WASM size check failed"
    exit 1
fi
echo "  ✓ Scenario 1 passed"
