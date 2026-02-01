package serviceimpl

import (
	"context"
	"errors"
	"fmt"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserServiceImpl struct {
	userRepo           repositories.UserRepository
	jwtSecret          string
	googleClientID     string
	googleClientSecret string
	googleRedirectURL  string
}

func NewUserService(userRepo repositories.UserRepository, jwtSecret, googleClientID, googleClientSecret, googleRedirectURL string) services.UserService {
	return &UserServiceImpl{
		userRepo:           userRepo,
		jwtSecret:          jwtSecret,
		googleClientID:     googleClientID,
		googleClientSecret: googleClientSecret,
		googleRedirectURL:  googleRedirectURL,
	}
}

func (s *UserServiceImpl) Register(ctx context.Context, req *dto.CreateUserRequest) (*models.User, error) {
	existingUser, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existingUser != nil {
		logger.WarnContext(ctx, "Email already exists", "email", req.Email)
		return nil, errors.New("email already exists")
	}

	existingUser, _ = s.userRepo.GetByUsername(ctx, req.Username)
	if existingUser != nil {
		logger.WarnContext(ctx, "Username already exists", "username", req.Username)
		return nil, errors.New("username already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to hash password", "error", err)
		return nil, err
	}

	user := &models.User{
		ID:        uuid.New(),
		Email:     req.Email,
		Username:  req.Username,
		Password:  string(hashedPassword),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create user in database", "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "User created successfully", "user_id", user.ID, "email", user.Email)

	return user, nil
}

func (s *UserServiceImpl) Login(ctx context.Context, req *dto.LoginRequest) (string, *models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		logger.WarnContext(ctx, "Login failed - email not found", "email", req.Email)
		return "", nil, errors.New("invalid email or password")
	}

	if !user.IsActive {
		logger.WarnContext(ctx, "Login failed - account disabled", "user_id", user.ID, "email", req.Email)
		return "", nil, errors.New("account is disabled")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		logger.WarnContext(ctx, "Login failed - invalid password", "user_id", user.ID, "email", req.Email)
		return "", nil, errors.New("invalid email or password")
	}

	token, err := s.GenerateJWT(user)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to generate JWT", "user_id", user.ID, "error", err)
		return "", nil, err
	}

	logger.InfoContext(ctx, "User logged in successfully", "user_id", user.ID, "email", user.Email)

	return token, user, nil
}

func (s *UserServiceImpl) GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *UserServiceImpl) UpdateProfile(ctx context.Context, userID uuid.UUID, req *dto.UpdateUserRequest) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.WarnContext(ctx, "User not found for profile update", "user_id", userID)
		return nil, errors.New("user not found")
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	user.UpdatedAt = time.Now()

	err = s.userRepo.Update(ctx, userID, user)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update user profile", "user_id", userID, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "User profile updated", "user_id", userID)

	return user, nil
}

func (s *UserServiceImpl) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	err := s.userRepo.Delete(ctx, userID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete user", "user_id", userID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "User deleted", "user_id", userID)
	return nil
}

func (s *UserServiceImpl) ListUsers(ctx context.Context, offset, limit int) ([]*models.User, int64, error) {
	users, err := s.userRepo.List(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list users", "offset", offset, "limit", limit, "error", err)
		return nil, 0, err
	}

	count, err := s.userRepo.Count(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count users", "error", err)
		return nil, 0, err
	}

	return users, count, nil
}

func (s *UserServiceImpl) GenerateJWT(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID.String(),
		"username": user.Username,
		"email":    user.Email,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Hour * 24 * 7).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *UserServiceImpl) ValidateJWT(tokenString string) (*models.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			return nil, errors.New("invalid token claims")
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, errors.New("invalid user ID in token")
		}

		user, err := s.userRepo.GetByID(context.Background(), userID)
		if err != nil {
			return nil, errors.New("user not found")
		}

		return user, nil
	}

	return nil, errors.New("invalid token")
}

