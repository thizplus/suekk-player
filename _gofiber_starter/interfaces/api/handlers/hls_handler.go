package handlers

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"gofiber-template/domain/ports"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

type HLSHandler struct {
	videoService services.VideoService
	storage      ports.StoragePort
	cdnBaseURL   string // Cloudflare CDN URL
	jwtSecret    string // JWT Secret สำหรับ sign token
}

func NewHLSHandler(videoService services.VideoService, storage ports.StoragePort, cdnBaseURL, jwtSecret string) *HLSHandler {
	return &HLSHandler{
		videoService: videoService,
		storage:      storage,
		cdnBaseURL:   cdnBaseURL,
		jwtSecret:    jwtSecret,
	}
}

// HLSAccessClaims JWT claims สำหรับ HLS access
type HLSAccessClaims struct {
	VideoCode string `json:"video_code"`
	VideoID   string `json:"video_id"`
	jwt.RegisteredClaims
}

// GetAccess สร้าง JWT token และ URL สำหรับเข้าถึง HLS
// GET /api/v1/hls/:code/access
func (h *HLSHandler) GetAccess(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Video code is required",
		})
	}

	// ตรวจสอบว่า video มีอยู่และพร้อม stream
	video, err := h.videoService.GetByCode(ctx, code)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "code", code)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Video not found",
		})
	}

	if !video.IsReady() {
		logger.WarnContext(ctx, "Video not ready", "code", code, "status", video.Status)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Video is not ready for streaming",
		})
	}

	// สร้าง JWT token (หมดอายุ 4 ชั่วโมง)
	expiresAt := time.Now().Add(4 * time.Hour)
	claims := HLSAccessClaims{
		VideoCode: video.Code,
		VideoID:   video.ID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "suekk-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		logger.ErrorContext(ctx, "Failed to sign JWT", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate access token",
		})
	}

	// สร้าง URL สำหรับ HLS playlist (ผ่าน Cloudflare CDN)
	// Format: {cdnBaseURL}/hls/{videoCode}/master.m3u8?token={jwt}
	playlistURL := fmt.Sprintf("%s/hls/%s/master.m3u8?token=%s",
		h.cdnBaseURL, video.Code, tokenString)

	// Increment views
	go h.videoService.IncrementViews(ctx, video.ID)

	logger.InfoContext(ctx, "HLS access granted", "code", code, "expires_at", expiresAt)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"video_id":     video.ID,
			"video_code":   video.Code,
			"title":        video.Title,
			"playlist_url": playlistURL,
			"token":        tokenString,
			"expires_at":   expiresAt.Unix(),
			"cdn_base_url": h.cdnBaseURL,
		},
	})
}

// GetAccessByID สร้าง access token โดยใช้ video ID
func (h *HLSHandler) GetAccessByID(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	if idParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Video ID is required",
		})
	}

	// Parse UUID
	video, err := h.videoService.GetByCode(ctx, idParam)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "id", idParam)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Video not found",
		})
	}

	// Redirect ไปยัง GetAccess
	c.Request().URI().SetPath("/api/v1/hls/" + video.Code + "/access")
	return h.GetAccess(c)
}

// VerifyToken endpoint สำหรับ debug
// GET /api/v1/hls/verify?token=xxx
func (h *HLSHandler) VerifyToken(c *fiber.Ctx) error {
	tokenString := c.Query("token")
	if tokenString == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Token is required",
		})
	}

	claims := &HLSAccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid token",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"video_code": claims.VideoCode,
			"video_id":   claims.VideoID,
			"expires_at": claims.ExpiresAt.Time.Unix(),
			"issued_at":  claims.IssuedAt.Time.Unix(),
		},
	})
}

