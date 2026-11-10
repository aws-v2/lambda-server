DROP TABLE IF EXISTS functions;

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- for gen_random_uuid()

CREATE TABLE NOT EXISTS functions (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                     TEXT NOT NULL UNIQUE,
    arn                      TEXT UNIQUE,
    user_id                  TEXT,
    type                     TEXT NOT NULL,
    image                    TEXT,
    execution                TEXT NOT NULL,
    resources                TEXT NOT NULL,
    env                      TEXT NOT NULL DEFAULT '{}',
    timeout_ms               INTEGER NOT NULL,
    description              TEXT,
    provisioned_concurrency  INTEGER DEFAULT 0,
    sha256                   TEXT,
    region                   TEXT DEFAULT '',
    runtime                  TEXT NOT NULL,
    handler                  TEXT NOT NULL,
    timeout                  INTEGER NOT NULL,
    memory                   INTEGER NOT NULL,
    version                  TEXT NOT NULL,
    created_at               TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at               TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_functions_arn ON functions (arn);
CREATE INDEX idx_functions_created_at ON functions (created_at);
CREATE INDEX idx_functions_type ON functions (type);
CREATE INDEX idx_functions_user_id ON functions (user_id);