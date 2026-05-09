#!/bin/bash
# Master E2E Test Runner
#
# Runs all three end-to-end test scenarios:
# 1. Time app with shell driver
# 2. Random Wikipedia app with HTTP driver
# 3. Temp file manager app with file driver

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track overall results
TOTAL_SUITES=0
PASSED_SUITES=0
FAILED_SUITES=0

echo ""
echo "========================================"
echo "  WazeOS v2 E2E Test Suite"
echo "========================================"
echo ""
echo "Running 3 test scenarios..."
echo ""

# Test 1: Time App
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}▶ Scenario 1: Time App with Shell Driver${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

((TOTAL_SUITES++))
if bash "$SCRIPT_DIR/test_time_app.sh"; then
    echo -e "${GREEN}✓ Scenario 1 PASSED${NC}"
    ((PASSED_SUITES++))
else
    echo -e "${RED}✗ Scenario 1 FAILED${NC}"
    ((FAILED_SUITES++))
fi

echo ""

# Test 2: Random App
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}▶ Scenario 2: Random Wikipedia App${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

((TOTAL_SUITES++))
if bash "$SCRIPT_DIR/test_random_app.sh"; then
    echo -e "${GREEN}✓ Scenario 2 PASSED${NC}"
    ((PASSED_SUITES++))
else
    echo -e "${RED}✗ Scenario 2 FAILED${NC}"
    ((FAILED_SUITES++))
fi

echo ""

# Test 3: Temp App
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}▶ Scenario 3: Temp File Manager App${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

((TOTAL_SUITES++))
if bash "$SCRIPT_DIR/test_temp_app.sh"; then
    echo -e "${GREEN}✓ Scenario 3 PASSED${NC}"
    ((PASSED_SUITES++))
else
    echo -e "${RED}✗ Scenario 3 FAILED${NC}"
    ((FAILED_SUITES++))
fi

echo ""

# Overall results
echo "========================================"
echo "  Overall Test Results"
echo "========================================"
echo -e "${GREEN}Passed: $PASSED_SUITES / $TOTAL_SUITES${NC}"
echo -e "${RED}Failed: $FAILED_SUITES / $TOTAL_SUITES${NC}"
echo ""

if [[ $FAILED_SUITES -eq 0 ]]; then
    echo -e "${GREEN}✓✓✓ ALL E2E TESTS PASSED ✓✓✓${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}✗✗✗ SOME E2E TESTS FAILED ✗✗✗${NC}"
    echo ""
    echo "Review the test output above for details."
    exit 1
fi
