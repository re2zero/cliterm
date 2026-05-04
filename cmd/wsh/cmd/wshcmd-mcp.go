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
)

var mcpToolsFlag string

var mcpCmd = &cobra.Command{
	Use:    "mcp",
	Short:  "Start an MCP stdio server",
	Long:   "Start a Model Context Protocol server over stdio. Use --tools to select which tool groups to expose.",
	Example: "  wsh mcp --tools=team\n  wsh mcp --tools=team,file",
	Hidden: true,
	RunE:   mcpRun,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().StringVar(&mcpToolsFlag, "tools", "", "Comma-separated list of tool groups to expose (e.g. team,file)")
	mcpCmd.MarkFlagRequired("tools")
}

type jsonRPCRequest struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCResponse struct {
	JsonRPC interface{} `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var teamTools = []mcpTool{
	{
		Name:        "team_update_task",
		Description: "Update your assigned task status and result. You MUST call this when done or on error. Uses WAVE_TASK_ID from environment.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":   map[string]any{"type": "string", "description": "Task status: done, failed, paused, or working", "enum": []string{"done", "failed", "paused", "working"}},
				"result":   map[string]any{"type": "string", "description": "Brief summary of what was accomplished"},
				"error":    map[string]any{"type": "string", "description": "Error description if status is failed"},
				"progress": map[string]any{"type": "integer", "description": "Progress percentage 0-100", "minimum": 0, "maximum": 100},
			},
			"required":             []string{"status"},
			"additionalProperties": false,
		},
	},
	{
		Name:        "team_dispatch",
		Description: "Send a message or instruction to another team worker by name, or broadcast to all. Resolves worker names automatically.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target":  map[string]any{"type": "string", "description": "Worker name, worker ID, or 'all' to broadcast"},
				"message": map[string]any{"type": "string", "description": "The message or instruction to send"},
			},
			"required":             []string{"target", "message"},
			"additionalProperties": false,
		},
	},
	{
		Name:        "team_get_status",
		Description: "Get current team status: worker counts and task counts by state.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
	},
	{
		Name:        "team_list_workers",
		Description: "List all team workers with their status.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
	},
}

func mcpRun(cmd *cobra.Command, args []string) error {
	if err := preRunSetupRpcClient(cmd, args); err != nil {
		return err
	}

	toolMap := map[string][]mcpTool{
		"team": teamTools,
	}

	var activeTools []mcpTool
	toolGroups := splitComma(mcpToolsFlag)
	for _, g := range toolGroups {
		if tools, ok := toolMap[g]; ok {
			activeTools = append(activeTools, tools...)
		}
	}

	if len(activeTools) == 0 {
		return fmt.Errorf("no valid tool groups specified. Available: team")
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(nil, -32700, "parse error")
			continue
		}

		switch req.Method {
		case "initialize":
			sendResult(req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "wsh-mcp", "version": "1.0"},
			})
		case "notifications/initialized":
		case "tools/list":
			sendResult(req.ID, map[string]any{"tools": activeTools})
		case "tools/call":
			handleToolCall(req)
		default:
			sendError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
		}
	}
	return nil
}

func handleToolCall(req jsonRPCRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendError(req.ID, -32600, "invalid params")
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
		handleTeamListWorkers(req.ID)
	default:
		sendError(req.ID, -32601, fmt.Sprintf("unknown tool: %s", params.Name))
	}
}

func handleTeamUpdateTask(id interface{}, raw json.RawMessage) {
	var args struct {
		Status   string `json:"status"`
		Result   string `json:"result"`
		Error    string `json:"error"`
		Progress int    `json:"progress"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		sendError(id, -32600, "invalid arguments")
		return
	}

	taskId := os.Getenv("WAVE_TASK_ID")
	if taskId == "" {
		sendError(id, -32602, "WAVE_TASK_ID not set")
		return
	}

	validStatuses := map[string]bool{"done": true, "failed": true, "paused": true, "working": true}
	if !validStatuses[args.Status] {
		sendError(id, -32602, fmt.Sprintf("invalid status %q, must be: done, failed, paused, working", args.Status))
		return
	}

	data := wshrpc.TeamUpdateTaskData{
		TaskID:   taskId,
		Status:   args.Status,
		Result:   args.Result,
		Error:    args.Error,
		Progress: args.Progress,
	}

	result, err := wshclient.TeamUpdateTaskCommand(RpcClient, data, nil)
	if err != nil {
		sendError(id, -32000, err.Error())
		return
	}

	sendToolResult(id, fmt.Sprintf("Task %s updated: status=%s", result.TaskID, result.Status))
}

