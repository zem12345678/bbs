package http

import (
	"bytes"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"time"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxRegistryRequestBytes int64 = 2 << 20

type registryRequest struct {
	Key    *string         `json:"key"`
	Value  json.RawMessage `json:"value"`
	Scope  *[]string       `json:"scope"`
	Domain *string         `json:"domain"`
}

func (h *Handler) setRegistryItem(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.Set, "set") {
		return
	}
	var req registryRequest
	if !bindRegistryJSON(c, &req) || !requireRegistryKeyAndScope(c, &req) {
		return
	}
	if *req.Key == "" || len(req.Value) == 0 {
		writeError(c, stdhttp.StatusBadRequest, "key and value are required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.UserRegistry.SetRegistryItem(ctx, &userpb.SetRegistryItemRequest{
		UserId: currentUserID(c), Domain: registryDomain(c, req.Domain), Scope: *req.Scope,
		Key: *req.Key, ValueJson: append([]byte(nil), req.Value...),
	})
	if err != nil {
		writeRegistryRPCError(c, err, "")
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) getRegistryItem(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.Get, "get") {
		return
	}
	req, ok := bindRegistryKeyAndScope(c)
	if !ok {
		return
	}
	item, ok := h.fetchRegistryItem(c, req, registryDomain(c, req.Domain))
	if ok {
		writeRawRegistryJSON(c, item.GetValueJson())
	}
}

func (h *Handler) getAllRegistryItems(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.GetAll, "get-all") {
		return
	}
	req, ok := bindRegistryScope(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserRegistry.ListRegistryItems(ctx, &userpb.ListRegistryItemsRequest{
		UserId: currentUserID(c), Domain: registryDomain(c, req.Domain), Scope: *req.Scope,
	})
	if err != nil {
		writeRegistryRPCError(c, err, "")
		return
	}
	values := make(map[string]json.RawMessage, len(result.GetItems()))
	for _, item := range result.GetItems() {
		if item == nil || !json.Valid(item.GetValueJson()) {
			writeInvalidRegistryResponse(c)
			return
		}
		values[item.GetKey()] = append(json.RawMessage(nil), item.GetValueJson()...)
	}
	c.JSON(stdhttp.StatusOK, values)
}

func (h *Handler) getRegistryItemDetail(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.GetDetail, "get-detail") {
		return
	}
	req, ok := bindRegistryKeyAndScope(c)
	if !ok {
		return
	}
	item, ok := h.fetchRegistryItem(c, req, registryDomain(c, req.Domain))
	if !ok {
		return
	}
	c.JSON(stdhttp.StatusOK, struct {
		UpdatedAt string          `json:"updatedAt"`
		Value     json.RawMessage `json:"value"`
	}{
		UpdatedAt: time.UnixMilli(item.GetUpdatedAt()).UTC().Format(time.RFC3339Nano),
		Value:     append(json.RawMessage(nil), item.GetValueJson()...),
	})
}

func (h *Handler) getUnsecureRegistryItem(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.GetUnsecure, "get-unsecure") {
		return
	}
	var req registryRequest
	if !bindRegistryJSON(c, &req) {
		return
	}
	if req.Key == nil {
		writeError(c, stdhttp.StatusBadRequest, "key is required", "bad_request")
		return
	}
	if *req.Key != "reactions" && *req.Key != "defaultNoteVisibility" {
		c.Status(stdhttp.StatusNoContent)
		return
	}
	scope := []string{}
	if req.Scope != nil {
		scope = *req.Scope
	}
	req.Scope = &scope
	item, ok := h.fetchRegistryItem(c, &req, nil)
	if ok {
		writeRawRegistryJSON(c, item.GetValueJson())
	}
}

func (h *Handler) listRegistryKeys(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.Keys, "keys") {
		return
	}
	items, ok := h.listRegistryItems(c)
	if !ok {
		return
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil {
			keys = append(keys, item.GetKey())
		}
	}
	c.JSON(stdhttp.StatusOK, keys)
}

func (h *Handler) listRegistryKeysWithType(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.KeysWithType, "keys-with-type") {
		return
	}
	items, ok := h.listRegistryItems(c)
	if !ok {
		return
	}
	types := make(map[string]string, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		valueType, valid := registryJSONType(item.GetValueJson())
		if !valid {
			writeInvalidRegistryResponse(c)
			return
		}
		types[item.GetKey()] = valueType
	}
	c.JSON(stdhttp.StatusOK, types)
}

