-- Rollback: Add user_id to functions
DROP INDEX idx_functions_user_id;
ALTER TABLE functions DROP COLUMN user_id;
