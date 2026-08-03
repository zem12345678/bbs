package http

import (
	"context"
	"net/http"
	"strings"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/searchpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) searchArticles(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		writeError(c, http.StatusBadRequest, "q is required", "bad_request")
		return
	}
	page, pageSize, ok := searchPagination(c)
	if !ok {
		return
	}
	if !h.allowSearchRateLimit(c, h.searchRateLimits.Content, searchRateLimitContent) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Search.SearchArticles(ctx, &searchpb.SearchArticlesRequest{Keyword: keyword, Page: page, PageSize: pageSize})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if err := h.filterPublicArticleSearchResults(ctx, resp); err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) searchTopics(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		writeError(c, http.StatusBadRequest, "q is required", "bad_request")
		return
	}
	page, pageSize, ok := searchPagination(c)
	if !ok {
		return
	}
	if !h.allowSearchRateLimit(c, h.searchRateLimits.Content, searchRateLimitContent) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Search.SearchTopics(ctx, &searchpb.SearchTopicsRequest{Keyword: keyword, Page: page, PageSize: pageSize})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if err := h.filterPublicTopicSearchResults(ctx, resp); err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

// Search documents are maintained asynchronously. Verify every public hit
// against content-service so a delayed or failed de-index event cannot expose
// hidden or archived content through a stale Elasticsearch document.
func (h *Handler) filterPublicArticleSearchResults(ctx context.Context, resp *searchpb.SearchArticlesResponse) error {
	if resp == nil {
		return nil
	}
	if h.clients == nil || h.clients.Content == nil {
		return status.Error(codes.Unavailable, "content service unavailable")
	}
	items := make([]*searchpb.ArticleHit, 0, len(resp.GetItems()))
	for _, hit := range resp.GetItems() {
		id := hit.GetArticle().GetId()
		if id <= 0 {
			continue
		}
		current, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{
			Key:       &contentpb.GetArticleRequest_Id{Id: id},
			TrackView: false,
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			return err
		}
		article := current.GetArticle()
		if article == nil || article.GetId() != id || article.GetStatus() != contentStatusPublished {
			continue
		}
		items = append(items, hit)
	}
	resp.Items = items
	resp.Total = int64(len(items))
	return nil
}

func (h *Handler) filterPublicTopicSearchResults(ctx context.Context, resp *searchpb.SearchTopicsResponse) error {
	if resp == nil {
		return nil
	}
	if h.clients == nil || h.clients.Content == nil {
		return status.Error(codes.Unavailable, "content service unavailable")
	}
	items := make([]*searchpb.TopicHit, 0, len(resp.GetItems()))
	for _, hit := range resp.GetItems() {
		id := hit.GetTopic().GetId()
		if id <= 0 {
			continue
		}
		current, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{
			Key:       &contentpb.GetTopicRequest_Id{Id: id},
			TrackView: false,
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				continue
			}
			return err
		}
		topic := current.GetTopic()
		if topic == nil || topic.GetId() != id || topic.GetStatus() != contentStatusPublished {
			continue
		}
		items = append(items, hit)
	}
	resp.Items = items
	resp.Total = int64(len(items))
	return nil
}

func (h *Handler) searchUsers(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		writeError(c, http.StatusBadRequest, "q is required", "bad_request")
		return
	}
	page, pageSize, ok := searchPagination(c)
	if !ok {
		return
	}
	if !h.allowSearchRateLimit(c, h.searchRateLimits.User, searchRateLimitUser) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	searchResp, err := h.clients.Search.SearchUsers(ctx, &searchpb.SearchUsersRequest{
		Keyword:  keyword,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	resp, err := h.resolvePublicUserSearchResults(ctx, searchResp)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileThemes(ctx, resp.GetItems())
	response.Success(c, toPublicUserListResponse(resp))
}

// User search documents are maintained asynchronously. Resolve the ranked ES
// IDs through user-service before returning them so a delayed delete or status
// update cannot expose an inactive account.
func (h *Handler) resolvePublicUserSearchResults(ctx context.Context, resp *searchpb.SearchUsersResponse) (*userpb.UserListResponse, error) {
	result := &userpb.UserListResponse{}
	if resp == nil {
		return result, nil
	}
	ids := make([]int64, 0, len(resp.GetItems()))
	requested := make(map[int64]bool, len(resp.GetItems()))
	for _, hit := range resp.GetItems() {
		id := hit.GetUser().GetId()
		if id <= 0 || requested[id] {
			continue
		}
		requested[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return result, nil
	}
	if h.clients == nil || h.clients.User == nil {
		return nil, status.Error(codes.Unavailable, "user service unavailable")
	}
	users, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
		Ids:      ids,
		Status:   userStatusActive,
		Page:     1,
		PageSize: int32(len(ids)),
	})
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*userpb.UserInfo, len(users.GetItems()))
	for _, user := range users.GetItems() {
		if user == nil || user.GetStatus() != userStatusActive || !publicAccountStateActive(user.GetAccountState()) || !requested[user.GetId()] {
			continue
		}
		byID[user.GetId()] = user
	}
	result.Items = make([]*userpb.UserInfo, 0, len(ids))
	for _, id := range ids {
		if user := byID[id]; user != nil {
			result.Items = append(result.Items, user)
		}
	}
	result.Total = int64(len(result.Items))
	return result, nil
}

func publicAccountStateActive(value string) bool {
	state := strings.ToLower(strings.TrimSpace(value))
	return state == "" || state == "active"
}
