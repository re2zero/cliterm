// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
)

var teamMcpCmd = &cobra.Command{
	Use:    "team-mcp",
	Short:  "Start MCP server for team management",
	Long:   "Starts a stdio-based MCP (Model Context Protocol) server that exposes team management tools to Worker LLMs",
	RunE:   teamMcpRun,
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(teamMcpCmd)
}

// JSON-RPC 2.0 request structure
type jsonRPCRequest struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// JSON-RPC 2.0 response structure
type jsonRPCResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

// MCP tool definitions
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content []mcpContentItem `json:"content"`
}

type mcpToolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

// MCP server info for initialize
type mcpServerInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities struct{} `json:"capabilities"`
}

func teamMcpRun(cmd *cobra.Command, args []string) error {
	if err := preRunSetupRpcClient(cmd, args); err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(req.ID, -32700, "Parse error")
			continue
		}

		if req.JsonRPC != "2.0" {
			sendError(req.ID, -32600, "Invalid JSON-RPC version")
			continue
		}

		handleRequest(&req)
	}

	return scanner.Err()
}

func handleRequest(req *jsonRPCRequest) {
	switch req.Method {
	case "initialize":
		handleInitialize(req)
	case "notifications/initialized":
		// No response needed for notifications
	case "tools/list":
		handleToolsList(req)
	case "tools/call":
		handleToolsCall(req)
	default:
		sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func handleInitialize(req *jsonRPCRequest) {
	serverInfo := mcpServerInfo{
		Name:    "waveterm-team-mcp",
		Version: "1.0.0",
	}
	sendResult(req.ID, serverInfo)
}

func handleToolsList(req *jsonRPCRequest) {
	tools := []mcpTool{
		{
			Name:        "team_update_task",
			Description: "Update a team task's status, progress, result, or error",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"taskId": map[string]any{
						"type":        "string",
						"description": "The task ID to update (uses WAVE_TASK_ID env var if empty)",
					},
					"status": map[string]any{
						"type":        "string",
						"enum":        []string{"done", "failed", "paused"},
						"description": "New task status",
					},
					"progress": map[string]any{
						"type":        "integer",
						"description": "Progress percentage (0-100)",
					},
					"result": map[string]any{
						"type":        "string",
						"description": "Task result summary",
					},
					"error": map[string]any{
						"type":        "string",
						"description": "Error message if task failed",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "team_dispatch",
			Description: "Dispatch a prompt to a specific worker or all workers",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target": map[string]any{
						"type":        "string",
						"description": "Target worker name or 'all' for all active workers",
					},
					"prompt": map[string]any{
						"type":        "string",
						"description": "The prompt to dispatch",
					},
				},
				"required": []string{"target", "prompt"},
			},
		},
		{
			Name:        "team_get_status",
			Description: "Get overall team status including workers and tasks",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
		{
			Name:        "team_list_workers",
			Description: "List all workers for a member (or all workers if memberId is empty)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"memberId": map[string]any{
						"type":        "string",
						"description": "Optional member ID to filter workers",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "team_list_projects",
			Description: "List all team projects",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
		{
			Name:        "team_get_project",
			Description: "Get a specific project by name",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Project name to search for",
					},
				},
				"required": []string{"name"},
			},
		},
	}

	result := mcpToolsListResult{Tools: tools}
	sendResult(req.ID, result)
}

