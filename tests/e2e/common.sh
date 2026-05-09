#!/bin/bash
# Common utilities for E2E tests

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test result tracking
TESTS_PASSED=0
TESTS_FAILED=0

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    echo -e "\n${YELLOW}▶ TEST:${NC} $1"
}

# Test assertion functions
assert_eq() {
    local actual="$1"
    local expected="$2"
    local message="${3:-Values should be equal}"

    if [[ "$actual" == "$expected" ]]; then
        log_info "✓ PASS: $message"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "✗ FAIL: $message"
        log_error "  Expected: $expected"
        log_error "  Actual:   $actual"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="${3:-String should contain substring}"

    if [[ "$haystack" == *"$needle"* ]]; then
        log_info "✓ PASS: $message"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "✗ FAIL: $message"
        log_error "  Expected to find: $needle"
        log_error "  In: $haystack"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_file_exists() {
    local file="$1"
    local message="${2:-File should exist: $file}"

    if [[ -f "$file" ]]; then
        log_info "✓ PASS: $message"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "✗ FAIL: $message"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_cmd_success() {
    local cmd="$1"
    local message="${2:-Command should succeed: $cmd}"

    if eval "$cmd" > /dev/null 2>&1; then
        log_info "✓ PASS: $message"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "✗ FAIL: $message"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Test suite functions
start_test_suite() {
    local suite_name="$1"
    echo ""
    echo "========================================"
    echo "  E2E Test Suite: $suite_name"
    echo "========================================"
    echo ""
}

finish_test_suite() {
    local suite_name="$1"
    echo ""
    echo "========================================"
    echo "  Test Results: $suite_name"
    echo "========================================"
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    echo ""

    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
        return 0
    else
        echo -e "${RED}✗ SOME TESTS FAILED${NC}"
        return 1
    fi
}

# Build functions
build_go_driver() {
    local driver_path="$1"
    local driver_name="$2"

    log_info "Building Go driver: $driver_name"

    cd "$driver_path"
    go build -o "${driver_name}.so" .
    cd - > /dev/null

    assert_file_exists "$driver_path/${driver_name}.so" "Driver binary created"
}

build_rust_app() {
    local app_path="$1"
    local app_name="$2"

    log_info "Building Rust app: $app_name"

    cd "$app_path"
    cargo build --release --target wasm32-wasi 2>&1 | grep -v "warning:" || true
    cd - > /dev/null

    assert_file_exists "$app_path/target/wasm32-wasi/release/${app_name}.wasm" "App WASM binary created"
}

# Cleanup functions
cleanup_temp_files() {
    log_info "Cleaning up temporary files"
    rm -f /tmp/wazeos-test-* 2>/dev/null || true
}

# Setup test environment
setup_test_env() {
    export WAZEOS_ROOT="${WAZEOS_ROOT:-$(pwd)/v2}"
    export WAZEOS_BIN="${WAZEOS_BIN:-${WAZEOS_ROOT}/bin/wazeos}"
    export PATH="${WAZEOS_ROOT}/bin:$PATH"

    log_info "WAZEOS_ROOT: $WAZEOS_ROOT"
    log_info "WAZEOS_BIN: $WAZEOS_BIN"
}

# Mock wazeos commands (until CLI is built)
wazeos() {
    local cmd="$1"
    shift

    case "$cmd" in
        "driver")
            mock_driver_cmd "$@"
            ;;
        "app")
            mock_app_cmd "$@"
            ;;
        "invoke")
            mock_invoke_cmd "$@"
            ;;
        *)
            log_error "Unknown wazeos command: $cmd"
            return 1
            ;;
    esac
}

mock_driver_cmd() {
    local subcmd="$1"
    shift

    case "$subcmd" in
        "install")
            log_info "Mock: Installing driver $1"
            return 0
            ;;
        *)
            log_error "Unknown driver subcommand: $subcmd"
            return 1
            ;;
    esac
}

mock_app_cmd() {
    local subcmd="$1"
    shift

    case "$subcmd" in
        "install")
            log_info "Mock: Installing app $1"
            return 0
            ;;
        *)
            log_error "Unknown app subcommand: $subcmd"
            return 1
            ;;
    esac
}

mock_invoke_cmd() {
    local tool_name="$1"
    shift

    log_info "Mock: Invoking tool $tool_name"

    # Mock responses for each tool
    case "$tool_name" in
        "time/get_time")
            echo '{"local_time": "2026-05-08 14:30:00 PDT", "source": "shell:date"}'
            ;;
        "random/random_article")
            echo '{"title": "Quantum Entanglement", "url": "https://en.wikipedia.org/wiki/Quantum_Entanglement", "size_bytes": 125847}'
            ;;
        "temp/list_and_save")
            echo '{"files_listed": 42, "file_created": "/tmp/contents.txt", "content_length": 2847, "verification": "PASS", "preview": "Files in /tmp:\\n\\nDIR           0 bytes  logs\\nFILE       1024 bytes  test.txt"}'
            ;;
        *)
            log_error "Unknown tool: $tool_name"
            return 1
            ;;
    esac
}
