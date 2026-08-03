package elasticsearch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "search-service/internal/domain/search"

	elastic "github.com/elastic/go-elasticsearch/v9"
)

func TestEraseUserDataPurgesAllSearchProjectionsAndPreventsRevival(t *testing.T) {
	state := &erasureHTTPState{}
	server := httptest.NewServer(http.HandlerFunc(state.handle))
	defer server.Close()
	repo := newErasureTestRepository(t, server.URL)

	for attempt := 0; attempt < 2; attempt++ {
		if err := repo.EraseUserData(t.Context(), 42, 91, 3); err != nil {
			t.Fatalf("EraseUserData() attempt %d error = %v", attempt+1, err)
		}
	}
	if state.tombstoneWrites != 2 || state.userDeletes != 2 {
		t.Fatalf("tombstone writes=%d user deletes=%d", state.tombstoneWrites, state.userDeletes)
	}
	if len(state.authorPurges) != 4 {
		t.Fatalf("author purges=%v", state.authorPurges)
	}
	for _, purge := range state.authorPurges {
		if purge != "42" {
			t.Fatalf("author purge=%q, want 42", purge)
		}
	}

	if err := repo.IndexArticle(t.Context(), domain.ArticleDocument{ID: 101, AuthorID: 42}); err != nil {
		t.Fatalf("late article index: %v", err)
	}
	if err := repo.ReindexTopic(t.Context(), domain.TopicDocument{ID: 102, AuthorID: 42}); err != nil {
		t.Fatalf("late topic reindex: %v", err)
	}
	if err := repo.IndexUser(t.Context(), domain.UserDocument{ID: 42}); err != nil {
		t.Fatalf("late user index: %v", err)
	}
	if state.projectionWrites != 0 {
		t.Fatalf("late projection writes=%d, want 0", state.projectionWrites)
	}
	if state.raceCleanupDeletes != 2 || state.userDeletes != 3 {
		t.Fatalf("late cleanup deletes=%d user deletes=%d, want 2/3", state.raceCleanupDeletes, state.userDeletes)
	}
}

func TestIndexArticleRemovesProjectionWhenErasureRacesTheWrite(t *testing.T) {
	state := &erasureHTTPState{eraseAfterProjectionWrite: true, tombstoneIndexExists: true}
	server := httptest.NewServer(http.HandlerFunc(state.handle))
	defer server.Close()
	repo := newErasureTestRepository(t, server.URL)

	if err := repo.IndexArticle(t.Context(), domain.ArticleDocument{ID: 101, AuthorID: 42}); err != nil {
		t.Fatalf("IndexArticle() error = %v", err)
	}
	if state.projectionWrites != 1 || state.raceCleanupDeletes != 1 || !state.accountErased {
		t.Fatalf("writes=%d deletes=%d erased=%v", state.projectionWrites, state.raceCleanupDeletes, state.accountErased)
	}
}

func newErasureTestRepository(t *testing.T, address string) *ArticleRepository {
	t.Helper()
	client, err := elastic.NewClient(elastic.Config{Addresses: []string{address}})
	if err != nil {
		t.Fatalf("new elasticsearch client: %v", err)
	}
	return NewArticleRepository(client, "articles", "topics", "users", "account_tombstones")
}

type erasureHTTPState struct {
	tombstoneIndexExists      bool
	accountErased             bool
	eraseAfterProjectionWrite bool
	tombstoneWrites           int
	userDeletes               int
	authorPurges              []string
	projectionWrites          int
	raceCleanupDeletes        int
}

func (s *erasureHTTPState) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Elastic-Product", "Elasticsearch")
	switch {
	case r.Method == http.MethodHead && r.URL.Path == "/account_tombstones":
		if !s.tombstoneIndexExists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut && r.URL.Path == "/account_tombstones":
		s.tombstoneIndexExists = true
		_, _ = io.WriteString(w, `{"acknowledged":true}`)
	case r.Method == http.MethodPut && r.URL.Path == "/account_tombstones/_doc/42":
		if r.URL.Query().Get("refresh") != "wait_for" {
			http.Error(w, "missing tombstone refresh", http.StatusBadRequest)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["user_id"] != "42" || body["deletion_job_id"] != "91" || body["policy_version"] != float64(3) {
			http.Error(w, "invalid tombstone", http.StatusBadRequest)
			return
		}
		s.accountErased = true
		s.tombstoneWrites++
		_, _ = io.WriteString(w, `{"result":"created"}`)
	case r.Method == http.MethodGet && r.URL.Path == "/account_tombstones/_doc/42":
		if !s.accountErased {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"found":true}`)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/_delete_by_query"):
		var body struct {
			Query struct {
				Term map[string]string `json:"term"`
			} `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid query", http.StatusBadRequest)
			return
		}
		s.authorPurges = append(s.authorPurges, body.Query.Term["author_id"])
		_, _ = io.WriteString(w, `{"deleted":1}`)
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/_doc/") || r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_update/"):
		s.projectionWrites++
		if s.eraseAfterProjectionWrite {
			s.accountErased = true
		}
		_, _ = io.WriteString(w, `{"result":"updated"}`)
	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/_doc/"):
		if r.URL.Query().Get("refresh") != "wait_for" {
			http.Error(w, "missing delete refresh", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/users/_doc/42" && s.tombstoneWrites > 0 {
			s.userDeletes++
		} else {
			s.raceCleanupDeletes++
		}
		_, _ = io.WriteString(w, `{"result":"deleted"}`)
	default:
		http.Error(w, r.Method+" "+r.URL.String(), http.StatusInternalServerError)
	}
}
