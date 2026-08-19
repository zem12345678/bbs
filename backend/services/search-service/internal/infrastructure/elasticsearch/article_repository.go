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

	elastic "github.com/elastic/go-elasticsearch/v9"
)

type ArticleRepository struct {
	client                *elastic.Client
	articleIndex          string
	topicIndex            string
	userIndex             string
	accountTombstoneIndex string
}

func NewArticleRepository(client *elastic.Client, articleIndex string, indices ...string) *ArticleRepository {
	if client == nil {
		panic("elasticsearch client is required")
	}
	articleIndex = strings.TrimSpace(articleIndex)
	if articleIndex == "" {
		articleIndex = "bbs_articles"
	}
	topicIndexName := optionalIndex(indices, 0, "bbs_topics")
	userIndexName := optionalIndex(indices, 1, "bbs_users_v2")
	accountTombstoneIndexName := optionalIndex(indices, 2, "bbs_search_account_tombstones_v1")
	return &ArticleRepository{
		client:                client,
		articleIndex:          articleIndex,
		topicIndex:            topicIndexName,
		userIndex:             userIndexName,
		accountTombstoneIndex: accountTombstoneIndexName,
	}
}

func optionalIndex(indices []string, index int, fallback string) string {
	if len(indices) > index && strings.TrimSpace(indices[index]) != "" {
		return strings.TrimSpace(indices[index])
	}
	return fallback
}

func (r *ArticleRepository) endpoint(path string) string {
	if r.articleIndex == "" {
		r.articleIndex = "bbs_articles"
	}
	if r.topicIndex == "" {
		r.topicIndex = "bbs_topics"
	}
	if r.userIndex == "" {
		r.userIndex = "bbs_users_v2"
	}
	if r.accountTombstoneIndex == "" {
		r.accountTombstoneIndex = "bbs_search_account_tombstones_v1"
	}
	return path
}

func (r *ArticleRepository) EnsureArticleIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.endpoint("/"+r.articleIndex), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Perform(req)
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
	resp, err := r.client.Perform(req)
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
				"view_count":      map[string]any{"type": "long"},
				"category_id":     map[string]any{"type": "keyword"},
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

func (r *ArticleRepository) EnsureUserIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.endpoint("/"+r.userIndex), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Perform(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check index %s: status %d", r.userIndex, resp.StatusCode)
	}
	body := map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"dynamic": "false",
			"properties": map[string]any{
				"id":         map[string]any{"type": "keyword"},
				"username":   textWithKeyword(),
				"nickname":   textWithKeyword(),
				"status":     map[string]any{"type": "integer"},
				"created_at": map[string]any{"type": "date", "format": "epoch_millis"},
				"updated_at": map[string]any{"type": "date", "format": "epoch_millis"},
			},
		},
	}
	return r.createIndex(ctx, r.userIndex, body)
}

func (r *ArticleRepository) ensureAccountTombstoneIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.endpoint("/"+r.accountTombstoneIndex), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Perform(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check index %s: status %d", r.accountTombstoneIndex, resp.StatusCode)
	}
	body := map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"dynamic": "false",
			"properties": map[string]any{
				"user_id":         map[string]any{"type": "keyword"},
				"deletion_job_id": map[string]any{"type": "keyword"},
				"policy_version":  map[string]any{"type": "integer"},
				"erased_at":       map[string]any{"type": "date", "format": "epoch_millis"},
			},
		},
	}
	return r.createIndex(ctx, r.accountTombstoneIndex, body)
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
	resp, err := r.client.Perform(req)
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
	return r.writeAccountProjection(ctx, doc.AuthorID, func() error {
		body := articleIndexBody(doc)
		path := "/" + r.articleIndex + "/_doc/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
		return r.doJSON(ctx, http.MethodPut, path, body, nil)
	}, func() error {
		return r.deleteWithRefresh(ctx, r.articleIndex, doc.ID, "article")
	})
}

