package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/searchpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	searchNotesByTagDefaultLimit    int32 = 10
	searchNotesByTagMaxLimit        int32 = 100
	searchNotesByTagMaxGroups             = 8
	searchNotesByTagMaxTagsPerGroup       = 8
	searchNotesByTagMaxTagRunes           = 128
)

type searchNullableBool struct {
	Present bool
	Null    bool
	Value   bool
}

func (v *searchNullableBool) UnmarshalJSON(data []byte) error {
	v.Present = true
	if strings.TrimSpace(string(data)) == "null" {
		v.Null = true
		return nil
	}
	return json.Unmarshal(data, &v.Value)
}

type searchOptionalString struct {
	Present bool
	Null    bool
	Value   string
}

func (v *searchOptionalString) UnmarshalJSON(data []byte) error {
	v.Present = true
	if strings.TrimSpace(string(data)) == "null" {
		v.Null = true
		return nil
	}
	return json.Unmarshal(data, &v.Value)
}

type searchOptionalInt32 struct {
	Present bool
	Value   int32
}

func (v *searchOptionalInt32) UnmarshalJSON(data []byte) error {
	v.Present = true
	if strings.TrimSpace(string(data)) == "null" {
		return fmt.Errorf("integer must not be null")
	}
	return json.Unmarshal(data, &v.Value)
}

type searchOptionalPositiveID struct {
	Present bool
	Value   int64
}

func (v *searchOptionalPositiveID) UnmarshalJSON(data []byte) error {
	v.Present = true
	if strings.TrimSpace(string(data)) == "null" {
		return fmt.Errorf("id must not be null")
	}
	var value jsonInt64
	if err := value.UnmarshalJSON(data); err != nil {
		return err
	}
	if value.Int64() <= 0 {
		return fmt.Errorf("id must be positive")
	}
	v.Value = value.Int64()
	return nil
}

type searchTagQuery struct {
	Present bool
	Null    bool
	Groups  [][]string
}

func (v *searchTagQuery) UnmarshalJSON(data []byte) error {
	v.Present = true
	if strings.TrimSpace(string(data)) == "null" {
		v.Null = true
		return nil
	}
	return json.Unmarshal(data, &v.Groups)
}

type searchNotesByTagRequest struct {
	Reply     searchNullableBool       `json:"reply"`
	Renote    searchNullableBool       `json:"renote"`
	WithFiles searchNullableBool       `json:"withFiles"`
	Poll      searchNullableBool       `json:"poll"`
	SinceID   searchOptionalPositiveID `json:"sinceId"`
	UntilID   searchOptionalPositiveID `json:"untilId"`
	Limit     searchOptionalInt32      `json:"limit"`
	Tag       searchOptionalString     `json:"tag"`
	Scope     searchOptionalString     `json:"scope"`
	Query     searchTagQuery           `json:"query"`
}

type searchArticleProjection struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	ContentExcerpt string   `json:"content_excerpt"`
	TagIDs         []string `json:"tag_ids"`
	TagNames       []string `json:"tag_names"`
	AuthorID       string   `json:"author_id"`
	AuthorNickname string   `json:"author_nickname"`
	Status         int32    `json:"status"`
	ViewCount      string   `json:"view_count"`
	CommentCount   string   `json:"comment_count"`
	LikeCount      string   `json:"like_count"`
	FavoriteCount  string   `json:"favorite_count"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type searchTopicProjection struct {
	ID             string   `json:"id"`
	Slug           string   `json:"slug"`
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	ContentExcerpt string   `json:"content_excerpt"`
	TagNames       []string `json:"tag_names"`
	AuthorID       string   `json:"author_id"`
	Status         int32    `json:"status"`
	CommentCount   string   `json:"comment_count"`
	LikeCount      string   `json:"like_count"`
	FavoriteCount  string   `json:"favorite_count"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	ViewCount      string   `json:"view_count"`
	CategoryID     string   `json:"category_id,omitempty"`
}

