// Package protocol implements ACP (Agent Control Protocol) connection management
//
// This file provides the core ACP connection implementation including:
// - Process communication over stdio
// - JSON-RPC request/response handling
// - Session management
// - Streaming support
// - permission routing
package protocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionState represents the connection state
type ConnectionState int

const (
	ConnectionStateDisconnected ConnectionState = iota
	ConnectionStateConnecting
	ConnectionStateConnected
	ConnectionStateClosing
)

// ConnectionStatus represents the connection status
type ConnectionStatus struct {
	State      ConnectionState `json:"state"`
	HasSession bool            `json:"hasSession"`
	LastError  error           `json:"lastError,omitempty"`
	LastSeen   time.Time       `json:"lastSeen"`
	ProcessID  int             `json:"processId,omitempty"`
}

// PendingRequest represents an in-flight JSON-RPC request waiting for response
type PendingRequest struct {
	ID        int
	Method    string
	Response  chan *AcpResponse
	Timeout   time.Duration
	CreatedAt time.Time
}

// StreamCallback is called for each chunk of streaming data
type StreamCallback func(*AcpSessionUpdate) error

// AcpCallbacks contains callbacks for various ACP events
type AcpCallbacks struct {
	// OnSessionUpdate is called for session update notifications
	OnSessionUpdate func(*AcpSessionUpdate) error

	// OnPermission is called when agent requests permission
	OnPermission func(*AcpPermissionRequest) error

	// OnError is called when fatal errors occur
	OnError func(error)

	// OnDisconnect is called when connection is lost
	OnDisconnect func(*AcpDisconnectInfo)
}

// AcpConnection represents an ACP connection to an agent process
type AcpConnection struct {
	mu sync.RWMutex

	// Connection state
	state     atomic.Int32 // ConnectionState
	config    AcpSessionConfig
	process   *exec.Cmd
	processID int

	// stdio pipes
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader

	// Session tracking
	sessionID  string
	hasSession bool

	// Request management
	requestID  atomic.Int32
	pendingReq map[int]*PendingRequest
	reqMu      sync.RWMutex

	// Event handling
	callbacks  AcpCallbacks
	background sync.WaitGroup

	// Error tracking
	lastError error
	lastSeen  atomic.Value // time.Time

	promptDoneCh chan struct{}
	shutdownCh   chan struct{}
}

// Connection interface provides the ACP connection contract
type Connection interface {
	// Initialize starts the connection and process
	Initialize(config AcpSessionConfig) error

	// Close terminates the connection and cleans up resources
	Close() error

	// IsConnected returns whether the connection is active
	IsConnected() bool

	// HasSession returns whether a session is established
	HasSession() bool

	// GetSessionID returns the current session ID
	GetSessionID() string

	// SetSessionMode sets the session mode before creating a session
	SetSessionMode(mode string)

	// NewSession creates a new session with the agent
	NewSession(ctx context.Context) (*SessionNewResult, error)

	// LoadSession loads an existing session
	LoadSession(ctx context.Context, sessionID string) (*SessionLoadResult, error)

	// SendMessage sends a JSON-RPC request and waits for response
	SendMessage(ctx context.Context, method string, params map[string]interface{}, timeout time.Duration) (*AcpResponse, error)

	// SendNotification sends a JSON-RPC notification (no response expected)
	SendNotification(method string, params map[string]interface{}) error

	// StreamPrompt sends a prompt and receives streaming responses
	StreamPrompt(ctx context.Context, sessionID, prompt string, opts AcpPromptOptions, callback StreamCallback) error

	// ConfirmPermission responds to a permission request
	ConfirmPermission(ctx context.Context, callID, optionID string) error

	// GetStatus returns current connection status
	GetStatus() ConnectionStatus

	// SetCallbacks registers event callbacks
	SetCallbacks(callbacks AcpCallbacks)

	// WaitForDone returns a channel that is closed when the current prompt/stream completes
	WaitForDone() <-chan struct{}
}

