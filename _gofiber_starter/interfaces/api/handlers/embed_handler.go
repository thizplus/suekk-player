package handlers

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/gofiber/fiber/v2"

	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

type EmbedHandler struct {
	videoService services.VideoService
	baseURL      string
	template     *template.Template
}

func NewEmbedHandler(videoService services.VideoService, baseURL string) *EmbedHandler {
	// Parse embed template
	tmpl := template.Must(template.New("embed").Parse(embedHTML))

	return &EmbedHandler{
		videoService: videoService,
		baseURL:      baseURL,
		template:     tmpl,
	}
}

// EmbedData ข้อมูลที่ส่งให้ template
type EmbedData struct {
	VideoCode    string
	VideoTitle   string
	Duration     int
	ThumbnailURL string
	StreamURL    string        // HLS URL (H.265)
	StreamURLH264 string       // HLS URL (H.264 fallback)
	HasH264      bool
	Qualities    []QualityInfo
	BaseURL      string
}

// QualityInfo ข้อมูล quality
type QualityInfo struct {
	Name   string `json:"name"`   // "1080p", "720p", "480p"
	URL    string `json:"url"`
	Height int    `json:"height"`
}

// ServeEmbed serves the embed player HTML page
func (h *EmbedHandler) ServeEmbed(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Video code is required")
	}

	// ดึงข้อมูล video
	video, err := h.videoService.GetByCode(ctx, code)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for embed", "code", code)
		return c.Status(fiber.StatusNotFound).SendString("Video not found")
	}

	// ตรวจสอบว่า video พร้อม
	if !video.IsReady() {
		logger.WarnContext(ctx, "Video not ready for embed", "code", code, "status", video.Status)
		return c.Status(fiber.StatusServiceUnavailable).SendString("Video is not ready")
	}

	// Increment views
	go h.videoService.IncrementViews(ctx, video.ID)

	// สร้าง streaming URLs
	baseURL := strings.TrimSuffix(h.baseURL, "/")
	streamURL := fmt.Sprintf("%s/stream/%s/master.m3u8", baseURL, video.Code)

	var streamURLH264 string
	if video.HasH264Fallback() {
		streamURLH264 = fmt.Sprintf("%s/stream/%s/h264/master.m3u8", baseURL, video.Code)
	}

	thumbnailURL := video.ThumbnailURL
	if thumbnailURL == "" {
		thumbnailURL = fmt.Sprintf("%s/stream/%s/thumb", baseURL, video.Code)
	}

	// สร้าง embed data
	data := EmbedData{
		VideoCode:     video.Code,
		VideoTitle:    video.Title,
		Duration:      video.Duration,
		ThumbnailURL:  thumbnailURL,
		StreamURL:     streamURL,
		StreamURLH264: streamURLH264,
		HasH264:       video.HasH264Fallback(),
		BaseURL:       baseURL,
	}

	// Set content type
	c.Set("Content-Type", "text/html; charset=utf-8")

	// Render template
	var buf strings.Builder
	if err := h.template.Execute(&buf, data); err != nil {
		logger.ErrorContext(ctx, "Failed to render embed template", "error", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render player")
	}

	return c.SendString(buf.String())
}

// GetEmbedInfo returns video info for embed (JSON)
func (h *EmbedHandler) GetEmbedInfo(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Video code is required",
		})
	}

	video, err := h.videoService.GetByCode(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Video not found",
		})
	}

	if !video.IsReady() {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Video is not ready",
		})
	}

	baseURL := strings.TrimSuffix(h.baseURL, "/")

	return c.JSON(fiber.Map{
		"code":         video.Code,
		"title":        video.Title,
		"duration":     video.Duration,
		"quality":      video.Quality,
		"thumbnail":    video.ThumbnailURL,
		"streamUrl":    fmt.Sprintf("%s/stream/%s/master.m3u8", baseURL, video.Code),
		"streamUrlH264": func() string {
			if video.HasH264Fallback() {
				return fmt.Sprintf("%s/stream/%s/h264/master.m3u8", baseURL, video.Code)
			}
			return ""
		}(),
		"hasH264":      video.HasH264Fallback(),
	})
}

