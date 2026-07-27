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
	"google.golang.org/grpc"
)

func TestSendSystemNotificationRouteRequiresPermissionAndAudits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeSystemNotificationAdminClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications/system", strings.NewReader(`{"recipient_ids":["339000000000000011","339000000000000012"],"title":"维护通知","content":"今晚维护","idempotency_key":"system-notification-1"}`))
	req.Header.Set("Authorization", "Bearer low-privilege-admin-token")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if len(adminClient.sent) != 0 {
		t.Fatalf("SendSystemNotification calls = %d, want 0", len(adminClient.sent))
	}

	adminClient.permissions = []string{"system:send_system_notification"}
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications/system", strings.NewReader(`{"recipient_ids":["339000000000000011","339000000000000012"],"title":"维护通知","content":"今晚维护","idempotency_key":"system-notification-1"}`))
	req.Header.Set("Authorization", "Bearer notification-admin-token")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(adminClient.sent) != 1 {
		t.Fatalf("SendSystemNotification calls = %d, want 1", len(adminClient.sent))
	}
	sent := adminClient.sent[0]
	if sent.GetActor().GetId() != 2 || sent.GetActor().GetUsername() != "operator" {
		t.Fatalf("actor = %+v, want current admin", sent.GetActor())
	}
	if got := sent.GetRecipientIds(); len(got) != 2 || got[0] != 339000000000000011 || got[1] != 339000000000000012 {
		t.Fatalf("recipient_ids = %v, want exact Snowflake IDs", got)
	}
	if sent.GetTitle() != "维护通知" || sent.GetContent() != "今晚维护" || sent.GetIdempotencyKey() != "system-notification-1" {
		t.Fatalf("forwarded request = %+v", sent)
	}
	if len(adminClient.operationLogs) != 2 {
		t.Fatalf("operation log calls = %d, want 2", len(adminClient.operationLogs))
	}
	log := adminClient.operationLogs[1]
	if log.GetMethod() != "/api/v1/admin/notifications/system" || log.GetStatus() != 1 {
		t.Fatalf("operation log = %+v, want successful notification audit", log)
	}
}

type fakeSystemNotificationAdminClient struct {
	adminpb.AdminServiceClient
	permissions   []string
	sent          []*adminpb.SendSystemNotificationRequest
	operationLogs []*adminpb.RecordOperationLogRequest
}

func (f *fakeSystemNotificationAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{
		User:        &adminpb.AdminUserInfo{Id: 2, Username: "operator"},
		Permissions: f.permissions,
	}, nil
}

func (f *fakeSystemNotificationAdminClient) SendSystemNotification(_ context.Context, req *adminpb.SendSystemNotificationRequest, _ ...grpc.CallOption) (*adminpb.SendSystemNotificationResponse, error) {
	f.sent = append(f.sent, req)
	return &adminpb.SendSystemNotificationResponse{Success: true, Message: "ok", DeliveredCount: int32(len(req.GetRecipientIds()))}, nil
}

func (f *fakeSystemNotificationAdminClient) RecordOperationLog(_ context.Context, req *adminpb.RecordOperationLogRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	f.operationLogs = append(f.operationLogs, req)
	return &adminpb.SimpleResponse{Success: true}, nil
}
