package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"
	realtimechat "api-gateway/internal/realtime/chat"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const testSessionJTI = "3ff8a1b2c3d4e5f60718293a4b5c6d7e"

// serveSessionRequestWithRevocations mirrors serveSessionRequest but keeps a
// revocation store and a jti claim so session scoping can be asserted.
func serveSessionRequestWithRevocations(
	t *testing.T,
	sessionClient *fakeUserSessionClient,
	revocations TokenRevocationStore,
	method string,
	target string,
	jti string,
) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandlerWithTokenRevocationStore(
		&clients.Clients{UserSessions: sessionClient}, "Authorization", "Bearer", testJWTSecret, revocations,
	)
	router := gin.New()
	NewInitControllers(h)(router)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	claims := jwt.MapClaims{"sub": "9223372036854770007"}
	if jti != "" {
		claims["jti"] = jti
	}
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, claims))
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestListCurrentUserSessionsMarksCallerSessionAndActivity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	future := time.Now().Add(time.Hour).Unix()
	past := time.Now().Add(-time.Hour).Unix()
	sessionClient := &fakeUserSessionClient{
		listSessions: &userpb.SessionListResponse{
			Total: 3,
			Items: []*userpb.SessionInfo{
				{SessionId: testSessionJTI, UserId: testSessionUserID, ExpiresAt: future},
				{SessionId: "0011223344556677", UserId: testSessionUserID, ExpiresAt: future},
				{SessionId: "8899aabbccddeeff", UserId: testSessionUserID, ExpiresAt: past},
			},
		},
	}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, &fakeTokenRevocationStore{}, stdhttp.MethodGet, "/api/v1/users/me/sessions", testSessionJTI,
	)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Items []struct {
				SessionID string `json:"session_id"`
				Current   bool   `json:"current"`
				Active    bool   `json:"active"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 3)

	require.True(t, envelope.Data.Items[0].Current, "caller session must be flagged current")
	require.True(t, envelope.Data.Items[0].Active)

	require.False(t, envelope.Data.Items[1].Current, "another device must not be current")
	require.True(t, envelope.Data.Items[1].Active)

	require.False(t, envelope.Data.Items[2].Current)
	require.False(t, envelope.Data.Items[2].Active, "expired session must not be active")
}

// A legacy token carries no jti, so no session may claim to be the current one.
func TestListCurrentUserSessionsMarksNothingCurrentWithoutJTI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessionClient := &fakeUserSessionClient{
		listSessions: &userpb.SessionListResponse{
			Total: 1,
			Items: []*userpb.SessionInfo{{SessionId: testSessionJTI, UserId: testSessionUserID}},
		},
	}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, &fakeTokenRevocationStore{}, stdhttp.MethodGet, "/api/v1/users/me/sessions", "",
	)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Items []struct {
				Current bool `json:"current"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.False(t, envelope.Data.Items[0].Current)
}

func TestRevokeCurrentUserSessionBlocksSessionInRevocationStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expiresAt := time.Now().Add(2 * time.Hour).Unix()
	revocations := &fakeTokenRevocationStore{}
	sessionClient := &fakeUserSessionClient{
		revokeSession: &userpb.SessionResponse{Session: &userpb.SessionInfo{
			SessionId: "0011223344556677",
			UserId:    testSessionUserID,
			ExpiresAt: expiresAt,
			RevokedAt: time.Now().Unix(),
		}},
	}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, revocations, stdhttp.MethodDelete, "/api/v1/users/me/sessions/0011223344556677", testSessionJTI,
	)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "0011223344556677", revocations.revokedSession,
		"revoking a device must block its session id, not only the database row")
	require.Equal(t, expiresAt, revocations.sessionExpiry.Unix())
}

