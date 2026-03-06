package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

// ============================================================================
// Phase 1: Core Components Tests
// ============================================================================

// TestMessageBuffer_Add tests adding messages to the buffer
func TestMessageBuffer_Add(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add a message
	msg := buffer.Add("channel1", "payload1", time.Now().Unix())

	if msg.ID != 1 {
		t.Errorf("Expected message ID 1, got %d", msg.ID)
	}
	if msg.Channel != "channel1" {
		t.Errorf("Expected channel 'channel1', got %s", msg.Channel)
	}
	if msg.Payload != "payload1" {
		t.Errorf("Expected payload 'payload1', got %s", msg.Payload)
	}
	if msg.Acknowledged {
		t.Error("Expected message to be unacknowledged")
	}
	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1, got %d", buffer.Size())
	}
}

// TestMessageBuffer_AddMultiple tests adding multiple messages
func TestMessageBuffer_AddMultiple(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add multiple messages
	for i := 1; i <= 5; i++ {
		msg := buffer.Add("channel1", "payload", time.Now().Unix())
		if msg.ID != int64(i) {
			t.Errorf("Expected message ID %d, got %d", i, msg.ID)
		}
	}

	if buffer.Size() != 5 {
		t.Errorf("Expected buffer size 5, got %d", buffer.Size())
	}
}

// TestMessageBuffer_AddOverCapacity tests buffer overflow behavior
func TestMessageBuffer_AddOverCapacity(t *testing.T) {
	buffer := NewMessageBuffer(3)

	// Add messages up to capacity
	buffer.Add("channel1", "payload1", time.Now().Unix())
	buffer.Add("channel1", "payload2", time.Now().Unix())
	buffer.Add("channel1", "payload3", time.Now().Unix())

	if buffer.Size() != 3 {
		t.Errorf("Expected buffer size 3, got %d", buffer.Size())
	}

	// Add one more message (should drop oldest unacknowledged)
	buffer.Add("channel1", "payload4", time.Now().Unix())

	if buffer.Size() != 3 {
		t.Errorf("Expected buffer size 3 after overflow, got %d", buffer.Size())
	}

	// Get messages and verify oldest was dropped
	messages, _ := buffer.Get(10, false)
	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}
	if messages[0].Payload != "payload2" {
		t.Errorf("Expected first message to be 'payload2', got %s", messages[0].Payload)
	}
}

// TestMessageBuffer_Get tests retrieving messages
func TestMessageBuffer_Get(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add some messages
	buffer.Add("channel1", "payload1", time.Now().Unix())
	buffer.Add("channel1", "payload2", time.Now().Unix())
	buffer.Add("channel1", "payload3", time.Now().Unix())

	// Get messages without auto-ack
	messages, moreAvailable := buffer.Get(2, false)

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	if !moreAvailable {
		t.Error("Expected more messages to be available")
	}
	if buffer.Size() != 3 {
		t.Errorf("Expected buffer size 3 (no auto-ack), got %d", buffer.Size())
	}
}

// TestMessageBuffer_GetWithAutoAck tests the Get method with auto-acknowledgment
func TestMessageBuffer_GetWithAutoAck(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add some messages
	buffer.Add("channel1", "payload1", time.Now().Unix())
	buffer.Add("channel1", "payload2", time.Now().Unix())
	buffer.Add("channel1", "payload3", time.Now().Unix())

	// Get messages with auto-acknowledgment
	messages, moreAvailable := buffer.Get(2, true)

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	if !moreAvailable {
		t.Error("Expected more messages to be available")
	}

	// Verify buffer size (only unacknowledged messages)
	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1 after auto-ack, got %d", buffer.Size())
	}

	// Get remaining messages
	messages, moreAvailable = buffer.Get(10, true)

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if moreAvailable {
		t.Error("Expected no more messages to be available")
	}

	// Verify buffer is now empty
	if buffer.Size() != 0 {
		t.Errorf("Expected buffer size 0, got %d", buffer.Size())
	}
}

// TestMessageBuffer_GetWithoutAutoAck tests the Get method without auto-acknowledgment
func TestMessageBuffer_GetWithoutAutoAck(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add some messages
	msg1 := buffer.Add("channel1", "payload1", time.Now().Unix())
	msg2 := buffer.Add("channel1", "payload2", time.Now().Unix())
	buffer.Add("channel1", "payload3", time.Now().Unix())

	// Get messages without auto-acknowledgment
	messages, moreAvailable := buffer.Get(2, false)

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	if !moreAvailable {
		t.Error("Expected more messages to be available")
	}

	// Verify buffer size (all messages still unacknowledged)
	if buffer.Size() != 3 {
		t.Errorf("Expected buffer size 3 (no auto-ack), got %d", buffer.Size())
	}

	// Manually acknowledge the first two messages
	acknowledged := buffer.Acknowledge([]int64{msg1.ID, msg2.ID})
	if acknowledged != 2 {
		t.Errorf("Expected 2 messages acknowledged, got %d", acknowledged)
	}

	// Verify buffer size decreased
	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1 after manual ack, got %d", buffer.Size())
	}
}

