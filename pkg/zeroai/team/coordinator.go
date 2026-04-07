// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTeamNotFound      = errors.New("team not found")
	ErrMemberNotFound    = errors.New("member not found")
	ErrTaskNotFound      = errors.New("task not found")
	ErrTeamAlreadyExists = errors.New("team already exists")
)

// Coordinator manages team lifecycle, task assignment, and member coordination
type Coordinator struct {
	store TeamStore
	mu    sync.RWMutex

	// Task worker loop state
	workerLoops map[string]*workerLoopState // teamID -> worker loop state

	// Execution configuration
	blockMgr   *BlockManager
	router     *MessageRouter
	maxRetries int
}

// workerLoopState tracks the state of a team's task worker loop
type workerLoopState struct {
	running bool
	stopCh  chan struct{}
}

// NewCoordinator creates a new team coordinator
func NewCoordinator(store TeamStore) (*Coordinator, error) {
	if store == nil {
		return nil, errors.New("team store is required")
	}

	c := &Coordinator{
		store:       store,
		workerLoops: make(map[string]*workerLoopState),
	}

	return c, nil
}

// NewCoordinatorWithDeps creates a coordinator with block manager and message router for task execution
func NewCoordinatorWithDeps(store TeamStore, blockMgr *BlockManager, router *MessageRouter, opts ...CoordinatorOption) (*Coordinator, error) {
	if store == nil {
		return nil, errors.New("team store is required")
	}

	c := &Coordinator{
		store:       store,
		workerLoops: make(map[string]*workerLoopState),
		blockMgr:    blockMgr,
		router:      router,
		maxRetries:  3,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// CoordinatorOption is a functional option for configuring the coordinator
type CoordinatorOption func(*Coordinator)

// WithMaxRetries sets the maximum number of retries for failed tasks
func WithMaxRetries(maxRetries int) CoordinatorOption {
	return func(c *Coordinator) {
		c.maxRetries = maxRetries
	}
}

// WithBlockManager sets the block manager for task execution
func WithBlockManager(blockMgr *BlockManager) CoordinatorOption {
	return func(c *Coordinator) {
		c.blockMgr = blockMgr
	}
}

// CreateTeam creates a new team with the given leader
func (c *Coordinator) CreateTeam(name string, leaderID string) (*Team, error) {
	if name == "" {
		return nil, errors.New("team name is required")
	}
	if leaderID == "" {
		return nil, errors.New("leader ID is required")
	}

	teamID := uuid.New().String()

	team := &Team{
		TeamID:  teamID,
		Name:    name,
		Created: time.Now().Unix(),
		Status:  TeamStatusActive,
	}

	if err := c.store.CreateTeam(team); err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	// Add leader as first member
	leader := &TeamMember{
		AgentID:  leaderID,
		Role:     MemberRoleLeader,
		Status:   MemberStatusActive,
		JoinedAt: time.Now().Unix(),
	}

	if err := c.store.AddMember(teamID, leader); err != nil {
		// Rollback team creation on member add failure
		_ = c.store.DeleteTeam(teamID)
		return nil, fmt.Errorf("failed to add leader to team: %w", err)
	}

	return team, nil
}

// GetTeam retrieves a team by ID
func (c *Coordinator) GetTeam(teamID string) (*Team, error) {
	return c.store.GetTeam(teamID)
}

// ListTeams lists all teams with optional filtering
func (c *Coordinator) ListTeams(opts ListTeamsOptions) ([]*Team, error) {
	return c.store.ListTeams(opts)
}

// AddMember adds a new member to a team
func (c *Coordinator) AddMember(teamID, agentID string, role MemberRole) (*TeamMember, error) {
	if teamID == "" {
		return nil, errors.New("team ID is required")
	}
	if agentID == "" {
		return nil, errors.New("agent ID is required")
	}

	if role == "" {
		role = MemberRoleWorker
	}

	// Verify team exists
	if _, err := c.store.GetTeam(teamID); err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	member := &TeamMember{
		AgentID:  agentID,
		Role:     role,
		Status:   MemberStatusIdle,
		JoinedAt: time.Now().Unix(),
	}

	if err := c.store.AddMember(teamID, member); err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	return member, nil
}

// GetMembers retrieves all members of a team
func (c *Coordinator) GetMembers(teamID string) ([]*TeamMember, error) {
	return c.store.GetMembers(teamID)
}

// UpdateMemberStatus updates a member's status
func (c *Coordinator) UpdateMemberStatus(teamID, agentID string, status MemberStatus) error {
	// Get current member
	members, err := c.store.GetMembers(teamID)
	if err != nil {
		return err
	}

	var existingMember *TeamMember
	for _, m := range members {
		if m.AgentID == agentID {
			existingMember = m
			break
		}
	}

	if existingMember == nil {
		return ErrMemberNotFound
	}

	// Update status
	existingMember.Status = status
	return c.store.UpdateMember(teamID, agentID, existingMember)
}

// CreateTask creates a new task and optionally assigns it to a specific agent
func (c *Coordinator) CreateTask(teamID, description string, assignedAgentID string) (*Task, error) {
	if teamID == "" {
		return nil, errors.New("team ID is required")
	}
	if description == "" {
		return nil, errors.New("task description is required")
	}

	task := &Task{
		TaskID:          uuid.New().String(),
		TeamID:          teamID,
		AssignedAgentID: assignedAgentID,
		Status:          TaskStatusPending,
		Description:     description,
		CreatedAt:       time.Now().Unix(),
	}

	if err := c.store.CreateTask(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return task, nil
}

// GetTask retrieves a task by ID
func (c *Coordinator) GetTask(taskID string) (*Task, error) {
	return c.store.GetTask(taskID)
}

// ListTasks lists tasks for a team with optional filtering
func (c *Coordinator) ListTasks(teamID string, opts ListTasksOptions) ([]*Task, error) {
	return c.store.ListTasks(teamID, opts)
}

// GetNextPendingTask retrieves the next pending task for a team
func (c *Coordinator) GetNextPendingTask(teamID string) (*Task, error) {
	opts := ListTasksOptions{
		Status: TaskStatusPending,
		Limit:  1,
	}

	tasks, err := c.store.ListTasks(teamID, opts)
	if err != nil {
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, nil // No pending tasks
	}

	return tasks[0], nil
}

// AssignTask assigns a task to a specific agent
func (c *Coordinator) AssignTask(taskID, agentID string) error {
	task, err := c.store.GetTask(taskID)
	if err != nil {
		return err
	}

	if task.Status != TaskStatusPending && task.Status != TaskStatusBlocked {
		return errors.New("task is not in a assignable state")
	}

	task.AssignedAgentID = agentID
	task.Status = TaskStatusInProgress

	return c.store.UpdateTask(task)
}

// StartTask marks a task as in progress
func (c *Coordinator) StartTask(taskID string) error {
	task, err := c.store.GetTask(taskID)
	if err != nil {
		return err
	}

	if task.Status != TaskStatusPending {
		return errors.New("task is not in pending state")
	}

	task.Status = TaskStatusInProgress
	return c.store.UpdateTask(task)
}

// CompleteTask marks a task as completed
func (c *Coordinator) CompleteTask(taskID string) error {
	task, err := c.store.GetTask(taskID)
	if err != nil {
		return err
	}

	if task.Status != TaskStatusInProgress && task.Status != TaskStatusBlocked {
		return errors.New("task is not in progress or blocked state")
	}

	task.Status = TaskStatusCompleted
	task.CompletedAt = time.Now().Unix()

	return c.store.UpdateTask(task)
}

// FailTask marks a task as failed
func (c *Coordinator) FailTask(taskID string) error {
	task, err := c.store.GetTask(taskID)
	if err != nil {
		return err
	}

	task.Status = TaskStatusFailed
	task.CompletedAt = time.Now().Unix()

	return c.store.UpdateTask(task)
}

// BlockTask marks a task as blocked with a dependency
func (c *Coordinator) BlockTask(taskID string, blockedBy Task) error {
	task, err := c.store.GetTask(taskID)
	if err != nil {
		return err
	}

	task.Status = TaskStatusBlocked
	return c.store.UpdateTask(task)
}

// GetAgentTasks retrieves all tasks assigned to a specific agent
func (c *Coordinator) GetAgentTasks(teamID, agentID string) ([]*Task, error) {
	opts := ListTasksOptions{
		AssignedAgentID: agentID,
	}

	return c.store.ListTasks(teamID, opts)
}

// GetActiveTask retrieves the currently active (in_progress) task for an agent
func (c *Coordinator) GetActiveTask(teamID, agentID string) (*Task, error) {
	opts := ListTasksOptions{
		Status:          TaskStatusInProgress,
		AssignedAgentID: agentID,
		Limit:           1,
	}

	tasks, err := c.store.ListTasks(teamID, opts)
	if err != nil {
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, nil // No active task
	}

	return tasks[0], nil
}

// DeleteTeam deletes a team and all its members and tasks
func (c *Coordinator) DeleteTeam(teamID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop worker loop if running
	if state, exists := c.workerLoops[teamID]; exists && state.running {
		close(state.stopCh)
		state.running = false
		delete(c.workerLoops, teamID)
	}

	return c.store.DeleteTeam(teamID)
}

// StartWorkerLoop starts a background goroutine that processes tasks for a team sequentially.
// It picks up pending tasks, assigns them to available agents, and monitors completion.
func (c *Coordinator) StartWorkerLoop(teamID string) error {
	c.mu.Lock()
	if state, exists := c.workerLoops[teamID]; exists && state.running {
		c.mu.Unlock()
		return fmt.Errorf("worker loop already running for team %s", teamID)
	}
	stopCh := make(chan struct{})
	c.workerLoops[teamID] = &workerLoopState{running: true, stopCh: stopCh}
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			if state, exists := c.workerLoops[teamID]; exists {
				state.running = false
			}
			c.mu.Unlock()
		}()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				c.processNextTask(teamID)
			}
		}
	}()

	return nil
}

