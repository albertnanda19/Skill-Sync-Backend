BEGIN;

ALTER TABLE job_skills
  ADD COLUMN IF NOT EXISTS required_level INTEGER;

ALTER TABLE job_skills
  ADD COLUMN IF NOT EXISTS is_mandatory BOOLEAN;

ALTER TABLE job_skills
  ADD COLUMN IF NOT EXISTS required_years INTEGER;

ALTER TABLE job_skills
  ADD COLUMN IF NOT EXISTS source_version SMALLINT NOT NULL DEFAULT 1;

UPDATE job_skills
SET required_level = importance_weight
WHERE required_level IS NULL;

UPDATE job_skills
SET is_mandatory = (importance_weight >= 4)
WHERE is_mandatory IS NULL;

UPDATE job_skills
SET required_years = importance_weight
WHERE required_years IS NULL;

UPDATE job_skills
SET source_version = 1
WHERE source_version IS NULL;

CREATE INDEX IF NOT EXISTS idx_job_skills_job_id ON job_skills(job_id);
CREATE INDEX IF NOT EXISTS idx_job_skills_skill_id ON job_skills(skill_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_job_skills_job_id_skill_id ON job_skills(job_id, skill_id);
CREATE INDEX IF NOT EXISTS idx_job_skills_job_id_is_mandatory ON job_skills(job_id, is_mandatory);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'job_skills_required_level_check'
  ) THEN
    ALTER TABLE job_skills
      ADD CONSTRAINT job_skills_required_level_check
      CHECK (required_level IS NULL OR (required_level BETWEEN 1 AND 5));
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'job_skills_importance_weight_check'
  ) THEN
    ALTER TABLE job_skills
      ADD CONSTRAINT job_skills_importance_weight_check
      CHECK (importance_weight IS NULL OR (importance_weight BETWEEN 1 AND 5));
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'job_skills_required_years_check'
  ) THEN
    ALTER TABLE job_skills
      ADD CONSTRAINT job_skills_required_years_check
      CHECK (required_years IS NULL OR required_years >= 0);
  END IF;
END $$;

COMMIT;