func (c *AcpConnection) WaitForDone() <-chan struct{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.promptDoneCh != nil {
		return c.promptDoneCh
	}
	done := make(chan struct{})
	close(done)
	return done
}

// NewAcpConnection creates a new ACP connection
func NewAcpConnection() *AcpConnection {
	conn := &AcpConnection{
		pendingReq: make(map[int]*PendingRequest),
		shutdownCh: make(chan struct{}),
	}
	conn.state.Store(int32(ConnectionStateDisconnected))
	return conn
}

// Initialize starts the connection and the agent process
func (c *AcpConnection) Initialize(config AcpSessionConfig) error {
	c.mu.Lock()

	if c.GetState() != ConnectionStateDisconnected {
		c.mu.Unlock()
		return fmt.Errorf("connection already initialized")
	}

	c.config = config
	c.state.Store(int32(ConnectionStateConnecting))

	cliCmd, cliArgs, err := c.buildCommand(config)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to build command: %w", err)
	}

	c.process = exec.Command(cliCmd, cliArgs...)

	c.process.Env = os.Environ()
	for k, v := range config.Env {
		c.process.Env = append(c.process.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if config.Cwd != "" {
		c.process.Dir = config.Cwd
	}

	stdin, err := c.process.StdinPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	c.stdin = stdin

	stdout, err := c.process.StdoutPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	c.stdout = stdout

	stderr, err := c.process.StderrPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	c.stderr = stderr

	if err := c.process.Start(); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to start process: %w", err)
	}

	c.processID = c.process.Process.Pid
	c.lastSeen.Store(time.Now())

	c.background.Add(1)
	go c.readOutputLoop()

	c.background.Add(1)
	go c.readStderrLoop()

	initID := int(c.requestID.Add(1))
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      initID,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": 1,
			"clientInfo": map[string]string{
				"name":    "zeroai",
				"version": "0.1.0",
			},
			"capabilities": map[string]interface{}{},
		},
	}

	initReqBytes, _ := json.Marshal(initReq)

	respCh := make(chan *AcpResponse, 1)
	c.reqMu.Lock()
	c.pendingReq[initID] = &PendingRequest{
		ID:        initID,
		Method:    "initialize",
		Response:  respCh,
		Timeout:   15 * time.Second,
		CreatedAt: time.Now(),
	}
	c.reqMu.Unlock()

	c.mu.Unlock()

	if err := c.sendData(initReqBytes); err != nil {
		c.process.Process.Kill()
		return fmt.Errorf("failed to send initialize: %w", err)
	}

	initCtx, initCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer initCancel()

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			c.process.Process.Kill()
			return fmt.Errorf("initialize error: %v", resp.Error)
		}
	case <-initCtx.Done():
		c.process.Process.Kill()
		return fmt.Errorf("initialize timeout")
	case <-c.shutdownCh:
		c.process.Process.Kill()
		return fmt.Errorf("connection closed during initialize")
	}

	initializedNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifData, _ := json.Marshal(initializedNotif)
	if err := c.sendData(notifData); err != nil {
		c.process.Process.Kill()
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	c.state.Store(int32(ConnectionStateConnected))
	return nil
}

// Close terminates the connection and cleans up resources
func (c *AcpConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.GetState() == ConnectionStateDisconnected {
		return nil
	}

	c.state.Store(int32(ConnectionStateClosing))

	// Signal shutdown
	close(c.shutdownCh)

	// Close stdin first to signal process
	if c.stdin != nil {
		c.stdin.Close()
	}

	// Wait for background readers to finish
	done := make(chan struct{})
	go func() {
		c.background.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Force kill if readers don't stop
		if c.process != nil && c.process.Process != nil {
			c.process.Process.Kill()
		}
	}

	// Clear pending requests
	c.reqMu.Lock()
	for _, req := range c.pendingReq {
		close(req.Response)
	}
	c.pendingReq = make(map[int]*PendingRequest)
	c.reqMu.Unlock()

	// Clear session
	c.sessionID = ""
	c.hasSession = false

	c.state.Store(int32(ConnectionStateDisconnected))
	return nil
}

