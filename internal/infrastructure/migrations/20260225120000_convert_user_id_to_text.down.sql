-- Rollback: Convert user_id back to UUID (may fail if non-UUID data exists)
ALTER TABLE functions ALTER COLUMN user_id TYPE UUID USING user_id::UUID;
ALTER TABLE lambda_metrics ALTER COLUMN user_id TYPE UUID USING user_id::UUID;
