// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
  "context"
  "testing"
  "time"

  _ "github.com/mattn/go-sqlite3"
)

// --- Member Tests ---

func TestCreateMember(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Go Developer", Tool: ToolClaude}
  err := CreateMember(ctx, m)
  if err != nil {
    t.Fatalf("CreateMember failed: %v", err)
  }
  if m.MemberID == "" {
    t.Error("MemberID should be auto-generated")
  }
  if m.Tool != ToolClaude {
    t.Errorf("expected tool '%s', got '%s'", ToolClaude, m.Tool)
  }
  if m.Memory != MemorySession {
    t.Errorf("expected memory '%s', got '%s'", MemorySession, m.Memory)
  }
  if m.MaxConcurrency != 3 {
    t.Errorf("expected maxConcurrency 3, got %d", m.MaxConcurrency)
  }
  if m.MaxRetries != 3 {
    t.Errorf("expected maxRetries 3, got %d", m.MaxRetries)
  }
  if m.CreatedAt == 0 {
    t.Error("CreatedAt should be set")
  }
}

func TestCreateMemberWithJSONFields(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{
    Name:         "Full Member",
    Tool:         ToolOpenCode,
    Skills:       []string{"go-testing", "debugging"},
    Capabilities: []string{"Read", "Write", "Edit", "Bash"},
    McpServers: []MCPConfig{
      {Name: "context7", Type: "stdio", Command: "npx", Args: []string{"-y", "@upstash/context7-mcp"}},
    },
    Model:          "anthropic/claude-sonnet",
    MaxConcurrency: 5,
    MaxRetries:     10,
    Memory:         MemoryPersistent,
    Persona:        "You are a Go developer.",
    PersonaPath:    "./personas/go.md",
    Color:          "#3B82F6",
  }
  err := CreateMember(ctx, m)
  if err != nil {
    t.Fatalf("CreateMember failed: %v", err)
  }

  got, err := GetMember(ctx, m.MemberID)
  if err != nil {
    t.Fatalf("GetMember failed: %v", err)
  }
  if len(got.Skills) != 2 || got.Skills[0] != "go-testing" || got.Skills[1] != "debugging" {
    t.Errorf("expected skills [go-testing, debugging], got %v", got.Skills)
  }
  if len(got.Capabilities) != 4 {
    t.Errorf("expected 4 capabilities, got %d", len(got.Capabilities))
  }
  if len(got.McpServers) != 1 || got.McpServers[0].Name != "context7" {
    t.Errorf("expected 1 mcpServer with name 'context7', got %v", got.McpServers)
  }
  if got.Model != "anthropic/claude-sonnet" {
    t.Errorf("expected model 'anthropic/claude-sonnet', got '%s'", got.Model)
  }
  if got.MaxConcurrency != 5 {
    t.Errorf("expected maxConcurrency 5, got %d", got.MaxConcurrency)
  }
  if got.Persona != "You are a Go developer." {
    t.Errorf("expected persona to be set, got '%s'", got.Persona)
  }
  if got.PersonaPath != "./personas/go.md" {
    t.Errorf("expected personaPath './personas/go.md', got '%s'", got.PersonaPath)
  }
  if got.Color != "#3B82F6" {
    t.Errorf("expected color '#3B82F6', got '%s'", got.Color)
  }
}

func TestGetMember(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Test Member", Description: "A developer"}
  err := CreateMember(ctx, m)
  if err != nil {
    t.Fatalf("CreateMember failed: %v", err)
  }

  got, err := GetMember(ctx, m.MemberID)
  if err != nil {
    t.Fatalf("GetMember failed: %v", err)
  }
  if got.MemberID != m.MemberID {
    t.Errorf("expected MemberID '%s', got '%s'", m.MemberID, got.MemberID)
  }
  if got.Name != "Test Member" {
    t.Errorf("expected name 'Test Member', got '%s'", got.Name)
  }
  if got.Description != "A developer" {
    t.Errorf("expected description 'A developer', got '%s'", got.Description)
  }
}