// IsConnected returns whether the connection is active
func (c *AcpConnection) IsConnected() bool {
	return c.GetState() == ConnectionStateConnected
}

// HasSession returns whether a session is established
func (c *AcpConnection) HasSession() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hasSession
}

// GetSessionID returns the current session ID
func (c *AcpConnection) GetSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// SetSessionMode sets the session mode (yolo/bypass permissions)
func (c *AcpConnection) SetSessionMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.YoloMode = true
}

// GetStatus returns current connection status
func (c *AcpConnection) GetStatus() ConnectionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := ConnectionState(c.state.Load())
	lastSeen, _ := c.lastSeen.Load().(time.Time)

	return ConnectionStatus{
		State:      state,
		HasSession: c.hasSession,
		LastError:  c.lastError,
		LastSeen:   lastSeen,
		ProcessID:  c.processID,
	}
}

// SetCallbacks registers event callbacks
func (c *AcpConnection) SetCallbacks(callbacks AcpCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = callbacks
}

// GetState returns the current connection state
func (c *AcpConnection) GetState() ConnectionState {
	return ConnectionState(c.state.Load())
}

// NewSession creates a new session with the agent
func (c *AcpConnection) NewSession(ctx context.Context) (*SessionNewResult, error) {
	log.Printf("[DEBUG] NewSession: starting for backend=%s", c.config.Backend)

	backendCfg, _ := GetBackendConfig(c.config.Backend)
	displayName := string(c.config.Backend)
	if backendCfg != nil {
		displayName = backendCfg.Name
	}

	cwd := c.config.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	log.Printf("[DEBUG] NewSession: sending session/new with displayName=%s, cwd=%s", displayName, cwd)

	resp, err := c.SendMessage(ctx, "session/new", map[string]interface{}{
		"displayName": displayName,
		"cwd":         cwd,
		"workspace":   cwd,
		"mcpServers":  []interface{}{},
	}, 30*time.Second)
	if err != nil {
		log.Printf("[DEBUG] NewSession: SendMessage error: %v", err)
		return nil, err
	}

	if resp.Error != nil {
		log.Printf("[DEBUG] NewSession: response error: %v", resp.Error)
		return nil, resp.Error
	}

	log.Printf("[DEBUG] NewSession: response received, result=%v", resp.Result)

	var result SessionNewResult
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to encode session result: %w", err)
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse session result: %w", err)
	}

	c.mu.Lock()
	c.sessionID = result.SessionID
	c.hasSession = true
	c.mu.Unlock()

	return &result, nil
}

// LoadSession loads an existing session
func (c *AcpConnection) LoadSession(ctx context.Context, sessionID string) (*SessionLoadResult, error) {
	params := map[string]interface{}{
		"sessionId": sessionID,
	}
	if c.config.Cwd != "" {
		params["cwd"] = c.config.Cwd
	}

	resp, err := c.SendMessage(ctx, "session/load", params, 30*time.Second)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	var result SessionLoadResult
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to encode load result: %w", err)
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse load result: %w", err)
	}

	if result.SessionID != "" {
		c.mu.Lock()
		c.sessionID = result.SessionID
		c.hasSession = true
		c.mu.Unlock()
	}

	return &result, nil
}

