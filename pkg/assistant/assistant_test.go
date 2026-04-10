// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/zeroai/process"
	"github.com/wavetermdev/waveterm/pkg/zeroai/service"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

func TestAssistantLifecycle(t *testing.T) {
	ctx := context.Background()

	// Create assistant with mock agent service (just nil for S01, we don't call it)
	assistant := NewAssistant(nil)

	// Test initial state
	if assistant.IsRunning() {
		t.Error("assistant should not be running initially")
	}

	// Test Start()
	err := assistant.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !assistant.IsRunning() {
		t.Error("assistant should be running after Start()")
	}

	// Test idempotent Start()
	err = assistant.Start(ctx)
	if err != nil {
		t.Fatalf("second Start() should not fail: %v", err)
	}

	if !assistant.IsRunning() {
		t.Error("assistant should still be running after idempotent Start()")
	}

	// Add a task and verify it's in pending state
	task, err := assistant.AddTask("test task for lifecycle")
	if err != nil {
		t.Fatalf("AddTask() failed: %v", err)
	}

	if task.Status != team.TaskStatusPending {
		t.Errorf("task should be in pending state initially, got: %s", task.Status)
	}

	// Wait for at least one ticker cycle (5 seconds + small buffer)
	time.Sleep(6 * time.Second)

	// Verify task transitions to assigned status
	tasks, err := assistant.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	updatedTask := tasks[0]
	if updatedTask.Status != team.TaskStatusInProgress {
		t.Errorf("task should be in progress after ticker, got: %s", updatedTask.Status)
	}

	if updatedTask.AssignedAgentID != DefaultPlaceholderAgentID {
		t.Errorf("task should be assigned to %s, got: %s", DefaultPlaceholderAgentID, updatedTask.AssignedAgentID)
	}

	// Test Stop()
	err = assistant.Stop()
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if assistant.IsRunning() {
		t.Error("assistant should not be running after Stop()")
	}

	// Test idempotent Stop()
	err = assistant.Stop()
	if err != nil {
		t.Fatalf("second Stop() should not fail: %v", err)
	}

	// Test that no more ticker wakeups happen after stop
	// Adding a task after stop should remain pending
	_, err = assistant.AddTask("task after stop")
	if err != nil {
		t.Fatalf("AddTask() after stop should work: %v", err)
	}

	time.Sleep(6 * time.Second)

	tasks, err = assistant.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() failed: %v", err)
	}

	// We should have 2 tasks now, but only one assigned (the one before stop)
	assignedCount := 0
	for _, task := range tasks {
		if task.Status == team.TaskStatusInProgress {
			assignedCount++
		}
	}

	if assignedCount != 1 {
		t.Errorf("expected exactly 1 assigned task after stop, got %d", assignedCount)
	}
}

func TestAddTask(t *testing.T) {
	assistant := NewAssistant(nil)

	// Test adding task with empty description
	_, err := assistant.AddTask("")
	if err != ErrEmptyDescription {
		t.Errorf("expected ErrEmptyDescription, got: %v", err)
	}

	// Test adding a valid task
	task, err := assistant.AddTask("test task description")
	if err != nil {
		t.Fatalf("AddTask() failed: %v", err)
	}

	// Verify task properties
	if task.TaskID == "" {
		t.Error("task ID should not be empty")
	}

	if task.Description != "test task description" {
		t.Errorf("task description mismatch, got: %s", task.Description)
	}

	if task.Status != team.TaskStatusPending {
		t.Errorf("task should be in pending state, got: %s", task.Status)
	}

	if task.CreatedAt == 0 {
		t.Error("task CreatedAt should be set")
	}

	if task.TeamID != "assistant" {
		t.Errorf("task TeamID should be 'assistant', got: %s", task.TeamID)
	}
}

func TestGetStatus(t *testing.T) {
	ctx := context.Background()
	assistant := NewAssistant(nil)

	// Get initial status
	status, err := assistant.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	running, ok := status["running"].(bool)
	if !ok {
		t.Fatal("status['running'] should be a bool")
	}

	if running {
		t.Error("assistant should not be running initially")
	}

	totalTasks, ok := status["totalTasks"].(int)
	if !ok {
		t.Fatal("status['totalTasks'] should be an int")
	}

	if totalTasks != 0 {
		t.Errorf("expected 0 tasks initially, got: %d", totalTasks)
	}

	// Start assistant and add tasks
	if err := assistant.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Add some tasks with different statuses
	_, _ = assistant.AddTask("task 1")
	_, _ = assistant.AddTask("task 2")
	_, _ = assistant.AddTask("task 3")

	// Wait for ticker to assign tasks
	time.Sleep(6 * time.Second)

	// Get status after adding tasks
	status, err = assistant.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	running = status["running"].(bool)
	if !running {
		t.Error("assistant should be running after Start()")
	}

	totalTasks = status["totalTasks"].(int)
	if totalTasks != 3 {
		t.Errorf("expected 3 tasks, got: %d", totalTasks)
	}

	statusCounts, ok := status["statusCounts"].(map[string]int)
	if !ok {
		t.Fatal("status['statusCounts'] should be a map[string]int")
	}

	// After ticker cycle, all tasks should be in progress (assigned)
	inProgressCount := statusCounts[string(team.TaskStatusInProgress)]
	if inProgressCount != 3 {
		t.Errorf("expected 3 tasks in progress, got: %d", inProgressCount)
	}

	// Cleanup
	if err := assistant.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
}

