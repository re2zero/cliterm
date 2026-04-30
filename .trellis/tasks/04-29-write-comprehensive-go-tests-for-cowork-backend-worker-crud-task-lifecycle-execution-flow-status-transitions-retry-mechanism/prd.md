# Write Comprehensive Go Tests for Cowork Backend

> Parent: cowork

## Overview

Cowork backend has a complete RPC chain (`wshrpctypes.go` → `wshserver.go` → `cowork/cowork_db.go`) but **zero test coverage**. We need comprehensive tests for the DB layer (`pkg/cowork/`) that cover all CRUD operations, state transitions, and edge cases — so the functionality can be verified before frontend integration.

## Key Requirements

### 1. Test Infrastructure (`pkg/cowork/cowork_test.go`)

- **In-memory SQLite DB** setup/teardown per test (no file I/O)
- Migrations applied via `dbfs.WStoreMigrationFS` to create `cowork_tasks`, `cowork_workers`, `cowork_activity` tables
- `context.Background()` for all operations
- Standard `testing.T` pattern matching project conventions (`t.Fatalf`, `t.Errorf`)

### 2. Task CRUD Tests

- **CreateTask**: Auto-generated TaskId, defaults (status=pending, priority=medium), timestamps
- **GetTask**: Found / not-found
- **UpdateTask**: Partial field updates, status transitions (pending→working→done/failed), CompletedAt set on done/failed
- **DeleteTask**: Success / non-existent ID
- **ListTasks**: All / filter by status / filter by priority / both filters / empty result

### 3. Worker CRUD Tests

- **RegisterWorker**: Auto-generated WorkerId, defaults (status=idle), timestamps
- **GetWorker**: Found / not-found
- **UpdateWorker**: All updatable fields (name, tool, status, role, desc, soul, skills, mcpServers, customCmd, capabilities, concurrency, timeout, maxRetries, assignedTask, lastOutputHash, errorMsg, blockId, tabId), LastActiveAt auto-updated
- **DeleteWorker**: Success / non-existent ID
- **ListWorkers**: Multiple workers, ordered by created_at DESC

### 4. Status & State Transition Tests

- **GetStatus**: Counts by task status (pending/working/done/failed), worker status (working/idle)
- **Task state machine**: pending → working → done, pending → working → failed, invalid transitions
- **PauseTask**: Only from working/pending status, rejects done/failed
- **ResumeTask**: Only from paused status, rejects non-paused
- **RetryTask**: Only from failed status, exponential backoff delay, max retries enforcement

### 5. Activity Tests

- **AddActivity**: Creates with timestamp, all fields populated
- **ListActivities**: Default limit (100), custom limit, ordered by created_at DESC
- **CleanupOldActivities**: Deletes oldest entries beyond maxCount

### 6. Edge Cases

- Empty DB queries (ListTasks, ListWorkers, ListActivities)
- Non-existent IDs for Get/Update/Delete
- Task with all optional fields populated vs minimal fields
- Worker with tool="custom" and CustomCmd set
- Concurrent task creation (basic safety)

## Technical Approach

1. Create `pkg/cowork/cowork_test.go` with test helpers
2. Use `sqlx.Open("sqlite3", ":memory:")` for in-memory DB
3. Apply migrations using `github.com/mattes/migrate` or direct SQL execution from `db/migrations-wstore/`
4. Set `wstore.globalDB` for tests via package-level variable or init function
5. Each test is independent (fresh DB or cleanup via transactions)

## Acceptance Criteria

- [ ] `go test ./pkg/cowork/...` passes all tests
- [ ] Coverage of `cowork_db.go` functions ≥ 90%
- [ ] Every DB function has at least 2 test cases (happy path + error/edge case)
- [ ] No external dependencies (no real filesystem, no network)
- [ ] Tests run in < 5 seconds total