func TestGetMemberNotFound(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  _, err := GetMember(ctx, "non-existent-id")
  if err == nil {
    t.Error("expected error for non-existent member, got nil")
  }
}

func TestUpdateMember(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Original", Tool: ToolClaude, Skills: []string{"go"}}
  err := CreateMember(ctx, m)
  if err != nil {
    t.Fatalf("CreateMember failed: %v", err)
  }
  originalUpdatedAt := m.UpdatedAt
  time.Sleep(1100 * time.Millisecond)

  m.Name = "Updated"
  m.Skills = []string{"go", "typescript", "react"}
  m.Model = "anthropic/claude-opus"
  err = UpdateMember(ctx, m)
  if err != nil {
    t.Fatalf("UpdateMember failed: %v", err)
  }

  got, err := GetMember(ctx, m.MemberID)
  if err != nil {
    t.Fatalf("GetMember failed: %v", err)
  }
  if got.Name != "Updated" {
    t.Errorf("expected name 'Updated', got '%s'", got.Name)
  }
  if len(got.Skills) != 3 {
    t.Errorf("expected 3 skills, got %d", len(got.Skills))
  }
  if got.Model != "anthropic/claude-opus" {
    t.Errorf("expected model 'anthropic/claude-opus', got '%s'", got.Model)
  }
  if got.UpdatedAt <= originalUpdatedAt {
    t.Errorf("UpdatedAt should be updated: was %d, now %d", originalUpdatedAt, got.UpdatedAt)
  }
}

func TestDeleteMember(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "To Delete"}
  err := CreateMember(ctx, m)
  if err != nil {
    t.Fatalf("CreateMember failed: %v", err)
  }

  err = DeleteMember(ctx, m.MemberID)
  if err != nil {
    t.Fatalf("DeleteMember failed: %v", err)
  }

  _, err = GetMember(ctx, m.MemberID)
  if err == nil {
    t.Error("expected error when getting deleted member, got nil")
  }
}

func TestDeleteMemberCascade(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "With Workers"}
  err := CreateMember(ctx, m)
  if err != nil {
    t.Fatalf("CreateMember failed: %v", err)
  }

  w := &TeamWorker{MemberID: m.MemberID, Name: "Worker 1"}
  err = CreateWorker(ctx, w)
  if err != nil {
    t.Fatalf("CreateWorker failed: %v", err)
  }

  err = DeleteMember(ctx, m.MemberID)
  if err != nil {
    t.Fatalf("DeleteMember failed: %v", err)
  }

  workers, err := ListWorkers(ctx, "")
  if err != nil {
    t.Fatalf("ListWorkers failed: %v", err)
  }
  if len(workers) != 0 {
    t.Errorf("expected 0 workers after member cascade delete, got %d", len(workers))
  }
}

func TestListMembers(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m1 := &TeamMember{Name: "Member 1", Tool: ToolClaude}
  m2 := &TeamMember{Name: "Member 2", Tool: ToolOpenCode}
  CreateMember(ctx, m1)
  time.Sleep(1100 * time.Millisecond)
  CreateMember(ctx, m2)

  members, err := ListMembers(ctx)
  if err != nil {
    t.Fatalf("ListMembers failed: %v", err)
  }
  if len(members) != 2 {
    t.Errorf("expected 2 members, got %d", len(members))
  }
  if members[0].Name != "Member 2" {
    t.Errorf("expected first member to be 'Member 2' (DESC order), got '%s'", members[0].Name)
  }
}

func TestListMembersEmpty(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  members, err := ListMembers(ctx)
  if err != nil {
    t.Fatalf("ListMembers failed: %v", err)
  }
  if len(members) != 0 {
    t.Errorf("expected 0 members, got %d", len(members))
  }
}

// --- Worker Tests ---

