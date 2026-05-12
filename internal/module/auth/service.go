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
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidEmail        = errors.New("invalid email")
	ErrWeakPassword        = errors.New("weak password")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrUserNotFound        = errors.New("user not found")
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error)
	Logout(ctx context.Context, req LogoutRequest) error
	Me(ctx context.Context, userID int64) (*MeResponse, error)
}

var _ Service = (*AuthService)(nil)

type Option func(*AuthService)

func WithNow(now func() time.Time) Option {
	return func(s *AuthService) {
		s.now = now
	}
}

type AuthService struct {
	userRepo         user.Repository
	refreshTokenRepo RefreshTokenRepository
	tokenManager     TokenManager
	now              func() time.Time
}

func NewService(userRepo user.Repository, refreshTokenRepo RefreshTokenRepository, tokenManager TokenManager, opts ...Option) *AuthService {
	svc := &AuthService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		tokenManager:     tokenManager,
		now:              func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
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
		if errors.Is(err, user.ErrEmailAlreadyExists) {
			return nil, ErrEmailAlreadyExists
		}
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

func (s *AuthService) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	now := s.now()
	tokenHash := HashRefreshToken(refreshToken)

	tokenRecord, err := s.refreshTokenRepo.FindValidByHash(ctx, tokenHash, now)
	if errors.Is(err, ErrRefreshTokenNotFound) {
		return nil, ErrInvalidRefreshToken
	}

	if err != nil {
		return nil, fmt.Errorf("find refresh token: %w", err)
	}

	u, err := s.userRepo.FindByID(ctx, tokenRecord.UserID)
	if errors.Is(err, user.ErrUserNotFound) {
		return nil, ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	if err := s.refreshTokenRepo.RevokeByHash(ctx, tokenHash, now); err != nil {
		return nil, fmt.Errorf("revoke refresh token: %w", err)
	}

	accessToken, expiresAt, err := s.tokenManager.GenerateAccessToken(u.ID, u.Email)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	newRefreshToken, newRefreshTokenHash, err := s.tokenManager.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	newTokenRecord := &RefreshToken{
		UserID:    u.ID,
		TokenHash: newRefreshTokenHash,
		ExpiresAt: now.Add(s.tokenManager.RefreshTokenTTL()),
	}
	if err := s.refreshTokenRepo.Create(ctx, newTokenRecord); err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(expiresAt).Seconds()),
		User:         AuthUser{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName},
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req LogoutRequest) error {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		return ErrInvalidRefreshToken
	}

	tokenHash := HashRefreshToken(refreshToken)
	if err := s.refreshTokenRepo.RevokeByHash(ctx, tokenHash, s.now()); err != nil {
		if errors.Is(err, ErrRefreshTokenNotFound) {
			return ErrInvalidRefreshToken
		}
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

func (s *AuthService) Me(ctx context.Context, userID int64) (*MeResponse, error) {
	u, err := s.userRepo.FindByID(ctx, userID)
	if errors.Is(err, user.ErrUserNotFound) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find current user: %w", err)
	}

	return &MeResponse{
		User: AuthUser{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName},
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
