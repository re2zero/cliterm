package protocol

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClaudeStreamConnection_RealClaudeCLI(t *testing.T) {
	cliPath, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude CLI not found")
	}

	env := map[string]string{
		"NO_COLOR": "1",
	}

	homeDir, _ := os.UserHomeDir()
	settingsFile := homeDir + "/.claude/settings.json"
	if _, err := os.Stat(settingsFile); err != nil {
		t.Skip("Claude settings.json not found")
	}

	conn := NewClaudeStreamConnection()

	var updates []string
	var updatesMu sync.Mutex

	conn.SetCallbacks(AcpCallbacks{
		OnSessionUpdate: func(update *AcpSessionUpdate) error {
			updatesMu.Lock()
			updates = append(updates, fmt.Sprintf("%s:%s", update.SessionUpdate, update.Content))
			updatesMu.Unlock()
			return nil
		},
		OnError: func(err error) {
			t.Logf("error: %v", err)
		},
		OnDisconnect: func(info *AcpDisconnectInfo) {
			t.Logf("disconnected: %s", info.Reason)
		},
	})

	config := AcpSessionConfig{
		Backend:  AcpBackendClaude,
		CliPath:  cliPath,
		YoloMode: true,
		Env:      env,
	}

	if err := conn.Initialize(config); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	t.Log("Initialize succeeded")
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := conn.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	t.Logf("Session created: %s", result.SessionID)

	err = conn.StreamPrompt(ctx, result.SessionID, "Say just 'hello' and nothing else", AcpPromptOptions{}, nil)
	if err != nil {
		t.Fatalf("StreamPrompt failed: %v", err)
	}
	t.Log("StreamPrompt sent, waiting for updates...")

	deadline := time.After(120 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			updatesMu.Lock()
			t.Fatalf("timeout. Updates: %v", updates)
		case <-ticker.C:
			updatesMu.Lock()
			gotEnd := false
			for _, u := range updates {
				if strings.HasPrefix(u, "end_turn:") {
					gotEnd = true
				}
			}
			cnt := len(updates)
			updatesMu.Unlock()
			if cnt > 0 && cnt%5 == 0 {
				t.Logf("update count: %d", cnt)
			}
			if gotEnd {
				updatesMu.Lock()
				t.Logf("All updates (%d): %v", len(updates), updates)
				updatesMu.Unlock()
				return
			}
		}
	}
}
