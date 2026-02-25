package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// AuthClient จัดการ authentication สำหรับ API
type AuthClient struct {
	apiURL     string
	email      string
	password   string
	httpClient *http.Client
	logger     *slog.Logger

	// Token cache
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at,omitempty"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

func NewAuthClient(apiURL, email, password string) *AuthClient {
	return &AuthClient{
		apiURL:   apiURL,
		email:    email,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: slog.Default().With("component", "auth_client"),
	}
}

// GetToken คืน valid token (login ใหม่ถ้าหมดอายุ)
func (c *AuthClient) GetToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	// Check if token is still valid (with 5 min buffer)
	if c.token != "" && time.Now().Add(5*time.Minute).Before(c.expiresAt) {
		token := c.token
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// Need to login
	return c.login(ctx)
}

func (c *AuthClient) login(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.token != "" && time.Now().Add(5*time.Minute).Before(c.expiresAt) {
		return c.token, nil
	}

	url := fmt.Sprintf("%s/api/v1/auth/login", c.apiURL)

	reqBody := loginRequest{
		Email:    c.email,
		Password: c.password,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login request: %w", err)
	}

	// Retry logic for transient network errors
	var resp *http.Response
	var lastErr error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
		if err != nil {
			return "", fmt.Errorf("failed to create login request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		c.logger.InfoContext(ctx, "Logging in", "url", url, "email", c.email, "attempt", i+1)

		resp, err = c.httpClient.Do(req)
		if err == nil {
			break // Success
		}

		lastErr = err
		c.logger.WarnContext(ctx, "Login request failed, retrying",
			"attempt", i+1,
			"error", err,
		)

		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 2 * time.Second) // Exponential backoff
		}
	}

	if resp == nil {
		return "", fmt.Errorf("login request failed after %d retries: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed: %d - %s", resp.StatusCode, string(body))
	}

	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	if !loginResp.Success {
		return "", fmt.Errorf("login error: %s", loginResp.Error)
	}

	c.token = loginResp.Data.Token

	// Set expiry (default 7 days if not provided)
	if loginResp.Data.ExpiresAt > 0 {
		c.expiresAt = time.Unix(loginResp.Data.ExpiresAt, 0)
	} else {
		c.expiresAt = time.Now().Add(7 * 24 * time.Hour)
	}

	c.logger.InfoContext(ctx, "Login successful",
		"email", c.email,
		"expires_at", c.expiresAt,
	)

	return c.token, nil
}

// InvalidateToken ล้าง token (เรียกเมื่อได้ 401)
func (c *AuthClient) InvalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
	c.expiresAt = time.Time{}
}
