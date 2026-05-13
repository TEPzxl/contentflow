package collector

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type CollectionRequester interface {
	RequestCollection(ctx context.Context, req CollectSourceRequest) (*RequestCollectionResponse, error)
}

type AsyncHandler struct {
	requester CollectionRequester
}

func NewAsyncHandler(requester CollectionRequester) *AsyncHandler {
	return &AsyncHandler{requester: requester}
}

func (h *AsyncHandler) RequestCollection(c *gin.Context) {
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

	result, err := h.requester.RequestCollection(c.Request.Context(), CollectSourceRequest{
		UserID:   userID,
		SourceID: sourceID,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"data": requestCollectionHTTPResp{
			CollectionTask: collectionTaskHTTPResp{
				TaskID:   result.TaskID,
				SourceID: result.SourceID,
				Status:   result.Status,
			},
		},
	})
}

type requestCollectionHTTPResp struct {
	CollectionTask collectionTaskHTTPResp `json:"collection_task"`
}

type collectionTaskHTTPResp struct {
	TaskID   string `json:"task_id"`
	SourceID int64  `json:"source_id"`
	Status   string `json:"status"`
}
