package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/creditpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateTopicAllowsQABountyDraftWithoutMembership(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"slug":"qa-bounty","type":"qa","title":"如何排查支付回调？","body":"已经检查网关日志。","tags":["支付"],"category_id":3,"bounty_score":50,"publish":false}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, contentClient.createReq)
	require.Equal(t, "qa", contentClient.createReq.GetType())
	require.EqualValues(t, 50, contentClient.createReq.GetBountyScore())
	require.EqualValues(t, 42, contentClient.createReq.GetAuthorId())
	require.Nil(t, mallClient.req)
	require.Nil(t, contentClient.publishReq)
}

func TestCreateTopicPublishesQABountyWhenDirtyMembershipPrecedesValidGrant(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
		},
	}
	creditClient := &fakeTopicCreditClient{total: 100}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"slug":"qa-bounty","type":"qa","title":"如何排查支付回调？","body":"已经检查网关日志。","tags":["支付"],"category_id":3,"bounty_score":50,"publish":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.Equal(t, digitalEntitlementLookupLimit, mallClient.req.GetLimit())
	require.NotNil(t, contentClient.createReq)
	require.NotNil(t, contentClient.publishReq)
	require.EqualValues(t, 1001, contentClient.publishReq.GetId())
	require.NotNil(t, creditClient.balanceReq)
	require.EqualValues(t, 42, creditClient.balanceReq.GetUserId())
}

func TestCreateTopicRejectsQABountyPublishWithInsufficientCreditBeforeContentCreate(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
		},
	}
	creditClient := &fakeTopicCreditClient{total: 49}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"slug":"qa-bounty","type":"qa","title":"如何排查支付回调？","body":"已经检查网关日志。","tags":["支付"],"category_id":3,"bounty_score":50,"publish":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopic(c)

	require.Equal(t, http.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.NotNil(t, creditClient.balanceReq)
	require.Nil(t, contentClient.createReq)
	require.Nil(t, contentClient.publishReq)
	require.Contains(t, recorder.Body.String(), bountyCreditInsufficientMessage)
}

func TestUserHasActiveDigitalEntitlementScansMembershipPages(t *testing.T) {
	mallClient := &pagedMembershipMallClient{
		pages: map[int32][]*mallpb.DigitalEntitlement{
			0: dirtyMembershipEntitlements(20),
			20: {
				{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
			},
		},
	}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	allowed, err := h.userHasActiveDigitalEntitlement(context.Background(), 42, digitalEntitlementGrantTypeMembership, "")

	require.NoError(t, err)
	require.True(t, allowed)
	require.Len(t, mallClient.reqs, 2)
	require.Equal(t, int32(0), mallClient.reqs[0].GetOffset())
	require.Equal(t, digitalEntitlementLookupLimit, mallClient.reqs[1].GetOffset())
}

func TestUserHasActiveDigitalEntitlementScansThemePages(t *testing.T) {
	mallClient := &pagedMembershipMallClient{
		pages: map[int32][]*mallpb.DigitalEntitlement{
			0: dirtyThemeEntitlements(int(digitalEntitlementLookupLimit)),
			digitalEntitlementLookupLimit: {
				{GrantType: "theme", GrantKey: "theme-pro", Status: "ACTIVE"},
			},
		},
	}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	allowed, err := h.userHasActiveDigitalEntitlement(context.Background(), 42, "theme", "theme-pro")

	require.NoError(t, err)
	require.True(t, allowed)
	require.Len(t, mallClient.reqs, 2)
	require.Equal(t, "theme", mallClient.reqs[0].GetGrantType())
	require.Equal(t, "theme-pro", mallClient.reqs[0].GetGrantKey())
	require.Equal(t, int32(0), mallClient.reqs[0].GetOffset())
	require.Equal(t, digitalEntitlementLookupLimit, mallClient.reqs[1].GetOffset())
}

func TestCreateTopicRejectsQABountyPublishWithoutMembership(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"slug":"qa-bounty","type":"qa","title":"如何排查支付回调？","body":"已经检查网关日志。","tags":["支付"],"category_id":3,"bounty_score":50,"publish":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Equal(t, "membership", mallClient.req.GetGrantType())
	require.Equal(t, digitalEntitlementLookupLimit, mallClient.req.GetLimit())
	require.Nil(t, contentClient.createReq)
}

func TestCreateTopicRejectsQABountyAfterMembershipExpired(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(-time.Hour).UnixMilli()},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"slug":"qa-bounty","type":"qa","title":"如何排查支付回调？","body":"已经检查网关日志。","tags":["支付"],"category_id":3,"bounty_score":50,"publish":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Nil(t, contentClient.createReq)
}

