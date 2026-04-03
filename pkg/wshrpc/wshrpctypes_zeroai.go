// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// ZeroAI WSH RPC type definitions
package wshrpc

// ZeroAI backend constants
const (
	ZeroAiBackendClaude   = "claude"
	ZeroAiBackendQwen     = "qwen"
	ZeroAiBackendCodex    = "codex"
	ZeroAiBackendOpenCode = "opencode"
)

// ZeroAI role constants
const (
	ZeroAiRoleUser      = "user"
	ZeroAiRoleAssistant = "assistant"
	ZeroAiRoleSystem    = "system"
)

// ZeroAI event types
const (
	ZeroAiEventTypeMessage    = "message"
	ZeroAiEventTypePlan       = "plan"
	ZeroAiEventTypePermission = "permission"
)

// ZeroAiSessionWrapper wraps ZeroAiSession for RPC transport
// gotypes: gen
type ZeroAiSessionWrapper struct {
	ID            string                 `json:"id"`
	Backend       string                 `json:"backend"`
	WorkDir       string                 `json:"workDir"`
	Model         string                 `json:"model"`
	Provider      string                 `json:"provider"`
	ThinkingLevel string                 `json:"thinkingLevel"`
	YoloMode      bool                   `json:"yoloMode"`
	SessionID     string                 `json:"sessionId"`
	CreatedAt     int64                  `json:"createdAt"`
	UpdatedAt     int64                  `json:"updatedAt"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" tstype:"null | Record<string, any>"`
}

// ZeroAiMessageWrapper wraps ZeroAiMessage for RPC transport
// gotypes: gen
type ZeroAiMessageWrapper struct {
	ID        int64                  `json:"id"`
	SessionID string                 `json:"sessionId"`
	Role      string                 `json:"role" tstype:"\"user\" | \"assistant\" | \"system\""`
	Content   string                 `json:"content"`
	EventType string                 `json:"eventType,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" tstype:"null | Record<string, any>"`
	CreatedAt int64                  `json:"createdAt"`
}

// ZeroAiCreateSessionData is the request data for creating a new session
// gotypes: request
type ZeroAiCreateSessionData struct {
	Backend       string `json:"backend"`
	Model         string `json:"model"`
	Provider      string `json:"provider,omitempty"`
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
	YoloMode      bool   `json:"yoloMode,omitempty"`
	WorkDir       string `json:"workDir,omitempty"`
}