// SendMessage sends a JSON-RPC request and waits for response
func (c *AcpConnection) SendMessage(ctx context.Context, method string, params map[string]interface{}, timeout time.Duration) (*AcpResponse, error) {
	if !c.IsConnected() {
		return nil, &AcpError{
			Type:    ErrorConnection,
			Message: "not connected",
		}
	}

	// Generate request ID
	id := int(c.requestID.Add(1))

	// Create pending request
	respCh := make(chan *AcpResponse, 1)
	req := &PendingRequest{
		ID:        id,
		Method:    method,
		Response:  respCh,
		Timeout:   timeout,
		CreatedAt: time.Now(),
	}

	// Register pending request
	c.reqMu.Lock()
	c.pendingReq[id] = req
	c.reqMu.Unlock()

	// Clean up pending request on exit
	defer func() {
		c.reqMu.Lock()
		delete(c.pendingReq, id)
		c.reqMu.Unlock()
		close(respCh)
	}()

	// Encode and send request
	data, err := EncodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	if err := c.sendData(data); err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		return nil, &AcpError{
			Type:    ErrorTimeout,
			Message: "context canceled while waiting for response",
		}
	case <-time.After(timeout):
		return nil, &AcpError{
			Type:    ErrorTimeout,
			Message: fmt.Sprintf("request timed out after %v", timeout),
		}
	case <-c.shutdownCh:
		return nil, &AcpError{
			Type:    ErrorConnection,
			Message: "connection shutdown",
		}
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (c *AcpConnection) SendNotification(method string, params map[string]interface{}) error {
	if !c.IsConnected() {
		return &AcpError{
			Type:    ErrorConnection,
			Message: "not connected",
		}
	}

	data, err := EncodeNotification(method, params)
	if err != nil {
		return err
	}

	return c.sendData(data)
}

// StreamPrompt sends a prompt and receives streaming responses via session/update notifications
func (c *AcpConnection) StreamPrompt(ctx context.Context, sessionID, prompt string, opts AcpPromptOptions, callback StreamCallback) error {
	log.Printf("[DEBUG] StreamPrompt: sending session/prompt with params: sessionId=%s, prompt=%s", sessionID, prompt)

	params := map[string]interface{}{
		"sessionId": sessionID,
		"prompt": []map[string]interface{}{
			{
				"type": "text",
				"text": prompt,
			},
		},
	}
	if len(opts.Files) > 0 {
		params["files"] = opts.Files
	}
	if opts.ModelOverride != "" {
		params["model"] = opts.ModelOverride
	}

	// Send session/prompt request - wait for response
	log.Printf("[DEBUG] StreamPrompt: about to call SendMessage for session/prompt")
	resp, err := c.SendMessage(ctx, "session/prompt", params, 5*time.Minute)
	log.Printf("[DEBUG] StreamPrompt: SendMessage returned, err=%v, resp=%v", err, resp)

	if err != nil {
		log.Printf("[DEBUG] StreamPrompt: SendMessage error: %v", err)
		return err
	}
	if resp == nil {
		log.Printf("[DEBUG] StreamPrompt: response is nil!")
		return nil
	}

	log.Printf("[DEBUG] StreamPrompt: response received successfully, result=%v", resp.Result)

	log.Printf("[DEBUG] StreamPrompt: sent successfully, waiting for notifications")

	c.mu.Lock()
	c.promptDoneCh = make(chan struct{})
	c.mu.Unlock()

	return nil
}

// ConfirmPermission responds to a permission request
func (c *AcpConnection) ConfirmPermission(ctx context.Context, callID, optionID string) error {
	_, err := c.SendMessage(ctx, "permission/confirm", map[string]interface{}{
		"callId":   callID,
		"optionId": optionID,
	}, 10*time.Second)
	return err
}

// sendData writes data to stdin with newline
func (c *AcpConnection) sendData(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stdin == nil {
		return &AcpError{
			Type:    ErrorConnection,
			Message: "stdin not available",
		}
	}

	log.Printf("[DEBUG] sendData: sending %d bytes: %s", len(data), string(data))

	// Ensure message ends with newline
	buf := data
	if !bytes.HasSuffix(buf, []byte("\n")) {
		buf = append(buf, '\n')
	}

	if _, err := c.stdin.Write(buf); err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	if flusher, ok := c.stdin.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return fmt.Errorf("failed to flush stdin: %w", err)
		}
	}

	c.lastSeen.Store(time.Now())
	return nil
}

