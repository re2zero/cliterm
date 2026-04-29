ALTER TABLE cowork_workers ADD COLUMN concurrency INTEGER DEFAULT 0;
ALTER TABLE cowork_workers ADD COLUMN timeout INTEGER DEFAULT 0;
ALTER TABLE cowork_workers ADD COLUMN max_retries INTEGER DEFAULT 0;
ALTER TABLE cowork_workers ADD COLUMN capabilities TEXT DEFAULT '';
