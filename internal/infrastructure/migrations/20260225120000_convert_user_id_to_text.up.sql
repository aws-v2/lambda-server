-- Migration: Convert user_id to TEXT to avoid UUID syntax errors on empty strings
ALTER TABLE functions ALTER COLUMN user_id TYPE TEXT;
ALTER TABLE lambda_metrics ALTER COLUMN user_id TYPE TEXT;
