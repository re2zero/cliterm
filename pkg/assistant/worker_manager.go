// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wavetermdev/waveterm/pkg/zeroai/process"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

const (
	// DefaultMaxConcurrentWorkers is the default maximum number of concurrent workers
	DefaultMaxConcurrentWorkers = 10
)

// WorkerInfo contains information about a worker
type WorkerInfo struct {
	TaskID       string
	WorkerType   string
	ProcessState process.ProcessState
	PID          int
	BlockID      string
	StartedAt    time.Time
	StatusDir    string
}

// WorkerManager manages worker processes and blocks
type WorkerManager struct {
	statusMonitor          *StatusMonitor
	blockManager           *team.BlockManager
	processManager         process.ProcessManager
	taskStore              TaskStore
	processes              map[string]*process.AgentProcess // taskID -> process
	processMetadata        map[string]*ProcessMetadata      // taskID -> metadata (blockID, etc.)
	mu                     sync.RWMutex
	maxConcurrentWorkers   int
	statusDir              string // Root directory for worker status files
	defaultTabID           string // Default tab ID for spawning blocks
	// Crash monitor fields
	monitorTicker    *time.Ticker
	monitorStopped   chan struct{}
	monitorMu        sync.Mutex // Protects monitor loop lifecycle
}

// ProcessMetadata tracks additional metadata for a worker process
type ProcessMetadata struct {
	TaskID     string
	WorkerType string
	BlockID    string
	StatusDir  string
	StartedAt  time.Time
}

// NewWorkerManager creates a new WorkerManager instance
func NewWorkerManager(statusMonitor *StatusMonitor, taskStore TaskStore) *WorkerManager {
	waveHome := wavebase.GetHomeDir()
	statusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers")

	return &WorkerManager{
		statusMonitor:        statusMonitor,
		blockManager:         team.NewBlockManager(nil),
		processManager:       process.NewWSHProcessManager(),
		taskStore:            taskStore,
		processes:            make(map[string]*process.AgentProcess),
		processMetadata:      make(map[string]*ProcessMetadata),
		maxConcurrentWorkers: DefaultMaxConcurrentWorkers,
		statusDir:            statusDir,
		defaultTabID:         "",
	}
}

// SetBlockManager sets a custom BlockManager (for testing or DI)
func (wm *WorkerManager) SetBlockManager(blockManager *team.BlockManager) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.blockManager = blockManager
}

// SetDefaultTabID sets the default tab ID for spawning blocks
func (wm *WorkerManager) SetDefaultTabID(tabID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.defaultTabID = tabID
}

// SetProcessManager sets a custom ProcessManager (for testing)
func (wm *WorkerManager) SetProcessManager(processManager process.ProcessManager) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.processManager = processManager
}

// SetTaskStore sets a custom TaskStore (for testing)
func (wm *WorkerManager) SetTaskStore(taskStore TaskStore) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.taskStore = taskStore
}

// getWorkerCommand returns the CLI command for a given worker type
func (wm *WorkerManager) getWorkerCommand(workerType string) string {
	// S03: Use placeholder commands for all worker types
	// In future slices, lookup from ProviderService based on workerType
	cmdMap := map[string]string{
		"claude":   "echo \"[claude worker processing task\"",
		"opencode": "echo \"[opencode worker processing task\"",
		"codex":    "echo \"[codex worker processing task\"",
		"":         "echo \"[default worker processing task\"",
		"default":  "echo \"[default worker processing task\"",
	}

	if cmd, ok := cmdMap[workerType]; ok {
		return cmd + " && sleep 0.1\"" // Small sleep to simulate work
	}

	// Unknown worker type - use default
	return fmt.Sprintf("echo \"[worker type: %s processing task\" && sleep 0.1", workerType)
}

