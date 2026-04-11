// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package agentregistry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testStore *AgentDB

func setupTestDB(t *testing.T) {
	// Create a temporary in-memory database for testing
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create the agents table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			soul TEXT,
			skills TEXT,
			mcp_connections TEXT,
			config TEXT,
			enabled INTEGER DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	require.NoError(t, err)

	testStore = &AgentDB{db: db}
}

func teardownTestDB(t *testing.T) {
	if testStore != nil && testStore.db != nil {
		testStore.db.Close()
	}
	testStore = nil
}

func createTestAgent(id, name, role string, enabled bool) *Agent {
	return &Agent{
		ID:      id,
		Name:    name,
		Role:    role,
		Soul:    "test-agent-soul",
		Skills:  []string{"test-skill-1", "test-skill-2"},
		Config:  map[string]any{"key1": "value1", "key2": 42},
		Enabled: enabled,
	}
}

func TestAgentStore_CreateAndGet(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	agent := createTestAgent("agent-001", "Test Agent", "developer", true)

	err := testStore.Create(context.Background(), agent)
	require.NoError(t, err)
	assert.Equal(t, "agent-001", agent.ID)
	assert.NotZero(t, agent.CreatedAt)
	assert.NotZero(t, agent.UpdatedAt)
	assert.Equal(t, agent.CreatedAt, agent.UpdatedAt)

	// Get the agent back
	retrieved, err := testStore.Get(context.Background(), "agent-001")
	require.NoError(t, err)
	assert.Equal(t, "agent-001", retrieved.ID)
	assert.Equal(t, "Test Agent", retrieved.Name)
	assert.Equal(t, "developer", retrieved.Role)
	assert.Equal(t, "test-agent-soul", retrieved.Soul)
	assert.Equal(t, []string{"test-skill-1", "test-skill-2"}, retrieved.Skills)
	assert.Equal(t, map[string]any{"key1": "value1", "key2": float64(42)}, retrieved.Config)
	assert.True(t, retrieved.Enabled)
}

func TestAgentStore_ListAgents(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()
	now := time.Now().UnixMilli()

	// Create multiple agents
	agents := []*Agent{
		{ID: "agent-001", Name: "Agent One", Role: "developer", Enabled: true, CreatedAt: now + 3, UpdatedAt: now + 3},
		{ID: "agent-002", Name: "Agent Two", Role: "tester", Enabled: true, CreatedAt: now + 2, UpdatedAt: now + 2},
		{ID: "agent-003", Name: "Agent Three", Role: "developer", Enabled: false, CreatedAt: now + 1, UpdatedAt: now + 1},
	}

	for _, agent := range agents {
		err := testStore.Create(ctx, agent)
		require.NoError(t, err)
	}

	// List all agents
	list, err := testStore.List(ctx, ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 3, len(list))

	// Verify order is descending by created_at (most recent first)
	assert.Equal(t, "agent-001", list[0].ID)
	assert.Equal(t, "agent-002", list[1].ID)
	assert.Equal(t, "agent-003", list[2].ID)
}

func TestAgentStore_UpdateAgent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create an agent
	agent := createTestAgent("agent-001", "Original Name", "developer", true)
	originalCreatedAt := time.Now().UnixMilli()
	agent.CreatedAt = originalCreatedAt
	agent.UpdatedAt = originalCreatedAt

	err := testStore.Create(ctx, agent)
	require.NoError(t, err)

	// Update the agent
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	agent.Name = "Updated Name"
	agent.Role = "tester"
	agent.Skills = append(agent.Skills, "new-skill")

	err = testStore.Update(ctx, agent)
	require.NoError(t, err)

	// Verify the update
	retrieved, err := testStore.Get(ctx, "agent-001")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
	assert.Equal(t, "tester", retrieved.Role)
	assert.Equal(t, 3, len(retrieved.Skills))
	assert.Contains(t, retrieved.Skills, "new-skill")
	assert.Equal(t, originalCreatedAt, retrieved.CreatedAt) // CreatedAt should not change
	assert.Greater(t, retrieved.UpdatedAt, originalCreatedAt) // UpdatedAt should increase
}

