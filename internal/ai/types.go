package ai

type JobContext struct {
	ID          string
	Title       string
	Company     string
	Location    string
	Description string
	Source      string
	URL         string
}

type AIRecommendation struct {
	JobID  string  `json:"job_id"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}
