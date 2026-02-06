package middleware

import (
	"errors"
	"log"

	"skill-sync/internal/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type AppError struct {
	StatusCode int
	Message    string
	Data       interface{}
	Cause      error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewAppError(statusCode int, message string, data interface{}, cause error) *AppError {
	return &AppError{StatusCode: statusCode, Message: message, Data: data, Cause: cause}
}

type ErrorMiddleware struct{}

func NewErrorMiddleware() *ErrorMiddleware {
	return &ErrorMiddleware{}
}

func (m *ErrorMiddleware) Middleware() fiber.Handler {
	return func(c fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered: %v", r)
				err = response.Error(c, fiber.StatusInternalServerError, response.MessageInternalServerError, nil)
			}
		}()

		err = c.Next()
		if err == nil {
			return nil
		}

		status, msg, data := normalizeError(err)
		return response.Error(c, status, msg, data)
	}
}

func normalizeError(err error) (int, string, interface{}) {
	if err == nil {
		return fiber.StatusInternalServerError, response.MessageInternalServerError, nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		if appErr.StatusCode <= 0 {
			return fiber.StatusInternalServerError, response.MessageInternalServerError, nil
		}

		status := appErr.StatusCode
		msg := appErr.Message
		if msg == "" {
			msg = defaultMessageForStatus(status)
		}

		if status >= 500 {
			return fiber.StatusInternalServerError, response.MessageInternalServerError, nil
		}
		return status, msg, appErr.Data
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status := fiberErr.Code
		if status <= 0 {
			status = fiber.StatusInternalServerError
		}

		if status >= 500 {
			return fiber.StatusInternalServerError, response.MessageInternalServerError, nil
		}

		msg := fiberErr.Message
		if msg == "" {
			msg = defaultMessageForStatus(status)
		}
		return status, msg, nil
	}

	return fiber.StatusInternalServerError, response.MessageInternalServerError, nil
}

func defaultMessageForStatus(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return response.MessageBadRequest
	case fiber.StatusUnauthorized:
		return response.MessageUnauthorized
	case fiber.StatusForbidden:
		return response.MessageForbidden
	case fiber.StatusNotFound:
		return response.MessageNotFound
	case fiber.StatusConflict:
		return response.MessageConflict
	case fiber.StatusUnprocessableEntity:
		return response.MessageUnprocessableEntity
	default:
		if status >= 500 {
			return response.MessageInternalServerError
		}
		return response.MessageError
	}
}
