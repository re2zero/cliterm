# ClawTeam Data Model Reference

<!-- clawteam:task-statuses -->
## Task Statuses

| Status | Description |
|--------|-------------|
| `pending` | Not yet started, waiting for a worker |
| `in_progress` | Currently being worked on by a worker |
| `completed` | Done (auto-unblocks dependents) |
| `blocked` | Waiting on other tasks (auto-set via `--blocked-by`) |

<!-- clawteam:message-types -->
## Message Types

| Type | Description |
|------|-------------|
| `message` | Point-to-point communication |
| `broadcast` | Sent to all team members |
| `join_request` / `join_approved` / `join_rejected` | Join protocol |
| `plan_approval_request` / `plan_approved` / `plan_rejected` | Plan review |
| `shutdown_request` / `shutdown_approved` / `shutdown_rejected` | Shutdown |
| `idle` | Worker idle notification (no more tasks) |

<!-- clawteam:agent-roles -->
## Agent Roles

| Role | Description |
|------|-------------|
| `leader` | Orchestrates the team: creates tasks, spawns workers, monitors progress, reviews output, integrates results |
| `general-purpose` | Default worker role: executes assigned tasks, reports progress |

<!-- clawteam:file-storage -->
## File Storage Layout

```
~/.clawteam/
├── teams/{team}/
│   ├── config.json          # TeamConfig (name, members, leader)
│   ├── spawn_registry.json  # Spawn backend info (tmux target, wsh block-id, pid)
│   └── inboxes/{agent}/     # msg-{timestamp}-{uuid}.json files
├── tasks/{team}/
│   └── task-{id}.json       # Individual task files
├── workspaces/{team}/{agent}/  # Git worktrees (isolated per worker)
├── sessions/{team}/
│   └── {agent}.json         # Session persistence for resume
├── costs/{team}/            # Cost tracking data
└── plans/
    └── {agent}-{id}.md      # Plan documents
```

<!-- clawteam:env-vars -->
## Environment Variables

| Variable | Description |
|----------|-------------|
| `CLAWTEAM_AGENT_ID` | Unique agent identifier |
| `CLAWTEAM_AGENT_NAME` | Human-readable name (e.g., "worker1", "leader") |
| `CLAWTEAM_AGENT_TYPE` | Role: `leader`, `general-purpose` |
| `CLAWTEAM_TEAM_NAME` | Team name |
| `CLAWTEAM_DATA_DIR` | Override data directory (default: `~/.clawteam`) |
| `CLAWTEAM_WORKSPACE_DIR` | Working directory for the agent's worktree |
| `CLAWTEAM_AGENT_LEADER` | "0" for all spawned agents (internal flag) |

<!-- clawteam:spawn-backends -->
## Spawn Backends

| Backend | Command | Description |
|---------|---------|-------------|
| `auto` (default) | `clawteam spawn --team ...` | Auto-selects: wsh > tmux > subprocess |
| `wsh` | `clawteam spawn wsh --team ...` | TideTerm/WaveTerminal blocks |
| `tmux` | `clawteam spawn tmux --team ...` | Tmux windows (visual monitoring) |
| `subprocess` | `clawteam spawn subprocess --team ...` | Background processes |

<!-- clawteam:stalled-detection -->
## Stalled Worker Detection

A worker is considered "stalled" when:
- Has `in_progress` task but no status update for an extended period
- Terminal output shows idle state (prompt indicator, no active work)
- Sent `idle` notification while tasks are still assigned

**Detection methods:**
- `clawteam board show <team>` — check for stale `in_progress` tasks
- `tmux capture-pane -p -t clawteam-<team>:<name>` — inspect terminal
- `wsh file cat wavefile://<block-id>/term` — inspect terminal (wsh)

For CLI usage, run `clawteam <command> --help` for full options and examples.
