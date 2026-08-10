package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestAdminAdListMapsFiltersAndReturnsBareContract(t *testing.T) {
	adminClient := &adHandlerAdminClient{
		permissions: []string{"governance:list_ads"},
		listResponse: &adminpb.AdListResponse{Items: []*adminpb.AdInfo{{
			Id: 9007199254740993, Url: "https://example.com", Memo: "campaign", Place: "horizontal",
			Priority: "high", Ratio: 2, StartsAt: 1700000000000, ExpiresAt: 1700086400000,
			ImageUrl: "https://cdn.example.com/ad.png", DayOfWeek: 127,
		}}},
	}
	router := adHandlerTestRouter(adminClient)

	recorder := performAdminAdRequest(router, "/api/v1/admin/ad/list", `{"limit":25,"sinceId":"9007199254740990","untilId":"9007199254740999","publishing":false}`)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(25), adminClient.listRequest.GetLimit())
	require.Equal(t, int64(9007199254740990), adminClient.listRequest.GetSinceId())
	require.Equal(t, int64(9007199254740999), adminClient.listRequest.GetUntilId())
	require.NotNil(t, adminClient.listRequest.Publishing)
	require.False(t, adminClient.listRequest.GetPublishing())
	require.Equal(t, int64(42), adminClient.listRequest.GetActor().GetId())

	var payload []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.Equal(t, "9007199254740993", payload[0]["id"])
	require.Equal(t, "2023-11-14T22:13:20Z", payload[0]["startsAt"])
	require.NotContains(t, payload[0], "data")
}

func TestAdminAdCreateMapsRequestAndReturnsBareAd(t *testing.T) {
	adminClient := &adHandlerAdminClient{
		permissions: []string{"governance:create_ad"},
		createResponse: &adminpb.AdResponse{Ad: &adminpb.AdInfo{
			Id: 81, Url: "https://example.com", Place: "vertical", Priority: "normal", Ratio: 0,
			StartsAt: 1700000000000, ExpiresAt: 1700086400000, ImageUrl: "https://cdn.example.com/ad.png",
			DayOfWeek: 0,
		}},
	}
	router := adHandlerTestRouter(adminClient)
	body := `{"url":"https://example.com","memo":"","place":"vertical","priority":"normal","ratio":0,"expiresAt":1700086400000,"startsAt":1700000000000,"imageUrl":"https://cdn.example.com/ad.png","dayOfWeek":0}`

	recorder := performAdminAdRequest(router, "/api/v1/admin/ad/create", body)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), adminClient.createRequest.GetActor().GetId())
	require.Equal(t, int32(0), adminClient.createRequest.GetRatio())
	require.Equal(t, int32(0), adminClient.createRequest.GetDayOfWeek())
	require.Equal(t, int64(1700000000000), adminClient.createRequest.GetStartsAt())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "81", payload["id"])
	require.NotContains(t, payload, "data")
}

func TestAdminAdUpdateAndDeleteReturnNoContent(t *testing.T) {
	adminClient := &adHandlerAdminClient{permissions: []string{"governance:update_ad", "governance:delete_ad"}}
	router := adHandlerTestRouter(adminClient)

	updated := performAdminAdRequest(router, "/api/v1/admin/ad/update", `{"id":"9007199254740993","ratio":0,"memo":""}`)
	require.Equal(t, stdhttp.StatusNoContent, updated.Code, updated.Body.String())
	require.Equal(t, int64(9007199254740993), adminClient.updateRequest.GetId())
	require.NotNil(t, adminClient.updateRequest.Ratio)
	require.Equal(t, int32(0), adminClient.updateRequest.GetRatio())
	require.NotNil(t, adminClient.updateRequest.Memo)
	require.Empty(t, adminClient.updateRequest.GetMemo())
	require.Nil(t, adminClient.updateRequest.Url)

	deleted := performAdminAdRequest(router, "/api/v1/admin/ad/delete", `{"id":"9007199254740993"}`)
	require.Equal(t, stdhttp.StatusNoContent, deleted.Code, deleted.Body.String())
	require.Equal(t, int64(9007199254740993), adminClient.deleteRequest.GetId())
}

