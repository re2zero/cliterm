// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"log"

	"github.com/wavetermdev/waveterm/pkg/agentregistry"
	"github.com/wavetermdev/waveterm/pkg/panichandler"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

type WshRpcAgentRegistryServer struct {
	registry *agentregistry.AgentRegistry
}

func (*WshRpcAgentRegistryServer) WshServerImpl() {}

func NewWshRpcAgentRegistryServer(registry *agentregistry.AgentRegistry) *WshRpcAgentRegistryServer {
	return &WshRpcAgentRegistryServer{
		registry: registry,
	}
}

func (ars *WshRpcAgentRegistryServer) AgentRegisterCommand(ctx context.Context, data wshrpc.CommandAgentRegisterData) (wshrpc.CommandAgentRegisterRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AgentRegisterCommand", recover())
	}()

	agent := &agentregistry.Agent{
		Name:   data.Name,
		Role:   data.Role,
		Soul:   data.Soul,
		Skills: data.Skills,
		MCPConnections: convertRPCMCPConnections(data.MCPConnections),
		Config:  data.Config,
		Enabled: data.Enabled,
	}

	registeredAgent, err := ars.registry.RegisterAgent(ctx, agent)
	if err != nil {
		log.Printf("[agentregistry rpc error] Register: %v", err)
		return wshrpc.CommandAgentRegisterRtnData{}, fmt.Errorf("failed to register agent: %w", err)
	}

	return wshrpc.CommandAgentRegisterRtnData{
		ID: registeredAgent.ID,
	}, nil
}

func (ars *WshRpcAgentRegistryServer) AgentGetCommand(ctx context.Context, data wshrpc.CommandAgentGetData) (wshrpc.CommandAgentGetRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AgentGetCommand", recover())
	}()

	agent, err := ars.registry.GetAgent(ctx, data.ID)
	if err != nil {
		log.Printf("[agentregistry rpc error] Get: %v", err)
		return wshrpc.CommandAgentGetRtnData{}, fmt.Errorf("failed to get agent: %w", err)
	}

	return wshrpc.CommandAgentGetRtnData{
		Agent: convertAgentToRPCAgent(agent),
	}, nil
}

func (ars *WshRpcAgentRegistryServer) AgentListCommand(ctx context.Context, data wshrpc.CommandAgentListData) (wshrpc.CommandAgentListRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AgentListCommand", recover())
	}()

	opts := agentregistry.ListOptions{
		Enabled: data.Enabled,
		Role:    data.Role,
		Limit:   data.Limit,
		Offset:  data.Offset,
	}

	agents, err := ars.registry.ListAgents(ctx, opts)
	if err != nil {
		log.Printf("[agentregistry rpc error] List: %v", err)
		return wshrpc.CommandAgentListRtnData{}, fmt.Errorf("failed to list agents: %w", err)
	}

	rpcAgents := make([]wshrpc.Agent, len(agents))
	for i, agent := range agents {
		rpcAgents[i] = convertAgentToRPCAgent(agent)
	}

	return wshrpc.CommandAgentListRtnData{
		Agents: rpcAgents,
	}, nil
}

func (ars *WshRpcAgentRegistryServer) AgentUpdateCommand(ctx context.Context, data wshrpc.CommandAgentUpdateData) (wshrpc.CommandAgentUpdateRtnData, error) {
	defer func() {
		panichandler.PanicHandler("AgentUpdateCommand", recover())
	}()

	agent := &agentregistry.Agent{
		ID:             data.ID,
		Name:           data.Name,
		Role:           data.Role,
		Soul:           data.Soul,
		Skills:         data.Skills,
		MCPConnections: convertRPCMCPConnections(data.MCPConnections),
		Config:         data.Config,
		Enabled:        data.Enabled,
	}

	updatedAgent, err := ars.registry.UpdateAgent(ctx, agent)
	if err != nil {
		log.Printf("[agentregistry rpc error] Update: %v", err)
		return wshrpc.CommandAgentUpdateRtnData{}, fmt.Errorf("failed to update agent: %w", err)
	}

	return wshrpc.CommandAgentUpdateRtnData{
		ID: updatedAgent.ID,
	}, nil
}

func (ars *WshRpcAgentRegistryServer) AgentDeleteCommand(ctx context.Context, data wshrpc.CommandAgentDeleteData) error {
	defer func() {
		panichandler.PanicHandler("AgentDeleteCommand", recover())
	}()

	err := ars.registry.UnregisterAgent(ctx, data.ID)
	if err != nil {
		log.Printf("[agentregistry rpc error] Delete: %v", err)
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	log.Printf("[agentregistry rpc] deleted agent %s", data.ID)
	return nil
}

// convertAgentToRPCAgent converts an internal Agent to the RPC Agent type
func convertAgentToRPCAgent(agent *agentregistry.Agent) wshrpc.Agent {
	return wshrpc.Agent{
		ID:             agent.ID,
		Name:           agent.Name,
		Role:           agent.Role,
		Soul:           agent.Soul,
		Skills:         agent.Skills,
		MCPConnections: convertAgentMCPConnections(agent.MCPConnections),
		Config:         agent.Config,
		Enabled:        agent.Enabled,
		CreatedAt:      agent.CreatedAt,
		UpdatedAt:      agent.UpdatedAt,
	}
}

// convertRPCMCPConnections converts RPC MCPConnection slice to agentregistry MCPConnection slice
func convertRPCMCPConnections(conns []wshrpc.MCPConnection) []agentregistry.MCPConnection {
	result := make([]agentregistry.MCPConnection, len(conns))
	for i, conn := range conns {
		result[i] = agentregistry.MCPConnection{
			ServerName: conn.ServerName,
			Config:     conn.Config,
		}
	}
	return result
}

// convertAgentMCPConnections converts agentregistry MCPConnection slice to RPC MCPConnection slice
func convertAgentMCPConnections(conns []agentregistry.MCPConnection) []wshrpc.MCPConnection {
	result := make([]wshrpc.MCPConnection, len(conns))
	for i, conn := range conns {
		result[i] = wshrpc.MCPConnection{
			ServerName: conn.ServerName,
			Config:     conn.Config,
		}
	}
	return result
}
