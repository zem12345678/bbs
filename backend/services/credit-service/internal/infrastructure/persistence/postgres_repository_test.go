package persistence

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "credit-service/internal/domain/credit"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLockCreditUsersUsesStableSortedOrder(t *testing.T) {
	db := &creditUserLockQueryer{}
	if err := lockCreditUsers(context.Background(), db, 42, 7, 42, -3, 0); err != nil {
		t.Fatalf("lockCreditUsers() error = %v", err)
	}
	want := []int64{-3, 7, 42}
	if !reflect.DeepEqual(db.userIDs, want) {
		t.Fatalf("locked user IDs = %v, want %v", db.userIDs, want)
	}
}

func TestNormalizePostgresErrorMapsAccountErasureTrigger(t *testing.T) {
	err := fmt.Errorf("insert credit ledger: %w", &pgconn.PgError{Code: "P0001", Message: "credit account erased"})
	if got := normalizePostgresError(err); !errors.Is(got, domain.ErrAccountErased) {
		t.Fatalf("normalizePostgresError() = %v, want ErrAccountErased", got)
	}

	other := &pgconn.PgError{Code: "P0001", Message: "other trigger failure"}
	if got := normalizePostgresError(other); got != other {
		t.Fatalf("normalizePostgresError() changed unrelated error: %v", got)
	}

	row := normalizedRow{Row: creditScanRow{err: err}}
	if got := row.Scan(); !errors.Is(got, domain.ErrAccountErased) {
		t.Fatalf("normalizedRow.Scan() = %v, want ErrAccountErased", got)
	}
}

