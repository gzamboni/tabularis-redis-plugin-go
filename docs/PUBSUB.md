# Redis Pub/Sub Guide for Tabularis Redis Plugin

This guide provides comprehensive documentation for using Redis Pub/Sub (Publish/Subscribe) functionality in the Tabularis Redis plugin.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
- [JSON-RPC Methods](#json-rpc-methods)
- [Virtual Tables](#virtual-tables)
- [Usage Examples](#usage-examples)
- [Best Practices](#best-practices)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)

## Overview

### What is Redis Pub/Sub?

Redis Pub/Sub is a messaging pattern where:
- **Publishers** send messages to channels without knowledge of subscribers
- **Subscribers** receive messages from channels they're interested in
- Messages are delivered in real-time to all active subscribers

### How It Works in Tabularis

The plugin bridges Redis's asynchronous Pub/Sub model with Tabularis's synchronous JSON-RPC protocol using:

1. **Server-Side Subscriptions** — Subscriptions persist on the plugin side
2. **Message Buffering** — Messages are buffered for reliable retrieval
3. **Polling-Based Retrieval** — Clients poll for messages at their own pace
4. **SQL-Like Queries** — Query channels, messages, and subscriptions using familiar SQL syntax

### Key Features

- ✅ Subscribe to specific channels or pattern-based channels (e.g., `user:*`)
- ✅ Publish messages to channels
- ✅ Poll messages from subscriptions with configurable batch sizes
- ✅ Message acknowledgment for at-least-once delivery semantics
- ✅ Automatic subscription expiration with configurable TTL
- ✅ Query channels, messages, and subscriptions using SQL
- ✅ Monitor subscription health and buffer usage

## Architecture

### Components

```
┌─────────────┐
│  Tabularis  │
└──────┬──────┘
       │ JSON-RPC
       ▼
┌─────────────────────────────────────┐
│         Plugin Process              │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  SubscriptionManager         │  │
│  │  - Manages subscriptions     │  │
│  │  - Cleanup expired subs      │  │
│  └──────────────────────────────┘  │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  Subscription (per channel)  │  │
│  │  - Background goroutine      │  │
│  │  - MessageBuffer (1000 msgs) │  │
│  │  - TTL-based expiration      │  │
│  └──────────────────────────────┘  │
└─────────────┬───────────────────────┘
              │ Redis Protocol
              ▼
       ┌─────────────┐
       │    Redis    │
       │   Server    │
       └─────────────┘
```

### Message Flow

1. **Subscribe**: Client creates subscription → Plugin starts background listener
2. **Publish**: Publisher sends message → Redis broadcasts to all subscribers
3. **Receive**: Plugin's background goroutine receives message → Adds to buffer
4. **Poll**: Client polls for messages → Plugin returns buffered messages
5. **Acknowledge**: Client acknowledges messages → Plugin marks as processed

## Getting Started

### Basic Workflow

```
1. Subscribe to a channel
   ↓
2. Publish messages (from any client)
   ↓
3. Poll for messages
   ↓
4. Process messages
   ↓
5. Acknowledge messages
   ↓
6. Unsubscribe when done
```

### Quick Example

```json
// 1. Subscribe
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "pubsub_subscribe",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "notifications",
    "is_pattern": false,
    "buffer_size": 1000,
    "ttl": 3600
  }
}

// 2. Publish
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "pubsub_publish",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "notifications",
    "message": "Hello, World!"
  }
}

// 3. Poll
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "pubsub_poll_messages",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "subscription_id": "sub_abc123def456",
    "max_messages": 100,
    "auto_acknowledge": true
  }
}
```

## JSON-RPC Methods

### pubsub_subscribe

Creates a new subscription to a Redis Pub/Sub channel.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "pubsub_subscribe",
  "params": {
    "params": {
      "driver": "redis",
      "host": "localhost",
      "port": 6379,
      "database": "0"
    },
    "channel": "notifications",
    "is_pattern": false,
    "buffer_size": 1000,
    "ttl": 3600
  }
}
```

**Parameters:**
- `params` (required): Connection parameters
- `channel` (required): Channel name or pattern (e.g., `user:*`)
- `is_pattern` (optional): `true` for pattern subscriptions, default: `false`
- `buffer_size` (optional): Maximum messages to buffer, default: `1000`
- `ttl` (optional): Subscription time-to-live in seconds, default: `3600`

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "subscription_id": "sub_abc123def456",
    "channel": "notifications",
    "is_pattern": false,
    "created_at": 1709673600,
    "buffer_size": 0,
    "max_buffer": 1000,
    "ttl": 3600
  }
}
```

**Response Fields:**
- `subscription_id`: Unique identifier for this subscription (use for polling/unsubscribing)
- `channel`: Subscribed channel or pattern
- `is_pattern`: Whether this is a pattern subscription
- `created_at`: Unix timestamp when subscription was created
- `buffer_size`: Current number of buffered messages (initially 0)
- `max_buffer`: Maximum buffer capacity
- `ttl`: Time-to-live in seconds

---

### pubsub_unsubscribe

Terminates a subscription and releases resources.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "pubsub_unsubscribe",
  "params": {
    "params": {
      "driver": "redis",
      "host": "localhost",
      "port": 6379,
      "database": "0"
    },
    "subscription_id": "sub_abc123def456"
  }
}
```

**Parameters:**
- `params` (required): Connection parameters
- `subscription_id` (required): Subscription ID from `pubsub_subscribe`

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "success": true,
    "subscription_id": "sub_abc123def456",
    "messages_dropped": 5
  }
}
```

**Response Fields:**
- `success`: Whether unsubscribe was successful
- `subscription_id`: The unsubscribed subscription ID
- `messages_dropped`: Number of unacknowledged messages that were dropped

---

### pubsub_publish

Publishes a message to a Redis Pub/Sub channel.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "pubsub_publish",
  "params": {
    "params": {
      "driver": "redis",
      "host": "localhost",
      "port": 6379,
      "database": "0"
    },
    "channel": "notifications",
    "message": "Hello, Redis Pub/Sub!"
  }
}
```

**Parameters:**
- `params` (required): Connection parameters
- `channel` (required): Channel name to publish to
- `message` (required): Message payload (string)

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "success": true,
    "channel": "notifications",
    "receivers": 2
  }
}
```

**Response Fields:**
- `success`: Whether publish was successful
- `channel`: Channel the message was published to
- `receivers`: Number of subscribers that received the message

---

### pubsub_poll_messages

Retrieves messages from a subscription's buffer.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "pubsub_poll_messages",
  "params": {
    "params": {
      "driver": "redis",
      "host": "localhost",
      "port": 6379,
      "database": "0"
    },
    "subscription_id": "sub_abc123def456",
    "max_messages": 100,
    "timeout_ms": 1000,
    "auto_acknowledge": false
  }
}
```

**Parameters:**
- `params` (required): Connection parameters
- `subscription_id` (required): Subscription ID from `pubsub_subscribe`
- `max_messages` (optional): Maximum messages to retrieve, default: `100`
- `timeout_ms` (optional): Timeout in milliseconds (currently unused), default: `1000`
- `auto_acknowledge` (optional): Automatically acknowledge messages, default: `false`

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "messages": [
      {
        "message_id": 1,
        "channel": "notifications",
        "payload": "Hello, Redis Pub/Sub!",
        "published_at": 1709673610,
        "received_at": 1709673610
      },
      {
        "message_id": 2,
        "channel": "notifications",
        "payload": "Another message",
        "published_at": 1709673615,
        "received_at": 1709673615
      }
    ],
    "more_available": false,
    "subscription_id": "sub_abc123def456",
    "buffer_size": 0
  }
}
```

