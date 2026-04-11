// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package agentregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sawka/txwrap"
	dbfs "github.com/wavetermdev/waveterm/db"
	"github.com/wavetermdev/waveterm/pkg/util/migrateutil"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
)

type TxWrap = txwrap.TxWrap

const AgentDBName = "agents.db"

var globalAgentDB *sqlx.DB

type ListOptions struct {
	Enabled *bool  // Filter by enabled status
	Role    string // Filter by role (empty string for no filter)
	Limit   int    // Maximum number of results to return
	Offset  int    // Number of results to skip
}

type AgentDB struct {
	db *sqlx.DB
}

// InitAgentStore initializes the agent store by opening the SQLite database and running migrations
func InitAgentStore() error {
	ctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFn()

	var err error
	globalAgentDB, err = makeAgentDB(ctx)
	if err != nil {
		return fmt.Errorf("[agentregistry] failed to create agent db: %w", err)
	}

	err = migrateAgentDB()
	if err != nil {
		return fmt.Errorf("[agentregistry] failed to run migrations: %w", err)
	}

	log.Printf("[agentregistry] initialized\n")
	return nil
}

// GetAgentDB returns a reference to the global agent DB
func GetAgentDB() *sqlx.DB {
	return globalAgentDB
}

// NewAgentStore creates a new AgentDB instance using the global DB
func NewAgentStore() *AgentDB {
	return &AgentDB{
		db: globalAgentDB,
	}
}

// makeAgentDB creates a new SQLite database connection for agents
func makeAgentDB(ctx context.Context) (*sqlx.DB, error) {
	dbName := getAgentDBName()
	db, err := sqlx.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc&_journal_mode=WAL&_busy_timeout=5000", dbName))
	if err != nil {
		return nil, err
	}
	db.DB.SetMaxOpenConns(1)
	return db, nil
}

// getAgentDBName returns the full path to the agents database
func getAgentDBName() string {
	waveHome := wavebase.GetWaveDataDir()
	return filepath.Join(waveHome, wavebase.WaveDBDir, AgentDBName)
}

// migrateAgentDB runs the database migrations for agents
func migrateAgentDB() error {
	if globalAgentDB == nil {
		return fmt.Errorf("agent database not initialized")
	}

	return migrateutil.Migrate("agentregistry", globalAgentDB.DB, dbfs.AgentMigrationFS, "migrations-agent")
}

