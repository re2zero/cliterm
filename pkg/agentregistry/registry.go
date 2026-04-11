// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package agentregistry

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// AgentRegistry provides the service layer for agent management
type AgentRegistry struct {
	store *AgentDB
}

// NewAgentRegistry creates a new AgentRegistry instance
func NewAgentRegistry(store *AgentDB) *AgentRegistry {
	return &AgentRegistry{
		store: store,
	}
}

// RegisterAgent registers a new agent with validation
func (r *AgentRegistry) RegisterAgent(ctx context.Context, agent *Agent) (*Agent, error) {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}

	if agent.Name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	if agent.Role == "" {
		return nil, fmt.Errorf("agent role cannot be empty")
	}

	err := r.store.Create(ctx, agent)
	if err != nil {
		log.Printf("[agentregistry] failed to register agent %s: %v", agent.ID, err)
		return nil, fmt.Errorf("failed to register agent: %w", err)
	}

	log.Printf("[agentregistry] registered agent %s (name: %s, role: %s)", agent.ID, agent.Name, agent.Role)
	return agent, nil
}

// GetAgent retrieves an agent by ID
func (r *AgentRegistry) GetAgent(ctx context.Context, id string) (*Agent, error) {
	agent, err := r.store.Get(ctx, id)
	if err != nil {
		log.Printf("[agentregistry] failed to get agent %s: %v", id, err)
		return nil, fmt.Errorf("agent not found: %s", id)
	}

	return agent, nil
}

// ListAgents retrieves agents according to the provided options
func (r *AgentRegistry) ListAgents(ctx context.Context, opts ListOptions) ([]*Agent, error) {
	agents, err := r.store.List(ctx, opts)
	if err != nil {
		log.Printf("[agentregistry] failed to list agents: %v", err)
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	log.Printf("[agentregistry] listed %d agents", len(agents))
	return agents, nil
}

// UpdateAgent updates an existing agent
func (r *AgentRegistry) UpdateAgent(ctx context.Context, agent *Agent) (*Agent, error) {
	if agent.ID == "" {
		return nil, fmt.Errorf("agent ID cannot be empty")
	}

	if agent.Name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	if agent.Role == "" {
		return nil, fmt.Errorf("agent role cannot be empty")
	}

	err := r.store.Update(ctx, agent)
	if err != nil {
		log.Printf("[agentregistry] failed to update agent %s: %v", agent.ID, err)
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	log.Printf("[agentregistry] updated agent %s (name: %s, role: %s)", agent.ID, agent.Name, agent.Role)
	return agent, nil
}

// UnregisterAgent removes an agent by ID
func (r *AgentRegistry) UnregisterAgent(ctx context.Context, id string) error {
	err := r.store.Delete(ctx, id)
	if err != nil {
		log.Printf("[agentregistry] failed to unregister agent %s: %v", id, err)
		return fmt.Errorf("failed to unregister agent: %w", err)
	}

	log.Printf("[agentregistry] unregistered agent %s", id)
	return nil
}
