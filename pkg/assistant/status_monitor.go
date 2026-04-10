// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

const (
	// StatusMonitorInterval is the interval for polling status files
	StatusMonitorInterval = 5 * time.Second
	// DefaultStallTimeout is the default timeout for considering a worker stalled
	DefaultStallTimeout = 60 * time.Second
	// StatusFileName is the name of the status file
	StatusFileName = "status.json"
)

// WorkerStatus represents the status of a worker from the status file
type WorkerStatus struct {
	Status        string `json:"status"`                  // "running", "completed", "failed"
	Message       string `json:"message,omitempty"`       // Optional message
	LastUpdate    int64  `json:"last_update"`             // Unix timestamp
	OutputSummary string `json:"output_summary,omitempty"` // Optional output summary
}

// WorkerTracker tracks the state of a worker for stall detection
type WorkerTracker struct {
	TaskID      string
	LastUpdate  time.Time
	Status      string // "running", "completed", "failed"
	BlockID     string // Associated block ID (if any)
}

// StatusMonitor monitors worker status files and updates task states
type StatusMonitor struct {
	taskStore     TaskStore
	statusDir     string
	stallTimeout  time.Duration
	tracker       map[string]*WorkerTracker  // taskID -> tracker
	trackerMu     sync.RWMutex
	ticker        *time.Ticker
	stopCh        chan struct{}
	tickerStopped chan struct{}
	running       bool
}

// NewStatusMonitor creates a new StatusMonitor instance
func NewStatusMonitor(taskStore TaskStore) *StatusMonitor {
	waveHome := wavebase.GetHomeDir()
	statusDir := filepath.Join(waveHome, ".gsd", "assistant", "workers")

	return &StatusMonitor{
		taskStore:     taskStore,
		statusDir:     statusDir,
		stallTimeout:  DefaultStallTimeout,
		tracker:       make(map[string]*WorkerTracker),
		stopCh:        nil,
		ticker:        nil,
		tickerStopped: nil,
		running:       false,
	}
}

// Start starts the status monitor ticker
func (sm *StatusMonitor) Start(ctx context.Context) error {
	sm.trackerMu.Lock()
	if sm.running {
		sm.trackerMu.Unlock()
		return nil
	}
	sm.running = true
	sm.stopCh = make(chan struct{})
	sm.tickerStopped = make(chan struct{})
	sm.trackerMu.Unlock()

	sm.trackerMu.Lock()
	sm.ticker = time.NewTicker(StatusMonitorInterval)
	sm.trackerMu.Unlock()

	go sm.monitorLoop(ctx)

	log.Printf("[status-monitor] started (interval: %s, status_dir: %s)", StatusMonitorInterval, sm.statusDir)
	return nil
}

// Stop stops the status monitor gracefully
func (sm *StatusMonitor) Stop() error {
	sm.trackerMu.Lock()
	if !sm.running {
		sm.trackerMu.Unlock()
		return nil
	}

	// Capture references before clearing state
	stopCh := sm.stopCh
	ticker := sm.ticker

	sm.running = false
	sm.stopCh = nil
	sm.tickerStopped = nil
	sm.ticker = nil
	sm.trackerMu.Unlock()

	// Close channels and stop outside lock
	if ticker != nil {
		ticker.Stop()
	}
	if stopCh != nil {
		close(stopCh)
	}

	// Give the goroutine a moment to notice the closed channel and exit
	// Since we set sm.running = false, the goroutine will exit on its next loop iteration
	time.Sleep(10 * time.Millisecond)
	log.Printf("[status-monitor] stopped")
	return nil
}

// monitorLoop is the main ticker loop for polling status files
func (sm *StatusMonitor) monitorLoop(ctx context.Context) {
	log.Printf("[status-monitor] monitor loop started")

	for {
		// Get ticker reference safely
		var ticker *time.Ticker
		var running bool
		sm.trackerMu.RLock()
		ticker = sm.ticker
		running = sm.running
		sm.trackerMu.RUnlock()

		if !running || ticker == nil {
			// Monitor has been stopped, exit loop
			log.Printf("[status-monitor] ticker stopped, exiting monitor loop")
			return
		}

		select {
		case <-ctx.Done():
			log.Printf("[status-monitor] monitor loop stopped: context cancelled")
			return
		case <-sm.stopCh:
			log.Printf("[status-monitor] monitor loop stopped: stop signal received")
			return
		case <-ticker.C:
			sm.PollStatusFiles()
			sm.CheckForStalls()
		}
	}
}

