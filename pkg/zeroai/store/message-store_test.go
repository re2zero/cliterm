// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageStore_AddMessage(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	msg := &Message{
		SessionID: "test-session-msg-1",
		Role:      "user",
		Content:   "Hello, world!",
		EventType: "text",
		Metadata:  `{"key":"value"}`,
	}

	err = store.Add(msg)
	require.NoError(t, err)
	assert.Greater(t, msg.ID, int64(0))
	assert.Greater(t, msg.CreatedAt, int64(0))
}

func TestMessageStore_GetSessionMessages(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	sessionID := "test-session-get-1"

	// Add multiple messages
	msg1 := &Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   "First message",
	}
	msg2 := &Message{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "Second message",
		EventType: "text_chunk",
	}
	msg3 := &Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   "Third message",
	}

	err = store.Add(msg1)
	require.NoError(t, err)
	err = store.Add(msg2)
	require.NoError(t, err)
	err = store.Add(msg3)
	require.NoError(t, err)

	// Retrieve messages
	messages, err := store.GetSessionMessages(sessionID)
	require.NoError(t, err)
	assert.Len(t, messages, 3)

	// Verify order (should be in created_at ascending order)
	assert.Equal(t, "First message", messages[0].Content)
	assert.Equal(t, "Second message", messages[1].Content)
	assert.Equal(t, "Third message", messages[2].Content)

	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "user", messages[2].Role)

	assert.Equal(t, "text_chunk", messages[1].EventType)

	// Cleanup
	_ = store.Delete(sessionID)
}

func TestMessageStore_GetEmptySession(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	messages, err := store.GetSessionMessages("non-existent-session")
	require.NoError(t, err)
	assert.Len(t, messages, 0)
}

func TestMessageStore_DeleteSession(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	sessionID := "test-session-delete-1"

	// Add messages
	msg1 := &Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   "Message to delete",
	}
	msg2 := &Message{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "Another message to delete",
	}

	err = store.Add(msg1)
	require.NoError(t, err)
	err = store.Add(msg2)
	require.NoError(t, err)

	// Verify messages exist
	messages, _ := store.GetSessionMessages(sessionID)
	assert.Len(t, messages, 2)

	// Delete all messages for session
	err = store.Delete(sessionID)
	require.NoError(t, err)

	// Verify messages are deleted
	messages, err = store.GetSessionMessages(sessionID)
	require.NoError(t, err)
	assert.Len(t, messages, 0)
}

func TestMessageStore_MultipleSessions(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	// Add messages to different sessions
	session1 := "session-a-1"
	session2 := "session-b-1"

	msg1 := &Message{
		SessionID: session1,
		Role:      "user",
		Content:   "Message for session A",
	}
	msg2 := &Message{
		SessionID: session2,
		Role:      "user",
		Content:   "Message for session B",
	}

	err = store.Add(msg1)
	require.NoError(t, err)
	err = store.Add(msg2)
	require.NoError(t, err)

	// Verify sessions are isolated
	messagesA, _ := store.GetSessionMessages(session1)
	messagesB, _ := store.GetSessionMessages(session2)

	assert.Len(t, messagesA, 1)
	assert.Len(t, messagesB, 1)
	assert.Equal(t, "Message for session A", messagesA[0].Content)
	assert.Equal(t, "Message for session B", messagesB[0].Content)

	// Delete only session A
	_ = store.Delete(session1)

	messagesA, _ = store.GetSessionMessages(session1)
	messagesB, _ = store.GetSessionMessages(session2)

	assert.Len(t, messagesA, 0)
	assert.Len(t, messagesB, 1)

	// Cleanup
	_ = store.Delete(session2)
}

func TestMessageStore_Timestamps(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	beforeCreate := time.Now().Unix()

	msg := &Message{
		SessionID: "test-session-ts-1",
		Role:      "user",
		Content:   "Timestamp test",
	}

	err = store.Add(msg)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, msg.CreatedAt, beforeCreate)
	assert.LessOrEqual(t, msg.CreatedAt, time.Now().Unix())

	// Retrieve and verify
	messages, err := store.GetSessionMessages(msg.SessionID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, msg.CreatedAt, messages[0].CreatedAt)

	// Cleanup
	_ = store.Delete(msg.SessionID)
}

func TestMessageStore_Metadata(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	sessionID := "test-session-meta-1"

	metadata1 := `{"type":"code","language":"go"}`
	metadata2 := `{"type":"text"}`

	msg1 := &Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   "Code message",
		Metadata:  metadata1,
	}
	msg2 := &Message{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "Text message",
		Metadata:  metadata2,
	}

	err = store.Add(msg1)
	require.NoError(t, err)
	err = store.Add(msg2)
	require.NoError(t, err)

	// Retrieve and verify metadata
	messages, err := store.GetSessionMessages(sessionID)
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, metadata1, messages[0].Metadata)
	assert.Equal(t, metadata2, messages[1].Metadata)

	// Cleanup
	_ = store.Delete(sessionID)
}

func TestMessageStore_EventTypes(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	sessionID := "test-session-event-1"

	events := []struct {
		eventType string
		content   string
	}{
		{"text", "Normal text"},
		{"text_chunk", "Streaming chunk"},
		{"tool_call", `{"tool":"test"}`},
		{"permission", `{"permission":"execute"}`},
		{"error", "Error message"},
		{"end_turn", ""},
	}

	for i, event := range events {
		msg := &Message{
			SessionID: sessionID,
			Role:      "assistant",
			Content:   event.content,
			EventType: event.eventType,
		}
		err = store.Add(msg)
		require.NoError(t, err)
		assert.Greater(t, msg.ID, int64(i))
	}

	// Retrieve all
	messages, err := store.GetSessionMessages(sessionID)
	require.NoError(t, err)
	assert.Len(t, messages, len(events))

	for i, event := range events {
		assert.Equal(t, event.eventType, messages[i].EventType)
		assert.Equal(t, event.content, messages[i].Content)
	}

	// Cleanup
	_ = store.Delete(sessionID)
}

func TestMessageStore_Roles(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	sessionID := "test-session-roles-1"

	roles := []string{"user", "assistant", "system", "tool"}

	for i, role := range roles {
		msg := &Message{
			SessionID: sessionID,
			Role:      role,
			Content:   "Message from " + role,
		}
		err = store.Add(msg)
		require.NoError(t, err)
		assert.Greater(t, msg.ID, int64(i))
	}

	// Retrieve and verify roles
	messages, err := store.GetSessionMessages(sessionID)
	require.NoError(t, err)
	assert.Len(t, messages, len(roles))

	for i, role := range roles {
		assert.Equal(t, role, messages[i].Role)
	}

	// Cleanup
	_ = store.Delete(sessionID)
}

func TestMessageStore_LargeContent(t *testing.T) {
	store, err := NewMessageStore()
	require.NoError(t, err)

	sessionID := "test-session-large-1"

	// Create a large content (10KB)
	largeContent := make([]byte, 10240)
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}

	msg := &Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   string(largeContent),
	}

	err = store.Add(msg)
	require.NoError(t, err)

	// Retrieve and verify
	messages, err := store.GetSessionMessages(sessionID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, len(largeContent), len(messages[0].Content))
	assert.Equal(t, string(largeContent), messages[0].Content)

	// Cleanup
	_ = store.Delete(sessionID)
}
