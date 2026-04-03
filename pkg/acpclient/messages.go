// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package acpclient

import "encoding/json"

// ACPMessage 表示 JSON-RPC 2.0 消息
type ACPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *ACPError   `json:"error,omitempty"`
}

// ACPMessageContent 表示 ACP 消息内容
type ACPMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// PromptBlock 表示 prompt 消息
type PromptBlock struct {
	Role    string              `json:"role,omitempty"`
	Content []ACPMessageContent `json:"content,omitempty"`
}

// ACPPromptItem 表示 session/prompt 的 prompt 数组项
type ACPPromptItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// AIAPIInfo 表示 AI API 信息
type AIAPIInfo struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Version  string `json:"version,omitempty"`
}

// ACPSessionNewRequest 表示会话创建请求
type ACPSessionNewRequest struct {
	DisplayName string        `json:"displayName"`
	Workspace   string        `json:"workspace,omitempty"`
	CWD         string        `json:"cwd"`
	MCPServers  []interface{} `json:"mcpServers"`
	AIAPI       *AIAPIInfo    `json:"ai,omitempty"`
}

// ACPSessionNewResponse 表示会话创建响应
type ACPSessionNewResponse struct {
	SessionID string `json:"sessionId"`
}

// ACPSessionPromptRequest 表示 prompt 请求
type ACPSessionPromptRequest struct {
	SessionID string          `json:"sessionId"`
	Prompt    []ACPPromptItem `json:"prompt"`
}

// ACPSessionPromptResponse 表示 prompt 响应
type ACPSessionPromptResponse struct {
	StopReason string              `json:"stopReason"`
	Content    []ACPMessageContent `json:"content"`
}

// ACPUpdateContent 表示 session/update 中的 content
type ACPUpdateContent struct {
	Text        string `json:"text"`
	Type        string `json:"type"`
	DiffPath    string `json:"path,omitempty"`
	DiffOldText string `json:"oldText,omitempty"`
	DiffNewText string `json:"newText,omitempty"`
}

// ACPSessionUpdateNotification 表示会话更新通知
type ACPSessionUpdateNotification struct {
	SessionID string           `json:"sessionId"`
	Update    *ACPUpdateDetail `json:"update,omitempty"`
}

// ACPUpdateDetail 表示 update 字段的详细内容
type ACPUpdateDetail struct {
	SessionUpdate     string                 `json:"sessionUpdate"`
	Content           *ACPUpdateContent      `json:"content,omitempty"`
	ToolCallID        string                 `json:"toolCallId,omitempty"`
	Status            string                 `json:"status,omitempty"`
	Title             string                 `json:"title,omitempty"`
	Kind              string                 `json:"kind,omitempty"`
	RawInput          map[string]interface{} `json:"rawInput,omitempty"`
	AvailableCommands []ACPAvailableCommand  `json:"availableCommands,omitempty"`
	Entries           []ACPPlanEntry         `json:"entries,omitempty"`
}

// ACPAvailableCommand 表示可用命令
type ACPAvailableCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Input       *struct {
		Hint *string `json:"hint"`
	} `json:"input"`
}

// ACPPlanEntry 表示计划条目
type ACPPlanEntry struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority,omitempty"`
}

// ACPPermissionRequest 表示权限请求
type ACPPermissionRequest struct {
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
}

// NewSessionNewRequest 创建会话创建请求
func NewSessionNewRequest(displayName string, workspace string, cwd string) *ACPMessage {
	return &ACPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "session/new",
		Params: ACPSessionNewRequest{
			DisplayName: displayName,
			Workspace:   workspace,
			CWD:         cwd,
			MCPServers:  []interface{}{},
		},
	}
}

// NewSessionPromptRequest 创建 prompt 请求
func NewSessionPromptRequest(sessionID string, prompt string) *ACPMessage {
	return &ACPMessage{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "session/prompt",
		Params: ACPSessionPromptRequest{
			SessionID: sessionID,
			Prompt: []ACPPromptItem{
				{
					Type: "text",
					Text: prompt,
				},
			},
		},
	}
}

// ParseACPMessage 从原始 JSON 消息解析 ACP 消息
func ParseACPMessage(raw []byte) (*ACPMessage, error) {
	var msg ACPMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, NewProtocolError("failed to unmarshal ACP message: %v", err)
	}

	if msg.JSONRPC != "2.0" {
		return nil, NewProtocolError("invalid JSON-RPC version: %s", msg.JSONRPC)
	}

	return &msg, nil
}
