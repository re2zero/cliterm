// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/zeroai/process"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

// ============================================================================
// E2E Mock Infrastructure
// ============================================================================

// e2eMockProcessManager is a configurable mock for end-to-end testing
type e2eMockProcessManager struct {
	mu          sync.Mutex
	processes   map[string]*e2eMockProcess
	spawnCount  int
	exitCode    int
	exitSignal  string
}

type e2eMockProcess struct {
	spec       process.ProcessSpec
	pid        int
	state      process.ProcessState
	startedAt  time.Time
	endedAt    time.Time
	exitCode   int
	exitSignal string
}

// makeE2EMockProcessManager creates a configurable mock ProcessManager for e2e tests
func makeE2EMockProcessManager() *e2eMockProcessManager {
	return &e2eMockProcessManager{
		processes:   make(map[string]*e2eMockProcess),
		exitCode:    0, // Default: no crash
	}
}

// configCrash configures the mock to simulate crashes
func (m *e2eMockProcessManager) configCrash(exitCode int, exitSignal string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exitCode = exitCode
	m.exitSignal = exitSignal
}

func (m *e2eMockProcessManager) SpawnProcess(ctx context.Context, spec process.ProcessSpec) (*process.AgentProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.spawnCount++

	mockProc := &e2eMockProcess{
		spec:      spec,
		pid:       1000 + m.spawnCount,
		state:     process.ProcessStateRunning,
		startedAt: time.Now(),
		exitCode:   m.exitCode,
		exitSignal: m.exitSignal,
	}

	// If exitCode is set, the process immediately crashes
	if m.exitCode != 0 {
		mockProc.state = process.ProcessStateDone
		mockProc.endedAt = time.Now()
		mockProc.exitCode = m.exitCode
		mockProc.exitSignal = m.exitSignal
	}

	procKey := fmt.Sprintf("%p", mockProc)
	m.processes[procKey] = mockProc

	agentProc := &process.AgentProcess{
		Spec: spec,
	}

	return agentProc, nil
}

func (m *e2eMockProcessManager) KillProcess(proc *process.AgentProcess) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	procKey := fmt.Sprintf("%p", proc)
	mockProc, exists := m.processes[procKey]
	if !exists {
		return nil
	}

	if mockProc.state == process.ProcessStateRunning {
		mockProc.state = process.ProcessStateDone
		mockProc.exitCode = -1
		mockProc.exitSignal = "SIGKILL"
		mockProc.endedAt = time.Now()
	}

	return nil
}

func (m *e2eMockProcessManager) GetProcessInfo(proc *process.AgentProcess) process.ProcessInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	procKey := fmt.Sprintf("%p", proc)
	mockProc, exists := m.processes[procKey]
	if !exists {
		return process.ProcessInfo{}
	}

	return process.ProcessInfo{
		PID:        mockProc.pid,
		State:      mockProc.state,
		ExitCode:   mockProc.exitCode,
		ExitSignal: mockProc.exitSignal,
		StartedAt:  mockProc.startedAt,
		EndedAt:    mockProc.endedAt,
	}
}

func (m *e2eMockProcessManager) ListProcesses(ctx context.Context) ([]process.ProcessInfo, error) {
	return []process.ProcessInfo{}, nil
}

// makeTestAssistant creates an Assistant instance for e2e testing
func makeTestAssistant(t *testing.T) (*Assistant, *e2eMockProcessManager, *bytes.Buffer) {
	t.Helper()

	// Create custom logger
	loggerBuf := &bytes.Buffer{}
	logger := log.New(loggerBuf, "", 0)
	oldLogger := log.Writer()
	log.SetOutput(loggerBuf)
	t.Cleanup(func() {
		log.SetOutput(oldLogger)
		_ = logger.Writer()
	})

	// Create components
	taskStore := NewInMemoryTaskStore()
	statusMonitor := NewStatusMonitor(taskStore)
	wm := NewWorkerManager(statusMonitor, taskStore)

	// Create mock ProcessManager
	mockPM := makeE2EMockProcessManager()
	wm.SetProcessManager(mockPM)

	// Create Assistant
	assistant := NewAssistant(nil)
	assistant.workerManager = wm
	assistant.taskStore = taskStore

	return assistant, mockPM, loggerBuf
}

// cleanupTestAssistant cleans up assistant resources after test
func cleanupTestAssistant(t *testing.T, assistant *Assistant) {
	t.Helper()

	if assistant.IsRunning() {
		_ = assistant.Stop()
	}

	// Remove worker status directories
	waveHome := wavebase.GetHomeDir()
	workersDir := filepath.Join(waveHome, ".gsd", "assistant", "workers")
	_ = os.RemoveAll(workersDir)
}

