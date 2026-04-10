// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"github.com/wavetermdev/waveterm/pkg/zeroai/service"
	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

const (
	// TickerInterval defines how often the ticker checks for pending tasks
	TickerInterval = 5 * time.Second
	// DefaultPlaceholderAgentID is the placeholder agent ID used for task assignment in S01
	DefaultPlaceholderAgentID = "worker-1"
)

// Assistant manages task assignment and worker coordination
type Assistant struct {
	mu            sync.RWMutex
	running       bool
	taskStore     TaskStore
	agentService  *service.AgentService
	stopCh        chan struct{}
	ticker        *time.Ticker
	tickerStopped chan struct{}
}

// NewAssistant creates a new Assistant instance
func NewAssistant(agentSvc *service.AgentService) *Assistant {
	return &Assistant{
		taskStore:     NewInMemoryTaskStore(),
		agentService:  agentSvc,
		stopCh:        nil, // Created on Start
		ticker:        nil, // Created on Start
		tickerStopped: nil, // Created on Start
	}
}

// Start starts the assistant service if not already running
func (a *Assistant) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		log.Printf("[assistant] already running")
		return nil
	}

	a.stopCh = make(chan struct{})
	a.tickerStopped = make(chan struct{})
	a.running = true

	// Start ticker goroutine
	a.ticker = time.NewTicker(TickerInterval)
	go a.tickerLoop(ctx)

	log.Printf("[assistant] started")
	return nil
}

// Stop stops the assistant service gracefully
func (a *Assistant) Stop() error {
	a.mu.Lock()

	if !a.running {
		a.mu.Unlock()
		return nil
	}

	// Signal ticker goroutine to stop
	close(a.stopCh)

	// Stop the ticker
	if a.ticker != nil {
		a.ticker.Stop()
	}

	a.running = false

	// Capture references before unlocking
	tickerStopped := a.tickerStopped

	// Clear running state but keep tickerStopped channel for the goroutine to close
	a.stopCh = nil
	a.ticker = nil
	// NOTE: Don't set a.tickerStopped = nil here - the goroutine will close it

	a.mu.Unlock()

	// Wait for ticker goroutine to exit and close the channel
	if tickerStopped == nil {
		log.Printf("[assistant] stopped (no tickerStopped channel)")
		return nil
	}

	select {
	case <-tickerStopped:
		log.Printf("[assistant] stopped")
		return nil
	case <-time.After(5 * time.Second):
		log.Printf("[assistant] timeout waiting for ticker goroutine to stop")
		return fmt.Errorf("timeout waiting for ticker goroutine to stop")
	}
}

// IsRunning returns whether the assistant is currently running
func (a *Assistant) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.running
}

// AddTask adds a new task to the assistant
func (a *Assistant) AddTask(description string) (*team.Task, error) {
	if description == "" {
		return nil, ErrEmptyDescription
	}

	task, err := a.taskStore.AddTask(description)
	if err != nil {
		return nil, fmt.Errorf("failed to add task: %w", err)
	}

	log.Printf("[assistant] task added: %s - %s", task.TaskID, description)
	return task, nil
}

// TaskYAML represents the YAML format for task definition
type TaskYAML struct {
	TaskID    string                 `yaml:"task_id,omitempty"`
	Description string                `yaml:"description"`
	Metadata  map[string]interface{} `yaml:"metadata,omitempty"`
}

// AddTaskFromYAML adds a task from YAML format
func (a *Assistant) AddTaskFromYAML(yamlData string) (*team.Task, error) {
	if yamlData == "" {
		return nil, fmt.Errorf("YAML data cannot be empty")
	}

	var taskYAML TaskYAML
	err := yaml.Unmarshal([]byte(yamlData), &taskYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if taskYAML.Description == "" {
		return nil, fmt.Errorf("task description is required in YAML")
	}

	// Allow task_id override? For now, ignore and let store generate
	task, err := a.taskStore.AddTaskWithMetadata(taskYAML.Description, taskYAML.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to add task from YAML: %w", err)
	}

	workerType := ""
	if taskYAML.Metadata != nil {
		if wt, ok := taskYAML.Metadata["worker_type"].(string); ok {
			workerType = wt
		}
	}

	log.Printf("[assistant] task added from YAML: %s - %s (worker_type: %s)",
		task.TaskID, taskYAML.Description, workerType)

	return task, nil
}

// ListTasks returns all tasks
func (a *Assistant) ListTasks() ([]*team.Task, error) {
	return a.taskStore.GetTasks()
}

// GetStatus returns the current status of the assistant
func (a *Assistant) GetStatus() (map[string]interface{}, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Get task counts
	tasks, err := a.taskStore.GetTasks()
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	statusCounts := map[string]int{
		string(team.TaskStatusPending):    0,
		string(team.TaskStatusInProgress): 0,
		string(team.TaskStatusCompleted):  0,
		string(team.TaskStatusFailed):     0,
		string(team.TaskStatusBlocked):    0,
	}

	for _, task := range tasks {
		statusCounts[string(task.Status)]++
	}

	status := map[string]interface{}{
		"running":        a.running,
		"totalTasks":     len(tasks),
		"statusCounts":   statusCounts,
		"tickerInterval": TickerInterval.String(),
	}

	return status, nil
}

// tickerLoop runs the periodic task processing loop
func (a *Assistant) tickerLoop(ctx context.Context) {
	// Capture and nil the stopped channel before goroutine exits
	// We capture it now so Stop() can wait on it
	log.Printf("[assistant] ticker loop started (interval: %s)", TickerInterval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[assistant] ticker loop stopped: context cancelled")
			a.closeTickerStopped()
			return
		case <-a.stopCh:
			log.Printf("[assistant] ticker loop stopped: stop signal received")
			a.closeTickerStopped()
			return
		case <-a.ticker.C:
			log.Printf("[assistant] ticker wakeup")
			a.processPendingTasks(ctx)
		}
	}
}

// closeTickerStopped safely closes the tickerStopped channel
func (a *Assistant) closeTickerStopped() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.tickerStopped != nil {
		close(a.tickerStopped)
		a.tickerStopped = nil
	}
}

// processPendingTasks processes all pending tasks and assigns them to workers
func (a *Assistant) processPendingTasks(ctx context.Context) {
	tasks, err := a.taskStore.GetTasks()
	if err != nil {
		log.Printf("[assistant] error getting tasks: %v", err)
		return
	}

	processedCount := 0
	assignedCount := 0

	for _, task := range tasks {
		if task.Status != team.TaskStatusPending {
			continue
		}

		processedCount++

		// For S02, still assign to placeholder agent
		// In S03, worker_type matching will be implemented
		agentID := DefaultPlaceholderAgentID

		// Check for worker_type in metadata for future S03 integration
		workerType := ""
		if task.Metadata != nil {
			if wt, ok := task.Metadata["worker_type"].(string); ok {
				workerType = wt
				log.Printf("[assistant] task %s has worker_type: %s (matching will be implemented in S03)",
					task.TaskID, workerType)
			}
		}

		err := a.taskStore.AssignTask(task.TaskID, agentID)
		if err != nil {
			log.Printf("[assistant] failed to assign task %s: %v", task.TaskID, err)
			continue
		}

		assignedCount++
		log.Printf("[assistant] task assigned: %s -> %s", task.TaskID, agentID)
	}

	if processedCount > 0 {
		log.Printf("[assistant] processed %d pending tasks, assigned %d", processedCount, assignedCount)
	}
}
