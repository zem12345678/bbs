package elasticsearch

import (
	"testing"

	domain "search-service/internal/domain/search"
)

func TestReindexBodiesDoNotOverwriteExistingCounters(t *testing.T) {
	article := domain.ArticleDocument{ID: 11, CommentCount: 4, LikeCount: 5, FavoriteCount: 6}
	articleUpdate := articleReindexBody(article)
	for _, key := range []string{"comment_count", "like_count", "favorite_count"} {
		if _, ok := articleUpdate[key]; ok {
			t.Fatalf("article update unexpectedly contains %q: %#v", key, articleUpdate)
		}
	}
	articleUpsert := articleIndexBody(article)
	if articleUpsert["comment_count"] != int64(4) || articleUpsert["like_count"] != int64(5) || articleUpsert["favorite_count"] != int64(6) {
		t.Fatalf("article upsert counters = %#v", articleUpsert)
	}

	topic := domain.TopicDocument{ID: 22, CommentCount: 7, LikeCount: 8, FavoriteCount: 9}
	topicUpdate := topicReindexBody(topic)
	for _, key := range []string{"comment_count", "like_count", "favorite_count"} {
		if _, ok := topicUpdate[key]; ok {
			t.Fatalf("topic update unexpectedly contains %q: %#v", key, topicUpdate)
		}
	}
	topicUpsert := topicIndexBody(topic)
	if topicUpsert["comment_count"] != int64(7) || topicUpsert["like_count"] != int64(8) || topicUpsert["favorite_count"] != int64(9) {
		t.Fatalf("topic upsert counters = %#v", topicUpsert)
	}
}
