// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package process provides process spawning utilities for ZeroAI agents
package process

import (
	"os/exec"
	"runtime"

	"github.com/wavetermdev/waveterm/pkg/zeroai/protocol"
)

// DefaultProcessManager returns the default process manager for the current platform
func DefaultProcessManager() ProcessManager {
	return NewWSHProcessManager()
}

// BuildAcpCommand builds the CLI command and arguments for a given backend
// Returns: (cliPath, cliArgs)
// Note: Different CLI tools have different command formats
func BuildAcpCommand(backend protocol.AcpBackend) (string, []string) {
	switch backend {
	case protocol.AcpBackendClaude:
		// claude CLI supports --system-prompt
		return "claude", []string{}

	case protocol.AcpBackendQwen:
		// qwen CLI (standard interactive)
		cmd, err := exec.LookPath("qwen")
		if err == nil {
			return cmd, []string{}
		}
		return "", nil

	case protocol.AcpBackendCodex:
		// codex CLI (standard interactive, no system-prompt)
		cmd, err := exec.LookPath("codex")
		if err == nil {
			return cmd, []string{}
		}
		return "", nil

	case protocol.AcpBackendOpenCode:
		// opencode CLI (TUI mode, no arguments needed)
		cmd, err := exec.LookPath("opencode")
		if err == nil {
			return cmd, []string{}
		}
		return "", nil

	case protocol.AcpBackendCustom:
		// Custom backends use configurable command
		return "", nil

	default:
		return "", nil
	}
}

// BuildAgentEnv builds the environment variables for an agent process
func BuildAgentEnv(backend protocol.AcpBackend, yoloMode bool, model string, customEnv map[string]string) map[string]string {
	env := make(map[string]string)

	// Add backend-specific environment variables
	switch backend {
	case protocol.AcpBackendClaude:
		if yoloMode {
			env["ANTHROPIC_YOLO"] = "1"
		}
		// Note: CLAUDE_MODEL should be set via --model flag

	case protocol.AcpBackendQwen:
		// Qwen yolo mode is passed via command line, not env
		// But we may need other env vars in the future

	case protocol.AcpBackendCodex:
		if yoloMode {
			env["GOOSE_MODE"] = "1"
		}

	case protocol.AcpBackendOpenCode:
		// No special env vars for opencode
	}

	// Add custom environment variables
	if customEnv != nil {
		for k, v := range customEnv {
			env[k] = v
		}
	}

	return env
}

// BuildProcessSpec builds a ProcessSpec from ACP configuration
func BuildProcessSpec(backend protocol.AcpBackend, cliPath string, sessionID string, forkSession bool, yoloMode bool, model string, cwd string, env map[string]string) ProcessSpec {
	var cmd string
	var args []string

	// Use explicit cliPath if provided, otherwise build default command
	if cliPath != "" {
		cmd = cliPath
	} else {
		cmd, args = BuildAcpCommand(backend)
	}

	// Build environment
	agentEnv := BuildAgentEnv(backend, yoloMode, model, env)

	return ProcessSpec{
		Command:     cmd,
		Args:        args,
		Cwd:         cwd,
		Env:         agentEnv,
		Backend:     backend,
		SessionID:   sessionID,
		ForkSession: forkSession,
		YoloMode:    yoloMode,
		Model:       model,
	}
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsUnix returns true if running on Unix-like systems
func IsUnix() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin"
}