func TestValidateTransferLedgerStateRejectsPartialTransfer(t *testing.T) {
	tests := []struct {
		name         string
		debitExists  bool
		creditExists bool
		wantErr      bool
	}{
		{name: "new transfer", debitExists: false, creditExists: false},
		{name: "idempotent transfer", debitExists: true, creditExists: true},
		{name: "debit only", debitExists: true, creditExists: false, wantErr: true},
		{name: "credit only", debitExists: false, creditExists: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransferLedgerState(tt.debitExists, tt.creditExists)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInconsistentCreditTransfer) {
					t.Fatalf("validateTransferLedgerState() error = %v, want inconsistent transfer", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTransferLedgerState() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateTransferBalanceRejectsUnbalancedTransfer(t *testing.T) {
	tests := []struct {
		name    string
		debit   int64
		credit  int64
		wantErr bool
	}{
		{name: "balanced", debit: -50, credit: 50},
		{name: "over credit", debit: -50, credit: 80, wantErr: true},
		{name: "under credit", debit: -50, credit: 30, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransferBalance(
				domain.LedgerEntry{Delta: tt.debit},
				domain.LedgerEntry{Delta: tt.credit},
			)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrUnbalancedCreditTransfer) {
					t.Fatalf("validateTransferBalance() error = %v, want unbalanced transfer", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTransferBalance() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateDuplicateLedgerRequiresMatchingMutation(t *testing.T) {
	existing := domain.LedgerEntry{Delta: -120, SourceType: "mall_order", SourceID: 811}
	tests := []struct {
		name      string
		requested domain.LedgerEntry
		wantErr   bool
	}{
		{name: "same mutation", requested: domain.LedgerEntry{Delta: -120, SourceType: "mall_order", SourceID: 811}},
		{name: "different amount", requested: domain.LedgerEntry{Delta: -100, SourceType: "mall_order", SourceID: 811}, wantErr: true},
		{name: "different source type", requested: domain.LedgerEntry{Delta: -120, SourceType: "mall_refund", SourceID: 811}, wantErr: true},
		{name: "different source id", requested: domain.LedgerEntry{Delta: -120, SourceType: "mall_order", SourceID: 812}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDuplicateLedger(existing, tt.requested)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrCreditLedgerMismatch) {
					t.Fatalf("validateDuplicateLedger() error = %v, want ledger mismatch", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateDuplicateLedger() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateDuplicateReservationRequiresMatchingReservation(t *testing.T) {
	existing := domain.CreditReservation{Amount: 50, SourceType: "topic", SourceID: 101}
	tests := []struct {
		name      string
		requested domain.CreditReservation
		wantErr   bool
	}{
		{name: "same reservation", requested: domain.CreditReservation{Amount: 50, SourceType: "topic", SourceID: 101}},
		{name: "different amount", requested: domain.CreditReservation{Amount: 30, SourceType: "topic", SourceID: 101}, wantErr: true},
		{name: "different source type", requested: domain.CreditReservation{Amount: 50, SourceType: "mall_order", SourceID: 101}, wantErr: true},
		{name: "different source id", requested: domain.CreditReservation{Amount: 50, SourceType: "topic", SourceID: 102}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDuplicateReservation(existing, tt.requested)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrCreditReservationMismatch) {
					t.Fatalf("validateDuplicateReservation() error = %v, want reservation mismatch", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateDuplicateReservation() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateReservationSettlementRequiresExactAmount(t *testing.T) {
	tests := []struct {
		name      string
		existing  domain.CreditReservation
		requested domain.CreditReservation
		credit    domain.LedgerEntry
		wantErr   bool
	}{
		{
			name:      "exact amount and source",
			existing:  domain.CreditReservation{Amount: 50, SourceID: 101},
			requested: domain.CreditReservation{SourceID: 101},
			credit:    domain.LedgerEntry{Delta: 50},
		},
		{
			name:      "answer reward ledger may use comment source",
			existing:  domain.CreditReservation{Amount: 50, SourceType: "topic", SourceID: 101},
			requested: domain.CreditReservation{SourceType: "topic", SourceID: 101},
			credit:    domain.LedgerEntry{Delta: 50, SourceType: "comment", SourceID: 9001},
		},
		{
			name:      "smaller reward leaves reserved credit unbalanced",
			existing:  domain.CreditReservation{Amount: 50, SourceID: 101},
			requested: domain.CreditReservation{SourceID: 101},
			credit:    domain.LedgerEntry{Delta: 10},
			wantErr:   true,
		},
		{
			name:      "larger reward exceeds reserved credit",
			existing:  domain.CreditReservation{Amount: 50, SourceID: 101},
			requested: domain.CreditReservation{SourceID: 101},
			credit:    domain.LedgerEntry{Delta: 80},
			wantErr:   true,
		},
		{
			name:      "source mismatch",
			existing:  domain.CreditReservation{Amount: 50, SourceID: 101},
			requested: domain.CreditReservation{SourceID: 102},
			credit:    domain.LedgerEntry{Delta: 50},
			wantErr:   true,
		},
		{
			name:      "source type mismatch",
			existing:  domain.CreditReservation{Amount: 50, SourceType: "topic", SourceID: 101},
			requested: domain.CreditReservation{SourceType: "mall_order", SourceID: 101},
			credit:    domain.LedgerEntry{Delta: 50, SourceType: "topic"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReservationSettlement(tt.existing, tt.requested, tt.credit)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrCreditReservationMismatch) {
					t.Fatalf("validateReservationSettlement() error = %v, want mismatch", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateReservationSettlement() error = %v, want nil", err)
			}
		})
	}
}

func TestNextCheckInStreakPreservesOnlyConsecutiveShanghaiDays(t *testing.T) {
	tests := []struct {
		name       string
		latestDay  string
		requested  string
		current    int32
		wantStreak int32
		wantErr    error
	}{
		{name: "next day extends streak", latestDay: "2026-07-19", requested: "2026-07-20", current: 3, wantStreak: 4},
		{name: "missed day resets streak", latestDay: "2026-07-19", requested: "2026-07-21", current: 3, wantStreak: 1},
		{name: "same day needs an existing ledger", latestDay: "2026-07-19", requested: "2026-07-19", current: 3, wantErr: domain.ErrCheckInStateMismatch},
		{name: "older day is rejected", latestDay: "2026-07-20", requested: "2026-07-19", current: 3, wantErr: domain.ErrCheckInDayRegression},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streak, err := nextCheckInStreak(tt.latestDay, tt.requested, tt.current)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("nextCheckInStreak() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("nextCheckInStreak() error = %v", err)
			}
			if streak != tt.wantStreak {
				t.Fatalf("nextCheckInStreak() = %d, want %d", streak, tt.wantStreak)
			}
		})
	}
}

func TestLockBalanceBeforeLedgerLookupLocksBalanceBeforeLedgerLookup(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	db := &creditMutationQueryer{
		balance:   domain.Balance{UserID: 7, Total: 120, UpdatedAt: now},
		ledgerErr: pgx.ErrNoRows,
	}

	balance, _, duplicate, err := lockBalanceBeforeLedgerLookup(context.Background(), db, 7, "mall.order.pay:1", "mall_order_paid", true)
	if err != nil {
		t.Fatalf("lockBalanceBeforeLedgerLookup() error = %v", err)
	}
	if duplicate {
		t.Fatal("lockBalanceBeforeLedgerLookup() duplicate = true, want false")
	}
	if balance.Total != 120 {
		t.Fatalf("balance total = %d, want 120", balance.Total)
	}
	wantOps := []string{"ensure_balance", "balance_for_update", "ledger_by_event"}
	if !reflect.DeepEqual(db.ops, wantOps) {
		t.Fatalf("ops = %+v, want %+v", db.ops, wantOps)
	}
}

func TestLockBalanceBeforeLedgerLookupReturnsExistingLedgerAfterLock(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 30, 0, 0, time.UTC)
	db := &creditMutationQueryer{
		balance: domain.Balance{UserID: 7, Total: 90, UpdatedAt: now},
		ledger: domain.LedgerEntry{
			ID:            501,
			UserID:        7,
			Delta:         -30,
			BalanceAfter:  90,
			Reason:        "mall_order_paid",
			Description:   "order paid",
			SourceEventID: "mall.order.pay:1",
			SourceType:    "mall_order",
			SourceID:      9001,
			CreatedAt:     now,
		},
	}

	balance, ledger, duplicate, err := lockBalanceBeforeLedgerLookup(context.Background(), db, 7, "mall.order.pay:1", "mall_order_paid", false)
	if err != nil {
		t.Fatalf("lockBalanceBeforeLedgerLookup() error = %v", err)
	}
	if !duplicate {
		t.Fatal("lockBalanceBeforeLedgerLookup() duplicate = false, want true")
	}
	if balance.Total != 90 || ledger.ID != 501 {
		t.Fatalf("balance/ledger = %+v/%+v, want existing duplicate state", balance, ledger)
	}
	wantOps := []string{"balance_for_update", "ledger_by_event"}
	if !reflect.DeepEqual(db.ops, wantOps) {
		t.Fatalf("ops = %+v, want %+v", db.ops, wantOps)
	}
}

type creditMutationQueryer struct {
	ops       []string
	balance   domain.Balance
	ledger    domain.LedgerEntry
	ledgerErr error
}

type creditUserLockQueryer struct {
	userIDs []int64
}

func (q *creditUserLockQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if query != creditUserAdvisoryLockSQL || len(args) != 1 {
		return pgconn.CommandTag{}, errors.New("unexpected advisory lock query")
	}
	userID, ok := args[0].(int64)
	if !ok {
		return pgconn.CommandTag{}, errors.New("unexpected advisory lock user ID")
	}
	q.userIDs = append(q.userIDs, userID)
	return pgconn.CommandTag{}, nil
}

func (*creditUserLockQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return creditScanRow{err: errors.New("unexpected query row")}
}

func (q *creditMutationQueryer) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "INSERT INTO credit_balances") {
		q.ops = append(q.ops, "ensure_balance")
	}
	return pgconn.CommandTag{}, nil
}

func (q *creditMutationQueryer) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(query, "FROM credit_balances"):
		q.ops = append(q.ops, "balance_for_update")
		return creditScanRow{values: []any{q.balance.UserID, q.balance.Total, q.balance.UpdatedAt}}
	case strings.Contains(query, "FROM credit_ledger"):
		q.ops = append(q.ops, "ledger_by_event")
		if q.ledgerErr != nil {
			return creditScanRow{err: q.ledgerErr}
		}
		return creditScanRow{values: []any{
			q.ledger.ID,
			q.ledger.UserID,
			q.ledger.Delta,
			q.ledger.BalanceAfter,
			q.ledger.Reason,
			q.ledger.Description,
			q.ledger.SourceEventID,
			q.ledger.SourceType,
			q.ledger.SourceID,
			q.ledger.CreatedAt,
		}}
	default:
		return creditScanRow{err: errors.New("unexpected query")}
	}
}

type creditScanRow struct {
	values []any
	err    error
}

func (r creditScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("scan destination count mismatch")
	}
	for i, value := range r.values {
		switch d := dest[i].(type) {
		case *int64:
			v, ok := value.(int64)
			if !ok {
				return errors.New("expected int64 scan value")
			}
			*d = v
		case *string:
			v, ok := value.(string)
			if !ok {
				return errors.New("expected string scan value")
			}
			*d = v
		case *time.Time:
			v, ok := value.(time.Time)
			if !ok {
				return errors.New("expected time scan value")
			}
			*d = v
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
