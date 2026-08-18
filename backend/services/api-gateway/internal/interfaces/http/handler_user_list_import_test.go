package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestParseUserListImportRecordsKeepsLocalUniquePairs(t *testing.T) {
	payload := []byte("\ufeffFriends,alice@bbs.example.com\r\nFriends,bob@bbs.example.com\n" +
		"friends,ALICE@bbs.example.com\nRemote,carol@example.net\n\"Comma, list\",dave@bbs.example.com\n" +
		"Unquoted, comma list,erin@bbs.example.com\nbad\n")

	records, err := parseUserListImportRecords(payload, "bbs.example.com")

	require.NoError(t, err)
	require.Equal(t, []userListImportRecord{
		{Name: "Friends", Username: "alice"},
		{Name: "Friends", Username: "bob"},
		{Name: "Comma, list", Username: "dave"},
		{Name: "Unquoted, comma list", Username: "erin"},
	}, records)
}

func TestImportUserListsCreatesListsAndAddsResolvedLocalMembers(t *testing.T) {
	payload := []byte("Friends,alice@bbs.example.com\nFriends,bob@bbs.example.com\nNew list,carol@bbs.example.com\nRemote,remote@example.net\nUnknown,missing@bbs.example.com\n")
	store := newFakeUserFileStore()
	store.objects["imports/42/lists.csv"] = fakeUserFileObject{data: payload, contentType: "text/csv"}
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
		Id: 91, OwnerId: 42, ObjectKey: "imports/42/lists.csv", SizeBytes: int64(len(payload)), Status: "ACTIVE",
	}}}
	users := &userListImportUserStub{users: map[string]*userpb.UserInfo{
		"alice": {Id: 71, Username: "alice", Status: userStatusActive, AccountState: "active"},
		"bob":   {Id: 72, Username: "bob", Status: userStatusActive, AccountState: "active"},
		"carol": {Id: 73, Username: "carol", Status: userStatusActive, AccountState: "active"},
	}}
	lists := &userListImportClientStub{lists: []*userpb.UserListInfo{{Id: 10, OwnerId: 42, Name: "Friends"}}, nextID: 20}
	limiter := &safetyImportLimiterStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, User: users, UserLists: lists}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("https://bbs.example.com")
	h.SetUserListImportLimit(limiter)

	recorder := performUserListImport(h, `{"fileId":"91"}`, 42)

	require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Equal(t, []string{"rate:imports:user-lists:user:42"}, limiter.keys)
	require.Equal(t, int64(42), files.getReq.GetOwnerId())
	require.Equal(t, []string{"imports/42/lists.csv"}, store.openKeys)
	require.Len(t, users.requests, 1)
	require.ElementsMatch(t, []string{"alice", "bob", "carol", "missing"}, users.requests[0].GetUsernames())
	require.Equal(t, []string{"New list"}, lists.createdNames())
	require.Equal(t, []int64{71, 72, 73}, lists.addedUserIDs())
	for _, request := range lists.addRequests {
		require.Equal(t, int64(42), request.GetOwnerId())
	}
}

func TestImportUserListsSkipsIdempotentMemberErrors(t *testing.T) {
	payload := []byte("Friends,alice@bbs.example.com\nFriends,bob@bbs.example.com\nFriends,carol@bbs.example.com\n")
	store := newFakeUserFileStore()
	store.objects["lists.csv"] = fakeUserFileObject{data: payload}
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "lists.csv", SizeBytes: int64(len(payload))}}}
	users := &userListImportUserStub{users: map[string]*userpb.UserInfo{
		"alice": {Id: 71, Username: "alice", Status: userStatusActive, AccountState: "active"},
		"bob":   {Id: 72, Username: "bob", Status: userStatusActive, AccountState: "active"},
		"carol": {Id: 73, Username: "carol", Status: userStatusActive, AccountState: "active"},
	}}
	lists := &userListImportClientStub{
		lists: []*userpb.UserListInfo{{Id: 10, OwnerId: 42, Name: "Friends"}},
		addErrors: map[int64]error{
			71: status.Error(codes.AlreadyExists, "already a member"),
			72: status.Error(codes.NotFound, "user disappeared"),
			73: status.Error(codes.FailedPrecondition, "member blocked"),
		},
	}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, User: users, UserLists: lists}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("https://bbs.example.com")

	recorder := performUserListImport(h, `{"fileId":"7"}`, 42)

	require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{71, 72, 73}, lists.addedUserIDs())
}

