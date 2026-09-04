-- Phase A1: Lead Intelligence Foundation

-- Extend leads for lifecycle & company linkage
ALTER TABLE leads ADD COLUMN status TEXT DEFAULT 'NEW';
ALTER TABLE leads ADD COLUMN company_id INTEGER REFERENCES companies(id);
ALTER TABLE leads ADD COLUMN icp_score INTEGER DEFAULT 0;
ALTER TABLE leads ADD COLUMN data_quality_score INTEGER DEFAULT 0;
ALTER TABLE leads ADD COLUMN intent_score INTEGER DEFAULT 0;
ALTER TABLE leads ADD COLUMN engagement_score INTEGER DEFAULT 0;
ALTER TABLE leads ADD COLUMN overall_score INTEGER DEFAULT 0;
ALTER TABLE leads ADD COLUMN freshness_status TEXT DEFAULT 'FRESH';
ALTER TABLE leads ADD COLUMN last_enriched_at DATETIME;
ALTER TABLE leads ADD COLUMN seniority TEXT;
ALTER TABLE leads ADD COLUMN persona_confidence REAL;

CREATE INDEX IF NOT EXISTS idx_leads_status ON leads(status);
CREATE INDEX IF NOT EXISTS idx_leads_company_id ON leads(company_id);
CREATE INDEX IF NOT EXISTS idx_leads_overall ON leads(overall_score);
CREATE INDEX IF NOT EXISTS idx_leads_freshness ON leads(freshness_status);

-- Core lifecycle tables
CREATE TABLE IF NOT EXISTS lead_status_history (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 old_status TEXT,
 new_status TEXT NOT NULL,
 actor TEXT DEFAULT 'system',
 reason TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_status_history_lead ON lead_status_history(lead_id);

CREATE TABLE IF NOT EXISTS lead_score_history (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 score_type TEXT NOT NULL,
 old_score INTEGER,
 new_score INTEGER,
 breakdown TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_score_history_lead ON lead_score_history(lead_id);

CREATE TABLE IF NOT EXISTS lead_enrichments (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 field_name TEXT NOT NULL,
 old_value TEXT,
 new_value TEXT,
 provider TEXT,
 confidence REAL,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_enrichments_lead ON lead_enrichments(lead_id);

-- Supporting tables
CREATE TABLE IF NOT EXISTS tags (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT UNIQUE NOT NULL,
 slug TEXT UNIQUE NOT NULL,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS lead_tags (
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(lead_id, tag_id)
);
CREATE TABLE IF NOT EXISTS lead_relationships (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 from_lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 to_lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 relation_type TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS lead_notes (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 content TEXT NOT NULL,
 author TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS lead_tasks (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 title TEXT NOT NULL,
 status TEXT DEFAULT 'pending',
 due_at DATETIME,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Technology intelligence
CREATE TABLE IF NOT EXISTS technologies (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT UNIQUE NOT NULL,
 slug TEXT UNIQUE NOT NULL,
 category TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS company_technologies (
 company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 technology_id INTEGER NOT NULL REFERENCES technologies(id) ON DELETE CASCADE,
 confidence REAL DEFAULT 1.0,
 detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(company_id, technology_id)
);
CREATE INDEX IF NOT EXISTS idx_tech_name ON technologies(name);

-- Employment & strategy
CREATE TABLE IF NOT EXISTS employment_history (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 lead_id INTEGER NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
 company_id INTEGER REFERENCES companies(id),
 title TEXT,
 start_date DATETIME,
 end_date DATETIME,
 source TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_employment_lead ON employment_history(lead_id);

CREATE TABLE IF NOT EXISTS research_strategies (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 objective_type TEXT,
 strategy_key TEXT UNIQUE NOT NULL,
 description TEXT,
 executions INTEGER DEFAULT 0,
 successful_executions INTEGER DEFAULT 0,
 leads_discovered INTEGER DEFAULT 0,
 qualified_leads INTEGER DEFAULT 0,
 average_score REAL DEFAULT 0,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Segments / saved searches / exports
CREATE TABLE IF NOT EXISTS segments (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL,
 description TEXT,
 filters_json TEXT NOT NULL,
 lead_count INTEGER DEFAULT 0,
 last_calculated DATETIME,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS saved_searches (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL,
 filters_json TEXT NOT NULL,
 sort_json TEXT,
 columns_json TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS export_profiles (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL,
 fields_json TEXT NOT NULL,
 format TEXT DEFAULT 'csv',
 filters_json TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Seed technologies
INSERT OR IGNORE INTO technologies(name,slug,category) VALUES
 ('Laravel','laravel','Framework'),('React','react','Framework'),('AWS','aws','Infrastructure'),
 ('Docker','docker','Infrastructure'),('Kubernetes','kubernetes','Infrastructure'),
 ('PostgreSQL','postgresql','Database'),('Salesforce','salesforce','CRM');
