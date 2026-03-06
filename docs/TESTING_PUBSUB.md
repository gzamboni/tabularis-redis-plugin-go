# Redis Pub/Sub Testing Guide

This document describes the comprehensive test scripts for verifying Redis Pub/Sub functionality in the Tabularis Redis Plugin.

## Overview

We provide two test scripts for comprehensive end-to-end testing of Redis Pub/Sub features:

1. **`../tests/test_pubsub_e2e.sh`** - Uses Docker to start a Redis instance (recommended for CI/CD)
2. **`../tests/test_pubsub_local.sh`** - Uses a locally running Redis instance (recommended for development)

Both scripts test the same functionality and provide identical coverage.

## What Gets Tested

### JSON-RPC Methods
- ✅ `pubsub_subscribe` - Subscribe to exact and pattern-based channels
- ✅ `pubsub_unsubscribe` - Unsubscribe from channels
- ✅ `pubsub_publish` - Publish messages to channels
- ✅ `pubsub_poll_messages` - Poll messages from subscriptions
- ✅ `pubsub_acknowledge_messages` - Acknowledge received messages

### Virtual Tables
- ✅ `pubsub_channels` - Query active Redis channels
- ✅ `pubsub_messages` - Query buffered messages
- ✅ `pubsub_subscriptions` - Query active subscriptions

### Advanced Features
- ✅ Pattern-based subscriptions (e.g., `user:*`)
- ✅ Message acknowledgment (manual and automatic)
- ✅ Subscription TTL (time-to-live)
- ✅ Buffer overflow handling
- ✅ Complex SQL queries (WHERE, ORDER BY, LIMIT)

## Test Script 1: Docker-Based Testing

### Prerequisites
- Docker installed and running
- Go 1.19+ installed

### Usage

```bash
# Make executable (first time only)
chmod +x ../tests/test_pubsub_e2e.sh

# Run tests
../tests/test_pubsub_e2e.sh
```

### What It Does

1. Builds the Tabularis Redis plugin
2. Starts a Redis container on port 6381
3. Runs 16 comprehensive tests
4. Cleans up the container automatically (even on failure)

### Advantages
- ✅ No local Redis installation required
- ✅ Isolated test environment
- ✅ Perfect for CI/CD pipelines
- ✅ Automatic cleanup

### Example Output

```
═══════════════════════════════════════════════════════════
  SETUP: Building Plugin and Starting Redis
═══════════════════════════════════════════════════════════

ℹ Building plugin...
✅ Plugin built successfully
ℹ Starting Redis via Docker on port 6381...
ℹ Waiting for Redis to start...
✅ Redis is running (Container ID: a1b2c3d4e5f6)

═══════════════════════════════════════════════════════════
  TEST 1: Connection Test
═══════════════════════════════════════════════════════════

✅ Connection test passed

...

═══════════════════════════════════════════════════════════
  TEST SUMMARY
═══════════════════════════════════════════════════════════

Total Tests: 16
Passed: 16
Failed: 0

🎉 All tests passed successfully!
```

## Test Script 2: Local Redis Testing

### Prerequisites
- Redis server running locally
- Go 1.19+ installed

### Usage

