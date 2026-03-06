package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// PubSubMessage represents a message received from a Redis channel
type PubSubMessage struct {
	ID           int64  `json:"message_id"`
	Channel      string `json:"channel"`
	Payload      string `json:"payload"`
	PublishedAt  int64  `json:"published_at"`
	ReceivedAt   int64  `json:"received_at"`
	Acknowledged bool   `json:"-"`
}

// MessageBuffer is a thread-safe buffer for storing messages
type MessageBuffer struct {
	messages []*PubSubMessage
	capacity int
	nextID   int64
	mu       sync.Mutex
}

// NewMessageBuffer creates a new message buffer with the specified capacity
func NewMessageBuffer(capacity int) *MessageBuffer {
	if capacity <= 0 {
		capacity = 1000 // Default capacity
	}
	return &MessageBuffer{
		messages: make([]*PubSubMessage, 0, capacity),
		capacity: capacity,
		nextID:   1,
	}
}

// Add adds a message to the buffer. If the buffer is full, the oldest unacknowledged message is dropped.
func (mb *MessageBuffer) Add(channel, payload string, publishedAt int64) *PubSubMessage {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	msg := &PubSubMessage{
		ID:           mb.nextID,
		Channel:      channel,
		Payload:      payload,
		PublishedAt:  publishedAt,
		ReceivedAt:   time.Now().Unix(),
		Acknowledged: false,
	}
	mb.nextID++

	// If buffer is full, remove oldest unacknowledged message
	if len(mb.messages) >= mb.capacity {
		// Find and remove the first unacknowledged message
		for i, m := range mb.messages {
			if !m.Acknowledged {
				mb.messages = append(mb.messages[:i], mb.messages[i+1:]...)
				break
			}
		}
		// If all messages are acknowledged, remove the oldest one
		if len(mb.messages) >= mb.capacity {
			mb.messages = mb.messages[1:]
		}
	}

	mb.messages = append(mb.messages, msg)
	return msg
}

// Get retrieves up to maxMessages unacknowledged messages from the buffer.
// If autoAck is true, messages are automatically marked as acknowledged.
// Returns the messages and a boolean indicating if more messages are available.
func (mb *MessageBuffer) Get(maxMessages int, autoAck bool) ([]*PubSubMessage, bool) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if maxMessages <= 0 {
		maxMessages = 100 // Default max messages
	}

	result := make([]*PubSubMessage, 0, maxMessages)
	for _, msg := range mb.messages {
		if !msg.Acknowledged {
			result = append(result, msg)
			if autoAck {
				msg.Acknowledged = true
			}
			if len(result) >= maxMessages {
				break
			}
		}
	}

	// Check if there are more unacknowledged messages
	moreAvailable := false
	if len(result) >= maxMessages {
		for _, msg := range mb.messages {
			if !msg.Acknowledged {
				found := false
				for _, r := range result {
					if r.ID == msg.ID {
						found = true
						break
					}
				}
				if !found {
					moreAvailable = true
					break
				}
			}
		}
	}

	return result, moreAvailable
}

// Acknowledge marks messages with the specified IDs as acknowledged.
// Returns the number of messages that were successfully acknowledged.
func (mb *MessageBuffer) Acknowledge(messageIDs []int64) int {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	acknowledged := 0
	for _, id := range messageIDs {
		for _, msg := range mb.messages {
			if msg.ID == id && !msg.Acknowledged {
				msg.Acknowledged = true
				acknowledged++
				break
			}
		}
	}

	return acknowledged
}

// Size returns the current number of unacknowledged messages in the buffer
func (mb *MessageBuffer) Size() int {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	count := 0
	for _, msg := range mb.messages {
		if !msg.Acknowledged {
			count++
		}
	}
	return count
}

// Clear removes all acknowledged messages from the buffer
func (mb *MessageBuffer) Clear() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	newMessages := make([]*PubSubMessage, 0, len(mb.messages))
	for _, msg := range mb.messages {
		if !msg.Acknowledged {
			newMessages = append(newMessages, msg)
		}
	}
	mb.messages = newMessages
}

// Subscription represents an active subscription to a Redis channel
type Subscription struct {
	ID        string
	Channel   string
	IsPattern bool
	CreatedAt int64
	ExpiresAt int64
	Client    *redis.Client
	PubSub    *redis.PubSub
	Buffer    *MessageBuffer
	stopChan  chan struct{}
	mu        sync.Mutex
	running   bool
}

// NewSubscription creates a new subscription
func NewSubscription(id, channel string, isPattern bool, bufferSize, ttl int) *Subscription {
	now := time.Now().Unix()
	expiresAt := now + int64(ttl)
	if ttl <= 0 {
		expiresAt = now + 3600 // Default 1 hour TTL
	}

	return &Subscription{
		ID:        id,
		Channel:   channel,
		IsPattern: isPattern,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Buffer:    NewMessageBuffer(bufferSize),
		stopChan:  make(chan struct{}),
		running:   false,
	}
}

