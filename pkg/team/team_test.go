// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// team_test.go provides cross-cutting integration tests and covers gaps
// not addressed by the individual test files (team_db_test, team_fork_test,
// team_config_test, team_heartbeat_test, team_inject_test).

// --- State Machine: boundary and edge case tests ---

func TestValidateWorkerTransition_UnknownFromStatus(t *testing.T) {
	err := ValidateWorkerTransition("nonexistent", WorkerStatusIdle)
	if err == nil {
		t.Fatal("expected error for unknown from-status")
	}
}

func TestValidateTaskTransition_UnknownFromStatus(t *testing.T) {
	err := ValidateTaskTransition("nonexistent", TaskStatusPending)
	if err == nil {
		t.Fatal("expected error for unknown from-status")
	}
}

func TestValidateTaskTransition_TerminalStatesNoExit(t *testing.T) {
	terminalStates := []string{TaskStatusDone, "cancelled"}
	for _, from := range terminalStates {
		for _, to := range []string{
			TaskStatusPending, TaskStatusAssigned, TaskStatusWorking,
			TaskStatusDone, TaskStatusFailed, TaskStatusPaused, "cancelled",
		} {
			err := ValidateTaskTransition(from, to)
			if err == nil {
				t.Errorf("terminal state %q should not allow transition to %q", from, to)
			}
		}
	}
}

func TestValidateWorkerTransition_AllWorkerStatuses(t *testing.T) {
	allStatuses := []string{
		WorkerStatusIdle, WorkerStatusWorking,
		WorkerStatusError, WorkerStatusOffline,
	}
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			err := ValidateWorkerTransition(from, to)
			if from == to {
				// self-transition is never explicitly defined
				if err == nil {
					t.Errorf("unexpected self-transition allowed: %q -> %q", from, to)
				}
			}
		}
	}
}

func TestValidateTaskTransition_SelfTransitionsBlocked(t *testing.T) {
	terminalStates := []string{TaskStatusDone, "cancelled"}
	for _, s := range terminalStates {
		err := ValidateTaskTransition(s, s)
		if err == nil {
			t.Errorf("self-transition should be blocked for terminal status %q", s)
		}
	}
}

func TestValidTaskTransitions_AllowedTransitionsValid(t *testing.T) {
	// Verify every declared target actually validates
	for from, targets := range ValidTaskTransitions {
		for _, to := range targets {
			if err := ValidateTaskTransition(from, to); err != nil {
				t.Errorf("declared transition %q -> %q failed validation: %v", from, to, err)
			}
		}
	}
}

func TestValidWorkerTransitions_AllowedTransitionsValid(t *testing.T) {
	for from, targets := range ValidWorkerTransitions {
		for _, to := range targets {
			if err := ValidateWorkerTransition(from, to); err != nil {
				t.Errorf("declared transition %q -> %q failed validation: %v", from, to, err)
			}
		}
	}
}

// --- Integration: full Member→Worker→Task lifecycle ---

