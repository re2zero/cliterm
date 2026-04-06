package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type ClaudeMessage struct {
	Type          string                 `json:"type"`
	Subtype       string                 `json:"subtype,omitempty"`
	Content       string                 `json:"content,omitempty"`
	ToolName      string                 `json:"tool_name,omitempty"`
	ToolCallID    string                 `json:"tool_call_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	Result        string                 `json:"result,omitempty"`
	IsError       bool                   `json:"is_error,omitempty"`
	CostUSD       float64                `json:"total_cost_usd,omitempty"`
	DurationMs    int64                  `json:"duration_ms,omitempty"`
	DurationApiMs int64                  `json:"duration_api_ms,omitempty"`
	NumTurns      int                    `json:"num_turns,omitempty"`
	HookID        string                 `json:"hook_id,omitempty"`
	HookName      string                 `json:"hook_name,omitempty"`
	Output        string                 `json:"output,omitempty"`
	Stderr        string                 `json:"stderr,omitempty"`
	ExitCode      int                    `json:"exit_code,omitempty"`
	Outcome       string                 `json:"outcome,omitempty"`
	UUID          string                 `json:"uuid,omitempty"`
	Raw           map[string]interface{} `json:"-"`
}

type ClaudeStreamConnection struct {
	mu sync.RWMutex

	state   atomic.Int32
	config  AcpSessionConfig
	cliPath string

	sessionID  string
	hasSession bool

	callbacks AcpCallbacks

	lastError error
	lastSeen  atomic.Value

	activeProcess *exec.Cmd
	activeStdin   io.WriteCloser
	activeMu      sync.Mutex

	promptDoneCh chan struct{}
	shutdownCh   chan struct{}
}

func NewClaudeStreamConnection() *ClaudeStreamConnection {
	conn := &ClaudeStreamConnection{
		shutdownCh: make(chan struct{}),
	}
	conn.state.Store(int32(ConnectionStateDisconnected))
	return conn
}

func (c *ClaudeStreamConnection) Initialize(config AcpSessionConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.GetState() != ConnectionStateDisconnected {
		return fmt.Errorf("connection already initialized")
	}

	cliPath := config.CliPath
	if cliPath == "" {
		backendCfg, err := GetBackendConfig(config.Backend)
		if err != nil {
			return fmt.Errorf("failed to get backend config: %w", err)
		}
		cliPath = backendCfg.DefaultCliPath
	}
	if cliPath == "" {
		return fmt.Errorf("no CLI path specified for Claude")
	}

	c.config = config
	c.cliPath = cliPath
	c.state.Store(int32(ConnectionStateConnected))
	c.lastSeen.Store(time.Now())

	return nil
}

func (c *ClaudeStreamConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.GetState() == ConnectionStateDisconnected {
		return nil
	}

	c.state.Store(int32(ConnectionStateClosing))
	close(c.shutdownCh)

	c.activeMu.Lock()
	if c.activeProcess != nil && c.activeProcess.Process != nil {
		c.activeProcess.Process.Kill()
	}
	c.activeStdin = nil
	c.activeProcess = nil
	c.activeMu.Unlock()

	c.sessionID = ""
	c.hasSession = false
	c.state.Store(int32(ConnectionStateDisconnected))
	return nil
}

func (c *ClaudeStreamConnection) IsConnected() bool {
	return c.GetState() == ConnectionStateConnected
}

func (c *ClaudeStreamConnection) HasSession() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hasSession
}

func (c *ClaudeStreamConnection) GetSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

func (c *ClaudeStreamConnection) SetSessionMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.YoloMode = true
}

func (c *ClaudeStreamConnection) GetState() ConnectionState {
	return ConnectionState(c.state.Load())
}

func (c *ClaudeStreamConnection) GetStatus() ConnectionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := ConnectionState(c.state.Load())
	lastSeen, _ := c.lastSeen.Load().(time.Time)

	return ConnectionStatus{
		State:      state,
		HasSession: c.hasSession,
		LastError:  c.lastError,
		LastSeen:   lastSeen,
	}
}

func (c *ClaudeStreamConnection) SetCallbacks(callbacks AcpCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = callbacks
}

func (c *ClaudeStreamConnection) WaitForDone() <-chan struct{} {
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
	if c.promptDoneCh != nil {
		return c.promptDoneCh
	}
	done := make(chan struct{})
	close(done)
	return done
}

func (c *ClaudeStreamConnection) NewSession(ctx context.Context) (*SessionNewResult, error) {
	c.mu.Lock()
	c.sessionID = fmt.Sprintf("claude-%d", time.Now().UnixMilli())
	c.hasSession = true
	sid := c.sessionID
	c.mu.Unlock()

	return &SessionNewResult{SessionID: sid}, nil
}

func (c *ClaudeStreamConnection) LoadSession(ctx context.Context, sessionID string) (*SessionLoadResult, error) {
	c.mu.Lock()
	c.sessionID = sessionID
	c.hasSession = true
	c.mu.Unlock()

	return &SessionLoadResult{SessionID: sessionID}, nil
}

func (c *ClaudeStreamConnection) SendMessage(ctx context.Context, method string, params map[string]interface{}, timeout time.Duration) (*AcpResponse, error) {
	return nil, &AcpError{Type: ErrorConnection, Message: "Claude stream connection does not support JSON-RPC"}
}

func (c *ClaudeStreamConnection) SendNotification(method string, params map[string]interface{}) error {
	return &AcpError{Type: ErrorConnection, Message: "Claude stream connection does not support JSON-RPC"}
}

func (c *ClaudeStreamConnection) StreamPrompt(ctx context.Context, sessionID, prompt string, opts AcpPromptOptions, callback StreamCallback) error {
	if !c.IsConnected() {
		return &AcpError{Type: ErrorConnection, Message: "not connected"}
	}

	args := []string{
		"-p",
		"--verbose",
		"--output-format", "stream-json",
	}

	c.mu.RLock()
	cfg := c.config
	cliPath := c.cliPath
	hasSession := c.hasSession
	existingSessionID := c.sessionID
	c.mu.RUnlock()

	if cfg.YoloMode {
		args = append(args, "--dangerously-skip-permissions")
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	args = append(args, prompt)

	log.Printf("[DEBUG] Claude StreamPrompt: cliPath=%s, hasSession=%v, sessionId=%s, args=%v, prompt_len=%d", cliPath, hasSession, existingSessionID, args, len(prompt))

	cmd := exec.CommandContext(ctx, cliPath, args...)
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if cfg.Cwd != "" {
		cmd.Dir = cfg.Cwd
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[DEBUG] Claude StreamPrompt: cmd.Start() failed: %v", err)
		return fmt.Errorf("failed to start claude process: %w", err)
	}

	log.Printf("[DEBUG] Claude StreamPrompt: process started, pid=%d", cmd.Process.Pid)

	c.activeMu.Lock()
	c.activeProcess = cmd
	c.promptDoneCh = make(chan struct{})
	c.activeMu.Unlock()

	c.lastSeen.Store(time.Now())

	go c.readProcessStderr(stderr)

	go c.readProcessOutput(stdout, cmd)

	return nil
}

func (c *ClaudeStreamConnection) ConfirmPermission(ctx context.Context, callID, optionID string) error {
	return &AcpError{Type: ErrorPermission, Message: "Claude stream handles permissions via --dangerously-skip-permissions"}
}

func (c *ClaudeStreamConnection) readProcessOutput(stdout io.Reader, cmd *exec.Cmd) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-c.shutdownCh:
			return
		default:
		}

		line := scanner.Bytes()
		c.lastSeen.Store(time.Now())

		var msg ClaudeMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			c.logError(fmt.Sprintf("parse error: %v", err))
			continue
		}

		msg.Raw = make(map[string]interface{})
		json.Unmarshal(line, &msg.Raw)

		if msg.SessionID != "" {
			c.mu.Lock()
			c.sessionID = msg.SessionID
			c.mu.Unlock()
		}

		c.handleClaudeMessage(&msg)
	}

	if cmd != nil {
		cmd.Wait()
	}

	c.activeMu.Lock()
	if c.promptDoneCh != nil {
		close(c.promptDoneCh)
		c.promptDoneCh = nil
	}
	c.activeMu.Unlock()

	c.mu.RLock()
	callbacks := c.callbacks
	c.mu.RUnlock()

	if callbacks.OnDisconnect != nil {
		callbacks.OnDisconnect(&AcpDisconnectInfo{Reason: "prompt completed"})
	}
}

func (c *ClaudeStreamConnection) readProcessStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		c.logError(fmt.Sprintf("stderr: %s", scanner.Text()))
	}
}

func extractContentBlocks(raw map[string]interface{}) []map[string]interface{} {
	if raw == nil {
		return nil
	}

	msgRaw, ok := raw["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	contentRaw, ok := msgRaw["content"]
	if !ok {
		return nil
	}

	switch content := contentRaw.(type) {
	case []interface{}:
		var blocks []map[string]interface{}
		for _, item := range content {
			if block, ok := item.(map[string]interface{}); ok {
				blocks = append(blocks, block)
			}
		}
		return blocks
	case string:
		return []map[string]interface{}{
			{"type": "text", "text": content},
		}
	}
	return nil
}

func (c *ClaudeStreamConnection) convertContentBlock(block map[string]interface{}) *AcpSessionUpdate {
	blockType, _ := block["type"].(string)

	switch blockType {
	case "text":
		text, _ := block["text"].(string)
		return &AcpSessionUpdate{
			SessionUpdate: "text",
			Content:       text,
		}
	case "tool_use":
		name, _ := block["name"].(string)
		id, _ := block["id"].(string)
		desc, _ := block["description"].(string)
		if desc == "" {
			desc = name
		}
		return &AcpSessionUpdate{
			SessionUpdate: "tool_call",
			Content:       desc,
			ToolCall: &AcpToolCall{
				CallID:      id,
				ToolName:    name,
				Description: desc,
			},
		}
	case "tool_result":
		id, _ := block["tool_use_id"].(string)
		content := extractTextFromBlock(block)
		return &AcpSessionUpdate{
			SessionUpdate: "tool_completed",
			Content:       content,
			ToolCall: &AcpToolCall{
				CallID: id,
			},
		}
	}

	return nil
}

func extractTextFromBlock(block map[string]interface{}) string {
	if content, ok := block["content"]; ok {
		switch c := content.(type) {
		case string:
			return c
		case []interface{}:
			var parts []string
			for _, item := range c {
				if subBlock, ok := item.(map[string]interface{}); ok {
					if subBlock["type"] == "text" {
						if text, ok := subBlock["text"].(string); ok {
							parts = append(parts, text)
						}
					}
				}
			}
			if len(parts) > 0 {
				result := ""
				for i, p := range parts {
					if i > 0 {
						result += "\n"
					}
					result += p
				}
				return result
			}
		}
	}
	return ""
}

func (c *ClaudeStreamConnection) handleClaudeMessage(msg *ClaudeMessage) {
	switch msg.Type {
	case "assistant":
		c.handleAssistantMessage(msg)
	case "system":
		c.handleSystemMessage(msg)
	case "result":
		c.handleResultMessage(msg)
	case "user":
	default:
		c.logError(fmt.Sprintf("unknown message type: %s", msg.Type))
	}
}

func (c *ClaudeStreamConnection) handleSystemMessage(msg *ClaudeMessage) {
	c.mu.RLock()
	callbacks := c.callbacks
	c.mu.RUnlock()

	if callbacks.OnSessionUpdate != nil {
		update := &AcpSessionUpdate{
			SessionUpdate: "system",
			Content:       fmt.Sprintf("[%s/%s] %s", msg.Subtype, msg.HookName, msg.Content),
			Metadata: map[string]interface{}{
				"subtype":  msg.Subtype,
				"hookName": msg.HookName,
			},
		}
		callbacks.OnSessionUpdate(update)
	}
}

func (c *ClaudeStreamConnection) handleAssistantMessage(msg *ClaudeMessage) {
	contentBlocks := extractContentBlocks(msg.Raw)
	if len(contentBlocks) == 0 {
		return
	}

	for _, block := range contentBlocks {
		update := c.convertContentBlock(block)
		if update == nil {
			continue
		}

		c.mu.RLock()
		callbacks := c.callbacks
		c.mu.RUnlock()

		if callbacks.OnSessionUpdate != nil {
			if err := callbacks.OnSessionUpdate(update); err != nil {
				c.logError(fmt.Sprintf("session update callback error: %v", err))
			}
		}
	}
}

func (c *ClaudeStreamConnection) handleResultMessage(msg *ClaudeMessage) {
	update := &AcpSessionUpdate{
		SessionUpdate: "end_turn",
		Metadata: map[string]interface{}{
			"costUsd":    msg.CostUSD,
			"durationMs": msg.DurationMs,
			"numTurns":   msg.NumTurns,
			"isError":    msg.IsError,
		},
	}

	c.mu.RLock()
	callbacks := c.callbacks
	c.mu.RUnlock()

	if callbacks.OnSessionUpdate != nil {
		callbacks.OnSessionUpdate(update)
	}
}

func (c *ClaudeStreamConnection) convertToSessionUpdate(msg *ClaudeMessage) *AcpSessionUpdate {
	switch msg.Subtype {
	case "text":
		return &AcpSessionUpdate{
			SessionUpdate: "text",
			Content:       msg.Content,
		}
	case "tool_use":
		return &AcpSessionUpdate{
			SessionUpdate: "tool_call",
			Content:       msg.Content,
			ToolCall: &AcpToolCall{
				CallID:      msg.ToolCallID,
				ToolName:    msg.ToolName,
				Description: msg.Content,
			},
		}
	case "tool_result":
		return &AcpSessionUpdate{
			SessionUpdate: "tool_completed",
			Content:       msg.Content,
			ToolCall: &AcpToolCall{
				CallID: msg.ToolCallID,
			},
		}
	default:
		return &AcpSessionUpdate{
			SessionUpdate: msg.Subtype,
			Content:       msg.Content,
		}
	}
}

func (c *ClaudeStreamConnection) logError(msg string) {
	c.mu.Lock()
	c.lastError = fmt.Errorf("%s", msg)
	c.mu.Unlock()

	c.mu.RLock()
	callbacks := c.callbacks
	c.mu.RUnlock()

	if callbacks.OnError != nil {
		callbacks.OnError(fmt.Errorf("%s", msg))
	}
}

func (c *ClaudeStreamConnection) buildCommand(config AcpSessionConfig) (string, []string, error) {
	cliPath := config.CliPath
	if cliPath == "" {
		backendCfg, err := GetBackendConfig(config.Backend)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get backend config: %w", err)
		}
		cliPath = backendCfg.DefaultCliPath
	}
	if cliPath == "" {
		return "", nil, fmt.Errorf("no CLI path specified")
	}

	args := []string{
		"-p",
		"--verbose",
		"--output-format", "stream-json",
	}

	if config.YoloMode {
		args = append(args, "--dangerously-skip-permissions")
	}
	if config.Model != "" {
		args = append(args, "--model", config.Model)
	}

	return cliPath, args, nil
}
