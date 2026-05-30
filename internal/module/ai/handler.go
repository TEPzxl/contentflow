package ai

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
	"github.com/tepzxl/contentflow/internal/module/article"
)

type HandlerService interface {
	RequestSummary(ctx context.Context, req RequestSummaryRequest) (*SummaryDTO, error)
	GetSummary(ctx context.Context, req GetSummaryRequest) (*SummaryDTO, error)
	GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*EmbeddingDTO, error)
	SimilarArticles(ctx context.Context, req SimilarArticlesRequest) ([]SimilarArticleDTO, error)
	GenerateDigest(ctx context.Context, req GenerateDigestRequest) (*DigestDTO, error)
	GetDigest(ctx context.Context, req GetDigestRequest) (*DigestDTO, error)
	RAGSearch(ctx context.Context, req RAGSearchRequest) (*RAGAnswerDTO, error)
	GetAISettings(ctx context.Context, req GetAISettingsRequest) (*AISettingsDTO, error)
	UpdateAISettings(ctx context.Context, req UpdateAISettingsRequest) (*AISettingsDTO, error)
}

type Handler struct {
	service HandlerService
}

func NewHandler(service HandlerService) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	articles := rg.Group("/articles")
	articles.Use(authRequired)
	articles.POST("/:id/summary", h.RequestSummary)
	articles.GET("/:id/summary", h.GetSummary)
	articles.POST("/:id/embedding", h.GenerateEmbedding)
	articles.GET("/:id/similar", h.SimilarArticles)

	ai := rg.Group("/ai")
	ai.Use(authRequired)
	ai.POST("/digests/:date", h.GenerateDigest)
	ai.GET("/digests/:date", h.GetDigest)
	ai.POST("/rag-search", h.RAGSearch)
	ai.GET("/settings", h.GetAISettings)
	ai.PUT("/settings", h.UpdateAISettings)
}

func (h *Handler) RequestSummary(c *gin.Context) {
	userID, articleID, ok := userAndArticleID(c)
	if !ok {
		return
	}
	var req requestSummaryHTTPReq
	_ = c.ShouldBindJSON(&req)

	result, err := h.service.RequestSummary(c.Request.Context(), RequestSummaryRequest{
		UserID:     userID,
		ArticleID:  articleID,
		Regenerate: req.Regenerate,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, summaryHTTPWrapper{Summary: *result})
}

func (h *Handler) GetSummary(c *gin.Context) {
	userID, articleID, ok := userAndArticleID(c)
	if !ok {
		return
	}
	result, err := h.service.GetSummary(c.Request.Context(), GetSummaryRequest{
		UserID:    userID,
		ArticleID: articleID,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, summaryHTTPWrapper{Summary: *result})
}

func (h *Handler) GenerateEmbedding(c *gin.Context) {
	userID, articleID, ok := userAndArticleID(c)
	if !ok {
		return
	}
	result, err := h.service.GenerateEmbedding(c.Request.Context(), GenerateEmbeddingRequest{
		UserID:    userID,
		ArticleID: articleID,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, embeddingHTTPWrapper{Embedding: *result})
}

func (h *Handler) SimilarArticles(c *gin.Context) {
	userID, articleID, ok := userAndArticleID(c)
	if !ok {
		return
	}
	limit, err := parseIntQuery(c, "limit", 5)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_limit", "invalid limit")
		return
	}
	result, err := h.service.SimilarArticles(c.Request.Context(), SimilarArticlesRequest{
		UserID:    userID,
		ArticleID: articleID,
		Limit:     limit,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, similarArticlesHTTPResp{Articles: result})
}

func (h *Handler) GenerateDigest(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	date, ok := parseDateParam(c)
	if !ok {
		return
	}
	result, err := h.service.GenerateDigest(c.Request.Context(), GenerateDigestRequest{UserID: userID, Date: date})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, digestHTTPWrapper{Digest: *result})
}

func (h *Handler) GetDigest(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	date, ok := parseDateParam(c)
	if !ok {
		return
	}
	result, err := h.service.GetDigest(c.Request.Context(), GetDigestRequest{UserID: userID, Date: date})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, digestHTTPWrapper{Digest: *result})
}

func (h *Handler) RAGSearch(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	var req ragSearchHTTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid rag search request")
		return
	}
	result, err := h.service.RAGSearch(c.Request.Context(), RAGSearchRequest{
		UserID: userID,
		Query:  req.Query,
		Limit:  req.Limit,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, ragAnswerHTTPWrapper{Answer: *result})
}

func (h *Handler) GetAISettings(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	result, err := h.service.GetAISettings(c.Request.Context(), GetAISettingsRequest{UserID: userID})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, aiSettingsHTTPWrapper{Settings: *result})
}