func TestAdminAdRoutesRequireDedicatedPermissions(t *testing.T) {
	adminClient := &adHandlerAdminClient{}
	router := adHandlerTestRouter(adminClient)

	for _, route := range []struct {
		path string
		body string
	}{
		{path: "/api/v1/admin/ad/list", body: `{}`},
		{path: "/api/v1/admin/ad/create", body: `{}`},
		{path: "/api/v1/admin/ad/update", body: `{}`},
		{path: "/api/v1/admin/ad/delete", body: `{}`},
	} {
		recorder := performAdminAdRequest(router, route.path, route.body)
		require.Equal(t, stdhttp.StatusForbidden, recorder.Code, route.path)
	}
	require.Nil(t, adminClient.listRequest)
	require.Nil(t, adminClient.createRequest)
	require.Nil(t, adminClient.updateRequest)
	require.Nil(t, adminClient.deleteRequest)
}

func TestAdminAdRequestsRejectInvalidIDsAndMissingCreateFields(t *testing.T) {
	adminClient := &adHandlerAdminClient{permissions: []string{
		"governance:list_ads", "governance:create_ad", "governance:update_ad", "governance:delete_ad",
	}}
	router := adHandlerTestRouter(adminClient)

	cases := []struct {
		path string
		body string
	}{
		{path: "/api/v1/admin/ad/list", body: `{"limit":101}`},
		{path: "/api/v1/admin/ad/list", body: `{"sinceId":"invalid"}`},
		{path: "/api/v1/admin/ad/create", body: `{"url":"https://example.com"}`},
		{path: "/api/v1/admin/ad/create", body: `{"url":"https://example.com","place":"vertical","priority":"normal","ratio":1,"expiresAt":1700086400000,"startsAt":1700000000000,"imageUrl":"https://cdn.example.com/ad.png","dayOfWeek":127}`},
		{path: "/api/v1/admin/ad/update", body: `{"id":"0"}`},
		{path: "/api/v1/admin/ad/delete", body: `{"id":"-1"}`},
	}
	for _, tt := range cases {
		recorder := performAdminAdRequest(router, tt.path, tt.body)
		require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, tt.path+" "+tt.body)
	}
}

func adHandlerTestRouter(adminClient adminpb.AdminServiceClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInitControllers(NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret))(router)
	return router
}

func performAdminAdRequest(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer ad-admin-token")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

type adHandlerAdminClient struct {
	adminpb.AdminServiceClient
	permissions    []string
	listRequest    *adminpb.ListAdsRequest
	listResponse   *adminpb.AdListResponse
	createRequest  *adminpb.CreateAdRequest
	createResponse *adminpb.AdResponse
	updateRequest  *adminpb.UpdateAdRequest
	deleteRequest  *adminpb.AdIDRequest
}

func (c *adHandlerAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{
		User:        &adminpb.AdminUserInfo{Id: 42, Username: "operator"},
		Permissions: c.permissions,
	}, nil
}

func (c *adHandlerAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}

func (c *adHandlerAdminClient) ListAds(_ context.Context, req *adminpb.ListAdsRequest, _ ...grpc.CallOption) (*adminpb.AdListResponse, error) {
	c.listRequest = req
	return c.listResponse, nil
}

func (c *adHandlerAdminClient) CreateAd(_ context.Context, req *adminpb.CreateAdRequest, _ ...grpc.CallOption) (*adminpb.AdResponse, error) {
	c.createRequest = req
	return c.createResponse, nil
}

func (c *adHandlerAdminClient) UpdateAd(_ context.Context, req *adminpb.UpdateAdRequest, _ ...grpc.CallOption) (*adminpb.AdResponse, error) {
	c.updateRequest = req
	return &adminpb.AdResponse{Success: true}, nil
}

func (c *adHandlerAdminClient) DeleteAd(_ context.Context, req *adminpb.AdIDRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	c.deleteRequest = req
	return &adminpb.SimpleResponse{Success: true}, nil
}
