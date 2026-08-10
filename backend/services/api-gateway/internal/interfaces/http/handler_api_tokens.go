package http

import (
	"net/http"
	"strings"
	"time"

	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	defaultAPITokenListLimit = 30
	maxAPITokenListLimit     = 100
)

type createAPITokenRequest struct {
	Name               string   `json:"name"`
	Scopes             []string `json:"scopes"`
	Permission         []string `json:"permission"`
	ExpiresInDays      int32    `json:"expires_in_days"`
	ExpiresInDaysCamel int32    `json:"expiresInDays"`
}

type listAPITokensRequest struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

type revokeAPITokenRequest struct {
	TokenID      string `json:"token_id"`
	TokenIDCamel string `json:"tokenId"`
}

func (h *Handler) createAPIToken(c *gin.Context) {
	if h.clients == nil || h.clients.UserAPITokens == nil {
		writeError(c, http.StatusServiceUnavailable, "api token service unavailable", "service_unavailable")
		return
	}
	var req createAPITokenRequest
	if !bindJSON(c, &req) {
		return
	}
	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = req.Permission
	}
	expiresInDays := req.ExpiresInDays
	if expiresInDays == 0 {
		expiresInDays = req.ExpiresInDaysCamel
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAPITokens.CreateAPIToken(ctx, &userpb.CreateAPITokenRequest{
		UserId: currentUserID(c), Name: strings.TrimSpace(req.Name), Scopes: scopes,
		ExpiresInDays: expiresInDays, Client: sessionClientInfo(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	token := result.GetApiToken()
	data := gin.H{"token": result.GetToken(), "id": token.GetId(), "api_token": apiTokenPayload(token)}
	response.Success(c, data)
}

func (h *Handler) listAPITokens(c *gin.Context) {
	if h.clients == nil || h.clients.UserAPITokens == nil {
		writeError(c, http.StatusServiceUnavailable, "api token service unavailable", "service_unavailable")
		return
	}
	limit, offset, ok := apiTokenListPage(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAPITokens.ListAPITokens(ctx, &userpb.ListAPITokensRequest{UserId: currentUserID(c), Limit: limit, Offset: offset})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]gin.H, 0, len(result.GetItems()))
	for _, item := range result.GetItems() {
		items = append(items, apiTokenPayload(item))
	}
	response.Success(c, gin.H{"items": items, "total": result.GetTotal()})
}

func (h *Handler) revokeAPIToken(c *gin.Context) {
	if h.clients == nil || h.clients.UserAPITokens == nil {
		writeError(c, http.StatusServiceUnavailable, "api token service unavailable", "service_unavailable")
		return
	}
	tokenID, ok := apiTokenID(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAPITokens.RevokeAPIToken(ctx, &userpb.RevokeAPITokenRequest{UserId: currentUserID(c), TokenId: tokenID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	token := result.GetApiToken()
	if err := h.revokeSessionID(c, token.GetId(), token.GetExpiresAt()); err != nil {
		writeError(c, http.StatusServiceUnavailable, err.Error(), "service_unavailable")
		return
	}
	response.Success(c, gin.H{"api_token": apiTokenPayload(token)})
}

func apiTokenListPage(c *gin.Context) (int32, int32, bool) {
	limit, offset := int32(defaultAPITokenListLimit), int32(0)
	if c.Request.Method == http.MethodPost {
		var req listAPITokensRequest
		if c.Request.ContentLength != 0 {
			if !bindJSON(c, &req) {
				return 0, 0, false
			}
			limit, offset = req.Limit, req.Offset
		}
	} else {
		limit = queryInt32(c, "limit", defaultAPITokenListLimit)
		offset = queryInt32(c, "offset", 0)
	}
	if limit == 0 {
		limit = defaultAPITokenListLimit
	}
	if limit < 1 || limit > maxAPITokenListLimit || offset < 0 {
		writeError(c, http.StatusBadRequest, "invalid api token list parameters", "bad_request")
		return 0, 0, false
	}
	return limit, offset, true
}

func apiTokenID(c *gin.Context) (string, bool) {
	tokenID := strings.TrimSpace(c.Param("tokenId"))
	if tokenID == "" {
		var req revokeAPITokenRequest
		if !bindJSON(c, &req) {
			return "", false
		}
		tokenID = strings.TrimSpace(req.TokenID)
		if tokenID == "" {
			tokenID = strings.TrimSpace(req.TokenIDCamel)
		}
	}
	if tokenID == "" {
		writeError(c, http.StatusBadRequest, "invalid token id", "bad_request")
		return "", false
	}
	return tokenID, true
}

func apiTokenPayload(token *userpb.APITokenInfo) gin.H {
	if token == nil {
		return gin.H{}
	}
	active := token.GetRevokedAt() == 0 && (token.GetExpiresAt() == 0 || time.Unix(token.GetExpiresAt(), 0).After(time.Now())) && token.GetCredentialValid()
	return gin.H{
		"id": token.GetId(), "user_id": token.GetUserId(), "name": token.GetName(), "scopes": token.GetScopes(),
		"created_at": token.GetCreatedAt(), "expires_at": token.GetExpiresAt(), "revoked_at": token.GetRevokedAt(),
		"credential_valid": token.GetCredentialValid(), "active": active,
	}
}
