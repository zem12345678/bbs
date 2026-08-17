package http

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestBuildFollowingExportUsesExclusiveKeysetAndAppliesOptions(t *testing.T) {
	users := make([]*userpb.UserInfo, 0, 101)
	for id := int64(1); id <= 101; id++ {
		users = append(users, &userpb.UserInfo{Id: id, Username: fmt.Sprintf("member%03d", id), UpdatedAt: time.Now().UnixMilli()})
	}
	users[2].UpdatedAt = time.Now().Add(-91 * 24 * time.Hour).UnixMilli()
	userClient := &followingExportUserStub{items: users}
	safetyClient := &safetyExportClientStub{muted: []*userpb.UserInfo{{Id: 2, Username: "member002"}}}
	h := NewHandler(&clients.Clients{User: userClient, UserSafety: safetyClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://例子.测试:8443")

	payload, err := h.buildFollowingExport(context.Background(), 42, followingExportRequest{ExcludeMuting: true, ExcludeInactive: true})

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(string(payload), "\n"), "\n")
	require.Len(t, lines, 99)
	require.Equal(t, "member001@xn--fsqu00a.xn--0zwm56d:8443", lines[0])
	require.Equal(t, "member004@xn--fsqu00a.xn--0zwm56d:8443", lines[1])
	require.Equal(t, "member101@xn--fsqu00a.xn--0zwm56d:8443", lines[98])
	require.Equal(t, []int64{0, 100}, userClient.afterIDs)
	require.Equal(t, []int64{42, 42}, userClient.userIDs)
	require.Equal(t, []int64{0}, safetyClient.mutingAfterIDs)
}

func TestBuildFollowingExportDefaultsIncludeMutedInactiveAndReturnsZeroBytesWhenEmpty(t *testing.T) {
	users := []*userpb.UserInfo{
		{Id: 1, Username: "quiet", UpdatedAt: time.Now().Add(-100 * 24 * time.Hour).UnixMilli()},
		{Id: 2, Username: "never-updated"},
	}
	h := NewHandler(&clients.Clients{User: &followingExportUserStub{items: users}}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")

	payload, err := h.buildFollowingExport(context.Background(), 42, followingExportRequest{})
	require.NoError(t, err)
	require.Equal(t, "quiet@bbs.example.com\nnever-updated@bbs.example.com\n", string(payload))

	h.clients.User = &followingExportUserStub{}
	payload, err = h.buildFollowingExport(context.Background(), 42, followingExportRequest{})
	require.NoError(t, err)
	require.Empty(t, payload)
}

func TestBuildUserListsExportPreservesRawReferenceFormatAndPaginatesMembers(t *testing.T) {
	members := make([]*userpb.UserInfo, 0, 101)
	for id := int64(1); id <= 101; id++ {
		members = append(members, &userpb.UserInfo{Id: id, Username: fmt.Sprintf("member%03d", id)})
	}
	client := &userListExportStub{
		lists: []*userpb.UserListInfo{
			{Id: 10, OwnerId: 42, Name: "研发,核心", IsPublic: false},
			{Id: 11, OwnerId: 42, Name: "空列表", IsPublic: true},
		},
		members: map[int64][]*userpb.UserInfo{10: members},
	}
	h := NewHandler(&clients.Clients{UserLists: client}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")

	payload, err := h.buildUserListsExport(context.Background(), 42)

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(string(payload), "\n"), "\n")
	require.Len(t, lines, 101)
	require.Equal(t, "研发,核心,member001@bbs.example.com", lines[0])
	require.Equal(t, "研发,核心,member101@bbs.example.com", lines[100])
	require.Equal(t, []int32{1}, client.listPages)
	require.Equal(t, []int32{1, 2}, client.memberPages[10])
	require.Equal(t, []int32{1}, client.memberPages[11])
	require.Equal(t, int64(42), client.listRequests[0].GetViewerId())
	require.Equal(t, int64(42), client.listRequests[0].GetOwnerId())
}

func TestBuildUserListsExportReturnsZeroByteFile(t *testing.T) {
	h := NewHandler(&clients.Clients{UserLists: &userListExportStub{}}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")

	payload, err := h.buildUserListsExport(context.Background(), 42)

	require.NoError(t, err)
	require.Empty(t, payload)
}

func TestRelationshipExportsRegisterCSVAndCompletionNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		filenamePrefix string
		exportedEntity string
		setup          func(*Handler)
		export         func(*Handler, *gin.Context)
		body           string
	}{
		{
			name: "following", filenamePrefix: "following", exportedEntity: "following", body: `{}`,
			setup: func(handler *Handler) {
				handler.SetFollowingExportGate(&clipExportGateStub{permit: &clipExportPermitStub{}})
			},
			export: (*Handler).exportFollowing,
		},
		{
			name: "user lists", filenamePrefix: "user-lists", exportedEntity: "userList", body: `{}`,
			setup: func(handler *Handler) {
				handler.SetUserListExportGate(&clipExportGateStub{permit: &clipExportPermitStub{}})
			},
			export: (*Handler).exportUserLists,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{Id: 9030, OwnerId: 42}}}
			notifications := &clipExportNotificationStub{}
			store := newFakeUserFileStore()
			h := NewHandlerWithAttachmentStore(&clients.Clients{
				User: &followingExportUserStub{}, UserSafety: &safetyExportClientStub{},
				UserLists: &userListExportStub{}, File: fileClient, NotificationInternal: notifications,
			}, "Authorization", "Bearer", testJWTSecret, store)
			h.SetPublicBaseURL("https://bbs.example.com")
			test.setup(h)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-"+test.filenamePrefix, strings.NewReader(test.body))
			c.Set("user_id", int64(42))
			test.export(h, c)

			require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
			require.Equal(t, "exports", fileClient.createReq.GetBizType())
			require.Equal(t, "text/csv; charset=utf-8", fileClient.createReq.GetContentType())
			require.Regexp(t, regexp.MustCompile(`^`+test.filenamePrefix+`-\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}\.csv$`), fileClient.createReq.GetOriginalName())
			require.Equal(t, test.exportedEntity, notifications.req.GetExportedEntity())
			require.Len(t, store.objects, 1)
		})
	}
}

