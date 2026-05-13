package collector

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
	"github.com/tepzxl/contentflow/internal/module/source"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CollectSource(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}

	sourceID, err := parseSourceIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_source_id", "invalid source id")
		return
	}

	result, err := h.service.CollectSource(c.Request.Context(), CollectSourceRequest{
		UserID:   userID,
		SourceID: sourceID,
	})
	if err != nil {
		if result != nil && errors.Is(err, ErrCollectionFailed) {
			response.OK(c, collectSourceHTTPResp{
				CollectionRun: toCollectionRunHTTPResp(result),
			})
			return
		}

		h.handleCollectionError(c, err)
		return
	}

	response.OK(c, collectSourceHTTPResp{
		CollectionRun: toCollectionRunHTTPResp(result),
	})
}

func (h *Handler) handleCollectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, source.ErrSourceNotAccessible):
		response.Error(c, http.StatusNotFound, "source_not_found", "source not found")
	case errors.Is(err, ErrCollectorNotFound):
		response.Error(c, http.StatusBadRequest, "collector_not_found", "collector not found")
	default:
		response.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseSourceIDParam(c *gin.Context) (int64, error) {
	raw := c.Param("id")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}

	return id, nil
}

func toCollectionRunHTTPResp(resp *CollectSourceResponse) collectionRunHTTPResp {
	return collectionRunHTTPResp{
		RunID:           resp.RunID,
		SourceID:        resp.SourceID,
		Status:          resp.Status,
		FetchedCount:    resp.FetchedCount,
		InsertedCount:   resp.InsertedCount,
		DuplicatedCount: resp.DuplicatedCount,
		ErrorMessage:    resp.ErrorMessage,
	}
}

type collectSourceHTTPResp struct {
	CollectionRun collectionRunHTTPResp `json:"collection_run"`
}

type collectionRunHTTPResp struct {
	RunID           int64  `json:"run_id"`
	SourceID        int64  `json:"source_id"`
	Status          string `json:"status"`
	FetchedCount    int    `json:"fetched_count"`
	InsertedCount   int    `json:"inserted_count"`
	DuplicatedCount int    `json:"duplicated_count"`
	ErrorMessage    string `json:"error_message"`
}