// Start begins listening for messages on the subscribed channel
func (s *Subscription) Start(client *redis.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("subscription already running")
	}

	s.Client = client

	// Create PubSub instance
	if s.IsPattern {
		s.PubSub = client.PSubscribe(context.Background(), s.Channel)
	} else {
		s.PubSub = client.Subscribe(context.Background(), s.Channel)
	}

	// Wait for confirmation
	_, err := s.PubSub.Receive(context.Background())
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	s.running = true

	// Start goroutine to receive messages
	go s.receiveMessages()

	return nil
}

// receiveMessages listens for messages and adds them to the buffer
func (s *Subscription) receiveMessages() {
	ch := s.PubSub.Channel()

	for {
		select {
		case <-s.stopChan:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg != nil {
				// Add message to buffer
				s.Buffer.Add(msg.Channel, msg.Payload, time.Now().Unix())
			}
		}
	}
}

// Stop ends the subscription and closes the Redis connection
func (s *Subscription) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	close(s.stopChan)
	s.running = false

	if s.PubSub != nil {
		err := s.PubSub.Close()
		if err != nil {
			return fmt.Errorf("failed to close pubsub: %w", err)
		}
	}

	return nil
}

// IsExpired checks if the subscription has expired
func (s *Subscription) IsExpired() bool {
	return time.Now().Unix() > s.ExpiresAt
}

// Extend extends the subscription's TTL by the specified number of seconds
func (s *Subscription) Extend(ttl int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExpiresAt = time.Now().Unix() + int64(ttl)
}

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

// Subscribe creates a new subscription to a Redis channel
func (sm *SubscriptionManager) Subscribe(client *redis.Client, channel string, isPattern bool, bufferSize int, ttl int) (*Subscription, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate unique subscription ID
	id := generateSubscriptionID()

	// Create subscription
	sub := NewSubscription(id, channel, isPattern, bufferSize, ttl)

	// Start the subscription
	err := sub.Start(client)
	if err != nil {
		return nil, fmt.Errorf("failed to start subscription: %w", err)
	}

	// Store subscription
	sm.subscriptions[id] = sub

	return sub, nil
}

// Unsubscribe removes a subscription and returns the number of messages dropped
func (sm *SubscriptionManager) Unsubscribe(subscriptionID string) (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sub, exists := sm.subscriptions[subscriptionID]
	if !exists {
		return 0, fmt.Errorf("subscription not found: %s", subscriptionID)
	}

	// Stop the subscription
	err := sub.Stop()
	if err != nil {
		return 0, fmt.Errorf("failed to stop subscription: %w", err)
	}

	// Count dropped messages
	messagesDropped := sub.Buffer.Size()

	// Remove from map
	delete(sm.subscriptions, subscriptionID)

	return messagesDropped, nil
}

// GetSubscription retrieves a subscription by ID
func (sm *SubscriptionManager) GetSubscription(subscriptionID string) (*Subscription, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sub, exists := sm.subscriptions[subscriptionID]
	if !exists {
		return nil, fmt.Errorf("subscription not found: %s", subscriptionID)
	}

	return sub, nil
}

// CleanupExpiredSubscriptions removes expired subscriptions
func (sm *SubscriptionManager) CleanupExpiredSubscriptions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, sub := range sm.subscriptions {
		if sub.IsExpired() {
			sub.Stop()
			delete(sm.subscriptions, id)
			fmt.Fprintf(os.Stderr, "DEBUG: Cleaned up expired subscription: %s\n", id)
		}
	}
}

// ListSubscriptions returns all active subscriptions
func (sm *SubscriptionManager) ListSubscriptions() []*Subscription {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs := make([]*Subscription, 0, len(sm.subscriptions))
	for _, sub := range sm.subscriptions {
		subs = append(subs, sub)
	}
	return subs
}

// generateSubscriptionID generates a unique subscription ID
func generateSubscriptionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "sub_" + hex.EncodeToString(bytes)
}

// Global subscription manager instance
var globalSubscriptionManager = NewSubscriptionManager()

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
	Success         bool   `json:"success"`
	SubscriptionID  string `json:"subscription_id"`
	MessagesDropped int    `json:"messages_dropped"`
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
	Success              bool   `json:"success"`
	SubscriptionID       string `json:"subscription_id"`
	MessagesAcknowledged int    `json:"messages_acknowledged"`
}

