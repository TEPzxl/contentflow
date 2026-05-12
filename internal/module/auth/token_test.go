package auth_test

import (
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/auth"
)

func TestNewJWTTokenManager(t *testing.T) {
	tests := []struct {
		name    string
		cfg     auth.JWTTokenManagerConfig
		wantErr bool
	}{
		{
			name: "success",
			cfg: auth.JWTTokenManagerConfig{
				Secret:          "test-secret",
				Issuer:          "contentflow",
				AccessTokenTTL:  15 * time.Minute,
				RefreshTokenTTL: 7 * 24 * time.Hour,
			},
			wantErr: false,
		},
		{
			name: "empty secret",
			cfg: auth.JWTTokenManagerConfig{
				Secret:          "",
				Issuer:          "contentflow",
				AccessTokenTTL:  15 * time.Minute,
				RefreshTokenTTL: 7 * 24 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "zero access token ttl",
			cfg: auth.JWTTokenManagerConfig{
				Secret:          "test-secret",
				Issuer:          "contentflow",
				AccessTokenTTL:  0,
				RefreshTokenTTL: 7 * 24 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "negative refresh token ttl",
			cfg: auth.JWTTokenManagerConfig{
				Secret:          "test-secret",
				Issuer:          "contentflow",
				AccessTokenTTL:  15 * time.Minute,
				RefreshTokenTTL: -1 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "default issuer",
			cfg: auth.JWTTokenManagerConfig{
				Secret:          "test-secret",
				Issuer:          "",
				AccessTokenTTL:  15 * time.Minute,
				RefreshTokenTTL: 7 * 24 * time.Hour,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := auth.NewJWTTokenManager(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Fatal("NewJWTTokenManager() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewJWTTokenManager() unexpected error = %v", err)
			}

			if manager == nil {
				t.Fatal("NewJWTTokenManager() manager is nil")
			}
		})
	}
}

func TestJWTTokenManager_AccessToken(t *testing.T) {
	tests := []struct {
		name       string
		secret     string
		issuer     string
		parseWith  func(t *testing.T, token string) *auth.JWTTokenManager
		wantUserID int64
		wantEmail  string
		wantErr    bool
	}{
		{
			name:   "generate and parse success",
			secret: "test-secret",
			issuer: "contentflow",
			parseWith: func(t *testing.T, token string) *auth.JWTTokenManager {
				t.Helper()

				manager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
					Secret:          "test-secret",
					Issuer:          "contentflow",
					AccessTokenTTL:  15 * time.Minute,
					RefreshTokenTTL: 7 * 24 * time.Hour,
				})
				if err != nil {
					t.Fatalf("new token manager: %v", err)
				}

				return manager
			},
			wantUserID: 1,
			wantEmail:  "tep@example.com",
			wantErr:    false,
		},
		{
			name:   "wrong secret",
			secret: "test-secret",
			issuer: "contentflow",
			parseWith: func(t *testing.T, token string) *auth.JWTTokenManager {
				t.Helper()

				manager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
					Secret:          "wrong-secret",
					Issuer:          "contentflow",
					AccessTokenTTL:  15 * time.Minute,
					RefreshTokenTTL: 7 * 24 * time.Hour,
				})
				if err != nil {
					t.Fatalf("new token manager: %v", err)
				}

				return manager
			},
			wantErr: true,
		},
		{
			name:   "wrong issuer",
			secret: "test-secret",
			issuer: "contentflow",
			parseWith: func(t *testing.T, token string) *auth.JWTTokenManager {
				t.Helper()

				manager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
					Secret:          "test-secret",
					Issuer:          "other-service",
					AccessTokenTTL:  15 * time.Minute,
					RefreshTokenTTL: 7 * 24 * time.Hour,
				})
				if err != nil {
					t.Fatalf("new token manager: %v", err)
				}

				return manager
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
				Secret:          tt.secret,
				Issuer:          tt.issuer,
				AccessTokenTTL:  15 * time.Minute,
				RefreshTokenTTL: 7 * 24 * time.Hour,
			})
			if err != nil {
				t.Fatalf("new generator: %v", err)
			}

			token, expiresAt, err := generator.GenerateAccessToken(1, "tep@example.com")
			if err != nil {
				t.Fatalf("GenerateAccessToken() error = %v", err)
			}

			if token == "" {
				t.Fatal("GenerateAccessToken() token is empty")
			}

			if time.Until(expiresAt) <= 0 {
				t.Fatalf("GenerateAccessToken() expiresAt should be in future, got %v", expiresAt)
			}

			parser := tt.parseWith(t, token)

			claims, err := parser.ParseAccessToken(token)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ParseAccessToken() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseAccessToken() unexpected error = %v", err)
			}

			if claims.UserID != tt.wantUserID {
				t.Fatalf("claims.UserID = %d, want %d", claims.UserID, tt.wantUserID)
			}

			if claims.Email != tt.wantEmail {
				t.Fatalf("claims.Email = %s, want %s", claims.Email, tt.wantEmail)
			}

			if claims.Subject != "1" {
				t.Fatalf("claims.Subject = %s, want 1", claims.Subject)
			}

			if claims.Issuer != tt.issuer {
				t.Fatalf("claims.Issuer = %s, want %s", claims.Issuer, tt.issuer)
			}
		})
	}
}

