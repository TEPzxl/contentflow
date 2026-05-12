package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/auth"
	authmocks "github.com/tepzxl/contentflow/internal/module/auth/mocks"
	"github.com/tepzxl/contentflow/internal/module/user"
	usermocks "github.com/tepzxl/contentflow/internal/module/user/mocks"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name string
		req  auth.RegisterRequest
		mock func(
			ctx context.Context,
			userRepo *usermocks.MockRepository,
			refreshRepo *authmocks.MockRefreshTokenRepository,
			tokenManager *authmocks.MockTokenManager,
		)
		wantErr error
	}{
		{
			name: "success",
			req: auth.RegisterRequest{
				Email:       "  TEP@example.com ",
				Password:    "12345678",
				DisplayName: "tep",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				userRepo.EXPECT().
					FindByEmail(ctx, "tep@example.com").
					Return(nil, user.ErrUserNotFound)

				userRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&user.User{})).
					DoAndReturn(func(_ context.Context, u *user.User) error {
						if u.Email != "tep@example.com" {
							t.Fatalf("unexpected email: %s", u.Email)
						}

						if u.DisplayName != "tep" {
							t.Fatalf("unexpected display name: %s", u.DisplayName)
						}

						if u.PasswordHash == "" {
							t.Fatal("password hash should not be empty")
						}

						if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("12345678")); err != nil {
							t.Fatalf("password hash does not match password: %v", err)
						}

						u.ID = 1
						return nil
					})
			},
			wantErr: nil,
		},
		{
			name: "email already exists from pre check",
			req: auth.RegisterRequest{
				Email:       "tep@example.com",
				Password:    "12345678",
				DisplayName: "tep",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				userRepo.EXPECT().
					FindByEmail(ctx, "tep@example.com").
					Return(&user.User{
						ID:    1,
						Email: "tep@example.com",
					}, nil)
			},
			wantErr: auth.ErrEmailAlreadyExists,
		},
		{
			name: "weak password",
			req: auth.RegisterRequest{
				Email:       "tep@example.com",
				Password:    "123",
				DisplayName: "tep",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
			},
			wantErr: auth.ErrWeakPassword,
		},
		{
			name: "invalid email",
			req: auth.RegisterRequest{
				Email:       "bad-email",
				Password:    "12345678",
				DisplayName: "tep",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
			},
			wantErr: auth.ErrInvalidEmail,
		},
		{
			name: "email already exists from repository create",
			req: auth.RegisterRequest{
				Email:       "tep@example.com",
				Password:    "12345678",
				DisplayName: "tep",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				userRepo.EXPECT().
					FindByEmail(ctx, "tep@example.com").
					Return(nil, user.ErrUserNotFound)

				userRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&user.User{})).
					Return(user.ErrEmailAlreadyExists)
			},
			wantErr: auth.ErrEmailAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			userRepo := usermocks.NewMockRepository(ctrl)
			refreshRepo := authmocks.NewMockRefreshTokenRepository(ctrl)
			tokenManager := authmocks.NewMockTokenManager(ctrl)

			tt.mock(ctx, userRepo, refreshRepo, tokenManager)

			svc := auth.NewService(userRepo, refreshRepo, tokenManager)

			resp, err := svc.Register(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Register() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Register() unexpected error = %v", err)
			}

			if resp == nil {
				t.Fatal("Register() response is nil")
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {

	fixedNow := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	accessExpiresAt := fixedNow.Add(15 * time.Minute)

	tests := []struct {
		name string
		req  auth.LoginRequest
		mock func(
			t *testing.T,
			ctx context.Context,
			userRepo *usermocks.MockRepository,
			refreshRepo *authmocks.MockRefreshTokenRepository,
			tokenManager *authmocks.MockTokenManager,
		)
		wantErr error
	}{
		{
			name: "success",
			req: auth.LoginRequest{
				Email:    "tep@example.com",
				Password: "12345678",
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				passwordHash := mustPasswordHash(t, "12345678")

				userRepo.EXPECT().
					FindByEmail(ctx, "tep@example.com").
					Return(&user.User{
						ID:           1,
						Email:        "tep@example.com",
						DisplayName:  "tep",
						PasswordHash: passwordHash,
					}, nil)

				tokenManager.EXPECT().
					GenerateAccessToken(int64(1), "tep@example.com").
					Return("access-token", accessExpiresAt, nil)

				tokenManager.EXPECT().
					GenerateRefreshToken().
					Return("refresh-token", "refresh-token-hash", nil)

				tokenManager.EXPECT().
					RefreshTokenTTL().
					Return(7 * 24 * time.Hour)

				refreshRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&auth.RefreshToken{})).
					DoAndReturn(func(_ context.Context, token *auth.RefreshToken) error {
						if token.UserID != 1 {
							t.Fatalf("unexpected user id: %d", token.UserID)
						}

						if token.TokenHash != "refresh-token-hash" {
							t.Fatalf("unexpected token hash: %s", token.TokenHash)
						}

						wantExpiresAt := fixedNow.Add(7 * 24 * time.Hour)
						if !token.ExpiresAt.Equal(wantExpiresAt) {
							t.Fatalf("unexpected expires_at: got %v want %v", token.ExpiresAt, wantExpiresAt)
						}

						return nil
					})
			},
			wantErr: nil,
		},
		{
			name: "user not found",
			req: auth.LoginRequest{
				Email:    "missing@example.com",
				Password: "12345678",
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				userRepo.EXPECT().
					FindByEmail(ctx, "missing@example.com").
					Return(nil, user.ErrUserNotFound)
			},
			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name: "wrong password",
			req: auth.LoginRequest{
				Email:    "tep@example.com",
				Password: "wrong-password",
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				passwordHash := mustPasswordHash(t, "correct-password")

				userRepo.EXPECT().
					FindByEmail(ctx, "tep@example.com").
					Return(&user.User{
						ID:           1,
						Email:        "tep@example.com",
						PasswordHash: passwordHash,
					}, nil)
			},
			wantErr: auth.ErrInvalidCredentials,
		},
		{
			name: "invalid email returns invalid credentials",
			req: auth.LoginRequest{
				Email:    "bad-email",
				Password: "12345678",
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
			},
			wantErr: auth.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			userRepo := usermocks.NewMockRepository(ctrl)
			refreshRepo := authmocks.NewMockRefreshTokenRepository(ctrl)
			tokenManager := authmocks.NewMockTokenManager(ctrl)

			tt.mock(t, ctx, userRepo, refreshRepo, tokenManager)

			svc := auth.NewService(
				userRepo,
				refreshRepo,
				tokenManager,
				auth.WithNow(func() time.Time {
					return fixedNow
				}),
			)

			resp, err := svc.Login(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Login() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Login() unexpected error = %v", err)
			}

			if resp == nil {
				t.Fatal("Login() response is nil")
			}

			if resp.AccessToken == "" {
				t.Fatal("Login() access token should not be empty")
			}

			if resp.RefreshToken == "" {
				t.Fatal("Login() refresh token should not be empty")
			}
		})
	}
}