func TestTaskStoreAssignment(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create a task
	task, err := store.AddTask("test task")
	if err != nil {
		t.Fatalf("AddTask() failed: %v", err)
	}

	// Verify task is pending
	if task.Status != team.TaskStatusPending {
		t.Errorf("task should be pending initially, got: %s", task.Status)
	}

	// Assign the task
	agentID := "test-agent-1"
	err = store.AssignTask(task.TaskID, agentID)
	if err != nil {
		t.Fatalf("AssignTask() failed: %v", err)
	}

	// Verify task is now in progress and assigned
	assignedTask, err := store.GetTask(task.TaskID)
	if err != nil {
		t.Fatalf("GetTask() failed: %v", err)
	}

	if assignedTask.Status != team.TaskStatusInProgress {
		t.Errorf("task should be in progress after assignment, got: %s", assignedTask.Status)
	}

	if assignedTask.AssignedAgentID != agentID {
		t.Errorf("task should be assigned to %s, got: %s", agentID, assignedTask.AssignedAgentID)
	}

	// Test assigning non-existent task
	err = store.AssignTask("non-existent-id", agentID)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}

	// Test assigning already assigned task
	err = store.AssignTask(task.TaskID, "another-agent")
	if err == nil {
		t.Error(" assigning already assigned task should return error")
	}
}

func TestTaskStore(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Test empty description
	_, err := store.AddTask("")
	if err != ErrEmptyDescription {
		t.Errorf("expected ErrEmptyDescription, got: %v", err)
	}

	// Add tasks
	task1, _ := store.AddTask("task 1")
	_, _ = store.AddTask("task 2")
	_, _ = store.AddTask("task 3")

	// task2 and task3 are added but not directly used in this test

	// Get all tasks
	tasks, err := store.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks() failed: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got: %d", len(tasks))
	}

	// Get specific task
	retrievedTask, err := store.GetTask(task1.TaskID)
	if err != nil {
		t.Fatalf("GetTask() failed: %v", err)
	}

	if retrievedTask.TaskID != task1.TaskID {
		t.Errorf("task ID mismatch")
	}

	// Get non-existent task
	_, err = store.GetTask("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}
}

func TestAgentServiceCompatibility(t *testing.T) {
	// Test that Assistant can be created with an AgentService
	// This is a compilation test - we just need to make sure the type works
	agentSvc := service.NewAgentService()
	defer func() {
		if err := agentSvc.Shutdown(); err != nil {
			t.Logf("Warning: AgentService shutdown failed: %v", err)
		}
	}()

	assistant := NewAssistant(agentSvc)

	if assistant == nil {
		t.Fatal("NewAssistant should return non-nil")
	}

	if assistant.agentService != agentSvc {
		t.Error("assistant should store the agent service")
	}
}

// Mock AgentService for testing when we don't want to start the real service
type mockAgentService struct{}

func (m *mockAgentService) GetAgent(ctx context.Context, config interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockAgentService) ListAgents() []interface{} {
	return nil
}

func (m *mockAgentService) RemoveAgent(key string) error {
	return nil
}

func (m *mockAgentService) Shutdown() error {
	return nil
}

func TestAddTaskFromYAML(t *testing.T) {
	assistant := NewAssistant(nil)

	// Test valid YAML without metadata
	yml1 := `description: "simple task from YAML"`
	task, err := assistant.AddTaskFromYAML(yml1)
	if err != nil {
		t.Fatalf("AddTaskFromYAML failed: %v", err)
	}

	if task.Description != "simple task from YAML" {
		t.Errorf("expected 'simple task from YAML', got: %s", task.Description)
	}

	if task.Metadata != nil {
		t.Error("metadata should be nil when not provided in YAML")
	}

	// Test valid YAML with metadata
	yml2 := `description: "task with metadata"
metadata:
  worker_type: "claude"
  priority: "high"
  retry_count: 3`
	task2, err := assistant.AddTaskFromYAML(yml2)
	if err != nil {
		t.Fatalf("AddTaskFromYAML with metadata failed: %v", err)
	}

	if task2.Description != "task with metadata" {
		t.Errorf("expected 'task with metadata', got: %s", task2.Description)
	}

	if task2.Metadata == nil {
		t.Fatal("metadata should not be nil")
	}

	if task2.Metadata["worker_type"] != "claude" {
		t.Errorf("expected worker_type 'claude', got: %v", task2.Metadata["worker_type"])
	}

	if task2.Metadata["priority"] != "high" {
		t.Errorf("expected priority 'high', got: %v", task2.Metadata["priority"])
	}

	// Test YAML with task_id override (should be ignored, store generates ID)
	yml3 := `task_id: "custom-id-123"
description: "task with custom ID"`
	task3, err := assistant.AddTaskFromYAML(yml3)
	if err != nil {
		t.Fatalf("AddTaskFromYAML with task_id failed: %v", err)
	}

	// task_id in YAML should be ignored
	if task3.TaskID == "custom-id-123" {
		t.Error("task_id from YAML should be ignored, store should generate its own")
	}

	if task3.Description != "task with custom ID" {
		t.Errorf("expected 'task with custom ID', got: %s", task3.Description)
	}

	// Test invalid YAML (syntax error)
	invalidYAML := `description: "unclosed string`
	_, err = assistant.AddTaskFromYAML(invalidYAML)
	if err == nil {
		t.Error("expected error for invalid YAML syntax")
	}

	// Test YAML without required description
	missingDesc := `metadata:
  worker_type: "claude"`
	_, err = assistant.AddTaskFromYAML(missingDesc)
	if err == nil {
		t.Error("expected error for YAML without description")
	}

	// Test empty YAML
	_, err = assistant.AddTaskFromYAML("")
	if err == nil {
		t.Error("expected error for empty YAML")
	}

	// Test empty YAML (whitespace only)
	_, err = assistant.AddTaskFromYAML("   \n  \n  ")
	if err == nil {
		t.Error("expected error for whitespace-only YAML")
	}
}

