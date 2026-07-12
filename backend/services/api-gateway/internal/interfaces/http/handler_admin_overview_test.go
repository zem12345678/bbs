package http

import (
	"context"
	"errors"
	"testing"

	"api-gateway/api/proto/adminpb"
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

type failedOverviewAdminClient struct {
	adminpb.AdminServiceClient
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
