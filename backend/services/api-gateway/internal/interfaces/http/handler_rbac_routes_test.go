package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestAdminRoutesRejectMissingPermissionsBeforeBusinessRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{}
	mallClient := &fakeRouteRBACMallClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "dashboard read", method: http.MethodGet, path: "/api/v1/admin/overview"},
		{name: "governance read", method: http.MethodGet, path: "/api/v1/admin/reports"},
		{name: "governance write", method: http.MethodPost, path: "/api/v1/admin/reports/1/audit", body: `{}`},
		{name: "mall read", method: http.MethodGet, path: "/api/v1/admin/mall/refunds"},
		{name: "mall category update", method: http.MethodPut, path: "/api/v1/admin/mall/categories/1", body: `{}`},
		{name: "mall product update", method: http.MethodPut, path: "/api/v1/admin/mall/products/1", body: `{}`},
		{name: "mall digital entitlements read", method: http.MethodGet, path: "/api/v1/admin/mall/digital-entitlements"},
		{name: "mall digital entitlement revoke", method: http.MethodPost, path: "/api/v1/admin/mall/digital-entitlements/501/revoke", body: `{"reason":"manual review"}`},
		{name: "mall coupon update", method: http.MethodPut, path: "/api/v1/admin/mall/coupons/1", body: `{}`},
		{name: "mall write", method: http.MethodPost, path: "/api/v1/admin/mall/refunds/1/review", body: `{}`},
		{name: "mall close expired", method: http.MethodPost, path: "/api/v1/admin/mall/orders/expire", body: `{}`},
		{name: "mall recover paying", method: http.MethodPost, path: "/api/v1/admin/mall/orders/recover-paying", body: `{}`},
		{name: "mall order logs", method: http.MethodGet, path: "/api/v1/admin/mall/orders/1/logs"},
		{name: "mall order payments", method: http.MethodGet, path: "/api/v1/admin/mall/orders/1/payments"},
		{name: "mall requeue outbox", method: http.MethodPost, path: "/api/v1/admin/mall/outbox/requeue", body: `{}`},
		{name: "mall list outbox requeue audits", method: http.MethodGet, path: "/api/v1/admin/mall/outbox/requeue-audits"},
		{name: "rbac read", method: http.MethodGet, path: "/api/v1/admin/rbac/users"},
		{name: "rbac write", method: http.MethodPost, path: "/api/v1/admin/rbac/users", body: `{}`},
		{name: "system read", method: http.MethodGet, path: "/api/v1/admin/system/users"},
		{name: "system write", method: http.MethodPost, path: "/api/v1/admin/system/users", body: `{}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer low-privilege-admin-token")
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		})
	}

	require.Equal(t, len(cases), adminClient.profileCalls)
	require.Zero(t, adminClient.listUsersCalls)
	require.Zero(t, adminClient.listReportsCalls)
	require.Zero(t, adminClient.auditReportCalls)
	require.Zero(t, adminClient.listAdminUsersCalls)
	require.Zero(t, adminClient.createAdminUserCalls)
	require.Zero(t, adminClient.listSystemUsersCalls)
	require.Zero(t, adminClient.createSystemUserCalls)
	require.Zero(t, mallClient.listRefundsCalls)
	require.Zero(t, mallClient.adminListEntitlementsCalls)
	require.Zero(t, mallClient.adminRevokeEntitlementCalls)
	require.Zero(t, mallClient.reviewRefundCalls)
	require.Zero(t, mallClient.recoverPayingCalls)
	require.Zero(t, mallClient.requeueOutboxCalls)
	require.Zero(t, mallClient.listOutboxRequeueAuditsCalls)
}

func TestAdminMallDigitalEntitlementsRequiresDedicatedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{permissions: []string{"mall:list_orders"}}
	mallClient := &fakeRouteRBACMallClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/mall/digital-entitlements?status=ACTIVE&limit=5", nil)
	req.Header.Set("Authorization", "Bearer order-only-admin-token")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Zero(t, mallClient.adminListEntitlementsCalls)

	adminClient.permissions = []string{"mall:list_digital_entitlements"}
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/mall/digital-entitlements?status=ACTIVE&limit=5", nil)
	req.Header.Set("Authorization", "Bearer entitlement-admin-token")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, mallClient.adminListEntitlementsCalls)
	require.NotNil(t, mallClient.adminListEntitlementsReq)
	require.Equal(t, "ACTIVE", mallClient.adminListEntitlementsReq.GetStatus())
	require.Equal(t, int32(5), mallClient.adminListEntitlementsReq.GetLimit())
}

func TestAdminMallDigitalEntitlementRevokeRequiresDedicatedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{permissions: []string{"mall:list_digital_entitlements"}}
	mallClient := &fakeRouteRBACMallClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/mall/digital-entitlements/501/revoke", strings.NewReader(`{"reason":"manual review"}`))
	req.Header.Set("Authorization", "Bearer entitlement-viewer-token")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Zero(t, mallClient.adminRevokeEntitlementCalls)

	adminClient.permissions = []string{"mall:revoke_digital_entitlement"}
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/mall/digital-entitlements/501/revoke", strings.NewReader(`{"reason":"manual review"}`))
	req.Header.Set("Authorization", "Bearer entitlement-operator-token")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, mallClient.adminRevokeEntitlementCalls)
	require.NotNil(t, mallClient.adminRevokeEntitlementReq)
	require.Equal(t, int64(501), mallClient.adminRevokeEntitlementReq.GetId())
	require.Equal(t, "manual review", mallClient.adminRevokeEntitlementReq.GetReason())
	require.NotEmpty(t, mallClient.adminRevokeEntitlementReq.GetOperatorId())
}

func TestAdminRouteAllowsMatchingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{permissions: []string{"governance:list_reports"}}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reports", nil)
	req.Header.Set("Authorization", "Bearer permitted-admin-token")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, adminClient.listReportsCalls)
}

func TestAdminOverviewAllowsDashboardPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{permissions: []string{"system:view_dashboard"}}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer permitted-admin-token")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, adminClient.listUsersCalls)
}

func TestRecoverStalePayingOrdersRequiresDedicatedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{permissions: []string{"mall:recover_paying_orders"}}
	mallClient := &fakeRouteRBACMallClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/mall/orders/recover-paying", strings.NewReader(`{"stale_after_seconds":900,"limit":7}`))
	req.Header.Set("Authorization", "Bearer permitted-admin-token")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, mallClient.recoverPayingCalls)
	require.Equal(t, int64(900), mallClient.recoverPayingReq.GetStaleAfterSeconds())
	require.Equal(t, int32(7), mallClient.recoverPayingReq.GetLimit())

	var envelope struct {
		Data struct {
			Recovered int64 `json:"recovered"`
			Failed    int64 `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(2), envelope.Data.Recovered)
	require.Equal(t, int64(1), envelope.Data.Failed)
}