func TestCreateWorker(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Test Member", Tool: ToolClaude}
  CreateMember(ctx, m)

  w := &TeamWorker{MemberID: m.MemberID, Name: "Worker 1", BlockID: "block-1", TabID: "tab-1"}
  err := CreateWorker(ctx, w)
  if err != nil {
    t.Fatalf("CreateWorker failed: %v", err)
  }
  if w.WorkerID == "" {
    t.Error("WorkerID should be auto-generated")
  }
  if w.Status != WorkerStatusIdle {
    t.Errorf("expected status '%s', got '%s'", WorkerStatusIdle, w.Status)
  }
  if w.CreatedAt == 0 {
    t.Error("CreatedAt should be set")
  }
  if w.LastHeartbeat == 0 {
    t.Error("LastHeartbeat should be set")
  }
}

func TestGetWorker(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Test Member", Tool: ToolClaude}
  CreateMember(ctx, m)

  w := &TeamWorker{MemberID: m.MemberID, Name: "Test Worker", Status: WorkerStatusWorking, PID: 12345}
  err := CreateWorker(ctx, w)
  if err != nil {
    t.Fatalf("CreateWorker failed: %v", err)
  }

  got, err := GetWorker(ctx, w.WorkerID)
  if err != nil {
    t.Fatalf("GetWorker failed: %v", err)
  }
  if got.WorkerID != w.WorkerID {
    t.Errorf("expected WorkerID '%s', got '%s'", w.WorkerID, got.WorkerID)
  }
  if got.Name != "Test Worker" {
    t.Errorf("expected name 'Test Worker', got '%s'", got.Name)
  }
  if got.MemberID != m.MemberID {
    t.Errorf("expected MemberID '%s', got '%s'", m.MemberID, got.MemberID)
  }
  if got.PID != 12345 {
    t.Errorf("expected PID 12345, got %d", got.PID)
  }
}

func TestGetWorkerNotFound(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  _, err := GetWorker(ctx, "non-existent-id")
  if err == nil {
    t.Error("expected error for non-existent worker, got nil")
  }
}

func TestUpdateWorker(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Test Member", Tool: ToolClaude}
  CreateMember(ctx, m)

  w := &TeamWorker{MemberID: m.MemberID, Name: "Original", Status: WorkerStatusIdle}
  err := CreateWorker(ctx, w)
  if err != nil {
    t.Fatalf("CreateWorker failed: %v", err)
  }
  originalUpdatedAt := w.UpdatedAt
  time.Sleep(1100 * time.Millisecond)

  w.Name = "Updated Worker"
  w.Status = WorkerStatusWorking
  w.AssignedTaskID = "task-123"
  w.BlockID = "block-1"
  w.PID = 99999
  err = UpdateWorker(ctx, w)
  if err != nil {
    t.Fatalf("UpdateWorker failed: %v", err)
  }

  got, err := GetWorker(ctx, w.WorkerID)
  if err != nil {
    t.Fatalf("GetWorker failed: %v", err)
  }
  if got.Name != "Updated Worker" {
    t.Errorf("expected name 'Updated Worker', got '%s'", got.Name)
  }
  if got.Status != WorkerStatusWorking {
    t.Errorf("expected status '%s', got '%s'", WorkerStatusWorking, got.Status)
  }
  if got.AssignedTaskID != "task-123" {
    t.Errorf("expected assignedTaskId 'task-123', got '%s'", got.AssignedTaskID)
  }
  if got.PID != 99999 {
    t.Errorf("expected PID 99999, got %d", got.PID)
  }
  if got.UpdatedAt <= originalUpdatedAt {
    t.Errorf("UpdatedAt should be updated: was %d, now %d", originalUpdatedAt, got.UpdatedAt)
  }
}

