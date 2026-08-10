package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestAdminChannelListMapsGovernanceFiltersAndActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{permissions: []string{"governance:list_channels"}}
	router := channelGovernanceTestRouter(adminClient)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/channels?q=%20release%20&category_id=17&archived_status=2&limit=5&offset=10", nil)
	request.Header.Set("Authorization", "Bearer channel-reader-token")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, adminClient.listChannelsCalls)
	require.Equal(t, int64(42), adminClient.listChannelsRequest.GetActor().GetId())
	require.Equal(t, "operator", adminClient.listChannelsRequest.GetActor().GetUsername())
	require.Equal(t, "release", adminClient.listChannelsRequest.GetQuery())
	require.Equal(t, int64(17), adminClient.listChannelsRequest.GetCategoryId())
	require.Equal(t, int32(2), adminClient.listChannelsRequest.GetArchivedStatus())
	require.Equal(t, int32(5), adminClient.listChannelsRequest.GetLimit())
	require.Equal(t, int32(10), adminClient.listChannelsRequest.GetOffset())
}

func TestAdminChannelListRequiresPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{}
	router := channelGovernanceTestRouter(adminClient)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/channels", nil)
	request.Header.Set("Authorization", "Bearer channel-reader-token")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Zero(t, adminClient.listChannelsCalls)
}

func TestAdminChannelListValidatesFiltersBeforeAdminRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{permissions: []string{"governance:list_channels"}}
	router := channelGovernanceTestRouter(adminClient)

	cases := []string{
		"q=" + strings.Repeat("x", 101),
		"category_id=-1",
		"category_id=invalid",
		"category_id=",
		"archived_status=-1",
		"archived_status=3",
		"archived_status=invalid",
		"archived_status=",
		"limit=0",
		"limit=101",
		"limit=invalid",
		"limit=",
		"offset=-1",
		"offset=invalid",
		"offset=",
	}
	for _, query := range cases {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/channels?"+query, nil)
		request.Header.Set("Authorization", "Bearer channel-reader-token")
		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusBadRequest, recorder.Code, query)
	}
	require.Zero(t, adminClient.listChannelsCalls)
}

func TestAdminChannelListAcceptsArchivedStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{permissions: []string{"governance:list_channels"}}
	router := channelGovernanceTestRouter(adminClient)

	for _, archivedStatus := range []string{"0", "1", "2"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/channels?archived_status="+archivedStatus, nil)
		request.Header.Set("Authorization", "Bearer channel-reader-token")
		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, archivedStatus, string(rune('0'+adminClient.listChannelsRequest.GetArchivedStatus())))
	}
	require.Equal(t, 3, adminClient.listChannelsCalls)
}

func TestAdminChannelFeaturedRequiresPermissionAndCallsAdminClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{}
	router := channelGovernanceTestRouter(adminClient)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/channels/101/featured", strings.NewReader(`{"featured":true}`))
	request.Header.Set("Authorization", "Bearer channel-reader-token")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Zero(t, adminClient.setFeaturedCalls)

	adminClient.permissions = []string{"governance:feature_channel"}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/channels/101/featured", strings.NewReader(`{"featured":true}`))
	request.Header.Set("Authorization", "Bearer channel-featured-token")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, adminClient.setFeaturedCalls)
	require.Equal(t, int64(101), adminClient.setFeaturedRequest.GetId())
	require.True(t, adminClient.setFeaturedRequest.GetFeatured())
	require.Equal(t, int64(42), adminClient.setFeaturedRequest.GetActor().GetId())
}

func TestAdminChannelArchivedSelectsArchiveOrRestorePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{}
	router := channelGovernanceTestRouter(adminClient)

	cases := []struct {
		name       string
		body       string
		permission string
		archived   bool
	}{
		{name: "archive", body: `{"archived":true}`, permission: "governance:archive_channel", archived: true},
		{name: "restore", body: `{"archived":false}`, permission: "governance:restore_channel", archived: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			oppositePermission := "governance:archive_channel"
			if tt.archived {
				oppositePermission = "governance:restore_channel"
			}
			callsBefore := adminClient.setArchivedCalls
			adminClient.permissions = []string{oppositePermission}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/channels/202/archived", strings.NewReader(tt.body))
			request.Header.Set("Authorization", "Bearer wrong-channel-archive-token")
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			require.Equal(t, callsBefore, adminClient.setArchivedCalls)

			adminClient.permissions = []string{tt.permission}
			recorder = httptest.NewRecorder()
			request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/channels/202/archived", strings.NewReader(tt.body))
			request.Header.Set("Authorization", "Bearer channel-archive-token")
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, int64(202), adminClient.setArchivedRequest.GetId())
			require.Equal(t, tt.archived, adminClient.setArchivedRequest.GetArchived())
			require.Equal(t, int64(42), adminClient.setArchivedRequest.GetActor().GetId())
		})
	}
	require.Equal(t, 2, adminClient.setArchivedCalls)
}

