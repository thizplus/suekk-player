package middleware

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"gofiber-template/pkg/utils"
)

func ErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		errCode := utils.ErrCodeInternalError
		message := "Internal server error"

		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
			message = e.Message
			switch code {
			case fiber.StatusBadRequest:
				errCode = utils.ErrCodeBadRequest
			case fiber.StatusUnauthorized:
				errCode = utils.ErrCodeUnauthorized
			case fiber.StatusForbidden:
				errCode = utils.ErrCodeForbidden
			case fiber.StatusNotFound:
				errCode = utils.ErrCodeNotFound
			case fiber.StatusConflict:
				errCode = utils.ErrCodeConflict
			}
		}

		log.Printf("Error: %v", err)

		return utils.ErrorResponse(c, code, errCode, message, nil)
	}
}
