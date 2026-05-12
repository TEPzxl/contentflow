package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/middleware"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
)

func TestAuthRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		authHeader       string
		parseAccessToken middleware.ParseAccessTokenFunc
		wantStatus       int
		wantCalledNext   bool
		wantUserID       int64
	}{
		{
			name:       "missing authorization header",
			authHeader: "",
			parseAccessToken: func(token string) (int64, error) {
				t.Fatalf("parseAccessToken should not be called")
				return 0, nil
			},
			wantStatus:     http.StatusUnauthorized,
			wantCalledNext: false,
		},
		{
			name:       "invalid authorization format",
			authHeader: "invalid-token",
			parseAccessToken: func(token string) (int64, error) {
				t.Fatalf("parseAccessToken should not be called")
				return 0, nil
			},
			wantStatus:     http.StatusUnauthorized,
			wantCalledNext: false,
		},
		{
			name:       "unsupported auth scheme",
			authHeader: "Basic abc",
			parseAccessToken: func(token string) (int64, error) {
				t.Fatalf("parseAccessToken should not be called")
				return 0, nil
			},
			wantStatus:     http.StatusUnauthorized,
			wantCalledNext: false,
		},
		{
			name:       "invalid bearer token",
			authHeader: "Bearer bad-token",
			parseAccessToken: func(token string) (int64, error) {
				if token != "bad-token" {
					t.Fatalf("token = %s, want bad-token", token)
				}

				return 0, errors.New("invalid token")
			},
			wantStatus:     http.StatusUnauthorized,
			wantCalledNext: false,
		},
		{
			name:       "invalid user id from token",
			authHeader: "Bearer valid-token",
			parseAccessToken: func(token string) (int64, error) {
				if token != "valid-token" {
					t.Fatalf("token = %s, want valid-token", token)
				}

				return 0, nil
			},
			wantStatus:     http.StatusUnauthorized,
			wantCalledNext: false,
		},
		{
			name:       "success",
			authHeader: "Bearer valid-token",
			parseAccessToken: func(token string) (int64, error) {
				if token != "valid-token" {
					t.Fatalf("token = %s, want valid-token", token)
				}

				return 123, nil
			},
			wantStatus:     http.StatusOK,
			wantCalledNext: true,
			wantUserID:     123,
		},
		{
			name:       "bearer scheme is case insensitive",
			authHeader: "bearer valid-token",
			parseAccessToken: func(token string) (int64, error) {
				if token != "valid-token" {
					t.Fatalf("token = %s, want valid-token", token)
				}

				return 456, nil
			},
			wantStatus:     http.StatusOK,
			wantCalledNext: true,
			wantUserID:     456,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledNext := false

			router := gin.New()
			router.Use(middleware.AuthRequired(tt.parseAccessToken))
			router.GET("/protected", func(c *gin.Context) {
				calledNext = true

				userID, ok := requestctx.UserID(c.Request.Context())
				if tt.wantCalledNext {
					if !ok {
						t.Fatal("user id missing from request context")
					}

					if userID != tt.wantUserID {
						t.Fatalf("userID = %d, want %d", userID, tt.wantUserID)
					}
				}

				c.JSON(http.StatusOK, gin.H{
					"user_id": userID,
				})
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.wantStatus, w.Body.String())
			}

			if calledNext != tt.wantCalledNext {
				t.Fatalf("calledNext = %v, want %v", calledNext, tt.wantCalledNext)
			}
		})
	}
}
