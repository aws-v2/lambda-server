ALTER TABLE functions ADD COLUMN arn TEXT UNIQUE;
CREATE INDEX idx_functions_arn ON functions(arn);
