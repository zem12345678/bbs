package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetAdminUserFileCapacityMapsUsageAfterUserValidation(t *testing.T) {
	events := make([]string, 0, 2)
	userClient := &fakeAdminFileCapacityUserClient{
		response: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42}},
		events:   &events,
	}
	fileClient := &fakeAdminFileCapacityFileClient{
		getResponse: &filepb.FileUsageResponse{
			UsedBytes:             12_345,
			CapacityBytes:         2_048 * bytesPerMiB,
			FileCount:             7,
			PolicyCapacityBytes:   1_024 * bytesPerMiB,
			MaxFileSizeBytes:      50 * bytesPerMiB,
			OverrideCapacityBytes: 2_048 * bytesPerMiB,
			HasOverride:           true,
		},
		events: &events,
	}
	router := newAdminFileCapacityTestRouter(userClient, fileClient, "governance:list_user_file_capacity")

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/users/42/file-capacity", nil)
	req.Header.Set("Authorization", "Bearer capacity-reader")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []string{"get_user", "get_usage"}, events)
	require.EqualValues(t, 42, userClient.request.GetId())
	require.EqualValues(t, 42, fileClient.getRequest.GetOwnerId())
	data := decodeAdminFileCapacityData(t, recorder)
	require.Equal(t, int64(12_345), data.UsedBytes)
	require.Equal(t, int64(7), data.FileCount)
	require.Equal(t, int64(1_024), data.PolicyCapacityMB)
	require.Equal(t, int64(50), data.MaxFileSizeMB)
	require.NotNil(t, data.OverrideMB)
	require.Equal(t, int64(2_048), *data.OverrideMB)
	require.Equal(t, int64(2_048), data.EffectiveCapacityMB)
}

func TestUpdateAdminUserFileCapacitySavesOverrideAndReturnsLatestUsage(t *testing.T) {
	events := make([]string, 0, 2)
	userClient := &fakeAdminFileCapacityUserClient{
		response: &userpb.UserResponse{User: &userpb.UserInfo{Id: 84}},
		events:   &events,
	}
	fileClient := &fakeAdminFileCapacityFileClient{
		setResponse: &filepb.FileUsageResponse{
			UsedBytes:             90,
			CapacityBytes:         4_096 * bytesPerMiB,
			FileCount:             3,
			PolicyCapacityBytes:   1_024 * bytesPerMiB,
			MaxFileSizeBytes:      50 * bytesPerMiB,
			OverrideCapacityBytes: 4_096 * bytesPerMiB,
			HasOverride:           true,
		},
		events: &events,
	}
	router := newAdminFileCapacityTestRouter(
		userClient,
		fileClient,
		"governance:list_user_file_capacity",
		"governance:update_user_file_capacity",
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/users/84/file-capacity", strings.NewReader(`{"override_mb":4096}`))
	req.Header.Set("Authorization", "Bearer capacity-writer")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []string{"get_user", "set_capacity"}, events)
	require.EqualValues(t, 84, fileClient.setRequest.GetOwnerId())
	require.Equal(t, int64(4_096*bytesPerMiB), fileClient.setRequest.GetOverrideCapacityBytes())
	require.False(t, fileClient.setRequest.GetClearOverride())
	data := decodeAdminFileCapacityData(t, recorder)
	require.Equal(t, int64(90), data.UsedBytes)
	require.Equal(t, int64(3), data.FileCount)
	require.NotNil(t, data.OverrideMB)
	require.Equal(t, int64(4_096), *data.OverrideMB)
	require.Equal(t, int64(4_096), data.EffectiveCapacityMB)
}

func TestUpdateAdminUserFileCapacityClearsOverrideWithNull(t *testing.T) {
	userClient := &fakeAdminFileCapacityUserClient{
		response: &userpb.UserResponse{User: &userpb.UserInfo{Id: 84}},
	}
	fileClient := &fakeAdminFileCapacityFileClient{
		setResponse: &filepb.FileUsageResponse{
			CapacityBytes:       1_024 * bytesPerMiB,
			PolicyCapacityBytes: 1_024 * bytesPerMiB,
			MaxFileSizeBytes:    50 * bytesPerMiB,
		},
	}
	router := newAdminFileCapacityTestRouter(
		userClient,
		fileClient,
		"governance:list_user_file_capacity",
		"governance:update_user_file_capacity",
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/users/84/file-capacity", strings.NewReader(`{"override_mb":null}`))
	req.Header.Set("Authorization", "Bearer capacity-writer")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, fileClient.setRequest)
	require.Zero(t, fileClient.setRequest.GetOverrideCapacityBytes())
	require.True(t, fileClient.setRequest.GetClearOverride())
	data := decodeAdminFileCapacityData(t, recorder)
	require.Nil(t, data.OverrideMB)
	require.Equal(t, int64(1_024), data.EffectiveCapacityMB)
}