func (r *ArticleRepository) IndexTopic(ctx context.Context, doc domain.TopicDocument) error {
	return r.writeAccountProjection(ctx, doc.AuthorID, func() error {
		body := topicIndexBody(doc)
		path := "/" + r.topicIndex + "/_doc/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
		return r.doJSON(ctx, http.MethodPut, path, body, nil)
	}, func() error {
		return r.deleteWithRefresh(ctx, r.topicIndex, doc.ID, "topic")
	})
}

func (r *ArticleRepository) IndexUser(ctx context.Context, doc domain.UserDocument) error {
	return r.writeAccountProjection(ctx, doc.ID, func() error {
		body := userIndexBody(doc)
		path := "/" + r.userIndex + "/_doc/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
		return r.doJSON(ctx, http.MethodPut, path, body, nil)
	}, func() error {
		return r.deleteWithRefresh(ctx, r.userIndex, doc.ID, "user")
	})
}

func (r *ArticleRepository) ReindexArticle(ctx context.Context, doc domain.ArticleDocument) error {
	return r.writeAccountProjection(ctx, doc.AuthorID, func() error {
		body := map[string]any{
			"doc":    articleReindexBody(doc),
			"upsert": articleIndexBody(doc),
		}
		path := "/" + r.articleIndex + "/_update/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
		return r.doJSON(ctx, http.MethodPost, path, body, nil)
	}, func() error {
		return r.deleteWithRefresh(ctx, r.articleIndex, doc.ID, "article")
	})
}

func (r *ArticleRepository) ReindexTopic(ctx context.Context, doc domain.TopicDocument) error {
	return r.writeAccountProjection(ctx, doc.AuthorID, func() error {
		body := map[string]any{
			"doc":    topicReindexBody(doc),
			"upsert": topicIndexBody(doc),
		}
		path := "/" + r.topicIndex + "/_update/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
		return r.doJSON(ctx, http.MethodPost, path, body, nil)
	}, func() error {
		return r.deleteWithRefresh(ctx, r.topicIndex, doc.ID, "topic")
	})
}

func (r *ArticleRepository) ReindexUser(ctx context.Context, doc domain.UserDocument) error {
	return r.writeAccountProjection(ctx, doc.ID, func() error {
		body := map[string]any{
			"doc":    userIndexBody(doc),
			"upsert": userIndexBody(doc),
		}
		path := "/" + r.userIndex + "/_update/" + url.PathEscape(strconv.FormatInt(doc.ID, 10))
		return r.doJSON(ctx, http.MethodPost, path, body, nil)
	}, func() error {
		return r.deleteWithRefresh(ctx, r.userIndex, doc.ID, "user")
	})
}

func (r *ArticleRepository) writeAccountProjection(ctx context.Context, userID int64, write, remove func() error) error {
	erased, err := r.isAccountErased(ctx, userID)
	if err != nil {
		return err
	}
	if erased {
		return remove()
	}
	if err := write(); err != nil {
		return err
	}
	erased, err = r.isAccountErased(ctx, userID)
	if err != nil {
		return err
	}
	if erased {
		return remove()
	}
	return nil
}