// ZeroAiCreateSessionResponse is the RPC response for creating a session
// gotypes: response
type ZeroAiCreateSessionResponse struct {
	Session *ZeroAiSessionWrapper `json:"session" tstype:"ZeroAiSessionWrapper"`
	Error   string                `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiGetSessionData is the request data for retrieving a session
// gotypes: request
type ZeroAiGetSessionData struct {
	SessionID string `json:"sessionId"`
}

// ZeroAiGetSessionResponse is the RPC response for getting a session
// gotypes: response
type ZeroAiGetSessionResponse struct {
	Session *ZeroAiSessionWrapper `json:"session" tstype:"null | ZeroAiSessionWrapper"`
	Error   string                `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiListSessionsData is the request data for listing sessions
// gotypes: request
type ZeroAiListSessionsData struct {
	Backend string `json:"backend,omitempty"`
}

// ZeroAiListSessionsResponse is the RPC response for listing sessions
// gotypes: response
type ZeroAiListSessionsResponse struct {
	Sessions []*ZeroAiSessionWrapper `json:"sessions" tstype:"ZeroAiSessionWrapper[]"`
	Error    string                  `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiDeleteSessionData is the request data for deleting a session
// gotypes: request
type ZeroAiDeleteSessionData struct {
	SessionID string `json:"sessionId"`
}

// ZeroAiDeleteSessionResponse is the RPC response for deleting a session
// gotypes: response
type ZeroAiDeleteSessionResponse struct {
	Success bool   `json:"success" tstype:"boolean"`
	Error   string `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiSetWorkDirData is the request data for setting work directory
// gotypes: request
type ZeroAiSetWorkDirData struct {
	SessionID string `json:"sessionId"`
	WorkDir   string `json:"workDir"`
}

// ZeroAiSetWorkDirResponse is the RPC response for setting work directory
// gotypes: response
type ZeroAiSetWorkDirResponse struct {
	Success bool   `json:"success" tstype:"boolean"`
	Error   string `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiSendMessageData is the request data for sending a message
// gotypes: request
type ZeroAiSendMessageData struct {
	SessionID string                 `json:"sessionId"`
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	EventType string                 `json:"eventType,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" tstype:"null | Record<string, any>"`
}

// ZeroAiSendMessageResponse is the RPC response for sending a message
// gotypes: response
type ZeroAiSendMessageResponse struct {
	Message *ZeroAiMessageWrapper `json:"message" tstype:"ZeroAiMessageWrapper"`
	Error   string                `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiGetMessagesData is the request data for retrieving session messages
// gotypes: request
type ZeroAiGetMessagesData struct {
	SessionID string `json:"sessionId"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

// ZeroAiGetMessagesResponse is the RPC response for retrieving session messages
// gotypes: response
type ZeroAiGetMessagesResponse struct {
	Messages []*ZeroAiMessageWrapper `json:"messages" tstype:"ZeroAiMessageWrapper[]"`
	Error    string                  `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiGetAgentsData is the request data for getting available agents
// gotypes: request
type ZeroAiGetAgentsData struct {
	Backend string `json:"backend,omitempty"`
}

// ZeroAiGetAgentsRtnData is the response data for getting available agents
// gotypes: gen
type ZeroAiGetAgentsRtnData struct {
	Agents []ZeroAiAgentInfo `json:"agents" tstype:"ZeroAiAgentInfo[]"`
}

// ZeroAiGetAgentsResponse is the RPC response for getting available agents
// gotypes: response
type ZeroAiGetAgentsResponse struct {
	Agents []ZeroAiAgentInfo `json:"agents" tstype:"ZeroAiAgentInfo[]"`
	Error  string            `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiAgentInfo represents information about an available AI agent
// gotypes: gen
type ZeroAiAgentInfo struct {
	Backend      string   `json:"backend" tstype:"\"claude\" | \"qwen\" | \"codex\" | \"opencode\""`
	Model        string   `json:"model"`
	Provider     string   `json:"provider"`
	DisplayName  string   `json:"displayName"`
	Description  string   `json:"description"`
	Enabled      bool     `json:"enabled" tstype:"boolean"`
	SupportedOps []string `json:"supportedOps" tstype:"string[]"`
}

// ZeroAiConfirmPermissionData is the request data for confirming a permission
// gotypes: request
type ZeroAiConfirmPermissionData struct {
	SessionID  string `json:"sessionId"`
	CallID     string `json:"callId"`
	OptionID   string `json:"optionId"`
	ConfirmAll bool   `json:"confirmAll" tstype:"boolean"`
}

// ZeroAiConfirmPermissionResponse is the RPC response for confirming a permission
// gotypes: response
type ZeroAiConfirmPermissionResponse struct {
	Success bool   `json:"success" tstype:"boolean"`
	Error   string `json:"error,omitempty" tstype:"null | string"`
}

// ZeroAiSessionChunk represents a streaming content chunk (SSE event data)
// gotypes: gen
type ZeroAiSessionChunk struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty" tstype:"null | Record<string, any>"`
}

// ZeroAiPlanUpdate represents a plan update event
// gotypes: gen
type ZeroAiPlanUpdate struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty" tstype:"null | Record<string, any>"`
}

// ZeroAiPermissionRequest represents a permission request event
// gotypes: gen
type ZeroAiPermissionRequest struct {
	CallID      string                   `json:"callId"`
	ToolName    string                   `json:"toolName"`
	Description string                   `json:"description"`
	Options     []ZeroAiPermissionOption `json:"options" tstype:"ZeroAiPermissionOption[]"`
	SessionID   string                   `json:"sessionId"`
}

// ZeroAiPermissionOption represents a permission option
// gotypes: gen
type ZeroAiPermissionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// ZeroAiStreamStart represents the start of a stream
// gotypes: gen
type ZeroAiStreamStart struct {
	StreamID  string `json:"streamId"`
	EventType string `json:"eventType" tstype:"\"message\" | \"plan\" | \"permission\""`
}

// ZeroAiStreamEnd represents the end of a stream
// gotypes: gen
type ZeroAiStreamEnd struct {
	StreamID     string `json:"streamId"`
	FinishReason string `json:"finishReason,omitempty" tstype:"null | \"stop\" | \"length\" | \"error\""`
	Error        string `json:"error,omitempty" tstype:"null | string"`
}

// ===== Team Collaboration Types =====

// ZeroAiTeamInfo represents information about a team
// gotypes: gen
type ZeroAiTeamInfo struct {
	TeamID  string `json:"teamId"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Created int64  `json:"created"`
}

// ZeroAiTeamMemberInfo represents information about a team member
// gotypes: gen
type ZeroAiTeamMemberInfo struct {
	AgentID  string `json:"agentId"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	JoinedAt int64  `json:"joinedAt"`
}

// ZeroAiTaskInfo represents information about a task
// gotypes: gen
type ZeroAiTaskInfo struct {
	TaskID          string `json:"taskId"`
	TeamID          string `json:"teamId"`
	AssignedAgentID string `json:"assignedAgentId,omitempty"`
	Status          string `json:"status"`
	Description     string `json:"description"`
	CreatedAt       int64  `json:"createdAt"`
	CompletedAt     int64  `json:"completedAt,omitempty"`
}

// ===== Team Management Commands =====

// CommandZeroAiCreateTeamData is the request data for creating a new team
// gotypes: request
type CommandZeroAiCreateTeamData struct {
	Name     string `json:"name"`
	LeaderID string `json:"leaderId"`
}

// CommandZeroAiCreateTeamRtnData is the response data for creating a team
// gotypes: gen
type CommandZeroAiCreateTeamRtnData struct {
	TeamID string `json:"teamId"`
}

// CommandZeroAiGetTeamData is the request data for retrieving a team
// gotypes: request
type CommandZeroAiGetTeamData struct {
	TeamID string `json:"teamId"`
}

// CommandZeroAiGetTeamRtnData is the response data for getting a team
// gotypes: gen
type CommandZeroAiGetTeamRtnData struct {
	Team *ZeroAiTeamInfo `json:"team" tstype:"null | ZeroAiTeamInfo"`
}

// CommandZeroAiListTeamsData is the request data for listing teams
// gotypes: request
type CommandZeroAiListTeamsData struct {
	Status string `json:"status,omitempty"`
}

// CommandZeroAiListTeamsRtnData is the response data for listing teams
// gotypes: gen
type CommandZeroAiListTeamsRtnData struct {
	Teams []*ZeroAiTeamInfo `json:"teams" tstype:"ZeroAiTeamInfo[]"`
}

// CommandZeroAiDeleteTeamData is the request data for deleting a team
// gotypes: request
type CommandZeroAiDeleteTeamData struct {
	TeamID string `json:"teamId"`
}

// ===== Team Member Management Commands =====

// CommandZeroAiJoinTeamData is the request data for adding an agent to a team
// gotypes: request
type CommandZeroAiJoinTeamData struct {
	TeamID  string `json:"teamId"`
	AgentID string `json:"agentId"`
	Role    string `json:"role,omitempty"`
}

// CommandZeroAiJoinTeamRtnData is the response data for joining a team
// gotypes: gen
type CommandZeroAiJoinTeamRtnData struct {
	Success bool `json:"success" tstype:"boolean"`
}

// CommandZeroAiLeaveTeamData is the request data for removing an agent from a team
// gotypes: request
type CommandZeroAiLeaveTeamData struct {
	TeamID  string `json:"teamId"`
	AgentID string `json:"agentId"`
}

// CommandZeroAiListTeamMembersData is the request data for listing team members
// gotypes: request
type CommandZeroAiListTeamMembersData struct {
	TeamID string `json:"teamId"`
}

// CommandZeroAiListTeamMembersRtnData is the response data for listing team members
// gotypes: gen
type CommandZeroAiListTeamMembersRtnData struct {
	Members []*ZeroAiTeamMemberInfo `json:"members" tstype:"ZeroAiTeamMemberInfo[]"`
}

// ===== Task Management Commands =====

// CommandZeroAiCreateTaskData is the request data for creating a new task
// gotypes: request
type CommandZeroAiCreateTaskData struct {
	TeamID          string `json:"teamId"`
	Description     string `json:"description"`
	AssignedAgentID string `json:"assignedAgentId,omitempty"`
}

// CommandZeroAiCreateTaskRtnData is the response data for creating a task
// gotypes: gen
type CommandZeroAiCreateTaskRtnData struct {
	TaskID string `json:"taskId"`
}

// CommandZeroAiAssignTaskData is the request data for assigning a task
// gotypes: request
type CommandZeroAiAssignTaskData struct {
	TaskID  string `json:"taskId"`
	TeamID  string `json:"teamId"`
	AgentID string `json:"agentId"`
}

// CommandZeroAiAssignTaskRtnData is the response data for assigning a task
// gotypes: gen
type CommandZeroAiAssignTaskRtnData struct {
	Success bool `json:"success" tstype:"boolean"`
}

// CommandZeroAiListTasksData is the request data for listing tasks
// gotypes: request
type CommandZeroAiListTasksData struct {
	TeamID          string `json:"teamId"`
	AssignedAgentID string `json:"assignedAgentId,omitempty"`
	Status          string `json:"status,omitempty"`
}

// CommandZeroAiListTasksRtnData is the response data for listing tasks
// gotypes: gen
type CommandZeroAiListTasksRtnData struct {
	Tasks []*ZeroAiTaskInfo `json:"tasks" tstype:"ZeroAiTaskInfo[]"`
}

// CommandZeroAiGetTaskStatusData is the request data for getting task status
// gotypes: request
type CommandZeroAiGetTaskStatusData struct {
	TaskID string `json:"taskId"`
}

// CommandZeroAiGetTaskStatusRtnData is the response data for getting task status
// gotypes: gen
type CommandZeroAiGetTaskStatusRtnData struct {
	Task *ZeroAiTaskInfo `json:"task" tstype:"null | ZeroAiTaskInfo"`
}

// ===== Message Routing Commands =====

// CommandZeroAiSendToAgentData is the request data for sending a message to an agent
// gotypes: request
type CommandZeroAiSendToAgentData struct {
	TeamID  string                 `json:"teamId"`
	From    string                 `json:"from"`
	To      string                 `json:"to"`
	Type    string                 `json:"type,omitempty"`
	Content string                 `json:"content"`
	Payload map[string]interface{} `json:"payload,omitempty" tstype:"null | Record<string, any>"`
}

// CommandZeroAiSendToAgentRtnData is the response data for sending a message
// gotypes: gen
type CommandZeroAiSendToAgentRtnData struct {
	Success bool `json:"success" tstype:"boolean"`
}

// CommandZeroAiBroadcastData is the request data for broadcasting a message
// gotypes: request
type CommandZeroAiBroadcastData struct {
	TeamID  string                 `json:"teamId"`
	From    string                 `json:"from"`
	Type    string                 `json:"type,omitempty"`
	Content string                 `json:"content"`
	Payload map[string]interface{} `json:"payload,omitempty" tstype:"null | Record<string, any>"`
}

// CommandZeroAiBroadcastRtnData is the response data for broadcasting a message
// gotypes: gen
type CommandZeroAiBroadcastRtnData struct {
	RecipientCount int `json:"recipientCount"`
}

// ===== Provider Management Commands =====

// ZeroAiProviderInfo represents information about a ZeroAI provider
// gotypes: gen
type ZeroAiProviderInfo struct {
	ID                string            `json:"id"`
	DisplayName       string            `json:"displayName"`
	DisplayIcon       string            `json:"displayIcon,omitempty"`
	CliCommand        string            `json:"cliCommand"`
	CliPath           string            `json:"cliPath,omitempty"`
	CliArgs           []string          `json:"cliArgs,omitempty"`
	EnvVars           map[string]string `json:"envVars,omitempty"`
	SupportsStreaming bool              `json:"supportsStreaming" tstype:"boolean"`
	DefaultModel      string            `json:"defaultModel,omitempty"`
	AvailableModels   []string          `json:"availableModels,omitempty"`
	AuthRequired      bool              `json:"authRequired" tstype:"boolean"`
	IsCustom          bool              `json:"isCustom" tstype:"boolean"`
	IsAvailable       bool              `json:"isAvailable" tstype:"boolean"`
}

// ZeroAiTestProviderResult represents the result of testing a provider
// gotypes: gen
type ZeroAiTestProviderResult struct {
	Success   bool   `json:"success" tstype:"boolean"`
	Error     string `json:"error,omitempty"`
	Version   string `json:"version,omitempty"`
	ModelInfo string `json:"modelInfo,omitempty"`
	LatencyMs int64  `json:"latencyMs"`
}

// CommandZeroAiListProvidersData is the request data for listing providers
// gotypes: request
type CommandZeroAiListProvidersData struct{}

// CommandZeroAiListProvidersRtnData is the response data for listing providers
// gotypes: gen
type CommandZeroAiListProvidersRtnData struct {
	Providers []*ZeroAiProviderInfo `json:"providers" tstype:"ZeroAiProviderInfo[]"`
}

// CommandZeroAiSaveProviderData is the request data for saving a provider
// gotypes: request
type CommandZeroAiSaveProviderData struct {
	ProviderID        string            `json:"providerId"`
	DisplayName       string            `json:"displayName,omitempty"`
	DisplayIcon       string            `json:"displayIcon,omitempty"`
	CliCommand        string            `json:"cliCommand"`
	CliPath           string            `json:"cliPath,omitempty"`
	CliArgs           []string          `json:"cliArgs,omitempty"`
	EnvVars           map[string]string `json:"envVars,omitempty"`
	SupportsStreaming bool              `json:"supportsStreaming" tstype:"boolean"`
	DefaultModel      string            `json:"defaultModel,omitempty"`
	AvailableModels   []string          `json:"availableModels,omitempty"`
	AuthRequired      bool              `json:"authRequired" tstype:"boolean"`
}

// CommandZeroAiDeleteProviderData is the request data for deleting a provider
// gotypes: request
type CommandZeroAiDeleteProviderData struct {
	ProviderID string `json:"providerId"`
}

// CommandZeroAiTestProviderData is the request data for testing a provider
// gotypes: request
type CommandZeroAiTestProviderData struct {
	ProviderID string `json:"providerId"`
}

// CommandZeroAiTestProviderRtnData is the response data for testing a provider
// gotypes: gen
type CommandZeroAiTestProviderRtnData struct {
	Result *ZeroAiTestProviderResult `json:"result" tstype:"ZeroAiTestProviderResult"`
}
