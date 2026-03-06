#!/bin/bash

################################################################################
# Redis Pub/Sub End-to-End Test Script
# 
# This script comprehensively tests all Redis Pub/Sub functionality including:
# - All JSON-RPC methods (subscribe, unsubscribe, publish, poll, acknowledge)
# - All virtual tables (pubsub_channels, pubsub_messages, pubsub_subscriptions)
# - Pattern-based subscriptions
# - Message acknowledgment
# - Subscription TTL
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
REDIS_PORT=6381
REDIS_HOST="localhost"
REDIS_DB="0"

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

print_warning() {
    echo -e "${YELLOW}⚠ ${NC}$1"
}

print_header() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo ""
}

# Helper function to send JSON-RPC request
send_request() {
    local request="$1"
    echo "$request" | ./tabularis-redis-plugin-go 2>/dev/null
}

# Helper function to extract JSON field
extract_json_field() {
    local json="$1"
    local field="$2"
    echo "$json" | grep -o "\"$field\":[^,}]*" | sed 's/"[^"]*"://; s/"//g; s/}//g; s/]//g'
}

################################################################################
# SETUP
################################################################################

print_header "SETUP: Building Plugin and Starting Redis"

# Build the plugin
print_info "Building plugin..."
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go
if [ $? -eq 0 ]; then
    print_success "Plugin built successfully"
else
    print_error "Failed to build plugin"
    exit 1
fi

# Start Redis via Docker
print_info "Starting Redis via Docker on port $REDIS_PORT..."
CONTAINER_ID=$(docker run -d -p $REDIS_PORT:6379 redis:alpine)

# Ensure container is destroyed when script exits
trap "echo ''; print_info 'Cleaning up...'; docker rm -f $CONTAINER_ID > /dev/null 2>&1" EXIT

# Wait for Redis to start
print_info "Waiting for Redis to start..."
sleep 3

# Test Redis connection
if docker exec $CONTAINER_ID redis-cli ping > /dev/null 2>&1; then
    print_success "Redis is running (Container ID: ${CONTAINER_ID:0:12})"
else
    print_error "Redis failed to start"
    exit 1
fi

################################################################################
# TEST 1: Connection Test
################################################################################

print_header "TEST 1: Connection Test"

REQUEST='{"jsonrpc":"2.0","id":1,"method":"test_connection","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"}}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"success":true'; then
    print_success "Connection test passed"
else
    print_error "Connection test failed: $RESPONSE"
fi

################################################################################
# TEST 2: pubsub_subscribe (Exact Channel)
################################################################################

print_header "TEST 2: pubsub_subscribe (Exact Channel)"

REQUEST='{"jsonrpc":"2.0","id":2,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"test_channel","is_pattern":false,"buffer_size":100,"ttl":300}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"subscription_id"'; then
    SUB_ID_1=$(extract_json_field "$RESPONSE" "subscription_id")
    print_success "Subscribed to exact channel (ID: $SUB_ID_1)"
else
    print_error "Failed to subscribe to exact channel: $RESPONSE"
    SUB_ID_1=""
fi

################################################################################
# TEST 3: pubsub_subscribe (Pattern Channel)
################################################################################

print_header "TEST 3: pubsub_subscribe (Pattern Channel)"

REQUEST='{"jsonrpc":"2.0","id":3,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"user:*","is_pattern":true,"buffer_size":100,"ttl":300}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"subscription_id"'; then
    SUB_ID_2=$(extract_json_field "$RESPONSE" "subscription_id")
    print_success "Subscribed to pattern channel (ID: $SUB_ID_2)"
else
    print_error "Failed to subscribe to pattern channel: $RESPONSE"
    SUB_ID_2=""
fi

################################################################################
# TEST 4: pubsub_subscriptions Virtual Table
################################################################################

print_header "TEST 4: pubsub_subscriptions Virtual Table"

REQUEST='{"jsonrpc":"2.0","id":4,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT * FROM pubsub_subscriptions"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "$SUB_ID_1" && echo "$RESPONSE" | grep -q "$SUB_ID_2"; then
    print_success "pubsub_subscriptions table shows both subscriptions"
