package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestUpdateArticleRejectsMutedAuthor(t *testing.T) {
	contentClient := &fakeArticleUpdateContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusMuted}}}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "2001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/articles/2001",
		bytes.NewBufferString(`{"title":"更新后的文章","summary":"禁言用户不能修改公开内容。","body":"正文","cover_url":"","tags":["治理"]}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateArticle(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "user_muted")
	require.Nil(t, contentClient.updateReq)
}

func TestUpdatePublishedArticleRejectsUnverifiedAuthorWhenEmailGateEnabled(t *testing.T) {
	contentClient := &fakeArticleUpdateContentClient{}
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
	c.Params = gin.Params{{Key: "id", Value: "2001"}}
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/articles/2001",
		bytes.NewBufferString(`{"title":"更新后的公开文章","summary":"未验证用户不能修改公开内容。","body":"正文","cover_url":"","tags":["治理"]}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.updateArticle(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "email_not_verified")
	require.Nil(t, contentClient.updateReq)
}

type fakeArticleUpdateContentClient struct {
	contentpb.ContentServiceClient
	updateReq *contentpb.UpdateArticleRequest
}

func (f *fakeArticleUpdateContentClient) GetArticle(_ context.Context, _ *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	return &contentpb.ArticleResponse{Article: &contentpb.ArticleInfo{Id: 2001, AuthorId: 42, Status: contentStatusPublished}}, nil
}

func (f *fakeArticleUpdateContentClient) UpdateArticle(_ context.Context, req *contentpb.UpdateArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	f.updateReq = req
	return &contentpb.ArticleResponse{Article: &contentpb.ArticleInfo{Id: req.GetId(), AuthorId: 42, Status: contentStatusPublished}}, nil
}
