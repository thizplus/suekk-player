package utils

import (
	"github.com/gofiber/fiber/v2"
)

// ========== Response Structures ==========

type Response struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorInfo `json:"error,omitempty"`
}

type PaginatedResponse struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Meta    Meta       `json:"meta"`
	Error   *ErrorInfo `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Meta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"totalPages"`
	HasNext    bool  `json:"hasNext"`
	HasPrev    bool  `json:"hasPrev"`
}

// ========== Error Code Constants ==========

const (
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeForbidden     = "FORBIDDEN"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeConflict      = "CONFLICT"
	ErrCodeInternalError = "INTERNAL_ERROR"
	ErrCodeBadRequest    = "BAD_REQUEST"
)

// ========== Success Responses ==========

func SuccessResponse(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Data:    data,
	})
}

func CreatedResponse(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Success: true,
		Data:    data,
	})
}

func NoContentResponse(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

func PaginatedSuccessResponse(c *fiber.Ctx, data any, total int64, page, limit int) error {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	return c.Status(fiber.StatusOK).JSON(PaginatedResponse{
		Success: true,
		Data:    data,
		Meta: Meta{
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	})
}

// ========== Error Responses ==========

func ErrorResponse(c *fiber.Ctx, statusCode int, code, message string, details any) error {
	return c.Status(statusCode).JSON(Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func ValidationErrorResponse(c *fiber.Ctx, details any) error {
	return ErrorResponse(
		c,
		fiber.StatusBadRequest,
		ErrCodeValidation,
		"Validation failed",
		details,
	)
}

func BadRequestResponse(c *fiber.Ctx, message string) error {
	return ErrorResponse(
		c,
		fiber.StatusBadRequest,
		ErrCodeBadRequest,
		message,
		nil,
	)
}

func UnauthorizedResponse(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Unauthorized"
	}
	return ErrorResponse(
		c,
		fiber.StatusUnauthorized,
		ErrCodeUnauthorized,
		message,
		nil,
	)
}

func ForbiddenResponse(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Forbidden"
	}
	return ErrorResponse(
		c,
		fiber.StatusForbidden,
		ErrCodeForbidden,
		message,
		nil,
	)
}

func NotFoundResponse(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Resource not found"
	}
	return ErrorResponse(
		c,
		fiber.StatusNotFound,
		ErrCodeNotFound,
		message,
		nil,
	)
}

func ConflictResponse(c *fiber.Ctx, message string) error {
	return ErrorResponse(
		c,
		fiber.StatusConflict,
		ErrCodeConflict,
		message,
		nil,
	)
}

func InternalServerErrorResponse(c *fiber.Ctx) error {
	return ErrorResponse(
		c,
		fiber.StatusInternalServerError,
		ErrCodeInternalError,
		"Internal server error",
		nil,
	)
}
