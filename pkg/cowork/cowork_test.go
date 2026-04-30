// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cowork

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wstore"
	dbfs "github.com/wavetermdev/waveterm/db"
	"github.com/wavetermdev/waveterm/pkg/util/migrateutil"
)

func initTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	err = migrateutil.Migrate("wstore", db.DB, dbfs.WStoreMigrationFS, "migrations-wstore")
	if err != nil {
		db.Close()
		t.Fatalf("failed to apply migrations: %v", err)
	}
	wstore.SetGlobalDBForTest(db)
	return db
}

func cleanupTestDB(t *testing.T, db *sqlx.DB) {
	t.Helper()
	wstore.SetGlobalDBForTest(nil)
	err := db.Close()
	if err != nil {
		t.Fatalf("failed to close db: %v", err)
	}
}

func TestCreateTask(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &wshrpc.CoworkTask{
		Title: "Test Task",
	}
	err := CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.TaskId == "" {
		t.Error("TaskId should be auto-generated")
	}
	if task.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", task.Status)
	}
	if task.Priority != "medium" {
		t.Errorf("expected priority 'medium', got '%s'", task.Priority)
	}
	if task.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if task.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set")
	}
}

func TestGetTask(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &wshrpc.CoworkTask{
		Title:       "Test Task",
		Description: "Test Description",
		Priority:    "high",
	}
	err := CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	got, err := GetTask(ctx, task.TaskId)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.TaskId != task.TaskId {
		t.Errorf("expected TaskId '%s', got '%s'", task.TaskId, got.TaskId)
	}
	if got.Title != "Test Task" {
		t.Errorf("expected title 'Test Task', got '%s'", got.Title)
	}
	if got.Description != "Test Description" {
		t.Errorf("expected description 'Test Description', got '%s'", got.Description)
	}
	if got.Priority != "high" {
		t.Errorf("expected priority 'high', got '%s'", got.Priority)
	}
	if got.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", got.Status)
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

func TestUpdateTask(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &wshrpc.CoworkTask{
		Title:    "Original Title",
		Priority: "low",
		Status:   "pending",
	}
	err := CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	originalUpdatedAt := task.UpdatedAt
	time.Sleep(1100 * time.Millisecond)

	task.Title = "Updated Title"
	task.Priority = "high"
	task.Status = "working"
	err = UpdateTask(ctx, task)
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	got, err := GetTask(ctx, task.TaskId)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got '%s'", got.Title)
	}
	if got.Priority != "high" {
		t.Errorf("expected priority 'high', got '%s'", got.Priority)
	}
	if got.Status != "working" {
		t.Errorf("expected status 'working', got '%s'", got.Status)
	}
	if got.UpdatedAt <= originalUpdatedAt {
		t.Errorf("UpdatedAt should be updated: was %d, now %d", originalUpdatedAt, got.UpdatedAt)
	}
}

func TestDeleteTask(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &wshrpc.CoworkTask{
		Title: "Task to Delete",
	}
	err := CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	err = DeleteTask(ctx, task.TaskId)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	_, err = GetTask(ctx, task.TaskId)
	if err == nil {
		t.Error("expected error when getting deleted task, got nil")
	}
}

func TestListTasks(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task1 := &wshrpc.CoworkTask{Title: "Task 1", Priority: "high", Status: "pending"}
	task2 := &wshrpc.CoworkTask{Title: "Task 2", Priority: "low", Status: "working"}
	task3 := &wshrpc.CoworkTask{Title: "Task 3", Priority: "medium", Status: "done"}
	
	err := CreateTask(ctx, task1)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	err = CreateTask(ctx, task2)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	err = CreateTask(ctx, task3)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	tasks, err := ListTasks(ctx, "", "")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	pendingTasks, err := ListTasks(ctx, "pending", "")
	if err != nil {
		t.Fatalf("ListTasks with status filter failed: %v", err)
	}
	if len(pendingTasks) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(pendingTasks))
	}

	highTasks, err := ListTasks(ctx, "", "high")
	if err != nil {
		t.Fatalf("ListTasks with priority filter failed: %v", err)
	}
	if len(highTasks) != 1 {
		t.Errorf("expected 1 high priority task, got %d", len(highTasks))
	}
}

func TestListTasksEmpty(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	tasks, err := ListTasks(ctx, "", "")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestRegisterWorker(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	worker := &wshrpc.CoworkWorker{
		Name:    "Test Worker",
		Tool:    "claude",
		BlockId: "block-1",
		TabId:   "tab-1",
	}
	err := RegisterWorker(ctx, worker)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}
	if worker.WorkerId == "" {
		t.Error("WorkerId should be auto-generated")
	}
	if worker.Status != "idle" {
		t.Errorf("expected status 'idle', got '%s'", worker.Status)
	}
	if worker.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if worker.LastActiveAt == 0 {
		t.Error("LastActiveAt should be set")
	}
}

