BEGIN;

ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS url TEXT;

ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_source_id_url_unique
  ON jobs(source_id, url);

CREATE INDEX IF NOT EXISTS idx_jobs_source_id_scraped_at
  ON jobs(source_id, scraped_at);

COMMIT;
