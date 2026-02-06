
BEGIN;

CREATE TABLE users (
  id UUID PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE users IS 'Authentication and core user identity records.';

CREATE TABLE user_profiles (
  id UUID PRIMARY KEY,
  user_id UUID UNIQUE REFERENCES users(id),
  full_name TEXT,
  experience_level TEXT,
  preferred_roles TEXT[],
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE user_profiles IS 'Extended user profile attributes (1:1 with users).';

CREATE TABLE skills (
  id UUID PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  category TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE skills IS 'Normalized skill taxonomy.';

CREATE TABLE user_skills (
  id UUID PRIMARY KEY,
  user_id UUID REFERENCES users(id),
  skill_id UUID REFERENCES skills(id),
  proficiency_level SMALLINT,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, skill_id)
);

COMMENT ON TABLE user_skills IS 'User-to-skill mapping with proficiency.';

CREATE TABLE job_sources (
  id UUID PRIMARY KEY,
  name TEXT UNIQUE,
  base_url TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE job_sources IS 'Job listing sources used by the scraper.';

CREATE TABLE jobs (
  id UUID PRIMARY KEY,
  source_id UUID REFERENCES job_sources(id),
  external_job_id TEXT,
  title TEXT,
  company TEXT,
  location TEXT,
  employment_type TEXT,
  description TEXT,
  raw_description TEXT,
  posted_at TIMESTAMPTZ,
  scraped_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (source_id, external_job_id)
);

COMMENT ON TABLE jobs IS 'Scraped job postings (deduplicated per source + external id).';

CREATE TABLE job_skills (
  id UUID PRIMARY KEY,
  job_id UUID REFERENCES jobs(id),
  skill_id UUID REFERENCES skills(id),
  importance_weight SMALLINT,
  UNIQUE (job_id, skill_id)
);

COMMENT ON TABLE job_skills IS 'Skill requirements inferred/extracted for jobs.';

CREATE TABLE job_matches (
  id UUID PRIMARY KEY,
  user_id UUID REFERENCES users(id),
  job_id UUID REFERENCES jobs(id),
  match_score NUMERIC(5,2),
  matched_at TIMESTAMPTZ,
  UNIQUE (user_id, job_id)
);

COMMENT ON TABLE job_matches IS 'Computed match score between users and jobs.';

CREATE TABLE scrape_runs (
  id UUID PRIMARY KEY,
  source_id UUID REFERENCES job_sources(id),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  status TEXT
);

COMMENT ON TABLE scrape_runs IS 'Scraping execution runs per source.';

CREATE TABLE scrape_logs (
  id UUID PRIMARY KEY,
  scrape_run_id UUID REFERENCES scrape_runs(id),
  level TEXT,
  message TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE scrape_logs IS 'Log lines emitted during a scrape run.';

COMMIT;
