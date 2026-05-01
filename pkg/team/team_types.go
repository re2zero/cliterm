// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import "time"

const HeartbeatInterval = 30 * time.Second
const HeartbeatTimeout = 2 * time.Minute

const (
	TaskStatusPending  = "pending"
	TaskStatusAssigned = "assigned"
	TaskStatusWorking  = "working"
	TaskStatusDone     = "done"
	TaskStatusFailed   = "failed"
	TaskStatusPaused   = "paused"

	WorkerStatusIdle    = "idle"
	WorkerStatusWorking = "working"
	WorkerStatusOffline = "offline"
	WorkerStatusError   = "error"

	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"

	MemoryNone       = "none"
	MemorySession    = "session"
	MemoryPersistent = "persistent"

	ToolClaude   = "claude"
	ToolOpenCode = "opencode"
	ToolCursor   = "cursor"
	ToolAider    = "aider"
	ToolCustom   = "custom"
)

var ValidTaskTransitions = map[string][]string{
	TaskStatusPending:  {TaskStatusAssigned, "cancelled"},
	TaskStatusAssigned: {TaskStatusWorking, TaskStatusPending, "cancelled"},
	TaskStatusWorking:  {TaskStatusDone, TaskStatusFailed, TaskStatusPaused},
	TaskStatusPaused:   {TaskStatusWorking, "cancelled"},
	TaskStatusFailed:   {TaskStatusWorking},
	"cancelled":        {},
	TaskStatusDone:     {},
}

var ValidWorkerTransitions = map[string][]string{
	WorkerStatusIdle:    {WorkerStatusWorking, WorkerStatusOffline},
	WorkerStatusWorking: {WorkerStatusIdle, WorkerStatusError, WorkerStatusOffline},
	WorkerStatusError:   {WorkerStatusIdle, WorkerStatusOffline},
	WorkerStatusOffline: {WorkerStatusIdle},
}

type TeamProject struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Spec      string `json:"spec"` // development spec: "sdd", "trellis", etc. (empty = none)
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type TeamMember struct {
	MemberID       string      `json:"memberId"`
	Name           string      `json:"name"`
	Tool           string      `json:"tool"`
	CustomCmd      string      `json:"customCmd"`
	Description    string      `json:"description"`
	Persona        string      `json:"persona"`
	PersonaPath    string      `json:"personaPath"`
	Skills         []string    `json:"skills"`
	McpServers     []MCPConfig `json:"mcpServers"`
	Capabilities   []string    `json:"capabilities"`
	Model          string      `json:"model"`
	MaxConcurrency int         `json:"maxConcurrency"`
	MaxRetries     int         `json:"maxRetries"`
	Memory         string      `json:"memory"`
	Color          string      `json:"color"`
	ProjectID      string      `json:"projectId"` // optional: which project this member belongs to
	CreatedAt      int64       `json:"createdAt"`
	UpdatedAt      int64       `json:"updatedAt"`
}

type MCPConfig struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type TeamWorker struct {
	WorkerID       string `json:"workerId"`
	MemberID       string `json:"memberId"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	AssignedTaskID string `json:"assignedTaskId"`
	BlockID        string `json:"blockId"`
	TabID          string `json:"tabId"`
	PID            int    `json:"pid"`
	ProjectID      string `json:"projectId"` // inherited from member at fork time
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	LastHeartbeat  int64  `json:"lastHeartbeat"`
}

type TeamTask struct {
	TaskID           string       `json:"taskId"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	Priority         string       `json:"priority"`
	Status           string       `json:"status"`
	AssignedMemberID string       `json:"assignedMemberId"`
	AssignedWorkerID string       `json:"assignedWorkerId"`
	DependsOn        []string     `json:"dependsOn"`
	Result           string       `json:"result"`
	Error            string       `json:"error"`
	OutputHistory    []TaskOutput `json:"outputHistory"`
	Progress         int          `json:"progress"`
	RetryCount       int          `json:"retryCount"`
	MaxRetries       int          `json:"maxRetries"`
	NextRetryAt      int64        `json:"nextRetryAt"`
	CreatedAt        int64        `json:"createdAt"`
	UpdatedAt        int64        `json:"updatedAt"`
	CompletedAt      int64        `json:"completedAt"`
}

type TaskOutput struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Content   string `json:"content"`
}

type TeamActivity struct {
	Id          int64  `json:"id"`
	TaskID      string `json:"taskId"`
	WorkerID    string `json:"workerId"`
	MemberID    string `json:"memberId"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Meta        string `json:"meta"`
	CreatedAt   int64  `json:"createdAt"`
}

type TeamStatusData struct {
	TotalMembers   int `json:"totalMembers"`
	ActiveWorkers  int `json:"activeWorkers"`
	IdleWorkers    int `json:"idleWorkers"`
	OfflineWorkers int `json:"offlineWorkers"`
	PendingTasks   int `json:"pendingTasks"`
	WorkingTasks   int `json:"workingTasks"`
	DoneTasks      int `json:"doneTasks"`
	FailedTasks    int `json:"failedTasks"`
	PausedTasks    int `json:"pausedTasks"`
}