// Create creates a new agent in the database
func (s *AgentDB) Create(ctx context.Context, agent *Agent) error {
	return withAgentTx(ctx, s.db, func(tx *TxWrap) error {
		now := time.Now().UnixMilli()
		agent.CreatedAt = now
		agent.UpdatedAt = now

		skillsJSON, err := json.Marshal(agent.Skills)
		if err != nil {
			return fmt.Errorf("failed to marshal skills: %w", err)
		}

		mcpConnJSON, err := json.Marshal(agent.MCPConnections)
		if err != nil {
			return fmt.Errorf("failed to marshal MCP connections: %w", err)
		}

		configJSON, err := json.Marshal(agent.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		query := `
			INSERT INTO agents (id, name, role, soul, skills, mcp_connections, config, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		tx.Exec(query,
			agent.ID,
			agent.Name,
			agent.Role,
			agent.Soul,
			skillsJSON,
			mcpConnJSON,
			configJSON,
			boolToInt(agent.Enabled),
			agent.CreatedAt,
			agent.UpdatedAt,
		)

		if tx.Err != nil {
			return fmt.Errorf("failed to insert agent: %w", tx.Err)
		}

		return nil
	})
}

// Get retrieves an agent by ID
func (s *AgentDB) Get(ctx context.Context, id string) (*Agent, error) {
	var result struct {
		ID             string `db:"id"`
		Name           string `db:"name"`
		Role           string `db:"role"`
		Soul           string `db:"soul"`
		Skills         string `db:"skills"`
		MCPConnections string `db:"mcp_connections"`
		Config         string `db:"config"`
		Enabled        int    `db:"enabled"`
		CreatedAt      int64  `db:"created_at"`
		UpdatedAt      int64  `db:"updated_at"`
	}

	query := `SELECT id, name, role, soul, skills, mcp_connections, config, enabled, created_at, updated_at FROM agents WHERE id = ?`

	err := s.db.Get(&result, query, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %s", id)
	}

	return s.scanAgent(result), nil
}

// List retrieves agents according to the provided options
func (s *AgentDB) List(ctx context.Context, opts ListOptions) ([]*Agent, error) {
	var results []struct {
		ID             string `db:"id"`
		Name           string `db:"name"`
		Role           string `db:"role"`
		Soul           string `db:"soul"`
		Skills         string `db:"skills"`
		MCPConnections string `db:"mcp_connections"`
		Config         string `db:"config"`
		Enabled        int    `db:"enabled"`
		CreatedAt      int64  `db:"created_at"`
		UpdatedAt      int64  `db:"updated_at"`
	}

	query := `SELECT id, name, role, soul, skills, mcp_connections, config, enabled, created_at, updated_at FROM agents`
	var args []interface{}
	whereClauses := []string{}

	if opts.Enabled != nil {
		whereClauses = append(whereClauses, "enabled = ?")
		args = append(args, boolToInt(*opts.Enabled))
	}

	if opts.Role != "" {
		whereClauses = append(whereClauses, "role = ?")
		args = append(args, opts.Role)
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + joinWhereClauses(whereClauses)
	}

	query += " ORDER BY created_at DESC"

	if opts.Offset > 0 && opts.Limit == 0 {
		// SQLite requires LIMIT when using OFFSET
		query += " LIMIT -1"
	}

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	err := s.db.Select(&results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agents := make([]*Agent, len(results))
	for i, result := range results {
		agents[i] = s.scanAgent(result)
	}

	return agents, nil
}

// Update updates an existing agent in the database
func (s *AgentDB) Update(ctx context.Context, agent *Agent) error {
	return withAgentTx(ctx, s.db, func(tx *TxWrap) error {
		// Check if agent exists first
		var exists int
		found := tx.Get(&exists, "SELECT 1 FROM agents WHERE id = ?", agent.ID)
		if !found {
			return fmt.Errorf("agent not found: %s", agent.ID)
		}

		agent.UpdatedAt = time.Now().UnixMilli()

		skillsJSON, err := json.Marshal(agent.Skills)
		if err != nil {
			return fmt.Errorf("failed to marshal skills: %w", err)
		}

		mcpConnJSON, err := json.Marshal(agent.MCPConnections)
		if err != nil {
			return fmt.Errorf("failed to marshal MCP connections: %w", err)
		}

		configJSON, err := json.Marshal(agent.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		query := `
			UPDATE agents
			SET name = ?, role = ?, soul = ?, skills = ?, mcp_connections = ?, config = ?, enabled = ?, updated_at = ?
			WHERE id = ?
		`

		result := tx.Exec(query,
			agent.Name,
			agent.Role,
			agent.Soul,
			skillsJSON,
			mcpConnJSON,
			configJSON,
			boolToInt(agent.Enabled),
			agent.UpdatedAt,
			agent.ID,
		)

		if tx.Err != nil {
			return fmt.Errorf("failed to update agent: %w", tx.Err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("agent not found: %s", agent.ID)
		}

		return nil
	})
}

// Delete removes an agent from the database by ID
func (s *AgentDB) Delete(ctx context.Context, id string) error {
	return withAgentTx(ctx, s.db, func(tx *TxWrap) error {
		query := `DELETE FROM agents WHERE id = ?`
		result := tx.Exec(query, id)

		if tx.Err != nil {
			return fmt.Errorf("failed to delete agent: %w", tx.Err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("agent not found: %s", id)
		}

		return nil
	})
}

// scanAgent converts a database row result into an Agent struct
func (s *AgentDB) scanAgent(result struct {
	ID             string `db:"id"`
	Name           string `db:"name"`
	Role           string `db:"role"`
	Soul           string `db:"soul"`
	Skills         string `db:"skills"`
	MCPConnections string `db:"mcp_connections"`
	Config         string `db:"config"`
	Enabled        int    `db:"enabled"`
	CreatedAt      int64  `db:"created_at"`
	UpdatedAt      int64  `db:"updated_at"`
}) *Agent {
	var skills []string
	if result.Skills != "" {
		json.Unmarshal([]byte(result.Skills), &skills)
	}

	var mcpConns []MCPConnection
	if result.MCPConnections != "" {
		json.Unmarshal([]byte(result.MCPConnections), &mcpConns)
	}

	var config map[string]any
	if result.Config != "" {
		json.Unmarshal([]byte(result.Config), &config)
	}

	return &Agent{
		ID:             result.ID,
		Name:           result.Name,
		Role:           result.Role,
		Soul:           result.Soul,
		Skills:         skills,
		MCPConnections: mcpConns,
		Config:         config,
		Enabled:        intToBool(result.Enabled),
		CreatedAt:      result.CreatedAt,
		UpdatedAt:      result.UpdatedAt,
	}
}

// withAgentTx executes a function within a database transaction
func withAgentTx(ctx context.Context, db *sqlx.DB, fn func(tx *TxWrap) error) error {
	return txwrap.WithTx(ctx, db, fn)
}

// boolToInt converts a boolean to an integer for SQLite storage
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool converts an integer from SQLite to a boolean
func intToBool(i int) bool {
	return i > 0
}

// joinWhereClauses joins WHERE clause parts with " AND "
func joinWhereClauses(clauses []string) string {
	result := ""
	for i, clause := range clauses {
		if i > 0 {
			result += " AND "
		}
		result += clause
	}
	return result
}
