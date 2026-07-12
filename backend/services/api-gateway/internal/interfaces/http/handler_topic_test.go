package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestCreateTopicPassesQABountyToContentService(t *testing.T) {
	contentClient := &fakeTopicContentClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: 1}}}
	h := NewHandler(&clients.Clients{Content: contentClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

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
}

type fakeTopicContentClient struct {
	contentpb.ContentServiceClient
	createReq *contentpb.CreateTopicRequest
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