func handleToolsCall(req *jsonRPCRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params")
		return
	}

	switch params.Name {
	case "team_update_task":
		handleTeamUpdateTask(req.ID, params.Arguments)
	case "team_dispatch":
		handleTeamDispatch(req.ID, params.Arguments)
	case "team_get_status":
		handleTeamGetStatus(req.ID)
	case "team_list_workers":
		handleTeamListWorkers(req.ID, params.Arguments)
	case "team_list_projects":
		handleTeamListProjects(req.ID)
	case "team_get_project":
		handleTeamGetProject(req.ID, params.Arguments)
	default:
		sendError(req.ID, -32601, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

func handleTeamUpdateTask(id interface{}, args json.RawMessage) {
	var params struct {
		TaskID   string `json:"taskId"`
		Status   string `json:"status"`
		Progress int    `json:"progress"`
		Result   string `json:"result"`
		Error    string `json:"error"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		sendError(id, -32602, "Invalid arguments for team_update_task")
		return
	}

	// Use WAVE_TASK_ID env var if taskId is empty
	if params.TaskID == "" {
		params.TaskID = os.Getenv("WAVE_TASK_ID")
		if params.TaskID == "" {
			sendError(id, -32602, "taskId is required and WAVE_TASK_ID env var is not set")
			return
		}
	}

	// Validate status if provided
	if params.Status != "" {
		if params.Status != "done" && params.Status != "failed" && params.Status != "paused" {
			sendError(id, -32602, "status must be one of: done, failed, paused")
			return
		}
	}

	updateData := wshrpc.TeamUpdateTaskData{
		TaskID:   params.TaskID,
		Status:   params.Status,
		Progress: params.Progress,
		Result:   params.Result,
		Error:    params.Error,
	}

	task, err := wshclient.TeamUpdateTaskCommand(RpcClient, updateData, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		sendError(id, -32603, fmt.Sprintf("Failed to update task: %s", err.Error()))
		return
	}

	resultJSON, _ := json.MarshalIndent(task, "", "  ")
	sendResult(id, mcpToolResult{
		Content: []mcpContentItem{
			{Type: "text", Text: string(resultJSON)},
		},
	})
}

func handleTeamDispatch(id interface{}, args json.RawMessage) {
	var params struct {
		Target string `json:"target"`
		Prompt string `json:"prompt"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		sendError(id, -32602, "Invalid arguments for team_dispatch")
		return
	}

	if params.Target == "" {
		sendError(id, -32602, "target is required")
		return
	}

	if params.Prompt == "" {
		sendError(id, -32602, "prompt is required")
		return
	}

	// Get current worker ID from env
	workerID := os.Getenv("WAVE_WORKER_ID")

	// If target is "all", dispatch to all active workers
	if params.Target == "all" {
		workers, err := wshclient.TeamListWorkersCommand(RpcClient, "", &wshrpc.RpcOpts{Timeout: 10000})
		if err != nil {
			sendError(id, -32603, fmt.Sprintf("Failed to list workers: %s", err.Error()))
			return
		}

		var results []string
		for _, worker := range workers {
			if worker.Status == "active" && worker.WorkerID != workerID {
				if err := sendPromptToWorker(worker.WorkerID, params.Prompt); err != nil {
					results = append(results, fmt.Sprintf("Failed to dispatch to %s: %s", worker.Name, err.Error()))
				} else {
					results = append(results, fmt.Sprintf("Dispatched to %s", worker.Name))
				}
			}
		}

		resultText := strings.Join(results, "\n")
		sendResult(id, mcpToolResult{
			Content: []mcpContentItem{
				{Type: "text", Text: resultText},
			},
		})
		return
	}

	// Resolve target by name
	workers, err := wshclient.TeamListWorkersCommand(RpcClient, "", &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		sendError(id, -32603, fmt.Sprintf("Failed to list workers: %s", err.Error()))
		return
	}

	var targetWorkerID string
	for _, worker := range workers {
		if worker.Name == params.Target {
			targetWorkerID = worker.WorkerID
			break
		}
	}

	if targetWorkerID == "" {
		sendError(id, -32602, fmt.Sprintf("Worker not found: %s", params.Target))
		return
	}

	if err := sendPromptToWorker(targetWorkerID, params.Prompt); err != nil {
		sendError(id, -32603, fmt.Sprintf("Failed to dispatch prompt: %s", err.Error()))
		return
	}

	sendResult(id, mcpToolResult{
		Content: []mcpContentItem{
			{Type: "text", Text: fmt.Sprintf("Dispatched prompt to worker: %s", params.Target)},
		},
	})
}

func sendPromptToWorker(workerID string, prompt string) error {
	// Create a task for the worker
	createData := wshrpc.TeamCreateTaskData{
		Title:       fmt.Sprintf("Prompt: %s", prompt[:min(50, len(prompt))]),
		Description: prompt,
		Priority:    "normal",
	}

	task, err := wshclient.TeamCreateTaskCommand(RpcClient, createData, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	// Assign the task to the worker
	updateData := wshrpc.TeamUpdateTaskData{
		TaskID:           task.TaskID,
		AssignedWorkerID: workerID,
	}

	_, err = wshclient.TeamUpdateTaskCommand(RpcClient, updateData, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		return fmt.Errorf("failed to assign task to worker: %w", err)
	}

	return nil
}

func handleTeamGetStatus(id interface{}) {
	status, err := wshclient.TeamGetStatusCommand(RpcClient, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		sendError(id, -32603, fmt.Sprintf("Failed to get status: %s", err.Error()))
		return
	}

	resultJSON, _ := json.MarshalIndent(status, "", "  ")
	sendResult(id, mcpToolResult{
		Content: []mcpContentItem{
			{Type: "text", Text: string(resultJSON)},
		},
	})
}

func handleTeamListWorkers(id interface{}, args json.RawMessage) {
	var params struct {
		MemberID string `json:"memberId"`
	}

	_ = json.Unmarshal(args, &params) // MemberID is optional

	workers, err := wshclient.TeamListWorkersCommand(RpcClient, params.MemberID, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		sendError(id, -32603, fmt.Sprintf("Failed to list workers: %s", err.Error()))
		return
	}

	resultJSON, _ := json.MarshalIndent(workers, "", "  ")
	sendResult(id, mcpToolResult{
		Content: []mcpContentItem{
			{Type: "text", Text: string(resultJSON)},
		},
	})
}

func handleTeamListProjects(id interface{}) {
	projects, err := wshclient.TeamListProjectsCommand(RpcClient, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		sendError(id, -32603, fmt.Sprintf("Failed to list projects: %s", err.Error()))
		return
	}

	resultJSON, _ := json.MarshalIndent(projects, "", "  ")
	sendResult(id, mcpToolResult{
		Content: []mcpContentItem{
			{Type: "text", Text: string(resultJSON)},
		},
	})
}

func handleTeamGetProject(id interface{}, args json.RawMessage) {
	var params struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		sendError(id, -32602, "Invalid arguments for team_get_project")
		return
	}

	if params.Name == "" {
		sendError(id, -32602, "name is required")
		return
	}

	projects, err := wshclient.TeamListProjectsCommand(RpcClient, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		sendError(id, -32603, fmt.Sprintf("Failed to list projects: %s", err.Error()))
		return
	}

	for _, project := range projects {
		if project.Name == params.Name {
			resultJSON, _ := json.MarshalIndent(project, "", "  ")
			sendResult(id, mcpToolResult{
				Content: []mcpContentItem{
					{Type: "text", Text: string(resultJSON)},
				},
			})
			return
		}
	}

	sendError(id, -32602, fmt.Sprintf("Project not found: %s", params.Name))
}

func sendResult(id interface{}, result interface{}) {
	resp := jsonRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	sendResponse(resp)
}

func sendError(id interface{}, code int64, message string) {
	resp := jsonRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	}
	sendResponse(resp)
}

func sendResponse(resp jsonRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	fmt.Printf("%s\n", string(data))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseJSONRPCRequest parses a JSON-RPC request from a raw JSON string
func parseJSONRPCRequest(data string) (*jsonRPCRequest, error) {
	var req jsonRPCRequest
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return nil, err
	}
	return &req, nil
}
