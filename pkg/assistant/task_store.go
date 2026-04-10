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
	ErrTaskNotFound      = errors.New("task not found")
	ErrEmptyDescription  = errors.New("task description cannot be empty")
	ErrTaskAlreadyAssined = errors.New("task is already assigned")
)

// TaskStore defines the interface for task storage operations
type TaskStore interface {
	// AddTask adds a new task with the given description
	AddTask(description string) (*team.Task, error)

	// GetTask retrieves a task by ID
	GetTask(taskID string) (*team.Task, error)

	// GetTasks retrieves all tasks
	GetTasks() ([]*team.Task, error)

	// AssignTask assigns a task to an agent
	AssignTask(taskID string, agentID string) error
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
