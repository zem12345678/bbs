package http

import (
	"net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/filepb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultDriveChartLimit int32 = 30
	maxDriveChartLimit     int32 = 500
	maxDriveChartOffset    int64 = 8_640_000_000_000_000
)

type driveChartRequest struct {
	Span   string `json:"span"`
	Limit  *int32 `json:"limit"`
	Offset *int64 `json:"offset"`
	UserID string `json:"userId"`
}

func (h *Handler) driveChart(c *gin.Context) {
	h.handleDriveChart(c, false)
}

func (h *Handler) userDriveChart(c *gin.Context) {
	h.handleDriveChart(c, true)
}

func (h *Handler) handleDriveChart(c *gin.Context, userChart bool) {
	if h == nil || h.clients == nil || h.clients.File == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "drive chart service unavailable"))
		return
	}
	request, ok := bindDriveChartRequest(c, userChart)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.File.GetDriveChart(ctx, request)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if userChart {
		c.JSON(http.StatusOK, userDriveChartPayload(result.GetLocal()))
		return
	}
	c.JSON(http.StatusOK, driveChartPayload(result))
}

func bindDriveChartRequest(c *gin.Context, userChart bool) (*filepb.DriveChartRequest, bool) {
	req := driveChartRequest{}
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
	limit := defaultDriveChartLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > maxDriveChartLimit {
		writeError(c, http.StatusBadRequest, "invalid limit", "bad_request")
		return nil, false
	}
	if req.Offset != nil && (*req.Offset < 0 || *req.Offset > maxDriveChartOffset) {
		writeError(c, http.StatusBadRequest, "invalid offset", "bad_request")
		return nil, false
	}

	ownerID := int64(0)
	if userChart {
		var err error
		ownerID, err = strconv.ParseInt(strings.TrimSpace(req.UserID), 10, 64)
		if err != nil || ownerID <= 0 {
			writeError(c, http.StatusBadRequest, "invalid userId", "bad_request")
			return nil, false
		}
	}

	return &filepb.DriveChartRequest{Span: span, Limit: limit, Offset: req.Offset, OwnerId: ownerID}, true
}

func driveChartPayload(result *filepb.DriveChartResponse) gin.H {
	if result == nil {
		result = &filepb.DriveChartResponse{}
	}
	return gin.H{
		"local":  driveChartDeltaPayload(result.GetLocal()),
		"remote": driveChartDeltaPayload(result.GetRemote()),
	}
}

func driveChartDeltaPayload(series *filepb.DriveChartSeries) gin.H {
	if series == nil {
		series = &filepb.DriveChartSeries{}
	}
	return gin.H{
		"incCount": nonNilInt64s(series.GetIncCount()),
		"incSize":  nonNilFloat64s(series.GetIncSize()),
		"decCount": nonNilInt64s(series.GetDecCount()),
		"decSize":  nonNilFloat64s(series.GetDecSize()),
	}
}

func userDriveChartPayload(series *filepb.DriveChartSeries) gin.H {
	if series == nil {
		series = &filepb.DriveChartSeries{}
	}
	return gin.H{
		"totalCount": nonNilInt64s(series.GetTotalCount()),
		"totalSize":  nonNilFloat64s(series.GetTotalSize()),
		"incCount":   nonNilInt64s(series.GetIncCount()),
		"incSize":    nonNilFloat64s(series.GetIncSize()),
		"decCount":   nonNilInt64s(series.GetDecCount()),
		"decSize":    nonNilFloat64s(series.GetDecSize()),
	}
}

func nonNilFloat64s(values []float64) []float64 {
	if values == nil {
		return []float64{}
	}
	return values
}