type searchNoteProjection struct {
	Kind    string                   `json:"kind"`
	Article *searchArticleProjection `json:"article,omitempty"`
	Topic   *searchTopicProjection   `json:"topic,omitempty"`
}

func (h *Handler) searchNotesByTag(c *gin.Context) {
	var req searchNotesByTagRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeSearchByTagInvalidParam(c)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeSearchByTagInvalidParam(c)
		return
	}

	rpcReq, ok := validateSearchNotesByTagRequest(&req)
	if !ok {
		writeSearchByTagInvalidParam(c)
		return
	}
	if !h.allowSearchRateLimit(c, h.searchRateLimits.Content, searchRateLimitContent) {
		return
	}
	if h.clients == nil || h.clients.Search == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "search service unavailable"))
		return
	}

	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Search.SearchNotesByTag(ctx, rpcReq)
	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			writeSearchByTagInvalidParam(c)
			return
		}
		writeRPCError(c, err)
		return
	}
	items, err := h.resolveSearchNotesByTag(ctx, resp)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, items)
}

func validateSearchNotesByTagRequest(req *searchNotesByTagRequest) (*searchpb.SearchNotesByTagRequest, bool) {
	if req == nil || (req.Tag.Present && req.Tag.Null) || (req.Query.Present && req.Query.Null) {
		return nil, false
	}
	if req.WithFiles.Present && req.WithFiles.Null {
		return nil, false
	}
	for _, filter := range []searchNullableBool{req.Reply, req.Renote, req.Poll} {
		if filter.Present && !filter.Null {
			return nil, false
		}
	}
	if req.WithFiles.Present && req.WithFiles.Value {
		return nil, false
	}

	limit := searchNotesByTagDefaultLimit
	if req.Limit.Present {
		limit = req.Limit.Value
	}
	if limit < 1 || limit > searchNotesByTagMaxLimit {
		return nil, false
	}

	tag := ""
	if req.Tag.Present {
		tag = strings.ToLower(strings.TrimSpace(req.Tag.Value))
		if tag == "" || len([]rune(tag)) > searchNotesByTagMaxTagRunes {
			return nil, false
		}
	}
	query := make([]*searchpb.TagQueryGroup, 0, len(req.Query.Groups))
	if req.Query.Present {
		if len(req.Query.Groups) < 1 || len(req.Query.Groups) > searchNotesByTagMaxGroups {
			return nil, false
		}
		for _, group := range req.Query.Groups {
			if len(group) < 1 || len(group) > searchNotesByTagMaxTagsPerGroup {
				return nil, false
			}
			tags := make([]string, 0, len(group))
			for _, item := range group {
				item = strings.ToLower(strings.TrimSpace(item))
				if item == "" || len([]rune(item)) > searchNotesByTagMaxTagRunes {
					return nil, false
				}
				tags = append(tags, item)
			}
			query = append(query, &searchpb.TagQueryGroup{Tags: tags})
		}
	}
	if tag == "" && len(query) == 0 {
		return nil, false
	}

	scope := ""
	if req.Scope.Present && !req.Scope.Null {
		scope = strings.TrimSpace(req.Scope.Value)
		if scope != "local" && scope != "remote" {
			return nil, false
		}
		// The BBS search index has no federation origin dimension.
		return nil, false
	}

	return &searchpb.SearchNotesByTagRequest{
		WithFiles: false,
		SinceId:   req.SinceID.Value,
		UntilId:   req.UntilID.Value,
		Limit:     limit,
		Tag:       tag,
		Scope:     scope,
		Query:     query,
	}, true
}

