package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateUserListMapsOwnerAndReturnsStringIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userLists := &fakeUserListClient{createResponse: &userpb.UserListInfoResponse{
		Success:  true,
		UserList: &userpb.UserListInfo{Id: 9_223_372_036_854_775_000, OwnerId: 7, Name: "Editors", IsPublic: true},
	}}
	h := NewHandler(&clients.Clients{UserLists: userLists}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newUserListContext(stdhttp.MethodPost, "/api/v1/users/me/lists", `{"name":"  Editors  ","is_public":true}`)
	c.Set("user_id", int64(7))

	h.createUserList(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.Equal(t, int64(7), userLists.createRequest.GetOwnerId())
	require.Equal(t, "Editors", userLists.createRequest.GetName())
	require.True(t, userLists.createRequest.GetIsPublic())
	var envelope struct {
		Data struct {
			UserList userListView `json:"user_list"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9223372036854775000", envelope.Data.UserList.ID)
	require.Equal(t, "7", envelope.Data.UserList.OwnerID)
}

func TestListUserListsMapsAnonymousViewerAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userLists := &fakeUserListClient{listResponse: &userpb.UserListsResponse{Items: []*userpb.UserListInfo{{Id: 10, OwnerId: 9, Name: "Public", IsPublic: true}}, Total: 1}}
	h := NewHandler(&clients.Clients{UserLists: userLists}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newUserListContext(stdhttp.MethodGet, "/api/v1/users/9/lists?page=2&page_size=300", "")
	c.Params = gin.Params{{Key: "id", Value: "9"}}

	h.listUserLists(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.Equal(t, int64(0), userLists.listRequest.GetViewerId())
	require.Equal(t, int64(9), userLists.listRequest.GetOwnerId())
	require.Equal(t, int32(2), userLists.listRequest.GetPage())
	require.Equal(t, int32(100), userLists.listRequest.GetPageSize())
}

func TestAddUserListMemberAcceptsQuotedInt64(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userLists := &fakeUserListClient{}
	h := NewHandler(&clients.Clients{UserLists: userLists}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newUserListContext(stdhttp.MethodPost, "/api/v1/user-lists/10/members", `{"user_id":"9223372036854775000"}`)
	c.Params = gin.Params{{Key: "id", Value: "10"}}
	c.Set("user_id", int64(7))

	h.addUserListMember(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.Equal(t, int64(7), userLists.addMemberRequest.GetOwnerId())
	require.Equal(t, int64(10), userLists.addMemberRequest.GetListId())
	require.Equal(t, int64(9_223_372_036_854_775_000), userLists.addMemberRequest.GetUserId())
}

func TestCreateUserListConflictMapsToHTTPConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userLists := &fakeUserListClient{createErr: status.Error(codes.AlreadyExists, "user list name already exists")}
	h := NewHandler(&clients.Clients{UserLists: userLists}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newUserListContext(stdhttp.MethodPost, "/api/v1/users/me/lists", `{"name":"Editors"}`)
	c.Set("user_id", int64(7))

	h.createUserList(c)

	require.Equal(t, stdhttp.StatusConflict, recorder.Code)
}

func TestUserListFeedFiltersHiddenMembersBeforeFeedQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userLists := &fakeUserListClient{
		getResponse:     &userpb.UserListInfoResponse{Success: true, UserList: &userpb.UserListInfo{Id: 10, OwnerId: 7, IsPublic: true}},
		membersResponse: &userpb.UserListResponse{Items: []*userpb.UserInfo{{Id: 10}, {Id: 20}, {Id: 30}}, Total: 3},
	}
	feed := &capturingFeedClient{response: &feedpb.FeedListResponse{Items: []*feedpb.FeedItem{{Id: 101, AuthorId: 10}}}}
	h := NewHandler(&clients.Clients{
		UserLists:  userLists,
		UserSafety: &fakeUserSafetyClient{blocked: []*userpb.UserInfo{{Id: 20}}, muted: []*userpb.UserInfo{{Id: 30}}},
		Feed:       feed,
	}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newUserListContext(stdhttp.MethodGet, "/api/v1/user-lists/10/feed?limit=5&offset=2", "")
	c.Params = gin.Params{{Key: "id", Value: "10"}}
	c.Set("user_id", int64(1))

	h.userListFeed(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.Equal(t, int64(1), userLists.getRequest.GetViewerId())
	require.Equal(t, []int64{10}, feed.request.GetAuthorIds())
	require.Equal(t, int32(5), feed.request.GetLimit())
	require.Equal(t, int32(2), feed.request.GetOffset())
}

func TestUserListFeedStopsWhenListIsNotVisible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userLists := &fakeUserListClient{getErr: status.Error(codes.NotFound, "user list not found")}
	feed := &capturingFeedClient{}
	h := NewHandler(&clients.Clients{UserLists: userLists, Feed: feed}, "Authorization", "Bearer", testJWTSecret)
	c, recorder := newUserListContext(stdhttp.MethodGet, "/api/v1/user-lists/10/feed", "")
	c.Params = gin.Params{{Key: "id", Value: "10"}}

	h.userListFeed(c)

	require.Equal(t, stdhttp.StatusNotFound, recorder.Code)
	require.Nil(t, feed.request)
}

func newUserListContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	return c, recorder
}

type fakeUserListClient struct {
	userpb.UserServiceClient
	createRequest    *userpb.CreateUserListRequest
	createResponse   *userpb.UserListInfoResponse
	createErr        error
	listRequest      *userpb.ListUserListsRequest
	listResponse     *userpb.UserListsResponse
	getRequest       *userpb.GetUserListRequest
	getResponse      *userpb.UserListInfoResponse
	getErr           error
	addMemberRequest *userpb.UserListMemberRequest
	membersRequest   *userpb.ListUserListMembersRequest
	membersResponse  *userpb.UserListResponse
}

func (f *fakeUserListClient) CreateUserList(_ context.Context, req *userpb.CreateUserListRequest, _ ...grpc.CallOption) (*userpb.UserListInfoResponse, error) {
	f.createRequest = req
	if f.createResponse == nil && f.createErr == nil {
		f.createResponse = &userpb.UserListInfoResponse{Success: true}
	}
	return f.createResponse, f.createErr
}

func (f *fakeUserListClient) ListUserLists(_ context.Context, req *userpb.ListUserListsRequest, _ ...grpc.CallOption) (*userpb.UserListsResponse, error) {
	f.listRequest = req
	if f.listResponse == nil {
		f.listResponse = &userpb.UserListsResponse{}
	}
	return f.listResponse, nil
}

func (f *fakeUserListClient) GetUserList(_ context.Context, req *userpb.GetUserListRequest, _ ...grpc.CallOption) (*userpb.UserListInfoResponse, error) {
	f.getRequest = req
	if f.getResponse == nil && f.getErr == nil {
		f.getResponse = &userpb.UserListInfoResponse{Success: true}
	}
	return f.getResponse, f.getErr
}

func (f *fakeUserListClient) AddUserListMember(_ context.Context, req *userpb.UserListMemberRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	f.addMemberRequest = req
	return &userpb.SimpleResponse{Success: true}, nil
}

func (f *fakeUserListClient) ListUserListMembers(_ context.Context, req *userpb.ListUserListMembersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	f.membersRequest = req
	if f.membersResponse == nil {
		f.membersResponse = &userpb.UserListResponse{}
	}
	return f.membersResponse, nil
}

type capturingFeedClient struct {
	feedpb.FeedServiceClient
	request  *feedpb.ListFeedRequest
	response *feedpb.FeedListResponse
}

func (f *capturingFeedClient) ListLatest(_ context.Context, req *feedpb.ListFeedRequest, _ ...grpc.CallOption) (*feedpb.FeedListResponse, error) {
	f.request = req
	if f.response == nil {
		f.response = &feedpb.FeedListResponse{}
	}
	return f.response, nil
}