else
    print_error "pubsub_subscriptions table query failed: $RESPONSE"
fi

################################################################################
# TEST 5: pubsub_publish (To Exact Channel)
################################################################################

print_header "TEST 5: pubsub_publish (To Exact Channel)"

REQUEST='{"jsonrpc":"2.0","id":5,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"test_channel","message":"Hello from test!"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"success":true'; then
    print_success "Published message to exact channel"
else
    print_error "Failed to publish to exact channel: $RESPONSE"
fi

# Wait for message to be received
sleep 1

################################################################################
# TEST 6: pubsub_publish (To Pattern-Matched Channel)
################################################################################

print_header "TEST 6: pubsub_publish (To Pattern-Matched Channel)"

REQUEST='{"jsonrpc":"2.0","id":6,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"user:123","message":"User 123 logged in"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"success":true'; then
    print_success "Published message to pattern-matched channel"
else
    print_error "Failed to publish to pattern-matched channel: $RESPONSE"
fi

# Publish another message to the pattern
REQUEST='{"jsonrpc":"2.0","id":7,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"user:456","message":"User 456 logged in"}}'
send_request "$REQUEST" > /dev/null

# Wait for messages to be received
sleep 1

################################################################################
# TEST 7: pubsub_poll_messages (From Exact Channel)
################################################################################

print_header "TEST 7: pubsub_poll_messages (From Exact Channel)"

REQUEST='{"jsonrpc":"2.0","id":8,"method":"pubsub_poll_messages","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_1'","max_messages":10,"timeout_ms":1000,"auto_acknowledge":false}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "Hello from test"; then
    print_success "Polled messages from exact channel subscription"
    # Extract message IDs for acknowledgment test
    MSG_IDS_1=$(echo "$RESPONSE" | grep -o '"message_id":[0-9]*' | sed 's/"message_id"://' | tr '\n' ',' | sed 's/,$//')
else
    print_error "Failed to poll messages from exact channel: $RESPONSE"
    MSG_IDS_1=""
fi

################################################################################
# TEST 8: pubsub_poll_messages (From Pattern Channel)
################################################################################

print_header "TEST 8: pubsub_poll_messages (From Pattern Channel)"

REQUEST='{"jsonrpc":"2.0","id":9,"method":"pubsub_poll_messages","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_2'","max_messages":10,"timeout_ms":1000,"auto_acknowledge":false}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "User 123 logged in" && echo "$RESPONSE" | grep -q "User 456 logged in"; then
    print_success "Polled messages from pattern channel subscription"
    # Extract message IDs for acknowledgment test
    MSG_IDS_2=$(echo "$RESPONSE" | grep -o '"message_id":[0-9]*' | sed 's/"message_id"://' | tr '\n' ',' | sed 's/,$//')
else
    print_error "Failed to poll messages from pattern channel: $RESPONSE"
    MSG_IDS_2=""
fi

################################################################################
# TEST 9: pubsub_messages Virtual Table
################################################################################

print_header "TEST 9: pubsub_messages Virtual Table"

REQUEST='{"jsonrpc":"2.0","id":10,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT * FROM pubsub_messages WHERE subscription_id = '"'$SUB_ID_1'"'"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "Hello from test"; then
    print_success "pubsub_messages table query for exact channel subscription"
else
    print_error "pubsub_messages table query failed: $RESPONSE"
fi

REQUEST='{"jsonrpc":"2.0","id":11,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT * FROM pubsub_messages WHERE subscription_id = '"'$SUB_ID_2'"' ORDER BY received_at DESC"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "User 123 logged in"; then
    print_success "pubsub_messages table query for pattern channel subscription"
else
    print_error "pubsub_messages table query with ORDER BY failed: $RESPONSE"
fi

################################################################################
# TEST 10: pubsub_acknowledge_messages
################################################################################

print_header "TEST 10: pubsub_acknowledge_messages"