func TestGetWorker(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	worker := &wshrpc.CoworkWorker{
		Name:         "Test Worker",
		Tool:         "claude",
		Role:         "developer",
		Desc:         "A test worker",
		Soul:         "test-soul",
		Skills:       "go,typescript",
		McpServers:   "server1,server2",
		BlockId:      "block-1",
		TabId:        "tab-1",
		Capabilities: "read,write",
		Concurrency:  3,
		Timeout:      60,
		MaxRetries:   5,
	}
	err := RegisterWorker(ctx, worker)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	got, err := GetWorker(ctx, worker.WorkerId)
	if err != nil {
		t.Fatalf("GetWorker failed: %v", err)
	}
	if got.WorkerId != worker.WorkerId {
		t.Errorf("expected WorkerId '%s', got '%s'", worker.WorkerId, got.WorkerId)
	}
	if got.Name != "Test Worker" {
		t.Errorf("expected name 'Test Worker', got '%s'", got.Name)
	}
	if got.Tool != "claude" {
		t.Errorf("expected tool 'claude', got '%s'", got.Tool)
	}
	if got.Role != "developer" {
		t.Errorf("expected role 'developer', got '%s'", got.Role)
	}
	if got.Desc != "A test worker" {
		t.Errorf("expected description 'A test worker', got '%s'", got.Desc)
	}
	if got.Soul != "test-soul" {
		t.Errorf("expected soul 'test-soul', got '%s'", got.Soul)
	}
	if got.Skills != "go,typescript" {
		t.Errorf("expected skills 'go,typescript', got '%s'", got.Skills)
	}
	if got.McpServers != "server1,server2" {
		t.Errorf("expected mcpServers 'server1,server2', got '%s'", got.McpServers)
	}
	if got.Capabilities != "read,write" {
		t.Errorf("expected capabilities 'read,write', got '%s'", got.Capabilities)
	}
	if got.Concurrency != 3 {
		t.Errorf("expected concurrency 3, got %d", got.Concurrency)
	}
	if got.Timeout != 60 {
		t.Errorf("expected timeout 60, got %d", got.Timeout)
	}
	if got.MaxRetries != 5 {
		t.Errorf("expected maxRetries 5, got %d", got.MaxRetries)
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

	worker := &wshrpc.CoworkWorker{
		Name:    "Original Name",
		Tool:    "claude",
		Status:  "idle",
		BlockId: "block-1",
		TabId:   "tab-1",
	}
	err := RegisterWorker(ctx, worker)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}
	originalLastActiveAt := worker.LastActiveAt
	time.Sleep(1100 * time.Millisecond)

	worker.Name = "Updated Name"
	worker.Tool = "custom"
	worker.CustomCmd = "custom-command"
	worker.Role = "senior"
	worker.Desc = "Updated description"
	worker.Soul = "updated-soul"
	worker.Skills = "python,rust"
	worker.McpServers = "server3"
	worker.Status = "working"
	worker.AssignedTask = "task-123"
	worker.LastOutputHash = "hash-456"
	worker.ErrorMsg = "test error"
	worker.Capabilities = "execute"
	worker.Concurrency = 5
	worker.Timeout = 120
	worker.MaxRetries = 10

	time.Sleep(100 * time.Millisecond)
	err = UpdateWorker(ctx, worker)
	if err != nil {
		t.Fatalf("UpdateWorker failed: %v", err)
	}

	got, err := GetWorker(ctx, worker.WorkerId)
	if err != nil {
		t.Fatalf("GetWorker failed: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", got.Name)
	}
	if got.Tool != "custom" {
		t.Errorf("expected tool 'custom', got '%s'", got.Tool)
	}
	if got.CustomCmd != "custom-command" {
		t.Errorf("expected customCmd 'custom-command', got '%s'", got.CustomCmd)
	}
	if got.Role != "senior" {
		t.Errorf("expected role 'senior', got '%s'", got.Role)
	}
	if got.Desc != "Updated description" {
		t.Errorf("expected description 'Updated description', got '%s'", got.Desc)
	}
	if got.Soul != "updated-soul" {
		t.Errorf("expected soul 'updated-soul', got '%s'", got.Soul)
	}
	if got.Skills != "python,rust" {
		t.Errorf("expected skills 'python,rust', got '%s'", got.Skills)
	}
	if got.McpServers != "server3" {
		t.Errorf("expected mcpServers 'server3', got '%s'", got.McpServers)
	}
	if got.Status != "working" {
		t.Errorf("expected status 'working', got '%s'", got.Status)
	}
	if got.AssignedTask != "task-123" {
		t.Errorf("expected assignedTask 'task-123', got '%s'", got.AssignedTask)
	}
	if got.LastOutputHash != "hash-456" {
		t.Errorf("expected lastOutputHash 'hash-456', got '%s'", got.LastOutputHash)
	}
	if got.ErrorMsg != "test error" {
		t.Errorf("expected errorMsg 'test error', got '%s'", got.ErrorMsg)
	}
	if got.Capabilities != "execute" {
		t.Errorf("expected capabilities 'execute', got '%s'", got.Capabilities)
	}
	if got.Concurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", got.Concurrency)
	}
	if got.Timeout != 120 {
		t.Errorf("expected timeout 120, got %d", got.Timeout)
	}
	if got.MaxRetries != 10 {
		t.Errorf("expected maxRetries 10, got %d", got.MaxRetries)
	}
	if got.LastActiveAt <= originalLastActiveAt {
		t.Errorf("LastActiveAt should be updated: was %d, now %d", originalLastActiveAt, got.LastActiveAt)
	}
}

