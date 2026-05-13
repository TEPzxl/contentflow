package collector_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/module/collector"
	collectormocks "github.com/tepzxl/contentflow/internal/module/collector/mocks"
	"github.com/tepzxl/contentflow/internal/module/source"
	"go.uber.org/mock/gomock"
)

func TestCollectionHandler_CollectSource(t *testing.T) {
	tests := []struct {
		name       string
		userID     int64
		path       string
		mock       func(service *collectormocks.MockService)
		wantStatus int
		wantCode   string
		wantData   func(t *testing.T, body *bytes.Buffer)
	}{
		{
			name:   "success",
			userID: 100,
			path:   "/api/v1/sources/42/collect",
			mock: func(service *collectormocks.MockService) {
				service.EXPECT().
					CollectSource(gomock.Any(), collector.CollectSourceRequest{
						UserID:   100,
						SourceID: 42,
					}).
					Return(&collector.CollectSourceResponse{
						RunID:           11,
						SourceID:        42,
						Status:          collector.RunStatusSuccess,
						FetchedCount:    3,
						InsertedCount:   2,
						DuplicatedCount: 1,
						ErrorMessage:    "",
					}, nil)
			},
			wantStatus: http.StatusOK,
			wantData: func(t *testing.T, body *bytes.Buffer) {
				t.Helper()
				got := decodeCollectorJSONBody(t, body)
				run := collectionRunBody(t, got)
				assertFloatField(t, run, "run_id", 11)
				assertFloatField(t, run, "source_id", 42)
				assertStringField(t, run, "status", collector.RunStatusSuccess)
				assertFloatField(t, run, "fetched_count", 3)
				assertFloatField(t, run, "inserted_count", 2)
				assertFloatField(t, run, "duplicated_count", 1)
			},
		},
		{
			name:   "collection failed returns run result",
			userID: 100,
			path:   "/api/v1/sources/42/collect",
			mock: func(service *collectormocks.MockService) {
				service.EXPECT().
					CollectSource(gomock.Any(), collector.CollectSourceRequest{
						UserID:   100,
						SourceID: 42,
					}).
					Return(&collector.CollectSourceResponse{
						RunID:           12,
						SourceID:        42,
						Status:          collector.RunStatusFailed,
						FetchedCount:    0,
						InsertedCount:   0,
						DuplicatedCount: 0,
						ErrorMessage:    "fetch failed",
					}, collector.ErrCollectionFailed)
			},
			wantStatus: http.StatusOK,
			wantData: func(t *testing.T, body *bytes.Buffer) {
				t.Helper()
				got := decodeCollectorJSONBody(t, body)
				run := collectionRunBody(t, got)
				assertFloatField(t, run, "run_id", 12)
				assertStringField(t, run, "status", collector.RunStatusFailed)
				assertStringField(t, run, "error_message", "fetch failed")
			},
		},
		{
			name:   "source not accessible",
			userID: 100,
			path:   "/api/v1/sources/999/collect",
			mock: func(service *collectormocks.MockService) {
				service.EXPECT().
					CollectSource(gomock.Any(), collector.CollectSourceRequest{
						UserID:   100,
						SourceID: 999,
					}).
					Return(nil, source.ErrSourceNotAccessible)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "source_not_found",
		},
		{
			name:   "collector not found",
			userID: 100,
			path:   "/api/v1/sources/42/collect",
			mock: func(service *collectormocks.MockService) {
				service.EXPECT().
					CollectSource(gomock.Any(), collector.CollectSourceRequest{
						UserID:   100,
						SourceID: 42,
					}).
					Return(nil, collector.ErrCollectorNotFound)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "collector_not_found",
		},
		{
			name:   "internal service error",
			userID: 100,
			path:   "/api/v1/sources/42/collect",
			mock: func(service *collectormocks.MockService) {
				service.EXPECT().
					CollectSource(gomock.Any(), collector.CollectSourceRequest{
						UserID:   100,
						SourceID: 42,
					}).
					Return(nil, errors.New("database unavailable"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
		{
			name:   "invalid source id",
			userID: 100,
			path:   "/api/v1/sources/abc/collect",
			mock: func(service *collectormocks.MockService) {
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_source_id",
		},
		{
			name:   "missing user context",
			userID: 0,
			path:   "/api/v1/sources/42/collect",
			mock: func(service *collectormocks.MockService) {
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := collectormocks.NewMockService(ctrl)
			tt.mock(service)

			router := newCollectionTestRouter(service, tt.userID)
			w := performCollectionRequest(router, http.MethodPost, tt.path)

			assertCollectorStatus(t, w, tt.wantStatus)
			assertCollectorErrorCode(t, w, tt.wantCode)

			if tt.wantData != nil {
				tt.wantData(t, w.Body)
			}
		})
	}
}

func newCollectionTestRouter(service collector.Service, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	h := collector.NewHandler(service)
	api := r.Group("/api/v1")

	authRequired := func(c *gin.Context) {
		if userID > 0 {
			ctx := requestctx.WithUserID(c.Request.Context(), userID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}

	collector.RegisterRoutes(api, h, authRequired)

	return r
}

func performCollectionRequest(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w
}

func decodeCollectorJSONBody(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v, body=%s", err, body.String())
	}

	return got
}

func assertCollectorStatus(t *testing.T, w *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()

	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, wantStatus, w.Body.String())
	}
}

func assertCollectorErrorCode(t *testing.T, w *httptest.ResponseRecorder, wantCode string) {
	t.Helper()

	if wantCode == "" {
		return
	}

	got := decodeCollectorJSONBody(t, w.Body)
	errBody, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("response error body missing: %v", got)
	}

	if errBody["code"] != wantCode {
		t.Fatalf("error code = %v, want %s", errBody["code"], wantCode)
	}
}

func collectionRunBody(t *testing.T, got map[string]any) map[string]any {
	t.Helper()

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("response data body missing: %v", got)
	}

	run, ok := data["collection_run"].(map[string]any)
	if !ok {
		t.Fatalf("collection_run body missing: %v", data)
	}

	return run
}

func assertFloatField(t *testing.T, body map[string]any, field string, want float64) {
	t.Helper()

	if body[field] != want {
		t.Fatalf("%s = %v, want %v", field, body[field], want)
	}
}

func assertStringField(t *testing.T, body map[string]any, field string, want string) {
	t.Helper()

	if body[field] != want {
		t.Fatalf("%s = %v, want %s", field, body[field], want)
	}
}
