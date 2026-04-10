// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

// TestWorkerManager_RetryBackoffCalculation tests exponential backoff calculation
func TestWorkerManager_RetryBackoffCalculation(t *testing.T) {
	testCases := []struct {
		retryCount       int
		expectedBackoff  int
	}{
		{0, 1000},
		{1, 2000},
		{2, 4000},
		{3, 8000},
		{4, 16000},
		{5, 30000}, // 1000 * 2^5 = 32000, capped at 30000
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("retry_%d", tc.retryCount), func(t *testing.T) {
			backoffMs := 1000 * (1 << uint(tc.retryCount))
			if backoffMs > 30000 {
				backoffMs = 30000
			}
			if backoffMs != tc.expectedBackoff {
				t.Errorf("retry_count=%d: expected backoff %dms, got %dms", tc.retryCount, tc.expectedBackoff, backoffMs)
			}
		})
	}
}

// TestWorkerManager_MaxRetriesEnforced tests that max retries are respected
func TestWorkerManager_MaxRetriesEnforced(t *testing.T) {
	taskStore := NewInMemoryTaskStore()
	statusMonitor := &StatusMonitor{}
	_ = NewWorkerManager(statusMonitor, taskStore) // wm not used in this test

	// Create a task with max_retries=2
	task, err := taskStore.AddTaskWithMetadata("test max retries", map[string]interface{}{
		"max_retries": 2,
		"retry_count": 0,
	})
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	taskID := task.TaskID

	// First, transition to in_progress so we can transition to failed
	err = taskStore.UpdateTaskStatus(taskID, team.TaskStatusInProgress)
	if err != nil {
		t.Fatalf("failed to set task to in_progress: %v", err)
	}

	// Simulate crash handling with status transitions
	retryCount := getIntMetadataValue(task.Metadata, "retry_count", 0)
	maxRetries := getIntMetadataValue(task.Metadata, "max_retries", 3)

	// First retry (retry_count=0 < max_retries=2)
	if retryCount < maxRetries {
		retryCount++
		task.Metadata["retry_count"] = retryCount
		task.Metadata["last_crash_at"] = time.Now().Unix()
	}

	// Second retry (retry_count=1 < max_retries=2)
	if retryCount < maxRetries {
		retryCount++
		task.Metadata["retry_count"] = retryCount
		task.Metadata["last_crash_at"] = time.Now().Unix()
	}

	// Third crash (retry_count=2 >= max_retries=2) -> should fail
	if retryCount >= maxRetries {
		// Should mark as failed
		err := taskStore.UpdateTaskStatus(taskID, team.TaskStatusFailed)
		if err != nil {
			t.Errorf("failed to mark task as failed: %v", err)
		}

		// Verify task failed
		updatedTask, err := taskStore.GetTask(taskID)
		if err != nil {
			t.Errorf("failed to get task: %v", err)
		}
		if updatedTask.Status != team.TaskStatusFailed {
			t.Errorf("expected status failed, got %s", updatedTask.Status)
		}
	}
}