func TestFullLifecycle_MemberCreateAndList(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	m := &TeamMember{Name: "Lifecycle Member", Tool: ToolClaude}
	if err := CreateMember(ctx, m); err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	got, err := GetMember(ctx, m.MemberID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if got.Name != m.Name {
		t.Errorf("Name mismatch: %q vs %q", got.Name, m.Name)
	}
	if got.MemberID != m.MemberID {
		t.Errorf("MemberID mismatch: %q vs %q", got.MemberID, m.MemberID)
	}

	members, err := ListMembers(ctx)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
}

func TestFullLifecycle_MemberUpdateAndDelete(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	m := &TeamMember{Name: "Before", Tool: ToolClaude}
	if err := CreateMember(ctx, m); err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	m.Name = "After"
	m.Model = "anthropic/claude-opus"
	m.MaxConcurrency = 5
	if err := UpdateMember(ctx, m); err != nil {
		t.Fatalf("UpdateMember: %v", err)
	}
	got, err := GetMember(ctx, m.MemberID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if got.Name != "After" {
		t.Errorf("expected Name 'After', got %q", got.Name)
	}
	if got.Model != "anthropic/claude-opus" {
		t.Errorf("expected Model 'anthropic/claude-opus', got %q", got.Model)
	}
	if got.MaxConcurrency != 5 {
		t.Errorf("expected MaxConcurrency 5, got %d", got.MaxConcurrency)
	}

	if err := DeleteMember(ctx, m.MemberID); err != nil {
		t.Fatalf("DeleteMember: %v", err)
	}
	_, err = GetMember(ctx, m.MemberID)
	if err == nil {
		t.Fatal("GetMember should fail after delete")
	}
}

func TestFullLifecycle_WorkerCreateAndList(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-1", Name: "WTest", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	w := &TeamWorker{MemberID: "m-1", Name: "WTest-1"}
	if err := CreateWorker(ctx, w); err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	if w.WorkerID == "" {
		t.Error("WorkerID should be auto-generated")
	}
	if w.Status != WorkerStatusIdle {
		t.Errorf("expected status %q, got %q", WorkerStatusIdle, w.Status)
	}

	got, err := GetWorker(ctx, w.WorkerID)
	if err != nil {
		t.Fatalf("GetWorker: %v", err)
	}
	if got.Name != "WTest-1" {
		t.Errorf("Name mismatch: %q", got.Name)
	}

	workers, err := ListWorkers(ctx, "m-1")
	if err != nil {
		t.Fatalf("ListWorkers: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}

	if err := DeleteWorker(ctx, w.WorkerID); err != nil {
		t.Fatalf("DeleteWorker: %v", err)
	}
	_, err = GetWorker(ctx, w.WorkerID)
	if err == nil {
		t.Fatal("GetWorker should fail after delete")
	}
}

func TestFullLifecycle_TaskCreateAndList(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &TeamTask{Title: "Integration task", Description: "testing full lifecycle"}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.TaskID == "" {
		t.Error("TaskID should be auto-generated")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status %q, got %q", TaskStatusPending, task.Status)
	}
	if task.Priority != PriorityMedium {
		t.Errorf("expected priority %q, got %q", PriorityMedium, task.Priority)
	}

	got, err := GetTask(ctx, task.TaskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Title != task.Title {
		t.Errorf("Title mismatch: %q vs %q", got.Title, task.Title)
	}

	tasks, err := ListTasks(ctx, "", "", "")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if err := DeleteTask(ctx, task.TaskID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	_, err = GetTask(ctx, task.TaskID)
	if err == nil {
		t.Fatal("GetTask should fail after delete")
	}
}

// --- Task CRUD: priority and status edge cases ---

func TestCreateTask_AllPriorities(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	priorities := []string{PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent}
	for _, p := range priorities {
		task := &TeamTask{Title: "Task " + p, Priority: p}
		if err := CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask with priority %q: %v", p, err)
		}
		got, err := GetTask(ctx, task.TaskID)
		if err != nil {
			t.Fatalf("GetTask: %v", err)
		}
		if got.Priority != p {
			t.Errorf("expected priority %q, got %q", p, got.Priority)
		}
	}
}

func TestCreateTask_CustomTaskID(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	customID := "custom-task-001"
	task := &TeamTask{TaskID: customID, Title: "Custom ID Task"}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.TaskID != customID {
		t.Errorf("expected TaskID %q, got %q", customID, task.TaskID)
	}
	got, err := GetTask(ctx, customID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.TaskID != customID {
		t.Errorf("expected TaskID %q, got %q", customID, got.TaskID)
	}
}

func TestCreateWorker_CustomWorkerID(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-1", Name: "CustomW", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	customID := "custom-worker-001"
	w := &TeamWorker{WorkerID: customID, MemberID: "m-1", Name: "CustomW-1"}
	if err := CreateWorker(ctx, w); err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	if w.WorkerID != customID {
		t.Errorf("expected WorkerID %q, got %q", customID, w.WorkerID)
	}
	got, err := GetWorker(ctx, customID)
	if err != nil {
		t.Fatalf("GetWorker: %v", err)
	}
	if got.WorkerID != customID {
		t.Errorf("expected WorkerID %q, got %q", customID, got.WorkerID)
	}
}

func TestUpdateWorker_StatusTransitions(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-1", Name: "StatusTest", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	w := &TeamWorker{MemberID: "m-1", Name: "StatusTest-1"}
	if err := CreateWorker(ctx, w); err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}

	// idle -> working
	w.Status = WorkerStatusWorking
	w.PID = 12345
	if err := UpdateWorker(ctx, w); err != nil {
		t.Fatalf("UpdateWorker idle->working: %v", err)
	}
	got, _ := GetWorker(ctx, w.WorkerID)
	if got.Status != WorkerStatusWorking {
		t.Errorf("expected status %q, got %q", WorkerStatusWorking, got.Status)
	}
	if got.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", got.PID)
	}

	// working -> error
	w.Status = WorkerStatusError
	if err := UpdateWorker(ctx, w); err != nil {
		t.Fatalf("UpdateWorker working->error: %v", err)
	}
	got, _ = GetWorker(ctx, w.WorkerID)
	if got.Status != WorkerStatusError {
		t.Errorf("expected status %q, got %q", WorkerStatusError, got.Status)
	}
}

func TestUpdateTask_FullLifecycle(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &TeamTask{Title: "Lifecycle Task"}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// pending -> assigned
	task.Status = TaskStatusAssigned
	task.AssignedMemberID = "member-1"
	if err := UpdateTask(ctx, task); err != nil {
		t.Fatalf("UpdateTask pending->assigned: %v", err)
	}
	got, _ := GetTask(ctx, task.TaskID)
	if got.Status != TaskStatusAssigned {
		t.Errorf("expected status %q, got %q", TaskStatusAssigned, got.Status)
	}

	// assigned -> working
	task.Status = TaskStatusWorking
	task.AssignedWorkerID = "worker-1"
	task.Progress = 50
	if err := UpdateTask(ctx, task); err != nil {
		t.Fatalf("UpdateTask assigned->working: %v", err)
	}
	got, _ = GetTask(ctx, task.TaskID)
	if got.Progress != 50 {
		t.Errorf("expected Progress 50, got %d", got.Progress)
	}

	// working -> done
	task.Status = TaskStatusDone
	task.Progress = 100
	task.Result = "All done"
	completedTime := time.Now().Unix()
	task.CompletedAt = completedTime
	if err := UpdateTask(ctx, task); err != nil {
		t.Fatalf("UpdateTask working->done: %v", err)
	}
	got, _ = GetTask(ctx, task.TaskID)
	if got.Status != TaskStatusDone {
		t.Errorf("expected status %q, got %q", TaskStatusDone, got.Status)
	}
	if got.Result != "All done" {
		t.Errorf("expected Result 'All done', got %q", got.Result)
	}
	if got.CompletedAt == 0 {
		t.Error("CompletedAt should be set")
	}
}

func TestUpdateTask_FailAndRetry(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &TeamTask{Title: "Retry Task", MaxRetries: 3}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// pending -> working
	task.Status = TaskStatusWorking
	UpdateTask(ctx, task)

	// working -> failed
	task.Status = TaskStatusFailed
	task.Error = "something broke"
	task.RetryCount = 1
	UpdateTask(ctx, task)

	got, _ := GetTask(ctx, task.TaskID)
	if got.Status != TaskStatusFailed {
		t.Errorf("expected status %q, got %q", TaskStatusFailed, got.Status)
	}
	if got.RetryCount != 1 {
		t.Errorf("expected RetryCount 1, got %d", got.RetryCount)
	}

	// failed -> working (retry)
	task.Status = TaskStatusWorking
	task.Error = ""
	UpdateTask(ctx, task)

	got, _ = GetTask(ctx, task.TaskID)
	if got.Status != TaskStatusWorking {
		t.Errorf("expected status %q after retry, got %q", TaskStatusWorking, got.Status)
	}
}

func TestUpdateTask_PauseAndResume(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &TeamTask{Title: "Pause Task"}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// pending -> working
	task.Status = TaskStatusWorking
	UpdateTask(ctx, task)

	// working -> paused
	task.Status = TaskStatusPaused
	UpdateTask(ctx, task)
	got, _ := GetTask(ctx, task.TaskID)
	if got.Status != TaskStatusPaused {
		t.Errorf("expected status %q, got %q", TaskStatusPaused, got.Status)
	}

	// paused -> working (resume)
	task.Status = TaskStatusWorking
	UpdateTask(ctx, task)
	got, _ = GetTask(ctx, task.TaskID)
	if got.Status != TaskStatusWorking {
		t.Errorf("expected status %q after resume, got %q", TaskStatusWorking, got.Status)
	}
}

// --- Activity: combined filters and edge cases ---

func TestListActivities_CombinedFilters(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	// Create activities with different filters
	AddActivity(ctx, &TeamActivity{TaskID: "task-1", WorkerID: "w-1", MemberID: "m-1", Type: "created"})
	AddActivity(ctx, &TeamActivity{TaskID: "task-1", WorkerID: "w-2", MemberID: "m-1", Type: "assigned"})
	AddActivity(ctx, &TeamActivity{TaskID: "task-2", WorkerID: "w-1", MemberID: "m-2", Type: "created"})
	AddActivity(ctx, &TeamActivity{TaskID: "task-1", WorkerID: "w-1", MemberID: "m-1", Type: "completed"})

	// Filter by taskID AND workerID
	acts, err := ListActivities(ctx, 100, "task-1", "w-1", "")
	if err != nil {
		t.Fatalf("ListActivities: %v", err)
	}
	if len(acts) != 2 {
		t.Errorf("expected 2 activities for task-1+worker-1, got %d", len(acts))
	}

	// Filter by taskID AND memberID
	acts, err = ListActivities(ctx, 100, "task-1", "", "m-2")
	if err != nil {
		t.Fatalf("ListActivities: %v", err)
	}
	if len(acts) != 0 {
		t.Errorf("expected 0 activities for task-1+member-2, got %d", len(acts))
	}
}

func TestCleanupOldActivities_ZeroMax(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	AddActivity(ctx, &TeamActivity{Type: "test"})
	AddActivity(ctx, &TeamActivity{Type: "test"})

	if err := CleanupOldActivities(ctx, 0); err != nil {
		t.Fatalf("CleanupOldActivities(0): %v", err)
	}
	acts, _ := ListActivities(ctx, 100, "", "", "")
	if len(acts) != 0 {
		t.Errorf("expected 0 activities after cleanup with max=0, got %d", len(acts))
	}
}

func TestCleanupOldActivities_ExactlyMax(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		AddActivity(ctx, &TeamActivity{Type: "test"})
	}

	if err := CleanupOldActivities(ctx, 5); err != nil {
		t.Fatalf("CleanupOldActivities(5): %v", err)
	}
	acts, _ := ListActivities(ctx, 100, "", "", "")
	if len(acts) != 5 {
		t.Errorf("expected 5 activities after cleanup with max=5, got %d", len(acts))
	}
}

// --- Fork/Recycle: edge cases ---

func TestForkWorker_MemberWithHyphenatedName(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-hyphen", Name: "Go-Backend-Dev", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	w1, err := ForkWorker(ctx, "m-hyphen")
	if err != nil {
		t.Fatalf("ForkWorker: %v", err)
	}
	if w1.Name != "Go-Backend-Dev-1" {
		t.Errorf("expected name 'Go-Backend-Dev-1', got %q", w1.Name)
	}

	w2, err := ForkWorker(ctx, "m-hyphen")
	if err != nil {
		t.Fatalf("ForkWorker #2: %v", err)
	}
	if w2.Name != "Go-Backend-Dev-2" {
		t.Errorf("expected name 'Go-Backend-Dev-2', got %q", w2.Name)
	}
}

func TestForkAndRecycle_MultipleTimes(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-multi", Name: "Multi", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	// Fork 3 workers (max)
	for i := 1; i <= 3; i++ {
		w, err := ForkWorker(ctx, "m-multi")
		if err != nil {
			t.Fatalf("ForkWorker #%d: %v", i, err)
		}
		expectedName := fmt.Sprintf("Multi-%d", i)
		if w.Name != expectedName {
			t.Errorf("worker #%d: expected name %q, got %q", i, expectedName, w.Name)
		}
	}

	// 4th should fail
	_, err := ForkWorker(ctx, "m-multi")
	if err == nil {
		t.Fatal("expected concurrency limit error on 4th fork")
	}

	// Recycle one
	workers, _ := ListWorkers(ctx, "m-multi")
	if len(workers) < 1 {
		t.Fatal("expected at least 1 worker")
	}
	if err := RecycleWorker(ctx, workers[0].WorkerID); err != nil {
		t.Fatalf("RecycleWorker: %v", err)
	}

	// Now we can fork again (but naming continues from last max)
	w, err := ForkWorker(ctx, "m-multi")
	if err != nil {
		t.Fatalf("ForkWorker after recycle: %v", err)
	}
	if w.Name != "Multi-4" {
		t.Errorf("expected name 'Multi-4' after recycle, got %q", w.Name)
	}
}

func TestRecycleWorker_AlreadyOffline(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-recycle", Name: "Recycle", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	w, _ := ForkWorker(ctx, "m-recycle")
	// Manually set worker to offline
	w.Status = WorkerStatusOffline
	UpdateWorker(ctx, w)

	err := RecycleWorker(ctx, w.WorkerID)
	if err == nil {
		t.Fatal("RecycleWorker should fail for already-offline worker")
	}
}

func TestRecycleWorker_ErrorStateToOffline(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-err", Name: "ErrTest", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	w, _ := ForkWorker(ctx, "m-err")
	w.Status = WorkerStatusError
	UpdateWorker(ctx, w)

	if err := RecycleWorker(ctx, w.WorkerID); err != nil {
		t.Fatalf("RecycleWorker from error state: %v", err)
	}
	got, _ := GetWorker(ctx, w.WorkerID)
	if got.Status != WorkerStatusOffline {
		t.Errorf("expected status %q, got %q", WorkerStatusOffline, got.Status)
	}
	if got.PID != 0 {
		t.Errorf("expected PID 0 after recycle, got %d", got.PID)
	}
	if got.AssignedTaskID != "" {
		t.Errorf("expected empty AssignedTaskID after recycle, got %q", got.AssignedTaskID)
	}
}

// --- Config: ParseMemberYAML edge cases ---

func TestParseMemberYAML_AllTools(t *testing.T) {
	tools := []string{ToolClaude, ToolOpenCode, ToolCursor, ToolAider, ToolCustom}
	for _, tool := range tools {
		yaml := "name: Test " + tool + "\ntool: " + tool
		m, err := ParseMemberYAML([]byte(yaml))
		if err != nil {
			t.Fatalf("ParseMemberYAML with tool %q: %v", tool, err)
		}
		if m.Tool != tool {
			t.Errorf("expected tool %q, got %q", tool, m.Tool)
		}
	}
}

func TestParseMemberYAML_AllMemoryModes(t *testing.T) {
	modes := []string{MemoryNone, MemorySession, MemoryPersistent}
	for _, mode := range modes {
		yaml := "name: MemTest\nmemory: " + mode
		m, err := ParseMemberYAML([]byte(yaml))
		if err != nil {
			t.Fatalf("ParseMemberYAML with memory %q: %v", mode, err)
		}
		if m.Memory != mode {
			t.Errorf("expected memory %q, got %q", mode, m.Memory)
		}
	}
}

func TestParseMemberYAML_SkillsAsEmptyList(t *testing.T) {
	yaml := "name: EmptySkills\nskills: []"
	m, err := ParseMemberYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseMemberYAML: %v", err)
	}
	if m.Skills == nil {
		t.Error("Skills should not be nil for empty list")
	}
	if len(m.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(m.Skills))
	}
}

func TestParseMemberYAML_McpServersWithEnv(t *testing.T) {
	yaml := `
name: EnvTest
mcpServers:
  - name: test-srv
    type: stdio
    command: node
    args: ["server.js"]
    env:
      API_KEY: "secret123"
      PORT: "8080"
`
	m, err := ParseMemberYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseMemberYAML: %v", err)
	}
	if len(m.McpServers) != 1 {
		t.Fatalf("expected 1 MCP server, got %d", len(m.McpServers))
	}
	srv := m.McpServers[0]
	if srv.Env == nil {
		t.Fatal("Env should not be nil")
	}
	if srv.Env["API_KEY"] != "secret123" {
		t.Errorf("expected API_KEY 'secret123', got %q", srv.Env["API_KEY"])
	}
	if srv.Env["PORT"] != "8080" {
		t.Errorf("expected PORT '8080', got %q", srv.Env["PORT"])
	}
}

