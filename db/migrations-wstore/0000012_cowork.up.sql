-- 任务表
CREATE TABLE IF NOT EXISTS cowork_tasks (
    task_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    priority TEXT NOT NULL DEFAULT 'medium' CHECK(priority IN ('low', 'medium', 'high', 'urgent')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'assigned', 'working', 'done', 'failed')),
    assigned_worker TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    completed_at INTEGER DEFAULT NULL,
    result TEXT DEFAULT NULL,
    error TEXT DEFAULT NULL,
    progress TEXT DEFAULT NULL
);

-- Worker 注册表
CREATE TABLE IF NOT EXISTS cowork_workers (
    worker_id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    tool TEXT NOT NULL DEFAULT '' CHECK(tool IN ('claude', 'opencode', 'cursor', 'aider', 'custom')),
    custom_cmd TEXT DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'idle' CHECK(status IN ('idle', 'working', 'offline', 'error')),
    assigned_task TEXT DEFAULT NULL,
    block_id TEXT NOT NULL,
    tab_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    last_active_at INTEGER NOT NULL DEFAULT (unixepoch()),
    last_output_hash TEXT DEFAULT NULL,
    error_msg TEXT DEFAULT NULL
);

-- 活动日志（滚动窗口）
CREATE TABLE IF NOT EXISTS cowork_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT DEFAULT NULL,
    worker_id TEXT DEFAULT NULL,
    type TEXT NOT NULL DEFAULT 'info',
    description TEXT NOT NULL DEFAULT '',
    meta TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_cowork_tasks_status ON cowork_tasks(status, priority);
CREATE INDEX IF NOT EXISTS idx_cowork_tasks_worker ON cowork_tasks(assigned_worker);
CREATE INDEX IF NOT EXISTS idx_cowork_workers_status ON cowork_workers(status);
CREATE INDEX IF NOT EXISTS idx_cowork_activity_created ON cowork_activity(created_at);
