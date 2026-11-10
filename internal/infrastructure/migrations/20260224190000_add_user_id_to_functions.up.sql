   ALTER TABLE functions ADD COLUMN IF NOT EXISTS user_id UUID;
   CREATE INDEX IF NOT EXISTS idx_functions_user_id ON functions(user_id);