// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
)

func TestMain(m *testing.M) {
	// Ensure database directory exists for tests
	dbDir := filepath.Join(wavebase.GetWaveDataDir(), wavebase.WaveDBDir)
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(dbDir)

	os.Exit(code)
}

func TestSessionStore_CreateAndGet(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	session := &Session{
		ID:            "test-session-1",
		Backend:       "acp",
		WorkDir:       "/home/test",
		Model:         "claude-3",
		Provider:      "anthropic",
		ThinkingLevel: "high",
		YoloMode:      false,
	}

	err = store.Create(session)
	require.NoError(t, err)

	retrieved, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrieved.ID)
	assert.Equal(t, session.Backend, retrieved.Backend)
	assert.Equal(t, session.WorkDir, retrieved.WorkDir)
	assert.Equal(t, session.Model, retrieved.Model)
	assert.Equal(t, session.Provider, retrieved.Provider)
	assert.Equal(t, session.ThinkingLevel, retrieved.ThinkingLevel)
	assert.Equal(t, session.YoloMode, retrieved.YoloMode)
	assert.Greater(t, retrieved.CreatedAt, int64(0))
	assert.Greater(t, retrieved.UpdatedAt, int64(0))

	// Cleanup
	_ = store.Delete(session.ID)
}

func TestSessionStore_ListSessions(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	// Create multiple sessions
	session1 := &Session{
		ID:      "test-list-1",
		Backend: "acp",
		WorkDir: "/home/test1",
		Model:   "model-1",
	}
	session2 := &Session{
		ID:      "test-list-2",
		Backend: "acp",
		WorkDir: "/home/test2",
		Model:   "model-2",
	}

	err = store.Create(session1)
	require.NoError(t, err)
	err = store.Create(session2)
	require.NoError(t, err)

	// List all sessions
	sessions, err := store.List(ListOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 2)

	// List with backend filter
	acpSessions, err := store.List(ListOptions{Backend: "acp"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(acpSessions), 2)

	for _, s := range acpSessions {
		assert.Equal(t, "acp", s.Backend)
	}

	// List with limit
	limitedSessions, err := store.List(ListOptions{Limit: 1})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(limitedSessions), 1)

	// Cleanup
	_ = store.Delete(session1.ID)
	_ = store.Delete(session2.ID)
}

func TestSessionStore_DeleteSession(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	session := &Session{
		ID:      "test-delete-1",
		Backend: "acp",
		WorkDir: "/home/test",
		Model:   "test-model",
	}

	err = store.Create(session)
	require.NoError(t, err)

	// Delete the session
	err = store.Delete(session.ID)
	require.NoError(t, err)

	// Verify it's deleted
	_, err = store.Get(session.ID)
	assert.Error(t, err)
}

func TestSessionStore_UpdateSession(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	session := &Session{
		ID:      "test-update-1",
		Backend: "acp",
		WorkDir: "/original/path",
		Model:   "model-v1",
	}

	err = store.Create(session)
	require.NoError(t, err)
	originalUpdatedAt := session.UpdatedAt

	// Wait a bit to ensure timestamp changes
	time.Sleep(10 * time.Millisecond)

	// Update the session
	session.WorkDir = "/new/path"
	session.Model = "model-v2"
	err = store.Update(session)
	require.NoError(t, err)

	// Verify updates
	retrieved, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, "/new/path", retrieved.WorkDir)
	assert.Equal(t, "model-v2", retrieved.Model)
	assert.GreaterOrEqual(t, retrieved.UpdatedAt, originalUpdatedAt)

	// Cleanup
	_ = store.Delete(session.ID)
}

func TestSessionStore_UpdateNonExistent(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	session := &Session{
		ID:      "non-existent",
		Backend: "acp",
	}

	err = store.Update(session)
	assert.Error(t, err)
}

func TestSessionStore_GetNonExistent(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	_, err = store.Get("non-existent-id")
	assert.Error(t, err)
}

func TestSessionStore_DeleteNonExistent(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	err = store.Delete("non-existent-id")
	assert.Error(t, err)
}

func TestSessionStore_Timestamps(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	beforeCreate := time.Now().Unix()

	session := &Session{
		ID:      "test-ts-1",
		Backend: "acp",
		WorkDir: "/home/test",
	}

	err = store.Create(session)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, session.CreatedAt, beforeCreate)
	assert.GreaterOrEqual(t, session.UpdatedAt, beforeCreate)

	// Update and verify UpdatedAt changes
	time.Sleep(10 * time.Millisecond)
	beforeUpdate := time.Now().Unix()

	session.WorkDir = "/updated"
	err = store.Update(session)
	require.NoError(t, err)

	retrieved, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, retrieved.UpdatedAt, beforeUpdate)
	assert.Equal(t, retrieved.CreatedAt, session.CreatedAt)

	// Cleanup
	_ = store.Delete(session.ID)
}

func TestSessionStore_YoloMode(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	// Test with YoloMode true
	session1 := &Session{
		ID:       "test-yolo-true",
		Backend:  "acp",
		WorkDir:  "/home/test",
		YoloMode: true,
	}

	err = store.Create(session1)
	require.NoError(t, err)

	retrieved1, err := store.Get(session1.ID)
	require.NoError(t, err)
	assert.True(t, retrieved1.YoloMode)

	// Test with YoloMode false
	session2 := &Session{
		ID:       "test-yolo-false",
		Backend:  "acp",
		WorkDir:  "/home/test",
		YoloMode: false,
	}

	err = store.Create(session2)
	require.NoError(t, err)

	retrieved2, err := store.Get(session2.ID)
	require.NoError(t, err)
	assert.False(t, retrieved2.YoloMode)

	// Cleanup
	_ = store.Delete(session1.ID)
	_ = store.Delete(session2.ID)
}

func TestSessionStore_Metadata(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	metadata := `{"key":"value","nested":{"field":123}}`

	session := &Session{
		ID:       "test-metadata-1",
		Backend:  "acp",
		WorkDir:  "/home/test",
		Metadata: metadata,
	}

	err = store.Create(session)
	require.NoError(t, err)

	retrieved, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, metadata, retrieved.Metadata)

	// Update metadata
	newMetadata := `{"updated":"value"}`
	session.Metadata = newMetadata
	err = store.Update(session)
	require.NoError(t, err)

	retrieved2, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, newMetadata, retrieved2.Metadata)

	// Cleanup
	_ = store.Delete(session.ID)
}

func TestSessionStore_ListWithOffset(t *testing.T) {
	store, err := NewSessionStore()
	require.NoError(t, err)

	// Create sessions
	session1 := &Session{
		ID:      "test-offset-1",
		Backend: "acp",
		WorkDir: "/home/test1",
	}
	session2 := &Session{
		ID:      "test-offset-2",
		Backend: "acp",
		WorkDir: "/home/test2",
	}
	session3 := &Session{
		ID:      "test-offset-3",
		Backend: "acp",
		WorkDir: "/home/test3",
	}

	err = store.Create(session1)
	require.NoError(t, err)
	err = store.Create(session2)
	require.NoError(t, err)
	err = store.Create(session3)
	require.NoError(t, err)

	// List with offset
	page1, err := store.List(ListOptions{Limit: 2, Offset: 0})
	require.NoError(t, err)

	page2, err := store.List(ListOptions{Limit: 2, Offset: 2})
	require.NoError(t, err)

	// Total should be at least 3, page sizes should match limits
	assert.GreaterOrEqual(t, len(page1)+len(page2), 3)

	// Cleanup
	_ = store.Delete(session1.ID)
	_ = store.Delete(session2.ID)
	_ = store.Delete(session3.ID)
}
