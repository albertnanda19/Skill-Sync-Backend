package response

import "github.com/gofiber/fiber/v3"

type SemanticResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Error(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(SemanticResponse{Status: status, Message: message, Data: nil})
}
