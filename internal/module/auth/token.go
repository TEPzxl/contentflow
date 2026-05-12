package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager interface {
	GenerateAccessToken(userID int64, email string) (string, time.Time, error)
	GenerateRefreshToken() (string, string, error)
	RefreshTokenTTL() time.Duration
}

type JWTTokenManager struct {
	secret          []byte
	issuer          string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

type JWTTokenManagerConfig struct {
	Secret          string
	Issuer          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func NewJWTTokenManager(cfg JWTTokenManagerConfig) (*JWTTokenManager, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}
	if cfg.AccessTokenTTL <= 0 {
		return nil, fmt.Errorf("access token ttl must be positive")
	}
	if cfg.RefreshTokenTTL <= 0 {
		return nil, fmt.Errorf("refresh token ttl must be positive")
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "contentflow"
	}

	return &JWTTokenManager{
		secret:          []byte(cfg.Secret),
		issuer:          cfg.Issuer,
		accessTokenTTL:  cfg.AccessTokenTTL,
		refreshTokenTTL: cfg.RefreshTokenTTL,
	}, nil
}

type AccessTokenClaims struct {
	UserID int64  `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func (m *JWTTokenManager) GenerateAccessToken(userID int64, email string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTokenTTL)

	claims := AccessTokenClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    m.issuer,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (m *JWTTokenManager) GenerateRefreshToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := HashRefreshToken(token)

	return token, tokenHash, nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *JWTTokenManager) RefreshTokenTTL() time.Duration {
	return m.refreshTokenTTL
}
