# Quick Start: Testing Redis Pub/Sub

This is a quick reference for running the comprehensive Redis Pub/Sub test suite.

## TL;DR

```bash
# Option 1: Docker-based (recommended for CI/CD)
../tests/test_pubsub_e2e.sh

# Option 2: Local Redis (recommended for development)
../tests/test_pubsub_local.sh
```

## Prerequisites

### For Docker-based testing:
- ✅ Docker installed and running
- ✅ Go 1.19+

### For local testing:
- ✅ Redis server running (`redis-server`)
- ✅ Go 1.19+

## Test Files

| File | Purpose |
|------|---------|
| [`test_pubsub_e2e.sh`](../tests/test_pubsub_e2e.sh) | Docker-based comprehensive tests |
| [`test_pubsub_local.sh`](../tests/test_pubsub_local.sh) | Local Redis comprehensive tests |
| [`TESTING_PUBSUB.md`](TESTING_PUBSUB.md) | Complete testing guide |
| [`TEST_SCRIPT_DEMO.md`](TEST_SCRIPT_DEMO.md) | Expected output and coverage |

## What Gets Tested (20 Tests)

✅ **Connection** - Basic Redis connectivity  
✅ **Exact Subscriptions** - Subscribe to specific channels  
✅ **Pattern Subscriptions** - Subscribe to channel patterns (`user:*`)  
✅ **Publishing** - Publish messages to channels  
✅ **Polling** - Retrieve messages from subscriptions  
✅ **Acknowledgment** - Manual and automatic message acknowledgment  
✅ **Virtual Tables** - Query `pubsub_channels`, `pubsub_messages`, `pubsub_subscriptions`  
✅ **TTL** - Subscription expiration  
✅ **Buffer Management** - Overflow handling  
✅ **Cleanup** - Unsubscribe and resource cleanup  

## Quick Commands

```bash
# Make scripts executable (first time only)
chmod +x ../tests/test_pubsub_e2e.sh ../tests/test_pubsub_local.sh

# Run Docker-based tests
../tests/test_pubsub_e2e.sh

# Run local tests with default Redis (localhost:6379)
../tests/test_pubsub_local.sh

# Run local tests with custom Redis
REDIS_HOST=myhost REDIS_PORT=6380 ../tests/test_pubsub_local.sh

# Check exit code
echo $?  # 0 = all tests passed, 1 = some tests failed
```

## Expected Output

```
═══════════════════════════════════════════════════════════
  TEST SUMMARY
═══════════════════════════════════════════════════════════

Total Tests: 20
Passed: 20
Failed: 0

🎉 All tests passed successfully!
```

## Troubleshooting

### Docker script fails
```bash
# Check Docker is running
docker ps

# Check Docker version
docker --version
```

### Local script fails
```bash
# Check Redis is running
redis-cli ping
# Should return: PONG

# Start Redis if needed
redis-server
```

### Build fails
```bash
# Check Go version
go version
# Should be 1.19 or higher

# Try manual build
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go
```

## CI/CD Integration

```yaml
# GitHub Actions
- name: Test Pub/Sub
  run: ./tests/test_pubsub_e2e.sh
```

## Manual Testing

```bash
# Build plugin
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go

# Subscribe to channel
echo '{"jsonrpc":"2.0","id":1,"method":"pubsub_subscribe","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"test","is_pattern":false,"buffer_size":100,"ttl":300}}' | ./tabularis-redis-plugin-go

# Publish message
echo '{"jsonrpc":"2.0","id":2,"method":"pubsub_publish","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"channel":"test","message":"Hello!"}}' | ./tabularis-redis-plugin-go

# Query subscriptions
echo '{"jsonrpc":"2.0","id":3,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions"}}' | ./tabularis-redis-plugin-go
```

## Documentation

- **Full Testing Guide**: [`TESTING_PUBSUB.md`](TESTING_PUBSUB.md)
- **User Guide**: [`PUBSUB.md`](PUBSUB.md)
- **Virtual Tables**: [`PUBSUB_VIRTUAL_TABLES.md`](PUBSUB_VIRTUAL_TABLES.md)
- **Design Document**: [`redis_pubsub_design.md`](redis_pubsub_design.md)

## Support

For issues or questions:
1. Check [`TESTING_PUBSUB.md`](TESTING_PUBSUB.md) troubleshooting section
2. Review test output for specific error messages
3. Verify prerequisites are met
4. Check Redis connectivity manually

---

**Ready to test?** Run `../tests/test_pubsub_e2e.sh` or `../tests/test_pubsub_local.sh` now!