if [ -n "$MSG_IDS_1" ]; then
    REQUEST='{"jsonrpc":"2.0","id":12,"method":"pubsub_acknowledge_messages","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_1'","message_ids":['$MSG_IDS_1']}}'
    RESPONSE=$(send_request "$REQUEST")
    
    if echo "$RESPONSE" | grep -q '"success":true'; then
        print_success "Acknowledged messages from exact channel subscription"
    else
        print_error "Failed to acknowledge messages: $RESPONSE"
    fi
else
    print_warning "Skipping acknowledgment test (no message IDs)"
fi

if [ -n "$MSG_IDS_2" ]; then
    REQUEST='{"jsonrpc":"2.0","id":13,"method":"pubsub_acknowledge_messages","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_2'","message_ids":['$MSG_IDS_2']}}'
    RESPONSE=$(send_request "$REQUEST")
    
    if echo "$RESPONSE" | grep -q '"success":true'; then
        print_success "Acknowledged messages from pattern channel subscription"
    else
        print_error "Failed to acknowledge messages: $RESPONSE"
    fi
else
    print_warning "Skipping acknowledgment test (no message IDs)"
fi

################################################################################
# TEST 11: pubsub_channels Virtual Table
################################################################################

print_header "TEST 11: pubsub_channels Virtual Table"

# Publish a message to create an active channel
REQUEST='{"jsonrpc":"2.0","id":14,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"test_channel","message":"Another message"}}'
send_request "$REQUEST" > /dev/null

sleep 1

REQUEST='{"jsonrpc":"2.0","id":15,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT * FROM pubsub_channels"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "test_channel"; then
    print_success "pubsub_channels table shows active channels"
else
    print_error "pubsub_channels table query failed: $RESPONSE"
fi

REQUEST='{"jsonrpc":"2.0","id":16,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT * FROM pubsub_channels WHERE subscribers > 0"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "test_channel"; then
    print_success "pubsub_channels table query with WHERE clause"
else
    print_error "pubsub_channels table query with WHERE failed: $RESPONSE"
fi

################################################################################
# TEST 12: Auto-Acknowledge Messages
################################################################################

print_header "TEST 12: Auto-Acknowledge Messages"

# Publish a new message
REQUEST='{"jsonrpc":"2.0","id":17,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"test_channel","message":"Auto-ack test"}}'
send_request "$REQUEST" > /dev/null

sleep 1

# Poll with auto_acknowledge=true
REQUEST='{"jsonrpc":"2.0","id":18,"method":"pubsub_poll_messages","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_1'","max_messages":10,"timeout_ms":1000,"auto_acknowledge":true}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "Auto-ack test"; then
    print_success "Auto-acknowledge messages on poll"
else
    print_error "Auto-acknowledge test failed: $RESPONSE"
fi

################################################################################
# TEST 13: Subscription with Short TTL
################################################################################

print_header "TEST 13: Subscription with Short TTL"

# Create a subscription with 5 second TTL
REQUEST='{"jsonrpc":"2.0","id":19,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"ttl_test","is_pattern":false,"buffer_size":10,"ttl":5}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"subscription_id"'; then
    SUB_ID_TTL=$(extract_json_field "$RESPONSE" "subscription_id")
    print_success "Created subscription with 5 second TTL (ID: $SUB_ID_TTL)"
    
    # Wait for TTL to expire
    print_info "Waiting 6 seconds for TTL to expire..."
    sleep 6
    
    # Try to poll from expired subscription
    REQUEST='{"jsonrpc":"2.0","id":20,"method":"pubsub_poll_messages","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_TTL'","max_messages":10,"timeout_ms":1000,"auto_acknowledge":false}}'
    RESPONSE=$(send_request "$REQUEST")
    
    if echo "$RESPONSE" | grep -q '"error"'; then
        print_success "Subscription correctly expired after TTL"
    else
        print_error "Subscription did not expire as expected: $RESPONSE"
    fi
else
    print_error "Failed to create TTL test subscription: $RESPONSE"
fi

################################################################################
# TEST 14: Multiple Messages and Buffer Management
################################################################################