func handleTeamDispatch(id interface{}, raw json.RawMessage) {
	var args struct {
		Target  string `json:"target"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		sendError(id, -32600, "invalid arguments")
		return
	}

	workers, err := wshclient.TeamListWorkersCommand(RpcClient, "", nil)
	if err != nil {
		sendError(id, -32000, fmt.Sprintf("list workers: %v", err))
		return
	}

	if args.Target == "all" {
		sent := 0
		for _, w := range workers {
			if w.Status == "offline" || w.BlockID == "" {
				continue
			}
			err := wshclient.TeamSendPromptCommand(RpcClient, wshrpc.TeamSendPromptData{
				WorkerID: w.WorkerID,
				Prompt:   args.Message,
			}, nil)
			if err == nil {
				sent++
			}
		}
		sendToolResult(id, fmt.Sprintf("Dispatched to %d workers", sent))
		return
	}

	for _, w := range workers {
		if w.Name == args.Target || w.WorkerID == args.Target {
			if w.BlockID == "" {
				sendError(id, -32000, fmt.Sprintf("worker %s has no terminal block", w.Name))
				return
			}
			err := wshclient.TeamSendPromptCommand(RpcClient, wshrpc.TeamSendPromptData{
				WorkerID: w.WorkerID,
				Prompt:   args.Message,
			}, nil)
			if err != nil {
				sendError(id, -32000, fmt.Sprintf("send to %s: %v", w.Name, err))
				return
			}
			sendToolResult(id, fmt.Sprintf("Dispatched to %s", w.Name))
			return
		}
	}

	sendError(id, -32000, fmt.Sprintf("worker not found: %s", args.Target))
}

func handleTeamGetStatus(id interface{}) {
	status, err := wshclient.TeamGetStatusCommand(RpcClient, nil)
	if err != nil {
		sendError(id, -32000, err.Error())
		return
	}
	sendToolResult(id, fmt.Sprintf("Workers: %d active, %d idle, %d offline | Tasks: %d pending, %d working, %d done, %d failed",
		status.ActiveWorkers, status.IdleWorkers, status.OfflineWorkers,
		status.PendingTasks, status.WorkingTasks, status.DoneTasks, status.FailedTasks))
}

func handleTeamListWorkers(id interface{}) {
	workers, err := wshclient.TeamListWorkersCommand(RpcClient, "", nil)
	if err != nil {
		sendError(id, -32000, err.Error())
		return
	}
	result := ""
	for _, w := range workers {
		result += fmt.Sprintf("%-20s %-10s %s\n", w.Name, w.Status, w.WorkerID)
	}
	sendToolResult(id, result)
}

func sendResult(id interface{}, result interface{}) {
	resp := jsonRPCResponse{JsonRPC: "2.0", ID: id, Result: result}
	writeJSON(resp)
}

func sendToolResult(id interface{}, text string) {
	sendResult(id, map[string]any{
		"content": []mcpContentItem{{Type: "text", Text: text}},
	})
}

func sendError(id interface{}, code int64, message string) {
	resp := jsonRPCResponse{JsonRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
	writeJSON(resp)
}

func writeJSON(v interface{}) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(append(data, '\n'))
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
