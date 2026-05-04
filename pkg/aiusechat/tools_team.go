package aiusechat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wavetermdev/waveterm/pkg/aiusechat/uctypes"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/team"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
)

// --- team_fork_worker ---

type teamForkWorkerParams struct {
	MemberId string `json:"memberid"`
}

func teamForkWorkerCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamForkWorkerParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.MemberId == "" {
		return nil, fmt.Errorf("memberid is required")
	}

	rpcClient := wshclient.GetBareRpcClient()
	worker, err := wshclient.TeamForkWorkerCommand(rpcClient, parsed.MemberId, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to fork worker: %w", err)
	}

	return map[string]any{
		"success":  true,
		"workerid": worker.WorkerID,
		"memberid": worker.MemberID,
		"name":     worker.Name,
		"status":   worker.Status,
	}, nil
}

func GetTeamForkWorkerToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_fork_worker",
		DisplayName: "Fork Team Worker",
		Description: "Create a worker instance from a member template (fork = clone). Checks maxConcurrency. Use this to spin up a new worker for a specific member.",
		ToolLogName: "team:forkworker",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"memberid": map[string]any{
					"type":        "string",
					"description": "ID of the member template to fork a worker from",
				},
			},
			"required":             []string{"memberid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamForkWorkerParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("forking worker from member %s", parsed.MemberId)
		},
		ToolAnyCallback: teamForkWorkerCallback,
	}
}

// --- team_list_workers ---

type teamListWorkersParams struct {
	MemberId string `json:"memberid,omitempty"`
}

func teamListWorkersCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamListWorkersParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	rpcClient := wshclient.GetBareRpcClient()
	workers, err := wshclient.TeamListWorkersCommand(rpcClient, parsed.MemberId, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	result := make([]map[string]any, len(workers))
	for i, w := range workers {
		entry := map[string]any{
			"workerid": w.WorkerID,
			"memberid": w.MemberID,
			"name":     w.Name,
			"status":   w.Status,
		}
		if w.AssignedTaskID != "" {
			entry["assignedtaskid"] = w.AssignedTaskID
		}
		result[i] = entry
	}

	return map[string]any{
		"success": true,
		"workers": result,
		"count":   len(workers),
	}, nil
}

func GetTeamListWorkersToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_list_workers",
		DisplayName: "List Team Workers",
		Description: "List all active worker instances with their status, bound member, and current task. Optionally filter by memberid.",
		ToolLogName: "team:listworkers",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"memberid": map[string]any{
					"type":        "string",
					"description": "Optional member ID to filter workers by",
				},
			},
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			return "listing all workers"
		},
		ToolAnyCallback: teamListWorkersCallback,
	}
}

// --- team_list_members ---

func teamListMembersCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	rpcClient := wshclient.GetBareRpcClient()
	members, err := wshclient.TeamListMembersCommand(rpcClient, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}

	result := make([]map[string]any, len(members))
	for i, m := range members {
		entry := map[string]any{
			"memberid":       m.MemberID,
			"name":           m.Name,
			"tool":           m.Tool,
			"description":    m.Description,
			"maxconcurrency": m.MaxConcurrency,
			"maxretries":     m.MaxRetries,
		}
		if len(m.Skills) > 0 {
			entry["skills"] = m.Skills
		}
		if len(m.Capabilities) > 0 {
			entry["capabilities"] = m.Capabilities
		}
		if m.Model != "" {
			entry["model"] = m.Model
		}
		if m.Color != "" {
			entry["color"] = m.Color
		}
		result[i] = entry
	}

	return map[string]any{
		"success": true,
		"members": result,
		"count":   len(members),
	}, nil
}

func GetTeamListMembersToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_list_members",
		DisplayName: "List Team Members",
		Description: "List all team member templates with their capabilities, skills, tool type, and availability (maxConcurrency).",
		ToolLogName: "team:listmembers",
		Strict:      false,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			return "listing all team members"
		},
		ToolAnyCallback: teamListMembersCallback,
	}
}

// --- team_create_member ---

type teamCreateMemberParams struct {
	Name           string          `json:"name"`
	Tool           string          `json:"tool,omitempty"`
	CustomCmd      string          `json:"customcmd,omitempty"`
	Description    string          `json:"description,omitempty"`
	Persona        string          `json:"persona,omitempty"`
	PersonaPath    string          `json:"personapath,omitempty"`
	Skills         []string        `json:"skills,omitempty"`
	McpServers     []teamMCPParam  `json:"mcpservers,omitempty"`
	Capabilities   []string        `json:"capabilities,omitempty"`
	Model          string          `json:"model,omitempty"`
	MaxConcurrency int             `json:"maxconcurrency,omitempty"`
	MaxRetries     int             `json:"maxretries,omitempty"`
	Memory         string          `json:"memory,omitempty"`
	Color          string          `json:"color,omitempty"`
}

