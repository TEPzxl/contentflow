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

func (h *Handler) ListCollectionRuns(c *gin.Context) {
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

	limit, err := parseCollectionIntQuery(c, "limit", 20)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_query", "invalid limit query parameter")
		return
	}

	offset, err := parseCollectionIntQuery(c, "offset", 0)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_query", "invalid offset query parameter")
		return
	}

	result, err := h.service.ListCollectionRuns(c.Request.Context(), ListCollectionRunsRequest{
		UserID:   userID,
		SourceID: sourceID,
		Status:   c.Query("status"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		h.handleCollectionError(c, err)
		return
	}

	runs := make([]collectionRunHTTPResp, 0, len(result.Runs))
	for _, run := range result.Runs {
		runs = append(runs, toCollectionRunDTOHTTPResp(run))
	}

	response.OK(c, listCollectionRunsHTTPResp{
		CollectionRuns: runs,
		Total:          result.Total,
		Limit:          result.Limit,
		Offset:         result.Offset,
	})
}

func (h *Handler) GetCollectionRun(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}

	runID, err := parseCollectionIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_collection_run_id", "invalid collection run id")
		return
	}

	result, err := h.service.GetCollectionRun(c.Request.Context(), GetCollectionRunRequest{
		UserID: userID,
		RunID:  runID,
	})
	if err != nil {
		h.handleCollectionError(c, err)
		return
	}

	response.OK(c, getCollectionRunHTTPResp{
		CollectionRun: toCollectionRunDTOHTTPResp(result.Run),
	})
}

func (h *Handler) handleCollectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, source.ErrSourceNotAccessible):
		response.Error(c, http.StatusNotFound, "source_not_found", "source not found")
	case errors.Is(err, ErrCollectionRunNotFound):
		response.Error(c, http.StatusNotFound, "collection_run_not_found", "collection run not found")
	case errors.Is(err, ErrCollectorNotFound):
		response.Error(c, http.StatusBadRequest, "collector_not_found", "collector not found")
	case errors.Is(err, ErrCollectionInProgress):
		response.Error(c, http.StatusConflict, "collection_in_progress", "collection already in progress")
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

func parseCollectionIDParam(c *gin.Context, name string) (int64, error) {
	raw := c.Param(name)

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}

	return id, nil
}

func parseCollectionIntQuery(c *gin.Context, name string, defaultValue int) (int, error) {
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

func toCollectionRunDTOHTTPResp(run CollectionRunDTO) collectionRunHTTPResp {
	return collectionRunHTTPResp{
		RunID:           run.ID,
		SourceID:        run.SourceID,
		Status:          run.Status,
		StartedAt:       run.StartedAt,
		FinishedAt:      run.FinishedAt,
		FetchedCount:    run.FetchedCount,
		InsertedCount:   run.InsertedCount,
		DuplicatedCount: run.DuplicatedCount,
		ErrorMessage:    run.ErrorMessage,
	}
}

type collectSourceHTTPResp struct {
	CollectionRun collectionRunHTTPResp `json:"collection_run"`
}

type listCollectionRunsHTTPResp struct {
	CollectionRuns []collectionRunHTTPResp `json:"collection_runs"`
	Total          int64                   `json:"total"`
	Limit          int                     `json:"limit"`
	Offset         int                     `json:"offset"`
}

type getCollectionRunHTTPResp struct {
	CollectionRun collectionRunHTTPResp `json:"collection_run"`
}

type collectionRunHTTPResp struct {
	RunID           int64   `json:"run_id"`
	SourceID        int64   `json:"source_id"`
	Status          string  `json:"status"`
	StartedAt       string  `json:"started_at,omitempty"`
	FinishedAt      *string `json:"finished_at,omitempty"`
	FetchedCount    int     `json:"fetched_count"`
	InsertedCount   int     `json:"inserted_count"`
	DuplicatedCount int     `json:"duplicated_count"`
	ErrorMessage    string  `json:"error_message"`
}