func TestAdminChannelMutationsValidateIDAndBooleanBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{permissions: []string{
		"governance:feature_channel",
		"governance:archive_channel",
		"governance:restore_channel",
	}}
	router := channelGovernanceTestRouter(adminClient)

	cases := []struct {
		path string
		body string
	}{
		{path: "/api/v1/admin/channels/invalid/featured", body: `{"featured":true}`},
		{path: "/api/v1/admin/channels/1/featured", body: `{}`},
		{path: "/api/v1/admin/channels/1/featured", body: `{"featured":"true"}`},
		{path: "/api/v1/admin/channels/invalid/archived", body: `{"archived":true}`},
		{path: "/api/v1/admin/channels/1/archived", body: `{}`},
		{path: "/api/v1/admin/channels/1/archived", body: `{"archived":"true"}`},
	}
	for _, tt := range cases {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, tt.path, strings.NewReader(tt.body))
		request.Header.Set("Authorization", "Bearer channel-writer-token")
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusBadRequest, recorder.Code, tt.path+" "+tt.body)
	}
	require.Zero(t, adminClient.setFeaturedCalls)
	require.Zero(t, adminClient.setArchivedCalls)
}

func TestAdminChannelRoutesRejectMissingAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &channelGovernanceAdminClient{}
	router := channelGovernanceTestRouter(adminClient)

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/admin/channels"},
		{method: http.MethodPut, path: "/api/v1/admin/channels/1/featured"},
		{method: http.MethodPut, path: "/api/v1/admin/channels/1/archived"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(route.method, route.path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, route.path)
	}
	require.Zero(t, adminClient.profileCalls)
}

type channelGovernanceAdminClient struct {
	adminpb.AdminServiceClient
	permissions         []string
	profileCalls        int
	listChannelsCalls   int
	listChannelsRequest *adminpb.ListChannelsRequest
	setFeaturedCalls    int
	setFeaturedRequest  *adminpb.ChannelFeaturedRequest
	setArchivedCalls    int
	setArchivedRequest  *adminpb.ChannelArchivedRequest
}

func channelGovernanceTestRouter(adminClient adminpb.AdminServiceClient) *gin.Engine {
	router := gin.New()
	NewInitControllers(NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret))(router)
	return router
}

func (c *channelGovernanceAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	c.profileCalls++
	return &adminpb.ProfileResponse{
		User:        &adminpb.AdminUserInfo{Id: 42, Username: "operator"},
		Permissions: c.permissions,
	}, nil
}

func (c *channelGovernanceAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}

func (c *channelGovernanceAdminClient) ListChannels(_ context.Context, request *adminpb.ListChannelsRequest, _ ...grpc.CallOption) (*adminpb.ChannelListResponse, error) {
	c.listChannelsCalls++
	c.listChannelsRequest = request
	return &adminpb.ChannelListResponse{Items: []*adminpb.ChannelInfo{{Id: 101, Name: "Release"}}, Total: 1}, nil
}

func (c *channelGovernanceAdminClient) SetChannelFeatured(_ context.Context, request *adminpb.ChannelFeaturedRequest, _ ...grpc.CallOption) (*adminpb.ChannelResponse, error) {
	c.setFeaturedCalls++
	c.setFeaturedRequest = request
	return &adminpb.ChannelResponse{Success: true, Channel: &adminpb.ChannelInfo{Id: request.GetId(), IsFeatured: request.GetFeatured()}}, nil
}

func (c *channelGovernanceAdminClient) SetChannelArchived(_ context.Context, request *adminpb.ChannelArchivedRequest, _ ...grpc.CallOption) (*adminpb.ChannelResponse, error) {
	c.setArchivedCalls++
	c.setArchivedRequest = request
	return &adminpb.ChannelResponse{Success: true, Channel: &adminpb.ChannelInfo{Id: request.GetId(), IsArchived: request.GetArchived()}}, nil
}