type teamMCPParam struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func convertMCPParams(params []teamMCPParam) []wshrpc.TeamMCPConfig {
	if params == nil {
		return nil
	}
	result := make([]wshrpc.TeamMCPConfig, len(params))
	for i, p := range params {
		result[i] = wshrpc.TeamMCPConfig{
			Name:    p.Name,
			Type:    p.Type,
			Command: p.Command,
			Args:    p.Args,
			Env:     p.Env,
			URL:     p.URL,
			Headers: p.Headers,
		}
	}
	return result
}

func teamCreateMemberCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamCreateMemberParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	if parsed.Tool == "" {
		parsed.Tool = "claude"
	}

	data := wshrpc.TeamCreateMemberData{
		Name:           parsed.Name,
		Tool:           parsed.Tool,
		CustomCmd:      parsed.CustomCmd,
		Description:    parsed.Description,
		Persona:        parsed.Persona,
		PersonaPath:    parsed.PersonaPath,
		Skills:         parsed.Skills,
		McpServers:     convertMCPParams(parsed.McpServers),
		Capabilities:   parsed.Capabilities,
		Model:          parsed.Model,
		MaxConcurrency: parsed.MaxConcurrency,
		MaxRetries:     parsed.MaxRetries,
		Memory:         parsed.Memory,
		Color:          parsed.Color,
	}

	rpcClient := wshclient.GetBareRpcClient()
	member, err := wshclient.TeamCreateMemberCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create member: %w", err)
	}

	return map[string]any{
		"success":  true,
		"memberid": member.MemberID,
		"name":     member.Name,
		"tool":     member.Tool,
		"created":  member.CreatedAt,
	}, nil
}

func GetTeamCreateMemberToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_create_member",
		DisplayName: "Create Team Member",
		Description: "Define a new team member template with persona, skills, tools, MCP servers, and capabilities. Members are templates — use team_fork_worker to create runnable instances.",
		ToolLogName: "team:createmember",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Member name (e.g., 'Go Backend Developer')",
				},
				"tool": map[string]any{
					"type":        "string",
					"enum":        []string{"claude", "opencode", "cursor", "aider", "custom"},
					"default":     "claude",
					"description": "CLI tool type for this member's workers",
				},
				"customcmd": map[string]any{
					"type":        "string",
					"description": "Custom CLI command (required when tool=custom)",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Brief description of member capabilities (for scheduling decisions)",
				},
				"persona": map[string]any{
					"type":        "string",
					"description": "Inline system prompt / personality text (short text)",
				},
				"personapath": map[string]any{
					"type":        "string",
					"description": "Path to external persona .md file (takes priority over persona)",
				},
				"skills": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Skill names to equip the member with",
				},
				"capabilities": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Tool permission whitelist (e.g., Read, Write, Edit, Bash)",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Optional model override in provider/model format (e.g., anthropic/claude-sonnet)",
				},
				"maxconcurrency": map[string]any{
					"type":        "integer",
					"default":     3,
					"description": "Maximum concurrent worker instances for this member",
				},
				"maxretries": map[string]any{
					"type":        "integer",
					"default":     3,
					"description": "Maximum retry attempts per task",
				},
				"memory": map[string]any{
					"type":        "string",
					"enum":        []string{"none", "session", "persistent"},
					"default":     "session",
					"description": "Memory mode for workers forked from this member",
				},
				"color": map[string]any{
					"type":        "string",
					"description": "UI identifier color (e.g., '#3B82F6')",
				},
			},
			"required":             []string{"name"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamCreateMemberParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("creating member: %s (tool: %s)", parsed.Name, parsed.Tool)
		},
		ToolAnyCallback: teamCreateMemberCallback,
	}
}

// --- team_update_member ---

type teamUpdateMemberParams struct {
	MemberID       string         `json:"memberid"`
	Name           string         `json:"name,omitempty"`
	Tool           string         `json:"tool,omitempty"`
	CustomCmd      string         `json:"customcmd,omitempty"`
	Description    string         `json:"description,omitempty"`
	Persona        string         `json:"persona,omitempty"`
	PersonaPath    string         `json:"personapath,omitempty"`
	Skills         []string       `json:"skills,omitempty"`
	Capabilities   []string       `json:"capabilities,omitempty"`
	Model          string         `json:"model,omitempty"`
	MaxConcurrency int            `json:"maxconcurrency,omitempty"`
	MaxRetries     int            `json:"maxretries,omitempty"`
	Memory         string         `json:"memory,omitempty"`
	Color          string         `json:"color,omitempty"`
}

func teamUpdateMemberCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamUpdateMemberParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.MemberID == "" {
		return nil, fmt.Errorf("memberid is required")
	}

	data := wshrpc.TeamUpdateMemberData{
		MemberID:       parsed.MemberID,
		Name:           parsed.Name,
		Tool:           parsed.Tool,
		CustomCmd:      parsed.CustomCmd,
		Description:    parsed.Description,
		Persona:        parsed.Persona,
		PersonaPath:    parsed.PersonaPath,
		Skills:         parsed.Skills,
		Capabilities:   parsed.Capabilities,
		Model:          parsed.Model,
		MaxConcurrency: parsed.MaxConcurrency,
		MaxRetries:     parsed.MaxRetries,
		Memory:         parsed.Memory,
		Color:          parsed.Color,
	}

	rpcClient := wshclient.GetBareRpcClient()
	member, err := wshclient.TeamUpdateMemberCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to update member: %w", err)
	}

	return map[string]any{
		"success":  true,
		"memberid": member.MemberID,
		"name":     member.Name,
	}, nil
}

func GetTeamUpdateMemberToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_update_member",
		DisplayName: "Update Team Member",
		Description: "Update an existing member template's properties. Only provided fields will be updated.",
		ToolLogName: "team:updatemember",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"memberid": map[string]any{
					"type":        "string",
					"description": "ID of the member to update",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Updated member name",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Updated description",
				},
				"persona": map[string]any{
					"type":        "string",
					"description": "Updated inline persona/system prompt",
				},
				"personapath": map[string]any{
					"type":        "string",
					"description": "Updated external persona file path",
				},
				"skills": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Updated skill list",
				},
				"capabilities": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Updated capabilities whitelist",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Updated model override",
				},
				"maxconcurrency": map[string]any{
					"type":        "integer",
					"description": "Updated max concurrency",
				},
				"maxretries": map[string]any{
					"type":        "integer",
					"description": "Updated max retries",
				},
				"memory": map[string]any{
					"type":        "string",
					"enum":        []string{"none", "session", "persistent"},
					"description": "Updated memory mode",
				},
				"color": map[string]any{
					"type":        "string",
					"description": "Updated UI color",
				},
			},
			"required":             []string{"memberid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamUpdateMemberParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("updating member %s", parsed.MemberID)
		},
		ToolAnyCallback: teamUpdateMemberCallback,
	}
}

// --- team_delete_member ---

type teamDeleteMemberParams struct {
	MemberId string `json:"memberid"`
}

func teamDeleteMemberCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamDeleteMemberParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.MemberId == "" {
		return nil, fmt.Errorf("memberid is required")
	}

	rpcClient := wshclient.GetBareRpcClient()
	err := wshclient.TeamDeleteMemberCommand(rpcClient, parsed.MemberId, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to delete member: %w", err)
	}

	return map[string]any{
		"success":  true,
		"memberid": parsed.MemberId,
		"message":  "Member deleted (cascading workers also removed)",
	}, nil
}

func GetTeamDeleteMemberToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_delete_member",
		DisplayName: "Delete Team Member",
		Description: "Delete a member template and all its worker instances (cascade delete).",
		ToolLogName: "team:deletemember",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"memberid": map[string]any{
					"type":        "string",
					"description": "ID of the member to delete",
				},
			},
			"required":             []string{"memberid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamDeleteMemberParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("deleting member %s", parsed.MemberId)
		},
		ToolAnyCallback: teamDeleteMemberCallback,
	}
}

// --- team_create_task ---

type teamCreateTaskParams struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	DependsOn   []string `json:"dependson,omitempty"`
}

func teamCreateTaskCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamCreateTaskParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	if parsed.Priority == "" {
		parsed.Priority = "medium"
	}

	data := wshrpc.TeamCreateTaskData{
		Title:       parsed.Title,
		Description: parsed.Description,
		Priority:    parsed.Priority,
		DependsOn:   parsed.DependsOn,
	}

	rpcClient := wshclient.GetBareRpcClient()
	task, err := wshclient.TeamCreateTaskCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return map[string]any{
		"success":  true,
		"taskid":   task.TaskID,
		"title":    task.Title,
		"status":   task.Status,
		"priority": task.Priority,
		"created":  task.CreatedAt,
	}, nil
}

func GetTeamCreateTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_create_task",
		DisplayName: "Create Team Task",
		Description: "Create a new task with title, description, priority, and optional dependencies (dependsOn). Tasks start in 'pending' status.",
		ToolLogName: "team:createtask",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "Title of the task",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Detailed description of the task",
				},
				"priority": map[string]any{
					"type":        "string",
					"enum":        []string{"low", "medium", "high", "urgent"},
					"default":     "medium",
					"description": "Priority of the task",
				},
				"dependson": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "List of task IDs this task depends on",
				},
			},
			"required":             []string{"title"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamCreateTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("creating task: %s", parsed.Title)
		},
		ToolAnyCallback: teamCreateTaskCallback,
	}
}

// --- team_assign_task ---

