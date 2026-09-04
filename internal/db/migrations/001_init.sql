CREATE TABLE IF NOT EXISTS objectives (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL,
 description TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'idle',
 priority INTEGER DEFAULT 0,
 target_leads INTEGER,
 minimum_score INTEGER DEFAULT 0,
 max_iterations INTEGER DEFAULT 50,
 max_tasks INTEGER DEFAULT 500,
 iteration_count INTEGER DEFAULT 0,
 started_at DATETIME,
 completed_at DATETIME,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 allowed_tools TEXT,
 config_json TEXT
);
CREATE TABLE IF NOT EXISTS agent_runs (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 objective_id INTEGER NOT NULL REFERENCES objectives(id),
 status TEXT NOT NULL,
 iteration INTEGER DEFAULT 0,
 started_at DATETIME,
 finished_at DATETIME,
 total_tasks INTEGER DEFAULT 0,
 completed_tasks INTEGER DEFAULT 0,
 failed_tasks INTEGER DEFAULT 0,
 ai_requests INTEGER DEFAULT 0,
 api_requests INTEGER DEFAULT 0
);
CREATE TABLE IF NOT EXISTS tasks (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 objective_id INTEGER NOT NULL REFERENCES objectives(id),
 run_id INTEGER REFERENCES agent_runs(id),
 parent_task_id INTEGER,
 title TEXT NOT NULL,
 description TEXT,
 category TEXT NOT NULL,
 status TEXT NOT NULL,
 priority INTEGER DEFAULT 0,
 input_json TEXT,
 output_json TEXT,
 planned_steps TEXT,
 attempts INTEGER DEFAULT 0,
 max_attempts INTEGER DEFAULT 3,
 started_at DATETIME,
 completed_at DATETIME,
 error TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 lease_until DATETIME,
 worker_id TEXT
);
CREATE TABLE IF NOT EXISTS iterations (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 run_id INTEGER NOT NULL REFERENCES agent_runs(id),
 iteration_number INTEGER NOT NULL,
 objective_snapshot TEXT,
 plan TEXT,
 reasoning_summary TEXT,
 action TEXT,
 action_result TEXT,
 reflection TEXT,
 status TEXT,
 started_at DATETIME,
 completed_at DATETIME
);
CREATE TABLE IF NOT EXISTS leads (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 external_key TEXT UNIQUE,
 first_name TEXT,
 last_name TEXT,
 full_name TEXT,
 email TEXT,
 email_status TEXT,
 email_verified_at DATETIME,
 phone TEXT,
 company_name TEXT,
 company_domain TEXT,
 position_title TEXT,
 department TEXT,
 industry_name TEXT,
 country_code TEXT,
 country_name TEXT,
 city TEXT,
 linkedin_url TEXT,
 website_url TEXT,
 persona TEXT,
 lead_score INTEGER DEFAULT 0,
 score_breakdown TEXT,
 source TEXT,
 raw_data TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_leads_email ON leads(email);
CREATE INDEX IF NOT EXISTS idx_leads_company ON leads(company_name);
CREATE INDEX IF NOT EXISTS idx_leads_domain ON leads(company_domain);
CREATE INDEX IF NOT EXISTS idx_leads_title ON leads(position_title);
CREATE INDEX IF NOT EXISTS idx_leads_industry ON leads(industry_name);
CREATE INDEX IF NOT EXISTS idx_leads_country ON leads(country_code);
CREATE INDEX IF NOT EXISTS idx_leads_score ON leads(lead_score);
CREATE TABLE IF NOT EXISTS research_findings (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 objective_id INTEGER,
 lead_id INTEGER,
 type TEXT,
 title TEXT,
 content TEXT,
 source TEXT,
 confidence REAL,
 metadata_json TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS lead_merges (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 primary_lead_id INTEGER,
 duplicate_lead_id INTEGER,
 reason TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS lead_lists (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 external_id TEXT,
 name TEXT NOT NULL,
 description TEXT,
 source TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS lead_list_members (
 list_id INTEGER,
 lead_id INTEGER,
 added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(list_id, lead_id)
);
CREATE TABLE IF NOT EXISTS api_requests (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 provider TEXT,
 operation TEXT,
 objective_id INTEGER,
 task_id INTEGER,
 status TEXT,
 duration_ms INTEGER,
 request_size INTEGER,
 response_size INTEGER,
 http_status INTEGER,
 error TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS ai_usage (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 objective_id INTEGER,
 run_id INTEGER,
 model TEXT,
 request_type TEXT,
 input_tokens INTEGER,
 output_tokens INTEGER,
 total_tokens INTEGER,
 duration_ms INTEGER,
 success BOOLEAN,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS audit_log (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
 level TEXT,
 event_type TEXT,
 actor TEXT,
 objective_id INTEGER,
 run_id INTEGER,
 task_id INTEGER,
 lead_id INTEGER,
 message TEXT,
 metadata_json TEXT
);
CREATE TABLE IF NOT EXISTS search_history (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 objective_id INTEGER,
 query TEXT,
 filters_json TEXT,
 result_count INTEGER,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS jobs (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 type TEXT NOT NULL,
 payload_json TEXT,
 status TEXT NOT NULL,
 attempts INTEGER DEFAULT 0,
 max_attempts INTEGER DEFAULT 3,
 available_at DATETIME,
 locked_at DATETIME,
 locked_by TEXT,
 error TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS companies (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT,
 domain TEXT UNIQUE,
 industry TEXT,
 employee_count INTEGER,
 country TEXT,
 city TEXT,
 website TEXT,
 linkedin_url TEXT,
 description TEXT,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
