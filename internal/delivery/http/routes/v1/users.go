package v1

import (
	"skill-sync/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v3"
)

func RegisterUsers(r fiber.Router, userHandler *handler.UserHandler, userSkillHandler *handler.UserSkillHandler) {
	if r == nil {
		return
	}
	if userHandler == nil {
		return
	}

	userHandler.RegisterRoutes(r)
	if userSkillHandler != nil {
		userSkillHandler.RegisterRoutes(r)
	}
}