// GetEmbedCode returns embed code snippets
func (h *EmbedHandler) GetEmbedCode(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Video code is required",
		})
	}

	video, err := h.videoService.GetByCode(ctx, code)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Video not found",
		})
	}

	baseURL := strings.TrimSuffix(h.baseURL, "/")
	embedURL := fmt.Sprintf("%s/embed/%s", baseURL, video.Code)

	// Generate different embed code formats
	iframe := fmt.Sprintf(`<iframe src="%s" width="640" height="360" frameborder="0" allowfullscreen allow="autoplay; encrypted-media"></iframe>`, embedURL)

	responsive := fmt.Sprintf(`<div style="position:relative;padding-bottom:56.25%%;height:0;overflow:hidden;">
  <iframe src="%s" style="position:absolute;top:0;left:0;width:100%%;height:100%%;" frameborder="0" allowfullscreen allow="autoplay; encrypted-media"></iframe>
</div>`, embedURL)

	directLink := fmt.Sprintf("%s/stream/%s/master.m3u8", baseURL, video.Code)

	return c.JSON(fiber.Map{
		"code":       video.Code,
		"title":      video.Title,
		"embedUrl":   embedURL,
		"iframe":     iframe,
		"responsive": responsive,
		"directHls":  directLink,
	})
}

// Embed HTML Template
const embedHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.VideoTitle}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        html, body {
            width: 100%;
            height: 100%;
            background: #000;
            overflow: hidden;
        }
        .player-container {
            position: relative;
            width: 100%;
            height: 100%;
        }
        video {
            width: 100%;
            height: 100%;
            background: #000;
        }
        .loading {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            color: #fff;
            font-family: Arial, sans-serif;
            text-align: center;
        }
        .loading-spinner {
            width: 50px;
            height: 50px;
            border: 4px solid rgba(255,255,255,0.3);
            border-top-color: #fff;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 15px;
        }
        @keyframes spin {
            to { transform: rotate(360deg); }
        }
        .error {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            color: #ff4444;
            font-family: Arial, sans-serif;
            text-align: center;
            display: none;
        }
        .controls {
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            background: linear-gradient(transparent, rgba(0,0,0,0.8));
            padding: 20px;
            opacity: 0;
            transition: opacity 0.3s;
        }
        .player-container:hover .controls {
            opacity: 1;
        }
        .quality-selector {
            position: absolute;
            bottom: 60px;
            right: 20px;
            background: rgba(0,0,0,0.8);
            border-radius: 5px;
            padding: 5px 0;
            display: none;
        }
        .quality-selector.active {
            display: block;
        }
        .quality-option {
            padding: 8px 15px;
            color: #fff;
            cursor: pointer;
            font-family: Arial, sans-serif;
            font-size: 14px;
        }
        .quality-option:hover {
            background: rgba(255,255,255,0.2);
        }
        .quality-option.active {
            color: #00bfff;
        }
        .quality-btn {
            background: rgba(255,255,255,0.2);
            border: none;
            color: #fff;
            padding: 8px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
            position: absolute;
            bottom: 20px;
            right: 20px;
        }
        .quality-btn:hover {
            background: rgba(255,255,255,0.3);
        }
    </style>