// readOutputLoop reads and processes JSON-RPC messages from stdout
func (c *AcpConnection) readOutputLoop() {
	defer c.background.Done()

	scanner := bufio.NewScanner(c.stdout)

	for {
		select {
		case <-c.shutdownCh:
			return
		default:
		}

		if !scanner.Scan() {
			// Process ended or error
			if err := scanner.Err(); err != nil {
				c.handleDisconnect(&AcpDisconnectInfo{
					Reason: "stdout read error",
				})
			} else {
				c.handleDisconnect(&AcpDisconnectInfo{
					Reason: "process ended",
				})
			}
			return
		}

		line := scanner.Bytes()
		c.lastSeen.Store(time.Now())

		log.Printf("[DEBUG] readOutputLoop: received %d bytes: %s", len(line), string(line))

		// Decode message
		msg, err := DecodeMessage(line)
		if err != nil {
			c.logError(fmt.Sprintf("decode error: %v", err))
			continue
		}

		// Process message
		switch m := msg.(type) {
		case *AcpResponse:
			c.handleResponse(m)
		case *AcpNotification:
			c.handleNotification(m)
		}
	}
}

// readStderrLoop reads stderr for potential error messages
func (c *AcpConnection) readStderrLoop() {
	defer c.background.Done()

	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// Log stderr messages - could be sent to a callback
		c.logError(fmt.Sprintf("stderr: %s", line))
	}
}

// handleResponse routes a response to the pending request or callbacks
func (c *AcpConnection) handleResponse(resp *AcpResponse) {
	log.Printf("[DEBUG] handleResponse: received response for id=%v, hasResult=%v, hasError=%v", resp.ID, resp.Result != nil, resp.Error != nil)

	c.reqMu.RLock()
	req, exists := c.pendingReq[resp.ID]
	c.reqMu.RUnlock()

	if exists {
		log.Printf("[DEBUG] handleResponse: found pending request, sending response")
		select {
		case req.Response <- resp:
		default:
			log.Printf("[DEBUG] handleResponse: channel full or closed")
		}
	} else {
		log.Printf("[DEBUG] handleResponse: no pending request found for id=%v", resp.ID)
	}
}

// handleNotification processes incoming notifications
func (c *AcpConnection) handleNotification(notif *AcpNotification) {
	log.Printf("[DEBUG] handleNotification: received method=%s", notif.Method)
	switch notif.Method {
	case "session/update":
		c.handleSessionUpdate(notif.Params)
	case "permission/request":
		c.handlePermissionRequest(notif.Params)
	case "error":
		c.handleErrorNotification(notif.Params)
	case "notifications/initialized":
		log.Printf("[DEBUG] handleNotification: ignored notifications/initialized")
	default:
		log.Printf("[DEBUG] handleNotification: unhandled method=%s", notif.Method)
	}
}