// TestMessageBuffer_Acknowledge tests the Acknowledge method
func TestMessageBuffer_Acknowledge(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add some messages
	msg1 := buffer.Add("channel1", "payload1", time.Now().Unix())
	msg2 := buffer.Add("channel1", "payload2", time.Now().Unix())
	msg3 := buffer.Add("channel1", "payload3", time.Now().Unix())

	// Verify initial state
	if buffer.Size() != 3 {
		t.Errorf("Expected buffer size 3, got %d", buffer.Size())
	}

	// Acknowledge msg1 and msg2
	acknowledged := buffer.Acknowledge([]int64{msg1.ID, msg2.ID})
	if acknowledged != 2 {
		t.Errorf("Expected 2 messages acknowledged, got %d", acknowledged)
	}

	// Verify buffer size (only unacknowledged messages)
	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1 after acknowledgment, got %d", buffer.Size())
	}

	// Try to acknowledge the same messages again (should return 0)
	acknowledged = buffer.Acknowledge([]int64{msg1.ID, msg2.ID})
	if acknowledged != 0 {
		t.Errorf("Expected 0 messages acknowledged (already acked), got %d", acknowledged)
	}

	// Acknowledge msg3
	acknowledged = buffer.Acknowledge([]int64{msg3.ID})
	if acknowledged != 1 {
		t.Errorf("Expected 1 message acknowledged, got %d", acknowledged)
	}

	// Verify buffer is now empty
	if buffer.Size() != 0 {
		t.Errorf("Expected buffer size 0 after all acknowledgments, got %d", buffer.Size())
	}
}

// TestMessageBuffer_AcknowledgeNonExistent tests acknowledging non-existent message IDs
func TestMessageBuffer_AcknowledgeNonExistent(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add a message
	msg := buffer.Add("channel1", "payload1", time.Now().Unix())

	// Try to acknowledge a non-existent message ID
	acknowledged := buffer.Acknowledge([]int64{999, 1000})
	if acknowledged != 0 {
		t.Errorf("Expected 0 messages acknowledged (non-existent IDs), got %d", acknowledged)
	}

	// Verify the real message is still unacknowledged
	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1, got %d", buffer.Size())
	}

	// Acknowledge the real message along with non-existent ones
	acknowledged = buffer.Acknowledge([]int64{999, msg.ID, 1000})
	if acknowledged != 1 {
		t.Errorf("Expected 1 message acknowledged, got %d", acknowledged)
	}

	// Verify buffer is now empty
	if buffer.Size() != 0 {
		t.Errorf("Expected buffer size 0, got %d", buffer.Size())
	}
}

// TestMessageBuffer_Clear tests clearing acknowledged messages
func TestMessageBuffer_Clear(t *testing.T) {
	buffer := NewMessageBuffer(10)

	// Add messages
	msg1 := buffer.Add("channel1", "payload1", time.Now().Unix())
	msg2 := buffer.Add("channel1", "payload2", time.Now().Unix())
	buffer.Add("channel1", "payload3", time.Now().Unix())

	// Acknowledge some messages
	buffer.Acknowledge([]int64{msg1.ID, msg2.ID})

	// Clear acknowledged messages
	buffer.Clear()

	// Verify only unacknowledged messages remain
	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1 after clear, got %d", buffer.Size())
	}
}

// TestMessageBuffer_DefaultCapacity tests default capacity
func TestMessageBuffer_DefaultCapacity(t *testing.T) {
	buffer := NewMessageBuffer(0)

	if buffer.capacity != 1000 {
		t.Errorf("Expected default capacity 1000, got %d", buffer.capacity)
	}
}

// TestSubscriptionManager_Subscribe tests creating a subscription
func TestSubscriptionManager_Subscribe(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	manager := NewSubscriptionManager()

	// Create connection params
	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, err := getClient(params)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Subscribe to a channel
	sub, err := manager.Subscribe(client, "test-channel", false, 100, 3600)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer manager.Unsubscribe(sub.ID)

	if sub.ID == "" {
		t.Error("Expected non-empty subscription ID")
	}
	if sub.Channel != "test-channel" {
		t.Errorf("Expected channel 'test-channel', got %s", sub.Channel)
	}
	if sub.IsPattern {
		t.Error("Expected IsPattern to be false")
	}
	if sub.Buffer.capacity != 100 {
		t.Errorf("Expected buffer capacity 100, got %d", sub.Buffer.capacity)
	}
}

// TestSubscriptionManager_GetSubscription tests retrieving a subscription
func TestSubscriptionManager_GetSubscription(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	manager := NewSubscriptionManager()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub, _ := manager.Subscribe(client, "test-channel", false, 100, 3600)
	defer manager.Unsubscribe(sub.ID)

	// Get the subscription
	retrieved, err := manager.GetSubscription(sub.ID)
	if err != nil {
		t.Fatalf("Failed to get subscription: %v", err)
	}

	if retrieved.ID != sub.ID {
		t.Errorf("Expected subscription ID %s, got %s", sub.ID, retrieved.ID)
	}
}

// TestSubscriptionManager_GetSubscription_NotFound tests getting non-existent subscription
func TestSubscriptionManager_GetSubscription_NotFound(t *testing.T) {
	manager := NewSubscriptionManager()

	_, err := manager.GetSubscription("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent subscription")
	}
}

// TestSubscriptionManager_Unsubscribe tests removing a subscription
func TestSubscriptionManager_Unsubscribe(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	manager := NewSubscriptionManager()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub, _ := manager.Subscribe(client, "test-channel", false, 100, 3600)

	// Add some messages to the buffer
	sub.Buffer.Add("test-channel", "message1", time.Now().Unix())
	sub.Buffer.Add("test-channel", "message2", time.Now().Unix())

	// Unsubscribe
	messagesDropped, err := manager.Unsubscribe(sub.ID)
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	if messagesDropped != 2 {
		t.Errorf("Expected 2 messages dropped, got %d", messagesDropped)
	}

	// Verify subscription is removed
	_, err = manager.GetSubscription(sub.ID)
	if err == nil {
		t.Error("Expected error after unsubscribe")
	}
}

// TestSubscriptionManager_Unsubscribe_NotFound tests unsubscribing non-existent subscription
func TestSubscriptionManager_Unsubscribe_NotFound(t *testing.T) {
	manager := NewSubscriptionManager()

	_, err := manager.Unsubscribe("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent subscription")
	}
}

// TestSubscriptionManager_ListSubscriptions tests listing all subscriptions
func TestSubscriptionManager_ListSubscriptions(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	manager := NewSubscriptionManager()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)

	// Create multiple subscriptions
	sub1, _ := manager.Subscribe(client, "channel1", false, 100, 3600)
	sub2, _ := manager.Subscribe(client, "channel2", false, 100, 3600)
	defer manager.Unsubscribe(sub1.ID)
	defer manager.Unsubscribe(sub2.ID)

	// List subscriptions
	subs := manager.ListSubscriptions()

	if len(subs) != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", len(subs))
	}
}

// TestSubscription_IsExpired tests subscription expiration
func TestSubscription_IsExpired(t *testing.T) {
	sub := NewSubscription("test-id", "test-channel", false, 100, 1)

	// Should not be expired immediately
	if sub.IsExpired() {
		t.Error("Expected subscription not to be expired")
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	if !sub.IsExpired() {
		t.Error("Expected subscription to be expired")
	}
}

// TestSubscription_Extend tests extending subscription TTL
func TestSubscription_Extend(t *testing.T) {
	sub := NewSubscription("test-id", "test-channel", false, 100, 1)

	// Extend TTL
	sub.Extend(3600)

	// Should not be expired
	if sub.IsExpired() {
		t.Error("Expected subscription not to be expired after extension")
	}
}

// TestSubscription_StartStop tests starting and stopping a subscription
func TestSubscription_StartStop(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)

	sub := NewSubscription("test-id", "test-channel", false, 100, 3600)

	// Start subscription
	err := sub.Start(client)
	if err != nil {
		t.Fatalf("Failed to start subscription: %v", err)
	}

	// Try to start again (should fail)
	err = sub.Start(client)
	if err == nil {
		t.Error("Expected error when starting already running subscription")
	}

	// Stop subscription
	err = sub.Stop()
	if err != nil {
		t.Fatalf("Failed to stop subscription: %v", err)
	}

	// Stop again (should not error)
	err = sub.Stop()
	if err != nil {
		t.Errorf("Expected no error when stopping already stopped subscription, got: %v", err)
	}
}

// TestSubscription_ReceiveMessages tests receiving messages
func TestSubscription_ReceiveMessages(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub := NewSubscription("test-id", "test-channel", false, 100, 3600)

	err := sub.Start(client)
	if err != nil {
		t.Fatalf("Failed to start subscription: %v", err)
	}
	defer sub.Stop()

	// Publish a message using miniredis
	s.Publish("test-channel", "test-message")

	// Wait for message to be received
	time.Sleep(100 * time.Millisecond)

	// Check buffer
	messages, _ := sub.Buffer.Get(10, false)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message in buffer, got %d", len(messages))
	}
	if len(messages) > 0 && messages[0].Payload != "test-message" {
		t.Errorf("Expected payload 'test-message', got %s", messages[0].Payload)
	}
}

// ============================================================================
// Phase 2: JSON-RPC Methods Tests
// ============================================================================

// TestHandlePubSubSubscribe tests the pubsub_subscribe handler
func TestHandlePubSubSubscribe(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	req := PubSubSubscribeRequest{
		Params:     params,
		Channel:    "test-channel",
		IsPattern:  false,
		BufferSize: 500,
		TTL:        7200,
	}

	resp, err := HandlePubSubSubscribe(req)
	if err != nil {
		t.Fatalf("HandlePubSubSubscribe failed: %v", err)
	}
	defer globalSubscriptionManager.Unsubscribe(resp.SubscriptionID)

	if resp.SubscriptionID == "" {
		t.Error("Expected non-empty subscription ID")
	}
	if resp.Channel != "test-channel" {
		t.Errorf("Expected channel 'test-channel', got %s", resp.Channel)
	}
	if resp.IsPattern {
		t.Error("Expected IsPattern to be false")
	}
	if resp.MaxBuffer != 500 {
		t.Errorf("Expected max buffer 500, got %d", resp.MaxBuffer)
	}
	if resp.TTL != 7200 {
		t.Errorf("Expected TTL 7200, got %d", resp.TTL)
	}
}

// TestHandlePubSubSubscribe_EmptyChannel tests error handling for empty channel
func TestHandlePubSubSubscribe_EmptyChannel(t *testing.T) {
	req := PubSubSubscribeRequest{
		Channel: "",
	}

	_, err := HandlePubSubSubscribe(req)
	if err == nil {
		t.Error("Expected error for empty channel")
	}
}

// TestHandlePubSubSubscribe_Defaults tests default values
func TestHandlePubSubSubscribe_Defaults(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	req := PubSubSubscribeRequest{
		Params:  params,
		Channel: "test-channel",
		// BufferSize and TTL not set
	}

	resp, err := HandlePubSubSubscribe(req)
	if err != nil {
		t.Fatalf("HandlePubSubSubscribe failed: %v", err)
	}
	defer globalSubscriptionManager.Unsubscribe(resp.SubscriptionID)

	if resp.MaxBuffer != 1000 {
		t.Errorf("Expected default max buffer 1000, got %d", resp.MaxBuffer)
	}
	if resp.TTL != 3600 {
		t.Errorf("Expected default TTL 3600, got %d", resp.TTL)
	}
}

// TestHandlePubSubUnsubscribe tests the pubsub_unsubscribe handler
func TestHandlePubSubUnsubscribe(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	// Create a subscription first
	client, _ := getClient(params)
	sub, _ := globalSubscriptionManager.Subscribe(client, "test-channel", false, 100, 3600)

	// Add some messages
	sub.Buffer.Add("test-channel", "message1", time.Now().Unix())
	sub.Buffer.Add("test-channel", "message2", time.Now().Unix())

	// Unsubscribe
	req := PubSubUnsubscribeRequest{
		Params:         params,
		SubscriptionID: sub.ID,
	}

	resp, err := HandlePubSubUnsubscribe(req)
	if err != nil {
		t.Fatalf("HandlePubSubUnsubscribe failed: %v", err)
	}

	if !resp.Success {
		t.Error("Expected success to be true")
	}
	if resp.SubscriptionID != sub.ID {
		t.Errorf("Expected subscription ID %s, got %s", sub.ID, resp.SubscriptionID)
	}
	if resp.MessagesDropped != 2 {
		t.Errorf("Expected 2 messages dropped, got %d", resp.MessagesDropped)
	}
}

// TestHandlePubSubUnsubscribe_EmptySubscriptionID tests error handling
func TestHandlePubSubUnsubscribe_EmptySubscriptionID(t *testing.T) {
	req := PubSubUnsubscribeRequest{
		SubscriptionID: "",
	}

	_, err := HandlePubSubUnsubscribe(req)
	if err == nil {
		t.Error("Expected error for empty subscription ID")
	}
}

// TestHandlePubSubUnsubscribe_InvalidSubscriptionID tests error handling
func TestHandlePubSubUnsubscribe_InvalidSubscriptionID(t *testing.T) {
	req := PubSubUnsubscribeRequest{
		SubscriptionID: "invalid-id",
	}

	_, err := HandlePubSubUnsubscribe(req)
	if err == nil {
		t.Error("Expected error for invalid subscription ID")
	}
}

// TestHandlePubSubPublish tests the pubsub_publish handler
func TestHandlePubSubPublish(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	req := PubSubPublishRequest{
		Params:  params,
		Channel: "test-channel",
		Message: "test-message",
	}

	resp, err := HandlePubSubPublish(req)
	if err != nil {
		t.Fatalf("HandlePubSubPublish failed: %v", err)
	}

	if !resp.Success {
		t.Error("Expected success to be true")
	}
	if resp.Channel != "test-channel" {
		t.Errorf("Expected channel 'test-channel', got %s", resp.Channel)
	}
	// Receivers will be 0 since no one is subscribed
	if resp.Receivers < 0 {
		t.Errorf("Expected non-negative receivers, got %d", resp.Receivers)
	}
}

