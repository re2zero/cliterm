// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/util/migrateutil"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
	dbfs "github.com/wavetermdev/waveterm/db"
	"github.com/jmoiron/sqlx"
)

var (
	testDB *sqlx.DB
)

func initTestDB(t *testing.T) {
	t.Helper()
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	// Use in-memory database for testing
	testDB, err = sqlx.Open("sqlite3", "file::memory:?mode=rwc&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Run migrations
	err = migrateutil.Migrate("wstore-test", testDB.DB, dbfs.WStoreMigrationFS, "migrations-wstore")
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Override the global DB for testing
	wstore.SetTestDB(testDB)
}

func cleanupTestDB(t *testing.T) {
	t.Helper()
	if testDB != nil {
		testDB.Close()
		testDB = nil
	}
}

// Helper function to create a test scheduled task
func makeTestTask(t *testing.T, taskYAML, pattern string, nextRun time.Time) *waveobj.ScheduledTask {
	t.Helper()
	return &waveobj.ScheduledTask{
		OID:       uuid.NewString(),
		TaskYAML:  taskYAML,
		NextRun:   nextRun.UnixMilli(),
		Status:    "pending",
		Pattern:   pattern,
		ExecCount: 0,
		MaxExecs:  0,
		LastRun:   0,
		Meta:      make(map[string]any),
	}
}

func TestAddScheduledTask(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()
	task := makeTestTask(t, "task: test", "daily", time.Now().Add(1*time.Hour))

	err := AddScheduledTask(ctx, task)
	if err != nil {
		t.Fatalf("failed to add scheduled task: %v", err)
	}

	// Verify task was added
	retrieved, err := GetScheduledTask(ctx, task.OID)
	if err != nil {
		t.Fatalf("failed to get scheduled task: %v", err)
	}

	if retrieved == nil {
		t.Fatal("task not found")
	}

	if retrieved.OID != task.OID {
		t.Errorf("OID mismatch: got %s, want %s", retrieved.OID, task.OID)
	}

	if retrieved.TaskYAML != task.TaskYAML {
		t.Errorf("TaskYAML mismatch: got %s, want %s", retrieved.TaskYAML, task.TaskYAML)
	}

	if retrieved.Status != task.Status {
		t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, task.Status)
	}

	if retrieved.Pattern != task.Pattern {
		t.Errorf("Pattern mismatch: got %s, want %s", retrieved.Pattern, task.Pattern)
	}
}

func TestAddScheduledTaskEmptyOID(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()
	task := &waveobj.ScheduledTask{
		OID:       "", // Empty OID
		TaskYAML:  "task: test",
		NextRun:   time.Now().Add(1 * time.Hour).UnixMilli(),
		Status:    "pending",
		Pattern:   "daily",
		ExecCount: 0,
		Meta:      make(map[string]any),
	}

	err := AddScheduledTask(ctx, task)
	if err == nil {
		t.Fatal("expected error when adding task with empty OID")
	}

	if err.Error() != "cannot add scheduled task with empty ID" {
		t.Errorf("unexpected error message: got %v", err)
	}
}

