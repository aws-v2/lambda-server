-- Migration: Add user_id to functions
ALTER TABLE functions ADD COLUMN user_id UUID;
CREATE INDEX idx_functions_user_id ON functions(user_id);