func TestParseMemberYAML_McpServersHttpWithHeaders(t *testing.T) {
	yaml := `
name: HttpTest
mcpServers:
  - name: remote-api
    type: http
    url: https://api.example.com/mcp
    headers:
      Authorization: "Bearer token123"
      X-Custom: "value"
`
	m, err := ParseMemberYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseMemberYAML: %v", err)
	}
	srv := m.McpServers[0]
	if srv.Type != "http" {
		t.Errorf("expected type 'http', got %q", srv.Type)
	}
	if srv.URL != "https://api.example.com/mcp" {
		t.Errorf("unexpected URL: %q", srv.URL)
	}
	if srv.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("unexpected Authorization header: %q", srv.Headers["Authorization"])
	}
}

func TestParseMemberYAML_CustomCmd(t *testing.T) {
	yaml := "name: CustomTool\ntool: custom\ncustomCmd: 'my-custom-cli --flag'"
	m, err := ParseMemberYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseMemberYAML: %v", err)
	}
	if m.Tool != ToolCustom {
		t.Errorf("expected tool %q, got %q", ToolCustom, m.Tool)
	}
	if m.CustomCmd != "my-custom-cli --flag" {
		t.Errorf("expected customCmd 'my-custom-cli --flag', got %q", m.CustomCmd)
	}
}

