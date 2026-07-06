package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	domain "search-service/internal/domain/search"
)

type ArticleRepository struct {
	client       *http.Client
	addresses    []string
	articleIndex string
	topicIndex   string
}

func NewArticleRepository(addresses []string, articleIndex string, topicIndex ...string) *ArticleRepository {
	if len(addresses) == 0 {
		addresses = []string{"http://127.0.0.1:9200"}
	}
	if articleIndex == "" {
		articleIndex = "bbs_articles"
	}
	topicIndexName := "bbs_topics"
	if len(topicIndex) > 0 && topicIndex[0] != "" {
		topicIndexName = topicIndex[0]
	}
	return &ArticleRepository{
		client:       &http.Client{Timeout: 10 * time.Second},
		addresses:    normalizeAddresses(addresses),
		articleIndex: articleIndex,
		topicIndex:   topicIndexName,
	}
}

func normalizeAddresses(addresses []string) []string {
	out := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		addr = strings.TrimRight(addr, "/")
		if addr != "" {
			out = append(out, addr)
		}
	}
	return out
}

func (r *ArticleRepository) endpoint(path string) string {
	return r.addresses[0] + path
}

func (r *ArticleRepository) EnsureArticleIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.endpoint("/"+r.articleIndex), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check index %s: status %d", r.articleIndex, resp.StatusCode)
	}
	body := map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"dynamic": "false",
			"properties": map[string]any{
				"id":              map[string]any{"type": "keyword"},
				"title":           textWithKeyword(),
				"summary":         map[string]any{"type": "text"},
				"content_excerpt": map[string]any{"type": "text"},
				"tag_ids":         map[string]any{"type": "keyword"},
				"tag_names":       textWithKeyword(),
				"author_id":       map[string]any{"type": "keyword"},
				"author_nickname": textWithKeyword(),
				"status":          map[string]any{"type": "integer"},
				"view_count":      map[string]any{"type": "long"},
				"comment_count":   map[string]any{"type": "long"},
				"like_count":      map[string]any{"type": "long"},
				"favorite_count":  map[string]any{"type": "long"},
				"created_at":      map[string]any{"type": "date", "format": "epoch_millis"},
				"updated_at":      map[string]any{"type": "date", "format": "epoch_millis"},
			},
		},
	}
	return r.createIndex(ctx, r.articleIndex, body)
}

func (r *ArticleRepository) EnsureTopicIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.endpoint("/"+r.topicIndex), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check index %s: status %d", r.topicIndex, resp.StatusCode)
	}
	body := map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"dynamic": "false",
			"properties": map[string]any{
				"id":              map[string]any{"type": "keyword"},
				"slug":            map[string]any{"type": "keyword"},
				"type":            map[string]any{"type": "keyword"},
				"title":           textWithKeyword(),
				"content_excerpt": map[string]any{"type": "text"},
				"tag_names":       textWithKeyword(),
				"author_id":       map[string]any{"type": "keyword"},
				"status":          map[string]any{"type": "integer"},
				"comment_count":   map[string]any{"type": "long"},
				"like_count":      map[string]any{"type": "long"},
				"favorite_count":  map[string]any{"type": "long"},
				"created_at":      map[string]any{"type": "date", "format": "epoch_millis"},
				"updated_at":      map[string]any{"type": "date", "format": "epoch_millis"},
			},
		},
	}
	return r.createIndex(ctx, r.topicIndex, body)
}

func textWithKeyword() map[string]any {
	return map[string]any{
		"type": "text",
		"fields": map[string]any{
			"keyword": map[string]any{"type": "keyword", "ignore_above": 256},
		},
	}
}

func (r *ArticleRepository) createIndex(ctx context.Context, index string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	path := "/" + index
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, r.endpoint(path), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(responseBody), "resource_already_exists_exception") {
		return nil
	}
	return fmt.Errorf("elasticsearch PUT %s: status %d: %s", path, resp.StatusCode, string(responseBody))
}

func (r *ArticleRepository) IndexArticle(ctx context.Context, doc domain.ArticleDocument) error {
	body := map[string]any{
		"id":              strconv.FormatInt(doc.ID, 10),
		"title":           doc.Title,
		"summary":         doc.Summary,
		"content_excerpt": doc.ContentExcerpt,
		"tag_ids":         doc.TagIDs,
		"tag_names":       doc.TagNames,
		"author_id":       strconv.FormatInt(doc.AuthorID, 10),
		"author_nickname": doc.AuthorNickname,
		"status":          doc.Status,
		"view_count":      doc.ViewCount,
		"comment_count":   doc.CommentCount,
		"like_count":      doc.LikeCount,
		"favorite_count":  doc.FavoriteCount,
		"created_at":      doc.CreatedAt,
		"updated_at":      doc.UpdatedAt,
	}
	path := "/" + r.articleIndex + "/_doc/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
	return r.doJSON(ctx, http.MethodPut, path, body, nil)
}

