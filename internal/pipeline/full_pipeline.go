package pipeline

import (
	"context"
	"log"
	"time"

	"skill-sync/internal/repository"
	"skill-sync/internal/usecase"

	"github.com/google/uuid"
)

type FullPipeline struct {
	skillExtraction *JobSkillExtractionPipeline

	matchingV2 usecase.MatchingUsecaseV2
	recommend  usecase.JobRecommendationUsecase

	users   repository.UserQueryRepository
	jobsQry repository.JobQueryRepository
	matches repository.JobMatchRepository

	log *log.Logger
}

type FullPipelineParams struct {
	JobStreetPages   int
	JobStreetWorkers int
	DevtoPages       int
	DevtoWorkers     int

	ExtractionWorkers int
	ExtractionLimit   int

	MatchingWorkers int

	RecommendationLimit    int
	RecommendationMinScore int
}

func NewFullPipeline(
	skillExtraction *JobSkillExtractionPipeline,
	matchingV2 usecase.MatchingUsecaseV2,
	recommend usecase.JobRecommendationUsecase,
	users repository.UserQueryRepository,
	jobsQry repository.JobQueryRepository,
	matches repository.JobMatchRepository,
	logger *log.Logger,
) *FullPipeline {
	if logger == nil {
		logger = log.Default()
	}
	return &FullPipeline{
		skillExtraction: skillExtraction,
		matchingV2:      matchingV2,
		recommend:       recommend,
		users:           users,
		jobsQry:         jobsQry,
		matches:         matches,
		log:             logger,
	}
}

func (p *FullPipeline) Run(ctx context.Context, params FullPipelineParams) error {
	if p == nil {
		return nil
	}
	start := time.Now()

	p.log.Printf("pipeline=full status=started")
	defer func() {
		p.log.Printf("pipeline=full status=finished duration=%s", time.Since(start))
	}()

	if err := p.RunScraper(ctx, params); err != nil {
		p.log.Printf("pipeline=full step=scraper status=error err=%v", err)
	}

	if err := p.RunSkillExtraction(ctx, params); err != nil {
		p.log.Printf("pipeline=full step=skill_extraction status=error err=%v", err)
	}

	if err := p.RunMatchingEngineV2(ctx, params); err != nil {
		p.log.Printf("pipeline=full step=matching_v2 status=error err=%v", err)
	}

	if err := p.RunRecommendations(ctx, params); err != nil {
		p.log.Printf("pipeline=full step=recommendations status=error err=%v", err)
	}

	if p.jobsQry != nil {
		totalJobs, err := p.jobsQry.CountJobs(ctx)
		if err == nil {
			p.log.Printf("pipeline=full summary total_jobs=%d", totalJobs)
		}
		totalSkills, err := p.jobsQry.CountJobSkills(ctx)
		if err == nil {
			p.log.Printf("pipeline=full summary total_job_skills=%d", totalSkills)
		}
	}

	return nil
}

func (p *FullPipeline) RunScraper(ctx context.Context, params FullPipelineParams) error {
	if p == nil {
		return nil
	}
	_ = ctx
	_ = params
	return nil
}

func (p *FullPipeline) RunSkillExtraction(ctx context.Context, params FullPipelineParams) error {
	if p == nil || p.skillExtraction == nil {
		return nil
	}

	stepStart := time.Now()
	p.log.Printf("pipeline=full step=skill_extraction status=started")
	defer func() {
		p.log.Printf("pipeline=full step=skill_extraction status=finished duration=%s", time.Since(stepStart))
	}()

	return p.skillExtraction.Run(ctx, RunParams{Workers: params.ExtractionWorkers, Limit: params.ExtractionLimit})
}