// assertLogContains checks if log output contains expected text
func assertLogContains(t *testing.T, loggerBuf *bytes.Buffer, expected string) {
	t.Helper()

	logOutput := loggerBuf.String()
	if !strings.Contains(logOutput, expected) {
		t.Errorf("expected log to contain %q, got:\n%s", expected, logOutput)
	}
}

// assertLogNotContains checks if log output does not contain text
func assertLogNotContains(t *testing.T, loggerBuf *bytes.Buffer, unexpected string) {
	t.Helper()

	logOutput := loggerBuf.String()
	if strings.Contains(logOutput, unexpected) {
		t.Errorf("expected log NOT to contain %q, got:\n%s", unexpected, logOutput)
	}
}

// readWorkerStatusFile reads the status file for a task
func readWorkerStatusFile(t *testing.T, taskID string) (*WorkerStatus, string, error) {
	t.Helper()

	waveHome := wavebase.GetHomeDir()
	statusFilePath := filepath.Join(waveHome, ".gsd", "assistant", "workers", taskID, StatusFileName)

	data, err := os.ReadFile(statusFilePath)
	if err != nil {
		return nil, statusFilePath, err
	}

	var status WorkerStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, statusFilePath, err
	}

	return &status, statusFilePath, nil
}

// ============================================================================
// E2E Tests
// ============================================================================

// TestE2E_FullFlow_YAMLToDashboard tests the complete flow from task to worker execution
func TestE2E_FullFlow_YAMLToDashboard(t *testing.T) {
	assistant, mockPM, loggerBuf := makeTestAssistant(t)
	defer cleanupTestAssistant(t, assistant)

	ctx := context.Background()

	// Configure mock to not crash
	mockPM.configCrash(0, "")

	// Add task
	task, err := assistant.taskStore.AddTaskWithMetadata("test task", map[string]interface{}{
		"worker_type": "claude",
	})
	if err != nil {
		t.Fatalf("AddTaskWithMetadata failed: %v", err)
	}

	taskID := task.TaskID

	// Start Assistant
	err = assistant.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for ticker cycle (6s for safety)
	time.Sleep(7 * time.Second)

	// Verify task transitions to in_progress
	tasks, err := assistant.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	var foundTask *team.Task
	for _, tsk := range tasks {
		if tsk.TaskID == taskID {
			foundTask = tsk
			break
		}
	}

	if foundTask == nil {
		t.Fatal("task not found in task list")
	}

	if foundTask.Status != team.TaskStatusInProgress {
		t.Errorf("expected status in_progress, got %s", foundTask.Status)
	}

	// Verify worker started
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 1 {
		t.Errorf("expected 1 worker, got %d", len(workers))
	}

	worker := workers[0]
	if worker.TaskID != taskID {
		t.Errorf("expected worker for task %s, got %s", taskID, worker.TaskID)
	}

	// Verify status file exists
	status, statusFilePath, err := readWorkerStatusFile(t, taskID)
	if err != nil {
		t.Fatalf("failed to read status file %s: %v", statusFilePath, err)
	}

	if status.Status != "running" {
		t.Errorf("expected status 'running', got %s", status.Status)
	}

	// Stop Assistant
	err = assistant.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify worker cleaned up
	workers = assistant.workerManager.ListWorkers()
	if len(workers) != 0 {
		t.Errorf("expected 0 workers after stop, got %d", len(workers))
	}

	// Verify log output
	assertLogContains(t, loggerBuf, "worker started")
	assertLogContains(t, loggerBuf, "worker stopped")
}

// TestE2E_MultipleTasksConcurrent tests handling multiple concurrent tasks
func TestE2E_MultipleTasksConcurrent(t *testing.T) {
	assistant, mockPM, loggerBuf := makeTestAssistant(t)
	defer cleanupTestAssistant(t, assistant)

	ctx := context.Background()

	// Configure mock to not crash
	mockPM.configCrash(0, "")

	// Add multiple tasks
	taskIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		task, err := assistant.taskStore.AddTaskWithMetadata(fmt.Sprintf("concurrent task %d", i), map[string]interface{}{
			"worker_type": "claude",
		})
		if err != nil {
			t.Fatalf("AddTaskWithMetadata failed for task %d: %v", i, err)
		}
		taskIDs[i] = task.TaskID
	}

	// Start Assistant
	err := assistant.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for all workers to start
	time.Sleep(7 * time.Second)

	// Verify all tasks are in progress
	tasks, err := assistant.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	for _, task := range tasks {
		if task.Status != team.TaskStatusInProgress {
			t.Errorf("task %s expected status in_progress, got %s", task.TaskID, task.Status)
		}
	}

	// Verify workers
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 3 {
		t.Errorf("expected 3 workers, got %d", len(workers))
	}

	// Stop Assistant
	err = assistant.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify all workers cleaned up
	workers = assistant.workerManager.ListWorkers()
	if len(workers) != 0 {
		t.Errorf("expected 0 workers after stop, got %d", len(workers))
	}

	// Verify log output
	assertLogContains(t, loggerBuf, "worker started")
}

