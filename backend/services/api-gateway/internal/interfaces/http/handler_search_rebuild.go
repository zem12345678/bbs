package http

import (
	"strconv"

	"api-gateway/api/proto/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

// searchRebuildStatusView is the browser-facing form of the protobuf status.
// IDs cross the JSON boundary as strings so Snowflake-sized values retain
// their exact representation in JavaScript.
type searchRebuildStatusView struct {
	JobID          string `json:"job_id,omitempty"`
	State          string `json:"state,omitempty"`
	RequestedBy    string `json:"requested_by,omitempty"`
	ArticleTotal   int64  `json:"article_total,omitempty"`
	ArticleIndexed int64  `json:"article_indexed,omitempty"`
	TopicTotal     int64  `json:"topic_total,omitempty"`
	TopicIndexed   int64  `json:"topic_indexed,omitempty"`
	UserTotal      int64  `json:"user_total,omitempty"`
	UserIndexed    int64  `json:"user_indexed,omitempty"`
	StartedAt      int64  `json:"started_at,omitempty"`
	CompletedAt    int64  `json:"completed_at,omitempty"`
	UpdatedAt      int64  `json:"updated_at,omitempty"`
	Error          string `json:"error,omitempty"`
}

type searchRebuildStatusResponseView struct {
	Success bool                     `json:"success,omitempty"`
	Message string                   `json:"message,omitempty"`
	Status  *searchRebuildStatusView `json:"status,omitempty"`
}

func (h *Handler) startSearchRebuild(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.Admin.StartSearchRebuild(ctx, &adminpb.SearchRebuildRequest{Actor: currentActor(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, searchRebuildStatusResponseViewFromProto(result))
}

func (h *Handler) getSearchRebuildStatus(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.Admin.GetSearchRebuildStatus(ctx, &adminpb.SearchRebuildStatusRequest{Actor: currentActor(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, searchRebuildStatusResponseViewFromProto(result))
}

func searchRebuildStatusResponseViewFromProto(result *adminpb.SearchRebuildStatusResponse) *searchRebuildStatusResponseView {
	if result == nil {
		return nil
	}
	return &searchRebuildStatusResponseView{
		Success: result.GetSuccess(),
		Message: result.GetMessage(),
		Status:  searchRebuildStatusViewFromProto(result.GetStatus()),
	}
}

func searchRebuildStatusViewFromProto(status *adminpb.SearchRebuildStatus) *searchRebuildStatusView {
	if status == nil {
		return nil
	}
	view := &searchRebuildStatusView{
		JobID:          status.GetJobId(),
		State:          status.GetState(),
		ArticleTotal:   status.GetArticleTotal(),
		ArticleIndexed: status.GetArticleIndexed(),
		TopicTotal:     status.GetTopicTotal(),
		TopicIndexed:   status.GetTopicIndexed(),
		UserTotal:      status.GetUserTotal(),
		UserIndexed:    status.GetUserIndexed(),
		StartedAt:      status.GetStartedAt(),
		CompletedAt:    status.GetCompletedAt(),
		UpdatedAt:      status.GetUpdatedAt(),
		Error:          status.GetError(),
	}
	if requestedBy := status.GetRequestedBy(); requestedBy != 0 {
		view.RequestedBy = strconv.FormatInt(requestedBy, 10)
	}
	return view
}
