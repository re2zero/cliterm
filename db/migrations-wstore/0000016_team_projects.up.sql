-- Create team_projects table
CREATE TABLE IF NOT EXISTS team_projects (
    project_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    spec TEXT DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Add project_id column to team_members
ALTER TABLE team_members ADD COLUMN project_id TEXT DEFAULT '';

-- Add project_id column to team_workers
ALTER TABLE team_workers ADD COLUMN project_id TEXT DEFAULT '';

-- Create index for project lookups
CREATE INDEX IF NOT EXISTS idx_team_members_project ON team_members(project_id);
CREATE INDEX IF NOT EXISTS idx_team_workers_project ON team_workers(project_id);
