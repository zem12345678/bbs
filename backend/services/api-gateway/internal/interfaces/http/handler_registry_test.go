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

type fakeRegistryClient struct {
	userpb.UserServiceClient
	setReq    *userpb.SetRegistryItemRequest
	getReq    *userpb.GetRegistryItemRequest
	listReq   *userpb.ListRegistryItemsRequest
	removeReq *userpb.GetRegistryItemRequest
	scopeReq  *userpb.UserIDRequest
	setErr    error
	getErr    error
	listErr   error
	removeErr error
	scopeErr  error
	item      *userpb.RegistryItemInfo
	items     []*userpb.RegistryItemInfo
	groups    []*userpb.RegistryScopeDomainInfo
}

func (f *fakeRegistryClient) SetRegistryItem(_ context.Context, req *userpb.SetRegistryItemRequest, _ ...grpc.CallOption) (*userpb.RegistryItemResponse, error) {
	f.setReq = req
	return &userpb.RegistryItemResponse{Item: f.item}, f.setErr
}

func (f *fakeRegistryClient) GetRegistryItem(_ context.Context, req *userpb.GetRegistryItemRequest, _ ...grpc.CallOption) (*userpb.RegistryItemResponse, error) {
	f.getReq = req
	return &userpb.RegistryItemResponse{Item: f.item}, f.getErr
}

func (f *fakeRegistryClient) ListRegistryItems(_ context.Context, req *userpb.ListRegistryItemsRequest, _ ...grpc.CallOption) (*userpb.RegistryItemListResponse, error) {
	f.listReq = req
	return &userpb.RegistryItemListResponse{Items: f.items}, f.listErr
}

func (f *fakeRegistryClient) RemoveRegistryItem(_ context.Context, req *userpb.GetRegistryItemRequest, _ ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	f.removeReq = req
	return &userpb.SimpleResponse{Success: true}, f.removeErr
}

func (f *fakeRegistryClient) ListRegistryScopeDomains(_ context.Context, req *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.RegistryScopeDomainListResponse, error) {
	f.scopeReq = req
	return &userpb.RegistryScopeDomainListResponse{Items: f.groups}, f.scopeErr
}

