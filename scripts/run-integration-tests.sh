#!/bin/bash

# MCP Integration Test Runner
# This script runs comprehensive integration tests for the MCP provider

set -e

echo "🧪 MCP Integration Test Runner"
echo "=============================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TEST_TIMEOUT=${TEST_TIMEOUT:-5m}
VERBOSE=${VERBOSE:-false}
SKIP_REAL_SERVERS=${SKIP_REAL_SERVERS:-false}

echo "Configuration:"
echo "  Timeout: $TEST_TIMEOUT"
echo "  Verbose: $VERBOSE"
echo "  Skip Real Servers: $SKIP_REAL_SERVERS"
echo ""

# Function to run test with status reporting
run_test() {
    local test_name="$1"
    local test_pattern="$2"
    local description="$3"
    
    echo -n "Running $description... "
    
    # Build test command
    local cmd="go test ./internal/model/providers/mcp -timeout $TEST_TIMEOUT -run \"$test_pattern\""
    if [ "$VERBOSE" = "true" ]; then
        cmd="$cmd -v"
    fi
    
    # Run test and capture output
    if output=$(eval "$cmd" 2>&1); then
        echo -e "${GREEN}PASS${NC}"
        if [ "$VERBOSE" = "true" ]; then
            echo "$output" | sed 's/^/  /'
        fi
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "$output" | sed 's/^/  /'
        return 1
    fi
}

# Test results tracking
total_tests=0
passed_tests=0

# Core functionality tests
echo -e "${BLUE}Core Functionality Tests${NC}"
echo "========================"

tests=(
    "stdio_transport:TestStdioTransport:Stdio Transport Tests"
    "simple_integration:TestSimpleIntegration:Simple Integration Tests"
    "process_manager:TestProcessManager:Process Manager Tests"
)

for test in "${tests[@]}"; do
    IFS=':' read -r name pattern description <<< "$test"
    total_tests=$((total_tests + 1))
    if run_test "$name" "$pattern" "$description"; then
        passed_tests=$((passed_tests + 1))
    fi
done

echo ""

# Real server tests (optional)
if [ "$SKIP_REAL_SERVERS" != "true" ]; then
    echo -e "${BLUE}Real Server Integration Tests${NC}"
    echo "============================="
    
    # Check for required tools
    missing_tools=()
    
    if ! command -v echo >/dev/null 2>&1; then
        missing_tools+=("echo")
    fi
    
    if ! command -v python3 >/dev/null 2>&1; then
        missing_tools+=("python3")
    fi
    
    if ! command -v node >/dev/null 2>&1; then
        missing_tools+=("node")
    fi
    
    if ! command -v npx >/dev/null 2>&1; then
        missing_tools+=("npx")
    fi
    
    if [ ${#missing_tools[@]} -gt 0 ]; then
        echo -e "${YELLOW}Warning: Missing tools: ${missing_tools[*]}${NC}"
        echo "Some real server tests may be skipped"
    fi
    
    # Environment check
    if [ -z "$GITHUB_TOKEN" ]; then
        echo -e "${YELLOW}Warning: GITHUB_TOKEN not set. GitHub MCP tests will be skipped${NC}"
    fi
    
    # Real server tests
    real_server_tests=(
        "integration_real:TestIntegration_RealServers:Real MCP Server Tests"
        "integration_pm:TestIntegration_ProcessManager:Process Manager Integration"
        "integration_recovery:TestIntegration_ErrorRecovery:Error Recovery Integration"
    )
    
    for test in "${real_server_tests[@]}"; do
        IFS=':' read -r name pattern description <<< "$test"
        total_tests=$((total_tests + 1))
        if run_test "$name" "$pattern" "$description"; then
            passed_tests=$((passed_tests + 1))
        fi
    done
else
    echo -e "${YELLOW}Skipping real server tests (SKIP_REAL_SERVERS=true)${NC}"
fi

echo ""

# Summary
echo -e "${BLUE}Test Summary${NC}"
echo "============"
echo "Total tests: $total_tests"
echo -e "Passed: ${GREEN}$passed_tests${NC}"
echo -e "Failed: ${RED}$((total_tests - passed_tests))${NC}"

if [ $passed_tests -eq $total_tests ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi