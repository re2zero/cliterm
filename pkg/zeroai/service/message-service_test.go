// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavetermdev/waveterm/pkg/zeroai/store"
)

// mockMessageStore is a mock implementation of store.MessageStore for testing
type mockMessageStore struct {
	messages map[string][]*store.Message
}

func newMockMessageStore() *mockMessageStore {
	return &mockMessageStore{
		messages: make(map[string][]*store.Message),
	}
}

func (m *mockMessageStore) Add(msg *store.Message) error {
	if msg.CreatedAt == 0 {
		msg.CreatedAt = time.Now().Unix()
	}
	msg.ID = int64(len(m.messages[msg.SessionID]) + 1)
	m.messages[msg.SessionID] = append(m.messages[msg.SessionID], msg)
	return nil
}

func (m *mockMessageStore) GetSessionMessages(sessionID string) ([]*store.Message, error) {
	messages, exists := m.messages[sessionID]
	if !exists {
		return []*store.Message{}, nil
	}
	return messages, nil
}

func (m *mockMessageStore) Delete(sessionID string) error {
	if _, exists := m.messages[sessionID]; !exists {
		return errors.New("session not found")
	}
	delete(m.messages, sessionID)
	return nil
}

func TestMessageService_NewMessageService(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)
	require.NotNil(t, svc)

	// Test with nil store
	_, err = NewMessageService(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message store is required")
}

func TestMessageService_AddMessage(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)

	msg := &store.Message{
		SessionID: "test-session-1",
		Role:      "user",
		Content:   "Hello, world!",
	}

	err = svc.AddMessage(msg)
	require.NoError(t, err)
	assert.Greater(t, msg.ID, int64(0))

	// Verify message was added
	messages, err := mockStore.GetSessionMessages("test-session-1")
	require.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, "Hello, world!", messages[0].Content)
	assert.Equal(t, "user", messages[0].Role)
}

func TestMessageService_AddMessageValidation(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)

	// Test with nil message
	err = svc.AddMessage(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message is required")

	// Test with empty session ID
	msg := &store.Message{
		Role:    "user",
		Content: "test",
	}
	err = svc.AddMessage(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session ID is required")

	// Test with empty role
	msg = &store.Message{
		SessionID: "test-session",
		Content:   "test",
	}
	err = svc.AddMessage(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role is required")
}

func TestMessageService_GetSessionMessages(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)

	sessionID := "test-session-2"

	// Initially empty
	messages, err := svc.GetSessionMessages(sessionID)
	require.NoError(t, err)
	assert.Len(t, messages, 0)

	// Add some messages
	msg1 := &store.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   "First message",
	}
	msg2 := &store.Message{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "Second message",
	}

	err = svc.AddMessage(msg1)
	require.NoError(t, err)
	err = svc.AddMessage(msg2)
	require.NoError(t, err)

	// Retrieve messages
	messages, err = svc.GetSessionMessages(sessionID)
	require.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "First message", messages[0].Content)
	assert.Equal(t, "Second message", messages[1].Content)

	// Test with empty session ID
	_, err = svc.GetSessionMessages("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session ID is required")
}

func TestMessageService_DeleteSessionMessages(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)

	sessionID := "test-session-3"

	// Add messages
	msg1 := &store.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   "Message 1",
	}
	msg2 := &store.Message{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "Message 2",
	}

	err = svc.AddMessage(msg1)
	require.NoError(t, err)
	err = svc.AddMessage(msg2)
	require.NoError(t, err)

	// Verify messages exist
	messages, _ := svc.GetSessionMessages(sessionID)
	assert.Len(t, messages, 2)

	// Delete messages
	err = svc.DeleteSessionMessages(sessionID)
	require.NoError(t, err)

	// Verify messages are deleted
	messages, err = svc.GetSessionMessages(sessionID)
	require.NoError(t, err)
	assert.Len(t, messages, 0)

	// Test with empty session ID
	err = svc.DeleteSessionMessages("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session ID is required")
}

func TestMessageService_StreamMessages(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)

	ctx := context.Background()
	sessionID := "test-session-stream"

	// Add some messages first
	msg1 := &store.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   "Before stream",
	}
	_ = svc.AddMessage(msg1)

	// Create stream
	eventCh, err := svc.StreamMessages(ctx, sessionID)
	require.NoError(t, err)

	// Receive initial messages
	receivedCount := 0
	timeout := time.After(100 * time.Millisecond)
	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					done <- true
					return
				}
				if event.Message != nil {
					receivedCount++
				}
				if receivedCount >= 1 {
					done <- true
					return
				}
			case <-timeout:
				done <- true
				return
			}
		}
	}()

	<-done
	assert.Equal(t, 1, receivedCount)

	// Test with empty session ID
	_, err = svc.StreamMessages(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session ID is required")
}

func TestMessageService_AddMessageTimestamp(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)

	msg := &store.Message{
		SessionID: "test-session-ts",
		Role:      "user",
		Content:   "Test",
	}

	// CreatedAt should be set if zero
	assert.Equal(t, int64(0), msg.CreatedAt)

	err = svc.AddMessage(msg)
	require.NoError(t, err)

	assert.Greater(t, msg.CreatedAt, int64(0))
}

func TestMessageService_MultipleSessions(t *testing.T) {
	mockStore := newMockMessageStore()

	svc, err := NewMessageService(mockStore)
	require.NoError(t, err)

	// Add messages to different sessions
	msg1 := &store.Message{
		SessionID: "session-a",
		Role:      "user",
		Content:   "Message A",
	}
	msg2 := &store.Message{
		SessionID: "session-b",
		Role:      "user",
		Content:   "Message B",
	}

	_ = svc.AddMessage(msg1)
	_ = svc.AddMessage(msg2)

	// Verify sessions are isolated
	messagesA, _ := svc.GetSessionMessages("session-a")
	messagesB, _ := svc.GetSessionMessages("session-b")

	assert.Len(t, messagesA, 1)
	assert.Len(t, messagesB, 1)
	assert.Equal(t, "Message A", messagesA[0].Content)
	assert.Equal(t, "Message B", messagesB[0].Content)
}
