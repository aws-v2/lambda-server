CREATE TABLE IF NOT EXISTS api_keys (
    access_key_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    secret_key_hash TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    last_synced TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
