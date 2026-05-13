package source

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user information")
		return
	}

	var req createSourceHTTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request", "invalid create source request")
		return
	}

	result, err := h.service.CreateSource(c.Request.Context(), CreateSourceRequest{
		UserID: userID,
		Name:   req.Name,
		Type:   req.Type,
		URL:    req.URL,
		Config: req.Config,
	})
	if err != nil {
		h.handleSourceError(c, err)
		return
	}
	response.OK(c, createSourceHTTPResp{
		Source: toSourceHTTPResp(result.Source),
	})
}

func (h *Handler) List(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user information")
		return
	}

	limit, err := parseIntQuery(c, "limit", 20)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request", "invalid limit query parameter")
		return
	}

	offset, err := parseIntQuery(c, "offset", 0)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request", "invalid offset query parameter")
		return
	}

	result, err := h.service.ListSources(c.Request.Context(), ListSourcesRequest{
		UserID: userID,
		Type:   c.Query("type"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.handleSourceError(c, err)
		return
	}

	items := make([]sourceHTTPResp, 0, len(result.Sources))
	for _, src := range result.Sources {
		items = append(items, toSourceHTTPResp(src))
	}

	response.OK(c, listSourcesHTTPResp{
		Sources: items,
		Total:   result.Total,
		Limit:   result.Limit,
		Offset:  result.Offset,
	})
}

func (h *Handler) Get(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user information")
		return
	}

	sourceID, err := parseIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid source id", "invalid source id")
		return
	}

	result, err := h.service.GetSource(c.Request.Context(), GetSourceRequest{
		UserID:   userID,
		SourceID: sourceID,
	})

	if err != nil {
		h.handleSourceError(c, err)
		return
	}

	response.OK(c, getSourceHTTPResp{
		Source: toSourceHTTPResp(result.Source),
	})
}

func (h *Handler) Update(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}

	sourceID, err := parseIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_source_id", "invalid source id")
		return
	}

	var req updateSourceHTTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid update source request")
		return
	}

	result, err := h.service.UpdateSource(c.Request.Context(), UpdateSourceRequest{
		UserID:   userID,
		SourceID: sourceID,
		Name:     req.Name,
		URL:      req.URL,
		IsActive: req.IsActive,
		Config:   req.Config,
	})
	if err != nil {
		h.handleSourceError(c, err)
		return
	}

	response.OK(c, updateSourceHTTPResp{
		Source: toSourceHTTPResp(result.Source),
	})
}

func (h *Handler) Delete(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}

	sourceID, err := parseIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_source_id", "invalid source id")
		return
	}

	if err := h.service.DeleteSource(c.Request.Context(), DeleteSourceRequest{
		UserID:   userID,
		SourceID: sourceID,
	}); err != nil {
		h.handleSourceError(c, err)
		return
	}

	response.OK(c, gin.H{
		"message": "source deleted",
	})
}

func (h *Handler) handleSourceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidSourceName):
		response.Error(c, http.StatusBadRequest, "invalid_source_name", "invalid source name")

	case errors.Is(err, ErrInvalidSourceType):
		response.Error(c, http.StatusBadRequest, "invalid_source_type", "invalid source type")

	case errors.Is(err, ErrInvalidSourceURL):
		response.Error(c, http.StatusBadRequest, "invalid_source_url", "invalid source url")

	case errors.Is(err, ErrInvalidSourceConfig):
		response.Error(c, http.StatusBadRequest, "invalid_source_config", "invalid source config")

	case errors.Is(err, ErrSourceAlreadyExists):
		response.Error(c, http.StatusConflict, "source_already_exists", "source already exists")

	case errors.Is(err, ErrSourceNotAccessible):
		response.Error(c, http.StatusNotFound, "source_not_found", "source not found")

	default:
		response.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseIDParam(c *gin.Context, name string) (int64, error) {
	raw := c.Param(name)

	id, err := strconv.ParseInt(raw, 10, 64)
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

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}

	return value, nil
}

func toSourceHTTPResp(src SourceDTO) sourceHTTPResp {
	return sourceHTTPResp{
		ID:               src.ID,
		Name:             src.Name,
		Type:             src.Type,
		URL:              src.URL,
		Config:           src.Config,
		IsActive:         src.IsActive,
		LastFetchedAt:    src.LastFetchedAt,
		LastFetchStatus:  src.LastFetchStatus,
		LastFetchMessage: src.LastFetchMessage,
		CreatedAt:        src.CreatedAt,
		UpdatedAt:        src.UpdatedAt,
	}
}

type createSourceHTTPReq struct {
	Name   string          `json:"name" binding:"required"`
	Type   string          `json:"type" binding:"required"`
	URL    *string         `json:"url"`
	Config json.RawMessage `json:"config"`
}

type updateSourceHTTPReq struct {
	Name     *string         `json:"name"`
	URL      *string         `json:"url"`
	IsActive *bool           `json:"is_active"`
	Config   json.RawMessage `json:"config"`
}

type sourceHTTPResp struct {
	ID               int64           `json:"id"`
	Name             string          `json:"name"`
	Type             string          `json:"type"`
	URL              *string         `json:"url"`
	Config           json.RawMessage `json:"config"`
	IsActive         bool            `json:"is_active"`
	LastFetchedAt    *time.Time      `json:"last_fetched_at"`
	LastFetchStatus  string          `json:"last_fetch_status"`
	LastFetchMessage string          `json:"last_fetch_message"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type createSourceHTTPResp struct {
	Source sourceHTTPResp `json:"source"`
}

type getSourceHTTPResp struct {
	Source sourceHTTPResp `json:"source"`
}

type updateSourceHTTPResp struct {
	Source sourceHTTPResp `json:"source"`
}

type listSourcesHTTPResp struct {
	Sources []sourceHTTPResp `json:"sources"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}