func TestRegistryRoutesPreserveRawValuesAndResponseShapes(t *testing.T) {
	updatedAt := time.Date(2026, 8, 19, 8, 7, 6, 123000000, time.UTC)
	emptyDomain := ""
	client := &fakeRegistryClient{
		item: &userpb.RegistryItemInfo{
			Id: "9007199254740993", UserId: 42, Domain: &userpb.RegistryDomain{Value: emptyDomain},
			Scope: []string{"client", "preferences"}, Key: "large", ValueJson: []byte(`{"value":9223372036854775807}`),
			UpdatedAt: updatedAt.UnixMilli(),
		},
		items: []*userpb.RegistryItemInfo{
			{Key: "nil", ValueJson: []byte(`null`)},
			{Key: "array", ValueJson: []byte(`[9223372036854775807]`)},
			{Key: "number", ValueJson: []byte(`9223372036854775807`)},
			{Key: "string", ValueJson: []byte(`"text"`)},
			{Key: "boolean", ValueJson: []byte(`true`)},
			{Key: "object", ValueJson: []byte(`{"id":9223372036854775807}`)},
		},
		groups: []*userpb.RegistryScopeDomainInfo{
			{Domain: nil, Scopes: []*userpb.RegistryScope{{Segments: []string{}}, {Segments: []string{"native"}}}},
			{Domain: &userpb.RegistryDomain{Value: ""}, Scopes: []*userpb.RegistryScope{{Segments: []string{"empty"}}}},
		},
	}
	router := newRegistryTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	set := performRegistryRequest(router, "/api/v1/i/registry/set", `{"key":" large ","value":{"id":9223372036854775807},"scope":["client","preferences"],"domain":""}`, token)
	require.Equal(t, stdhttp.StatusNoContent, set.Code, set.Body.String())
	require.Equal(t, int64(42), client.setReq.GetUserId())
	require.Equal(t, " large ", client.setReq.GetKey())
	require.Equal(t, []string{"client", "preferences"}, client.setReq.GetScope())
	require.NotNil(t, client.setReq.GetDomain())
	require.Equal(t, "", client.setReq.GetDomain().GetValue())
	require.JSONEq(t, `{"id":9223372036854775807}`, string(client.setReq.GetValueJson()))

	get := performRegistryRequest(router, "/api/v1/i/registry/get", `{"key":"large","scope":[],"domain":null}`, token)
	require.Equal(t, stdhttp.StatusOK, get.Code, get.Body.String())
	require.JSONEq(t, `{"value":9223372036854775807}`, get.Body.String())

	all := performRegistryRequest(router, "/api/v1/i/registry/get-all", `{"scope":[],"domain":null}`, token)
	require.Equal(t, stdhttp.StatusOK, all.Code, all.Body.String())
	require.Contains(t, all.Body.String(), `"number":9223372036854775807`)
	require.Contains(t, all.Body.String(), `"array":[9223372036854775807]`)

	detail := performRegistryRequest(router, "/api/v1/i/registry/get-detail", `{"key":"large","scope":[],"domain":null}`, token)
	require.Equal(t, stdhttp.StatusOK, detail.Code, detail.Body.String())
	var detailBody struct {
		UpdatedAt string          `json:"updatedAt"`
		Value     json.RawMessage `json:"value"`
	}
	require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &detailBody))
	require.Equal(t, updatedAt.Format(time.RFC3339Nano), detailBody.UpdatedAt)
	require.JSONEq(t, `{"value":9223372036854775807}`, string(detailBody.Value))

	keys := performRegistryRequest(router, "/api/v1/i/registry/keys", `{"scope":[],"domain":null}`, token)
	require.Equal(t, stdhttp.StatusOK, keys.Code, keys.Body.String())
	require.JSONEq(t, `["nil","array","number","string","boolean","object"]`, keys.Body.String())

	types := performRegistryRequest(router, "/api/v1/i/registry/keys-with-type", `{"scope":[],"domain":null}`, token)
	require.Equal(t, stdhttp.StatusOK, types.Code, types.Body.String())
	require.JSONEq(t, `{"nil":"null","array":"array","number":"number","string":"string","boolean":"boolean","object":"object"}`, types.Body.String())

	remove := performRegistryRequest(router, "/api/v1/i/registry/remove", `{"key":"missing","scope":[],"domain":null}`, token)
	require.Equal(t, stdhttp.StatusNoContent, remove.Code, remove.Body.String())
	require.Equal(t, "missing", client.removeReq.GetKey())

	scopes := performRegistryRequest(router, "/api/v1/i/registry/scopes-with-domain", `{}`, token)
	require.Equal(t, stdhttp.StatusOK, scopes.Code, scopes.Body.String())
	require.JSONEq(t, `[{"domain":null,"scopes":[[],["native"]]},{"domain":"","scopes":[["empty"]]}]`, scopes.Body.String())
	require.Equal(t, int64(42), client.scopeReq.GetId())
}

