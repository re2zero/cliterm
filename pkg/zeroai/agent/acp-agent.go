package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/wavetermdev/waveterm/pkg/zeroai/protocol"
)

type AcpAgent struct {
	config       AgentConfig
	backend      protocol.AcpBackend
	conn         protocol.Connection
	adapter      *protocol.AcpAdapter
	sessions     map[string]*AgentSession
	eventChs     map[string]chan AgentEvent
	acpSessionID string
	status       AgentStatus
	mu           sync.Mutex
	cancelCtx    context.CancelFunc
}

func NewAcpAgent(config AgentConfig) (Agent, error) {
	backend, err := protocol.GetBackendFromString(config.Backend)
	if err != nil {
		return nil, fmt.Errorf("invalid backend: %w", err)
	}

	return &AcpAgent{
		config:   config,
		backend:  backend,
		adapter:  protocol.NewAcpAdapter(),
		sessions: make(map[string]*AgentSession),
		eventChs: make(map[string]chan AgentEvent),
		status: AgentStatus{
			IsConnected: false,
			HasSession:  false,
			IsStreaming: false,
			LastSeen:    time.Now(),
		},
	}, nil
}

func (a *AcpAgent) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	backendCfg, err := protocol.GetBackendConfig(a.backend)
	if err != nil {
		return fmt.Errorf("failed to get backend config: %w", err)
	}

	cliPath := a.config.CliPath
	if cliPath == "" {
		cliPath = backendCfg.DefaultCliPath
	}
	if cliPath == "" {
		cliPath = backendCfg.CliCommand
	}

	log.Printf("[DEBUG] AcpAgent.Start: backend=%s, cliPath=%s, transport=%s", a.backend, cliPath, backendCfg.Transport)

	sessionConfig := protocol.AcpSessionConfig{
		Backend: a.backend,
		CliPath: cliPath,
		Env:     protocol.MergeEnvVars(a.backend, a.config.Env),
	}

	if a.config.Env != nil {
		if sessionConfig.Env == nil {
			sessionConfig.Env = make(map[string]string)
		}
		for k, v := range a.config.Env {
			sessionConfig.Env[k] = v
		}
	}

	a.conn = protocol.NewConnection(a.backend)

	a.conn.SetCallbacks(protocol.AcpCallbacks{
		OnSessionUpdate: func(update *protocol.AcpSessionUpdate) error {
			log.Printf("[DEBUG] AcpAgent callback: OnSessionUpdate type=%s", update.SessionUpdate)
			return a.handleSessionUpdate(update)
		},
		OnPermission: func(req *protocol.AcpPermissionRequest) error {
			return a.handlePermissionRequest(req)
		},
		OnError: func(err error) {
			a.mu.Lock()
			a.status.LastError = err
			a.mu.Unlock()
			log.Printf("[DEBUG] AcpAgent callback: OnError: %v", err)
		},
		OnDisconnect: func(info *protocol.AcpDisconnectInfo) {
			a.mu.Lock()
			a.status.IsConnected = false
			a.status.LastError = fmt.Errorf("disconnected: %s", info.Reason)
			a.mu.Unlock()
			log.Printf("[DEBUG] AcpAgent callback: OnDisconnect: %s", info.Reason)
		},
	})

	log.Printf("[DEBUG] AcpAgent.Start: initializing connection with cliPath=%s", sessionConfig.CliPath)
	if err := a.conn.Initialize(sessionConfig); err != nil {
		return fmt.Errorf("failed to initialize ACP connection: %w", err)
	}

	a.status.IsConnected = true
	a.status.LastSeen = time.Now()

	log.Printf("[DEBUG] AcpAgent.Start: agent started successfully")

	return nil
}

func (a *AcpAgent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cancelCtx != nil {
		a.cancelCtx()
		a.cancelCtx = nil
	}

	if a.conn != nil {
		a.conn.Close()
	}

	for sid, ch := range a.eventChs {
		close(ch)
		delete(a.eventChs, sid)
	}

	a.status.IsConnected = false
	a.status.HasSession = false
	a.status.IsStreaming = false
	return nil
}