// TestHandlePubSubPublish_EmptyChannel tests error handling
func TestHandlePubSubPublish_EmptyChannel(t *testing.T) {
	req := PubSubPublishRequest{
		Channel: "",
		Message: "test",
	}

	_, err := HandlePubSubPublish(req)
	if err == nil {
		t.Error("Expected error for empty channel")
	}
}

// TestHandlePubSubPublish_EmptyMessage tests error handling
func TestHandlePubSubPublish_EmptyMessage(t *testing.T) {
	req := PubSubPublishRequest{
		Channel: "test-channel",
		Message: "",
	}

	_, err := HandlePubSubPublish(req)
	if err == nil {
		t.Error("Expected error for empty message")
	}
}

// TestHandlePubSubPollMessages tests the pubsub_poll_messages handler
func TestHandlePubSubPollMessages(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	// Create a subscription
	client, _ := getClient(params)
	sub, _ := globalSubscriptionManager.Subscribe(client, "test-channel", false, 100, 3600)
	defer globalSubscriptionManager.Unsubscribe(sub.ID)

	// Add messages to buffer
	sub.Buffer.Add("test-channel", "message1", time.Now().Unix())
	sub.Buffer.Add("test-channel", "message2", time.Now().Unix())
	sub.Buffer.Add("test-channel", "message3", time.Now().Unix())

	// Poll messages
	req := PubSubPollMessagesRequest{
		Params:          params,
		SubscriptionID:  sub.ID,
		MaxMessages:     2,
		AutoAcknowledge: false,
	}

	resp, err := HandlePubSubPollMessages(req)
	if err != nil {
		t.Fatalf("HandlePubSubPollMessages failed: %v", err)
	}

	if len(resp.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(resp.Messages))
	}
	if !resp.MoreAvailable {
		t.Error("Expected more messages to be available")
	}
	if resp.SubscriptionID != sub.ID {
		t.Errorf("Expected subscription ID %s, got %s", sub.ID, resp.SubscriptionID)
	}
	if resp.BufferSize != 3 {
		t.Errorf("Expected buffer size 3, got %d", resp.BufferSize)
	}
}

// TestHandlePubSubPollMessages_AutoAck tests auto-acknowledgment
func TestHandlePubSubPollMessages_AutoAck(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub, _ := globalSubscriptionManager.Subscribe(client, "test-channel", false, 100, 3600)
	defer globalSubscriptionManager.Unsubscribe(sub.ID)

	sub.Buffer.Add("test-channel", "message1", time.Now().Unix())
	sub.Buffer.Add("test-channel", "message2", time.Now().Unix())

	req := PubSubPollMessagesRequest{
		Params:          params,
		SubscriptionID:  sub.ID,
		MaxMessages:     2,
		AutoAcknowledge: true,
	}

	resp, err := HandlePubSubPollMessages(req)
	if err != nil {
		t.Fatalf("HandlePubSubPollMessages failed: %v", err)
	}

	if len(resp.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(resp.Messages))
	}
	if resp.BufferSize != 0 {
		t.Errorf("Expected buffer size 0 after auto-ack, got %d", resp.BufferSize)
	}
}

// TestHandlePubSubPollMessages_EmptySubscriptionID tests error handling
func TestHandlePubSubPollMessages_EmptySubscriptionID(t *testing.T) {
	req := PubSubPollMessagesRequest{
		SubscriptionID: "",
	}

	_, err := HandlePubSubPollMessages(req)
	if err == nil {
		t.Error("Expected error for empty subscription ID")
	}
}

// TestHandlePubSubPollMessages_InvalidSubscriptionID tests error handling
func TestHandlePubSubPollMessages_InvalidSubscriptionID(t *testing.T) {
	req := PubSubPollMessagesRequest{
		SubscriptionID: "invalid-id",
	}

	_, err := HandlePubSubPollMessages(req)
	if err == nil {
		t.Error("Expected error for invalid subscription ID")
	}
}

// TestHandlePubSubPollMessages_DefaultMaxMessages tests default max messages
func TestHandlePubSubPollMessages_DefaultMaxMessages(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub, _ := globalSubscriptionManager.Subscribe(client, "test-channel", false, 100, 3600)
	defer globalSubscriptionManager.Unsubscribe(sub.ID)

	// Add many messages
	for i := 0; i < 150; i++ {
		sub.Buffer.Add("test-channel", "message", time.Now().Unix())
	}

	req := PubSubPollMessagesRequest{
		Params:         params,
		SubscriptionID: sub.ID,
		MaxMessages:    0, // Should default to 100
	}

	resp, err := HandlePubSubPollMessages(req)
	if err != nil {
		t.Fatalf("HandlePubSubPollMessages failed: %v", err)
	}

	if len(resp.Messages) != 100 {
		t.Errorf("Expected 100 messages (default), got %d", len(resp.Messages))
	}
}

