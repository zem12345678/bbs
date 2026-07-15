package credit

import (
	"context"
	"errors"
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

	if err := svc.HandleQAAccepted(context.Background(), eventID, 101, "如何排查回调？", 10, 9001, 22, 10, time.Now()); err != nil {
		t.Fatalf("handle qa accepted: %v", err)
	}
	if err := svc.HandleQAAccepted(context.Background(), eventID, 101, "如何排查回调？", 10, 9001, 22, 10, time.Now()); err != nil {
		t.Fatalf("handle duplicate qa accepted: %v", err)
	}

	if len(repo.ledger) != 2 {
		t.Fatalf("ledger entries = %d, want 2", len(repo.ledger))
	}
	debit := repo.ledger[0]
	if debit.UserID != 10 || debit.Delta != -10 || debit.Reason != "qa_bounty_paid" || debit.SourceEventID != eventID || debit.SourceType != "topic" || debit.SourceID != 101 {
		t.Fatalf("debit ledger entry = %+v", debit)
	}
	reward := repo.ledger[1]
	if reward.UserID != 22 || reward.Delta != 10 || reward.Reason != "qa_answer_accepted" || reward.SourceEventID != eventID || reward.SourceType != "comment" || reward.SourceID != 9001 {
		t.Fatalf("reward ledger entry = %+v", reward)
	}
}

func TestHandleQAAcceptedUsesEventRewardCredits(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.HandleQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 10, 9001, 22, 50, time.Now()); err != nil {
		t.Fatalf("handle qa accepted: %v", err)
	}

	if len(repo.ledger) != 2 {
		t.Fatalf("ledger entries = %d, want 2", len(repo.ledger))
	}
	if repo.ledger[0].Delta != -50 {
		t.Fatalf("bounty delta = %d, want -50", repo.ledger[0].Delta)
	}
	if repo.ledger[1].Delta != 50 {
		t.Fatalf("reward delta = %d, want 50", repo.ledger[1].Delta)
	}
}

func TestHandleQAAcceptedDoesNotRewardWhenBountyDebitFails(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.debitErr = errors.New("insufficient credit")
	svc := NewService(repo)

	err := svc.HandleQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 10, 9001, 22, 50, time.Now())
	if err == nil {
		t.Fatal("handle qa accepted error = nil, want debit failure")
	}
	if len(repo.ledger) != 0 {
		t.Fatalf("ledger entries = %d, want 0", len(repo.ledger))
	}
}

func TestHandleQAAcceptedDoesNotWritePartialLedgerWhenTransferFails(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.transferErr = errors.New("reward write failed")
	svc := NewService(repo)

	err := svc.HandleQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 10, 9001, 22, 50, time.Now())
	if err == nil {
		t.Fatal("handle qa accepted error = nil, want transfer failure")
	}
	if len(repo.ledger) != 0 {
		t.Fatalf("ledger entries = %d, want 0", len(repo.ledger))
	}
}

func TestHandleQAAcceptedSkipsSelfAcceptedAnswerReward(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.HandleQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 22, 9001, 22, 50, time.Now()); err != nil {
		t.Fatalf("handle qa accepted: %v", err)
	}
	if len(repo.ledger) != 0 {
		t.Fatalf("ledger entries = %d, want 0", len(repo.ledger))
	}
}

func TestDebitCreditsTreatsRepeatedMallOrderPaymentAsDuplicate(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	occurredAt := time.Date(2026, time.July, 16, 10, 30, 0, 0, time.UTC)
	sourceEventID := "mall.order.pay:811:pay-811"

	ledger, _, duplicate, err := svc.DebitCredits(context.Background(), 42, 120, "mall_order_paid", "兑换订单 ORD-811", sourceEventID, "mall_order", 811, occurredAt)
	if err != nil {
		t.Fatalf("debit credits: %v", err)
	}
	if duplicate {
		t.Fatal("first debit duplicate = true, want false")
	}
	if ledger.UserID != 42 || ledger.Delta != -120 || ledger.Reason != "mall_order_paid" || ledger.SourceEventID != sourceEventID {
		t.Fatalf("debit ledger = %+v", ledger)
	}

	ledger, _, duplicate, err = svc.DebitCredits(context.Background(), 42, 120, "mall_order_paid", "兑换订单 ORD-811", sourceEventID, "mall_order", 811, occurredAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("duplicate debit credits: %v", err)
	}
	if !duplicate {
		t.Fatal("second debit duplicate = false, want true")
	}
	if ledger.UserID != 42 || ledger.Delta != -120 || ledger.Reason != "mall_order_paid" || ledger.SourceEventID != sourceEventID {
		t.Fatalf("duplicate debit ledger = %+v", ledger)
	}
	if len(repo.ledger) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(repo.ledger))
	}
}

