package v1

import (
	"skill-sync/internal/config"
	"skill-sync/internal/database"
	"skill-sync/internal/delivery/http/handler"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/infrastructure/persistence/postgres"
	"skill-sync/internal/pkg/jwt"
	"skill-sync/internal/usecase"
	useruc "skill-sync/internal/usecase/user"

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
	authUC := usecase.NewAuthUsecase(userRepo, jwtSvc)
	userUC := useruc.NewService(userRepo)

	authHandler := handler.NewAuthHandler(authUC)
	userHandler := handler.NewUserHandler(userUC)

	authGroup := r.Group("/auth")
	authHandler.RegisterRoutes(authGroup)

	protected := r.Group("", authMw.Middleware())

	usersGroup := protected.Group("/users")
	userHandler.RegisterRoutes(usersGroup)
}
