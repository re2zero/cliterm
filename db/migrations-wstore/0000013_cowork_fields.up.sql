-- Add worker extended fields
ALTER TABLE cowork_workers ADD COLUMN role TEXT DEFAULT '';
ALTER TABLE cowork_workers ADD COLUMN description TEXT DEFAULT '';
ALTER TABLE cowork_workers ADD COLUMN soul TEXT DEFAULT '';
ALTER TABLE cowork_workers ADD COLUMN skills TEXT DEFAULT '';
ALTER TABLE cowork_workers ADD COLUMN mcp_servers TEXT DEFAULT '';