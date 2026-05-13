package source_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/module/source"
	sourcemocks "github.com/tepzxl/contentflow/internal/module/source/mocks"
	"go.uber.org/mock/gomock"
)

func TestSourceHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mock       func(service *sourcemocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success rss source",
			body: `{
				"name": "Go Blog",
				"type": "rss",
				"url": "https://go.dev/blog/feed.atom",
				"config": {}
			}`,
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					CreateSource(gomock.Any(), gomock.AssignableToTypeOf(source.CreateSourceRequest{})).
					DoAndReturn(func(_ context.Context, req source.CreateSourceRequest) (*source.CreateSourceResponse, error) {
						if req.UserID != 1 {
							t.Fatalf("UserID = %d, want 1", req.UserID)
						}
						if req.Name != "Go Blog" {
							t.Fatalf("Name = %s, want Go Blog", req.Name)
						}
						if req.Type != "rss" {
							t.Fatalf("Type = %s, want rss", req.Type)
						}
						if req.URL == nil || *req.URL != "https://go.dev/blog/feed.atom" {
							t.Fatalf("unexpected URL: %v", req.URL)
						}

						return &source.CreateSourceResponse{
							Source: sampleSourceDTO(),
						}, nil
					})
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid json",
			body: `{`,
			mock: func(service *sourcemocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid request",
		},
		{
			name: "missing name",
			body: `{
				"type": "rss",
				"url": "https://go.dev/blog/feed.atom"
			}`,
			mock: func(service *sourcemocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid request",
		},
		{
			name: "invalid source type",
			body: `{
				"name": "Bad Source",
				"type": "unknown",
				"config": {}
			}`,
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					CreateSource(gomock.Any(), gomock.AssignableToTypeOf(source.CreateSourceRequest{})).
					Return(nil, source.ErrInvalidSourceType)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_source_type",
		},
		{
			name: "source already exists",
			body: `{
				"name": "Go Blog",
				"type": "rss",
				"url": "https://go.dev/blog/feed.atom",
				"config": {}
			}`,
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					CreateSource(gomock.Any(), gomock.AssignableToTypeOf(source.CreateSourceRequest{})).
					Return(nil, source.ErrSourceAlreadyExists)
			},
			wantStatus: http.StatusConflict,
			wantCode:   "source_already_exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := sourcemocks.NewMockService(ctrl)
			tt.mock(service)

			router := newSourceTestRouter(service, 1)

			w := performJSONRequest(router, http.MethodPost, "/api/v1/sources", tt.body)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}

func TestSourceHandler_List(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		mock       func(service *sourcemocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			path: "/api/v1/sources?type=rss&limit=10&offset=0",
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					ListSources(gomock.Any(), source.ListSourcesRequest{
						UserID: 1,
						Type:   "rss",
						Limit:  10,
						Offset: 0,
					}).
					Return(&source.ListSourcesResponse{
						Sources: []source.SourceDTO{sampleSourceDTO()},
						Total:   1,
						Limit:   10,
						Offset:  0,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "default pagination",
			path: "/api/v1/sources",
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					ListSources(gomock.Any(), source.ListSourcesRequest{
						UserID: 1,
						Type:   "",
						Limit:  20,
						Offset: 0,
					}).
					Return(&source.ListSourcesResponse{
						Sources: []source.SourceDTO{},
						Total:   0,
						Limit:   20,
						Offset:  0,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid limit",
			path: "/api/v1/sources?limit=abc",
			mock: func(service *sourcemocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid request",
		},
		{
			name: "invalid offset",
			path: "/api/v1/sources?offset=abc",
			mock: func(service *sourcemocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid request",
		},
		{
			name: "invalid source type",
			path: "/api/v1/sources?type=unknown",
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					ListSources(gomock.Any(), source.ListSourcesRequest{
						UserID: 1,
						Type:   "unknown",
						Limit:  20,
						Offset: 0,
					}).
					Return(nil, source.ErrInvalidSourceType)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_source_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := sourcemocks.NewMockService(ctrl)
			tt.mock(service)

			router := newSourceTestRouter(service, 1)

			w := performRequest(router, http.MethodGet, tt.path)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}

func TestSourceHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		mock       func(service *sourcemocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			path: "/api/v1/sources/1",
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					GetSource(gomock.Any(), source.GetSourceRequest{
						UserID:   1,
						SourceID: 1,
					}).
					Return(&source.GetSourceResponse{
						Source: sampleSourceDTO(),
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid source id",
			path: "/api/v1/sources/abc",
			mock: func(service *sourcemocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid source id",
		},
		{
			name: "not found",
			path: "/api/v1/sources/999",
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					GetSource(gomock.Any(), source.GetSourceRequest{
						UserID:   1,
						SourceID: 999,
					}).
					Return(nil, source.ErrSourceNotAccessible)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "source_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := sourcemocks.NewMockService(ctrl)
			tt.mock(service)

			router := newSourceTestRouter(service, 1)

			w := performRequest(router, http.MethodGet, tt.path)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}
func TestSourceHandler_Update(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		mock       func(service *sourcemocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name: "success",
			path: "/api/v1/sources/1",
			body: `{
				"name": "New Go Blog",
				"is_active": false
			}`,
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					UpdateSource(gomock.Any(), gomock.AssignableToTypeOf(source.UpdateSourceRequest{})).
					DoAndReturn(func(_ context.Context, req source.UpdateSourceRequest) (*source.UpdateSourceResponse, error) {
						if req.UserID != 1 {
							t.Fatalf("UserID = %d, want 1", req.UserID)
						}
						if req.SourceID != 1 {
							t.Fatalf("SourceID = %d, want 1", req.SourceID)
						}
						if req.Name == nil || *req.Name != "New Go Blog" {
							t.Fatalf("unexpected Name: %v", req.Name)
						}
						if req.IsActive == nil || *req.IsActive != false {
							t.Fatalf("unexpected IsActive: %v", req.IsActive)
						}

						dto := sampleSourceDTO()
						dto.Name = "New Go Blog"
						dto.IsActive = false

						return &source.UpdateSourceResponse{
							Source: dto,
						}, nil
					})
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid json",
			path: "/api/v1/sources/1",
			body: `{`,
			mock: func(service *sourcemocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "invalid source id",
			path: "/api/v1/sources/abc",
			body: `{"name":"New Name"}`,
			mock: func(service *sourcemocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_source_id",
		},
		{
			name: "not found",
			path: "/api/v1/sources/999",
			body: `{"name":"New Name"}`,
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					UpdateSource(gomock.Any(), gomock.AssignableToTypeOf(source.UpdateSourceRequest{})).
					Return(nil, source.ErrSourceNotAccessible)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "source_not_found",
		},
		{
			name: "invalid url",
			path: "/api/v1/sources/1",
			body: `{"url":"not-a-url"}`,
			mock: func(service *sourcemocks.MockService) {
				service.EXPECT().
					UpdateSource(gomock.Any(), gomock.AssignableToTypeOf(source.UpdateSourceRequest{})).
					Return(nil, source.ErrInvalidSourceURL)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_source_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := sourcemocks.NewMockService(ctrl)
			tt.mock(service)

			router := newSourceTestRouter(service, 1)

			w := performJSONRequest(router, http.MethodPatch, tt.path, tt.body)

			assertStatus(t, w, tt.wantStatus)
			assertErrorCode(t, w, tt.wantCode)
		})
	}
}
func TestSourceHandler_MissingUserContext(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/api/v1/sources",
			body:   `{"name":"Go Blog","type":"rss","url":"https://go.dev/blog/feed.atom"}`,
		},
		{
			name:   "list",
			method: http.MethodGet,
			path:   "/api/v1/sources",
		},
		{
			name:   "get",
			method: http.MethodGet,
			path:   "/api/v1/sources/1",
		},
		{
			name:   "update",
			method: http.MethodPatch,
			path:   "/api/v1/sources/1",
			body:   `{"name":"New Name"}`,
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   "/api/v1/sources/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := sourcemocks.NewMockService(ctrl)

			gin.SetMode(gin.TestMode)
			r := gin.New()

			h := source.NewHandler(service)
			api := r.Group("/api/v1")

			authRequiredWithoutUserContext := func(c *gin.Context) {
				c.Next()
			}

			source.RegisterRoutes(api, h, authRequiredWithoutUserContext)

			var w *httptest.ResponseRecorder
			if tt.body != "" {
				w = performJSONRequest(r, tt.method, tt.path, tt.body)
			} else {
				w = performRequest(r, tt.method, tt.path)
			}

			assertStatus(t, w, http.StatusUnauthorized)
			assertErrorCode(t, w, "unauthorized")
		})
	}
}

func newSourceTestRouter(service source.Service, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()

	h := source.NewHandler(service)

	api := r.Group("/api/v1")

	authRequired := func(c *gin.Context) {
		ctx := requestctx.WithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}

	source.RegisterRoutes(api, h, authRequired)

	return r
}

func performJSONRequest(router http.Handler, method, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w
}

func performRequest(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)

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

func sampleSourceDTO() source.SourceDTO {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	url := "https://go.dev/blog/feed.atom"

	return source.SourceDTO{
		ID:               1,
		Name:             "Go Blog",
		Type:             source.TypeRSS,
		URL:              &url,
		Config:           json.RawMessage(`{}`),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
