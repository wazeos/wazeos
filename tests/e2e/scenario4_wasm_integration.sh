#!/bin/bash
set -e

echo "Testing Scenario 4: Full WASM Integration"
echo "  This test verifies the complete stack:"
echo "  - WASM Runtime Driver"
echo "  - Host Functions"
echo "  - IO Bus Integration"
echo ""

# Test 4.1: Load and execute a simple WASM module
echo "  Test 4.1: Load and execute WASM module"
RESULT=$(cd ../../ && go run ./tests/e2e/helpers/test_wasm_integration.go 2>&1)
if echo "$RESULT" | grep -q "SUCCESS"; then
    echo "    ✓ WASM module loaded and executed"
else
    echo "    ✗ WASM execution failed: $RESULT"
    exit 1
fi

# Test 4.2: Verify all WASM apps are built
echo "  Test 4.2: Verify all WASM apps are built"
APPS_BUILT=true
for app in time random temp; do
    if [ ! -f "../../examples/apps/$app/target/wasm32-wasip1/release/${app}_app.wasm" ]; then
        echo "    ✗ $app app WASM not found"
        APPS_BUILT=false
    fi
done

if [ "$APPS_BUILT" = true ]; then
    echo "    ✓ All WASM apps built"
else
    exit 1
fi

# Test 4.3: Check WASM driver is registered
echo "  Test 4.3: Verify WASM runtime driver is registered"
if echo "$RESULT" | grep -q "wasm-runtime"; then
    echo "    ✓ WASM runtime driver registered"
else
    echo "    ✗ WASM runtime driver not found"
    exit 1
fi

echo "  ✓ Scenario 4 passed - Full WASM stack operational"
