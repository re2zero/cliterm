// Package protocol defines the ACP (Agent Control Protocol) types
//
// ACP is a JSON-RPC 2.0-based protocol for communicating with AI agents
// over stdio. This file contains all core type definitions.
package protocol

import "time"

// AcpBackend represents the supported ACP backend types
type AcpBackend string

const (
	AcpBackendClaude   AcpBackend = "claude"
	AcpBackendGemini   AcpBackend = "gemini"
	AcpBackendQwen     AcpBackend = "qwen"
	AcpBackendCodex    AcpBackend = "codex"
	AcpBackendOpenCode AcpBackend = "opencode"
	AcpBackendCustom   AcpBackend = "custom"
)

const (
	ClaudeACPBridgeVersion = "0.18.0"
	CodexACPBridgeVersion  = "0.9.5"
)

// Session mode constants (yolo/bypass permissions)
const (
	ClaudeYoloSessionMode    = "bypassPermissions"
	QwenYoloSessionMode      = "yolo"
	CodebuddyYoloSessionMode = "bypassPermissions"
	GooseYoloEnvVar          = "GOOSE_MODE"
	GooseYoloEnvValue        = "auto"
)

// AcpErrorType represents ACP error categories
type AcpErrorType string

const (
	ErrorConnection AcpErrorType = "CONNECTION"
	ErrorAuth       AcpErrorType = "AUTH"
	ErrorSession    AcpErrorType = "SESSION"
	ErrorNetwork    AcpErrorType = "NETWORK"
	ErrorTimeout    AcpErrorType = "TIMEOUT"
	ErrorPermission AcpErrorType = "PERMISSION"
	ErrorUnknown    AcpErrorType = "UNKNOWN"
)

// AcpError represents an ACP protocol error
type AcpError struct {
	Type    AcpErrorType           `json:"type"`
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *AcpError) Error() string {
	return e.Message
}

// IsRetryable indicates if the error can be retried
func (e *AcpError) IsRetryable() bool {
	return e.Type == ErrorNetwork || e.Type == ErrorTimeout
}

// AcpRequest represents a JSON-RPC 2.0 request
type AcpRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// AcpResponse represents a JSON-RPC 2.0 response
type AcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *AcpError   `json:"error,omitempty"`
}

// AcpNotification represents a JSON-RPC 2.0 notification (no ID)
type AcpNotification struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// AcpToolCall represents an ACP tool call
type AcpToolCall struct {
	CallID      string `json:"callId"`
	ToolName    string `json:"toolName"`
	Description string `json:"description"`
}

// AcpOption represents an ACP permission option
type AcpOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// AcpPermissionRequest represents a permission request from the agent
type AcpPermissionRequest struct {
	CallID      string      `json:"callId"`
	ToolName    string      `json:"toolName"`
	Description string      `json:"description"`
	Options     []AcpOption `json:"options"`
}

// AcpSessionUpdate represents a session update from the agent
type AcpSessionUpdate struct {
	SessionUpdate string                 `json:"sessionUpdate"`
	Content       interface{}            `json:"content,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ToolCall      *AcpToolCall           `json:"toolCall,omitempty"`
	Permission    *AcpPermissionRequest  `json:"permission,omitempty"`
	ToolCallID    string                 `json:"toolCallId,omitempty"`
	RawInput      map[string]interface{} `json:"rawInput,omitempty"`
	Title         string                 `json:"title,omitempty"`
	Status        string                 `json:"status,omitempty"`
	Kind          string                 `json:"kind,omitempty"`
	Meta          map[string]interface{} `json:"_meta,omitempty"`
}

func (u *AcpSessionUpdate) AsMetadata() map[string]interface{} {
	m := map[string]interface{}{
		"sessionUpdate": u.SessionUpdate,
		"status":        u.Status,
		"kind":          u.Kind,
		"title":         u.Title,
		"toolCallId":    u.ToolCallID,
	}
	if u.RawInput != nil {
		m["rawInput"] = u.RawInput
	}
	if u.Content != nil {
		m["content"] = u.Content
	}
	if u.Meta != nil {
		m["_meta"] = u.Meta
	}
	if u.Metadata != nil {
		for k, v := range u.Metadata {
			m[k] = v
		}
	}
	if u.Meta != nil {
		if cc, ok := u.Meta["claudeCode"].(map[string]interface{}); ok {
			if tn, ok := cc["toolName"].(string); ok {
				m["toolName"] = tn
			}
		}
	}
	return m
}

// AcpSessionConfigOption represents a configuration option for the session
type AcpSessionConfigOption struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Label   string      `json:"label"`
	Value   interface{} `json:"value"`
	Options []AcpOption `json:"options,omitempty"`
}

// AcpModelInfo represents model information
type AcpModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AcpSessionModels represents the models available in a session
type AcpSessionModels struct {
	DefaultModel string         `json:"defaultModel"`
	Models       []AcpModelInfo `json:"models"`
}

type TransportType string

const (
	TransportAcp          TransportType = "acp"
	TransportClaudeStream TransportType = "claude-stream"
)

type AcpBackendConfig struct {
	ID                AcpBackend        `json:"id"`
	Name              string            `json:"name"`
	CliCommand        string            `json:"cliCommand"`
	DefaultCliPath    string            `json:"defaultCliPath,omitempty"`
	AuthRequired      bool              `json:"authRequired"`
	Enabled           bool              `json:"enabled"`
	SupportsStreaming bool              `json:"supportsStreaming"`
	AcpArgs           []string          `json:"acpArgs"`
	Env               map[string]string `json:"env,omitempty"`
	Transport         TransportType     `json:"transport,omitempty"`
	NpxPackage        string            `json:"npxPackage,omitempty"`
	PermissionsArg    string            `json:"permissionsArg,omitempty"`
	OutputFormatArg   string            `json:"outputFormatArg,omitempty"`
	OutputFormat      string            `json:"outputFormat,omitempty"`
	PromptArg         string            `json:"promptArg,omitempty"`
	RunSubcommand     string            `json:"runSubcommand,omitempty"`
	UseStdinOnWin     bool              `json:"useStdinOnWin,omitempty"`
	ApprovalModeArg   string            `json:"approvalModeArg,omitempty"`
	ApprovalModeValue string            `json:"approvalModeValue,omitempty"`
	EnvOverrides      map[string]string `json:"envOverrides,omitempty"`
}

// AcpSessionConfig represents session configuration
type AcpSessionConfig struct {
	Backend       AcpBackend        `json:"backend"`
	CliPath       string            `json:"cliPath,omitempty"`
	Cwd           string            `json:"cwd"`
	ResumeSession bool              `json:"resumeSession,omitempty"`
	SessionID     string            `json:"sessionId,omitempty"`
	ForkSession   bool              `json:"forkSession,omitempty"`
	YoloMode      bool              `json:"yoloMode,omitempty"`
	Model         string            `json:"model,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Timeout       time.Duration     `json:"timeout,omitempty"`
}

// AcpPromptOptions represents options for prompting
type AcpPromptOptions struct {
	Files         []string `json:"files,omitempty"`
	ModelOverride string   `json:"modelOverride,omitempty"`
}

// AcpDisconnectInfo contains information about a disconnection
type AcpDisconnectInfo struct {
	Code   int
	Signal string
	Reason string
}
