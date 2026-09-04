ALTER TABLE leads ADD COLUMN email_status TEXT DEFAULT 'unchecked';
ALTER TABLE leads ADD COLUMN email_verified_at DATETIME;
