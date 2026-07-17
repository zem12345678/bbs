package persistence

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "credit-service/internal/domain/credit"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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