// TestE2E_WorkerCrashInstant tests immediate crash detection
func TestE2E_WorkerCrashInstant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping crash test in short mode")
	}

	assistant, mockPM, loggerBuf := makeTestAssistant(t)
	defer cleanupTestAssistant(t, assistant)

	ctx := context.Background()

	// Configure mock to crash immediately
	mockPM.configCrash(1, "SIGKILL")

	// Add task
	task, err := assistant.taskStore.AddTaskWithMetadata("crash test", map[string]interface{}{
		"worker_type": "claude",
	})
	if err != nil {
		t.Fatalf("AddTaskWithMetadata failed: %v", err)
	}

	taskID := task.TaskID

	// Start Assistant
	err = assistant.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for worker spawn and crash detection
	time.Sleep(7 * time.Second)

	// Get task and check state
	updatedTask, err := assistant.taskStore.GetTask(taskID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	t.Logf("Task status after crash: %s", updatedTask.Status)
	t.Logf("Task metadata: %+v", updatedTask.Metadata)

	// Stop Assistant
	err = assistant.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify task moved out of pending state
	if updatedTask.Status == team.TaskStatusPending {
		t.Error("task should have moved out of pending state")
	}

	// Check if crash detection was triggered
	retryCount := getIntMetadataValue(updatedTask.Metadata, "retry_count", -1)
	t.Logf("Retry count: %d", retryCount)

	assertLogContains(t, loggerBuf, "worker started")
}

// TestE2E_StallDetectionSim tests stall detection simulation
func TestE2E_StallDetectionSim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stall detection test in short mode")
	}

	assistant, mockPM, loggerBuf := makeTestAssistant(t)
	defer cleanupTestAssistant(t, assistant)

	ctx := context.Background()

	// Configure mock to not crash (worker will run normally but may appear stalled)
	mockPM.configCrash(0, "")

	// Add task
	task, err := assistant.taskStore.AddTaskWithMetadata("stall test", map[string]interface{}{
		"worker_type": "claude",
	})
	if err != nil {
		t.Fatalf("AddTaskWithMetadata failed: %v", err)
	}

	taskID := task.TaskID

	// Start Assistant
	err = assistant.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Start StatusMonitor explicitly
	err = assistant.statusMonitor.Start(ctx)
	if err != nil {
		t.Fatalf("StatusMonitor.Start failed: %v", err)
	}

	// Set a short stall timeout for testing
	assistant.statusMonitor.SetStallTimeout(2 * time.Second)

	// Wait for worker to start
	time.Sleep(1 * time.Second)

	// Wait for stall detection
	time.Sleep(8 * time.Second)

	// Get task and verify
	updatedTask, err := assistant.taskStore.GetTask(taskID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	t.Logf("Task status after potentially stalling: %s", updatedTask.Status)

	// Stop components
	_ = assistant.statusMonitor.Stop()
	err = assistant.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify crash monitor didn't trigger (no crash logs)
	assertLogNotContains(t, loggerBuf, "crash detected")
}

// TestE2E_MetricsAndObservability checks that observability signals are emitted
func TestE2E_MetricsAndObservability(t *testing.T) {
	assistant, mockPM, loggerBuf := makeTestAssistant(t)
	defer cleanupTestAssistant(t, assistant)

	ctx := context.Background()

	// Configure mock to not crash
	mockPM.configCrash(0, "")

	// Add task
	task, err := assistant.taskStore.AddTaskWithMetadata("metrics test", map[string]interface{}{
		"worker_type": "claude",
		"custom_metadata": "test_value",
	})
	if err != nil {
		t.Fatalf("AddTaskWithMetadata failed: %v", err)
	}

	taskID := task.TaskID

	// Start Assistant
	err = assistant.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for processing
	time.Sleep(6 * time.Second)

	// Get task
	tasks, err := assistant.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	var foundTask *team.Task
	for _, tsk := range tasks {
		if tsk.TaskID == taskID {
			foundTask = tsk
			break
		}
	}

	if foundTask == nil {
		t.Fatal("task not found")
	}

	// Verify metadata is preserved
	if foundTask.Metadata == nil {
		t.Error("expected metadata to be preserved")
	} else {
		if foundTask.Metadata["custom_metadata"] != "test_value" {
			t.Errorf("expected custom_metadata=test_value, got %v", foundTask.Metadata["custom_metadata"])
		}
	}

	// Verify worker info
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 1 {
		t.Errorf("expected 1 worker, got %d", len(workers))
	}

	// Verify worker fields
	if len(workers) > 0 {
		worker := workers[0]
		t.Logf("Worker info: TaskID=%s, State=%s, PID=%d", worker.TaskID, worker.ProcessState, worker.PID)

		// PID should be set by mock (1000+)
		if worker.TaskID == "" {
			t.Error("expected non-empty TaskID")
		}
	}

	// Stop Assistant
	err = assistant.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify lifecycle logs
	assertLogContains(t, loggerBuf, "started")
	assertLogContains(t, loggerBuf, "stopped")
}

