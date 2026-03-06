# Redis Pub/Sub Virtual Tables - Phase 2 Implementation

This document describes the implementation of three virtual tables for Redis Pub/Sub operations in the Tabularis Redis plugin.

## Overview

Phase 2 builds on the Pub/Sub infrastructure from Phase 1 to provide SQL-like query access to Redis Pub/Sub data through three virtual tables:

1. **pubsub_channels** - Information about available Redis channels
2. **pubsub_messages** - Messages from active subscriptions
3. **pubsub_subscriptions** - Active subscription information

## Virtual Tables

### 1. pubsub_channels

Represents available Redis Pub/Sub channels with subscriber information.

**Schema:**
| Column | Type | Description |
|--------|------|-------------|
| channel | STRING (PK) | Channel name |
| subscribers | INTEGER | Number of active subscribers |
| is_pattern | BOOLEAN | Whether this is a pattern channel |
| last_message_time | INTEGER | Unix timestamp of last message (0 = unknown) |

**Data Source:**
- `PUBSUB CHANNELS` - Lists all active channels
- `PUBSUB NUMSUB` - Gets subscriber count per channel

**Example Queries:**
```sql
-- List all active channels
SELECT * FROM pubsub_channels;

-- Find channels with subscribers
SELECT * FROM pubsub_channels WHERE subscribers > 0;

-- Sort by subscriber count
SELECT * FROM pubsub_channels ORDER BY subscribers DESC;

-- Find specific channel
SELECT * FROM pubsub_channels WHERE channel = 'notifications';
```

**Implementation:** [`executeScanPubSubChannels()`](internal/plugin/executor.go:755)

### 2. pubsub_messages

Provides access to messages from active subscriptions stored in message buffers.

**Schema:**
| Column | Type | Description |
|--------|------|-------------|
| subscription_id | STRING (PK) | Subscription identifier |
| message_id | INTEGER (PK) | Unique message ID within subscription |
| channel | STRING | Channel the message was received on |
| payload | STRING | Message content |
| published_at | INTEGER | Unix timestamp when published |
| received_at | INTEGER | Unix timestamp when received |

**Data Source:**
- MessageBuffer of active subscriptions (from Phase 1)
- Messages are retrieved without auto-acknowledgment to allow repeated queries

**Example Queries:**
```sql
-- List all messages from all subscriptions
SELECT * FROM pubsub_messages;

-- Get messages from a specific subscription
SELECT * FROM pubsub_messages WHERE subscription_id = 'sub_abc123';

-- Find messages on a specific channel
SELECT * FROM pubsub_messages WHERE channel = 'notifications';

-- Get recent messages
SELECT * FROM pubsub_messages ORDER BY received_at DESC LIMIT 10;

-- Filter by payload content
SELECT * FROM pubsub_messages WHERE payload LIKE '%error%';

-- Pagination
SELECT * FROM pubsub_messages LIMIT 50 OFFSET 100;
```

**Implementation:** [`executeScanPubSubMessages()`](internal/plugin/executor.go:831)

### 3. pubsub_subscriptions

Shows information about active Pub/Sub subscriptions managed by the plugin.

**Schema:**
| Column | Type | Description |
|--------|------|-------------|
| id | STRING (PK) | Unique subscription identifier |
| channel | STRING | Subscribed channel or pattern |
| is_pattern | BOOLEAN | Whether this is a pattern subscription |
| created_at | INTEGER | Unix timestamp when subscription was created |
| ttl | INTEGER | Time-to-live in seconds (time until expiration) |
| buffer_size | INTEGER | Maximum buffer capacity |
| buffer_used | INTEGER | Current number of unacknowledged messages |
| messages_received | INTEGER | Total messages received |
| messages_dropped | INTEGER | Estimated messages dropped due to buffer overflow |

**Data Source:**
- SubscriptionManager's active subscriptions (from Phase 1)
- Automatically cleans up expired subscriptions before querying

**Example Queries:**
```sql
-- List all active subscriptions
SELECT * FROM pubsub_subscriptions;

-- Find subscriptions about to expire
SELECT * FROM pubsub_subscriptions WHERE ttl < 300 ORDER BY ttl ASC;

-- Check buffer usage
SELECT id, channel, buffer_used, buffer_size 
FROM pubsub_subscriptions 
WHERE buffer_used > buffer_size * 0.8;

-- Find subscriptions with dropped messages
SELECT * FROM pubsub_subscriptions WHERE messages_dropped > 0;

-- Pattern subscriptions only
SELECT * FROM pubsub_subscriptions WHERE is_pattern = true;

-- Sort by activity
SELECT * FROM pubsub_subscriptions ORDER BY messages_received DESC;
```

**Implementation:** [`executeScanPubSubSubscriptions()`](internal/plugin/executor.go:903)

## Features

All three virtual tables support:

### ✅ Filtering (WHERE clauses)
- Equality: `WHERE channel = 'notifications'`
- Inequality: `WHERE subscribers != 0`
- Comparison: `WHERE ttl > 300`, `WHERE buffer_used < 100`
- Pattern matching: `WHERE channel LIKE 'user:%'`
- IN operator: `WHERE channel IN ('ch1', 'ch2')`
- Multiple conditions: `WHERE is_pattern = false AND ttl > 600`