func (p *FullPipeline) RunMatchingEngineV2(ctx context.Context, params FullPipelineParams) error {
	if p == nil || p.matchingV2 == nil || p.users == nil || p.jobsQry == nil || p.matches == nil {
		return nil
	}

	stepStart := time.Now()
	p.log.Printf("pipeline=full step=matching_v2 status=started")
	defer func() {
		p.log.Printf("pipeline=full step=matching_v2 status=finished duration=%s", time.Since(stepStart))
	}()

	workers := params.MatchingWorkers
	if workers <= 0 {
		workers = 10
	}

	userIDs := make([]uuid.UUID, 0)
	for off := 0; ; {
		ids, err := p.users.ListUserIDs(ctx, 500, off)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		userIDs = append(userIDs, ids...)
		off += len(ids)
	}
	if len(userIDs) == 0 {
		return nil
	}

	jobIDs := make([]uuid.UUID, 0)
	for off := 0; ; {
		ids, err := p.jobsQry.ListJobIDsWithSkills(ctx, 1000, off)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		jobIDs = append(jobIDs, ids...)
		off += len(ids)
	}
	if len(jobIDs) == 0 {
		return nil
	}

	total := len(userIDs) * len(jobIDs)
	p.log.Printf("pipeline=full step=matching_v2 status=info users=%d jobs=%d total_pairs=%d workers=%d", len(userIDs), len(jobIDs), total, workers)

	pool := NewWorkerPool(workers, workers*2)
	results := pool.Run(ctx)

	var submitted int
	for _, uid := range userIDs {
		uid := uid
		for _, jid := range jobIDs {
			jid := jid
			pool.Submit(func(ctx context.Context) Result {
				res, err := p.matchingV2.CalculateMatchV2(ctx, uid, jid)
				if err != nil {
					p.log.Printf("pipeline=full step=matching_v2 status=error user_id=%s job_id=%s err=%v", uid, jid, err)
					return Result{Err: err}
				}

				score := float64(res.MatchScore)
				if err := p.matches.Upsert(ctx, repository.JobMatchUpsert{UserID: uid, JobID: jid, Score: score, MatchedAt: time.Now().UTC()}); err != nil {
					p.log.Printf("pipeline=full step=matching_v2 status=error user_id=%s job_id=%s match_score=%d err=%v", uid, jid, res.MatchScore, err)
					return Result{Err: err}
				}

				p.log.Printf("pipeline=full step=matching_v2 status=ok user_id=%s job_id=%s match_score=%d mandatory_missing=%t", uid, jid, res.MatchScore, res.MandatoryMissing)
				return Result{Err: nil}
			})
			submitted++
		}
	}

	pool.Close()

	var failed int
	for r := range results {
		if r.Err != nil {
			failed++
		}
	}

	p.log.Printf("pipeline=full step=matching_v2 summary total=%d failed=%d", submitted, failed)
	return nil
}

func (p *FullPipeline) RunRecommendations(ctx context.Context, params FullPipelineParams) error {
	if p == nil || p.recommend == nil || p.users == nil {
		return nil
	}

	stepStart := time.Now()
	p.log.Printf("pipeline=full step=recommendations status=started")
	defer func() {
		p.log.Printf("pipeline=full step=recommendations status=finished duration=%s", time.Since(stepStart))
	}()

	limit := params.RecommendationLimit
	if limit <= 0 {
		limit = 10
	}
	minScore := params.RecommendationMinScore
	if minScore < 0 {
		minScore = 0
	}

	for off := 0; ; {
		ids, err := p.users.ListUserIDs(ctx, 200, off)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}

		for _, uid := range ids {
			uid := uid
			items, err := p.recommend.GetRecommendations(ctx, uid, usecase.JobRecommendationParams{Limit: limit, Offset: 0, MinScore: minScore})
			if err != nil {
				p.log.Printf("pipeline=full step=recommendations status=error user_id=%s err=%v", uid, err)
				continue
			}
			p.log.Printf("pipeline=full step=recommendations status=ok user_id=%s recommended=%d", uid, len(items))
		}

		off += len(ids)
	}

	return nil
}
