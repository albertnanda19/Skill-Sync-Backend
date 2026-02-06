package dto

import "time"

type PipelineStatusResponseData struct {
	Scraper        PipelineScraperStatus        `json:"scraper"`
	SkillExtraction PipelineSkillExtractionStatus `json:"skill_extraction"`
	MatchingEngine PipelineMatchingEngineStatus `json:"matching_engine"`
	Recommendation PipelineRecommendationStatus `json:"recommendation"`
	LastUpdated    time.Time                    `json:"last_updated"`
}

type PipelineScraperStatus struct {
	JobStreet      PipelineScraperSourceStatus `json:"jobstreet"`
	Devto          PipelineScraperSourceStatus `json:"devto"`
	Glints         PipelineScraperGlintsStatus `json:"glints"`
	CompanyCareers PipelineScraperSourceStatus `json:"company_careers"`
}

type PipelineScraperSourceStatus struct {
	JobsScraped int `json:"jobs_scraped"`
	Errors     int `json:"errors"`
}

type PipelineScraperGlintsStatus struct {
	JobsScraped int `json:"jobs_scraped"`
	LinkOnly   int `json:"link_only"`
	Errors     int `json:"errors"`
}

type PipelineSkillExtractionStatus struct {
	JobsProcessed               int `json:"jobs_processed"`
	JobsSkippedDescriptionNull  int `json:"jobs_skipped_description_null"`
	Errors                      int `json:"errors"`
}

type PipelineMatchingEngineStatus struct {
	TotalJobsMatched           int     `json:"total_jobs_matched"`
	AverageMatchScore          float64 `json:"average_match_score"`
	JobsWithMandatoryMissing   int     `json:"jobs_with_mandatory_missing"`
}

type PipelineRecommendationStatus struct {
	UserRecommendations []PipelineUserRecommendationSummary `json:"user_recommendations"`
}

type PipelineUserRecommendationSummary struct {
	UserID         string `json:"user_id"`
	JobsRecommended int    `json:"jobs_recommended"`
}
