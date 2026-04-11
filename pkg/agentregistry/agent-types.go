// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package agentregistry

// Agent represents a registered agent in the system
type Agent struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Role           string           `json:"role"`
	Soul           string           `json:"soul,omitempty"`
	Skills         []string         `json:"skills,omitempty"`
	MCPConnections []MCPConnection  `json:"mcpConnections,omitempty"`
	Config         map[string]any   `json:"config,omitempty"`
	Enabled        bool             `json:"enabled"`
	CreatedAt      int64            `json:"createdAt"`
	UpdatedAt      int64            `json:"updatedAt"`
}

// MCPConnection represents an MCP server connection configuration
type MCPConnection struct {
	ServerName string            `json:"serverName"`
	Config     map[string]any    `json:"config,omitempty"`
}
