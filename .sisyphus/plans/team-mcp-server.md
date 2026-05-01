# Team MCP Server Implementation Plan

## Goal

Implement a `wave-team` MCP Server so that **Worker LLMs** (claude/opencode/cursor/aider) can:
1. Update their own task status when done/failed
2. Communicate with other workers via `@mention` dispatch
3. Access project context via `#project` references
4. Query team status, list workers, get project info

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
    │ team_send_prompt to worker terminal│
    └────┬──────────────────────────────┘
         │
    Worker Terminal (claude CLI)
    Worker LLM sees "fix login bug"
         │ does the work
         │ calls MCP tool: team_update_task(status="done")
    ┌────▼──────────────────────────────┐
    │ wsh team-mcp-server (stdio MCP)   │
    │ → Go RPC → updates DB             │
    │ → PublishTaskUpdate WPS event     │
    └───────────────────────────────────┘
         │
    WaveAI receives WPS event → notifies user
```

---

## Implementation Steps

### Step 1: `wsh team-mcp-server` Command

**File**: `cmd/wsh/cmd/wshcmd-team-mcp.go` (new)

Register a new cobra command `team-mcp-server` that:
- Runs as a stdio JSON-RPC server (MCP protocol)
- Reads `WAVE_WORKER_ID` and `WAVE_TASK_ID` from environment
- Implements MCP `tools/list` and `tools/call` handlers
- Routes tool calls to existing Go backend RPC via `wshclient`

**Exposed MCP Tools**:

| Tool | Description | Maps to |
|------|-------------|---------|
| `team_update_task` | Update current task status/result/progress | `team.UpdateTask` via RPC |
| `team_dispatch` | Send message/task to another worker by name | `team_send_prompt` via RPC |
| `team_get_status` | Get full team overview | `team_get_status` via RPC |
| `team_list_workers` | List all active workers | `team_list_workers` via RPC |
| `team_get_project` | Get project details by name | `team_get_projects` + filter |
| `team_send_prompt` | Send prompt to specific worker | `team_send_prompt` via RPC |

**MCP Protocol Implementation**:
- Use `encoding/json` for JSON-RPC 2.0
- Read from stdin, write to stdout (stdio transport)
- No external MCP SDK dependency — keep it simple

**Validation**: Run `wsh team-mcp-server` manually, send JSON-RPC `initialize` + `tools/list` via stdin, verify tool list returned.

### Step 2: Environment Variable Injection

**File**: `pkg/wshrpc/wshserver/wshserver.go` (modify `TeamExecuteTaskCommand`)

When creating the terminal block for task execution, inject environment variables into the block's shell session:

```
WAVE_WORKER_ID=<worker.WorkerID>
WAVE_TASK_ID=<task.TaskID>
WAVE_TEAM_MCP=1
```

**Where to inject**: After creating the block and before sending the CLI start command (around line 1873), send an `export` command:

```go
envInput := &blockcontroller.BlockInputUnion{
    InputData: []byte(fmt.Sprintf(
        "export WAVE_WORKER_ID=%s WAVE_TASK_ID=%s WAVE_TEAM_MCP=1\n",
        worker.WorkerID, task.TaskID,
    )),
}
blockcontroller.SendInput(blockId, envInput)
time.Sleep(200 * time.Millisecond)
```

**Validation**: After task execution starts, check `env | grep WAVE_` in the worker terminal.

### Step 3: Auto-inject `wave-team` MCP Config

**File**: `pkg/team/team_inject.go` (modify `InjectWorkerConfig`)

After existing MCP injection, **always** append the `wave-team` MCP server config:

```go
// Always inject wave-team MCP server (enables worker ↔ team communication)
waveTeamMCP := MCPConfig{
    Name:    "wave-team",
    Type:    "stdio",
    Command: "wsh",
    Args:    []string{"team-mcp-server"},
}
member.McpServers = append(member.McpServers, waveTeamMCP)
if err := injectMCP(member); err != nil {
    return fmt.Errorf("injectMCP wave-team for worker %s: %w", worker.WorkerID, err)
}
// Remove the appended entry to avoid polluting member config
member.McpServers = member.McpServers[:len(member.McpServers)-1]
```

**Why append-then-remove**: `injectMCP` handles the per-tool config file writing (claude → `.claude/mcp/`, opencode → `.config/opencode/mcp.json`). We don't want to permanently store `wave-team` in the member's config.

**Validation**: After forking a worker, check `.claude/mcp/wave-team.json` exists with correct content.

### Step 4: Persona Injection — Task Completion Protocol

**File**: `pkg/team/team_inject.go` (modify `injectPersona` functions)

Append a standardized task completion protocol section to the persona content before injecting:

```
## Task Completion Protocol

