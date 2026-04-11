// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package mcpservers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/sawka/txwrap"
)

// MCPServer represents an MCP server in the database
type MCPServer struct {
	ID          string
	Name        string
	Description string
	Config      map[string]any
	Enabled     bool
	CreatedAt   int64
	UpdatedAt   int64
}

// MCPServerDB handles database operations for MCP servers
type MCPServerDB struct {
	db *sql.DB
}

// MakeMCPServerDB creates a new MCPServerDB instance
func MakeMCPServerDB(db *sql.DB) *MCPServerDB {
	return &MCPServerDB{
		db: db,
	}
}

// MakeMCPServerDBFromSqlx creates a new MCPServerDB instance from sqlx.DB
func MakeMCPServerDBFromSqlx(dbx *sqlx.DB) *MCPServerDB {
	return &MCPServerDB{
		db: dbx.DB,
	}
}

// boolToInt converts bool to int
func boolToInt(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}

// intToBool converts int to bool
func intToBool(enabled int) bool {
	return enabled == 1
}

// CreateMCPServer creates a new MCP server
func (m *MCPServerDB) CreateMCPServer(server *MCPServer) error {
	if server.Name == "" {
		return fmt.Errorf("server name is required")
	}

	tx, err := txwrap.Wrap(m.db)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// Marshal config to JSON
	configJSON, err := json.Marshal(server.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Insert MCP server
	_, err = tx.Exec(
		"INSERT INTO mcp_servers (id, name, description, config, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		server.ID,
		server.Name,
		server.Description,
		string(configJSON),
		boolToInt(server.Enabled),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert MCP server: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetMCPServer retrieves a single MCP server by id
func (m *MCPServerDB) GetMCPServer(id string) (*MCPServer, error) {
	var server MCPServer
	var configJSON string
	var enabled int

	err := m.db.QueryRow(
		"SELECT id, name, description, config, enabled, created_at, updated_at FROM mcp_servers WHERE id = ?",
		id,
	).Scan(&server.ID, &server.Name, &server.Description, &configJSON, &enabled, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("MCP server not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get MCP server: %w", err)
	}

	server.Enabled = intToBool(enabled)

	// Unmarshal config from JSON
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &server.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	} else {
		server.Config = make(map[string]any)
	}

	return &server, nil
}

// ListMCPServers retrieves all MCP servers
func (m *MCPServerDB) ListMCPServers() ([]*MCPServer, error) {
	rows, err := m.db.Query("SELECT id, name, description, config, enabled, created_at, updated_at FROM mcp_servers ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP servers: %w", err)
	}
	defer rows.Close()

	var servers []*MCPServer
	for rows.Next() {
		var server MCPServer
		var configJSON string
		var enabled int

		if err := rows.Scan(&server.ID, &server.Name, &server.Description, &configJSON, &enabled, &server.CreatedAt, &server.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan MCP server: %w", err)
		}

		server.Enabled = intToBool(enabled)

		// Unmarshal config from JSON
		if configJSON != "" {
			if err := json.Unmarshal([]byte(configJSON), &server.Config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal config: %w", err)
			}
		} else {
			server.Config = make(map[string]any)
		}

		servers = append(servers, &server)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating MCP servers: %w", err)
	}

	return servers, nil
}

// ListEnabledMCPServers retrieves only enabled MCP servers
func (m *MCPServerDB) ListEnabledMCPServers() ([]*MCPServer, error) {
	rows, err := m.db.Query("SELECT id, name, description, config, enabled, created_at, updated_at FROM mcp_servers WHERE enabled = 1 ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled MCP servers: %w", err)
	}
	defer rows.Close()

	var servers []*MCPServer
	for rows.Next() {
		var server MCPServer
		var configJSON string
		var enabled int

		if err := rows.Scan(&server.ID, &server.Name, &server.Description, &configJSON, &enabled, &server.CreatedAt, &server.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan MCP server: %w", err)
		}

		server.Enabled = true

		// Unmarshal config from JSON
		if configJSON != "" {
			if err := json.Unmarshal([]byte(configJSON), &server.Config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal config: %w", err)
			}
		} else {
			server.Config = make(map[string]any)
		}

		servers = append(servers, &server)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating MCP servers: %w", err)
	}

	return servers, nil
}

// UpdateMCPServer updates an existing MCP server
func (m *MCPServerDB) UpdateMCPServer(server *MCPServer) error {
	if server.Name == "" {
		return fmt.Errorf("server name is required")
	}

	tx, err := txwrap.Wrap(m.db)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()

	// Marshal config to JSON
	configJSON, err := json.Marshal(server.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Update MCP server
	result, err := tx.Exec(
		"UPDATE mcp_servers SET name = ?, description = ?, config = ?, enabled = ?, updated_at = ? WHERE id = ?",
		server.Name,
		server.Description,
		string(configJSON),
		boolToInt(server.Enabled),
		now,
		server.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update MCP server: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("MCP server not found: %s", server.ID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SetMCPServerEnabled sets the enabled status of an MCP server
func (m *MCPServerDB) SetMCPServerEnabled(id string, enabled bool) error {
	result, err := m.db.Exec(
		"UPDATE mcp_servers SET enabled = ?, updated_at = ? WHERE id = ?",
		boolToInt(enabled),
		time.Now().UnixMilli(),
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to set MCP server enabled status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("MCP server not found: %s", id)
	}

	return nil
}

// DeleteMCPServer deletes an MCP server by id
func (m *MCPServerDB) DeleteMCPServer(id string) error {
	result, err := m.db.Exec("DELETE FROM mcp_servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete MCP server: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("MCP server not found: %s", id)
	}

	return nil
}