// Search documents are updated asynchronously. Re-read each hit from
// content-service so stale hidden or archived documents cannot be returned.
func (h *Handler) resolveSearchNotesByTag(ctx context.Context, resp *searchpb.SearchNotesByTagResponse) ([]searchNoteProjection, error) {
	result := make([]searchNoteProjection, 0)
	if resp == nil || len(resp.GetItems()) == 0 {
		return result, nil
	}
	if h.clients == nil || h.clients.Content == nil {
		return nil, status.Error(codes.Unavailable, "content service unavailable")
	}
	result = make([]searchNoteProjection, 0, len(resp.GetItems()))
	for _, hit := range resp.GetItems() {
		if hit == nil {
			continue
		}
		switch hit.GetKind() {
		case "article":
			doc := hit.GetArticle()
			if doc == nil || doc.GetId() <= 0 {
				continue
			}
			current, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: doc.GetId()}, TrackView: false})
			if err != nil {
				if status.Code(err) == codes.NotFound {
					continue
				}
				return nil, err
			}
			article := current.GetArticle()
			if article == nil || article.GetId() != doc.GetId() || article.GetStatus() != contentStatusPublished {
				continue
			}
			result = append(result, searchNoteProjection{Kind: "article", Article: projectSearchArticle(article, doc)})
		case "topic":
			doc := hit.GetTopic()
			if doc == nil || doc.GetId() <= 0 {
				continue
			}
			current, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: doc.GetId()}, TrackView: false})
			if err != nil {
				if status.Code(err) == codes.NotFound {
					continue
				}
				return nil, err
			}
			topic := current.GetTopic()
			if topic == nil || topic.GetId() != doc.GetId() || topic.GetStatus() != contentStatusPublished {
				continue
			}
			result = append(result, searchNoteProjection{Kind: "topic", Topic: projectSearchTopic(topic, doc)})
		}
	}
	return result, nil
}

func projectSearchArticle(current *contentpb.ArticleInfo, indexed *searchpb.ArticleDocument) *searchArticleProjection {
	return &searchArticleProjection{
		ID: strconv.FormatInt(current.GetId(), 10), Title: current.GetTitle(), Summary: current.GetSummary(), ContentExcerpt: current.GetBody(),
		TagIDs: indexed.GetTagIds(), TagNames: nonNilSearchStrings(current.GetTags()), AuthorID: strconv.FormatInt(current.GetAuthorId(), 10),
		AuthorNickname: indexed.GetAuthorNickname(), Status: current.GetStatus(), ViewCount: strconv.FormatInt(current.GetViewCount(), 10),
		CommentCount: strconv.FormatInt(indexed.GetCommentCount(), 10), LikeCount: strconv.FormatInt(indexed.GetLikeCount(), 10),
		FavoriteCount: strconv.FormatInt(indexed.GetFavoriteCount(), 10), CreatedAt: strconv.FormatInt(current.GetCreatedAt(), 10),
		UpdatedAt: strconv.FormatInt(current.GetUpdatedAt(), 10),
	}
}

func projectSearchTopic(current *contentpb.TopicInfo, indexed *searchpb.TopicDocument) *searchTopicProjection {
	return &searchTopicProjection{
		ID: strconv.FormatInt(current.GetId(), 10), Slug: current.GetSlug(), Type: current.GetType(), Title: current.GetTitle(), ContentExcerpt: current.GetBody(),
		TagNames: nonNilSearchStrings(current.GetTags()), AuthorID: strconv.FormatInt(current.GetAuthorId(), 10), Status: current.GetStatus(),
		CommentCount: strconv.FormatInt(indexed.GetCommentCount(), 10), LikeCount: strconv.FormatInt(indexed.GetLikeCount(), 10),
		FavoriteCount: strconv.FormatInt(indexed.GetFavoriteCount(), 10), CreatedAt: strconv.FormatInt(current.GetCreatedAt(), 10),
		UpdatedAt: strconv.FormatInt(current.GetUpdatedAt(), 10), ViewCount: strconv.FormatInt(current.GetViewCount(), 10),
		CategoryID: optionalSearchInt64String(current.GetCategoryId()),
	}
}

func nonNilSearchStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func optionalSearchInt64String(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func writeSearchByTagInvalidParam(c *gin.Context) {
	writeError(c, stdhttp.StatusBadRequest, "Invalid param.", "INVALID_PARAM")
}