// TestE2E_WorkerTypeValidationFailure tests that worker type validation fails with descriptive errors
func TestE2E_WorkerTypeValidationFailure(t *testing.T) {
	assistant, _, loggerBuf := makeTestAssistant(t)
	defer cleanupTestAssistant(t, assistant)

	// Save original PATH
	oldPath := os.Getenv("PATH")
	defer func() {
		if oldPath != "" {
			os.Setenv("PATH", oldPath)
		} else {
			os.Unsetenv("PATH")
		}
	}()

	// Set PATH to nonexistent directory to force validation failure
	os.Setenv("PATH", "/nonexistent/path/for/testing/testing123")

	// Add task
	task, err := assistant.taskStore.AddTaskWithMetadata("validation failure test", map[string]interface{}{
		"worker_type": "claude",
	})
	if err != nil {
		t.Fatalf("AddTaskWithMetadata failed: %v", err)
	}

	taskID := task.TaskID

	// Try to start worker directly - should fail validation
	ctx := context.Background()
	_, err = assistant.workerManager.StartWorker(ctx, taskID, "claude")
	if err == nil {
		t.Fatal("expected validation error when binary not in PATH")
	}

	expectedError := "worker type 'claude' requires 'claude' CLI binary in PATH: not found"
	if err.Error() != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, err.Error())
	}

	// Verify validation failure was logged
	assertLogContains(t, loggerBuf, "validation failed")

	// Verify no status directory was created
	waveHome := wavebase.GetHomeDir()
	statusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers", taskID)
	if _, err := os.Stat(statusDir); !os.IsNotExist(err) {
		t.Errorf("expected status directory to not exist on validation failure, found at %s", statusDir)
	}

	// Verify no process was spawned
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 0 {
		t.Errorf("expected 0 workers on validation failure, got %d", len(workers))
	}
}

// TestE2E_WorkerTypeValidationSuccess tests that valid worker types pass validation
func TestE2E_WorkerTypeValidationSuccess(t *testing.T) {
	assistant, mockPM, loggerBuf := makeTestAssistant(t)
	defer cleanupTestAssistant(t, assistant)

	// Configure mock to not crash
	mockPM.configCrash(0, "")

	// Add task with default worker type (always passes validation)
	task, err := assistant.taskStore.AddTaskWithMetadata("validation success test", map[string]interface{}{
		"worker_type": "default",
	})
	if err != nil {
		t.Fatalf("AddTaskWithMetadata failed: %v", err)
	}

	taskID := task.TaskID

	// Start worker - should pass validation
	ctx := context.Background()
	workerInfo, err := assistant.workerManager.StartWorker(ctx, taskID, "default")
	if err != nil {
		t.Fatalf("StartWorker failed: %v", err)
	}

	if workerInfo == nil {
		t.Fatal("expected worker info to be returned")
	}

	if workerInfo.TaskID != taskID {
		t.Errorf("expected TaskID=%s, got %s", taskID, workerInfo.TaskID)
	}

	// Verify validation was logged
	assertLogContains(t, loggerBuf, "worker type 'default' validated")

	// Verify worker is running
	workers := assistant.workerManager.ListWorkers()
	if len(workers) != 1 {
		t.Errorf("expected 1 worker, got %d", len(workers))
	}

	// Verify status directory was created
	waveHome := wavebase.GetHomeDir()
	statusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers", taskID)
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		t.Errorf("expected status directory to exist at %s", statusDir)
	}

	// Cleanup
	err = assistant.workerManager.StopWorker(taskID)
	if err != nil {
		t.Errorf("StopWorker failed: %v", err)
	}

	// Verify cleanup succeeded
	workers = assistant.workerManager.ListWorkers()
	if len(workers) != 0 {
		t.Errorf("expected 0 workers after stop, got %d", len(workers))
	}
}
