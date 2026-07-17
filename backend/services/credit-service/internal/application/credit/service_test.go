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

func TestHandleQAAcceptedCanonicalizesEventID(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.HandleQAAccepted(context.Background(), "bus-envelope-1", 101, "如何排查回调？", 10, 9001, 22, 10, time.Now()); err != nil {
		t.Fatalf("handle qa accepted: %v", err)
	}
	if err := svc.HandleQAAccepted(context.Background(), "bus-envelope-2", 101, "如何排查回调？", 10, 9001, 22, 10, time.Now()); err != nil {
		t.Fatalf("handle duplicate qa accepted: %v", err)
	}

	if len(repo.ledger) != 2 {
		t.Fatalf("ledger entries = %d, want one canonical debit/reward pair", len(repo.ledger))
	}
	for _, entry := range repo.ledger {
		if entry.SourceEventID != QAAcceptedEventID(101, 9001) {
			t.Fatalf("ledger source event id = %q, want canonical accepted event id", entry.SourceEventID)
		}
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

func TestReserveCreditsDebitsAvailableBalanceIdempotently(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.balances[42] = 80
	svc := NewService(repo)
	sourceEventID := QABountyReservationEventID(101)

	reservation, balance, duplicate, err := svc.ReserveCredits(context.Background(), 42, 50, QABountyReservationReason, "问答悬赏冻结", sourceEventID, "topic", 101, time.Now())
	if err != nil {
		t.Fatalf("reserve credits: %v", err)
	}
	if duplicate {
		t.Fatal("first reserve duplicate = true, want false")
	}
	if reservation.UserID != 42 || reservation.Amount != 50 || reservation.Status != CreditReservationStatusActive || reservation.SourceEventID != sourceEventID {
		t.Fatalf("reservation = %+v", reservation)
	}
	if balance.Total != 30 || repo.balances[42] != 30 {
		t.Fatalf("balance = returned %d stored %d, want 30", balance.Total, repo.balances[42])
	}
	if len(repo.ledger) != 1 || repo.ledger[0].Delta != -50 || repo.ledger[0].Reason != QABountyReservationReason {
		t.Fatalf("reserve ledger = %+v", repo.ledger)
	}

	_, balance, duplicate, err = svc.ReserveCredits(context.Background(), 42, 50, QABountyReservationReason, "问答悬赏冻结", sourceEventID, "topic", 101, time.Now())
	if err != nil {
		t.Fatalf("duplicate reserve credits: %v", err)
	}
	if !duplicate {
		t.Fatal("second reserve duplicate = false, want true")
	}
	if balance.Total != 30 || len(repo.ledger) != 1 {
		t.Fatalf("duplicate reserve balance=%d ledger=%d, want balance 30 and one ledger", balance.Total, len(repo.ledger))
	}
}

func TestReserveCreditsRejectsMismatchedDuplicateReservation(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.balances[42] = 80
	svc := NewService(repo)
	sourceEventID := QABountyReservationEventID(101)

	if _, _, _, err := svc.ReserveCredits(context.Background(), 42, 50, QABountyReservationReason, "问答悬赏冻结", sourceEventID, "topic", 101, time.Now()); err != nil {
		t.Fatalf("reserve credits: %v", err)
	}
	_, _, duplicate, err := svc.ReserveCredits(context.Background(), 42, 30, QABountyReservationReason, "问答悬赏冻结", sourceEventID, "topic", 101, time.Now())
	if !errors.Is(err, domain.ErrCreditReservationMismatch) {
		t.Fatalf("mismatched duplicate reserve error = %v, want reservation mismatch", err)
	}
	if duplicate {
		t.Fatal("mismatched duplicate reserve duplicate = true, want false")
	}
	if repo.balances[42] != 30 || len(repo.ledger) != 1 {
		t.Fatalf("balance/ledger = %d/%d, want original reserve only", repo.balances[42], len(repo.ledger))
	}
}

func TestReleaseCreditsReturnsReservedBalanceIdempotently(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.balances[42] = 80
	svc := NewService(repo)
	sourceEventID := QABountyReservationEventID(101)

	if _, _, _, err := svc.ReserveCredits(context.Background(), 42, 50, QABountyReservationReason, "问答悬赏冻结", sourceEventID, "topic", 101, time.Now()); err != nil {
		t.Fatalf("reserve credits: %v", err)
	}
	reservation, balance, duplicate, err := svc.ReleaseCredits(context.Background(), 42, 50, QABountyReservationReason, QABountyReleaseReason, "问答悬赏返还", sourceEventID, "topic", 101, time.Now())
	if err != nil {
		t.Fatalf("release credits: %v", err)
	}
	if duplicate {
		t.Fatal("first release duplicate = true, want false")
	}
	if reservation.Status != CreditReservationStatusReleased {
		t.Fatalf("reservation status = %q, want RELEASED", reservation.Status)
	}
	if balance.Total != 80 || repo.balances[42] != 80 {
		t.Fatalf("balance = returned %d stored %d, want restored 80", balance.Total, repo.balances[42])
	}
	if len(repo.ledger) != 2 || repo.ledger[1].Delta != 50 || repo.ledger[1].Reason != QABountyReleaseReason {
		t.Fatalf("release ledger = %+v", repo.ledger)
	}

	_, balance, duplicate, err = svc.ReleaseCredits(context.Background(), 42, 50, QABountyReservationReason, QABountyReleaseReason, "问答悬赏返还", sourceEventID, "topic", 101, time.Now())
	if err != nil {
		t.Fatalf("duplicate release credits: %v", err)
	}
	if !duplicate {
		t.Fatal("second release duplicate = false, want true")
	}
	if balance.Total != 80 || len(repo.ledger) != 2 {
		t.Fatalf("duplicate release balance=%d ledger=%d, want balance 80 and two ledgers", balance.Total, len(repo.ledger))
	}
}

func TestHandleQAAcceptedSettlesReservedBountyWithoutSecondDebit(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.balances[10] = 50
	svc := NewService(repo)
	if _, _, _, err := svc.ReserveCredits(context.Background(), 10, 50, QABountyReservationReason, "问答悬赏冻结", QABountyReservationEventID(101), "topic", 101, time.Now()); err != nil {
		t.Fatalf("reserve credits: %v", err)
	}

	if err := svc.HandleQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 10, 9001, 22, 50, time.Now()); err != nil {
		t.Fatalf("handle qa accepted: %v", err)
	}

	if len(repo.ledger) != 2 {
		t.Fatalf("ledger entries = %d, want reserve and reward entries", len(repo.ledger))
	}
	if repo.ledger[0].Reason != QABountyReservationReason || repo.ledger[0].Delta != -50 {
		t.Fatalf("reserve ledger = %+v", repo.ledger[0])
	}
	reward := repo.ledger[1]
	if reward.UserID != 22 || reward.Delta != 50 || reward.Reason != "qa_answer_accepted" {
		t.Fatalf("reward ledger = %+v", reward)
	}
	if repo.balances[10] != 0 || repo.balances[22] != 50 {
		t.Fatalf("balances = asker:%d answerer:%d, want 0/50", repo.balances[10], repo.balances[22])
	}
}

func TestHandleQAAcceptedRejectsMismatchedReservedBounty(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.balances[10] = 50
	svc := NewService(repo)
	if _, _, _, err := svc.ReserveCredits(context.Background(), 10, 50, QABountyReservationReason, "问答悬赏冻结", QABountyReservationEventID(101), "topic", 101, time.Now()); err != nil {
		t.Fatalf("reserve credits: %v", err)
	}

	err := svc.HandleQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 10, 9001, 22, 10, time.Now())
	if !errors.Is(err, domain.ErrCreditReservationMismatch) {
		t.Fatalf("handle qa accepted error = %v, want reservation mismatch", err)
	}
	if len(repo.ledger) != 1 {
		t.Fatalf("ledger entries = %d, want only original reserve ledger", len(repo.ledger))
	}
	reservation := repo.reservations[reservationKey(domain.CreditReservation{
		UserID:        10,
		Reason:        QABountyReservationReason,
		SourceEventID: QABountyReservationEventID(101),
	})]
	if reservation.Status != CreditReservationStatusActive {
		t.Fatalf("reservation status = %q, want ACTIVE", reservation.Status)
	}
	if repo.balances[10] != 0 || repo.balances[22] != 0 {
		t.Fatalf("balances = asker:%d answerer:%d, want 0/0 after rejected mismatch", repo.balances[10], repo.balances[22])
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

func TestDebitCreditsRejectsMismatchedDuplicatePayment(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	sourceEventID := "mall.order.pay:811:pay-811"

	if _, _, _, err := svc.DebitCredits(context.Background(), 42, 120, "mall_order_paid", "兑换订单 ORD-811", sourceEventID, "mall_order", 811, time.Now()); err != nil {
		t.Fatalf("debit credits: %v", err)
	}
	_, _, duplicate, err := svc.DebitCredits(context.Background(), 42, 100, "mall_order_paid", "兑换订单 ORD-811", sourceEventID, "mall_order", 811, time.Now())
	if !errors.Is(err, domain.ErrCreditLedgerMismatch) {
		t.Fatalf("mismatched duplicate debit error = %v, want ledger mismatch", err)
	}
	if duplicate {
		t.Fatal("mismatched duplicate debit duplicate = true, want false")
	}
	if len(repo.ledger) != 1 {
		t.Fatalf("ledger entries = %d, want original debit only", len(repo.ledger))
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

func TestAdjustCreditsRejectsMismatchedDuplicateRefund(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	sourceEventID := "mall.refund:701"

	if _, _, _, err := svc.AdjustCredits(context.Background(), 42, 80, "mall_order_refund", "订单 ORD-701 售后退款", sourceEventID, "mall_refund", 701, time.Now()); err != nil {
		t.Fatalf("adjust credits: %v", err)
	}
	_, _, duplicate, err := svc.AdjustCredits(context.Background(), 42, 60, "mall_order_refund", "订单 ORD-701 售后退款", sourceEventID, "mall_refund", 701, time.Now())
	if !errors.Is(err, domain.ErrCreditLedgerMismatch) {
		t.Fatalf("mismatched duplicate adjust error = %v, want ledger mismatch", err)
	}
	if duplicate {
		t.Fatal("mismatched duplicate adjust duplicate = true, want false")
	}
	if len(repo.ledger) != 1 {
		t.Fatalf("ledger entries = %d, want original adjustment only", len(repo.ledger))
	}
}

type memoryRepo struct {
	ledger       []domain.LedgerEntry
	seen         map[string]domain.LedgerEntry
	balances     map[int64]int64
	reservations map[string]domain.CreditReservation
	debitErr     error
	transferErr  error
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		seen:         map[string]domain.LedgerEntry{},
		balances:     map[int64]int64{},
		reservations: map[string]domain.CreditReservation{},
	}
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
		if err := validateMemoryLedger(existing, entry); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
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
		if err := validateMemoryLedger(existing, entry); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		return existing, domain.Balance{}, true, nil
	}
	r.seen[key] = entry
	r.ledger = append(r.ledger, entry)
	return entry, domain.Balance{}, false, nil
}

func (r *memoryRepo) ReserveCredit(_ context.Context, reservation domain.CreditReservation, entry domain.LedgerEntry) (domain.CreditReservation, domain.Balance, bool, error) {
	key := reservationKey(reservation)
	if existing, ok := r.reservations[key]; ok {
		if err := validateMemoryReservation(existing, reservation); err != nil {
			return domain.CreditReservation{}, domain.Balance{}, false, err
		}
		return existing, domain.Balance{UserID: reservation.UserID, Total: r.balances[reservation.UserID]}, true, nil
	}
	if r.balances[reservation.UserID] < reservation.Amount {
		return domain.CreditReservation{}, domain.Balance{}, false, domain.ErrInsufficientCredit
	}
	r.balances[reservation.UserID] -= reservation.Amount
	reservation.ID = int64(len(r.reservations) + 1)
	reservation.Status = CreditReservationStatusActive
	r.reservations[key] = reservation
	entry.BalanceAfter = r.balances[reservation.UserID]
	r.seen[ledgerKey(entry)] = entry
	r.ledger = append(r.ledger, entry)
	return reservation, domain.Balance{UserID: reservation.UserID, Total: r.balances[reservation.UserID]}, false, nil
}

func (r *memoryRepo) ReleaseCredit(_ context.Context, reservation domain.CreditReservation, entry domain.LedgerEntry) (domain.CreditReservation, domain.Balance, bool, error) {
	key := reservationKey(reservation)
	existing, ok := r.reservations[key]
	if !ok {
		return domain.CreditReservation{}, domain.Balance{}, false, domain.ErrCreditReservationNotFound
	}
	if err := validateMemoryReservation(existing, reservation); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	if existing.Status == CreditReservationStatusReleased {
		return existing, domain.Balance{UserID: reservation.UserID, Total: r.balances[reservation.UserID]}, true, nil
	}
	if existing.Status != CreditReservationStatusActive {
		return domain.CreditReservation{}, domain.Balance{}, false, domain.ErrCreditReservationNotFound
	}
	r.balances[reservation.UserID] += existing.Amount
	entry.BalanceAfter = r.balances[reservation.UserID]
	r.seen[ledgerKey(entry)] = entry
	r.ledger = append(r.ledger, entry)
	existing.Status = CreditReservationStatusReleased
	r.reservations[key] = existing
	return existing, domain.Balance{UserID: reservation.UserID, Total: r.balances[reservation.UserID]}, false, nil
}

func (r *memoryRepo) SettleCreditReservation(_ context.Context, reservation domain.CreditReservation, credit domain.LedgerEntry) error {
	key := reservationKey(reservation)
	existing, ok := r.reservations[key]
	if !ok {
		return domain.ErrCreditReservationNotFound
	}
	if existing.Amount != credit.Delta {
		return domain.ErrCreditReservationMismatch
	}
	if existing.Status == CreditReservationStatusSettled {
		return nil
	}
	creditKey := ledgerKey(credit)
	if _, ok := r.seen[creditKey]; !ok {
		r.balances[credit.UserID] += credit.Delta
		credit.BalanceAfter = r.balances[credit.UserID]
		r.seen[creditKey] = credit
		r.ledger = append(r.ledger, credit)
	}
	existing.Status = CreditReservationStatusSettled
	r.reservations[key] = existing
	return nil
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

func reservationKey(reservation domain.CreditReservation) string {
	return fmt.Sprintf("%d:%s:%s", reservation.UserID, reservation.SourceEventID, reservation.Reason)
}

func validateMemoryLedger(existing domain.LedgerEntry, requested domain.LedgerEntry) error {
	if existing.Delta != requested.Delta ||
		(requested.SourceID > 0 && existing.SourceID != requested.SourceID) ||
		(requested.SourceType != "" && existing.SourceType != requested.SourceType) {
		return domain.ErrCreditLedgerMismatch
	}
	return nil
}

func validateMemoryReservation(existing domain.CreditReservation, requested domain.CreditReservation) error {
	if existing.Amount != requested.Amount ||
		(requested.SourceID > 0 && existing.SourceID != requested.SourceID) ||
		(requested.SourceType != "" && existing.SourceType != requested.SourceType) {
		return domain.ErrCreditReservationMismatch
	}
	return nil
}

var _ domain.Repository = (*memoryRepo)(nil)
