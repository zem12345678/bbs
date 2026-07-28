package elasticsearch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "search-service/internal/domain/search"

	elastic "github.com/elastic/go-elasticsearch/v9"
)

func TestSearchArticlesBuildsKeywordAndFuzzyQuery(t *testing.T) {
	body := captureSearchBody(t, "/bbs_articles/_search", func(repo *ArticleRepository) error {
		_, _, err := repo.SearchArticles(t.Context(), "codx", 2, 15)
		return err
	})

	assertSearchBody(t, body, "codx", float64(15), float64(15), []any{"title^3", "summary^2", "content_excerpt", "tag_names"})
}

func TestSearchTopicsBuildsKeywordAndFuzzyQuery(t *testing.T) {
	body := captureSearchBody(t, "/bbs_topics/_search", func(repo *ArticleRepository) error {
		_, _, err := repo.SearchTopics(t.Context(), "paymnt", 1, 20)
		return err
	})

	assertSearchBody(t, body, "paymnt", float64(0), float64(20), []any{"title^3", "content_excerpt", "tag_names"})
}

func captureSearchBody(t *testing.T, expectedPath string, search func(*ArticleRepository) error) map[string]any {
	t.Helper()
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != expectedPath {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = io.WriteString(w, `{"hits":{"total":{"value":0},"hits":[]}}`)
	}))
	defer server.Close()

	client, err := elastic.NewClient(elastic.Config{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatalf("new elasticsearch client: %v", err)
	}
	repo := NewArticleRepository(client, "bbs_articles", "bbs_topics")
	if err := search(repo); err != nil {
		t.Fatalf("search: %v", err)
	}
	return body
}

func assertSearchBody(t *testing.T, body map[string]any, keyword string, from float64, size float64, fields []any) {
	t.Helper()
	if body["from"] != from {
		t.Fatalf("from = %#v", body["from"])
	}
	if body["size"] != size {
		t.Fatalf("size = %#v", body["size"])
	}
	query, ok := body["query"].(map[string]any)
	if !ok {
		t.Fatalf("query = %#v", body["query"])
	}
	boolQuery, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatalf("bool query = %#v", query["bool"])
	}
	if boolQuery["minimum_should_match"] != float64(1) {
		t.Fatalf("minimum_should_match = %#v", boolQuery["minimum_should_match"])
	}
	should, ok := boolQuery["should"].([]any)
	if !ok || len(should) != 2 {
		t.Fatalf("should = %#v", boolQuery["should"])
	}
	exact := multiMatchClause(t, should[0])
	fuzzy := multiMatchClause(t, should[1])
	assertMultiMatch(t, exact, keyword, fields)
	assertMultiMatch(t, fuzzy, keyword, fields)
	if fuzzy["fuzziness"] != "AUTO" {
		t.Fatalf("fuzziness = %#v", fuzzy["fuzziness"])
	}
	if fuzzy["prefix_length"] != float64(1) {
		t.Fatalf("prefix_length = %#v", fuzzy["prefix_length"])
	}
	if fuzzy["max_expansions"] != float64(50) {
		t.Fatalf("max_expansions = %#v", fuzzy["max_expansions"])
	}
}

func multiMatchClause(t *testing.T, clause any) map[string]any {
	t.Helper()
	item, ok := clause.(map[string]any)
	if !ok {
		t.Fatalf("clause = %#v", clause)
	}
	multiMatch, ok := item["multi_match"].(map[string]any)
	if !ok {
		t.Fatalf("multi_match = %#v", item["multi_match"])
	}
	return multiMatch
}

func assertMultiMatch(t *testing.T, multiMatch map[string]any, keyword string, fields []any) {
	t.Helper()
	if multiMatch["query"] != keyword {
		t.Fatalf("query = %#v", multiMatch["query"])
	}
	got, ok := multiMatch["fields"].([]any)
	if !ok {
		t.Fatalf("fields = %#v", multiMatch["fields"])
	}
	if len(got) != len(fields) {
		t.Fatalf("fields len = %d, want %d", len(got), len(fields))
	}
	for i := range fields {
		if got[i] != fields[i] {
			t.Fatalf("fields[%d] = %#v, want %#v", i, got[i], fields[i])
		}
	}
}

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
