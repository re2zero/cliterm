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
	processes              map[string]*process.AgentProcess // taskID -> process
	processMetadata        map[string]*ProcessMetadata      // taskID -> metadata (blockID, etc.)
	mu                     sync.RWMutex
	maxConcurrentWorkers   int
	statusDir              string // Root directory for worker status files
	defaultTabID           string // Default tab ID for spawning blocks
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
func NewWorkerManager(statusMonitor *StatusMonitor) *WorkerManager {
	waveHome := wavebase.GetHomeDir()
	statusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers")

	return &WorkerManager{
		statusMonitor:        statusMonitor,
		blockManager:         team.NewBlockManager(nil),
		processManager:       process.NewWSHProcessManager(),
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
	log.Printf("[worker-manager] worker started for task %s (worker_type: %s, pid: %d)",
		taskID, workerType, agentProc.Command.Process.Pid)

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

	// Wait for process to finish
	_ = agentProc.Wait()

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
