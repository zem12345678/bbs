package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestSearchRebuildRoutesRequireDedicatedPermissionsAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeSearchRebuildAdminClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/search/rebuild", nil)
	req.Header.Set("Authorization", "Bearer low-privilege-admin-token")
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("start without permission status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if adminClient.start != nil {
		t.Fatal("StartSearchRebuild must not be called without permission")
	}

	adminClient.permissions = []string{"system:rebuild_search"}
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/search/rebuild", nil)
	req.Header.Set("Authorization", "Bearer rebuild-admin-token")
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("start status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if adminClient.start == nil || adminClient.start.GetActor().GetId() != 2 || adminClient.start.GetActor().GetUsername() != "operator" {
		t.Fatalf("start request = %#v", adminClient.start)
	}

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/search/rebuild", nil)
	req.Header.Set("Authorization", "Bearer rebuild-admin-token")
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status without view permission = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if adminClient.status != nil {
		t.Fatal("GetSearchRebuildStatus must not be called without view permission")
	}

	adminClient.permissions = []string{"system:view_search_rebuild"}
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/search/rebuild", nil)
	req.Header.Set("Authorization", "Bearer rebuild-admin-token")
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status query = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if adminClient.status == nil || adminClient.status.GetActor().GetId() != 2 {
		t.Fatalf("status request = %#v", adminClient.status)
	}
	var responseBody struct {
		Data struct {
			Status struct {
				RequestedBy string `json:"requested_by"`
			} `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
	require.Equal(t, "9223372036854770001", responseBody.Data.Status.RequestedBy)
	if len(adminClient.operationLogs) == 0 {
		t.Fatal("admin operations should be audited")
	}
	last := adminClient.operationLogs[len(adminClient.operationLogs)-1]
	if last.GetMethod() != "/api/v1/admin/search/rebuild" || last.GetStatus() != 1 {
		t.Fatalf("last audit log = %#v", last)
	}
}

type fakeSearchRebuildAdminClient struct {
	adminpb.AdminServiceClient
	permissions   []string
	start         *adminpb.SearchRebuildRequest
	status        *adminpb.SearchRebuildStatusRequest
	operationLogs []*adminpb.RecordOperationLogRequest
}

func (f *fakeSearchRebuildAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{
		User:        &adminpb.AdminUserInfo{Id: 2, Username: "operator"},
		Permissions: f.permissions,
	}, nil
}

func (f *fakeSearchRebuildAdminClient) StartSearchRebuild(_ context.Context, request *adminpb.SearchRebuildRequest, _ ...grpc.CallOption) (*adminpb.SearchRebuildStatusResponse, error) {
	f.start = request
	return &adminpb.SearchRebuildStatusResponse{Success: true, Status: &adminpb.SearchRebuildStatus{JobId: "job-1", State: "queued", RequestedBy: 9223372036854770001}}, nil
}

func (f *fakeSearchRebuildAdminClient) GetSearchRebuildStatus(_ context.Context, request *adminpb.SearchRebuildStatusRequest, _ ...grpc.CallOption) (*adminpb.SearchRebuildStatusResponse, error) {
	f.status = request
	return &adminpb.SearchRebuildStatusResponse{Success: true, Status: &adminpb.SearchRebuildStatus{JobId: "job-1", State: "running", RequestedBy: 9223372036854770001}}, nil
}

func (f *fakeSearchRebuildAdminClient) RecordOperationLog(_ context.Context, request *adminpb.RecordOperationLogRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	f.operationLogs = append(f.operationLogs, request)
	return &adminpb.SimpleResponse{Success: true}, nil
}
