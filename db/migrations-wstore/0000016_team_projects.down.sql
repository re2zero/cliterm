-- Drop indexes
DROP INDEX IF EXISTS idx_team_members_project;
DROP INDEX IF EXISTS idx_team_workers_project;

-- Remove project_id column from team_workers (SQLite doesn't support DROP COLUMN, so we recreate table)
CREATE TABLE IF NOT EXISTS team_workers_new (
    worker_id TEXT PRIMARY KEY,
    member_id TEXT NOT NULL REFERENCES team_members(member_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'idle' CHECK(status IN ('idle', 'working', 'offline', 'error')),
    assigned_task_id TEXT DEFAULT '',
    block_id TEXT DEFAULT '',
    tab_id TEXT DEFAULT '',
    pid INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    last_heartbeat INTEGER NOT NULL DEFAULT (unixepoch())
);

INSERT INTO team_workers_new SELECT worker_id, member_id, name, status, assigned_task_id, block_id, tab_id, pid, created_at, updated_at, last_heartbeat FROM team_workers;
DROP TABLE team_workers;
ALTER TABLE team_workers_new RENAME TO team_workers;

-- Remove project_id column from team_members (SQLite doesn't support DROP COLUMN, so we recreate table)
CREATE TABLE IF NOT EXISTS team_members_new (
    member_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    tool TEXT NOT NULL DEFAULT 'claude' CHECK(tool IN ('claude', 'opencode', 'cursor', 'aider', 'custom')),
    custom_cmd TEXT DEFAULT '',
    description TEXT DEFAULT '',
    persona TEXT DEFAULT '',
    persona_path TEXT DEFAULT '',
    skills TEXT DEFAULT '[]',
    mcp_servers TEXT DEFAULT '[]',
    capabilities TEXT DEFAULT '[]',
    model TEXT DEFAULT '',
    max_concurrency INTEGER DEFAULT 3,
    max_retries INTEGER DEFAULT 3,
    memory TEXT DEFAULT 'session' CHECK(memory IN ('none', 'session', 'persistent')),
    color TEXT DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

INSERT INTO team_members_new SELECT member_id, name, tool, custom_cmd, description, persona, persona_path, skills, mcp_servers, capabilities, model, max_concurrency, max_retries, memory, color, created_at, updated_at FROM team_members;
DROP TABLE team_members;
ALTER TABLE team_members_new RENAME TO team_members;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_team_workers_status ON team_workers(status);
CREATE INDEX IF NOT EXISTS idx_team_workers_member ON team_workers(member_id);

-- Drop team_projects table
DROP TABLE IF EXISTS team_projects;
