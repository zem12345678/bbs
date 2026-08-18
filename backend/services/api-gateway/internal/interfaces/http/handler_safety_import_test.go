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

func TestImportSafetyRelationsReadsLocalAccountsAndAppliesSelectedRelation(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		blocking bool
		wantKey  string
	}{
		{name: "blocking", blocking: true, wantKey: "rate:imports:blocking:user:42"},
		{name: "muting", blocking: false, wantKey: "rate:imports:muting:user:42"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			payload := []byte("\ufeffalice@bbs.example.com:8443,ignored\r\nremote@example.net\nALICE@bbs.example.com:8443\nbob@bbs.example.com:8443\nowner@bbs.example.com:8443\nmissing@bbs.example.com:8443\n")
			store := newFakeUserFileStore()
			store.objects["files/42/safety.csv"] = fakeUserFileObject{data: payload, contentType: "text/csv"}
			files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
				Id: 91, OwnerId: 42, ObjectKey: "files/42/safety.csv", SizeBytes: int64(len(payload)), Status: "ACTIVE",
			}}}
			users := &safetyImportUserClientStub{users: map[string]*userpb.UserInfo{
				"alice": {Id: 71, Username: "alice", Status: userStatusActive, AccountState: "active"},
				"bob":   {Id: 72, Username: "bob", Status: userStatusActive, AccountState: "active"},
				"owner": {Id: 42, Username: "owner", Status: userStatusActive, AccountState: "active"},
			}}
			safety := &safetyImportMutationStub{}
			limiter := &safetyImportLimiterStub{}
			h := NewHandlerWithAttachmentStore(&clients.Clients{File: files, User: users, UserSafety: safety}, "Authorization", "Bearer", testJWTSecret, store)
			h.SetPublicBaseURL("https://bbs.example.com:8443")
			if testCase.blocking {
				h.SetBlockingImportLimit(limiter)
			} else {
				h.SetMutingImportLimit(limiter)
			}

			recorder := performSafetyImport(h, `{"fileId":"91"}`, 42, testCase.blocking)

			require.Equal(t, stdhttp.StatusNoContent, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{testCase.wantKey}, limiter.keys)
			require.Equal(t, int64(42), files.getReq.GetOwnerId())
			require.Equal(t, []string{"files/42/safety.csv"}, store.openKeys)
			require.Len(t, users.requests, 1)
			require.ElementsMatch(t, []string{"alice", "bob", "owner", "missing"}, users.requests[0].GetUsernames())
			require.Equal(t, int32(userStatusActive), users.requests[0].GetStatus())
			if testCase.blocking {
				require.Equal(t, []int64{71, 72}, safety.blockedIDs())
				require.Empty(t, safety.mutedIDs())
			} else {
				require.Equal(t, []int64{71, 72}, safety.mutedIDs())
				require.Empty(t, safety.blockedIDs())
			}
		})
	}
}

func TestSafetyImportUsernamesSkipsInvalidRemoteAndDuplicateRows(t *testing.T) {
	payload := []byte("  \ufeff@Alice@bbs.example.com\r\ninvalid\nremote@example.net\nfoo@bar@bbs.example.com\n\"Bob@BBS.EXAMPLE.COM\",extra\n\"unterminated\nalice@bbs.example.com\n")

	usernames, err := safetyImportUsernames(payload, "bbs.example.com")

	require.NoError(t, err)
	require.Equal(t, []string{"Alice", "Bob"}, usernames)
}

func TestResolveSafetyImportTargetIDsBatchesExactUsernames(t *testing.T) {
	usernames := make([]string, 0, 101)
	users := make(map[string]*userpb.UserInfo, 101)
	for index := 0; index < 101; index++ {
		username := "member" + strings.Repeat("x", index/26) + string(rune('a'+index%26))
		usernames = append(usernames, username)
		users[strings.ToLower(username)] = &userpb.UserInfo{Id: int64(index + 100), Username: username, Status: userStatusActive, AccountState: "active"}
	}
	client := &safetyImportUserClientStub{users: users}
	h := NewHandler(&clients.Clients{User: client}, "Authorization", "Bearer", testJWTSecret)

	targetIDs, err := h.resolveSafetyImportTargetIDs(context.Background(), 42, usernames)

	require.NoError(t, err)
	require.Len(t, targetIDs, 101)
	require.Len(t, client.requests, 2)
	require.Len(t, client.requests[0].GetUsernames(), 100)
	require.Len(t, client.requests[1].GetUsernames(), 1)
	require.Equal(t, int32(100), client.requests[0].GetPageSize())
	require.Equal(t, int32(1), client.requests[1].GetPageSize())
}

