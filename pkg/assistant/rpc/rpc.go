// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"log"

	"github.com/wavetermdev/waveterm/pkg/assistant"
	"github.com/wavetermdev/waveterm/pkg/panichandler"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

type WshRpcAssistantServer struct {
	assistant *assistant.Assistant
}

func (*WshRpcAssistantServer) WshServerImpl() {}

func NewWshRpcAssistantServer(assistant *assistant.Assistant) *WshRpcAssistantServer {
	return &WshRpcAssistantServer{
		assistant: assistant,
	}
}

func (as *WshRpcAssistantServer) AssistantStartCommand(ctx context.Context, data wshrpc.CommandAssistantStartData) (wshrpc.CommandAssistantStartRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AssistantStartCommand", recover())
	}()

	err := as.assistant.Start(ctx)
	if err != nil {
		log.Printf("[assistant rpc error] Start: %v", err)
		return wshrpc.CommandAssistantStartRtnData{}, fmt.Errorf("failed to start assistant: %w", err)
	}

	return wshrpc.CommandAssistantStartRtnData{
		Running: as.assistant.IsRunning(),
	}, nil
}

func (as *WshRpcAssistantServer) AssistantStopCommand(ctx context.Context, data wshrpc.CommandAssistantStopData) error {
	defer func() {
		panichandler.PanicHandler("AssistantStopCommand", recover())
	}()

	err := as.assistant.Stop()
	if err != nil {
		log.Printf("[assistant rpc error] Stop: %v", err)
		return fmt.Errorf("failed to stop assistant: %w", err)
	}

	log.Printf("[assistant rpc] stopped successfully")
	return nil
}

func (as *WshRpcAssistantServer) AssistantStatusCommand(ctx context.Context, data wshrpc.CommandAssistantStatusData) (wshrpc.CommandAssistantStatusRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AssistantStatusCommand", recover())
	}()

	statusMap, err := as.assistant.GetStatus()
	if err != nil {
		log.Printf("[assistant rpc error] GetStatus: %v", err)
		return wshrpc.CommandAssistantStatusRtnData{}, fmt.Errorf("failed to get status: %w", err)
	}

	running, _ := statusMap["running"].(bool)
	taskCount := 0
	if totalTasks, ok := statusMap["totalTasks"].(int); ok {
		taskCount = totalTasks
	}

	return wshrpc.CommandAssistantStatusRtnData{
		Running:   running,
		TaskCount: taskCount,
	}, nil
}

func (as *WshRpcAssistantServer) AssistantAddTaskCommand(ctx context.Context, data wshrpc.CommandAssistantAddTaskData) (wshrpc.CommandAssistantAddTaskRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AssistantAddTaskCommand", recover())
	}()

	if data.Description == "" {
		log.Printf("[assistant rpc error] AddTask: empty description")
		return wshrpc.CommandAssistantAddTaskRtnData{}, fmt.Errorf("task description cannot be empty")
	}

	task, err := as.assistant.AddTask(data.Description)
	if err != nil {
		log.Printf("[assistant rpc error] AddTask: %v", err)
		return wshrpc.CommandAssistantAddTaskRtnData{}, fmt.Errorf("failed to add task: %w", err)
	}

	log.Printf("[assistant rpc] task added: %s", task.TaskID)

	return wshrpc.CommandAssistantAddTaskRtnData{
		TaskID: task.TaskID,
		Status: string(task.Status),
	}, nil
}

func (as *WshRpcAssistantServer) AssistantListTasksCommand(ctx context.Context, data wshrpc.CommandAssistantListTasksData) (wshrpc.CommandAssistantListTasksRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AssistantListTasksCommand", recover())
	}()

	tasks, err := as.assistant.ListTasks()
	if err != nil {
		log.Printf("[assistant rpc error] ListTasks: %v", err)
		return wshrpc.CommandAssistantListTasksRtnData{}, fmt.Errorf("failed to list tasks: %w", err)
	}

	result := make([]wshrpc.AssistantTaskInfo, len(tasks))
	for i, task := range tasks {
		result[i] = wshrpc.AssistantTaskInfo{
			TaskID:          task.TaskID,
			Description:     task.Description,
			Status:          string(task.Status),
			AssignedAgentID: task.AssignedAgentID,
			CreatedAt:       task.CreatedAt,
		}
	}

	return wshrpc.CommandAssistantListTasksRtnData{
		Tasks: result,
	}, nil
}

func (as *WshRpcAssistantServer) AssistantForwardAgentMessageCommand(ctx context.Context, data wshrpc.CommandForwardAgentMessageData) (wshrpc.CommandForwardAgentMessageRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AssistantForwardAgentMessageCommand", recover())
	}()

	// Validate input
	if data.From == "" {
		log.Printf("[assistant rpc error] ForwardAgentMessage: empty from agent")
		return wshrpc.CommandForwardAgentMessageRtnData{}, fmt.Errorf("from agent cannot be empty")
	}
	if data.To == "" {
		log.Printf("[assistant rpc error] ForwardAgentMessage: empty to agent")
		return wshrpc.CommandForwardAgentMessageRtnData{}, fmt.Errorf("to agent cannot be empty")
	}
	if data.Content == "" {
		log.Printf("[assistant rpc error] ForwardAgentMessage: empty content")
		return wshrpc.CommandForwardAgentMessageRtnData{}, fmt.Errorf("message content cannot be empty")
	}

	// Call Assistant's ForwardAgentMessage method
	success, message, warning, err := as.assistant.ForwardAgentMessage(ctx, data.From, data.To, data.Content)
	if err != nil {
		log.Printf("[assistant rpc error] ForwardAgentMessage: from=%s to=%s error=%v", data.From, data.To, err)
		if containsAgentNotFoundError(err) {
			return wshrpc.CommandForwardAgentMessageRtnData{}, fmt.Errorf("agent not found: %w (404)", err)
		}
		return wshrpc.CommandForwardAgentMessageRtnData{}, fmt.Errorf("failed to forward message: %w", err)
	}

	log.Printf("[assistant rpc] ForwardAgentMessage: from=%s to=%s success", data.From, data.To)

	return wshrpc.CommandForwardAgentMessageRtnData{
		Success: success,
		Message: message,
		Warning: warning,
	}, nil
}

// containsAgentNotFoundError checks if error is an agent not found error
func containsAgentNotFoundError(err error) bool {
	return err != nil && (err.Error() == "agent not found" || err.Error() == "recipient agent not found")
}
