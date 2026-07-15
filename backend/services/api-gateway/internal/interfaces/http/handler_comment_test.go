package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListTopicCommentsRequiresPublishedTopic(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		topic: &contentpb.TopicInfo{Id: 1001, Status: 1},
	}
	commentClient := &fakeListCommentClient{}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topics/1001/comments", nil)

	h.listTopicComments(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.topicReq)
	require.Nil(t, commentClient.listReq)
}

func TestListArticleCommentsForwardsPublishedArticle(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: contentStatusPublished},
	}
	commentClient := &fakeListCommentClient{}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "2001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/articles/2001/comments", nil)

	h.listComments(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.articleReq)
	require.NotNil(t, commentClient.listReq)
	require.Equal(t, "article", commentClient.listReq.GetEntityType())
	require.EqualValues(t, 2001, commentClient.listReq.GetEntityId())
}

func TestListRepliesRequiresVisibleRootComment(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{}
	commentClient := &fakeListCommentClient{
		root: &commentpb.CommentInfo{Id: 9001, EntityType: "topic", EntityId: 1001, Status: 0},
	}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/comments/9001/replies", nil)

	h.listReplies(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, commentClient.getReq)
	require.Nil(t, contentClient.topicReq)
	require.Nil(t, commentClient.repliesReq)
}

func TestListRepliesRequiresPublishedRootTarget(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		topic: &contentpb.TopicInfo{Id: 1001, Status: 1},
	}
	commentClient := &fakeListCommentClient{
		root: &commentpb.CommentInfo{Id: 9001, EntityType: "topic", EntityId: 1001, Status: 1},
	}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/comments/9001/replies", nil)

	h.listReplies(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, commentClient.getReq)
	require.NotNil(t, contentClient.topicReq)
	require.Nil(t, commentClient.repliesReq)
}

func TestListRepliesForwardsPublishedRootTarget(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: contentStatusPublished},
	}
	commentClient := &fakeListCommentClient{
		root: &commentpb.CommentInfo{Id: 9001, EntityType: "article", EntityId: 2001, Status: 1},
	}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/comments/9001/replies?page=2&page_size=5", nil)

	h.listReplies(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, commentClient.getReq)
	require.NotNil(t, contentClient.articleReq)
	require.NotNil(t, commentClient.repliesReq)
	require.EqualValues(t, 9001, commentClient.repliesReq.GetRootId())
	require.EqualValues(t, 2, commentClient.repliesReq.GetPage())
	require.EqualValues(t, 5, commentClient.repliesReq.GetPageSize())
}

func TestGetCommentRequiresPublishedTarget(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: 1},
	}
	commentClient := &fakeListCommentClient{
		root: &commentpb.CommentInfo{Id: 9001, EntityType: "article", EntityId: 2001, Status: 1},
	}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/comments/9001", nil)

	h.getComment(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, commentClient.getReq)
	require.NotNil(t, contentClient.articleReq)
}

func TestGetCommentForwardsPublishedTarget(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: contentStatusPublished},
	}
	commentClient := &fakeListCommentClient{
		root: &commentpb.CommentInfo{Id: 9001, EntityType: "article", EntityId: 2001, Status: 1},
	}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "9001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/comments/9001", nil)

	h.getComment(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, commentClient.getReq)
	require.NotNil(t, contentClient.articleReq)
}

func TestCreateTopicCommentRequiresPublishedTopic(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		topic: &contentpb.TopicInfo{Id: 1001, Status: 1},
	}
	commentClient := &fakeCreateCommentClient{}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/comments", bytes.NewBufferString(`{"content":"顶一下"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopicComment(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.topicReq)
	require.EqualValues(t, 1001, contentClient.topicReq.GetId())
	require.Nil(t, commentClient.req)
}