func TestGetScheduledTaskNotFound(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()
	_, err := GetScheduledTask(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error when getting nonexistent task")
	}

	if err != wstore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestListScheduledTasks(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()

	// Add multiple tasks
	task1 := makeTestTask(t, "task: test1", "daily", time.Now().Add(1*time.Hour))
	task2 := makeTestTask(t, "task: test2", "hourly", time.Now().Add(2*time.Hour))
	task3 := makeTestTask(t, "task: test3", "weekly", time.Now().Add(3*time.Hour))

	err := AddScheduledTask(ctx, task1)
	if err != nil {
		t.Fatalf("failed to add task1: %v", err)
	}

	err = AddScheduledTask(ctx, task2)
	if err != nil {
		t.Fatalf("failed to add task2: %v", err)
	}

	err = AddScheduledTask(ctx, task3)
	if err != nil {
		t.Fatalf("failed to add task3: %v", err)
	}

	// List all tasks
	tasks, err := ListScheduledTasks(ctx)
	if err != nil {
		t.Fatalf("failed to list scheduled tasks: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	// Verify all tasks are present
	taskMap := make(map[string]*waveobj.ScheduledTask)
	for _, task := range tasks {
		taskMap[task.OID] = task
	}

	if _, ok := taskMap[task1.OID]; !ok {
		t.Errorf("task1 not found in list")
	}

	if _, ok := taskMap[task2.OID]; !ok {
		t.Errorf("task2 not found in list")
	}

	if _, ok := taskMap[task3.OID]; !ok {
		t.Errorf("task3 not found in list")
	}
}

func TestGetDueScheduledTasks(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	now := time.Now()
	ctx := context.Background()

	// Add tasks at various times
	taskDue := makeTestTask(t, "task: due", "daily", now.Add(-1*time.Hour)) // Should be due
	taskFuture := makeTestTask(t, "task: future", "hourly", now.Add(1*time.Hour)) // Not due yet
	taskRunning := makeTestTask(t, "task: running", "daily", now.Add(-2*time.Hour))
	taskRunning.Status = "running" // Not pending

	taskDue.OID = uuid.NewString()
	taskFuture.OID = uuid.NewString()
	taskRunning.OID = uuid.NewString()

	err := AddScheduledTask(ctx, taskDue)
	if err != nil {
		t.Fatalf("failed to add taskDue: %v", err)
	}

	err = AddScheduledTask(ctx, taskFuture)
	if err != nil {
		t.Fatalf("failed to add taskFuture: %v", err)
	}

	err = AddScheduledTask(ctx, taskRunning)
	if err != nil {
		t.Fatalf("failed to add taskRunning: %v", err)
	}

	// Get due tasks
	dueTasks, err := GetDueScheduledTasks(ctx)
	if err != nil {
		t.Fatalf("failed to get due tasks: %v", err)
	}

	// Should only return the pending task that is due
	if len(dueTasks) != 1 {
		t.Errorf("expected 1 due task, got %d", len(dueTasks))
		for _, task := range dueTasks {
			t.Logf("due task: %s (status=%s, next_run=%d)", task.OID, task.Status, task.NextRun)
		}
	}

	if len(dueTasks) > 0 && dueTasks[0].OID != taskDue.OID {
		t.Errorf("expected taskDue, got %s", dueTasks[0].OID)
	}
}

func TestUpdateScheduledTask(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()
	task := makeTestTask(t, "task: test", "daily", time.Now().Add(1*time.Hour))

	err := AddScheduledTask(ctx, task)
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	// Update task
	task.Status = "scheduled"
	task.ExecCount = 5
	task.LastRun = time.Now().UnixMilli()

	err = UpdateScheduledTask(ctx, task)
	if err != nil {
		t.Fatalf("failed to update task: %v", err)
	}

	// Verify update
	retrieved, err := GetScheduledTask(ctx, task.OID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if retrieved.Status != "scheduled" {
		t.Errorf("expected status 'scheduled', got %s", retrieved.Status)
	}

	if retrieved.ExecCount != 5 {
		t.Errorf("expected ExecCount 5, got %d", retrieved.ExecCount)
	}

	if retrieved.LastRun != task.LastRun {
		t.Errorf("expected LastRun %d, got %d", task.LastRun, retrieved.LastRun)
	}
}

func TestUpdateScheduledTaskFn(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()
	task := makeTestTask(t, "task: test", "daily", time.Now().Add(1*time.Hour))

	err := AddScheduledTask(ctx, task)
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	// Update using function
	err = UpdateScheduledTaskFn(ctx, task.OID, func(t *waveobj.ScheduledTask) error {
		t.Status = "completed"
		t.ExecCount++
		t.LastRun = time.Now().UnixMilli()
		return nil
	})

	if err != nil {
		t.Fatalf("failed to update task with function: %v", err)
	}

	// Verify update
	retrieved, err := GetScheduledTask(ctx, task.OID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if retrieved.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", retrieved.Status)
	}

	if retrieved.ExecCount != 1 {
		t.Errorf("expected ExecCount 1, got %d", retrieved.ExecCount)
	}
}

func TestUpdateScheduledTaskFnError(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()
	task := makeTestTask(t, "task: test", "daily", time.Now().Add(1*time.Hour))

	err := AddScheduledTask(ctx, task)
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	// Update function returns error
	testErr := fmt.Errorf("test error")
	err = UpdateScheduledTaskFn(ctx, task.OID, func(t *waveobj.ScheduledTask) error {
		t.Status = "completed" // This should not be committed
		return testErr
	})

	if err != testErr {
		t.Fatalf("expected test error, got %v", err)
	}

	// Verify task was not updated
	retrieved, err := GetScheduledTask(ctx, task.OID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if retrieved.Status == "completed" {
		t.Error("task should not have been updated due to error")
	}
}

func TestDeleteScheduledTask(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	ctx := context.Background()
	task := makeTestTask(t, "task: test", "daily", time.Now().Add(1*time.Hour))

	err := AddScheduledTask(ctx, task)
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	// Delete task
	err = DeleteScheduledTask(ctx, task.OID)
	if err != nil {
		t.Fatalf("failed to delete task: %v", err)
	}

	// Verify deletion
	_, err = GetScheduledTask(ctx, task.OID)
	if err == nil {
		t.Fatal("expected error when getting deleted task")
	}

	if err != wstore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestMarkMissedOnStartup(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	now := time.Now()
	ctx := context.Background()

	// Add tasks in various states
	taskMissed1 := makeTestTask(t, "task: missed1", "daily", now.Add(-2*time.Hour))
	taskMissed2 := makeTestTask(t, "task: missed2", "hourly", now.Add(-1*time.Hour))
	taskFuture := makeTestTask(t, "task: future", "weekly", now.Add(1*time.Hour))
	taskCompleted := makeTestTask(t, "task: completed", "daily", now.Add(-3*time.Hour))
	taskCompleted.Status = "completed"

	taskMissed1.OID = uuid.NewString()
	taskMissed2.OID = uuid.NewString()
	taskFuture.OID = uuid.NewString()
	taskCompleted.OID = uuid.NewString()

	err := AddScheduledTask(ctx, taskMissed1)
	if err != nil {
		t.Fatalf("failed to add taskMissed1: %v", err)
	}

	err = AddScheduledTask(ctx, taskMissed2)
	if err != nil {
		t.Fatalf("failed to add taskMissed2: %v", err)
	}

	err = AddScheduledTask(ctx, taskFuture)
	if err != nil {
		t.Fatalf("failed to add taskFuture: %v", err)
	}

	err = AddScheduledTask(ctx, taskCompleted)
	if err != nil {
		t.Fatalf("failed to add taskCompleted: %v", err)
	}

	// Mark missed tasks
	markedCount, err := MarkMissedOnStartup(ctx)
	if err != nil {
		t.Fatalf("failed to mark missed tasks: %v", err)
	}

	if markedCount != 2 {
		t.Errorf("expected to mark 2 tasks as missed, got %d", markedCount)
	}

	// Verify tasks were marked correctly
	retrieved1, _ := GetScheduledTask(ctx, taskMissed1.OID)
	if retrieved1.Status != "missed" {
		t.Errorf("expected taskMissed1 status 'missed', got %s", retrieved1.Status)
	}

	retrieved2, _ := GetScheduledTask(ctx, taskMissed2.OID)
	if retrieved2.Status != "missed" {
		t.Errorf("expected taskMissed2 status 'missed', got %s", retrieved2.Status)
	}

	retrievedFuture, _ := GetScheduledTask(ctx, taskFuture.OID)
	if retrievedFuture.Status != "pending" {
		t.Errorf("expected taskFuture status 'pending', got %s", retrievedFuture.Status)
	}

	retrievedCompleted, _ := GetScheduledTask(ctx, taskCompleted.OID)
	if retrievedCompleted.Status != "completed" {
		t.Errorf("expected taskCompleted status 'completed', got %s", retrievedCompleted.Status)
	}
}

func TestMarkMissedOnStartupNoMissedTasks(t *testing.T) {
	initTestDB(t)
	defer cleanupTestDB(t)

	now := time.Now()
	ctx := context.Background()

	// Add only future tasks
	task1 := makeTestTask(t, "task: future1", "daily", now.Add(1*time.Hour))
	task2 := makeTestTask(t, "task: future2", "hourly", now.Add(2*time.Hour))

	err := AddScheduledTask(ctx, task1)
	if err != nil {
		t.Fatalf("failed to add task1: %v", err)
	}

	err = AddScheduledTask(ctx, task2)
	if err != nil {
		t.Fatalf("failed to add task2: %v", err)
	}

	// Mark missed tasks - should mark none
	markedCount, err := MarkMissedOnStartup(ctx)
	if err != nil {
		t.Fatalf("failed to mark missed tasks: %v", err)
	}

	if markedCount != 0 {
		t.Errorf("expected to mark 0 tasks as missed, got %d", markedCount)
	}
}
