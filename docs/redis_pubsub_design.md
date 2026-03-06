# Redis Pub/Sub Implementation Design for Tabularis Redis Plugin

## 1. Introduction

This document outlines a comprehensive design for implementing Redis Pub/Sub functionality in the Tabularis Redis plugin. The design addresses the challenges of integrating an asynchronous Pub/Sub model with the synchronous JSON-RPC protocol used by the plugin.

### 1.1 Background

Redis Pub/Sub is a messaging pattern where senders (publishers) send messages to channels without knowledge of which receivers (subscribers) will receive them. Subscribers express interest in one or more channels and only receive messages from the channels they are subscribed to.

### 1.2 Challenges

The implementation faces several challenges:

1. **Protocol Mismatch**: Redis Pub/Sub is asynchronous, while Tabularis uses a synchronous JSON-RPC protocol.
2. **Connection Management**: Each request creates a new Redis client, making persistent subscriptions challenging.
3. **Single-Threaded Processing**: The plugin processes requests sequentially.
4. **Output Constraints**: Only valid JSON-RPC responses can be written to stdout.
5. **Consistency**: The solution should be consistent with the existing virtual table model.

## 2. Design Approach

### 2.1 Architectural Overview

The design adopts a **polling-based approach** with **message buffering** to bridge the gap between Redis Pub/Sub's asynchronous nature and Tabularis's synchronous protocol. This approach maintains compatibility with the existing architecture while providing Pub/Sub functionality.

### 2.2 Key Components

1. **Message Buffer**: A server-side buffer to store messages received from subscribed channels.
2. **Subscription Manager**: Manages active subscriptions and their associated message buffers.
3. **Virtual Tables**: New virtual tables to represent channels and messages.
4. **JSON-RPC Methods**: New methods for publishing, subscribing, and retrieving messages.

### 2.3 Approach Justification

The polling-based approach was chosen over alternatives for the following reasons:

1. **Compatibility**: Maintains the existing JSON-RPC request-response pattern.
2. **Simplicity**: Avoids complex connection pooling or multi-threading.
3. **Reliability**: Messages are buffered and won't be lost between polls.
4. **Consistency**: Integrates with the existing virtual table model.

## 3. Virtual Table Schemas

### 3.1 Channels Table

The `pubsub_channels` table represents available channels and their metadata:

| Column Name | Data Type | Description | Primary Key |
|-------------|-----------|-------------|------------|
| channel     | STRING    | Channel name | Yes |
| num_subscribers | INTEGER | Number of subscribers (if available) | No |
| pattern     | BOOLEAN   | Whether it's a pattern subscription | No |
| last_message_time | INTEGER | Timestamp of last message (Unix time) | No |

### 3.2 Messages Table

The `pubsub_messages` table represents messages published to channels:

| Column Name | Data Type | Description | Primary Key |
|-------------|-----------|-------------|------------|
| channel     | STRING    | Channel name | Yes (composite) |
| message_id  | INTEGER   | Auto-incremented message ID | Yes (composite) |
| payload     | STRING    | Message content | No |
| published_at | INTEGER  | Timestamp when message was published (Unix time) | No |
| received_at | INTEGER   | Timestamp when message was received (Unix time) | No |

### 3.3 Subscriptions Table

The `pubsub_subscriptions` table represents active subscriptions:

| Column Name | Data Type | Description | Primary Key |
|-------------|-----------|-------------|------------|
| id          | STRING    | Subscription ID | Yes |
| channel     | STRING    | Channel name or pattern | No |
| is_pattern  | BOOLEAN   | Whether it's a pattern subscription | No |
| created_at  | INTEGER   | Subscription creation time (Unix time) | No |
| buffer_size | INTEGER   | Current number of buffered messages | No |
| max_buffer  | INTEGER   | Maximum buffer size | No |

## 4. JSON-RPC Methods

### 4.1 Subscribe Method

```
Method: pubsub_subscribe
```

**Parameters:**
```json
{
  "params": {
    "driver": "redis",
    "host": "localhost",
    "port": 6379,
    "database": "0"
  },
  "channel": "my-channel",
  "is_pattern": false,
  "buffer_size": 1000,
  "ttl": 3600
}
```

**Response:**
```json
{
  "subscription_id": "sub_12345",
  "channel": "my-channel",
  "is_pattern": false,
  "created_at": 1646245487,
  "buffer_size": 0,
  "max_buffer": 1000,
  "ttl": 3600
}
```

### 4.2 Unsubscribe Method

```
Method: pubsub_unsubscribe
```

