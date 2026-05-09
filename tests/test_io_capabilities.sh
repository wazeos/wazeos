#!/bin/bash
# Comprehensive test suite for WazeOS I/O capabilities
set -e

echo "========================================="
echo "WazeOS I/O Capabilities Test Suite"
echo "========================================="
echo ""

cd "$(dirname "$0")/.."

# Colors for output
GREEN='\033[0.32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

pass() {
    echo -e "${GREEN}✓${NC} $1"
}

fail() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

# Test 1: File Write
echo "Test 1: File Write"
RESULT=$(echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"write_file","path":"/tmp/wazeos-io-test.txt","content":"Hello from WazeOS!"}}}' | ./wazeos mcp server 2>/dev/null | head -1 | jq -r '.result.content[0].text | fromjson | .bytes_written')

if [ "$RESULT" = "19" ]; then
    pass "File write successful (19 bytes)"
else
    fail "File write failed"
fi

# Verify file exists
if [ -f "/tmp/wazeos-io-test.txt" ]; then
    CONTENT=$(cat /tmp/wazeos-io-test.txt)
    if [ "$CONTENT" = "Hello from WazeOS!" ]; then
        pass "File content verified"
    else
        fail "File content mismatch"
    fi
else
    fail "File was not created"
fi
echo ""

# Test 2: File Read
echo "Test 2: File Read"
RESULT=$(echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"read_file","path":"/tmp/wazeos-io-test.txt"}}}' | ./wazeos mcp server 2>/dev/null | head -1 | jq -r '.result.content[0].text | fromjson | .content')

if [ "$RESULT" = "Hello from WazeOS!" ]; then
    pass "File read successful"
else
    fail "File read failed: got '$RESULT'"
fi
echo ""

# Test 3: Shell Command Execution
echo "Test 3: Shell Command"
RESULT=$(echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"shell","command":"echo WazeOS"}}}' | ./wazeos mcp server 2>/dev/null | head -1 | jq -r '.result.content[0].text | fromjson | .output')

if [ "$RESULT" = "WazeOS" ]; then
    pass "Shell command successful"
else
    fail "Shell command failed: got '$RESULT'"
fi
echo ""

# Test 4: Permission Enforcement - Try to read unauthorized file
echo "Test 4: Permission Enforcement"
# Create file outside /tmp
echo "secret" > /var/tmp/secret-file.txt 2>/dev/null || true
RESULT=$(echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"read_file","path":"/var/tmp/secret-file.txt"}}}' | ./wazeos mcp server 2>/dev/null | head -1 | jq -r '.result.isError')

if [ "$RESULT" = "true" ]; then
    pass "Permission enforcement working (unauthorized access blocked)"
else
    fail "Permission enforcement failed (should have been blocked)"
fi
echo ""

# Test 5: Multiple Operations in Sequence
echo "Test 5: Sequential Operations"
# Write, read, modify, read again
echo '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"write_file","path":"/tmp/sequence-test.txt","content":"Step 1"}}}' | ./wazeos mcp server 2>/dev/null > /dev/null

STEP1=$(echo '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"read_file","path":"/tmp/sequence-test.txt"}}}' | ./wazeos mcp server 2>/dev/null | head -1 | jq -r '.result.content[0].text | fromjson | .content')

echo '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"write_file","path":"/tmp/sequence-test.txt","content":"Step 2"}}}' | ./wazeos mcp server 2>/dev/null > /dev/null

STEP2=$(echo '{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"test-tool","arguments":{"operation":"read_file","path":"/tmp/sequence-test.txt"}}}' | ./wazeos mcp server 2>/dev/null | head -1 | jq -r '.result.content[0].text | fromjson | .content')

if [ "$STEP1" = "Step 1" ] && [ "$STEP2" = "Step 2" ]; then
    pass "Sequential operations successful"
else
    fail "Sequential operations failed"
fi
echo ""

# Cleanup
rm -f /tmp/wazeos-io-test.txt /tmp/sequence-test.txt /var/tmp/secret-file.txt 2>/dev/null || true

echo "========================================="
echo "All tests passed! ✓"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✓ File write operations"
echo "  ✓ File read operations"
echo "  ✓ Shell command execution"
echo "  ✓ Permission enforcement"
echo "  ✓ Sequential operations"
echo ""
echo "WazeOS I/O capabilities are fully functional!"