// --- Status: comprehensive GetStatus ---

func TestGetStatus_AllWorkerStates(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-1", Name: "S1", Tool: ToolClaude,
		MaxConcurrency: 10, MaxRetries: 3, Memory: MemorySession,
	})

	// Create workers in different states
	CreateWorker(ctx, &TeamWorker{MemberID: "m-1", Name: "idle-1"})
	CreateWorker(ctx, &TeamWorker{MemberID: "m-1", Name: "working-1", Status: WorkerStatusWorking, PID: 1000})
	CreateWorker(ctx, &TeamWorker{MemberID: "m-1", Name: "offline-1", Status: WorkerStatusOffline})
	CreateWorker(ctx, &TeamWorker{MemberID: "m-1", Name: "error-1", Status: WorkerStatusError})

	// Create tasks in different states
	CreateTask(ctx, &TeamTask{Title: "P", Status: TaskStatusPending, Priority: PriorityLow})
	CreateTask(ctx, &TeamTask{Title: "W", Status: TaskStatusWorking, Priority: PriorityHigh})
	CreateTask(ctx, &TeamTask{Title: "D", Status: TaskStatusDone})
	CreateTask(ctx, &TeamTask{Title: "F", Status: TaskStatusFailed})
	CreateTask(ctx, &TeamTask{Title: "Pa", Status: TaskStatusPaused})

	status, err := GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.TotalMembers != 1 {
		t.Errorf("TotalMembers: expected 1, got %d", status.TotalMembers)
	}
	if status.ActiveWorkers != 1 {
		t.Errorf("ActiveWorkers: expected 1, got %d", status.ActiveWorkers)
	}
	if status.IdleWorkers != 1 {
		t.Errorf("IdleWorkers: expected 1, got %d", status.IdleWorkers)
	}
	if status.OfflineWorkers != 1 {
		t.Errorf("OfflineWorkers: expected 1, got %d", status.OfflineWorkers)
	}
	if status.PendingTasks != 1 {
		t.Errorf("PendingTasks: expected 1, got %d", status.PendingTasks)
	}
	if status.WorkingTasks != 1 {
		t.Errorf("WorkingTasks: expected 1, got %d", status.WorkingTasks)
	}
	if status.DoneTasks != 1 {
		t.Errorf("DoneTasks: expected 1, got %d", status.DoneTasks)
	}
	if status.FailedTasks != 1 {
		t.Errorf("FailedTasks: expected 1, got %d", status.FailedTasks)
	}
	if status.PausedTasks != 1 {
		t.Errorf("PausedTasks: expected 1, got %d", status.PausedTasks)
	}
}