// TestHandlePubSubAcknowledgeMessages tests the pubsub_acknowledge_messages handler
func TestHandlePubSubAcknowledgeMessages(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	// Create a subscription
	client, err := getClient(params)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	sub, err := globalSubscriptionManager.Subscribe(client, "test-channel", false, 100, 3600)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	defer globalSubscriptionManager.Unsubscribe(sub.ID)

	// Add some messages to the buffer manually
	msg1 := sub.Buffer.Add("test-channel", "message1", time.Now().Unix())
	msg2 := sub.Buffer.Add("test-channel", "message2", time.Now().Unix())
	msg3 := sub.Buffer.Add("test-channel", "message3", time.Now().Unix())

	// Verify initial buffer size
	if sub.Buffer.Size() != 3 {
		t.Errorf("Expected buffer size 3, got %d", sub.Buffer.Size())
	}

	// Test acknowledging messages
	req := PubSubAcknowledgeMessagesRequest{
		Params:         params,
		SubscriptionID: sub.ID,
		MessageIDs:     []int64{msg1.ID, msg2.ID},
	}

	resp, err := HandlePubSubAcknowledgeMessages(req)
	if err != nil {
		t.Fatalf("HandlePubSubAcknowledgeMessages failed: %v", err)
	}

	if !resp.Success {
		t.Error("Expected success to be true")
	}

	if resp.SubscriptionID != sub.ID {
		t.Errorf("Expected subscription ID %s, got %s", sub.ID, resp.SubscriptionID)
	}

	if resp.MessagesAcknowledged != 2 {
		t.Errorf("Expected 2 messages acknowledged, got %d", resp.MessagesAcknowledged)
	}

	// Verify buffer size decreased
	if sub.Buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1 after acknowledgment, got %d", sub.Buffer.Size())
	}

	// Acknowledge the last message
	req.MessageIDs = []int64{msg3.ID}
	resp, err = HandlePubSubAcknowledgeMessages(req)
	if err != nil {
		t.Fatalf("HandlePubSubAcknowledgeMessages failed: %v", err)
	}

	if resp.MessagesAcknowledged != 1 {
		t.Errorf("Expected 1 message acknowledged, got %d", resp.MessagesAcknowledged)
	}

	// Verify buffer is now empty
	if sub.Buffer.Size() != 0 {
		t.Errorf("Expected buffer size 0, got %d", sub.Buffer.Size())
	}
}

// TestHandlePubSubAcknowledgeMessages_EmptySubscriptionID tests error handling
func TestHandlePubSubAcknowledgeMessages_EmptySubscriptionID(t *testing.T) {
	req := PubSubAcknowledgeMessagesRequest{
		SubscriptionID: "",
		MessageIDs:     []int64{1, 2, 3},
	}

	_, err := HandlePubSubAcknowledgeMessages(req)
	if err == nil {
		t.Error("Expected error for empty subscription ID")
	}

	if err.Error() != "subscription_id is required" {
		t.Errorf("Expected 'subscription_id is required' error, got: %v", err)
	}
}

// TestHandlePubSubAcknowledgeMessages_EmptyMessageIDs tests error handling
func TestHandlePubSubAcknowledgeMessages_EmptyMessageIDs(t *testing.T) {
	req := PubSubAcknowledgeMessagesRequest{
		SubscriptionID: "sub_123",
		MessageIDs:     []int64{},
	}

	_, err := HandlePubSubAcknowledgeMessages(req)
	if err == nil {
		t.Error("Expected error for empty message IDs")
	}

	if err.Error() != "message_ids is required and must not be empty" {
		t.Errorf("Expected 'message_ids is required and must not be empty' error, got: %v", err)
	}
}

// TestHandlePubSubAcknowledgeMessages_InvalidSubscriptionID tests error handling
func TestHandlePubSubAcknowledgeMessages_InvalidSubscriptionID(t *testing.T) {
	req := PubSubAcknowledgeMessagesRequest{
		SubscriptionID: "invalid_sub_id",
		MessageIDs:     []int64{1, 2, 3},
	}

	_, err := HandlePubSubAcknowledgeMessages(req)
	if err == nil {
		t.Error("Expected error for invalid subscription ID")
	}

	// The error should contain "subscription not found"
	if err.Error() != "failed to get subscription: subscription not found: invalid_sub_id" {
		t.Errorf("Expected 'subscription not found' error, got: %v", err)
	}
}

// ============================================================================
// Phase 3: Virtual Tables Tests
// ============================================================================

// TestExecuteScanPubSubChannels tests the pubsub_channels virtual table
func TestExecuteScanPubSubChannels(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	// Create a real Redis client and subscribe to create active channels
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})
	defer client.Close()

	// Subscribe to a channel to make it active
	pubsub := client.Subscribe(context.Background(), "test-channel")
	defer pubsub.Close()

	// Wait for subscription to be active
	_, err := pubsub.Receive(context.Background())
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Execute query
	parser := parseQuery("SELECT * FROM pubsub_channels")
	result := executeScanPubSubChannels(params, parser, 1, 50)

	// Verify result structure
	if result["columns"] == nil {
		t.Error("Expected columns in result")
	}

	columns := result["columns"].([]string)
	expectedColumns := []string{"channel", "subscribers", "is_pattern", "last_message_time"}
	if len(columns) != len(expectedColumns) {
		t.Errorf("Expected %d columns, got %d", len(expectedColumns), len(columns))
	}

	rows := result["rows"].([][]interface{})
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}

	if len(rows) > 0 {
		if rows[0][0] != "test-channel" {
			t.Errorf("Expected channel 'test-channel', got %v", rows[0][0])
		}
	}
}

