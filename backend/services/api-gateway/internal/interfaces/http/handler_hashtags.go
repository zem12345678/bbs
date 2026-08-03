package http

import (
	stdhttp "net/http"
	"sort"
	"strings"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const (
	defaultHashtagLimit = int32(10)
	maxHashtagLimit     = int32(50)
	maxHashtagUserScan  = int32(100)
)

type hashtagRequest struct {
	Tag    string `json:"tag"`
	Query  string `json:"query"`
	Q      string `json:"q"`
	Limit  int32  `json:"limit"`
	Offset int32  `json:"offset"`
	Sort   string `json:"sort"`
}

type publicHashtag struct {
	Tag                       string `json:"tag"`
	Count                     int64  `json:"count"`
	MentionedUsersCount       int64  `json:"mentionedUsersCount"`
	MentionedLocalUsersCount  int64  `json:"mentionedLocalUsersCount"`
	MentionedRemoteUsersCount int64  `json:"mentionedRemoteUsersCount"`
	AttachedUsersCount        int64  `json:"attachedUsersCount"`
	AttachedLocalUsersCount   int64  `json:"attachedLocalUsersCount"`
	AttachedRemoteUsersCount  int64  `json:"attachedRemoteUsersCount"`
}

type publicHashtagListResponse struct {
	Items []publicHashtag `json:"items"`
	Total int64           `json:"total"`
}

type publicHashtagShowResponse struct {
	Hashtag publicHashtag `json:"hashtag"`
}

func (h *Handler) listHashtags(c *gin.Context) {
	req, ok := bindHashtagRequest(c, defaultHashtagLimit)
	if !ok {
		return
	}
	items, total, ok := h.fetchHashtags(c, req.Query, req.Limit, req.Offset, req.Sort)
	if !ok {
		return
	}
	response.Success(c, publicHashtagListResponse{Items: items, Total: total})
}

func (h *Handler) searchHashtags(c *gin.Context) {
	req, ok := bindHashtagRequest(c, defaultHashtagLimit)
	if !ok {
		return
	}
	items, total, ok := h.fetchHashtags(c, hashtagQuery(req), req.Limit, req.Offset, req.Sort)
	if !ok {
		return
	}
	response.Success(c, publicHashtagListResponse{Items: items, Total: total})
}

func (h *Handler) trendingHashtags(c *gin.Context) {
	req, ok := bindHashtagRequest(c, defaultHashtagLimit)
	if !ok {
		return
	}
	items, total, ok := h.fetchHashtags(c, "", req.Limit, req.Offset, "-mentionedUsers")
	if !ok {
		return
	}
	response.Success(c, publicHashtagListResponse{Items: items, Total: total})
}

func (h *Handler) showHashtag(c *gin.Context) {
	req, ok := bindHashtagRequest(c, 1)
	if !ok {
		return
	}
	tag := normalizeHashtag(req.Tag)
	if tag == "" {
		writeError(c, stdhttp.StatusBadRequest, "tag is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListTags(ctx, &contentpb.ListTagsRequest{
		Status: contentStatusPublished,
		Query:  tag,
		Limit:  maxHashtagLimit,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	for _, item := range resp.GetItems() {
		if strings.EqualFold(normalizeHashtag(item.GetName()), tag) {
			response.Success(c, publicHashtagShowResponse{Hashtag: toPublicHashtag(item)})
			return
		}
	}
	writeError(c, stdhttp.StatusNotFound, "hashtag not found", "not_found")
}

func (h *Handler) listHashtagUsers(c *gin.Context) {
	req, ok := bindHashtagRequest(c, defaultHashtagLimit)
	if !ok {
		return
	}
	tag := normalizeHashtag(req.Tag)
	if tag == "" {
		writeError(c, stdhttp.StatusBadRequest, "tag is required", "bad_request")
		return
	}
	limit := normalizeHashtagLimit(req.Limit)
	ctx, cancel := rpcContext(c)
	defer cancel()
	articles, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{
		Status: contentStatusPublished,
		Tag:    tag,
		Limit:  maxHashtagUserScan,
		Offset: 0,
		Sort:   "latest",
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	authorIDs := uniqueArticleAuthorIDs(articles.GetItems(), int(limit))
	if len(authorIDs) == 0 {
		response.Success(c, publicUserListResponse{Items: []*publicUserView{}, Total: 0})
		return
	}
	users, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
		Ids:      authorIDs,
		Status:   userStatusActive,
		Page:     1,
		PageSize: int32(len(authorIDs)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	orderedUsers := orderActiveUsersByID(authorIDs, users.GetItems())
	h.sanitizeUserProfileThemes(ctx, orderedUsers)
	response.Success(c, toPublicUserListResponse(&userpb.UserListResponse{Items: orderedUsers, Total: int64(len(orderedUsers))}))
}

func bindHashtagRequest(c *gin.Context, fallbackLimit int32) (hashtagRequest, bool) {
	req := hashtagRequest{Limit: fallbackLimit}
	if c.Request.Method == stdhttp.MethodPost {
		if !bindJSON(c, &req) {
			return hashtagRequest{}, false
		}
		if req.Limit == 0 {
			req.Limit = fallbackLimit
		}
		return normalizeHashtagRequest(req), true
	}
	req.Tag = c.Query("tag")
	req.Query = c.Query("query")
	req.Q = c.Query("q")
	req.Limit = queryInt32(c, "limit", fallbackLimit)
	req.Offset = queryInt32(c, "offset", 0)
	req.Sort = c.Query("sort")
	return normalizeHashtagRequest(req), true
}

func normalizeHashtagRequest(req hashtagRequest) hashtagRequest {
	req.Tag = normalizeHashtag(req.Tag)
	req.Query = strings.TrimSpace(req.Query)
	req.Q = strings.TrimSpace(req.Q)
	req.Limit = normalizeHashtagLimit(req.Limit)
	if req.Offset < 0 {
		req.Offset = 0
	}
	req.Sort = strings.TrimSpace(req.Sort)
	return req
}

func (h *Handler) fetchHashtags(c *gin.Context, query string, limit int32, offset int32, sortValue string) ([]publicHashtag, int64, bool) {
	limit = normalizeHashtagLimit(limit)
	if offset < 0 {
		offset = 0
	}
	fetchLimit := limit + offset
	if fetchLimit <= 0 || fetchLimit > maxHashtagLimit {
		fetchLimit = maxHashtagLimit
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListTags(ctx, &contentpb.ListTagsRequest{
		Status: contentStatusPublished,
		Query:  strings.TrimSpace(query),
		Limit:  fetchLimit,
	})
	if err != nil {
		writeRPCError(c, err)
		return nil, 0, false
	}
	items := make([]publicHashtag, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		if tag := toPublicHashtag(item); tag.Tag != "" {
			items = append(items, tag)
		}
	}
	sortPublicHashtags(items, sortValue)
	total := int64(len(items))
	start := int(offset)
	if start >= len(items) {
		return []publicHashtag{}, total, true
	}
	end := start + int(limit)
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, true
}

func toPublicHashtag(item *contentpb.TagInfo) publicHashtag {
	if item == nil {
		return publicHashtag{}
	}
	count := item.GetCount()
	return publicHashtag{
		Tag:                       normalizeHashtag(item.GetName()),
		Count:                     count,
		MentionedUsersCount:       count,
		MentionedLocalUsersCount:  count,
		MentionedRemoteUsersCount: 0,
		AttachedUsersCount:        0,
		AttachedLocalUsersCount:   0,
		AttachedRemoteUsersCount:  0,
	}
}

func hashtagQuery(req hashtagRequest) string {
	if req.Query != "" {
		return req.Query
	}
	return req.Q
}

func normalizeHashtag(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "#")
}

func normalizeHashtagLimit(value int32) int32 {
	if value <= 0 {
		return defaultHashtagLimit
	}
	if value > maxHashtagLimit {
		return maxHashtagLimit
	}
	return value
}

func sortPublicHashtags(items []publicHashtag, sortValue string) {
	descending := true
	if strings.HasPrefix(sortValue, "+") {
		descending = false
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].MentionedUsersCount == items[j].MentionedUsersCount {
			return items[i].Tag < items[j].Tag
		}
		if descending {
			return items[i].MentionedUsersCount > items[j].MentionedUsersCount
		}
		return items[i].MentionedUsersCount < items[j].MentionedUsersCount
	})
}

func uniqueArticleAuthorIDs(articles []*contentpb.ArticleInfo, limit int) []int64 {
	ids := make([]int64, 0, limit)
	seen := make(map[int64]struct{}, limit)
	for _, article := range articles {
		if article == nil || article.GetAuthorId() <= 0 {
			continue
		}
		id := article.GetAuthorId()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

func orderActiveUsersByID(ids []int64, users []*userpb.UserInfo) []*userpb.UserInfo {
	byID := make(map[int64]*userpb.UserInfo, len(users))
	for _, user := range users {
		if user != nil && user.GetStatus() == userStatusActive {
			byID[user.GetId()] = user
		}
	}
	ordered := make([]*userpb.UserInfo, 0, len(ids))
	for _, id := range ids {
		if user, ok := byID[id]; ok {
			ordered = append(ordered, user)
		}
	}
	return ordered
}
