// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/panichandler"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"

	"gopkg.in/yaml.v3"
)

type WshRpcSchedulerServer struct {
	scheduler *Scheduler
}

func (*WshRpcSchedulerServer) WshServerImpl() {}

func NewWshRpcSchedulerServer(scheduler *Scheduler) *WshRpcSchedulerServer {
	return &WshRpcSchedulerServer{
		scheduler: scheduler,
	}
}

// SchedulerAddScheduledTaskCommand adds a new scheduled task
func (ss *WshRpcSchedulerServer) SchedulerAddScheduledTaskCommand(ctx context.Context, data wshrpc.CommandSchedulerAddScheduledTaskData) (wshrpc.CommandSchedulerAddScheduledTaskRtnData, error) {
	defer func() {
		panichandler.PanicHandler("SchedulerAddScheduledTaskCommand", recover())
	}()

	// Validate TaskYAML is parseable YAML
	if data.TaskYAML == "" {
		log.Printf("[scheduler rpc error] AddScheduledTask: empty TaskYAML")
		return wshrpc.CommandSchedulerAddScheduledTaskRtnData{}, fmt.Errorf("TaskYAML cannot be empty")
	}

	// Parse YAML to validate structure
	var taskYAML map[string]interface{}
	err := yaml.Unmarshal([]byte(data.TaskYAML), &taskYAML)
	if err != nil {
		log.Printf("[scheduler rpc error] AddScheduledTask: invalid YAML: %v", err)
		return wshrpc.CommandSchedulerAddScheduledTaskRtnData{}, fmt.Errorf("invalid YAML: %w", err)
	}

	// Check for required description field
	description, _ := taskYAML["description"].(string)
	if description == "" {
		log.Printf("[scheduler rpc error] AddScheduledTask: missing description in YAML")
		return wshrpc.CommandSchedulerAddScheduledTaskRtnData{}, fmt.Errorf("YAML must contain a description field")
	}

	// Validate pattern
	validPatterns := map[string]bool{
		"once":   true,
		"daily":  true,
		"hourly": true,
		"weekly": true,
		"repeat": true,
	}

	if !validPatterns[data.Pattern] {
		log.Printf("[scheduler rpc error] AddScheduledTask: invalid pattern '%s'", data.Pattern)
		return wshrpc.CommandSchedulerAddScheduledTaskRtnData{}, fmt.Errorf("invalid pattern '%s', must be one of: once, daily, hourly, weekly, repeat", data.Pattern)
	}

	// Get first run time
	firstRun := data.FirstRunUnix
	if firstRun == 0 {
		// Default to now
		firstRun = time.Now().UnixMilli()
	}

	// Create the scheduled task
	task := &waveobj.ScheduledTask{
		OID:       uuid.New().String(),
		TaskYAML:  data.TaskYAML,
		Pattern:   data.Pattern,
		NextRun:   firstRun,
		LastRun:   0,
		Status:    "pending",
		ExecCount: 0,
		MaxExecs:  data.MaxExecs,
	}

	// Store in database
	err = AddScheduledTask(ctx, task)
	if err != nil {
		log.Printf("[scheduler rpc error] AddScheduledTask: failed to add task: %v", err)
		return wshrpc.CommandSchedulerAddScheduledTaskRtnData{}, fmt.Errorf("failed to add scheduled task: %w", err)
	}

	log.Printf("[scheduler rpc] scheduled task added: %s (pattern: %s, first_run: %d)",
		task.OID, data.Pattern, firstRun)

	return wshrpc.CommandSchedulerAddScheduledTaskRtnData{
		TaskID: task.OID,
	}, nil
}

// SchedulerListTasksCommand lists all scheduled tasks
func (ss *WshRpcSchedulerServer) SchedulerListTasksCommand(ctx context.Context, data wshrpc.CommandSchedulerListTasksData) (wshrpc.CommandSchedulerListTasksRtnData, error) {
	defer func() {
		panichandler.PanicHandler("SchedulerListTasksCommand", recover())
	}()

	tasks, err := ListScheduledTasks(ctx)
	if err != nil {
		log.Printf("[scheduler rpc error] ListTasks: failed to get tasks: %v", err)
		return wshrpc.CommandSchedulerListTasksRtnData{}, fmt.Errorf("failed to list scheduled tasks: %w", err)
	}

	result := make([]wshrpc.ScheduledTaskInfo, len(tasks))
	for i, task := range tasks {
		result[i] = wshrpc.ScheduledTaskInfo{
			OID:       task.OID,
			TaskYAML:  task.TaskYAML,
			NextRun:   task.NextRun,
			LastRun:   task.LastRun,
			Status:    task.Status,
			Pattern:   task.Pattern,
			ExecCount: task.ExecCount,
			MaxExecs:  task.MaxExecs,
		}
	}

	log.Printf("[scheduler rpc] listed %d scheduled tasks", len(tasks))

	return wshrpc.CommandSchedulerListTasksRtnData{
		Tasks: result,
	}, nil
}

// SchedulerDeleteTaskCommand deletes a scheduled task
func (ss *WshRpcSchedulerServer) SchedulerDeleteTaskCommand(ctx context.Context, data wshrpc.CommandSchedulerDeleteTaskData) error {
	defer func() {
		panichandler.PanicHandler("SchedulerDeleteTaskCommand", recover())
	}()

	if data.TaskID == "" {
		log.Printf("[scheduler rpc error] DeleteTask: empty TaskID")
		return fmt.Errorf("task ID cannot be empty")
	}

	err := DeleteScheduledTask(ctx, data.TaskID)
	if err != nil {
		log.Printf("[scheduler rpc error] DeleteTask: failed to delete task %s: %v", data.TaskID, err)
		return fmt.Errorf("failed to delete scheduled task: %w", err)
	}

	log.Printf("[scheduler rpc] scheduled task deleted: %s", data.TaskID)
	return nil
}
