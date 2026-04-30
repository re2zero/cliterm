# Refactor Cowork → Team: Implementation Plan

## Scope Summary

~6,370 lines across 44+ files. Break into 3 phases, Phase 1 first.

## Affected Files Inventory

### Backend Go Files (12 Go files referencing cowork)

| File | Action | Lines Affected |
|------|--------|---------------|
| `db/migrations-wstore/0000015_team.up.sql` | **CREATE** | ~80 |
| `db/migrations-wstore/0000015_team.down.sql` | **CREATE** | ~20 |
| `pkg/team/team_types.go` | **CREATE** | ~120 |
| `pkg/team/team_db.go` | **CREATE** | ~400 |
| `pkg/team/team_state.go` | **CREATE** | ~80 |
| `pkg/team/team_fork.go` | **CREATE** | ~100 |
| `pkg/team/team_inject.go` | **CREATE** | ~150 |
| `pkg/team/team_heartbeat.go` | **CREATE** | ~80 |
| `pkg/team/team_config.go` | **CREATE** | ~200 |
| `pkg/team/team_test.go` | **CREATE** | ~500 |
| `pkg/wshrpc/wshrpctypes.go` | **MODIFY** | Remove Cowork types (L961-1131), add Team types |
| `pkg/wshrpc/wshserver/wshserver.go` | **MODIFY** | Remove Cowork handlers, add Team handlers |
| `pkg/wshrpc/wshclient/wshclient.go` | **MODIFY** | Remove Cowork client, add Team client |
| `pkg/wps/wpstypes.go` | **MODIFY** | L37-38: rename 2 events, add 1 |
| `pkg/tsgen/tsgenevent.go` | **MODIFY** | L44-45: rename 2 events |
| `pkg/aiusechat/usechat-prompts.go` | **MODIFY** | L93: rewrite system prompt |
| `pkg/aiusechat/usechat.go` | **MODIFY** | L65, L637, L697, L702, L725-726: CoworkMode → TeamMode |
| `pkg/aiusechat/uctypes/uctypes.go` | **MODIFY** | L317, L510: CoworkMode → TeamMode |
| `pkg/aiusechat/tools_team.go` | **CREATE** | ~700 |
| `pkg/waveobj/objrtinfo.go` | **MODIFY** | L29: rename field |
| `pkg/cowork/` | **DELETE** | Entire directory (3 files) |
| `pkg/aiusechat/tools_cowork.go` | **DELETE** | ~740 lines |

### Frontend Files

| File | Action | Notes |
|------|--------|-------|
| `frontend/app/view/team/` | **CREATE** | Full rewrite from cowork |
| `frontend/app/block/blockregistry.ts` | **MODIFY** | cowork → team |
| `frontend/app/aipanel/cowork-workers-panel.tsx` | **DELETE** | Replaced by team-panel.tsx |
| `frontend/app/aipanel/*.tsx` | **MODIFY** | ~5 files with cowork refs |
| `frontend/app/workspace/widgets.tsx` | **MODIFY** | CoworkMode refs |
| `frontend/app/view/cowork/` | **DELETE** | Entire directory |
| Generated files | **AUTO** | via `task generate` |

### Key Differences from PRD (from codebase analysis)

1. **Existing cowork DB schema** uses `cowork_tasks`, `cowork_workers`, `cowork_activity` tables (migration 0000012). New migration 0000015 creates `team_*` tables. Old migration preserved per PRD.

2. **CoworkWorker** has many fields not in PRD's TeamWorker: `Role`, `Desc`, `Soul`, `Skills`(string), `McpServers`(string), `LastOutputHash`, `ErrorMsg`, `Concurrency`, `Timeout`, `MaxRetries`, `CompletedTasks`. These become Member attributes (not Worker).

3. **WPS events** are registered in BOTH `wpstypes.go` and `tsgen/tsgenevent.go` — both must be updated.

4. **No testify/gomock** — tests use raw `testing` package only.

5. **DB helpers**: `wstore.WithTx()`, `wstore.GetGlobalDB()`, `wstore.SetGlobalDBForTest()`. Migrations via `migrateutil.Migrate()`.

---

## Phase 1: Backend Foundation

### Batch 1: Foundation (Sequential — everything depends on these)