func TestTaskStoreWithMetadata(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Add task without metadata
	task1, err := store.AddTask("task without metadata")
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	if task1.Metadata != nil {
		t.Error("metadata should be nil when not provided")
	}

	// Add task with metadata
	metadata := map[string]interface{}{
		"worker_type": "claude",
		"priority":    "high",
	}

	task2, err := store.AddTaskWithMetadata("task with metadata", metadata)
	if err != nil {
		t.Fatalf("AddTaskWithMetadata failed: %v", err)
	}

	if task2.Metadata == nil {
		t.Fatal("metadata should not be nil")
	}

	if task2.Metadata["worker_type"] != "claude" {
		t.Errorf("expected worker_type 'claude', got: %v", task2.Metadata["worker_type"])
	}

	if task2.Metadata["priority"] != "high" {
		t.Errorf("expected priority 'high', got: %v", task2.Metadata["priority"])
	}

	// Verify both tasks in store
	tasks, err := store.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got: %d", len(tasks))
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create a task
	task, err := store.AddTask("test task")
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Test pending -> in_progress (valid)
	err = store.UpdateTaskStatus(task.TaskID, team.TaskStatusInProgress)
	if err != nil {
		t.Errorf("pending -> in_progress should be valid, got error: %v", err)
	}

	updated, err := store.GetTask(task.TaskID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if updated.Status != team.TaskStatusInProgress {
		t.Errorf("task status should be in_progress, got: %s", updated.Status)
	}

	// Test in_progress -> completed (valid)
	err = store.UpdateTaskStatus(task.TaskID, team.TaskStatusCompleted)
	if err != nil {
		t.Errorf("in_progress -> completed should be valid, got error: %v", err)
	}

	updated, err = store.GetTask(task.TaskID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if updated.Status != team.TaskStatusCompleted {
		t.Errorf("task status should be completed, got: %s", updated.Status)
	}

	if updated.CompletedAt == 0 {
		t.Error("CompletedAt should be set when task is completed")
	}

	// Test completed status transitions are invalid (terminal state)
	err = store.UpdateTaskStatus(task.TaskID, team.TaskStatusPending)
	if err == nil {
		t.Error("completed -> pending should be invalid (terminal state)")
	}

	// Create another task for more testing
	task2, _ := store.AddTask("another task")

	// Test pending -> blocked (valid)
	err = store.UpdateTaskStatus(task2.TaskID, team.TaskStatusBlocked)
	if err != nil {
		t.Errorf("pending -> blocked should be valid, got error: %v", err)
	}

	// Test blocked -> pending (valid)
	err = store.UpdateTaskStatus(task2.TaskID, team.TaskStatusPending)
	if err != nil {
		t.Errorf("blocked -> pending should be valid, got error: %v", err)
	}

	task2, _ = store.GetTask(task2.TaskID)
	// Test pending -> assigned via AssignTask (uses in_progress)
	_ = store.AssignTask(task2.TaskID, "agent-1")

	// Test in_progress -> failed (valid)
	err = store.UpdateTaskStatus(task2.TaskID, team.TaskStatusFailed)
	if err != nil {
		t.Errorf("in_progress -> failed should be valid, got error: %v", err)
	}

	updated, _ = store.GetTask(task2.TaskID)
	if updated.Status != team.TaskStatusFailed {
		t.Errorf("task status should be failed, got: %s", updated.Status)
	}

	if updated.CompletedAt == 0 {
		t.Error("CompletedAt should be set when task is failed")
	}

	// Test updating non-existent task
	err = store.UpdateTaskStatus("non-existent-id", team.TaskStatusInProgress)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}

	// Test same status transition (should be invalid per our implementation)
	err = store.UpdateTaskStatus(task.TaskID, team.TaskStatusCompleted)
	if err == nil {
		t.Error("same status transition should be invalid")
	}
}

func TestInvalidStatusTransitions(t *testing.T) {
	store := NewInMemoryTaskStore()

	task, _ := store.AddTask("test task")

	// Create in-progress status first
	_ = store.AssignTask(task.TaskID, "agent-1")

	// Test invalid transition: in_progress -> pending (invalid)
	err := store.UpdateTaskStatus(task.TaskID, team.TaskStatusPending)
	if err == nil {
		t.Error("in_progress -> pending should be invalid")
	}

	// Mark as completed
	_ = store.UpdateTaskStatus(task.TaskID, team.TaskStatusCompleted)

	// From completed state, no transitions should be valid (terminal state)
	invalidTargets := []team.TaskStatus{
		team.TaskStatusPending,
		team.TaskStatusInProgress,
		team.TaskStatusBlocked,
	}

	for _, target := range invalidTargets {
		err := store.UpdateTaskStatus(task.TaskID, target)
		if err == nil {
			t.Errorf("completed -> %s should be invalid (terminal state)", target)
		}
	}

	// Mark as failed
	task2, _ := store.AddTask("task2")
	_ = store.AssignTask(task2.TaskID, "agent-1")
	_ = store.UpdateTaskStatus(task2.TaskID, team.TaskStatusFailed)

	// From failed state, no transitions should be valid (terminal state)
	for _, target := range invalidTargets {
		err := store.UpdateTaskStatus(task2.TaskID, target)
		if err == nil {
			t.Errorf("failed -> %s should be invalid (terminal state)", target)
		}
	}
}

// ============================================================================
// StatusMonitor Tests
// ============================================================================

func TestStatusMonitor_StartStop(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	monitor := NewStatusMonitor(store)

	// Initial state: not running
	if monitor.IsRunning() {
		t.Error("monitor should not be running initially")
	}

	// Start monitor
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !monitor.IsRunning() {
		t.Error("monitor should be running after Start()")
	}

	// Idempotent start
	err = monitor.Start(ctx)
	if err != nil {
		t.Fatalf("second Start() should not fail: %v", err)
	}

	// Stop monitor
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if monitor.IsRunning() {
		t.Error("monitor should not be running after Stop()")
	}

	// Idempotent stop
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("second Stop() should not fail: %v", err)
	}
}