func (h *Handler) removeRegistryItem(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.Remove, "remove") {
		return
	}
	req, ok := bindRegistryKeyAndScope(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.UserRegistry.RemoveRegistryItem(ctx, &userpb.GetRegistryItemRequest{
		UserId: currentUserID(c), Domain: registryDomain(c, req.Domain), Scope: *req.Scope, Key: *req.Key,
	})
	if err != nil {
		writeRegistryRPCError(c, err, "NO_SUCH_KEY")
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) listRegistryScopesWithDomain(c *gin.Context) {
	if !h.allowRegistryRateLimit(c, h.registryRateLimits.ScopesWithDomain, "scopes-with-domain") {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserRegistry.ListRegistryScopeDomains(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRegistryRPCError(c, err, "")
		return
	}
	type scopeDomain struct {
		Scopes [][]string `json:"scopes"`
		Domain *string    `json:"domain"`
	}
	response := make([]scopeDomain, 0, len(result.GetItems()))
	for _, item := range result.GetItems() {
		if item == nil {
			continue
		}
		domain := registryDomainValue(item.GetDomain())
		scopes := make([][]string, 0, len(item.GetScopes()))
		for _, scope := range item.GetScopes() {
			if scope != nil {
				segments := make([]string, len(scope.GetSegments()))
				copy(segments, scope.GetSegments())
				scopes = append(scopes, segments)
			}
		}
		response = append(response, scopeDomain{Scopes: scopes, Domain: domain})
	}
	c.JSON(stdhttp.StatusOK, response)
}

func (h *Handler) listRegistryItems(c *gin.Context) ([]*userpb.RegistryItemInfo, bool) {
	req, ok := bindRegistryScope(c)
	if !ok {
		return nil, false
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserRegistry.ListRegistryItems(ctx, &userpb.ListRegistryItemsRequest{
		UserId: currentUserID(c), Domain: registryDomain(c, req.Domain), Scope: *req.Scope,
	})
	if err != nil {
		writeRegistryRPCError(c, err, "")
		return nil, false
	}
	return result.GetItems(), true
}

func (h *Handler) fetchRegistryItem(c *gin.Context, req *registryRequest, domain *userpb.RegistryDomain) (*userpb.RegistryItemInfo, bool) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserRegistry.GetRegistryItem(ctx, &userpb.GetRegistryItemRequest{
		UserId: currentUserID(c), Domain: domain, Scope: *req.Scope, Key: *req.Key,
	})
	if err != nil {
		writeRegistryRPCError(c, err, "NO_SUCH_KEY")
		return nil, false
	}
	item := result.GetItem()
	if item == nil || !json.Valid(item.GetValueJson()) {
		writeInvalidRegistryResponse(c)
		return nil, false
	}
	return item, true
}

func bindRegistryKeyAndScope(c *gin.Context) (*registryRequest, bool) {
	var req registryRequest
	if !bindRegistryJSON(c, &req) || !requireRegistryKeyAndScope(c, &req) {
		return nil, false
	}
	return &req, true
}

func bindRegistryScope(c *gin.Context) (*registryRequest, bool) {
	var req registryRequest
	if !bindRegistryJSON(c, &req) {
		return nil, false
	}
	if req.Scope == nil {
		writeError(c, stdhttp.StatusBadRequest, "scope is required", "bad_request")
		return nil, false
	}
	return &req, true
}

func bindRegistryJSON(c *gin.Context, out any) bool {
	c.Request.Body = stdhttp.MaxBytesReader(c.Writer, c.Request.Body, maxRegistryRequestBytes)
	if err := c.ShouldBindJSON(out); err != nil {
		var maxBytesError *stdhttp.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(c, stdhttp.StatusRequestEntityTooLarge, "registry request body is too large", "payload_too_large")
			return false
		}
		writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
		return false
	}
	return true
}

func requireRegistryKeyAndScope(c *gin.Context, req *registryRequest) bool {
	if req.Key == nil || req.Scope == nil {
		writeError(c, stdhttp.StatusBadRequest, "key and scope are required", "bad_request")
		return false
	}
	return true
}

func registryDomain(c *gin.Context, requested *string) *userpb.RegistryDomain {
	if currentAuthTokenType(c) == apiTokenType {
		return &userpb.RegistryDomain{Value: currentSessionID(c)}
	}
	if requested == nil {
		return nil
	}
	return &userpb.RegistryDomain{Value: *requested}
}

func registryDomainValue(domain *userpb.RegistryDomain) *string {
	if domain == nil {
		return nil
	}
	value := domain.GetValue()
	return &value
}

func registryJSONType(value []byte) (string, bool) {
	value = bytes.TrimSpace(value)
	if !json.Valid(value) || len(value) == 0 {
		return "", false
	}
	switch value[0] {
	case 'n':
		return "null", true
	case '[':
		return "array", true
	case '"':
		return "string", true
	case 't', 'f':
		return "boolean", true
	case '{':
		return "object", true
	default:
		return "number", true
	}
}

func writeRawRegistryJSON(c *gin.Context, value []byte) {
	if !json.Valid(value) {
		writeInvalidRegistryResponse(c)
		return
	}
	c.Data(stdhttp.StatusOK, "application/json; charset=utf-8", value)
}

func writeInvalidRegistryResponse(c *gin.Context) {
	writeError(c, stdhttp.StatusBadGateway, "invalid registry response", "bad_gateway")
}

func writeRegistryRPCError(c *gin.Context, err error, notFoundCode string) {
	code := status.Code(err)
	switch {
	case code == codes.InvalidArgument:
		writeError(c, stdhttp.StatusBadRequest, "Invalid param.", "INVALID_PARAM")
	case code == codes.NotFound && notFoundCode != "":
		writeError(c, stdhttp.StatusBadRequest, "No such key.", notFoundCode)
	case code == codes.FailedPrecondition:
		writeError(c, stdhttp.StatusBadRequest, "Too many registry keys.", "TOO_MANY_REGISTRY_KEYS")
	default:
		writeRPCError(c, err)
	}
}
