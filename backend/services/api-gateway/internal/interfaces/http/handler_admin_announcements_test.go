package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestAdminAnnouncementListReturnsBareCompatibilityContract(t *testing.T) {
	client := &announcementHandlerAdminClient{
		permissions: []string{"governance:list_announcements"},
		listResponse: &adminpb.AnnouncementListResponse{Items: []*adminpb.AnnouncementInfo{{
			Id: "maintenance", CreatedAt: 1700000000000, UpdatedAt: 1700000001000,
			Title: "维护公告", Text: "今晚维护", Icon: "warning", Display: "banner", Active: true, Reads: 3,
		}}},
	}
	router := announcementHandlerTestRouter(client)
	recorder := performAdminAnnouncementRequest(router, "/api/v1/admin/announcements/list", `{"limit":25,"untilId":"cursor","status":"all"}`)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(25), client.listRequest.GetLimit())
	require.Equal(t, "cursor", client.listRequest.GetUntilId())
	require.Equal(t, "all", client.listRequest.GetStatus())
	require.Equal(t, int64(42), client.listRequest.GetActor().GetId())
	var payload []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.Equal(t, "maintenance", payload[0]["id"])
	require.Equal(t, "2023-11-14T22:13:20Z", payload[0]["createdAt"])
	require.Equal(t, float64(3), payload[0]["reads"])
	require.NotContains(t, payload[0], "data")
}

func TestAdminAnnouncementCreateAcceptsNullableImageAndDefaultsActive(t *testing.T) {
	client := &announcementHandlerAdminClient{
		permissions:    []string{"governance:create_announcement"},
		createResponse: &adminpb.AnnouncementResponse{Announcement: &adminpb.AnnouncementInfo{Id: "launch", CreatedAt: 1700000000000, UpdatedAt: 1700000000000, Title: "上线", Text: "欢迎", Icon: "info", Display: "normal", Active: true}},
	}
	router := announcementHandlerTestRouter(client)
	recorder := performAdminAnnouncementRequest(router, "/api/v1/admin/announcements/create", `{"title":"上线","text":"欢迎","imageUrl":null}`)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "上线", client.createRequest.GetTitle())
	require.Empty(t, client.createRequest.GetImageUrl())
	require.True(t, client.createRequest.GetActive())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "launch", payload["id"])
	require.NotContains(t, payload, "data")
}

func TestAdminAnnouncementUpdatePreservesPatchPresenceAndReturnsNoContent(t *testing.T) {
	client := &announcementHandlerAdminClient{permissions: []string{"governance:update_announcement"}}
	router := announcementHandlerTestRouter(client)
	recorder := performAdminAnnouncementRequest(router, "/api/v1/admin/announcements/update", `{"id":"maintenance","imageUrl":null,"forRoles":[],"isActive":false,"startsAt":0}`)

	require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Equal(t, "maintenance", client.updateRequest.GetId())
	require.Nil(t, client.updateRequest.Title)
	require.NotNil(t, client.updateRequest.ImageUrl)
	require.Empty(t, client.updateRequest.GetImageUrl())
	require.NotNil(t, client.updateRequest.GetForRoles())
	require.Empty(t, client.updateRequest.GetForRoles().GetValues())
	require.NotNil(t, client.updateRequest.Active)
	require.False(t, client.updateRequest.GetActive())
	require.NotNil(t, client.updateRequest.StartsAt)
	require.Zero(t, client.updateRequest.GetStartsAt())
}

func TestAdminAnnouncementDeleteAndAccountReadReturnNoContent(t *testing.T) {
	client := &announcementHandlerAdminClient{permissions: []string{"governance:delete_announcement"}}
	router := announcementHandlerTestRouter(client)

	deleted := performAdminAnnouncementRequest(router, "/api/v1/admin/announcements/delete", `{"id":"maintenance"}`)
	require.Equal(t, stdhttp.StatusNoContent, deleted.Code, deleted.Body.String())
	require.Equal(t, "maintenance", client.deleteRequest.GetId())

	read := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/read-announcement", strings.NewReader(`{"announcementId":"maintenance"}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "77", "username": "reader", "exp": time.Now().Add(time.Hour).Unix()}))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(read, request)
	require.Equal(t, stdhttp.StatusNoContent, read.Code, read.Body.String())
	require.Equal(t, int64(77), client.readRequest.GetUserId())
	require.Equal(t, "maintenance", client.readRequest.GetAnnouncementId())
}

func TestAdminAnnouncementRoutesRequireDedicatedPermissions(t *testing.T) {
	client := &announcementHandlerAdminClient{}
	router := announcementHandlerTestRouter(client)
	for _, route := range []string{"list", "create", "update", "delete"} {
		recorder := performAdminAnnouncementRequest(router, "/api/v1/admin/announcements/"+route, `{}`)
		require.Equal(t, stdhttp.StatusForbidden, recorder.Code, route)
	}
	require.Nil(t, client.listRequest)
	require.Nil(t, client.createRequest)
	require.Nil(t, client.updateRequest)
	require.Nil(t, client.deleteRequest)
}

func announcementHandlerTestRouter(client adminpb.AdminServiceClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInitControllers(NewHandler(&clients.Clients{Admin: client}, "Authorization", "Bearer", testJWTSecret))(router)
	return router
}

func performAdminAnnouncementRequest(router *gin.Engine, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer announcement-admin-token")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

type announcementHandlerAdminClient struct {
	adminpb.AdminServiceClient
	permissions    []string
	listRequest    *adminpb.ListAnnouncementsRequest
	listResponse   *adminpb.AnnouncementListResponse
	createRequest  *adminpb.CreateAnnouncementRequest
	createResponse *adminpb.AnnouncementResponse
	updateRequest  *adminpb.UpdateAnnouncementRequest
	deleteRequest  *adminpb.AnnouncementIDRequest
	readRequest    *adminpb.ReadAnnouncementRequest
}

func (client *announcementHandlerAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{User: &adminpb.AdminUserInfo{Id: 42, Username: "operator"}, Permissions: client.permissions}, nil
}

func (client *announcementHandlerAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}

func (client *announcementHandlerAdminClient) ListAnnouncements(_ context.Context, req *adminpb.ListAnnouncementsRequest, _ ...grpc.CallOption) (*adminpb.AnnouncementListResponse, error) {
	client.listRequest = req
	return client.listResponse, nil
}

func (client *announcementHandlerAdminClient) CreateAnnouncement(_ context.Context, req *adminpb.CreateAnnouncementRequest, _ ...grpc.CallOption) (*adminpb.AnnouncementResponse, error) {
	client.createRequest = req
	return client.createResponse, nil
}

func (client *announcementHandlerAdminClient) UpdateAnnouncement(_ context.Context, req *adminpb.UpdateAnnouncementRequest, _ ...grpc.CallOption) (*adminpb.AnnouncementResponse, error) {
	client.updateRequest = req
	return &adminpb.AnnouncementResponse{Success: true}, nil
}

func (client *announcementHandlerAdminClient) DeleteAnnouncement(_ context.Context, req *adminpb.AnnouncementIDRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	client.deleteRequest = req
	return &adminpb.SimpleResponse{Success: true}, nil
}

func (client *announcementHandlerAdminClient) MarkAnnouncementRead(_ context.Context, req *adminpb.ReadAnnouncementRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	client.readRequest = req
	return &adminpb.SimpleResponse{Success: true}, nil
}