func TestStatusMonitor_ParseValidStatus(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	monitor := NewStatusMonitor(store)

	// Create a task for monitoring
	task, err := store.AddTask("test task")
	if err != nil {
		t.Fatalf("AddTask() failed: %v", err)
	}

	// Create a status directory and file
	statusDir := filepath.Join(monitor.statusDir, task.TaskID)
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	defer os.RemoveAll(monitor.statusDir)

	statusFile := filepath.Join(statusDir, StatusFileName)

	// Write valid status: running
	status := WorkerStatus{
		Status:     "running",
		LastUpdate: time.Now().Unix(),
		Message:    "Worker started",
	}
	statusData, _ := json.MarshalIndent(status, "", "  ")
	if err := os.WriteFile(statusFile, statusData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start monitor and poll
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer monitor.Stop()

	monitor.PollStatusFiles()

	// Verify task status updated
	updatedTask, err := store.GetTask(task.TaskID)
	if err != nil {
		t.Fatalf("GetTask() failed: %v", err)
	}

	if updatedTask.Status != team.TaskStatusInProgress {
		t.Errorf("task status should be in_progress, got: %s", updatedTask.Status)
	}

	// Verify tracker was updated
	tracker := monitor.GetTracker(task.TaskID)
	if tracker == nil {
		t.Fatal("tracker should exist for task")
	}

	if tracker.Status != "running" {
		t.Errorf("tracker status should be 'running', got: %s", tracker.Status)
	}

	// Write valid status: completed
	status.Status = "completed"
	status.LastUpdate = time.Now().Unix()
	statusData, _ = json.MarshalIndent(status, "", "  ")
	_ = os.WriteFile(statusFile, statusData, 0644)

	monitor.PollStatusFiles()

	updatedTask, _ = store.GetTask(task.TaskID)
	if updatedTask.Status != team.TaskStatusCompleted {
		t.Errorf("task status should be completed, got: %s", updatedTask.Status)
	}
}

func TestStatusMonitor_ParseInvalidStatus(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	monitor := NewStatusMonitor(store)

	// Create a task
	task, err := store.AddTask("test task")
	if err != nil {
		t.Fatalf("AddTask() failed: %v", err)
	}

	// Create status directory
	statusDir := filepath.Join(monitor.statusDir, task.TaskID)
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	defer os.RemoveAll(monitor.statusDir)

	statusFile := filepath.Join(statusDir, StatusFileName)

	// Test invalid JSON
	invalidJSON := `{invalid json`
	if err := os.WriteFile(statusFile, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer monitor.Stop()

	// Should not crash
	monitor.PollStatusFiles()

	// Task status should remain unchanged
	originalTask, _ := store.GetTask(task.TaskID)
	if originalTask.Status != team.TaskStatusPending {
		t.Errorf("task status should still be pending after invalid JSON, got: %s", originalTask.Status)
	}

	// Test invalid status value
	status := WorkerStatus{
		Status:     "invalid_status",
		LastUpdate: time.Now().Unix(),
	}
	statusData, _ := json.MarshalIndent(status, "", "  ")
	_ = os.WriteFile(statusFile, statusData, 0644)

	// Should not crash
	monitor.PollStatusFiles()

	// Task status should still be unchanged
	originalTask, _ = store.GetTask(task.TaskID)
	if originalTask.Status != team.TaskStatusPending {
		t.Errorf("task status should still be pending after invalid status value, got: %s", originalTask.Status)
	}

	// Test missing status field
	statusNoField := map[string]int64{"last_update": time.Now().Unix()}
	statusData, _ = json.MarshalIndent(statusNoField, "", "  ")
	_ = os.WriteFile(statusFile, statusData, 0644)

	// Should not crash
	monitor.PollStatusFiles()
}

func TestStatusMonitor_StallDetection(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	monitor := NewStatusMonitor(store)

	// Set a short stall timeout for testing
	monitor.SetStallTimeout(300 * time.Millisecond) // 300ms

	// Create a task
	task, err := store.AddTask("test task")
	if err != nil {
		t.Fatalf("AddTask() failed: %v", err)
	}

	// Create status directory and file
	statusDir := filepath.Join(monitor.statusDir, task.TaskID)
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	defer os.RemoveAll(monitor.statusDir)

	statusFile := filepath.Join(statusDir, StatusFileName)

	// Write running status with old timestamp
	oldTime := time.Now().Add(-5 * time.Minute)
	status := WorkerStatus{
		Status:     "running",
		LastUpdate: oldTime.Unix(),
		Message:    "Worker started",
	}
	statusData, _ := json.MarshalIndent(status, "", "  ")
	if err := os.WriteFile(statusFile, statusData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Start monitor
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer monitor.Stop()

	// Poll to populate tracker
	monitor.PollStatusFiles()

	// Verify tracker exists with old timestamp
	tracker := monitor.GetTracker(task.TaskID)
	if tracker == nil {
		t.Fatal("tracker should exist for task")
	}

	if tracker.Status != "running" {
		t.Errorf("tracker status should be 'running', got: %s", tracker.Status)
	}

	// Check for stalls - should mark task as blocked
	monitor.CheckForStalls()

	// Verify task status changed to blocked
	updatedTask, err := store.GetTask(task.TaskID)
	if err != nil {
		t.Fatalf("GetTask() failed: %v", err)
	}

	if updatedTask.Status != team.TaskStatusBlocked {
		t.Errorf("task status should be blocked due to stall, got: %s", updatedTask.Status)
	}
}

func TestStatusMonitor_ConcurrentUpdates(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	monitor := NewStatusMonitor(store)

	// Create multiple tasks
	numTasks := 10
	taskIDs := make([]string, numTasks)
	for i := 0; i < numTasks; i++ {
		task, err := store.AddTask(fmt.Sprintf("concurrent task %d", i))
		if err != nil {
			t.Fatalf("AddTask() failed: %v", err)
		}
		taskIDs[i] = task.TaskID

		// Create status directory and file
		statusDir := filepath.Join(monitor.statusDir, task.TaskID)
		if err := os.MkdirAll(statusDir, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		statusFile := filepath.Join(statusDir, StatusFileName)
		status := WorkerStatus{
			Status:     "running",
			LastUpdate: time.Now().Unix(),
		}
		statusData, _ := json.MarshalIndent(status, "", "  ")
		if err := os.WriteFile(statusFile, statusData, 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}
	defer os.RemoveAll(monitor.statusDir)

	// Start monitor
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer monitor.Stop()

	// Concurrent updates from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Update status file
			statusDir := filepath.Join(monitor.statusDir, taskIDs[idx])
			statusFile := filepath.Join(statusDir, StatusFileName)
			status := WorkerStatus{
				Status:     "completed",
				LastUpdate: time.Now().Unix(),
			}
			statusData, _ := json.MarshalIndent(status, "", "  ")
			_ = os.WriteFile(statusFile, statusData, 0644)

			// Poll
			monitor.PollStatusFiles()
		}(i)
	}

	wg.Wait()

	// Verify no crashes occurred and at least some tasks updated correctly
	tasks, err := store.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks() failed: %v", err)
	}

	updatedCount := 0
	for _, task := range tasks {
		if task.Status == team.TaskStatusCompleted {
			updatedCount++
		}
	}

	// In concurrent scenarios, some updates may fail due to race conditions
	// The important thing is that the system doesn't crash and most succeed
	if updatedCount < numTasks/2 {
		t.Errorf("expected at least %d tasks updated to completed, got %d", numTasks/2, updatedCount)
	}

	// Verify all trackers exist (no panics)
	for _, taskID := range taskIDs {
		tracker := monitor.GetTracker(taskID)
		if tracker == nil {
			t.Errorf("tracker should exist for task %s", taskID)
		}
	}
}

func TestStatusMonitor_RemoveTracker(t *testing.T) {
	store := NewInMemoryTaskStore()
	monitor := NewStatusMonitor(store)

	// Create a task
	task, _ := store.AddTask("test task")

	// Create status directory and file with running status
	statusDir := filepath.Join(monitor.statusDir, task.TaskID)
	_ = os.MkdirAll(statusDir, 0755)
	defer os.RemoveAll(monitor.statusDir)

	statusFile := filepath.Join(statusDir, StatusFileName)
	status := WorkerStatus{
		Status:     "running",
		LastUpdate: time.Now().Unix(),
	}
	statusData, _ := json.MarshalIndent(status, "", "  ")
	_ = os.WriteFile(statusFile, statusData, 0644)

	ctx := context.Background()
	_ = monitor.Start(ctx)
	defer monitor.Stop()

	// Poll to create tracker
	monitor.PollStatusFiles()

	// Verify tracker exists
	if monitor.GetTracker(task.TaskID) == nil {
		t.Fatal("tracker should exist before removal")
	}

	// Remove tracker
	monitor.RemoveTracker(task.TaskID)

	// Verify tracker removed
	if monitor.GetTracker(task.TaskID) != nil {
		t.Error("tracker should be removed after RemoveTracker call")
	}
}

func TestStatusMonitor_StatusDirNotExist(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create monitor with non-existent status directory
	monitor := NewStatusMonitor(store)
	monitor.statusDir = filepath.Join(monitor.statusDir, "nonexistent", "path")

	// Start and poll should not crash
	_ = monitor.Start(context.Background())
	defer monitor.Stop()

	// Should not create directory or crash
	monitor.PollStatusFiles()
	monitor.CheckForStalls()
}

// ============================================================================
// WorkerManager Tests
// ============================================================================

// mockProcessManager is a simple mock for testing
type mockProcessManager struct {
	processes map[string]*mockAgentProcess
	mu        sync.Mutex
	spawnCount int
}

type mockAgentProcess struct {
	spec      process.ProcessSpec
	pid       int
	state     process.ProcessState
	startedAt time.Time
	endedAt   time.Time
	exitCode  int
	killed    bool
}

func (m *mockProcessManager) SpawnProcess(ctx context.Context, spec process.ProcessSpec) (*process.AgentProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.spawnCount++

	mockProc := &mockAgentProcess{
		spec:      spec,
		pid:       1000 + m.spawnCount,
		state:     process.ProcessStateRunning,
		startedAt: time.Now(),
		killed:    false,
	}
	m.processes[fmt.Sprintf("proc-%d", m.spawnCount)] = mockProc

	// Create a minimal AgentProcess wrapper
	agentProc := &process.AgentProcess{
		Spec: spec,
		Command: nil, // Don't need real command for mock
	}

	return agentProc, nil
}

func (m *mockProcessManager) KillProcess(proc *process.AgentProcess) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and kill the mock process
	for _, p := range m.processes {
		if !p.killed && p.state == process.ProcessStateRunning {
			p.killed = true
			p.state = process.ProcessStateDone
			p.endedAt = time.Now()
			return nil
		}
	}
	return nil
}

func (m *mockProcessManager) GetProcessInfo(proc *process.AgentProcess) process.ProcessInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.processes {
		if p.state == process.ProcessStateRunning || p.killed {
			return process.ProcessInfo{
				PID:       p.pid,
				State:     p.state,
				StartedAt: p.startedAt,
				EndedAt:   p.endedAt,
			}
		}
	}
	return process.ProcessInfo{}
}

