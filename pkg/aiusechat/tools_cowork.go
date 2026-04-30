package aiusechat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wavetermdev/waveterm/pkg/aiusechat/uctypes"
	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
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
	WorkerId   string `json:"workerid"`
	TaskStatus string `json:"task_status"`
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

	taskStatus := parsed.TaskStatus
	if taskStatus == "" {
		taskStatus = "done"
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
		if taskErr == nil && task != nil && task.Status != "done" && task.Status != "failed" {
			task.Status = taskStatus
			task.AssignedWorker = ""
			task.UpdatedAt = time.Now().Unix()
			cowork.UpdateTask(ctx, task)
			releasedTask = task.TaskId
		}
	}

	var completedTaskIds []string
	if worker.CompletedTasks != "" {
		json.Unmarshal([]byte(worker.CompletedTasks), &completedTaskIds)
	}
	if releasedTask != "" {
		completedTaskIds = append(completedTaskIds, releasedTask)
	}
	completedJson, _ := json.Marshal(completedTaskIds)

	blockId := worker.BlockId

	_, err = wshclient.CoworkUpdateWorkerCommand(rpcClient, wshrpc.CoworkUpdateWorkerData{
		WorkerId:       worker.WorkerId,
		Status:         "idle",
		AssignedTask:   "",
		CompletedTasks: string(completedJson),
	}, &wshrpc.RpcOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to update worker: %w", err)
	}

	if blockId != "" {
		wshclient.DeleteBlockCommand(rpcClient, wshrpc.CommandDeleteBlockData{
			BlockId: blockId,
		}, &wshrpc.RpcOpts{})
	}

	cowork.PublishWorkerUpdate()
	cowork.PublishTaskUpdate()

	result := map[string]any{
		"success":       true,
		"workerid":      parsed.WorkerId,
		"name":          worker.Name,
		"message":       "Worker set to idle, task completed, terminal block closed",
		"completedtasks": len(completedTaskIds),
	}
	if releasedTask != "" {
		result["taskid"] = releasedTask
		result["taskstatus"] = taskStatus
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
		Description: "Execute a task by creating a terminal block and running the worker's CLI with the task description. Use this for NEW tasks that need a fresh terminal. For sending additional instructions to an already-running worker, use cowork_send_prompt instead.",
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
		Description: "Terminate a worker: closes its terminal block and updates task status. Use when a worker has finished its work or needs to be stopped.",
		ToolLogName: "cowork:terminateworker",
		Strict:      false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"workerid": map[string]any{
					"type":        "string",
					"description": "ID of the worker to terminate",
				},
				"task_status": map[string]any{
					"type":        "string",
					"description": "Status to set for the worker's assigned task. Use 'done' when task is completed, 'failed' if task failed. Default: 'done'",
					"enum":        []string{"done", "failed", "pending"},
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
		GetCoworkUpdateTaskToolDefinition(),
		GetCoworkGetStatusToolDefinition(),
		GetCoworkTerminateWorkerToolDefinition(),
		GetCoworkExecuteTaskToolDefinition(),
		GetCoworkSendPromptToolDefinition(),
	}
}

type coworkUpdateTaskParams struct {
	TaskId string `json:"taskid"`
	Status string `json:"status"`
}

func GetCoworkUpdateTaskToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_update_task",
		DisplayName: "Update Cowork Task Status",
		Description: "Update the status of a task directly by task ID. Use this when you need to mark a task as done or failed without terminating its worker, or to fix an incorrect task status.",
		ToolLogName: "cowork:updatetask",
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
					"description": "New status for the task",
					"enum":        []string{"done", "failed", "pending", "working", "assigned"},
				},
			},
			"required":             []string{"taskid", "status"},
			"additionalProperties": false,
		},
		ToolCallDesc: func(input any, output any, toolUseData *uctypes.UIMessageDataToolUse) string {
			parsed := &coworkUpdateTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			return fmt.Sprintf("updating task %s → %s", parsed.TaskId[:8], parsed.Status)
		},
		ToolAnyCallback: func(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			parsed := &coworkUpdateTaskParams{}
			inputBytes, _ := json.Marshal(input)
			json.Unmarshal(inputBytes, parsed)
			if parsed.TaskId == "" {
				return nil, fmt.Errorf("taskid is required")
			}
			if parsed.Status == "" {
				return nil, fmt.Errorf("status is required")
			}
			ctx := context.Background()
			task, err := cowork.GetTask(ctx, parsed.TaskId)
			if err != nil {
				return nil, fmt.Errorf("failed to get task: %w", err)
			}
			oldStatus := task.Status
			task.Status = parsed.Status
			task.UpdatedAt = time.Now().Unix()
			if err := cowork.UpdateTask(ctx, task); err != nil {
				return nil, fmt.Errorf("failed to update task: %w", err)
			}
			cowork.PublishTaskUpdate()
			return map[string]any{
				"success":   true,
				"taskid":    parsed.TaskId,
				"oldstatus": oldStatus,
				"status":    parsed.Status,
				"title":     task.Title,
			}, nil
		},
	}
}

type coworkSendPromptParams struct {
	WorkerId string `json:"workerid"`
	Prompt   string `json:"prompt"`
}

func GetCoworkSendPromptToolDefinition() uctypes.ToolDefinition {
	return uctypes.ToolDefinition{
		Name:        "cowork_send_prompt",
		DisplayName: "Send Prompt to Worker",
		Description: "Send a text prompt directly to a running worker's terminal block. Use this to give additional instructions to a worker that is already executing a task, without creating a new task. The text is typed into the worker's terminal and Enter is pressed.",
		ToolLogName: "cowork:sendprompt",
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
			var parsed coworkSendPromptParams
			json.Unmarshal(inputBytes, &parsed)
			promptPreview := parsed.Prompt
			if len(promptPreview) > 60 {
				promptPreview = promptPreview[:60] + "..."
			}
			return fmt.Sprintf("sending prompt to worker %s: %s", parsed.WorkerId[:8], promptPreview)
		},
		ToolAnyCallback: func(input any, toolUseData *uctypes.UIMessageDataToolUse) (any, error) {
			inputBytes, err := json.Marshal(input)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal input: %w", err)
			}
			var parsed coworkSendPromptParams
			if err := json.Unmarshal(inputBytes, &parsed); err != nil {
				return nil, fmt.Errorf("failed to unmarshal input: %w", err)
			}
			if parsed.WorkerId == "" {
				return nil, fmt.Errorf("workerid is required")
			}
			if parsed.Prompt == "" {
				return nil, fmt.Errorf("prompt is required")
			}

			ctx := context.Background()
			worker, err := cowork.GetWorker(ctx, parsed.WorkerId)
			if err != nil {
				return nil, fmt.Errorf("worker not found: %w", err)
			}
			if worker.BlockId == "" {
				return nil, fmt.Errorf("worker %s has no terminal block", worker.Name)
			}

			inputUnion := &blockcontroller.BlockInputUnion{
				InputData: []byte(parsed.Prompt + "\r"),
			}
			err = blockcontroller.SendInput(worker.BlockId, inputUnion)
			if err != nil {
				return nil, fmt.Errorf("failed to send prompt to worker terminal: %w", err)
			}

			return map[string]any{
				"success":  true,
				"workerid": parsed.WorkerId,
				"worker":   worker.Name,
				"prompt":   parsed.Prompt,
			}, nil
		},
	}
}