**Response Fields:**
- `messages`: Array of message objects
  - `message_id`: Unique ID within this subscription
  - `channel`: Channel the message was received on
  - `payload`: Message content
  - `published_at`: Unix timestamp when message was published
  - `received_at`: Unix timestamp when plugin received the message
- `more_available`: Whether more unacknowledged messages are available
- `subscription_id`: The subscription ID
- `buffer_size`: Current number of unacknowledged messages in buffer

---

### pubsub_acknowledge_messages

Marks messages as processed, freeing buffer space.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "pubsub_acknowledge_messages",
  "params": {
    "params": {
      "driver": "redis",
      "host": "localhost",
      "port": 6379,
      "database": "0"
    },
    "subscription_id": "sub_abc123def456",
    "message_ids": [1, 2, 3]
  }
}
```

**Parameters:**
- `params` (required): Connection parameters
- `subscription_id` (required): Subscription ID from `pubsub_subscribe`
- `message_ids` (required): Array of message IDs to acknowledge

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "success": true,
    "subscription_id": "sub_abc123def456",
    "messages_acknowledged": 3
  }
}
```

**Response Fields:**
- `success`: Whether acknowledgment was successful
- `subscription_id`: The subscription ID
- `messages_acknowledged`: Number of messages successfully acknowledged

