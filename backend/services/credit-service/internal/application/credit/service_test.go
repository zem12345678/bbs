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

func TestReverseQAAcceptanceReopensReservedBountyForReaccept(t *testing.T) {
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

	duplicate, err := svc.ReverseQAAcceptance(context.Background(), 10, 101, 9001, 22, 50, 0, "如何排查回调？", time.Now())
	if err != nil {
		t.Fatalf("reverse qa acceptance: %v", err)
	}
	if duplicate {
		t.Fatal("first reversal duplicate = true, want false")
	}
	reservation := repo.reservations[reservationKey(domain.CreditReservation{UserID: 10, Reason: QABountyReservationReason, SourceEventID: QABountyReservationEventID(101)})]
	if reservation.Status != CreditReservationStatusActive {
		t.Fatalf("reservation status = %q, want ACTIVE", reservation.Status)
	}
	if repo.balances[10] != 0 || repo.balances[22] != 0 {
		t.Fatalf("balances after reversal = asker:%d answerer:%d, want 0/0", repo.balances[10], repo.balances[22])
	}
	if last := repo.ledger[len(repo.ledger)-1]; last.Reason != QAAnswerUnacceptedReason || last.Delta != -50 || last.SourceEventID != QAAcceptanceReversalEventID(101, 9001, 0) {
		t.Fatalf("reversal ledger = %+v", last)
	}

	duplicate, err = svc.ReverseQAAcceptance(context.Background(), 10, 101, 9001, 22, 50, 0, "如何排查回调？", time.Now())
	if err != nil {
		t.Fatalf("duplicate reversal: %v", err)
	}
	if !duplicate {
		t.Fatal("duplicate reversal = false, want true")
	}
	if err := svc.HandleQAAcceptedWithCycle(context.Background(), "ignored", 101, "如何排查回调？", 10, 9001, 22, 50, 1, time.Now()); err != nil {
		t.Fatalf("reaccept same answer: %v", err)
	}
	if repo.balances[10] != 0 || repo.balances[22] != 50 {
		t.Fatalf("balances after reaccept = asker:%d answerer:%d, want 0/50", repo.balances[10], repo.balances[22])
	}
	if last := repo.ledger[len(repo.ledger)-1]; last.Reason != QAAnswerAcceptedReason || last.SourceEventID != QAAcceptedEventIDForCycle(101, 9001, 1) {
		t.Fatalf("reaccept reward ledger = %+v", last)
	}
}

