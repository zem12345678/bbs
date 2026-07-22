package persistence

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestValidatePaymentForOrder(t *testing.T) {
	order := domain.Order{ID: 11, UserID: 7, TotalCredits: 80}
	valid := domain.Payment{OrderID: 11, UserID: 7, AmountCredits: 80}
	if err := validatePaymentForOrder(valid, order, 7); err != nil {
		t.Fatalf("validatePaymentForOrder() error = %v", err)
	}

	for _, payment := range []domain.Payment{
		{OrderID: 12, UserID: 7, AmountCredits: 80},
		{OrderID: 11, UserID: 8, AmountCredits: 80},
		{OrderID: 11, UserID: 7, AmountCredits: 81},
	} {
		if err := validatePaymentForOrder(payment, order, 7); !errors.Is(err, domain.ErrInvalidOrderState) {
			t.Fatalf("validatePaymentForOrder(%+v) error = %v, want invalid order state", payment, err)
		}
	}
}

func TestCanCompleteOrderPaymentRequiresMatchingPaymentState(t *testing.T) {
	tests := []struct {
		name          string
		orderStatus   domain.OrderStatus
		paymentStatus domain.PaymentStatus
		want          bool
	}{
		{
			name:          "paying order with pending payment",
			orderStatus:   domain.OrderStatusPaying,
			paymentStatus: domain.PaymentStatusPending,
			want:          true,
		},
		{
			name:          "paid order with succeeded payment replay",
			orderStatus:   domain.OrderStatusPaid,
			paymentStatus: domain.PaymentStatusSucceeded,
			want:          true,
		},
		{
			name:          "completed order with succeeded payment replay",
			orderStatus:   domain.OrderStatusCompleted,
			paymentStatus: domain.PaymentStatusSucceeded,
			want:          true,
		},
		{
			name:          "paid order rejects pending payment replay",
			orderStatus:   domain.OrderStatusPaid,
			paymentStatus: domain.PaymentStatusPending,
			want:          false,
		},
		{
			name:          "completed order rejects failed payment replay",
			orderStatus:   domain.OrderStatusCompleted,
			paymentStatus: domain.PaymentStatusFailed,
			want:          false,
		},
		{
			name:          "paying order rejects already failed payment",
			orderStatus:   domain.OrderStatusPaying,
			paymentStatus: domain.PaymentStatusFailed,
			want:          false,
		},
		{
			name:          "pending order cannot complete payment",
			orderStatus:   domain.OrderStatusPendingPayment,
			paymentStatus: domain.PaymentStatusPending,
			want:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canCompleteOrderPayment(tt.orderStatus, tt.paymentStatus); got != tt.want {
				t.Fatalf("canCompleteOrderPayment(%q, %q) = %v, want %v", tt.orderStatus, tt.paymentStatus, got, tt.want)
			}
		})
	}
}

func TestGetPendingPaymentForOrderScopesLookupToCurrentAttempt(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	queryer := &pendingPaymentQueryer{payment: domain.Payment{
		ID:             501,
		OrderID:        601,
		UserID:         7,
		AmountCredits:  120,
		Provider:       domain.PaymentProviderCredits,
		IdempotencyKey: "original-payment-attempt",
		Status:         domain.PaymentStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}

	payment, err := getPendingPaymentForOrder(context.Background(), queryer, 601, 7, domain.PaymentProviderCredits)
	if err != nil {
		t.Fatalf("getPendingPaymentForOrder() error = %v", err)
	}
	if payment.IdempotencyKey != "original-payment-attempt" {
		t.Fatalf("payment idempotency key = %q, want original payment attempt", payment.IdempotencyKey)
	}
	for _, want := range []string{"WHERE order_id = $1", "AND user_id = $2", "AND provider = $3", "AND status = $4", "ORDER BY created_at ASC, id ASC"} {
		if !strings.Contains(queryer.query, want) {
			t.Fatalf("payment lookup query missing %q: %s", want, queryer.query)
		}
	}
	wantArgs := []any{int64(601), int64(7), string(domain.PaymentProviderCredits), string(domain.PaymentStatusPending)}
	if len(queryer.args) != len(wantArgs) {
		t.Fatalf("payment lookup args = %#v, want %#v", queryer.args, wantArgs)
	}
	for i := range wantArgs {
		if queryer.args[i] != wantArgs[i] {
			t.Fatalf("payment lookup arg %d = %#v, want %#v", i, queryer.args[i], wantArgs[i])
		}
	}
}

func TestPaymentSchemaEnforcesCanonicalLifecycleState(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_payments_lifecycle_check",
		"status = UPPER(TRIM(status))",
		"status = 'PENDING'",
		"status = 'SUCCEEDED'",
		"status = 'FAILED'",
		"paid_at IS NOT NULL",
		"provider_trade_no <> ''",
		"failure_reason <> ''",
		") NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing payment lifecycle constraint %q", want)
		}
	}
}

