package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testSessionUserID int64 = 9223372036854770007

func TestListCurrentUserSessionsScopesToTokenAndClampsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessionClient := &fakeUserSessionClient{
		listSessions: &userpb.SessionListResponse{
			Total: 1,
			Items: []*userpb.SessionInfo{{
				SessionId:   "session-abcdef0123456789",
				UserId:      testSessionUserID,
				IpAddress:   "203.0.113.7",
				UserAgent:   "Mozilla/5.0",
				LoginMethod: "password",
				CreatedAt:   1700000000,
				ExpiresAt:   1700086400,
			}},
		},
	}
	recorder := serveSessionRequest(t, sessionClient, stdhttp.MethodGet, "/api/v1/users/me/sessions?limit=5000")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, sessionClient.listSessionsReq)
	require.Equal(t, testSessionUserID, sessionClient.listSessionsReq.GetUserId())
	require.Equal(t, int32(maxSessionListLimit), sessionClient.listSessionsReq.GetLimit())

	var envelope struct {
		Data struct {
			Total int64 `json:"total"`
			Items []struct {
				SessionID   string `json:"session_id"`
				UserID      string `json:"user_id"`
				LoginMethod string `json:"login_method"`
				RevokedAt   int64  `json:"revoked_at"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "session-abcdef0123456789", envelope.Data.Items[0].SessionID)
	require.Equal(t, "9223372036854770007", envelope.Data.Items[0].UserID)
	require.Equal(t, "password", envelope.Data.Items[0].LoginMethod)
	require.Zero(t, envelope.Data.Items[0].RevokedAt)
}

func TestListCurrentUserSessionsDefaultsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessionClient := &fakeUserSessionClient{listSessions: &userpb.SessionListResponse{}}
	recorder := serveSessionRequest(t, sessionClient, stdhttp.MethodGet, "/api/v1/users/me/sessions")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(defaultSessionListLimit), sessionClient.listSessionsReq.GetLimit())
}

func TestGetCurrentUserSessionPassesSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessionClient := &fakeUserSessionClient{
		getSession: &userpb.SessionResponse{Session: &userpb.SessionInfo{SessionId: "session-abcdef0123456789", UserId: testSessionUserID}},
	}
	recorder := serveSessionRequest(t, sessionClient, stdhttp.MethodGet, "/api/v1/users/me/sessions/session-abcdef0123456789")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, sessionClient.getSessionReq)
	require.Equal(t, testSessionUserID, sessionClient.getSessionReq.GetUserId())
	require.Equal(t, "session-abcdef0123456789", sessionClient.getSessionReq.GetSessionId())
}

func TestGetCurrentUserSessionMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessionClient := &fakeUserSessionClient{getSessionErr: status.Error(codes.NotFound, "user session not found")}
	recorder := serveSessionRequest(t, sessionClient, stdhttp.MethodGet, "/api/v1/users/me/sessions/session-abcdef0123456789")

	require.Equal(t, stdhttp.StatusNotFound, recorder.Code, recorder.Body.String())
}

func TestRevokeCurrentUserSessionReturnsRevokedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessionClient := &fakeUserSessionClient{
		revokeSession: &userpb.SessionResponse{Session: &userpb.SessionInfo{
			SessionId: "session-abcdef0123456789",
			UserId:    testSessionUserID,
			RevokedAt: 1700090000,
		}},
	}
	recorder := serveSessionRequest(t, sessionClient, stdhttp.MethodDelete, "/api/v1/users/me/sessions/session-abcdef0123456789")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, sessionClient.revokeSessionReq)
	require.Equal(t, testSessionUserID, sessionClient.revokeSessionReq.GetUserId())
	require.Equal(t, "session-abcdef0123456789", sessionClient.revokeSessionReq.GetSessionId())

	var envelope struct {
		Data struct {
			Session struct {
				RevokedAt int64 `json:"revoked_at"`
			} `json:"session"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(1700090000), envelope.Data.Session.RevokedAt)
}

func TestListCurrentUserLoginEventsStringifiesID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessionClient := &fakeUserSessionClient{
		listLoginEvents: &userpb.LoginEventListResponse{
			Total: 2,
			Items: []*userpb.LoginEventInfo{
				{Id: "9223372036854770100", UserId: testSessionUserID, SessionId: "session-abcdef0123456789", Success: true},
				{Id: "9223372036854770101", UserId: testSessionUserID, Success: false, FailureReason: "bad_password"},
			},
		},
	}
	recorder := serveSessionRequest(t, sessionClient, stdhttp.MethodGet, "/api/v1/users/me/login-events?limit=50")

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(50), sessionClient.listLoginEventsReq.GetLimit())
	require.Equal(t, testSessionUserID, sessionClient.listLoginEventsReq.GetUserId())

	var envelope struct {
		Data struct {
			Total int64 `json:"total"`
			Items []struct {
				ID            string `json:"id"`
				UserID        string `json:"user_id"`
				SessionID     string `json:"session_id"`
				Success       bool   `json:"success"`
				FailureReason string `json:"failure_reason"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(2), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 2)
	require.Equal(t, "9223372036854770100", envelope.Data.Items[0].ID)
	require.Equal(t, "9223372036854770007", envelope.Data.Items[0].UserID)
	require.True(t, envelope.Data.Items[0].Success)
	require.Empty(t, envelope.Data.Items[1].SessionID)
	require.Equal(t, "bad_password", envelope.Data.Items[1].FailureReason)
}

func TestSessionRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{UserSessions: &fakeUserSessionClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	for _, route := range []struct {
		method string
		target string
	}{
		{stdhttp.MethodGet, "/api/v1/users/me/sessions"},
		{stdhttp.MethodGet, "/api/v1/users/me/sessions/session-abcdef0123456789"},
		{stdhttp.MethodDelete, "/api/v1/users/me/sessions/session-abcdef0123456789"},
		{stdhttp.MethodGet, "/api/v1/users/me/login-events"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(route.method, route.target, nil))
		require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code, "%s %s -> %s", route.method, route.target, recorder.Body.String())
	}
}

func TestSessionRoutesReportUnavailableWithoutClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/sessions", nil)
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "9223372036854770007"}))
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
}

func TestLoginRecordsRequestClientInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userClient := &authLoginUserClient{resp: &userpb.AuthResponse{Success: true, AccessToken: "token", User: &userpb.UserInfo{Id: 42}}}
	h := NewHandler(&clients.Clients{Admin: fakeAuthSettingsAdminClient{}, User: userClient}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"account":"alice","password":"secret"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "IntegrationTest/1.0")
	c.Request.RemoteAddr = "198.51.100.24:44321"

	h.login(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, userClient.req)
	require.Equal(t, "198.51.100.24", userClient.req.GetClient().GetIpAddress())
	require.Equal(t, "IntegrationTest/1.0", userClient.req.GetClient().GetUserAgent())
}

type fakeUserSessionClient struct {
	listSessionsReq    *userpb.ListSessionsRequest
	getSessionReq      *userpb.GetSessionRequest
	revokeSessionReq   *userpb.RevokeSessionRequest
	listLoginEventsReq *userpb.ListLoginEventsRequest

	listSessions    *userpb.SessionListResponse
	getSession      *userpb.SessionResponse
	revokeSession   *userpb.SessionResponse
	listLoginEvents *userpb.LoginEventListResponse

	getSessionErr error
}

func (c *fakeUserSessionClient) ListSessions(_ context.Context, req *userpb.ListSessionsRequest, _ ...grpc.CallOption) (*userpb.SessionListResponse, error) {
	c.listSessionsReq = req
	return c.listSessions, nil
}

func (c *fakeUserSessionClient) GetSession(_ context.Context, req *userpb.GetSessionRequest, _ ...grpc.CallOption) (*userpb.SessionResponse, error) {
	c.getSessionReq = req
	if c.getSessionErr != nil {
		return nil, c.getSessionErr
	}
	return c.getSession, nil
}

func (c *fakeUserSessionClient) RevokeSession(_ context.Context, req *userpb.RevokeSessionRequest, _ ...grpc.CallOption) (*userpb.SessionResponse, error) {
	c.revokeSessionReq = req
	return c.revokeSession, nil
}

func (c *fakeUserSessionClient) ListLoginEvents(_ context.Context, req *userpb.ListLoginEventsRequest, _ ...grpc.CallOption) (*userpb.LoginEventListResponse, error) {
	c.listLoginEventsReq = req
	return c.listLoginEvents, nil
}

func serveSessionRequest(t *testing.T, sessionClient *fakeUserSessionClient, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&clients.Clients{UserSessions: sessionClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "9223372036854770007"}))
	router.ServeHTTP(recorder, request)
	return recorder
}
