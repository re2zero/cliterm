// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavetermdev/waveterm/pkg/zeroai/agent"
	"github.com/wavetermdev/waveterm/pkg/zeroai/store"
)

var sessionCounter int64

func nextSessionID() string {
	id := atomic.AddInt64(&sessionCounter, 1)
	return fmt.Sprintf("test-session-%d-%d", time.Now().UnixNano(), id)
}

// mockAgent is a mock implementation of agent.Agent for testing
type mockAgent struct {
	sessions map[string]*agent.AgentSession
	running  bool
}

func newMockAgent() *mockAgent {
	return &mockAgent{
		sessions: make(map[string]*agent.AgentSession),
		running:  true,
	}
}

func (m *mockAgent) Start(ctx context.Context) error {
	m.running = true
	return nil
}

func (m *mockAgent) Stop() error {
	m.running = false
	return nil
}

func (m *mockAgent) IsRunning() bool {
	return m.running
}

func (m *mockAgent) CreateSession(ctx context.Context, opts agent.AgentSessionOptions) (*agent.AgentSession, error) {
	session := &agent.AgentSession{
		ID:        nextSessionID(),
		Backend:   opts.Backend,
		WorkDir:   opts.WorkDir,
		Model:     opts.Model,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *mockAgent) LoadSession(ctx context.Context, sessionID string) (*agent.AgentSession, error) {
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (m *mockAgent) GetSession(sessionID string) (*agent.AgentSession, error) {
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (m *mockAgent) ListSessions() ([]*agent.AgentSession, error) {
	sessions := make([]*agent.AgentSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (m *mockAgent) DeleteSession(sessionID string) error {
	if _, exists := m.sessions[sessionID]; !exists {
		return errors.New("session not found")
	}
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockAgent) SendMessage(ctx context.Context, sessionID string, message agent.SendMessageInput) (<-chan agent.AgentEvent, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAgent) ConfirmPermission(ctx context.Context, sessionID string, callID string, optionID string) error {
	return nil
}

func (m *mockAgent) GetStatus() agent.AgentStatus {
	return agent.AgentStatus{
		IsConnected: m.running,
	}
}

// mockSessionStore is a mock implementation of store.SessionStore for testing
type mockSessionStore struct {
	sessions map[string]*store.Session
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]*store.Session),
	}
}

func (m *mockSessionStore) Create(session *store.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionStore) Get(sessionID string) (*store.Session, error) {
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (m *mockSessionStore) Update(session *store.Session) error {
	if _, exists := m.sessions[session.ID]; !exists {
		return errors.New("session not found")
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionStore) Delete(sessionID string) error {
	if _, exists := m.sessions[sessionID]; !exists {
		return errors.New("session not found")
	}
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockSessionStore) List(opts store.ListOptions) ([]*store.Session, error) {
	sessions := make([]*store.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if opts.Backend == "" || s.Backend == opts.Backend {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

func TestSessionService_CreateSession(t *testing.T) {
	mockAg := newMockAgent()
	mockStore := newMockSessionStore()

	svc, err := NewSessionService(mockAg, mockStore)
	require.NoError(t, err)
	require.NotNil(t, svc)

	ctx := context.Background()
	opts := agent.AgentSessionOptions{
		Backend: "acp",
		WorkDir: "/home/test",
		Model:   "claude-3",
	}

	session, err := svc.CreateSession(ctx, opts)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "acp", session.Backend)
	assert.Equal(t, "/home/test", session.WorkDir)
	assert.Equal(t, "claude-3", session.Model)

	// Verify it was stored in the store
	storedSession, err := mockStore.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, storedSession.ID)
}

func TestSessionService_CreateAndGet(t *testing.T) {
	mockAg := newMockAgent()
	mockStore := newMockSessionStore()

	svc, err := NewSessionService(mockAg, mockStore)
	require.NoError(t, err)

	ctx := context.Background()
	opts := agent.AgentSessionOptions{Backend: "acp", Model: "test-model"}

	// Create a session first
	created, err := svc.CreateSession(ctx, opts)
	require.NoError(t, err)

	// Get the session
	retrieved, err := svc.GetSession(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Model, retrieved.Model)

	// Test getting non-existent session
	_, err = svc.GetSession("non-existent-id")
	assert.Error(t, err)
}

func TestSessionService_ListSessions(t *testing.T) {
	mockAg := newMockAgent()
	mockStore := newMockSessionStore()

	svc, err := NewSessionService(mockAg, mockStore)
	require.NoError(t, err)

	ctx := context.Background()

	// Get initial count
	sessions, err := svc.ListSessions()
	require.NoError(t, err)
	initialCount := len(sessions)

	// Create multiple sessions
	s1, err := svc.CreateSession(ctx, agent.AgentSessionOptions{Backend: "acp", Model: "model-1"})
	require.NoError(t, err)

	s2, err := svc.CreateSession(ctx, agent.AgentSessionOptions{Backend: "acp", Model: "model-2"})
	require.NoError(t, err)

	// List should return 2 more sessions
	sessions, err = svc.ListSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, initialCount+2)

	// Verify our sessions are in the list
	sessionIDs := make(map[string]bool)
	for _, s := range sessions {
		sessionIDs[s.ID] = true
	}
	assert.True(t, sessionIDs[s1.ID])
	assert.True(t, sessionIDs[s2.ID])
}

func TestSessionService_DeleteSession(t *testing.T) {
	mockAg := newMockAgent()
	mockStore := newMockSessionStore()

	svc, err := NewSessionService(mockAg, mockStore)
	require.NoError(t, err)

	ctx := context.Background()
	opts := agent.AgentSessionOptions{Backend: "acp", Model: "test-model"}

	// Create a session
	session, err := svc.CreateSession(ctx, opts)
	require.NoError(t, err)
	sessionID := session.ID

	// Delete the session
	err = svc.DeleteSession(sessionID)
	require.NoError(t, err)

	// Verify it's deleted from agent
	_, err = mockAg.GetSession(sessionID)
	assert.Error(t, err)

	// Verify it's deleted from store
	_, err = mockStore.Get(sessionID)
	assert.Error(t, err)

	// Test deleting non-existent session
	err = svc.DeleteSession("non-existent-id")
	assert.Error(t, err)
}

func TestSessionService_SetWorkDir(t *testing.T) {
	mockAg := newMockAgent()
	mockStore := newMockSessionStore()

	svc, err := NewSessionService(mockAg, mockStore)
	require.NoError(t, err)

	ctx := context.Background()
	opts := agent.AgentSessionOptions{Backend: "acp", WorkDir: "/original", Model: "test-model"}

	// Create a session
	session, err := svc.CreateSession(ctx, opts)
	require.NoError(t, err)

	// Update workdir
	newWorkDir := "/new/workdir"
	err = svc.SetWorkDir(session.ID, newWorkDir)
	require.NoError(t, err)

	// Verify the update
	stored, err := mockStore.Get(session.ID)
	require.NoError(t, err)
	assert.Equal(t, newWorkDir, stored.WorkDir)

	// Test with non-existent session
	err = svc.SetWorkDir("non-existent-id", "/some/path")
	assert.Error(t, err)
}

func TestSessionService_NewSessionServiceValidation(t *testing.T) {
	mockAg := newMockAgent()
	mockStore := newMockSessionStore()

	// Test with nil agent
	_, err := NewSessionService(nil, mockStore)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent is required")

	// Test with nil store
	_, err = NewSessionService(mockAg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session store is required")
}

func TestSessionService_DeleteSessionValidation(t *testing.T) {
	mockAg := newMockAgent()
	mockStore := newMockSessionStore()

	svc, err := NewSessionService(mockAg, mockStore)
	require.NoError(t, err)

	// Test with empty session ID
	err = svc.DeleteSession("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session ID is required")
}
