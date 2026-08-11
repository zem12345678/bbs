package http

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"api-gateway/api/proto/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

type emojiMutationRequest struct {
	ID                                      jsonInt64       `json:"id"`
	Name                                    *string         `json:"name"`
	URL                                     *string         `json:"url"`
	FileID                                  string          `json:"fileId"`
	Category                                json.RawMessage `json:"category"`
	Aliases                                 json.RawMessage `json:"aliases"`
	License                                 json.RawMessage `json:"license"`
	IsSensitive                             *bool           `json:"isSensitive"`
	LocalOnly                               *bool           `json:"localOnly"`
	RoleIDsThatCanBeUsedThisEmojiAsReaction json.RawMessage `json:"roleIdsThatCanBeUsedThisEmojiAsReaction"`
}

type emojiView struct {
	ID                                      string   `json:"id"`
	UpdatedAt                               *string  `json:"updatedAt"`
	Name                                    string   `json:"name"`
	Host                                    *string  `json:"host"`
	URL                                     string   `json:"url"`
	PublicURL                               string   `json:"publicUrl"`
	OriginalURL                             string   `json:"originalUrl"`
	URI                                     *string  `json:"uri"`
	Type                                    *string  `json:"type"`
	Aliases                                 []string `json:"aliases"`
	Category                                *string  `json:"category"`
	License                                 *string  `json:"license"`
	IsSensitive                             bool     `json:"isSensitive"`
	LocalOnly                               bool     `json:"localOnly"`
	RoleIDsThatCanBeUsedThisEmojiAsReaction []string `json:"roleIdsThatCanBeUsedThisEmojiAsReaction"`
}

type emojiV2ListRequest struct {
	Query    *emojiV2Query `json:"query"`
	SinceID  jsonInt64     `json:"sinceId"`
	UntilID  jsonInt64     `json:"untilId"`
	Limit    int32         `json:"limit"`
	Page     int32         `json:"page"`
	SortKeys []string      `json:"sortKeys"`
}

type emojiReactionRoleView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type emojiV2View struct {
	ID                                      string                  `json:"id"`
	UpdatedAt                               *string                 `json:"updatedAt"`
	Name                                    string                  `json:"name"`
	Host                                    *string                 `json:"host"`
	PublicURL                               string                  `json:"publicUrl"`
	OriginalURL                             string                  `json:"originalUrl"`
	URI                                     *string                 `json:"uri"`
	Type                                    *string                 `json:"type"`
	Aliases                                 []string                `json:"aliases"`
	Category                                *string                 `json:"category"`
	License                                 *string                 `json:"license"`
	IsSensitive                             bool                    `json:"isSensitive"`
	LocalOnly                               bool                    `json:"localOnly"`
	RoleIDsThatCanBeUsedThisEmojiAsReaction []emojiReactionRoleView `json:"roleIdsThatCanBeUsedThisEmojiAsReaction"`
}

type emojiV2Query struct {
	UpdatedAtFrom string   `json:"updatedAtFrom"`
	UpdatedAtTo   string   `json:"updatedAtTo"`
	Name          string   `json:"name"`
	Host          string   `json:"host"`
	URI           string   `json:"uri"`
	PublicURL     string   `json:"publicUrl"`
	OriginalURL   string   `json:"originalUrl"`
	Type          string   `json:"type"`
	Aliases       string   `json:"aliases"`
	Category      string   `json:"category"`
	License       string   `json:"license"`
	IsSensitive   *bool    `json:"isSensitive"`
	LocalOnly     *bool    `json:"localOnly"`
	HostType      string   `json:"hostType"`
	RoleIDs       []string `json:"roleIds"`
}

type emojiBulkAliasesRequest struct {
	IDs     json.RawMessage `json:"ids"`
	Aliases json.RawMessage `json:"aliases"`
}

type emojiBulkCategoryRequest struct {
	IDs      json.RawMessage `json:"ids"`
	Category json.RawMessage `json:"category"`
}

type emojiBulkLicenseRequest struct {
	IDs     json.RawMessage `json:"ids"`
	License json.RawMessage `json:"license"`
}

func (h *Handler) listEmojis(c *gin.Context) {
	emojis, ok := h.publicEmojiList(c)
	if !ok {
		return
	}
	response.Success(c, gin.H{"emojis": emojis})
}

func (h *Handler) listEmojisCompat(c *gin.Context) {
	emojis, ok := h.publicEmojiList(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"emojis": emojis})
}