// ServeHLS serves HLS files from storage (IDrive/S3) with byte range support
// Route: /hls/:code/*filepath
func (h *HLSHandler) ServeHLS(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")
	filePath := c.Params("*")

	if code == "" || filePath == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid path")
	}

	// Set content type based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	// Token validation only for master.m3u8
	// Sub-playlists (720p/playlist.m3u8) และ .ts segments ไม่ต้องตรวจ
	// เพราะ Chromecast โหลด sub-playlist ด้วย relative URL (ไม่มี token)
	isMasterPlaylist := ext == ".m3u8" && (filePath == "master.m3u8" || strings.HasSuffix(filePath, "/master.m3u8"))
	if isMasterPlaylist {
		tokenString := c.Get("X-Stream-Token")
		if tokenString == "" {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
		}

		claims := &HLSAccessClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(h.jwtSecret), nil
		})

		if err != nil || !token.Valid {
			logger.WarnContext(ctx, "Invalid HLS token", "code", code, "error", err)
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid token")
		}

		if claims.VideoCode != code {
			return c.Status(fiber.StatusForbidden).SendString("Forbidden")
		}
	}

	// Construct storage path: hls/{code}/{filepath}
	storagePath := fmt.Sprintf("hls/%s/%s", code, filePath)
	contentType := "application/octet-stream"
	switch ext {
	case ".m3u8":
		contentType = "application/vnd.apple.mpegurl"
	case ".ts":
		contentType = "video/MP2T"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	}

	// Set common headers
	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "inline") // ป้องกัน IDM ดักจับ
	c.Set("Accept-Ranges", "bytes")
	c.Set("Cache-Control", "public, max-age=31536000")

	// CORS headers for Chromecast (no Origin header sent)
	// Allow all origins for HLS streaming
	origin := c.Get("Origin")
	if origin == "" {
		// Chromecast doesn't send Origin - allow all
		c.Set("Access-Control-Allow-Origin", "*")
	} else {
		c.Set("Access-Control-Allow-Origin", origin)
	}
	c.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Set("Access-Control-Allow-Headers", "Range, X-Stream-Token")
	c.Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

	// Check for Range header (byte range requests)
	rangeHeader := c.Get("Range")

	// For .ts files with byte range requests, use GetFileRange
	if ext == ".ts" && rangeHeader != "" {
		return h.serveRangeRequest(c, storagePath, rangeHeader)
	}

	// For non-range requests or .m3u8 files, serve the full file
	reader, _, err := h.storage.GetFileContent(storagePath)
	if err != nil {
		logger.WarnContext(ctx, "HLS file not found", "path", storagePath, "error", err)
		return c.Status(fiber.StatusNotFound).SendString("File not found")
	}
	defer reader.Close()

	// Stream the file
	_, err = io.Copy(c.Response().BodyWriter(), reader)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to stream HLS file", "path", storagePath, "error", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Stream error")
	}

	return nil
}

// ServeSubtitle serves subtitle files from storage (IDrive/S3)
// Route: /subtitles/:code/*filepath
func (h *HLSHandler) ServeSubtitle(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")
	filePath := c.Params("*")

	if code == "" || filePath == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid path")
	}

	// Validate token for subtitle files
	tokenString := c.Get("X-Stream-Token")
	if tokenString == "" {
		tokenString = c.Query("token")
	}

	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	claims := &HLSAccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		logger.WarnContext(ctx, "Invalid subtitle token", "code", code, "error", err)
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid token")
	}

	if claims.VideoCode != code {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	// Construct storage path: subtitles/{code}/{filepath}
	storagePath := fmt.Sprintf("subtitles/%s/%s", code, filePath)

	// Set content type based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := "text/plain; charset=utf-8"
	switch ext {
	case ".srt":
		contentType = "application/x-subrip; charset=utf-8"
	case ".vtt":
		contentType = "text/vtt; charset=utf-8"
	case ".ass", ".ssa":
		contentType = "text/x-ssa; charset=utf-8"
	}

	// Set headers
	c.Set("Content-Type", contentType)
	c.Set("Cache-Control", "public, max-age=86400") // Cache 1 day

	// CORS headers for Chromecast (Chromecast receiver needs to fetch subtitle)
	origin := c.Get("Origin")
	if origin == "" {
		// Chromecast doesn't send Origin - allow all
		c.Set("Access-Control-Allow-Origin", "*")
	} else {
		c.Set("Access-Control-Allow-Origin", origin)
	}
	c.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Set("Access-Control-Allow-Headers", "X-Stream-Token")

	// Get file from storage
	reader, _, err := h.storage.GetFileContent(storagePath)
	if err != nil {
		logger.WarnContext(ctx, "Subtitle file not found", "path", storagePath, "error", err)
		return c.Status(fiber.StatusNotFound).SendString("File not found")
	}
	defer reader.Close()

	// Stream the file
	_, err = io.Copy(c.Response().BodyWriter(), reader)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to stream subtitle file", "path", storagePath, "error", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Stream error")
	}

	logger.InfoContext(ctx, "Subtitle served", "code", code, "path", filePath)
	return nil
}

