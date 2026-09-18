-- Extend the existing companies table (001_init.sql) for the Companies
-- section: firmographics, socials, tech signals and provenance. Plain
-- ADD COLUMN statements are idempotent here - Migrate() skips duplicate-column
-- errors on re-run.
ALTER TABLE companies ADD COLUMN employee_range TEXT;
ALTER TABLE companies ADD COLUMN founded_year TEXT;
ALTER TABLE companies ADD COLUMN hq_location TEXT;
ALTER TABLE companies ADD COLUMN revenue_estimate TEXT;
ALTER TABLE companies ADD COLUMN twitter_url TEXT;
ALTER TABLE companies ADD COLUMN facebook_url TEXT;
ALTER TABLE companies ADD COLUMN crunchbase_url TEXT;
ALTER TABLE companies ADD COLUMN tech_stack TEXT;
ALTER TABLE companies ADD COLUMN source TEXT;
ALTER TABLE companies ADD COLUMN registry_number TEXT;
ALTER TABLE companies ADD COLUMN registry_status TEXT;
ALTER TABLE companies ADD COLUMN registry_country TEXT;
ALTER TABLE companies ADD COLUMN raw_data TEXT;
ALTER TABLE companies ADD COLUMN updated_at DATETIME;
CREATE INDEX IF NOT EXISTS idx_companies_name ON companies(name);
