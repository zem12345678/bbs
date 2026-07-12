package persistence

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIssueDigitalEntitlementsInsertsFulfillmentCode(t *testing.T) {
	issuedAt := time.Date(2026, 7, 12, 12, 30, 0, 0, time.UTC)
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, issuedAt); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 1 {
		t.Fatalf("Exec() calls = %d, want 1", len(db.execArgs))
	}
	args := db.execArgs[0]
	if args[0] != order.ID || args[1] != order.Items[0].ProductID || args[2] != order.UserID {
		t.Fatalf("Exec() identity args = %+v", args[:3])
	}
	code, ok := args[6].(string)
	if !ok || !strings.HasPrefix(code, "BBS-") {
		t.Fatalf("fulfillment code = %#v, want BBS- prefix", args[6])
	}
	if args[7] != issuedAt {
		t.Fatalf("issued at arg = %#v, want %v", args[7], issuedAt)
	}
}

func TestIssueDigitalEntitlementsRetriesFulfillmentCodeCollision(t *testing.T) {
	db := &digitalEntitlementQueryer{
		execErrors: []error{&pgconn.PgError{Code: "23505"}},
	}
	order := domain.Order{
		ID:     9002,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 2 {
		t.Fatalf("Exec() calls = %d, want retry after unique violation", len(db.execArgs))
	}
}

type digitalEntitlementQueryer struct {
	execArgs   [][]any
	execErrors []error
}

func (q *digitalEntitlementQueryer) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	q.execArgs = append(q.execArgs, args)
	if len(q.execErrors) == 0 {
		return pgconn.CommandTag{}, nil
	}
	err := q.execErrors[0]
	q.execErrors = q.execErrors[1:]
	return pgconn.CommandTag{}, err
}

func (q *digitalEntitlementQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *digitalEntitlementQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}