func TestUpdateWorkerHeartbeat_ViaCreateWorker(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Test Member", Tool: ToolClaude}
  CreateMember(ctx, m)

  w := &TeamWorker{MemberID: m.MemberID, Name: "Worker"}
  CreateWorker(ctx, w)
  originalHB := w.LastHeartbeat
  time.Sleep(1100 * time.Millisecond)

  err := UpdateWorkerHeartbeat(ctx, w.WorkerID)
  if err != nil {
    t.Fatalf("UpdateWorkerHeartbeat failed: %v", err)
  }

  got, err := GetWorker(ctx, w.WorkerID)
  if err != nil {
    t.Fatalf("GetWorker failed: %v", err)
  }
  if got.LastHeartbeat <= originalHB {
    t.Errorf("LastHeartbeat should be updated: was %d, now %d", originalHB, got.LastHeartbeat)
  }
}

func TestDeleteWorker(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m := &TeamMember{Name: "Test Member", Tool: ToolClaude}
  CreateMember(ctx, m)

  w := &TeamWorker{MemberID: m.MemberID, Name: "To Delete"}
  CreateWorker(ctx, w)

  err := DeleteWorker(ctx, w.WorkerID)
  if err != nil {
    t.Fatalf("DeleteWorker failed: %v", err)
  }

  _, err = GetWorker(ctx, w.WorkerID)
  if err == nil {
    t.Error("expected error when getting deleted worker, got nil")
  }
}

func TestListWorkers(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  m1 := &TeamMember{Name: "Member 1", Tool: ToolClaude}
  m2 := &TeamMember{Name: "Member 2", Tool: ToolOpenCode}
  CreateMember(ctx, m1)
  CreateMember(ctx, m2)

  w1 := &TeamWorker{MemberID: m1.MemberID, Name: "Worker 1-1"}
  w2 := &TeamWorker{MemberID: m1.MemberID, Name: "Worker 1-2"}
  w3 := &TeamWorker{MemberID: m2.MemberID, Name: "Worker 2-1"}
  CreateWorker(ctx, w1)
  time.Sleep(1100 * time.Millisecond)
  CreateWorker(ctx, w2)
  time.Sleep(1100 * time.Millisecond)
  CreateWorker(ctx, w3)

  all, err := ListWorkers(ctx, "")
  if err != nil {
    t.Fatalf("ListWorkers failed: %v", err)
  }
  if len(all) != 3 {
    t.Errorf("expected 3 workers, got %d", len(all))
  }

  byMember, err := ListWorkers(ctx, m1.MemberID)
  if err != nil {
    t.Fatalf("ListWorkers by member failed: %v", err)
  }
  if len(byMember) != 2 {
    t.Errorf("expected 2 workers for member 1, got %d", len(byMember))
  }
}

func TestListWorkersEmpty(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  workers, err := ListWorkers(ctx, "")
  if err != nil {
    t.Fatalf("ListWorkers failed: %v", err)
  }
  if len(workers) != 0 {
    t.Errorf("expected 0 workers, got %d", len(workers))
  }
}

// --- Task Tests ---

func TestCreateTask(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  task := &TeamTask{Title: "Test Task"}
  err := CreateTask(ctx, task)
  if err != nil {
    t.Fatalf("CreateTask failed: %v", err)
  }
  if task.TaskID == "" {
    t.Error("TaskID should be auto-generated")
  }
  if task.Status != TaskStatusPending {
    t.Errorf("expected status '%s', got '%s'", TaskStatusPending, task.Status)
  }
  if task.Priority != PriorityMedium {
    t.Errorf("expected priority '%s', got '%s'", PriorityMedium, task.Priority)
  }
  if task.MaxRetries != 3 {
    t.Errorf("expected maxRetries 3, got %d", task.MaxRetries)
  }
  if task.CreatedAt == 0 {
    t.Error("CreatedAt should be set")
  }
}