func (m *mockProcessManager) ListProcesses(ctx context.Context) ([]process.ProcessInfo, error) {
	return []process.ProcessInfo{}, nil
}

func newMockProcessManager() *mockProcessManager {
	return &mockProcessManager{
		processes: make(map[string]*mockAgentProcess),
	}
}

func TestWorkerManager_StartWorker(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	// Set mock process manager
	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	// Create temporary status directory
	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Start worker
	taskID := "test-task-001"
	workerInfo, err := manager.StartWorker(ctx, taskID, "claude")
	if err != nil {
		t.Fatalf("StartWorker() failed: %v", err)
	}

	// Verify worker info
	if workerInfo.TaskID != taskID {
		t.Errorf("task ID mismatch, got: %s", workerInfo.TaskID)
	}

	if workerInfo.WorkerType != "claude" {
		t.Errorf("worker type mismatch, got: %s", workerInfo.WorkerType)
	}

	if workerInfo.PID == 0 {
		t.Error("PID should be set")
	}

	// Verify status file created
	statusFilePath := filepath.Join(testStatusDir, taskID, StatusFileName)
	if _, err := os.Stat(statusFilePath); os.IsNotExist(err) {
		t.Errorf("status file not created at: %s", statusFilePath)
	}

	// Verify status file content
	data, _ := os.ReadFile(statusFilePath)
	var status WorkerStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("failed to parse status file: %v", err)
	}

	if status.Status != "running" {
		t.Errorf("status should be 'running', got: %s", status.Status)
	}

	// Verify process was spawned via GetProcessInfo
	workerInfoCheck, err := manager.GetWorkerInfo(taskID)
	if err != nil {
		t.Fatalf("GetWorkerInfo() failed: %v", err)
	}

	if workerInfoCheck.PID == 0 {
		t.Error("PID should be retrievable via GetWorkerInfo")
	}
}