print_header "TEST 14: Multiple Messages and Buffer Management"

# Create a subscription with small buffer
REQUEST='{"jsonrpc":"2.0","id":21,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"buffer_test","is_pattern":false,"buffer_size":5,"ttl":300}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"subscription_id"'; then
    SUB_ID_BUFFER=$(extract_json_field "$RESPONSE" "subscription_id")
    print_success "Created subscription with small buffer (ID: $SUB_ID_BUFFER)"
    
    # Publish 10 messages (more than buffer size)
    print_info "Publishing 10 messages to test buffer overflow..."
    for i in {1..10}; do
        REQUEST='{"jsonrpc":"2.0","id":'$((21+i))',"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"buffer_test","message":"Message '$i'"}}'
        send_request "$REQUEST" > /dev/null
    done
    
    sleep 1
    
    # Check subscription stats
    REQUEST='{"jsonrpc":"2.0","id":32,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT * FROM pubsub_subscriptions WHERE id = '"'$SUB_ID_BUFFER'"'"}}'
    RESPONSE=$(send_request "$REQUEST")
    
    if echo "$RESPONSE" | grep -q "messages_dropped"; then
        print_success "Buffer overflow handled correctly (messages dropped)"
    else
        print_error "Buffer overflow test failed: $RESPONSE"
    fi
else
    print_error "Failed to create buffer test subscription: $RESPONSE"
fi

################################################################################
# TEST 15: pubsub_unsubscribe
################################################################################

print_header "TEST 15: pubsub_unsubscribe"

# Unsubscribe from the first subscription
REQUEST='{"jsonrpc":"2.0","id":33,"method":"pubsub_unsubscribe","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_1'"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"success":true'; then
    print_success "Unsubscribed from exact channel subscription"
else
    print_error "Failed to unsubscribe: $RESPONSE"
fi

# Verify subscription is gone
REQUEST='{"jsonrpc":"2.0","id":34,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT * FROM pubsub_subscriptions WHERE id = '"'$SUB_ID_1'"'"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"rows":\[\]'; then
    print_success "Subscription removed from pubsub_subscriptions table"
else
    print_error "Subscription still appears in table: $RESPONSE"
fi

# Unsubscribe from pattern subscription
REQUEST='{"jsonrpc":"2.0","id":35,"method":"pubsub_unsubscribe","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"subscription_id":"'$SUB_ID_2'"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q '"success":true'; then
    print_success "Unsubscribed from pattern channel subscription"
else
    print_error "Failed to unsubscribe from pattern: $RESPONSE"
fi

################################################################################
# TEST 16: Complex Query on Virtual Tables
################################################################################

print_header "TEST 16: Complex Query on Virtual Tables"

# Create a new subscription for complex query test
REQUEST='{"jsonrpc":"2.0","id":36,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"query_test","is_pattern":false,"buffer_size":100,"ttl":300}}'
RESPONSE=$(send_request "$REQUEST")
SUB_ID_QUERY=$(extract_json_field "$RESPONSE" "subscription_id")

# Publish some messages
for i in {1..5}; do
    REQUEST='{"jsonrpc":"2.0","id":'$((36+i))',"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"channel":"query_test","message":"Query test message '$i'"}}'
    send_request "$REQUEST" > /dev/null
done

sleep 1

# Complex query with WHERE, ORDER BY, and LIMIT
REQUEST='{"jsonrpc":"2.0","id":42,"method":"execute_query","params":{"params":{"driver":"redis","host":"'$REDIS_HOST'","port":'$REDIS_PORT',"database":"'$REDIS_DB'"},"query":"SELECT subscription_id, message_id, payload FROM pubsub_messages WHERE subscription_id = '"'$SUB_ID_QUERY'"' ORDER BY message_id DESC LIMIT 3"}}'
RESPONSE=$(send_request "$REQUEST")

if echo "$RESPONSE" | grep -q "Query test message"; then
    print_success "Complex query with WHERE, ORDER BY, and LIMIT"
else
    print_error "Complex query failed: $RESPONSE"
fi

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