// TestExecuteScanPubSubChannels_WithConditions tests filtering
func TestExecuteScanPubSubChannels_WithConditions(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	// Create subscriptions
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})
	defer client.Close()

	pubsub1 := client.Subscribe(context.Background(), "channel1")
	defer pubsub1.Close()
	pubsub1.Receive(context.Background())

	pubsub2 := client.Subscribe(context.Background(), "channel2")
	defer pubsub2.Close()
	pubsub2.Receive(context.Background())

	// Query with condition
	parser := parseQuery("SELECT * FROM pubsub_channels WHERE channel = 'channel1'")
	result := executeScanPubSubChannels(params, parser, 1, 50)

	rows := result["rows"].([][]interface{})
	if len(rows) != 1 {
		t.Errorf("Expected 1 row after filtering, got %d", len(rows))
	}
}

// TestExecuteScanPubSubMessages tests the pubsub_messages virtual table
func TestExecuteScanPubSubMessages(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	// Create a subscription and add messages
	client, _ := getClient(params)
	sub, _ := globalSubscriptionManager.Subscribe(client, "test-channel", false, 100, 3600)
	defer globalSubscriptionManager.Unsubscribe(sub.ID)

	sub.Buffer.Add("test-channel", "message1", time.Now().Unix())
	sub.Buffer.Add("test-channel", "message2", time.Now().Unix())

	// Execute query
	parser := parseQuery("SELECT * FROM pubsub_messages")
	result := executeScanPubSubMessages(params, parser, 1, 50)

	// Verify result structure
	columns := result["columns"].([]string)
	expectedColumns := []string{"subscription_id", "message_id", "channel", "payload", "published_at", "received_at"}
	if len(columns) != len(expectedColumns) {
		t.Errorf("Expected %d columns, got %d", len(expectedColumns), len(columns))
	}

	rows := result["rows"].([][]interface{})
	if len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rows))
	}

	if len(rows) > 0 {
		if rows[0][0] != sub.ID {
			t.Errorf("Expected subscription ID %s, got %v", sub.ID, rows[0][0])
		}
		if rows[0][2] != "test-channel" {
			t.Errorf("Expected channel 'test-channel', got %v", rows[0][2])
		}
	}
}

// TestExecuteScanPubSubMessages_WithSubscriptionFilter tests filtering by subscription ID
func TestExecuteScanPubSubMessages_WithSubscriptionFilter(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub1, _ := globalSubscriptionManager.Subscribe(client, "channel1", false, 100, 3600)
	sub2, _ := globalSubscriptionManager.Subscribe(client, "channel2", false, 100, 3600)
	defer globalSubscriptionManager.Unsubscribe(sub1.ID)
	defer globalSubscriptionManager.Unsubscribe(sub2.ID)

	sub1.Buffer.Add("channel1", "message1", time.Now().Unix())
	sub2.Buffer.Add("channel2", "message2", time.Now().Unix())

	// Query with subscription filter
	parser := parseQuery("SELECT * FROM pubsub_messages WHERE subscription_id = '" + sub1.ID + "'")
	result := executeScanPubSubMessages(params, parser, 1, 50)

	rows := result["rows"].([][]interface{})
	if len(rows) != 1 {
		t.Errorf("Expected 1 row after filtering, got %d", len(rows))
	}

	if len(rows) > 0 && rows[0][0] != sub1.ID {
		t.Errorf("Expected subscription ID %s, got %v", sub1.ID, rows[0][0])
	}
}

