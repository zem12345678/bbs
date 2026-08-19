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

func TestSearchByTagBuildsCrossIndexOROfANDQueryAndNumericCursors(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/bbs_articles,bbs_topics/_search" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = io.WriteString(w, `{"hits":{"hits":[{"_index":"bbs_articles","_source":{"id":"12","title":"Article","tag_names":["go","cloud"],"status":2,"created_at":200}},{"_index":"bbs_topics","_source":{"id":"11","title":"Topic","tag_names":["bbs"],"status":2,"created_at":100}}]}}`)
	}))
	defer server.Close()
	client, err := elastic.NewClient(elastic.Config{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	repo := NewArticleRepository(client, "bbs_articles", "bbs_topics")
	hits, err := repo.SearchByTag(t.Context(), domain.SearchByTagCriteria{
		Limit: 20, SinceID: 10, UntilID: 20,
		Query: []domain.TagQueryGroup{{Tags: []string{"go", "cloud"}}, {Tags: []string{"bbs"}}},
	})
	if err != nil {
		t.Fatalf("SearchByTag() error = %v", err)
	}
	if len(hits) != 2 || hits[0].Kind != domain.NoteLikeArticle || hits[0].Article == nil || hits[0].Article.ID != 12 || hits[1].Kind != domain.NoteLikeTopic || hits[1].Topic == nil || hits[1].Topic.ID != 11 {
		t.Fatalf("hits = %#v", hits)
	}
	if body["size"] != float64(20) {
		t.Fatalf("size = %#v", body["size"])
	}
	runtimeMappings := body["runtime_mappings"].(map[string]any)
	if _, ok := runtimeMappings["id_numeric"]; !ok {
		t.Fatalf("runtime mappings = %#v", runtimeMappings)
	}
	filters := body["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any)
	if len(filters) != 3 {
		t.Fatalf("filters = %#v", filters)
	}
	status := filters[0].(map[string]any)["term"].(map[string]any)["status"]
	if status != float64(2) {
		t.Fatalf("status filter = %#v", filters[0])
	}
	outer := filters[1].(map[string]any)["bool"].(map[string]any)
	if outer["minimum_should_match"] != float64(1) || len(outer["should"].([]any)) != 2 {
		t.Fatalf("tag OR filter = %#v", outer)
	}
	firstGroup := outer["should"].([]any)[0].(map[string]any)["bool"].(map[string]any)["filter"].([]any)
	if len(firstGroup) != 2 {
		t.Fatalf("tag AND filter = %#v", firstGroup)
	}
	rangeBounds := filters[2].(map[string]any)["range"].(map[string]any)["id_numeric"].(map[string]any)
	if rangeBounds["gt"] != float64(10) || rangeBounds["lt"] != float64(20) {
		t.Fatalf("cursor range = %#v", rangeBounds)
	}
	sort := body["sort"].([]any)
	if len(sort) != 3 {
		t.Fatalf("sort = %#v", sort)
	}
	for i, field := range []string{"id_numeric", "created_at", "_index"} {
		if _, ok := sort[i].(map[string]any)[field]; !ok {
			t.Fatalf("sort[%d] = %#v, want %s", i, sort[i], field)
		}
	}
}

func TestSearchByTagBuildsExactCaseInsensitiveTagFilter(t *testing.T) {
	body := captureSearchBody(t, "/bbs_articles,bbs_topics/_search", func(repo *ArticleRepository) error {
		_, err := repo.SearchByTag(t.Context(), domain.SearchByTagCriteria{Tag: "golang", Limit: 10})
		return err
	})
	filters := body["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any)
	term := filters[1].(map[string]any)["term"].(map[string]any)["tag_names.keyword"].(map[string]any)
	if term["value"] != "golang" || term["case_insensitive"] != true {
		t.Fatalf("tag term = %#v", term)
	}
}

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

func TestSearchUsersBuildsActiveKeywordAndFuzzyQuery(t *testing.T) {
	body := captureSearchBody(t, "/bbs_users_v2/_search", func(repo *ArticleRepository) error {
		_, _, err := repo.SearchUsers(t.Context(), "alcie", 2, 15)
		return err
	})

	assertSearchBody(t, body, "alcie", float64(15), float64(15), []any{"username^3", "nickname^2"})
	query := body["query"].(map[string]any)["bool"].(map[string]any)
	filters, ok := query["filter"].([]any)
	if !ok || len(filters) != 1 {
		t.Fatalf("filter = %#v", query["filter"])
	}
	term, ok := filters[0].(map[string]any)["term"].(map[string]any)
	if !ok || term["status"] != float64(1) {
		t.Fatalf("status filter = %#v", filters[0])
	}
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

func TestUserIndexBodyContainsOnlySearchProjection(t *testing.T) {
	body := userIndexBody(domain.UserDocument{
		ID:        42,
		Username:  "alice",
		Nickname:  "Alice",
		Status:    1,
		CreatedAt: 1000,
		UpdatedAt: 2000,
	})
	if len(body) != 6 {
		t.Fatalf("user index body = %#v", body)
	}
	for _, field := range []string{"id", "username", "nickname", "status", "created_at", "updated_at"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("user index body misses %q: %#v", field, body)
		}
	}
	for _, forbidden := range []string{"email", "avatar_url", "bio", "follower_count", "following_count", "background_url", "profile_theme"} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("user index body exposes %q: %#v", forbidden, body)
		}
	}
}

func TestEnsureUserIndexCreatesMinimalV2Mapping(t *testing.T) {
	var created map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case request.Method == http.MethodHead && request.URL.Path == "/bbs_users_v2":
			w.WriteHeader(http.StatusNotFound)
		case request.Method == http.MethodPut && request.URL.Path == "/bbs_users_v2":
			if err := json.NewDecoder(request.Body).Decode(&created); err != nil {
				t.Fatalf("decode mapping: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()

	client, err := elastic.NewClient(elastic.Config{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatalf("new elasticsearch client: %v", err)
	}
	if err := NewArticleRepository(client, "bbs_articles", "bbs_topics").EnsureUserIndex(t.Context()); err != nil {
		t.Fatalf("EnsureUserIndex() error = %v", err)
	}

	properties := created["mappings"].(map[string]any)["properties"].(map[string]any)
	if len(properties) != 6 {
		t.Fatalf("user mapping properties = %#v", properties)
	}
	for _, field := range []string{"id", "username", "nickname", "status", "created_at", "updated_at"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("user mapping misses %q: %#v", field, properties)
		}
	}
	for _, forbidden := range []string{"email", "avatar_url", "bio", "follower_count", "following_count", "background_url", "profile_theme"} {
		if _, ok := properties[forbidden]; ok {
			t.Fatalf("user mapping exposes %q: %#v", forbidden, properties)
		}
	}
}
