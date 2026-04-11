// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/assistant"
	"github.com/wavetermdev/waveterm/pkg/util/migrateutil"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wstore"
	dbfs "github.com/wavetermdev/waveterm/db"
	"github.com/jmoiron/sqlx"
)

func TestSchedulerRPC(t *testing.T) {
	// Setup in-memory test database
	testDB, err := sqlx.Open("sqlite3", "file::memory:?mode=rwc&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer testDB.Close()

	// Run migrations
	err = migrateutil.Migrate("wstore-test", testDB.DB, dbfs.WStoreMigrationFS, "migrations-wstore")
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Override the global DB for testing
	wstore.SetTestDB(testDB)
	defer func() {
		wstore.SetTestDB(nil)
	}()

	// Create assistant instance
	ass := assistant.NewAssistant(nil)
	err = ass.Start(context.Background())
	if err != nil {
		t.Fatalf("failed to start assistant: %v", err)
	}
	defer ass.Stop()

	// Create scheduler instance
	sched := NewScheduler(ass)
	schedulerServer := NewWshRpcSchedulerServer(sched)

	ctx := context.Background()

	// Test 1: Add a scheduled task via RPC
	t.Run("AddScheduledTask with once pattern", func(t *testing.T) {
		taskYAML := `
description: Test task
metadata:
  key: value
`
		data := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML:     taskYAML,
			Pattern:      "once",
			FirstRunUnix: time.Now().UnixMilli() + 1000,
		}

		result, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
		if err != nil {
			t.Fatalf("SchedulerAddScheduledTaskCommand failed: %v", err)
		}

		if result.TaskID == "" {
			t.Fatal("TaskID should not be empty")
		}

		t.Logf("Created scheduled task: %s", result.TaskID)
	})

	// Test 2: Add a scheduled task with repeat pattern
	t.Run("AddScheduledTask with repeat pattern", func(t *testing.T) {
		taskYAML := `
description: Repeat task
metadata:
  worker_type: test-worker
`
		data := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML:       taskYAML,
			Pattern:        "repeat",
			MaxExecs:       3,
			FirstRunUnix:   time.Now().UnixMilli() + 1000,
		}

		result, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
		if err != nil {
			t.Fatalf("SchedulerAddScheduledTaskCommand failed: %v", err)
		}

		if result.TaskID == "" {
			t.Fatal("TaskID should not be empty")
		}

		t.Logf("Created repeat scheduled task: %s", result.TaskID)
	})

	// Test 3: Add a scheduled task with default firstRunUnix
	t.Run("AddScheduledTask with default firstRunUnix", func(t *testing.T) {
		taskYAML := `
description: Default first run task
`
		data := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML: taskYAML,
			Pattern:  "daily",
		}

		result, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
		if err != nil {
			t.Fatalf("SchedulerAddScheduledTaskCommand failed: %v", err)
		}

		if result.TaskID == "" {
			t.Fatal("TaskID should not be empty")
		}

		t.Logf("Created scheduled task with default firstRunUnix: %s", result.TaskID)
	})

	// Test 4: Validation - empty TaskYAML should fail
	t.Run("AddScheduledTask with empty TaskYAML fails", func(t *testing.T) {
		data := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML: "",
			Pattern:  "once",
		}

		_, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
		if err == nil {
			t.Fatal("Expected error for empty TaskYAML")
		}

		if err.Error() != "TaskYAML cannot be empty" {
			t.Errorf("Expected 'TaskYAML cannot be empty' error, got: %v", err)
		}
	})

	// Test 5: Validation - invalid YAML should fail
	t.Run("AddScheduledTask with invalid YAML fails", func(t *testing.T) {
		data := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML: "invalid yaml: [",
			Pattern:  "once",
		}

		_, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
		if err == nil {
			t.Fatal("Expected error for invalid YAML")
		}

		if err.Error()[:12] != "invalid YAML" {
			t.Errorf("Expected 'invalid YAML' error, got: %v", err)
		}
	})

	// Test 6: Validation - missing description should fail
	t.Run("AddScheduledTask with missing description fails", func(t *testing.T) {
		data := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML: "metadata:\n  key: value",
			Pattern:  "once",
		}

		_, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
		if err == nil {
			t.Fatal("Expected error for missing description")
		}

		if err.Error() != "YAML must contain a description field" {
			t.Errorf("Expected 'YAML must contain a description field' error, got: %v", err)
		}
	})

	// Test 7: Validation - invalid pattern should fail
	t.Run("AddScheduledTask with invalid pattern fails", func(t *testing.T) {
		taskYAML := `
description: Test task
`
		data := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML: taskYAML,
			Pattern:  "invalid",
		}

		_, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
		if err == nil {
			t.Fatal("Expected error for invalid pattern")
		}

		if err.Error()[:15] != "invalid pattern" {
			t.Errorf("Expected 'invalid pattern' error, got: %v", err)
		}
	})

	// Test 8: Test all valid patterns
	t.Run("AddScheduledTask with all valid patterns", func(t *testing.T) {
		patterns := []string{"once", "daily", "hourly", "weekly", "repeat"}

		for _, pattern := range patterns {
			taskYAML := `
description: Test ` + pattern + ` task
`
			data := wshrpc.CommandSchedulerAddScheduledTaskData{
				TaskYAML:     taskYAML,
				Pattern:      pattern,
				FirstRunUnix: time.Now().UnixMilli() + 1000,
			}

			result, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, data)
			if err != nil {
				t.Fatalf("SchedulerAddScheduledTaskCommand failed for pattern %s: %v", pattern, err)
			}

			if result.TaskID == "" {
				t.Fatalf("TaskID should not be empty for pattern %s", pattern)
			}

			t.Logf("Created task with pattern '%s': %s", pattern, result.TaskID)
		}
	})

	// Test 9: List tasks
	t.Run("ListTasks returns tasks", func(t *testing.T) {
		data := wshrpc.CommandSchedulerListTasksData{}

		result, err := schedulerServer.SchedulerListTasksCommand(ctx, data)
		if err != nil {
			t.Fatalf("SchedulerListTasksCommand failed: %v", err)
		}

		if len(result.Tasks) == 0 {
			t.Error("Expected at least one task")
		}

		// Verify task structure
		for _, task := range result.Tasks {
			if task.OID == "" {
				t.Error("Task OID should not be empty")
			}
			if task.TaskYAML == "" {
				t.Error("TaskYAML should not be empty")
			}
			if task.Pattern == "" {
				t.Error("Pattern should not be empty")
			}
			if task.Status == "" {
				t.Error("Status should not be empty")
			}
		}

		t.Logf("Listed %d tasks", len(result.Tasks))
	})

	// Test 10: Delete a task
	t.Run("DeleteTask removes task", func(t *testing.T) {
		// First create a task
		taskYAML := `
description: Task to delete
`
		createData := wshrpc.CommandSchedulerAddScheduledTaskData{
			TaskYAML:     taskYAML,
			Pattern:      "once",
			FirstRunUnix: time.Now().UnixMilli() + 1000,
		}

		createResult, err := schedulerServer.SchedulerAddScheduledTaskCommand(ctx, createData)
		if err != nil {
			t.Fatalf("Failed to create task for deletion test: %v", err)
		}

		// Now delete it
		deleteData := wshrpc.CommandSchedulerDeleteTaskData{
			TaskID: createResult.TaskID,
		}

		err = schedulerServer.SchedulerDeleteTaskCommand(ctx, deleteData)
		if err != nil {
			t.Fatalf("SchedulerDeleteTaskCommand failed: %v", err)
		}

		// Verify task is gone
		tasks, err := ListScheduledTasks(ctx)
		if err != nil {
			t.Fatalf("Failed to get tasks for verification: %v", err)
		}

		for _, task := range tasks {
			if task.OID == createResult.TaskID {
				t.Fatal("Task should have been deleted")
			}
		}

		t.Logf("Deleted task: %s", createResult.TaskID)
	})

	// Test 11: Delete non-existent task
	t.Run("DeleteTask with non-existent task ID is idempotent", func(t *testing.T) {
		deleteData := wshrpc.CommandSchedulerDeleteTaskData{
			TaskID: "non-existent-task-id",
		}

		// For MVP, deletion is idempotent - no error for non-existent tasks
		err := schedulerServer.SchedulerDeleteTaskCommand(ctx, deleteData)
		if err != nil {
			t.Errorf("Expected no error for deleting non-existent task, got: %v", err)
		}
	})

	// Test 12: Delete with empty task ID
	t.Run("DeleteTask with empty task ID fails", func(t *testing.T) {
		deleteData := wshrpc.CommandSchedulerDeleteTaskData{
			TaskID: "",
		}

		err := schedulerServer.SchedulerDeleteTaskCommand(ctx, deleteData)
		if err == nil {
			t.Fatal("Expected error for empty task ID")
		}

		if err.Error() != "task ID cannot be empty" {
			t.Errorf("Expected 'task ID cannot be empty' error, got: %v", err)
		}
	})
}