func TestRelationshipExportRoutesRequireInteractiveAuthAndUseIndependentRateLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newHandler := func() *Handler {
		h := NewHandlerWithAttachmentStore(&clients.Clients{
			User: &followingExportUserStub{}, UserSafety: &safetyExportClientStub{},
			UserLists: &userListExportStub{}, File: &fakeUserFileClient{},
		}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
		h.SetFollowingExportGate(&clipExportGateStub{err: errExportRateLimited})
		h.SetUserListExportGate(&clipExportGateStub{err: errExportRateLimited})
		return h
	}
	paths := []string{
		"/i/export-following", "/api/i/export-following", "/api/v1/i/export-following",
		"/i/export-user-lists", "/api/i/export-user-lists", "/api/v1/i/export-user-lists",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			NewInitControllers(newHandler())(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
		})
	}

	for _, path := range []string{"/api/v1/i/export-following", "/api/v1/i/export-user-lists"} {
		t.Run("unauthenticated "+path, func(t *testing.T) {
			router := gin.New()
			NewInitControllers(newHandler())(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, recorder.Body.String())
		})

		t.Run("api token "+path, func(t *testing.T) {
			router := gin.New()
			NewInitControllers(newHandler())(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
				"sub": "42", "jti": "relationship-export-api-token", "exp": time.Now().Add(time.Hour).Unix(),
				credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"},
			}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
		})
	}
}

func TestExportFollowingRejectsMalformedOrUnknownOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"excludeMuting":`, `{"unknown":true}`, `{} {}`} {
		t.Run(body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-following", strings.NewReader(body))
			h := NewHandler(&clients.Clients{User: &followingExportUserStub{}}, "Authorization", "Bearer", testJWTSecret)
			h.exportFollowing(c)
			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
		})
	}
}

type followingExportUserStub struct {
	clients.UserClient
	items    []*userpb.UserInfo
	afterIDs []int64
	userIDs  []int64
}

func (s *followingExportUserStub) ListFollowing(_ context.Context, request *userpb.ListFollowsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	s.afterIDs = append(s.afterIDs, request.GetAfterUserId())
	s.userIDs = append(s.userIDs, request.GetUserId())
	start := 0
	for start < len(s.items) && s.items[start].GetId() <= request.GetAfterUserId() {
		start++
	}
	end := min(len(s.items), start+int(request.GetPageSize()))
	return &userpb.UserListResponse{Items: s.items[start:end], Total: int64(len(s.items))}, nil
}

type userListExportStub struct {
	clients.UserListClient
	lists        []*userpb.UserListInfo
	members      map[int64][]*userpb.UserInfo
	listPages    []int32
	memberPages  map[int64][]int32
	listRequests []*userpb.ListUserListsRequest
}

func (s *userListExportStub) ListUserLists(_ context.Context, request *userpb.ListUserListsRequest, _ ...grpc.CallOption) (*userpb.UserListsResponse, error) {
	s.listPages = append(s.listPages, request.GetPage())
	s.listRequests = append(s.listRequests, request)
	start := int(request.GetPage()-1) * int(request.GetPageSize())
	if start >= len(s.lists) {
		return &userpb.UserListsResponse{Total: int64(len(s.lists))}, nil
	}
	end := min(len(s.lists), start+int(request.GetPageSize()))
	return &userpb.UserListsResponse{Items: s.lists[start:end], Total: int64(len(s.lists))}, nil
}

func (s *userListExportStub) ListUserListMembers(_ context.Context, request *userpb.ListUserListMembersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	if s.memberPages == nil {
		s.memberPages = map[int64][]int32{}
	}
	s.memberPages[request.GetListId()] = append(s.memberPages[request.GetListId()], request.GetPage())
	items := s.members[request.GetListId()]
	start := int(request.GetPage()-1) * int(request.GetPageSize())
	if start >= len(items) {
		return &userpb.UserListResponse{Total: int64(len(items))}, nil
	}
	end := min(len(items), start+int(request.GetPageSize()))
	return &userpb.UserListResponse{Items: items[start:end], Total: int64(len(items))}, nil
}
