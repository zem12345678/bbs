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

func TestBuildSafetyRelationExportUsesExclusiveKeysetAndExactCSV(t *testing.T) {
	blocked := make([]*userpb.UserInfo, 0, 101)
	for index := int64(1); index <= 101; index++ {
		blocked = append(blocked, &userpb.UserInfo{Id: index, Username: "member" + formatExportTestIndex(index)})
	}
	client := &safetyExportClientStub{blocked: blocked, muted: []*userpb.UserInfo{{Id: 7, Username: "quiet"}}}
	h := NewHandler(&clients.Clients{UserSafety: client}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://例子.测试:8443")

	payload, err := h.buildSafetyRelationExport(context.Background(), 42, true)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(string(payload), "\n"), "\n")
	require.Len(t, lines, 101)
	require.Equal(t, "member001@xn--fsqu00a.xn--0zwm56d:8443", lines[0])
	require.Equal(t, "member101@xn--fsqu00a.xn--0zwm56d:8443", lines[100])
	require.True(t, strings.HasSuffix(string(payload), "\n"))
	require.Equal(t, []int64{0, 100}, client.blockingAfterIDs)
	require.Equal(t, []int64{42, 42}, client.blockingActorIDs)

	payload, err = h.buildSafetyRelationExport(context.Background(), 42, false)
	require.NoError(t, err)
	require.Equal(t, "quiet@xn--fsqu00a.xn--0zwm56d:8443\n", string(payload))
	require.Equal(t, []int64{0}, client.mutingAfterIDs)
}

func TestBuildSafetyRelationExportReturnsZeroByteFile(t *testing.T) {
	h := NewHandler(&clients.Clients{UserSafety: &safetyExportClientStub{}}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")

	payload, err := h.buildSafetyRelationExport(context.Background(), 42, true)

	require.NoError(t, err)
	require.Empty(t, payload)
}

func TestExportSafetyRelationsRegisterCSVAndCompletionNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		path           string
		filenamePrefix string
		exportedEntity string
		setGate        func(*Handler, ExportGate)
		export         func(*Handler, *gin.Context)
	}{
		{name: "blocking", path: "/i/export-blocking", filenamePrefix: "blocking", exportedEntity: "blocking", setGate: (*Handler).SetBlockingExportGate, export: (*Handler).exportBlocking},
		{name: "mute", path: "/i/export-mute", filenamePrefix: "mute", exportedEntity: "muting", setGate: (*Handler).SetMuteExportGate, export: (*Handler).exportMute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fileClient := &fakeUserFileClient{createResp: &filepb.FileResponse{File: &filepb.File{Id: 9020, OwnerId: 42}}}
			notifications := &clipExportNotificationStub{}
			store := newFakeUserFileStore()
			permit := &clipExportPermitStub{}
			h := NewHandlerWithAttachmentStore(&clients.Clients{
				UserSafety: &safetyExportClientStub{blocked: []*userpb.UserInfo{{Id: 7, Username: "blocked"}}, muted: []*userpb.UserInfo{{Id: 8, Username: "muted"}}},
				File:       fileClient, NotificationInternal: notifications,
			}, "Authorization", "Bearer", testJWTSecret, store)
			h.SetPublicBaseURL("https://bbs.example.com")
			test.setGate(h, &clipExportGateStub{permit: permit})

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(stdhttp.MethodPost, test.path, strings.NewReader(`{}`))
			c.Set("user_id", int64(42))
			test.export(h, c)

			require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
			require.True(t, permit.committed)
			require.False(t, permit.released)
			require.Equal(t, "exports", fileClient.createReq.GetBizType())
			require.Equal(t, "text/csv; charset=utf-8", fileClient.createReq.GetContentType())
			require.Regexp(t, regexp.MustCompile(`^`+test.filenamePrefix+`-\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}\.csv$`), fileClient.createReq.GetOriginalName())
			require.Equal(t, test.exportedEntity, notifications.req.GetExportedEntity())
			require.Len(t, store.objects, 1)
			for _, object := range store.objects {
				require.Equal(t, "text/csv; charset=utf-8", object.contentType)
				require.True(t, strings.HasSuffix(string(object.data), "@bbs.example.com\n"))
			}
		})
	}
}

func TestSafetyExportRoutesRequireInteractiveAuthAndUseIndependentRateLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newHandler := func() *Handler {
		h := NewHandlerWithAttachmentStore(&clients.Clients{
			UserSafety: &safetyExportClientStub{}, File: &fakeUserFileClient{},
		}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
		h.SetBlockingExportGate(&clipExportGateStub{err: errExportRateLimited})
		h.SetMuteExportGate(&clipExportGateStub{err: errExportRateLimited})
		return h
	}
	for _, path := range []string{
		"/i/export-blocking", "/api/i/export-blocking", "/api/v1/i/export-blocking",
		"/i/export-mute", "/api/i/export-mute", "/api/v1/i/export-mute",
	} {
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
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/export-blocking", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "safety-export-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
}

func formatExportTestIndex(value int64) string {
	return fmt.Sprintf("%03d", value)
}

type safetyExportClientStub struct {
	clients.UserSafetyClient
	blocked          []*userpb.UserInfo
	muted            []*userpb.UserInfo
	blockingAfterIDs []int64
	mutingAfterIDs   []int64
	blockingActorIDs []int64
}

func (s *safetyExportClientStub) ListBlockedUsers(_ context.Context, req *userpb.ListUserRelationsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	s.blockingAfterIDs = append(s.blockingAfterIDs, req.GetAfterTargetId())
	s.blockingActorIDs = append(s.blockingActorIDs, req.GetActorId())
	return safetyExportPage(s.blocked, req), nil
}

func (s *safetyExportClientStub) ListMutedUsers(_ context.Context, req *userpb.ListUserRelationsRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	s.mutingAfterIDs = append(s.mutingAfterIDs, req.GetAfterTargetId())
	return safetyExportPage(s.muted, req), nil
}

func safetyExportPage(items []*userpb.UserInfo, req *userpb.ListUserRelationsRequest) *userpb.UserListResponse {
	start := 0
	for start < len(items) && items[start].GetId() <= req.GetAfterTargetId() {
		start++
	}
	end := min(len(items), start+int(req.GetPageSize()))
	return &userpb.UserListResponse{Items: items[start:end], Total: int64(len(items))}
}
