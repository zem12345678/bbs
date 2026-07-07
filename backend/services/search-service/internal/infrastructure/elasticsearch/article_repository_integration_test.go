package elasticsearch

import (
	"context"
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
