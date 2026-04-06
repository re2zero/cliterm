package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func startMockCli(t *testing.T) (*AcpConnection, func()) {
	connStdinR, connStdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	cliStdoutR, cliStdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, cliStderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(connStdinR)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line := scanner.Bytes()
			var msg map[string]interface{}
			if err := json.Unmarshal(line, &msg); err != nil {
				continue
			}
			method, _ := msg["method"].(string)
			id := 0
			if idVal, ok := msg["id"].(float64); ok {
				id = int(idVal)
			}
			switch method {
			case "initialize":
				resp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]interface{}{
						"protocolVersion": 1,
						"capabilities":    map[string]interface{}{},
						"serverInfo": map[string]interface{}{
							"name":    "mock-cli",
							"version": "0.1.0",
						},
					},
				}
				data, _ := json.Marshal(resp)
				cliStdoutW.Write(append(data, '\n'))
			case "session/new":
				resp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]interface{}{
						"sessionId": "mock-session-123",
						"models": map[string]interface{}{
							"default": "mock-model",
						},
					},
				}
				data, _ := json.Marshal(resp)
				cliStdoutW.Write(append(data, '\n'))
			case "session/prompt":
				resp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"result":  map[string]interface{}{"status": "processing"},
				}
				data, _ := json.Marshal(resp)
				cliStdoutW.Write(append(data, '\n'))

				go func() {
					time.Sleep(50 * time.Millisecond)
					update1 := map[string]interface{}{
						"jsonrpc": "2.0",
						"method":  "session/update",
						"params": map[string]interface{}{
							"sessionId":     "mock-session-123",
							"sessionUpdate": "agent_message_chunk",
							"content":       "Hello from mock agent!",
						},
					}
					d1, _ := json.Marshal(update1)
					cliStdoutW.Write(append(d1, '\n'))

					time.Sleep(50 * time.Millisecond)
					update2 := map[string]interface{}{
						"jsonrpc": "2.0",
						"method":  "session/update",
						"params": map[string]interface{}{
							"sessionId":     "mock-session-123",
							"sessionUpdate": "end_turn",
						},
					}
					d2, _ := json.Marshal(update2)
					cliStdoutW.Write(append(d2, '\n'))
				}()
			}
		}
	}()

	conn := NewAcpConnection()
	conn.stdin = connStdinW
	conn.stdout = cliStdoutR
	conn.stderr = cliStderrW
	conn.processID = -1

	cleanup := func() {
		cancel()
		connStdinW.Close()
		cliStdoutW.Close()
		cliStderrW.Close()
		conn.Close()
		wg.Wait()
	}

	return conn, cleanup
}

func TestAcpConnection_FullFlow(t *testing.T) {
	conn, cleanup := startMockCli(t)
	defer cleanup()

	conn.state.Store(int32(ConnectionStateConnecting))

	conn.background.Add(1)
	go conn.readOutputLoop()

	initID := int(conn.requestID.Add(1))
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
		},
	}
	initData, _ := json.Marshal(initReq)

	respCh := make(chan *AcpResponse, 1)
	conn.reqMu.Lock()
	conn.pendingReq[initID] = &PendingRequest{
		ID:       initID,
		Method:   "initialize",
		Response: respCh,
		Timeout:  5 * time.Second,
	}
	conn.reqMu.Unlock()

	if err := conn.sendData(initData); err != nil {
		t.Fatalf("sendData: %v", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			t.Fatalf("initialize error: %v", resp.Error)
		}
		t.Logf("initialize response: %v", resp.Result)
	case <-time.After(5 * time.Second):
		t.Fatal("initialize timeout")
	}
	conn.state.Store(int32(ConnectionStateConnected))

	ctx := context.Background()
	result, err := conn.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if result.SessionID != "mock-session-123" {
		t.Errorf("expected 'mock-session-123', got '%s'", result.SessionID)
	}
	t.Logf("session: %s", result.SessionID)

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

	err = conn.StreamPrompt(ctx, result.SessionID, "Hello test", AcpPromptOptions{}, nil)
	if err != nil {
		t.Fatalf("StreamPrompt: %v", err)
	}

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-deadline:
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
		if strings.HasPrefix(u, "agent_message_chunk:") {
			hasContent = true
		}
		if strings.HasPrefix(u, "end_turn:") {
			hasEndTurn = true
		}
	}
	if !hasContent {
		t.Errorf("missing agent_message_chunk update in: %v", receivedUpdates)
	}
	if !hasEndTurn {
		t.Errorf("missing end_turn update in: %v", receivedUpdates)
	}
	t.Logf("received updates: %v", receivedUpdates)
}

func TestAcpConnection_NoDeadlock(t *testing.T) {
	conn := NewAcpConnection()
	conn.state.Store(int32(ConnectionStateDisconnected))

	done := make(chan error, 1)
	go func() {
		done <- conn.Initialize(AcpSessionConfig{
			Backend: AcpBackendClaude,
			CliPath: "/nonexistent/cli",
		})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error from nonexistent CLI")
		}
		t.Logf("correctly failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Initialize deadlocked - timed out")
	}
}