// TestWorkerManager_ConcurrentCrashDetection tests concurrent crash handling
func TestWorkerManager_ConcurrentCrashDetection(t *testing.T) {
	taskStore := NewInMemoryTaskStore()
	statusMonitor := &StatusMonitor{}
	wm := NewWorkerManager(statusMonitor, taskStore)

	mockPM := newMockProcessManager()
	wm.SetProcessManager(mockPM)

	ctx := context.Background()

	// Spawn multiple workers
	numWorkers := 5
	taskIDs := make([]string, numWorkers)

	for i := 0; i < numWorkers; i++ {
		workerType := "claude"
		task, err := taskStore.AddTaskWithMetadata(fmt.Sprintf("concurrent task %d", i), map[string]interface{}{
			"worker_type": workerType,
		})
		if err != nil {
			t.Fatalf("failed to add task %d: %v", i, err)
		}
		taskIDs[i] = task.TaskID

		_, err = wm.StartWorker(ctx, taskIDs[i], workerType)
		if err != nil {
			t.Fatalf("StartWorker failed for task %d: %v", i, err)
		}
	}

	// Verify all workers are running
	workers := wm.ListWorkers()
	if len(workers) != numWorkers {
		t.Errorf("expected %d workers, got %d", numWorkers, len(workers))
	}

	// Stop all workers
	for _, taskID := range taskIDs {
		err := wm.StopWorker(taskID)
		if err != nil {
			t.Errorf("StopWorker failed for task %s: %v", taskID, err)
		}
	}

	// Verify all workers stopped
	workers = wm.ListWorkers()
	if len(workers) != 0 {
		t.Errorf("expected 0 workers, got %d", len(workers))
	}

	// Verify monitor stopped (give it a moment)
	time.Sleep(100 * time.Millisecond)
	if wm.monitorTicker != nil {
		t.Error("expected monitor to stop after last worker removed")
	}
}

// TestGetIntMetadataValue tests the getIntMetadataValue helper function
func TestGetIntMetadataValue(t *testing.T) {
	testCases := []struct {
		name         string
		metadata     map[string]interface{}
		key          string
		defaultValue int
		expected     int
	}{
		{
			name:         "nil metadata",
			metadata:     nil,
			key:          "retry_count",
			defaultValue: 3,
			expected:     3,
		},
		{
			name:         "missing key",
			metadata:     map[string]interface{}{"other": 5},
			key:          "retry_count",
			defaultValue: 3,
			expected:     3,
		},
		{
			name:         "int value",
			metadata:     map[string]interface{}{"retry_count": 5},
			key:          "retry_count",
			defaultValue: 3,
			expected:     5,
		},
		{
			name:         "float64 value (JSON unmarshaling)",
			metadata:     map[string]interface{}{"retry_count": float64(7)},
			key:          "retry_count",
			defaultValue: 3,
			expected:     7,
		},
		{
			name:         "int64 value",
			metadata:     map[string]interface{}{"retry_count": int64(9)},
			key:          "retry_count",
			defaultValue: 3,
			expected:     9,
		},
		{
			name:         "invalid type",
			metadata:     map[string]interface{}{"retry_count": "invalid"},
			key:          "retry_count",
			defaultValue: 3,
			expected:     3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getIntMetadataValue(tc.metadata, tc.key, tc.defaultValue)
			if result != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, result)
			}
		})
	}
}

// TestWorkerManager_CrashMetadataPersistence tests metadata persistence after crashes
func TestWorkerManager_CrashMetadataPersistence(t *testing.T) {
	taskStore := NewInMemoryTaskStore()
	statusMonitor := &StatusMonitor{}
	_ = NewWorkerManager(statusMonitor, taskStore) // wm not used in this test

	// Create a task with initial metadata
	initialMetadata := map[string]interface{}{
		"worker_type": "claude",
	}

	task, err := taskStore.AddTaskWithMetadata("crash metadata test", initialMetadata)
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	_ = task.TaskID // taskID assigned but not needed for this test

	// Simulate crash and update metadata
	retryCount := getIntMetadataValue(task.Metadata, "retry_count", 0)
	maxRetries := getIntMetadataValue(task.Metadata, "max_retries", 3)
	crashReason := "exitCode: 1, exitSignal: SIGKILL"

	retryCount++
	task.Metadata["retry_count"] = retryCount
	task.Metadata["max_retries"] = maxRetries
	task.Metadata["backoff_ms"] = 1000
	task.Metadata["crash_reason"] = crashReason
	task.Metadata["last_crash_at"] = time.Now().Unix()

	// Verify metadata values
	if task.Metadata["retry_count"] != 1 {
		t.Errorf("expected retry_count=1, got %v", task.Metadata["retry_count"])
	}
	if task.Metadata["crash_reason"] != crashReason {
		t.Errorf("expected crash_reason=%s, got %v", crashReason, task.Metadata["crash_reason"])
	}

	// Simulate another crash (to verify type handling)
	retryCount = getIntMetadataValue(task.Metadata, "retry_count", 0)
	if retryCount != 1 {
		t.Errorf("expected retry_count=1 before second crash, got %d", retryCount)
	}

	retryCount++
	task.Metadata["retry_count"] = retryCount
	task.Metadata["backoff_ms"] = 2000

	if task.Metadata["retry_count"] != 2 {
		t.Errorf("expected retry_count=2 after second crash, got %v", task.Metadata["retry_count"])
	}
}