// --- Task: output history and dependsOn round-trip ---

func TestTaskOutputHistory_RoundTrip(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	history := []TaskOutput{
		{Timestamp: "2026-01-01T00:00:00Z", Type: "stdout", Content: "hello"},
		{Timestamp: "2026-01-01T00:00:01Z", Type: "stderr", Content: "error msg"},
		{Timestamp: "2026-01-01T00:00:02Z", Type: "result", Content: "done"},
	}
	task := &TeamTask{
		Title: "Output Test",
		OutputHistory: history,
	}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := GetTask(ctx, task.TaskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if len(got.OutputHistory) != 3 {
		t.Fatalf("expected 3 output entries, got %d", len(got.OutputHistory))
	}
	if got.OutputHistory[0].Content != "hello" {
		t.Errorf("entry 0: expected 'hello', got %q", got.OutputHistory[0].Content)
	}
	if got.OutputHistory[1].Type != "stderr" {
		t.Errorf("entry 1: expected type 'stderr', got %q", got.OutputHistory[1].Type)
	}
	if got.OutputHistory[2].Content != "done" {
		t.Errorf("entry 2: expected 'done', got %q", got.OutputHistory[2].Content)
	}
}

func TestTaskDependsOn_RoundTrip(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	deps := []string{"task-1", "task-2", "task-3"}
	task := &TeamTask{
		Title:     "Dependent Task",
		DependsOn: deps,
	}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := GetTask(ctx, task.TaskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if len(got.DependsOn) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(got.DependsOn))
	}
	for i, dep := range deps {
		if got.DependsOn[i] != dep {
			t.Errorf("dependency %d: expected %q, got %q", i, dep, got.DependsOn[i])
		}
	}
}

