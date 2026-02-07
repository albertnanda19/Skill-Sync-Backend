package search

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	OriginalIndex int
	ID            uuid.UUID
	Title         string
	CompanyName   string
	Location      string
	Description   string
	JobURL        string
	Source        string
	CreatedAt     time.Time
	PostedAt      *time.Time
}

type JobScore struct {
	JobID         uuid.UUID
	Relevance     float64
	Freshness     float64
	SourceQuality float64
	DataQuality   float64
	FinalScore    float64
}

var SourceWeights = map[string]float64{
	"linkedin":     3,
	"indeed":       3,
	"glassdoor":    3,
	"google":       2,
	"company_site": 4,
	"unknown":      1,
}

func ComputeRelevance(job Job, queryVariants []string) float64 {
	if len(queryVariants) == 0 {
		return 0
	}

	title := strings.ToLower(job.Title)
	desc := strings.ToLower(job.Description)
	company := strings.ToLower(job.CompanyName)

	score := 0.0
	for _, v := range queryVariants {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		v = strings.ToLower(v)
		if title != "" && strings.Contains(title, v) {
			score += 3
		}
		if desc != "" && strings.Contains(desc, v) {
			score += 1
		}
		if company != "" && strings.Contains(company, v) {
			score += 1
		}
		if score >= 10 {
			return 10
		}
	}
	if score > 10 {
		score = 10
	}
	return score
}

func ComputeFreshness(job Job) float64 {
	var t time.Time
	if job.PostedAt != nil && !job.PostedAt.IsZero() {
		t = *job.PostedAt
	} else if !job.CreatedAt.IsZero() {
		t = job.CreatedAt
	} else {
		return 0
	}

	now := time.Now().UTC()
	age := now.Sub(t)
	if age < 0 {
		age = 0
	}

	if age <= 24*time.Hour {
		return 5
	}
	if age <= 3*24*time.Hour {
		return 4
	}
	if age <= 7*24*time.Hour {
		return 3
	}
	if age <= 14*24*time.Hour {
		return 2
	}
	if age <= 30*24*time.Hour {
		return 1
	}
	return 0
}

func ComputeSourceQuality(source string) float64 {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		source = "unknown"
	}
	if w, ok := SourceWeights[source]; ok {
		return w
	}
	return 1
}

func ComputeDataQuality(job Job) float64 {
	score := 0.0
	if strings.TrimSpace(job.Title) != "" {
		score += 1
	}
	if strings.TrimSpace(job.CompanyName) != "" {
		score += 1
	}
	if strings.TrimSpace(job.Location) != "" {
		score += 1
	}
	if len(strings.TrimSpace(job.Description)) > 100 {
		score += 1
	}
	if strings.TrimSpace(job.JobURL) != "" {
		score += 1
	}
	if score > 5 {
		score = 5
	}
	return score
}

func ScoreJob(job Job, queryVariants []string) JobScore {
	rel := ComputeRelevance(job, queryVariants)
	fresh := ComputeFreshness(job)
	src := ComputeSourceQuality(job.Source)
	qual := ComputeDataQuality(job)

	final := (rel * 2.0) + (fresh * 1.5) + (src * 1.0) + (qual * 0.5)

	return JobScore{
		JobID:         job.ID,
		Relevance:     rel,
		Freshness:     fresh,
		SourceQuality: src,
		DataQuality:   qual,
		FinalScore:    final,
	}
}

func RankJobs(jobs []Job, queryVariants []string) []Job {
	if len(jobs) == 0 {
		return jobs
	}

	maxScore := 0.0
	scored := make([]struct {
		idx   int
		score float64
	}, len(jobs))

	for i := range jobs {
		s := ScoreJob(jobs[i], queryVariants).FinalScore
		scored[i] = struct {
			idx   int
			score float64
		}{idx: i, score: s}
		if s > maxScore {
			maxScore = s
		}
	}

	if maxScore == 0 {
		return jobs
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	out := make([]Job, 0, len(jobs))
	for _, it := range scored {
		out = append(out, jobs[it.idx])
	}
	return out
}
