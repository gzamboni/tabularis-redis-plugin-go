# Redis Pub/Sub Test Script Demonstration

This document demonstrates the comprehensive test scripts created for Redis Pub/Sub functionality.

## Test Scripts Created

### 1. `../tests/test_pubsub_e2e.sh` - Docker-Based Testing
A comprehensive end-to-end test script that:
- Automatically starts a Redis container using Docker
- Builds the plugin
- Runs 16 comprehensive tests
- Automatically cleans up resources

**Usage:**
```bash
../tests/test_pubsub_e2e.sh
```

### 2. `../tests/test_pubsub_local.sh` - Local Redis Testing
An identical test suite that works with a locally running Redis instance:
- No Docker required
- Configurable via environment variables
- Perfect for development

**Usage:**
```bash
# Default (localhost:6379)
../tests/test_pubsub_local.sh

# Custom Redis instance
REDIS_HOST=myhost REDIS_PORT=6380 ../tests/test_pubsub_local.sh
```

## Expected Test Output

When run successfully, the scripts produce output like this:

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

═══════════════════════════════════════════════════════════
  TEST 2: pubsub_subscribe (Exact Channel)
═══════════════════════════════════════════════════════════

✅ Subscribed to exact channel (ID: sub_1234567890)

═══════════════════════════════════════════════════════════
  TEST 3: pubsub_subscribe (Pattern Channel)
═══════════════════════════════════════════════════════════

✅ Subscribed to pattern channel (ID: sub_0987654321)

═══════════════════════════════════════════════════════════
  TEST 4: pubsub_subscriptions Virtual Table
═══════════════════════════════════════════════════════════

✅ pubsub_subscriptions table shows both subscriptions

═══════════════════════════════════════════════════════════
  TEST 5: pubsub_publish (To Exact Channel)
═══════════════════════════════════════════════════════════

✅ Published message to exact channel

═══════════════════════════════════════════════════════════
  TEST 6: pubsub_publish (To Pattern-Matched Channel)
═══════════════════════════════════════════════════════════

✅ Published message to pattern-matched channel

═══════════════════════════════════════════════════════════
  TEST 7: pubsub_poll_messages (From Exact Channel)
═══════════════════════════════════════════════════════════

✅ Polled messages from exact channel subscription

═══════════════════════════════════════════════════════════
  TEST 8: pubsub_poll_messages (From Pattern Channel)
═══════════════════════════════════════════════════════════

✅ Polled messages from pattern channel subscription

═══════════════════════════════════════════════════════════
  TEST 9: pubsub_messages Virtual Table
═══════════════════════════════════════════════════════════

✅ pubsub_messages table query for exact channel subscription
✅ pubsub_messages table query for pattern channel subscription

═══════════════════════════════════════════════════════════
  TEST 10: pubsub_acknowledge_messages
═══════════════════════════════════════════════════════════

✅ Acknowledged messages from exact channel subscription
✅ Acknowledged messages from pattern channel subscription

═══════════════════════════════════════════════════════════
  TEST 11: pubsub_channels Virtual Table
═══════════════════════════════════════════════════════════

✅ pubsub_channels table shows active channels
✅ pubsub_channels table query with WHERE clause

═══════════════════════════════════════════════════════════
  TEST 12: Auto-Acknowledge Messages
═══════════════════════════════════════════════════════════

✅ Auto-acknowledge messages on poll

═══════════════════════════════════════════════════════════
  TEST 13: Subscription with Short TTL
═══════════════════════════════════════════════════════════

✅ Created subscription with 5 second TTL (ID: sub_ttl_test)
ℹ Waiting 6 seconds for TTL to expire...
✅ Subscription correctly expired after TTL

═══════════════════════════════════════════════════════════
  TEST 14: Multiple Messages and Buffer Management
═══════════════════════════════════════════════════════════

✅ Created subscription with small buffer (ID: sub_buffer_test)
ℹ Publishing 10 messages to test buffer overflow...
✅ Buffer overflow handled correctly (messages dropped)

═══════════════════════════════════════════════════════════
  TEST 15: pubsub_unsubscribe
═══════════════════════════════════════════════════════════

✅ Unsubscribed from exact channel subscription
✅ Subscription removed from pubsub_subscriptions table
✅ Unsubscribed from pattern channel subscription

═══════════════════════════════════════════════════════════
  TEST 16: Complex Query on Virtual Tables
