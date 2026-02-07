package middleware

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type AccessLogMiddleware struct {
	logger *log.Logger
}

func NewAccessLogMiddleware(logger *log.Logger) *AccessLogMiddleware {
	if logger == nil {
		logger = log.Default()
	}
	return &AccessLogMiddleware{logger: logger}
}

func (m *AccessLogMiddleware) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		rid := c.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
			c.Set("X-Request-ID", rid)
		}

		err := c.Next()

		dur := time.Since(start)
		status := c.Response().StatusCode()

		ip := c.IP()
		host := c.Hostname()
		method := c.Method()
		path := c.OriginalURL()

		ua := c.Get("User-Agent")
		referer := c.Get("Referer")

		reqBytes := c.Request().Header.ContentLength()
		respBytes := c.Response().Header.ContentLength()

		if m != nil && m.logger != nil {
			m.logger.Printf(
				"HTTP access | rid=%s ip=%s host=%s method=%s path=%s status=%d latency=%s req_bytes=%d resp_bytes=%d ua=%q referer=%q",
				rid, ip, host, method, path, status, dur, reqBytes, respBytes, ua, referer,
			)
		}

		return err
	}
}
