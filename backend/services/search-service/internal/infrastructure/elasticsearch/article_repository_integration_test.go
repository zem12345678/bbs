package elasticsearch

import (
	"context"
	"os"
	"testing"
	"time"

	domain "search-service/internal/domain/search"
)

func TestArticleRepositorySmoke(t *testing.T) {
	if os.Getenv("BBS_ES_SMOKE") != "1" {
		t.Skip("set BBS_ES_SMOKE=1 to run against local Elasticsearch")
	}

	ctx := context.Background()
	repo := NewArticleRepository([]string{"http://127.0.0.1:9200"}, "bbs_articles")
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

	time.Sleep(time.Second)
	hits, total, err := repo.SearchArticles(ctx, "forum search", 1, 10)
	if err != nil {
		t.Fatalf("search articles: %v", err)
	}
	if total == 0 || len(hits) == 0 {
		t.Fatalf("expected at least one hit, total=%d len=%d", total, len(hits))
	}
	var got domain.ArticleDocument
	for _, hit := range hits {
		if hit.Document.ID == doc.ID {
			got = hit.Document
			break
		}
	}
	if got.ID == 0 {
		t.Fatalf("indexed article not found in hits")
	}
	if got.CommentCount != 2 || got.LikeCount != 3 || got.FavoriteCount != 4 {
		t.Fatalf("counts comment=%d like=%d favorite=%d", got.CommentCount, got.LikeCount, got.FavoriteCount)
	}
}