type teamAssignTaskParams struct {
	TaskId   string `json:"taskid"`
	MemberId string `json:"memberid"`
}

func teamAssignTaskCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamAssignTaskParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.TaskId == "" {
		return nil, fmt.Errorf("taskid is required")
	}
	if parsed.MemberId == "" {
		return nil, fmt.Errorf("memberid is required")
	}

	data := wshrpc.TeamUpdateTaskData{
		TaskID:           parsed.TaskId,
		AssignedMemberID: parsed.MemberId,
		Status:           team.TaskStatusAssigned,
	}

	rpcClient := wshclient.GetBareRpcClient()
	task, err := wshclient.TeamUpdateTaskCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to assign task: %w", err)
	}

	return map[string]any{
		"success":  true,
		"taskid":   parsed.TaskId,
		"memberid": parsed.MemberId,
		"status":   task.Status,
		"message":  "Task assigned to member",
	}, nil
}

func GetTeamAssignTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_assign_task",
		DisplayName: "Assign Task to Member",
		Description: "Assign a task to a member (auto-forks a worker if needed, respects maxConcurrency). Use after creating a task and choosing the right member.",
		ToolLogName: "team:assigntask",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"taskid": map[string]any{
					"type":        "string",
					"description": "ID of the task to assign",
				},
				"memberid": map[string]any{
					"type":        "string",
					"description": "ID of the member to assign the task to",
				},
			},
			"required":             []string{"taskid", "memberid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamAssignTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("assigning task %s to member %s", parsed.TaskId, parsed.MemberId)
		},
		ToolAnyCallback: teamAssignTaskCallback,
	}
}

// --- team_update_task ---