func TestCreateTopicRejectsQABountyWithBlankMembershipGrantKey(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", Status: "ACTIVE"},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"slug":"qa-bounty","type":"qa","title":"如何排查支付回调？","body":"已经检查网关日志。","tags":["支付"],"category_id":3,"bounty_score":50,"publish":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Nil(t, contentClient.createReq)
}

func TestCreateTopicRejectsQABountyWithPerpetualMembership(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE"},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/topics",
		bytes.NewBufferString(`{"slug":"qa-bounty","type":"qa","title":"如何排查支付回调？","body":"已经检查网关日志。","tags":["支付"],"category_id":3,"bounty_score":50,"publish":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Nil(t, contentClient.createReq)
}

type pagedMembershipMallClient struct {
	mallpb.MallServiceClient
	pages map[int32][]*mallpb.DigitalEntitlement
	reqs  []*mallpb.ListUserDigitalEntitlementsRequest
}

func (c *pagedMembershipMallClient) ListUserDigitalEntitlements(_ context.Context, req *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	c.reqs = append(c.reqs, req)
	items := c.pages[req.GetOffset()]
	return &mallpb.ListDigitalEntitlementsResponse{Items: items, Total: int64(len(items))}, nil
}

func dirtyMembershipEntitlements(count int) []*mallpb.DigitalEntitlement {
	items := make([]*mallpb.DigitalEntitlement, 0, count)
	expiresAt := time.Now().Add(time.Hour).UnixMilli()
	for i := 0; i < count; i++ {
		items = append(items, &mallpb.DigitalEntitlement{GrantType: "membership", Status: "ACTIVE", ExpiresAt: expiresAt})
	}
	return items
}

func dirtyThemeEntitlements(count int) []*mallpb.DigitalEntitlement {
	items := make([]*mallpb.DigitalEntitlement, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, &mallpb.DigitalEntitlement{GrantType: "theme", GrantKey: "theme-pro", Status: "ACTIVE", RevokedAt: time.Now().UnixMilli()})
	}
	return items
}

func TestUpdateTopicAllowsQABountyDraftWithoutMembership(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		getTopicResp: &contentpb.TopicResponse{
			Success: true,
			Message: "ok",
			Topic: &contentpb.TopicInfo{
				Id:          1001,
				Type:        "qa",
				Title:       "如何排查支付回调？",
				AuthorId:    42,
				Status:      1,
				BountyScore: 50,
			},
		},
	}
	mallClient := &captureThemeMallClient{}
	h := NewHandler(&clients.Clients{Content: contentClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/topics/1001",
		bytes.NewBufferString(`{"title":"如何排查支付回调？","body":"补充更多上下文。","tags":["支付"],"category_id":3,"bounty_score":50}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Nil(t, mallClient.req)
	require.NotNil(t, contentClient.updateReq)
	require.EqualValues(t, 1001, contentClient.updateReq.GetId())
	require.EqualValues(t, 50, contentClient.updateReq.GetBountyScore())
}

func TestUpdateTopicAllowsQABountyWithMembership(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
		},
	}
	creditClient := &fakeTopicCreditClient{total: 100}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/topics/1001",
		bytes.NewBufferString(`{"title":"如何排查支付回调？","body":"补充更多上下文。","tags":["支付"],"category_id":3,"bounty_score":50}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.NotNil(t, contentClient.updateReq)
	require.EqualValues(t, 1001, contentClient.updateReq.GetId())
	require.EqualValues(t, 50, contentClient.updateReq.GetBountyScore())
	require.NotNil(t, creditClient.balanceReq)
}

func TestUpdateTopicRejectsQABountyWithInsufficientCreditBeforeContentUpdate(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
		},
	}
	creditClient := &fakeTopicCreditClient{total: 49}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/topics/1001",
		bytes.NewBufferString(`{"title":"如何排查支付回调？","body":"补充更多上下文。","tags":["支付"],"category_id":3,"bounty_score":50}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateTopic(c)

	require.Equal(t, http.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.NotNil(t, creditClient.balanceReq)
	require.Nil(t, contentClient.updateReq)
	require.Contains(t, recorder.Body.String(), bountyCreditInsufficientMessage)
}

func TestUpdateTopicRejectsQABountyAfterMembershipRevoked(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", RevokedAt: 1783970000000},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/topics/1001",
		bytes.NewBufferString(`{"title":"如何排查支付回调？","body":"补充更多上下文。","tags":["支付"],"category_id":3,"bounty_score":50}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Nil(t, contentClient.updateReq)
}

func TestUpdateTopicRejectsPublishedQABountyEditAfterMembershipRevoked(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		getTopicResp: &contentpb.TopicResponse{
			Success: true,
			Message: "ok",
			Topic: &contentpb.TopicInfo{
				Id:          1001,
				Type:        "qa",
				Title:       "如何排查支付回调？",
				AuthorId:    42,
				Status:      contentStatusPublished,
				BountyScore: 50,
			},
		},
	}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", RevokedAt: 1783970000000},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/topics/1001",
		bytes.NewBufferString(`{"title":"如何排查支付回调？","body":"补充更多上下文。","tags":["支付"],"category_id":3,"bounty_score":50}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementGrantTypeMembership, mallClient.req.GetGrantType())
	require.Nil(t, contentClient.updateReq)
}

func TestUpdateTopicRejectsQABountyAfterMembershipExpired(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(-time.Hour).UnixMilli()},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/topics/1001",
		bytes.NewBufferString(`{"title":"如何排查支付回调？","body":"补充更多上下文。","tags":["支付"],"category_id":3,"bounty_score":50}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Nil(t, contentClient.updateReq)
}

func TestDigitalEntitlementIsActiveRequiresExplicitActiveStatus(t *testing.T) {
	now := time.UnixMilli(2000)
	tests := []struct {
		name        string
		entitlement *mallpb.DigitalEntitlement
		want        bool
	}{
		{name: "active", entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE"}, want: true},
		{name: "blank status", entitlement: &mallpb.DigitalEntitlement{}, want: false},
		{name: "revoked", entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", RevokedAt: 1000}, want: false},
		{name: "expired", entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", ExpiresAt: 1999}, want: false},
		{name: "inactive status", entitlement: &mallpb.DigitalEntitlement{Status: "REVOKED"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, digitalEntitlementIsActive(tt.entitlement, now))
		})
	}
}

func TestPublishTopicRejectsMutedAuthor(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusMuted}}}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/publish", nil)

	h.publishTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "user_muted")
	require.Nil(t, contentClient.publishReq)
}

func TestPublishTopicRejectsUnverifiedAuthorWhenEmailGateEnabled(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		User:    userClient,
		Admin:   fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "true")}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/publish", nil)

	h.publishTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "email_not_verified")
	require.Nil(t, contentClient.publishReq)
}

func TestPublishTopicAllowsVerifiedActiveAuthor(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive, EmailVerified: true}}}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		User:    userClient,
		Admin:   fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "true")}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/publish", nil)

	h.publishTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.publishReq)
	require.EqualValues(t, 1001, contentClient.publishReq.GetId())
}

func TestPublishTopicRejectsQABountyWithoutMembershipBeforeContentPublish(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		getTopicResp: &contentpb.TopicResponse{
			Success: true,
			Message: "ok",
			Topic: &contentpb.TopicInfo{
				Id:          1001,
				Type:        "qa",
				Title:       "如何排查支付回调？",
				AuthorId:    42,
				Status:      1,
				BountyScore: 50,
			},
		},
	}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", RevokedAt: 1783970000000},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/publish", nil)

	h.publishTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Equal(t, digitalEntitlementGrantTypeMembership, mallClient.req.GetGrantType())
	require.Nil(t, contentClient.publishReq)
}

func TestPublishTopicAllowsQABountyWithMembership(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		getTopicResp: &contentpb.TopicResponse{
			Success: true,
			Message: "ok",
			Topic: &contentpb.TopicInfo{
				Id:          1001,
				Type:        "qa",
				Title:       "如何排查支付回调？",
				AuthorId:    42,
				Status:      1,
				BountyScore: 50,
			},
		},
	}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
		},
	}
	creditClient := &fakeTopicCreditClient{total: 100}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/publish", nil)

	h.publishTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.Equal(t, digitalEntitlementGrantTypeMembership, mallClient.req.GetGrantType())
	require.NotNil(t, creditClient.balanceReq)
	require.NotNil(t, contentClient.publishReq)
	require.EqualValues(t, 1001, contentClient.publishReq.GetId())
}

func TestPublishTopicRejectsQABountyWithInsufficientCreditBeforeContentPublish(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		getTopicResp: &contentpb.TopicResponse{
			Success: true,
			Message: "ok",
			Topic: &contentpb.TopicInfo{
				Id:          1001,
				Type:        "qa",
				Title:       "如何排查支付回调？",
				AuthorId:    42,
				Status:      1,
				BountyScore: 50,
			},
		},
	}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	mallClient := &captureThemeMallClient{
		entitlements: []*mallpb.DigitalEntitlement{
			{GrantType: "membership", GrantKey: "member-pro", Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
		},
	}
	creditClient := &fakeTopicCreditClient{total: 49}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient, Mall: mallClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/publish", nil)

	h.publishTopic(c)

	require.Equal(t, http.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.NotNil(t, creditClient.balanceReq)
	require.Nil(t, contentClient.publishReq)
	require.Contains(t, recorder.Body.String(), bountyCreditInsufficientMessage)
}

func TestPublishTopicMapsMembershipPermissionDeniedMessage(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		publishErr: status.Error(codes.PermissionDenied, "TOPIC_MEMBERSHIP_ENTITLEMENT_REQUIRED"),
	}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/publish", nil)

	h.publishTopic(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.publishReq)
	require.EqualValues(t, 1001, contentClient.publishReq.GetId())

	var envelope struct {
		Message string         `json:"message"`
		Meta    map[string]any `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "TOPIC_MEMBERSHIP_ENTITLEMENT_REQUIRED", envelope.Message)
	require.Equal(t, codes.PermissionDenied.String(), envelope.Meta["legacy_code"])
}