func TestAdjustCreditsTreatsRepeatedMallRefundAsDuplicate(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	occurredAt := time.Date(2026, time.July, 16, 10, 30, 0, 0, time.UTC)
	sourceEventID := "mall.refund:701"

	ledger, _, duplicate, err := svc.AdjustCredits(context.Background(), 42, 80, "mall_order_refund", "订单 ORD-701 售后退款", sourceEventID, "mall_refund", 701, occurredAt)
	if err != nil {
		t.Fatalf("adjust credits: %v", err)
	}
	if duplicate {
		t.Fatal("first adjust duplicate = true, want false")
	}
	if ledger.UserID != 42 || ledger.Delta != 80 || ledger.Reason != "mall_order_refund" || ledger.SourceEventID != sourceEventID {
		t.Fatalf("adjust ledger = %+v", ledger)
	}

	ledger, _, duplicate, err = svc.AdjustCredits(context.Background(), 42, 80, "mall_order_refund", "订单 ORD-701 售后退款", sourceEventID, "mall_refund", 701, occurredAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("duplicate adjust credits: %v", err)
	}
	if !duplicate {
		t.Fatal("second adjust duplicate = false, want true")
	}
	if ledger.UserID != 42 || ledger.Delta != 80 || ledger.Reason != "mall_order_refund" || ledger.SourceEventID != sourceEventID {
		t.Fatalf("duplicate adjust ledger = %+v", ledger)
	}
	if len(repo.ledger) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(repo.ledger))
	}
}

type memoryRepo struct {
	ledger      []domain.LedgerEntry
	seen        map[string]domain.LedgerEntry
	debitErr    error
	transferErr error
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{seen: map[string]domain.LedgerEntry{}}
}

func (r *memoryRepo) EnsureSchema(context.Context) error { return nil }
func (r *memoryRepo) SaveArticle(context.Context, domain.ArticleRef, time.Time) error {
	return nil
}
func (r *memoryRepo) GetArticle(context.Context, int64) (domain.ArticleRef, error) {
	return domain.ArticleRef{}, nil
}

func (r *memoryRepo) AddCredit(_ context.Context, entry domain.LedgerEntry) error {
	key := ledgerKey(entry)
	if _, ok := r.seen[key]; ok {
		return nil
	}
	r.seen[key] = entry
	r.ledger = append(r.ledger, entry)
	return nil
}

func (r *memoryRepo) AdjustCredit(_ context.Context, entry domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	key := ledgerKey(entry)
	if existing, ok := r.seen[key]; ok {
		return existing, domain.Balance{}, true, nil
	}
	r.seen[key] = entry
	r.ledger = append(r.ledger, entry)
	return entry, domain.Balance{}, false, nil
}

func (r *memoryRepo) DebitCredit(_ context.Context, entry domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	if r.debitErr != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, r.debitErr
	}
	key := ledgerKey(entry)
	if existing, ok := r.seen[key]; ok {
		return existing, domain.Balance{}, true, nil
	}
	r.seen[key] = entry
	r.ledger = append(r.ledger, entry)
	return entry, domain.Balance{}, false, nil
}

func (r *memoryRepo) TransferCredit(_ context.Context, debit domain.LedgerEntry, credit domain.LedgerEntry) error {
	if r.transferErr != nil {
		return r.transferErr
	}
	if r.debitErr != nil {
		return r.debitErr
	}
	for _, entry := range []domain.LedgerEntry{debit, credit} {
		key := ledgerKey(entry)
		if _, ok := r.seen[key]; ok {
			continue
		}
		r.seen[key] = entry
		r.ledger = append(r.ledger, entry)
	}
	return nil
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

func ledgerKey(entry domain.LedgerEntry) string {
	return fmt.Sprintf("%d:%s:%s", entry.UserID, entry.SourceEventID, entry.Reason)
}

var _ domain.Repository = (*memoryRepo)(nil)