func TestAuthService_Refresh(t *testing.T) {
	fixedNow := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	accessExpiresAt := fixedNow.Add(15 * time.Minute)

	tests := []struct {
		name string
		req  auth.RefreshRequest
		mock func(
			ctx context.Context,
			userRepo *usermocks.MockRepository,
			refreshRepo *authmocks.MockRefreshTokenRepository,
			tokenManager *authmocks.MockTokenManager,
		)
		wantErr error
	}{
		{
			name: "success",
			req: auth.RefreshRequest{
				RefreshToken: "old-refresh-token",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				oldHash := auth.HashRefreshToken("old-refresh-token")

				refreshRepo.EXPECT().
					FindValidByHash(ctx, oldHash, fixedNow).
					Return(&auth.RefreshToken{
						ID:        10,
						UserID:    1,
						TokenHash: oldHash,
						ExpiresAt: fixedNow.Add(7 * 24 * time.Hour),
					}, nil)

				userRepo.EXPECT().
					FindByID(ctx, int64(1)).
					Return(&user.User{
						ID:          1,
						Email:       "tep@example.com",
						DisplayName: "tep",
					}, nil)

				refreshRepo.EXPECT().
					RevokeByHash(ctx, oldHash, fixedNow).
					Return(nil)

				tokenManager.EXPECT().
					GenerateAccessToken(int64(1), "tep@example.com").
					Return("new-access-token", accessExpiresAt, nil)

				tokenManager.EXPECT().
					GenerateRefreshToken().
					Return("new-refresh-token", "new-refresh-token-hash", nil)

				tokenManager.EXPECT().
					RefreshTokenTTL().
					Return(7 * 24 * time.Hour)

				refreshRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&auth.RefreshToken{})).
					Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "empty token",
			req: auth.RefreshRequest{
				RefreshToken: "",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
			},
			wantErr: auth.ErrInvalidRefreshToken,
		},
		{
			name: "token not found",
			req: auth.RefreshRequest{
				RefreshToken: "missing-refresh-token",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				hash := auth.HashRefreshToken("missing-refresh-token")

				refreshRepo.EXPECT().
					FindValidByHash(ctx, hash, fixedNow).
					Return(nil, auth.ErrRefreshTokenNotFound)
			},
			wantErr: auth.ErrInvalidRefreshToken,
		},
		{
			name: "user not found",
			req: auth.RefreshRequest{
				RefreshToken: "old-refresh-token",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				oldHash := auth.HashRefreshToken("old-refresh-token")

				refreshRepo.EXPECT().
					FindValidByHash(ctx, oldHash, fixedNow).
					Return(&auth.RefreshToken{
						ID:        10,
						UserID:    1,
						TokenHash: oldHash,
						ExpiresAt: fixedNow.Add(7 * 24 * time.Hour),
					}, nil)

				userRepo.EXPECT().
					FindByID(ctx, int64(1)).
					Return(nil, user.ErrUserNotFound)
			},
			wantErr: auth.ErrInvalidRefreshToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			userRepo := usermocks.NewMockRepository(ctrl)
			refreshRepo := authmocks.NewMockRefreshTokenRepository(ctrl)
			tokenManager := authmocks.NewMockTokenManager(ctrl)

			tt.mock(ctx, userRepo, refreshRepo, tokenManager)

			svc := auth.NewService(
				userRepo,
				refreshRepo,
				tokenManager,
				auth.WithNow(func() time.Time {
					return fixedNow
				}),
			)

			resp, err := svc.Refresh(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Refresh() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Refresh() unexpected error = %v", err)
			}

			if resp == nil {
				t.Fatal("Refresh() response is nil")
			}

			if resp.AccessToken == "" {
				t.Fatal("Refresh() access token should not be empty")
			}

			if resp.RefreshToken == "" {
				t.Fatal("Refresh() refresh token should not be empty")
			}
		})
	}
}