func TestRequeueMallOutboxRequiresDedicatedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{permissions: []string{"mall:requeue_outbox_events"}}
	mallClient := &fakeRouteRBACMallClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/mall/outbox/requeue", strings.NewReader(`{"statuses":["failed","dead_letter"],"limit":9}`))
	req.Header.Set("Authorization", "Bearer permitted-admin-token")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, mallClient.requeueOutboxCalls)
	require.Equal(t, []string{"failed", "dead_letter"}, mallClient.requeueOutboxReq.GetStatuses())
	require.Equal(t, int32(9), mallClient.requeueOutboxReq.GetLimit())
	require.NotEmpty(t, mallClient.requeueOutboxReq.GetOperatorId())
	require.Equal(t, 1, adminClient.recordOperationLogCalls)
	require.Equal(t, "/api/v1/admin/mall/outbox/requeue", adminClient.lastOperationLog.GetMethod())
	require.Equal(t, http.MethodPost, adminClient.lastOperationLog.GetRequestMethod())
	require.Contains(t, adminClient.lastOperationLog.GetParams(), `"statuses":["failed","dead_letter"]`)

	var envelope struct {
		Data struct {
			Requeued int64    `json:"requeued"`
			EventIDs []string `json:"event_ids"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(3), envelope.Data.Requeued)
	require.Equal(t, []string{"evt-1", "evt-2", "evt-3"}, envelope.Data.EventIDs)
}

func TestListOutboxRequeueAuditsRequiresDedicatedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{permissions: []string{"mall:requeue_outbox_events"}}
	mallClient := &fakeRouteRBACMallClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/mall/outbox/requeue-audits?limit=5&offset=10&event_id=evt-7&aggregate_type=order&aggregate_id=9001", nil)
	req.Header.Set("Authorization", "Bearer permitted-admin-token")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, mallClient.listOutboxRequeueAuditsCalls)
	require.Equal(t, int32(5), mallClient.listOutboxRequeueAuditsReq.GetLimit())
	require.Equal(t, int32(10), mallClient.listOutboxRequeueAuditsReq.GetOffset())
	require.Equal(t, "evt-7", mallClient.listOutboxRequeueAuditsReq.GetEventId())
	require.Equal(t, "order", mallClient.listOutboxRequeueAuditsReq.GetAggregateType())
	require.Equal(t, int64(9001), mallClient.listOutboxRequeueAuditsReq.GetAggregateId())

	var envelope struct {
		Data struct {
			Items []struct {
				EventID        string `json:"event_id"`
				PreviousError  string `json:"previous_error"`
				PreviousStatus string `json:"previous_status"`
				OperatorID     string `json:"operator_id"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "evt-7", envelope.Data.Items[0].EventID)
	require.Equal(t, "publisher down", envelope.Data.Items[0].PreviousError)
	require.Equal(t, "dead_letter", envelope.Data.Items[0].PreviousStatus)
	require.Equal(t, "42", envelope.Data.Items[0].OperatorID)
}

