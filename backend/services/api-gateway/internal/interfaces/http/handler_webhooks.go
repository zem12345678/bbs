package http

import (
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/notificationpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

type createWebhookRequest struct {
	Name   *string   `json:"name"`
	URL    *string   `json:"url"`
	Secret *string   `json:"secret"`
	On     *[]string `json:"on"`
}

type webhookIDRequest struct {
	WebhookID string `json:"webhookId"`
}

type updateWebhookRequest struct {
	WebhookID string    `json:"webhookId"`
	Name      *string   `json:"name"`
	URL       *string   `json:"url"`
	Secret    *string   `json:"secret"`
	On        *[]string `json:"on"`
	Active    *bool     `json:"active"`
}

type testWebhookRequest struct {
	WebhookID string `json:"webhookId"`
	Type      string `json:"type"`
	Override  *struct {
		URL    string  `json:"url"`
		Secret *string `json:"secret"`
	} `json:"override"`
}

type webhookPayload struct {
	ID           string   `json:"id"`
	UserID       string   `json:"userId"`
	Name         string   `json:"name"`
	On           []string `json:"on"`
	URL          string   `json:"url"`
	Secret       *string  `json:"secret,omitempty"`
	Active       bool     `json:"active"`
	LatestSentAt *string  `json:"latestSentAt"`
	LatestStatus *int32   `json:"latestStatus"`
}

func (h *Handler) listWebhooks(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.Notification.ListWebhooks(ctx, &notificationpb.ListWebhooksRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := webhookPayloads(result.GetItems())
	if isWebhookCompatibilityRoute(c) {
		c.JSON(stdhttp.StatusOK, items)
		return
	}
	redactWebhookSecrets(items)
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) createWebhook(c *gin.Context) {
	var req createWebhookRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Name == nil || req.URL == nil || req.On == nil || strings.TrimSpace(*req.Name) == "" || strings.TrimSpace(*req.URL) == "" || len(*req.On) == 0 {
		writeError(c, stdhttp.StatusBadRequest, "name, url and on are required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.Notification.CreateWebhook(ctx, &notificationpb.CreateWebhookRequest{
		UserId: currentUserID(c), Name: *req.Name, Url: *req.URL, Secret: optionalWebhookString(req.Secret), On: *req.On,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	payload, ok := webhookPayloadFromProto(result.GetWebhook())
	if !ok {
		writeError(c, stdhttp.StatusInternalServerError, "notification service returned an empty webhook", "internal_error")
		return
	}
	if isWebhookCompatibilityRoute(c) {
		c.JSON(stdhttp.StatusOK, payload)
		return
	}
	payload.Secret = nil
	response.Success(c, gin.H{"webhook": payload})
}

func (h *Handler) showWebhook(c *gin.Context) {
	var req webhookIDRequest
	if strings.TrimSpace(c.Param("webhookId")) == "" && !bindJSON(c, &req) {
		return
	}
	webhookID, ok := webhookIDFromRequest(c, req.WebhookID)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.Notification.ShowWebhook(ctx, &notificationpb.ShowWebhookRequest{UserId: currentUserID(c), WebhookId: webhookID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	payload, present := webhookPayloadFromProto(result.GetWebhook())
	if !present {
		writeError(c, stdhttp.StatusInternalServerError, "notification service returned an empty webhook", "internal_error")
		return
	}
	if isWebhookCompatibilityRoute(c) {
		c.JSON(stdhttp.StatusOK, payload)
		return
	}
	payload.Secret = nil
	response.Success(c, gin.H{"webhook": payload})
}

func (h *Handler) updateWebhook(c *gin.Context) {
	var req updateWebhookRequest
	if !bindJSON(c, &req) {
		return
	}
	webhookID, ok := webhookIDFromRequest(c, req.WebhookID)
	if !ok {
		return
	}
	request := &notificationpb.UpdateWebhookRequest{
		UserId: currentUserID(c), WebhookId: webhookID, Name: optionalWebhookString(req.Name), Url: optionalWebhookString(req.URL),
		Secret: optionalWebhookString(req.Secret), SecretSet: req.Secret != nil, ActiveSet: req.Active != nil,
	}
	if req.On != nil {
		request.On = *req.On
	}
	if req.Active != nil {
		request.Active = *req.Active
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.Notification.UpdateWebhook(ctx, request)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if isWebhookCompatibilityRoute(c) {
		c.Status(stdhttp.StatusNoContent)
		return
	}
	payload, present := webhookPayloadFromProto(result.GetWebhook())
	if !present {
		writeError(c, stdhttp.StatusInternalServerError, "notification service returned an empty webhook", "internal_error")
		return
	}
	payload.Secret = nil
	response.Success(c, gin.H{"webhook": payload})
}

func (h *Handler) deleteWebhook(c *gin.Context) {
	var req webhookIDRequest
	if strings.TrimSpace(c.Param("webhookId")) == "" && !bindJSON(c, &req) {
		return
	}
	webhookID, ok := webhookIDFromRequest(c, req.WebhookID)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.Notification.DeleteWebhook(ctx, &notificationpb.DeleteWebhookRequest{UserId: currentUserID(c), WebhookId: webhookID}); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) testWebhook(c *gin.Context) {
	var req testWebhookRequest
	if !bindJSON(c, &req) {
		return
	}
	webhookID, ok := webhookIDFromRequest(c, req.WebhookID)
	if !ok {
		return
	}
	eventType := strings.TrimSpace(req.Type)
	if eventType == "" {
		writeError(c, stdhttp.StatusBadRequest, "type is required", "bad_request")
		return
	}
	request := &notificationpb.TestWebhookRequest{UserId: currentUserID(c), WebhookId: webhookID, Type: eventType}
	if req.Override != nil {
		request.OverrideUrl = req.Override.URL
		request.OverrideSecret = optionalWebhookString(req.Override.Secret)
		request.OverrideSecretSet = req.Override.Secret != nil
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.Notification.TestWebhook(ctx, request); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func webhookIDFromRequest(c *gin.Context, bodyID string) (int64, bool) {
	raw := strings.TrimSpace(c.Param("webhookId"))
	if raw == "" {
		raw = strings.TrimSpace(bodyID)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "webhookId must be a positive integer", "bad_request")
		return 0, false
	}
	return value, true
}

func optionalWebhookString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func isWebhookCompatibilityRoute(c *gin.Context) bool {
	return strings.Contains(c.FullPath(), "/i/webhooks/")
}

func webhookPayloads(items []*notificationpb.Webhook) []webhookPayload {
	result := make([]webhookPayload, 0, len(items))
	for _, item := range items {
		if payload, ok := webhookPayloadFromProto(item); ok {
			result = append(result, payload)
		}
	}
	return result
}

func webhookPayloadFromProto(item *notificationpb.Webhook) (webhookPayload, bool) {
	if item == nil {
		return webhookPayload{}, false
	}
	events := make([]string, len(item.GetOn()))
	copy(events, item.GetOn())
	var latestSentAt *string
	if item.GetLatestSentAt() > 0 {
		value := time.UnixMilli(item.GetLatestSentAt()).UTC().Format(time.RFC3339Nano)
		latestSentAt = &value
	}
	var latestStatus *int32
	if item.GetLatestStatus() > 0 {
		value := item.GetLatestStatus()
		latestStatus = &value
	}
	secret := item.GetSecret()
	return webhookPayload{
		ID: strconv.FormatInt(item.GetId(), 10), UserID: strconv.FormatInt(item.GetUserId(), 10), Name: item.GetName(),
		On: events, URL: item.GetUrl(), Secret: &secret, Active: item.GetActive(), LatestSentAt: latestSentAt, LatestStatus: latestStatus,
	}, true
}

func redactWebhookSecrets(items []webhookPayload) {
	for index := range items {
		items[index].Secret = nil
	}
}