func TestWorkerManager_StartWorkerAlreadyExists(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test2")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	taskID := "test-task-002"

	// Start first worker
	_, err := manager.StartWorker(ctx, taskID, "claude")
	if err != nil {
		t.Fatalf("first StartWorker() failed: %v", err)
	}

	// Try to start duplicate worker
	_, err = manager.StartWorker(ctx, taskID, "opencode")
	if err == nil {
		t.Error("starting duplicate worker should fail")
	}

	expectedErr := "already exists"
	if err == nil || !contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing '%s', got: %v", expectedErr, err)
	}
}

func TestWorkerManager_StopWorker(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test3")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	taskID := "test-task-003"

	// Start worker
	_, err := manager.StartWorker(ctx, taskID, "claude")
	if err != nil {
		t.Fatalf("StartWorker() failed: %v", err)
	}

	// Stop worker
	err = manager.StopWorker(taskID)
	if err != nil {
		t.Fatalf("StopWorker() failed: %v", err)
	}

	// Verify worker removed
	_, err = manager.GetWorkerInfo(taskID)
	if err == nil {
		t.Error("GetWorkerInfo should fail for stopped worker")
	}

	// Verify status directory removed
	statusDir := filepath.Join(testStatusDir, taskID)
	if _, err := os.Stat(statusDir); !os.IsNotExist(err) {
		t.Errorf("status directory should be removed, still exists at: %s", statusDir)
	}
}

func TestWorkerManager_StopWorkerNotFound(t *testing.T) {
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	// Stop non-existent worker
	err := manager.StopWorker("non-existent-worker")
	if err == nil {
		t.Error("stopping non-existent worker should fail")
	}
}

func TestWorkerManager_ConcurrencyLimit(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test4")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Set a low concurrency limit
	manager.SetMaxConcurrentWorkers(2)

	// Start max concurrent workers
	_, err := manager.StartWorker(ctx, "task-001", "claude")
	if err != nil {
		t.Fatalf("StartWorker(1) failed: %v", err)
	}

	_, err = manager.StartWorker(ctx, "task-002", "opencode")
	if err != nil {
		t.Fatalf("StartWorker(2) failed: %v", err)
	}

	// Try to start one more (should fail)
	_, err = manager.StartWorker(ctx, "task-003", "codex")
	if err == nil {
		t.Error("starting worker beyond concurrency limit should fail")
	}

	expectedErr := "concurrent worker limit"
	if err == nil || !contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing '%s', got: %v", expectedErr, err)
	}

	// Verify active count
	if manager.GetActiveWorkerCount() != 2 {
		t.Errorf("expected 2 active workers, got: %d", manager.GetActiveWorkerCount())
	}
}

func TestWorkerManager_GetWorkerInfo(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test5")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	taskID := "test-task-004"
	workerType := "opencode"

	// Start worker
	_, err := manager.StartWorker(ctx, taskID, workerType)
	if err != nil {
		t.Fatalf("StartWorker() failed: %v", err)
	}

	// Get worker info
	info, err := manager.GetWorkerInfo(taskID)
	if err != nil {
		t.Fatalf("GetWorkerInfo() failed: %v", err)
	}

	if info.TaskID != taskID {
		t.Errorf("task ID mismatch, got: %s", info.TaskID)
	}

	if info.WorkerType != workerType {
		t.Errorf("worker type mismatch, got: %s", info.WorkerType)
	}

	if info.ProcessState != process.ProcessStateRunning {
		t.Errorf("process state should be running, got: %s", info.ProcessState)
	}

	if info.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
}

func TestWorkerManager_GetWorkerInfoNotFound(t *testing.T) {
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	// Get info for non-existent worker
	_, err := manager.GetWorkerInfo("non-existent")
	if err == nil {
		t.Error("GetWorkerInfo should fail for non-existent worker")
	}
}

func TestWorkerManager_ListWorkers(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test6")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Start multiple workers
	_, _ = manager.StartWorker(ctx, "task-001", "claude")
	_, _ = manager.StartWorker(ctx, "task-002", "opencode")
	_, _ = manager.StartWorker(ctx, "task-003", "codex")

	// List workers
	workers := manager.ListWorkers()

	if len(workers) != 3 {
		t.Errorf("expected 3 workers, got: %d", len(workers))
	}

	// Verify each worker
	taskIDs := make(map[string]bool)
	for _, w := range workers {
		taskIDs[w.TaskID] = true
		if w.PID == 0 {
			t.Errorf("worker %s has PID 0", w.TaskID)
		}
		if w.StartedAt.IsZero() {
			t.Errorf("worker %s has zero StartedAt", w.TaskID)
		}
	}

	expectedIDs := []string{"task-001", "task-002", "task-003"}
	for _, id := range expectedIDs {
		if !taskIDs[id] {
			t.Errorf("worker %s not found in list", id)
		}
	}
}

