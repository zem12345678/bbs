package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	domain "search-service/internal/domain/search"

	elastic "github.com/elastic/go-elasticsearch/v9"
)

func TestArticleRepositorySmoke(t *testing.T) {
	if os.Getenv("BBS_ES_SMOKE") != "1" {
		t.Skip("set BBS_ES_SMOKE=1 to run against local Elasticsearch")
	}

	ctx := context.Background()
	client, err := elastic.NewClient(elastic.Config{Addresses: []string{"http://127.0.0.1:9200"}})
	if err != nil {
		t.Fatalf("new elasticsearch client: %v", err)
	}
	repo := NewArticleRepository(client, "bbs_articles")
	if err := repo.EnsureArticleIndex(ctx); err != nil {
		t.Fatalf("ensure index: %v", err)
	}

	now := time.Now().UnixMilli()
	doc := domain.ArticleDocument{
		ID:             9000001,
		Title:          "BBS local search smoke article",
		Summary:        "Elasticsearch smoke test",
		ContentExcerpt: "community forum article search indexing smoke test",
		TagNames:       []string{"bbs", "search"},
		AuthorID:       1,
		AuthorNickname: "dev",
		Status:         2,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.IndexArticle(ctx, doc); err != nil {
		t.Fatalf("index article: %v", err)
	}
	defer func() { _ = repo.DeleteArticle(ctx, doc.ID) }()
	if err := repo.IncrementArticleCommentCount(ctx, doc.ID, 2); err != nil {
		t.Fatalf("increment comment count: %v", err)
	}
	if err := repo.SetArticleLikeCount(ctx, doc.ID, 3); err != nil {
		t.Fatalf("set like count: %v", err)
	}
	if err := repo.SetArticleFavoriteCount(ctx, doc.ID, 4); err != nil {
		t.Fatalf("set favorite count: %v", err)
	}
	if err := repo.doJSON(ctx, http.MethodPost, "/"+repo.articleIndex+"/_refresh", nil, nil); err != nil {
		t.Fatalf("refresh article index: %v", err)
	}

	hits, total, err := repo.SearchArticles(ctx, "forum search", 1, 10)
	if err != nil {
		t.Fatalf("search articles: %v", err)
	}
	if total == 0 || len(hits) == 0 {
		t.Fatalf("expected at least one hit, total=%d len=%d", total, len(hits))
	}
	var got domain.ArticleHit
	for _, hit := range hits {
		if hit.Document.ID == doc.ID {
			got = hit
			break
		}
	}
	if got.Document.ID == 0 {
		t.Fatalf("indexed article not found in hits")
	}
	if got.Document.CommentCount != 2 || got.Document.LikeCount != 3 || got.Document.FavoriteCount != 4 {
		t.Fatalf("counts comment=%d like=%d favorite=%d", got.Document.CommentCount, got.Document.LikeCount, got.Document.FavoriteCount)
	}
	if !hasMarkedHighlight(got.Highlight) {
		t.Fatalf("expected highlighted fragments, got %#v", got.Highlight)
	}
}

func TestAccountErasureRepositorySmoke(t *testing.T) {
	if os.Getenv("BBS_ES_SMOKE") != "1" {
		t.Skip("set BBS_ES_SMOKE=1 to run against local Elasticsearch")
	}
	ctx := context.Background()
	client, err := elastic.NewClient(elastic.Config{Addresses: []string{"http://127.0.0.1:9200"}})
	if err != nil {
		t.Fatalf("new elasticsearch client: %v", err)
	}
	suffix := time.Now().UnixNano()
	repo := NewArticleRepository(client,
		fmt.Sprintf("bbs_test_articles_%d", suffix),
		fmt.Sprintf("bbs_test_topics_%d", suffix),
		fmt.Sprintf("bbs_test_users_%d", suffix),
		fmt.Sprintf("bbs_test_account_tombstones_%d", suffix),
	)
	defer func() {
		for _, index := range []string{repo.articleIndex, repo.topicIndex, repo.userIndex, repo.accountTombstoneIndex} {
			_ = repo.doJSON(context.Background(), http.MethodDelete, "/"+index, nil, nil)
		}
	}()
	if err := repo.EnsureArticleIndex(ctx); err != nil {
		t.Fatalf("ensure article index: %v", err)
	}
	if err := repo.EnsureTopicIndex(ctx); err != nil {
		t.Fatalf("ensure topic index: %v", err)
	}
	if err := repo.EnsureUserIndex(ctx); err != nil {
		t.Fatalf("ensure user index: %v", err)
	}

	userID := suffix
	otherUserID := suffix + 1
	articleID := suffix + 10
	topicID := suffix + 20
	now := time.Now().UnixMilli()
	if err := repo.IndexArticle(ctx, domain.ArticleDocument{ID: articleID, AuthorID: userID, Title: "erasure target article", Status: 2, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("index target article: %v", err)
	}
	if err := repo.IndexTopic(ctx, domain.TopicDocument{ID: topicID, AuthorID: userID, Title: "erasure target topic", Status: 2, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("index target topic: %v", err)
	}
	if err := repo.IndexUser(ctx, domain.UserDocument{ID: userID, Username: "erasure-target", Nickname: "Erasure Target", Status: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("index target user: %v", err)
	}
	if err := repo.IndexArticle(ctx, domain.ArticleDocument{ID: articleID + 1, AuthorID: otherUserID, Title: "erasure retained article", Status: 2, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("index retained article: %v", err)
	}
	if err := repo.EraseUserData(ctx, userID, suffix+100, 3); err != nil {
		t.Fatalf("erase user data: %v", err)
	}
	if err := repo.EraseUserData(ctx, userID, suffix+100, 3); err != nil {
		t.Fatalf("repeat erase user data: %v", err)
	}

	for _, item := range []struct {
		index string
		id    int64
	}{{repo.articleIndex, articleID}, {repo.topicIndex, topicID}, {repo.userIndex, userID}} {
		exists, err := searchDocumentExists(ctx, client, item.index, item.id)
		if err != nil || exists {
			t.Fatalf("purged document %s/%d exists=%v error=%v", item.index, item.id, exists, err)
		}
	}
	retained, err := searchDocumentExists(ctx, client, repo.articleIndex, articleID+1)
	if err != nil || !retained {
		t.Fatalf("retained article exists=%v error=%v", retained, err)
	}

	if err := repo.IndexArticle(ctx, domain.ArticleDocument{ID: articleID, AuthorID: userID, Title: "must not revive"}); err != nil {
		t.Fatalf("late index article: %v", err)
	}
	if err := repo.ReindexTopic(ctx, domain.TopicDocument{ID: topicID, AuthorID: userID, Title: "must not revive"}); err != nil {
		t.Fatalf("late reindex topic: %v", err)
	}
	if err := repo.IndexUser(ctx, domain.UserDocument{ID: userID, Username: "must-not-revive"}); err != nil {
		t.Fatalf("late index user: %v", err)
	}
	for _, item := range []struct {
		index string
		id    int64
	}{{repo.articleIndex, articleID}, {repo.topicIndex, topicID}, {repo.userIndex, userID}} {
		exists, err := searchDocumentExists(ctx, client, item.index, item.id)
		if err != nil || exists {
			t.Fatalf("late document %s/%d exists=%v error=%v", item.index, item.id, exists, err)
		}
	}
}

func searchDocumentExists(ctx context.Context, client *elastic.Client, index string, id int64) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("/%s/_doc/%d", index, id), nil)
	if err != nil {
		return false, err
	}
	response, err := client.Perform(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return response.StatusCode == http.StatusOK, nil
}

func hasMarkedHighlight(highlight domain.SearchHighlight) bool {
	for _, fragments := range [][]string{
		highlight.Title,
		highlight.Summary,
		highlight.ContentExcerpt,
		highlight.TagNames,
	} {
		for _, fragment := range fragments {
			if strings.Contains(fragment, "<mark>") && strings.Contains(fragment, "</mark>") {
				return true
			}
		}
	}
	return false
}
