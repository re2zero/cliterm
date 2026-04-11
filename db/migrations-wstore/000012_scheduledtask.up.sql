CREATE TABLE IF NOT EXISTS db_scheduledtask (
    oid varchar(36) PRIMARY KEY,
    version int NOT NULL,
    data json NOT NULL
);

-- Index on next_run for efficient due-task queries
-- SQLite uses json_extract instead of ->> operator
CREATE INDEX IF NOT EXISTS idx_scheduledtask_next_run ON db_scheduledtask (json_extract(data, '$.nextrun'));
