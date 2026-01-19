package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type FileHandler struct {
	fileService services.FileService
}

func NewFileHandler(fileService services.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

// UploadFile handles file uploads with flexible path support
// Supports two approaches:
// 1. Custom Path: Provide 'custom_path' in form-data to specify exact upload path
// 2. Structured Path: Use 'category', 'entity_id', and 'file_type' for organized structure
func (h *FileHandler) UploadFile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	file, err := c.FormFile("file")
	if err != nil {
		logger.WarnContext(ctx, "No file provided", "error", err)
		return utils.BadRequestResponse(c, "No file provided")
	}

	if file.Size == 0 {
		logger.WarnContext(ctx, "Empty file not allowed", "filename", file.Filename)
		return utils.BadRequestResponse(c, "Empty file not allowed")
	}

	options := &dto.UploadFileRequest{
		CustomPath: c.FormValue("custom_path"),
		Category:   c.FormValue("category"),
		EntityID:   c.FormValue("entity_id"),
		FileType:   c.FormValue("file_type"),
	}

	if err := utils.ValidateStruct(options); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	hasCustomPath := options.CustomPath != ""
	hasStructuredPath := options.Category != "" || options.EntityID != "" || options.FileType != ""

	if hasCustomPath && hasStructuredPath {
		logger.WarnContext(ctx, "Cannot use both custom_path and structured path fields")
		return utils.BadRequestResponse(c, "Cannot use both custom_path and structured path fields simultaneously")
	}

	logger.InfoContext(ctx, "File upload attempt", "user_id", user.ID, "filename", file.Filename, "size", file.Size)

	fileModel, err := h.fileService.UploadFile(ctx, user.ID, file, options)
	if err != nil {
		logger.WarnContext(ctx, "File upload failed", "user_id", user.ID, "filename", file.Filename, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	pathType := "structured"
	if options.CustomPath != "" {
		pathType = "custom"
	}

	logger.InfoContext(ctx, "File uploaded", "file_id", fileModel.ID, "user_id", user.ID, "path_type", pathType)

	uploadResponse := &dto.UploadResponse{
		FileID:   fileModel.ID,
		FileName: fileModel.FileName,
		URL:      fileModel.URL,
		CDNPath:  fileModel.CDNPath,
		FileSize: fileModel.FileSize,
		MimeType: fileModel.MimeType,
		PathType: pathType,
	}

	return utils.CreatedResponse(c, uploadResponse)
}

func (h *FileHandler) GetFile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	fileIDStr := c.Params("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid file ID", "file_id", fileIDStr)
		return utils.BadRequestResponse(c, "Invalid file ID")
	}

	file, err := h.fileService.GetFile(ctx, fileID)
	if err != nil {
		logger.WarnContext(ctx, "File not found", "file_id", fileID)
		return utils.NotFoundResponse(c, "File not found")
	}

	fileResponse := dto.FileToFileResponse(file)
	return utils.SuccessResponse(c, fileResponse)
}

func (h *FileHandler) DeleteFile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	fileIDStr := c.Params("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid file ID", "file_id", fileIDStr)
		return utils.BadRequestResponse(c, "Invalid file ID")
	}

	logger.InfoContext(ctx, "File deletion attempt", "file_id", fileID)

	err = h.fileService.DeleteFile(ctx, fileID)
	if err != nil {
		logger.WarnContext(ctx, "File deletion failed", "file_id", fileID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "File deleted", "file_id", fileID)

	return utils.NoContentResponse(c)
}

func (h *FileHandler) GetUserFiles(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

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
	files, total, err := h.fileService.GetUserFiles(ctx, user.ID, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retrieve user files", "user_id", user.ID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	fileResponses := make([]dto.FileResponse, len(files))
	for i, file := range files {
		fileResponses[i] = *dto.FileToFileResponse(file)
	}

	return utils.PaginatedSuccessResponse(c, fileResponses, total, page, limit)
}

func (h *FileHandler) ListFiles(c *fiber.Ctx) error {
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
	files, total, err := h.fileService.ListFiles(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retrieve files", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	fileResponses := make([]dto.FileResponse, len(files))
	for i, file := range files {
		fileResponses[i] = *dto.FileToFileResponse(file)
	}

	return utils.PaginatedSuccessResponse(c, fileResponses, total, page, limit)
}
