// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"log"

	"github.com/wavetermdev/waveterm/pkg/mcpservers"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

// MCPServerRpcServer handles WSH RPC commands for MCP servers
type MCPServerRpcServer struct {
	store *mcpservers.MCPServerDB
}

// MakeMCPServerRpcServer creates a new MCP server RPC server
func MakeMCPServerRpcServer(store *mcpservers.MCPServerDB) *MCPServerRpcServer {
	return &MCPServerRpcServer{
		store: store,
	}
}

// WshServerImpl implements wshutil.ServerImpl interface
func (*MCPServerRpcServer) WshServerImpl() {}

// MCPServerListCommand lists all MCP servers
func (m *MCPServerRpcServer) MCPServerListCommand(ctx context.Context, data wshrpc.CommandMCPServerListData) (wshrpc.CommandMCPServerListRtnData, error) {
	servers, err := m.store.ListMCPServers()
	if err != nil {
		return wshrpc.CommandMCPServerListRtnData{}, fmt.Errorf("failed to list MCP servers: %w", err)
	}

	serverInfos := make([]wshrpc.MCPServerInfo, len(servers))
	for i, server := range servers {
		serverInfos[i] = wshrpc.MCPServerInfo{
			ID:          server.ID,
			Name:        server.Name,
			Description: server.Description,
			Config:      server.Config,
			Enabled:     server.Enabled,
			CreatedAt:   server.CreatedAt,
			UpdatedAt:   server.UpdatedAt,
		}
	}

	log.Printf("[mcpserver] listed %d MCP servers", len(serverInfos))
	return wshrpc.CommandMCPServerListRtnData{
		Servers: serverInfos,
	}, nil
}

// MCPServerRegisterCommand creates a new MCP server
func (m *MCPServerRpcServer) MCPServerRegisterCommand(ctx context.Context, data wshrpc.CommandMCPServerRegisterData) (wshrpc.CommandMCPServerRegisterRtnData, error) {
	server := &mcpservers.MCPServer{
		ID:          m.store.GenerateID(),
		Name:        data.Name,
		Description: data.Description,
		Config:      data.Config,
		Enabled:     data.Enabled,
	}

	if err := m.store.CreateMCPServer(server); err != nil {
		log.Printf("[mcpserver rpc error] failed to create MCP server: %v", err)
		return wshrpc.CommandMCPServerRegisterRtnData{}, fmt.Errorf("failed to create MCP server: %w", err)
	}

	log.Printf("[mcpserver] registered MCP server: %s (%s)", server.ID, server.Name)
	return wshrpc.CommandMCPServerRegisterRtnData{
		ID: server.ID,
	}, nil
}

// MCPServerUpdateCommand updates an existing MCP server
func (m *MCPServerRpcServer) MCPServerUpdateCommand(ctx context.Context, data wshrpc.CommandMCPServerUpdateData) (wshrpc.CommandMCPServerUpdateRtnData, error) {
	server, err := m.store.GetMCPServer(data.ID)
	if err != nil {
		log.Printf("[mcpserver rpc error] failed to get MCP server for update: %v", err)
		return wshrpc.CommandMCPServerUpdateRtnData{}, fmt.Errorf("failed to get MCP server: %w", err)
	}

	// Update fields
	server.Name = data.Name
	server.Description = data.Description
	server.Config = data.Config
	server.Enabled = data.Enabled

	if err := m.store.UpdateMCPServer(server); err != nil {
		log.Printf("[mcpserver rpc error] failed to update MCP server: %v", err)
		return wshrpc.CommandMCPServerUpdateRtnData{}, fmt.Errorf("failed to update MCP server: %w", err)
	}

	log.Printf("[mcpserver] updated MCP server: %s", server.ID)
	return wshrpc.CommandMCPServerUpdateRtnData{
		ID: server.ID,
	}, nil
}

// MCPServerSetEnabledCommand sets the enabled status of an MCP server
func (m *MCPServerRpcServer) MCPServerSetEnabledCommand(ctx context.Context, data wshrpc.CommandMCPServerSetEnabledData) error {
	if err := m.store.SetMCPServerEnabled(data.ID, data.Enabled); err != nil {
		log.Printf("[mcpserver rpc error] failed to set MCP server enabled: %v", err)
		return fmt.Errorf("failed to set MCP server enabled: %w", err)
	}

	log.Printf("[mcpserver] set MCP server enabled: %s = %t", data.ID, data.Enabled)
	return nil
}

// MCPServerDeleteCommand deletes an MCP server
func (m *MCPServerRpcServer) MCPServerDeleteCommand(ctx context.Context, data wshrpc.CommandMCPServerDeleteData) error {
	if err := m.store.DeleteMCPServer(data.ID); err != nil {
		log.Printf("[mcpserver rpc error] failed to delete MCP server: %v", err)
		return fmt.Errorf("failed to delete MCP server: %w", err)
	}

	log.Printf("[mcpserver] deleted MCP server: %s", data.ID)
	return nil
}