type teamUpdateTaskParams struct {
	TaskId     string `json:"taskid"`
	Status     string `json:"status,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	Progress   int    `json:"progress,omitempty"`
	Title      string `json:"title,omitempty"`
}

func teamUpdateTaskCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamUpdateTaskParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.TaskId == "" {
		return nil, fmt.Errorf("taskid is required")
	}

	data := wshrpc.TeamUpdateTaskData{
		TaskID:   parsed.TaskId,
		Title:    parsed.Title,
		Status:   parsed.Status,
		Result:   parsed.Result,
		Error:    parsed.Error,
		Progress: parsed.Progress,
	}

	rpcClient := wshclient.GetBareRpcClient()
	task, err := wshclient.TeamUpdateTaskCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return map[string]any{
		"success": true,
		"taskid":  parsed.TaskId,
		"status":  task.Status,
		"title":   task.Title,
	}, nil
}

func GetTeamUpdateTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_update_task",
		DisplayName: "Update Team Task",
		Description: "Update a task's status, progress, result, error, or title. Use to mark tasks done/failed or update progress.",
		ToolLogName: "team:updatetask",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"taskid": map[string]any{
					"type":        "string",
					"description": "ID of the task to update",
				},
				"status": map[string]any{
					"type":        "string",
					"enum":        []string{"pending", "assigned", "working", "done", "failed", "paused"},
					"description": "New status for the task",
				},
				"result": map[string]any{
					"type":        "string",
					"description": "Task result summary (set when marking done)",
				},
				"error": map[string]any{
					"type":        "string",
					"description": "Error message (set when marking failed)",
				},
				"progress": map[string]any{
					"type":        "integer",
					"description": "Progress percentage (0-100)",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Updated task title",
				},
			},
			"required":             []string{"taskid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamUpdateTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("updating task %s", parsed.TaskId)
		},
		ToolAnyCallback: teamUpdateTaskCallback,
	}
}

// --- team_get_status ---

func teamGetStatusCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	ctx := context.Background()
	rpcClient := wshclient.GetBareRpcClient()

	status, err := wshclient.TeamGetStatusCommand(rpcClient, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	members, _ := team.ListMembers(ctx)
	memberList := make([]map[string]any, 0, len(members))
	for _, m := range members {
		entry := map[string]any{
			"memberid":       m.MemberID,
			"name":           m.Name,
			"tool":           m.Tool,
			"description":    m.Description,
			"maxconcurrency": m.MaxConcurrency,
		}
		if len(m.Skills) > 0 {
			entry["skills"] = m.Skills
		}
		memberList = append(memberList, entry)
	}

	workers, _ := team.ListWorkers(ctx, "")
	workerList := make([]map[string]any, 0, len(workers))
	for _, w := range workers {
		entry := map[string]any{
			"workerid": w.WorkerID,
			"memberid": w.MemberID,
			"name":     w.Name,
			"status":   w.Status,
		}
		if w.AssignedTaskID != "" {
			entry["assignedtaskid"] = w.AssignedTaskID
		}
		workerList = append(workerList, entry)
	}

	tasks, _ := team.ListTasks(ctx, "", "", "")
	taskList := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		entry := map[string]any{
			"taskid":   t.TaskID,
			"title":    t.Title,
			"status":   t.Status,
			"priority": t.Priority,
		}
		if t.Description != "" {
			entry["description"] = t.Description
		}
		if t.AssignedMemberID != "" {
			entry["assignedmemberid"] = t.AssignedMemberID
		}
		if t.AssignedWorkerID != "" {
			entry["assignedworkerid"] = t.AssignedWorkerID
		}
		if t.Error != "" {
			entry["error"] = t.Error
		}
		taskList = append(taskList, entry)
	}

	return map[string]any{
		"success":       true,
		"totalmembers":  status.TotalMembers,
		"activeworkers": status.ActiveWorkers,
		"idleworkers":   status.IdleWorkers,
		"pendingtasks":  status.PendingTasks,
		"workingtasks":  status.WorkingTasks,
		"donetasks":     status.DoneTasks,
		"failedtasks":   status.FailedTasks,
		"pausedtasks":   status.PausedTasks,
		"members":       memberList,
		"workers":       workerList,
		"tasks":         taskList,
	}, nil
}

func GetTeamGetStatusToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_get_status",
		DisplayName: "Get Team Status",
		Description: "Get full team overview: aggregated stats, all members with capabilities, all workers with status, and all tasks. Call this first to understand current state before making decisions.",
		ToolLogName: "team:getstatus",
		Strict:      false,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			return "getting team status"
		},
		ToolAnyCallback: teamGetStatusCallback,
	}
}

// --- team_execute_task ---

type teamExecuteTaskParams struct {
	WorkerId string `json:"workerid"`
	TaskId   string `json:"taskid"`
	Command  string `json:"command,omitempty"`
}

func teamExecuteTaskCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamExecuteTaskParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.WorkerId == "" {
		return nil, fmt.Errorf("workerid is required")
	}
	if parsed.TaskId == "" {
		return nil, fmt.Errorf("taskid is required")
	}

	data := wshrpc.TeamExecuteTaskData{
		WorkerID: parsed.WorkerId,
		TaskID:   parsed.TaskId,
		Command:  parsed.Command,
	}

	rpcClient := wshclient.GetBareRpcClient()
	resp, err := wshclient.TeamExecuteTaskCommand(rpcClient, data, &wshrpc.RpcOpts{Timeout: 30000})
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}

	return map[string]any{
		"success": resp.Success,
		"blockid": resp.BlockID,
		"tabid":   resp.TabID,
	}, nil
}

func GetTeamExecuteTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_execute_task",
		DisplayName: "Execute Team Task",
		Description: "Start task execution: creates a terminal block and sends the command to the worker. For sending additional instructions to a running worker, use team_send_prompt instead.",
		ToolLogName: "team:executetask",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"workerid": map[string]any{
					"type":        "string",
					"description": "ID of the worker to execute the task",
				},
				"taskid": map[string]any{
					"type":        "string",
					"description": "ID of the task to execute",
				},
				"command": map[string]any{
					"type":        "string",
					"description": "Optional command override (default: worker's configured CLI)",
				},
			},
			"required":             []string{"workerid", "taskid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamExecuteTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("executing task %s on worker %s", parsed.TaskId, parsed.WorkerId)
		},
		ToolAnyCallback: teamExecuteTaskCallback,
	}
}

// --- team_recycle_worker ---

type teamRecycleWorkerParams struct {
	WorkerId string `json:"workerid"`
}

func teamRecycleWorkerCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamRecycleWorkerParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.WorkerId == "" {
		return nil, fmt.Errorf("workerid is required")
	}

	rpcClient := wshclient.GetBareRpcClient()
	err := wshclient.TeamRecycleWorkerCommand(rpcClient, parsed.WorkerId, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to recycle worker: %w", err)
	}

	return map[string]any{
		"success":  true,
		"workerid": parsed.WorkerId,
		"message":  "Worker recycled, terminal block released",
	}, nil
}

func GetTeamRecycleWorkerToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_recycle_worker",
		DisplayName: "Recycle Team Worker",
		Description: "Recycle a worker: releases its terminal block and cleans up resources. Use when a worker has finished its work or needs to be stopped.",
		ToolLogName: "team:recycleworker",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"workerid": map[string]any{
					"type":        "string",
					"description": "ID of the worker to recycle",
				},
			},
			"required":             []string{"workerid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamRecycleWorkerParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("recycling worker %s", parsed.WorkerId)
		},
		ToolAnyCallback: teamRecycleWorkerCallback,
	}
}

// --- team_send_prompt ---

type teamSendPromptParams struct {
	WorkerId string `json:"workerid"`
	Prompt   string `json:"prompt"`
}

func teamSendPromptCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	inputBytes, _ := json.Marshal(input)
	var parsed teamSendPromptParams
	json.Unmarshal(inputBytes, &parsed)

	if parsed.WorkerId == "" {
		return nil, fmt.Errorf("workerid is required")
	}
	if parsed.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	ctx := context.Background()
	worker, err := team.GetWorker(ctx, parsed.WorkerId)
	if err != nil {
		return nil, fmt.Errorf("worker not found: %w", err)
	}

	// Worker has an active terminal block — send prompt directly
	if worker.BlockID != "" {
		ctrl := blockcontroller.HasController(worker.BlockID)
		if !ctrl {
			return nil, fmt.Errorf("worker %s block %s has no active controller (block may have been closed); try team_dispatch instead", worker.Name, worker.BlockID)
		}
		inputUnion := &blockcontroller.BlockInputUnion{
			InputData: []byte(parsed.Prompt + "\r"),
		}
		err = blockcontroller.SendInput(worker.BlockID, inputUnion)
		if err != nil {
			return nil, fmt.Errorf("failed to send prompt to worker terminal: %w", err)
		}
		return map[string]any{
			"success":  true,
			"workerid": parsed.WorkerId,
			"worker":   worker.Name,
			"prompt":   parsed.Prompt,
		}, nil
	}

	// Worker is idle (no terminal block) — auto-create task and execute
	taskTitle := parsed.Prompt
	if len(taskTitle) > 80 {
		taskTitle = taskTitle[:77] + "..."
	}

	taskData := wshrpc.TeamCreateTaskData{
		Title:       taskTitle,
		Description: parsed.Prompt,
		Priority:    team.PriorityMedium,
	}

	rpcClient := wshclient.GetBareRpcClient()
	task, err := wshclient.TeamCreateTaskCommand(rpcClient, taskData, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create task for worker %s: %w", worker.Name, err)
	}

	_, err = wshclient.TeamUpdateTaskCommand(rpcClient, wshrpc.TeamUpdateTaskData{
		TaskID:           task.TaskID,
		AssignedMemberID: worker.MemberID,
		AssignedWorkerID: worker.WorkerID,
		Status:           team.TaskStatusAssigned,
	}, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to assign task for worker %s: %w", worker.Name, err)
	}

	resp, err := wshclient.TeamExecuteTaskCommand(rpcClient, wshrpc.TeamExecuteTaskData{
		WorkerID: parsed.WorkerId,
		TaskID:   task.TaskID,
	}, &wshrpc.RpcOpts{Timeout: 30000})
	if err != nil {
		return nil, fmt.Errorf("failed to execute task on worker %s: %w", worker.Name, err)
	}

	return map[string]any{
		"success":  true,
		"workerid": parsed.WorkerId,
		"worker":   worker.Name,
		"prompt":   parsed.Prompt,
		"taskid":   task.TaskID,
		"blockid":  resp.BlockID,
		"tabid":    resp.TabID,
		"created":  true,
	}, nil
}

func GetTeamSendPromptToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_send_prompt",
		DisplayName: "Send Prompt to Worker",
		Description: "Send a prompt to a worker. If the worker already has an active terminal block, the text is typed into it. If the worker is idle (no terminal block), a task is automatically created, a terminal block is opened, the worker's CLI is started in the project directory, and the prompt is sent as the task description. Use this for both @mention dispatch and follow-up instructions.",
		ToolLogName: "team:sendprompt",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"workerid": map[string]any{
					"type":        "string",
					"description": "ID of the worker to send the prompt to",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "The text prompt to send to the worker's terminal",
				},
			},
			"required":             []string{"workerid", "prompt"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			inputBytes, _ := json.Marshal(input)
			var parsed teamSendPromptParams
			json.Unmarshal(inputBytes, &parsed)
			promptPreview := parsed.Prompt
			if len(promptPreview) > 60 {
				promptPreview = promptPreview[:60] + "..."
			}
			return fmt.Sprintf("sending prompt to worker %s: %s", parsed.WorkerId, promptPreview)
		},
		ToolAnyCallback: teamSendPromptCallback,
	}
}

// --- team_dispatch ---

type teamDispatchParams struct {
	Target    string `json:"target"`
	Message   string `json:"message"`
	ProjectId string `json:"projectid,omitempty"`
}

func teamDispatchCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	inputBytes, _ := json.Marshal(input)
	var parsed teamDispatchParams
	json.Unmarshal(inputBytes, &parsed)

	if parsed.Target == "" {
		return nil, fmt.Errorf("target is required")
	}
	if parsed.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	ctx := context.Background()
	message := parsed.Message
	rpcClient := wshclient.GetBareRpcClient()

	if parsed.ProjectId != "" {
		proj, err := team.GetProject(ctx, parsed.ProjectId)
		if err == nil && proj != nil {
			message = fmt.Sprintf("[Project: %s, Path: %s]\n%s", proj.Name, proj.Path, message)
		}
	}

	allWorkers, err := team.ListWorkers(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	var results []dispatchResult

	if parsed.Target == "all" {
		for _, w := range allWorkers {
			if w.Status == "offline" {
				continue
			}
			if w.BlockID == "" {
				dr, dispatchErr := autoStartWorker(rpcClient, w.WorkerID, w.MemberID, message)
				results = append(results, *dr)
				if dispatchErr != nil {
					results[len(results)-1].Error = dispatchErr.Error()
				}
				continue
			}
			ctrl := blockcontroller.HasController(w.BlockID)
			if !ctrl {
				dr, dispatchErr := autoStartWorker(rpcClient, w.WorkerID, w.MemberID, message)
				results = append(results, *dr)
				if dispatchErr != nil {
					results[len(results)-1].Error = dispatchErr.Error()
				}
				continue
			}
			inputUnion := &blockcontroller.BlockInputUnion{
				InputData: []byte(message + "\r"),
			}
			err := blockcontroller.SendInput(w.BlockID, inputUnion)
			results = append(results, dispatchResult{
				Worker:  w.Name,
				Success: err == nil,
				Error:   errToString(err),
			})
		}
	} else {
		var targetWorker *team.TeamWorker
		for _, w := range allWorkers {
			if w.Name == parsed.Target || w.WorkerID == parsed.Target {
				targetWorker = w
				break
			}
		}
		if targetWorker == nil {
			return nil, fmt.Errorf("worker not found: %s", parsed.Target)
		}
		if targetWorker.BlockID == "" {
			dispatchResult, dispatchErr := autoStartWorker(rpcClient, targetWorker.WorkerID, targetWorker.MemberID, message)
			if dispatchErr != nil {
				return nil, dispatchErr
			}
			results = append(results, *dispatchResult)
		} else {
			ctrl := blockcontroller.HasController(targetWorker.BlockID)
			if !ctrl {
				dispatchResult, dispatchErr := autoStartWorker(rpcClient, targetWorker.WorkerID, targetWorker.MemberID, message)
				if dispatchErr != nil {
					return nil, dispatchErr
				}
				results = append(results, *dispatchResult)
			} else {
				inputUnion := &blockcontroller.BlockInputUnion{
					InputData: []byte(message + "\r"),
				}
				err := blockcontroller.SendInput(targetWorker.BlockID, inputUnion)
				results = append(results, dispatchResult{
					Worker:  targetWorker.Name,
					Success: err == nil,
					Error:   errToString(err),
				})
			}
		}
	}

	return map[string]any{
		"target":  parsed.Target,
		"results": results,
	}, nil
}

type dispatchResult struct {
	Worker  string `json:"worker"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func autoStartWorker(rpcClient *wshutil.WshRpc, workerId string, memberId string, prompt string) (*dispatchResult, error) {
	taskTitle := prompt
	if len(taskTitle) > 80 {
		taskTitle = taskTitle[:77] + "..."
	}

	task, err := wshclient.TeamCreateTaskCommand(rpcClient, wshrpc.TeamCreateTaskData{
		Title:       taskTitle,
		Description: prompt,
		Priority:    team.PriorityMedium,
	}, &wshrpc.RpcOpts{})
	if err != nil {
		return &dispatchResult{Worker: workerId, Success: false, Error: err.Error()}, err
	}

	_, err = wshclient.TeamUpdateTaskCommand(rpcClient, wshrpc.TeamUpdateTaskData{
		TaskID:           task.TaskID,
		AssignedMemberID: memberId,
		AssignedWorkerID: workerId,
		Status:           team.TaskStatusAssigned,
	}, &wshrpc.RpcOpts{})
	if err != nil {
		return &dispatchResult{Worker: workerId, Success: false, Error: err.Error()}, err
	}

	resp, err := wshclient.TeamExecuteTaskCommand(rpcClient, wshrpc.TeamExecuteTaskData{
		WorkerID: workerId,
		TaskID:   task.TaskID,
	}, &wshrpc.RpcOpts{Timeout: 30000})
	if err != nil {
		return &dispatchResult{Worker: workerId, Success: false, Error: err.Error()}, err
	}

	return &dispatchResult{
		Worker:  workerId,
		Success: resp.Success,
	}, nil
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func GetTeamDispatchToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_dispatch",
		DisplayName: "Dispatch to Worker",
		Description: "Send a message or instruction to a worker by name or ID. Use 'all' as target to broadcast to all active workers. Idle workers (no terminal block) are automatically started. Optionally include a projectid to inject project context into the message. Resolves worker names automatically.",
		ToolLogName: "team:dispatch",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"description": "Worker name, worker ID, or 'all' to broadcast",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "The message or instruction to send",
				},
				"projectid": map[string]any{
					"type":        "string",
					"description": "Optional project ID to inject project context into the message",
				},
			},
			"required":             []string{"target", "message"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			inputBytes, _ := json.Marshal(input)
			var parsed teamDispatchParams
			json.Unmarshal(inputBytes, &parsed)
			return fmt.Sprintf("dispatching to %s", parsed.Target)
		},
		ToolAnyCallback: teamDispatchCallback,
	}
}

