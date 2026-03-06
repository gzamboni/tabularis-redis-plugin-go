#!/bin/bash

# Test script for Redis Pub/Sub virtual tables
# This script demonstrates querying the three new virtual tables

echo "=== Testing Redis Pub/Sub Virtual Tables ==="
echo ""

# Build the plugin
echo "Building plugin..."
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go
echo ""

# Test 1: Query pubsub_subscriptions table (should be empty initially)
echo "Test 1: Query pubsub_subscriptions table"
echo '{"jsonrpc":"2.0","id":1,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions"}}' | ./tabularis-redis-plugin-go
echo ""

# Test 2: Query pubsub_messages table (should be empty initially)
echo "Test 2: Query pubsub_messages table"
echo '{"jsonrpc":"2.0","id":2,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_messages"}}' | ./tabularis-redis-plugin-go
echo ""

# Test 3: Query pubsub_channels table
echo "Test 3: Query pubsub_channels table"
echo '{"jsonrpc":"2.0","id":3,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_channels"}}' | ./tabularis-redis-plugin-go
echo ""

# Test 4: Query pubsub_channels with WHERE clause
echo "Test 4: Query pubsub_channels with WHERE clause"
echo '{"jsonrpc":"2.0","id":4,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_channels WHERE subscribers > 0"}}' | ./tabularis-redis-plugin-go
echo ""

# Test 5: Query pubsub_subscriptions with ORDER BY
echo "Test 5: Query pubsub_subscriptions with ORDER BY"
echo '{"jsonrpc":"2.0","id":5,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions ORDER BY created_at DESC"}}' | ./tabularis-redis-plugin-go
echo ""

# Test 6: Query pubsub_messages with LIMIT
echo "Test 6: Query pubsub_messages with LIMIT"
echo '{"jsonrpc":"2.0","id":6,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_messages LIMIT 10"}}' | ./tabularis-redis-plugin-go
echo ""

echo "=== All tests completed ==="