func TestRegistryAPITokenDomainIsolationAndSecureRoute(t *testing.T) {
	client := &fakeRegistryClient{item: &userpb.RegistryItemInfo{ValueJson: []byte(`true`)}}
	router := newRegistryTestRouter(client)
	claims := jwt.MapClaims{
		"sub": "42", "jti": "api-token-domain", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: credentialVersionInitial, "token_type": apiTokenType, "scopes": []string{"read", "write"},
	}
	token := signedAuthToken(t, claims)

	set := performRegistryRequest(router, "/i/registry/set", `{"key":"x","value":true,"scope":[],"domain":"attacker-domain"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, set.Code, set.Body.String())
	require.Equal(t, "api-token-domain", client.setReq.GetDomain().GetValue())

	get := performRegistryRequest(router, "/api/i/registry/get", `{"key":"x","scope":[],"domain":null}`, token)
	require.Equal(t, stdhttp.StatusOK, get.Code, get.Body.String())
	require.Equal(t, "api-token-domain", client.getReq.GetDomain().GetValue())

	unsecure := performRegistryRequest(router, "/api/v1/i/registry/get-unsecure", `{"key":"reactions"}`, token)
	require.Equal(t, stdhttp.StatusOK, unsecure.Code, unsecure.Body.String())
	require.Nil(t, client.getReq.GetDomain())
	require.Empty(t, client.getReq.GetScope())

	client.getReq = nil
	unsupported := performRegistryRequest(router, "/api/v1/i/registry/get-unsecure", `{"key":"private"}`, token)
	require.Equal(t, stdhttp.StatusNoContent, unsupported.Code, unsupported.Body.String())
	require.Nil(t, client.getReq)

	secure := performRegistryRequest(router, "/api/v1/i/registry/scopes-with-domain", `{}`, token)
	require.Equal(t, stdhttp.StatusForbidden, secure.Code, secure.Body.String())
	require.Nil(t, client.scopeReq)
}

func TestRegistryCompatibilityErrorsAndRateLimit(t *testing.T) {
	client := &fakeRegistryClient{getErr: status.Error(codes.NotFound, "registry item not found")}
	handler := NewHandler(&clients.Clients{UserRegistry: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(handler)(router)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})

	missing := performRegistryRequest(router, "/api/v1/i/registry/get", `{"key":"missing","scope":[]}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, missing.Code, missing.Body.String())
	require.Contains(t, missing.Body.String(), `"legacy_code":"NO_SUCH_KEY"`)

	client.getErr = nil
	client.setErr = status.Error(codes.FailedPrecondition, "registry key limit reached")
	overflow := performRegistryRequest(router, "/api/v1/i/registry/set", `{"key":"new","value":true,"scope":[]}`, token)
	require.Equal(t, stdhttp.StatusBadRequest, overflow.Code, overflow.Body.String())
	require.Contains(t, overflow.Body.String(), `"legacy_code":"TOO_MANY_REGISTRY_KEYS"`)

	handler.SetRegistryRateLimits(RegistryRateLimits{Keys: fixedRegistryLimiter{limited: true}})
	limited := performRegistryRequest(router, "/api/v1/i/registry/keys", `{"scope":[]}`, token)
	require.Equal(t, stdhttp.StatusTooManyRequests, limited.Code, limited.Body.String())
	require.Contains(t, limited.Body.String(), `"legacy_code":"RATE_LIMIT_EXCEEDED"`)
}

func TestRegistrySetRouteAliases(t *testing.T) {
	client := &fakeRegistryClient{}
	router := newRegistryTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})
	for _, path := range []string{"/i/registry/set", "/api/i/registry/set", "/api/v1/i/registry/set"} {
		recorder := performRegistryRequest(router, path, `{"key":"alias","value":true,"scope":[]}`, token)
		require.Equal(t, stdhttp.StatusNoContent, recorder.Code, "%s: %s", path, recorder.Body.String())
	}
}

func TestRegistryRejectsOversizedRequestBody(t *testing.T) {
	client := &fakeRegistryClient{}
	router := newRegistryTestRouter(client)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42"})
	body := `{"key":"large","value":"` + strings.Repeat("a", int(maxRegistryRequestBytes)) + `","scope":[]}`

	response := performRegistryRequest(router, "/api/v1/i/registry/set", body, token)
	require.Equal(t, stdhttp.StatusRequestEntityTooLarge, response.Code, response.Body.String())
	require.Nil(t, client.setReq)
}

type fixedRegistryLimiter struct {
	limited bool
	err     error
}

func (f fixedRegistryLimiter) Limit(context.Context, string) (bool, error) {
	return f.limited, f.err
}

func newRegistryTestRouter(client *fakeRegistryClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInitControllers(NewHandler(&clients.Clients{UserRegistry: client}, "Authorization", "Bearer", testJWTSecret))(router)
	return router
}

func performRegistryRequest(router *gin.Engine, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
