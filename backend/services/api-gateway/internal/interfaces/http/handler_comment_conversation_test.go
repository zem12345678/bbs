package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCommentConversationCanonicalRouteForwardsPagination(t *testing.T) {
	commentClient := newCommentConversationHTTPClient()
	router := commentConversationTestRouter(commentClient, publishedConversationContentClient(), &fakeUserClient{})

	response := performCommentConversationRequest(router, stdhttp.MethodGet, "/api/v1/comments/9003/conversation?limit=2&offset=1", "")

	require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
	require.NotNil(t, commentClient.getRequest)
	require.EqualValues(t, 9003, commentClient.getRequest.GetId())
	require.NotNil(t, commentClient.conversationRequest)
	require.EqualValues(t, 9003, commentClient.conversationRequest.GetCommentId())
	require.EqualValues(t, 2, commentClient.conversationRequest.GetLimit())
	require.EqualValues(t, 1, commentClient.conversationRequest.GetOffset())
	var envelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 2)
}

func TestCommentConversationCanonicalRouteRejectsInvalidPagination(t *testing.T) {
	for _, path := range []string{
		"/api/v1/comments/9003/conversation?limit=0",
		"/api/v1/comments/9003/conversation?limit=101",
		"/api/v1/comments/9003/conversation?offset=-1",
		"/api/v1/comments/9003/conversation?offset=10001",
		"/api/v1/comments/9003/conversation?limit=invalid",
	} {
		t.Run(path, func(t *testing.T) {
			commentClient := newCommentConversationHTTPClient()
			router := commentConversationTestRouter(commentClient, publishedConversationContentClient(), &fakeUserClient{})
			response := performCommentConversationRequest(router, stdhttp.MethodGet, path, "")
			require.Equal(t, stdhttp.StatusBadRequest, response.Code, response.Body.String())
			require.Nil(t, commentClient.getRequest)
			require.Nil(t, commentClient.conversationRequest)
		})
	}
}

func TestCommentConversationRequiresPublishedTarget(t *testing.T) {
	commentClient := newCommentConversationHTTPClient()
	contentClient := &fakeCommentTargetContentClient{article: &contentpb.ArticleInfo{Id: 2001, Status: 1}}
	router := commentConversationTestRouter(commentClient, contentClient, &fakeUserClient{})

	response := performCommentConversationRequest(router, stdhttp.MethodGet, "/api/v1/comments/9003/conversation", "")

	require.Equal(t, stdhttp.StatusNotFound, response.Code, response.Body.String())
	require.NotNil(t, commentClient.getRequest)
	require.Nil(t, commentClient.conversationRequest)
}

func TestCommentConversationCompatibilityRoutesReturnMisskeyNoteArray(t *testing.T) {
	for _, path := range []string{"/api/notes/conversation", "/notes/conversation"} {
		t.Run(path, func(t *testing.T) {
			commentClient := newCommentConversationHTTPClient()
			userClient := &fakeUserClient{users: []*userpb.UserInfo{
				{Id: 42, Username: "root", Nickname: "Root User", AvatarUrl: "https://cdn.example/root.png", Bio: "root bio", Status: userStatusActive, CreatedAt: 1700000000000},
				{Id: 43, Username: "parent", Nickname: "Parent User", Status: userStatusActive, CreatedAt: 1700000001000},
			}}
			router := commentConversationTestRouter(commentClient, publishedConversationContentClient(), userClient)

			response := performCommentConversationRequest(router, stdhttp.MethodPost, path, `{"noteId":"9003"}`)

			require.Equal(t, stdhttp.StatusOK, response.Code, response.Body.String())
			require.NotNil(t, commentClient.conversationRequest)
			require.EqualValues(t, 10, commentClient.conversationRequest.GetLimit())
			require.Zero(t, commentClient.conversationRequest.GetOffset())
			require.NotNil(t, userClient.listUsersReq)
			require.ElementsMatch(t, []int64{42, 43}, userClient.listUsersReq.GetIds())
			var notes []map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &notes))
			require.Len(t, notes, 2)
			require.Equal(t, "9002", notes[0]["id"])
			require.Equal(t, "9001", notes[0]["threadId"])
			require.Equal(t, "42", notes[0]["userId"])
			require.Equal(t, "9001", notes[0]["replyId"])
			require.Equal(t, "public", notes[0]["visibility"])
			require.Equal(t, float64(3), notes[0]["reactionCount"])
			require.Equal(t, float64(1), notes[0]["repliesCount"])
			require.NotContains(t, notes[0], "data")
			for _, required := range []string{"createdAt", "text", "userHost", "user", "mentions", "visibleUserIds", "fileIds", "files", "tags", "isMutingThread", "isMutingNote", "isFavorited", "isRenoted", "bypassSilence", "emojis", "reactionAcceptance", "reactionEmojis", "reactions", "reactionCount", "renoteCount", "viewsCount"} {
				require.Contains(t, notes[0], required)
			}
			user := notes[0]["user"].(map[string]any)
			for _, required := range []string{"id", "name", "username", "host", "createdAt", "approved", "avatarDecorations", "followersCount", "followingCount", "notesCount", "level", "emojis", "onlineStatus", "attributionDomains"} {
				require.Contains(t, user, required)
			}
			require.Nil(t, notes[1]["replyId"])
		})
	}
}