func (r *ArticleRepository) IndexTopic(ctx context.Context, doc domain.TopicDocument) error {
	body := map[string]any{
		"id":              strconv.FormatInt(doc.ID, 10),
		"slug":            doc.Slug,
		"type":            doc.Type,
		"title":           doc.Title,
		"content_excerpt": doc.ContentExcerpt,
		"tag_names":       doc.TagNames,
		"author_id":       strconv.FormatInt(doc.AuthorID, 10),
		"status":          doc.Status,
		"comment_count":   doc.CommentCount,
		"like_count":      doc.LikeCount,
		"favorite_count":  doc.FavoriteCount,
		"created_at":      doc.CreatedAt,
		"updated_at":      doc.UpdatedAt,
	}
	path := "/" + r.topicIndex + "/_doc/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
	return r.doJSON(ctx, http.MethodPut, path, body, nil)
}

func (r *ArticleRepository) DeleteArticle(ctx context.Context, id int64) error {
	return r.delete(ctx, r.articleIndex, id, "article")
}

func (r *ArticleRepository) DeleteTopic(ctx context.Context, id int64) error {
	return r.delete(ctx, r.topicIndex, id, "topic")
}

func (r *ArticleRepository) delete(ctx context.Context, index string, id int64, entity string) error {
	path := "/" + index + "/_doc/" + url.PathEscape(strconv.FormatInt(id, 10))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, r.endpoint(path), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("delete %s index: status %d", entity, resp.StatusCode)
}

func (r *ArticleRepository) IncrementArticleCommentCount(ctx context.Context, id int64, delta int64) error {
	return r.incrementCommentCount(ctx, r.articleIndex, id, delta)
}

func (r *ArticleRepository) IncrementTopicCommentCount(ctx context.Context, id int64, delta int64) error {
	return r.incrementCommentCount(ctx, r.topicIndex, id, delta)
}

func (r *ArticleRepository) incrementCommentCount(ctx context.Context, index string, id int64, delta int64) error {
	script := `
if (ctx._source.comment_count == null) {
  ctx._source.comment_count = 0;
}
ctx._source.comment_count += params.delta;
if (ctx._source.comment_count < 0) {
  ctx._source.comment_count = 0;
}
ctx._source.updated_at = params.updated_at;
`
	return r.updateDocument(ctx, index, id, script, map[string]any{
		"delta":      delta,
		"updated_at": time.Now().UnixMilli(),
	})
}

func (r *ArticleRepository) SetArticleLikeCount(ctx context.Context, id int64, count int64) error {
	if count < 0 {
		count = 0
	}
	return r.setCounter(ctx, r.articleIndex, id, "like_count", count)
}

func (r *ArticleRepository) SetTopicLikeCount(ctx context.Context, id int64, count int64) error {
	if count < 0 {
		count = 0
	}
	return r.setCounter(ctx, r.topicIndex, id, "like_count", count)
}

func (r *ArticleRepository) SetArticleFavoriteCount(ctx context.Context, id int64, count int64) error {
	if count < 0 {
		count = 0
	}
	return r.setCounter(ctx, r.articleIndex, id, "favorite_count", count)
}

func (r *ArticleRepository) SetTopicFavoriteCount(ctx context.Context, id int64, count int64) error {
	if count < 0 {
		count = 0
	}
	return r.setCounter(ctx, r.topicIndex, id, "favorite_count", count)
}

func (r *ArticleRepository) setCounter(ctx context.Context, index string, id int64, field string, count int64) error {
	script := fmt.Sprintf("ctx._source.%s = params.count; ctx._source.updated_at = params.updated_at;", field)
	return r.updateDocument(ctx, index, id, script, map[string]any{
		"count":      count,
		"updated_at": time.Now().UnixMilli(),
	})
}

