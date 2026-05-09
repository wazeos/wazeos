#!/bin/bash
# WazeOS v2 E2E Test Runner
set -e
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
PASSED=0
FAILED=0
SKIPPED=0

echo "========================================"
echo "   WazeOS v2 E2E Test Suite"
echo "========================================"
echo ""

run_test() {
    local test_name=$1
    local test_script=$2
    echo -n "Running: $test_name ... "
    if [ ! -f "$test_script" ]; then
        echo -e "${YELLOW}SKIPPED${NC} (script not found)"
        SKIPPED=$((SKIPPED + 1))
        return
    fi
    if bash "$test_script" > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAIL${NC}"
        FAILED=$((FAILED + 1))
    fi
}

echo "Running E2E Test Scenarios:"
echo ""

run_test "Scenario 1: Shell Driver (Time App)" "./scenario1_shell.sh"
run_test "Scenario 2: HTTP Driver (Random App)" "./scenario2_http.sh"
run_test "Scenario 3: File Driver (Temp App)" "./scenario3_file.sh"
run_test "Scenario 4: WASM Integration (Full Stack)" "./scenario4_wasm_integration.sh"
run_test "Scenario 5: WASM Drivers (Shell, HTTP, File)" "./scenario5_wasm_drivers.sh"

echo ""
echo "========================================"
echo "   Test Results"
echo "========================================"
echo -e "Passed:  ${GREEN}$PASSED${NC}"
echo -e "Failed:  ${RED}$FAILED${NC}"
echo -e "Skipped: ${YELLOW}$SKIPPED${NC}"
echo "----------------------------------------"
echo "Total:   $((PASSED + FAILED + SKIPPED))"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
else
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
fi
