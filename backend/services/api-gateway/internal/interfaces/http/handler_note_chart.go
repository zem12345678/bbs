package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultNoteChartLimit int32 = 30
	maxNoteChartLimit     int32 = 500
	maxNoteChartOffset    int64 = 8_640_000_000_000_000
)

type noteChartRequest struct {
	Span   string `json:"span"`
	Limit  *int32 `json:"limit"`
	Offset *int64 `json:"offset"`
	UserID string `json:"userId"`
}

type noteChartParams struct {
	Span   string
	Limit  int32
	Offset *int64
	UserID int64
}

type noteChartDiffs struct {
	Normal   []int64
	Reply    []int64
	Renote   []int64
	WithFile []int64
}

type noteChartSeries struct {
	Total []int64
	Inc   []int64
	Dec   []int64
	Diffs noteChartDiffs
}

type noteChartResult struct {
	Local  noteChartSeries
	Remote noteChartSeries
}

type noteChartSeriesArrays struct {
	Total    []int64
	Inc      []int64
	Dec      []int64
	Normal   []int64
	Reply    []int64
	Renote   []int64
	WithFile []int64
}

func (h *Handler) notesChart(c *gin.Context) {
	h.handleNoteChart(c, false)
}

func (h *Handler) userNotesChart(c *gin.Context) {
	h.handleNoteChart(c, true)
}

func (h *Handler) handleNoteChart(c *gin.Context, userChart bool) {
	if h == nil || h.clients == nil || h.clients.Content == nil || h.clients.Comment == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "note chart service unavailable"))
		return
	}
	params, ok := bindNoteChartRequest(c, userChart)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.getNoteChart(ctx, params)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if userChart {
		c.JSON(http.StatusOK, noteChartSeriesPayload(result.Local))
		return
	}
	c.JSON(http.StatusOK, noteChartPayload(result))
}

func bindNoteChartRequest(c *gin.Context, userChart bool) (noteChartParams, bool) {
	req := noteChartRequest{}
	if c.Request.Method == http.MethodGet {
		req.Span = c.Query("span")
		req.UserID = c.Query("userId")
		if raw, exists := c.GetQuery("limit"); exists {
			value, err := strconv.ParseInt(raw, 10, 32)
			if err != nil {
				writeError(c, http.StatusBadRequest, "invalid limit", "bad_request")
				return noteChartParams{}, false
			}
			limit := int32(value)
			req.Limit = &limit
		}
		if raw, exists := c.GetQuery("offset"); exists {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				writeError(c, http.StatusBadRequest, "invalid offset", "bad_request")
				return noteChartParams{}, false
			}
			req.Offset = &value
		}
	} else if !bindJSON(c, &req) {
		return noteChartParams{}, false
	}

	span := strings.TrimSpace(req.Span)
	if span != "day" && span != "hour" {
		writeError(c, http.StatusBadRequest, "invalid span", "bad_request")
		return noteChartParams{}, false
	}
	limit := defaultNoteChartLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > maxNoteChartLimit {
		writeError(c, http.StatusBadRequest, "invalid limit", "bad_request")
		return noteChartParams{}, false
	}
	if req.Offset != nil && (*req.Offset < 0 || *req.Offset > maxNoteChartOffset) {
		writeError(c, http.StatusBadRequest, "invalid offset", "bad_request")
		return noteChartParams{}, false
	}

	userID := int64(0)
	if userChart {
		var err error
		userID, err = strconv.ParseInt(strings.TrimSpace(req.UserID), 10, 64)
		if err != nil || userID <= 0 {
			writeError(c, http.StatusBadRequest, "invalid userId", "bad_request")
			return noteChartParams{}, false
		}
	}
	return noteChartParams{Span: span, Limit: limit, Offset: req.Offset, UserID: userID}, true
}

