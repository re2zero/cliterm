package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClaudeStreamConnection_Initialize(t *testing.T) {
	conn := NewClaudeStreamConnection()

	config := AcpSessionConfig{
		Backend: AcpBackendClaude,
		CliPath: "claude",
	}

	err := conn.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !conn.IsConnected() {
		t.Fatal("expected connected after Initialize")
	}
	conn.Close()
}

func TestClaudeStreamConnection_StreamPrompt_WithMock(t *testing.T) {
	cliStdoutR, cliStdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, cliStderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	go func() {
		msg1 := map[string]interface{}{
			"type": "assistant",
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Hello from Claude mock!",
					},
				},
			},
		}
		d1, _ := json.Marshal(msg1)
		cliStdoutW.Write(append(d1, '\n'))

		msg2 := map[string]interface{}{
			"type":           "result",
			"result":         "Hello from Claude mock!",
			"session_id":     "mock-session-456",
			"total_cost_usd": 0.001,
			"duration_ms":    150,
			"num_turns":      1,
		}
		d2, _ := json.Marshal(msg2)
		cliStdoutW.Write(append(d2, '\n'))
		cliStdoutW.Close()
	}()
	cliStderrW.Close()

	conn := NewClaudeStreamConnection()
	conn.cliPath = "mock"
	conn.state.Store(int32(ConnectionStateConnected))

	var receivedUpdates []string
	var updatesMu sync.Mutex
	conn.SetCallbacks(AcpCallbacks{
		OnSessionUpdate: func(update *AcpSessionUpdate) error {
			updatesMu.Lock()
			receivedUpdates = append(receivedUpdates, fmt.Sprintf("%s:%s", update.SessionUpdate, update.Content))
			updatesMu.Unlock()
			return nil
		},
	})

	go conn.readProcessOutput(cliStdoutR, nil)

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-deadline:
			updatesMu.Lock()
			t.Fatalf("timeout waiting for updates, got: %v", receivedUpdates)
		case <-ticker.C:
			updatesMu.Lock()
			done := len(receivedUpdates) >= 2
			updatesMu.Unlock()
			if done {
				break waitLoop
			}
		}
	}

	updatesMu.Lock()
	defer updatesMu.Unlock()
	hasContent := false
	hasEndTurn := false
	for _, u := range receivedUpdates {
		if strings.Contains(u, "Hello from Claude mock!") {
			hasContent = true
		}
		if strings.HasPrefix(u, "end_turn:") {
			hasEndTurn = true
		}
	}
	if !hasContent {
		t.Errorf("missing text content in updates: %v", receivedUpdates)
	}
	if !hasEndTurn {
		t.Errorf("missing end_turn in updates: %v", receivedUpdates)
	}
	t.Logf("received updates: %v", receivedUpdates)

	conn.Close()
}

func TestClaudeStreamConnection_ParseAssistantMessage(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantType    string
		wantSubtype string
		wantContent string
	}{
		{
			name:        "text message",
			json:        `{"type":"assistant","subtype":"text","content":"Hello world"}`,
			wantType:    "assistant",
			wantSubtype: "text",
			wantContent: "Hello world",
		},
		{
			name:        "tool_use message",
			json:        `{"type":"assistant","subtype":"tool_use","tool_name":"Read","tool_call_id":"call_123","content":"Reading file"}`,
			wantType:    "assistant",
			wantSubtype: "tool_use",
			wantContent: "Reading file",
		},
		{
			name:        "system message",
			json:        `{"type":"system","subtype":"init","session_id":"sess-789"}`,
			wantType:    "system",
			wantSubtype: "init",
			wantContent: "",
		},
		{
			name:        "result message",
			json:        `{"type":"result","result":"done","session_id":"sess-789"}`,
			wantType:    "result",
			wantSubtype: "",
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg ClaudeMessage
			if err := json.Unmarshal([]byte(tt.json), &msg); err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			if msg.Type != tt.wantType {
				t.Errorf("got type %q, want %q", msg.Type, tt.wantType)
			}
			if msg.Subtype != tt.wantSubtype {
				t.Errorf("got subtype %q, want %q", msg.Subtype, tt.wantSubtype)
			}
			if msg.Content != tt.wantContent {
				t.Errorf("got content %q, want %q", msg.Content, tt.wantContent)
			}
		})
	}
}

func TestClaudeStreamConnection_ConvertToSessionUpdate(t *testing.T) {
	conn := NewClaudeStreamConnection()

	textMsg := ClaudeMessage{
		Type:    "assistant",
		Subtype: "text",
		Content: "Hello!",
	}
	update := conn.convertToSessionUpdate(&textMsg)
	if update.SessionUpdate != "text" {
		t.Errorf("expected 'text', got %q", update.SessionUpdate)
	}
	if update.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", update.Content)
	}

	toolMsg := ClaudeMessage{
		Type:       "assistant",
		Subtype:    "tool_use",
		ToolName:   "Read",
		ToolCallID: "call_123",
		Content:    "Reading file",
	}
	update = conn.convertToSessionUpdate(&toolMsg)
	if update.SessionUpdate != "tool_call" {
		t.Errorf("expected 'tool_call', got %q", update.SessionUpdate)
	}
	if update.ToolCall == nil {
		t.Fatal("expected tool call data")
	}
	if update.ToolCall.ToolName != "Read" {
		t.Errorf("expected tool name 'Read', got %q", update.ToolCall.ToolName)
	}
}

func TestClaudeStreamConnection_BuildCommand(t *testing.T) {
	conn := NewClaudeStreamConnection()
	cmd, args, err := conn.buildCommand(AcpSessionConfig{
		Backend: AcpBackendClaude,
		CliPath: "/usr/local/bin/claude",
	})
	if err != nil {
		t.Fatalf("buildCommand failed: %v", err)
	}
	if cmd != "/usr/local/bin/claude" {
		t.Errorf("expected /usr/local/bin/claude, got %q", cmd)
	}
	expectedArgs := []string{"-p", "--verbose", "--output-format", "stream-json"}
	if len(args) != len(expectedArgs) {
		t.Errorf("expected %v args, got %v: %v", len(expectedArgs), len(args), args)
	}
	for i, arg := range expectedArgs {
		if args[i] != arg {
			t.Errorf("arg[%d]: expected %q, got %q", i, arg, args[i])
		}
	}
}

func TestClaudeStreamConnection_BuildCommandWithModel(t *testing.T) {
	conn := NewClaudeStreamConnection()
	_, args, err := conn.buildCommand(AcpSessionConfig{
		Backend: AcpBackendClaude,
		CliPath: "claude",
		Model:   "claude-opus-4-20250514",
	})
	if err != nil {
		t.Fatalf("buildCommand failed: %v", err)
	}
	found := false
	for _, arg := range args {
		if arg == "--model" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --model in args: %v", args)
	}
}

func TestClaudeStreamConnection_NoSessionNeeded(t *testing.T) {
	conn := NewClaudeStreamConnection()
	if conn.HasSession() {
		t.Error("Claude stream connection should not need explicit sessions initially")
	}
	if conn.GetSessionID() != "" {
		t.Error("Claude stream should have empty session ID initially")
	}
}
