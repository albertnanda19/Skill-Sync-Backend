package domain

import "time"

type SourceStat struct {
	Source      string    `json:"source"`
	TotalJobs   int       `json:"total_jobs"`
	LastJobTime time.Time `json:"last_job_time"`
}

type PipelineStatus struct {
	TotalJobs       int          `json:"total_jobs"`
	JobsToday       int          `json:"jobs_today"`
	Sources         []SourceStat `json:"sources"`
	DatabaseHealthy bool         `json:"database_healthy"`
	RedisHealthy    bool         `json:"redis_healthy"`
	ServerTime      time.Time    `json:"server_time"`
}
