package article_test

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
	"github.com/tepzxl/contentflow/internal/module/article"
	"github.com/tepzxl/contentflow/internal/module/collector"
)

func TestArticleHandler_List(t *testing.T) {
	now := fixedTime()
	service := &fakeArticleService{
		listResp: &article.ListArticlesResponse{
			Articles: []article.ArticleDTO{sampleArticleDTO(now)},
			Total:    1,
			Limit:    10,
			Offset:   0,
		},
	}
	router := newArticleTestRouter(service, 100)

	w := performArticleRequest(router, http.MethodGet, "/api/v1/articles?source_id=1&q=go&is_read=false&is_saved=true&limit=10&offset=0", nil)

	assertArticleStatus(t, w, http.StatusOK)
	if len(service.listReqs) != 1 {
		t.Fatalf("list request count = %d, want 1", len(service.listReqs))
	}
	req := service.listReqs[0]
	if req.UserID != 100 || req.SourceID != 1 || req.Query != "go" || req.Limit != 10 || req.Offset != 0 {
		t.Fatalf("list req = %#v", req)
	}
	if req.IsRead == nil || *req.IsRead != false || req.IsSaved == nil || *req.IsSaved != true {
		t.Fatalf("state filters = read %v saved %v", req.IsRead, req.IsSaved)
	}
}

func TestArticleHandler_Get(t *testing.T) {
	now := fixedTime()
	service := &fakeArticleService{
		getResp: &article.GetArticleResponse{Article: sampleArticleDTO(now)},
	}
	router := newArticleTestRouter(service, 100)

	w := performArticleRequest(router, http.MethodGet, "/api/v1/articles/1", nil)

	assertArticleStatus(t, w, http.StatusOK)
	if len(service.getReqs) != 1 || service.getReqs[0].UserID != 100 || service.getReqs[0].ArticleID != 1 {
		t.Fatalf("get reqs = %#v", service.getReqs)
	}
}

func TestArticleHandler_UpdateReadAndSave(t *testing.T) {
	now := fixedTime()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		assertReq  func(t *testing.T, req article.UpdateArticleStateRequest)
		wantStatus int
	}{
		{
			name:   "mark read",
			method: http.MethodPatch,
			path:   "/api/v1/articles/1/read",
			body:   `{"is_read":true}`,
			assertReq: func(t *testing.T, req article.UpdateArticleStateRequest) {
				t.Helper()
				if req.IsRead == nil || *req.IsRead != true || req.IsSaved != nil {
					t.Fatalf("update req = %#v", req)
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "save article",
			method: http.MethodPatch,
			path:   "/api/v1/articles/1/save",
			body:   `{"is_saved":true}`,
			assertReq: func(t *testing.T, req article.UpdateArticleStateRequest) {
				t.Helper()
				if req.IsSaved == nil || *req.IsSaved != true || req.IsRead != nil {
					t.Fatalf("update req = %#v", req)
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid id",
			method:     http.MethodPatch,
			path:       "/api/v1/articles/abc/read",
			body:       `{"is_read":true}`,
			assertReq:  nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeArticleService{
				updateResp: &article.UpdateArticleStateResponse{Article: sampleArticleDTO(now)},
			}
			router := newArticleTestRouter(service, 100)

			w := performArticleRequest(router, tt.method, tt.path, bytes.NewBufferString(tt.body))

			assertArticleStatus(t, w, tt.wantStatus)
			if tt.assertReq != nil {
				if len(service.updateReqs) != 1 {
					t.Fatalf("update request count = %d, want 1", len(service.updateReqs))
				}
				tt.assertReq(t, service.updateReqs[0])
			}
		})
	}
}

type fakeArticleService struct {
	listResp   *article.ListArticlesResponse
	listErr    error
	listReqs   []article.ListArticlesRequest
	getResp    *article.GetArticleResponse
	getErr     error
	getReqs    []article.GetArticleRequest
	updateResp *article.UpdateArticleStateResponse
	updateErr  error
	updateReqs []article.UpdateArticleStateRequest
}

func (f *fakeArticleService) SaveCollectedItems(ctx context.Context, items []collector.CollectedItem) (*collector.ArticleWriteResult, error) {
	return nil, nil
}

func (f *fakeArticleService) ListArticles(ctx context.Context, req article.ListArticlesRequest) (*article.ListArticlesResponse, error) {
	f.listReqs = append(f.listReqs, req)
	return f.listResp, f.listErr
}

func (f *fakeArticleService) GetArticle(ctx context.Context, req article.GetArticleRequest) (*article.GetArticleResponse, error) {
	f.getReqs = append(f.getReqs, req)
	return f.getResp, f.getErr
}

func (f *fakeArticleService) UpdateState(ctx context.Context, req article.UpdateArticleStateRequest) (*article.UpdateArticleStateResponse, error) {
	f.updateReqs = append(f.updateReqs, req)
	return f.updateResp, f.updateErr
}

func newArticleTestRouter(service article.HandlerService, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := article.NewHandler(service)
	api := r.Group("/api/v1")
	authRequired := func(c *gin.Context) {
		ctx := requestctx.WithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
	article.RegisterRoutes(api, h, authRequired)
	return r
}

func performArticleRequest(router http.Handler, method, path string, body *bytes.Buffer) *httptest.ResponseRecorder {
	if body == nil {
		body = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func assertArticleStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, want, w.Body.String())
	}
}

func decodeArticleJSONBody(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(body.Bytes(), &got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return got
}

var _ = decodeArticleJSONBody
var _ = time.Time{}
