package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestActiveUsersChartGETDeduplicatesUsersAcrossServices(t *testing.T) {
	users := &activeUsersChartUserClient{response: &userpb.ActiveUsersChartResponse{Buckets: []*userpb.ActiveUsersChartBucket{
		{ReadUserIds: []int64{1, 1, 2}, RegisteredWithinWeek: 2, RegisteredWithinMonth: 2, RegisteredWithinYear: 2},
		{ReadUserIds: []int64{3}, RegisteredOutsideWeek: 1, RegisteredOutsideMonth: 1, RegisteredOutsideYear: 1},
	}}}
	content := &activeUsersChartContentClient{response: &contentpb.ActiveUsersChartResponse{Buckets: []*contentpb.ActiveUsersChartBucket{
		{WriterUserIds: []int64{2, 3, 3}}, {WriterUserIds: []int64{3}},
	}}}
	comments := &activeUsersChartCommentClient{response: &commentpb.ActiveUsersChartResponse{Buckets: []*commentpb.ActiveUsersChartBucket{
		{WriterUserIds: []int64{2, 3, 4}}, {WriterUserIds: []int64{4, 4}},
	}}}
	router := activeUsersChartTestRouter([]string{"governance:list_users"}, users, content, comments)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/charts/active-users?span=hour&limit=2&offset=0", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, users.requests, 1)
	require.Len(t, content.requests, 1)
	require.Len(t, comments.requests, 1)
	require.Equal(t, int32(2), users.requests[0].GetLimit())
	require.Equal(t, int64(0), users.requests[0].GetOffset())
	require.JSONEq(t, `{
		"readWrite":[1,1],"read":[2,1],"write":[3,2],
		"registeredWithinWeek":[2,0],"registeredWithinMonth":[2,0],"registeredWithinYear":[2,0],
		"registeredOutsideWeek":[0,1],"registeredOutsideMonth":[0,1],"registeredOutsideYear":[0,1]
	}`, recorder.Body.String())
}

func TestActiveUsersChartPOSTSupportsAPIV1Route(t *testing.T) {
	users := &activeUsersChartUserClient{response: activeUsersUserResponse(1)}
	content := &activeUsersChartContentClient{response: activeUsersContentResponse(1)}
	comments := &activeUsersChartCommentClient{response: activeUsersCommentResponse(1)}
	router := activeUsersChartTestRouter([]string{"governance:list_users"}, users, content, comments)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/charts/active-users", strings.NewReader(`{"span":"day","limit":1}`))
	request.Header.Set("Authorization", "Bearer admin-token")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "day", users.requests[0].GetSpan())
	require.Nil(t, users.requests[0].Offset)
	require.JSONEq(t, `{
		"readWrite":[0],"read":[0],"write":[0],
		"registeredWithinWeek":[0],"registeredWithinMonth":[0],"registeredWithinYear":[0],
		"registeredOutsideWeek":[0],"registeredOutsideMonth":[0],"registeredOutsideYear":[0]
	}`, recorder.Body.String())
}

func TestActiveUsersChartRequiresAdminPermission(t *testing.T) {
	t.Run("credential", func(t *testing.T) {
		users := &activeUsersChartUserClient{}
		router := activeUsersChartTestRouter([]string{"governance:list_users"}, users, &activeUsersChartContentClient{}, &activeUsersChartCommentClient{})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/active-users?span=day", nil))
		require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
		require.Empty(t, users.requests)
	})

	t.Run("permission", func(t *testing.T) {
		users := &activeUsersChartUserClient{}
		router := activeUsersChartTestRouter([]string{"governance:list_articles"}, users, &activeUsersChartContentClient{}, &activeUsersChartCommentClient{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/charts/active-users?span=day", nil)
		request.Header.Set("Authorization", "Bearer admin-token")
		router.ServeHTTP(recorder, request)
		require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
		require.Empty(t, users.requests)
	})
}

func TestActiveUsersChartRejectsInvalidParameters(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "missing span", method: stdhttp.MethodGet, path: "/charts/active-users"},
		{name: "invalid limit", method: stdhttp.MethodGet, path: "/charts/active-users?span=day&limit=501"},
		{name: "invalid offset", method: stdhttp.MethodGet, path: "/charts/active-users?span=day&offset=-1"},
		{name: "invalid json", method: stdhttp.MethodPost, path: "/charts/active-users", body: `{"span":`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := &activeUsersChartUserClient{}
			content := &activeUsersChartContentClient{}
			comments := &activeUsersChartCommentClient{}
			router := activeUsersChartTestRouter([]string{"governance:list_users"}, users, content, comments)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer admin-token")
			if test.method == stdhttp.MethodPost {
				request.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(recorder, request)
			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Empty(t, users.requests)
			require.Empty(t, content.requests)
			require.Empty(t, comments.requests)
		})
	}
}

