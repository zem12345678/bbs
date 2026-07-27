package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/userpb"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestRedisCredentialVersionStoreRepairsStaleCacheFromUserService(t *testing.T) {
	commands := &fakeCredentialVersionCommands{value: "rotated-version"}
	authority := &fakeCredentialVersionAuthority{version: "authoritative-version"}
	store := NewRedisCredentialVersionStore(commands, authority)

	version, err := store.Current(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, "authoritative-version", version)
	require.Equal(t, "bbs:auth:credential-version:42", commands.key)
	require.Equal(t, 1, authority.calls)
	require.Equal(t, "bbs:auth:credential-version:42", commands.setKey)
	require.Equal(t, "authoritative-version", commands.setValue)
}

func TestRedisCredentialVersionStoreRehydratesCacheFromUserServiceOnMiss(t *testing.T) {
	commands := &fakeCredentialVersionCommands{err: redis.Nil}
	authority := &fakeCredentialVersionAuthority{version: credentialVersionInitial}
	store := NewRedisCredentialVersionStore(commands, authority)

	version, err := store.Current(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, credentialVersionInitial, version)
	require.Equal(t, 1, authority.calls)
	require.Equal(t, int64(42), authority.userID)
	require.Equal(t, "bbs:auth:credential-version:42", commands.setKey)
	require.Equal(t, credentialVersionInitial, commands.setValue)
	require.Zero(t, commands.setTTL)
}

func TestRedisCredentialVersionStoreFailsClosedWhenCacheMissCannotReachAuthority(t *testing.T) {
	store := NewRedisCredentialVersionStore(
		&fakeCredentialVersionCommands{err: redis.Nil},
		&fakeCredentialVersionAuthority{err: errors.New("user service unavailable")},
	)

	_, err := store.Current(context.Background(), 42)

	require.Error(t, err)
}

func TestRedisCredentialVersionStoreFailsClosedWhenCacheRehydrateCannotBePersisted(t *testing.T) {
	store := NewRedisCredentialVersionStore(
		&fakeCredentialVersionCommands{err: redis.Nil, setErr: errors.New("redis unavailable")},
		&fakeCredentialVersionAuthority{version: "rotated-version"},
	)

	_, err := store.Current(context.Background(), 42)

	require.Error(t, err)
}

func TestUserCredentialVersionAuthorityUsesInternalUserRPC(t *testing.T) {
	client := &fakeCredentialVersionUserClient{response: &userpb.CredentialVersionResponse{UserId: 42, CredentialVersion: "rotated-version"}}
	authority := NewUserCredentialVersionAuthority(client)

	version, err := authority.Current(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, "rotated-version", version)
	require.NotNil(t, client.request)
	require.Equal(t, int64(42), client.request.GetId())
}

func TestRequireAuthRejectsTokensIssuedBeforePasswordCredentialChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	versions := &fakeCredentialVersionStore{version: "rotated-version"}
	handler := NewHandlerWithCredentialVersionStore(nil, "Authorization", "Bearer", testJWTSecret, versions)
	router := newCredentialVersionTestRouter(handler)
	beforeChange := signedAuthToken(t, jwt.MapClaims{"sub": "42", credentialVersionClaim: credentialVersionInitial})
	legacyBeforeChange := signedAuthToken(t, jwt.MapClaims{"sub": "42"})
	afterChange := signedAuthToken(t, jwt.MapClaims{"sub": "42", credentialVersionClaim: "rotated-version"})

	require.Equal(t, stdhttp.StatusUnauthorized, performCredentialVersionRequest(router, beforeChange).Code)
	require.Equal(t, stdhttp.StatusUnauthorized, performCredentialVersionRequest(router, legacyBeforeChange).Code)
	require.Equal(t, stdhttp.StatusNoContent, performCredentialVersionRequest(router, afterChange).Code)
}

func TestRequireAuthAllowsLegacyTokensUntilFirstPasswordCredentialChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithCredentialVersionStore(nil, "Authorization", "Bearer", testJWTSecret, &fakeCredentialVersionStore{version: credentialVersionInitial})
	router := newCredentialVersionTestRouter(handler)
	legacyToken := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	require.Equal(t, stdhttp.StatusNoContent, performCredentialVersionRequest(router, legacyToken).Code)
}

func TestRequireAuthFailsClosedWhenCredentialVersionStoreIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithCredentialVersionStore(nil, "Authorization", "Bearer", testJWTSecret, &fakeCredentialVersionStore{err: errors.New("redis unavailable")})
	router := newCredentialVersionTestRouter(handler)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", credentialVersionClaim: credentialVersionInitial})

	recorder := performCredentialVersionRequest(router, token)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "redis unavailable")
}

func newCredentialVersionTestRouter(handler *Handler) *gin.Engine {
	router := gin.New()
	router.GET("/protected", handler.requireAuth(), func(c *gin.Context) {
		c.Status(stdhttp.StatusNoContent)
	})
	return router
}

func performCredentialVersionRequest(router stdhttp.Handler, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	return recorder
}

type fakeCredentialVersionStore struct {
	version string
	err     error
}

func (s *fakeCredentialVersionStore) Current(_ context.Context, _ int64) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.version, nil
}

type fakeCredentialVersionCommands struct {
	key      string
	value    string
	err      error
	setKey   string
	setValue interface{}
	setTTL   time.Duration
	setErr   error
}

func (c *fakeCredentialVersionCommands) Get(ctx context.Context, key string) *redis.StringCmd {
	c.key = key
	cmd := redis.NewStringCmd(ctx, "get", key)
	if c.err != nil {
		cmd.SetErr(c.err)
	} else {
		cmd.SetVal(c.value)
	}
	return cmd
}

func (c *fakeCredentialVersionCommands) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	c.setKey = key
	c.setValue = value
	c.setTTL = ttl
	cmd := redis.NewStatusCmd(ctx, "set", key, value, ttl)
	if c.setErr != nil {
		cmd.SetErr(c.setErr)
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

type fakeCredentialVersionAuthority struct {
	userID  int64
	version string
	err     error
	calls   int
}

func (s *fakeCredentialVersionAuthority) Current(_ context.Context, userID int64) (string, error) {
	s.calls++
	s.userID = userID
	if s.err != nil {
		return "", s.err
	}
	return s.version, nil
}

type fakeCredentialVersionUserClient struct {
	userpb.UserServiceClient
	request  *userpb.UserIDRequest
	response *userpb.CredentialVersionResponse
	err      error
}

func (c *fakeCredentialVersionUserClient) GetCredentialVersion(_ context.Context, req *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.CredentialVersionResponse, error) {
	c.request = req
	return c.response, c.err
}
