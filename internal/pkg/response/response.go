package response

import "github.com/gofiber/fiber/v3"

type SemanticResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

const (
	MessageOK                  = "ok"
	MessageBadRequest          = "bad request"
	MessageUnauthorized        = "unauthorized"
	MessageForbidden           = "forbidden"
	MessageNotFound            = "not found"
	MessageConflict            = "conflict"
	MessageUnprocessableEntity = "unprocessable entity"
	MessageInternalServerError = "internal server error"
	MessageError               = "error"
)

func Success(c fiber.Ctx, status int, message string, data interface{}) error {
	st := normalizeStatus(status)
	msg := normalizeMessage(message, st)
	return c.Status(st).JSON(SemanticResponse{Status: st, Message: msg, Data: data})
}

func Error(c fiber.Ctx, status int, message string, data interface{}) error {
	st := normalizeStatus(status)
	msg := normalizeMessage(message, st)
	return c.Status(st).JSON(SemanticResponse{Status: st, Message: msg, Data: data})
}

func normalizeStatus(status int) int {
	if status < 100 || status > 599 {
		return fiber.StatusInternalServerError
	}
	return status
}

func normalizeMessage(message string, status int) string {
	if message != "" {
		return message
	}
	return defaultMessageForStatus(status)
}

func defaultMessageForStatus(status int) string {
	switch status {
	case fiber.StatusOK:
		return MessageOK
	case fiber.StatusBadRequest:
		return MessageBadRequest
	case fiber.StatusUnauthorized:
		return MessageUnauthorized
	case fiber.StatusForbidden:
		return MessageForbidden
	case fiber.StatusNotFound:
		return MessageNotFound
	case fiber.StatusConflict:
		return MessageConflict
	case fiber.StatusUnprocessableEntity:
		return MessageUnprocessableEntity
	default:
		if status >= 500 {
			return MessageInternalServerError
		}
		return MessageError
	}
}