func TestAuthService_Logout(t *testing.T) {
	fixedNow := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		req  auth.LogoutRequest
		mock func(
			ctx context.Context,
			userRepo *usermocks.MockRepository,
			refreshRepo *authmocks.MockRefreshTokenRepository,
			tokenManager *authmocks.MockTokenManager,
		)
		wantErr error
	}{
		{
			name: "success",
			req: auth.LogoutRequest{
				RefreshToken: "refresh-token",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				hash := auth.HashRefreshToken("refresh-token")

				refreshRepo.EXPECT().
					RevokeByHash(ctx, hash, fixedNow).
					Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "empty token",
			req: auth.LogoutRequest{
				RefreshToken: "",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
			},
			wantErr: auth.ErrInvalidRefreshToken,
		},
		{
			name: "token not found",
			req: auth.LogoutRequest{
				RefreshToken: "missing-token",
			},
			mock: func(
				ctx context.Context,
				userRepo *usermocks.MockRepository,
				refreshRepo *authmocks.MockRefreshTokenRepository,
				tokenManager *authmocks.MockTokenManager,
			) {
				hash := auth.HashRefreshToken("missing-token")

				refreshRepo.EXPECT().
					RevokeByHash(ctx, hash, fixedNow).
					Return(auth.ErrRefreshTokenNotFound)
			},
			wantErr: auth.ErrInvalidRefreshToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			userRepo := usermocks.NewMockRepository(ctrl)
			refreshRepo := authmocks.NewMockRefreshTokenRepository(ctrl)
			tokenManager := authmocks.NewMockTokenManager(ctrl)

			tt.mock(ctx, userRepo, refreshRepo, tokenManager)

			svc := auth.NewService(
				userRepo,
				refreshRepo,
				tokenManager,
				auth.WithNow(func() time.Time {
					return fixedNow
				}),
			)

			err := svc.Logout(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Logout() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Logout() unexpected error = %v", err)
			}
		})
	}
}
func mustPasswordHash(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	return string(hash)
}