func (h *Handler) getNoteChart(ctx context.Context, params noteChartParams) (noteChartResult, error) {
	contentRequest := &contentpb.NoteChartRequest{Span: params.Span, Limit: params.Limit, Offset: params.Offset, UserId: params.UserID}
	commentRequest := &commentpb.NoteChartRequest{Span: params.Span, Limit: params.Limit, Offset: params.Offset, UserId: params.UserID}

	var contentResult *contentpb.NoteChartResponse
	var commentResult *commentpb.NoteChartResponse
	var contentErr, commentErr error
	var calls sync.WaitGroup
	calls.Add(2)
	go func() {
		defer calls.Done()
		contentResult, contentErr = h.clients.Content.GetNoteChart(ctx, contentRequest)
	}()
	go func() {
		defer calls.Done()
		commentResult, commentErr = h.clients.Comment.GetNoteChart(ctx, commentRequest)
	}()
	calls.Wait()
	if contentErr != nil {
		return noteChartResult{}, contentErr
	}
	if commentErr != nil {
		return noteChartResult{}, commentErr
	}

	local, err := mergeNoteChartSeries(
		int(params.Limit), "local",
		contentNoteChartArrays(contentResult.GetLocal()),
		commentNoteChartArrays(commentResult.GetLocal()),
	)
	if err != nil {
		return noteChartResult{}, err
	}
	remote, err := mergeNoteChartSeries(
		int(params.Limit), "remote",
		contentNoteChartArrays(contentResult.GetRemote()),
		commentNoteChartArrays(commentResult.GetRemote()),
	)
	if err != nil {
		return noteChartResult{}, err
	}
	return noteChartResult{Local: local, Remote: remote}, nil
}

func contentNoteChartArrays(series *contentpb.NoteChartSeries) noteChartSeriesArrays {
	return noteChartSeriesArrays{
		Total: series.GetTotal(), Inc: series.GetInc(), Dec: series.GetDec(),
		Normal: series.GetDiffs().GetNormal(), Reply: series.GetDiffs().GetReply(),
		Renote: series.GetDiffs().GetRenote(), WithFile: series.GetDiffs().GetWithFile(),
	}
}

func commentNoteChartArrays(series *commentpb.NoteChartSeries) noteChartSeriesArrays {
	return noteChartSeriesArrays{
		Total: series.GetTotal(), Inc: series.GetInc(), Dec: series.GetDec(),
		Normal: series.GetDiffs().GetNormal(), Reply: series.GetDiffs().GetReply(),
		Renote: series.GetDiffs().GetRenote(), WithFile: series.GetDiffs().GetWithFile(),
	}
}

func mergeNoteChartSeries(limit int, scope string, content, comments noteChartSeriesArrays) (noteChartSeries, error) {
	if !noteChartArraysHaveLength(content, limit) || !noteChartArraysHaveLength(comments, limit) {
		return noteChartSeries{}, status.Errorf(codes.DataLoss, "invalid %s note chart response", scope)
	}
	return noteChartSeries{
		Total: addNoteChartArrays(content.Total, comments.Total),
		Inc:   addNoteChartArrays(content.Inc, comments.Inc),
		Dec:   addNoteChartArrays(content.Dec, comments.Dec),
		Diffs: noteChartDiffs{
			Normal:   addNoteChartArrays(content.Normal, comments.Normal),
			Reply:    addNoteChartArrays(content.Reply, comments.Reply),
			Renote:   addNoteChartArrays(content.Renote, comments.Renote),
			WithFile: addNoteChartArrays(content.WithFile, comments.WithFile),
		},
	}, nil
}

func noteChartArraysHaveLength(series noteChartSeriesArrays, length int) bool {
	return len(series.Total) == length && len(series.Inc) == length && len(series.Dec) == length &&
		len(series.Normal) == length && len(series.Reply) == length && len(series.Renote) == length && len(series.WithFile) == length
}

func addNoteChartArrays(left, right []int64) []int64 {
	result := make([]int64, len(left))
	for index := range left {
		result[index] = left[index] + right[index]
	}
	return result
}

func noteChartPayload(result noteChartResult) gin.H {
	return gin.H{
		"local":  noteChartSeriesPayload(result.Local),
		"remote": noteChartSeriesPayload(result.Remote),
	}
}

func noteChartSeriesPayload(series noteChartSeries) gin.H {
	return gin.H{
		"total": nonNilInt64s(series.Total),
		"inc":   nonNilInt64s(series.Inc),
		"dec":   nonNilInt64s(series.Dec),
		"diffs": gin.H{
			"normal":   nonNilInt64s(series.Diffs.Normal),
			"reply":    nonNilInt64s(series.Diffs.Reply),
			"renote":   nonNilInt64s(series.Diffs.Renote),
			"withFile": nonNilInt64s(series.Diffs.WithFile),
		},
	}
}
