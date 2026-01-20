DROP TABLE IF EXISTS functions;

CREATE TABLE functions (
	name TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	image TEXT,
	execution TEXT NOT NULL,
	resources TEXT NOT NULL,
	env TEXT NOT NULL DEFAULT '{}',
	timeout_ms INTEGER NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);




CREATE INDEX IF NOT EXISTS idx_functions_created_at ON functions(created_at);
CREATE INDEX IF NOT EXISTS idx_functions_type ON functions(type);