func (r *ArticleRepository) updateDocument(ctx context.Context, index string, id int64, script string, params map[string]any) error {
	path := "/" + index + "/_update/" + url.PathEscape(strconv.FormatInt(id, 10))
	body := map[string]any{
		"script": map[string]any{
			"lang":   "painless",
			"source": script,
			"params": params,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint(path), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("elasticsearch POST %s: status %d: %s", path, resp.StatusCode, string(payload))
	}
	return nil
}

func (r *ArticleRepository) SearchArticles(ctx context.Context, keyword string, page, pageSize int32) ([]domain.ArticleHit, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	from := (page - 1) * pageSize
	body := map[string]any{
		"from": from,
		"size": pageSize,
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  keyword,
				"fields": []string{"title^3", "summary^2", "content_excerpt", "tag_names"},
			},
		},
		"sort": []any{
			map[string]any{"_score": "desc"},
			map[string]any{"created_at": "desc"},
		},
	}
	var result searchResponse
	if err := r.doJSON(ctx, http.MethodPost, "/"+r.articleIndex+"/_search", body, &result); err != nil {
		return nil, 0, err
	}
	items := make([]domain.ArticleHit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		items = append(items, domain.ArticleHit{Document: hit.Source.toDomain(), Score: hit.Score})
	}
	return items, result.Hits.Total.Value, nil
}

func (r *ArticleRepository) SearchTopics(ctx context.Context, keyword string, page, pageSize int32) ([]domain.TopicHit, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	from := (page - 1) * pageSize
	body := map[string]any{
		"from": from,
		"size": pageSize,
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  keyword,
				"fields": []string{"title^3", "content_excerpt", "tag_names"},
			},
		},
		"sort": []any{
			map[string]any{"_score": "desc"},
			map[string]any{"created_at": "desc"},
		},
	}
	var result topicSearchResponse
	if err := r.doJSON(ctx, http.MethodPost, "/"+r.topicIndex+"/_search", body, &result); err != nil {
		return nil, 0, err
	}
	items := make([]domain.TopicHit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		items = append(items, domain.TopicHit{Document: hit.Source.toDomain(), Score: hit.Score})
	}
	return items, result.Hits.Total.Value, nil
}

func (r *ArticleRepository) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.endpoint(path), reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("elasticsearch %s %s: status %d: %s", method, path, resp.StatusCode, string(payload))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

type searchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Score  float64         `json:"_score"`
			Source articleDocument `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type topicSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Score  float64       `json:"_score"`
			Source topicDocument `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type articleDocument struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	ContentExcerpt string   `json:"content_excerpt"`
	TagIDs         []string `json:"tag_ids"`
	TagNames       []string `json:"tag_names"`
	AuthorID       string   `json:"author_id"`
	AuthorNickname string   `json:"author_nickname"`
	Status         int32    `json:"status"`
	ViewCount      int64    `json:"view_count"`
	CommentCount   int64    `json:"comment_count"`
	LikeCount      int64    `json:"like_count"`
	FavoriteCount  int64    `json:"favorite_count"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

func (d articleDocument) toDomain() domain.ArticleDocument {
	id, _ := strconv.ParseInt(d.ID, 10, 64)
	authorID, _ := strconv.ParseInt(d.AuthorID, 10, 64)
	return domain.ArticleDocument{
		ID:             id,
		Title:          d.Title,
		Summary:        d.Summary,
		ContentExcerpt: d.ContentExcerpt,
		TagIDs:         d.TagIDs,
		TagNames:       d.TagNames,
		AuthorID:       authorID,
		AuthorNickname: d.AuthorNickname,
		Status:         d.Status,
		ViewCount:      d.ViewCount,
		CommentCount:   d.CommentCount,
		LikeCount:      d.LikeCount,
		FavoriteCount:  d.FavoriteCount,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

type topicDocument struct {
	ID             string   `json:"id"`
	Slug           string   `json:"slug"`
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	ContentExcerpt string   `json:"content_excerpt"`
	TagNames       []string `json:"tag_names"`
	AuthorID       string   `json:"author_id"`
	Status         int32    `json:"status"`
	CommentCount   int64    `json:"comment_count"`
	LikeCount      int64    `json:"like_count"`
	FavoriteCount  int64    `json:"favorite_count"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

func (d topicDocument) toDomain() domain.TopicDocument {
	id, _ := strconv.ParseInt(d.ID, 10, 64)
	authorID, _ := strconv.ParseInt(d.AuthorID, 10, 64)
	return domain.TopicDocument{
		ID:             id,
		Slug:           d.Slug,
		Type:           d.Type,
		Title:          d.Title,
		ContentExcerpt: d.ContentExcerpt,
		TagNames:       d.TagNames,
		AuthorID:       authorID,
		Status:         d.Status,
		CommentCount:   d.CommentCount,
		LikeCount:      d.LikeCount,
		FavoriteCount:  d.FavoriteCount,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}