## Virtual Tables

### pubsub_channels

Lists all active Redis Pub/Sub channels with subscriber information.

**Schema:**
| Column | Type | Description |
|--------|------|-------------|
| `channel` | STRING | Channel name |
| `subscribers` | INTEGER | Number of active subscribers |
| `is_pattern` | BOOLEAN | Whether this is a pattern channel (always `false`) |
| `last_message_time` | INTEGER | Unix timestamp of last message (always `0`) |

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

-- Find channels matching a pattern
SELECT * FROM pubsub_channels WHERE channel LIKE 'user:%';
```

**Notes:**
- Uses Redis `PUBSUB CHANNELS` and `PUBSUB NUMSUB` commands
- `is_pattern` always returns `false` (Redis limitation)
- `last_message_time` always returns `0` (not tracked by Redis)

---

### pubsub_messages

Provides access to messages from active subscriptions.

**Schema:**
| Column | Type | Description |
|--------|------|-------------|
| `subscription_id` | STRING | Subscription identifier |
| `message_id` | INTEGER | Unique message ID within subscription |
| `channel` | STRING | Channel the message was received on |
| `payload` | STRING | Message content |
| `published_at` | INTEGER | Unix timestamp when published |
| `received_at` | INTEGER | Unix timestamp when received |

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

-- Get messages from the last hour
SELECT * FROM pubsub_messages 
WHERE received_at > UNIX_TIMESTAMP() - 3600;

-- Pagination
SELECT * FROM pubsub_messages LIMIT 50 OFFSET 100;
```

**Notes:**
- Only shows messages in active subscription buffers
- Messages are not auto-acknowledged when queried
- Limited by buffer size (default: 1000 messages per subscription)

---

### pubsub_subscriptions

Shows information about active Pub/Sub subscriptions.

**Schema:**
| Column | Type | Description |
|--------|------|-------------|
| `id` | STRING | Unique subscription identifier |
| `channel` | STRING | Subscribed channel or pattern |
| `is_pattern` | BOOLEAN | Whether this is a pattern subscription |
| `created_at` | INTEGER | Unix timestamp when subscription was created |
| `ttl` | INTEGER | Time-to-live in seconds (time until expiration) |
| `buffer_size` | INTEGER | Maximum buffer capacity |
| `buffer_used` | INTEGER | Current number of unacknowledged messages |
| `messages_received` | INTEGER | Total messages received |
| `messages_dropped` | INTEGER | Estimated messages dropped due to buffer overflow |

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

-- Monitor subscription health
SELECT 
    id,
    channel,
    ttl,
    ROUND(buffer_used * 100.0 / buffer_size, 2) AS buffer_usage_pct,
    messages_dropped
FROM pubsub_subscriptions
WHERE buffer_used > 0 OR messages_dropped > 0;
```

**Notes:**
- Automatically cleans up expired subscriptions before querying
- `messages_dropped` is an estimate based on buffer overflow
- Only shows subscriptions created through this plugin instance

## Usage Examples

### Example 1: Simple Notification System

```json
// 1. Subscribe to notifications channel
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "pubsub_subscribe",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "notifications",
    "buffer_size": 500,
    "ttl": 1800
  }
}
// Response: {"subscription_id": "sub_xyz789"}

// 2. Publish a notification
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "pubsub_publish",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "notifications",
    "message": "New order received: #12345"
  }
}

// 3. Poll for notifications every 5 seconds
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "pubsub_poll_messages",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "subscription_id": "sub_xyz789",
    "max_messages": 10,
    "auto_acknowledge": true
  }
}
```

### Example 2: Pattern-Based Subscription

```json
// Subscribe to all user-related channels (user:*)
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "pubsub_subscribe",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "user:*",
    "is_pattern": true,
    "buffer_size": 2000,
    "ttl": 7200
  }
}

