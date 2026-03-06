#!/bin/bash

################################################################################
# Comprehensive Redis Pub/Sub Test
# Tests all Pub/Sub functionality in a single plugin process
################################################################################

set -e

echo "=== Comprehensive Redis Pub/Sub Test ==="
echo ""

# Build plugin
echo "Building plugin..."
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go
echo "✅ Plugin built"
echo ""

# Run comprehensive test
echo "Running tests..."
echo ""

# Send all requests to single plugin instance and capture output
OUTPUT=$(cat << 'EOF' | ./tabularis-redis-plugin-go 2>&1
{"jsonrpc":"2.0","id":1,"method":"test_connection","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"}}}
{"jsonrpc":"2.0","id":2,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"test_channel","is_pattern":false,"buffer_size":100,"ttl":300}}
{"jsonrpc":"2.0","id":3,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"user:*","is_pattern":true,"buffer_size":100,"ttl":300}}
{"jsonrpc":"2.0","id":4,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions"}}
{"jsonrpc":"2.0","id":5,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"test_channel","message":"Hello from test!"}}
{"jsonrpc":"2.0","id":6,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"user:123","message":"User 123 logged in"}}
{"jsonrpc":"2.0","id":7,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"user:456","message":"User 456 logged in"}}
{"jsonrpc":"2.0","id":8,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_channels"}}
{"jsonrpc":"2.0","id":9,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions WHERE is_pattern = true"}}
EOF
)

# Parse results
PASSED=0
FAILED=0

# Test 1: Connection
if echo "$OUTPUT" | grep -q '"id":1.*"success":true'; then
    echo "✅ TEST 1: Connection successful"
    ((PASSED++))
else
    echo "❌ TEST 1: Connection failed"
    ((FAILED++))
fi

# Test 2: Subscribe to exact channel
if echo "$OUTPUT" | grep -q '"id":2.*"subscription_id"'; then
    echo "✅ TEST 2: Subscribed to exact channel"
    ((PASSED++))
    SUB_ID_1=$(echo "$OUTPUT" | grep '"id":2' | grep -o '"subscription_id":"[^"]*"' | cut -d'"' -f4)
    echo "   Subscription ID: $SUB_ID_1"
else
    echo "❌ TEST 2: Failed to subscribe to exact channel"
    ((FAILED++))
fi

# Test 3: Subscribe to pattern channel
if echo "$OUTPUT" | grep -q '"id":3.*"subscription_id"'; then
    echo "✅ TEST 3: Subscribed to pattern channel"
    ((PASSED++))
    SUB_ID_2=$(echo "$OUTPUT" | grep '"id":3' | grep -o '"subscription_id":"[^"]*"' | cut -d'"' -f4)
    echo "   Subscription ID: $SUB_ID_2"
else
    echo "❌ TEST 3: Failed to subscribe to pattern channel"
    ((FAILED++))
fi

# Test 4: Query subscriptions table
if echo "$OUTPUT" | grep -q '"id":4.*"total_rows":2'; then
    echo "✅ TEST 4: pubsub_subscriptions table shows 2 subscriptions"
    ((PASSED++))
else
    echo "❌ TEST 4: pubsub_subscriptions table query failed"
    ((FAILED++))
fi

# Test 5: Publish to exact channel
if echo "$OUTPUT" | grep -q '"id":5.*"success":true'; then
    echo "✅ TEST 5: Published message to exact channel"
    ((PASSED++))
else
    echo "❌ TEST 5: Failed to publish to exact channel"
    ((FAILED++))
fi

# Test 6: Publish to pattern channel (user:123)
if echo "$OUTPUT" | grep -q '"id":6.*"success":true'; then
    echo "✅ TEST 6: Published message to user:123"
    ((PASSED++))
else
    echo "❌ TEST 6: Failed to publish to user:123"
    ((FAILED++))
fi

# Test 7: Publish to pattern channel (user:456)
if echo "$OUTPUT" | grep -q '"id":7.*"success":true'; then
    echo "✅ TEST 7: Published message to user:456"
    ((PASSED++))
else
    echo "❌ TEST 7: Failed to publish to user:456"
    ((FAILED++))
fi

# Test 8: Query channels table
if echo "$OUTPUT" | grep -q '"id":8.*"channel"'; then
    echo "✅ TEST 8: pubsub_channels table query successful"
    ((PASSED++))
else
    echo "❌ TEST 8: pubsub_channels table query failed"
    ((FAILED++))
fi

# Test 9: Query with WHERE clause
if echo "$OUTPUT" | grep -q '"id":9.*"total_rows":1'; then
    echo "✅ TEST 9: Query with WHERE clause (is_pattern = true) returned 1 row"
    ((PASSED++))
else
    echo "❌ TEST 9: Query with WHERE clause failed"
    ((FAILED++))
fi

echo ""
echo "=== Test Summary ==="
echo "Total: $((PASSED + FAILED))"
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo "🎉 All tests passed!"
    exit 0
else
    echo "⚠️  Some tests failed"
    exit 1
fi
