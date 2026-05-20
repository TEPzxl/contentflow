package collectionjob

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type DLQHandler struct {
	service *DLQService
}

func NewDLQHandler(service *DLQService) *DLQHandler {
	return &DLQHandler{service: service}
}

func (h *DLQHandler) List(c *gin.Context) {
	if !authenticated(c) {
		return
	}

	limit, err := parseDLQIntQuery(c, "limit", 20)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_limit", "invalid limit")
		return
	}
	offset, err := parseDLQIntQuery(c, "offset", 0)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_offset", "invalid offset")
		return
	}

	result, _, err := h.service.List(c.Request.Context(), ListDLQItemsRequest{
		Status: c.Query("status"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.OK(c, dlqListHTTPResp{
		Items:  toDLQItemHTTPRespList(result.Items),
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	})
}

func (h *DLQHandler) Replay(c *gin.Context) {
	if !authenticated(c) {
		return
	}

	id, err := parseDLQIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_dlq_id", "invalid dlq id")
		return
	}

	result, err := h.service.Replay(c.Request.Context(), ReplayDLQItemRequest{ID: id})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.OK(c, dlqItemHTTPWrapper{Item: toDLQItemHTTPResp(result.Item)})
}

func (h *DLQHandler) MarkHandled(c *gin.Context) {
	if !authenticated(c) {
		return
	}

	id, err := parseDLQIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_dlq_id", "invalid dlq id")
		return
	}

	result, err := h.service.MarkHandled(c.Request.Context(), MarkDLQHandledRequest{ID: id})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.OK(c, dlqItemHTTPWrapper{Item: toDLQItemHTTPResp(result.Item)})
}

func (h *DLQHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrDLQItemNotFound):
		response.Error(c, http.StatusNotFound, "dlq_item_not_found", "dlq item not found")
	default:
		response.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func authenticated(c *gin.Context) bool {
	if _, ok := requestctx.UserID(c.Request.Context()); !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return false
	}
	return true
}

func parseDLQIDParam(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}

func parseDLQIntQuery(c *gin.Context, name string, defaultValue int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(raw)
}

type dlqListHTTPResp struct {
	Items  []dlqItemHTTPResp `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

type dlqItemHTTPWrapper struct {
	Item dlqItemHTTPResp `json:"item"`
}

type dlqItemHTTPResp struct {
	ID             int64   `json:"id"`
	TaskID         string  `json:"task_id"`
	UserID         int64   `json:"user_id"`
	SourceID       int64   `json:"source_id"`
	IdempotencyKey string  `json:"idempotency_key"`
	Attempt        int     `json:"attempt"`
	ErrorMessage   string  `json:"error_message"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	ReplayedAt     *string `json:"replayed_at,omitempty"`
	HandledAt      *string `json:"handled_at,omitempty"`
}

func toDLQItemHTTPRespList(items []DLQItemDTO) []dlqItemHTTPResp {
	result := make([]dlqItemHTTPResp, 0, len(items))
	for _, item := range items {
		result = append(result, toDLQItemHTTPResp(item))
	}
	return result
}

func toDLQItemHTTPResp(item DLQItemDTO) dlqItemHTTPResp {
	return dlqItemHTTPResp{
		ID:             item.ID,
		TaskID:         item.TaskID,
		UserID:         item.UserID,
		SourceID:       item.SourceID,
		IdempotencyKey: item.IdempotencyKey,
		Attempt:        item.Attempt,
		ErrorMessage:   item.ErrorMessage,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		ReplayedAt:     item.ReplayedAt,
		HandledAt:      item.HandledAt,
	}
}
