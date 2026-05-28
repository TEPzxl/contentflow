package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
)

func TestHandler_UpdateAISettingsRedactsAPIKey(t *testing.T) {
	service := &fakeHandlerService{
		updateSettings: func(_ context.Context, req UpdateAISettingsRequest) (*AISettingsDTO, error) {
			if req.UserID != 10 {
				t.Fatalf("UserID = %d, want 10", req.UserID)
			}
			if req.APIKey == nil || *req.APIKey != "sk-test" {
				t.Fatalf("APIKey = %v, want sk-test", req.APIKey)
			}
			return &AISettingsDTO{
				Provider:       "openai-compatible",
				BaseURL:        "http://ai.local/v1",
				Model:          "chat-model",
				EmbeddingModel: "embed-model",
				HasAPIKey:      true,
			}, nil
		},
	}
	router := newAIHandlerTestRouter(service, 10)

	w := performAIJSONRequest(router, http.MethodPut, "/api/v1/ai/settings", `{
		"provider": "openai-compatible",
		"base_url": "http://ai.local/v1",
		"model": "chat-model",
		"embedding_model": "embed-model",
		"api_key": "sk-test"
	}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "sk-test") {
		t.Fatalf("response leaked api key: %s", w.Body.String())
	}
	var payload struct {
		Data struct {
			Settings AISettingsDTO `json:"settings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Data.Settings.HasAPIKey {
		t.Fatalf("settings = %#v, want has_api_key", payload.Data.Settings)
	}
}

func TestHandler_GetAISettings(t *testing.T) {
	service := &fakeHandlerService{
		getSettings: func(_ context.Context, req GetAISettingsRequest) (*AISettingsDTO, error) {
			if req.UserID != 10 {
				t.Fatalf("UserID = %d, want 10", req.UserID)
			}
			return &AISettingsDTO{Provider: "local", BaseURL: DefaultOpenAIBaseURL, HasAPIKey: false}, nil
		},
	}
	router := newAIHandlerTestRouter(service, 10)

	w := performAIJSONRequest(router, http.MethodGet, "/api/v1/ai/settings", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

type fakeHandlerService struct {
	getSettings    func(context.Context, GetAISettingsRequest) (*AISettingsDTO, error)
	updateSettings func(context.Context, UpdateAISettingsRequest) (*AISettingsDTO, error)
}

func (s *fakeHandlerService) RequestSummary(context.Context, RequestSummaryRequest) (*SummaryDTO, error) {
	return nil, nil
}

func (s *fakeHandlerService) GetSummary(context.Context, GetSummaryRequest) (*SummaryDTO, error) {
	return nil, nil
}

func (s *fakeHandlerService) GenerateEmbedding(context.Context, GenerateEmbeddingRequest) (*EmbeddingDTO, error) {
	return nil, nil
}

func (s *fakeHandlerService) SimilarArticles(context.Context, SimilarArticlesRequest) ([]SimilarArticleDTO, error) {
	return nil, nil
}

func (s *fakeHandlerService) GenerateDigest(context.Context, GenerateDigestRequest) (*DigestDTO, error) {
	return nil, nil
}

func (s *fakeHandlerService) GetDigest(context.Context, GetDigestRequest) (*DigestDTO, error) {
	return nil, nil
}

func (s *fakeHandlerService) RAGSearch(context.Context, RAGSearchRequest) (*RAGAnswerDTO, error) {
	return nil, nil
}

func (s *fakeHandlerService) GetAISettings(ctx context.Context, req GetAISettingsRequest) (*AISettingsDTO, error) {
	return s.getSettings(ctx, req)
}

func (s *fakeHandlerService) UpdateAISettings(ctx context.Context, req UpdateAISettingsRequest) (*AISettingsDTO, error) {
	return s.updateSettings(ctx, req)
}

func newAIHandlerTestRouter(service HandlerService, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(service)
	api := r.Group("/api/v1")
	authRequired := func(c *gin.Context) {
		ctx := requestctx.WithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
	RegisterRoutes(api, h, authRequired)
	return r
}

func performAIJSONRequest(router http.Handler, method, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
