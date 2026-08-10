package http

import (
	"net/http"
	"runtime"
	"strings"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const instanceSoftwareVersion = "local"

type instanceMetaRequest struct {
	Detail *bool `json:"detail"`
}

func (h *Handler) instancePing(c *gin.Context) {
	response.Success(c, gin.H{"pong": time.Now().UnixMilli()})
}

func (h *Handler) instanceMeta(c *gin.Context) {
	detail, ok := instanceMetaDetail(c)
	if !ok {
		return
	}
	settings, ok := h.loadPublicSiteSettings(c)
	if !ok {
		return
	}
	ads, ok := h.loadPublicAds(c)
	if !ok {
		return
	}

	payload := gin.H{
		"name":          strings.TrimSpace(settings["site_name"]),
		"shortName":     strings.TrimSpace(settings["site_name"]),
		"uri":           h.publicURL(c, ""),
		"description":   strings.TrimSpace(settings["site_description"]),
		"version":       instanceSoftwareVersion,
		"iconUrl":       strings.TrimSpace(settings["site_logo_url"]),
		"logoImageUrl":  strings.TrimSpace(settings["site_logo_url"]),
		"ads":           ads,
		"notesPerOneAd": 0,
	}
	if detail {
		payload["seoKeywords"] = strings.TrimSpace(settings["seo_keywords"])
		payload["navigation"] = parseSiteNavigation(settings["site_navigation"])
		payload["software"] = gin.H{"name": "bbs", "version": instanceSoftwareVersion}
		payload["features"] = gin.H{
			"articles": true,
			"topics":   true,
			"comments": true,
			"mall":     true,
			"chat":     true,
			"search":   true,
		}
	}
	response.Success(c, payload)
}

func (h *Handler) loadPublicAds(c *gin.Context) ([]publicAdPayload, bool) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListActiveAds(ctx, &adminpb.ListActiveAdsRequest{})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	items := make([]publicAdPayload, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, publicAdFromProto(item))
	}
	return items, true
}

func (h *Handler) instanceServerInfo(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	response.Success(c, gin.H{
		"machine": "bbs-api-gateway",
		"cpu": gin.H{
			"model": runtime.GOARCH,
			"cores": runtime.NumCPU(),
		},
		"mem": gin.H{"total": mem.Sys},
		"fs": gin.H{
			"total": 0,
			"used":  0,
		},
		"software": gin.H{"name": "bbs", "version": instanceSoftwareVersion},
	})
}

func (h *Handler) instanceStats(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.User == nil || h.clients.Content == nil || h.clients.Comment == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "public stats dependencies unavailable"))
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()

	users, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{Status: userStatusActive, Page: 1, PageSize: 1})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	articles, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{Status: contentStatusPublished, Limit: 1})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	topics, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{Status: contentStatusPublished, Limit: 1})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	comments, err := h.clients.Comment.ListRecentComments(ctx, &commentpb.ListRecentCommentsRequest{Status: 1, Page: 1, PageSize: 1})
	if err != nil {
		writeRPCError(c, err)
		return
	}

	notesCount := articles.GetTotal() + topics.GetTotal()
	response.Success(c, gin.H{
		"notesCount":         notesCount,
		"originalNotesCount": notesCount,
		"usersCount":         users.GetTotal(),
		"originalUsersCount": users.GetTotal(),
		"instances":          1,
		"driveUsageLocal":    0,
		"driveUsageRemote":   0,
		"articlesCount":      articles.GetTotal(),
		"topicsCount":        topics.GetTotal(),
		"commentsCount":      comments.GetTotal(),
	})
}

func (h *Handler) loadPublicSiteSettings(c *gin.Context) (map[string]string, bool) {
	if h == nil || h.clients == nil || h.clients.Admin == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "admin service unavailable"))
		return nil, false
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListPublicSettings(ctx, &adminpb.ListPublicSettingsRequest{})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	settings := make(map[string]string, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		settings[strings.ToLower(strings.TrimSpace(item.GetKey()))] = item.GetValue()
	}
	return settings, true
}

func instanceMetaDetail(c *gin.Context) (bool, bool) {
	detail := queryBool(c, "detail", true)
	if c.Request.Method != http.MethodPost || c.Request.Body == nil || c.Request.ContentLength == 0 {
		return detail, true
	}
	var req instanceMetaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body", "bad_request")
		return false, false
	}
	if req.Detail != nil {
		detail = *req.Detail
	}
	return detail, true
}
