package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"sort"
	"strings"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/searchpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type notesSearchCompatRequest struct {
	Query     string          `json:"query"`
	SinceID   json.RawMessage `json:"sinceId"`
	UntilID   json.RawMessage `json:"untilId"`
	Limit     *int32          `json:"limit"`
	Offset    *int32          `json:"offset"`
	Host      json.RawMessage `json:"host"`
	FileType  json.RawMessage `json:"filetype"`
	UserID    json.RawMessage `json:"userId"`
	ChannelID json.RawMessage `json:"channelId"`
	Order     string          `json:"order"`
}

type notesSearchCandidate struct {
	kind      string
	id        int64
	authorID  int64
	channelID int64
	createdAt int64
	score     float64
}

func (h *Handler) registerNotesSearchCompatRoutes(router *gin.Engine) {
	for _, prefix := range []string{"", "/api", "/api/v1"} {
		router.POST(prefix+"/notes/search", h.optionalAuth(), h.searchNotesCompat)
	}
}

func (h *Handler) searchNotesCompat(c *gin.Context) {
	var request notesSearchCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) {
		writeSearchByTagInvalidParam(c)
		return
	}
	query := strings.TrimSpace(request.Query)
	if query == "" || len([]rune(query)) > 256 {
		writeSearchByTagInvalidParam(c)
		return
	}
	limit := int32(10)
	if request.Limit != nil {
		limit = *request.Limit
	}
	offset := int32(0)
	if request.Offset != nil {
		offset = *request.Offset
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		writeSearchByTagInvalidParam(c)
		return
	}
	if request.Order != "" && request.Order != "asc" && request.Order != "desc" {
		writeSearchByTagInvalidParam(c)
		return
	}
	if !notesSearchHostIsLocal(request.Host) || !notesSearchFileTypeIsSupported(request.FileType) {
		writeSearchByTagInvalidParam(c)
		return
	}
	sinceID, sinceSet, sinceValid := decodeCompatOptionalPositiveID(request.SinceID)
	untilID, untilSet, untilValid := decodeCompatOptionalPositiveID(request.UntilID)
	userID, userSet, userValid := decodeCompatOptionalPositiveID(request.UserID)
	channelID, channelSet, channelValid := decodeCompatOptionalPositiveID(request.ChannelID)
	if !sinceValid || !untilValid || !userValid || !channelValid {
		writeSearchByTagInvalidParam(c)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if h.clients == nil || h.clients.Search == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "search service unavailable"))
		return
	}
	if !h.allowSearchRateLimit(c, h.searchRateLimits.Content, searchRateLimitContent) {
		return
	}
	need := int(offset + limit)
	candidates, err := h.collectNotesSearchCandidates(ctx, query, need)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	order := request.Order
	if order == "" {
		order = "desc"
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if order == "asc" {
			if candidates[i].createdAt == candidates[j].createdAt {
				return candidates[i].id < candidates[j].id
			}
			return candidates[i].createdAt < candidates[j].createdAt
		}
		if candidates[i].createdAt == candidates[j].createdAt {
			return candidates[i].id > candidates[j].id
		}
		return candidates[i].createdAt > candidates[j].createdAt
	})
	items := make([]misskeyClipNote, 0, limit)
	for _, candidate := range candidates {
		if sinceSet && candidate.id <= sinceID || untilSet && candidate.id >= untilID || userSet && candidate.authorID != userID {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		var note misskeyClipNote
		var ok bool
		switch candidate.kind {
		case "article":
			response, getErr := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: candidate.id}, TrackView: false})
			if getErr != nil || response.GetArticle() == nil || response.GetArticle().GetStatus() != contentStatusPublished {
				continue
			}
			if channelSet {
				continue
			}
			note, ok = h.misskeyNoteFromArticle(c, ctx, response.GetArticle())
		case "topic":
			response, getErr := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: candidate.id}, TrackView: false, ViewerUserId: currentUserID(c)})
			if getErr != nil || response.GetTopic() == nil || response.GetTopic().GetStatus() != contentStatusPublished {
				continue
			}
			if channelSet && response.GetTopic().GetChannelId() != channelID {
				continue
			}
			note, ok = h.misskeyNoteFromTopic(c, ctx, response.GetTopic())
		}
		if !ok {
			if c.IsAborted() {
				return
			}
			continue
		}
		items = append(items, note)
		if int32(len(items)) >= limit {
			break
		}
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) collectNotesSearchCandidates(ctx context.Context, keyword string, need int) ([]notesSearchCandidate, error) {
	if need < 1 {
		return []notesSearchCandidate{}, nil
	}
	if need > 10100 {
		need = 10100
	}
	const pageSize int32 = 100
	result := make([]notesSearchCandidate, 0, need*2)
	for page := int32(1); ; page++ {
		response, err := h.clients.Search.SearchArticles(ctx, &searchpb.SearchArticlesRequest{Keyword: keyword, Page: page, PageSize: pageSize})
		if err != nil {
			return nil, err
		}
		total := response.GetTotal()
		if err := h.filterPublicArticleSearchResults(ctx, response); err != nil {
			return nil, err
		}
		for _, hit := range response.GetItems() {
			article := hit.GetArticle()
			if article != nil && article.GetId() > 0 {
				result = append(result, notesSearchCandidate{kind: "article", id: article.GetId(), authorID: article.GetAuthorId(), createdAt: article.GetCreatedAt(), score: hit.GetScore()})
			}
		}
		if int32(len(response.GetItems())) < pageSize || int64(page)*int64(pageSize) >= total {
			break
		}
	}
	for page := int32(1); ; page++ {
		response, err := h.clients.Search.SearchTopics(ctx, &searchpb.SearchTopicsRequest{Keyword: keyword, Page: page, PageSize: pageSize})
		if err != nil {
			return nil, err
		}
		total := response.GetTotal()
		if err := h.filterPublicTopicSearchResults(ctx, response); err != nil {
			return nil, err
		}
		for _, hit := range response.GetItems() {
			topic := hit.GetTopic()
			if topic != nil && topic.GetId() > 0 {
				result = append(result, notesSearchCandidate{kind: "topic", id: topic.GetId(), authorID: topic.GetAuthorId(), createdAt: topic.GetCreatedAt(), score: hit.GetScore()})
			}
		}
		if int32(len(response.GetItems())) < pageSize || int64(page)*int64(pageSize) >= total {
			break
		}
	}
	return result, nil
}

func decodeCompatOptionalPositiveID(raw json.RawMessage) (value int64, set bool, valid bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return 0, false, true
	}
	var id jsonInt64
	if err := id.UnmarshalJSON(trimmed); err != nil || id.Int64() <= 0 {
		return 0, true, false
	}
	return id.Int64(), true, true
}

func notesSearchHostIsLocal(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return true
	}
	var value string
	if json.Unmarshal(trimmed, &value) != nil {
		return false
	}
	return strings.TrimSpace(value) == "."
}

func notesSearchFileTypeIsSupported(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return true
	}
	return false
}