func TestCreateTaskWithJSONFields(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  task := &TeamTask{
    Title:       "Complex Task",
    Description: "A task with dependencies and output",
    Priority:    PriorityHigh,
    DependsOn:   []string{"task-1", "task-2"},
    OutputHistory: []TaskOutput{
      {Timestamp: "2026-01-01T00:00:00Z", Type: "stdout", Content: "Hello"},
      {Timestamp: "2026-01-01T00:00:01Z", Type: "stderr", Content: "Error"},
    },
  }
  err := CreateTask(ctx, task)
  if err != nil {
    t.Fatalf("CreateTask failed: %v", err)
  }

  got, err := GetTask(ctx, task.TaskID)
  if err != nil {
    t.Fatalf("GetTask failed: %v", err)
  }
  if len(got.DependsOn) != 2 || got.DependsOn[0] != "task-1" {
    t.Errorf("expected dependsOn [task-1, task-2], got %v", got.DependsOn)
  }
  if len(got.OutputHistory) != 2 {
    t.Errorf("expected 2 output entries, got %d", len(got.OutputHistory))
  }
  if got.OutputHistory[0].Content != "Hello" {
    t.Errorf("expected output content 'Hello', got '%s'", got.OutputHistory[0].Content)
  }
  if got.Priority != PriorityHigh {
    t.Errorf("expected priority '%s', got '%s'", PriorityHigh, got.Priority)
  }
}

func TestGetTask(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  task := &TeamTask{
    Title:       "Test Task",
    Description: "Test Description",
    Priority:    PriorityHigh,
  }
  err := CreateTask(ctx, task)
  if err != nil {
    t.Fatalf("CreateTask failed: %v", err)
  }

  got, err := GetTask(ctx, task.TaskID)
  if err != nil {
    t.Fatalf("GetTask failed: %v", err)
  }
  if got.TaskID != task.TaskID {
    t.Errorf("expected TaskID '%s', got '%s'", task.TaskID, got.TaskID)
  }
  if got.Title != "Test Task" {
    t.Errorf("expected title 'Test Task', got '%s'", got.Title)
  }
  if got.Description != "Test Description" {
    t.Errorf("expected description 'Test Description', got '%s'", got.Description)
  }
}

func TestGetTaskNotFound(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  _, err := GetTask(ctx, "non-existent-id")
  if err == nil {
    t.Error("expected error for non-existent task, got nil")
  }
}

func TestGetTaskNullableFields(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  now := time.Now().Unix()
  task := &TeamTask{
    Title:       "Task with nullables",
    NextRetryAt: now + 3600,
    CompletedAt: now,
  }
  err := CreateTask(ctx, task)
  if err != nil {
    t.Fatalf("CreateTask failed: %v", err)
  }

  got, err := GetTask(ctx, task.TaskID)
  if err != nil {
    t.Fatalf("GetTask failed: %v", err)
  }
  if got.NextRetryAt == 0 {
    t.Error("NextRetryAt should be set")
  }
  if got.CompletedAt == 0 {
    t.Error("CompletedAt should be set")
  }

  // Test with null values (defaults)
  task2 := &TeamTask{Title: "Task without nullables"}
  err = CreateTask(ctx, task2)
  if err != nil {
    t.Fatalf("CreateTask failed: %v", err)
  }
  got2, _ := GetTask(ctx, task2.TaskID)
  if got2.NextRetryAt != 0 {
    t.Errorf("expected NextRetryAt 0 (null), got %d", got2.NextRetryAt)
  }
  if got2.CompletedAt != 0 {
    t.Errorf("expected CompletedAt 0 (null), got %d", got2.CompletedAt)
  }
}