```bash
# Start Redis (if not already running)
redis-server

# In another terminal, make executable (first time only)
chmod +x ../tests/test_pubsub_local.sh

# Run tests with default settings (localhost:6379)
../tests/test_pubsub_local.sh

# Or specify custom Redis connection
REDIS_HOST=myhost REDIS_PORT=6380 ../tests/test_pubsub_local.sh
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `localhost` | Redis server hostname |
| `REDIS_PORT` | `6379` | Redis server port |
| `REDIS_DB` | `0` | Redis database number |

### Advantages
- ✅ Faster execution (no Docker overhead)
- ✅ Better for development/debugging
- ✅ Can use existing Redis instance
- ✅ Supports custom Redis configurations

## Test Coverage Details

### Test 1: Connection Test
Verifies basic connectivity to Redis server.

### Test 2-3: Subscriptions
- Creates exact channel subscription (`test_channel`)
- Creates pattern-based subscription (`user:*`)

### Test 4: pubsub_subscriptions Virtual Table
Queries the virtual table to verify both subscriptions are tracked.

### Test 5-6: Publishing Messages
- Publishes to exact channel
- Publishes to pattern-matched channels (`user:123`, `user:456`)

### Test 7-8: Polling Messages
- Polls messages from exact channel subscription
- Polls messages from pattern subscription
- Extracts message IDs for acknowledgment

### Test 9: pubsub_messages Virtual Table
- Queries messages by subscription ID
- Tests ORDER BY clause

### Test 10: Message Acknowledgment
- Acknowledges messages from both subscriptions
- Verifies acknowledgment success

### Test 11: pubsub_channels Virtual Table
- Queries active channels
- Tests WHERE clause filtering

### Test 12: Auto-Acknowledgment
- Tests automatic message acknowledgment on poll

### Test 13: Subscription TTL
- Creates subscription with 5-second TTL
- Waits for expiration
- Verifies subscription is removed

### Test 14: Buffer Management
- Creates subscription with small buffer (5 messages)
- Publishes 10 messages (overflow)
- Verifies buffer overflow handling

### Test 15: Unsubscribe
- Unsubscribes from active subscriptions
- Verifies subscriptions are removed

### Test 16: Complex Queries
- Tests complex SQL with WHERE, ORDER BY, and LIMIT
- Verifies column filtering

## Interpreting Results

### Success
```
✅ Test description
```
Test passed successfully.

### Failure
```
❌ Test description: error details
```
Test failed. Review the error details for debugging.

### Warning
```
⚠ Warning message
```
Non-critical issue or skipped test.

## Troubleshooting

### Docker Script Issues

**Problem:** `docker: command not found`
```
Solution: Install Docker or use ../tests/test_pubsub_local.sh instead
```

**Problem:** `Cannot connect to the Docker daemon`
```
Solution: Start Docker Desktop or Docker daemon
```

**Problem:** `Port 6381 already in use`
```
Solution: Stop the conflicting service or modify REDIS_PORT in the script
```

### Local Script Issues

**Problem:** `Redis is not accessible`
```
Solution: Start Redis with: redis-server
```

**Problem:** `Connection refused`
```
Solution: Check Redis is running on the correct port:
  redis-cli -p 6379 ping
```

**Problem:** `Plugin build failed`
```
Solution: Ensure Go 1.19+ is installed:
  go version
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Test Pub/Sub

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.19'
      
      - name: Run Pub/Sub E2E Tests
        run: ./tests/test_pubsub_e2e.sh
```

### GitLab CI Example

```yaml
test-pubsub:
  image: golang:1.19
  services:
    - docker:dind
  script:
    - ./tests/test_pubsub_e2e.sh
```

## Manual Testing

For manual testing of individual features, you can use the plugin directly:

```bash
# Build plugin
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go

# Subscribe to a channel
echo '{"jsonrpc":"2.0","id":1,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"test","is_pattern":false,"buffer_size":100,"ttl":300}}' | ./tabularis-redis-plugin-go

# Publish a message
echo '{"jsonrpc":"2.0","id":2,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"test","message":"Hello!"}}' | ./tabularis-redis-plugin-go

# Poll messages (use subscription_id from subscribe response)
echo '{"jsonrpc":"2.0","id":3,"method":"pubsub_poll_messages","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"subscription_id":"sub_xxx","max_messages":10,"timeout_ms":1000,"auto_acknowledge":false}}' | ./tabularis-redis-plugin-go

# Query virtual tables
echo '{"jsonrpc":"2.0","id":4,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions"}}' | ./tabularis-redis-plugin-go
```

## Performance Considerations

- **Buffer Size:** Larger buffers consume more memory but reduce message loss
- **TTL:** Shorter TTLs reduce memory usage but require more frequent re-subscription
- **Polling Frequency:** Balance between latency and CPU usage
- **Acknowledgment:** Manual acknowledgment provides better reliability but requires more code

## Best Practices

1. **Always clean up subscriptions** when done to free resources
2. **Monitor buffer usage** via `pubsub_subscriptions` table
3. **Use pattern subscriptions** sparingly (they're more expensive)
4. **Set appropriate TTLs** based on your use case
5. **Handle message loss** gracefully (buffer overflow scenarios)

## Related Documentation

- [`PUBSUB.md`](PUBSUB.md) - User guide for Pub/Sub features
- [`PUBSUB_VIRTUAL_TABLES.md`](PUBSUB_VIRTUAL_TABLES.md) - Virtual tables reference
- [`redis_pubsub_design.md`](redis_pubsub_design.md) - Design document
- [`AGENTS.md`](../AGENTS.md) - AI agent guidance

## Contributing

When adding new Pub/Sub features:

1. Add corresponding tests to both test scripts
2. Update this documentation
3. Ensure all tests pass before submitting PR
4. Add examples to the manual testing section

## License

MIT License - See [`LICENSE`](../LICENSE) for details
