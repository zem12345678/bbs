package persistence

import (
	"context"
	"errors"
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

func TestIncrementProductSalesRequiresAffectedProduct(t *testing.T) {
	now := time.Date(2026, 7, 17, 18, 30, 0, 0, time.UTC)

	err := incrementProductSales(context.Background(), &paymentStateQueryer{tag: pgconn.NewCommandTag("UPDATE 0")}, 501, 2, now)
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("incrementProductSales() error = %v, want product not found", err)
	}

	err = incrementProductSales(context.Background(), &paymentStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")}, 501, 2, now)
	if err != nil {
		t.Fatalf("incrementProductSales() error = %v, want nil", err)
	}

	err = incrementProductSales(context.Background(), &paymentStateQueryer{tag: pgconn.NewCommandTag("UPDATE 0")}, 501, 0, now)
	if err != nil {
		t.Fatalf("incrementProductSales() zero quantity error = %v, want nil", err)
	}
}

type paymentStateQueryer struct {
	tag pgconn.CommandTag
}

func (q *paymentStateQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return q.tag, nil
}

func (q *paymentStateQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *paymentStateQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}