func TestDeleteWorker(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	worker := &wshrpc.CoworkWorker{
		Name:    "Worker to Delete",
		Tool:    "claude",
		BlockId: "block-1",
		TabId:   "tab-1",
	}
	err := RegisterWorker(ctx, worker)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	err = DeleteWorker(ctx, worker.WorkerId)
	if err != nil {
		t.Fatalf("DeleteWorker failed: %v", err)
	}

	_, err = GetWorker(ctx, worker.WorkerId)
	if err == nil {
		t.Error("expected error when getting deleted worker, got nil")
	}
}

func TestListWorkers(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	worker1 := &wshrpc.CoworkWorker{Name: "Worker 1", Tool: "claude", BlockId: "block-1", TabId: "tab-1"}
	worker2 := &wshrpc.CoworkWorker{Name: "Worker 2", Tool: "claude", BlockId: "block-2", TabId: "tab-2"}
	worker3 := &wshrpc.CoworkWorker{Name: "Worker 3", Tool: "claude", BlockId: "block-3", TabId: "tab-3"}

	err := RegisterWorker(ctx, worker1)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	err = RegisterWorker(ctx, worker2)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	err = RegisterWorker(ctx, worker3)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	workers, err := ListWorkers(ctx)
	if err != nil {
		t.Fatalf("ListWorkers failed: %v", err)
	}
	if len(workers) != 3 {
		t.Errorf("expected 3 workers, got %d", len(workers))
	}
	if workers[0].Name != "Worker 3" {
		t.Errorf("expected first worker to be 'Worker 3', got '%s'", workers[0].Name)
	}
	if workers[2].Name != "Worker 1" {
		t.Errorf("expected last worker to be 'Worker 1', got '%s'", workers[2].Name)
	}
}

func TestAddActivity(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	activity := &wshrpc.CoworkActivity{
		TaskId:      "task-123",
		WorkerId:    "worker-456",
		Type:        "test",
		Description: "Test activity",
		Meta:        `{"key":"value"}`,
	}
	err := AddActivity(ctx, activity)
	if err != nil {
		t.Fatalf("AddActivity failed: %v", err)
	}
	if activity.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}

	activities, err := ListActivities(ctx, 10)
	if err != nil {
		t.Fatalf("ListActivities failed: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].Id == 0 {
		t.Error("Id should be auto-generated")
	}
	if activities[0].TaskId != "task-123" {
		t.Errorf("expected TaskId 'task-123', got '%s'", activities[0].TaskId)
	}
	if activities[0].WorkerId != "worker-456" {
		t.Errorf("expected WorkerId 'worker-456', got '%s'", activities[0].WorkerId)
	}
}