═══════════════════════════════════════════════════════════

✅ Complex query with WHERE, ORDER BY, and LIMIT

═══════════════════════════════════════════════════════════
  CLEANUP
═══════════════════════════════════════════════════════════

✅ Cleaned up all test subscriptions

═══════════════════════════════════════════════════════════
  TEST SUMMARY
═══════════════════════════════════════════════════════════

Total Tests: 20
Passed: 20
Failed: 0

🎉 All tests passed successfully!
```

## Test Coverage Summary

| Category | Tests | Description |
|----------|-------|-------------|
| **Connection** | 1 | Basic Redis connectivity |
| **Subscriptions** | 2 | Exact and pattern-based subscriptions |
| **Publishing** | 2 | Publishing to exact and pattern channels |
| **Polling** | 2 | Polling messages from subscriptions |
| **Acknowledgment** | 3 | Manual and auto-acknowledgment |
| **Virtual Tables** | 4 | Querying pubsub_* tables |
| **Advanced Features** | 3 | TTL, buffer management, complex queries |
| **Cleanup** | 3 | Unsubscribe and resource cleanup |
| **Total** | **20** | **Comprehensive coverage** |

## Features Tested

### ✅ JSON-RPC Methods
- `pubsub_subscribe` - Both exact and pattern subscriptions
- `pubsub_unsubscribe` - Clean subscription termination
- `pubsub_publish` - Message publishing
- `pubsub_poll_messages` - Message retrieval with auto-ack option
- `pubsub_acknowledge_messages` - Manual message acknowledgment

### ✅ Virtual Tables
- `pubsub_channels` - Active channel listing
- `pubsub_messages` - Message buffer queries
- `pubsub_subscriptions` - Subscription management

### ✅ Advanced Scenarios
- Pattern-based subscriptions (`user:*`)
- Message acknowledgment (manual and automatic)
- Subscription TTL expiration
- Buffer overflow handling
- Complex SQL queries (WHERE, ORDER BY, LIMIT)
- Resource cleanup

## Running the Tests

### Prerequisites

**For Docker-based tests (`../tests/test_pubsub_e2e.sh`):**
- Docker installed and running
- Go 1.19+

**For local tests (`../tests/test_pubsub_local.sh`):**
- Redis server running
- Go 1.19+

### Execution

```bash
# Make scripts executable
chmod +x ../tests/test_pubsub_e2e.sh ../tests/test_pubsub_local.sh

# Run Docker-based tests
../tests/test_pubsub_e2e.sh

# Or run local tests
../tests/test_pubsub_local.sh
```

## Integration with CI/CD

The `test_pubsub_e2e.sh` script is designed for CI/CD integration:

```yaml
# GitHub Actions example
- name: Run Pub/Sub Tests
  run: ./tests/test_pubsub_e2e.sh
```

The script:
- Returns exit code 0 on success, 1 on failure
- Automatically cleans up Docker containers
- Provides clear, colored output
- Counts passed/failed tests

## Test Script Architecture

Both scripts follow the same structure:

1. **Setup Phase**
   - Build the plugin
   - Start/verify Redis connection
   
2. **Test Execution**
   - 16 comprehensive test scenarios
   - Each test is independent
   - Clear pass/fail indicators
   
3. **Cleanup Phase**
   - Unsubscribe from all subscriptions
   - Stop Docker container (if applicable)
   
4. **Summary**
   - Total test count
   - Pass/fail breakdown
   - Exit with appropriate code

## Error Handling

The scripts include comprehensive error handling:

- **Build failures** - Stop immediately
- **Connection failures** - Clear error messages
- **Test failures** - Continue testing, report at end
- **Cleanup** - Always runs (via trap for Docker script)

## Next Steps

To run these tests in your environment:

1. Ensure prerequisites are met (Docker or Redis + Go)
2. Make scripts executable: `chmod +x test_pubsub_*.sh`
3. Run the appropriate script for your environment
4. Review the output for any failures
5. Check the exit code: `echo $?` (0 = success)

## Documentation

For more details, see:
- [`TESTING_PUBSUB.md`](TESTING_PUBSUB.md) - Complete testing guide
- [`PUBSUB.md`](PUBSUB.md) - Pub/Sub user guide
- [`PUBSUB_VIRTUAL_TABLES.md`](PUBSUB_VIRTUAL_TABLES.md) - Virtual tables reference
