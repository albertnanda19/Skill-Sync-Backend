package v1

import (
	"skill-sync/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v3"
)

func RegisterUsers(r fiber.Router, userHandler *handler.UserHandler) {
	if r == nil {
		return
	}
	if userHandler == nil {
		return
	}

	userHandler.RegisterRoutes(r)
}