// --- team_get_task_output ---

type teamGetTaskOutputParams struct {
	TaskId string `json:"taskid"`
}

func teamGetTaskOutputCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamGetTaskOutputParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.TaskId == "" {
		return nil, fmt.Errorf("taskid is required")
	}

	rpcClient := wshclient.GetBareRpcClient()
	output, err := wshclient.TeamGetTaskOutputHistoryCommand(rpcClient, parsed.TaskId, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get task output: %w", err)
	}

	result := make([]map[string]any, len(output))
	for i, o := range output {
		result[i] = map[string]any{
			"timestamp": o.Timestamp,
			"type":      o.Type,
			"content":   o.Content,
		}
	}

	return map[string]any{
		"success": true,
		"taskid":  parsed.TaskId,
		"output":  result,
		"count":   len(output),
	}, nil
}

func GetTeamGetTaskOutputToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_get_task_output",
		DisplayName: "Get Task Output",
		Description: "Get the output history of a task (collected terminal output). Useful for reviewing what a worker produced.",
		ToolLogName: "team:gettaskoutput",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"taskid": map[string]any{
					"type":        "string",
					"description": "ID of the task to get output for",
				},
			},
			"required":             []string{"taskid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &teamGetTaskOutputParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("getting output for task %s", parsed.TaskId)
		},
		ToolAnyCallback: teamGetTaskOutputCallback,
	}
}