### ✅ Sorting (ORDER BY)
- Single column: `ORDER BY created_at DESC`
- Multiple columns: `ORDER BY channel ASC, messages_received DESC`
- Numeric and string sorting supported

### ✅ Pagination
- LIMIT: `LIMIT 10`
- OFFSET: `OFFSET 20`
- Combined: `LIMIT 10 OFFSET 20`
- Page-based pagination via API parameters

### ✅ Error Handling
- Connection failures return descriptive errors
- Invalid queries return error messages
- Missing subscriptions handled gracefully
- Redis command failures caught and reported

## Integration Points

### Modified Files

1. **[`internal/plugin/executor.go`](internal/plugin/executor.go)**
   - Added `executeScanPubSubChannels()` (lines 755-829)
   - Added `executeScanPubSubMessages()` (lines 831-901)
   - Added `executeScanPubSubSubscriptions()` (lines 903-980)

2. **[`internal/plugin/plugin.go`](internal/plugin/plugin.go)**
   - Updated `getTableColumns()` to include schemas for new tables (lines 85-87)
   - Updated `get_tables` handler to include new tables (line 112)
   - Updated `get_schema_snapshot` to include new tables (line 121)
   - Updated `execute_query` handler to route to new executors (lines 161-163)

3. **[`internal/plugin/plugin_test.go`](internal/plugin/plugin_test.go)**
   - Updated test expectations from 5 to 8 tables (lines 129, 182, 187)

### Dependencies on Phase 1

The virtual tables rely on Phase 1 infrastructure:

- **`globalSubscriptionManager`** - Manages active subscriptions
- **`Subscription`** - Subscription data structure with buffer
- **`MessageBuffer`** - Stores messages with acknowledgment tracking
- **`PubSubMessage`** - Message data structure

## Usage Examples

### Monitoring Active Subscriptions

```sql
-- Dashboard query: Show subscription health
SELECT 
    id,
    channel,
    ttl,
    buffer_used,
    buffer_size,
    messages_received,
    messages_dropped
FROM pubsub_subscriptions
ORDER BY messages_dropped DESC, buffer_used DESC;
```

### Finding Messages

```sql
-- Get recent error messages from all subscriptions
SELECT 
    subscription_id,
    channel,
    payload,
    received_at
FROM pubsub_messages
WHERE payload LIKE '%error%'
ORDER BY received_at DESC
LIMIT 20;
```

### Channel Discovery

```sql
-- Find active channels with subscribers
SELECT 
    channel,
    subscribers
FROM pubsub_channels
WHERE subscribers > 0
ORDER BY subscribers DESC;
```

## Testing

### Unit Tests
All existing tests pass with the new tables:
```bash
go test -v ./internal/plugin/...
```

### Manual Testing
Use the provided test script:
```bash
../tests/test_pubsub_tables.sh
```

Or test individual queries:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"SELECT * FROM pubsub_subscriptions"}}' | ./tabularis-redis-plugin-go
```

## Performance Considerations

1. **pubsub_channels**: Makes Redis PUBSUB commands for each query
   - `PUBSUB CHANNELS *` - Lists all channels
   - `PUBSUB NUMSUB <channel>` - Called once per channel
   - Performance scales with number of active channels

2. **pubsub_messages**: Reads from in-memory buffers
   - Very fast, no Redis calls
   - Limited by buffer size (default 1000 messages per subscription)
   - Messages are not auto-acknowledged, allowing repeated queries

3. **pubsub_subscriptions**: Reads from in-memory subscription manager
   - Very fast, no Redis calls
   - Automatically cleans up expired subscriptions on each query

## Limitations

1. **pubsub_channels**:
   - `last_message_time` always returns 0 (Redis doesn't track this)
   - `is_pattern` always returns false (pattern channels tracked separately)
   - Requires active Redis connection

2. **pubsub_messages**:
   - Only shows messages in active subscription buffers
   - Historical messages before subscription are not available
   - Buffer size limits total messages retained

3. **pubsub_subscriptions**:
   - Only shows subscriptions created via this plugin instance
   - Subscriptions from other clients are not visible
   - `messages_dropped` is an estimate based on buffer overflow

## Future Enhancements

Potential improvements for future phases:

1. Track `last_message_time` for channels in subscription metadata
2. Support for pattern channel detection
3. Message acknowledgment via UPDATE queries
4. Subscription creation via INSERT queries
5. Subscription deletion via DELETE queries
6. Real-time message streaming support
7. Message filtering at buffer level for better performance
8. Persistent message storage option

## Conclusion

Phase 2 successfully implements three virtual tables that provide SQL-like access to Redis Pub/Sub data. The implementation:

- ✅ Integrates seamlessly with Phase 1 infrastructure
- ✅ Supports comprehensive filtering, sorting, and pagination
- ✅ Handles errors gracefully
- ✅ Passes all existing tests
- ✅ Follows the established virtual table pattern
- ✅ Is well-documented with inline comments

Users can now query Pub/Sub channels, messages, and subscriptions using familiar SQL syntax through the Tabularis interface.