func TestUpdateTask(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  task := &TeamTask{Title: "Original", Priority: PriorityLow}
  err := CreateTask(ctx, task)
  if err != nil {
    t.Fatalf("CreateTask failed: %v", err)
  }
  originalUpdatedAt := task.UpdatedAt
  time.Sleep(1100 * time.Millisecond)

  task.Title = "Updated"
  task.Priority = PriorityUrgent
  task.Status = TaskStatusWorking
  task.Progress = 50
  task.AssignedMemberID = "member-1"
  task.AssignedWorkerID = "worker-1"
  task.Result = "in progress"
  task.Error = "none yet"
  err = UpdateTask(ctx, task)
  if err != nil {
    t.Fatalf("UpdateTask failed: %v", err)
  }

  got, err := GetTask(ctx, task.TaskID)
  if err != nil {
    t.Fatalf("GetTask failed: %v", err)
  }
  if got.Title != "Updated" {
    t.Errorf("expected title 'Updated', got '%s'", got.Title)
  }
  if got.Priority != PriorityUrgent {
    t.Errorf("expected priority '%s', got '%s'", PriorityUrgent, got.Priority)
  }
  if got.Status != TaskStatusWorking {
    t.Errorf("expected status '%s', got '%s'", TaskStatusWorking, got.Status)
  }
  if got.Progress != 50 {
    t.Errorf("expected progress 50, got %d", got.Progress)
  }
  if got.Result != "in progress" {
    t.Errorf("expected result 'in progress', got '%s'", got.Result)
  }
  if got.UpdatedAt <= originalUpdatedAt {
    t.Errorf("UpdatedAt should be updated")
  }
}

func TestUpdateTaskClearNullableFields(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  now := time.Now().Unix()
  task := &TeamTask{Title: "Task", NextRetryAt: now + 3600, CompletedAt: now}
  CreateTask(ctx, task)

  task.NextRetryAt = 0
  task.CompletedAt = 0
  err := UpdateTask(ctx, task)
  if err != nil {
    t.Fatalf("UpdateTask failed: %v", err)
  }

  got, _ := GetTask(ctx, task.TaskID)
  if got.NextRetryAt != 0 {
    t.Errorf("expected NextRetryAt 0 after clearing, got %d", got.NextRetryAt)
  }
  if got.CompletedAt != 0 {
    t.Errorf("expected CompletedAt 0 after clearing, got %d", got.CompletedAt)
  }
}

func TestDeleteTask(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  task := &TeamTask{Title: "To Delete"}
  CreateTask(ctx, task)

  err := DeleteTask(ctx, task.TaskID)
  if err != nil {
    t.Fatalf("DeleteTask failed: %v", err)
  }

  _, err = GetTask(ctx, task.TaskID)
  if err == nil {
    t.Error("expected error when getting deleted task, got nil")
  }
}

func TestListTasks(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  t1 := &TeamTask{Title: "Task 1", Priority: PriorityHigh, Status: TaskStatusPending}
  t2 := &TeamTask{Title: "Task 2", Priority: PriorityLow, Status: TaskStatusWorking}
  t3 := &TeamTask{Title: "Task 3", Priority: PriorityMedium, Status: TaskStatusDone, AssignedMemberID: "m1"}
  CreateTask(ctx, t1)
  CreateTask(ctx, t2)
  CreateTask(ctx, t3)

  all, err := ListTasks(ctx, "", "", "")
  if err != nil {
    t.Fatalf("ListTasks failed: %v", err)
  }
  if len(all) != 3 {
    t.Errorf("expected 3 tasks, got %d", len(all))
  }

  byStatus, err := ListTasks(ctx, TaskStatusPending, "", "")
  if err != nil {
    t.Fatalf("ListTasks by status failed: %v", err)
  }
  if len(byStatus) != 1 {
    t.Errorf("expected 1 pending task, got %d", len(byStatus))
  }

  byPriority, err := ListTasks(ctx, "", PriorityHigh, "")
  if err != nil {
    t.Fatalf("ListTasks by priority failed: %v", err)
  }
  if len(byPriority) != 1 {
    t.Errorf("expected 1 high priority task, got %d", len(byPriority))
  }

  byMember, err := ListTasks(ctx, "", "", "m1")
  if err != nil {
    t.Fatalf("ListTasks by member failed: %v", err)
  }
  if len(byMember) != 1 {
    t.Errorf("expected 1 task for member m1, got %d", len(byMember))
  }

  combined, err := ListTasks(ctx, TaskStatusWorking, PriorityLow, "")
  if err != nil {
    t.Fatalf("ListTasks with combined filters failed: %v", err)
  }
  if len(combined) != 1 {
    t.Errorf("expected 1 working+low task, got %d", len(combined))
  }

  noMatch, err := ListTasks(ctx, TaskStatusFailed, "", "")
  if err != nil {
    t.Fatalf("ListTasks failed: %v", err)
  }
  if len(noMatch) != 0 {
    t.Errorf("expected 0 tasks with non-matching filter, got %d", len(noMatch))
  }
}

