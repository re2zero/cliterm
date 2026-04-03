// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package acpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ACPConfig 表示 ACP 客户端配置
type ACPConfig struct {
	CLI      string
	Model    string
	AcpxPath string
	WorkDir  string
	CmdArgs  []string
	EnvVars  map[string]string
}

// ACPClient 表示 ACP 客户端
type ACPClient struct {
	config      *ACPConfig
	transport   *StdioTransport
	sessionID   string
	initialized bool
	mu          struct {
		sync.Mutex
		nextID   int
		messages map[int]chan *ACPMessage
	}
}

// NPX package versions for different ACP backends
const (
	CodexACPBridgeVersion     = "0.9.5"
	CodeBuddyACPBridgeVersion = "0.1.0" // approximate
	ClaudeACPBridgeVersion    = "0.18.0"
)

const (
	jsonRPCVersion = "2.0"
)

// getNPXCommand returns the npx package string for agents that use npx
func getNPXCommand(cli string) (string, bool) {
	switch cli {
	case "codex":
		return fmt.Sprintf("@zed-industries/codex-acp@%s", CodexACPBridgeVersion), true
	case "claude":
		return fmt.Sprintf("@zed-industries/claude-agent-acp@%s", ClaudeACPBridgeVersion), true
	case "codebuddy":
		return fmt.Sprintf("@tencent-ai/codebuddy-code"), true
	default:
		return "", false
	}
}

// getAcpArgs returns ACP mode arguments for different agents
// Based on AionUi reference: https://github.com/AionUi/multi-agents-pre
func getAcpArgs(cli string) []string {
	switch cli {
	case "goose", "opencode", "kimi":
		return []string{"acp"}
	case "gemini":
		// Gemini CLI uses 'acp' subcommand or no args for ACP mode
		return []string{"acp"}
	case "droid":
		return []string{"exec", "--output-format", "acp"}
	case "codex":
		return []string{} // npx codex-acp is ACP by default
	case "copilot":
		return []string{"--acp", "--stdio"}
	case "claude":
		// npx @zed-industries/claude-agent-acp is ACP by default, no args needed
		return []string{}
	case "codebuddy":
		// npx @tencent-ai/codebuddy-code is ACP by default, no args
		return []string{}
	case "qwen", "auggie", "qoder", "nanobot":
		return []string{"--acp"}
	default:
		return []string{"--experimental-acp"}
	}
}

// NewACPClient 创建新的 ACP 客户端
func NewACPClient(config *ACPConfig) *ACPClient {
	command := config.CLI
	if command == "" {
		command = config.Model
	}

	client := &ACPClient{
		config: config,
	}

	client.mu.messages = make(map[int]chan *ACPMessage)

	fmt.Printf("[ACP-CLIENT] NewACPClient created for CLI=%s Model=%s\n", config.CLI, config.Model)

	return client
}

// Initialize 初始化 ACP 连接
func (c *ACPClient) Initialize(ctx context.Context) error {
	// Generate ID before taking lock (avoid deadlock with nextID which also takes lock)
	msgID := c.nextID()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	fmt.Printf("[ACP-CLIENT] Initialize: CLI=%s Model=%s\n", c.config.CLI, c.config.Model)

	// Check if this agent uses npx
	npxPackage, useNpx := getNPXCommand(c.config.CLI)
	var command string
	var args []string

	if useNpx {
		command = "npx"
		args = []string{"--yes", npxPackage}
		fmt.Printf("[ACP-CLIENT] Using NPX: package=%s\n", npxPackage)
	} else {
		command = c.config.AcpxPath
		if command == "" {
			command = c.config.CLI
		}
		args = getAcpArgs(c.config.CLI)
		fmt.Printf("[ACP-CLIENT] Using direct command: %s %v\n", command, args)
	}

	c.transport = NewStdioTransport(command, args)

	if err := c.transport.Connect(ctx); err != nil {
		fmt.Printf("[ACP-CLIENT] Transport Connect failed: %v\n", err)
		return fmt.Errorf("failed to connect to agent: %w", err)
	}
	fmt.Printf("[ACP-CLIENT] Transport connected successfully\n")

	fmt.Printf("[ACP-CLIENT] About to create initMsg (id=%d)...\n", msgID)
	fmt.Printf("[ACP-CLIENT] About to create initMsg (id=%d)...\n", msgID)

	initMsg := &ACPMessage{
		JSONRPC: jsonRPCVersion,
		ID:      msgID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": 1,
			"capabilities": map[string]interface{}{
				"resources": map[string]interface{}{},
				"tools":     map[string]interface{}{},
			},
		},
	}
	fmt.Printf("[ACP-CLIENT] initMsg created, about to WriteMessage...\n")

	if err := c.transport.WriteMessage(initMsg); err != nil {
		c.transport.Close()
		fmt.Printf("[ACP-CLIENT] Failed to send initialize: %v\n", err)
		return fmt.Errorf("failed to send initialize: %w", err)
	}
	fmt.Printf("[ACP-CLIENT] Initialize request sent\n")

	response, err := c.transport.ReadMessage()
	if err != nil {
		c.transport.Close()
		fmt.Printf("[ACP-CLIENT] Failed to read initialize response: %v\n", err)
		return fmt.Errorf("failed to read initialize response: %w", err)
	}
	fmt.Printf("[ACP-CLIENT] Initialize response received\n")

	if response.Error != nil {
		c.transport.Close()
		fmt.Printf("[ACP-CLIENT] Initialize error: %v\n", response.Error)
		return fmt.Errorf("initialize error: %w", response.Error)
	}

	c.initialized = true
	fmt.Printf("[ACP-CLIENT] Initialize completed successfully\n")
	return nil
}

