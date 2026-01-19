package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v2"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/config"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type AuthHandler struct {
	userService  services.UserService
	googleConfig config.GoogleOAuthConfig
}

func NewAuthHandler(userService services.UserService, googleConfig config.GoogleOAuthConfig) *AuthHandler {
	return &AuthHandler{
		userService:  userService,
		googleConfig: googleConfig,
	}
}

// GoogleLogin redirect ไปยัง Google OAuth consent screen
func (h *AuthHandler) GoogleLogin(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// สร้าง state สำหรับ CSRF protection
	state := utils.GenerateRandomString(32)

	// เก็บ state ใน session/cookie (ในกรณีจริงควรเก็บใน Redis)
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HTTPOnly: true,
		Secure:   false, // ตั้งเป็น true ใน production
		SameSite: "Lax",
		MaxAge:   300, // 5 นาที
	})

	oauthURL := h.userService.GetGoogleOAuthURL(state)

	logger.InfoContext(ctx, "Redirecting to Google OAuth", "state", state)

	return c.Redirect(oauthURL, fiber.StatusTemporaryRedirect)
}

// GoogleCallback รับ callback จาก Google OAuth
func (h *AuthHandler) GoogleCallback(c *fiber.Ctx) error {
	ctx := c.UserContext()

	code := c.Query("code")
	state := c.Query("state")
	errorParam := c.Query("error")

	frontendURL := h.googleConfig.FrontendURL

	if errorParam != "" {
		logger.WarnContext(ctx, "Google OAuth error", "error", errorParam)
		return c.Redirect(frontendURL+"/login?error="+url.QueryEscape(errorParam), fiber.StatusTemporaryRedirect)
	}

	if code == "" {
		logger.WarnContext(ctx, "No code in Google callback")
		return c.Redirect(frontendURL+"/login?error=no_code", fiber.StatusTemporaryRedirect)
	}

	// ตรวจสอบ state (CSRF protection)
	savedState := c.Cookies("oauth_state")
	if savedState == "" || savedState != state {
		logger.WarnContext(ctx, "Invalid OAuth state", "expected", savedState, "got", state)
		return c.Redirect(frontendURL+"/login?error=invalid_state", fiber.StatusTemporaryRedirect)
	}

	// ลบ state cookie
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    "",
		MaxAge:   -1,
		HTTPOnly: true,
	})

	// แลก code เป็น token
	tokenResp, err := h.exchangeCodeForToken(code)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to exchange code for token", "error", err)
		return c.Redirect(frontendURL+"/login?error=token_exchange_failed", fiber.StatusTemporaryRedirect)
	}

	// ดึงข้อมูล user จาก Google
	googleUser, err := h.getGoogleUserInfo(tokenResp.AccessToken)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get Google user info", "error", err)
		return c.Redirect(frontendURL+"/login?error=user_info_failed", fiber.StatusTemporaryRedirect)
	}

	logger.InfoContext(ctx, "Got Google user info", "google_id", googleUser.ID, "email", googleUser.Email)

	// Login หรือ Register
	token, user, err := h.userService.LoginOrRegisterWithGoogle(ctx, googleUser)
	if err != nil {
		logger.WarnContext(ctx, "Google login/register failed", "error", err)
		return c.Redirect(frontendURL+"/login?error="+url.QueryEscape(err.Error()), fiber.StatusTemporaryRedirect)
	}

	logger.InfoContext(ctx, "Google auth successful", "user_id", user.ID, "email", user.Email)

	// Redirect กลับไปยัง frontend พร้อม token
	redirectURL := frontendURL + "/auth/google/callback?token=" + url.QueryEscape(token) + "&user_id=" + user.ID.String()

	return c.Redirect(redirectURL, fiber.StatusTemporaryRedirect)
}

// exchangeCodeForToken แลก authorization code เป็น access token
func (h *AuthHandler) exchangeCodeForToken(code string) (*dto.GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", h.googleConfig.ClientID)
	data.Set("client_secret", h.googleConfig.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", h.googleConfig.RedirectURL)

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp dto.GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// getGoogleUserInfo ดึงข้อมูล user จาก Google API
func (h *AuthHandler) getGoogleUserInfo(accessToken string) (*dto.GoogleUserInfo, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo dto.GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}
