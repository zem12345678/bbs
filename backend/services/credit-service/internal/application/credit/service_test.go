package credit

import (
	"context"
	"fmt"
	"testing"
	"time"

	domain "credit-service/internal/domain/credit"
)

func TestHandleQAAcceptedAddsRewardCreditIdempotently(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	eventID := "content.qa.accepted:101:9001"

	if err := svc.HandleQAAccepted(context.Background(), eventID, 101, "如何排查回调？", 9001, 22, 10, time.Now()); err != nil {
		t.Fatalf("handle qa accepted: %v", err)
	}
	if err := svc.HandleQAAccepted(context.Background(), eventID, 101, "如何排查回调？", 9001, 22, 10, time.Now()); err != nil {
		t.Fatalf("handle duplicate qa accepted: %v", err)
	}

	if len(repo.ledger) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(repo.ledger))
	}
	entry := repo.ledger[0]
	if entry.UserID != 22 || entry.Delta != 10 || entry.Reason != "qa_answer_accepted" || entry.SourceEventID != eventID || entry.SourceType != "comment" || entry.SourceID != 9001 {
		t.Fatalf("ledger entry = %+v", entry)
	}
}

func TestHandleQAAcceptedUsesEventRewardCredits(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.HandleQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 9001, 22, 50, time.Now()); err != nil {
		t.Fatalf("handle qa accepted: %v", err)
	}

	if len(repo.ledger) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(repo.ledger))
	}
	if repo.ledger[0].Delta != 50 {
		t.Fatalf("reward delta = %d, want 50", repo.ledger[0].Delta)
	}
}

type memoryRepo struct {
	ledger []domain.LedgerEntry
	seen   map[string]struct{}
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{seen: map[string]struct{}{}}
}

func (r *memoryRepo) EnsureSchema(context.Context) error { return nil }
func (r *memoryRepo) SaveArticle(context.Context, domain.ArticleRef, time.Time) error {
	return nil
}
func (r *memoryRepo) GetArticle(context.Context, int64) (domain.ArticleRef, error) {
	return domain.ArticleRef{}, nil
}

func (r *memoryRepo) AddCredit(_ context.Context, entry domain.LedgerEntry) error {
	key := fmt.Sprintf("%d:%s:%s", entry.UserID, entry.SourceEventID, entry.Reason)
	if _, ok := r.seen[key]; ok {
		return nil
	}
	r.seen[key] = struct{}{}
	r.ledger = append(r.ledger, entry)
	return nil
}

func (r *memoryRepo) AdjustCredit(context.Context, domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	return domain.LedgerEntry{}, domain.Balance{}, false, nil
}

func (r *memoryRepo) DebitCredit(context.Context, domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	return domain.LedgerEntry{}, domain.Balance{}, false, nil
}

func (r *memoryRepo) SavePendingArticleCredit(context.Context, string, string, int64, int64, int64, string, int64, time.Time) error {
	return nil
}

func (r *memoryRepo) FlushPendingArticleCredits(context.Context, domain.ArticleRef) error {
	return nil
}

func (r *memoryRepo) GetBalance(context.Context, int64) (domain.Balance, error) {
	return domain.Balance{}, nil
}

func (r *memoryRepo) ListLedger(context.Context, int64, int32, int32) ([]domain.LedgerEntry, int64, domain.Balance, error) {
	return nil, 0, domain.Balance{}, nil
}

var _ domain.Repository = (*memoryRepo)(nil)
