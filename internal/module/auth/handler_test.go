package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/module/auth"
	authmocks "github.com/tepzxl/contentflow/internal/module/auth/mocks"
	"go.uber.org/mock/gomock"
)

func newAuthTestRouter(service auth.Service, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()

	h := auth.NewHandler(service)

	api := r.Group("/api/v1")

	authRequired := func(c *gin.Context) {
		ctx := requestctx.WithUserID(c.Request.Context(), authUserID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}

	auth.RegisterRoutes(api, h, authRequired)

	return r
}

func performJSONRequest(router http.Handler, method, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w
}

func decodeJSONBody(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v, body=%s", err, body.String())
	}

	return got
}

func TestAuthHandler_Register(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mock       func(ctx context.Context, service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			body: `{
				"email": "tep@example.com",
				"password": "12345678",
				"display_name": "tep"
			}`,
			mock: func(ctx context.Context, service *authmocks.MockService) {
				service.EXPECT().
					Register(gomock.Any(), auth.RegisterRequest{
						Email:       "tep@example.com",
						Password:    "12345678",
						DisplayName: "tep",
					}).
					Return(&auth.RegisterResponse{
						User: auth.AuthUser{
							ID:          1,
							Email:       "tep@example.com",
							DisplayName: "tep",
						},
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid json body",
			body: `{`,
			mock: func(ctx context.Context, service *authmocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "missing email",
			body: `{
				"password": "12345678",
				"display_name": "tep"
			}`,
			mock: func(ctx context.Context, service *authmocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "email already exists",
			body: `{
				"email": "tep@example.com",
				"password": "12345678",
				"display_name": "tep"
			}`,
			mock: func(ctx context.Context, service *authmocks.MockService) {
				service.EXPECT().
					Register(gomock.Any(), auth.RegisterRequest{
						Email:       "tep@example.com",
						Password:    "12345678",
						DisplayName: "tep",
					}).
					Return(nil, auth.ErrEmailAlreadyExists)
			},
			wantStatus: http.StatusConflict,
			wantCode:   "email_already_exists",
		},
		{
			name: "weak password",
			body: `{
				"email": "tep@example.com",
				"password": "12345678",
				"display_name": "tep"
			}`,
			mock: func(ctx context.Context, service *authmocks.MockService) {
				service.EXPECT().
					Register(gomock.Any(), auth.RegisterRequest{
						Email:       "tep@example.com",
						Password:    "12345678",
						DisplayName: "tep",
					}).
					Return(nil, auth.ErrWeakPassword)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "weak_password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := authmocks.NewMockService(ctrl)
			tt.mock(context.Background(), service)

			router := newAuthTestRouter(service, 1)

			w := performJSONRequest(router, http.MethodPost, "/api/v1/auth/register", tt.body)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}
func TestAuthHandler_Login(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mock       func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			body: `{
				"email": "tep@example.com",
				"password": "12345678"
			}`,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Login(gomock.Any(), auth.LoginRequest{
						Email:    "tep@example.com",
						Password: "12345678",
					}).
					Return(&auth.LoginResponse{
						User: auth.AuthUser{
							ID:          1,
							Email:       "tep@example.com",
							DisplayName: "tep",
						},
						AccessToken:  "access-token",
						RefreshToken: "refresh-token",
						TokenType:    "Bearer",
						ExpiresIn:    900,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid request",
			body: `{
				"email": "bad-email",
				"password": "12345678"
			}`,
			mock: func(service *authmocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "invalid credentials",
			body: `{
				"email": "tep@example.com",
				"password": "wrong-password"
			}`,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Login(gomock.Any(), auth.LoginRequest{
						Email:    "tep@example.com",
						Password: "wrong-password",
					}).
					Return(nil, auth.ErrInvalidCredentials)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := authmocks.NewMockService(ctrl)
			tt.mock(service)

			router := newAuthTestRouter(service, 1)

			w := performJSONRequest(router, http.MethodPost, "/api/v1/auth/login", tt.body)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}

func TestAuthHandler_LoginSetsHttpOnlyRefreshCookie(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := authmocks.NewMockService(ctrl)
	service.EXPECT().
		Login(gomock.Any(), auth.LoginRequest{
			Email:    "tep@example.com",
			Password: "12345678",
		}).
		Return(&auth.LoginResponse{
			User:         auth.AuthUser{ID: 1, Email: "tep@example.com", DisplayName: "tep"},
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			TokenType:    "Bearer",
			ExpiresIn:    900,
		}, nil)

	router := newAuthTestRouter(service, 1)
	w := performJSONRequest(router, http.MethodPost, "/api/v1/auth/login", `{"email":"tep@example.com","password":"12345678"}`)

	assertStatus(t, w, http.StatusOK)
	cookie := findCookie(w.Result().Cookies(), "contentflow_refresh_token")
	if cookie == nil {
		t.Fatal("refresh cookie missing")
	}
	if !cookie.HttpOnly {
		t.Fatal("refresh cookie must be HttpOnly")
	}
	if cookie.Value != "refresh-token" {
		t.Fatalf("refresh cookie value = %q, want refresh-token", cookie.Value)
	}
	assertNoRefreshTokenInData(t, w)
}

func TestAuthHandler_Refresh(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mock       func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			body: `{
				"refresh_token": "old-refresh-token"
			}`,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Refresh(gomock.Any(), auth.RefreshRequest{
						RefreshToken: "old-refresh-token",
					}).
					Return(&auth.RefreshResponse{
						User: auth.AuthUser{
							ID:          1,
							Email:       "tep@example.com",
							DisplayName: "tep",
						},
						AccessToken:  "new-access-token",
						RefreshToken: "new-refresh-token",
						TokenType:    "Bearer",
						ExpiresIn:    900,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing refresh token",
			body: `{}`,
			mock: func(service *authmocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "invalid refresh token",
			body: `{
				"refresh_token": "bad-token"
			}`,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Refresh(gomock.Any(), auth.RefreshRequest{
						RefreshToken: "bad-token",
					}).
					Return(nil, auth.ErrInvalidRefreshToken)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_refresh_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := authmocks.NewMockService(ctrl)
			tt.mock(service)

			router := newAuthTestRouter(service, 1)

			w := performJSONRequest(router, http.MethodPost, "/api/v1/auth/refresh", tt.body)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}

func TestAuthHandler_RefreshUsesHttpOnlyCookie(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := authmocks.NewMockService(ctrl)
	service.EXPECT().
		Refresh(gomock.Any(), auth.RefreshRequest{RefreshToken: "old-refresh-token"}).
		Return(&auth.RefreshResponse{
			User:         auth.AuthUser{ID: 1, Email: "tep@example.com", DisplayName: "tep"},
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			TokenType:    "Bearer",
			ExpiresIn:    900,
		}, nil)

	router := newAuthTestRouter(service, 1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "contentflow_refresh_token", Value: "old-refresh-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusOK)
	cookie := findCookie(w.Result().Cookies(), "contentflow_refresh_token")
	if cookie == nil || cookie.Value != "new-refresh-token" || !cookie.HttpOnly {
		t.Fatalf("refresh cookie = %#v, want rotated HttpOnly cookie", cookie)
	}
	assertNoRefreshTokenInData(t, w)
}

func TestAuthHandler_Logout(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mock       func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			body: `{
				"refresh_token": "refresh-token"
			}`,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Logout(gomock.Any(), auth.LogoutRequest{
						RefreshToken: "refresh-token",
					}).
					Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing refresh token",
			body: `{}`,
			mock: func(service *authmocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "invalid refresh token",
			body: `{
				"refresh_token": "bad-token"
			}`,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Logout(gomock.Any(), auth.LogoutRequest{
						RefreshToken: "bad-token",
					}).
					Return(auth.ErrInvalidRefreshToken)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_refresh_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := authmocks.NewMockService(ctrl)
			tt.mock(service)

			router := newAuthTestRouter(service, 1)

			w := performJSONRequest(router, http.MethodPost, "/api/v1/auth/logout", tt.body)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}

func TestAuthHandler_LogoutUsesAndClearsHttpOnlyCookie(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := authmocks.NewMockService(ctrl)
	service.EXPECT().
		Logout(gomock.Any(), auth.LogoutRequest{RefreshToken: "refresh-token"}).
		Return(nil)

	router := newAuthTestRouter(service, 1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "contentflow_refresh_token", Value: "refresh-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusOK)
	cookie := findCookie(w.Result().Cookies(), "contentflow_refresh_token")
	if cookie == nil {
		t.Fatal("cleared refresh cookie missing")
	}
	if cookie.Value != "" || cookie.MaxAge >= 0 {
		t.Fatalf("cleared refresh cookie = %#v", cookie)
	}
}

func TestAuthHandler_Me(t *testing.T) {
	tests := []struct {
		name       string
		authUserID int64
		mock       func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "success",
			authUserID: 1,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Me(gomock.Any(), int64(1)).
					Return(&auth.MeResponse{
						User: auth.AuthUser{
							ID:          1,
							Email:       "tep@example.com",
							DisplayName: "tep",
						},
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "user not found",
			authUserID: 1,
			mock: func(service *authmocks.MockService) {
				service.EXPECT().
					Me(gomock.Any(), int64(1)).
					Return(nil, auth.ErrUserNotFound)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := authmocks.NewMockService(ctrl)
			tt.mock(service)

			router := newAuthTestRouter(service, tt.authUserID)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func assertNoRefreshTokenInData(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	got := decodeJSONBody(t, w.Body)
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("response data missing: %v", got)
	}
	if _, ok := data["refresh_token"]; ok {
		t.Fatalf("response leaks refresh_token: %v", data)
	}
}
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()

	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, wantStatus, w.Body.String())
	}
}

func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, wantCode string) {
	t.Helper()

	if wantCode == "" {
		return
	}

	got := decodeJSONBody(t, w.Body)

	errBody, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("response error body missing: %v", got)
	}

	if errBody["code"] != wantCode {
		t.Fatalf("error code = %v, want %s", errBody["code"], wantCode)
	}
}
