-- Remove worker extended fields
ALTER TABLE cowork_workers DROP COLUMN role;
ALTER TABLE cowork_workers DROP COLUMN description;
ALTER TABLE cowork_workers DROP COLUMN soul;
ALTER TABLE cowork_workers DROP COLUMN skills;
ALTER TABLE cowork_workers DROP COLUMN mcp_servers;