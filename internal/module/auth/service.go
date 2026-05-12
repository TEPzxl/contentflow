package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/tepzxl/contentflow/internal/module/user"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrWeakPassword       = errors.New("weak password")
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
}

type AuthService struct {
	userRepo         user.Repository
	refreshTokenRepo RefreshTokenRepository
	tokenManager     TokenManager
	now              func() time.Time
}

func NewAuthService(userRepo user.Repository, refreshTokenRepo RefreshTokenRepository, tokenManager TokenManager) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		tokenManager:     tokenManager,
		now:              func() time.Time { return time.Now().UTC() },
	}
}

func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}

	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}

	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	displayName := strings.TrimSpace(req.DisplayName)

	u := &user.User{
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
	}

	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &RegisterResponse{
		User: AuthUser{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
		},
	}, nil

}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	u, err := s.userRepo.FindByEmail(ctx, email)
	if errors.Is(err, user.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}

	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	if err := verifyPassword(u.PasswordHash, req.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, expiresAt, err := s.tokenManager.GenerateAccessToken(u.ID, email)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	refreshToken, refreshTokenHash, err := s.tokenManager.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := s.now()

	tokenRecord := &RefreshToken{
		UserID:    u.ID,
		TokenHash: refreshTokenHash,
		ExpiresAt: now.Add(s.tokenManager.RefreshTokenTTL()),
	}

	if err := s.refreshTokenRepo.Create(ctx, tokenRecord); err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &LoginResponse{
		User: AuthUser{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(expiresAt).Seconds()),
	}, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", ErrInvalidEmail
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func validatePassword(password string) error {
	if len(password) < 8 || password == "" {
		return ErrWeakPassword
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func verifyPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	return nil
}
