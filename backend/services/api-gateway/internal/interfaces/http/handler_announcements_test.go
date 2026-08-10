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

func TestListAnnouncementsReturnsConfiguredActiveItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := announcementsTestRouter(`[{
		"id":"maintenance",
		"title":"维护公告",
		"text":"今晚 23:00 维护",
		"icon":"warning",
		"display":"banner",
		"active":true,
		"created_at":1735689600000
	},{
		"id":"draft",
		"title":"草稿",
		"text":"不应公开",
		"active":false
	},{
		"id":"maintenance",
		"title":"重复",
		"text":"不应重复"
	}]`)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/announcements?limit=5", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Items []publicAnnouncement `json:"items"`
			Total int                  `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, 1, envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "maintenance", envelope.Data.Items[0].ID)
	require.Equal(t, "维护公告", envelope.Data.Items[0].Title)
	require.Equal(t, "今晚 23:00 维护", envelope.Data.Items[0].Text)
	require.Equal(t, "warning", envelope.Data.Items[0].Icon)
	require.Equal(t, "banner", envelope.Data.Items[0].Display)
	require.True(t, envelope.Data.Items[0].Active)
}

func TestListAnnouncementsSupportsInactiveCompatibilityFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := announcementsTestRouter(`[{
		"id":"active",
		"title":"已发布",
		"text":"可见"
	},{
		"id":"inactive",
		"title":"已停用",
		"text":"预览可见",
		"active":false
	}]`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/announcements", strings.NewReader(`{"isActive":false,"limit":1000}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var items []publicAnnouncement
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &items))
	require.Len(t, items, 1)
	require.Equal(t, "inactive", items[0].ID)
}

func TestShowAnnouncementReturnsOnlyActiveConfiguredItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := announcementsTestRouter(`[{
		"id":"launch",
		"title":"上线公告",
		"content":"欢迎使用",
		"imageUrl":"https://cdn.example.com/banner.png",
		"needConfirmationToRead":true,
		"forYou":true
	},{
		"id":"disabled",
		"title":"停用公告",
		"text":"不应可见",
		"active":false
	}]`)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/announcements/launch", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Announcement publicAnnouncement `json:"announcement"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "launch", envelope.Data.Announcement.ID)
	require.Equal(t, "欢迎使用", envelope.Data.Announcement.Text)
	require.Equal(t, "https://cdn.example.com/banner.png", envelope.Data.Announcement.ImageURL)
	require.True(t, envelope.Data.Announcement.NeedConfirmationToRead)
	require.True(t, envelope.Data.Announcement.ForYou)

	missingRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingRecorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/announcements/show", strings.NewReader(`{"announcementId":"disabled"}`)))
	require.Equal(t, stdhttp.StatusNotFound, missingRecorder.Code, missingRecorder.Body.String())
}

func TestParsePublicAnnouncementsRejectsInvalidJSONAndOutOfWindowItems(t *testing.T) {
	require.Empty(t, parsePublicAnnouncements("not-json", 1000))
	items := parsePublicAnnouncements(`[{
		"id":"future",
		"title":"未开始",
		"text":"later",
		"starts_at":2000
	},{
		"id":"expired",
		"title":"已结束",
		"text":"past",
		"ends_at":500
	},{
		"id":"",
		"title":"缺 ID",
		"text":"ignored"
	}]`, 1000)
	require.Len(t, items, 2)
	require.False(t, items[0].Active)
	require.False(t, items[1].Active)
}

func TestTypedAnnouncementCompatibilityReturnsCamelCaseReadState(t *testing.T) {
	client := &typedAnnouncementAdminClient{listResponse: &adminpb.AnnouncementListResponse{Items: []*adminpb.AnnouncementInfo{{
		Id: "targeted", CreatedAt: 1700000000000, UpdatedAt: 1700000001000, Title: "定向公告", Text: "正文",
		Icon: "info", Display: "dialog", Active: false, UserId: 77, ForYou: true, IsRead: true,
	}}}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInitControllers(NewHandler(&clients.Clients{Admin: client}, "Authorization", "Bearer", testJWTSecret))(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/announcements", strings.NewReader(`{"limit":5,"isActive":false}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "77", "username": "reader", "exp": time.Now().Add(time.Hour).Unix()}))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(77), client.listRequest.GetUserId())
	require.NotNil(t, client.listRequest.Active)
	require.False(t, client.listRequest.GetActive())
	var payload []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload, 1)
	require.Equal(t, "2023-11-14T22:13:20Z", payload[0]["createdAt"])
	require.Equal(t, true, payload[0]["forYou"])
	require.Equal(t, true, payload[0]["isRead"])
	require.NotContains(t, payload[0], "created_at")
}

func announcementsTestRouter(rawAnnouncements string) *gin.Engine {
	h := NewHandler(&clients.Clients{Admin: fakePublicSettingsAdminClient{items: []*adminpb.SettingInfo{
		{Key: "site_announcements", Value: rawAnnouncements},
		{Key: "auth.github.client_secret", Value: "must-not-be-exposed"},
	}}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type typedAnnouncementAdminClient struct {
	adminpb.AdminServiceClient
	listRequest  *adminpb.ListPublicAnnouncementsRequest
	listResponse *adminpb.AnnouncementListResponse
}

func (client *typedAnnouncementAdminClient) ListPublicAnnouncements(_ context.Context, req *adminpb.ListPublicAnnouncementsRequest, _ ...grpc.CallOption) (*adminpb.AnnouncementListResponse, error) {
	client.listRequest = req
	return client.listResponse, nil
}
