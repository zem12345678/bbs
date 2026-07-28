package persistence

import (
	"testing"
	"time"
)

func TestSearchUserRowsIncludesFuzzyUsernameMatches(t *testing.T) {
	now := time.Now()
	rows := []userPO{
		{ID: 1, Username: "alice_dev", Nickname: "Alice Dev", Email: "alice@example.com", CreatedAt: now.Add(-time.Hour)},
		{ID: 2, Username: "bob", Nickname: "Bob", Email: "bob@example.com", CreatedAt: now},
	}

	got, total := searchUserRows(rows, "alcie", 1, 10)
	if total != 1 || len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("searchUserRows fuzzy result ids=%v total=%d, want alice only", userRowIDs(got), total)
	}
}

func TestSearchUserRowsKeepsExactMatchesBeforeFuzzyMatches(t *testing.T) {
	now := time.Now()
	rows := []userPO{
		{ID: 1, Username: "alcie", Nickname: "Typo User", Email: "typo@example.com", CreatedAt: now},
		{ID: 2, Username: "alice", Nickname: "Alice", Email: "alice@example.com", CreatedAt: now.Add(-time.Hour)},
	}

	got, total := searchUserRows(rows, "alice", 1, 10)
	if total != 2 || len(got) != 2 {
		t.Fatalf("searchUserRows result ids=%v total=%d, want two matches", userRowIDs(got), total)
	}
	if got[0].ID != 2 || got[1].ID != 1 {
		t.Fatalf("searchUserRows order ids=%v, want exact match first", userRowIDs(got))
	}
}

func TestSearchUserRowsDoesNotMatchEmail(t *testing.T) {
	now := time.Now()
	rows := []userPO{
		{ID: 1, Username: "alice", Nickname: "Alice", Email: "alice@example.com", CreatedAt: now},
	}

	got, total := searchUserRows(rows, "alice@example.com", 1, 10)
	if total != 0 || len(got) != 0 {
		t.Fatalf("searchUserRows email query ids=%v total=%d, want no public matches", userRowIDs(got), total)
	}
}

func TestSearchUserRowsPaginatesRankedMatches(t *testing.T) {
	now := time.Now()
	rows := []userPO{
		{ID: 1, Username: "alice", Nickname: "Alice", Email: "alice@example.com", CreatedAt: now.Add(-3 * time.Hour)},
		{ID: 2, Username: "alice_ops", Nickname: "Alice Ops", Email: "ops@example.com", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: 3, Username: "alice_admin", Nickname: "Alice Admin", Email: "admin@example.com", CreatedAt: now.Add(-time.Hour)},
	}

	got, total := searchUserRows(rows, "alice", 2, 2)
	if total != 3 || len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("searchUserRows page ids=%v total=%d, want second page with id 2", userRowIDs(got), total)
	}
}

func userRowIDs(rows []userPO) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}