// --- team_list_activity ---

type teamListActivityParams struct {
	Limit    int    `json:"limit,omitempty"`
	TaskId   string `json:"taskid,omitempty"`
	WorkerId string `json:"workerid,omitempty"`
	MemberId string `json:"memberid,omitempty"`
}

func teamListActivityCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &teamListActivityParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.Limit == 0 {
		parsed.Limit = 50
	}

	data := wshrpc.TeamListActivityData{
		Limit:    parsed.Limit,
		TaskID:   parsed.TaskId,
		WorkerID: parsed.WorkerId,
		MemberID: parsed.MemberId,
	}

	rpcClient := wshclient.GetBareRpcClient()
	activities, err := wshclient.TeamListActivityCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list activity: %w", err)
	}

	result := make([]map[string]any, len(activities))
	for i, a := range activities {
		entry := map[string]any{
			"id":          a.Id,
			"type":        a.Type,
			"description": a.Description,
			"createdat":   a.CreatedAt,
		}
		if a.TaskID != "" {
			entry["taskid"] = a.TaskID
		}
		if a.WorkerID != "" {
			entry["workerid"] = a.WorkerID
		}
		if a.MemberID != "" {
			entry["memberid"] = a.MemberID
		}
		result[i] = entry
	}

	return map[string]any{
		"success":    true,
		"activities": result,
		"count":      len(activities),
	}, nil
}

func GetTeamListActivityToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "team_list_activity",
		DisplayName: "List Team Activity",
		Description: "Get the activity log filtered by task, worker, or member. Shows events like created, assigned, started, completed, failed, forked, recycled.",
		ToolLogName: "team:listactivity",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"default":     50,
					"description": "Maximum number of activity entries to return",
				},
				"taskid": map[string]any{
					"type":        "string",
					"description": "Filter by task ID",
				},
				"workerid": map[string]any{
					"type":        "string",
					"description": "Filter by worker ID",
				},
				"memberid": map[string]any{
					"type":        "string",
					"description": "Filter by member ID",
				},
			},
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			return "listing team activity"
		},
		ToolAnyCallback: teamListActivityCallback,
	}
}

// --- GetTeamToolDefinitions ---

func GetTeamToolDefinitions() []uctypes.ToolDefinition {
	return []uctypes.ToolDefinition{
		GetTeamForkWorkerToolDefinition(),
		GetTeamListWorkersToolDefinition(),
		GetTeamListMembersToolDefinition(),
		GetTeamCreateMemberToolDefinition(),
		GetTeamUpdateMemberToolDefinition(),
		GetTeamDeleteMemberToolDefinition(),
		GetTeamCreateTaskToolDefinition(),
		GetTeamAssignTaskToolDefinition(),
		GetTeamUpdateTaskToolDefinition(),
		GetTeamGetStatusToolDefinition(),
		GetTeamExecuteTaskToolDefinition(),
		GetTeamRecycleWorkerToolDefinition(),
		GetTeamSendPromptToolDefinition(),
		GetTeamDispatchToolDefinition(),
		GetTeamGetTaskOutputToolDefinition(),
		GetTeamListActivityToolDefinition(),
	}
}
