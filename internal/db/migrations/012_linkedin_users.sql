CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    linkedin_sub TEXT NOT NULL UNIQUE,
    email TEXT,
    name TEXT NOT NULL DEFAULT '',
    picture_url TEXT,
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_linkedin_sub ON users(linkedin_sub);
