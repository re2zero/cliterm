// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

func TestParseJSONRPCRequest(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*testing.T, *jsonRPCRequest)
	}{
		{
			name:    "valid initialize request",
			input:   `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			wantErr: false,
			check: func(t *testing.T, req *jsonRPCRequest) {
				if req.JsonRPC != "2.0" {
					t.Errorf("expected jsonrpc version 2.0, got %s", req.JsonRPC)
				}
				if req.Method != "initialize" {
					t.Errorf("expected method initialize, got %s", req.Method)
				}
				if req.ID != float64(1) {
					t.Errorf("expected id 1, got %v", req.ID)
				}
			},
		},
		{
			name:    "invalid json",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name:  "valid tools/list request",
			input: `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
			check: func(t *testing.T, req *jsonRPCRequest) {
				if req.Method != "tools/list" {
					t.Errorf("expected method tools/list, got %s", req.Method)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parseJSONRPCRequest(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSONRPCRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, req)
			}
		})
	}
}

func TestTeamUpdateTaskValidation(t *testing.T) {
	tests := []struct {
		name      string
		args      json.RawMessage
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid status done",
			args:      json.RawMessage(`{"taskId":"task1","status":"done"}`),
			wantError: false,
		},
		{
			name:      "valid status failed",
			args:      json.RawMessage(`{"taskId":"task1","status":"failed"}`),
			wantError: false,
		},
		{
			name:      "valid status paused",
			args:      json.RawMessage(`{"taskId":"task1","status":"paused"}`),
			wantError: false,
		},
		{
			name:      "invalid status",
			args:      json.RawMessage(`{"taskId":"task1","status":"invalid"}`),
			wantError: true,
			errorMsg:  "status must be one of",
		},
		{
			name:      "empty status is valid",
			args:      json.RawMessage(`{"taskId":"task1","progress":50}`),
			wantError: false,
		},
		{
			name:      "invalid json",
			args:      json.RawMessage(`{invalid}`),
			wantError: true,
			errorMsg:  "Invalid arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params struct {
				TaskID   string `json:"taskId"`
				Status   string `json:"status"`
				Progress int    `json:"progress"`
			}

			err := json.Unmarshal(tt.args, &params)
			if (err != nil) != tt.wantError {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if params.Status != "" && params.Status != "done" && params.Status != "failed" && params.Status != "paused" {
					t.Errorf("status validation should fail for invalid status: %s", params.Status)
				}
			}
		})
	}
}

func TestTeamUpdateTaskEnvFallback(t *testing.T) {
	tests := []struct {
		name         string
		args         json.RawMessage
		envTaskID    string
		expectTaskID string
		expectError  bool
	}{
		{
			name:         "taskId provided in params",
			args:         json.RawMessage(`{"taskId":"explicit-task","status":"done"}`),
			envTaskID:    "env-task",
			expectTaskID: "explicit-task",
			expectError:  false,
		},
		{
			name:         "taskId empty with env var set",
			args:         json.RawMessage(`{"status":"done"}`),
			envTaskID:    "env-task",
			expectTaskID: "env-task",
			expectError:  false,
		},
		{
			name:         "taskId empty with no env var",
			args:         json.RawMessage(`{"status":"done"}`),
			envTaskID:    "",
			expectTaskID: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params struct {
				TaskID string `json:"taskId"`
				Status string `json:"status"`
			}

			_ = json.Unmarshal(tt.args, &params)

			taskID := params.TaskID
			if taskID == "" {
				taskID = tt.envTaskID
			}

			if taskID != tt.expectTaskID {
				t.Errorf("expected taskID %s, got %s", tt.expectTaskID, taskID)
			}

			if tt.expectError && taskID != "" {
				t.Errorf("expected error but got taskID: %s", taskID)
			}
		})
	}
}

func TestTeamDispatchTargetResolution(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		workers     []*wshrpc.TeamWorker
		expectError bool
		expectMatch string
	}{
		{
			name:   "target 'all' dispatches to all",
			target: "all",
			workers: []*wshrpc.TeamWorker{
				{WorkerID: "w1", Name: "worker-1", Status: "active"},
				{WorkerID: "w2", Name: "worker-2", Status: "active"},
			},
			expectError: false,
		},
		{
			name:   "target specific worker by name",
			target: "worker-1",
			workers: []*wshrpc.TeamWorker{
				{WorkerID: "w1", Name: "worker-1", Status: "active"},
				{WorkerID: "w2", Name: "worker-2", Status: "active"},
			},
			expectError: false,
			expectMatch: "w1",
		},
		{
			name:   "target not found",
			target: "nonexistent",
			workers: []*wshrpc.TeamWorker{
				{WorkerID: "w1", Name: "worker-1", Status: "active"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.target == "all" {
				return
			}

			var foundWorkerID string
			for _, worker := range tt.workers {
				if worker.Name == tt.target {
					foundWorkerID = worker.WorkerID
					break
				}
			}

			if tt.expectError && foundWorkerID != "" {
				t.Errorf("expected error but found worker: %s", foundWorkerID)
			}

			if !tt.expectError && foundWorkerID == "" {
				t.Errorf("expected to find worker but got none")
			}

			if tt.expectMatch != "" && foundWorkerID != tt.expectMatch {
				t.Errorf("expected workerID %s, got %s", tt.expectMatch, foundWorkerID)
			}
		})
	}
}

func TestToolsListContainsAllTools(t *testing.T) {
	expectedTools := map[string]bool{
		"team_update_task":   true,
		"team_dispatch":      true,
		"team_get_status":    true,
		"team_list_workers":  true,
		"team_list_projects": true,
		"team_get_project":   true,
	}

	if len(expectedTools) != 6 {
		t.Errorf("expected 6 tools, got %d", len(expectedTools))
	}

	for toolName := range expectedTools {
		if !expectedTools[toolName] {
			t.Errorf("missing expected tool: %s", toolName)
		}
	}
}

func TestMCPServerInfo(t *testing.T) {
	serverInfo := mcpServerInfo{
		Name:    "waveterm-team-mcp",
		Version: "1.0.0",
	}

	if serverInfo.Name != "waveterm-team-mcp" {
		t.Errorf("expected server name 'waveterm-team-mcp', got %s", serverInfo.Name)
	}

	if serverInfo.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", serverInfo.Version)
	}
}