// StartWorker starts a new worker process for a task
func (wm *WorkerManager) StartWorker(ctx context.Context, taskID string, workerType string) (*WorkerInfo, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Check if worker already exists
	if _, exists := wm.processes[taskID]; exists {
		return nil, fmt.Errorf("worker for task %s already exists", taskID)
	}

	// Check concurrency limit
	if len(wm.processes) >= wm.maxConcurrentWorkers {
		return nil, fmt.Errorf("concurrent worker limit reached (%d)", wm.maxConcurrentWorkers)
	}

	// Get worker command
	command := wm.getWorkerCommand(workerType)

	// Create status directory
	taskStatusDir := filepath.Join(wm.statusDir, taskID)
	if err := os.MkdirAll(taskStatusDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create status directory: %w", err)
	}

	// Create initial status file
	statusFilePath := filepath.Join(taskStatusDir, StatusFileName)
	status := WorkerStatus{
		Status:     "running",
		LastUpdate: time.Now().Unix(),
		Message:    fmt.Sprintf("Worker started for task %s", taskID),
	}

	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		_ = os.RemoveAll(taskStatusDir)
		return nil, fmt.Errorf("failed to marshal status: %w", err)
	}

	if err := os.WriteFile(statusFilePath, statusData, 0644); err != nil {
		_ = os.RemoveAll(taskStatusDir)
		return nil, fmt.Errorf("failed to write status file: %w", err)
	}

	// Parse command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		_ = os.RemoveAll(taskStatusDir)
		return nil, fmt.Errorf("invalid command: %s", command)
	}

	// Create process spec
	spec := process.ProcessSpec{
		Command: parts[0],
		Args:    parts[1:],
		Cwd:     wavebase.GetHomeDir(),
	}

	// Spawn process
	agentProc, err := wm.processManager.SpawnProcess(ctx, spec)
	if err != nil {
		_ = os.RemoveAll(taskStatusDir)
		return nil, fmt.Errorf("failed to spawn worker process: %w", err)
	}

	// Track process
	wm.processes[taskID] = agentProc

	// Store metadata
	metadata := &ProcessMetadata{
		TaskID:     taskID,
		WorkerType: workerType,
		BlockID:    "", // Will be set when block is spawned
		StatusDir:  taskStatusDir,
		StartedAt:  time.Now(),
	}
	wm.processMetadata[taskID] = metadata

	// Note: We skip block spawning in S03 to keep it simple
	// Blocks will be spawned in future slices when Dashboard is implemented
	if agentProc.Command != nil && agentProc.Command.Process != nil {
		log.Printf("[worker-manager] worker started for task %s (worker_type: %s, pid: %d)",
			taskID, workerType, agentProc.Command.Process.Pid)
	} else {
		log.Printf("[worker-manager] worker started for task %s (worker_type: %s)",
			taskID, workerType)
	}

	// Create worker info
	processInfo := wm.processManager.GetProcessInfo(agentProc)
	workerInfo := &WorkerInfo{
		TaskID:       taskID,
		WorkerType:   workerType,
		ProcessState: processInfo.State,
		PID:          processInfo.PID,
		BlockID:      "",
		StartedAt:    processInfo.StartedAt,
		StatusDir:    taskStatusDir,
	}

	// Start crash monitor if not already running
	wm.startWorkerMonitor()

	return workerInfo, nil
}

// StopWorker stops a worker process and cleans up resources
func (wm *WorkerManager) StopWorker(workerID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	agentProc, exists := wm.processes[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	metadata, _ := wm.processMetadata[workerID]

	// Kill process
	if err := wm.processManager.KillProcess(agentProc); err != nil {
		log.Printf("[worker-manager] warning: failed to kill worker %s: %v", workerID, err)
	}

	// Wait for process to finish (with timeout for safety)
	done := make(chan error, 1)
	go func() {
		done <- agentProc.Wait()
	}()
	select {
	case <-time.After(1 * time.Second):
		log.Printf("[worker-manager] warning: timeout waiting for worker %s to exit", workerID)
	case err := <-done:
		if err != nil {
			log.Printf("[worker-manager] worker %s exited with error: %v", workerID, err)
		}
	}

	// Destroy agent block if exists
	if metadata != nil && metadata.BlockID != "" && wm.blockManager != nil {
		if err := wm.blockManager.DestroyAgentBlock(context.Background(), workerID); err != nil {
			log.Printf("[worker-manager] warning: failed to destroy block for worker %s: %v", workerID, err)
		}
	}

	// Remove status directory
	if metadata != nil && metadata.StatusDir != "" {
		if err := os.RemoveAll(metadata.StatusDir); err != nil {
			log.Printf("[worker-manager] warning: failed to remove status directory %s: %v", metadata.StatusDir, err)
		}
	}

	// Remove from tracking
	delete(wm.processes, workerID)
	delete(wm.processMetadata, workerID)
	wm.statusMonitor.RemoveTracker(workerID)

	// Stop crash monitor if this was the last worker
	if len(wm.processes) == 0 {
		wm.stopWorkerMonitor()
	}

	log.Printf("[worker-manager] worker stopped for task %s", workerID)
	return nil
}

// GetWorkerInfo returns information about a worker
func (wm *WorkerManager) GetWorkerInfo(workerID string) (*WorkerInfo, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	agentProc, exists := wm.processes[workerID]
	if !exists {
		return nil, fmt.Errorf("worker %s not found", workerID)
	}

	metadata, _ := wm.processMetadata[workerID]

	processInfo := wm.processManager.GetProcessInfo(agentProc)

	return &WorkerInfo{
		TaskID:       workerID,
		WorkerType:   metadata.WorkerType,
		ProcessState: processInfo.State,
		PID:          processInfo.PID,
		BlockID:      metadata.BlockID,
		StartedAt:    processInfo.StartedAt,
		StatusDir:    metadata.StatusDir,
	}, nil
}

// ListWorkers returns information about all active workers
func (wm *WorkerManager) ListWorkers() []*WorkerInfo {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workers := make([]*WorkerInfo, 0, len(wm.processes))

	for taskID, agentProc := range wm.processes {
		metadata, _ := wm.processMetadata[taskID]
		processInfo := wm.processManager.GetProcessInfo(agentProc)

		workers = append(workers, &WorkerInfo{
			TaskID:       taskID,
			WorkerType:   metadata.WorkerType,
			ProcessState: processInfo.State,
			PID:          processInfo.PID,
			BlockID:      metadata.BlockID,
			StartedAt:    processInfo.StartedAt,
			StatusDir:    metadata.StatusDir,
		})
	}

	return workers
}

// SetBlockID sets the block ID for a worker
func (wm *WorkerManager) SetBlockID(workerID, blockID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if metadata, exists := wm.processMetadata[workerID]; exists {
		metadata.BlockID = blockID
	}
}

// GetActiveWorkerCount returns the number of currently active workers
func (wm *WorkerManager) GetActiveWorkerCount() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return len(wm.processes)
}

