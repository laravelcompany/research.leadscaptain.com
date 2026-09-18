-- Cached website analysis reports from the Websites section so repeat checks
-- of the same domain are instant and free.
CREATE TABLE IF NOT EXISTS website_analyses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    domain TEXT NOT NULL,
    reachable INTEGER DEFAULT 0,
    status_code INTEGER,
    load_ms INTEGER,
    title TEXT,
    meta_description TEXT,
    h1_tags TEXT,
    health_score INTEGER,
    seo_checks TEXT,
    contacts TEXT,
    keywords TEXT,
    traffic_note TEXT,
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_website_analyses_domain ON website_analyses(domain);
CREATE INDEX IF NOT EXISTS idx_website_analyses_created ON website_analyses(created_at);