func TestListTasksEmpty(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  tasks, err := ListTasks(ctx, "", "", "")
  if err != nil {
    t.Fatalf("ListTasks failed: %v", err)
  }
  if len(tasks) != 0 {
    t.Errorf("expected 0 tasks, got %d", len(tasks))
  }
}

// --- Activity Tests ---

func TestAddActivity(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  a := &TeamActivity{
    TaskID:      "task-1",
    WorkerID:    "worker-1",
    MemberID:    "member-1",
    Type:        "forked",
    Description: "Worker forked from member",
    Meta:        `{"key":"value"}`,
  }
  err := AddActivity(ctx, a)
  if err != nil {
    t.Fatalf("AddActivity failed: %v", err)
  }
  if a.CreatedAt == 0 {
    t.Error("CreatedAt should be set")
  }
}

func TestListActivities(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  a1 := &TeamActivity{Type: "created", TaskID: "task-1", Description: "Activity 1"}
  a2 := &TeamActivity{Type: "assigned", TaskID: "task-1", Description: "Activity 2"}
  a3 := &TeamActivity{Type: "started", TaskID: "task-2", Description: "Activity 3"}
  AddActivity(ctx, a1)
  time.Sleep(10 * time.Millisecond)
  AddActivity(ctx, a2)
  time.Sleep(10 * time.Millisecond)
  AddActivity(ctx, a3)

  all, err := ListActivities(ctx, 100, "", "", "")
  if err != nil {
    t.Fatalf("ListActivities failed: %v", err)
  }
  if len(all) != 3 {
    t.Errorf("expected 3 activities, got %d", len(all))
  }
  if all[0].Description != "Activity 3" {
    t.Errorf("expected first activity to be 'Activity 3' (DESC), got '%s'", all[0].Description)
  }

  byTask, err := ListActivities(ctx, 100, "task-1", "", "")
  if err != nil {
    t.Fatalf("ListActivities by task failed: %v", err)
  }
  if len(byTask) != 2 {
    t.Errorf("expected 2 activities for task-1, got %d", len(byTask))
  }

  byWorker, err := ListActivities(ctx, 100, "", "worker-1", "")
  if err != nil {
    t.Fatalf("ListActivities by worker failed: %v", err)
  }
  if len(byWorker) != 0 {
    t.Errorf("expected 0 activities for non-existent worker, got %d", len(byWorker))
  }

  limited, err := ListActivities(ctx, 2, "", "", "")
  if err != nil {
    t.Fatalf("ListActivities with limit failed: %v", err)
  }
  if len(limited) != 2 {
    t.Errorf("expected 2 activities with limit, got %d", len(limited))
  }
}

func TestListActivitiesDefaultLimit(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  for i := 0; i < 150; i++ {
    AddActivity(ctx, &TeamActivity{Type: "test", Description: "Activity"})
  }

  activities, err := ListActivities(ctx, 0, "", "", "")
  if err != nil {
    t.Fatalf("ListActivities failed: %v", err)
  }
  if len(activities) != 100 {
    t.Errorf("expected default limit of 100, got %d", len(activities))
  }
}

