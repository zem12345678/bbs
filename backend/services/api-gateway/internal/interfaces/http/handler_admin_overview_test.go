package http

import (
	"context"
	"errors"
	"testing"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestBuildAdminOverviewDegradesFailedSource(t *testing.T) {
	h := NewHandler(&clients.Clients{Admin: &failedOverviewAdminClient{}}, "Authorization", "Bearer", testJWTSecret)
	payload, err := h.buildAdminOverview(context.Background(), &adminpb.Actor{})

	require.NoError(t, err)
	require.Len(t, payload["metrics"], 4)
	require.NotNil(t, payload["chart"])
	require.NotNil(t, payload["progress"])
	require.NotNil(t, payload["daily"])
	require.NotNil(t, payload["latest"])
	require.Equal(t, []string{"users"}, payload["degraded_sources"])
	require.EqualValues(t, 0, payload["metrics"].([]gin.H)[0]["value"])
}

func TestBuildAdminOverviewPrefersUserChartOverAdminSample(t *testing.T) {
	chart := &overviewUserChartClient{response: &userpb.UserChartResponse{Local: &userpb.UserChartSeries{
		Inc: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
	}}}
	h := NewHandler(&clients.Clients{
		Admin:      &sampledOverviewAdminClient{},
		UserCharts: chart,
	}, "Authorization", "Bearer", testJWTSecret)

	payload, err := h.buildAdminOverview(context.Background(), &adminpb.Actor{})

	require.NoError(t, err)
	require.Equal(t, "day", chart.request.GetSpan())
	require.Equal(t, int32(14), chart.request.GetLimit())
	require.Equal(t, []int{7, 6, 5, 4, 3, 2, 1}, payload["metrics"].([]gin.H)[0]["data"])
	require.Equal(t, 1, payload["daily"].([]gin.H)[0]["newUsers"])
	require.Empty(t, payload["degraded_sources"])
}

func TestBuildAdminOverviewFallsBackToAdminSampleWhenUserChartFails(t *testing.T) {
	h := NewHandler(&clients.Clients{
		Admin:      &sampledOverviewAdminClient{},
		UserCharts: &overviewUserChartClient{err: errors.New("chart unavailable")},
	}, "Authorization", "Bearer", testJWTSecret)

	payload, err := h.buildAdminOverview(context.Background(), &adminpb.Actor{})

	require.NoError(t, err)
	metricData := payload["metrics"].([]gin.H)[0]["data"].([]int)
	require.Equal(t, 1, metricData[len(metricData)-1])
	require.Equal(t, []string{"user_chart"}, payload["degraded_sources"])
}

func TestOverviewDayBucketsUseUTCAtTimezoneBoundary(t *testing.T) {
	labels, keys := overviewDayKeysAt(2, time.Date(2026, time.August, 7, 0, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60)))
	require.Equal(t, []string{"2026-08-05", "2026-08-06"}, labels)

	series := make([]int, 2)
	timestamp := time.Date(2026, time.August, 6, 16, 30, 0, 0, time.UTC).UnixMilli()
	bumpOverviewSeries(series, keys, timestamp)
	require.Equal(t, []int{0, 1}, series)
}

type failedOverviewAdminClient struct {
	adminpb.AdminServiceClient
}

type sampledOverviewAdminClient struct {
	failedOverviewAdminClient
}

func (sampledOverviewAdminClient) ListUsers(context.Context, *adminpb.ListUsersRequest, ...grpc.CallOption) (*adminpb.UserListResponse, error) {
	return &adminpb.UserListResponse{
		Items: []*adminpb.UserInfo{{Id: 1, CreatedAt: time.Now().UnixMilli()}},
		Total: 1,
	}, nil
}

type overviewUserChartClient struct {
	request  *userpb.UserChartRequest
	response *userpb.UserChartResponse
	err      error
}

func (f *overviewUserChartClient) GetUserChart(_ context.Context, request *userpb.UserChartRequest, _ ...grpc.CallOption) (*userpb.UserChartResponse, error) {
	f.request = request
	return f.response, f.err
}

func (failedOverviewAdminClient) ListUsers(context.Context, *adminpb.ListUsersRequest, ...grpc.CallOption) (*adminpb.UserListResponse, error) {
	return nil, errors.New("users unavailable")
}

func (failedOverviewAdminClient) ListArticles(context.Context, *adminpb.ListArticlesRequest, ...grpc.CallOption) (*adminpb.ArticleListResponse, error) {
	return &adminpb.ArticleListResponse{}, nil
}

func (failedOverviewAdminClient) ListTopics(context.Context, *adminpb.ListTopicsRequest, ...grpc.CallOption) (*adminpb.TopicListResponse, error) {
	return &adminpb.TopicListResponse{}, nil
}

func (failedOverviewAdminClient) ListComments(context.Context, *adminpb.ListCommentsRequest, ...grpc.CallOption) (*adminpb.CommentListResponse, error) {
	return &adminpb.CommentListResponse{}, nil
}

func (failedOverviewAdminClient) ListReports(context.Context, *adminpb.ListReportsRequest, ...grpc.CallOption) (*adminpb.ReportListResponse, error) {
	return &adminpb.ReportListResponse{}, nil
}

func (failedOverviewAdminClient) ListLoginLogs(context.Context, *adminpb.ListLoginLogsRequest, ...grpc.CallOption) (*adminpb.LoginLogListResponse, error) {
	return &adminpb.LoginLogListResponse{}, nil
}

func (failedOverviewAdminClient) ListOperationLogs(context.Context, *adminpb.ListOperationLogsRequest, ...grpc.CallOption) (*adminpb.OperationLogListResponse, error) {
	return &adminpb.OperationLogListResponse{}, nil
}
