// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/wavetermdev/waveterm/pkg/acpclient"
	"github.com/wavetermdev/waveterm/pkg/zeroai/protocol"
)

// AcpAgent implements the Agent interface using the ACP protocol via acpclient
type AcpAgent struct {
	config   AgentConfig
	backend  protocol.AcpBackend
	client   *acpclient.ACPClient
	sessions map[string]*AgentSession
	eventChs map[string]chan AgentEvent
	status   AgentStatus
	running  bool
}

// NewAcpAgent creates a new ACP agent
func NewAcpAgent(config AgentConfig) (Agent, error) {
	backend, err := protocol.GetBackendFromString(config.Backend)
	if err != nil {
		return nil, fmt.Errorf("invalid backend: %w", err)
	}

	agent := &AcpAgent{
		config:   config,
		backend:  backend,
		sessions: make(map[string]*AgentSession),
		eventChs: make(map[string]chan AgentEvent),
		status: AgentStatus{
			IsConnected: false,
			HasSession:  false,
			IsStreaming: false,
			LastSeen:    time.Now(),
		},
		running: false,
	}

	return agent, nil
}

// Start initializes the connection to the agent process
func (a *AcpAgent) Start(ctx context.Context) error {
	aConfig := &acpclient.ACPConfig{
		CLI:     string(a.backend),
		Model:   "",
		WorkDir: "",
	}
	if a.config.CliPath != "" {
		aConfig.CLI = a.config.CliPath
	}
	if a.config.Env != nil {
		aConfig.EnvVars = a.config.Env
	}

	a.client = acpclient.NewACPClient(aConfig)

	if err := a.client.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize ACP client: %w", err)
	}

	a.running = true
	a.status.IsConnected = true
	a.status.LastSeen = time.Now()

	return nil
}

// Stop shuts down the agent connection
func (a *AcpAgent) Stop() error {
	if a.client != nil {
		a.client.Close()
	}
	a.running = false
	a.status.IsConnected = false
	a.status.HasSession = false
	a.status.IsStreaming = false
	return nil
}

// IsRunning returns whether the agent is running
func (a *AcpAgent) IsRunning() bool {
	return a.running
}

// CreateSession creates a new session with the agent
func (a *AcpAgent) CreateSession(ctx context.Context, opts AgentSessionOptions) (*AgentSession, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent not running")
	}

	sessionID, err := a.client.CreateSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	eventCh := make(chan AgentEvent, 100)

	now := time.Now().Unix()
	session := &AgentSession{
		ID:            sessionID,
		Backend:       string(a.backend),
		WorkDir:       opts.WorkDir,
		Model:         "",
		Provider:      string(a.backend),
		ThinkingLevel: "",
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      make(map[string]interface{}),
	}

	a.sessions[sessionID] = session
	a.eventChs[sessionID] = eventCh
	a.status.HasSession = true

	return session, nil
}

// LoadSession loads an existing session
func (a *AcpAgent) LoadSession(ctx context.Context, sessionID string) (*AgentSession, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent not running")
	}

	eventCh := make(chan AgentEvent, 100)

	now := time.Now().Unix()
	session := &AgentSession{
		ID:            sessionID,
		Backend:       string(a.backend),
		WorkDir:       "",
		Model:         "",
		Provider:      string(a.backend),
		ThinkingLevel: "",
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      make(map[string]interface{}),
	}

	a.sessions[sessionID] = session
	a.eventChs[sessionID] = eventCh

	return session, nil
}