func TestReverseQAAcceptanceRejectsInsufficientAnswererBalance(t *testing.T) {
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
	repo.balances[22] = 0

	_, err := svc.ReverseQAAcceptance(context.Background(), 10, 101, 9001, 22, 50, 0, "如何排查回调？", time.Now())
	if !errors.Is(err, domain.ErrInsufficientCredit) {
		t.Fatalf("err = %v, want insufficient credit", err)
	}
	reservation := repo.reservations[reservationKey(domain.CreditReservation{UserID: 10, Reason: QABountyReservationReason, SourceEventID: QABountyReservationEventID(101)})]
	if reservation.Status != CreditReservationStatusSettled {
		t.Fatalf("reservation status = %q, want SETTLED", reservation.Status)
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

func TestTransferCreditsWritesBalancedLedgersOnce(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	command := TransferCreditsCommand{
		PayerUserID:       42,
		PayeeUserID:       9,
		Amount:            7,
		DebitReason:       "attachment_download",
		DebitDescription:  "下载付费附件《guide.pdf》",
		CreditReason:      "attachment_sale",
		CreditDescription: "售卖付费附件《guide.pdf》",
		SourceEventID:     "attachment-download:101:42",
		SourceType:        "attachment",
		SourceID:          101,
	}

	if err := svc.TransferCredits(context.Background(), command); err != nil {
		t.Fatalf("transfer credits: %v", err)
	}
	if err := svc.TransferCredits(context.Background(), command); err != nil {
		t.Fatalf("duplicate transfer credits: %v", err)
	}
	if len(repo.ledger) != 2 {
		t.Fatalf("ledger entries = %d, want one debit and one credit", len(repo.ledger))
	}
	debit, credit := repo.ledger[0], repo.ledger[1]
	if debit.UserID != 42 || debit.Delta != -7 || debit.Reason != "attachment_download" || debit.SourceEventID != command.SourceEventID {
		t.Fatalf("debit ledger = %+v", debit)
	}
	if credit.UserID != 9 || credit.Delta != 7 || credit.Reason != "attachment_sale" || credit.SourceEventID != command.SourceEventID {
		t.Fatalf("credit ledger = %+v", credit)
	}
}

func TestTransferCreditsRejectsInvalidCommand(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	err := NewService(repo).TransferCredits(context.Background(), TransferCreditsCommand{
		PayerUserID:   42,
		PayeeUserID:   42,
		Amount:        7,
		DebitReason:   "attachment_download",
		CreditReason:  "attachment_sale",
		SourceEventID: "attachment-download:101:42",
	})
	if !errors.Is(err, domain.ErrInvalidCreditTransfer) {
		t.Fatalf("TransferCredits() error = %v, want invalid transfer", err)
	}
	if len(repo.ledger) != 0 {
		t.Fatalf("ledger entries = %d, want none", len(repo.ledger))
	}
}

func TestDailyCheckInAddsRewardOncePerShanghaiDay(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	occurredAt := time.Date(2026, time.July, 19, 14, 30, 0, 0, time.UTC)

	checkIn, ledger, balance, duplicate, err := svc.DailyCheckIn(context.Background(), 42, occurredAt)
	if err != nil {
		t.Fatalf("daily check-in: %v", err)
	}
	if duplicate {
		t.Fatal("first daily check-in duplicate = true, want false")
	}
	if checkIn.LatestDay != "2026-07-19" || checkIn.ConsecutiveDays != 1 {
		t.Fatalf("check-in = %+v, want July 19 with one-day streak", checkIn)
	}
	if ledger.Delta != DailyCheckInDelta || ledger.Reason != "daily_check_in" || ledger.SourceEventID != DailyCheckInEventID(42, "2026-07-19") {
		t.Fatalf("daily check-in ledger = %+v", ledger)
	}
	if balance.Total != DailyCheckInDelta || len(repo.ledger) != 1 {
		t.Fatalf("first daily check-in balance/ledger = %d/%d, want %d/1", balance.Total, len(repo.ledger), DailyCheckInDelta)
	}

	checkIn, ledger, balance, duplicate, err = svc.DailyCheckIn(context.Background(), 42, occurredAt.Add(20*time.Minute))
	if err != nil {
		t.Fatalf("duplicate daily check-in: %v", err)
	}
	if !duplicate {
		t.Fatal("second daily check-in duplicate = false, want true")
	}
	if checkIn.ConsecutiveDays != 1 || ledger.Delta != DailyCheckInDelta || balance.Total != DailyCheckInDelta || len(repo.ledger) != 1 {
		t.Fatalf("duplicate daily check-in state = checkIn:%+v ledger:%+v balance:%+v count:%d", checkIn, ledger, balance, len(repo.ledger))
	}
}

func TestDailyCheckInTracksShanghaiDateBoundariesAndStreaks(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if _, _, _, _, err := svc.DailyCheckIn(context.Background(), 42, time.Date(2026, time.July, 19, 15, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("first daily check-in: %v", err)
	}
	checkIn, _, balance, duplicate, err := svc.DailyCheckIn(context.Background(), 42, time.Date(2026, time.July, 19, 16, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("next Shanghai day check-in: %v", err)
	}
	if duplicate || checkIn.LatestDay != "2026-07-20" || checkIn.ConsecutiveDays != 2 || balance.Total != DailyCheckInDelta*2 {
		t.Fatalf("consecutive daily check-in = checkIn:%+v balance:%+v duplicate:%v", checkIn, balance, duplicate)
	}

	checkIn, _, balance, duplicate, err = svc.DailyCheckIn(context.Background(), 42, time.Date(2026, time.July, 21, 16, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("missed-day daily check-in: %v", err)
	}
	if duplicate || checkIn.LatestDay != "2026-07-22" || checkIn.ConsecutiveDays != 1 || balance.Total != DailyCheckInDelta*3 {
		t.Fatalf("reset daily check-in = checkIn:%+v balance:%+v duplicate:%v", checkIn, balance, duplicate)
	}
}

func TestGetCheckInStatusUsesShanghaiDay(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	status, checkedIn, err := svc.GetCheckInStatus(context.Background(), 42, time.Date(2026, time.July, 19, 15, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("get empty check-in status: %v", err)
	}
	if checkedIn || status.UserID != 42 || status.ConsecutiveDays != 0 {
		t.Fatalf("empty check-in status = %+v checked:%v", status, checkedIn)
	}

	if _, _, _, _, err := svc.DailyCheckIn(context.Background(), 42, time.Date(2026, time.July, 19, 15, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("daily check-in: %v", err)
	}
	status, checkedIn, err = svc.GetCheckInStatus(context.Background(), 42, time.Date(2026, time.July, 19, 15, 45, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("get completed check-in status: %v", err)
	}
	if !checkedIn || status.LatestDay != "2026-07-19" || status.ConsecutiveDays != 1 {
		t.Fatalf("completed check-in status = %+v checked:%v", status, checkedIn)
	}
}

func TestDailyCheckInTaskAwardsOnlyCompletedCheckInAndIsIdempotent(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	occurredAt := time.Date(2026, time.July, 19, 15, 30, 0, 0, time.UTC)

	status, err := svc.GetTaskClaimStatus(context.Background(), 42, 7, TaskKeyDailyCheckIn, occurredAt)
	if err != nil {
		t.Fatalf("get initial task status: %v", err)
	}
	if status.Completed || status.Claimed || status.Cycle != "2026-07-19" {
		t.Fatalf("initial task status = %+v", status)
	}

	_, _, _, _, err = svc.ClaimTask(context.Background(), 42, 7, TaskKeyDailyCheckIn, 12, "每日签到", occurredAt)
	if !errors.Is(err, domain.ErrTaskNotCompleted) {
		t.Fatalf("claim before check-in error = %v, want task not completed", err)
	}

	if _, _, _, _, err := svc.DailyCheckIn(context.Background(), 42, occurredAt); err != nil {
		t.Fatalf("daily check-in: %v", err)
	}
	status, err = svc.GetTaskClaimStatus(context.Background(), 42, 7, TaskKeyDailyCheckIn, occurredAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("get completed task status: %v", err)
	}
	if !status.Completed || status.Claimed {
		t.Fatalf("completed task status = %+v", status)
	}

	status, ledger, balance, duplicate, err := svc.ClaimTask(context.Background(), 42, 7, TaskKeyDailyCheckIn, 12, "每日签到", occurredAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("claim completed task: %v", err)
	}
	if duplicate || !status.Completed || !status.Claimed {
		t.Fatalf("first task claim status/duplicate = %+v/%v", status, duplicate)
	}
	if ledger.Delta != 12 || ledger.Reason != TaskDailyCheckInRewardReason || ledger.SourceEventID != TaskClaimEventID(42, 7, TaskKeyDailyCheckIn, "2026-07-19") {
		t.Fatalf("task claim ledger = %+v", ledger)
	}
	if balance.Total != DailyCheckInDelta+12 || len(repo.ledger) != 2 {
		t.Fatalf("task claim balance/ledger count = %d/%d", balance.Total, len(repo.ledger))
	}

	status, duplicateLedger, duplicateBalance, duplicate, err := svc.ClaimTask(context.Background(), 42, 7, TaskKeyDailyCheckIn, 99, "每日签到", occurredAt.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("duplicate task claim after reward update: %v", err)
	}
	if !duplicate || !status.Claimed || duplicateLedger.Delta != 12 || duplicateBalance.Total != DailyCheckInDelta+12 || len(repo.ledger) != 2 {
		t.Fatalf("duplicate task claim = status:%+v ledger:%+v balance:%+v duplicate:%v count:%d", status, duplicateLedger, duplicateBalance, duplicate, len(repo.ledger))
	}
}

func TestTaskClaimRejectsUnsupportedTask(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemoryRepo())
	_, err := svc.GetTaskClaimStatus(context.Background(), 42, 7, "first-topic", time.Now())
	if !errors.Is(err, domain.ErrUnsupportedTask) {
		t.Fatalf("unsupported task status error = %v, want unsupported task", err)
	}
	_, _, _, _, err = svc.ClaimTask(context.Background(), 42, 7, TaskKeyDailyCheckIn, 0, "每日签到", time.Now())
	if !errors.Is(err, domain.ErrInvalidTaskClaim) {
		t.Fatalf("zero reward task claim error = %v, want invalid task claim", err)
	}
}

func TestListTaskClaimStatusesUsesSingleSnapshotAndPreservesOrder(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	occurredAt := time.Date(2026, time.July, 20, 9, 0, 0, 0, time.UTC)
	repo.checkIns[42] = domain.CheckIn{UserID: 42, LatestDay: dailyCheckInDay(occurredAt)}
	if err := repo.SavePublishedTopic(context.Background(), domain.TopicPublicationRef{ID: 501, AuthorID: 42}, occurredAt); err != nil {
		t.Fatalf("save published topic: %v", err)
	}
	if err := repo.SaveCreatedComment(context.Background(), domain.CommentCreationRef{ID: 601, AuthorID: 42, EntityType: "topic", EntityID: 501}, occurredAt); err != nil {
		t.Fatalf("save created comment: %v", err)
	}
	claimedLookup := domain.TaskClaimLedgerLookup{
		SourceEventID: TaskClaimEventID(42, 9, TaskKeyFirstTopic, oneTimeTaskCycle),
		Reason:        TaskFirstTopicRewardReason,
	}
	repo.seen[ledgerKey(domain.LedgerEntry{UserID: 42, SourceEventID: claimedLookup.SourceEventID, Reason: claimedLookup.Reason})] = domain.LedgerEntry{
		UserID:        42,
		SourceEventID: claimedLookup.SourceEventID,
		Reason:        claimedLookup.Reason,
	}

	statuses, err := svc.ListTaskClaimStatuses(context.Background(), 42, []TaskClaimStatusInput{
		{TaskID: 8, TaskKey: TaskKeyDailyCheckIn},
		{TaskID: 9, TaskKey: TaskKeyFirstTopic},
		{TaskID: 10, TaskKey: TaskKeyFirstComment},
	}, occurredAt)
	if err != nil {
		t.Fatalf("list task claim statuses: %v", err)
	}
	if repo.taskClaimSnapshotCalls != 1 || len(repo.taskClaimSnapshotLookups) != 3 {
		t.Fatalf("task claim snapshot calls/lookups = %d/%d, want 1/3", repo.taskClaimSnapshotCalls, len(repo.taskClaimSnapshotLookups))
	}
	if len(statuses) != 3 {
		t.Fatalf("status count = %d, want 3", len(statuses))
	}
	if statuses[0].TaskID != 8 || statuses[0].TaskKey != TaskKeyDailyCheckIn || !statuses[0].Completed || statuses[0].Claimed {
		t.Fatalf("daily check-in status = %+v", statuses[0])
	}
	if statuses[1].TaskID != 9 || statuses[1].TaskKey != TaskKeyFirstTopic || !statuses[1].Completed || !statuses[1].Claimed {
		t.Fatalf("first topic status = %+v", statuses[1])
	}
	if statuses[2].TaskID != 10 || statuses[2].TaskKey != TaskKeyFirstComment || !statuses[2].Completed || statuses[2].Claimed {
		t.Fatalf("first comment status = %+v", statuses[2])
	}
}

func TestFirstTopicTaskRequiresPublishedTopicAndIsIdempotent(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	occurredAt := time.Date(2026, time.July, 20, 9, 0, 0, 0, time.UTC)

	status, err := svc.GetTaskClaimStatus(context.Background(), 42, 11, TaskKeyFirstTopic, occurredAt)
	if err != nil {
		t.Fatalf("get initial first topic status: %v", err)
	}
	if status.Completed || status.Claimed || status.Cycle != oneTimeTaskCycle {
		t.Fatalf("initial first topic status = %+v", status)
	}

	_, _, _, _, err = svc.ClaimTask(context.Background(), 42, 11, TaskKeyFirstTopic, 20, "发布第一条话题", occurredAt)
	if !errors.Is(err, domain.ErrTaskNotCompleted) {
		t.Fatalf("claim before publishing topic error = %v, want task not completed", err)
	}

	if err := svc.HandleTopicPublished(context.Background(), 501, 42, "首个话题", occurredAt); err != nil {
		t.Fatalf("project published topic: %v", err)
	}
	status, err = svc.GetTaskClaimStatus(context.Background(), 42, 11, TaskKeyFirstTopic, occurredAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("get completed first topic status: %v", err)
	}
	if !status.Completed || status.Claimed {
		t.Fatalf("completed first topic status = %+v", status)
	}

	status, ledger, balance, duplicate, err := svc.ClaimTask(context.Background(), 42, 11, TaskKeyFirstTopic, 20, "发布第一条话题", occurredAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("claim first topic task: %v", err)
	}
	if duplicate || !status.Completed || !status.Claimed {
		t.Fatalf("first topic claim status/duplicate = %+v/%v", status, duplicate)
	}
	if ledger.Delta != 20 || ledger.Reason != TaskFirstTopicRewardReason || ledger.SourceEventID != TaskClaimEventID(42, 11, TaskKeyFirstTopic, oneTimeTaskCycle) {
		t.Fatalf("first topic task ledger = %+v", ledger)
	}
	if balance.Total != 20 || len(repo.ledger) != 1 {
		t.Fatalf("first topic task balance/ledger count = %d/%d", balance.Total, len(repo.ledger))
	}

	status, duplicateLedger, duplicateBalance, duplicate, err := svc.ClaimTask(context.Background(), 42, 11, TaskKeyFirstTopic, 99, "发布第一条话题", occurredAt.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("duplicate first topic claim after reward update: %v", err)
	}
	if !duplicate || !status.Claimed || duplicateLedger.Delta != 20 || duplicateBalance.Total != 20 || len(repo.ledger) != 1 {
		t.Fatalf("duplicate first topic claim = status:%+v ledger:%+v balance:%+v duplicate:%v count:%d", status, duplicateLedger, duplicateBalance, duplicate, len(repo.ledger))
	}
}

func TestFirstCommentTaskRequiresCreatedCommentAndIsIdempotent(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)
	occurredAt := time.Date(2026, time.July, 20, 10, 0, 0, 0, time.UTC)

	status, err := svc.GetTaskClaimStatus(context.Background(), 42, 12, TaskKeyFirstComment, occurredAt)
	if err != nil {
		t.Fatalf("get initial first comment status: %v", err)
	}
	if status.Completed || status.Claimed || status.Cycle != oneTimeTaskCycle {
		t.Fatalf("initial first comment status = %+v", status)
	}

	_, _, _, _, err = svc.ClaimTask(context.Background(), 42, 12, TaskKeyFirstComment, 10, "完成一次评论", occurredAt)
	if !errors.Is(err, domain.ErrTaskNotCompleted) {
		t.Fatalf("claim before creating comment error = %v, want task not completed", err)
	}

	if err := svc.HandleCommentCreated(context.Background(), "comment-created:601", 601, "topic", 501, 42, occurredAt); err != nil {
		t.Fatalf("project created topic comment: %v", err)
	}
	status, err = svc.GetTaskClaimStatus(context.Background(), 42, 12, TaskKeyFirstComment, occurredAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("get completed first comment status: %v", err)
	}
	if !status.Completed || status.Claimed {
		t.Fatalf("completed first comment status = %+v", status)
	}

	status, ledger, balance, duplicate, err := svc.ClaimTask(context.Background(), 42, 12, TaskKeyFirstComment, 10, "完成一次评论", occurredAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("claim first comment task: %v", err)
	}
	if duplicate || !status.Completed || !status.Claimed {
		t.Fatalf("first comment claim status/duplicate = %+v/%v", status, duplicate)
	}
	if ledger.Delta != 10 || ledger.Reason != TaskFirstCommentRewardReason || ledger.SourceEventID != TaskClaimEventID(42, 12, TaskKeyFirstComment, oneTimeTaskCycle) {
		t.Fatalf("first comment task ledger = %+v", ledger)
	}
	if balance.Total != 10 || len(repo.ledger) != 1 {
		t.Fatalf("first comment task balance/ledger count = %d/%d", balance.Total, len(repo.ledger))
	}

	status, duplicateLedger, duplicateBalance, duplicate, err := svc.ClaimTask(context.Background(), 42, 12, TaskKeyFirstComment, 99, "完成一次评论", occurredAt.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("duplicate first comment claim after reward update: %v", err)
	}
	if !duplicate || !status.Claimed || duplicateLedger.Delta != 10 || duplicateBalance.Total != 10 || len(repo.ledger) != 1 {
		t.Fatalf("duplicate first comment claim = status:%+v ledger:%+v balance:%+v duplicate:%v count:%d", status, duplicateLedger, duplicateBalance, duplicate, len(repo.ledger))
	}
}

func TestListLeaderboardNormalizesLimit(t *testing.T) {
	repo := newMemoryRepo()
	repo.leaderboard = []domain.LeaderboardEntry{
		{UserID: 42, Total: 120, Rank: 1},
		{UserID: 7, Total: 95, Rank: 2},
	}
	svc := NewService(repo)

	items, err := svc.ListLeaderboard(context.Background(), 0)
	if err != nil {
		t.Fatalf("list default leaderboard: %v", err)
	}
	if repo.leaderboardLimit != LeaderboardDefaultLimit {
		t.Fatalf("default leaderboard limit = %d, want %d", repo.leaderboardLimit, LeaderboardDefaultLimit)
	}
	if len(items) != 2 || items[0].UserID != 42 || items[1].UserID != 7 {
		t.Fatalf("leaderboard items = %+v", items)
	}

	if _, err := svc.ListLeaderboard(context.Background(), LeaderboardMaxLimit+1); err != nil {
		t.Fatalf("list capped leaderboard: %v", err)
	}
	if repo.leaderboardLimit != LeaderboardMaxLimit {
		t.Fatalf("capped leaderboard limit = %d, want %d", repo.leaderboardLimit, LeaderboardMaxLimit)
	}
}

type memoryRepo struct {
	ledger                   []domain.LedgerEntry
	seen                     map[string]domain.LedgerEntry
	balances                 map[int64]int64
	reservations             map[string]domain.CreditReservation
	checkIns                 map[int64]domain.CheckIn
	publishedTopics          map[int64]domain.TopicPublicationRef
	createdComments          map[int64]domain.CommentCreationRef
	nextCheckInID            int64
	taskClaimSnapshotCalls   int
	taskClaimSnapshotLookups []domain.TaskClaimLedgerLookup
	debitErr                 error
	transferErr              error
	leaderboard              []domain.LeaderboardEntry
	leaderboardLimit         int32
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		seen:            map[string]domain.LedgerEntry{},
		balances:        map[int64]int64{},
		reservations:    map[string]domain.CreditReservation{},
		checkIns:        map[int64]domain.CheckIn{},
		publishedTopics: map[int64]domain.TopicPublicationRef{},
		createdComments: map[int64]domain.CommentCreationRef{},
	}
}

func (r *memoryRepo) EnsureSchema(context.Context) error { return nil }
func (r *memoryRepo) SaveArticle(context.Context, domain.ArticleRef, time.Time) error {
	return nil
}
func (r *memoryRepo) GetArticle(context.Context, int64) (domain.ArticleRef, error) {
	return domain.ArticleRef{}, nil
}
func (r *memoryRepo) SavePublishedTopic(_ context.Context, topic domain.TopicPublicationRef, _ time.Time) error {
	if topic.ID > 0 && topic.AuthorID > 0 {
		if _, exists := r.publishedTopics[topic.ID]; !exists {
			r.publishedTopics[topic.ID] = topic
		}
	}
	return nil
}
func (r *memoryRepo) HasPublishedTopic(_ context.Context, userID int64) (bool, error) {
	for _, topic := range r.publishedTopics {
		if topic.AuthorID == userID {
			return true, nil
		}
	}
	return false, nil
}
func (r *memoryRepo) SaveCreatedComment(_ context.Context, comment domain.CommentCreationRef, _ time.Time) error {
	if comment.ID > 0 && comment.AuthorID > 0 && comment.EntityID > 0 && (comment.EntityType == "article" || comment.EntityType == "topic") {
		if _, exists := r.createdComments[comment.ID]; !exists {
			r.createdComments[comment.ID] = comment
		}
	}
	return nil
}
func (r *memoryRepo) HasCreatedComment(_ context.Context, userID int64) (bool, error) {
	for _, comment := range r.createdComments {
		if comment.AuthorID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memoryRepo) GetTaskClaimSnapshot(_ context.Context, userID int64, lookups []domain.TaskClaimLedgerLookup) (domain.TaskClaimSnapshot, error) {
	r.taskClaimSnapshotCalls++
	r.taskClaimSnapshotLookups = append([]domain.TaskClaimLedgerLookup(nil), lookups...)
	snapshot := domain.TaskClaimSnapshot{
		ClaimedLedgerLookups: make(map[domain.TaskClaimLedgerLookup]bool, len(lookups)),
	}
	if checkIn, ok := r.checkIns[userID]; ok {
		snapshot.LatestCheckInDay = checkIn.LatestDay
	}
	for _, topic := range r.publishedTopics {
		if topic.AuthorID == userID {
			snapshot.HasPublishedTopic = true
			break
		}
	}
	for _, comment := range r.createdComments {
		if comment.AuthorID == userID {
			snapshot.HasCreatedComment = true
			break
		}
	}
	for _, lookup := range lookups {
		if _, ok := r.seen[ledgerKey(domain.LedgerEntry{UserID: userID, SourceEventID: lookup.SourceEventID, Reason: lookup.Reason})]; ok {
			snapshot.ClaimedLedgerLookups[lookup] = true
		}
	}
	return snapshot, nil
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
		return existing, domain.Balance{UserID: entry.UserID, Total: r.balances[entry.UserID]}, true, nil
	}
	if r.balances[entry.UserID]+entry.Delta < 0 {
		return domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrInsufficientCredit
	}
	r.balances[entry.UserID] += entry.Delta
	entry.BalanceAfter = r.balances[entry.UserID]
	r.seen[key] = entry
	r.ledger = append(r.ledger, entry)
	return entry, domain.Balance{UserID: entry.UserID, Total: r.balances[entry.UserID], UpdatedAt: entry.CreatedAt}, false, nil
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

func (r *memoryRepo) GetCheckIn(_ context.Context, userID int64) (domain.CheckIn, error) {
	if checkIn, ok := r.checkIns[userID]; ok {
		return checkIn, nil
	}
	return domain.CheckIn{UserID: userID}, nil
}

func (r *memoryRepo) RecordCheckIn(_ context.Context, requested domain.CheckIn, ledger domain.LedgerEntry) (domain.CheckIn, domain.LedgerEntry, domain.Balance, bool, error) {
	existing, exists := r.checkIns[requested.UserID]
	key := ledgerKey(ledger)
	if exists && existing.LatestDay == requested.LatestDay {
		existingLedger, ok := r.seen[key]
		if !ok {
			return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrCheckInStateMismatch
		}
		if err := validateMemoryLedger(existingLedger, ledger); err != nil {
			return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		return existing, existingLedger, domain.Balance{UserID: requested.UserID, Total: r.balances[requested.UserID]}, true, nil
	}
	if exists && requested.LatestDay < existing.LatestDay {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrCheckInDayRegression
	}
	if exists {
		previousDay, err := time.Parse(time.DateOnly, existing.LatestDay)
		if err != nil {
			return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrCheckInStateMismatch
		}
		if previousDay.AddDate(0, 0, 1).Format(time.DateOnly) == requested.LatestDay {
			requested.ConsecutiveDays = existing.ConsecutiveDays + 1
		} else {
			requested.ConsecutiveDays = 1
		}
		requested.ID = existing.ID
		requested.CreatedAt = existing.CreatedAt
	} else {
		r.nextCheckInID++
		requested.ID = r.nextCheckInID
		requested.ConsecutiveDays = 1
		requested.CreatedAt = ledger.CreatedAt
	}
	requested.UpdatedAt = ledger.CreatedAt
	r.checkIns[requested.UserID] = requested
	r.balances[requested.UserID] += ledger.Delta
	ledger.BalanceAfter = r.balances[requested.UserID]
	r.seen[key] = ledger
	r.ledger = append(r.ledger, ledger)
	return requested, ledger, domain.Balance{UserID: requested.UserID, Total: r.balances[requested.UserID], UpdatedAt: ledger.CreatedAt}, false, nil
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

func (r *memoryRepo) ReverseQAAcceptance(_ context.Context, reversal domain.QAAcceptanceReversal) (bool, error) {
	debit := domain.LedgerEntry{
		UserID:        reversal.AcceptedCommentAuthorID,
		Delta:         -reversal.Amount,
		Reason:        QAAnswerUnacceptedReason,
		SourceEventID: reversal.ReversalEventID,
		SourceType:    "comment",
		SourceID:      reversal.AcceptedCommentID,
	}
	key := ledgerKey(debit)
	if existing, ok := r.seen[key]; ok {
		if err := validateMemoryLedger(existing, debit); err != nil {
			return false, err
		}
		return true, nil
	}
	reservationKey := reservationKey(domain.CreditReservation{
		UserID:        reversal.QuestionAuthorID,
		Amount:        reversal.Amount,
		Reason:        QABountyReservationReason,
		SourceEventID: QABountyReservationEventID(reversal.TopicID),
		SourceType:    "topic",
		SourceID:      reversal.TopicID,
	})
	reservation, ok := r.reservations[reservationKey]
	if !ok || reservation.Status != CreditReservationStatusSettled {
		return false, domain.ErrQAAcceptanceSettlementPending
	}
	if r.balances[debit.UserID]+debit.Delta < 0 {
		return false, domain.ErrInsufficientCredit
	}
	r.balances[debit.UserID] += debit.Delta
	debit.BalanceAfter = r.balances[debit.UserID]
	r.seen[key] = debit
	r.ledger = append(r.ledger, debit)
	reservation.Status = CreditReservationStatusActive
	r.reservations[reservationKey] = reservation
	return false, nil
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

func (r *memoryRepo) GetBalance(_ context.Context, userID int64) (domain.Balance, error) {
	return domain.Balance{UserID: userID, Total: r.balances[userID]}, nil
}

func (r *memoryRepo) ListLedger(context.Context, int64, int32, int32) ([]domain.LedgerEntry, int64, domain.Balance, error) {
	return nil, 0, domain.Balance{}, nil
}

func (r *memoryRepo) ListLeaderboard(_ context.Context, limit int32) ([]domain.LeaderboardEntry, error) {
	r.leaderboardLimit = limit
	return append([]domain.LeaderboardEntry(nil), r.leaderboard...), nil
}

func (r *memoryRepo) GetLedgerEntry(_ context.Context, userID int64, sourceEventID, reason string) (domain.LedgerEntry, bool, error) {
	entry, ok := r.seen[ledgerKey(domain.LedgerEntry{UserID: userID, SourceEventID: sourceEventID, Reason: reason})]
	return entry, ok, nil
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
