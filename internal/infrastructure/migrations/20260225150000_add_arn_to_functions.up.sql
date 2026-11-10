ALTER TABLE functions ADD COLUMN NOT EXISTS  arn TEXT UNIQUE;
CREATE INDEX idx_functions_arn ON functions(arn);
