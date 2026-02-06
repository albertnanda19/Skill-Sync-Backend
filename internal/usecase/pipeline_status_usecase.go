package usecase

import (
	"context"
	"log"
	"sync"
	"time"

	"skill-sync/internal/delivery/http/dto"
	"skill-sync/internal/repository"
)

type PipelineStatusUsecase interface {
	GetStatus(ctx context.Context) (dto.PipelineStatusResponseData, error)
}

type PipelineStatus struct {
	repo repository.PipelineStatusRepository
	log  *log.Logger
}

func NewPipelineStatusUsecase(repo repository.PipelineStatusRepository, logger *log.Logger) *PipelineStatus {
	if logger == nil {
		logger = log.Default()
	}
	return &PipelineStatus{repo: repo, log: logger}
}

func (u *PipelineStatus) GetStatus(ctx context.Context) (dto.PipelineStatusResponseData, error) {
	if u == nil || u.repo == nil {
		return dto.PipelineStatusResponseData{LastUpdated: time.Now().UTC()}, nil
	}

	var (
		jobstreet repository.PipelineScraperSourceSummary
		devto     repository.PipelineScraperSourceSummary
		glints    repository.PipelineScraperSourceSummary
		careers   repository.PipelineScraperSourceSummary

		glintsLinkOnly int

		skillExtraction repository.PipelineSkillExtractionSummary
		matching        repository.PipelineMatchingEngineSummary
		recByUser       []repository.PipelineUserRecommendationSummary

		errJobstreet error
		errDevto     error
		errGlints    error
		errCareers   error
		errLinkOnly  error
		errSkill     error
		errMatch     error
		errRec       error
	)

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		jobstreet, errJobstreet = u.repo.GetScraperSourceSummary(ctx, "JobStreet")
		if errJobstreet != nil {
			u.log.Printf("pipeline_status step=scraper source=jobstreet status=error err=%v", errJobstreet)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		devto, errDevto = u.repo.GetScraperSourceSummary(ctx, "Dev.to Jobs")
		if errDevto != nil {
			u.log.Printf("pipeline_status step=scraper source=devto status=error err=%v", errDevto)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		glints, errGlints = u.repo.GetScraperSourceSummary(ctx, "Glints")
		if errGlints != nil {
			u.log.Printf("pipeline_status step=scraper source=glints status=error err=%v", errGlints)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		careers, errCareers = u.repo.GetScraperSourceSummary(ctx, "Company Careers")
		if errCareers != nil {
			u.log.Printf("pipeline_status step=scraper source=company_careers status=error err=%v", errCareers)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		glintsLinkOnly, errLinkOnly = u.repo.GetGlintsLinkOnlyCount(ctx)
		if errLinkOnly != nil {
			u.log.Printf("pipeline_status step=scraper source=glints metric=link_only status=error err=%v", errLinkOnly)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		skillExtraction, errSkill = u.repo.GetSkillExtractionSummary(ctx)
		if errSkill != nil {
			u.log.Printf("pipeline_status step=skill_extraction status=error err=%v", errSkill)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		matching, errMatch = u.repo.GetMatchingEngineSummary(ctx)
		if errMatch != nil {
			u.log.Printf("pipeline_status step=matching_engine status=error err=%v", errMatch)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		recByUser, errRec = u.repo.ListRecommendationSummaryByUser(ctx, 100)
		if errRec != nil {
			u.log.Printf("pipeline_status step=recommendation status=error err=%v", errRec)
		}
	}()

	wg.Wait()

	data := dto.PipelineStatusResponseData{
		Scraper: dto.PipelineScraperStatus{
			JobStreet: dto.PipelineScraperSourceStatus{JobsScraped: jobstreet.JobsScraped, Errors: jobstreet.Errors},
			Devto:     dto.PipelineScraperSourceStatus{JobsScraped: devto.JobsScraped, Errors: devto.Errors},
			Glints: dto.PipelineScraperGlintsStatus{
				JobsScraped: glints.JobsScraped,
				LinkOnly:    glintsLinkOnly,
				Errors:      glints.Errors,
			},
			CompanyCareers: dto.PipelineScraperSourceStatus{JobsScraped: careers.JobsScraped, Errors: careers.Errors},
		},
		SkillExtraction: dto.PipelineSkillExtractionStatus{
			JobsProcessed:              skillExtraction.JobsProcessed,
			JobsSkippedDescriptionNull: skillExtraction.JobsSkippedDescriptionNull,
			Errors:                     skillExtraction.Errors,
		},
		MatchingEngine: dto.PipelineMatchingEngineStatus{
			TotalJobsMatched:         matching.TotalJobsMatched,
			AverageMatchScore:        matching.AverageMatchScore,
			JobsWithMandatoryMissing: matching.JobsWithMandatoryMissing,
		},
		Recommendation: dto.PipelineRecommendationStatus{UserRecommendations: make([]dto.PipelineUserRecommendationSummary, 0)},
		LastUpdated:    time.Now().UTC(),
	}

	if errRec == nil {
		for _, it := range recByUser {
			data.Recommendation.UserRecommendations = append(data.Recommendation.UserRecommendations, dto.PipelineUserRecommendationSummary{
				UserID:          it.UserID.String(),
				JobsRecommended: it.JobsRecommended,
			})
		}
	}

	return data, nil
}