</head>
<body>
    <div class="player-container" id="playerContainer">
        <video id="video" controls playsinline poster="{{.ThumbnailURL}}"></video>

        <div class="loading" id="loading">
            <div class="loading-spinner"></div>
            <div>Loading video...</div>
        </div>

        <div class="error" id="error">
            <div style="font-size: 48px; margin-bottom: 10px;">⚠️</div>
            <div id="errorMessage">Failed to load video</div>
        </div>

        <button class="quality-btn" id="qualityBtn" style="display:none;">
            <span id="currentQuality">Auto</span> ⚙️
        </button>

        <div class="quality-selector" id="qualitySelector"></div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/hls.js@1"></script>
    <script>
        const videoData = {
            code: "{{.VideoCode}}",
            title: "{{.VideoTitle}}",
            streamUrl: "{{.StreamURL}}",
            streamUrlH264: "{{.StreamURLH264}}",
            hasH264: {{.HasH264}},
            thumbnail: "{{.ThumbnailURL}}"
        };

        const video = document.getElementById('video');
        const loading = document.getElementById('loading');
        const error = document.getElementById('error');
        const errorMessage = document.getElementById('errorMessage');
        const qualityBtn = document.getElementById('qualityBtn');
        const qualitySelector = document.getElementById('qualitySelector');
        const currentQualitySpan = document.getElementById('currentQuality');

        let hls = null;
        let currentLevelIndex = -1; // -1 = auto

        function hideLoading() {
            loading.style.display = 'none';
        }

        function showError(msg) {
            loading.style.display = 'none';
            error.style.display = 'block';
            errorMessage.textContent = msg;
        }

        function canPlayHEVC() {
            const video = document.createElement('video');
            // Check for HEVC/H.265 support
            const hevcTypes = [
                'video/mp4; codecs="hvc1"',
                'video/mp4; codecs="hev1"',
                'video/mp4; codecs="hevc"'
            ];
            for (const type of hevcTypes) {
                if (video.canPlayType(type) === 'probably' || video.canPlayType(type) === 'maybe') {
                    return true;
                }
            }
            return false;
        }

        function setupQualitySelector(levels) {
            if (!levels || levels.length <= 1) return;

            qualityBtn.style.display = 'block';
            qualitySelector.innerHTML = '';

            // Add Auto option
            const autoOption = document.createElement('div');
            autoOption.className = 'quality-option active';
            autoOption.textContent = 'Auto';
            autoOption.onclick = () => selectQuality(-1);
            qualitySelector.appendChild(autoOption);

            // Add quality levels
            levels.forEach((level, index) => {
                const option = document.createElement('div');
                option.className = 'quality-option';
                option.textContent = level.height + 'p';
                option.onclick = () => selectQuality(index);
                qualitySelector.appendChild(option);
            });

            // Toggle selector
            qualityBtn.onclick = (e) => {
                e.stopPropagation();
                qualitySelector.classList.toggle('active');
            };

            // Close selector on click outside
            document.addEventListener('click', () => {
                qualitySelector.classList.remove('active');
            });
        }

        function selectQuality(levelIndex) {
            if (!hls) return;

            currentLevelIndex = levelIndex;
            hls.currentLevel = levelIndex;

            // Update UI
            const options = qualitySelector.querySelectorAll('.quality-option');
            options.forEach((opt, i) => {
                opt.classList.toggle('active', i === levelIndex + 1);
            });

            if (levelIndex === -1) {
                currentQualitySpan.textContent = 'Auto';
            } else {
                const level = hls.levels[levelIndex];
                currentQualitySpan.textContent = level.height + 'p';
            }

            qualitySelector.classList.remove('active');
        }

        function initPlayer() {
            const canHEVC = canPlayHEVC();
            let streamUrl = videoData.streamUrl;

            // If can't play HEVC and H.264 is available, use H.264
            if (!canHEVC && videoData.hasH264 && videoData.streamUrlH264) {
                console.log('Using H.264 fallback');
                streamUrl = videoData.streamUrlH264;
            }

            if (Hls.isSupported()) {
                hls = new Hls({
                    enableWorker: true,
                    lowLatencyMode: false,
                    backBufferLength: 90
                });

                hls.loadSource(streamUrl);
                hls.attachMedia(video);

                hls.on(Hls.Events.MANIFEST_PARSED, function(event, data) {
                    hideLoading();
                    setupQualitySelector(hls.levels);
                    video.play().catch(() => {});
                });

                hls.on(Hls.Events.LEVEL_SWITCHED, function(event, data) {
                    if (currentLevelIndex === -1) {
                        const level = hls.levels[data.level];
                        currentQualitySpan.textContent = 'Auto (' + level.height + 'p)';
                    }
                });

                hls.on(Hls.Events.ERROR, function(event, data) {
                    console.error('HLS Error:', data);
                    if (data.fatal) {
                        switch (data.type) {
                            case Hls.ErrorTypes.NETWORK_ERROR:
                                console.log('Network error, trying to recover...');
                                hls.startLoad();
                                break;
                            case Hls.ErrorTypes.MEDIA_ERROR:
                                console.log('Media error, trying to recover...');
                                hls.recoverMediaError();
                                break;
                            default:
                                showError('Failed to load video');
                                hls.destroy();
                                break;
                        }
                    }
                });

            } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
                // Native HLS support (Safari)
                video.src = streamUrl;
                video.addEventListener('loadedmetadata', function() {
                    hideLoading();
                    video.play().catch(() => {});
                });
                video.addEventListener('error', function() {
                    showError('Failed to load video');
                });
            } else {
                showError('HLS is not supported in your browser');
            }
        }

        // Start player
        initPlayer();
    </script>
</body>
</html>`
