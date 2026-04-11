-- Drop indexes
DROP INDEX IF EXISTS idx_mcp_servers_enabled;
DROP INDEX IF EXISTS idx_mcp_servers_name;
DROP INDEX IF EXISTS idx_skills_name;

-- Drop tables
DROP TABLE IF EXISTS mcp_servers;
DROP TABLE IF EXISTS skills;
