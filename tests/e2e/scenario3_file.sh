#!/bin/bash
set -e
echo "Testing Scenario 3: File Driver + Temp App"
echo "  Test 3.1: File driver lists directory"
RESULT=$(cd ../../ && go run ./tests/e2e/helpers/test_file_driver.go 2>&1)
if echo "$RESULT" | grep -q "200"; then
    echo "    ✓ File driver works"
else
    echo "    ✗ File driver failed: $RESULT"
    exit 1
fi
echo "  Test 3.2: Temp app WASM exists"
if [ -f "../../examples/apps/temp/target/wasm32-wasip1/release/temp_app.wasm" ]; then
    echo "    ✓ Temp app WASM found"
else
    echo "    ✗ Temp app WASM not found"
    exit 1
fi
echo "  Test 3.3: WASM size is reasonable"
ls -lh "../../examples/apps/temp/target/wasm32-wasip1/release/temp_app.wasm" | grep -q "K"
if [ $? -eq 0 ]; then
    echo "    ✓ WASM size OK"
else
    echo "    ✗ WASM size check failed"
    exit 1
fi
echo "  ✓ Scenario 3 passed"