func TestUpdateAdminUserFileCapacityRejectsInvalidOverride(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing", body: `{}`},
		{name: "negative", body: `{"override_mb":-1}`},
		{name: "above maximum", body: `{"override_mb":10485761}`},
		{name: "not an integer", body: `{"override_mb":1.5}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userClient := &fakeAdminFileCapacityUserClient{
				response: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42}},
			}
			fileClient := &fakeAdminFileCapacityFileClient{}
			router := newAdminFileCapacityTestRouter(
				userClient,
				fileClient,
				"governance:list_user_file_capacity",
				"governance:update_user_file_capacity",
			)

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/users/42/file-capacity", strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer capacity-writer")
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)

			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Nil(t, userClient.request)
			require.Nil(t, fileClient.setRequest)
		})
	}
}

func TestGetAdminUserFileCapacityPropagatesUserNotFound(t *testing.T) {
	userClient := &fakeAdminFileCapacityUserClient{err: status.Error(codes.NotFound, "user not found")}
	fileClient := &fakeAdminFileCapacityFileClient{}
	router := newAdminFileCapacityTestRouter(userClient, fileClient, "governance:list_user_file_capacity")

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/users/404/file-capacity", nil)
	req.Header.Set("Authorization", "Bearer capacity-reader")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusNotFound, recorder.Code, recorder.Body.String())
	require.EqualValues(t, 404, userClient.request.GetId())
	require.Nil(t, fileClient.getRequest)
}

func TestGetAdminUserFileCapacityRejectsEmptyFileServiceResponse(t *testing.T) {
	userClient := &fakeAdminFileCapacityUserClient{
		response: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42}},
	}
	router := newAdminFileCapacityTestRouter(
		userClient,
		&fakeAdminFileCapacityFileClient{},
		"governance:list_user_file_capacity",
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/users/42/file-capacity", nil)
	req.Header.Set("Authorization", "Bearer capacity-reader")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadGateway, recorder.Code, recorder.Body.String())
}

func TestUpdateAdminUserFileCapacityRejectsEmptyFileServiceResponse(t *testing.T) {
	userClient := &fakeAdminFileCapacityUserClient{
		response: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42}},
	}
	router := newAdminFileCapacityTestRouter(
		userClient,
		&fakeAdminFileCapacityFileClient{},
		"governance:list_user_file_capacity",
		"governance:update_user_file_capacity",
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/users/42/file-capacity", strings.NewReader(`{"override_mb":2048}`))
	req.Header.Set("Authorization", "Bearer capacity-writer")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusBadGateway, recorder.Code, recorder.Body.String())
}

func TestUpdateAdminUserFileCapacityRequiresListPermission(t *testing.T) {
	userClient := &fakeAdminFileCapacityUserClient{}
	fileClient := &fakeAdminFileCapacityFileClient{}
	router := newAdminFileCapacityTestRouter(
		userClient,
		fileClient,
		"governance:update_user_file_capacity",
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/users/42/file-capacity", strings.NewReader(`{"override_mb":2048}`))
	req.Header.Set("Authorization", "Bearer capacity-writer")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Nil(t, userClient.request)
	require.Nil(t, fileClient.setRequest)
}

type adminFileCapacityData struct {
	UsedBytes           int64  `json:"used_bytes"`
	FileCount           int64  `json:"file_count"`
	PolicyCapacityMB    int64  `json:"policy_capacity_mb"`
	MaxFileSizeMB       int64  `json:"max_file_size_mb"`
	OverrideMB          *int64 `json:"override_mb"`
	EffectiveCapacityMB int64  `json:"effective_capacity_mb"`
}

func decodeAdminFileCapacityData(t *testing.T, recorder *httptest.ResponseRecorder) adminFileCapacityData {
	t.Helper()
	var envelope struct {
		Data adminFileCapacityData `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope.Data
}

func newAdminFileCapacityTestRouter(user clients.UserClient, file filepb.FileServiceClient, permissions ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{
		Admin: &fakeAdminFileCapacityAdminClient{permissions: permissions},
		User:  user,
		File:  file,
	}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type fakeAdminFileCapacityAdminClient struct {
	adminpb.AdminServiceClient
	permissions []string
}

func (f *fakeAdminFileCapacityAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{
		User:        &adminpb.AdminUserInfo{Id: 2, Username: "capacity-admin"},
		Permissions: f.permissions,
	}, nil
}

func (f *fakeAdminFileCapacityAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}

type fakeAdminFileCapacityUserClient struct {
	userpb.UserServiceClient
	request  *userpb.UserIDRequest
	response *userpb.UserResponse
	err      error
	events   *[]string
}

func (f *fakeAdminFileCapacityUserClient) GetUser(_ context.Context, request *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	f.request = request
	if f.events != nil {
		*f.events = append(*f.events, "get_user")
	}
	return f.response, f.err
}

type fakeAdminFileCapacityFileClient struct {
	filepb.FileServiceClient
	getRequest  *filepb.GetFileUsageRequest
	getResponse *filepb.FileUsageResponse
	getErr      error
	setRequest  *filepb.SetFileCapacityRequest
	setResponse *filepb.FileUsageResponse
	setErr      error
	events      *[]string
}

func (f *fakeAdminFileCapacityFileClient) GetFileUsage(_ context.Context, request *filepb.GetFileUsageRequest, _ ...grpc.CallOption) (*filepb.FileUsageResponse, error) {
	f.getRequest = request
	if f.events != nil {
		*f.events = append(*f.events, "get_usage")
	}
	return f.getResponse, f.getErr
}

func (f *fakeAdminFileCapacityFileClient) SetFileCapacity(_ context.Context, request *filepb.SetFileCapacityRequest, _ ...grpc.CallOption) (*filepb.FileUsageResponse, error) {
	f.setRequest = request
	if f.events != nil {
		*f.events = append(*f.events, "set_capacity")
	}
	return f.setResponse, f.setErr
}
