package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestLikeTopicRequiresPublishedTopic(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		topic: &contentpb.TopicInfo{Id: 1001, Status: 1},
	}
	reactionClient := &fakeReactionClient{}
	h := NewHandler(&clients.Clients{Content: contentClient, Reaction: reactionClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/like", nil)

	h.likeTopic(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.topicReq)
	require.Nil(t, reactionClient.likeReq)
}

func TestFavoriteArticleForwardsPublishedArticle(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: contentStatusPublished},
	}
	reactionClient := &fakeReactionClient{}
	h := NewHandler(&clients.Clients{Content: contentClient, Reaction: reactionClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "2001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/articles/2001/favorite", nil)

	h.favoriteArticle(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.articleReq)
	require.NotNil(t, reactionClient.favoriteReq)
	require.Equal(t, "article", reactionClient.favoriteReq.GetEntity().GetEntityType())
	require.EqualValues(t, 2001, reactionClient.favoriteReq.GetEntity().GetEntityId())
}

func TestUnlikeTopicDoesNotRequirePublishedTopic(t *testing.T) {
	reactionClient := &fakeReactionClient{}
	h := NewHandler(&clients.Clients{Reaction: reactionClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/topics/1001/like", nil)

	h.unlikeTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, reactionClient.unlikeReq)
	require.Equal(t, "topic", reactionClient.unlikeReq.GetEntity().GetEntityType())
}

func TestReportArticleRequiresPublishedArticle(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: 1},
	}
	reactionClient := &fakeReactionClient{}
	h := NewHandler(&clients.Clients{Content: contentClient, Reaction: reactionClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "2001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/articles/2001/report", bytes.NewBufferString(`{"reason":"spam","description":"bad"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.reportArticle(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.articleReq)
	require.Nil(t, reactionClient.reportReq)
}

func TestReportTopicForwardsPublishedTopic(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		topic: &contentpb.TopicInfo{Id: 1001, Status: contentStatusPublished},
	}
	reactionClient := &fakeReactionClient{}
	h := NewHandler(&clients.Clients{Content: contentClient, Reaction: reactionClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/report", bytes.NewBufferString(`{"reason":"spam","description":"bad"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.reportTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, reactionClient.reportReq)
	require.Equal(t, "topic", reactionClient.reportReq.GetEntity().GetEntityType())
	require.EqualValues(t, 1001, reactionClient.reportReq.GetEntity().GetEntityId())
	require.EqualValues(t, 42, reactionClient.reportReq.GetReporterId())
}

func TestReportCommentRequiresVisibleComment(t *testing.T) {
	commentClient := &fakeReportCommentClient{
		comment: &commentpb.CommentInfo{Id: 9001, Status: 0},
	}
	reactionClient := &fakeReactionClient{}
	h := NewHandler(&clients.Clients{Comment: commentClient, Reaction: reactionClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/comments/9001/report", bytes.NewBufferString(`{"reason":"spam","description":"bad"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.reportComment(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, commentClient.req)
	require.Nil(t, reactionClient.reportReq)
}

type fakeReactionClient struct {
	reactionpb.ReactionServiceClient
	likeReq     *reactionpb.ReactRequest
	unlikeReq   *reactionpb.ReactRequest
	favoriteReq *reactionpb.ReactRequest
	reportReq   *reactionpb.SubmitReportRequest
}

func (f *fakeReactionClient) Like(_ context.Context, req *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	f.likeReq = req
	return &reactionpb.ReactResponse{Success: true, Count: 1, Changed: true}, nil
}

func (f *fakeReactionClient) Unlike(_ context.Context, req *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	f.unlikeReq = req
	return &reactionpb.ReactResponse{Success: true, Count: 0, Changed: true}, nil
}

func (f *fakeReactionClient) Favorite(_ context.Context, req *reactionpb.ReactRequest, _ ...grpc.CallOption) (*reactionpb.ReactResponse, error) {
	f.favoriteReq = req
	return &reactionpb.ReactResponse{Success: true, Count: 1, Changed: true}, nil
}

func (f *fakeReactionClient) SubmitReport(_ context.Context, req *reactionpb.SubmitReportRequest, _ ...grpc.CallOption) (*reactionpb.ReportResponse, error) {
	f.reportReq = req
	return &reactionpb.ReportResponse{Success: true, Created: true, Report: &reactionpb.ReportInfo{Id: 7001, Entity: req.GetEntity(), ReporterId: req.GetReporterId(), Reason: req.GetReason(), Description: req.GetDescription()}}, nil
}

type fakeReportCommentClient struct {
	commentpb.CommentServiceClient
	comment *commentpb.CommentInfo
	req     *commentpb.GetCommentRequest
}

func (f *fakeReportCommentClient) GetComment(_ context.Context, req *commentpb.GetCommentRequest, _ ...grpc.CallOption) (*commentpb.CommentResponse, error) {
	f.req = req
	return &commentpb.CommentResponse{Success: true, Comment: f.comment}, nil
}
