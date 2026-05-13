package collector_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/module/collector"
)

func TestAsyncCollectionHandler_RequestCollection(t *testing.T) {
	requester := &fakeCollectionRequester{
		resp: &collector.RequestCollectionResponse{
			TaskID:   "task-1",
			SourceID: 42,
			Status:   collector.RunStatusQueued,
		},
	}

	router := newAsyncCollectionTestRouter(requester, 100)
	w := performCollectionRequest(router, http.MethodPost, "/api/v1/sources/42/collect")

	assertCollectorStatus(t, w, http.StatusAccepted)

	got := decodeCollectorJSONBody(t, w.Body)
	task := collectionTaskBody(t, got)
	assertStringField(t, task, "task_id", "task-1")
	assertFloatField(t, task, "source_id", 42)
	assertStringField(t, task, "status", collector.RunStatusQueued)

	if len(requester.reqs) != 1 {
		t.Fatalf("request count = %d, want 1", len(requester.reqs))
	}
	if requester.reqs[0].UserID != 100 || requester.reqs[0].SourceID != 42 {
		t.Fatalf("request = %#v", requester.reqs[0])
	}
}

type fakeCollectionRequester struct {
	resp *collector.RequestCollectionResponse
	err  error
	reqs []collector.CollectSourceRequest
}

func (f *fakeCollectionRequester) RequestCollection(ctx context.Context, req collector.CollectSourceRequest) (*collector.RequestCollectionResponse, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newAsyncCollectionTestRouter(requester collector.CollectionRequester, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	h := collector.NewAsyncHandler(requester)
	api := r.Group("/api/v1")

	authRequired := func(c *gin.Context) {
		if userID > 0 {
			ctx := requestctx.WithUserID(c.Request.Context(), userID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}

	collector.RegisterAsyncRoutes(api, h, authRequired)

	return r
}

func collectionTaskBody(t *testing.T, got map[string]any) map[string]any {
	t.Helper()

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("response data body missing: %v", got)
	}

	task, ok := data["collection_task"].(map[string]any)
	if !ok {
		t.Fatalf("collection_task body missing: %v", data)
	}

	return task
}
