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

const (
	defaultUserChartLimit int32 = 30
	maxUserChartLimit     int32 = 500
	maxUserChartOffset    int64 = 8_640_000_000_000_000
)

type userChartRequest struct {
	Span   string `json:"span"`
	Limit  *int32 `json:"limit"`
	Offset *int64 `json:"offset"`
}

func (h *Handler) userChart(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.UserCharts == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "user chart service unavailable"))
		return
	}
	request, ok := bindUserChartRequest(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserCharts.GetUserChart(ctx, request)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, userChartPayload(result))
}

func bindUserChartRequest(c *gin.Context) (*userpb.UserChartRequest, bool) {
	req := userChartRequest{}
	if c.Request.Method == http.MethodGet {
		req.Span = c.Query("span")
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
	return &userpb.UserChartRequest{Span: span, Limit: limit, Offset: req.Offset}, true
}

func userChartPayload(result *userpb.UserChartResponse) gin.H {
	if result == nil {
		result = &userpb.UserChartResponse{}
	}
	return gin.H{
		"local":  userChartSeriesPayload(result.GetLocal()),
		"remote": userChartSeriesPayload(result.GetRemote()),
	}
}

func userChartSeriesPayload(series *userpb.UserChartSeries) gin.H {
	if series == nil {
		series = &userpb.UserChartSeries{}
	}
	return gin.H{
		"total": nonNilInt64s(series.GetTotal()),
		"inc":   nonNilInt64s(series.GetInc()),
		"dec":   nonNilInt64s(series.GetDec()),
	}
}

func nonNilInt64s(values []int64) []int64 {
	if values == nil {
		return []int64{}
	}
	return values
}