func TestActiveUsersChartMapsUpstreamErrorsAndRejectsWrongLengths(t *testing.T) {
	t.Run("upstream error", func(t *testing.T) {
		users := &activeUsersChartUserClient{err: status.Error(codes.Unavailable, "users unavailable")}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodGet, "/charts/active-users?span=day&limit=1", nil)
		request.Header.Set("Authorization", "Bearer admin-token")
		activeUsersChartTestRouter([]string{"governance:list_users"}, users, &activeUsersChartContentClient{}, &activeUsersChartCommentClient{}).ServeHTTP(recorder, request)
		require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	})

	t.Run("wrong length", func(t *testing.T) {
		users := &activeUsersChartUserClient{response: activeUsersUserResponse(1)}
		content := &activeUsersChartContentClient{response: activeUsersContentResponse(0)}
		comments := &activeUsersChartCommentClient{response: activeUsersCommentResponse(1)}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodGet, "/charts/active-users?span=day&limit=1", nil)
		request.Header.Set("Authorization", "Bearer admin-token")
		activeUsersChartTestRouter([]string{"governance:list_users"}, users, content, comments).ServeHTTP(recorder, request)
		require.Equal(t, stdhttp.StatusBadGateway, recorder.Code, recorder.Body.String())
	})
}

func activeUsersChartTestRouter(permissions []string, users clients.UserActiveUsersChartClient, content contentpb.ContentServiceClient, comments commentpb.CommentServiceClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: &activeUsersChartAdminClient{permissions: permissions}, UserActiveUsersCharts: users, Content: content, Comment: comments,
	}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type activeUsersChartAdminClient struct {
	adminpb.AdminServiceClient
	permissions []string
}

func (f *activeUsersChartAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{User: &adminpb.AdminUserInfo{Id: 2, Username: "chart-admin"}, Permissions: f.permissions}, nil
}

func (f *activeUsersChartAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}

type activeUsersChartUserClient struct {
	requests []*userpb.ActiveUsersChartRequest
	response *userpb.ActiveUsersChartResponse
	err      error
}

func (f *activeUsersChartUserClient) GetActiveUsersChart(_ context.Context, request *userpb.ActiveUsersChartRequest, _ ...grpc.CallOption) (*userpb.ActiveUsersChartResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

type activeUsersChartContentClient struct {
	contentpb.ContentServiceClient
	requests []*contentpb.ActiveUsersChartRequest
	response *contentpb.ActiveUsersChartResponse
	err      error
}

func (f *activeUsersChartContentClient) GetActiveUsersChart(_ context.Context, request *contentpb.ActiveUsersChartRequest, _ ...grpc.CallOption) (*contentpb.ActiveUsersChartResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

type activeUsersChartCommentClient struct {
	commentpb.CommentServiceClient
	requests []*commentpb.ActiveUsersChartRequest
	response *commentpb.ActiveUsersChartResponse
	err      error
}

func (f *activeUsersChartCommentClient) GetActiveUsersChart(_ context.Context, request *commentpb.ActiveUsersChartRequest, _ ...grpc.CallOption) (*commentpb.ActiveUsersChartResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.err
}

func activeUsersUserResponse(length int) *userpb.ActiveUsersChartResponse {
	buckets := make([]*userpb.ActiveUsersChartBucket, length)
	for index := range buckets {
		buckets[index] = &userpb.ActiveUsersChartBucket{}
	}
	return &userpb.ActiveUsersChartResponse{Buckets: buckets}
}

func activeUsersContentResponse(length int) *contentpb.ActiveUsersChartResponse {
	buckets := make([]*contentpb.ActiveUsersChartBucket, length)
	for index := range buckets {
		buckets[index] = &contentpb.ActiveUsersChartBucket{}
	}
	return &contentpb.ActiveUsersChartResponse{Buckets: buckets}
}

func activeUsersCommentResponse(length int) *commentpb.ActiveUsersChartResponse {
	buckets := make([]*commentpb.ActiveUsersChartBucket, length)
	for index := range buckets {
		buckets[index] = &commentpb.ActiveUsersChartBucket{}
	}
	return &commentpb.ActiveUsersChartResponse{Buckets: buckets}
}