func TestImportUserListsReturnsInfrastructureErrors(t *testing.T) {
	payload := []byte("Friends,alice@bbs.example.com\n")
	store := newFakeUserFileStore()
	store.objects["lists.csv"] = fakeUserFileObject{data: payload}
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{Id: 7, OwnerId: 42, ObjectKey: "lists.csv", SizeBytes: int64(len(payload))}}}
	users := &userListImportUserStub{users: map[string]*userpb.UserInfo{
		"alice": {Id: 71, Username: "alice", Status: userStatusActive, AccountState: "active"},
	}}
	lists := &userListImportClientStub{lists: []*userpb.UserListInfo{{Id: 10, OwnerId: 42, Name: "Friends"}}, addErrors: map[int64]error{
		71: status.Error(codes.Unavailable, "user service unavailable"),
	}}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, User: users, UserLists: lists}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("https://bbs.example.com")

	recorder := performUserListImport(h, `{"fileId":"7"}`, 42)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
}

func TestImportUserListRoutesRequireInteractiveSessionAndUseRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/i/import-user-lists", "/api/i/import-user-lists", "/api/v1/i/import-user-lists"} {
		t.Run(path, func(t *testing.T) {
			limiter := &safetyImportLimiterStub{limited: true}
			h := newUserListImportTestHandler()
			h.SetUserListImportLimit(limiter)
			router := gin.New()
			NewInitControllers(h)(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{"fileId":"7"}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{"rate:imports:user-lists:user:42"}, limiter.keys)
		})
	}

	limiter := &safetyImportLimiterStub{limited: true}
	h := newUserListImportTestHandler()
	h.SetUserListImportLimit(limiter)
	router := gin.New()
	NewInitControllers(h)(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/import-user-lists", strings.NewReader(`{"fileId":"7"}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "user-list-import-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"write"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Empty(t, limiter.keys)
}

func newUserListImportTestHandler() *Handler {
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		File: &fakeUserFileClient{}, User: &userListImportUserStub{}, UserLists: &userListImportClientStub{},
	}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
	h.SetPublicBaseURL("https://bbs.example.com")
	return h
}

func performUserListImport(h *Handler, body string, ownerID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/import-user-lists", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", ownerID)
	h.importUserLists(c)
	c.Writer.WriteHeaderNow()
	return recorder
}

type userListImportUserStub struct {
	clients.UserClient
	users    map[string]*userpb.UserInfo
	requests []*userpb.ListUsersRequest
	mu       sync.Mutex
}

func (s *userListImportUserStub) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	s.mu.Lock()
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	items := make([]*userpb.UserInfo, 0, len(request.GetUsernames()))
	for _, username := range request.GetUsernames() {
		if user := s.users[strings.ToLower(username)]; user != nil {
			items = append(items, user)
		}
	}
	return &userpb.UserListResponse{Items: items, Total: int64(len(items))}, nil
}

type userListImportClientStub struct {
	clients.UserListClient
	lists       []*userpb.UserListInfo
	listPages   []int32
	createReqs  []*userpb.CreateUserListRequest
	addRequests []*userpb.UserListMemberRequest
	createErr   error
	addErrors   map[int64]error
	nextID      int64
}

func (s *userListImportClientStub) ListUserLists(_ context.Context, request *userpb.ListUserListsRequest, _ ...grpc.CallOption) (*userpb.UserListsResponse, error) {
	s.listPages = append(s.listPages, request.GetPage())
	start := int(request.GetPage()-1) * int(request.GetPageSize())
	if start >= len(s.lists) {
		return &userpb.UserListsResponse{Total: int64(len(s.lists))}, nil
	}
	end := min(len(s.lists), start+int(request.GetPageSize()))
	return &userpb.UserListsResponse{Items: s.lists[start:end], Total: int64(len(s.lists))}, nil
}

func (s *userListImportClientStub) CreateUserList(_ context.Context, request *userpb.CreateUserListRequest, _ ...grpc.CallOption) (*userpb.UserListInfoResponse, error) {
	s.createReqs = append(s.createReqs, request)
	if s.createErr != nil {
		return nil, s.createErr
	}
	for _, list := range s.lists {
		if strings.EqualFold(list.GetName(), request.GetName()) {
			return nil, status.Error(codes.AlreadyExists, "list already exists")
		}
	}
	s.nextID++
	list := &userpb.UserListInfo{Id: s.nextID, OwnerId: request.GetOwnerId(), Name: request.GetName(), IsPublic: request.GetIsPublic()}
	s.lists = append(s.lists, list)
	return &userpb.UserListInfoResponse{Success: true, UserList: list}, nil
}

func (s *userListImportClientStub) AddUserListMember(_ context.Context, request *userpb.UserListMemberRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	s.addRequests = append(s.addRequests, request)
	if err := s.addErrors[request.GetUserId()]; err != nil {
		return nil, err
	}
	return &userpb.SimpleResponse{Success: true}, nil
}

func (s *userListImportClientStub) createdNames() []string {
	result := make([]string, 0, len(s.createReqs))
	for _, request := range s.createReqs {
		result = append(result, request.GetName())
	}
	return result
}

func (s *userListImportClientStub) addedUserIDs() []int64 {
	result := make([]int64, 0, len(s.addRequests))
	for _, request := range s.addRequests {
		result = append(result, request.GetUserId())
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
