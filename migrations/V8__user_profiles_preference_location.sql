ALTER TABLE user_profiles
  ADD COLUMN IF NOT EXISTS preference_location TEXT;
