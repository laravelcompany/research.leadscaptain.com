-- Phase B: Operations
CREATE TABLE IF NOT EXISTS bulk_operations (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 type TEXT NOT NULL,
 lead_ids_json TEXT,
 parameters_json TEXT,
 status TEXT DEFAULT 'queued',
 progress INTEGER DEFAULT 0,
 total INTEGER DEFAULT 0,
 success_count INTEGER DEFAULT 0,
 failure_count INTEGER DEFAULT 0,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 completed_at DATETIME
);
CREATE TABLE IF NOT EXISTS research_queue (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 status TEXT DEFAULT 'QUEUED',
 started_at DATETIME,
 completed_at DATETIME,
 error TEXT,
 result TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_research_status ON research_queue(status);
CREATE TABLE IF NOT EXISTS imports (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 filename TEXT,
 format TEXT,
 status TEXT DEFAULT 'pending',
 total_rows INTEGER DEFAULT 0,
 valid_rows INTEGER DEFAULT 0,
 duplicate_rows INTEGER DEFAULT 0,
 imported_rows INTEGER DEFAULT 0,
 mapping_json TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