// Publish to specific user channels
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "pubsub_publish",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "user:1001",
    "message": "Profile updated"
  }
}

{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "pubsub_publish",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "user:1002",
    "message": "New message received"
  }
}

// Poll receives messages from all matching channels
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "pubsub_poll_messages",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "subscription_id": "sub_pattern123",
    "max_messages": 50,
    "auto_acknowledge": false
  }
}
```

### Example 3: Manual Message Acknowledgment

```json
// 1. Subscribe with manual acknowledgment
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "pubsub_subscribe",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "channel": "tasks",
    "buffer_size": 1000,
    "ttl": 3600
  }
}

// 2. Poll without auto-acknowledgment
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "pubsub_poll_messages",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "subscription_id": "sub_tasks456",
    "max_messages": 10,
    "auto_acknowledge": false
  }
}
// Response: {"messages": [{"message_id": 1, ...}, {"message_id": 2, ...}]}

// 3. Process messages in your application
// ... (application logic) ...

// 4. Acknowledge after successful processing
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "pubsub_acknowledge_messages",
  "params": {
    "params": {"driver": "redis", "host": "localhost", "port": 6379, "database": "0"},
    "subscription_id": "sub_tasks456",
    "message_ids": [1, 2]
  }
}
```

### Example 4: Monitoring with SQL

```sql
-- Dashboard query: Subscription health overview
SELECT 
    id,
    channel,
    CASE 
        WHEN ttl < 300 THEN 'EXPIRING SOON'
        WHEN buffer_used > buffer_size * 0.8 THEN 'BUFFER FULL'
        WHEN messages_dropped > 0 THEN 'DROPPING MESSAGES'
        ELSE 'HEALTHY'
    END AS status,
    ttl,
    buffer_used,
    buffer_size,
    messages_received,
    messages_dropped
FROM pubsub_subscriptions
ORDER BY 
    CASE 
        WHEN messages_dropped > 0 THEN 1
        WHEN buffer_used > buffer_size * 0.8 THEN 2
        WHEN ttl < 300 THEN 3
        ELSE 4
    END;

-- Find error messages across all subscriptions
SELECT 
    subscription_id,
    channel,
    payload,
    received_at
FROM pubsub_messages
WHERE payload LIKE '%error%' OR payload LIKE '%ERROR%'
ORDER BY received_at DESC
LIMIT 50;

-- Channel activity report
SELECT 
    channel,
    subscribers,
    CASE 
        WHEN subscribers = 0 THEN 'INACTIVE'
        WHEN subscribers < 5 THEN 'LOW'
        WHEN subscribers < 20 THEN 'MEDIUM'
        ELSE 'HIGH'
    END AS activity_level
FROM pubsub_channels
ORDER BY subscribers DESC;
```

## Best Practices

### 1. Buffer Sizing

**Recommendation:** Set buffer size based on expected message volume and polling frequency.

```
buffer_size = (messages_per_second × poll_interval_seconds) × safety_factor
```

**Example:**
- 10 messages/second
- Poll every 5 seconds
- Safety factor: 2x
- Buffer size: 10 × 5 × 2 = **100 messages**

**Guidelines:**
- Default (1000) is suitable for most use cases
- Increase for high-volume channels or infrequent polling
- Decrease for low-volume channels to save memory

### 2. TTL Management

**Recommendation:** Set TTL based on expected subscription lifetime.

**Guidelines:**
- Short-lived subscriptions (minutes): `ttl: 300-900` (5-15 minutes)
- Medium-lived subscriptions (hours): `ttl: 3600-7200` (1-2 hours)
- Long-lived subscriptions (days): `ttl: 86400` (24 hours)
- Always unsubscribe explicitly when done to free resources immediately

### 3. Polling Frequency

**Recommendation:** Balance responsiveness with resource usage.

**Guidelines:**
- Real-time applications: Poll every 1-5 seconds
- Near real-time: Poll every 5-30 seconds
- Batch processing: Poll every 1-5 minutes
- Use `auto_acknowledge: true` for simple use cases
- Use `auto_acknowledge: false` for critical messages requiring processing confirmation

### 4. Message Acknowledgment

**Recommendation:** Use manual acknowledgment for critical messages.

**When to use auto-acknowledgment:**
- Messages can be safely lost
- Processing is idempotent
- Simplicity is preferred

**When to use manual acknowledgment:**
- Messages must be processed exactly once
- Processing may fail and require retry
- You need at-least-once delivery semantics

### 5. Pattern Subscriptions

**Recommendation:** Use patterns to subscribe to multiple related channels efficiently.

**Examples:**
```json
// Subscribe to all user channels
{"channel": "user:*", "is_pattern": true}