func TestCreateTopicCommentForwardsPublishedTopic(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		topic: &contentpb.TopicInfo{Id: 1001, Status: contentStatusPublished},
	}
	commentClient := &fakeCreateCommentClient{}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topics/1001/comments", bytes.NewBufferString(`{"content":"顶一下"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.createTopicComment(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, commentClient.req)
	require.Equal(t, "topic", commentClient.req.GetEntityType())
	require.EqualValues(t, 1001, commentClient.req.GetEntityId())
	require.EqualValues(t, 42, commentClient.req.GetAuthorId())
}

func TestCreateArticleCommentRequiresPublishedArticle(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: 1},
	}
	commentClient := &fakeCreateCommentClient{}
	h := NewHandler(&clients.Clients{
		Content: contentClient,
		Comment: commentClient,
		User:    &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}},
	}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user_id", int64(42))
	c.Params = gin.Params{{Key: "id", Value: "2001"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/articles/2001/comments", bytes.NewBufferString(`{"content":"不错"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.createComment(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.articleReq)
	require.EqualValues(t, 2001, contentClient.articleReq.GetId())
	require.Nil(t, commentClient.req)
}

type fakeCommentTargetContentClient struct {
	contentpb.ContentServiceClient
	topic      *contentpb.TopicInfo
	article    *contentpb.ArticleInfo
	topicReq   *contentpb.GetTopicRequest
	articleReq *contentpb.GetArticleRequest
}

func (f *fakeCommentTargetContentClient) GetTopic(_ context.Context, req *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	f.topicReq = req
	return &contentpb.TopicResponse{Success: true, Topic: f.topic}, nil
}

func (f *fakeCommentTargetContentClient) GetArticle(_ context.Context, req *contentpb.GetArticleRequest, _ ...grpc.CallOption) (*contentpb.ArticleResponse, error) {
	f.articleReq = req
	return &contentpb.ArticleResponse{Success: true, Article: f.article}, nil
}

type fakeCreateCommentClient struct {
	commentpb.CommentServiceClient
	req *commentpb.CreateCommentRequest
}

func (f *fakeCreateCommentClient) CreateComment(_ context.Context, req *commentpb.CreateCommentRequest, _ ...grpc.CallOption) (*commentpb.CommentResponse, error) {
	f.req = req
	return &commentpb.CommentResponse{
		Success: true,
		Comment: &commentpb.CommentInfo{
			Id:         9001,
			EntityType: req.GetEntityType(),
			EntityId:   req.GetEntityId(),
			AuthorId:   req.GetAuthorId(),
			Content:    req.GetContent(),
			Status:     1,
		},
	}, nil
}

type fakeListCommentClient struct {
	commentpb.CommentServiceClient
	listReq    *commentpb.ListCommentsRequest
	getReq     *commentpb.GetCommentRequest
	repliesReq *commentpb.ListRepliesRequest
	root       *commentpb.CommentInfo
}

func (f *fakeListCommentClient) ListComments(_ context.Context, req *commentpb.ListCommentsRequest, _ ...grpc.CallOption) (*commentpb.CommentListResponse, error) {
	f.listReq = req
	return &commentpb.CommentListResponse{Items: []*commentpb.CommentInfo{{Id: 9001, EntityType: req.GetEntityType(), EntityId: req.GetEntityId(), AuthorId: 42, Status: 1}}, Total: 1}, nil
}

func (f *fakeListCommentClient) GetComment(_ context.Context, req *commentpb.GetCommentRequest, _ ...grpc.CallOption) (*commentpb.CommentResponse, error) {
	f.getReq = req
	return &commentpb.CommentResponse{Success: true, Comment: f.root}, nil
}

func (f *fakeListCommentClient) ListReplies(_ context.Context, req *commentpb.ListRepliesRequest, _ ...grpc.CallOption) (*commentpb.CommentListResponse, error) {
	f.repliesReq = req
	return &commentpb.CommentListResponse{Items: []*commentpb.CommentInfo{{Id: 9101, RootId: req.GetRootId(), AuthorId: 43, Status: 1}}, Total: 1}, nil
}