func TestPaymentFailureStateUpdatesRequireAffectedRows(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	for _, tt := range []struct {
		name string
		run  func(context.Context, queryer) error
	}{
		{
			name: "mark payment failed",
			run: func(ctx context.Context, db queryer) error {
				return markPaymentFailed(ctx, db, 501, 601, 7, 120, "insufficient credits", now)
			},
		},
		{
			name: "reopen order after payment failure",
			run: func(ctx context.Context, db queryer) error {
				return reopenOrderAfterPaymentFailure(ctx, db, 601, 7, now)
			},
		},
	} {
		t.Run(tt.name+"/missing row", func(t *testing.T) {
			err := tt.run(context.Background(), &paymentStateQueryer{tag: pgconn.NewCommandTag("UPDATE 0")})
			if !errors.Is(err, domain.ErrInvalidOrderState) {
				t.Fatalf("%s() error = %v, want invalid order state", tt.name, err)
			}
		})
		t.Run(tt.name+"/updated row", func(t *testing.T) {
			err := tt.run(context.Background(), &paymentStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")})
			if err != nil {
				t.Fatalf("%s() error = %v, want nil", tt.name, err)
			}
		})
	}
}

func TestIncrementOrderProductSalesBatchesRows(t *testing.T) {
	now := time.Date(2026, 7, 17, 18, 30, 0, 0, time.UTC)
	items := []domain.OrderItem{
		{ProductID: 102, Quantity: 3},
		{ProductID: 101, Quantity: 2},
		{ProductID: 102, Quantity: 1},
		{ProductID: 103, Quantity: 0},
	}
	db := &paymentStateQueryer{expectedCount: 2, updatedCount: 2}

	if err := incrementOrderProductSales(context.Background(), db, items, now); err != nil {
		t.Fatalf("incrementOrderProductSales() error = %v", err)
	}
	if db.queryRows != 1 {
		t.Fatalf("QueryRow() calls = %d, want one batch update", db.queryRows)
	}
	for _, want := range []string{
		"unnest($1::BIGINT[], $2::BIGINT[])",
		"SUM(input.quantity)::BIGINT",
		"UPDATE mall_products AS product",
		"sales_count = product.sales_count + requested.quantity",
		"SELECT (SELECT COUNT(*) FROM requested), COUNT(*) FROM updated",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("batch product sales query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{[]int64{102, 101, 102}, []int64{3, 2, 1}, now}
	if !reflect.DeepEqual(db.args, wantArgs) {
		t.Fatalf("batch product sales args = %#v, want %#v", db.args, wantArgs)
	}

	err := incrementOrderProductSales(context.Background(), &paymentStateQueryer{expectedCount: 2, updatedCount: 1}, items, now)
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("incrementOrderProductSales() error = %v, want product not found", err)
	}

	zeroQuantityDB := &paymentStateQueryer{}
	err = incrementOrderProductSales(context.Background(), zeroQuantityDB, []domain.OrderItem{{ProductID: 501, Quantity: 0}}, now)
	if err != nil {
		t.Fatalf("incrementOrderProductSales() zero quantity error = %v, want nil", err)
	}
	if zeroQuantityDB.queryRows != 0 {
		t.Fatalf("zero quantity QueryRow() calls = %d, want 0", zeroQuantityDB.queryRows)
	}
}

type paymentStateQueryer struct {
	tag           pgconn.CommandTag
	expectedCount int64
	updatedCount  int64
	query         string
	args          []any
	queryRows     int
}

func (q *paymentStateQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return q.tag, nil
}

func (q *paymentStateQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *paymentStateQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryRows++
	q.query = query
	q.args = append([]any(nil), args...)
	return paymentStateBatchRow{expectedCount: q.expectedCount, updatedCount: q.updatedCount}
}

type paymentStateBatchRow struct {
	expectedCount int64
	updatedCount  int64
}

func (r paymentStateBatchRow) Scan(dest ...any) error {
	if len(dest) != 2 {
		return errors.New("expected product sales counts")
	}
	expectedCount, expectedOK := dest[0].(*int64)
	updatedCount, updatedOK := dest[1].(*int64)
	if !expectedOK || !updatedOK {
		return errors.New("expected product sales count destinations")
	}
	*expectedCount = r.expectedCount
	*updatedCount = r.updatedCount
	return nil
}

type pendingPaymentQueryer struct {
	payment domain.Payment
	query   string
	args    []any
}

func (q *pendingPaymentQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *pendingPaymentQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *pendingPaymentQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.query = query
	q.args = args
	return pendingPaymentScanRow{values: []any{
		q.payment.ID,
		q.payment.OrderID,
		q.payment.UserID,
		q.payment.AmountCredits,
		q.payment.Provider,
		q.payment.IdempotencyKey,
		string(q.payment.Status),
		q.payment.ProviderTradeNo,
		q.payment.FailureReason,
		sql.NullTime{},
		q.payment.CreatedAt,
		q.payment.UpdatedAt,
	}}
}

type pendingPaymentScanRow struct {
	values []any
}

func (r pendingPaymentScanRow) Scan(dest ...any) error {
	return testScanner(r.values).Scan(dest...)
}
