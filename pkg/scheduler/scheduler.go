// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/wavetermdev/waveterm/pkg/assistant"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
)

const (
	// TickerInterval defines how often the scheduler checks for due tasks
	TickerInterval = 30 * time.Second
)

// Scheduler manages the execution of scheduled tasks with recurrence support
type Scheduler struct {
	mu         sync.RWMutex
	running    bool
	assistant  *assistant.Assistant
	stopCh     chan struct{}
	ticker     *time.Ticker
	tickerDone chan struct{}
}

// NewScheduler creates a new Scheduler instance
func NewScheduler(assistant *assistant.Assistant) *Scheduler {
	return &Scheduler{
		assistant:  assistant,
		running:    false,
		stopCh:     nil,
		ticker:     nil,
		tickerDone: nil,
	}
}

// Start starts the scheduler service if not already running
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		log.Printf("[scheduler] already running")
		return nil
	}

	log.Printf("[scheduler] starting")
	s.stopCh = make(chan struct{})
	s.tickerDone = make(chan struct{})
	s.ticker = time.NewTicker(TickerInterval)
	s.running = true

	go s.tickerLoop()

	log.Printf("[scheduler] started (interval: %s)", TickerInterval)
	return nil
}

// Stop stops the scheduler service gracefully
func (s *Scheduler) Stop() error {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		return nil
	}

	log.Printf("[scheduler] stopping")

	// Signal ticker goroutine to stop
	close(s.stopCh)

	// Stop the ticker
	if s.ticker != nil {
		s.ticker.Stop()
	}

	s.running = false

	// Capture references before unlocking
	tickerDone := s.tickerDone

	// Clear running state but keep ticker/tickerDone for the goroutine to finalize
	s.stopCh = nil
	// NOTE: Don't set s.ticker = nil or s.tickerDone = nil here - the goroutine is using them

	s.mu.Unlock()

	// Wait for ticker goroutine to exit and close the channel
	if tickerDone == nil {
		log.Printf("[scheduler] stopped (no tickerDone channel)")
		return nil
	}

	select {
	case <-tickerDone:
		log.Printf("[scheduler] stopped")
		return nil
	case <-time.After(5 * time.Second):
		log.Printf("[scheduler] timeout waiting for ticker goroutine to stop")
		return fmt.Errorf("timeout waiting for ticker goroutine to stop")
	}
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// tickerLoop runs the periodic task processing loop
func (s *Scheduler) tickerLoop() {
	log.Printf("[scheduler] ticker loop started")

	for {
		select {
		case <-s.stopCh:
			log.Printf("[scheduler] ticker loop stopped: stop signal received")
			s.closeTickerDone()
			return
		case <-s.ticker.C:
			log.Printf("[scheduler] ticker wakeup")
			s.processDueTasks()
		}
	}
}

// closeTickerDone safely closes the tickerDone channel
func (s *Scheduler) closeTickerDone() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tickerDone != nil {
		close(s.tickerDone)
		s.tickerDone = nil
	}
	// Now safe to nil out the ticker since the goroutine has exited
	s.ticker = nil
}

// processDueTasks processes all tasks that are due to run
func (s *Scheduler) processDueTasks() {
	ctx := context.Background()

	// Get all due tasks
	tasks, err := GetDueScheduledTasks(ctx)
	if err != nil {
		log.Printf("[scheduler] error getting due tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		log.Printf("[scheduler] no due tasks to process")
		return
	}

	log.Printf("[scheduler] processing %d due tasks", len(tasks))

	processedCount := 0
	for _, task := range tasks {
		// Execute the task by calling AddTaskFromYAML
		log.Printf("[scheduler] executing due task: %s (pattern: %s, next_run: %d)",
			task.OID, task.Pattern, task.NextRun)

		_, err := s.assistant.AddTaskFromYAML(task.TaskYAML)
		if err != nil {
			log.Printf("[scheduler] failed to execute task %s via AddTaskFromYAML: %v", task.OID, err)
			// Continue with other tasks even if one fails
			continue
		}

		log.Printf("[scheduler] task %s executed successfully via Assistant", task.OID)

		// Calculate next run time
		nextRun, status, execCount := s.calculateNextRun(task)

		// Update the task
		err = UpdateScheduledTaskFn(ctx, task.OID, func(t *waveobj.ScheduledTask) error {
			t.LastRun = time.Now().UnixMilli()
			t.ExecCount = execCount

			if status != "" {
				t.Status = status
			}

			if nextRun > 0 {
				t.NextRun = nextRun
			}

			return nil
		})

		if err != nil {
			log.Printf("[scheduler] failed to update task %s: %v", task.OID, err)
			continue
		}

		processedCount++
		log.Printf("[scheduler] task %s updated: next_run=%d, status=%s, execcount=%d",
			task.OID, nextRun, status, execCount)
	}

	log.Printf("[scheduler] processed %d due tasks successfully", processedCount)
}

// calculateNextRun calculates the next execution time and new status based on the task pattern
func (s *Scheduler) calculateNextRun(task *waveobj.ScheduledTask) (nextRun int64, status string, execCount int) {
	execCount = task.ExecCount + 1

	switch task.Pattern {
	case "once":
		// One-time execution: mark as completed
		nextRun = 0
		status = "completed"
		log.Printf("[scheduler] pattern 'once': task %s marked completed", task.OID)

	case "daily":
		// Daily recurrence: add 24 hours
		nextRun = task.NextRun + (24 * time.Hour).Milliseconds()
		status = "pending"
		log.Printf("[scheduler] pattern 'daily': task %s next run in 24h", task.OID)

	case "hourly":
		// Hourly recurrence: add 1 hour
		nextRun = task.NextRun + (1 * time.Hour).Milliseconds()
		status = "pending"
		log.Printf("[scheduler] pattern 'hourly': task %s next run in 1h", task.OID)

	case "weekly":
		// Weekly recurrence: add 7 days (168 hours)
		nextRun = task.NextRun + (168 * time.Hour).Milliseconds()
		status = "pending"
		log.Printf("[scheduler] pattern 'weekly': task %s next run in 7d", task.OID)

	case "repeat":
		// Finite execution: check max_execs
		if task.MaxExecs > 0 && execCount >= task.MaxExecs {
			nextRun = 0
			status = "completed"
			log.Printf("[scheduler] pattern 'repeat': task %s reached max_execs (%d), marked completed",
				task.OID, task.MaxExecs)
		} else {
			// Default to hourly for repeat pattern if no specific interval is set
			// This could be enhanced to support custom intervals in metadata
			nextRun = task.NextRun + (1 * time.Hour).Milliseconds()
			status = "pending"
			log.Printf("[scheduler] pattern 'repeat': task %s next run in 1h (exec_count=%d/%d)",
				task.OID, execCount, task.MaxExecs)
		}

	default:
		// Unknown pattern: treat as once
		log.Printf("[scheduler] unknown pattern '%s' for task %s, treating as 'once'",
			task.Pattern, task.OID)
		nextRun = 0
		status = "completed"
	}

	return nextRun, status, execCount
}

// ProcessMissedTasksOnStartup scans for tasks that should have run during shutdown
// and marks them as missed. This should be called on server startup.
func ProcessMissedTasksOnStartup(ctx context.Context) (int, error) {
	count, err := MarkMissedOnStartup(ctx)
	if err != nil {
		return 0, err
	}

	if count > 0 {
		log.Printf("[scheduler] marked %d tasks as missed on startup", count)
	}

	return count, nil
}