func TestAgentStore_DeleteAgent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create an agent
	agent := createTestAgent("agent-001", "Test Agent", "developer", true)
	err := testStore.Create(ctx, agent)
	require.NoError(t, err)

	// Verify it exists
	_, err = testStore.Get(ctx, "agent-001")
	require.NoError(t, err)

	// Delete the agent
	err = testStore.Delete(ctx, "agent-001")
	require.NoError(t, err)

	// Verify it's gone
	_, err = testStore.Get(ctx, "agent-001")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

func TestAgentStore_GetNonExistent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Try to get a non-existent agent
	_, err := testStore.Get(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

func TestAgentStore_UpdateNonExistent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Try to update a non-existent agent
	agent := createTestAgent("non-existent-id", "Test Agent", "developer", true)
	err := testStore.Update(ctx, agent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

func TestAgentStore_DeleteNonExistent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Try to delete a non-existent agent
	err := testStore.Delete(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

func TestAgentStore_EnabledFilter(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create agents with different enabled states
	enabled := true
	for i := 0; i < 3; i++ {
		agent := createTestAgent(string(rune('a'+i)), "Agent", "developer", enabled)
		err := testStore.Create(ctx, agent)
		require.NoError(t, err)
		enabled = !enabled
	}

	// List only enabled agents
	enabledTrue := true
	list, err := testStore.List(ctx, ListOptions{Enabled: &enabledTrue})
	require.NoError(t, err)
	// Should have 2 enabled agents (created with true, true, false pattern)
	for _, agent := range list {
		assert.True(t, agent.Enabled)
	}

	// List only disabled agents
	enabledFalse := false
	list, err = testStore.List(ctx, ListOptions{Enabled: &enabledFalse})
	require.NoError(t, err)
	// Should have 1 disabled agent
	for _, agent := range list {
		assert.False(t, agent.Enabled)
	}
}

func TestAgentStore_Timestamps(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create an agent and note the timestamps
	agent := createTestAgent("agent-001", "Test Agent", "developer", true)
	err := testStore.Create(ctx, agent)
	require.NoError(t, err)

	createdAt := agent.CreatedAt
	updatedAt := agent.UpdatedAt
	assert.NotZero(t, createdAt)
	assert.NotZero(t, updatedAt)
	assert.Equal(t, createdAt, updatedAt)

	// Wait a bit and update
	time.Sleep(10 * time.Millisecond)
	agent.Name = "Updated"

	err = testStore.Update(ctx, agent)
	require.NoError(t, err)

	// Verify timestamps changed correctly
	retrieved, err := testStore.Get(ctx, "agent-001")
	require.NoError(t, err)
	assert.Equal(t, createdAt, retrieved.CreatedAt) // CreatedAt unchanged
	assert.Greater(t, retrieved.UpdatedAt, updatedAt) // UpdatedAt increased
}

func TestAgentStore_SkillsAndMCPConnections(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create an agent with skills and MCP connections
	agent := &Agent{
		ID:      "agent-001",
		Name:    "Test Agent",
		Role:    "developer",
		Skills:  []string{"typescript", "golang", "testing"},
		MCPConnections: []MCPConnection{
			{ServerName: "github", Config: map[string]any{"token": "ghp_test"}},
			{ServerName: "linear", Config: map[string]any{"apiKey": "lin_test"}},
		},
		Enabled: true,
	}

	err := testStore.Create(ctx, agent)
	require.NoError(t, err)

	// Retrieve and verify
	retrieved, err := testStore.Get(ctx, "agent-001")
	require.NoError(t, err)
	assert.Equal(t, []string{"typescript", "golang", "testing"}, retrieved.Skills)
	assert.Equal(t, 2, len(retrieved.MCPConnections))
	assert.Equal(t, "github", retrieved.MCPConnections[0].ServerName)
	assert.Equal(t, map[string]any{"token": "ghp_test"}, retrieved.MCPConnections[0].Config)
	assert.Equal(t, "linear", retrieved.MCPConnections[1].ServerName)
}

func TestAgentStore_RoleFilter(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create agents with different roles
	agents := []*Agent{
		createTestAgent("agent-001", "Agent 1", "developer", true),
		createTestAgent("agent-002", "Agent 2", "tester", true),
		createTestAgent("agent-003", "Agent 3", "developer", true),
		createTestAgent("agent-004", "Agent 4", "reviewer", true),
	}

	for _, agent := range agents {
		err := testStore.Create(ctx, agent)
		require.NoError(t, err)
	}

	// Filter by developer role
	list, err := testStore.List(ctx, ListOptions{Role: "developer"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(list))

	// Verify all are developers
	for _, agent := range list {
		assert.Equal(t, "developer", agent.Role)
	}
}

func TestAgentStore_LimitAndOffset(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create multiple agents
	for i := 0; i < 5; i++ {
		agent := createTestAgent(string(rune('a'+i)), "Agent", "developer", true)
		err := testStore.Create(ctx, agent)
		require.NoError(t, err)
	}

	// Test limit
	list, err := testStore.List(ctx, ListOptions{Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 2, len(list))

	// Test offset
	list, err = testStore.List(ctx, ListOptions{Offset: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, len(list)) // 5 total - 2 offset

	// Test limit with offset
	list, err = testStore.List(ctx, ListOptions{Offset: 1, Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 2, len(list))
}

func TestAgentStore_BoolToInt(t *testing.T) {
	assert.Equal(t, 1, boolToInt(true))
	assert.Equal(t, 0, boolToInt(false))
}

func TestAgentStore_IntToBool(t *testing.T) {
	assert.True(t, intToBool(1))
	assert.True(t, intToBool(100))
	assert.False(t, intToBool(0))
	assert.False(t, intToBool(-1))
}

func TestAgentStore_ConfigHandling(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create agent with complex config
	agent := &Agent{
		ID:     "agent-001",
		Name:   "Test Agent",
		Role:   "developer",
		Config: map[string]any{
			"string": "value",
			"number": 42,
			"bool":   true,
			"nested": map[string]any{
				"key": "nested-value",
			},
			"array": []interface{}{1, 2, 3},
		},
		Enabled: true,
	}

	err := testStore.Create(ctx, agent)
	require.NoError(t, err)

	// Retrieve and verify config persists correctly
	retrieved, err := testStore.Get(ctx, "agent-001")
	require.NoError(t, err)

	assert.Equal(t, "value", retrieved.Config["string"])
	assert.Equal(t, float64(42), retrieved.Config["number"]) // JSON numbers become float64
	assert.Equal(t, true, retrieved.Config["bool"])

	nested, ok := retrieved.Config["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nested-value", nested["key"])

	arr, ok := retrieved.Config["array"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 3, len(arr))
}

func TestAgentStore_SoulOptional(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Create agent without soul
	agent := &Agent{
		ID:      "agent-001",
		Name:    "Test Agent",
		Role:    "developer",
		Soul:    "",
		Enabled: true,
	}

	err := testStore.Create(ctx, agent)
	require.NoError(t, err)

	// Retrieve and verify soul is empty
	retrieved, err := testStore.Get(ctx, "agent-001")
	require.NoError(t, err)
	assert.Equal(t, "", retrieved.Soul)
}

func TestAgentStore_CombinationFilter(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	agents := []*Agent{
		{ID: "agent-001", Name: "A1", Role: "developer", Enabled: true},
		{ID: "agent-002", Name: "A2", Role: "developer", Enabled: false},
		{ID: "agent-003", Name: "A3", Role: "tester", Enabled: true},
		{ID: "agent-004", Name: "A4", Role: "tester", Enabled: false},
	}

	for _, agent := range agents {
		err := testStore.Create(ctx, agent)
		require.NoError(t, err)
	}

	// Filter by role AND enabled
	enabled := true
	list, err := testStore.List(ctx, ListOptions{Role: "developer", Enabled: &enabled})
	require.NoError(t, err)
	assert.Equal(t, 1, len(list))
	assert.Equal(t, "agent-001", list[0].ID)

	// Filter by role AND disabled
	enabled = false
	list, err = testStore.List(ctx, ListOptions{Role: "tester", Enabled: &enabled})
	require.NoError(t, err)
	assert.Equal(t, 1, len(list))
	assert.Equal(t, "agent-004", list[0].ID)
}

// Ensure test doesn't interfere with other tests that may use environment variables
func TestMain(m *testing.M) {
	// Save original environment variables
	originalEnv := make(map[string]string)
	for _, env := range []string{"WAVE_DATA_DIR"} {
		originalEnv[env] = os.Getenv(env)
	}

	// Run tests
	code := m.Run()

	// Restore environment variables
	for key, value := range originalEnv {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}

	os.Exit(code)
}