// TestExecuteScanPubSubSubscriptions tests the pubsub_subscriptions virtual table
func TestExecuteScanPubSubSubscriptions(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	// Create subscriptions
	client, _ := getClient(params)
	sub1, _ := globalSubscriptionManager.Subscribe(client, "channel1", false, 100, 3600)
	sub2, _ := globalSubscriptionManager.Subscribe(client, "channel2", true, 200, 7200)
	defer globalSubscriptionManager.Unsubscribe(sub1.ID)
	defer globalSubscriptionManager.Unsubscribe(sub2.ID)

	// Add messages to buffers
	sub1.Buffer.Add("channel1", "message1", time.Now().Unix())
	sub2.Buffer.Add("channel2", "message2", time.Now().Unix())
	sub2.Buffer.Add("channel2", "message3", time.Now().Unix())

	// Execute query
	parser := parseQuery("SELECT * FROM pubsub_subscriptions")
	result := executeScanPubSubSubscriptions(params, parser, 1, 50)

	// Verify result structure
	columns := result["columns"].([]string)
	expectedColumns := []string{"id", "channel", "is_pattern", "created_at", "ttl", "buffer_size", "buffer_used", "messages_received", "messages_dropped"}
	if len(columns) != len(expectedColumns) {
		t.Errorf("Expected %d columns, got %d", len(expectedColumns), len(columns))
	}

	rows := result["rows"].([][]interface{})
	if len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rows))
	}

	// Verify subscription details
	for _, row := range rows {
		id := row[0].(string)
		channel := row[1].(string)
		isPattern := row[2].(bool)
		bufferUsed := row[6].(int64)

		if id == sub1.ID {
			if channel != "channel1" {
				t.Errorf("Expected channel 'channel1', got %s", channel)
			}
			if isPattern {
				t.Error("Expected is_pattern to be false for sub1")
			}
			if bufferUsed != 1 {
				t.Errorf("Expected buffer_used 1 for sub1, got %d", bufferUsed)
			}
		} else if id == sub2.ID {
			if channel != "channel2" {
				t.Errorf("Expected channel 'channel2', got %s", channel)
			}
			if !isPattern {
				t.Error("Expected is_pattern to be true for sub2")
			}
			if bufferUsed != 2 {
				t.Errorf("Expected buffer_used 2 for sub2, got %d", bufferUsed)
			}
		}
	}
}

// TestExecuteScanPubSubSubscriptions_WithConditions tests filtering
func TestExecuteScanPubSubSubscriptions_WithConditions(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub1, _ := globalSubscriptionManager.Subscribe(client, "channel1", false, 100, 3600)
	sub2, _ := globalSubscriptionManager.Subscribe(client, "channel2", true, 200, 7200)
	defer globalSubscriptionManager.Unsubscribe(sub1.ID)
	defer globalSubscriptionManager.Unsubscribe(sub2.ID)

	// Query with condition
	parser := parseQuery("SELECT * FROM pubsub_subscriptions WHERE is_pattern = true")
	result := executeScanPubSubSubscriptions(params, parser, 1, 50)

	rows := result["rows"].([][]interface{})
	if len(rows) != 1 {
		t.Errorf("Expected 1 row after filtering, got %d", len(rows))
	}

	if len(rows) > 0 && rows[0][0] != sub2.ID {
		t.Errorf("Expected subscription ID %s, got %v", sub2.ID, rows[0][0])
	}
}

// TestExecuteScanPubSubSubscriptions_OrderBy tests sorting
func TestExecuteScanPubSubSubscriptions_OrderBy(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)
	sub1, _ := globalSubscriptionManager.Subscribe(client, "zzz-channel", false, 100, 3600)
	sub2, _ := globalSubscriptionManager.Subscribe(client, "aaa-channel", false, 200, 7200)
	defer globalSubscriptionManager.Unsubscribe(sub1.ID)
	defer globalSubscriptionManager.Unsubscribe(sub2.ID)

	// Query with ORDER BY
	parser := parseQuery("SELECT * FROM pubsub_subscriptions ORDER BY channel ASC")
	result := executeScanPubSubSubscriptions(params, parser, 1, 50)

	rows := result["rows"].([][]interface{})
	if len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rows))
	}

	if len(rows) == 2 {
		if rows[0][1] != "aaa-channel" {
			t.Errorf("Expected first row channel 'aaa-channel', got %v", rows[0][1])
		}
		if rows[1][1] != "zzz-channel" {
			t.Errorf("Expected second row channel 'zzz-channel', got %v", rows[1][1])
		}
	}
}

// TestExecuteScanPubSubSubscriptions_Pagination tests pagination
func TestExecuteScanPubSubSubscriptions_Pagination(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	host := s.Host()
	port := s.Server().Addr().Port
	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}

	client, _ := getClient(params)

	// Create multiple subscriptions
	var subIDs []string
	for i := 0; i < 5; i++ {
		sub, _ := globalSubscriptionManager.Subscribe(client, "channel", false, 100, 3600)
		subIDs = append(subIDs, sub.ID)
	}
	defer func() {
		for _, id := range subIDs {
			globalSubscriptionManager.Unsubscribe(id)
		}
	}()

	// Query with pagination
	parser := parseQuery("SELECT * FROM pubsub_subscriptions")
	result := executeScanPubSubSubscriptions(params, parser, 1, 2)

	rows := result["rows"].([][]interface{})
	if len(rows) != 2 {
		t.Errorf("Expected 2 rows (page size), got %d", len(rows))
	}

	pagination := result["pagination"].(map[string]interface{})
	if pagination["has_more"] != true {
		t.Error("Expected has_more to be true")
	}
	if pagination["total_rows"] != 5 {
		t.Errorf("Expected total_rows 5, got %v", pagination["total_rows"])
	}
}
