package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"seo-worker/domain/ports"
)

const (
	elevenLabsAPIURL = "https://api.elevenlabs.io/v1"
	defaultTimeout   = 60 * time.Second
)

type ElevenLabsClient struct {
	apiKey     string
	voiceID    string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
}

type ElevenLabsConfig struct {
	APIKey  string
	VoiceID string
	Model   string
}

func NewElevenLabsClient(cfg ElevenLabsConfig) *ElevenLabsClient {
	model := cfg.Model
	if model == "" {
		model = "eleven_v3"
	}
	voiceID := cfg.VoiceID
	if voiceID == "" {
		voiceID = "q0IMILNRPxOgtBTS4taI"
	}

	return &ElevenLabsClient{
		apiKey:  cfg.APIKey,
		voiceID: voiceID,
		model:   model,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		logger: slog.Default().With("component", "elevenlabs"),
	}
}

type ttsRequest struct {
	Text          string        `json:"text"`
	ModelID       string        `json:"model_id"`
	VoiceSettings voiceSettings `json:"voice_settings"`
}

type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style"`
	UseSpeakerBoost bool    `json:"use_speaker_boost"`
}

func (c *ElevenLabsClient) GenerateAudio(ctx context.Context, text string, voiceID string) (*ports.TTSResult, error) {
	// Use provided voiceID or fall back to configured default
	if voiceID == "" {
		voiceID = c.voiceID
	}

	url := fmt.Sprintf("%s/text-to-speech/%s", elevenLabsAPIURL, voiceID)

	reqBody := ttsRequest{
		Text:    text,
		ModelID: c.model,
		VoiceSettings: voiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.75,
			Style:           0.0,
			UseSpeakerBoost: true,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("Accept", "audio/mpeg")

	c.logger.InfoContext(ctx, "Generating TTS audio",
		"voice_id", voiceID,
		"char_count", len([]rune(text)),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API error: %d - %s", resp.StatusCode, string(body))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	charCount := len([]rune(text))

	// Calculate duration from MP3 file size (ElevenLabs uses ~128kbps MP3)
	// Duration = (file_size_bytes * 8) / bitrate_bps
	audioSize := len(audioData)
	duration := (audioSize * 8) / 128000 // 128 kbps
	if duration < 1 {
		duration = 1 // minimum 1 second
	}

	c.logger.InfoContext(ctx, "TTS audio generated",
		"voice_id", voiceID,
		"char_count", charCount,
		"audio_size", audioSize,
		"duration_sec", duration,
	)

	return &ports.TTSResult{
		AudioData: audioData,
		Duration:  duration,
		CharCount: charCount,
	}, nil
}

// Verify interface implementation
var _ ports.TTSPort = (*ElevenLabsClient)(nil)
