// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wavetermdev/waveterm/pkg/blockcontroller"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wcore"
)

type AgentBlock struct {
	BlockID   string
	AgentID   string
	AgentName string
	TeamID    string
	TabID     string
	Role      MemberRole
	Command   string
	WorkDir   string
	CreatedAt int64
}

type BlockManager struct {
	blocks     map[string]*AgentBlock
	blockMu    sync.RWMutex
	router     *MessageRouter
	forwarders map[string]chan struct{}
}

func NewBlockManager(router *MessageRouter) *BlockManager {
	return &BlockManager{
		blocks:     make(map[string]*AgentBlock),
		router:     router,
		forwarders: make(map[string]chan struct{}),
	}
}

type SpawnBlockOpts struct {
	TabID         string
	AgentID       string
	AgentName     string
	TeamID        string
	Role          MemberRole
	Command       string
	WorkDir       string
	TargetBlockID string
	TargetAction  string
	Prompt        string
}

func (bm *BlockManager) SpawnAgentBlock(ctx context.Context, opts SpawnBlockOpts) (*AgentBlock, error) {
	if opts.AgentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}
	if opts.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if opts.TabID == "" {
		return nil, fmt.Errorf("tab ID is required")
	}

	bm.blockMu.Lock()
	if existing, ok := bm.blocks[opts.AgentID]; ok {
		bm.blockMu.Unlock()
		return existing, nil
	}
	bm.blockMu.Unlock()

	meta := waveobj.MetaMapType{
		waveobj.MetaKey_View:       "term",
		waveobj.MetaKey_Controller: "cmd",
		waveobj.MetaKey_Cmd:        opts.Command,
		waveobj.MetaKey_CmdCwd:     opts.WorkDir,
	}

	blockDef := &waveobj.BlockDef{
		Meta: meta,
	}

	block, err := wcore.CreateBlock(ctx, opts.TabID, blockDef, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create block: %w", err)
	}

	agentBlock := &AgentBlock{
		BlockID:   block.OID,
		AgentID:   opts.AgentID,
		AgentName: opts.AgentName,
		TeamID:    opts.TeamID,
		TabID:     opts.TabID,
		Role:      opts.Role,
		Command:   opts.Command,
		WorkDir:   opts.WorkDir,
		CreatedAt: time.Now().Unix(),
	}

	bm.blockMu.Lock()
	bm.blocks[opts.AgentID] = agentBlock
	bm.blockMu.Unlock()

	if bm.router != nil {
		bm.router.RegisterAgent(opts.AgentID, 256)
		bm.startForwarder(opts.AgentID)
	}

	if opts.Prompt != "" {
		time.Sleep(1500 * time.Millisecond)
		if err := bm.SendToBlock(agentBlock.BlockID, opts.Prompt); err != nil {
			_ = bm.DestroyAgentBlock(context.Background(), opts.AgentID)
			return nil, fmt.Errorf("failed to inject prompt: %w", err)
		}
	}

	return agentBlock, nil
}

func (bm *BlockManager) SendToBlock(blockID string, input string) error {
	inputUnion := &blockcontroller.BlockInputUnion{
		InputData: []byte(input + "\n"),
	}
	return blockcontroller.SendInput(blockID, inputUnion)
}

func (bm *BlockManager) SendToAgent(agentID string, input string) error {
	bm.blockMu.RLock()
	block, ok := bm.blocks[agentID]
	bm.blockMu.RUnlock()
	if !ok {
		return fmt.Errorf("agent %s has no block", agentID)
	}
	return bm.SendToBlock(block.BlockID, input)
}

func (bm *BlockManager) DestroyAgentBlock(ctx context.Context, agentID string) error {
	bm.blockMu.Lock()
	block, ok := bm.blocks[agentID]
	if ok {
		delete(bm.blocks, agentID)
	}
	bm.blockMu.Unlock()

	if !ok {
		return nil
	}

	if bm.router != nil {
		bm.router.UnregisterAgent(agentID)
	}
	bm.stopForwarder(agentID)

	return wcore.DeleteBlock(ctx, block.BlockID, false)
}

