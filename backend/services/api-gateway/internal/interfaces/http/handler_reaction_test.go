package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

type fakeReactionClient struct {
	reactionpb.ReactionServiceClient
	likeReq     *reactionpb.ReactRequest
	unlikeReq   *reactionpb.ReactRequest
	favoriteReq *reactionpb.ReactRequest
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