func articleIndexBody(doc domain.ArticleDocument) map[string]any {
	return map[string]any{
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
}

func articleReindexBody(doc domain.ArticleDocument) map[string]any {
	return map[string]any{
		"id":              strconv.FormatInt(doc.ID, 10),
		"title":           doc.Title,
		"summary":         doc.Summary,
		"content_excerpt": doc.ContentExcerpt,
		"tag_names":       doc.TagNames,
		"author_id":       strconv.FormatInt(doc.AuthorID, 10),
		"status":          doc.Status,
		"view_count":      doc.ViewCount,
		"created_at":      doc.CreatedAt,
		"updated_at":      doc.UpdatedAt,
	}
}

func topicIndexBody(doc domain.TopicDocument) map[string]any {
	return map[string]any{
		"id":              strconv.FormatInt(doc.ID, 10),
		"slug":            doc.Slug,
		"type":            doc.Type,
		"title":           doc.Title,
		"content_excerpt": doc.ContentExcerpt,
		"tag_names":       doc.TagNames,
		"author_id":       strconv.FormatInt(doc.AuthorID, 10),
		"status":          doc.Status,
		"view_count":      doc.ViewCount,
		"category_id":     strconv.FormatInt(doc.CategoryID, 10),
		"comment_count":   doc.CommentCount,
		"like_count":      doc.LikeCount,
		"favorite_count":  doc.FavoriteCount,
		"created_at":      doc.CreatedAt,
		"updated_at":      doc.UpdatedAt,
	}
}

func topicReindexBody(doc domain.TopicDocument) map[string]any {
	return map[string]any{
		"id":              strconv.FormatInt(doc.ID, 10),
		"slug":            doc.Slug,
		"type":            doc.Type,
		"title":           doc.Title,
		"content_excerpt": doc.ContentExcerpt,
		"tag_names":       doc.TagNames,
		"author_id":       strconv.FormatInt(doc.AuthorID, 10),
		"status":          doc.Status,
		"view_count":      doc.ViewCount,
		"category_id":     strconv.FormatInt(doc.CategoryID, 10),
		"created_at":      doc.CreatedAt,
		"updated_at":      doc.UpdatedAt,
	}
}

func userIndexBody(doc domain.UserDocument) map[string]any {
	return map[string]any{
		"id":         strconv.FormatInt(doc.ID, 10),
		"username":   doc.Username,
		"nickname":   doc.Nickname,
		"status":     doc.Status,
		"created_at": doc.CreatedAt,
		"updated_at": doc.UpdatedAt,
	}
}

func (r *ArticleRepository) DeleteArticle(ctx context.Context, id int64) error {
	return r.delete(ctx, r.articleIndex, id, "article")
}

func (r *ArticleRepository) DeleteTopic(ctx context.Context, id int64) error {
	return r.delete(ctx, r.topicIndex, id, "topic")
}

func (r *ArticleRepository) DeleteUser(ctx context.Context, id int64) error {
	return r.delete(ctx, r.userIndex, id, "user")
}

func (r *ArticleRepository) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) error {
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	if deletionJobID <= 0 {
		return domain.ErrInvalidDeletionJobID
	}
	if policyVersion <= 0 {
		return domain.ErrInvalidPolicyVersion
	}
	if err := r.ensureAccountTombstoneIndex(ctx); err != nil {
		return err
	}
	tombstone := map[string]any{
		"user_id":         strconv.FormatInt(userID, 10),
		"deletion_job_id": strconv.FormatInt(deletionJobID, 10),
		"policy_version":  policyVersion,
		"erased_at":       time.Now().UnixMilli(),
	}
	tombstonePath := "/" + r.accountTombstoneIndex + "/_doc/" + url.PathEscape(strconv.FormatInt(userID, 10)) + "?refresh=wait_for"
	if err := r.doJSON(ctx, http.MethodPut, tombstonePath, tombstone, nil); err != nil {
		return fmt.Errorf("write search account tombstone: %w", err)
	}
	if err := r.deleteWithRefresh(ctx, r.userIndex, userID, "user"); err != nil {
		return err
	}
	if err := r.deleteAuthorDocuments(ctx, r.articleIndex, userID, "article"); err != nil {
		return err
	}
	if err := r.deleteAuthorDocuments(ctx, r.topicIndex, userID, "topic"); err != nil {
		return err
	}
	return nil
}

func (r *ArticleRepository) isAccountErased(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	path := "/" + r.accountTombstoneIndex + "/_doc/" + url.PathEscape(strconv.FormatInt(userID, 10))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.endpoint(path), nil)
	if err != nil {
		return false, err
	}
	resp, err := r.client.Perform(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return false, fmt.Errorf("check search account tombstone: status %d: %s", resp.StatusCode, string(payload))
	}
}