// Subscribe to all log levels
{"channel": "log:*", "is_pattern": true}

// Subscribe to all events for a specific entity
{"channel": "order:*:events", "is_pattern": true}
```

**Guidelines:**
- Patterns use Redis glob-style matching (`*` = any characters, `?` = single character)
- More specific patterns are better than overly broad ones
- Monitor buffer usage carefully with pattern subscriptions

### 6. Resource Cleanup

**Recommendation:** Always unsubscribe when done.

**Best practices:**
- Unsubscribe explicitly rather than relying on TTL expiration
- Monitor `messages_dropped` to detect buffer overflow
- Use SQL queries to identify and clean up stale subscriptions

```sql
-- Find subscriptions that should be cleaned up
SELECT id, channel, ttl, messages_dropped
FROM pubsub_subscriptions
WHERE ttl < 60 OR messages_dropped > 100;
```

### 7. Error Handling

**Recommendation:** Handle errors gracefully and implement retry logic.

**Common errors:**
- Subscription not found (expired or invalid ID)
- Buffer overflow (messages dropped)
- Connection failures

**Example error handling:**
```javascript
// Pseudo-code
try {
    response = poll_messages(subscription_id)
} catch (SubscriptionNotFoundError) {
    // Re-subscribe
    subscription_id = subscribe(channel)
} catch (ConnectionError) {
    // Retry with exponential backoff
    wait_and_retry()
}
```

## Limitations

### 1. Plugin-Scoped Subscriptions

**Limitation:** Only subscriptions created through this plugin instance are visible.

**Impact:**
- Subscriptions from other Redis clients are not shown in `pubsub_subscriptions`
- Messages from other clients' subscriptions are not accessible

**Workaround:** Use `pubsub_channels` to see all active channels regardless of client.

### 2. Buffer Overflow

**Limitation:** Messages may be dropped if buffer fills up before acknowledgment.

**Impact:**
- Oldest unacknowledged messages are dropped when buffer is full
- `messages_dropped` counter is an estimate

**Mitigation:**
- Increase buffer size for high-volume channels
- Poll more frequently
- Acknowledge messages promptly
- Monitor `messages_dropped` metric

### 3. No Historical Messages

**Limitation:** Only messages received after subscription are available.

**Impact:**
- Cannot retrieve messages published before subscription was created
- No message persistence across plugin restarts

**Workaround:** Use Redis Streams for persistent message history.

### 4. Pattern Detection

**Limitation:** `is_pattern` in `pubsub_channels` always returns `false`.

**Impact:**
- Cannot distinguish pattern channels from regular channels in `pubsub_channels` table

**Reason:** Redis `PUBSUB CHANNELS` command doesn't provide this information.

### 5. Last Message Time

**Limitation:** `last_message_time` in `pubsub_channels` always returns `0`.

**Impact:**
- Cannot determine when a channel last received a message

**Reason:** Redis doesn't track this metadata.

### 6. Single Plugin Instance

**Limitation:** Subscriptions are not shared across multiple plugin instances.

**Impact:**
- Each Tabularis connection creates its own subscriptions
- No subscription pooling or sharing

**Workaround:** Use a single long-lived connection for Pub/Sub operations.

### 7. Memory Usage

**Limitation:** Each subscription consumes memory for its buffer.

**Impact:**
- Many subscriptions with large buffers can consume significant memory
- Default: 1000 messages × ~1KB per message = ~1MB per subscription

**Mitigation:**
- Reduce buffer size for low-volume channels
- Limit number of concurrent subscriptions
- Monitor memory usage

## Troubleshooting

### Issue: Subscription Not Found

**Symptoms:**
```json
{
  "error": {
    "code": -32603,
    "message": "failed to get subscription: subscription not found: sub_xyz789"
  }
}
```

**Possible Causes:**
1. Subscription expired (TTL reached)
2. Invalid subscription ID
3. Subscription was unsubscribed

**Solutions:**
1. Check subscription TTL: `SELECT ttl FROM pubsub_subscriptions WHERE id = 'sub_xyz789'`
2. Verify subscription exists: `SELECT * FROM pubsub_subscriptions`
3. Re-subscribe if needed

### Issue: Messages Being Dropped

**Symptoms:**
```sql
SELECT * FROM pubsub_subscriptions WHERE messages_dropped > 0;
-- Shows subscriptions with dropped messages
```

**Possible Causes:**
1. Buffer too small for message volume
2. Polling too infrequent
3. Messages not being acknowledged

**Solutions:**
1. Increase buffer size: `"buffer_size": 2000`
2. Poll more frequently
3. Acknowledge messages after processing
4. Use auto-acknowledgment if appropriate

### Issue: No Messages Received

**Symptoms:**
- `pubsub_poll_messages` returns empty array
- `buffer_used` is 0

**Possible Causes:**
1. No messages published to channel
2. Subscription created after messages were published
3. Messages already acknowledged

**Solutions:**
1. Verify channel has publishers: `SELECT * FROM pubsub_channels WHERE channel = 'your_channel'`
2. Publish a test message
3. Check if messages were auto-acknowledged

### Issue: High Memory Usage

**Symptoms:**
- Plugin process consuming excessive memory
- System slowdown

**Possible Causes:**
1. Too many subscriptions
2. Large buffer sizes
3. Messages not being acknowledged

**Solutions:**
1. Reduce number of subscriptions
2. Decrease buffer sizes
3. Acknowledge messages promptly
4. Unsubscribe from unused channels

```sql
-- Find subscriptions with large buffers
SELECT id, channel, buffer_size, buffer_used
FROM pubsub_subscriptions
ORDER BY buffer_size DESC;
```

### Issue: Subscription Expiring Too Quickly

**Symptoms:**
- Frequent "subscription not found" errors
- Need to re-subscribe often

**Solutions:**
1. Increase TTL: `"ttl": 7200` (2 hours)
2. Implement automatic re-subscription logic
3. Monitor TTL and extend before expiration

```sql
-- Find subscriptions expiring soon
SELECT id, channel, ttl
FROM pubsub_subscriptions
WHERE ttl < 300
ORDER BY ttl ASC;
```

### Debug Checklist

- [ ] Verify Redis server is running: `redis-cli ping`
- [ ] Check subscription exists: `SELECT * FROM pubsub_subscriptions WHERE id = 'sub_id'`
- [ ] Verify channel has subscribers: `SELECT * FROM pubsub_channels WHERE channel = 'channel_name'`
- [ ] Check buffer usage: `SELECT buffer_used, buffer_size FROM pubsub_subscriptions`
- [ ] Monitor dropped messages: `SELECT messages_dropped FROM pubsub_subscriptions`
- [ ] Test with simple publish: `pubsub_publish` with test message
- [ ] Check TTL: `SELECT ttl FROM pubsub_subscriptions`

---

## Additional Resources

- **Design Document:** [`redis_pubsub_design.md`](redis_pubsub_design.md) - Architectural design and implementation details
- **Virtual Tables Implementation:** [`PUBSUB_VIRTUAL_TABLES.md`](PUBSUB_VIRTUAL_TABLES.md) - Technical details of virtual table implementation
- **Agent Guide:** [`AGENTS.md`](../AGENTS.md) - AI agent development guidance
- **Main README:** [`README.md`](../README.md) - General plugin documentation
- **Redis Pub/Sub Documentation:** [https://redis.io/docs/manual/pubsub/](https://redis.io/docs/manual/pubsub/)

---

**Questions or Issues?**

If you encounter issues not covered in this guide, please:
1. Check the troubleshooting section above
2. Review the design document for architectural details
3. Examine the source code in [`internal/plugin/pubsub.go`](../internal/plugin/pubsub.go)
4. Open an issue on the GitHub repository