func TestListActivities(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	activity1 := &wshrpc.CoworkActivity{Type: "test1", Description: "Activity 1"}
	activity2 := &wshrpc.CoworkActivity{Type: "test2", Description: "Activity 2"}
	activity3 := &wshrpc.CoworkActivity{Type: "test3", Description: "Activity 3"}

	err := AddActivity(ctx, activity1)
	if err != nil {
		t.Fatalf("AddActivity failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = AddActivity(ctx, activity2)
	if err != nil {
		t.Fatalf("AddActivity failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = AddActivity(ctx, activity3)
	if err != nil {
		t.Fatalf("AddActivity failed: %v", err)
	}

	activities, err := ListActivities(ctx, 100)
	if err != nil {
		t.Fatalf("ListActivities failed: %v", err)
	}
	if len(activities) != 3 {
		t.Errorf("expected 3 activities, got %d", len(activities))
	}
	if activities[0].Description != "Activity 3" {
		t.Errorf("expected first activity to be 'Activity 3', got '%s'", activities[0].Description)
	}

	limited, err := ListActivities(ctx, 2)
	if err != nil {
		t.Fatalf("ListActivities with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("expected 2 activities with limit, got %d", len(limited))
	}
}

func TestCleanupOldActivities(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		activity := &wshrpc.CoworkActivity{
			Type:        "test",
			Description: "Activity",
		}
		err := AddActivity(ctx, activity)
		if err != nil {
			t.Fatalf("AddActivity failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	err := CleanupOldActivities(ctx, 3)
	if err != nil {
		t.Fatalf("CleanupOldActivities failed: %v", err)
	}

	activities, err := ListActivities(ctx, 100)
	if err != nil {
		t.Fatalf("ListActivities failed: %v", err)
	}
	if len(activities) != 3 {
		t.Errorf("expected 3 activities after cleanup, got %d", len(activities))
	}
}

func TestGetStatus(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task1 := &wshrpc.CoworkTask{Title: "Task 1", Status: "pending"}
	task2 := &wshrpc.CoworkTask{Title: "Task 2", Status: "working"}
	task3 := &wshrpc.CoworkTask{Title: "Task 3", Status: "done"}
	task4 := &wshrpc.CoworkTask{Title: "Task 4", Status: "failed"}
	CreateTask(ctx, task1)
	CreateTask(ctx, task2)
	CreateTask(ctx, task3)
	CreateTask(ctx, task4)

	worker1 := &wshrpc.CoworkWorker{Name: "Worker 1", Tool: "claude", Status: "working", BlockId: "block-1", TabId: "tab-1"}
	worker2 := &wshrpc.CoworkWorker{Name: "Worker 2", Tool: "claude", Status: "idle", BlockId: "block-2", TabId: "tab-2"}
	worker3 := &wshrpc.CoworkWorker{Name: "Worker 3", Tool: "claude", Status: "idle", BlockId: "block-3", TabId: "tab-3"}
	RegisterWorker(ctx, worker1)
	RegisterWorker(ctx, worker2)
	RegisterWorker(ctx, worker3)

	status, err := GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
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
	if status.ActiveWorkers != 1 {
		t.Errorf("expected 1 active worker, got %d", status.ActiveWorkers)
	}
	if status.IdleWorkers != 2 {
		t.Errorf("expected 2 idle workers, got %d", status.IdleWorkers)
	}
}

func TestCreateTaskAllFields(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &wshrpc.CoworkTask{
		Title:          "Full Task",
		Description:    "Complete description",
		Priority:       "high",
		Status:         "pending",
		AssignedWorker: "worker-123",
		Result:         "success",
		Error:          "none",
		Progress:       "50%",
	}
	err := CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	got, err := GetTask(ctx, task.TaskId)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Description != "Complete description" {
		t.Errorf("expected description 'Complete description', got '%s'", got.Description)
	}
	if got.Priority != "high" {
		t.Errorf("expected priority 'high', got '%s'", got.Priority)
	}
	if got.AssignedWorker != "worker-123" {
		t.Errorf("expected assignedWorker 'worker-123', got '%s'", got.AssignedWorker)
	}
	if got.Result != "success" {
		t.Errorf("expected result 'success', got '%s'", got.Result)
	}
	if got.Error != "none" {
		t.Errorf("expected error 'none', got '%s'", got.Error)
	}
	if got.Progress != "50%" {
		t.Errorf("expected progress '50%%', got '%s'", got.Progress)
	}
}

func TestRegisterWorkerWithCustomTool(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	worker := &wshrpc.CoworkWorker{
		Name:      "Custom Worker",
		Tool:      "custom",
		CustomCmd: "my-custom-tool --arg",
		BlockId:   "block-1",
		TabId:     "tab-1",
	}
	err := RegisterWorker(ctx, worker)
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	got, err := GetWorker(ctx, worker.WorkerId)
	if err != nil {
		t.Fatalf("GetWorker failed: %v", err)
	}
	if got.Tool != "custom" {
		t.Errorf("expected tool 'custom', got '%s'", got.Tool)
	}
	if got.CustomCmd != "my-custom-tool --arg" {
		t.Errorf("expected customCmd 'my-custom-tool --arg', got '%s'", got.CustomCmd)
	}
}

func TestUpdateTaskStatusTransition(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &wshrpc.CoworkTask{
		Title:  "Task to Complete",
		Status: "working",
	}
	err := CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	task.Status = "done"
	task.CompletedAt = time.Now().Unix()
	err = UpdateTask(ctx, task)
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	got, err := GetTask(ctx, task.TaskId)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != "done" {
		t.Errorf("expected status 'done', got '%s'", got.Status)
	}
	if got.CompletedAt == 0 {
		t.Error("CompletedAt should be set")
	}
}

func TestListTasksFilterByStatus(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task1 := &wshrpc.CoworkTask{Title: "Task 1", Status: "pending", Priority: "high"}
	task2 := &wshrpc.CoworkTask{Title: "Task 2", Status: "pending", Priority: "low"}
	task3 := &wshrpc.CoworkTask{Title: "Task 3", Status: "working", Priority: "high"}
	CreateTask(ctx, task1)
	CreateTask(ctx, task2)
	CreateTask(ctx, task3)

	tasks, err := ListTasks(ctx, "pending", "")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(tasks))
	}
}

func TestListTasksFilterByPriority(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task1 := &wshrpc.CoworkTask{Title: "Task 1", Status: "pending", Priority: "high"}
	task2 := &wshrpc.CoworkTask{Title: "Task 2", Status: "working", Priority: "high"}
	task3 := &wshrpc.CoworkTask{Title: "Task 3", Status: "done", Priority: "low"}
	CreateTask(ctx, task1)
	CreateTask(ctx, task2)
	CreateTask(ctx, task3)

	tasks, err := ListTasks(ctx, "", "high")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 high priority tasks, got %d", len(tasks))
	}
}

func TestListTasksEmptyWithFilters(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task := &wshrpc.CoworkTask{Title: "Task", Status: "pending", Priority: "high"}
	CreateTask(ctx, task)

	tasks, err := ListTasks(ctx, "done", "")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks with non-matching filter, got %d", len(tasks))
	}

	tasks, err = ListTasks(ctx, "", "low")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks with non-matching priority filter, got %d", len(tasks))
	}
}

func TestListTasksBothFilters(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	task1 := &wshrpc.CoworkTask{Title: "Task 1", Status: "pending", Priority: "high"}
	task2 := &wshrpc.CoworkTask{Title: "Task 2", Status: "pending", Priority: "low"}
	task3 := &wshrpc.CoworkTask{Title: "Task 3", Status: "working", Priority: "high"}
	CreateTask(ctx, task1)
	CreateTask(ctx, task2)
	CreateTask(ctx, task3)

	tasks, err := ListTasks(ctx, "pending", "high")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 pending+high task, got %d", len(tasks))
	}
}

func TestListWorkersEmpty(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	workers, err := ListWorkers(ctx)
	if err != nil {
		t.Fatalf("ListWorkers failed: %v", err)
	}
	if len(workers) != 0 {
		t.Errorf("expected 0 workers, got %d", len(workers))
	}
}

func TestListActivitiesEmpty(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	activities, err := ListActivities(ctx, 100)
	if err != nil {
		t.Fatalf("ListActivities failed: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("expected 0 activities, got %d", len(activities))
	}
}

func TestListActivitiesDefaultLimit(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	for i := 0; i < 150; i++ {
		activity := &wshrpc.CoworkActivity{
			Type:        "test",
			Description: "Activity",
		}
		err := AddActivity(ctx, activity)
		if err != nil {
			t.Fatalf("AddActivity failed: %v", err)
		}
	}

	activities, err := ListActivities(ctx, 0)
	if err != nil {
		t.Fatalf("ListActivities failed: %v", err)
	}
	if len(activities) != 100 {
		t.Errorf("expected default limit of 100 activities, got %d", len(activities))
	}
}

func TestListActivitiesNegativeLimit(t *testing.T) {
	db := initTestDB(t)
	defer cleanupTestDB(t, db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		activity := &wshrpc.CoworkActivity{
			Type:        "test",
			Description: "Activity",
		}
		err := AddActivity(ctx, activity)
		if err != nil {
			t.Fatalf("AddActivity failed: %v", err)
		}
	}

	activities, err := ListActivities(ctx, -5)
	if err != nil {
		t.Fatalf("ListActivities failed: %v", err)
	}
	if len(activities) != 10 {
		t.Errorf("expected 10 activities with negative limit (defaults to 100), got %d", len(activities))
	}
}
