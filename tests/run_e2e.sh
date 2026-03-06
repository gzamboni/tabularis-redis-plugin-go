#!/bin/bash
set -e

echo "Building plugin..."
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go

echo "Starting Redis via Docker..."
# Use port 6380 to avoid conflicts with any local Redis instance
CONTAINER_ID=$(docker run -d -p 6380:6379 redis:alpine)

# Ensure container is destroyed when script exits
trap "echo 'Cleaning up...'; docker rm -f $CONTAINER_ID > /dev/null" EXIT

echo "Waiting for Redis to start..."
sleep 2

echo "Seeding data..."
docker exec $CONTAINER_ID redis-cli set e2e_key "hello" > /dev/null
docker exec $CONTAINER_ID redis-cli hset e2e_hash myfield "myvalue" > /dev/null

echo "Running E2E tests..."

echo "Test 1: test_connection"
RESULT_1=$(echo '{"jsonrpc":"2.0","id":1,"method":"test_connection","params":{"params":{"driver":"redis","host":"localhost","port":6380,"database":"0"}}}' | ./tabularis-redis-plugin-go)
if echo "$RESULT_1" | grep -q '"success":true'; then
    echo "✅ test_connection passed"
else
    echo "❌ test_connection failed: $RESULT_1"
    exit 1
fi

echo "Test 2: execute_query (keys)"
RESULT_2=$(echo '{"jsonrpc":"2.0","id":2,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6380,"database":"0"}, "query":"SELECT * FROM keys", "page":0, "page_size":10}}' | ./tabularis-redis-plugin-go)
if echo "$RESULT_2" | grep -q '"e2e_key"'; then
    echo "✅ execute_query (keys) passed"
else
    echo "❌ execute_query (keys) failed: $RESULT_2"
    exit 1
fi

echo "Test 3: execute_query (hashes)"
RESULT_3=$(echo '{"jsonrpc":"2.0","id":3,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6380,"database":"0"}, "query":"SELECT * FROM hashes WHERE key = '"'e2e_hash'"'", "page":0, "page_size":10}}' | ./tabularis-redis-plugin-go)
if echo "$RESULT_3" | grep -q '"myvalue"'; then
    echo "✅ execute_query (hashes) passed"
else
    echo "❌ execute_query (hashes) failed: $RESULT_3"
    exit 1
fi

echo "Test 4: update_record (value and TTL)"
# Update value and set TTL to 100
echo '{"jsonrpc":"2.0","id":4,"method":"update_record","params":{"params":{"driver":"redis","host":"localhost","port":6380,"database":"0"}, "table":"keys", "pk_col":"key", "pk_val":"e2e_key", "col_name":"value", "new_val":"updated_hello"}}' | ./tabularis-redis-plugin-go > /dev/null
echo '{"jsonrpc":"2.0","id":5,"method":"update_record","params":{"params":{"driver":"redis","host":"localhost","port":6380,"database":"0"}, "table":"keys", "pk_col":"key", "pk_val":"e2e_key", "col_name":"ttl", "new_val":100}}' | ./tabularis-redis-plugin-go > /dev/null

# Verify update
RESULT_4=$(docker exec $CONTAINER_ID redis-cli get e2e_key)
TTL_4=$(docker exec $CONTAINER_ID redis-cli ttl e2e_key)
if [ "$RESULT_4" == "updated_hello" ] && [ "$TTL_4" -gt 0 ]; then
    echo "✅ update_record (value and TTL) passed"
else
    echo "❌ update_record failed: value=$RESULT_4, ttl=$TTL_4"
    exit 1
fi

echo "Test 5: insert_record (hash)"
echo '{"jsonrpc":"2.0","id":6,"method":"insert_record","params":{"params":{"driver":"redis","host":"localhost","port":6380,"database":"0"}, "table":"hashes", "data":{"key":"e2e_new_hash", "field":"new_field", "value":"new_value"}}}' | ./tabularis-redis-plugin-go > /dev/null
RESULT_5=$(docker exec $CONTAINER_ID redis-cli hget e2e_new_hash new_field)
if [ "$RESULT_5" == "new_value" ]; then
    echo "✅ insert_record (hash) passed"
else
    echo "❌ insert_record failed: $RESULT_5"
    exit 1
fi

echo "Test 6: delete_record (key)"
echo '{"jsonrpc":"2.0","id":7,"method":"delete_record","params":{"params":{"driver":"redis","host":"localhost","port":6380,"database":"0"}, "table":"keys", "pk_col":"key", "pk_val":"e2e_key"}}' | ./tabularis-redis-plugin-go > /dev/null
RESULT_6=$(docker exec $CONTAINER_ID redis-cli exists e2e_key)
if [ "$RESULT_6" == "0" ]; then
    echo "✅ delete_record (key) passed"
else
    echo "❌ delete_record failed: $RESULT_6"
    exit 1
fi

echo "🎉 All E2E tests passed successfully!"