func (h *Handler) UpdateAISettings(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	var req updateAISettingsHTTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid ai settings request")
		return
	}
	result, err := h.service.UpdateAISettings(c.Request.Context(), UpdateAISettingsRequest{
		UserID:         userID,
		Provider:       req.Provider,
		BaseURL:        req.BaseURL,
		Model:          req.Model,
		EmbeddingModel: req.EmbeddingModel,
		APIKey:         req.APIKey,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, aiSettingsHTTPWrapper{Settings: *result})
}

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, article.ErrArticleNotFound):
		response.Error(c, http.StatusNotFound, "article_not_found", "article not found")
	case errors.Is(err, ErrSummaryNotFound):
		response.Error(c, http.StatusNotFound, "summary_not_found", "summary not found")
	case errors.Is(err, ErrEmbeddingNotFound):
		response.Error(c, http.StatusNotFound, "embedding_not_found", "embedding not found")
	case errors.Is(err, ErrDigestNotFound):
		response.Error(c, http.StatusNotFound, "digest_not_found", "digest not found")
	case errors.Is(err, ErrEmptyQuery):
		response.Error(c, http.StatusBadRequest, "empty_query", "query is required")
	case errors.Is(err, ErrAISettingsEncryptionKeyRequired):
		response.Error(c, http.StatusBadRequest, "ai_settings_encryption_key_required", "ai settings encryption key is required")
	case errors.Is(err, ErrInvalidAIProvider):
		response.Error(c, http.StatusBadRequest, "invalid_ai_provider", "invalid ai provider")
	case errors.Is(err, ErrInvalidAIBaseURL):
		response.Error(c, http.StatusBadRequest, "invalid_ai_base_url", "invalid ai base url")
	case errors.Is(err, ErrInvalidAIModel):
		response.Error(c, http.StatusBadRequest, "invalid_ai_model", "invalid ai model")
	default:
		response.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func userAndArticleID(c *gin.Context) (int64, int64, bool) {
	userID, ok := userID(c)
	if !ok {
		return 0, 0, false
	}
	articleID, err := parseIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_article_id", "invalid article id")
		return 0, 0, false
	}
	return userID, articleID, true
}

func userID(c *gin.Context) (int64, bool) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return 0, false
	}
	return userID, true
}

func parseIDParam(c *gin.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}

func parseIntQuery(c *gin.Context, name string, defaultValue int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(raw)
}

func parseDateParam(c *gin.Context) (time.Time, bool) {
	date, err := time.Parse(time.DateOnly, c.Param("date"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_date", "invalid date")
		return time.Time{}, false
	}
	return date, true
}

type requestSummaryHTTPReq struct {
	Regenerate bool `json:"regenerate"`
}

type ragSearchHTTPReq struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type updateAISettingsHTTPReq struct {
	Provider       string  `json:"provider"`
	BaseURL        string  `json:"base_url"`
	Model          string  `json:"model"`
	EmbeddingModel string  `json:"embedding_model"`
	APIKey         *string `json:"api_key"`
}

type summaryHTTPWrapper struct {
	Summary SummaryDTO `json:"summary"`
}

type embeddingHTTPWrapper struct {
	Embedding EmbeddingDTO `json:"embedding"`
}

type similarArticlesHTTPResp struct {
	Articles []SimilarArticleDTO `json:"articles"`
}

type digestHTTPWrapper struct {
	Digest DigestDTO `json:"digest"`
}

type ragAnswerHTTPWrapper struct {
	Answer RAGAnswerDTO `json:"answer"`
}

type aiSettingsHTTPWrapper struct {
	Settings AISettingsDTO `json:"settings"`
}
