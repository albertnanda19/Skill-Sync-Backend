package v1

import (
	"skill-sync/internal/config"
	"skill-sync/internal/database"
	"skill-sync/internal/delivery/http/handler"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/infrastructure/persistence/postgres"
	"skill-sync/internal/pkg/jwt"
	"skill-sync/internal/repository"
	"skill-sync/internal/usecase"

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

	userRepo := postgres.NewUserRepository(db)
	userSkillRepo := repository.NewPostgresUserSkillRepository(db)
	jobRepo := repository.NewPostgresJobRepository(db)
	jobSkillV2Repo := repository.NewPostgresJobSkillV2Repository(db)
	authUC := usecase.NewAuthUsecase(userRepo, jwtSvc)
	userUC := usecase.NewUserUsecase(userRepo)
	userSkillUC := usecase.NewUserSkillUsecase(userSkillRepo)
	matchingV2UC := usecase.NewMatchingUsecaseV2(jobRepo, jobSkillV2Repo, userSkillRepo)

	authHandler := handler.NewAuthHandler(authUC)
	userHandler := handler.NewUserHandler(userUC)
	userSkillHandler := handler.NewUserSkillHandler(userSkillUC)
	matchV2Handler := handler.NewMatchV2Handler(matchingV2UC)

	authGroup := r.Group("/auth")
	authHandler.RegisterRoutes(authGroup)

	protected := r.Group("", authMw.Middleware())

	usersGroup := protected.Group("/users")
	RegisterUsers(usersGroup, userHandler, userSkillHandler)
	matchV2Handler.RegisterRoutes(protected)
}
