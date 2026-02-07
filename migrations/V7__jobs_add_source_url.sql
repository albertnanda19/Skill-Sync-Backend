BEGIN;

ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS source_url TEXT;

CREATE INDEX IF NOT EXISTS idx_jobs_source_url
  ON jobs(source_url);

COMMIT;