// SetPassword ตั้ง password สำหรับ Google users ที่ยังไม่มี password
func (s *UserServiceImpl) SetPassword(ctx context.Context, userID uuid.UUID, req *dto.SetPasswordRequest) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.WarnContext(ctx, "User not found for set password", "user_id", userID)
		return errors.New("user not found")
	}

	// ตรวจสอบว่าเป็น Google user และยังไม่มี password
	if !user.IsGoogleUser() {
		logger.WarnContext(ctx, "Set password failed - not a Google user", "user_id", userID)
		return errors.New("only Google users can set password without current password")
	}

	if user.Password != "" {
		logger.WarnContext(ctx, "Set password failed - user already has password", "user_id", userID)
		return errors.New("user already has a password, use change password instead")
	}

	// Hash password ใหม่
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to hash password", "user_id", userID, "error", err)
		return errors.New("failed to set password")
	}

	user.Password = string(hashedPassword)
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, userID, user); err != nil {
		logger.ErrorContext(ctx, "Failed to update user password", "user_id", userID, "error", err)
		return errors.New("failed to set password")
	}

	logger.InfoContext(ctx, "Password set successfully for Google user", "user_id", userID)
	return nil
}

// GetGoogleOAuthURL สร้าง URL สำหรับ redirect ไป Google OAuth
func (s *UserServiceImpl) GetGoogleOAuthURL(state string) string {
	params := url.Values{}
	params.Add("client_id", s.googleClientID)
	params.Add("redirect_uri", s.googleRedirectURL)
	params.Add("response_type", "code")
	params.Add("scope", "openid email profile")
	params.Add("access_type", "offline")
	params.Add("state", state)

	return fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?%s", params.Encode())
}

// LoginOrRegisterWithGoogle สร้างหรือ login user จาก Google
func (s *UserServiceImpl) LoginOrRegisterWithGoogle(ctx context.Context, googleUser *dto.GoogleUserInfo) (string, *models.User, error) {
	// ค้นหา user จาก Google ID
	user, err := s.userRepo.GetByGoogleID(ctx, googleUser.ID)
	if err == nil && user != nil {
		// user มีอยู่แล้ว - ทำ login
		if !user.IsActive {
			logger.WarnContext(ctx, "Google login failed - account disabled", "google_id", googleUser.ID)
			return "", nil, errors.New("account is disabled")
		}

		token, err := s.GenerateJWT(user)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to generate JWT for Google user", "user_id", user.ID, "error", err)
			return "", nil, err
		}

		logger.InfoContext(ctx, "Google login successful", "user_id", user.ID, "email", user.Email)
		return token, user, nil
	}

	// ตรวจสอบว่ามี email ซ้ำไหม
	existingUser, _ := s.userRepo.GetByEmail(ctx, googleUser.Email)
	if existingUser != nil {
		// มี email แล้ว แต่ไม่ได้ผูกกับ Google - อัพเดท Google ID
		existingUser.GoogleID = &googleUser.ID
		if existingUser.Avatar == "" && googleUser.Picture != "" {
			existingUser.Avatar = googleUser.Picture
		}
		existingUser.UpdatedAt = time.Now()

		if err := s.userRepo.Update(ctx, existingUser.ID, existingUser); err != nil {
			logger.ErrorContext(ctx, "Failed to link Google account", "user_id", existingUser.ID, "error", err)
			return "", nil, err
		}

		token, err := s.GenerateJWT(existingUser)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to generate JWT", "user_id", existingUser.ID, "error", err)
			return "", nil, err
		}

		logger.InfoContext(ctx, "Google account linked", "user_id", existingUser.ID, "google_id", googleUser.ID)
		return token, existingUser, nil
	}

	// สร้าง user ใหม่
	username := generateUniqueUsername(googleUser.Email)

	user = &models.User{
		ID:        uuid.New(),
		GoogleID:  &googleUser.ID,
		Email:     googleUser.Email,
		Username:  username,
		Password:  "", // Google users ไม่มี password
		FirstName: googleUser.GivenName,
		LastName:  googleUser.FamilyName,
		Avatar:    googleUser.Picture,
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.ErrorContext(ctx, "Failed to create Google user", "google_id", googleUser.ID, "error", err)
		return "", nil, err
	}

	token, err := s.GenerateJWT(user)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to generate JWT for new Google user", "user_id", user.ID, "error", err)
		return "", nil, err
	}

	logger.InfoContext(ctx, "Google user registered", "user_id", user.ID, "email", user.Email, "google_id", googleUser.ID)
	return token, user, nil
}

// generateUniqueUsername สร้าง username จาก email
func generateUniqueUsername(email string) string {
	atIndex := 0
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}

	base := email[:atIndex]
	// เพิ่ม random suffix เพื่อป้องกัน duplicate
	return base + "_" + utils.GenerateRandomString(6)
}
