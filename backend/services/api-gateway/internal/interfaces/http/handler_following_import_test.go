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

func TestImportFollowingReadsLocalAccountsAndAppliesFollow(t *testing.T) {
	payload := []byte("\ufeffalice@bbs.example.com:8443,ignored\r\nremote@example.net\nALICE@bbs.example.com:8443\nbob@bbs.example.com:8443\nowner@bbs.example.com:8443\nmissing@bbs.example.com:8443\n")
	store := newFakeUserFileStore()
	store.objects["files/42/following.csv"] = fakeUserFileObject{data: payload, contentType: "text/csv"}
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
		Id: 91, OwnerId: 42, ObjectKey: "files/42/following.csv", SizeBytes: int64(len(payload)), Status: "ACTIVE",
	}}}
	users := &followingImportUserStub{users: map[string]*userpb.UserInfo{
		"alice": {Id: 71, Username: "alice", Status: userStatusActive, AccountState: "active"},
		"bob":   {Id: 72, Username: "bob", Status: userStatusActive, AccountState: "active"},
		"owner": {Id: 42, Username: "owner", Status: userStatusActive, AccountState: "active"},
	}}
	limiter := &safetyImportLimiterStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, User: users}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("https://bbs.example.com:8443")
	h.SetFollowingImportLimit(limiter)

	recorder := performFollowingImport(h, `{"fileId":"91","withReplies":true}`, 42)

	require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Equal(t, []string{"rate:imports:following:user:42"}, limiter.keys)
	require.Equal(t, int64(42), files.getReq.GetOwnerId())
	require.Equal(t, []string{"files/42/following.csv"}, store.openKeys)
	require.Len(t, users.requests, 1)
	require.ElementsMatch(t, []string{"alice", "bob", "owner", "missing"}, users.requests[0].GetUsernames())
	require.ElementsMatch(t, []int64{71, 72}, users.followedIDs())
	for _, request := range users.followRequests() {
		require.Equal(t, int64(42), request.GetFollowerId())
		require.NotNil(t, request.WithReplies)
		require.True(t, request.GetWithReplies())
	}
}

func TestApplyFollowingImportSkipsIdempotentErrors(t *testing.T) {
	users := &followingImportUserStub{errors: map[int64]error{
		71: status.Error(codes.AlreadyExists, "already following"),
		72: status.Error(codes.NotFound, "gone"),
		73: status.Error(codes.FailedPrecondition, "blocked"),
	}}
	h := NewHandler(&clients.Clients{User: users}, "Authorization", "Bearer", testJWTSecret)

	require.NoError(t, h.applyFollowingImport(context.Background(), 42, []int64{71, 72, 73}))

	users.errors[74] = status.Error(codes.Unavailable, "database unavailable")
	err := h.applyFollowingImport(context.Background(), 42, []int64{74})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestImportFollowingRejectsOversizedFileBeforeOpeningObject(t *testing.T) {
	store := newFakeUserFileStore()
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
		Id: 7, OwnerId: 42, ObjectKey: "large.csv", SizeBytes: safetyImportMaxBytes + 1,
	}}}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		File: files, User: &followingImportUserStub{},
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("https://bbs.example.com")

	recorder := performFollowingImport(h, `{"fileId":"7"}`, 42)

	require.Equal(t, stdhttp.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
	require.Empty(t, store.openKeys)
}

func TestImportFollowingRoutesRequireInteractiveSessionAndApplyRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/i/import-following", "/api/i/import-following", "/api/v1/i/import-following"} {
		t.Run(path, func(t *testing.T) {
			limiter := &safetyImportLimiterStub{limited: true}
			h := newFollowingImportTestHandler()
			h.SetFollowingImportLimit(limiter)
			router := gin.New()
			NewInitControllers(h)(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{"fileId":"7","withReplies":false}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{"rate:imports:following:user:42"}, limiter.keys)
		})
	}

	limiter := &safetyImportLimiterStub{limited: true}
	h := newFollowingImportTestHandler()
	h.SetFollowingImportLimit(limiter)
	router := gin.New()
	NewInitControllers(h)(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/import-following", strings.NewReader(`{"fileId":"7"}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "following-import-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"write"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Empty(t, limiter.keys)
}

func newFollowingImportTestHandler() *Handler {
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		File: &fakeUserFileClient{}, User: &followingImportUserStub{},
	}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
	h.SetPublicBaseURL("https://bbs.example.com")
	return h
}

func performFollowingImport(h *Handler, body string, ownerID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/import-following", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", ownerID)
	h.importFollowing(c)
	c.Writer.WriteHeaderNow()
	return recorder
}

type followingImportUserStub struct {
	clients.UserClient
	users    map[string]*userpb.UserInfo
	requests []*userpb.ListUsersRequest
	follows  []*userpb.FollowRequest
	errors   map[int64]error
	mu       sync.Mutex
}

func (s *followingImportUserStub) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
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

func (s *followingImportUserStub) Follow(_ context.Context, request *userpb.FollowRequest, _ ...grpc.CallOption) (*userpb.FollowResponse, error) {
	s.mu.Lock()
	s.follows = append(s.follows, request)
	err := s.errors[request.GetFolloweeId()]
	s.mu.Unlock()
	return &userpb.FollowResponse{Success: err == nil}, err
}

func (s *followingImportUserStub) followedIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]int64, 0, len(s.follows))
	for _, request := range s.follows {
		result = append(result, request.GetFolloweeId())
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (s *followingImportUserStub) followRequests() []*userpb.FollowRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*userpb.FollowRequest(nil), s.follows...)
}
