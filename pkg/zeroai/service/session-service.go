// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/wavetermdev/waveterm/pkg/zeroai/agent"
	"github.com/wavetermdev/waveterm/pkg/zeroai/store"
)

// SessionService provides business logic for session operations
type SessionService struct {
	agent   agent.Agent
	agentFn func(ctx context.Context, backend string) (agent.Agent, error)
	store   store.SessionStore
}

// NewSessionService creates a new session service
func NewSessionService(ag agent.Agent, sessionStore store.SessionStore) (*SessionService, error) {
	if ag == nil {
		return nil, fmt.Errorf("agent is required")
	}
	if sessionStore == nil {
		return nil, fmt.Errorf("session store is required")
	}

	return &SessionService{
		agent: ag,
		store: sessionStore,
	}, nil
}

// NewSessionServiceWithAgentFactory creates a session service that dynamically resolves agents
func NewSessionServiceWithAgentFactory(
	agentFn func(ctx context.Context, backend string) (agent.Agent, error),
	sessionStore store.SessionStore,
) (*SessionService, error) {
	if agentFn == nil {
		return nil, fmt.Errorf("agent factory is required")
	}
	if sessionStore == nil {
		return nil, fmt.Errorf("session store is required")
	}

	return &SessionService{
		agentFn: agentFn,
		store:   sessionStore,
	}, nil
}

func (s *SessionService) resolveAgent(ctx context.Context, backend string) (agent.Agent, error) {
	if s.agentFn != nil {
		return s.agentFn(ctx, backend)
	}
	return s.agent, nil
}

// CreateSession creates a new session
func (s *SessionService) CreateSession(ctx context.Context, opts agent.AgentSessionOptions) (*agent.AgentSession, error) {
	ag, err := s.resolveAgent(ctx, opts.Backend)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent: %w", err)
	}
	session, err := ag.CreateSession(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create session in agent: %w", err)
	}

	storeSession := &store.Session{
		ID:            session.ID,
		Backend:       session.Backend,
		WorkDir:       session.WorkDir,
		Model:         session.Model,
		Provider:      session.Provider,
		ThinkingLevel: session.ThinkingLevel,
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.UpdatedAt,
	}

	if err := s.store.Create(storeSession); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by ID
func (s *SessionService) GetSession(sessionID string) (*agent.AgentSession, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	storeSession, err := s.store.Get(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session from store: %w", err)
	}

	return &agent.AgentSession{
		ID:            storeSession.ID,
		Backend:       storeSession.Backend,
		WorkDir:       storeSession.WorkDir,
		Model:         storeSession.Model,
		Provider:      storeSession.Provider,
		ThinkingLevel: storeSession.ThinkingLevel,
		CreatedAt:     storeSession.CreatedAt,
		UpdatedAt:     storeSession.UpdatedAt,
	}, nil
}

// ListSessions returns all sessions
func (s *SessionService) ListSessions() ([]*agent.AgentSession, error) {
	storeSessions, err := s.store.List(store.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions from store: %w", err)
	}

	result := make([]*agent.AgentSession, len(storeSessions))
	for i, ss := range storeSessions {
		result[i] = &agent.AgentSession{
			ID:            ss.ID,
			Backend:       ss.Backend,
			WorkDir:       ss.WorkDir,
			Model:         ss.Model,
			Provider:      ss.Provider,
			ThinkingLevel: ss.ThinkingLevel,
			CreatedAt:     ss.CreatedAt,
			UpdatedAt:     ss.UpdatedAt,
		}
	}
	return result, nil
}

// DeleteSession deletes a session
func (s *SessionService) DeleteSession(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	if err := s.store.Delete(sessionID); err != nil {
		return fmt.Errorf("failed to delete session from store: %w", err)
	}

	return nil
}

// SetWorkDir sets the working directory for a session
func (s *SessionService) SetWorkDir(sessionID string, workDir string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	storeSession, err := s.store.Get(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session from store: %w", err)
	}

	storeSession.WorkDir = workDir

	if err := s.store.Update(storeSession); err != nil {
		return fmt.Errorf("failed to update session in store: %w", err)
	}

	return nil
}

// StoreSession persists a session to the store
func (s *SessionService) StoreSession(session *store.Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	return s.store.Create(session)
}