// GetSession retrieves a session by ID
func (a *AcpAgent) GetSession(sessionID string) (*AgentSession, error) {
	session, exists := a.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// ListSessions returns all sessions
func (a *AcpAgent) ListSessions() ([]*AgentSession, error) {
	result := make([]*AgentSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		result = append(result, s)
	}
	return result, nil
}

// DeleteSession deletes a session
func (a *AcpAgent) DeleteSession(sessionID string) error {
	if _, exists := a.sessions[sessionID]; !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(a.sessions, sessionID)
	delete(a.eventChs, sessionID)

	if len(a.sessions) == 0 {
		a.status.HasSession = false
	}

	return nil
}

// SendMessage sends a message and returns a channel of streaming events
func (a *AcpAgent) SendMessage(ctx context.Context, sessionID string, message SendMessageInput) (<-chan AgentEvent, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent not running")
	}

	if _, exists := a.sessions[sessionID]; !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	eventCh, exists := a.eventChs[sessionID]
	if !exists {
		eventCh = make(chan AgentEvent, 100)
		a.eventChs[sessionID] = eventCh
	}

	a.status.IsStreaming = true
	a.status.LastSeen = time.Now()

	// Use acpclient.SendPrompt which returns msgCh and errCh
	msgCh, errCh := a.client.SendPrompt(ctx, message.Content)

	// Forward messages from msgCh to eventCh
	go func() {
		defer func() {
			a.status.IsStreaming = false
		}()

		for {
			select {
			case msg := <-msgCh:
				if msg == nil {
					// End of stream
					a.sendEvent(sessionID, AgentEvent{
						Type:    EventTypeEndTurn,
						Session: sessionID,
						Created: time.Now().Unix(),
					})
					return
				}

				// Convert ACP message to AgentEvent
				event := a.convertACPMessageToEvent(sessionID, msg)
				if event != nil {
					a.sendEvent(sessionID, *event)
				}

			case err := <-errCh:
				if err != nil {
					a.sendEvent(sessionID, AgentEvent{
						Type:    EventTypeError,
						Session: sessionID,
						Error:   err,
						Created: time.Now().Unix(),
					})
				}
				return

			case <-ctx.Done():
				return
			}
		}
	}()

	return eventCh, nil
}

// ConfirmPermission confirms a permission request
func (a *AcpAgent) ConfirmPermission(ctx context.Context, sessionID string, callID string, optionID string) error {
	if !a.IsRunning() {
		return fmt.Errorf("agent not running")
	}
	// TODO: implement via acpclient
	return fmt.Errorf("not implemented")
}

// GetStatus returns the current agent status
func (a *AcpAgent) GetStatus() AgentStatus {
	a.status.LastSeen = time.Now()
	return a.status
}

// GetSessionID returns the current session ID
func (a *AcpAgent) GetSessionID() string {
	if a.client != nil {
		return a.client.GetSessionID()
	}
	return ""
}

func (a *AcpAgent) sendEvent(sessionID string, event AgentEvent) {
	ch, exists := a.eventChs[sessionID]
	if !exists {
		return
	}

	select {
	case ch <- event:
	case <-time.After(100 * time.Millisecond):
	case <-a.ctxDone():
	}
}

func (a *AcpAgent) ctxDone() <-chan struct{} {
	// Simple channel that never closes for now
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (a *AcpAgent) convertACPMessageToEvent(sessionID string, msg *acpclient.ACPMessage) *AgentEvent {
	if msg == nil {
		return nil
	}

	// Check for session/update notification
	if msg.Method == "session/update" {
		var update protocol.AcpSessionUpdate
		if params, ok := msg.Params.(map[string]interface{}); ok {
			if sessionUpdate, ok := params["sessionUpdate"].(string); ok {
				update.SessionUpdate = sessionUpdate
			}
			if content, ok := params["content"].(string); ok {
				update.Content = content
			}
		}

		eventType := EventTypeContent
		switch update.SessionUpdate {
		case "text", "text_chunk":
			eventType = EventTypeContent
		case "tool_call", "tool_started", "tool_completed", "tool_failed":
			eventType = EventType(update.SessionUpdate)
		case "end_turn":
			eventType = EventTypeEndTurn
		case "error":
			eventType = EventTypeError
		default:
			eventType = EventTypeContent
		}

		return &AgentEvent{
			Type:    eventType,
			Session: sessionID,
			Data:    update,
			Created: time.Now().Unix(),
		}
	}

	// Check for final response (no method)
	if msg.Method == "" {
		// This is the final response, handled by nil check in SendPrompt
		return nil
	}

	return nil
}
