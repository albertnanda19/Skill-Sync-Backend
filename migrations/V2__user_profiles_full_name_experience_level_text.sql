ALTER TABLE user_profiles
  ADD COLUMN IF NOT EXISTS full_name TEXT;

ALTER TABLE user_profiles
  ALTER COLUMN experience_level TYPE TEXT USING experience_level::TEXT;