func (bm *BlockManager) GetBlock(agentID string) (*AgentBlock, bool) {
	bm.blockMu.RLock()
	defer bm.blockMu.RUnlock()
	block, ok := bm.blocks[agentID]
	return block, ok
}

func (bm *BlockManager) ListBlocks() []*AgentBlock {
	bm.blockMu.RLock()
	defer bm.blockMu.RUnlock()
	result := make([]*AgentBlock, 0, len(bm.blocks))
	for _, b := range bm.blocks {
		result = append(result, b)
	}
	return result
}

func (bm *BlockManager) DestroyTeamBlocks(ctx context.Context, teamID string) error {
	bm.blockMu.Lock()
	var toDelete []*AgentBlock
	var agentIDs []string
	for agentID, block := range bm.blocks {
		if block.TeamID == teamID {
			toDelete = append(toDelete, block)
			agentIDs = append(agentIDs, agentID)
		}
	}
	for _, agentID := range agentIDs {
		delete(bm.blocks, agentID)
	}
	bm.blockMu.Unlock()

	var firstErr error
	for i, agentID := range agentIDs {
		if bm.router != nil {
			bm.router.UnregisterAgent(agentID)
		}
		bm.stopForwarder(agentID)
		if err := wcore.DeleteBlock(ctx, toDelete[i].BlockID, false); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (bm *BlockManager) RouteMessageToBlock(msg *Message) error {
	if msg.To == "" {
		return fmt.Errorf("message has no target agent")
	}

	bm.blockMu.RLock()
	block, ok := bm.blocks[msg.To]
	bm.blockMu.RUnlock()
	if !ok {
		return fmt.Errorf("agent %s has no block", msg.To)
	}

	content := formatMessageForBlock(msg)
	return bm.SendToBlock(block.BlockID, content)
}

func formatMessageForBlock(msg *Message) string {
	switch msg.Type {
	case MessageTypeTaskAssignment:
		if payload, ok := msg.Payload.(*TaskAssignment); ok {
			return fmt.Sprintf("[TASK ASSIGNED] Task %s\nPriority: %d\nParameters: %v\nFrom: %s",
				payload.TaskID, payload.Priority, payload.Parameters, msg.From)
		}
	case MessageTypeStatusUpdate:
		if payload, ok := msg.Payload.(*StatusUpdate); ok {
			return fmt.Sprintf("[STATUS UPDATE from %s] %s: %s\nData: %v",
				msg.From, payload.StatusCode, payload.Message, payload.Data)
		}
	case MessageTypeAgentToAgent:
		if payload, ok := msg.Payload.(*AgentToAgentMessage); ok {
			return fmt.Sprintf("[MESSAGE from %s]\n%s", msg.From, payload.Content)
		}
	}
	return fmt.Sprintf("[Message from %s] %v", msg.From, msg.Payload)
}

func (bm *BlockManager) startForwarder(agentID string) {
	stopCh := make(chan struct{})
	bm.blockMu.Lock()
	bm.forwarders[agentID] = stopCh
	bm.blockMu.Unlock()

	go bm.forwardMessages(agentID, stopCh)
}

func (bm *BlockManager) stopForwarder(agentID string) {
	bm.blockMu.Lock()
	stopCh, ok := bm.forwarders[agentID]
	if ok {
		delete(bm.forwarders, agentID)
	}
	bm.blockMu.Unlock()

	if ok {
		close(stopCh)
	}
}

func (bm *BlockManager) forwardMessages(agentID string, stopCh chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if bm.router == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		queue, ok := bm.router.GetQueue(agentID)
		if !ok {
			time.Sleep(1 * time.Second)
			continue
		}

		msg := queue.DequeueWithTimeout(5 * time.Second)
		if msg == nil {
			continue
		}

		if err := bm.RouteMessageToBlock(msg); err != nil {
			fmt.Printf("block-manager: failed to forward message to agent %s: %v\n", agentID, err)
		}
	}
}