// processNextTask picks and processes the next pending task for a team
func (c *Coordinator) processNextTask(teamID string) {
	if c.blockMgr == nil {
		return // No block manager, cannot execute tasks
	}

	task, err := c.GetNextPendingTask(teamID)
	if err != nil || task == nil {
		return // No pending tasks
	}

	// Check dependencies are met
	if !c.areDependenciesMet(task) {
		return // Dependencies not ready yet
	}

	// Find available agent for this task
	agentID := task.AssignedAgentID
	if agentID == "" {
		agentID = c.findAvailableAgent(teamID)
		if agentID == "" {
			return // No available agents
		}
	}

	// Assign and start task
	if err := c.AssignTask(task.TaskID, agentID); err != nil {
		log.Printf("[coordinator] failed to assign task %s: %v", task.TaskID, err)
		return
	}

	// Build and send prompt to agent
	prompt := c.buildTaskPrompt(task, teamID)
	if prompt != "" {
		if err := c.blockMgr.SendToAgent(agentID, prompt); err != nil {
			log.Printf("[coordinator] failed to send prompt to agent %s: %v", agentID, err)
			_ = c.FailTask(task.TaskID)
			return
		}
	}

	// Monitor task with retry logic
	c.monitorTaskWithRetry(task.TaskID, agentID)
}