func TestAdminAuthMenusProjectsCurrentRouteMenus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeRouteRBACAdminClient{
		currentMenus: []*adminpb.SystemMenuInfo{
			{Id: 135, Name: "mall", Title: "商城管理", Type: "M", Path: "/mall", Visible: "0", IsHide: "0"},
			{Id: 140, ParentId: 135, Name: "mall.orders", Title: "订单管理", Type: "C", Path: "/mall/orders", Component: "mall/orders/index", Permission: "mall:list_orders", Visible: "0", IsHide: "0"},
			{Id: 141, ParentId: 140, Name: "mall.orders.query", Title: "查询", Type: "F", Permission: "mall:list_orders", Visible: "1", IsHide: "1"},
			{Id: 148, ParentId: 140, Name: "mall.orders.close-expired", Title: "关闭超时", Type: "F", Permission: "mall:close_expired_orders", Visible: "1", IsHide: "1"},
			{Id: 164, ParentId: 135, Name: "mall.overview", Title: "商城概览", Type: "C", Path: "/mall/overview", Component: "mall/overview/index", Permission: "mall:list_orders", Visible: "0", IsHide: "0"},
			{Id: 165, ParentId: 135, Name: "mall.entitlements", Title: "权益台账", Type: "C", Path: "/mall/entitlements", Component: "mall/entitlements/index", Permission: "mall:list_digital_entitlements", Visible: "0", IsHide: "0"},
		},
	}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/menus", nil)
	req.Header.Set("Authorization", "Bearer mall-order-viewer-token")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, adminClient.profileCalls)
	require.Equal(t, 1, adminClient.listCurrentSystemMenusCalls)

	var envelope struct {
		Data struct {
			Items     []*systemMenuNode `json:"items"`
			FlatItems []*systemMenuNode `json:"flat_items"`
			Total     int               `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, 4, envelope.Data.Total)
	require.Len(t, envelope.Data.FlatItems, 4)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "mall", envelope.Data.Items[0].Name)
	require.Len(t, envelope.Data.Items[0].Children, 3)
	require.ElementsMatch(t, []string{"mall.orders", "mall.overview", "mall.entitlements"}, []string{
		envelope.Data.Items[0].Children[0].Name,
		envelope.Data.Items[0].Children[1].Name,
		envelope.Data.Items[0].Children[2].Name,
	})
	for _, item := range envelope.Data.FlatItems {
		require.NotEqual(t, "F", item.Type)
		require.NotContains(t, item.Name, "close-expired")
	}
}

type fakeRouteRBACAdminClient struct {
	adminpb.AdminServiceClient
	permissions                 []string
	currentMenus                []*adminpb.SystemMenuInfo
	profileCalls                int
	listUsersCalls              int
	listReportsCalls            int
	auditReportCalls            int
	listAdminUsersCalls         int
	createAdminUserCalls        int
	listSystemUsersCalls        int
	createSystemUserCalls       int
	listCurrentSystemMenusCalls int
	recordOperationLogCalls     int
	lastOperationLog            *adminpb.RecordOperationLogRequest
}

func (f *fakeRouteRBACAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	f.profileCalls++
	return &adminpb.ProfileResponse{User: &adminpb.AdminUserInfo{Id: 2, Username: "operator"}, Permissions: f.permissions}, nil
}

func (f *fakeRouteRBACAdminClient) ListUsers(context.Context, *adminpb.ListUsersRequest, ...grpc.CallOption) (*adminpb.UserListResponse, error) {
	f.listUsersCalls++
	return &adminpb.UserListResponse{}, nil
}

func (f *fakeRouteRBACAdminClient) ListArticles(context.Context, *adminpb.ListArticlesRequest, ...grpc.CallOption) (*adminpb.ArticleListResponse, error) {
	return &adminpb.ArticleListResponse{}, nil
}

func (f *fakeRouteRBACAdminClient) ListTopics(context.Context, *adminpb.ListTopicsRequest, ...grpc.CallOption) (*adminpb.TopicListResponse, error) {
	return &adminpb.TopicListResponse{}, nil
}

func (f *fakeRouteRBACAdminClient) ListComments(context.Context, *adminpb.ListCommentsRequest, ...grpc.CallOption) (*adminpb.CommentListResponse, error) {
	return &adminpb.CommentListResponse{}, nil
}

func (f *fakeRouteRBACAdminClient) ListReports(context.Context, *adminpb.ListReportsRequest, ...grpc.CallOption) (*adminpb.ReportListResponse, error) {
	f.listReportsCalls++
	return &adminpb.ReportListResponse{}, nil
}

func (f *fakeRouteRBACAdminClient) ListLoginLogs(context.Context, *adminpb.ListLoginLogsRequest, ...grpc.CallOption) (*adminpb.LoginLogListResponse, error) {
	return &adminpb.LoginLogListResponse{}, nil
}

func (f *fakeRouteRBACAdminClient) ListOperationLogs(context.Context, *adminpb.ListOperationLogsRequest, ...grpc.CallOption) (*adminpb.OperationLogListResponse, error) {
	return &adminpb.OperationLogListResponse{}, nil
}

func (f *fakeRouteRBACAdminClient) AuditReport(context.Context, *adminpb.AuditReportRequest, ...grpc.CallOption) (*adminpb.ReportResponse, error) {
	f.auditReportCalls++
	return nil, nil
}

func (f *fakeRouteRBACAdminClient) ListAdminUsers(context.Context, *adminpb.ListAdminUsersRequest, ...grpc.CallOption) (*adminpb.AdminUserListResponse, error) {
	f.listAdminUsersCalls++
	return nil, nil
}

func (f *fakeRouteRBACAdminClient) CreateAdminUser(context.Context, *adminpb.CreateAdminUserRequest, ...grpc.CallOption) (*adminpb.AdminUserResponse, error) {
	f.createAdminUserCalls++
	return nil, nil
}

func (f *fakeRouteRBACAdminClient) ListSystemUsers(context.Context, *adminpb.ListSystemUsersRequest, ...grpc.CallOption) (*adminpb.SystemUserListResponse, error) {
	f.listSystemUsersCalls++
	return nil, nil
}

func (f *fakeRouteRBACAdminClient) CreateSystemUser(context.Context, *adminpb.UpsertSystemUserRequest, ...grpc.CallOption) (*adminpb.SystemUserResponse, error) {
	f.createSystemUserCalls++
	return nil, nil
}

func (f *fakeRouteRBACAdminClient) ListCurrentSystemMenus(context.Context, *adminpb.CurrentSystemMenusRequest, ...grpc.CallOption) (*adminpb.SystemMenuListResponse, error) {
	f.listCurrentSystemMenusCalls++
	return &adminpb.SystemMenuListResponse{Items: f.currentMenus, Total: int64(len(f.currentMenus))}, nil
}

func (f *fakeRouteRBACAdminClient) RecordOperationLog(_ context.Context, req *adminpb.RecordOperationLogRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	f.recordOperationLogCalls++
	f.lastOperationLog = req
	return &adminpb.SimpleResponse{}, nil
}

type fakeRouteRBACMallClient struct {
	mallpb.MallServiceClient
	listRefundsCalls             int
	adminListEntitlementsCalls   int
	adminListEntitlementsReq     *mallpb.AdminListDigitalEntitlementsRequest
	adminRevokeEntitlementCalls  int
	adminRevokeEntitlementReq    *mallpb.AdminRevokeDigitalEntitlementRequest
	reviewRefundCalls            int
	recoverPayingCalls           int
	recoverPayingReq             *mallpb.RecoverStalePayingOrdersRequest
	requeueOutboxCalls           int
	requeueOutboxReq             *mallpb.AdminRequeueOutboxEventsRequest
	listOutboxRequeueAuditsCalls int
	listOutboxRequeueAuditsReq   *mallpb.AdminListOutboxRequeueAuditsRequest
}

func (f *fakeRouteRBACMallClient) AdminListRefundRequests(context.Context, *mallpb.AdminListRefundRequestsRequest, ...grpc.CallOption) (*mallpb.ListRefundRequestsResponse, error) {
	f.listRefundsCalls++
	return nil, nil
}

func (f *fakeRouteRBACMallClient) AdminListDigitalEntitlements(_ context.Context, req *mallpb.AdminListDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	f.adminListEntitlementsCalls++
	f.adminListEntitlementsReq = req
	return &mallpb.ListDigitalEntitlementsResponse{Items: []*mallpb.DigitalEntitlement{{Id: 501, Status: "ACTIVE"}}, Total: 1}, nil
}

func (f *fakeRouteRBACMallClient) AdminRevokeDigitalEntitlement(_ context.Context, req *mallpb.AdminRevokeDigitalEntitlementRequest, _ ...grpc.CallOption) (*mallpb.DigitalEntitlementResponse, error) {
	f.adminRevokeEntitlementCalls++
	f.adminRevokeEntitlementReq = req
	return &mallpb.DigitalEntitlementResponse{Entitlement: &mallpb.DigitalEntitlement{Id: req.GetId(), Status: "REVOKED", RevokedBy: req.GetOperatorId(), RevokeReason: req.GetReason()}}, nil
}

func (f *fakeRouteRBACMallClient) AdminReviewRefundRequest(context.Context, *mallpb.AdminReviewRefundRequestRequest, ...grpc.CallOption) (*mallpb.RefundRequestResponse, error) {
	f.reviewRefundCalls++
	return nil, nil
}

func (f *fakeRouteRBACMallClient) RecoverStalePayingOrders(_ context.Context, req *mallpb.RecoverStalePayingOrdersRequest, _ ...grpc.CallOption) (*mallpb.RecoverStalePayingOrdersResponse, error) {
	f.recoverPayingCalls++
	f.recoverPayingReq = req
	return &mallpb.RecoverStalePayingOrdersResponse{Recovered: 2, Failed: 1}, nil
}

func (f *fakeRouteRBACMallClient) AdminRequeueOutboxEvents(_ context.Context, req *mallpb.AdminRequeueOutboxEventsRequest, _ ...grpc.CallOption) (*mallpb.AdminRequeueOutboxEventsResponse, error) {
	f.requeueOutboxCalls++
	f.requeueOutboxReq = req
	return &mallpb.AdminRequeueOutboxEventsResponse{Requeued: 3, EventIds: []string{"evt-1", "evt-2", "evt-3"}}, nil
}

func (f *fakeRouteRBACMallClient) AdminListOutboxRequeueAudits(_ context.Context, req *mallpb.AdminListOutboxRequeueAuditsRequest, _ ...grpc.CallOption) (*mallpb.AdminListOutboxRequeueAuditsResponse, error) {
	f.listOutboxRequeueAuditsCalls++
	f.listOutboxRequeueAuditsReq = req
	return &mallpb.AdminListOutboxRequeueAuditsResponse{
		Items: []*mallpb.OutboxRequeueAudit{
			{
				Id:               7,
				EventId:          "evt-7",
				AggregateType:    "order",
				AggregateId:      9001,
				PreviousStatus:   "dead_letter",
				PreviousAttempts: 5,
				PreviousError:    "publisher down",
				OperatorId:       "42",
				RequeuedAt:       1700000000000,
			},
		},
		Total: 1,
	}, nil
}