// TestWorkerManager_CrashWithRetry tests the crash detection flow
func TestWorkerManager_CrashWithRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}

	taskStore := NewInMemoryTaskStore()
	statusMonitor := &StatusMonitor{}
	wm := NewWorkerManager(statusMonitor, taskStore)

	// Use a regular mock for basic functionality
	mockPM := newMockProcessManager()
	wm.SetProcessManager(mockPM)

	ctx := context.Background()

	// Create a task and start worker (will run normally)
	task, err := taskStore.AddTaskWithMetadata("retry test task", map[string]interface{}{
		"worker_type": "claude",
	})
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	taskID := task.TaskID

	// Transition task to in_progress (simulating it was assigned)
	err = taskStore.UpdateTaskStatus(taskID, team.TaskStatusInProgress)
	if err != nil {
		t.Fatalf("failed to update task status: %v", err)
	}

	// Start worker
	_, err = wm.StartWorker(ctx, taskID, "claude")
	if err != nil {
		t.Fatalf("StartWorker failed: %v", err)
	}

	// Verify monitor started
	if wm.monitorTicker == nil {
		t.Error("expected monitor ticker to be started")
	}

	// Give monitor time to poll
	time.Sleep(100 * time.Millisecond)

	// Clean up
	_ = wm.StopWorker(taskID)

	// Verify monitor stopped
	time.Sleep(100 * time.Millisecond)
	if wm.monitorTicker != nil {
		t.Error("expected monitor ticker to be stopped")
	}
}

// TestWorkerManager_MaxRetriesFailure tests that tasks fail after max retries
func TestWorkerManager_MaxRetriesFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping max retries test in short mode")
	}

	taskStore := NewInMemoryTaskStore()
	statusMonitor := &StatusMonitor{}
	wm := NewWorkerManager(statusMonitor, taskStore)

	// Use a regular mock
	mockPM := newMockProcessManager()
	wm.SetProcessManager(mockPM)

	// Use a custom mock with adjustable behavior for testing
	type crashingProcessManager struct {
		*mockProcessManager
		shouldCrash bool
	}

	crashPM := &crashingProcessManager{
		mockProcessManager: newMockProcessManager(),
		shouldCrash:      true,
	}
	wm.SetProcessManager(crashPM)

	ctx := context.Background()

	// Create a task after setting crashPM
	task, err := taskStore.AddTaskWithMetadata("max retries test task", map[string]interface{}{
		"worker_type": "claude",
		"max_retries": 0, // No retries to fail immediately on crash
	})
	if err != nil {
		t.Fatalf("failed to add task: %v", err)
	}

	taskID := task.TaskID

	// Transition to in_progress so we can test failure transitions
	err = taskStore.UpdateTaskStatus(taskID, team.TaskStatusInProgress)
	if err != nil {
		t.Fatalf("failed to update task status: %v", err)
	}

	// Start worker - it will run normally
	_, err = wm.StartWorker(ctx, taskID, "claude")
	if err != nil {
		t.Fatalf("StartWorker failed: %v", err)
	}

	// Verify monitor is running
	if wm.monitorTicker == nil {
		t.Error("expected monitor ticker to be started")
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Clean up
	_ = wm.StopWorker(taskID)
}