func TestAcceptTopicCommentRequiresOwnerAndCallsContentService(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		getTopicResp: &contentpb.TopicResponse{
			Success: true,
			Message: "ok",
			Topic: &contentpb.TopicInfo{
				Id:       1001,
				Type:     "qa",
				Title:    "如何排查支付回调？",
				AuthorId: 42,
				Status:   2,
			},
		},
	}
	h := NewHandler(&clients.Clients{Content: contentClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}, {Key: "commentId", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/comments/9001/accept", nil)

	h.acceptTopicComment(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, contentClient.acceptReq)
	require.EqualValues(t, 1001, contentClient.acceptReq.GetTopicId())
	require.EqualValues(t, 9001, contentClient.acceptReq.GetCommentId())
	require.EqualValues(t, 42, contentClient.acceptReq.GetUserId())
}

func TestAcceptTopicCommentRejectsQABountyWithInsufficientCreditBeforeContentAccept(t *testing.T) {
	contentClient := &fakeTopicContentClient{
		getTopicResp: &contentpb.TopicResponse{
			Success: true,
			Message: "ok",
			Topic: &contentpb.TopicInfo{
				Id:          1001,
				Type:        "qa",
				Title:       "如何排查支付回调？",
				AuthorId:    42,
				Status:      2,
				BountyScore: 50,
			},
		},
	}
	creditClient := &fakeTopicCreditClient{total: 49}
	h := NewHandler(&clients.Clients{Content: contentClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}, {Key: "commentId", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/comments/9001/accept", nil)

	h.acceptTopicComment(c)

	require.Equal(t, http.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.NotNil(t, creditClient.balanceReq)
	require.Nil(t, contentClient.acceptReq)
	require.Contains(t, recorder.Body.String(), bountyCreditInsufficientMessage)
}

type fakeTopicContentClient struct {
	contentpb.ContentServiceClient
	createReq    *contentpb.CreateTopicRequest
	updateReq    *contentpb.UpdateTopicRequest
	publishReq   *contentpb.TopicIDRequest
	acceptReq    *contentpb.AcceptTopicCommentRequest
	getTopicResp *contentpb.TopicResponse
	publishErr   error
}

func (f *fakeTopicContentClient) CreateTopic(_ context.Context, req *contentpb.CreateTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	f.createReq = req
	return &contentpb.TopicResponse{
		Success: true,
		Message: "ok",
		Topic: &contentpb.TopicInfo{
			Id:          1001,
			Slug:        req.GetSlug(),
			Type:        req.GetType(),
			Title:       req.GetTitle(),
			Body:        req.GetBody(),
			Tags:        req.GetTags(),
			AuthorId:    req.GetAuthorId(),
			CategoryId:  req.GetCategoryId(),
			BountyScore: req.GetBountyScore(),
			QaStatus:    "open",
			Status:      1,
		},
	}, nil
}

func (f *fakeTopicContentClient) UpdateTopic(_ context.Context, req *contentpb.UpdateTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	f.updateReq = req
	return &contentpb.TopicResponse{
		Success: true,
		Message: "ok",
		Topic: &contentpb.TopicInfo{
			Id:          req.GetId(),
			Type:        "qa",
			Title:       req.GetTitle(),
			Body:        req.GetBody(),
			Tags:        req.GetTags(),
			AuthorId:    42,
			CategoryId:  req.GetCategoryId(),
			BountyScore: req.GetBountyScore(),
			QaStatus:    "open",
			Status:      2,
		},
	}, nil
}

func (f *fakeTopicContentClient) PublishTopic(_ context.Context, req *contentpb.TopicIDRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	f.publishReq = req
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	return &contentpb.TopicResponse{
		Success: true,
		Message: "ok",
		Topic: &contentpb.TopicInfo{
			Id:       req.GetId(),
			Type:     "qa",
			Title:    "如何排查支付回调？",
			AuthorId: 42,
			Status:   2,
		},
	}, nil
}

func (f *fakeTopicContentClient) GetTopic(_ context.Context, _ *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	if f.getTopicResp != nil {
		return f.getTopicResp, nil
	}
	return &contentpb.TopicResponse{
		Success: true,
		Message: "ok",
		Topic: &contentpb.TopicInfo{
			Id:       1001,
			Type:     "qa",
			Title:    "如何排查支付回调？",
			AuthorId: 42,
			Status:   2,
		},
	}, nil
}

func (f *fakeTopicContentClient) AcceptTopicComment(_ context.Context, req *contentpb.AcceptTopicCommentRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	f.acceptReq = req
	return &contentpb.TopicResponse{
		Success: true,
		Message: "ok",
		Topic: &contentpb.TopicInfo{
			Id:                req.GetTopicId(),
			Type:              "qa",
			Title:             "如何排查支付回调？",
			AuthorId:          42,
			Status:            2,
			QaStatus:          "resolved",
			AcceptedCommentId: req.GetCommentId(),
		},
	}, nil
}

type fakeTopicCreditClient struct {
	creditpb.CreditServiceClient
	total      int64
	balanceReq *creditpb.GetBalanceRequest
	balanceErr error
}

func (f *fakeTopicCreditClient) GetBalance(_ context.Context, req *creditpb.GetBalanceRequest, _ ...grpc.CallOption) (*creditpb.BalanceResponse, error) {
	f.balanceReq = req
	if f.balanceErr != nil {
		return nil, f.balanceErr
	}
	return &creditpb.BalanceResponse{
		Balance: &creditpb.Balance{
			UserId: req.GetUserId(),
			Total:  f.total,
		},
	}, nil
}
