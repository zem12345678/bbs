package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestCreateTopicPassesQABountyToContentService(t *testing.T) {
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

func TestCreateTopicRejectsQABountyWithoutMembership(t *testing.T) {
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

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.req)
	require.EqualValues(t, 42, mallClient.req.GetUserId())
	require.Equal(t, digitalEntitlementStatusActive, mallClient.req.GetStatus())
	require.Nil(t, contentClient.createReq)
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
}

type fakeTopicContentClient struct {
	contentpb.ContentServiceClient
	createReq    *contentpb.CreateTopicRequest
	acceptReq    *contentpb.AcceptTopicCommentRequest
	getTopicResp *contentpb.TopicResponse
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
