CREATE TABLE IF NOT EXISTS team_members (
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

CREATE TABLE IF NOT EXISTS team_workers (
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

CREATE TABLE IF NOT EXISTS team_tasks (
    task_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    priority TEXT NOT NULL DEFAULT 'medium' CHECK(priority IN ('low', 'medium', 'high', 'urgent')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'assigned', 'working', 'done', 'failed', 'paused')),
    assigned_member_id TEXT DEFAULT '',
    assigned_worker_id TEXT DEFAULT '',
    depends_on TEXT DEFAULT '[]',
    result TEXT DEFAULT '',
    error TEXT DEFAULT '',
    output_history TEXT DEFAULT '[]',
    progress INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    next_retry_at INTEGER DEFAULT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    completed_at INTEGER DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS team_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT DEFAULT '',
    worker_id TEXT DEFAULT '',
    member_id TEXT DEFAULT '',
    type TEXT NOT NULL,
    description TEXT DEFAULT '',
    meta TEXT DEFAULT '{}',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_team_tasks_status ON team_tasks(status);
CREATE INDEX IF NOT EXISTS idx_team_tasks_priority ON team_tasks(priority);
CREATE INDEX IF NOT EXISTS idx_team_tasks_status_priority ON team_tasks(status, priority);
CREATE INDEX IF NOT EXISTS idx_team_tasks_member ON team_tasks(assigned_member_id);
CREATE INDEX IF NOT EXISTS idx_team_workers_status ON team_workers(status);
CREATE INDEX IF NOT EXISTS idx_team_workers_member ON team_workers(member_id);
CREATE INDEX IF NOT EXISTS idx_team_activity_task ON team_activity(task_id);
CREATE INDEX IF NOT EXISTS idx_team_activity_worker ON team_activity(worker_id);
CREATE INDEX IF NOT EXISTS idx_team_activity_member ON team_activity(member_id);
CREATE INDEX IF NOT EXISTS idx_team_activity_created ON team_activity(created_at);