// handleSessionUpdate processes session update notifications
func (c *AcpConnection) handleSessionUpdate(params map[string]interface{}) {
	log.Printf("[DEBUG] handleSessionUpdate: params keys = %v", reflect.ValueOf(params).MapKeys())

	updateObj, hasUpdate := params["update"].(map[string]interface{})
	if !hasUpdate {
		log.Printf("[DEBUG] handleSessionUpdate: no 'update' key in params")
		return
	}

	sessionUpdate, _ := updateObj["sessionUpdate"].(string)
	log.Printf("[DEBUG] handleSessionUpdate: sessionUpdate type = %s", sessionUpdate)

	data, err := json.Marshal(updateObj)
	if err != nil {
		c.logError(fmt.Sprintf("failed to encode session update: %v", err))
		return
	}

	log.Printf("[DEBUG] handleSessionUpdate: raw update = %s", string(data))

	var update AcpSessionUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		c.logError(fmt.Sprintf("failed to parse session update: %v", err))
		return
	}

	update.SessionUpdate = sessionUpdate

	switch v := update.Content.(type) {
	case string:
	case map[string]interface{}:
		if text, ok := v["text"].(string); ok {
			update.Content = text
		}
	}

	log.Printf("[DEBUG] handleSessionUpdate: parsed SessionUpdate=%s, Content=%v", update.SessionUpdate, update.Content)

	if update.SessionUpdate == "end_turn" {
		c.mu.Lock()
		if c.promptDoneCh != nil {
			close(c.promptDoneCh)
			c.promptDoneCh = nil
		}
		c.mu.Unlock()
	}

	log.Printf("[DEBUG] handleSessionUpdate: update object keys = %v", reflect.ValueOf(updateObj).MapKeys())

	c.mu.RLock()
	callbacks := c.callbacks
	c.mu.RUnlock()

	if callbacks.OnSessionUpdate != nil {
		log.Printf("[DEBUG] handleSessionUpdate: calling OnSessionUpdate callback")
		if err := callbacks.OnSessionUpdate(&update); err != nil {
			c.logError(fmt.Sprintf("session update callback error: %v", err))
		}
		log.Printf("[DEBUG] handleSessionUpdate: callback returned")
	}
}

// handlePermissionRequest processes permission request notifications
func (c *AcpConnection) handlePermissionRequest(params map[string]interface{}) {
	var permReq AcpPermissionRequest
	data, err := json.Marshal(params)
	if err != nil {
		c.logError(fmt.Sprintf("failed to encode permission request: %v", err))
		return
	}

	if err := json.Unmarshal(data, &permReq); err != nil {
		c.logError(fmt.Sprintf("failed to parse permission request: %v", err))
		return
	}

	c.mu.RLock()
	callbacks := c.callbacks
	c.mu.RUnlock()

	if callbacks.OnPermission != nil {
		if err := callbacks.OnPermission(&permReq); err != nil {
			c.logError(fmt.Sprintf("permission callback error: %v", err))
		}
	}
}

// handleErrorNotification processes error notifications
func (c *AcpConnection) handleErrorNotification(params map[string]interface{}) {
	var errMsg struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	data, err := json.Marshal(params)
	if err != nil {
		return
	}
	json.Unmarshal(data, &errMsg)

	c.logError(fmt.Sprintf("notification error: %s - %s", errMsg.Type, errMsg.Message))
}

// handleDisconnect handles disconnection events
func (c *AcpConnection) handleDisconnect(info *AcpDisconnectInfo) {
	c.state.Store(int32(ConnectionStateDisconnected))

	c.mu.RLock()
	callbacks := c.callbacks
	c.mu.RUnlock()

	if callbacks.OnDisconnect != nil {
		callbacks.OnDisconnect(info)
	}
}

// logError logs errors - could be enhanced with proper logging
func (c *AcpConnection) logError(msg string) {
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

// buildCommand builds the CLI command and arguments based on backend
func (c *AcpConnection) buildCommand(config AcpSessionConfig) (string, []string, error) {
	backendCfg, err := GetBackendConfig(config.Backend)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get backend config: %w", err)
	}

	// If backend has an NPX package, use npx to run it
	if backendCfg.NpxPackage != "" {
		return "npx", []string{"--yes", backendCfg.NpxPackage}, nil
	}

	cliPath := config.CliPath
	if cliPath == "" {
		cliPath = backendCfg.DefaultCliPath
	}

	if cliPath == "" {
		return "", nil, fmt.Errorf("no CLI path specified and no default found")
	}

	// Build ACP arguments
	args := []string{}
	args = append(args, backendCfg.AcpArgs...)

	return cliPath, args, nil
}
