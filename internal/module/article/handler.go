package article

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type HandlerService interface {
	ListArticles(ctx context.Context, req ListArticlesRequest) (*ListArticlesResponse, error)
	GetArticle(ctx context.Context, req GetArticleRequest) (*GetArticleResponse, error)
	UpdateState(ctx context.Context, req UpdateArticleStateRequest) (*UpdateArticleStateResponse, error)
}

type Handler struct {
	service HandlerService
}

func NewHandler(service HandlerService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}

	sourceID, err := parseOptionalInt64Query(c, "source_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_source_id", "invalid source id")
		return
	}

	isRead, err := parseOptionalBoolQuery(c, "is_read")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_is_read", "invalid is_read")
		return
	}

	isSaved, err := parseOptionalBoolQuery(c, "is_saved")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_is_saved", "invalid is_saved")
		return
	}

	limit, err := parseIntQuery(c, "limit", 20)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_limit", "invalid limit")
		return
	}

	offset, err := parseIntQuery(c, "offset", 0)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_offset", "invalid offset")
		return
	}

	result, err := h.service.ListArticles(c.Request.Context(), ListArticlesRequest{
		UserID:   userID,
		SourceID: sourceID,
		Query:    c.Query("q"),
		IsRead:   isRead,
		IsSaved:  isSaved,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.OK(c, listArticlesHTTPResp{
		Articles: toArticleHTTPRespList(result.Articles),
		Total:    result.Total,
		Limit:    result.Limit,
		Offset:   result.Offset,
	})
}

func (h *Handler) Get(c *gin.Context) {
	userID, articleID, ok := h.userAndArticleID(c)
	if !ok {
		return
	}

	result, err := h.service.GetArticle(c.Request.Context(), GetArticleRequest{
		UserID:    userID,
		ArticleID: articleID,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.OK(c, articleHTTPWrapper{Article: toArticleHTTPResp(result.Article)})
}

func (h *Handler) MarkRead(c *gin.Context) {
	userID, articleID, ok := h.userAndArticleID(c)
	if !ok {
		return
	}

	var req updateReadHTTPReq
	if err := c.ShouldBindJSON(&req); err != nil || req.IsRead == nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid read request")
		return
	}

	result, err := h.service.UpdateState(c.Request.Context(), UpdateArticleStateRequest{
		UserID:    userID,
		ArticleID: articleID,
		IsRead:    req.IsRead,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.OK(c, articleHTTPWrapper{Article: toArticleHTTPResp(result.Article)})
}

func (h *Handler) Save(c *gin.Context) {
	userID, articleID, ok := h.userAndArticleID(c)
	if !ok {
		return
	}

	var req updateSaveHTTPReq
	if err := c.ShouldBindJSON(&req); err != nil || req.IsSaved == nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid save request")
		return
	}

	result, err := h.service.UpdateState(c.Request.Context(), UpdateArticleStateRequest{
		UserID:    userID,
		ArticleID: articleID,
		IsSaved:   req.IsSaved,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.OK(c, articleHTTPWrapper{Article: toArticleHTTPResp(result.Article)})
}

func (h *Handler) userAndArticleID(c *gin.Context) (int64, int64, bool) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return 0, 0, false
	}

	articleID, err := parseIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_article_id", "invalid article id")
		return 0, 0, false
	}

	return userID, articleID, true
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrArticleNotFound):
		response.Error(c, http.StatusNotFound, "article_not_found", "article not found")
	default:
		response.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseIDParam(c *gin.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}

func parseOptionalInt64Query(c *gin.Context, name string) (int64, error) {
	raw := c.Query(name)
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}

func parseOptionalBoolQuery(c *gin.Context, name string) (*bool, error) {
	raw := c.Query(name)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func parseIntQuery(c *gin.Context, name string, defaultValue int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(raw)
}
