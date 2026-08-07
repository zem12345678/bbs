package http

import (
	"net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userFollowingChartRequest struct {
	Span   string `json:"span"`
	Limit  *int32 `json:"limit"`
	Offset *int64 `json:"offset"`
	UserID string `json:"userId"`
}

func (h *Handler) userFollowingChart(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.UserFollowingCharts == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "user following chart service unavailable"))
		return
	}
	request, ok := bindUserFollowingChartRequest(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserFollowingCharts.GetUserFollowingChart(ctx, request)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, userFollowingChartPayload(result))
}

func bindUserFollowingChartRequest(c *gin.Context) (*userpb.UserFollowingChartRequest, bool) {
	req := userFollowingChartRequest{}
	if c.Request.Method == http.MethodGet {
		req.Span = c.Query("span")
		req.UserID = c.Query("userId")
		if raw, exists := c.GetQuery("limit"); exists {
			value, err := strconv.ParseInt(raw, 10, 32)
			if err != nil {
				writeError(c, http.StatusBadRequest, "invalid limit", "bad_request")
				return nil, false
			}
			limit := int32(value)
			req.Limit = &limit
		}
		if raw, exists := c.GetQuery("offset"); exists {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				writeError(c, http.StatusBadRequest, "invalid offset", "bad_request")
				return nil, false
			}
			req.Offset = &value
		}
	} else if !bindJSON(c, &req) {
		return nil, false
	}

	span := strings.TrimSpace(req.Span)
	if span != "day" && span != "hour" {
		writeError(c, http.StatusBadRequest, "invalid span", "bad_request")
		return nil, false
	}
	limit := defaultUserChartLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > maxUserChartLimit {
		writeError(c, http.StatusBadRequest, "invalid limit", "bad_request")
		return nil, false
	}
	if req.Offset != nil && (*req.Offset < 0 || *req.Offset > maxUserChartOffset) {
		writeError(c, http.StatusBadRequest, "invalid offset", "bad_request")
		return nil, false
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(req.UserID), 10, 64)
	if err != nil || userID <= 0 {
		writeError(c, http.StatusBadRequest, "invalid userId", "bad_request")
		return nil, false
	}
	return &userpb.UserFollowingChartRequest{
		Span: span, Limit: limit, Offset: req.Offset, UserId: userID,
	}, true
}

func userFollowingChartPayload(result *userpb.UserFollowingChartResponse) gin.H {
	if result == nil {
		result = &userpb.UserFollowingChartResponse{}
	}
	return gin.H{
		"local":  userFollowingChartScopePayload(result.GetLocal()),
		"remote": userFollowingChartScopePayload(result.GetRemote()),
	}
}

func userFollowingChartScopePayload(scope *userpb.UserFollowingChartScope) gin.H {
	if scope == nil {
		scope = &userpb.UserFollowingChartScope{}
	}
	return gin.H{
		"followings": userChartSeriesPayload(scope.GetFollowings()),
		"followers":  userChartSeriesPayload(scope.GetFollowers()),
	}
}