// SetMaxConcurrentWorkers sets the maximum number of concurrent workers
func (wm *WorkerManager) SetMaxConcurrentWorkers(max int) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.maxConcurrentWorkers = max
}

// startWorkerMonitor starts the crash detection monitor (idempotent)
func (wm *WorkerManager) startWorkerMonitor() {
	wm.monitorMu.Lock()
	defer wm.monitorMu.Unlock()

	// Check if already running
	if wm.monitorTicker != nil {
		return
	}

	wm.monitorTicker = time.NewTicker(500 * time.Millisecond)
	wm.monitorStopped = make(chan struct{})

	go wm.monitorLoop()

	log.Printf("[worker-manager] worker crash monitor started (interval: 500ms)")
}

// stopWorkerMonitor stops the crash detection monitor (idempotent)
func (wm *WorkerManager) stopWorkerMonitor() {
	wm.monitorMu.Lock()
	defer wm.monitorMu.Unlock()

	if wm.monitorTicker == nil {
		return
	}

	// Get references before clearing state
	ticker := wm.monitorTicker
	stopped := wm.monitorStopped

	// Clear state
	wm.monitorTicker = nil
	wm.monitorStopped = nil

	// Stop ticker and close channel outside lock
	ticker.Stop()
	close(stopped)

	log.Printf("[worker-manager] worker crash monitor stopped")
}

// monitorLoop is the main crash detection loop
func (wm *WorkerManager) monitorLoop() {
	for {
		wm.monitorMu.Lock()
		ticker := wm.monitorTicker
		stopped := wm.monitorStopped
		wm.monitorMu.Unlock()

		if ticker == nil || stopped == nil {
			// Monitor stopped, exit loop
			return
		}

		select {
		case <-ticker.C:
			wm.checkWorkersForCrashes()
		case <-stopped:
			return
		}
	}
}

// checkWorkersForCrashes iterates through all workers and checks for crashes
func (wm *WorkerManager) checkWorkersForCrashes() {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	for taskID, agentProc := range wm.processes {
		processInfo := wm.processManager.GetProcessInfo(agentProc)

		// Detect crash: process is done with non-zero exit code or exit signal
		if processInfo.State == process.ProcessStateDone &&
			(processInfo.ExitCode != 0 || processInfo.ExitSignal != "") {
			// Handle crash in goroutine to avoid blocking the monitor loop
			go wm.handleWorkerCrash(taskID, processInfo.ExitCode, processInfo.ExitSignal)
		}
	}
}