func (a *AcpAgent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conn != nil && a.conn.IsConnected()
}

func (a *AcpAgent) CreateSession(ctx context.Context, opts AgentSessionOptions) (*AgentSession, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent not running")
	}

	result, err := a.conn.NewSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	if opts.YoloMode {
		log.Printf("[DEBUG] AcpAgent.CreateSession: enabling bypassPermissions mode via ACP")
		if setErr := a.conn.SetSessionModeViaAcp(ctx, "bypassPermissions"); setErr != nil {
			log.Printf("[DEBUG] AcpAgent.CreateSession: SetSessionModeViaAcp failed: %v", setErr)
			return nil, fmt.Errorf("failed to set bypassPermissions mode: %w", setErr)
		}
		log.Printf("[DEBUG] AcpAgent.CreateSession: bypassPermissions mode set successfully")
	}

	a.mu.Lock()
	a.acpSessionID = result.SessionID
	a.mu.Unlock()

	now := time.Now().Unix()
	session := &AgentSession{
		ID:            result.SessionID,
		Backend:       string(a.backend),
		WorkDir:       opts.WorkDir,
		Model:         opts.Model,
		Provider:      string(a.backend),
		ThinkingLevel: "",
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      make(map[string]interface{}),
	}

	eventCh := make(chan AgentEvent, 100)
	a.mu.Lock()
	a.sessions[result.SessionID] = session
	a.eventChs[result.SessionID] = eventCh
	a.status.HasSession = true
	a.mu.Unlock()

	return session, nil
}

func (a *AcpAgent) LoadSession(ctx context.Context, sessionID string) (*AgentSession, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent not running")
	}

	_, err := a.conn.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
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

	a.mu.Lock()
	a.sessions[sessionID] = session
	a.eventChs[sessionID] = eventCh
	a.mu.Unlock()

	return session, nil
}

func (a *AcpAgent) GetSession(sessionID string) (*AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	session, exists := a.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

func (a *AcpAgent) ListSessions() ([]*AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]*AgentSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		result = append(result, s)
	}
	return result, nil
}

func (a *AcpAgent) DeleteSession(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.sessions[sessionID]; !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(a.sessions, sessionID)
	if ch, exists := a.eventChs[sessionID]; exists {
		close(ch)
		delete(a.eventChs, sessionID)
	}

	if len(a.sessions) == 0 {
		a.status.HasSession = false
	}

	return nil
}

func (a *AcpAgent) SendMessage(ctx context.Context, sessionID string, message SendMessageInput) (<-chan AgentEvent, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent not running")
	}

	a.mu.Lock()
	if _, exists := a.sessions[sessionID]; !exists {
		a.mu.Unlock()
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	eventCh := make(chan AgentEvent, 256)
	a.eventChs[sessionID] = eventCh
	acpSID := a.acpSessionID
	a.mu.Unlock()

	log.Printf("[DEBUG] AcpAgent.SendMessage: sessionID=%s, acpSessionID=%s, content_len=%d", sessionID, acpSID, len(message.Content))

	a.status.IsStreaming = true
	a.status.LastSeen = time.Now()

	promptOpts := protocol.AcpPromptOptions{
		Files:         message.Files,
		ModelOverride: message.Model,
	}

	promptCtx, promptCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	a.mu.Lock()
	a.cancelCtx = promptCancel
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.status.IsStreaming = false
			ch := a.eventChs[sessionID]
			delete(a.eventChs, sessionID)
			a.mu.Unlock()
			promptCancel()
			if ch != nil {
				close(ch)
			}
		}()

		err := a.conn.StreamPrompt(promptCtx, acpSID, message.Content, promptOpts, func(update *protocol.AcpSessionUpdate) error {
			event := a.convertUpdateToEvent(sessionID, update)
			if event != nil {
				a.sendEvent(sessionID, *event)
			}
			return nil
		})

		if err != nil {
			log.Printf("[DEBUG] AcpAgent.SendMessage goroutine: StreamPrompt error: %v", err)
			a.sendEvent(sessionID, AgentEvent{
				Type:    EventTypeError,
				Session: sessionID,
				Data:    fmt.Sprintf("Stream error: %v", err),
				Created: time.Now().Unix(),
			})
		}
	}()

	return eventCh, nil
}