// areDependenciesMet checks if all task dependencies are completed
func (c *Coordinator) areDependenciesMet(task *Task) bool {
	if c.router == nil {
		return true // No router, assume no dependencies
	}

	// Check if task has pending dependencies by looking at blocked tasks
	// In a full implementation, this would query TaskDependency records
	// For now, we check if the task is not blocked
	return task.Status != TaskStatusBlocked
}

// findAvailableAgent finds an active/idle agent in the team
func (c *Coordinator) findAvailableAgent(teamID string) string {
	members, err := c.GetMembers(teamID)
	if err != nil {
		return ""
	}

	for _, m := range members {
		if m.Status == MemberStatusActive || m.Status == MemberStatusIdle {
			return m.AgentID
		}
	}
	return ""
}

// buildTaskPrompt creates a prompt for the agent to execute the task
func (c *Coordinator) buildTaskPrompt(task *Task, teamID string) string {
	team, err := c.GetTeam(teamID)
	if err != nil {
		return task.Description
	}

	members, err := c.GetMembers(teamID)
	if err != nil || len(members) == 0 {
		return task.Description
	}

	leaderName := ""
	for _, m := range members {
		if m.Role == MemberRoleLeader {
			leaderName = m.AgentID
			break
		}
	}

	prompt := BuildAgentPrompt(PromptBuilderOpts{
		AgentName:  task.AssignedAgentID,
		AgentID:    task.AssignedAgentID,
		TeamName:   team.Name,
		LeaderName: leaderName,
		Role:       MemberRoleWorker,
		Task:       task.Description,
	})

	return prompt
}

// monitorTaskWithRetry monitors task completion with retry logic
func (c *Coordinator) monitorTaskWithRetry(taskID, agentID string) {
	go func() {
		teamID := ""
		retryOpts := DefaultRetryOptions()
		retryOpts.MaxRetries = c.maxRetries
		retryOpts.OnRetry = func(attempt int, err error, delay time.Duration) {
			log.Printf("[coordinator] task %s retry %d/%d after %v: %v", taskID, attempt, c.maxRetries, delay, err)
		}

		_, err := WithRetry(func() (bool, error) {
			currentTask, getErr := c.GetTask(taskID)
			if getErr != nil {
				return false, getErr
			}
			teamID = currentTask.TeamID // Capture for later

			switch currentTask.Status {
			case TaskStatusCompleted:
				return true, nil
			case TaskStatusFailed:
				return false, fmt.Errorf("task failed")
			default:
				return false, nil // Still in progress, will retry
			}
		}, retryOpts)

		if err != nil {
			log.Printf("[coordinator] task %s failed after retries: %v", taskID, err)
			_ = c.FailTask(taskID)
		} else {
			log.Printf("[coordinator] task %s completed", taskID)
			if teamID != "" {
				_ = c.UpdateMemberStatus(teamID, agentID, MemberStatusIdle)
			}
		}
	}()
}

// StopWorkerLoop stops the task worker loop for a team
func (c *Coordinator) StopWorkerLoop(teamID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if state, exists := c.workerLoops[teamID]; exists && state.running {
		close(state.stopCh)
		state.running = false
		delete(c.workerLoops, teamID)
	}
}

// RemoveMember removes a member from a team
func (c *Coordinator) RemoveMember(teamID, agentID string) error {
	// Check if member has active tasks
	tasks, err := c.GetAgentTasks(teamID, agentID)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task.Status == TaskStatusInProgress {
			return fmt.Errorf("member has active task: %s", task.TaskID)
		}
	}

	return c.store.RemoveMember(teamID, agentID)
}
