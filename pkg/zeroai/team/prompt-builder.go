// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import (
	"fmt"
	"strings"
)

type PromptBuilderOpts struct {
	AgentName  string
	AgentID    string
	TeamName   string
	LeaderName string
	Role       MemberRole
	Task       string
	WorkDir    string
	Branch     string
	TeamID     string
}

func BuildAgentPrompt(opts PromptBuilderOpts) string {
	var sections []string

	sections = append(sections, buildIdentitySection(opts))
	sections = append(sections, buildWorkspaceSection(opts))

	if opts.Task != "" {
		sections = append(sections, buildTaskSection(opts.Task))
	}

	if opts.Role == MemberRoleLeader {
		sections = append(sections, buildLeaderProtocol(opts))
	} else {
		sections = append(sections, buildWorkerProtocol(opts))
	}

	return strings.Join(sections, "\n\n")
}

func buildIdentitySection(opts PromptBuilderOpts) string {
	return fmt.Sprintf(`## Identity
- Name: %s
- ID: %s
- Team: %s
- Role: %s
- Leader: %s`, opts.AgentName, opts.AgentID, opts.TeamName, opts.Role, opts.LeaderName)
}

func buildWorkspaceSection(opts PromptBuilderOpts) string {
	var lines []string
	lines = append(lines, "## Workspace")
	if opts.WorkDir != "" {
		lines = append(lines, fmt.Sprintf("- Working directory: %s", opts.WorkDir))
	}
	if opts.Branch != "" {
		lines = append(lines, fmt.Sprintf("- Branch: %s", opts.Branch))
	}
	return strings.Join(lines, "\n")
}

func buildTaskSection(task string) string {
	return fmt.Sprintf(`## Task
%s`, task)
}

func buildLeaderProtocol(opts PromptBuilderOpts) string {
	return `## Leader Supervision Loop
You are the team leader. Follow this supervision loop:

1. Check team status — review all member statuses
2. Check inbox — receive messages from workers
3. Process messages (completions, help requests, idle reports)
4. Assign new tasks to available workers
5. Check for stalled workers
6. If all tasks done, proceed to integration
7. Wait 10-30 seconds, then repeat

## Stalled Worker Protocol
- Detect: Check worker block output or last activity
- Nudge: Send a message to wake the worker
- Escalate: Reassign or respawn if no response after 2 nudges

## Integration
1. Review each worker's changes
2. Merge all work
3. Run tests and verify
4. Report completion`
}

func buildWorkerProtocol(opts PromptBuilderOpts) string {
	return fmt.Sprintf(`## Worker Loop Protocol
You MUST keep looping. Do NOT exit after your first task.

1. Check for assigned tasks
2. If task found:
   a. Set status to in_progress
   b. Do the work
   c. Commit changes
   d. Set status to completed
   e. Report to leader (%s)
   f. Go to step 1
3. If no task found:
   a. Check inbox for messages
   b. Process message or wait
   c. If idle for too long: report idle to leader

## Exit Conditions
Only exit when leader sends shutdown request.

## Communication
- Report task progress to leader: %s
- Ask for help when blocked
- Notify leader when idle`, opts.LeaderName, opts.LeaderName)
}

func BuildContextInjection(overlaps []FileOverlap, recentChanges []AgentChange) string {
	var sections []string

	if len(overlaps) > 0 {
		var lines []string
		lines = append(lines, "## File Overlap Warnings")
		for _, o := range overlaps {
			lines = append(lines, fmt.Sprintf("- `%s` also modified by: %s", o.FilePath, strings.Join(o.Agents, ", ")))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(recentChanges) > 0 {
		var lines []string
		lines = append(lines, "## Recent Related Changes")
		for _, c := range recentChanges {
			lines = append(lines, fmt.Sprintf("- [%s] %s \"%s\" (files: %s)",
				c.AgentName, c.CommitHash, c.Message, strings.Join(c.Files, ", ")))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

type FileOverlap struct {
	FilePath string
	Agents   []string
}

type AgentChange struct {
	AgentName  string
	CommitHash string
	Message    string
	Files      []string
}
