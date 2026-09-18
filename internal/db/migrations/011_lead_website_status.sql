-- Website liveness verdict per lead, filled by the ingestion pipeline's
-- check-website step (lead found -> check website -> check email -> score).
ALTER TABLE leads ADD COLUMN website_status TEXT;
ALTER TABLE leads ADD COLUMN website_checked_at DATETIME;