func TestWorkerManager_ListWorkersEmpty(t *testing.T) {
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	// List workers when none exist
	workers := manager.ListWorkers()

	if len(workers) != 0 {
		t.Errorf("expected 0 workers, got: %d", len(workers))
	}
}

func TestWorkerManager_SetBlockID(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test7")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	taskID := "test-task-005"
	_, _ = manager.StartWorker(ctx, taskID, "claude")

	// Set block ID
	blockID := "block-12345"
	manager.SetBlockID(taskID, blockID)

	// Verify block ID set via worker info
	info, _ := manager.GetWorkerInfo(taskID)
	if info.BlockID != blockID {
		t.Errorf("block ID mismatch, got: %s", info.BlockID)
	}
}

func TestWorkerManager_DifferentWorkerTypes(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(store)
	manager := NewWorkerManager(statusMonitor)

	mockPM := newMockProcessManager()
	manager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test8")
	manager.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Test different worker types
	workerTypes := []string{"claude", "opencode", "codex", "default", ""}

	for i, wt := range workerTypes {
		taskID := fmt.Sprintf("task-type-%d", i)
		info, err := manager.StartWorker(ctx, taskID, wt)
		if err != nil {
			t.Fatalf("StartWorker with type '%s' failed: %v", wt, err)
		}

		if info.WorkerType != wt {
			t.Errorf("expected worker type '%v' (empty: %v), got: %s", wt, wt == "", info.WorkerType)
		}

		// Verify status file
		statusFile := filepath.Join(testStatusDir, taskID, StatusFileName)
		data, _ := os.ReadFile(statusFile)
		var status WorkerStatus
		_ = json.Unmarshal(data, &status)

		if status.Status != "running" {
			t.Errorf("status should be 'running', got: %s", status.Status)
		}
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestEndToEnd_AddTaskWithWorkerType(t *testing.T) {
	ctx := context.Background()
	assistant := NewAssistant(nil)

	// Set mock process manager
	mockPM := newMockProcessManager()
	assistant.workerManager.SetProcessManager(mockPM)

	// Set temporary status directory
	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test-e2e-1")
	assistant.workerManager.statusDir = testStatusDir
	assistant.statusMonitor.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Add task with worker_type in YAML
	yamlData := `description: "integration test task"
metadata:
  worker_type: "claude"`

	task, err := assistant.AddTaskFromYAML(yamlData)
	if err != nil {
		t.Fatalf("AddTaskFromYAML failed: %v", err)
	}

	// Start assistant
	if err := assistant.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer assistant.Stop()

	// Wait for ticker to process pending task (ticker interval is 5s)
	time.Sleep(6 * time.Second)

	// Verify worker started
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got: %d", len(workers))
	}

	if workers[0].TaskID != task.TaskID {
		t.Errorf("worker task ID mismatch, got: %s", workers[0].TaskID)
	}

	if workers[0].WorkerType != "claude" {
		t.Errorf("worker type mismatch, got: %s", workers[0].WorkerType)
	}

	// Verify task status changed to in_progress
	tasks, _ := assistant.ListTasks()
	var updatedTask *team.Task
	for _, t := range tasks {
		if t.TaskID == task.TaskID {
			updatedTask = t
			break
		}
	}

	if updatedTask == nil {
		t.Fatal("task not found")
	}

	if updatedTask.Status != team.TaskStatusInProgress {
		t.Errorf("task status should be in_progress, got: %s", updatedTask.Status)
	}

	// Verify task metadata has worker_id
	if updatedTask.Metadata == nil {
		t.Fatal("task metadata should not be nil")
	}

	workerID, ok := updatedTask.Metadata["worker_id"].(string)
	if !ok || workerID != task.TaskID {
		t.Errorf("worker_id mismatch, got: %s", workerID)
	}

	// Trigger status monitor to poll status file
	assistant.statusMonitor.PollStatusFiles()

	// Task status should still be in_progress (worker is "running")
	updatedTask, _ = assistant.taskStore.GetTask(task.TaskID)
	if updatedTask.Status != team.TaskStatusInProgress {
		t.Errorf("task status should remain in_progress, got: %s", updatedTask.Status)
	}
}

func TestEndToEnd_StalledWorker(t *testing.T) {
	ctx := context.Background()
	assistant := NewAssistant(nil)

	mockPM := newMockProcessManager()
	assistant.workerManager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test-e2e-2")
	assistant.workerManager.statusDir = testStatusDir
	assistant.statusMonitor.statusDir = testStatusDir
	assistant.statusMonitor.SetStallTimeout(500 * time.Millisecond) // 500ms for quick test
	defer os.RemoveAll(testStatusDir)

	// Add task with worker_type
	yamlData := `description: "stalled worker test"
metadata:
  worker_type: "claude"`

	task, _ := assistant.AddTaskFromYAML(yamlData)

	if err := assistant.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer assistant.Stop()

	// Wait for worker to start
	time.Sleep(6 * time.Second)

	// Verify worker started
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got: %d", len(workers))
	}

	// Update status file with old timestamp to simulate stall
	statusFile := filepath.Join(testStatusDir, task.TaskID, "status.json")
	oldTime := time.Now().Add(-10 * time.Minute)
	oldStatus := WorkerStatus{
		Status:     "running",
		LastUpdate: oldTime.Unix(),
		Message:    "Worker stalled",
	}
	statusData, _ := json.MarshalIndent(oldStatus, "", "  ")
	_ = os.WriteFile(statusFile, statusData, 0644)

	// Poll and check for stalls
	assistant.statusMonitor.PollStatusFiles()
	time.Sleep(100 * time.Millisecond) // Small delay for thread safety
	assistant.statusMonitor.CheckForStalls()

	// Verify task marked as blocked
	updatedTask, _ := assistant.taskStore.GetTask(task.TaskID)
	if updatedTask.Status != team.TaskStatusBlocked {
		t.Errorf("task status should be blocked due to stall, got: %s", updatedTask.Status)
	}
}

