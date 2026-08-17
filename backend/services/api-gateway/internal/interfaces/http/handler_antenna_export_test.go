package http

import (
	"context"
	"encoding/json"
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

func TestBuildAntennaExportMapsReferenceFieldsAndListAccounts(t *testing.T) {
	members := make([]*userpb.UserInfo, 0, 101)
	for index := 0; index < 101; index++ {
		members = append(members, &userpb.UserInfo{Id: int64(index + 1), Username: "member" + string(rune('a'+index%26))})
	}
	antennas := &antennaExportAntennaStub{items: []*userpb.AntennaInfo{
		{
			Id: 8, OwnerId: 42, Name: "list antenna", Source: "list", UserListId: 77,
			Keywords:        []*userpb.AntennaKeywordGroup{{Terms: []string{"Go", "Rust"}}},
			ExcludeKeywords: []*userpb.AntennaKeywordGroup{{Terms: []string{"spam"}}},
			Users:           []string{"alice@example.net"}, CaseSensitive: true, LocalOnly: true,
			ExcludeBots: true, WithReplies: true, WithFile: true, ExcludeNotesInSensitiveChannel: true, IsActive: true,
		},
		{Id: 7, OwnerId: 42, Name: "all antenna", Source: "all"},
	}}
	lists := &antennaExportUserListStub{members: members}
	h := NewHandler(&clients.Clients{UserAntennas: antennas, UserLists: lists}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com:8443")

	payload, err := h.buildAntennaExport(context.Background(), 42)
	require.NoError(t, err)
	var exported []antennaExportRecord
	require.NoError(t, json.Unmarshal(payload, &exported))
	require.Len(t, exported, 2)
	require.Equal(t, "list antenna", exported[0].Name)
	require.Equal(t, [][]string{{"Go", "Rust"}}, exported[0].Keywords)
	require.Equal(t, []string{"spam"}, exported[0].ExcludeKeywords[0])
	require.Len(t, exported[0].UserListAccts, 101)
	require.Equal(t, "membera@bbs.example.com", exported[0].UserListAccts[0])
	require.Equal(t, "memberw@bbs.example.com", exported[0].UserListAccts[100])
	require.Nil(t, exported[1].UserListAccts)
	require.Equal(t, []int32{1, 2}, lists.pages)

	var raw []map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	require.Len(t, raw[0], 11)
	require.Contains(t, raw[0], "userListAccts")
	require.Nil(t, raw[1]["userListAccts"])
	require.NotContains(t, raw[0], "id")
	require.NotContains(t, raw[0], "userListId")
	require.NotContains(t, raw[0], "excludeNotesInSensitiveChannel")
}

func TestBuildAntennaExportReturnsEmptyArray(t *testing.T) {
	h := NewHandler(&clients.Clients{
		UserAntennas: &antennaExportAntennaStub{}, UserLists: &antennaExportUserListStub{},
	}, "Authorization", "Bearer", testJWTSecret)

	payload, err := h.buildAntennaExport(context.Background(), 42)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(payload))
}

func TestAntennaUserListAccountsUsesPunycodeAndKeepsEmptyArray(t *testing.T) {
	h := NewHandler(&clients.Clients{UserLists: &antennaExportUserListStub{}}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://例子.测试")

	accounts, err := h.antennaUserListAccounts(context.Background(), 42, 77)

	require.NoError(t, err)
	require.NotNil(t, accounts)
	require.Empty(t, accounts)
	payload, err := json.Marshal(antennaExportRecord{UserListAccts: accounts})
	require.NoError(t, err)
	require.Contains(t, string(payload), `"userListAccts":[]`)
}

func TestExportAntennasRegistersFileAndCompletionNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{Id: 9010, OwnerId: 42}}}
	notifications := &clipExportNotificationStub{}
	store := newFakeUserFileStore()
	permit := &clipExportPermitStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		UserAntennas: &antennaExportAntennaStub{items: []*userpb.AntennaInfo{{Id: 1, OwnerId: 42, Name: "watch", Source: "all"}}},
		UserLists:    &antennaExportUserListStub{}, File: fileClient, NotificationInternal: notifications,
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetAntennaExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-antennas", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportAntennas(c)

	require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
	require.True(t, permit.committed)
	require.False(t, permit.released)
	require.Equal(t, int64(42), fileClient.createReq.GetOwnerId())
	require.Equal(t, "exports", fileClient.createReq.GetBizType())
	require.Equal(t, "application/json", fileClient.createReq.GetContentType())
	require.Regexp(t, regexp.MustCompile(`^antennas-\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}\.json$`), fileClient.createReq.GetOriginalName())
	require.Equal(t, int64(42), notifications.req.GetRecipientId())
	require.Equal(t, int64(9010), notifications.req.GetFileId())
	require.Equal(t, "antenna", notifications.req.GetExportedEntity())
	require.Len(t, store.objects, 1)
}

func TestAntennaExportRoutesApplyInteractiveAuthenticationAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newHandler := func() *Handler {
		h := NewHandlerWithAttachmentStore(&clients.Clients{
			UserAntennas: &antennaExportAntennaStub{}, UserLists: &antennaExportUserListStub{}, File: &fakeUserFileClient{},
		}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
		h.SetAntennaExportGate(&clipExportGateStub{err: errExportRateLimited})
		return h
	}
	for _, path := range []string{"/i/export-antennas", "/api/i/export-antennas", "/api/v1/i/export-antennas"} {
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

	router := gin.New()
	NewInitControllers(newHandler())(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/export-antennas", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "antenna-export-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
}

type antennaExportAntennaStub struct {
	clients.UserAntennaClient
	items []*userpb.AntennaInfo
}

func (s *antennaExportAntennaStub) ListAntennas(_ context.Context, _ *userpb.ListAntennasRequest, _ ...grpc.CallOption) (*userpb.AntennaListResponse, error) {
	return &userpb.AntennaListResponse{Items: s.items, Total: int64(len(s.items))}, nil
}

type antennaExportUserListStub struct {
	clients.UserListClient
	members []*userpb.UserInfo
	pages   []int32
}

func (s *antennaExportUserListStub) ListUserListMembers(_ context.Context, req *userpb.ListUserListMembersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	s.pages = append(s.pages, req.GetPage())
	start := int(req.GetPage()-1) * int(req.GetPageSize())
	if start >= len(s.members) {
		return &userpb.UserListResponse{Total: int64(len(s.members))}, nil
	}
	end := min(len(s.members), start+int(req.GetPageSize()))
	return &userpb.UserListResponse{Items: s.members[start:end], Total: int64(len(s.members))}, nil
}
