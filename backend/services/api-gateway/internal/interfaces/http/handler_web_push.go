package http

import (
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"net/url"
	"strconv"
	"strings"

	"api-gateway/api/proto/notificationpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	maxWebPushEndpointLength = 2048
	maxWebPushKeyLength      = 512
)

type webPushRegisterRequest struct {
	Endpoint        string `json:"endpoint"`
	Auth            string `json:"auth"`
	PublicKey       string `json:"publickey"`
	SendReadMessage bool   `json:"sendReadMessage"`
}

type webPushEndpointRequest struct {
	Endpoint string `json:"endpoint"`
}

func (h *Handler) webPushConfig(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.GetWebPushConfig(ctx, &notificationpb.GetWebPushConfigRequest{})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) registerWebPushSubscription(c *gin.Context) {
	var req webPushRegisterRequest
	if !bindStrictWebPushJSON(c, &req) {
		return
	}
	endpoint, ok := validWebPushEndpoint(c, req.Endpoint)
	if !ok {
		return
	}
	req.Auth = strings.TrimSpace(req.Auth)
	req.PublicKey = strings.TrimSpace(req.PublicKey)
	if req.Auth == "" || len(req.Auth) > maxWebPushKeyLength || req.PublicKey == "" || len(req.PublicKey) > maxWebPushKeyLength {
		writeError(c, stdhttp.StatusBadRequest, "invalid web push keys", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.RegisterWebPushSubscription(ctx, &notificationpb.RegisterWebPushSubscriptionRequest{
		UserId:          currentUserID(c),
		Endpoint:        endpoint,
		Auth:            req.Auth,
		PublicKey:       req.PublicKey,
		SendReadMessage: req.SendReadMessage,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, webPushRegisterPayload(resp))
}

func (h *Handler) showWebPushSubscription(c *gin.Context) {
	var req webPushEndpointRequest
	if !bindStrictWebPushJSON(c, &req) {
		return
	}
	endpoint, ok := validWebPushEndpoint(c, req.Endpoint)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.GetWebPushSubscription(ctx, &notificationpb.GetWebPushSubscriptionRequest{
		UserId: currentUserID(c), Endpoint: endpoint,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if !resp.GetRegistered() {
		c.Status(stdhttp.StatusNoContent)
		return
	}
	c.JSON(stdhttp.StatusOK, gin.H{
		"userId":          strconv.FormatInt(resp.GetUserId(), 10),
		"endpoint":        resp.GetEndpoint(),
		"sendReadMessage": resp.GetSendReadMessage(),
	})
}

func (h *Handler) unregisterWebPushSubscription(c *gin.Context) {
	var req webPushEndpointRequest
	if !bindStrictWebPushJSON(c, &req) {
		return
	}
	endpoint, ok := validWebPushEndpoint(c, req.Endpoint)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.Notification.UnregisterWebPushSubscription(ctx, &notificationpb.UnregisterWebPushSubscriptionRequest{
		UserId: currentUserID(c), Endpoint: endpoint,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func webPushRegisterPayload(resp *notificationpb.WebPushSubscriptionResponse) gin.H {
	state := resp.GetState()
	if state == "active" {
		state = "subscribed"
	}
	return gin.H{
		"state":           state,
		"key":             nil,
		"userId":          strconv.FormatInt(resp.GetUserId(), 10),
		"endpoint":        resp.GetEndpoint(),
		"sendReadMessage": resp.GetSendReadMessage(),
	}
}

func bindStrictWebPushJSON(c *gin.Context, out any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
		return false
	}
	return true
}

func validWebPushEndpoint(c *gin.Context, raw string) (string, bool) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" || len(endpoint) > maxWebPushEndpointLength {
		writeError(c, stdhttp.StatusBadRequest, "invalid web push endpoint", "bad_request")
		return "", false
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		writeError(c, stdhttp.StatusBadRequest, "invalid web push endpoint", "bad_request")
		return "", false
	}
	hostname := strings.ToLower(parsed.Hostname())
	localHTTP := hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && localHTTP) {
		writeError(c, stdhttp.StatusBadRequest, "web push endpoint must use https", "bad_request")
		return "", false
	}
	return endpoint, true
}
