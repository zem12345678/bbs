package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNotesChartGETAggregatesContentAndComments(t *testing.T) {
	content := &noteChartContentClient{response: contentNoteChartResponse(
		[]int64{2, 3}, []int64{1, 1}, []int64{0, 0}, []int64{1, 1}, []int64{0, 0},
	)}
	comments := &noteChartCommentClient{response: commentNoteChartResponse(
		[]int64{4, 5}, []int64{0, 1}, []int64{0, 0}, []int64{0, 0}, []int64{0, 1},
	)}
	router := noteChartTestRouter(content, comments)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/notes?span=day&limit=2&offset=0", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, content.requests, 1)
	require.Len(t, comments.requests, 1)
	require.Equal(t, int32(2), content.requests[0].GetLimit())
	require.Equal(t, int64(0), content.requests[0].GetOffset())
	require.Zero(t, content.requests[0].GetUserId())
	require.JSONEq(t, `{
		"local":{"total":[6,8],"inc":[1,2],"dec":[0,0],"diffs":{"normal":[1,1],"reply":[0,1],"renote":[0,0],"withFile":[0,0]}},
		"remote":{"total":[0,0],"inc":[0,0],"dec":[0,0],"diffs":{"normal":[0,0],"reply":[0,0],"renote":[0,0],"withFile":[0,0]}}
	}`, recorder.Body.String())
}

func TestUserNotesChartPOSTMapsUserAndReturnsLocalSeries(t *testing.T) {
	content := &noteChartContentClient{response: contentNoteChartResponse(
		[]int64{2}, []int64{1}, []int64{0}, []int64{1}, []int64{0},
	)}
	comments := &noteChartCommentClient{response: commentNoteChartResponse(
		[]int64{3}, []int64{1}, []int64{0}, []int64{0}, []int64{1},
	)}
	router := noteChartTestRouter(content, comments)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/charts/user/notes", strings.NewReader(`{"span":"hour","limit":1,"offset":0,"userId":"9223372036854775807"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(9223372036854775807), content.requests[0].GetUserId())
	require.Equal(t, int64(9223372036854775807), comments.requests[0].GetUserId())
	require.JSONEq(t, `{"total":[5],"inc":[2],"dec":[0],"diffs":{"normal":[1],"reply":[1],"renote":[0],"withFile":[0]}}`, recorder.Body.String())
}

