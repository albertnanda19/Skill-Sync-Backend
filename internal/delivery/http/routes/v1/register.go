package v1

import (
	"log"
	"os"
	"strings"

	"skill-sync/internal/config"
	"skill-sync/internal/database"
	"skill-sync/internal/delivery/http/handler"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/infrastructure/cache"
	"skill-sync/internal/infrastructure/persistence/postgres"
	"skill-sync/internal/infrastructure/scraper"
	"skill-sync/internal/pkg/jwt"
	"skill-sync/internal/repository"
	"skill-sync/internal/usecase"
	jobuc "skill-sync/internal/usecase/job"

	"github.com/gofiber/fiber/v3"
)

func Register(r fiber.Router, cfg config.Config, db database.DB) {
	if r == nil {
		return
	}

	jwtSvc := jwt.NewHMACService(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpiresIn,
		cfg.JWT.RefreshExpiresIn,
	)

	authMw := middleware.NewAuthMiddleware(jwtSvc)

	skillRepo := repository.NewPostgresSkillRepository(db)

	userRepo := postgres.NewUserRepository(db)
	userSkillRepo := repository.NewPostgresUserSkillRepository(db)
	jobRepo := repository.NewPostgresJobRepository(db)
	jobSkillRepo := repository.NewPostgresJobSkillRepository(db)
	jobSkillV2Repo := repository.NewPostgresJobSkillV2Repository(db)
	pipelineStatusRepo := repository.NewPostgresPipelineStatusRepository(db)
	pipelineRepo := repository.NewPostgresPipelineRepository(db)

	logger := log.Default()
	redisCache := cache.NewRedis(logger)
	scraperClient := scraper.NewScraperClient(cfg.ScraperBaseURL, logger)
	freshnessSvc := jobuc.NewFreshnessService(jobRepo, scraperClient, redisCache, logger, cfg.SearchFreshnessMinutes)
	authUC := usecase.NewAuthUsecase(userRepo, jwtSvc)
	userUC := usecase.NewUserUsecase(userRepo)
	userSkillUC := usecase.NewUserSkillUsecase(userSkillRepo)
	skillUC := usecase.NewSkillUsecase(skillRepo)
	jobRecommendationUC := usecase.NewJobRecommendationUsecase(jobRepo, jobSkillRepo, userSkillRepo)
	matchingV2UC := usecase.NewMatchingUsecaseV2(jobRepo, jobSkillV2Repo, userSkillRepo)
	jobListUC := usecase.NewJobListUsecase(jobRepo, jobSkillRepo, freshnessSvc, redisCache, logger)
	pipelineStatusUC := usecase.NewPipelineStatusUsecase(pipelineStatusRepo, nil)
	pipelineUC := usecase.NewPipelineUsecase(pipelineRepo, db, redisCache)

	authHandler := handler.NewAuthHandler(authUC)
	userHandler := handler.NewUserHandler(userUC)
	userSkillHandler := handler.NewUserSkillHandler(userSkillUC)
	skillHandler := handler.NewSkillHandler(skillUC)
	jobRecommendationHandler := handler.NewJobRecommendationHandler(jobRecommendationUC)
	matchV2Handler := handler.NewMatchV2Handler(matchingV2UC)
	jobsHandler := handler.NewJobsHandler(jobListUC)
	pipelineStatusHandler := handler.NewPipelineStatusHandler(pipelineStatusUC, nil)
	pipelineHandler := handler.NewPipelineHandler(pipelineUC)

	authGroup := r.Group("/auth")
	authHandler.RegisterRoutes(authGroup)
	skillHandler.RegisterRoutes(r)

	protected := r.Group("", authMw.Middleware())

	publicJobs := strings.EqualFold(strings.TrimSpace(os.Getenv("PUBLIC_JOBS")), "true")
	if publicJobs {
		r.Get("/jobs", jobsHandler.HandleListJobs)
	} else {
		protected.Get("/jobs", jobsHandler.HandleListJobs)
	}

	usersGroup := protected.Group("/users")
	RegisterUsers(usersGroup, userHandler, userSkillHandler)
	RegisterJobs(protected, jobRecommendationHandler)
	matchV2Handler.RegisterRoutes(protected)
	pipelineStatusHandler.RegisterRoutes(protected)
	pipelineHandler.RegisterRoutes(protected)
}
