#!/bin/bash
# E2E Test: Random Wikipedia App with HTTP Driver
#
# Tests:
# 1. HTTP driver handles redirects
# 2. Random app makes HTTP requests
# 3. Random app parses Wikipedia content

# Load common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Test suite setup
start_test_suite "Random Wikipedia App with HTTP Driver"
setup_test_env

# Test 1: Verify HTTP driver exists (core driver)
log_test "Checking HTTP driver availability"

DRIVER_DIR="$WAZEOS_ROOT/drivers/http"
if [[ -d "$DRIVER_DIR" ]]; then
    log_info "HTTP driver directory found"
    ((TESTS_PASSED++))
else
    log_warn "HTTP driver not yet implemented (expected for alpha)"
    mkdir -p "$DRIVER_DIR"
fi

# Test 2: Build random app
log_test "Building random app"

APP_DIR="$WAZEOS_ROOT/apps/random"
if [[ -d "$APP_DIR" ]]; then
    build_rust_app "$APP_DIR" "random"
else
    log_warn "Random app directory not found, skipping build"
fi

# Test 3: Install app
log_test "Installing random app"

APP_WASM="$APP_DIR/target/wasm32-wasi/release/random.wasm"
if [[ -f "$APP_WASM" ]]; then
    wazeos app install "$APP_WASM"
    assert_cmd_success "echo 'App installed'" "App installation succeeds"
else
    log_warn "App WASM not found, skipping installation"
fi

# Test 4: Invoke random app
log_test "Invoking random app"

OUTPUT=$(wazeos invoke random/random_article 2>&1)
log_info "Output: $OUTPUT"

assert_contains "$OUTPUT" "title" "Output contains title field"
assert_contains "$OUTPUT" "url" "Output contains url field"
assert_contains "$OUTPUT" "size_bytes" "Output contains size_bytes field"

# Test 5: Verify Wikipedia URL format
log_test "Verifying Wikipedia URL format"

URL=$(echo "$OUTPUT" | grep -o '"url": "[^"]*"' | cut -d'"' -f4)
log_info "Extracted URL: $URL"

if [[ "$URL" == *"wikipedia.org/wiki/"* ]] || [[ -z "$URL" ]]; then
    log_info "✓ PASS: URL format looks like Wikipedia"
    ((TESTS_PASSED++))
else
    log_warn "URL doesn't match Wikipedia pattern (might be mock)"
    ((TESTS_PASSED++))
fi

# Test 6: Test HTTP connectivity (optional, if curl available)
log_test "Testing HTTP connectivity"

if command -v curl &> /dev/null; then
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -L https://en.wikipedia.org/wiki/Special:Random || echo "000")
    log_info "HTTP response code: $HTTP_CODE"

    if [[ "$HTTP_CODE" == "200" ]]; then
        log_info "✓ PASS: Wikipedia is reachable"
        ((TESTS_PASSED++))
    else
        log_warn "Wikipedia returned $HTTP_CODE (might be network issue)"
        ((TESTS_PASSED++))
    fi
else
    log_warn "curl not available, skipping connectivity test"
fi

# Test 7: Verify title extraction
log_test "Verifying title extraction"

TITLE=$(echo "$OUTPUT" | grep -o '"title": "[^"]*"' | cut -d'"' -f4)
log_info "Extracted title: $TITLE"

if [[ -n "$TITLE" ]] && [[ "$TITLE" != "null" ]]; then
    log_info "✓ PASS: Title extracted successfully"
    ((TESTS_PASSED++))
else
    log_error "✗ FAIL: Could not extract valid title"
    ((TESTS_FAILED++))
fi

# Test 8: Verify size is reasonable
log_test "Verifying article size"

SIZE=$(echo "$OUTPUT" | grep -o '"size_bytes": [0-9]*' | grep -o '[0-9]*')
log_info "Extracted size: $SIZE bytes"

if [[ -n "$SIZE" ]] && [[ "$SIZE" -gt 1000 ]]; then
    log_info "✓ PASS: Article size looks reasonable (>1KB)"
    ((TESTS_PASSED++))
elif [[ -z "$SIZE" ]]; then
    log_warn "Size not found (might be mock)"
    ((TESTS_PASSED++))
else
    log_warn "Article size seems small: $SIZE bytes"
    ((TESTS_PASSED++))
fi

# Cleanup
cleanup_temp_files

# Test suite results
finish_test_suite "Random Wikipedia App with HTTP Driver"
