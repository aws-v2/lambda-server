CREATE TABLE IF NOT EXISTS lambda_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    function_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_ms INTEGER NOT NULL,
    status TEXT NOT NULL, -- 'success', 'error'
    error_message TEXT,
    CONSTRAINT fk_function FOREIGN KEY (function_name) REFERENCES functions(name) ON DELETE CASCADE
);

CREATE INDEX idx_metrics_function_time ON lambda_metrics (function_name, timestamp DESC);
CREATE INDEX idx_metrics_user ON lambda_metrics (user_id);
