package collectionjob

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
)

func TestDLQHandler_ListReplayAndMarkHandled(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	repo := newMemoryDLQRepository()
	writer := &fakeEventWriter{}
	service := NewDLQService(repo, writer, WithDLQNow(func() time.Time { return now }))
	item, err := repo.Create(context.Background(), CreateDLQItemParams{
		Event: CollectionRequested{
			TaskID:         "task-1",
			UserID:         100,
			SourceID:       42,
			IdempotencyKey: "collection:source:42",
			Attempt:        2,
			RequestedAt:    now,
		},
		ErrorMessage: "collect failed",
		CreatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	router := newDLQTestRouter(service)

	listResp := performDLQRequest(router, http.MethodGet, "/api/v1/collection-dlq?status=pending")
	assertDLQStatus(t, listResp, http.StatusOK)
	body := decodeDLQJSONBody(t, listResp.Body)
	data := body["data"].(map[string]any)
	if data["total"] != float64(1) {
		t.Fatalf("total = %v, want 1", data["total"])
	}

	replayResp := performDLQRequest(router, http.MethodPost, "/api/v1/collection-dlq/1/replay")
	assertDLQStatus(t, replayResp, http.StatusOK)
	if len(writer.events) != 1 {
		t.Fatalf("written events = %d, want 1", len(writer.events))
	}

	handledResp := performDLQRequest(router, http.MethodPost, "/api/v1/collection-dlq/1/handled")
	assertDLQStatus(t, handledResp, http.StatusOK)
	updated, err := repo.FindByID(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if updated.Status != DLQStatusHandled {
		t.Fatalf("status = %s, want %s", updated.Status, DLQStatusHandled)
	}
}

func TestDLQHandler_ReturnsNotFoundForOtherUsersDLQItem(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	repo := newMemoryDLQRepository()
	writer := &fakeEventWriter{}
	service := NewDLQService(repo, writer, WithDLQNow(func() time.Time { return now }))
	if _, err := repo.Create(context.Background(), CreateDLQItemParams{
		Event: CollectionRequested{
			TaskID:         "other-task",
			UserID:         200,
			SourceID:       99,
			IdempotencyKey: "collection:source:99",
			Attempt:        2,
			RequestedAt:    now,
		},
		ErrorMessage: "other user failure",
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	router := newDLQTestRouter(service)

	listResp := performDLQRequest(router, http.MethodGet, "/api/v1/collection-dlq?status=pending")
	assertDLQStatus(t, listResp, http.StatusOK)
	body := decodeDLQJSONBody(t, listResp.Body)
	data := body["data"].(map[string]any)
	if data["total"] != float64(0) {
		t.Fatalf("total = %v, want 0", data["total"])
	}

	replayResp := performDLQRequest(router, http.MethodPost, "/api/v1/collection-dlq/1/replay")
	assertDLQStatus(t, replayResp, http.StatusNotFound)
	if len(writer.events) != 0 {
		t.Fatalf("written events = %d, want 0", len(writer.events))
	}

	handledResp := performDLQRequest(router, http.MethodPost, "/api/v1/collection-dlq/1/handled")
	assertDLQStatus(t, handledResp, http.StatusNotFound)
}

func newDLQTestRouter(service *DLQService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	authRequired := func(c *gin.Context) {
		ctx := requestctx.WithUserID(c.Request.Context(), 100)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
	RegisterDLQRoutes(api, NewDLQHandler(service), authRequired)
	return r
}

func performDLQRequest(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func assertDLQStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, want, w.Body.String())
	}
}

func decodeDLQJSONBody(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return got
}
