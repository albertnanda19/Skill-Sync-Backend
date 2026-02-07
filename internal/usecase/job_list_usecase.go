package usecase

import (
	"context"
	"log"
	"strings"
	"time"

	"skill-sync/internal/repository"
	"skill-sync/internal/search"
	"skill-sync/internal/service"

	"github.com/google/uuid"
)

type JobListParams struct {
	Title       string
	CompanyName string
	Location    string
	Skills      []string
	Limit       int
	Offset      int
}

type JobListItem struct {
	JobID       uuid.UUID
	Title       string
	CompanyName string
	Location    string
	SourceURL   string
	Description string
	Skills      []string
	PostedAt    *time.Time
}

type JobListUsecase interface {
	ListJobs(ctx context.Context, params JobListParams) ([]JobListItem, bool, error)
}

type freshnessEnsurer interface {
	EnsureFresh(ctx context.Context, query, location string)
}

type JobList struct {
	jobs      repository.JobRepository
	jobSkills repository.JobSkillRepository
	freshness freshnessEnsurer
	cache     SearchCache
	logger    *log.Logger
}

func NewJobListUsecase(jobs repository.JobRepository, jobSkills repository.JobSkillRepository, freshness freshnessEnsurer, cache SearchCache, logger *log.Logger) *JobList {
	return &JobList{jobs: jobs, jobSkills: jobSkills, freshness: freshness, cache: cache, logger: logger}
}

func (u *JobList) ListJobs(ctx context.Context, params JobListParams) ([]JobListItem, bool, error) {
	limit := params.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 0 || limit > 50 {
		return nil, false, ErrInvalidInput
	}
	offset := params.Offset
	if offset < 0 {
		return nil, false, ErrInvalidInput
	}

	skills := make([]string, 0, len(params.Skills))
	for _, s := range params.Skills {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		skills = append(skills, s)
	}

	params.Limit = limit
	params.Offset = offset
	params.Skills = skills

	sp := service.SearchParams{
		Title:       params.Title,
		CompanyName: params.CompanyName,
		Location:    params.Location,
		Skills:      skills,
		Limit:       limit,
		Offset:      offset,
	}

	qctx := search.ProcessQuery(params.Title)

	cacheable := sp.HasFilter()
	cacheKey := ""
	lockKey := ""
	if u != nil && u.freshness != nil {
		title := strings.TrimSpace(params.Title)
		loc := strings.TrimSpace(params.Location)
		if loc == "" {
			loc = "Indonesia"
		}
		if title != "" {
			u.freshness.EnsureFresh(ctx, title, loc)
		}
	}
	if cacheable {
		cacheKey = JobsSearchCacheKey(params)
		lockKey = JobsSearchLockKey(cacheKey)

		if u != nil && u.cache != nil {
			var cached []JobListItem
			hit, err := u.cache.GetJSON(ctx, cacheKey, &cached)
			if err == nil && hit {
				if u.logger != nil {
					u.logger.Printf("[Jobs] Cache HIT: %s", cacheKey)
				}
				return cached, false, nil
			}
			if u.logger != nil {
				u.logger.Printf("[Jobs] Cache MISS: %s", cacheKey)
			}
		}
	}

	partial := false
	lockAcquired := false
	if cacheable && u != nil && u.cache != nil {
		ok, err := u.cache.SetIfNotExists(ctx, lockKey, "1", 30*time.Second)
		if err == nil && ok {
			lockAcquired = true
			if u.logger != nil {
				u.logger.Printf("[Jobs] Lock acquired: %s", lockKey)
			}
		} else if err == nil && !ok {
			jitterMs := time.Duration(time.Now().UnixNano()%201) * time.Millisecond
			wait := 300*time.Millisecond + jitterMs
			time.Sleep(wait)
			var cached []JobListItem
			hit, err2 := u.cache.GetJSON(ctx, cacheKey, &cached)
			if err2 == nil && hit {
				if u.logger != nil {
					u.logger.Printf("[Jobs] Cache HIT: %s", cacheKey)
				}
				return cached, false, nil
			}
			if u.logger != nil {
				u.logger.Printf("[Jobs] Lock wait fallback: %s", lockKey)
			}
		}
	}

	f := repository.JobListFilter{
		Title:         params.Title,
		TitleVariants: qctx.Variants,
		CompanyName:   params.CompanyName,
		Location:      params.Location,
		Skills:        skills,
		Limit:         limit,
		Offset:        offset,
	}
	rows, err := u.jobs.ListJobsForListing(ctx, f)
	if err != nil {
		return nil, partial, ErrInternal
	}

	if len(rows) < 5 {
		fb := search.FallbackFirstWord(qctx.Normalized)
		if fb != "" && fb != qctx.Normalized {
			fbCtx := search.ProcessQuery(fb)
			f.TitleVariants = fbCtx.Variants
			rows2, err2 := u.jobs.ListJobsForListing(ctx, f)
			if err2 == nil {
				rows = rows2
			}
		}
	}

	if len(rows) > 0 {
		rankInput := make([]search.Job, 0, len(rows))
		for i := range rows {
			r := rows[i]
			rankInput = append(rankInput, search.Job{
				OriginalIndex: i,
				ID:            r.ID,
				Title:         r.Title,
				CompanyName:   r.Company,
				Location:      r.Location,
				Description:   r.Description,
				JobURL:        r.SourceURL,
				Source:        r.Source,
				CreatedAt:     r.CreatedAt,
				PostedAt:      r.PostedAt,
			})
		}

		ranked := search.RankJobs(rankInput, qctx.Variants)
		if len(ranked) == len(rows) {
			ordered := make([]repository.JobListRow, 0, len(rows))
			for _, it := range ranked {
				idx := it.OriginalIndex
				if idx < 0 || idx >= len(rows) {
					continue
				}
				ordered = append(ordered, rows[idx])
			}
			if len(ordered) == len(rows) {
				rows = ordered
			}
		}
	}

	jobIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		if r.ID == uuid.Nil {
			continue
		}
		jobIDs = append(jobIDs, r.ID)
	}

	reqsByJobID, err := u.jobSkills.FindByJobIDs(ctx, jobIDs)
	if err != nil {
		return nil, partial, ErrInternal
	}

	out := make([]JobListItem, 0, len(rows))
	for _, r := range rows {
		reqs := reqsByJobID[r.ID]
		jobSkills := make([]string, 0, len(reqs))
		for _, it := range reqs {
			if it.SkillName == "" {
				continue
			}
			jobSkills = append(jobSkills, it.SkillName)
		}

		out = append(out, JobListItem{
			JobID:       r.ID,
			Title:       r.Title,
			CompanyName: r.Company,
			Location:    r.Location,
			SourceURL:   r.SourceURL,
			Description: r.Description,
			Skills:      jobSkills,
			PostedAt:    r.PostedAt,
		})
	}

	if cacheable && u != nil && u.cache != nil {
		_ = u.cache.SetJSON(ctx, cacheKey, out, 0)
		if u.logger != nil {
			u.logger.Printf("[Jobs] Cache SET: %s", cacheKey)
		}
		if lockAcquired {
			_ = u.cache.Delete(ctx, lockKey)
		}
	}
	return out, partial, nil
}
