package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type UserHandler struct {
	userService services.UserService
}

func NewUserHandler(userService services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) Register(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Registration attempt", "email", req.Email, "username", req.Username)

	user, err := h.userService.Register(ctx, &req)
	if err != nil {
		logger.WarnContext(ctx, "Registration failed", "email", req.Email, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "User registered", "user_id", user.ID, "email", user.Email)

	userResponse := dto.UserToUserResponse(user)
	return utils.CreatedResponse(c, userResponse)
}

func (h *UserHandler) Login(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Login attempt", "email", req.Email)

	token, user, err := h.userService.Login(ctx, &req)
	if err != nil {
		logger.WarnContext(ctx, "Login failed", "email", req.Email, "reason", err.Error())
		return utils.UnauthorizedResponse(c, "Invalid credentials")
	}

	logger.InfoContext(ctx, "Login successful", "user_id", user.ID, "email", user.Email)

	loginResponse := &dto.LoginResponse{
		Token: token,
		User:  *dto.UserToUserResponse(user),
	}
	return utils.SuccessResponse(c, loginResponse)
}

func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	profile, err := h.userService.GetProfile(ctx, user.ID)
	if err != nil {
		logger.WarnContext(ctx, "Profile not found", "user_id", user.ID)
		return utils.NotFoundResponse(c, "User not found")
	}

	profileResponse := dto.UserToUserResponse(profile)
	return utils.SuccessResponse(c, profileResponse)
}

func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	var req dto.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	logger.InfoContext(ctx, "Profile update attempt", "user_id", user.ID)

	updatedUser, err := h.userService.UpdateProfile(ctx, user.ID, &req)
	if err != nil {
		logger.ErrorContext(ctx, "Profile update failed", "user_id", user.ID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Profile updated", "user_id", user.ID)

	userResponse := dto.UserToUserResponse(updatedUser)
	return utils.SuccessResponse(c, userResponse)
}

func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	logger.InfoContext(ctx, "User deletion attempt", "user_id", user.ID)

	err = h.userService.DeleteUser(ctx, user.ID)
	if err != nil {
		logger.ErrorContext(ctx, "User deletion failed", "user_id", user.ID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "User deleted", "user_id", user.ID)

	return utils.NoContentResponse(c)
}

func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	ctx := c.UserContext()

	pageStr := c.Query("page", "1")
	limitStr := c.Query("limit", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		logger.WarnContext(ctx, "Invalid page parameter", "page", pageStr)
		return utils.BadRequestResponse(c, "Invalid page parameter")
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		logger.WarnContext(ctx, "Invalid limit parameter", "limit", limitStr)
		return utils.BadRequestResponse(c, "Invalid limit parameter")
	}

	offset := (page - 1) * limit
	users, total, err := h.userService.ListUsers(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retrieve users", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	userResponses := make([]dto.UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = *dto.UserToUserResponse(user)
	}

	return utils.PaginatedSuccessResponse(c, userResponses, total, page, limit)
}

// SetPassword ตั้ง password สำหรับ Google users ที่ยังไม่มี password
// POST /api/v1/users/set-password
func (h *UserHandler) SetPassword(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	var req dto.SetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Set password attempt", "user_id", user.ID)

	if err := h.userService.SetPassword(ctx, user.ID, &req); err != nil {
		logger.WarnContext(ctx, "Set password failed", "user_id", user.ID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Password set successfully", "user_id", user.ID)

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Password set successfully",
	})
}
