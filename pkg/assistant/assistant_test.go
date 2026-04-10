// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/zeroai/service"
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
