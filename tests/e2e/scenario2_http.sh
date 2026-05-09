#!/bin/bash
set -e
echo "Testing Scenario 2: HTTP Driver + Random App"
echo "  Test 2.1: HTTP driver makes GET request"
RESULT=$(cd ../../ && go run ./tests/e2e/helpers/test_http_driver.go 2>&1)
if echo "$RESULT" | grep -q "200"; then
    echo "    ✓ HTTP driver works"
else
    echo "    ✗ HTTP driver failed: $RESULT"
    exit 1
fi
echo "  Test 2.2: Random app WASM exists"
if [ -f "../../examples/apps/random/target/wasm32-wasip1/release/random_app.wasm" ]; then
    echo "    ✓ Random app WASM found"
else
    echo "    ✗ Random app WASM not found"
    exit 1
fi
echo "  Test 2.3: WASM size is reasonable"
ls -lh "../../examples/apps/random/target/wasm32-wasip1/release/random_app.wasm" | grep -q "K"
if [ $? -eq 0 ]; then
    echo "    ✓ WASM size OK"
else
    echo "    ✗ WASM size check failed"
    exit 1
fi
echo "  ✓ Scenario 2 passed"