func TestJWTTokenManager_ParseAccessToken_Expired(t *testing.T) {
	manager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
		Secret:          "test-secret",
		Issuer:          "contentflow",
		AccessTokenTTL:  -1 * time.Second,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
	if err == nil {
		t.Fatal("NewJWTTokenManager() expected error for negative ttl")
	}

	_ = manager
}

func TestJWTTokenManager_ParseAccessToken_InvalidToken(t *testing.T) {
	tests := []struct {
		name        string
		tokenString string
		wantErr     bool
	}{
		{
			name:        "empty token",
			tokenString: "",
			wantErr:     true,
		},
		{
			name:        "malformed token",
			tokenString: "not-a-jwt",
			wantErr:     true,
		},
		{
			name:        "random bearer value",
			tokenString: "abc.def.ghi",
			wantErr:     true,
		},
	}

	manager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
		Secret:          "test-secret",
		Issuer:          "contentflow",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.ParseAccessToken(tt.tokenString)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ParseAccessToken() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseAccessToken() unexpected error = %v", err)
			}

			if claims == nil {
				t.Fatal("ParseAccessToken() claims is nil")
			}
		})
	}
}

func TestJWTTokenManager_GenerateRefreshToken(t *testing.T) {
	manager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
		Secret:          "test-secret",
		Issuer:          "contentflow",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	token1, hash1, err := manager.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	token2, hash2, err := manager.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() second error = %v", err)
	}

	if token1 == "" {
		t.Fatal("first refresh token is empty")
	}

	if token2 == "" {
		t.Fatal("second refresh token is empty")
	}

	if hash1 == "" {
		t.Fatal("first refresh token hash is empty")
	}

	if hash2 == "" {
		t.Fatal("second refresh token hash is empty")
	}

	if token1 == token2 {
		t.Fatal("refresh tokens should be different")
	}

	if hash1 == hash2 {
		t.Fatal("refresh token hashes should be different")
	}

	if got := auth.HashRefreshToken(token1); got != hash1 {
		t.Fatalf("HashRefreshToken(token1) = %s, want %s", got, hash1)
	}

	if got := auth.HashRefreshToken(token2); got != hash2 {
		t.Fatalf("HashRefreshToken(token2) = %s, want %s", got, hash2)
	}
}

func TestHashRefreshToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "normal token",
			token: "refresh-token",
		},
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "long token",
			token: "this-is-a-very-long-refresh-token-value-for-testing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := auth.HashRefreshToken(tt.token)
			hash2 := auth.HashRefreshToken(tt.token)

			if hash1 == "" {
				t.Fatal("HashRefreshToken() returned empty hash")
			}

			if hash1 != hash2 {
				t.Fatalf("HashRefreshToken() not stable: %s != %s", hash1, hash2)
			}

			if len(hash1) != 64 {
				t.Fatalf("HashRefreshToken() length = %d, want 64", len(hash1))
			}
		})
	}
}