// HandlePubSubSubscribe handles the pubsub_subscribe JSON-RPC method
func HandlePubSubSubscribe(req PubSubSubscribeRequest) (PubSubSubscribeResponse, error) {
	// Validate channel
	if req.Channel == "" {
		return PubSubSubscribeResponse{}, fmt.Errorf("channel is required")
	}

	// Set defaults
	if req.BufferSize <= 0 {
		req.BufferSize = 1000
	}
	if req.TTL <= 0 {
		req.TTL = 3600
	}

	// Create Redis client
	client, err := getClient(req.Params)
	if err != nil {
		return PubSubSubscribeResponse{}, fmt.Errorf("failed to create Redis client: %w", err)
	}

	// Create subscription
	sub, err := globalSubscriptionManager.Subscribe(client, req.Channel, req.IsPattern, req.BufferSize, req.TTL)
	if err != nil {
		return PubSubSubscribeResponse{}, fmt.Errorf("failed to create subscription: %w", err)
	}

	return PubSubSubscribeResponse{
		SubscriptionID: sub.ID,
		Channel:        sub.Channel,
		IsPattern:      sub.IsPattern,
		CreatedAt:      sub.CreatedAt,
		BufferSize:     sub.Buffer.Size(),
		MaxBuffer:      req.BufferSize,
		TTL:            req.TTL,
	}, nil
}

// HandlePubSubUnsubscribe handles the pubsub_unsubscribe JSON-RPC method
func HandlePubSubUnsubscribe(req PubSubUnsubscribeRequest) (PubSubUnsubscribeResponse, error) {
	// Validate subscription ID
	if req.SubscriptionID == "" {
		return PubSubUnsubscribeResponse{}, fmt.Errorf("subscription_id is required")
	}

	// Unsubscribe
	messagesDropped, err := globalSubscriptionManager.Unsubscribe(req.SubscriptionID)
	if err != nil {
		return PubSubUnsubscribeResponse{}, fmt.Errorf("failed to unsubscribe: %w", err)
	}

	return PubSubUnsubscribeResponse{
		Success:         true,
		SubscriptionID:  req.SubscriptionID,
		MessagesDropped: messagesDropped,
	}, nil
}

// HandlePubSubPublish handles the pubsub_publish JSON-RPC method
func HandlePubSubPublish(req PubSubPublishRequest) (PubSubPublishResponse, error) {
	// Validate channel and message
	if req.Channel == "" {
		return PubSubPublishResponse{}, fmt.Errorf("channel is required")
	}
	if req.Message == "" {
		return PubSubPublishResponse{}, fmt.Errorf("message is required")
	}

	// Create Redis client
	client, err := getClient(req.Params)
	if err != nil {
		return PubSubPublishResponse{}, fmt.Errorf("failed to create Redis client: %w", err)
	}
	defer client.Close()

	// Publish message
	receivers, err := client.Publish(context.Background(), req.Channel, req.Message).Result()
	if err != nil {
		return PubSubPublishResponse{}, fmt.Errorf("failed to publish message: %w", err)
	}

	return PubSubPublishResponse{
		Success:   true,
		Channel:   req.Channel,
		Receivers: receivers,
	}, nil
}

// HandlePubSubPollMessages handles the pubsub_poll_messages JSON-RPC method
func HandlePubSubPollMessages(req PubSubPollMessagesRequest) (PubSubPollMessagesResponse, error) {
	// Validate subscription ID
	if req.SubscriptionID == "" {
		return PubSubPollMessagesResponse{}, fmt.Errorf("subscription_id is required")
	}

	// Get subscription
	sub, err := globalSubscriptionManager.GetSubscription(req.SubscriptionID)
	if err != nil {
		return PubSubPollMessagesResponse{}, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Set defaults
	if req.MaxMessages <= 0 {
		req.MaxMessages = 100
	}

	// Get messages from buffer
	messages, moreAvailable := sub.Buffer.Get(req.MaxMessages, req.AutoAcknowledge)

	return PubSubPollMessagesResponse{
		Messages:       messages,
		MoreAvailable:  moreAvailable,
		SubscriptionID: req.SubscriptionID,
		BufferSize:     sub.Buffer.Size(),
	}, nil
}

// HandlePubSubAcknowledgeMessages handles the pubsub_acknowledge_messages JSON-RPC method
func HandlePubSubAcknowledgeMessages(req PubSubAcknowledgeMessagesRequest) (PubSubAcknowledgeMessagesResponse, error) {
	// Validate subscription ID
	if req.SubscriptionID == "" {
		return PubSubAcknowledgeMessagesResponse{}, fmt.Errorf("subscription_id is required")
	}

	// Validate message IDs
	if len(req.MessageIDs) == 0 {
		return PubSubAcknowledgeMessagesResponse{}, fmt.Errorf("message_ids is required and must not be empty")
	}

	// Get subscription
	sub, err := globalSubscriptionManager.GetSubscription(req.SubscriptionID)
	if err != nil {
		return PubSubAcknowledgeMessagesResponse{}, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Acknowledge messages in the buffer
	acknowledged := sub.Buffer.Acknowledge(req.MessageIDs)

	return PubSubAcknowledgeMessagesResponse{
		Success:              true,
		SubscriptionID:       req.SubscriptionID,
		MessagesAcknowledged: acknowledged,
	}, nil
}
