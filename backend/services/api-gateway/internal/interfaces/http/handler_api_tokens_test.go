package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testAPITokenID = "api-token-abcdef012345"

type fakeUserAPITokenClient struct {
	createReq  *userpb.CreateAPITokenRequest
	listReq    *userpb.ListAPITokensRequest
	revokeReq  *userpb.RevokeAPITokenRequest
	createResp *userpb.CreateAPITokenResponse
	listResp   *userpb.APITokenListResponse
	revokeResp *userpb.APITokenResponse
	createErr  error
	listErr    error
	revokeErr  error
}

func (c *fakeUserAPITokenClient) CreateAPIToken(_ context.Context, req *userpb.CreateAPITokenRequest, _ ...grpc.CallOption) (*userpb.CreateAPITokenResponse, error) {
	c.createReq = req
	return c.createResp, c.createErr
}

func (c *fakeUserAPITokenClient) ListAPITokens(_ context.Context, req *userpb.ListAPITokensRequest, _ ...grpc.CallOption) (*userpb.APITokenListResponse, error) {
	c.listReq = req
	return c.listResp, c.listErr
}

func (c *fakeUserAPITokenClient) RevokeAPIToken(_ context.Context, req *userpb.RevokeAPITokenRequest, _ ...grpc.CallOption) (*userpb.APITokenResponse, error) {
	c.revokeReq = req
	return c.revokeResp, c.revokeErr
}

func TestCreateAPITokenMapsRequestAndReturnsSecretOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeUserAPITokenClient{createResp: &userpb.CreateAPITokenResponse{
		Token:    "secret-token",
		ApiToken: &userpb.APITokenInfo{Id: testAPITokenID, UserId: 42, Name: "Deploy", Scopes: []string{"read", "write"}, CredentialValid: true, ExpiresAt: time.Now().Add(time.Hour).Unix()},
	}}
	h := NewHandler(&clients.Clients{UserAPITokens: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/users/me/api-tokens", strings.NewReader(`{"name":"Deploy","scopes":["write","read"],"expires_in_days":30}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.createReq.GetUserId())
	require.Equal(t, "Deploy", client.createReq.GetName())
	require.Equal(t, []string{"write", "read"}, client.createReq.GetScopes())
	require.Equal(t, int32(30), client.createReq.GetExpiresInDays())
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "secret-token", envelope.Data["token"])
	require.Equal(t, testAPITokenID, envelope.Data["id"])
	apiToken := envelope.Data["api_token"].(map[string]any)
	require.NotContains(t, apiToken, "token")
}

func TestListAPITokensSupportsPOSTReadScopeAndNeverReturnsSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeUserAPITokenClient{listResp: &userpb.APITokenListResponse{Total: 1, Items: []*userpb.APITokenInfo{{Id: testAPITokenID, UserId: 42, Name: "Read", Scopes: []string{"read"}, CredentialValid: true}}}}
	h := NewHandler(&clients.Clients{UserAPITokens: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	claims := jwt.MapClaims{"sub": "42", "jti": "api-jti-abcdef012345", "exp": time.Now().Add(time.Hour).Unix(), credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"}}
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/tokens/list", strings.NewReader(`{"limit":40,"offset":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, claims))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.listReq.GetUserId())
	require.Equal(t, int32(40), client.listReq.GetLimit())
	require.Equal(t, int32(2), client.listReq.GetOffset())
	require.NotContains(t, recorder.Body.String(), "secret-token")
}

func TestAPITokenScopesAreMethodAwareAndManagementNeedsInteractiveSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeUserAPITokenClient{listResp: &userpb.APITokenListResponse{}}
	h := NewHandler(&clients.Clients{UserAPITokens: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	readClaims := jwt.MapClaims{"sub": "42", "jti": "api-jti-read-abcdef", "exp": time.Now().Add(time.Hour).Unix(), credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"}}
	writeClaims := jwt.MapClaims{"sub": "42", "jti": "api-jti-write-abcdef", "exp": time.Now().Add(time.Hour).Unix(), credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"write"}}

	readCreate := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/users/me/api-tokens", strings.NewReader(`{"name":"x","scopes":["read"]}`))
	readCreate.Header.Set("Authorization", "Bearer "+signedAuthToken(t, readClaims))
	readCreateRecorder := httptest.NewRecorder()
	router.ServeHTTP(readCreateRecorder, readCreate)
	require.Equal(t, stdhttp.StatusForbidden, readCreateRecorder.Code)

	writeList := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/api-tokens", nil)
	writeList.Header.Set("Authorization", "Bearer "+signedAuthToken(t, writeClaims))
	writeListRecorder := httptest.NewRecorder()
	router.ServeHTTP(writeListRecorder, writeList)
	require.Equal(t, stdhttp.StatusForbidden, writeListRecorder.Code)
	require.Nil(t, client.listReq)
}

func TestRevokeAPITokenMirrorsSessionRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expiresAt := time.Now().Add(time.Hour).Unix()
	client := &fakeUserAPITokenClient{revokeResp: &userpb.APITokenResponse{ApiToken: &userpb.APITokenInfo{Id: testAPITokenID, UserId: 42, ExpiresAt: expiresAt, RevokedAt: time.Now().Unix()}}}
	revocations := &fakeTokenRevocationStore{}
	h := NewHandlerWithTokenRevocationStore(&clients.Clients{UserAPITokens: client}, "Authorization", "Bearer", testJWTSecret, revocations)
	router := gin.New()
	NewInitControllers(h)(router)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/tokens/revoke", strings.NewReader(`{"tokenId":"`+testAPITokenID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.revokeReq.GetUserId())
	require.Equal(t, testAPITokenID, client.revokeReq.GetTokenId())
	require.Equal(t, testAPITokenID, revocations.revokedSession)
	require.Equal(t, expiresAt, revocations.sessionExpiry.Unix())
}

func TestAPITokenRoutesRejectMalformedClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeUserAPITokenClient{listResp: &userpb.APITokenListResponse{}}
	h := NewHandler(&clients.Clients{UserAPITokens: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/api-tokens", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42", "token_type": apiTokenType, "scopes": []string{"read"}}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code)
}

func TestListAPITokensMapsUpstreamInvalidArgument(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeUserAPITokenClient{listErr: status.Error(codes.InvalidArgument, "invalid api token list parameters")}
	h := NewHandler(&clients.Clients{UserAPITokens: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/api-tokens?limit=101", nil)
	req.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code)
}
