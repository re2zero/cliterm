// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/zeroai/team"
)

var (
	ErrTaskNotFound          = errors.New("task not found")
	ErrEmptyDescription      = errors.New("task description cannot be empty")
	ErrTaskAlreadyAssined    = errors.New("task is already assigned")
	ErrInvalidStatusTransition = errors.New("invalid task status transition")
)

// TaskStore defines the interface for task storage operations
type TaskStore interface {
	// AddTask adds a new task with the given description
	AddTask(description string) (*team.Task, error)

	// AddTaskWithMetadata adds a new task with the given description and metadata
	AddTaskWithMetadata(description string, metadata map[string]interface{}) (*team.Task, error)

	// GetTask retrieves a task by ID
	GetTask(taskID string) (*team.Task, error)

	// GetTasks retrieves all tasks
	GetTasks() ([]*team.Task, error)

	// AssignTask assigns a task to an agent
	AssignTask(taskID string, agentID string) error

	// UpdateTaskStatus updates the status of a task
	UpdateTaskStatus(taskID string, status team.TaskStatus) error
}

// InMemoryTaskStore provides an in-memory implementation of TaskStore
type InMemoryTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*team.Task
}

// NewInMemoryTaskStore creates a new in-memory task store
func NewInMemoryTaskStore() *InMemoryTaskStore {
	return &InMemoryTaskStore{
		tasks: make(map[string]*team.Task),
	}
}

// AddTask adds a new task to the store
func (s *InMemoryTaskStore) AddTask(description string) (*team.Task, error) {
	return s.AddTaskWithMetadata(description, nil)
}

// AddTaskWithMetadata adds a new task to the store with metadata
func (s *InMemoryTaskStore) AddTaskWithMetadata(description string, metadata map[string]interface{}) (*team.Task, error) {
	if description == "" {
		return nil, ErrEmptyDescription
	}

	taskID := uuid.New().String()
	task := &team.Task{
		TaskID:      taskID,
		TeamID:      "assistant", // Fixed team ID for assistant
		Status:      team.TaskStatusPending,
		Description: description,
		CreatedAt:   time.Now().Unix(),
		Metadata:    metadata,
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	return task, nil
}

// GetTask retrieves a task by ID
func (s *InMemoryTaskStore) GetTask(taskID string) (*team.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

// GetTasks retrieves all tasks from the store
func (s *InMemoryTaskStore) GetTasks() ([]*team.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*team.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// AssignTask assigns a task to an agent
func (s *InMemoryTaskStore) AssignTask(taskID string, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != team.TaskStatusPending && task.Status != team.TaskStatusBlocked {
		return fmt.Errorf("task is not in assignable state (current: %s)", task.Status)
	}

	task.AssignedAgentID = agentID
	task.Status = team.TaskStatusInProgress

	return nil
}

// UpdateTaskStatus updates the status of a task with validation
func (s *InMemoryTaskStore) UpdateTaskStatus(taskID string, status team.TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	// Validate state transition
	if !isValidTransition(task.Status, status) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatusTransition, task.Status, status)
	}

	// Update completed time if transitioning to terminal state
	if status == team.TaskStatusCompleted || status == team.TaskStatusFailed {
		task.CompletedAt = time.Now().Unix()
	}

	task.Status = status
	return nil
}

// isValidTransition checks if a status transition is valid
func isValidTransition(from, to team.TaskStatus) bool {
	// Allow same status (idempotent)
	if from == to {
		return false
	}

	// Valid transitions
	switch from {
	case team.TaskStatusPending:
		return to == team.TaskStatusInProgress || to == team.TaskStatusBlocked
	case team.TaskStatusInProgress:
		return to == team.TaskStatusCompleted || to == team.TaskStatusFailed || to == team.TaskStatusBlocked
	case team.TaskStatusBlocked:
		return to == team.TaskStatusPending || to == team.TaskStatusInProgress
	case team.TaskStatusCompleted, team.TaskStatusFailed:
		// Terminal states - no transitions allowed
		return false
	default:
		return false
	}
}
