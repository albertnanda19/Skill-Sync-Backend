package routes

import (
	"skill-sync/internal/config"
	"skill-sync/internal/database"
	v1 "skill-sync/internal/delivery/http/routes/v1"

	"github.com/gofiber/fiber/v3"
)

func RegisterV1(r fiber.Router, cfg config.Config, db database.DB) {
	if r == nil {
		return
	}

	v1.Register(r, cfg, db)
}
