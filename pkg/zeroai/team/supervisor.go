// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"context"
	"log"
	"sync"
	"time"
)

// SupervisorConfig configures the supervisor behavior
type SupervisorConfig struct {
	HeartbeatInterval time.Duration // How often to check all members (default: 30s)
	StaleThreshold    time.Duration // Time after which a member is considered stale (default: 5min)
	MaxNudges         int           // Max nudges before declaring agent failed (default: 3)
	MaxRetries        int           // Max recovery retries before marking as failed (default: 3)
}

// DefaultSupervisorConfig returns sensible defaults
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		HeartbeatInterval: 30 * time.Second,
		StaleThreshold:    5 * time.Minute,
		MaxNudges:         3,
		MaxRetries:        3,
	}
}

// Supervisor manages agent health monitoring and automatic recovery
type Supervisor struct {
	mu          sync.Mutex
	config      SupervisorConfig
	coordinator *Coordinator
	blockMgr    *BlockManager
	running     bool
	stopCh      chan struct{}
	doneCh      chan struct{}

	// Per-agent tracking
	activityStates   map[string]*AgentActivityState // agentID -> activity state
	nudgeCounts      map[string]int                 // agentID -> nudge count
	recoveryAttempts map[string]int                 // agentID -> recovery attempt count
}

// NewSupervisor creates a new supervisor
func NewSupervisor(coord *Coordinator, blockMgr *BlockManager, config SupervisorConfig) *Supervisor {
	return &Supervisor{
		config:           config,
		coordinator:      coord,
		blockMgr:         blockMgr,
		activityStates:   make(map[string]*AgentActivityState),
		nudgeCounts:      make(map[string]int),
		recoveryAttempts: make(map[string]int),
	}
}

// Start begins the heartbeat monitoring loop
func (s *Supervisor) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	go s.heartbeatLoop()
}

// Stop halts the heartbeat monitoring
func (s *Supervisor) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	s.running = false
	s.mu.Unlock()

	<-s.doneCh
}

// heartbeatLoop is the main monitoring goroutine
func (s *Supervisor) heartbeatLoop() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAllMembers()
		}
	}
}

// checkAllMembers scans all team members for health issues
func (s *Supervisor) checkAllMembers() {
	teams, err := s.coordinator.ListTeams(ListTeamsOptions{})
	if err != nil {
		log.Printf("[supervisor] failed to list teams: %v", err)
		return
	}

	for _, team := range teams {
		if team.Status != TeamStatusActive {
			continue
		}

		members, err := s.coordinator.GetMembers(team.TeamID)
		if err != nil {
			log.Printf("[supervisor] failed to get members of team %s: %v", team.TeamID, err)
			continue
		}

		for _, member := range members {
			s.checkMemberHealth(team.TeamID, member)
		}
	}
}

// checkMemberHealth checks a single member's health
func (s *Supervisor) checkMemberHealth(teamID string, member *TeamMember) {
	agentID := member.AgentID

	// Initialize activity state if needed
	s.mu.Lock()
	if _, exists := s.activityStates[agentID]; !exists {
		s.activityStates[agentID] = NewAgentActivityState()
	}
	state := s.activityStates[agentID]
	s.mu.Unlock()

	// Update LastActive from member data
	if member.LastActive > 0 {
		state.LastStepTime = time.Unix(member.LastActive, 0)
	}

	// Check if agent is stuck
	if IsStuck(state, s.config.StaleThreshold) {
		s.handleStuckAgent(teamID, agentID, member)
		return
	}

	// Try to update activity state from terminal output
	s.updateActivityFromOutput(agentID, state)

	// Reset nudge count if agent is active
	if state.CurrentStep != StepIdle && state.CurrentStep != StepUnknown {
		s.mu.Lock()
		s.nudgeCounts[agentID] = 0
		s.mu.Unlock()
	}
}

// updateActivityFromOutput reads recent terminal output and updates activity state
func (s *Supervisor) updateActivityFromOutput(agentID string, state *AgentActivityState) {
	block := s.getBlockForAgent(agentID)
	if block == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := s.blockMgr.ReadFromAgent(ctx, agentID, 10)
	if err != nil {
		return // Silently skip — output reading is best-effort
	}

	lines := splitLines(output)
	for _, line := range lines {
		state.UpdateState(line)
	}
}

// handleStuckAgent handles a detected stuck agent
func (s *Supervisor) handleStuckAgent(teamID, agentID string, member *TeamMember) {
	s.mu.Lock()
	nudgeCount := s.nudgeCounts[agentID]
	s.mu.Unlock()

	if nudgeCount >= s.config.MaxNudges {
		// Agent is beyond nudges — attempt recovery
		s.attemptRecovery(teamID, agentID, member)
		return
	}

	// Send a nudge message
	s.nudgeAgent(agentID)
	s.mu.Lock()
	s.nudgeCounts[agentID] = nudgeCount + 1
	s.mu.Unlock()

	log.Printf("[supervisor] agent %s is stuck (nudge %d/%d)", agentID, nudgeCount+1, s.config.MaxNudges)
}

// nudgeAgent sends a wakeup message to an agent
func (s *Supervisor) nudgeAgent(agentID string) {
	if s.blockMgr == nil {
		return
	}
	// Send a simple prompt to check responsiveness
	_ = s.blockMgr.SendToAgent(agentID, "")
}

// attemptRecovery tries to recover a stuck agent
func (s *Supervisor) attemptRecovery(teamID, agentID string, member *TeamMember) {
	s.mu.Lock()
	attempts := s.recoveryAttempts[agentID]
	s.mu.Unlock()

	if attempts >= s.config.MaxRetries {
		log.Printf("[supervisor] agent %s failed after %d recovery attempts, marking as offline", agentID, attempts)
		if err := s.coordinator.UpdateMemberStatus(teamID, agentID, MemberStatusOffline); err != nil {
			log.Printf("[supervisor] failed to update status for %s: %v", agentID, err)
		}
		return
	}

	log.Printf("[supervisor] attempting recovery %d/%d for agent %s", attempts+1, s.config.MaxRetries, agentID)

	// Try sending a "check status" prompt
	statusPrompt := "[SYSTEM] Please report your current status."
	err := s.blockMgr.SendToAgent(agentID, statusPrompt)
	if err != nil {
		log.Printf("[supervisor] failed to send status prompt to %s: %v", agentID, err)
	}

	s.mu.Lock()
	s.recoveryAttempts[agentID] = attempts + 1
	s.mu.Unlock()
}

// getBlockForAgent finds the block associated with an agent
func (s *Supervisor) getBlockForAgent(agentID string) *AgentBlock {
	if s.blockMgr == nil {
		return nil
	}
	block, _ := s.blockMgr.GetBlock(agentID)
	return block
}

// splitLines splits output into lines for processing
func splitLines(output string) []string {
	if output == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(output); i++ {
		if output[i] == '\n' {
			if line := output[start:i]; line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(output) {
		if line := output[start:]; line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// GetAgentActivity returns the current activity state for an agent (for debugging/UI)
func (s *Supervisor) GetAgentActivity(agentID string) *AgentActivityState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.activityStates[agentID]
	if !exists {
		return nil
	}
	// Return a copy to avoid race conditions
	copy := *state
	copy.StepCount = make(map[AgentStep]int)
	for k, v := range state.StepCount {
		copy.StepCount[k] = v
	}
	return &copy
}

// ResetAgentState clears tracking data for an agent (e.g., after respawn)
func (s *Supervisor) ResetAgentState(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activityStates, agentID)
	delete(s.nudgeCounts, agentID)
	delete(s.recoveryAttempts, agentID)
}
