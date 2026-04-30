package aiusechat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wavetermdev/waveterm/pkg/aiusechat/uctypes"
	"github.com/wavetermdev/waveterm/pkg/cowork"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

type coworkCreateWorkerParams struct {
	Name       string `json:"name,omitempty"`
	Type       string `json:"type"`
	Role       string `json:"role,omitempty"`
	Desc       string `json:"desc,omitempty"`
	Soul       string `json:"soul,omitempty"`
	Skills     string `json:"skills,omitempty"`
	McpServers string `json:"mcpservers,omitempty"`
	CustomCmd  string `json:"customcmd,omitempty"`
	WorkDir    string `json:"workdir,omitempty"`
	MaxTasks   int    `json:"maxtasks,omitempty"`
}

type coworkListWorkersParams struct{}

type coworkCreateTaskParams struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
}

type coworkAssignTaskParams struct {
	TaskId   string `json:"taskid"`
	WorkerId string `json:"workerid"`
}

type coworkGetStatusParams struct{}

type coworkTerminateWorkerParams struct {
	WorkerId string `json:"workerid"`
}

func coworkCreateWorkerCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &coworkCreateWorkerParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.Type == "" {
		parsed.Type = "claude"
	}

	data := wshrpc.CoworkRegisterWorkerData{
		Name:       parsed.Name,
		Tool:       parsed.Type,
		Role:       parsed.Role,
		Desc:       parsed.Desc,
		Soul:       parsed.Soul,
		Skills:     parsed.Skills,
		McpServers: parsed.McpServers,
		CustomCmd:  parsed.CustomCmd,
		BlockId:    toolUseData.BlockId,
		TabId:      toolUseData.TabId,
	}

	if toolUseData.BlockId != "" {
		block, err := wstore.DBGet[*waveobj.Block](context.Background(), toolUseData.BlockId)
		if err == nil && block != nil && block.ParentORef != "" {
			oref, err := waveobj.ParseORef(block.ParentORef)
			if err == nil && oref.OType == "tab" {
				data.TabId = oref.OID
			}
		}
	}

	rpcClient := wshclient.GetBareRpcClient()
	worker, err := wshclient.CoworkRegisterWorkerCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}

	return map[string]any{
		"success":  true,
		"workerid": worker.WorkerId,
		"name":     worker.Name,
		"tool":     worker.Tool,
		"role":     worker.Role,
		"status":   worker.Status,
		"created":  worker.CreatedAt,
	}, nil
}

func coworkListWorkersCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	rpcClient := wshclient.GetBareRpcClient()
	workers, err := wshclient.CoworkListWorkersCommand(rpcClient, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	result := make([]map[string]any, len(workers))
	for i, w := range workers {
		result[i] = map[string]any{
			"workerid": w.WorkerId,
			"name":     w.Name,
			"tool":     w.Tool,
			"status":   w.Status,
			"created":  w.CreatedAt,
		}
	}

	return map[string]any{
		"success": true,
		"workers": result,
		"count":   len(workers),
	}, nil
}

func coworkCreateTaskCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &coworkCreateTaskParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	if parsed.Priority == "" {
		parsed.Priority = "medium"
	}

	data := wshrpc.CoworkCreateTaskData{
		Title:       parsed.Title,
		Description: parsed.Description,
		Priority:    parsed.Priority,
	}

	rpcClient := wshclient.GetBareRpcClient()
	task, err := wshclient.CoworkCreateTaskCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return map[string]any{
		"success":  true,
		"taskid":   task.TaskId,
		"title":    task.Title,
		"status":   task.Status,
		"priority": task.Priority,
		"created":  task.CreatedAt,
	}, nil
}

func coworkAssignTaskCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &coworkAssignTaskParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.TaskId == "" || parsed.WorkerId == "" {
		return nil, fmt.Errorf("taskid and workerid are required")
	}

	data := wshrpc.CoworkUpdateTaskData{
		TaskId:         parsed.TaskId,
		AssignedWorker: parsed.WorkerId,
		Status:         "assigned",
	}

	rpcClient := wshclient.GetBareRpcClient()
	_, err := wshclient.CoworkUpdateTaskCommand(rpcClient, data, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to assign task: %w", err)
	}

	return map[string]any{
		"success":  true,
		"taskid":   parsed.TaskId,
		"workerid": parsed.WorkerId,
		"message":  "Task assigned to worker",
	}, nil
}

func coworkGetStatusCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	ctx := context.Background()
	rpcClient := wshclient.GetBareRpcClient()
	status, err := wshclient.CoworkGetStatusCommand(rpcClient, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	workers, _ := wshclient.CoworkListWorkersCommand(rpcClient, &wshrpc.RpcOpts{})
	workerList := make([]map[string]any, 0, len(workers))
	for _, w := range workers {
		workerList = append(workerList, map[string]any{
			"workerid": w.WorkerId,
			"name":     w.Name,
			"tool":     w.Tool,
			"status":   w.Status,
		})
	}

	tasks, _ := cowork.ListTasks(ctx, "", "")
	taskList := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		entry := map[string]any{
			"taskid":   t.TaskId,
			"title":    t.Title,
			"status":   t.Status,
			"priority": t.Priority,
		}
		if t.Description != "" {
			entry["description"] = t.Description
		}
		if t.AssignedWorker != "" {
			entry["assignedworker"] = t.AssignedWorker
		}
		if t.Error != "" {
			entry["error"] = t.Error
		}
		taskList = append(taskList, entry)
	}

	return map[string]any{
		"success":       true,
		"pendingtasks":  status.PendingTasks,
		"workingtasks":  status.WorkingTasks,
		"donetasks":     status.DoneTasks,
		"failedtasks":   status.FailedTasks,
		"activeworkers": status.ActiveWorkers,
		"idleworkers":   status.IdleWorkers,
		"totalworkers":  status.TotalWorkers,
		"workers":       workerList,
		"tasks":         taskList,
	}, nil
}

type coworkExecuteTaskParams struct {
	WorkerId string `json:"workerid"`
	TaskId   string `json:"taskid"`
	Command  string `json:"command,omitempty"`
}

func coworkExecuteTaskCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &coworkExecuteTaskParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.WorkerId == "" {
		return nil, fmt.Errorf("workerid is required")
	}
	if parsed.TaskId == "" {
		return nil, fmt.Errorf("taskid is required")
	}

	data := wshrpc.CoworkExecuteTaskData{
		WorkerId: parsed.WorkerId,
		TaskId:   parsed.TaskId,
		Command:  parsed.Command,
	}

	rpcClient := wshclient.GetBareRpcClient()
	resp, err := wshclient.CoworkExecuteTaskCommand(rpcClient, data, &wshrpc.RpcOpts{Timeout: 30000})
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}

	return map[string]any{
		"success": resp.Success,
		"blockid": resp.BlockId,
		"tabid":   resp.TabId,
		"error":   resp.Error,
	}, nil
}

func coworkTerminateWorkerCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &coworkTerminateWorkerParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.WorkerId == "" {
		return nil, fmt.Errorf("workerid is required")
	}

	ctx := context.Background()
	rpcClient := wshclient.GetBareRpcClient()

	worker, err := cowork.GetWorker(ctx, parsed.WorkerId)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker: %w", err)
	}

	releasedTask := ""
	if worker.AssignedTask != "" {
		task, taskErr := cowork.GetTask(ctx, worker.AssignedTask)
		if taskErr == nil && task != nil && (task.Status == "working" || task.Status == "assigned") {
			task.Status = "pending"
			task.AssignedWorker = ""
			task.UpdatedAt = time.Now().Unix()
			cowork.UpdateTask(ctx, task)
			releasedTask = task.TaskId
		}
	}

	worker.Status = "idle"
	worker.AssignedTask = ""
	worker.BlockId = ""
	worker.LastActiveAt = time.Now().Unix()
	_, err = wshclient.CoworkUpdateWorkerCommand(rpcClient, wshrpc.CoworkUpdateWorkerData{
		WorkerId: worker.WorkerId,
		Status:   "idle",
	}, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to release worker: %w", err)
	}

	cowork.PublishWorkerUpdate()
	cowork.PublishTaskUpdate()

	result := map[string]any{
		"success":  true,
		"workerid": parsed.WorkerId,
		"name":     worker.Name,
		"message":  "Worker released and set to idle",
	}
	if releasedTask != "" {
		result["releasedtask"] = releasedTask
		result["taskstatus"] = "pending"
	}
	return result, nil
}

func GetCoworkCreateWorkerToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_create_worker",
		DisplayName: "Create Cowork Worker",
		Description: "Create a new AI worker agent that can execute tasks. The worker runs in a terminal and can be supervised by the main AI.",
		ToolLogName: "cowork:createworker",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name for the worker",
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"claude", "opencode", "cursor", "aider", "custom"},
					"default":     "claude",
					"description": "Type of AI worker: claude, opencode, cursor, aider, or custom (with custom_cmd)",
				},
				"role": map[string]any{
					"type":        "string",
					"description": "Worker role (e.g., 'frontend-dev', 'backend-dev', 'reviewer')",
				},
				"desc": map[string]any{
					"type":        "string",
					"description": "Description of what this worker does",
				},
				"soul": map[string]any{
					"type":        "string",
					"description": "Core personality/system prompt for the worker",
				},
				"skills": map[string]any{
					"type":        "string",
					"description": "Comma-separated skill names to equip the worker with",
				},
				"mcpservers": map[string]any{
					"type":        "string",
					"description": "Comma-separated MCP server names to attach to the worker",
				},
				"customcmd": map[string]any{
					"type":        "string",
					"description": "Custom CLI command to run (required when type=custom)",
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory for the worker",
				},
				"maxtasks": map[string]any{
					"type":        "integer",
					"default":     10,
					"description": "Maximum number of tasks the worker can accept",
				},
			},
			"required":             []string{"type"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &coworkCreateWorkerParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("creating worker (type: %s, role: %s)", parsed.Type, parsed.Role)
		},
		ToolAnyCallback: coworkCreateWorkerCallback,
	}
}

func GetCoworkListWorkersToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_list_workers",
		DisplayName: "List Cowork Workers",
		Description: "List all available AI worker agents and their current status.",
		ToolLogName: "cowork:listworkers",
		Strict:      false,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			return "listing all workers"
		},
		ToolAnyCallback: coworkListWorkersCallback,
	}
}

func GetCoworkCreateTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_create_task",
		DisplayName: "Create Cowork Task",
		Description: "Create a new task that can be assigned to a worker agent.",
		ToolLogName: "cowork:createtask",
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
					"enum":        []string{"low", "medium", "high"},
					"default":     "medium",
					"description": "Priority of the task",
				},
			},
			"required":             []string{"title"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &coworkCreateTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("creating task: %s", parsed.Title)
		},
		ToolAnyCallback: coworkCreateTaskCallback,
	}
}

func GetCoworkAssignTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_assign_task",
		DisplayName: "Assign Task to Worker",
		Description: "Assign an existing task to a specific worker for execution.",
		ToolLogName: "cowork:assigntask",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"taskid": map[string]any{
					"type":        "string",
					"description": "ID of the task to assign",
				},
				"workerid": map[string]any{
					"type":        "string",
					"description": "ID of the worker to assign the task to",
				},
			},
			"required":             []string{"taskid", "workerid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &coworkAssignTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("assigning task %s to worker %s", parsed.TaskId, parsed.WorkerId)
		},
		ToolAnyCallback: coworkAssignTaskCallback,
	}
}

func GetCoworkGetStatusToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_get_status",
		DisplayName: "Get Cowork Status",
		Description: "Get the full Cowork overview: task counts, all workers with their status, and all tasks with their status/priority/assignment. Call this first to understand the current state before creating tasks, assigning workers, or executing tasks.",
		ToolLogName: "cowork:getstatus",
		Strict:      false,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			return "getting Cowork status"
		},
		ToolAnyCallback: coworkGetStatusCallback,
	}
}

func GetCoworkExecuteTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_execute_task",
		DisplayName: "Execute Task on Worker",
		Description: "Execute a task by spawning a terminal block and running the worker's CLI with the task description.",
		ToolLogName: "cowork:executetask",
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
			parsed := &coworkExecuteTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("executing task %s on worker %s", parsed.TaskId, parsed.WorkerId)
		},
		ToolAnyCallback: coworkExecuteTaskCallback,
	}
}

func GetCoworkTerminateWorkerToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_terminate_worker",
		DisplayName: "Terminate Cowork Worker",
		Description: "Terminate and clean up a running worker agent.",
		ToolLogName: "cowork:terminateworker",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"workerid": map[string]any{
					"type":        "string",
					"description": "ID of the worker to terminate",
				},
			},
			"required":             []string{"workerid"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &coworkTerminateWorkerParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("terminating worker: %s", parsed.WorkerId)
		},
		ToolAnyCallback: coworkTerminateWorkerCallback,
	}
}

func GetCoworkToolDefinitions() []uctypes.ToolDefinition {
	return []uctypes.ToolDefinition{
		GetCoworkCreateWorkerToolDefinition(),
		GetCoworkListWorkersToolDefinition(),
		GetCoworkCreateTaskToolDefinition(),
		GetCoworkAssignTaskToolDefinition(),
		GetCoworkGetStatusToolDefinition(),
		GetCoworkTerminateWorkerToolDefinition(),
		GetCoworkExecuteTaskToolDefinition(),
	}
}