**1.1 DB Migration**
- Create `db/migrations-wstore/0000015_team.up.sql` with 4 tables: `team_members`, `team_workers`, `team_tasks`, `team_activity`
- Create `db/migrations-wstore/0000015_team.down.sql` for rollback
- Key schema changes from old cowork:
  - `team_members` is NEW (replaces worker attributes)
  - `team_workers.member_id` FK references `team_members`
  - `team_tasks.assigned_member_id` + `assigned_worker_id` (was just `assigned_worker`)
  - `team_tasks.depends_on` JSON array (new)
  - `team_tasks.output_history` JSON array (new)
  - `team_tasks.retry_count`, `next_retry_at` (new)
  - `team_workers.pid`, `last_heartbeat` (new for health checks)
  - `team_activity.member_id` (new)

**1.2 pkg/team/team_types.go**
- Define all types: TeamMember, TeamWorker, TeamTask, TeamActivity, MCPConfig, TaskOutput, TeamStatusData, TeamCreateMemberData, TeamUpdateMemberData, TeamForkWorkerData, etc.
- Use same JSON tag style as existing codebase (lowercase, no omitempty on required fields)
- Use `int64` for timestamps (consistent with existing cowork code)

### Batch 2: Core Package (Parallel — depends on Batch 1)

**1.3 pkg/team/team_db.go** — CRUD layer
- Member CRUD: Create/Get/Update/Delete/ListMembers
- Worker CRUD: Create/Get/Update/Delete/ListWorkers (filter by member_id)
- Task CRUD: Create/Get/Update/Delete/ListTasks (filter by status/priority/member)
- Activity: Add/List (filter by task/worker/member, limit)
- GetStatus: aggregate query
- All WPS event publishing calls
- Use existing patterns from `pkg/cowork/cowork_db.go`

**1.4 pkg/team/team_state.go** — State machine
- ValidTaskTransitions map
- ValidWorkerTransitions map
- ValidateTaskTransition / ValidateWorkerTransition functions

**1.5 pkg/team/team_fork.go** — Worker fork logic
- ForkWorker: check MaxConcurrency, generate runtime name, create DB entry, record Activity
- RecycleWorker: update status, record Activity
- GetNextWorkerNumber: atomic counter for "{MemberName}-{N}" naming

**1.6 pkg/team/team_inject.go** — CLI config injection
- InjectWorkerConfig: dispatch to tool-specific injection
- loadPersona: read personaPath or use inline persona
- linkSkills: create symlinks from team-skills to CLI native dirs
- unlinkSkills: cleanup symlinks
- injectMCP: create MCP config files per CLI tool type

**1.7 pkg/team/team_heartbeat.go** — Worker health check
- CheckWorkerHealth: query workers with status=working, check PID via os.FindProcess, mark offline if dead
- StartHeartbeatLoop: periodic check (goroutine with ticker)