You are running as a team worker. When you complete your assigned task:
- Call team_update_task(status="done", result="brief summary of what was accomplished")
- If you encounter an error you cannot resolve, call team_update_task(status="failed", error="description")
- For long tasks, report progress with team_update_task(progress=N) where N is 0-100
- To communicate with another worker, call team_dispatch(target="worker_name", message="your message")
- To check team status, call team_get_status()
```

This is injected via the existing `teamPersonaMarker` mechanism — appended to the persona string before `injectPersonaClaude`/`injectPersonaOpenCode` is called.

**Validation**: Check `CLAUDE.md` content after worker fork contains the task completion protocol section.

### Step 5: WaveAI System Prompt Enhancement

**File**: `pkg/aiusechat/usechat-prompts.go` (modify `SystemPromptText_TeamMode`)

Add a section about `@` and `#` routing to the team mode system prompt:

```
## @mention and #project Routing

When the user's message contains @worker_name or #project_name:
- @worker_name → The user wants to communicate with that worker. Call team_send_prompt(workerid=<resolved>, prompt=<message>)
- #project_name → The user is referring to a project context. Inject project info into the task description.
- @all → Broadcast to all active workers.
Use team_list_workers to resolve names to worker IDs before dispatching.
```

Also add the new `team_dispatch` tool description to the WaveAI tools list (for convenience, even though WaveAI can also call the individual tools directly).

### Step 6: `team_dispatch` Tool for WaveAI (in-process)

**File**: `pkg/aiusechat/tools_team.go` (add new tool)

Add a `team_dispatch` tool that WaveAI can call directly (without going through MCP):

```go
type teamDispatchParams struct {
    Target    string `json:"target"`     // worker name/ID or "all"
    Message   string `json:"message"`    // instruction/message
    ProjectId string `json:"projectid,omitempty"` // optional project context
}
```

Behavior:
1. Resolve `target` by name → find matching worker in `team.ListWorkers`
2. If `target == "all"` → iterate all active workers, send prompt to each
3. If `projectId` provided → prepend project info to message
4. Send via `team_send_prompt` to resolved worker(s)

Register in `GetTeamToolDefinitions()`.

**Validation**: Test via WaveAI chat — type "@coder hello there" and verify worker terminal receives the message.

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `cmd/wsh/cmd/wshcmd-team-mcp.go` | **NEW** | MCP server command + JSON-RPC handler |
| `pkg/wshrpc/wshserver/wshserver.go` | MODIFY | Inject env vars in `TeamExecuteTaskCommand` |
| `pkg/team/team_inject.go` | MODIFY | Auto-inject wave-team MCP + task protocol in persona |
| `pkg/aiusechat/tools_team.go` | MODIFY | Add `team_dispatch` tool definition |
| `pkg/aiusechat/usechat-prompts.go` | MODIFY | Enhance team mode prompt with @/# routing |

---

## Verification Checklist

- [ ] `wsh team-mcp-server` responds to MCP `initialize` + `tools/list`
- [ ] Worker terminal has `WAVE_WORKER_ID` and `WAVE_TASK_ID` env vars
- [ ] `.claude/mcp/wave-team.json` (or opencode equivalent) auto-created on fork
- [ ] Worker LLM can call `team_update_task` via MCP and DB updates
- [ ] Worker LLM can call `team_dispatch` to send message to another worker
- [ ] WaveAI `@worker` in chat → message arrives in worker terminal
- [ ] WaveAI `#project` in chat → project context included in dispatch
- [ ] Task status transitions: pending → assigned → working → done/failed
- [ ] WPS `team:taskupdate` event fires on status change
- [ ] CLAUDE.md contains task completion protocol section

---

## Non-Goals (Out of Scope)

- New message queue/inbox system (use terminal text input directly)
- New RPC commands (reuse all existing team RPCs)
- File-based inter-process communication
- Custom communication protocol beyond MCP stdio
- Changes to worker CLI tools themselves (claude/opencode are unmodified)