func (h *Handler) publicEmojiList(c *gin.Context) ([]gin.H, bool) {
	items, ok := h.loadAllEmojis(c, nil)
	if !ok {
		return nil, false
	}
	emojis := make([]gin.H, 0, len(items))
	for _, item := range items {
		view := toEmojiView(item)
		emojis = append(emojis, gin.H{
			"aliases": view.Aliases, "name": view.Name, "category": view.Category, "url": view.URL,
			"localOnly": view.LocalOnly, "isSensitive": view.IsSensitive,
			"roleIdsThatCanBeUsedThisEmojiAsReaction": view.RoleIDsThatCanBeUsedThisEmojiAsReaction,
		})
	}
	return emojis, true
}

func (h *Handler) getEmoji(c *gin.Context) {
	h.getEmojiWithResponse(c, false)
}

func (h *Handler) getEmojiCompat(c *gin.Context) {
	h.getEmojiWithResponse(c, true)
}

func (h *Handler) getEmojiWithResponse(c *gin.Context, raw bool) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" && c.Request.Method != "GET" {
		var req struct {
			Name string `json:"name"`
		}
		if !bindJSON(c, &req) {
			return
		}
		name = strings.TrimSpace(req.Name)
	}
	if name == "" {
		writeError(c, 400, "emoji name is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.GetEmoji(ctx, &adminpb.GetEmojiRequest{Name: name})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	view := toEmojiView(resp.GetEmoji())
	if raw {
		c.JSON(http.StatusOK, view)
		return
	}
	response.Success(c, view)
}

func (h *Handler) listAdminEmojis(c *gin.Context) {
	h.listAdminEmojisWith(c, strings.TrimSpace(c.Query("query")), queryInt32(c, "limit", 20), queryInt32(c, "offset", 0))
}

func (h *Handler) listAdminEmojisCompat(c *gin.Context) {
	var req struct {
		Query   string    `json:"query"`
		Limit   int32     `json:"limit"`
		Offset  int32     `json:"offset"`
		SinceID jsonInt64 `json:"sinceId"`
		UntilID jsonInt64 `json:"untilId"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Limit < 1 || req.Limit > 100 || req.Offset < 0 || req.SinceID.Int64() < 0 || req.UntilID.Int64() < 0 {
		writeError(c, http.StatusBadRequest, "invalid emoji list request", "bad_request")
		return
	}
	items, ok := h.loadAllEmojisQuery(c, currentActor(c), req.Query)
	if !ok {
		return
	}
	items = filterCompatAdminEmojis(items, req.Query, req.SinceID.Int64(), req.UntilID.Int64())
	start := int(req.Offset)
	if start > len(items) {
		start = len(items)
	}
	end := start + int(req.Limit)
	if end > len(items) {
		end = len(items)
	}
	c.JSON(http.StatusOK, toEmojiViews(items[start:end]))
}

func (h *Handler) listAdminEmojisV2(c *gin.Context) {
	var req emojiV2ListRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Limit == 0 {
		req.Limit = 10
	} else if req.Limit < 1 || req.Limit > 100 {
		writeError(c, http.StatusBadRequest, "invalid emoji list limit", "bad_request")
		return
	}
	if req.Page < 0 {
		writeError(c, http.StatusBadRequest, "invalid emoji list page", "bad_request")
		return
	}
	items, ok := h.loadAllEmojis(c, currentActor(c))
	if !ok {
		return
	}
	items, ok = filterV2Emojis(c, items, req)
	if !ok {
		return
	}
	if len(req.SortKeys) == 0 {
		req.SortKeys = []string{"-id"}
	}
	if !sortV2Emojis(c, items, req.SortKeys) {
		return
	}
	allCount := len(items)
	offset := 0
	if req.Page > 1 {
		offset = int(req.Page-1) * int(req.Limit)
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + int(req.Limit)
	if end > len(items) {
		end = len(items)
	}
	pageItems := items[offset:end]
	roles, ok := h.loadEmojiReactionRoles(c, currentActor(c), pageItems)
	if !ok {
		return
	}
	page := toEmojiV2Views(pageItems, roles)
	c.JSON(http.StatusOK, gin.H{
		"emojis": page, "count": len(page), "allCount": allCount,
		"allPages": int(math.Ceil(float64(allCount) / float64(req.Limit))),
	})
}

func (h *Handler) loadAllEmojis(c *gin.Context, actor *adminpb.Actor) ([]*adminpb.EmojiInfo, bool) {
	return h.loadAllEmojisQuery(c, actor, "")
}

func (h *Handler) loadAllEmojisQuery(c *gin.Context, actor *adminpb.Actor, query string) ([]*adminpb.EmojiInfo, bool) {
	const pageSize int32 = 1000
	items := make([]*adminpb.EmojiInfo, 0)
	for offset := int32(0); ; offset += pageSize {
		ctx, cancel := rpcContext(c)
		resp, err := h.clients.Admin.ListEmojis(ctx, &adminpb.ListEmojisRequest{Actor: actor, Query: strings.TrimSpace(query), Limit: pageSize, Offset: offset})
		cancel()
		if err != nil {
			writeRPCError(c, err)
			return nil, false
		}
		batch := resp.GetItems()
		items = append(items, batch...)
		if len(batch) == 0 || int64(len(items)) >= resp.GetTotal() || len(batch) < int(pageSize) {
			return items, true
		}
	}
}

func filterCompatAdminEmojis(items []*adminpb.EmojiInfo, query string, sinceID, untilID int64) []*adminpb.EmojiInfo {
	filtered := make([]*adminpb.EmojiInfo, 0, len(items))
	for _, item := range items {
		if item == nil || (sinceID > 0 && item.GetId() <= sinceID) || (untilID > 0 && item.GetId() >= untilID) {
			continue
		}
		filtered = append(filtered, item)
	}
	if strings.TrimSpace(query) != "" {
		sort.SliceStable(filtered, func(i, j int) bool {
			leftLength := utf8.RuneCountInString(filtered[i].GetName())
			rightLength := utf8.RuneCountInString(filtered[j].GetName())
			if leftLength != rightLength {
				return leftLength < rightLength
			}
			return filtered[i].GetId() > filtered[j].GetId()
		})
		return filtered
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if sinceID > 0 && untilID == 0 {
			return filtered[i].GetId() < filtered[j].GetId()
		}
		return filtered[i].GetId() > filtered[j].GetId()
	})
	return filtered
}

func (h *Handler) loadEmojiReactionRoles(c *gin.Context, actor *adminpb.Actor, items []*adminpb.EmojiInfo) (map[string]emojiReactionRoleView, bool) {
	roles := make(map[string]emojiReactionRoleView)
	for _, item := range items {
		if item == nil {
			continue
		}
		for _, rawRoleID := range item.GetRoleIdsThatCanBeUsedThisEmojiAsReaction() {
			roleID := strings.TrimSpace(rawRoleID)
			if roleID != "" {
				roles[roleID] = emojiReactionRoleView{ID: roleID, Name: ""}
			}
		}
	}
	if len(roles) == 0 {
		return roles, true
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListRoles(ctx, &adminpb.ListRolesRequest{Actor: actor})
	if err != nil {
		return roles, true
	}
	for _, role := range resp.GetItems() {
		if role == nil {
			continue
		}
		id := strconv.FormatInt(role.GetId(), 10)
		if _, referenced := roles[id]; referenced {
			roles[id] = emojiReactionRoleView{ID: id, Name: role.GetName()}
		}
	}
	return roles, true
}

func filterV2Emojis(c *gin.Context, items []*adminpb.EmojiInfo, req emojiV2ListRequest) ([]*adminpb.EmojiInfo, bool) {
	var fromMillis, toMillis int64
	var hasFrom, hasTo bool
	if req.Query != nil {
		var ok bool
		if fromMillis, hasFrom, ok = emojiTimeFilter(req.Query.UpdatedAtFrom); !ok {
			writeError(c, http.StatusBadRequest, "invalid updatedAtFrom", "bad_request")
			return nil, false
		}
		if toMillis, hasTo, ok = emojiTimeFilter(req.Query.UpdatedAtTo); !ok {
			writeError(c, http.StatusBadRequest, "invalid updatedAtTo", "bad_request")
			return nil, false
		}
		hostType := strings.ToLower(strings.TrimSpace(req.Query.HostType))
		if hostType != "" && hostType != "all" && hostType != "local" && hostType != "remote" {
			writeError(c, http.StatusBadRequest, "invalid emoji hostType", "bad_request")
			return nil, false
		}
	}
	filtered := make([]*adminpb.EmojiInfo, 0, len(items))
	for _, item := range items {
		if item == nil || (req.SinceID.Int64() > 0 && item.GetId() <= req.SinceID.Int64()) || (req.UntilID.Int64() > 0 && item.GetId() >= req.UntilID.Int64()) {
			continue
		}
		query := req.Query
		if query != nil {
			hostType := strings.ToLower(strings.TrimSpace(query.HostType))
			if hostType == "remote" || strings.TrimSpace(query.Host) != "" || strings.TrimSpace(query.URI) != "" {
				continue
			}
			if hasFrom && item.GetUpdatedAt() < fromMillis || hasTo && item.GetUpdatedAt() > toMillis {
				continue
			}
			if !emojiContains(item.GetName(), query.Name) || !emojiContains(item.GetUrl(), query.PublicURL) || !emojiContains(item.GetOriginalUrl(), query.OriginalURL) || !emojiContains(item.GetContentType(), query.Type) {
				continue
			}
			if !emojiContains(optionalEmojiText(item.Category), query.Category) || !emojiContains(optionalEmojiText(item.License), query.License) || !emojiListContains(item.GetAliases(), query.Aliases) {
				continue
			}
			if query.IsSensitive != nil && item.GetIsSensitive() != *query.IsSensitive || query.LocalOnly != nil && item.GetLocalOnly() != *query.LocalOnly {
				continue
			}
			if len(query.RoleIDs) > 0 && !emojiListsIntersect(item.GetRoleIdsThatCanBeUsedThisEmojiAsReaction(), query.RoleIDs) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered, true
}

func emojiTimeFilter(raw string) (int64, bool, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, true
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0, true, false
	}
	return parsed.UnixMilli(), true, true
}

func emojiContains(value, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	return query == "" || strings.Contains(strings.ToLower(value), query)
}

func emojiListContains(values []string, query string) bool {
	if strings.TrimSpace(query) == "" {
		return true
	}
	for _, value := range values {
		if emojiContains(value, query) {
			return true
		}
	}
	return false
}

func emojiListsIntersect(left, right []string) bool {
	values := make(map[string]struct{}, len(left))
	for _, value := range left {
		values[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, value := range right {
		if _, ok := values[strings.ToLower(strings.TrimSpace(value))]; ok {
			return true
		}
	}
	return false
}

func optionalEmojiText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sortV2Emojis(c *gin.Context, items []*adminpb.EmojiInfo, sortKeys []string) bool {
	allowed := map[string]bool{
		"id": true, "updatedAt": true, "name": true, "host": true, "uri": true,
		"publicUrl": true, "type": true, "aliases": true, "category": true, "license": true,
		"isSensitive": true, "localOnly": true, "roleIdsThatCanBeUsedThisEmojiAsReaction": true,
	}
	for _, raw := range sortKeys {
		if len(raw) < 2 || (raw[0] != '+' && raw[0] != '-') || !allowed[raw[1:]] {
			writeError(c, http.StatusBadRequest, "invalid emoji sort key", "bad_request")
			return false
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		for _, raw := range sortKeys {
			comparison := compareEmojiField(items[i], items[j], raw[1:])
			if comparison == 0 {
				continue
			}
			if raw[0] == '-' {
				return comparison > 0
			}
			return comparison < 0
		}
		return items[i].GetId() < items[j].GetId()
	})
	return true
}

func compareEmojiField(left, right *adminpb.EmojiInfo, field string) int {
	switch field {
	case "id":
		return compareInt64(left.GetId(), right.GetId())
	case "updatedAt":
		return compareInt64(left.GetUpdatedAt(), right.GetUpdatedAt())
	case "isSensitive":
		return compareBool(left.GetIsSensitive(), right.GetIsSensitive())
	case "localOnly":
		return compareBool(left.GetLocalOnly(), right.GetLocalOnly())
	case "name":
		return strings.Compare(strings.ToLower(left.GetName()), strings.ToLower(right.GetName()))
	case "publicUrl":
		return strings.Compare(strings.ToLower(left.GetUrl()), strings.ToLower(right.GetUrl()))
	case "type":
		return strings.Compare(strings.ToLower(left.GetContentType()), strings.ToLower(right.GetContentType()))
	case "aliases":
		return strings.Compare(strings.ToLower(strings.Join(left.GetAliases(), "\x00")), strings.ToLower(strings.Join(right.GetAliases(), "\x00")))
	case "category":
		return strings.Compare(strings.ToLower(optionalEmojiText(left.Category)), strings.ToLower(optionalEmojiText(right.Category)))
	case "license":
		return strings.Compare(strings.ToLower(optionalEmojiText(left.License)), strings.ToLower(optionalEmojiText(right.License)))
	case "roleIdsThatCanBeUsedThisEmojiAsReaction":
		return strings.Compare(strings.Join(left.GetRoleIdsThatCanBeUsedThisEmojiAsReaction(), "\x00"), strings.Join(right.GetRoleIdsThatCanBeUsedThisEmojiAsReaction(), "\x00"))
	default:
		return 0
	}
}

func compareInt64(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareBool(left, right bool) int {
	if left == right {
		return 0
	}
	if !left {
		return -1
	}
	return 1
}

func (h *Handler) listAdminEmojisWith(c *gin.Context, query string, limit, offset int32) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListEmojis(ctx, &adminpb.ListEmojisRequest{Actor: currentActor(c), Query: query, Limit: limit, Offset: offset})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := toEmojiViews(resp.GetItems())
	response.Success(c, gin.H{"items": items, "total": resp.GetTotal()})
}

func (h *Handler) createAdminEmoji(c *gin.Context) {
	h.createAdminEmojiWithResponse(c, false)
}

func (h *Handler) createAdminEmojiCompat(c *gin.Context) {
	h.createAdminEmojiWithResponse(c, true)
}

func (h *Handler) createAdminEmojiWithResponse(c *gin.Context, raw bool) {
	var req emojiMutationRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Name == nil {
		writeError(c, 400, "emoji name is required", "bad_request")
		return
	}
	emojiURL, contentType, ok := h.emojiURL(c, req.URL, req.FileID)
	if !ok {
		return
	}
	category, _, ok := nullableJSONText(req.Category)
	if !ok {
		writeError(c, 400, "invalid category", "bad_request")
		return
	}
	license, _, ok := nullableJSONText(req.License)
	if !ok {
		writeError(c, 400, "invalid license", "bad_request")
		return
	}
	aliases, _, ok := optionalStringList(req.Aliases)
	if !ok {
		writeError(c, 400, "invalid aliases", "bad_request")
		return
	}
	roles, _, ok := optionalStringList(req.RoleIDsThatCanBeUsedThisEmojiAsReaction)
	if !ok {
		writeError(c, 400, "invalid role ids", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateEmoji(ctx, &adminpb.CreateEmojiRequest{
		Actor: currentActor(c), Name: strings.TrimSpace(*req.Name), Url: emojiURL, OriginalUrl: emojiURL, ContentType: contentType,
		Category: category, Aliases: aliases, License: license, IsSensitive: emojiBoolValue(req.IsSensitive), LocalOnly: emojiBoolValue(req.LocalOnly),
		RoleIdsThatCanBeUsedThisEmojiAsReaction: roles,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	view := toEmojiView(resp.GetEmoji())
	if raw {
		c.JSON(http.StatusOK, view)
		return
	}
	response.Success(c, view)
}

func (h *Handler) updateAdminEmoji(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	h.updateAdminEmojiID(c, id, false)
}

func (h *Handler) updateAdminEmojiCompat(c *gin.Context) {
	var req emojiMutationRequest
	if !bindJSON(c, &req) {
		return
	}
	id := req.ID.Int64()
	if id <= 0 && req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		ctx, cancel := rpcContext(c)
		resp, err := h.clients.Admin.GetEmoji(ctx, &adminpb.GetEmojiRequest{Name: strings.TrimSpace(*req.Name)})
		cancel()
		if err != nil {
			writeRPCError(c, err)
			return
		}
		id = resp.GetEmoji().GetId()
	}
	if id <= 0 {
		writeError(c, 400, "invalid emoji id", "bad_request")
		return
	}
	h.updateAdminEmojiRequest(c, id, req, true)
}

func (h *Handler) updateAdminEmojiID(c *gin.Context, id int64, noContent bool) {
	var req emojiMutationRequest
	if !bindJSON(c, &req) {
		return
	}
	h.updateAdminEmojiRequest(c, id, req, noContent)
}

func (h *Handler) updateAdminEmojiRequest(c *gin.Context, id int64, req emojiMutationRequest, noContent bool) {
	pbReq := &adminpb.UpdateEmojiRequest{Actor: currentActor(c), Id: id, Name: req.Name, IsSensitive: req.IsSensitive, LocalOnly: req.LocalOnly}
	if req.URL != nil || strings.TrimSpace(req.FileID) != "" {
		emojiURL, contentType, ok := h.emojiURL(c, req.URL, req.FileID)
		if !ok {
			return
		}
		pbReq.Url, pbReq.OriginalUrl, pbReq.ContentType = &emojiURL, &emojiURL, &contentType
	}
	if value, present, ok := nullableJSONText(req.Category); !ok {
		writeError(c, 400, "invalid category", "bad_request")
		return
	} else if present {
		if value == nil {
			pbReq.ClearCategory = true
		} else {
			pbReq.Category = value
		}
	}
	if value, present, ok := nullableJSONText(req.License); !ok {
		writeError(c, 400, "invalid license", "bad_request")
		return
	} else if present {
		if value == nil {
			pbReq.ClearLicense = true
		} else {
			pbReq.License = value
		}
	}
	if values, present, ok := optionalStringList(req.Aliases); !ok {
		writeError(c, 400, "invalid aliases", "bad_request")
		return
	} else if present {
		pbReq.Aliases = &adminpb.OptionalStringList{Items: values}
	}
	if values, present, ok := optionalStringList(req.RoleIDsThatCanBeUsedThisEmojiAsReaction); !ok {
		writeError(c, 400, "invalid role ids", "bad_request")
		return
	} else if present {
		pbReq.RoleIdsThatCanBeUsedThisEmojiAsReaction = &adminpb.OptionalStringList{Items: values}
	}
	var oldItem *adminpb.EmojiInfo
	var allItems []*adminpb.EmojiInfo
	if pbReq.Url != nil {
		var ok bool
		allItems, ok = h.loadAllEmojis(c, nil)
		if !ok {
			return
		}
		for _, item := range allItems {
			if item != nil && item.GetId() == id {
				oldItem = item
				break
			}
		}
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateEmoji(ctx, pbReq)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if oldItem != nil && pbReq.Url != nil && oldItem.GetUrl() != *pbReq.Url {
		h.cleanupUnreferencedEmojiObject(c, oldItem.GetUrl(), allItems, map[int64]struct{}{id: {}})
	}
	if noContent {
		c.Status(http.StatusNoContent)
		return
	}
	response.Success(c, toEmojiView(resp.GetEmoji()))
}

func (h *Handler) addAdminEmojiAliasesBulk(c *gin.Context) {
	h.mutateAdminEmojiAliasesBulk(c, "add")
}

func (h *Handler) removeAdminEmojiAliasesBulk(c *gin.Context) {
	h.mutateAdminEmojiAliasesBulk(c, "remove")
}

func (h *Handler) setAdminEmojiAliasesBulk(c *gin.Context) {
	h.mutateAdminEmojiAliasesBulk(c, "set")
}

func (h *Handler) mutateAdminEmojiAliasesBulk(c *gin.Context, mode string) {
	var req emojiBulkAliasesRequest
	if !bindJSON(c, &req) {
		return
	}
	ids, ok := parseEmojiIDs(req.IDs)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid emoji ids", "bad_request")
		return
	}
	aliases, present, ok := optionalStringList(req.Aliases)
	if !ok || !present {
		writeError(c, http.StatusBadRequest, "invalid aliases", "bad_request")
		return
	}
	itemsByID, ok := h.localEmojisByID(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	for _, id := range ids {
		item := itemsByID[id]
		if item == nil {
			continue
		}
		values := aliases
		switch mode {
		case "add":
			values = append(append([]string(nil), item.GetAliases()...), aliases...)
		case "remove":
			removed := make(map[string]struct{}, len(aliases))
			for _, alias := range aliases {
				removed[strings.TrimSpace(alias)] = struct{}{}
			}
			values = make([]string, 0, len(item.GetAliases()))
			for _, alias := range item.GetAliases() {
				if _, remove := removed[alias]; !remove {
					values = append(values, alias)
				}
			}
		}
		_, err := h.clients.Admin.UpdateEmoji(ctx, &adminpb.UpdateEmojiRequest{
			Actor: currentActor(c), Id: id, Aliases: &adminpb.OptionalStringList{Items: values},
		})
		if err != nil {
			writeRPCError(c, err)
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) setAdminEmojiCategoryBulk(c *gin.Context) {
	var req emojiBulkCategoryRequest
	if !bindJSON(c, &req) {
		return
	}
	ids, ok := parseEmojiIDs(req.IDs)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid emoji ids", "bad_request")
		return
	}
	value, present, ok := nullableJSONText(req.Category)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid category", "bad_request")
		return
	}
	h.setAdminEmojiNullableTextBulk(c, ids, value, present, "category")
}

func (h *Handler) setAdminEmojiLicenseBulk(c *gin.Context) {
	var req emojiBulkLicenseRequest
	if !bindJSON(c, &req) {
		return
	}
	ids, ok := parseEmojiIDs(req.IDs)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid emoji ids", "bad_request")
		return
	}
	value, present, ok := nullableJSONText(req.License)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid license", "bad_request")
		return
	}
	h.setAdminEmojiNullableTextBulk(c, ids, value, present, "license")
}

func (h *Handler) setAdminEmojiNullableTextBulk(c *gin.Context, ids []int64, value *string, present bool, field string) {
	itemsByID, ok := h.localEmojisByID(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	for _, id := range ids {
		if itemsByID[id] == nil {
			continue
		}
		req := &adminpb.UpdateEmojiRequest{Actor: currentActor(c), Id: id}
		if field == "category" {
			if !present || value == nil {
				req.ClearCategory = true
			} else {
				req.Category = value
			}
		} else if !present || value == nil {
			req.ClearLicense = true
		} else {
			req.License = value
		}
		if _, err := h.clients.Admin.UpdateEmoji(ctx, req); err != nil {
			writeRPCError(c, err)
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) deleteAdminEmojisBulk(c *gin.Context) {
	var req struct {
		IDs json.RawMessage `json:"ids"`
	}
	if !bindJSON(c, &req) {
		return
	}
	ids, ok := parseEmojiIDs(req.IDs)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid emoji ids", "bad_request")
		return
	}
	itemsByID, ok := h.localEmojisByID(c)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	deletedIDs := make(map[int64]struct{}, len(ids))
	deletedItems := make([]*adminpb.EmojiInfo, 0, len(ids))
	for _, id := range ids {
		item := itemsByID[id]
		if item == nil {
			continue
		}
		if _, err := h.clients.Admin.DeleteEmoji(ctx, &adminpb.EmojiIDRequest{Actor: currentActor(c), Id: id}); err != nil {
			writeRPCError(c, err)
			return
		}
		deletedIDs[id] = struct{}{}
		deletedItems = append(deletedItems, item)
	}
	allItems := make([]*adminpb.EmojiInfo, 0, len(itemsByID))
	for _, candidate := range itemsByID {
		allItems = append(allItems, candidate)
	}
	for _, item := range deletedItems {
		h.cleanupUnreferencedEmojiObject(c, item.GetUrl(), allItems, deletedIDs)
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) localEmojisByID(c *gin.Context) (map[int64]*adminpb.EmojiInfo, bool) {
	items, ok := h.loadAllEmojis(c, nil)
	if !ok {
		return nil, false
	}
	byID := make(map[int64]*adminpb.EmojiInfo, len(items))
	for _, item := range items {
		if item != nil {
			byID[item.GetId()] = item
		}
	}
	return byID, true
}

func parseEmojiIDs(raw json.RawMessage) ([]int64, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false
	}
	var values []jsonInt64
	if err := json.Unmarshal(trimmed, &values); err != nil || values == nil {
		return nil, false
	}
	ids := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		id := value.Int64()
		if id <= 0 {
			return nil, false
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, true
}

func (h *Handler) deleteAdminEmoji(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	h.deleteAdminEmojiID(c, id, false)
}

func (h *Handler) deleteAdminEmojiCompat(c *gin.Context) {
	var req struct {
		ID jsonInt64 `json:"id"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.ID.Int64() <= 0 {
		writeError(c, 400, "invalid emoji id", "bad_request")
		return
	}
	h.deleteAdminEmojiID(c, req.ID.Int64(), true)
}

func (h *Handler) deleteAdminEmojiID(c *gin.Context, id int64, noContent bool) {
	items, ok := h.loadAllEmojis(c, nil)
	if !ok {
		return
	}
	var item *adminpb.EmojiInfo
	for _, candidate := range items {
		if candidate != nil && candidate.GetId() == id {
			item = candidate
			break
		}
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteEmoji(ctx, &adminpb.EmojiIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if item != nil {
		h.cleanupUnreferencedEmojiObject(c, item.GetUrl(), items, map[int64]struct{}{id: {}})
	}
	if noContent {
		c.Status(http.StatusNoContent)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) cleanupUnreferencedEmojiObject(c *gin.Context, rawURL string, items []*adminpb.EmojiInfo, excludedIDs map[int64]struct{}) {
	objectKey, ok := h.localEmojiObjectKey(c, rawURL)
	if !ok {
		return
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, excluded := excludedIDs[item.GetId()]; excluded {
			continue
		}
		candidateKey, local := h.localEmojiObjectKey(c, item.GetUrl())
		if local && candidateKey == objectKey {
			return
		}
	}
	cleanupCtx, cancel := context.WithTimeout(c.Request.Context(), imageTransferTimeout)
	_ = h.cleanupUploadedObject(cleanupCtx, objectKey)
	cancel()
}

func (h *Handler) localEmojiObjectKey(c *gin.Context, rawURL string) (string, bool) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	publicBase, err := url.ParseRequestURI(h.publicURL(c, "/"))
	if err != nil || !strings.EqualFold(parsed.Scheme, publicBase.Scheme) || !strings.EqualFold(parsed.Host, publicBase.Host) {
		return "", false
	}
	prefix := strings.TrimRight(publicBase.Path, "/") + "/uploads/emojis/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(parsed.Path, prefix)
	objectKey, _, ok := publicImageObject("emojis", name)
	return objectKey, ok
}

func (h *Handler) emojiURL(c *gin.Context, rawURL *string, fileID string) (string, string, bool) {
	value := ""
	if rawURL != nil {
		value = strings.TrimSpace(*rawURL)
	}
	if value == "" {
		value = strings.TrimSpace(fileID)
	}
	if value == "" {
		writeError(c, 400, "emoji image is required", "bad_request")
		return "", "", false
	}
	if !strings.Contains(value, "://") {
		if filepath.Base(value) != value || !allowedAvatarExt(strings.ToLower(filepath.Ext(value))) {
			writeError(c, 400, "invalid emoji fileId", "bad_request")
			return "", "", false
		}
		value = h.publicURL(c, "/uploads/emojis/"+value)
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		writeError(c, 400, "invalid emoji image URL", "bad_request")
		return "", "", false
	}
	return value, imageContentType(strings.ToLower(filepath.Ext(parsed.Path))), true
}

func nullableJSONText(raw json.RawMessage) (*string, bool, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false, true
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, true, true
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, true, false
	}
	return &value, true, true
}

func optionalStringList(raw json.RawMessage) ([]string, bool, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false, true
	}
	var values []string
	if err := json.Unmarshal(trimmed, &values); err != nil || values == nil {
		return nil, true, false
	}
	return values, true, true
}

func emojiBoolValue(value *bool) bool { return value != nil && *value }

func toEmojiViews(items []*adminpb.EmojiInfo) []emojiView {
	out := make([]emojiView, 0, len(items))
	for _, item := range items {
		out = append(out, toEmojiView(item))
	}
	return out
}

func toEmojiV2Views(items []*adminpb.EmojiInfo, roles map[string]emojiReactionRoleView) []emojiV2View {
	out := make([]emojiV2View, 0, len(items))
	for _, item := range items {
		view := toEmojiView(item)
		reactionRoles := make([]emojiReactionRoleView, 0, len(view.RoleIDsThatCanBeUsedThisEmojiAsReaction))
		for _, roleID := range view.RoleIDsThatCanBeUsedThisEmojiAsReaction {
			if role, ok := roles[roleID]; ok {
				reactionRoles = append(reactionRoles, role)
			}
		}
		out = append(out, emojiV2View{
			ID: view.ID, UpdatedAt: view.UpdatedAt, Name: view.Name, Host: view.Host,
			PublicURL: view.PublicURL, OriginalURL: view.OriginalURL, URI: view.URI, Type: view.Type,
			Aliases: view.Aliases, Category: view.Category, License: view.License,
			IsSensitive: view.IsSensitive, LocalOnly: view.LocalOnly,
			RoleIDsThatCanBeUsedThisEmojiAsReaction: reactionRoles,
		})
	}
	return out
}

func toEmojiView(item *adminpb.EmojiInfo) emojiView {
	if item == nil {
		return emojiView{Aliases: []string{}, RoleIDsThatCanBeUsedThisEmojiAsReaction: []string{}}
	}
	var updatedAt *string
	if item.GetUpdatedAt() > 0 {
		value := time.UnixMilli(item.GetUpdatedAt()).UTC().Format(time.RFC3339Nano)
		updatedAt = &value
	}
	contentType := strings.TrimSpace(item.GetContentType())
	var contentTypePtr *string
	if contentType != "" {
		contentTypePtr = &contentType
	}
	return emojiView{
		ID: strconv.FormatInt(item.GetId(), 10), UpdatedAt: updatedAt, Name: item.GetName(), URL: item.GetUrl(), PublicURL: item.GetUrl(), OriginalURL: item.GetOriginalUrl(),
		Type: contentTypePtr, Aliases: append([]string{}, item.GetAliases()...), Category: item.Category, License: item.License,
		IsSensitive: item.GetIsSensitive(), LocalOnly: item.GetLocalOnly(), RoleIDsThatCanBeUsedThisEmojiAsReaction: append([]string{}, item.GetRoleIdsThatCanBeUsedThisEmojiAsReaction()...),
	}
}