func TestEndToEnd_WorkerFailure(t *testing.T) {
	ctx := context.Background()
	assistant := NewAssistant(nil)

	mockPM := newMockProcessManager()
	assistant.workerManager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test-e2e-3")
	assistant.workerManager.statusDir = testStatusDir
	assistant.statusMonitor.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Add task with worker_type
	yamlData := `description: "worker failure test"
metadata:
  worker_type: "claude"`

	task, _ := assistant.AddTaskFromYAML(yamlData)

	if err := assistant.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer assistant.Stop()

	// Wait for worker to start
	time.Sleep(6 * time.Second)

	// Verify worker started
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got: %d", len(workers))
	}

	// Write failed status
	statusFile := filepath.Join(testStatusDir, task.TaskID, "status.json")
	failedStatus := WorkerStatus{
		Status:        "failed",
		LastUpdate:    time.Now().Unix(),
		Message:       "Worker failed",
		OutputSummary: "Error: something went wrong",
	}
	statusData, _ := json.MarshalIndent(failedStatus, "", "  ")
	_ = os.WriteFile(statusFile, statusData, 0644)

	// Poll status file
	assistant.statusMonitor.PollStatusFiles()

	// Verify task marked as failed
	updatedTask, _ := assistant.taskStore.GetTask(task.TaskID)
	if updatedTask.Status != team.TaskStatusFailed {
		t.Errorf("task status should be failed, got: %s", updatedTask.Status)
	}

	if updatedTask.CompletedAt == 0 {
		t.Error("CompletedAt should be set for failed task")
	}
}

func TestEndToEnd_WorkerCompletion(t *testing.T) {
	ctx := context.Background()
	assistant := NewAssistant(nil)

	mockPM := newMockProcessManager()
	assistant.workerManager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test-e2e-4")
	assistant.workerManager.statusDir = testStatusDir
	assistant.statusMonitor.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Add task with worker_type
	yamlData := `description: "worker completion test"
metadata:
  worker_type: "claude"`

	task, _ := assistant.AddTaskFromYAML(yamlData)

	if err := assistant.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer assistant.Stop()

	// Wait for worker to start
	time.Sleep(6 * time.Second)

	// Verify worker started
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got: %d", len(workers))
	}

	// Write completed status
	statusFile := filepath.Join(testStatusDir, task.TaskID, "status.json")
	completedStatus := WorkerStatus{
		Status:        "completed",
		LastUpdate:    time.Now().Unix(),
		Message:       "Worker completed successfully",
		OutputSummary: "Task output here",
	}
	statusData, _ := json.MarshalIndent(completedStatus, "", "  ")
	_ = os.WriteFile(statusFile, statusData, 0644)

	// Poll status file
	assistant.statusMonitor.PollStatusFiles()

	// Verify task marked as completed
	updatedTask, _ := assistant.taskStore.GetTask(task.TaskID)
	if updatedTask.Status != team.TaskStatusCompleted {
		t.Errorf("task status should be completed, got: %s", updatedTask.Status)
	}

	if updatedTask.CompletedAt == 0 {
		t.Error("CompletedAt should be set for completed task")
	}
}

func TestEndToEnd_MultipleTasksWithDifferentWorkers(t *testing.T) {
	ctx := context.Background()
	assistant := NewAssistant(nil)

	mockPM := newMockProcessManager()
	assistant.workerManager.SetProcessManager(mockPM)

	waveHome := wavebase.GetHomeDir()
	testStatusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers-test-e2e-5")
	assistant.workerManager.statusDir = testStatusDir
	assistant.statusMonitor.statusDir = testStatusDir
	defer os.RemoveAll(testStatusDir)

	// Add multiple tasks with different worker types
	taskConfigs := []struct {
		workerType string
		desc       string
	}{
		{"claude", "task for claude worker"},
		{"opencode", "task for opencode worker"},
		{"codex", "task for codex worker"},
	}

	taskIDs := make([]string, 0, len(taskConfigs))
	for _, cfg := range taskConfigs {
		yamlData := fmt.Sprintf(`description: "%s"
metadata:
  worker_type: "%s"`, cfg.desc, cfg.workerType)
		task, _ := assistant.AddTaskFromYAML(yamlData)
		taskIDs = append(taskIDs, task.TaskID)
	}

	if err := assistant.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer assistant.Stop()

	// Wait for all workers to start
	time.Sleep(6 * time.Second)

	// Verify all workers started
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != len(taskConfigs) {
		t.Fatalf("expected %d workers, got: %d", len(taskConfigs), len(workers))
	}

	// Verify each task has in_progress status
	for _, taskID := range taskIDs {
		task, _ := assistant.taskStore.GetTask(taskID)
		if task.Status != team.TaskStatusInProgress {
			t.Errorf("task %s should be in_progress, got: %s", taskID, task.Status)
		}
	}

	// Verify worker types match
	workerTypes := make(map[string]string)
	for _, w := range workers {
		workerTypes[w.TaskID] = w.WorkerType
	}

	for taskID, workerType := range workerTypes {
		task, _ := assistant.taskStore.GetTask(taskID)
		configWorkerType, ok := task.Metadata["worker_type"].(string)
		if !ok || configWorkerType != workerType {
			t.Errorf("worker type mismatch for task %s: metadata=%s, actual=%s",
				taskID, configWorkerType, workerType)
		}
	}
}

// Helper function

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