func TestTaskResultAndError_SpecialCharacters(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &TeamTask{Title: "Special Chars"}
	CreateTask(ctx, task)

	specialResult := `{"key": "value with \"quotes\" and\nnewlines"}`
	specialError := "error: panic: runtime error: index out of range [0] with length 0"
	task.Result = specialResult
	task.Error = specialError
	task.Status = TaskStatusFailed
	UpdateTask(ctx, task)

	got, _ := GetTask(ctx, task.TaskID)
	if got.Result != specialResult {
		t.Errorf("Result mismatch:\n  expected: %q\n  got:      %q", specialResult, got.Result)
	}
	if got.Error != specialError {
		t.Errorf("Error mismatch:\n  expected: %q\n  got:      %q", specialError, got.Error)
	}
}

// --- Member: update preserves non-modified fields ---

func TestUpdateMember_PreservesFields(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	m := &TeamMember{
		Name:           "Preserve",
		Tool:           ToolOpenCode,
		Description:    "original desc",
		Persona:        "original persona",
		Skills:         []string{"go-testing", "debugging"},
		MaxConcurrency: 5,
		MaxRetries:     2,
		Memory:         MemoryPersistent,
		Color:          "#FF0000",
		Model:          "anthropic/claude-opus",
	}
	if err := CreateMember(ctx, m); err != nil {
		t.Fatalf("CreateMember: %v", err)
	}

	// Update only the name
	m.Name = "Updated Preserve"
	if err := UpdateMember(ctx, m); err != nil {
		t.Fatalf("UpdateMember: %v", err)
	}

	got, err := GetMember(ctx, m.MemberID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if got.Name != "Updated Preserve" {
		t.Errorf("Name not updated: %q", got.Name)
	}
	if got.Tool != ToolOpenCode {
		t.Errorf("Tool changed unexpectedly: %q", got.Tool)
	}
	if got.Description != "original desc" {
		t.Errorf("Description changed unexpectedly: %q", got.Description)
	}
	if got.Persona != "original persona" {
		t.Errorf("Persona changed unexpectedly: %q", got.Persona)
	}
	if got.MaxConcurrency != 5 {
		t.Errorf("MaxConcurrency changed: %d", got.MaxConcurrency)
	}
	if got.Memory != MemoryPersistent {
		t.Errorf("Memory changed: %q", got.Memory)
	}
	if got.Color != "#FF0000" {
		t.Errorf("Color changed: %q", got.Color)
	}
	if got.Model != "anthropic/claude-opus" {
		t.Errorf("Model changed: %q", got.Model)
	}
}

func TestUpdateMember_SkillsUpdate(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	m := &TeamMember{
		Name:       "SkillUp",
		Tool:       ToolClaude,
		Skills:     []string{"go"},
		Capabilities: []string{"Read", "Write"},
	}
	if err := CreateMember(ctx, m); err != nil {
		t.Fatalf("CreateMember: %v", err)
	}

	m.Skills = []string{"go", "react", "typescript"}
	m.Capabilities = []string{"Read", "Write", "Edit", "Bash"}
	if err := UpdateMember(ctx, m); err != nil {
		t.Fatalf("UpdateMember: %v", err)
	}

	got, _ := GetMember(ctx, m.MemberID)
	if len(got.Skills) != 3 {
		t.Errorf("expected 3 skills, got %d", len(got.Skills))
	}
	if len(got.Capabilities) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(got.Capabilities))
	}
}

// --- ListTasks: filtering combinations ---