// handleWorkerCrash handles a crashed worker by scheduling retry or marking failed
func (wm *WorkerManager) handleWorkerCrash(taskID string, exitCode int, exitSignal string) {
	log.Printf("[worker-manager] crash detected for task %s (exitCode: %d, exitSignal: %s)", taskID, exitCode, exitSignal)

	// Get task to read metadata
	task, err := wm.taskStore.GetTask(taskID)
	if err != nil {
		log.Printf("[worker-manager] failed to get task %s: %v", taskID, err)
		return
	}

	// Extract retry metadata with defaults
	retryCount := getIntMetadataValue(task.Metadata, "retry_count", 0)
	maxRetries := getIntMetadataValue(task.Metadata, "max_retries", 3)

	// Construct crash reason
	crashReason := fmt.Sprintf("exitCode: %d", exitCode)
	if exitSignal != "" {
		crashReason = fmt.Sprintf("%s, exitSignal: %s", crashReason, exitSignal)
	}

	// Check if retry count exceeded
	if retryCount >= maxRetries {
		log.Printf("[worker-manager] max retries exceeded for task %s, marking as failed (crash reason: %s)", taskID, crashReason)
		wm.markTaskAsFailed(taskID, crashReason)
		return
	}

	// Calculate exponential backoff
	backoffMs := 1000 * (1 << uint(retryCount)) // 1s, 2s, 4s, 8s, ...
	if backoffMs > 30000 {
		backoffMs = 30000 // Cap at 30s
	}

	// Increment retry count and update metadata
	retryCount++
	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	task.Metadata["retry_count"] = retryCount
	task.Metadata["max_retries"] = maxRetries
	task.Metadata["backoff_ms"] = backoffMs
	task.Metadata["crash_reason"] = crashReason
	task.Metadata["last_crash_at"] = time.Now().Unix()

	// Update task status to blocked (while waiting for retry)
	if err := wm.taskStore.UpdateTaskStatus(taskID, team.TaskStatusBlocked); err != nil {
		log.Printf("[worker-manager] failed to update task %s status to blocked: %v", taskID, err)
	}

	log.Printf("[worker-manager] retrying worker %s (attempt %d/%d, backoff: %dms)", taskID, retryCount, maxRetries, backoffMs)

	// Schedule retry with backoff
	go func() {
		time.Sleep(time.Duration(backoffMs) * time.Millisecond)
		wm.retryWorker(taskID)
	}()
}

// retryWorker respawns a worker for the given task
func (wm *WorkerManager) retryWorker(taskID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Check if worker still exists (may have been stopped externally)
	metadata, exists := wm.processMetadata[taskID]
	if !exists {
		log.Printf("[worker-manager] worker %s not found, skipping retry", taskID)
		return
	}

	workerType := metadata.WorkerType

	// Remove the old process from processes map
	delete(wm.processes, taskID)

	// Respawn the worker
	ctx := context.Background()
	_, err := wm.StartWorker(ctx, taskID, workerType)
	if err != nil {
		log.Printf("[worker-manager] failed to respawn worker %s: %v", taskID, err)
		// Mark task as failed if respawn fails
		wm.mu.Unlock()
		wm.markTaskAsFailed(taskID, fmt.Sprintf("failed to respawn: %v", err))
		wm.mu.Lock()
		return
	}

	log.Printf("[worker-manager] worker %s respawned successfully", taskID)
}

// markTaskAsFailed marks a task as failed and writes the status file
func (wm *WorkerManager) markTaskAsFailed(taskID, crashReason string) {
	// Get current task status
	task, err := wm.taskStore.GetTask(taskID)
	if err != nil {
		log.Printf("[worker-manager] failed to get task %s: %v", taskID, err)
		return
	}

	// Ensure task is in a state that can transition to failed
	// Valid paths: pending -> in_progress -> failed
	//             blocked -> in_progress -> failed
	if task.Status == team.TaskStatusPending || task.Status == team.TaskStatusBlocked {
		// First transition to in_progress
		if err := wm.taskStore.UpdateTaskStatus(taskID, team.TaskStatusInProgress); err != nil {
			log.Printf("[worker-manager] failed to update task %s status to in_progress: %v", taskID, err)
		}
	}

	// Now it's safe to transition to failed
	if err := wm.taskStore.UpdateTaskStatus(taskID, team.TaskStatusFailed); err != nil {
		log.Printf("[worker-manager] failed to update task %s status to failed: %v", taskID, err)
	}

	// Write status file with crash reason
	wm.mu.RLock()
	metadata, hasMetadata := wm.processMetadata[taskID]
	wm.mu.RUnlock()

	if hasMetadata && metadata.StatusDir != "" {
		status := WorkerStatus{
			Status:        "failed",
			Message:       fmt.Sprintf("Task failed after max retries: %s", crashReason),
			LastUpdate:    time.Now().Unix(),
			OutputSummary: crashReason,
		}

		statusData, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			log.Printf("[worker-manager] failed to marshal failed status for task %s: %v", taskID, err)
		} else {
			statusFilePath := filepath.Join(metadata.StatusDir, StatusFileName)
			if err := os.WriteFile(statusFilePath, statusData, 0644); err != nil {
				log.Printf("[worker-manager] failed to write failed status file for task %s: %v", taskID, err)
			}
		}
	}

	// Clean up worker
	_ = wm.StopWorker(taskID)
}

// getIntMetadataValue extracts an int value from metadata with a default
func getIntMetadataValue(metadata map[string]interface{}, key string, defaultValue int) int {
	if metadata == nil {
		return defaultValue
	}

	val, exists := metadata[key]
	if !exists {
		return defaultValue
	}

	// Handle int and float64 (JSON unmarshals numbers as float64)
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	default:
		return defaultValue
	}
}