func (a *AcpAgent) CancelPrompt() {
	a.mu.Lock()
	if a.cancelCtx != nil {
		a.cancelCtx()
		a.cancelCtx = nil
	}
	a.status.IsStreaming = false
	a.mu.Unlock()
}

func (a *AcpAgent) ConfirmPermission(ctx context.Context, sessionID string, callID string, optionID string) error {
	if !a.IsRunning() {
		return fmt.Errorf("agent not running")
	}
	return a.conn.ConfirmPermission(ctx, callID, optionID)
}

func (a *AcpAgent) GetStatus() AgentStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.LastSeen = time.Now()
	return a.status
}

func (a *AcpAgent) GetSessionID() string {
	if a.conn != nil {
		return a.conn.GetSessionID()
	}
	return ""
}

func (a *AcpAgent) getAnySessionID() string {
	for sid := range a.sessions {
		return sid
	}
	return ""
}

func (a *AcpAgent) handleSessionUpdate(update *protocol.AcpSessionUpdate) error {
	a.mu.Lock()
	a.status.LastSeen = time.Now()
	a.mu.Unlock()

	sessionID := a.getAnySessionID()
	if sessionID == "" {
		return nil
	}

	event := a.convertUpdateToEvent(sessionID, update)
	if event != nil {
		a.sendEvent(sessionID, *event)
	}
	return nil
}

func (a *AcpAgent) handlePermissionRequest(req *protocol.AcpPermissionRequest) error {
	sessionID := a.getAnySessionID()
	if sessionID == "" {
		return nil
	}

	a.sendEvent(sessionID, AgentEvent{
		Type:    EventTypePermission,
		Session: sessionID,
		Data:    req,
		Created: time.Now().Unix(),
	})
	return nil
}

func (a *AcpAgent) sendEvent(sessionID string, event AgentEvent) bool {
	a.mu.Lock()
	ch, exists := a.eventChs[sessionID]
	a.mu.Unlock()

	if !exists {
		return false
	}

	defer func() {
		recover()
	}()

	select {
	case ch <- event:
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

func (a *AcpAgent) convertUpdateToEvent(sessionID string, update *protocol.AcpSessionUpdate) *AgentEvent {
	if update == nil {
		return nil
	}

	now := time.Now().Unix()

	switch update.SessionUpdate {
	case "agent_message_chunk", "text", "text_chunk":
		return &AgentEvent{
			Type:    EventTypeContent,
			Session: sessionID,
			Data:    update.Content,
			Created: now,
		}

	case "tool_call", "tool_started":
		return &AgentEvent{
			Type:    EventTypeToolCall,
			Session: sessionID,
			Data:    update,
			Created: now,
		}

	case "tool_completed", "tool_failed", "tool_call_update":
		return &AgentEvent{
			Type:    EventTypeToolCall,
			Session: sessionID,
			Data:    update,
			Created: now,
		}

	case "end_turn":
		return &AgentEvent{
			Type:    EventTypeEndTurn,
			Session: sessionID,
			Created: now,
		}

	case "error":
		return &AgentEvent{
			Type:    EventTypeError,
			Session: sessionID,
			Error:   fmt.Errorf("%s", update.Content),
			Created: now,
		}

	default:
		data, _ := json.Marshal(update)
		return &AgentEvent{
			Type:    EventTypeContent,
			Session: sessionID,
			Data:    string(data),
			Created: now,
		}
	}
}