// CreateSession 创建新会话
func (c *ACPClient) CreateSession(ctx context.Context) (string, error) {
	// Generate ID before taking lock (avoid deadlock with nextID which also takes lock)
	msgID := c.nextID()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return "", NewProtocolError("client not initialized")
	}

	req := &ACPMessage{
		JSONRPC: "2.0",
		ID:      msgID,
		Method:  "session/new",
		Params: ACPSessionNewRequest{
			DisplayName: fmt.Sprintf("%s@%s", c.config.CLI, c.config.Model),
			Workspace:   c.config.WorkDir,
			CWD:         c.config.WorkDir, // Required field for ACP protocol
			MCPServers:  []interface{}{},  // Empty MCP servers for now
		},
	}

	if err := c.transport.WriteMessage(req); err != nil {
		return "", err
	}

	response, err := c.transport.ReadMessage()
	if err != nil {
		return "", err
	}

	if response.Error != nil {
		return "", response.Error
	}

	var result ACPSessionNewResponse
	resultBytes, _ := json.Marshal(response.Result)
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return "", fmt.Errorf("failed to parse session response: %w", err)
	}

	c.sessionID = result.SessionID
	return c.sessionID, nil
}

// SendPrompt 发送 prompt
func (c *ACPClient) SendPrompt(ctx context.Context, prompt string) (<-chan *ACPMessage, <-chan error) {
	// Generate ID before taking lock (avoid deadlock with nextID which also takes lock)
	msgID := c.nextID()

	msgCh := make(chan *ACPMessage, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(msgCh)
		defer close(errCh)

		c.mu.Lock()

		if !c.initialized {
			c.mu.Unlock()
			errCh <- NewProtocolError("client not initialized")
			return
		}

		if c.sessionID == "" {
			c.mu.Unlock()
			errCh <- NewProtocolError("no session created")
			return
		}

		req := NewSessionPromptRequest(c.sessionID, prompt)
		req.ID = msgID

		if err := c.transport.WriteMessage(req); err != nil {
			c.mu.Unlock()
			errCh <- err
			return
		}
		c.mu.Unlock()

		// 读取流式响应
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("[ACP-CLIENT] Context cancelled\n")
				errCh <- ctx.Err()
				return
			default:
			}

			msg, err := c.transport.ReadMessage()
			if err != nil {
				fmt.Printf("[ACP-CLIENT] ReadMessage error: %v\n", err)
				errCh <- err
				return
			}

			msgJSON, _ := json.Marshal(msg)
			fmt.Printf("[ACP-CLIENT] Received message: %s\n", string(msgJSON))

			msgCh <- msg

			// 检查是否是最终响应（无 method 字段表示响应完成）
			if msg.Method == "" {
				fmt.Printf("[ACP-CLIENT] Final response received, exiting\n")
				return
			}
		}
	}()

	return msgCh, errCh
}

// Close 关闭客户端
func (c *ACPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.transport != nil {
		c.transport.Close()
	}

	return nil
}

// nextID 生成下一个消息 ID
func (c *ACPClient) nextID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.mu.nextID
	c.mu.nextID++
	return id
}

// GetSessionID 获取当前会话 ID
func (c *ACPClient) GetSessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}