**Parameters:**
```json
{
  "params": {
    "driver": "redis",
    "host": "localhost",
    "port": 6379,
    "database": "0"
  },
  "subscription_id": "sub_12345"
}
```

**Response:**
```json
{
  "success": true,
  "subscription_id": "sub_12345",
  "messages_dropped": 5
}
```

### 4.3 Publish Method

```
Method: pubsub_publish
```

**Parameters:**
```json
{
  "params": {
    "driver": "redis",
    "host": "localhost",
    "port": 6379,
    "database": "0"
  },
  "channel": "my-channel",
  "message": "Hello, Redis!"
}
```

**Response:**
```json
{
  "success": true,
  "channel": "my-channel",
  "receivers": 3
}
```

### 4.4 Poll Messages Method

```
Method: pubsub_poll_messages
```

**Parameters:**
```json
{
  "params": {
    "driver": "redis",
    "host": "localhost",
    "port": 6379,
    "database": "0"
  },
  "subscription_id": "sub_12345",
  "max_messages": 100,
  "timeout_ms": 1000,
  "auto_acknowledge": true
}
```

**Response:**
```json
{
  "messages": [
    {
      "channel": "my-channel",
      "message_id": 1,
      "payload": "Hello, Redis!",
      "published_at": 1646245487,
      "received_at": 1646245488
    },
    {
      "channel": "my-channel",
      "message_id": 2,
      "payload": "Another message",
      "published_at": 1646245490,
      "received_at": 1646245491
    }
  ],
  "more_available": true,
  "subscription_id": "sub_12345",
  "buffer_size": 8
}
```

### 4.5 Acknowledge Messages Method

```
Method: pubsub_acknowledge_messages
```

**Parameters:**
```json
{
  "params": {
    "driver": "redis",
    "host": "localhost",
    "port": 6379,
    "database": "0"
  },
  "subscription_id": "sub_12345",
  "message_ids": [1, 2]
}
```

**Response:**
```json
{
  "success": true,
  "subscription_id": "sub_12345",
  "acknowledged": 2
}
```

## 5. Data Structures and Interfaces

### 5.1 Subscription Manager

```go
// SubscriptionManager manages active subscriptions
type SubscriptionManager struct {
    subscriptions map[string]*Subscription
    mu            sync.Mutex
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager() *SubscriptionManager {
    return &SubscriptionManager{
        subscriptions: make(map[string]*Subscription),
    }
}

// Subscribe creates a new subscription
func (sm *SubscriptionManager) Subscribe(channel string, isPattern bool, bufferSize int, ttl int) (*Subscription, error) {
    // Implementation
}

// Unsubscribe removes a subscription
func (sm *SubscriptionManager) Unsubscribe(subscriptionID string) error {
    // Implementation
}

// GetSubscription retrieves a subscription by ID
func (sm *SubscriptionManager) GetSubscription(subscriptionID string) (*Subscription, error) {
    // Implementation
}

// CleanupExpiredSubscriptions removes expired subscriptions
func (sm *SubscriptionManager) CleanupExpiredSubscriptions() {
    // Implementation
}
```

### 5.2 Subscription

```go
// Subscription represents an active subscription to a Redis channel
type Subscription struct {
    ID          string
    Channel     string
    IsPattern   bool
    CreatedAt   int64
    ExpiresAt   int64
    Client      *redis.Client
    PubSub      *redis.PubSub
    MessageChan <-chan *redis.Message
    Buffer      *MessageBuffer
    mu          sync.Mutex
}

// NewSubscription creates a new subscription
func NewSubscription(id, channel string, isPattern bool, bufferSize, ttl int) *Subscription {
    // Implementation
}

// Start begins listening for messages
func (s *Subscription) Start(client *redis.Client) error {
    // Implementation
}

// Stop ends the subscription
func (s *Subscription) Stop() error {
    // Implementation
}

// IsExpired checks if the subscription has expired
func (s *Subscription) IsExpired() bool {
    // Implementation
}

// Extend extends the subscription's TTL
func (s *Subscription) Extend(ttl int) {
    // Implementation
}
```

### 5.3 Message Buffer

```go
// MessageBuffer is a thread-safe buffer for storing messages
type MessageBuffer struct {
    messages     []*PubSubMessage
    capacity     int
    nextID       int64
    mu           sync.Mutex
}

// NewMessageBuffer creates a new message buffer
func NewMessageBuffer(capacity int) *MessageBuffer {
    // Implementation
}

// Add adds a message to the buffer
func (mb *MessageBuffer) Add(channel, payload string, publishedAt int64) *PubSubMessage {
    // Implementation
}

// Get retrieves messages from the buffer
func (mb *MessageBuffer) Get(maxMessages int, autoAck bool) ([]*PubSubMessage, bool) {
    // Implementation
}

// Acknowledge marks messages as acknowledged
func (mb *MessageBuffer) Acknowledge(messageIDs []int64) int {
    // Implementation
}

// Size returns the current buffer size
func (mb *MessageBuffer) Size() int {
    // Implementation
}
```