func TestApplySafetyImportSkipsIdempotentErrorsAndReturnsInfrastructureFailure(t *testing.T) {
	safety := &safetyImportMutationStub{errors: map[int64]error{
		71: status.Error(codes.AlreadyExists, "already muted"),
		72: status.Error(codes.NotFound, "gone"),
		73: status.Error(codes.FailedPrecondition, "inactive"),
	}}
	h := NewHandler(&clients.Clients{UserSafety: safety}, "Authorization", "Bearer", testJWTSecret)
	require.NoError(t, h.applySafetyImport(context.Background(), 42, []int64{71, 72, 73}, false))

	failing := &safetyImportMutationStub{errors: map[int64]error{74: status.Error(codes.Unavailable, "database unavailable")}}
	h.clients.UserSafety = failing
	err := h.applySafetyImport(context.Background(), 42, []int64{74}, true)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestImportSafetyRelationsRejectsOversizedFileBeforeOpeningObject(t *testing.T) {
	store := newFakeUserFileStore()
	files := &fakeUserFileClient{getResp: &filepb.FileResponse{File: &filepb.File{
		Id: 7, OwnerId: 42, ObjectKey: "large.csv", SizeBytes: safetyImportMaxBytes + 1,
	}}}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		File: files, User: &safetyImportUserClientStub{}, UserSafety: &safetyImportMutationStub{},
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetPublicBaseURL("https://bbs.example.com")

	recorder := performSafetyImport(h, `{"fileId":"7"}`, 42, true)

	require.Equal(t, stdhttp.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
	require.Empty(t, store.openKeys)
}

func TestSafetyImportRoutesRequireInteractiveSessionAndUseIndependentLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []struct {
		path     string
		blocking bool
		wantKey  string
	}{
		{path: "/i/import-blocking", blocking: true, wantKey: "rate:imports:blocking:user:42"},
		{path: "/api/i/import-blocking", blocking: true, wantKey: "rate:imports:blocking:user:42"},
		{path: "/api/v1/i/import-blocking", blocking: true, wantKey: "rate:imports:blocking:user:42"},
		{path: "/i/import-muting", wantKey: "rate:imports:muting:user:42"},
		{path: "/api/i/import-muting", wantKey: "rate:imports:muting:user:42"},
		{path: "/api/v1/i/import-muting", wantKey: "rate:imports:muting:user:42"},
	} {
		t.Run(endpoint.path, func(t *testing.T) {
			limiter := &safetyImportLimiterStub{limited: true}
			h := newSafetyImportTestHandler()
			if endpoint.blocking {
				h.SetBlockingImportLimit(limiter)
			} else {
				h.SetMutingImportLimit(limiter)
			}
			router := gin.New()
			NewInitControllers(h)(router)
			request := httptest.NewRequest(stdhttp.MethodPost, endpoint.path, strings.NewReader(`{"fileId":"7"}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
			require.Equal(t, []string{endpoint.wantKey}, limiter.keys)
		})
	}

	limiter := &safetyImportLimiterStub{limited: true}
	h := newSafetyImportTestHandler()
	h.SetBlockingImportLimit(limiter)
	router := gin.New()
	NewInitControllers(h)(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/import-blocking", strings.NewReader(`{"fileId":"7"}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "safety-import-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"write"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Empty(t, limiter.keys)
}

func newSafetyImportTestHandler() *Handler {
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		File: &fakeUserFileClient{}, User: &safetyImportUserClientStub{}, UserSafety: &safetyImportMutationStub{},
	}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
	h.SetPublicBaseURL("https://bbs.example.com")
	return h
}

func performSafetyImport(h *Handler, body string, ownerID int64, blocking bool) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/import-safety", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", ownerID)
	h.importSafetyRelations(c, blocking)
	c.Writer.WriteHeaderNow()
	return recorder
}

type safetyImportUserClientStub struct {
	clients.UserClient
	users    map[string]*userpb.UserInfo
	requests []*userpb.ListUsersRequest
	mu       sync.Mutex
}

func (s *safetyImportUserClientStub) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
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

type safetyImportMutationStub struct {
	clients.UserSafetyClient
	blocked []int64
	muted   []int64
	errors  map[int64]error
	mu      sync.Mutex
}

func (s *safetyImportMutationStub) Block(_ context.Context, request *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocked = append(s.blocked, request.GetTargetId())
	return &userpb.SimpleResponse{Success: true}, s.errors[request.GetTargetId()]
}

func (s *safetyImportMutationStub) Mute(_ context.Context, request *userpb.UserRelationRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.muted = append(s.muted, request.GetTargetId())
	return &userpb.SimpleResponse{Success: true}, s.errors[request.GetTargetId()]
}

func (s *safetyImportMutationStub) blockedIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := append([]int64(nil), s.blocked...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (s *safetyImportMutationStub) mutedIDs() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := append([]int64(nil), s.muted...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

type safetyImportLimiterStub struct {
	limited bool
	err     error
	keys    []string
}

func (s *safetyImportLimiterStub) Limit(_ context.Context, key string) (bool, error) {
	s.keys = append(s.keys, key)
	return s.limited, s.err
}