// Losing Redis must not report a revocation that will not be enforced.
func TestRevokeCurrentUserSessionFailsClosedWhenStoreIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	revocations := &fakeTokenRevocationStore{revokeSessionEr: errors.New("redis unavailable")}
	sessionClient := &fakeUserSessionClient{
		revokeSession: &userpb.SessionResponse{Session: &userpb.SessionInfo{
			SessionId: "0011223344556677",
			UserId:    testSessionUserID,
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}},
	}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, revocations, stdhttp.MethodDelete, "/api/v1/users/me/sessions/0011223344556677", testSessionJTI,
	)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
}

func TestRequireAuthRejectsRevokedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	revocations := &fakeTokenRevocationStore{revokedSession: testSessionJTI}
	sessionClient := &fakeUserSessionClient{listSessions: &userpb.SessionListResponse{}}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, revocations, stdhttp.MethodGet, "/api/v1/users/me/sessions", testSessionJTI,
	)

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code,
		"a token whose session was revoked must stop working")
	require.Nil(t, sessionClient.listSessionsReq, "revoked session must not reach the user service")
}

// Revoking one device must not sign the account's other devices out.
func TestRequireAuthAcceptsSessionThatWasNotRevoked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	revocations := &fakeTokenRevocationStore{revokedSession: "0011223344556677"}
	sessionClient := &fakeUserSessionClient{listSessions: &userpb.SessionListResponse{}}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, revocations, stdhttp.MethodGet, "/api/v1/users/me/sessions", testSessionJTI,
	)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, sessionClient.listSessionsReq)
}

// Tokens issued before session tracking have no jti and must keep working.
func TestRequireAuthAcceptsLegacyTokenWithoutSessionClaim(t *testing.T) {
	gin.SetMode(gin.TestMode)
	revocations := &fakeTokenRevocationStore{revokedSession: testSessionJTI}
	sessionClient := &fakeUserSessionClient{listSessions: &userpb.SessionListResponse{}}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, revocations, stdhttp.MethodGet, "/api/v1/users/me/sessions", "",
	)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
}

func TestRequireAuthFailsClosedWhenSessionRevocationCheckFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	revocations := &fakeTokenRevocationStore{sessionErr: errors.New("redis unavailable")}
	sessionClient := &fakeUserSessionClient{listSessions: &userpb.SessionListResponse{}}
	recorder := serveSessionRequestWithRevocations(
		t, sessionClient, revocations, stdhttp.MethodGet, "/api/v1/users/me/sessions", testSessionJTI,
	)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
}

func TestChatSessionValidatorRejectsRevokedSession(t *testing.T) {
	revocations := &fakeTokenRevocationStore{}
	versions := &fakeCredentialVersionStore{version: "credential-v2"}
	handler := NewHandlerWithRealtimeAndRateLimitsAndTokenSecurityStores(
		nil, "Authorization", "Bearer", testJWTSecret, nil, nil, nil, nil, revocations, versions,
	)
	ticket := realtimechat.Ticket{
		UserID:                 42,
		TokenFingerprint:       tokenRevocationFingerprint("access-token"),
		SessionID:              testSessionJTI,
		CredentialVersion:      "credential-v2",
		CredentialVersionClaim: true,
	}

	require.NoError(t, handler.ValidateChatSession(context.Background(), ticket))

	revocations.revokedSession = testSessionJTI
	require.ErrorIs(t, handler.ValidateChatSession(context.Background(), ticket), realtimechat.ErrSessionInvalid,
		"a revoked device must lose its websocket")

	// Another device's revocation must leave this socket connected.
	revocations.revokedSession = "0011223344556677"
	require.NoError(t, handler.ValidateChatSession(context.Background(), ticket))

	// Tickets minted before session tracking carry no session id.
	revocations.revokedSession = testSessionJTI
	legacy := ticket
	legacy.SessionID = ""
	require.NoError(t, handler.ValidateChatSession(context.Background(), legacy))

	revocations.revokedSession = testSessionJTI
	revocations.sessionErr = errors.New("redis unavailable")
	require.ErrorIs(t, handler.ValidateChatSession(context.Background(), ticket), realtimechat.ErrSessionValidationUnavailable)
}