### 5.4 PubSub Message

```go
// PubSubMessage represents a message received from a Redis channel
type PubSubMessage struct {
    ID          int64  `json:"message_id"`
    Channel     string `json:"channel"`
    Payload     string `json:"payload"`
    PublishedAt int64  `json:"published_at"`
    ReceivedAt  int64  `json:"received_at"`
    Acknowledged bool  `json:"-"`
}
```

### 5.5 Request/Response Types

```go
// PubSubSubscribeRequest represents a subscribe request
type PubSubSubscribeRequest struct {
    Params     ConnectionParams `json:"params"`
    Channel    string           `json:"channel"`
    IsPattern  bool             `json:"is_pattern"`
    BufferSize int              `json:"buffer_size"`
    TTL        int              `json:"ttl"`
}

// PubSubSubscribeResponse represents a subscribe response
type PubSubSubscribeResponse struct {
    SubscriptionID string `json:"subscription_id"`
    Channel        string `json:"channel"`
    IsPattern      bool   `json:"is_pattern"`
    CreatedAt      int64  `json:"created_at"`
    BufferSize     int    `json:"buffer_size"`
    MaxBuffer      int    `json:"max_buffer"`
    TTL            int    `json:"ttl"`
}

// PubSubUnsubscribeRequest represents an unsubscribe request
type PubSubUnsubscribeRequest struct {
    Params         ConnectionParams `json:"params"`
    SubscriptionID string           `json:"subscription_id"`
}

// PubSubUnsubscribeResponse represents an unsubscribe response
type PubSubUnsubscribeResponse struct {
    Success        bool   `json:"success"`
    SubscriptionID string `json:"subscription_id"`
    MessagesDropped int   `json:"messages_dropped"`
}

// PubSubPublishRequest represents a publish request
type PubSubPublishRequest struct {
    Params  ConnectionParams `json:"params"`
    Channel string           `json:"channel"`
    Message string           `json:"message"`
}

// PubSubPublishResponse represents a publish response
type PubSubPublishResponse struct {
    Success   bool   `json:"success"`
    Channel   string `json:"channel"`
    Receivers int64  `json:"receivers"`
}

// PubSubPollMessagesRequest represents a poll messages request
type PubSubPollMessagesRequest struct {
    Params          ConnectionParams `json:"params"`
    SubscriptionID  string           `json:"subscription_id"`
    MaxMessages     int              `json:"max_messages"`
    TimeoutMs       int              `json:"timeout_ms"`
    AutoAcknowledge bool             `json:"auto_acknowledge"`
}

// PubSubPollMessagesResponse represents a poll messages response
type PubSubPollMessagesResponse struct {
    Messages       []*PubSubMessage `json:"messages"`
    MoreAvailable  bool             `json:"more_available"`
    SubscriptionID string           `json:"subscription_id"`
    BufferSize     int              `json:"buffer_size"`
}

// PubSubAcknowledgeMessagesRequest represents an acknowledge messages request
type PubSubAcknowledgeMessagesRequest struct {
    Params         ConnectionParams `json:"params"`
    SubscriptionID string           `json:"subscription_id"`
    MessageIDs     []int64          `json:"message_ids"`
}

// PubSubAcknowledgeMessagesResponse represents an acknowledge messages response
type PubSubAcknowledgeMessagesResponse struct {
    Success        bool   `json:"success"`
    SubscriptionID string `json:"subscription_id"`
    Acknowledged   int    `json:"acknowledged"`
}
```

## 6. Implementation Details

### 6.1 Subscription Lifecycle

1. **Creation**: When a client calls `pubsub_subscribe`, a new subscription is created with a unique ID.
2. **Message Reception**: The subscription starts a goroutine that listens for messages and adds them to the buffer.
3. **Message Retrieval**: Clients poll for messages using `pubsub_poll_messages`.
4. **Acknowledgment**: Clients acknowledge processed messages using `pubsub_acknowledge_messages`.
5. **Expiration**: Subscriptions expire after their TTL unless extended.
6. **Termination**: Clients can explicitly terminate subscriptions using `pubsub_unsubscribe`.

### 6.2 Message Buffering

