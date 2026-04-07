package team

import (
	"encoding/json"
	"strings"
	"time"
)

// AgentStep represents the current activity step of an agent
type AgentStep string

const (
	StepReadingCode  AgentStep = "Reading code"
	StepImplementing AgentStep = "Implementing"
	StepWritingTests AgentStep = "Writing tests"
	StepTesting      AgentStep = "Testing"
	StepLinting      AgentStep = "Linting"
	StepStaging      AgentStep = "Staging"
	StepCommitting   AgentStep = "Committing"
	StepIdle         AgentStep = "Idle"
	StepUnknown      AgentStep = "Unknown"
)

// AgentActivityState tracks the current step and timing for an agent
type AgentActivityState struct {
	CurrentStep  AgentStep
	LastStepTime time.Time
	StepCount    map[AgentStep]int
	FirstSeen    time.Time
}

// NewAgentActivityState creates a fresh activity state tracker
func NewAgentActivityState() *AgentActivityState {
	return &AgentActivityState{
		CurrentStep:  StepIdle,
		LastStepTime: time.Now(),
		FirstSeen:    time.Now(),
		StepCount:    make(map[AgentStep]int),
	}
}

// UpdateState updates the activity state with a new output line.
// Returns true if the step changed.
func (s *AgentActivityState) UpdateState(line string) bool {
	step := DetectStepFromOutput(line)
	if step == StepUnknown || step == s.CurrentStep {
		return false
	}
	s.CurrentStep = step
	s.LastStepTime = time.Now()
	s.StepCount[step]++
	return true
}

// DetectStepFromOutput analyzes a single output line and returns the detected step.
// Ported from Ralphy base.ts detectStepFromOutput — adapted for Go ACP JSON output.
// Returns StepUnknown for non-matching lines.
func DetectStepFromOutput(line string) AgentStep {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") {
		return StepUnknown
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return StepUnknown
	}

	// Extract fields for pattern matching
	toolName := ""
	if v, ok := parsed["tool"]; ok {
		if s, ok := v.(string); ok {
			toolName = strings.ToLower(s)
		}
	}
	if toolName == "" {
		if v, ok := parsed["name"]; ok {
			if s, ok := v.(string); ok {
				toolName = strings.ToLower(s)
			}
		}
	}
	if toolName == "" {
		if v, ok := parsed["tool_name"]; ok {
			if s, ok := v.(string); ok {
				toolName = strings.ToLower(s)
			}
		}
	}

	command := ""
	if v, ok := parsed["command"]; ok {
		if s, ok := v.(string); ok {
			command = strings.ToLower(s)
		}
	}

	filePath := ""
	if v, ok := parsed["file_path"]; ok {
		if s, ok := v.(string); ok {
			filePath = strings.ToLower(s)
		}
	}
	if filePath == "" {
		if v, ok := parsed["filePath"]; ok {
			if s, ok := v.(string); ok {
				filePath = strings.ToLower(s)
			}
		}
	}
	if filePath == "" {
		if v, ok := parsed["path"]; ok {
			if s, ok := v.(string); ok {
				filePath = strings.ToLower(s)
			}
		}
	}

	description := ""
	if v, ok := parsed["description"]; ok {
		if s, ok := v.(string); ok {
			description = strings.ToLower(s)
		}
	}

	// Reading code
	if toolName == "read" || toolName == "glob" || toolName == "grep" {
		return StepReadingCode
	}

	// Git commit
	if strings.Contains(command, "git commit") || strings.Contains(description, "git commit") {
		return StepCommitting
	}

	// Git staging
	if strings.Contains(command, "git add") || strings.Contains(description, "git add") {
		return StepStaging
	}

	// Linting
	if strings.Contains(command, "lint") || strings.Contains(command, "eslint") ||
		strings.Contains(command, "biome") || strings.Contains(command, "prettier") {
		return StepLinting
	}

	// Testing
	if strings.Contains(command, "vitest") || strings.Contains(command, "jest") ||
		strings.Contains(command, "bun test") || strings.Contains(command, "npm test") ||
		strings.Contains(command, "pytest") || strings.Contains(command, "go test") {
		return StepTesting
	}

	// Writing tests — only for write operations to test files
	if (toolName == "write" || toolName == "edit") && isTestFile(filePath) {
		return StepWritingTests
	}

	// Writing/Editing code
	if toolName == "write" || toolName == "edit" {
		return StepImplementing
	}

	return StepUnknown
}

// isTestFile checks if a file path looks like a test file
func isTestFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "__tests__") ||
		strings.Contains(lower, "_test.go")
}

// IsStuck checks if an agent appears to be stuck based on time since last step change
func IsStuck(state *AgentActivityState, threshold time.Duration) bool {
	if state == nil {
		return false
	}
	return time.Since(state.LastStepTime) > threshold
}
