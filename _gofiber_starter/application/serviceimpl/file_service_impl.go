package serviceimpl

import (
	"context"
	"errors"
	"fmt"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FileServiceImpl struct {
	fileRepo repositories.FileRepository
	userRepo repositories.UserRepository
	storage  ports.StoragePort
}

func NewFileService(fileRepo repositories.FileRepository, userRepo repositories.UserRepository, storage ports.StoragePort) services.FileService {
	return &FileServiceImpl{
		fileRepo: fileRepo,
		userRepo: userRepo,
		storage:  storage,
	}
}

func (s *FileServiceImpl) UploadFile(ctx context.Context, userID uuid.UUID, fileHeader *multipart.FileHeader, options *dto.UploadFileRequest) (*models.File, error) {
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.WarnContext(ctx, "User not found for file upload", "user_id", userID)
		return nil, errors.New("user not found")
	}

	file, err := fileHeader.Open()
	if err != nil {
		logger.ErrorContext(ctx, "Failed to open uploaded file", "filename", fileHeader.Filename, "error", err)
		return nil, err
	}
	defer file.Close()

	// Sanitize the filename
	sanitizedFileName := utils.SanitizeFileName(fileHeader.Filename)
	fileExt := filepath.Ext(sanitizedFileName)
	uniqueFileName := fmt.Sprintf("%s%s", uuid.New().String(), fileExt)

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = s.getMimeTypeFromExtension(fileExt)
	}

	// Determine the path based on whether custom path is provided
	var cdnPath string
	if options != nil && options.CustomPath != "" {
		// Use custom path approach
		validatedPath, err := utils.ValidateAndSanitizePath(options.CustomPath)
		if err != nil {
			logger.WarnContext(ctx, "Invalid custom path", "custom_path", options.CustomPath, "error", err)
			return nil, fmt.Errorf("invalid custom path: %w", err)
		}
		cdnPath = filepath.Join(validatedPath, uniqueFileName)
	} else {
		// Use structured path approach
		category := ""
		entityID := ""
		fileType := ""

		if options != nil {
			category = options.Category
			entityID = options.EntityID
			fileType = options.FileType
		}

		structuredPath := utils.GenerateStructuredPath(userID.String(), category, entityID, fileType)
		cdnPath = filepath.Join(structuredPath, uniqueFileName)
	}

	// Normalize path separators for storage
	cdnPath = strings.ReplaceAll(cdnPath, "\\", "/")

	logger.InfoContext(ctx, "Uploading file to storage", "user_id", userID, "cdn_path", cdnPath, "size", fileHeader.Size)

	url, err := s.storage.UploadFile(file, cdnPath, mimeType)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to upload file to storage", "cdn_path", cdnPath, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "File uploaded to storage successfully", "cdn_path", cdnPath, "url", url)

	fileModel := &models.File{
		ID:        uuid.New(),
		FileName:  sanitizedFileName,
		FileSize:  fileHeader.Size,
		MimeType:  mimeType,
		URL:       url,
		CDNPath:   cdnPath,
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.fileRepo.Create(ctx, fileModel)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to save file record, rolling back storage", "file_id", fileModel.ID, "error", err)
		s.storage.DeleteFile(cdnPath)
		return nil, err
	}

	logger.InfoContext(ctx, "File record saved successfully", "file_id", fileModel.ID, "user_id", userID)

	return fileModel, nil
}

func (s *FileServiceImpl) GetFile(ctx context.Context, fileID uuid.UUID) (*models.File, error) {
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return nil, errors.New("file not found")
	}
	return file, nil
}

func (s *FileServiceImpl) GetUserFiles(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.File, int64, error) {
	files, err := s.fileRepo.GetByUserID(ctx, userID, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get user files", "user_id", userID, "error", err)
		return nil, 0, err
	}

	count, err := s.fileRepo.CountByUserID(ctx, userID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count user files", "user_id", userID, "error", err)
		return nil, 0, err
	}

	return files, count, nil
}

func (s *FileServiceImpl) DeleteFile(ctx context.Context, fileID uuid.UUID) error {
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		logger.WarnContext(ctx, "File not found for deletion", "file_id", fileID)
		return errors.New("file not found")
	}

	logger.InfoContext(ctx, "Deleting file from storage", "file_id", fileID, "cdn_path", file.CDNPath)

	err = s.storage.DeleteFile(file.CDNPath)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete file from storage", "file_id", fileID, "cdn_path", file.CDNPath, "error", err)
		return err
	}

	err = s.fileRepo.Delete(ctx, fileID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete file record", "file_id", fileID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "File deleted successfully", "file_id", fileID)
	return nil
}

func (s *FileServiceImpl) ListFiles(ctx context.Context, offset, limit int) ([]*models.File, int64, error) {
	files, err := s.fileRepo.List(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list files", "offset", offset, "limit", limit, "error", err)
		return nil, 0, err
	}

	count, err := s.fileRepo.Count(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count files", "error", err)
		return nil, 0, err
	}

	return files, count, nil
}

func (s *FileServiceImpl) getMimeTypeFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".pdf":  "application/pdf",
		".txt":  "text/plain",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".zip":  "application/zip",
	}

	if mimeType, exists := mimeTypes[ext]; exists {
		return mimeType
	}
	return "application/octet-stream"
}