func TestListTasks_MultipleFilters(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-f", Name: "Filter", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	CreateTask(ctx, &TeamTask{Title: "A", Priority: PriorityHigh, Status: TaskStatusPending, AssignedMemberID: "m-f"})
	CreateTask(ctx, &TeamTask{Title: "B", Priority: PriorityLow, Status: TaskStatusWorking, AssignedMemberID: "m-f"})
	CreateTask(ctx, &TeamTask{Title: "C", Priority: PriorityHigh, Status: TaskStatusWorking, AssignedMemberID: "m-f"})
	CreateTask(ctx, &TeamTask{Title: "D", Priority: PriorityHigh, Status: TaskStatusPending, AssignedMemberID: "other"})

	// Filter: status=working, priority=high, member=m-f → only C
	tasks, err := ListTasks(ctx, TaskStatusWorking, PriorityHigh, "m-f")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "C" {
		t.Errorf("expected task 'C', got %q", tasks[0].Title)
	}

	// Filter: status=pending, priority=high, member=other → only D
	tasks, err = ListTasks(ctx, TaskStatusPending, PriorityHigh, "other")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "D" {
		t.Errorf("expected task 'D', got %q", tasks[0].Title)
	}

	// Filter: status=working → B and C
	tasks, err = ListTasks(ctx, TaskStatusWorking, "", "")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

// --- JSON serialization check ---

func TestTeamMember_JSONRoundTrip(t *testing.T) {
	original := TeamMember{
		MemberID:       "test-id",
		Name:           "JSON Test",
		Tool:           ToolClaude,
		Skills:         []string{"go", "react"},
		MaxConcurrency: 5,
		Memory:         MemoryPersistent,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded TeamMember
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.MemberID != original.MemberID {
		t.Errorf("MemberID mismatch: %q vs %q", decoded.MemberID, original.MemberID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: %q vs %q", decoded.Name, original.Name)
	}
	if decoded.Tool != original.Tool {
		t.Errorf("Tool mismatch: %q vs %q", decoded.Tool, original.Tool)
	}
	if len(decoded.Skills) != len(original.Skills) {
		t.Errorf("Skills length: %d vs %d", len(decoded.Skills), len(original.Skills))
	}
	if decoded.MaxConcurrency != original.MaxConcurrency {
		t.Errorf("MaxConcurrency: %d vs %d", decoded.MaxConcurrency, original.MaxConcurrency)
	}
	if decoded.Memory != original.Memory {
		t.Errorf("Memory: %q vs %q", decoded.Memory, original.Memory)
	}
}

func TestMCPConfig_JSONRoundTrip(t *testing.T) {
	original := MCPConfig{
		Name:    "test-mcp",
		Type:    "stdio",
		Command: "npx",
		Args:    []string{"-y", "test-server"},
		Env:     map[string]string{"KEY": "val"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MCPConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: %q vs %q", decoded.Name, original.Name)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: %q vs %q", decoded.Type, original.Type)
	}
	if decoded.Env["KEY"] != "val" {
		t.Errorf("Env KEY: %q", decoded.Env["KEY"])
	}
}

func TestTaskOutput_JSONRoundTrip(t *testing.T) {
	original := TaskOutput{
		Timestamp: "2026-04-30T12:00:00Z",
		Type:      "stdout",
		Content:   "line1\nline2\nline3",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded TaskOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content mismatch: %q vs %q", decoded.Content, original.Content)
	}
}

func TestTeamActivity_JSONRoundTrip(t *testing.T) {
	original := TeamActivity{
		Id:          42,
		TaskID:      "task-1",
		WorkerID:    "worker-1",
		MemberID:    "member-1",
		Type:        "forked",
		Description: "forked worker",
		Meta:        `{"key": "value"}`,
		CreatedAt:   time.Now().Unix(),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded TeamActivity
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Id != original.Id {
		t.Errorf("Id mismatch: %d vs %d", decoded.Id, original.Id)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: %q vs %q", decoded.Type, original.Type)
	}
	if decoded.Meta != original.Meta {
		t.Errorf("Meta mismatch: %q vs %q", decoded.Meta, original.Meta)
	}
}

// --- Constants validation ---

func TestConstants_NonEmpty(t *testing.T) {
	if TaskStatusPending == "" || TaskStatusAssigned == "" || TaskStatusWorking == "" ||
		TaskStatusDone == "" || TaskStatusFailed == "" || TaskStatusPaused == "" {
		t.Error("task status constants should not be empty")
	}
	if WorkerStatusIdle == "" || WorkerStatusWorking == "" ||
		WorkerStatusOffline == "" || WorkerStatusError == "" {
		t.Error("worker status constants should not be empty")
	}
	if PriorityLow == "" || PriorityMedium == "" || PriorityHigh == "" || PriorityUrgent == "" {
		t.Error("priority constants should not be empty")
	}
	if MemoryNone == "" || MemorySession == "" || MemoryPersistent == "" {
		t.Error("memory constants should not be empty")
	}
	if ToolClaude == "" || ToolOpenCode == "" || ToolCursor == "" ||
		ToolAider == "" || ToolCustom == "" {
		t.Error("tool constants should not be empty")
	}
}

func TestHeartbeatTimeout_GreaterThanInterval(t *testing.T) {
	if HeartbeatTimeout <= HeartbeatInterval {
		t.Errorf("HeartbeatTimeout (%v) should be > HeartbeatInterval (%v)",
			HeartbeatTimeout, HeartbeatInterval)
	}
}

// --- Delete: non-existent IDs don't panic ---

func TestDeleteMember_NotExist(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	err := DeleteMember(ctx, "nonexistent-member-id")
	if err != nil {
		t.Fatalf("DeleteMember non-existent should not error: %v", err)
	}
}

func TestDeleteWorker_NotExist(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	err := DeleteWorker(ctx, "nonexistent-worker-id")
	if err != nil {
		t.Fatalf("DeleteWorker non-existent should not error: %v", err)
	}
}

func TestDeleteTask_NotExist(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	err := DeleteTask(ctx, "nonexistent-task-id")
	if err != nil {
		t.Fatalf("DeleteTask non-existent should not error: %v", err)
	}
}

// --- Config: edge cases for defaults and parsing ---

func TestApplyDefaults_NegativeValuesGetDefault(t *testing.T) {
	m := &TeamMember{MaxConcurrency: -1, MaxRetries: -5}
	applyDefaults(m)
	if m.MaxConcurrency != 3 {
		t.Errorf("negative MaxConcurrency should default to 3, got %d", m.MaxConcurrency)
	}
	if m.MaxRetries != 3 {
		t.Errorf("negative MaxRetries should default to 3, got %d", m.MaxRetries)
	}
}

func TestDerefInt(t *testing.T) {
	val := 5
	if got := derefInt(&val, 0); got != 5 {
		t.Errorf("expected 5, got %d", got)
	}
	if got := derefInt(nil, 42); got != 42 {
		t.Errorf("expected fallback 42, got %d", got)
	}
}

func TestLoadDefaultTemplates_CountAndNames(t *testing.T) {
	members, err := LoadDefaultTemplates()
	if err != nil {
		t.Fatalf("LoadDefaultTemplates: %v", err)
	}
	if len(members) != 4 {
		t.Fatalf("expected 4 default templates, got %d", len(members))
	}
	names := make(map[string]bool)
	for _, m := range members {
		if m.Name == "" {
			t.Error("default template should have a name")
		}
		if names[m.Name] {
			t.Errorf("duplicate template name: %q", m.Name)
		}
		names[m.Name] = true
		if m.MaxConcurrency <= 0 {
			t.Errorf("template %q: MaxConcurrency should be > 0", m.Name)
		}
		if m.Memory == "" {
			t.Errorf("template %q: Memory should not be empty", m.Name)
		}
		if m.Tool == "" {
			t.Errorf("template %q: Tool should not be empty", m.Name)
		}
	}
}

func TestIsYAMLFile_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"config.yaml", true},
		{"config.yml", true},
		{"config.YAML", false},
		{"config", false},
		{"config.json", false},
		{"", false},
		{".yaml", true},
	}
	for _, tt := range tests {
		got := isYAMLFile(tt.name)
		if got != tt.want {
			t.Errorf("isYAMLFile(%q): expected %v, got %v", tt.name, tt.want, got)
		}
	}
}

// --- DB: list members error path via rows.Err() ---

func TestCreateMember_WithMcpServers(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	m := &TeamMember{
		Name: "MCP Member",
		Tool: ToolClaude,
		McpServers: []MCPConfig{
			{Name: "srv1", Type: "stdio", Command: "npx", Args: []string{"-y", "pkg"}},
			{Name: "srv2", Type: "http", URL: "https://example.com/mcp", Headers: map[string]string{"Auth": "Bearer x"}},
		},
	}
	if err := CreateMember(ctx, m); err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	got, err := GetMember(ctx, m.MemberID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if len(got.McpServers) != 2 {
		t.Fatalf("expected 2 MCP servers, got %d", len(got.McpServers))
	}
	if got.McpServers[0].Command != "npx" {
		t.Errorf("server 1 command: expected 'npx', got %q", got.McpServers[0].Command)
	}
	if got.McpServers[1].URL != "https://example.com/mcp" {
		t.Errorf("server 2 URL: %q", got.McpServers[1].URL)
	}
}

// --- Fork: worker naming with numbers in member name ---

func TestForkWorker_MemberNameWithNumber(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	createTestMember(ctx, t, &TeamMember{
		MemberID: "m-num", Name: "Worker2", Tool: ToolClaude,
		MaxConcurrency: 3, MaxRetries: 3, Memory: MemorySession,
	})

	w, err := ForkWorker(ctx, "m-num")
	if err != nil {
		t.Fatalf("ForkWorker: %v", err)
	}
	if w.Name != "Worker2-1" {
		t.Errorf("expected 'Worker2-1', got %q", w.Name)
	}
}

// --- Task: create with all fields populated ---

func TestCreateTask_AllFields(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	now := time.Now().Unix()
	task := &TeamTask{
		Title:            "Full Task",
		Description:      "desc",
		Priority:         PriorityUrgent,
		Status:           TaskStatusWorking,
		AssignedMemberID: "m-1",
		AssignedWorkerID: "w-1",
		DependsOn:        []string{"dep-1"},
		Result:           "result text",
		Error:            "error text",
		OutputHistory:    []TaskOutput{{Timestamp: "t1", Type: "stdout", Content: "out"}},
		Progress:         75,
		RetryCount:       2,
		MaxRetries:       5,
		NextRetryAt:      now + 60,
	}
	if err := CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// Verify CreateTask overrides status/priority for defaults
	if task.Status != TaskStatusWorking {
		t.Errorf("expected status %q preserved, got %q", TaskStatusWorking, task.Status)
	}
	if task.Priority != PriorityUrgent {
		t.Errorf("expected priority %q preserved, got %q", PriorityUrgent, task.Priority)
	}

	got, err := GetTask(ctx, task.TaskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Progress != 75 {
		t.Errorf("Progress: expected 75, got %d", got.Progress)
	}
	if got.RetryCount != 2 {
		t.Errorf("RetryCount: expected 2, got %d", got.RetryCount)
	}
	if got.MaxRetries != 5 {
		t.Errorf("MaxRetries: expected 5, got %d", got.MaxRetries)
	}
	if got.NextRetryAt == 0 {
		t.Error("NextRetryAt should be set")
	}
	if got.AssignedMemberID != "m-1" {
		t.Errorf("AssignedMemberID: expected 'm-1', got %q", got.AssignedMemberID)
	}
	if got.AssignedWorkerID != "w-1" {
		t.Errorf("AssignedWorkerID: expected 'w-1', got %q", got.AssignedWorkerID)
	}
}

func TestValidateTaskTransition_CancelledIsTerminal(t *testing.T) {
	// Note: "cancelled" exists in ValidTaskTransitions but is NOT a valid DB value
	// (DB CHECK constraint only allows pending/assigned/working/done/failed/paused).
	// This tests the in-memory state machine only.
	for _, to := range []string{
		TaskStatusPending, TaskStatusAssigned, TaskStatusWorking,
		TaskStatusDone, TaskStatusFailed, TaskStatusPaused,
	} {
		err := ValidateTaskTransition("cancelled", to)
		if err == nil {
			t.Errorf("cancelled -> %q should be blocked", to)
		}
	}
}

func TestValidateTaskTransition_DoneIsTerminal(t *testing.T) {
	for _, to := range []string{
		TaskStatusPending, TaskStatusAssigned, TaskStatusWorking,
		TaskStatusFailed, TaskStatusPaused, "cancelled",
	} {
		err := ValidateTaskTransition(TaskStatusDone, to)
		if err == nil {
			t.Errorf("done -> %q should be blocked", to)
		}
	}
}

