package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gofiber-template/pkg/logger"
)

// LoggerMiddleware structured logging สำหรับทุก request
func LoggerMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Log request start
		logger.InfoContext(c.UserContext(), "Request started",
			"method", c.Method(),
			"path", c.Path(),
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
		)

		// Process request
		err := c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		status := c.Response().StatusCode()

		// Log request completed
		logFunc := logger.InfoContext
		if status >= 500 {
			logFunc = logger.ErrorContext
		} else if status >= 400 {
			logFunc = logger.WarnContext
		}

		logFunc(c.UserContext(), "Request completed",
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"latency", latency.String(),
			"bytes", len(c.Response().Body()),
		)

		return err
	}
}
