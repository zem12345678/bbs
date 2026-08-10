package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSiteConfigReturnsOnlySupportedPublicFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{Admin: fakePublicSettingsAdminClient{items: []*adminpb.SettingInfo{
		{Key: "site_name", Value: "示例社区"},
		{Key: "site_description", Value: "面向创作者的社区"},
		{Key: "seo_keywords", Value: "bbs, 创作"},
		{Key: "site_logo_url", Value: "https://cdn.example.com/logo.png"},
		{Key: "site_navigation", Value: `[{"key":"plaza","label":"发现"},{"key":"chat","label":"聊天室"},{"key":"unknown","label":"忽略"},{"key":"plaza","label":"重复"}]`},
		{Key: "auth.github.client_secret", Value: "must-not-be-exposed"},
	}}}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/site-config", nil)
	h.siteConfig(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data struct {
			SiteName        string               `json:"site_name"`
			SiteDescription string               `json:"site_description"`
			SEOKeywords     string               `json:"seo_keywords"`
			LogoURL         string               `json:"logo_url"`
			Navigation      []siteNavigationItem `json:"navigation"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "示例社区", envelope.Data.SiteName)
	require.Equal(t, "面向创作者的社区", envelope.Data.SiteDescription)
	require.Equal(t, "bbs, 创作", envelope.Data.SEOKeywords)
	require.Equal(t, "https://cdn.example.com/logo.png", envelope.Data.LogoURL)
	require.Equal(t, []siteNavigationItem{{Key: "plaza", Label: "发现"}, {Key: "chat", Label: "聊天室"}}, envelope.Data.Navigation)
	require.NotContains(t, recorder.Body.String(), "must-not-be-exposed")
}

func TestParseSiteNavigationRejectsInvalidAndUnknownItems(t *testing.T) {
	require.Empty(t, parseSiteNavigation("not-json"))
	require.Equal(t, []siteNavigationItem{{Key: "home", Label: "首页"}, {Key: "chat", Label: "聊天室"}}, parseSiteNavigation(`[{"key":" HOME ","label":" 首页 "},{"key":"chat","label":"聊天室"},{"key":"external","label":"外部"}]`))
}

type fakePublicSettingsAdminClient struct {
	adminpb.AdminServiceClient
	items []*adminpb.SettingInfo
}

func (f fakePublicSettingsAdminClient) ListPublicSettings(context.Context, *adminpb.ListPublicSettingsRequest, ...grpc.CallOption) (*adminpb.SettingListResponse, error) {
	return &adminpb.SettingListResponse{Items: f.items, Total: int64(len(f.items))}, nil
}

func (f fakePublicSettingsAdminClient) ListPublicAnnouncements(context.Context, *adminpb.ListPublicAnnouncementsRequest, ...grpc.CallOption) (*adminpb.AnnouncementListResponse, error) {
	return nil, status.Error(codes.Unimplemented, "typed announcements are not configured by this test double")
}

func (f fakePublicSettingsAdminClient) GetPublicAnnouncement(context.Context, *adminpb.GetPublicAnnouncementRequest, ...grpc.CallOption) (*adminpb.AnnouncementResponse, error) {
	return nil, status.Error(codes.Unimplemented, "typed announcements are not configured by this test double")
}