// PollStatusFiles reads status files and updates task states
func (sm *StatusMonitor) PollStatusFiles() {
	if _, err := os.Stat(sm.statusDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(sm.statusDir)
	if err != nil {
		log.Printf("[status-monitor] error reading status dir: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		taskID := entry.Name()
		statusFilePath := filepath.Join(sm.statusDir, taskID, StatusFileName)

		sm.pollStatusFile(taskID, statusFilePath)
	}
}

// pollStatusFile reads a single status file and updates the task state
func (sm *StatusMonitor) pollStatusFile(taskID, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[status-monitor] error reading status file %s: %v", filePath, err)
		}
		return
	}

	var status WorkerStatus
	if err := json.Unmarshal(data, &status); err != nil {
		log.Printf("[status-monitor] error parsing status file %s: %v", filePath, err)
		return
	}

	// Validate status field
	validStatuses := map[string]bool{
		"running":   true,
		"completed": true,
		"failed":    true,
	}
	if !validStatuses[status.Status] {
		log.Printf("[status-monitor] invalid status '%s' in file %s", status.Status, filePath)
		return
	}

	lastUpdate := time.Unix(status.LastUpdate, 0)

	// Update tracker
	sm.trackerMu.Lock()
	tracker, exists := sm.tracker[taskID]
	if !exists {
		tracker = &WorkerTracker{
			TaskID: taskID,
		}
		sm.tracker[taskID] = tracker
	}
	tracker.LastUpdate = lastUpdate
	tracker.Status = status.Status
	sm.trackerMu.Unlock()

	// Map worker status to task status
	var taskStatus string
	switch status.Status {
	case "running":
		taskStatus = string(team.TaskStatusInProgress)
	case "completed":
		taskStatus = string(team.TaskStatusCompleted)
	case "failed":
		taskStatus = string(team.TaskStatusFailed)
	default:
		return
	}

	// Update task status via TaskStore
	err = sm.taskStore.UpdateTaskStatus(taskID, team.TaskStatus(taskStatus))
	if err != nil {
		log.Printf("[status-monitor] failed to update task %s status to %s: %v", taskID, taskStatus, err)
	} else {
		log.Printf("[status-monitor] task %s status updated: %s (last_update: %s)", taskID, status.Status, lastUpdate.Format(time.RFC3339))
	}
}

// CheckForStalls checks for workers that haven't updated recently
func (sm *StatusMonitor) CheckForStalls() {
	sm.trackerMu.RLock()
	defer sm.trackerMu.RUnlock()

	now := time.Now()

	for taskID, tracker := range sm.tracker {
		if tracker.Status != "running" {
			continue
		}

		timeSinceUpdate := now.Sub(tracker.LastUpdate)
		if timeSinceUpdate > sm.stallTimeout {
			// Worker appears stalled - mark task as blocked
			err := sm.taskStore.UpdateTaskStatus(taskID, team.TaskStatusBlocked)
			if err != nil {
				log.Printf("[status-monitor] WARNING: task %s appears stalled (last update %s ago), failed to mark as blocked: %v",
					taskID, timeSinceUpdate, err)
			} else {
				log.Printf("[status-monitor] WARNING: task %s stalled and marked as blocked (last update %s ago)",
					taskID, timeSinceUpdate)
			}
		}
	}
}

// SetStallTimeout sets the stall timeout duration
func (sm *StatusMonitor) SetStallTimeout(timeout time.Duration) {
	sm.stallTimeout = timeout
}

// GetTracker returns the tracker for a specific task
func (sm *StatusMonitor) GetTracker(taskID string) *WorkerTracker {
	sm.trackerMu.RLock()
	defer sm.trackerMu.RUnlock()
	return sm.tracker[taskID]
}

// RemoveTracker removes the tracker for a task (called when worker is stopped)
func (sm *StatusMonitor) RemoveTracker(taskID string) {
	sm.trackerMu.Lock()
	defer sm.trackerMu.Unlock()
	delete(sm.tracker, taskID)
}

// IsRunning returns whether the monitor is currently running
func (sm *StatusMonitor) IsRunning() bool {
	return sm.running
}
