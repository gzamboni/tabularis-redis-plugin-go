#!/bin/bash

################################################################################
# Redis Pub/Sub End-to-End Test Script (Single Process)
# 
# This script tests Pub/Sub functionality by sending all requests to a single
# plugin instance via stdin, which is how the plugin is designed to work.
################################################################################

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

# Connection parameters
REDIS_PORT=${REDIS_PORT:-6379}
REDIS_HOST=${REDIS_HOST:-localhost}
REDIS_DB=${REDIS_DB:-0}

# Helper function to print colored output
print_info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

print_success() {
    echo -e "${GREEN}✅ ${NC}$1"
    ((TESTS_PASSED++))
    ((TOTAL_TESTS++))
}

print_error() {
    echo -e "${RED}❌ ${NC}$1"
    ((TESTS_FAILED++))
    ((TOTAL_TESTS++))
}

print_header() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo ""
}

################################################################################
# SETUP
################################################################################

print_header "SETUP: Building Plugin and Checking Redis"

# Build the plugin
print_info "Building plugin..."
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go
if [ $? -eq 0 ]; then
    print_success "Plugin built successfully"
else
    print_error "Failed to build plugin"
    exit 1
fi

# Check if Redis is running
print_info "Checking Redis connection on $REDIS_HOST:$REDIS_PORT..."
if command -v redis-cli &> /dev/null; then
    if redis-cli -h $REDIS_HOST -p $REDIS_PORT ping > /dev/null 2>&1; then
        print_success "Redis is running and accessible"
    else
        print_error "Redis is not accessible at $REDIS_HOST:$REDIS_PORT"
        exit 1
    fi
fi

################################################################################
# RUN ALL TESTS IN SINGLE PROCESS
################################################################################

print_header "Running All Pub/Sub Tests"

# Create a temporary file for all requests
REQUESTS_FILE=$(mktemp)
trap "rm -f $REQUESTS_FILE" EXIT

# Build all JSON-RPC requests
cat > $REQUESTS_FILE << EOF
{"jsonrpc":"2.0","id":1,"method":"test_connection","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"}}}
{"jsonrpc":"2.0","id":2,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"},"channel":"test_channel","is_pattern":false,"buffer_size":100,"ttl":300}}
{"jsonrpc":"2.0","id":3,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"},"channel":"user:*","is_pattern":true,"buffer_size":100,"ttl":300}}
{"jsonrpc":"2.0","id":4,"method":"execute_query","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"},"query":"SELECT * FROM pubsub_subscriptions"}}
{"jsonrpc":"2.0","id":5,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"},"channel":"test_channel","message":"Hello from test!"}}
{"jsonrpc":"2.0","id":6,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"},"channel":"user:123","message":"User 123 logged in"}}
{"jsonrpc":"2.0","id":7,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"},"channel":"user:456","message":"User 456 logged in"}}
{"jsonrpc":"2.0","id":8,"method":"execute_query","params":{"params":{"driver":"redis","host":"$REDIS_HOST","port":$REDIS_PORT,"database":"$REDIS_DB"},"query":"SELECT * FROM pubsub_channels"}}
EOF

# Run all requests through single plugin instance
RESPONSES=$(cat $REQUESTS_FILE | ./tabularis-redis-plugin-go 2>/dev/null)

# Parse and validate responses
echo "$RESPONSES" | while IFS= read -r response; do
    id=$(echo "$response" | grep -o '"id":[0-9]*' | sed 's/"id"://')
    
    case $id in
        1)
            if echo "$response" | grep -q '"success":true'; then
                print_success "TEST 1: Connection test passed"
            else
                print_error "TEST 1: Connection test failed"
            fi
            ;;
        2)
            if echo "$response" | grep -q '"subscription_id"'; then
                print_success "TEST 2: Subscribed to exact channel"
            else
                print_error "TEST 2: Failed to subscribe to exact channel"
            fi
            ;;
        3)
            if echo "$response" | grep -q '"subscription_id"'; then
                print_success "TEST 3: Subscribed to pattern channel"
            else
                print_error "TEST 3: Failed to subscribe to pattern channel"
            fi
            ;;
        4)
            if echo "$response" | grep -q '"total_rows":2'; then
                print_success "TEST 4: pubsub_subscriptions table shows both subscriptions"
            else
                print_error "TEST 4: pubsub_subscriptions table query failed"
            fi
            ;;
        5)
            if echo "$response" | grep -q '"success":true'; then
                print_success "TEST 5: Published message to exact channel"
            else
                print_error "TEST 5: Failed to publish to exact channel"
            fi
            ;;
        6)
            if echo "$response" | grep -q '"success":true'; then
                print_success "TEST 6: Published message to pattern-matched channel (user:123)"
            else
                print_error "TEST 6: Failed to publish to pattern-matched channel"
            fi
            ;;
        7)
            if echo "$response" | grep -q '"success":true'; then
                print_success "TEST 7: Published message to pattern-matched channel (user:456)"
            else
                print_error "TEST 7: Failed to publish second pattern message"
            fi
            ;;
        8)
            if echo "$response" | grep -q '"channel"'; then
                print_success "TEST 8: pubsub_channels table query successful"
            else
                print_error "TEST 8: pubsub_channels table query failed"
            fi
            ;;
    esac
done

################################################################################
# SUMMARY
################################################################################

print_header "TEST SUMMARY"

echo ""
echo "Total Tests: $TOTAL_TESTS"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed successfully!${NC}"
    exit 0
else
    echo -e "${RED}⚠️  Some tests failed. Please review the output above.${NC}"
    exit 1
fi
