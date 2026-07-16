package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRefundRequestForLockedOrderUsesCurrentOrderSnapshot(t *testing.T) {
	requestedAt := time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC)
	request, err := refundRequestForLockedOrder(domain.RefundRequest{
		OrderID:       501,
		OrderNo:       "STALE",
		UserID:        7,
		AmountCredits: 1,
		Status:        domain.RefundStatusApproved,
		Reason:        "after_sale",
		UserNote:      "需要退款",
		RequestedAt:   requestedAt,
		CreatedAt:     requestedAt,
		UpdatedAt:     requestedAt,
	}, domain.Order{
		ID:           501,
		OrderNo:      "M20260716001",
		UserID:       7,
		TotalCredits: 180,
		Status:       domain.OrderStatusCompleted,
	})
	if err != nil {
		t.Fatalf("refundRequestForLockedOrder() error = %v", err)
	}
	if request.OrderNo != "M20260716001" || request.AmountCredits != 180 || request.Status != domain.RefundStatusRequested {
		t.Fatalf("refund request snapshot = %+v, want current order no/amount and requested status", request)
	}
}

func TestRefundRequestForLockedOrderRejectsInvalidOrder(t *testing.T) {
	request := domain.RefundRequest{OrderID: 501, UserID: 7}
	if _, err := refundRequestForLockedOrder(request, domain.Order{
		ID:     501,
		UserID: 8,
		Status: domain.OrderStatusCompleted,
	}); !errors.Is(err, domain.ErrOrderOwnerMismatch) {
		t.Fatalf("owner mismatch error = %v, want owner mismatch", err)
	}
	if _, err := refundRequestForLockedOrder(request, domain.Order{
		ID:     501,
		UserID: 7,
		Status: domain.OrderStatusPendingPayment,
	}); !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("invalid status error = %v, want invalid order state", err)
	}
}

func TestRefundRequestForLockedOrderRejectsMembershipOrder(t *testing.T) {
	request := domain.RefundRequest{OrderID: 501, UserID: 7}
	if _, err := refundRequestForLockedOrder(request, domain.Order{
		ID:     501,
		UserID: 7,
		Status: domain.OrderStatusPaid,
		Items: []domain.OrderItem{
			{ProductID: 101, GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
		},
	}); !errors.Is(err, domain.ErrMembershipRefundUnavailable) {
		t.Fatalf("membership item error = %v, want membership refund unavailable", err)
	}
	if _, err := refundRequestForLockedOrder(request, domain.Order{
		ID:     501,
		UserID: 7,
		Status: domain.OrderStatusPaid,
		DigitalEntitlements: []domain.DigitalEntitlement{
			{ProductID: 101, GrantKey: "vip-month", Status: domain.DigitalEntitlementStatusActive},
		},
	}); !errors.Is(err, domain.ErrMembershipRefundUnavailable) {
		t.Fatalf("membership entitlement error = %v, want membership refund unavailable", err)
	}
}

func TestInsertRefundRequestReturnsExistingDuplicate(t *testing.T) {
	now := time.Date(2026, 7, 16, 11, 30, 0, 0, time.UTC)
	existing := domain.RefundRequest{
		ID:            7602,
		OrderID:       602,
		OrderNo:       "M602",
		UserID:        7,
		AmountCredits: 180,
		Status:        domain.RefundStatusRequested,
		Reason:        "quality_issue",
		UserNote:      "原申请说明",
		RequestedAt:   now.Add(-time.Hour),
		CreatedAt:     now.Add(-time.Hour),
		UpdatedAt:     now.Add(-time.Hour),
	}
	refund, duplicate, err := insertRefundRequest(context.Background(), &refundInsertQueryer{existing: existing}, domain.RefundRequest{
		OrderID:       602,
		OrderNo:       "M602",
		UserID:        7,
		AmountCredits: 180,
		Status:        domain.RefundStatusRequested,
		Reason:        "after_sale",
		UserNote:      "新的重复申请说明",
		RequestedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("insertRefundRequest() error = %v", err)
	}
	if !duplicate {
		t.Fatal("insertRefundRequest() duplicate = false, want true")
	}
	if refund.ID != existing.ID || refund.UserNote != existing.UserNote {
		t.Fatalf("insertRefundRequest() refund = %+v, want existing request", refund)
	}
}

func TestRefundRequestKeywordConditionCoversOperationalIds(t *testing.T) {
	condition := refundRequestKeywordCondition(3)
	for _, want := range []string{
		"id::TEXT = $3",
		"order_id::TEXT = $3",
		"order_no ILIKE '%' || $3 || '%'",
		"reason ILIKE '%' || $3 || '%'",
		"user_note ILIKE '%' || $3 || '%'",
		"admin_note ILIKE '%' || $3 || '%'",
		"operator_id ILIKE '%' || $3 || '%'",
	} {
		if !strings.Contains(condition, want) {
			t.Fatalf("refund keyword condition = %q, want %q", condition, want)
		}
	}
}

func TestRefundOrderStateUpdateRequiresAffectedRows(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)

	t.Run("missing row", func(t *testing.T) {
		err := markOrderRefunded(context.Background(), &refundStateQueryer{tag: pgconn.NewCommandTag("UPDATE 0")}, 501, domain.OrderStatusCompleted, now)
		if !errors.Is(err, domain.ErrInvalidOrderState) {
			t.Fatalf("markOrderRefunded() error = %v, want invalid order state", err)
		}
	})

	t.Run("updated row", func(t *testing.T) {
		err := markOrderRefunded(context.Background(), &refundStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")}, 501, domain.OrderStatusCompleted, now)
		if err != nil {
			t.Fatalf("markOrderRefunded() error = %v, want nil", err)
		}
	})
}

type refundInsertQueryer struct {
	existing domain.RefundRequest
	queryRow int
}

func (q *refundInsertQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *refundInsertQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *refundInsertQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	q.queryRow++
	if q.queryRow == 1 {
		return refundScanRow{err: pgx.ErrNoRows}
	}
	return refundScanRow{values: refundRequestValues(q.existing)}
}

type refundStateQueryer struct {
	tag pgconn.CommandTag
}

func (q *refundStateQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return q.tag, nil
}

func (q *refundStateQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *refundStateQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

type refundScanRow struct {
	values []any
	err    error
}

func (r refundScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return testScanner(r.values).Scan(dest...)
}

func refundRequestValues(refund domain.RefundRequest) []any {
	var reviewedAt sql.NullTime
	if refund.ReviewedAt != nil {
		reviewedAt = sql.NullTime{Time: *refund.ReviewedAt, Valid: true}
	}
	var refundedAt sql.NullTime
	if refund.RefundedAt != nil {
		refundedAt = sql.NullTime{Time: *refund.RefundedAt, Valid: true}
	}
	return []any{
		refund.ID,
		refund.OrderID,
		refund.OrderNo,
		refund.UserID,
		refund.AmountCredits,
		string(refund.Status),
		refund.Reason,
		refund.UserNote,
		refund.AdminNote,
		refund.RestoreStock,
		refund.OperatorID,
		refund.RequestedAt,
		reviewedAt,
		refundedAt,
		refund.CreatedAt,
		refund.UpdatedAt,
	}
}
