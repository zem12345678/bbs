package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/internal/clients"
	"api-gateway/internal/clients/pb/adminpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func TestUpdateSettingForwardsClearValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeUpdateSettingAdminClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "key", Value: "auth.github.client_secret"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/settings/auth.github.client_secret",
		strings.NewReader(`{"key":"auth.github.client_secret","value":"","value_type":"password","group":"auth","status":2,"clear_value":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateSetting(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if adminClient.req == nil || !adminClient.req.GetClearValue() {
		t.Fatalf("expected clear_value to be forwarded, got %+v", adminClient.req)
	}
}

type fakeUpdateSettingAdminClient struct {
	adminpb.AdminServiceClient
	req *adminpb.UpsertSettingRequest
}

func (f *fakeUpdateSettingAdminClient) UpdateSetting(_ context.Context, req *adminpb.UpsertSettingRequest, _ ...grpc.CallOption) (*adminpb.SettingResponse, error) {
	f.req = req
	return &adminpb.SettingResponse{
		Success: true,
		Message: "ok",
		Setting: &adminpb.SettingInfo{
			Key:       req.GetKey(),
			Value:     req.GetValue(),
			Group:     req.GetGroup(),
			ValueType: req.GetValueType(),
			Status:    req.GetStatus(),
		},
	}, nil
}