**1.8 pkg/team/team_config.go** — Config file system
- LoadGlobalTemplates: read ~/.waveterm/team-templates/*.yaml
- LoadProjectConfig: read .wave/team.yaml
- ParseMemberYAML: unmarshal YAML to TeamMember
- GetEffectiveConfig: merge project + global
- Default templates: 4 built-in members

### Batch 3: RPC + Integration (Parallel — depends on Batch 2)

**1.9 pkg/wshrpc/wshrpctypes.go** — Remove Cowork, add Team
- Remove all Cowork* types (L961-1131)
- Remove all Cowork* methods from WshRPCServer interface (L187-204)
- Add Team* types (TeamMember, TeamWorker, TeamTask, TeamActivity, MCPConfig, TaskOutput, TeamStatusData, all Data/Return types)
- Add Team* methods to WshRPCServer interface
- New RPC commands: TeamCreateMember, TeamGetMember, TeamUpdateMember, TeamDeleteMember, TeamListMembers, TeamForkWorker, TeamRecycleWorker, TeamExecuteTask, etc.

**1.10 pkg/wshrpc/wshserver/wshserver.go** — Team handlers
- Remove Cowork handler registrations
- Add Team handler implementations (delegate to pkg/team/*)
- Register handlers in init()

**1.11 pkg/wshrpc/wshclient/wshclient.go** — Team client stubs
- Remove Cowork client methods
- Add Team client methods (follow existing pattern)

**1.12 WPS Events**
- `pkg/wps/wpstypes.go`: Event_CoworkTaskUpdate → Event_TeamTaskUpdate, Event_CoworkWorkerUpdate → Event_TeamWorkerUpdate, add Event_TeamMemberUpdate
- `pkg/tsgen/tsgenevent.go`: update event registrations

**1.13 AI Tools — pkg/aiusechat/tools_team.go**
- 15 tools: team_fork_worker, team_list_workers, team_list_members, team_create_member, team_update_member, team_delete_member, team_create_task, team_assign_task, team_update_task, team_get_status, team_execute_task, team_recycle_worker, team_send_prompt, team_get_task_output, team_list_activity
- Follow existing callback pattern from tools_cowork.go
- Use pkg/team/ instead of pkg/cowork/

**1.14 System Prompt + Mode**
- `pkg/aiusechat/usechat-prompts.go`: SystemPromptText_CoworkMode → SystemPromptText_TeamMode (new content from PRD)
- `pkg/aiusechat/usechat.go`: CoworkMode → TeamMode
- `pkg/aiusechat/uctypes/uctypes.go`: CoworkMode → TeamMode
- `pkg/waveobj/objrtinfo.go`: WaveAICoworkMode → WaveAITeamMode

### Batch 4: Cleanup + Verification

**1.15 Delete old code**
- `pkg/cowork/` — entire directory
- `pkg/aiusechat/tools_cowork.go`

**1.16 Codegen**
- Run `task generate` to update gotypes.d.ts, wshclientapi.ts, waveevent.d.ts

**1.17 Tests**
- Copy and adapt `pkg/cowork/cowork_test.go` → `pkg/team/team_test.go`
- Add new tests for: Member CRUD, state machine validation, fork logic, config parsing
- Run `go test ./pkg/team/...`

---

## Phase 2: Frontend Rewrite

### Batch 5: Frontend (after task generate)

**2.1 Create frontend/app/view/team/**
- team-model.ts (TeamViewModel — Jotai singleton)
- team.tsx (TeamView)
- team-types.ts
- board-view.tsx, board-column.tsx, board-card.tsx
- member-panel.tsx (replaces worker-panel + worker-sidebar + worker-config-dialog)
- task-detail.tsx

**2.2 Update integrations**
- blockregistry.ts: cowork → team
- aipanel/ files: CoworkMode → TeamMode
- Delete cowork-workers-panel.tsx, create team-panel.tsx
- workspace widgets: CoworkMode → TeamMode
- Delete frontend/app/view/cowork/

**2.3 Verification**
- `task check:ts` — typecheck passes
- Manual review of all imports

---

## Phase 3: Configuration

**3.1 Template system** — depends on Phase 1 pkg/team/team_config.go
**3.2 UI for templates** — depends on Phase 2 frontend

---

## Parallel Execution Strategy

```
Batch 1 (Sequential):
  1.1 DB migration ──► 1.2 team_types.go

Batch 2 (All parallel, after Batch 1):
  1.3 team_db.go     ┐
  1.4 team_state.go  │
  1.5 team_fork.go   ├─ All independent, only depend on types
  1.6 team_inject.go │
  1.7 team_heartbeat │
  1.8 team_config.go ┘

Batch 3 (After Batch 2):
  1.9  wshrpctypes.go  ──► (1.10 + 1.11 parallel)
  1.12 WPS events       ┐
  1.13 AI tools         ├─ Parallel after 1.9
  1.14 System prompt    ┘

Batch 4 (After Batch 3):
  1.15 Delete old code ──► 1.16 task generate ──► 1.17 Tests

Batch 5 (After Batch 4):
  Phase 2 frontend (parallel subtasks)
```

## Risk Areas

1. **team_fork.go**: Terminal block creation requires understanding of existing block/controller system — may need cowork/wshserver exploration
2. **team_inject.go**: Symlink operations and MCP config formats vary by CLI tool — start simple, iterate
3. **tools_team.go**: Most complex file (~700 lines) — needs careful delegation
4. **Frontend Jotai model**: Must follow singleton pattern exactly — read existing models for reference