1. Messages are stored in a fixed-size buffer per subscription.
2. When the buffer is full, the oldest unacknowledged messages are dropped.
3. Messages are assigned sequential IDs within each subscription.
4. Clients can control buffer size and acknowledgment behavior.

### 6.3 Virtual Table Integration

The virtual tables are implemented using the existing query execution framework:

1. **Table Registration**: Add new tables to the `get_tables` handler.
2. **Column Definition**: Define columns in the `getTableColumns` function.
3. **Query Execution**: Implement query handlers for each table.

### 6.4 Background Processing

A background goroutine performs maintenance tasks:

1. **Subscription Cleanup**: Remove expired subscriptions.
2. **Buffer Management**: Enforce buffer size limits.
3. **Connection Health**: Monitor Redis connections.

## 7. Error Handling and Edge Cases

### 7.1 Connection Failures

1. **Subscription Creation**: If Redis connection fails during subscription, return an error.
2. **Message Reception**: If connection drops, buffer stops receiving messages. Reconnection is attempted on the next poll.
3. **Publish Failures**: If publish fails, return an error with details.

### 7.2 Resource Management

1. **Memory Usage**: Limit total number of subscriptions and buffer sizes.
2. **Subscription Cleanup**: Automatically clean up expired subscriptions.
3. **Client Disconnection**: Detect and clean up abandoned subscriptions.

### 7.3 Race Conditions

1. **Concurrent Access**: Use mutexes to protect shared data structures.
2. **Message Ordering**: Ensure message IDs are assigned in a thread-safe manner.
3. **Subscription State**: Protect subscription state with locks.

### 7.4 Edge Cases

1. **Pattern Subscriptions**: Handle special considerations for pattern-based subscriptions.
2. **Empty Buffers**: Handle polling from empty buffers gracefully.
3. **Large Messages**: Handle messages that exceed buffer capacity.
4. **High Message Volume**: Implement backpressure mechanisms.

## 8. Performance Considerations

### 8.1 Memory Usage

1. **Buffer Sizing**: Default buffer size should be reasonable (e.g., 1000 messages).
2. **Message Retention**: Implement TTL for buffered messages.
3. **Subscription Limits**: Limit the number of active subscriptions per connection.

### 8.2 CPU Usage

1. **Polling Frequency**: Recommend reasonable polling intervals to clients.
2. **Background Processing**: Limit frequency of background maintenance tasks.
3. **Message Processing**: Optimize message serialization and deserialization.

### 8.3 Network Usage

1. **Batch Processing**: Support retrieving multiple messages in a single poll.
2. **Selective Polling**: Allow polling specific subscriptions.
3. **Compression**: Consider compressing large message payloads.

## 9. Testing Strategy

### 9.1 Unit Tests

1. **Subscription Manager**: Test subscription creation, retrieval, and deletion.
2. **Message Buffer**: Test adding, retrieving, and acknowledging messages.
3. **JSON-RPC Handlers**: Test request parsing and response generation.

### 9.2 Integration Tests

1. **Redis Interaction**: Test actual Redis Pub/Sub operations.
2. **End-to-End Flow**: Test the complete publish-subscribe-poll flow.
3. **Error Handling**: Test recovery from connection failures.

### 9.3 Performance Tests

1. **High Volume**: Test with high message rates.
2. **Large Messages**: Test with large message payloads.
3. **Many Subscriptions**: Test with many concurrent subscriptions.

## 10. Implementation Plan

### 10.1 Phase 1: Core Infrastructure

1. Implement subscription manager and message buffer.
2. Add new JSON-RPC methods for subscribe, unsubscribe, and publish.
3. Implement background maintenance tasks.

### 10.2 Phase 2: Virtual Tables

1. Implement `pubsub_channels` virtual table.
2. Implement `pubsub_messages` virtual table.
3. Implement `pubsub_subscriptions` virtual table.

### 10.3 Phase 3: Advanced Features

1. Implement pattern-based subscriptions.
2. Add message acknowledgment and buffer management.
3. Implement subscription TTL and auto-cleanup.

### 10.4 Phase 4: Testing and Optimization

1. Write comprehensive tests.
2. Optimize performance.
3. Document the API and usage patterns.

## 11. Conclusion

This design provides a comprehensive approach to implementing Redis Pub/Sub functionality in the Tabularis Redis plugin. By using a polling-based approach with message buffering, we can bridge the gap between Redis's asynchronous Pub/Sub model and Tabularis's synchronous JSON-RPC protocol.

The implementation maintains compatibility with the existing architecture while providing powerful Pub/Sub capabilities through both direct JSON-RPC methods and virtual tables. This allows users to leverage Redis Pub/Sub functionality within the familiar SQL-like interface of Tabularis.