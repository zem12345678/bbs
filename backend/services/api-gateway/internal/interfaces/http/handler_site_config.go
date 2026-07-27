package http

import (
	"encoding/json"
	"strings"

	"api-gateway/api/proto/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

type siteNavigationItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

var publicSiteNavigationKeys = map[string]struct{}{
	"home":      {},
	"plaza":     {},
	"circles":   {},
	"chat":      {},
	"help":      {},
	"resources": {},
	"shop":      {},
	"member":    {},
	"more":      {},
}

func (h *Handler) siteConfig(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListPublicSettings(ctx, &adminpb.ListPublicSettingsRequest{})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	settings := make(map[string]string, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		settings[strings.ToLower(strings.TrimSpace(item.GetKey()))] = item.GetValue()
	}
	response.Success(c, gin.H{
		"site_name":        strings.TrimSpace(settings["site_name"]),
		"site_description": strings.TrimSpace(settings["site_description"]),
		"seo_keywords":     strings.TrimSpace(settings["seo_keywords"]),
		"logo_url":         strings.TrimSpace(settings["site_logo_url"]),
		"navigation":       parseSiteNavigation(settings["site_navigation"]),
	})
}

func parseSiteNavigation(raw string) []siteNavigationItem {
	var input []siteNavigationItem
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return []siteNavigationItem{}
	}
	items := make([]siteNavigationItem, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		item.Key = strings.ToLower(strings.TrimSpace(item.Key))
		item.Label = strings.TrimSpace(item.Label)
		if _, allowed := publicSiteNavigationKeys[item.Key]; !allowed {
			continue
		}
		if _, duplicate := seen[item.Key]; duplicate {
			continue
		}
		seen[item.Key] = struct{}{}
		items = append(items, item)
	}
	return items
}