func TestCommentConversationCompatibilityRejectsInvalidBodyAndRange(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"noteId":"invalid"}`,
		`{"noteId":"9003","limit":0}`,
		`{"noteId":"9003","limit":101}`,
		`{"noteId":"9003","offset":-1}`,
		`{"noteId":"9003","offset":10001}`,
	} {
		t.Run(body, func(t *testing.T) {
			commentClient := newCommentConversationHTTPClient()
			router := commentConversationTestRouter(commentClient, publishedConversationContentClient(), &fakeUserClient{})
			response := performCommentConversationRequest(router, stdhttp.MethodPost, "/api/notes/conversation", body)
			require.Equal(t, stdhttp.StatusBadRequest, response.Code, response.Body.String())
			require.Nil(t, commentClient.getRequest)
			require.Nil(t, commentClient.conversationRequest)
		})
	}
}

func TestCommentConversationPropagatesRPCError(t *testing.T) {
	commentClient := newCommentConversationHTTPClient()
	commentClient.conversationError = status.Error(codes.InvalidArgument, "invalid parent chain")
	router := commentConversationTestRouter(commentClient, publishedConversationContentClient(), &fakeUserClient{})

	response := performCommentConversationRequest(router, stdhttp.MethodGet, "/api/v1/comments/9003/conversation", "")

	require.Equal(t, stdhttp.StatusBadRequest, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "invalid parent chain")
}

func commentConversationTestRouter(commentClient commentpb.CommentServiceClient, contentClient contentpb.ContentServiceClient, userClient clients.UserClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&clients.Clients{Comment: commentClient, Content: contentClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(handler)(router)
	return router
}

func publishedConversationContentClient() *fakeCommentTargetContentClient {
	return &fakeCommentTargetContentClient{article: &contentpb.ArticleInfo{Id: 2001, Status: contentStatusPublished}}
}

func performCommentConversationRequest(router stdhttp.Handler, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

type commentConversationHTTPClient struct {
	commentpb.CommentServiceClient
	getRequest          *commentpb.GetCommentRequest
	conversationRequest *commentpb.GetCommentConversationRequest
	conversationError   error
}

func newCommentConversationHTTPClient() *commentConversationHTTPClient {
	return &commentConversationHTTPClient{}
}

func (c *commentConversationHTTPClient) GetComment(_ context.Context, req *commentpb.GetCommentRequest, _ ...grpc.CallOption) (*commentpb.CommentResponse, error) {
	c.getRequest = req
	return &commentpb.CommentResponse{Success: true, Comment: &commentpb.CommentInfo{
		Id: 9003, EntityType: "article", EntityId: 2001, RootId: 9001, ParentId: 9002, AuthorId: 44, Status: 1,
	}}, nil
}

func (c *commentConversationHTTPClient) GetCommentConversation(_ context.Context, req *commentpb.GetCommentConversationRequest, _ ...grpc.CallOption) (*commentpb.CommentListResponse, error) {
	c.conversationRequest = req
	if c.conversationError != nil {
		return nil, c.conversationError
	}
	return &commentpb.CommentListResponse{Items: []*commentpb.CommentInfo{
		{Id: 9002, EntityType: "article", EntityId: 2001, RootId: 9001, ParentId: 9001, AuthorId: 42, Content: "parent", Status: 1, ReplyCount: 1, LikeCount: 3, CreatedAt: 1700000000000},
		{Id: 9001, EntityType: "article", EntityId: 2001, AuthorId: 43, Content: "root", Status: 1, CreatedAt: 1700000001000},
	}, Total: 2}, nil
}
