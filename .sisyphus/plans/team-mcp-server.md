# Team MCP Server Implementation Plan

## Goal

Implement a `wave-team` MCP Server (`wsh mcp --tools=team`) so that **Worker LLMs** (claude/opencode/cursor) can:
1. Update their own task status when done/failed
2. Communicate with other workers via `@mention` dispatch
3. Access team status and worker list
4. Do all of the above through MCP tools (always visible to LLM, never forgotten)

This creates a **bidirectional communication channel** between all agents in the team.

---

## Architecture

```
User types "@coder fix login bug"
         │
    WaveAI (in-process) parses @coder
         │ calls team_dispatch tool
    ┌────▼──────────────────────────────┐
    │ Wave Backend (Go RPC)             │
    │ resolves "coder" → workerID      │
    │ TeamSendPromptCommand to terminal │
    └────┬──────────────────────────────┘
         │
    Worker Terminal (claude CLI)
    Worker LLM sees "fix login bug"
         │ does the work
         │ calls MCP tool: team_update_task(status="done")
    ┌────▼──────────────────────────────┐
    │ wsh mcp --tools=team (stdio MCP)  │
    │ → Go RPC → updates DB             │
    │ → PublishTaskUpdate WPS event     │
    └───────────────────────────────────┘
         │
    WaveAI receives WPS event → notifies user
```

### Key Design Decision: MCP over CLI

**MCP** was chosen over CLI commands because:
- MCP tools are **always visible** to the LLM in its tool list — never forgotten even in long conversations
- MCP tools have **structured JSON Schema** — LLMs call them correctly
- MCP tools provide **structured responses** — errors are clear
- MCP is **natively supported** by claude, opencode, and other modern AI tools

---

## Implementation Steps

### Step 1: `wsh mcp --tools=team` Command ✅

**File**: `cmd/wsh/cmd/wshcmd-mcp.go` (new)

Register a cobra command `mcp` with `--tools` flag that:
- Runs as a stdio JSON-RPC server (MCP protocol 2024-11-05)
- Reads `WAVE_WORKER_ID` and `WAVE_TASK_ID` from environment
- Implements MCP `initialize`, `tools/list`, and `tools/call` handlers
- Routes tool calls to existing Go backend RPC via `wshclient`
- Extensible: future tool groups can be added (e.g., `--tools=team,file`)

**Exposed MCP Tools**:

| Tool | Description | Maps to |
|------|-------------|---------|
| `team_update_task` | Update current task status/result/progress | `TeamUpdateTaskCommand` RPC |
| `team_dispatch` | Send message to worker by name or "all" | `TeamSendPromptCommand` RPC |
| `team_get_status` | Get team overview (counts) | `TeamGetStatusCommand` RPC |
| `team_list_workers` | List all workers with status | `TeamListWorkersCommand` RPC |

### Step 2: Environment Variable Injection ✅

**File**: `pkg/wshrpc/wshserver/wshserver.go` (modify `TeamExecuteTaskCommand`)

Inject env vars into worker terminal before CLI starts:
```
WAVE_WORKER_ID=<worker.WorkerID>
WAVE_TASK_ID=<task.TaskID>
WAVE_TEAM_MCP=1
```

### Step 3: Auto-inject `wave-team` MCP Config ✅

**File**: `pkg/team/team_inject.go` (modify `InjectWorkerConfig`)

Always inject `wave-team` MCP server config into Worker's tool config:
```json
{
  "command": "wsh",
  "args": ["mcp", "--tools=team"]
}
```

Written to `~/.claude/mcp/wave-team.json` (claude) or equivalent paths.

### Step 4: Persona Injection — Task Completion Protocol ✅

**File**: `pkg/team/team_inject.go` (modify `InjectWorkerConfig`)

Append task completion protocol to persona, even when persona is empty:
```
## Task Completion Protocol
- Use team_update_task MCP tool with status="done"/"failed"
- Use team_dispatch to communicate with other workers
```

### Step 5: WaveAI System Prompt Enhancement ✅

**File**: `pkg/aiusechat/usechat-prompts.go`

Added `@mention` and `#project` routing instructions. WaveAI uses `team_dispatch` tool.

### Step 6: `team_dispatch` Tool for WaveAI (in-process) ✅

**File**: `pkg/aiusechat/tools_team.go`

WaveAI calls `team_dispatch` directly (in-process, no MCP needed).

### Step 7: `TeamSendPromptCommand` RPC ✅ (NEW)

**Files**: `wshrpctypes.go`, `wshclient.go`, `wshserver.go`

New RPC command that sends text input to a worker's terminal block by workerID.
Enables MCP server (out-of-process) to dispatch messages to workers.

### Step 8: Build Fixes ✅ (NEW)

- Added missing `wshclient` stubs for jobmanager (`StreamDataCommand`, `JobCmdExitedCommand`, `JobController*`)
- Completed `JobManagerStatusUpdate` type with missing fields
- Excluded broken `wshcmd-jobdebug.go` with `//go:build ignore`
- Fixed `tools_team.go` reference to non-existent `resp.Error`

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `cmd/wsh/cmd/wshcmd-mcp.go` | **NEW** | MCP server command (`wsh mcp --tools=team`) |
| `cmd/wsh/cmd/wshcmd-jobdebug.go` | MODIFY | Excluded with `//go:build ignore` (pre-existing build errors) |
| `pkg/wshrpc/wshrpctypes.go` | MODIFY | Added `TeamSendPromptCommand` interface + `TeamSendPromptData` type, completed `JobManagerStatusUpdate` |
| `pkg/wshrpc/wshclient/wshclient.go` | MODIFY | Added `TeamSendPromptCommand` + jobmanager stubs |
| `pkg/wshrpc/wshserver/wshserver.go` | MODIFY | Added `TeamSendPromptCommand` handler, env var injection |
| `pkg/team/team_inject.go` | MODIFY | Auto-inject wave-team MCP + task protocol in persona + default worker config |
| `pkg/team/team_inject_test.go` | MODIFY | Tests for persona injection, MCP auto-inject, member mutation guard |
| `pkg/aiusechat/tools_team.go` | MODIFY | Added `team_dispatch` tool, fixed `resp.Error` reference |
| `pkg/aiusechat/usechat-prompts.go` | MODIFY | @/# routing instructions, team_dispatch in tool list |

---

## Verification Checklist

- [x] `wsh mcp --tools=team` compiles and registers as cobra command
- [ ] `wsh mcp --tools=team` responds to MCP `initialize` + `tools/list` (needs Wave JWT)
- [ ] Worker terminal has `WAVE_WORKER_ID` and `WAVE_TASK_ID` env vars after task execution
- [ ] `.claude/mcp/wave-team.json` auto-created on worker fork
- [ ] Worker LLM sees `team_update_task` tool in `/mcp` listing
- [ ] Worker LLM can call `team_update_task` and DB updates
- [ ] Worker LLM can call `team_dispatch` to send message to another worker
- [ ] WaveAI `@worker` in chat → message arrives in worker terminal
- [ ] WaveAI `#project` in chat → project context included in dispatch
- [ ] CLAUDE.md contains task completion protocol section

---

## Non-Goals (Out of Scope)

- New message queue/inbox system (use terminal text input directly)
- File-based inter-process communication
- Changes to worker CLI tools themselves (claude/opencode are unmodified)
- CLI command approach (replaced by MCP for better LLM tool visibility)