func TestNotesChartsRejectInvalidParameters(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "missing span", method: stdhttp.MethodGet, path: "/charts/notes"},
		{name: "unknown span", method: stdhttp.MethodGet, path: "/charts/notes?span=week"},
		{name: "invalid limit", method: stdhttp.MethodGet, path: "/charts/notes?span=day&limit=nope"},
		{name: "zero limit", method: stdhttp.MethodGet, path: "/charts/notes?span=day&limit=0"},
		{name: "large limit", method: stdhttp.MethodGet, path: "/charts/notes?span=day&limit=501"},
		{name: "negative offset", method: stdhttp.MethodGet, path: "/charts/notes?span=day&offset=-1"},
		{name: "large offset", method: stdhttp.MethodGet, path: "/charts/notes?span=day&offset=8640000000000001"},
		{name: "invalid offset", method: stdhttp.MethodGet, path: "/charts/notes?span=day&offset=nope"},
		{name: "missing user", method: stdhttp.MethodGet, path: "/charts/user/notes?span=day"},
		{name: "zero user", method: stdhttp.MethodGet, path: "/charts/user/notes?span=day&userId=0"},
		{name: "negative user", method: stdhttp.MethodGet, path: "/charts/user/notes?span=day&userId=-1"},
		{name: "invalid user", method: stdhttp.MethodGet, path: "/charts/user/notes?span=day&userId=user"},
		{name: "overflow user", method: stdhttp.MethodGet, path: "/charts/user/notes?span=day&userId=9223372036854775808"},
		{name: "invalid json", method: stdhttp.MethodPost, path: "/charts/notes", body: `{"span":`},
		{name: "numeric json user", method: stdhttp.MethodPost, path: "/charts/user/notes", body: `{"span":"day","userId":42}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := &noteChartContentClient{}
			comments := &noteChartCommentClient{}
			router := noteChartTestRouter(content, comments)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.method == stdhttp.MethodPost {
				request.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Empty(t, content.requests)
			require.Empty(t, comments.requests)
		})
	}
}

func TestNotesChartMapsUpstreamErrorsAndRejectsWrongLengths(t *testing.T) {
	t.Run("upstream error", func(t *testing.T) {
		content := &noteChartContentClient{err: status.Error(codes.Unavailable, "content unavailable")}
		comments := &noteChartCommentClient{}
		recorder := httptest.NewRecorder()
		noteChartTestRouter(content, comments).ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/notes?span=day&limit=1", nil))
		require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	})

	t.Run("wrong length", func(t *testing.T) {
		content := &noteChartContentClient{response: contentNoteChartResponse(nil, nil, nil, nil, nil)}
		comments := &noteChartCommentClient{response: commentNoteChartResponse([]int64{0}, []int64{0}, []int64{0}, []int64{0}, []int64{0})}
		recorder := httptest.NewRecorder()
		noteChartTestRouter(content, comments).ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/notes?span=day&limit=1", nil))
		require.Equal(t, stdhttp.StatusBadGateway, recorder.Code, recorder.Body.String())
	})
}

func noteChartTestRouter(content contentpb.ContentServiceClient, comments commentpb.CommentServiceClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{Content: content, Comment: comments}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type noteChartContentClient struct {
	contentpb.ContentServiceClient
	requests []*contentpb.NoteChartRequest
	response *contentpb.NoteChartResponse
	err      error
}

func (f *noteChartContentClient) GetNoteChart(_ context.Context, request *contentpb.NoteChartRequest, _ ...grpc.CallOption) (*contentpb.NoteChartResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

type noteChartCommentClient struct {
	commentpb.CommentServiceClient
	requests []*commentpb.NoteChartRequest
	response *commentpb.NoteChartResponse
	err      error
}

func (f *noteChartCommentClient) GetNoteChart(_ context.Context, request *commentpb.NoteChartRequest, _ ...grpc.CallOption) (*commentpb.NoteChartResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

func contentNoteChartResponse(total, inc, dec, normal, reply []int64) *contentpb.NoteChartResponse {
	return &contentpb.NoteChartResponse{
		Local: &contentpb.NoteChartSeries{Total: total, Inc: inc, Dec: dec, Diffs: &contentpb.NoteChartDiffs{
			Normal: normal, Reply: reply, Renote: make([]int64, len(total)), WithFile: make([]int64, len(total)),
		}},
		Remote: &contentpb.NoteChartSeries{Total: make([]int64, len(total)), Inc: make([]int64, len(total)), Dec: make([]int64, len(total)), Diffs: &contentpb.NoteChartDiffs{
			Normal: make([]int64, len(total)), Reply: make([]int64, len(total)), Renote: make([]int64, len(total)), WithFile: make([]int64, len(total)),
		}},
	}
}

func commentNoteChartResponse(total, inc, dec, normal, reply []int64) *commentpb.NoteChartResponse {
	return &commentpb.NoteChartResponse{
		Local: &commentpb.NoteChartSeries{Total: total, Inc: inc, Dec: dec, Diffs: &commentpb.NoteChartDiffs{
			Normal: normal, Reply: reply, Renote: make([]int64, len(total)), WithFile: make([]int64, len(total)),
		}},
		Remote: &commentpb.NoteChartSeries{Total: make([]int64, len(total)), Inc: make([]int64, len(total)), Dec: make([]int64, len(total)), Diffs: &commentpb.NoteChartDiffs{
			Normal: make([]int64, len(total)), Reply: make([]int64, len(total)), Renote: make([]int64, len(total)), WithFile: make([]int64, len(total)),
		}},
	}
}
