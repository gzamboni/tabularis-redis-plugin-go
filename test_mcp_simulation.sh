#!/bin/bash
# Simulate Tabularis MCP requests with SQL comments and column mapping

set -e

echo "=========================================================="
echo "Testing PubSub INSERT Operations (MCP Simulation)"
echo "=========================================================="

# Build the plugin
echo "Building plugin..."
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go

echo ""
echo "Test 1: INSERT with SQL comment (single column)"
echo "Query: -- Create a subscription"
echo "       INSERT INTO pubsub_subscriptions (channel) VALUES ('mcp_test')"
result=$(timeout 2 bash -c 'echo '"'"'{"jsonrpc":"2.0","id":1,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"-- Create a subscription\nINSERT INTO pubsub_subscriptions (channel) VALUES ('"'"'"'"'"'"'"'"'mcp_test'"'"'"'"'"'"'"'"')"}}'"'"' | ./tabularis-redis-plugin-go 2>&1 | grep -v DEBUG' || true)
echo "$result"
if echo "$result" | grep -q '"affected_rows":1'; then
    echo "✅ Test 1 PASSED - SQL comment stripped, INSERT successful"
else
    echo "❌ Test 1 FAILED"
    exit 1
fi

echo ""
echo "Test 2: INSERT with multiline SQL comment and column mapping"
echo "Query: -- Publish a message to a channel"
echo "       -- This is a test message"
echo "       INSERT INTO pubsub_messages (channel, payload)"
echo "       VALUES ('notifications', 'Hello from MCP!')"
result=$(timeout 2 bash -c 'echo '"'"'{"jsonrpc":"2.0","id":2,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"-- Publish a message to a channel\n-- This is a test message\nINSERT INTO pubsub_messages (channel, payload)\nVALUES ('"'"'"'"'"'"'"'"'notifications'"'"'"'"'"'"'"'"', '"'"'"'"'"'"'"'"'Hello from MCP!'"'"'"'"'"'"'"'"')"}}'"'"' | ./tabularis-redis-plugin-go 2>&1 | grep -v DEBUG' || true)
echo "$result"
if echo "$result" | grep -q '"affected_rows":1'; then
    echo "✅ Test 2 PASSED - Multiline comments stripped, INSERT successful"
else
    echo "❌ Test 2 FAILED"
    exit 1
fi

echo ""
echo "Test 3: INSERT with reversed column order and comment"
echo "Query: -- Test reversed columns"
echo "       INSERT INTO pubsub_messages (payload, channel)"
echo "       VALUES ('Reversed!', 'test_channel')"
result=$(timeout 2 bash -c 'echo '"'"'{"jsonrpc":"2.0","id":3,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"-- Test reversed columns\nINSERT INTO pubsub_messages (payload, channel)\nVALUES ('"'"'"'"'"'"'"'"'Reversed!'"'"'"'"'"'"'"'"', '"'"'"'"'"'"'"'"'test_channel'"'"'"'"'"'"'"'"')"}}'"'"' | ./tabularis-redis-plugin-go 2>&1 | grep -v DEBUG' || true)
echo "$result"
if echo "$result" | grep -q '"affected_rows":1'; then
    echo "✅ Test 3 PASSED - Reversed columns with comment works"
else
    echo "❌ Test 3 FAILED"
    exit 1
fi

echo ""
echo "Test 4: INSERT with all subscription columns and comment"
echo "Query: -- Create pattern subscription with custom settings"
echo "       INSERT INTO pubsub_subscriptions (channel, is_pattern, buffer_size, ttl)"
echo "       VALUES ('user:*', 'true', 2000, 7200)"
result=$(timeout 2 bash -c 'echo '"'"'{"jsonrpc":"2.0","id":4,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"-- Create pattern subscription with custom settings\nINSERT INTO pubsub_subscriptions (channel, is_pattern, buffer_size, ttl)\nVALUES ('"'"'"'"'"'"'"'"'user:*'"'"'"'"'"'"'"'"', '"'"'"'"'"'"'"'"'true'"'"'"'"'"'"'"'"', '"'"'"'"'"'"'"'"'2000'"'"'"'"'"'"'"'"', '"'"'"'"'"'"'"'"'7200'"'"'"'"'"'"'"'"')"}}'"'"' | ./tabularis-redis-plugin-go 2>&1 | grep -v DEBUG' || true)
echo "$result"
if echo "$result" | grep -q '"affected_rows":1'; then
    echo "✅ Test 4 PASSED - All columns with comment works"
else
    echo "❌ Test 4 FAILED"
    exit 1
fi

echo ""
echo "Test 5: Verify subscription was created (within same process)"
result=$(timeout 2 bash -c 'echo '"'"'{"jsonrpc":"2.0","id":1,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"INSERT INTO pubsub_subscriptions (channel) VALUES ('"'"'"'"'"'"'"'"'verify_test'"'"'"'"'"'"'"'"')"}}
{"jsonrpc":"2.0","id":2,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions WHERE channel = '"'"'"'"'"'"'"'"'verify_test'"'"'"'"'"'"'"'"'"}}'"'"' | ./tabularis-redis-plugin-go 2>&1 | grep -v DEBUG' || true)
echo "$result"
if echo "$result" | grep -q '"verify_test"'; then
    echo "✅ Test 5 PASSED - Subscription persists and can be queried"
else
    echo "❌ Test 5 FAILED"
    exit 1
fi

echo ""
echo "=========================================================="
echo "✅ ALL MCP SIMULATION TESTS PASSED!"
echo "=========================================================="
echo ""
echo "Summary:"
echo "  ✅ SQL comments are properly stripped"
echo "  ✅ Column name mapping works correctly"
echo "  ✅ Reversed column order is supported"
echo "  ✅ All subscription columns can be specified"
echo "  ✅ Subscriptions persist within same process"
echo ""
echo "The plugin is ready for use with Tabularis!"
