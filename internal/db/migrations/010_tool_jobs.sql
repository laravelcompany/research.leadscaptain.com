-- Queued bulk jobs from the Tools page (paste 10-20 emails or URLs, process
-- in the background, poll for results).
CREATE TABLE IF NOT EXISTS tool_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    total INTEGER DEFAULT 0,
    completed INTEGER DEFAULT 0,
    items TEXT,
    results TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tool_jobs_created ON tool_jobs(created_at);