// ServeReel serves reel output files from storage (IDrive/S3)
// Route: /stream/reels/:reelId/*filepath
// Storage path: reels/{reelId}/output.mp4, reels/{reelId}/thumb.jpg, etc.
func (h *HLSHandler) ServeReel(c *fiber.Ctx) error {
	ctx := c.UserContext()
	reelId := c.Params("code") // param name is :code but it's actually reel_id
	filePath := c.Params("*")

	if reelId == "" || filePath == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid path")
	}

	// Validate X-Stream-Token header
	tokenString := c.Get("X-Stream-Token")
	if tokenString == "" {
		// Fallback to query param for compatibility
		tokenString = c.Query("token")
	}

	if tokenString == "" {
		logger.WarnContext(ctx, "Missing reel token", "reel_id", reelId, "path", filePath)
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	// Parse and validate JWT (just check if token is valid, no video code match for reels)
	claims := &HLSAccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		logger.WarnContext(ctx, "Invalid reel token", "reel_id", reelId, "error", err)
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid token")
	}

	// Note: We don't validate video code for reels since path is reels/{reel_id}/...
	// Token just needs to be valid (user is authenticated)

	// Construct storage path: reels/{reelId}/{filepath}
	storagePath := fmt.Sprintf("reels/%s/%s", reelId, filePath)

	// Set content type based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := "application/octet-stream"
	switch ext {
	case ".mp4":
		contentType = "video/mp4"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	}

	// Set headers
	c.Set("Content-Type", contentType)
	c.Set("Cache-Control", "public, max-age=86400") // Cache 1 day
	c.Set("Accept-Ranges", "bytes")

	// Check for Range header (for video seeking)
	rangeHeader := c.Get("Range")
	if ext == ".mp4" && rangeHeader != "" {
		return h.serveRangeRequest(c, storagePath, rangeHeader)
	}

	// Get file from storage
	reader, _, err := h.storage.GetFileContent(storagePath)
	if err != nil {
		logger.WarnContext(ctx, "Reel file not found", "path", storagePath, "error", err)
		return c.Status(fiber.StatusNotFound).SendString("File not found")
	}
	defer reader.Close()

	// Stream the file
	_, err = io.Copy(c.Response().BodyWriter(), reader)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to stream reel file", "path", storagePath, "error", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Stream error")
	}

	logger.InfoContext(ctx, "Reel served", "reel_id", reelId, "path", filePath)
	return nil
}

// ServeGallery streams gallery images from storage
// GET /gallery/:code/001.jpg
func (h *HLSHandler) ServeGallery(c *fiber.Ctx) error {
	ctx := c.UserContext()
	videoCode := c.Params("code")
	filePath := c.Params("*")

	if videoCode == "" || filePath == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid path")
	}

	// Validate X-Stream-Token header
	tokenString := c.Get("X-Stream-Token")
	if tokenString == "" {
		tokenString = c.Query("token")
	}

	if tokenString == "" {
		logger.WarnContext(ctx, "Missing gallery token", "code", videoCode, "path", filePath)
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	// Parse and validate JWT
	claims := &HLSAccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		logger.WarnContext(ctx, "Invalid gallery token", "code", videoCode, "error", err)
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid token")
	}

	// Construct storage path: gallery/{code}/{filepath}
	storagePath := fmt.Sprintf("gallery/%s/%s", videoCode, filePath)

	// Set content type for images
	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	}

	// Set headers
	c.Set("Content-Type", contentType)
	c.Set("Cache-Control", "public, max-age=31536000") // Cache 1 year (images don't change)

	// Get file from storage
	reader, _, err := h.storage.GetFileContent(storagePath)
	if err != nil {
		logger.WarnContext(ctx, "Gallery file not found", "path", storagePath, "error", err)
		return c.Status(fiber.StatusNotFound).SendString("File not found")
	}
	defer reader.Close()

	// Stream the file
	_, err = io.Copy(c.Response().BodyWriter(), reader)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to stream gallery file", "path", storagePath, "error", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Stream error")
	}

	return nil
}

// serveRangeRequest handles HTTP Range requests for byte-range HLS
func (h *HLSHandler) serveRangeRequest(c *fiber.Ctx, storagePath, rangeHeader string) error {
	ctx := c.UserContext()

	// Parse Range header: "bytes=start-end" or "bytes=start-"
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeHeader, "-")
	if len(parts) != 2 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid range format")
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid range start")
	}

	var end int64 = -1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid range end")
		}
	}

	// Get file with byte range from storage
	reader, totalSize, err := h.storage.GetFileRange(storagePath, start, end)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get file range", "path", storagePath, "start", start, "end", end, "error", err)
		return c.Status(fiber.StatusNotFound).SendString("File not found")
	}
	defer reader.Close()

	// Calculate actual end position
	if end < 0 || end >= totalSize {
		end = totalSize - 1
	}
	contentLength := end - start + 1

	// Set 206 Partial Content headers
	c.Status(fiber.StatusPartialContent)
	c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize))
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))

	// Stream the range
	_, err = io.Copy(c.Response().BodyWriter(), reader)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to stream range", "path", storagePath, "error", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Stream error")
	}

	return nil
}
