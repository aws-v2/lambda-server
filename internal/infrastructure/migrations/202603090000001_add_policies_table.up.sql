-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE lambda_scaling_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    tenant_id TEXT NOT NULL,
    function_id TEXT NOT NULL,
    metric_name TEXT NOT NULL,

    scale_up_threshold DOUBLE PRECISION NOT NULL,
    scale_down_threshold DOUBLE PRECISION NOT NULL,

    max_concurrency_limit INT NOT NULL,
    min_concurrency_limit INT NOT NULL,

    scale_step INT NOT NULL,
    cooldown_seconds INT NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT unique_policy UNIQUE (tenant_id, function_id, metric_name)
);

CREATE INDEX idx_lsp_tenant ON lambda_scaling_policies (tenant_id);
CREATE INDEX idx_lsp_function ON lambda_scaling_policies (function_id);
CREATE INDEX idx_lsp_metric ON lambda_scaling_policies (metric_name);

-- auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON lambda_scaling_policies
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd