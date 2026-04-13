package aiusechat

import (
	"encoding/json"
	"fmt"

	"github.com/wavetermdev/waveterm/pkg/aiusechat/uctypes"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
)

type coworkCreateWorkerParams struct {
	Name     string `json:"name,omitempty"`
	Type     string `json:"type"`
	Prompt   string `json:"prompt,omitempty"`
	WorkDir  string `json:"workdir,omitempty"`
	MaxTasks int    `json:"maxtasks,omitempty"`
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
		Name:    parsed.Name,
		Tool:    parsed.Type,
		BlockId: toolUseData.BlockId,
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
	rpcClient := wshclient.GetBareRpcClient()
	status, err := wshclient.CoworkGetStatusCommand(rpcClient, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
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
	}, nil
}

func coworkTerminateWorkerCallback(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
	parsed := &coworkTerminateWorkerParams{}
	inputBytes, _ := json.Marshal(input)
	json.Unmarshal(inputBytes, parsed)

	if parsed.WorkerId == "" {
		return nil, fmt.Errorf("workerid is required")
	}

	rpcClient := wshclient.GetBareRpcClient()
	err := wshclient.CoworkDeleteWorkerCommand(rpcClient, parsed.WorkerId, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to terminate worker: %w", err)
	}

	return map[string]any{
		"success":  true,
		"workerid": parsed.WorkerId,
		"message":  "Worker terminated successfully",
	}, nil
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
					"description": "Optional name for the worker",
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"claude", "opencode", "cursor", "aider"},
					"default":     "claude",
					"description": "Type of AI worker: claude, opencode, cursor, or aider",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "System prompt/instructions for the worker",
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
			return fmt.Sprintf("creating worker (type: %s)", parsed.Type)
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
		Description: "Get the overall status of Cowork including task counts and worker statistics.",
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
	}
}