func (r *ArticleRepository) deleteWithRefresh(ctx context.Context, index string, id int64, entity string) error {
	path := "/" + index + "/_doc/" + url.PathEscape(strconv.FormatInt(id, 10)) + "?refresh=wait_for"
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, r.endpoint(path), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Perform(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("delete %s search projection: status %d: %s", entity, resp.StatusCode, string(payload))
}

func (r *ArticleRepository) deleteAuthorDocuments(ctx context.Context, index string, userID int64, entity string) error {
	path := "/" + index + "/_delete_by_query?refresh=true&conflicts=proceed&wait_for_completion=true"
	body := map[string]any{
		"query": map[string]any{
			"term": map[string]any{"author_id": strconv.FormatInt(userID, 10)},
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
	resp, err := r.client.Perform(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("delete %s search projections by author: status %d: %s", entity, resp.StatusCode, string(responseBody))
}

func (r *ArticleRepository) delete(ctx context.Context, index string, id int64, entity string) error {
	path := "/" + index + "/_doc/" + url.PathEscape(strconv.FormatInt(id, 10))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, r.endpoint(path), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Perform(req)
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

func (r *ArticleRepository) SetArticleViewCount(ctx context.Context, id int64, count int64) error {
	if count < 0 {
		count = 0
	}
	return r.setCounter(ctx, r.articleIndex, id, "view_count", count)
}

func (r *ArticleRepository) SetTopicViewCount(ctx context.Context, id int64, count int64) error {
	if count < 0 {
		count = 0
	}
	return r.setCounter(ctx, r.topicIndex, id, "view_count", count)
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
	resp, err := r.client.Perform(req)
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
		"from":  from,
		"size":  pageSize,
		"query": keywordSearchQuery(keyword, []string{"title^3", "summary^2", "content_excerpt", "tag_names"}),
		"sort": []any{
			map[string]any{"_score": "desc"},
			map[string]any{"created_at": "desc"},
		},
		"highlight": highlightFields("title", "summary", "content_excerpt", "tag_names"),
	}
	var result searchResponse
	if err := r.doJSON(ctx, http.MethodPost, "/"+r.articleIndex+"/_search", body, &result); err != nil {
		return nil, 0, err
	}
	items := make([]domain.ArticleHit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		items = append(items, domain.ArticleHit{
			Document:  hit.Source.toDomain(),
			Score:     hit.Score,
			Highlight: hit.Highlight.toDomain(),
		})
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
		"from":  from,
		"size":  pageSize,
		"query": keywordSearchQuery(keyword, []string{"title^3", "content_excerpt", "tag_names"}),
		"sort": []any{
			map[string]any{"_score": "desc"},
			map[string]any{"created_at": "desc"},
		},
		"highlight": highlightFields("title", "content_excerpt", "tag_names"),
	}
	var result topicSearchResponse
	if err := r.doJSON(ctx, http.MethodPost, "/"+r.topicIndex+"/_search", body, &result); err != nil {
		return nil, 0, err
	}
	items := make([]domain.TopicHit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		items = append(items, domain.TopicHit{
			Document:  hit.Source.toDomain(),
			Score:     hit.Score,
			Highlight: hit.Highlight.toDomain(),
		})
	}
	return items, result.Hits.Total.Value, nil
}

func (r *ArticleRepository) SearchUsers(ctx context.Context, keyword string, page, pageSize int32) ([]domain.UserHit, int64, error) {
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
		"from":  from,
		"size":  pageSize,
		"query": userSearchQuery(keyword),
		"sort": []any{
			map[string]any{"_score": "desc"},
			map[string]any{"updated_at": "desc"},
			map[string]any{"created_at": "desc"},
		},
	}
	var result userSearchResponse
	if err := r.doJSON(ctx, http.MethodPost, "/"+r.userIndex+"/_search", body, &result); err != nil {
		return nil, 0, err
	}
	items := make([]domain.UserHit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		items = append(items, domain.UserHit{Document: hit.Source.toDomain(), Score: hit.Score})
	}
	return items, result.Hits.Total.Value, nil
}

func (r *ArticleRepository) SearchByTag(ctx context.Context, criteria domain.SearchByTagCriteria) ([]domain.NoteLikeHit, error) {
	tagFilter := tagSearchFilter(criteria)
	filters := []any{
		map[string]any{"term": map[string]any{"status": 2}},
		tagFilter,
	}
	if criteria.SinceID > 0 || criteria.UntilID > 0 {
		bounds := map[string]any{}
		if criteria.SinceID > 0 {
			bounds["gt"] = criteria.SinceID
		}
		if criteria.UntilID > 0 {
			bounds["lt"] = criteria.UntilID
		}
		filters = append(filters, map[string]any{"range": map[string]any{"id_numeric": bounds}})
	}
	body := map[string]any{
		"size": criteria.Limit,
		"runtime_mappings": map[string]any{
			"id_numeric": map[string]any{
				"type":   "long",
				"script": map[string]any{"source": "if (doc['id'].size() != 0) emit(Long.parseLong(doc['id'].value));"},
			},
		},
		"query": map[string]any{"bool": map[string]any{"filter": filters}},
		"sort": []any{
			map[string]any{"id_numeric": map[string]any{"order": "desc"}},
			map[string]any{"created_at": map[string]any{"order": "desc"}},
			map[string]any{"_index": map[string]any{"order": "asc"}},
		},
	}
	var result tagSearchResponse
	path := "/" + r.articleIndex + "," + r.topicIndex + "/_search"
	if err := r.doJSON(ctx, http.MethodPost, path, body, &result); err != nil {
		return nil, err
	}
	items := make([]domain.NoteLikeHit, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		switch hit.Index {
		case r.articleIndex:
			var source articleDocument
			if err := json.Unmarshal(hit.Source, &source); err != nil {
				return nil, fmt.Errorf("decode article tag-search hit: %w", err)
			}
			doc := source.toDomain()
			items = append(items, domain.NoteLikeHit{Kind: domain.NoteLikeArticle, Article: &doc})
		case r.topicIndex:
			var source topicDocument
			if err := json.Unmarshal(hit.Source, &source); err != nil {
				return nil, fmt.Errorf("decode topic tag-search hit: %w", err)
			}
			doc := source.toDomain()
			items = append(items, domain.NoteLikeHit{Kind: domain.NoteLikeTopic, Topic: &doc})
		default:
			return nil, fmt.Errorf("unexpected tag-search index %q", hit.Index)
		}
	}
	return items, nil
}

func tagSearchFilter(criteria domain.SearchByTagCriteria) map[string]any {
	if criteria.Tag != "" {
		return exactTagClause(criteria.Tag)
	}
	groups := make([]any, 0, len(criteria.Query))
	for _, group := range criteria.Query {
		allTags := make([]any, 0, len(group.Tags))
		for _, tag := range group.Tags {
			allTags = append(allTags, exactTagClause(tag))
		}
		groups = append(groups, map[string]any{"bool": map[string]any{"filter": allTags}})
	}
	return map[string]any{"bool": map[string]any{"should": groups, "minimum_should_match": 1}}
}

func exactTagClause(tag string) map[string]any {
	return map[string]any{
		"term": map[string]any{
			"tag_names.keyword": map[string]any{"value": tag, "case_insensitive": true},
		},
	}
}

func keywordSearchQuery(keyword string, fields []string) map[string]any {
	return map[string]any{
		"bool": map[string]any{
			"should": []any{
				map[string]any{
					"multi_match": map[string]any{
						"query":  keyword,
						"fields": fields,
					},
				},
				map[string]any{
					"multi_match": map[string]any{
						"query":          keyword,
						"fields":         fields,
						"fuzziness":      "AUTO",
						"prefix_length":  1,
						"max_expansions": 50,
					},
				},
			},
			"minimum_should_match": 1,
		},
	}
}

func userSearchQuery(keyword string) map[string]any {
	fields := []string{"username^3", "nickname^2"}
	return map[string]any{
		"bool": map[string]any{
			"filter": []any{
				map[string]any{"term": map[string]any{"status": 1}},
			},
			"should": []any{
				map[string]any{
					"multi_match": map[string]any{
						"query":  keyword,
						"fields": fields,
					},
				},
				map[string]any{
					"multi_match": map[string]any{
						"query":          keyword,
						"fields":         fields,
						"fuzziness":      "AUTO",
						"prefix_length":  1,
						"max_expansions": 50,
					},
				},
			},
			"minimum_should_match": 1,
		},
	}
}

func highlightFields(fields ...string) map[string]any {
	items := make(map[string]any, len(fields))
	for _, field := range fields {
		items[field] = map[string]any{
			"number_of_fragments": 2,
			"fragment_size":       140,
		}
	}
	return map[string]any{
		"pre_tags":  []string{"<mark>"},
		"post_tags": []string{"</mark>"},
		"fields":    items,
	}
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
	resp, err := r.client.Perform(req)
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
			Score     float64         `json:"_score"`
			Source    articleDocument `json:"_source"`
			Highlight searchHighlight `json:"highlight"`
		} `json:"hits"`
	} `json:"hits"`
}

type topicSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Score     float64         `json:"_score"`
			Source    topicDocument   `json:"_source"`
			Highlight searchHighlight `json:"highlight"`
		} `json:"hits"`
	} `json:"hits"`
}

type userSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Score  float64      `json:"_score"`
			Source userDocument `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type tagSearchResponse struct {
	Hits struct {
		Hits []struct {
			Index  string          `json:"_index"`
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type searchHighlight struct {
	Title          []string `json:"title"`
	Summary        []string `json:"summary"`
	ContentExcerpt []string `json:"content_excerpt"`
	TagNames       []string `json:"tag_names"`
}

func (h searchHighlight) toDomain() domain.SearchHighlight {
	return domain.SearchHighlight{
		Title:          h.Title,
		Summary:        h.Summary,
		ContentExcerpt: h.ContentExcerpt,
		TagNames:       h.TagNames,
	}
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
	ViewCount      int64    `json:"view_count"`
	CategoryID     string   `json:"category_id"`
	CommentCount   int64    `json:"comment_count"`
	LikeCount      int64    `json:"like_count"`
	FavoriteCount  int64    `json:"favorite_count"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

func (d topicDocument) toDomain() domain.TopicDocument {
	id, _ := strconv.ParseInt(d.ID, 10, 64)
	authorID, _ := strconv.ParseInt(d.AuthorID, 10, 64)
	categoryID, _ := strconv.ParseInt(d.CategoryID, 10, 64)
	return domain.TopicDocument{
		ID:             id,
		Slug:           d.Slug,
		Type:           d.Type,
		Title:          d.Title,
		ContentExcerpt: d.ContentExcerpt,
		TagNames:       d.TagNames,
		AuthorID:       authorID,
		Status:         d.Status,
		ViewCount:      d.ViewCount,
		CategoryID:     categoryID,
		CommentCount:   d.CommentCount,
		LikeCount:      d.LikeCount,
		FavoriteCount:  d.FavoriteCount,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

type userDocument struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Status    int32  `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (d userDocument) toDomain() domain.UserDocument {
	id, _ := strconv.ParseInt(d.ID, 10, 64)
	return domain.UserDocument{
		ID:        id,
		Username:  d.Username,
		Nickname:  d.Nickname,
		Status:    d.Status,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}
