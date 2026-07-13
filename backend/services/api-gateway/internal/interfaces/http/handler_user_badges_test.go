package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListUserBadgesMergesActiveBadgeEntitlements(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &fakeUserBadgesUserClient{
		user: &userpb.UserInfo{Id: 42, Username: "alice"},
	}
	adminClient := &fakeUserBadgesAdminClient{
		badges: []*adminpb.BadgeInfo{
			{
				Id:          7,
				Key:         "badge-founder",
				Name:        "创始成员",
				Description: "通过商城数字徽章获得的创始成员标识。",
				IconUrl:     "https://example.test/badge.png",
				RuleType:    "manual",
				Status:      2,
			},
		},
	}
	mallClient := &fakeUserBadgesMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{
				Id:              501,
				OrderId:         88,
				OrderNo:         "O-88",
				ProductId:       1001,
				Sku:             "BADGE-FOUNDER",
				Title:           "创始成员数字徽章",
				FulfillmentCode: "BBS-BADGE-501",
				GrantType:       "badge",
				GrantKey:        "badge-founder",
				Status:          "ACTIVE",
				IssuedAt:        1783848000000,
			},
		},
	}
	h := NewHandler(&clients.Clients{User: userClient, Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42/badges?limit=10&offset=0", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), userClient.req.GetId())
	require.Equal(t, int32(2), adminClient.req.GetStatus())
	require.Equal(t, int64(42), mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Equal(t, int32(100), mallClient.req.GetLimit())

	var envelope struct {
		Data struct {
			Items []struct {
				ID              string `json:"id"`
				Name            string `json:"name"`
				Description     string `json:"description"`
				IconURL         string `json:"icon_url"`
				AwardedAt       int64  `json:"awarded_at"`
				Source          string `json:"source"`
				EntitlementID   int64  `json:"entitlement_id"`
				OrderNo         string `json:"order_no"`
				FulfillmentCode string `json:"fulfillment_code"`
				GrantType       string `json:"grant_type"`
				GrantKey        string `json:"grant_key"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "badge-founder", envelope.Data.Items[0].ID)
	require.Equal(t, "创始成员", envelope.Data.Items[0].Name)
	require.Equal(t, "通过商城数字徽章获得的创始成员标识。", envelope.Data.Items[0].Description)
	require.Equal(t, "https://example.test/badge.png", envelope.Data.Items[0].IconURL)
	require.Equal(t, int64(1783848000000), envelope.Data.Items[0].AwardedAt)
	require.Equal(t, "digital_entitlement", envelope.Data.Items[0].Source)
	require.Equal(t, int64(501), envelope.Data.Items[0].EntitlementID)
	require.Equal(t, "O-88", envelope.Data.Items[0].OrderNo)
	require.Equal(t, "BBS-BADGE-501", envelope.Data.Items[0].FulfillmentCode)
	require.Equal(t, "badge", envelope.Data.Items[0].GrantType)
	require.Equal(t, "badge-founder", envelope.Data.Items[0].GrantKey)
}

func TestListUserBadgesKeepsRuleBadgesWhenMallEntitlementsFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &fakeUserBadgesUserClient{
		user: &userpb.UserInfo{Id: 42, Username: "alice", CreatedAt: 1783848000000},
	}
	adminClient := &fakeUserBadgesAdminClient{
		badges: []*adminpb.BadgeInfo{
			{
				Id:          1,
				Key:         "community-member",
				Name:        "社区成员",
				Description: "已创建社区账号并加入讨论。",
				RuleType:    "account_created",
				Status:      2,
			},
		},
	}
	mallClient := &fakeUserBadgesMallClient{err: errors.New("mall unavailable")}
	h := NewHandler(&clients.Clients{User: userClient, Admin: adminClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42/badges", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Items []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "community-member", envelope.Data.Items[0].ID)
	require.Equal(t, "社区成员", envelope.Data.Items[0].Name)
}

type fakeUserBadgesUserClient struct {
	userpb.UserServiceClient
	user *userpb.UserInfo
	req  *userpb.UserIDRequest
}

func (f *fakeUserBadgesUserClient) GetUser(_ context.Context, req *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	f.req = req
	return &userpb.UserResponse{User: f.user}, nil
}

type fakeUserBadgesAdminClient struct {
	adminpb.AdminServiceClient
	badges []*adminpb.BadgeInfo
	req    *adminpb.ListBadgesRequest
}

func (f *fakeUserBadgesAdminClient) ListBadges(_ context.Context, req *adminpb.ListBadgesRequest, _ ...grpc.CallOption) (*adminpb.BadgeListResponse, error) {
	f.req = req
	return &adminpb.BadgeListResponse{Items: f.badges, Total: int64(len(f.badges))}, nil
}

type fakeUserBadgesMallClient struct {
	mallpb.MallServiceClient
	entitlements []*mallpb.DigitalEntitlement
	err          error
	req          *mallpb.ListUserDigitalEntitlementsRequest
}

func (f *fakeUserBadgesMallClient) ListUserDigitalEntitlements(_ context.Context, req *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	f.req = req
	if f.err != nil {
		return nil, f.err
	}
	return &mallpb.ListDigitalEntitlementsResponse{Items: f.entitlements, Total: int64(len(f.entitlements))}, nil
}