func TestCleanupOldActivities(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  for i := 0; i < 5; i++ {
    AddActivity(ctx, &TeamActivity{Type: "test"})
    time.Sleep(10 * time.Millisecond)
  }

  err := CleanupOldActivities(ctx, 3)
  if err != nil {
    t.Fatalf("CleanupOldActivities failed: %v", err)
  }

  activities, err := ListActivities(ctx, 100, "", "", "")
  if err != nil {
    t.Fatalf("ListActivities failed: %v", err)
  }
  if len(activities) != 3 {
    t.Errorf("expected 3 activities after cleanup, got %d", len(activities))
  }
}

// --- Status Tests ---

func TestGetStatus(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  // Create a member
  m := &TeamMember{Name: "Member", Tool: ToolClaude}
  CreateMember(ctx, m)

  // Create workers
  w1 := &TeamWorker{MemberID: m.MemberID, Name: "W1", Status: WorkerStatusWorking}
  w2 := &TeamWorker{MemberID: m.MemberID, Name: "W2", Status: WorkerStatusIdle}
  w3 := &TeamWorker{MemberID: m.MemberID, Name: "W3", Status: WorkerStatusIdle}
  w4 := &TeamWorker{MemberID: m.MemberID, Name: "W4", Status: WorkerStatusOffline}
  CreateWorker(ctx, w1)
  CreateWorker(ctx, w2)
  CreateWorker(ctx, w3)
  CreateWorker(ctx, w4)

  // Create tasks
  CreateTask(ctx, &TeamTask{Title: "T1", Status: TaskStatusPending})
  CreateTask(ctx, &TeamTask{Title: "T2", Status: TaskStatusWorking})
  CreateTask(ctx, &TeamTask{Title: "T3", Status: TaskStatusDone})
  CreateTask(ctx, &TeamTask{Title: "T4", Status: TaskStatusFailed})
  CreateTask(ctx, &TeamTask{Title: "T5", Status: TaskStatusPaused})

  status, err := GetStatus(ctx)
  if err != nil {
    t.Fatalf("GetStatus failed: %v", err)
  }
  if status.TotalMembers != 1 {
    t.Errorf("expected 1 member, got %d", status.TotalMembers)
  }
  if status.ActiveWorkers != 1 {
    t.Errorf("expected 1 active worker, got %d", status.ActiveWorkers)
  }
  if status.IdleWorkers != 2 {
    t.Errorf("expected 2 idle workers, got %d", status.IdleWorkers)
  }
  if status.OfflineWorkers != 1 {
    t.Errorf("expected 1 offline worker, got %d", status.OfflineWorkers)
  }
  if status.PendingTasks != 1 {
    t.Errorf("expected 1 pending task, got %d", status.PendingTasks)
  }
  if status.WorkingTasks != 1 {
    t.Errorf("expected 1 working task, got %d", status.WorkingTasks)
  }
  if status.DoneTasks != 1 {
    t.Errorf("expected 1 done task, got %d", status.DoneTasks)
  }
  if status.FailedTasks != 1 {
    t.Errorf("expected 1 failed task, got %d", status.FailedTasks)
  }
  if status.PausedTasks != 1 {
    t.Errorf("expected 1 paused task, got %d", status.PausedTasks)
  }
}

func TestGetStatusEmpty(t *testing.T) {
  db := initTestDB(t)
  defer cleanupTestDB(t, db)
  ctx := context.Background()

  status, err := GetStatus(ctx)
  if err != nil {
    t.Fatalf("GetStatus failed: %v", err)
  }
  if status.TotalMembers != 0 {
    t.Errorf("expected 0 members, got %d", status.TotalMembers)
  }
  if status.ActiveWorkers != 0 {
    t.Errorf("expected 0 active workers, got %d", status.ActiveWorkers)
  }
  if status.PendingTasks != 0 {
    t.Errorf("expected 0 pending tasks, got %d", status.PendingTasks)
  }
}

func TestPublishTaskUpdate(t *testing.T) {
	PublishTaskUpdate()
}

func TestPublishWorkerUpdate(t *testing.T) {
	PublishWorkerUpdate()
}

func TestPublishMemberUpdate(t *testing.T) {
	PublishMemberUpdate()
}
