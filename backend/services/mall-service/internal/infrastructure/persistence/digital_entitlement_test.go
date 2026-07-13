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
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1},
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
	if args[7] != domain.DigitalEntitlementStatusActive {
		t.Fatalf("status arg = %#v, want %s", args[7], domain.DigitalEntitlementStatusActive)
	}
	if args[8] != issuedAt {
		t.Fatalf("issued at arg = %#v, want %v", args[8], issuedAt)
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
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 2 {
		t.Fatalf("Exec() calls = %d, want retry after unique violation", len(db.execArgs))
	}
}

func TestIssueDigitalEntitlementsSkipsPhysicalItems(t *testing.T) {
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9003,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 0 {
		t.Fatalf("Exec() calls = %d, want 0 for physical item", len(db.execArgs))
	}
}

func TestIsDigitalOnlyOrderUsesItemCategorySnapshot(t *testing.T) {
	order := domain.Order{
		Receiver: "Alice",
		Phone:    "13800000000",
		Address:  "上海市数字权益也可填写地址",
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1},
		},
	}

	if !isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = false, want true for digital item even when address fields are present")
	}
}

func TestIsDigitalOnlyOrderRequiresAllItemsToBeDigital(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1},
			{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
		},
	}

	if isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = true, want false when any item requires shipping")
	}
}

func TestIsDigitalOnlyOrderDoesNotTreatMissingCategoryAsDigital(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "LEGACY", Title: "历史订单项", Quantity: 1},
		},
	}

	if isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = true, want false when item category snapshot is missing")
	}
}

func TestRevokeDigitalEntitlementsForRefundMarksOrderEntitlementsRevoked(t *testing.T) {
	revokedAt := time.Date(2026, 7, 13, 9, 45, 0, 0, time.UTC)
	db := &digitalEntitlementQueryer{}

	if err := revokeDigitalEntitlementsForRefund(context.Background(), db, 9004, 7001, revokedAt); err != nil {
		t.Fatalf("revokeDigitalEntitlementsForRefund() error = %v", err)
	}
	if len(db.execArgs) != 1 {
		t.Fatalf("Exec() calls = %d, want 1", len(db.execArgs))
	}
	args := db.execArgs[0]
	if args[0] != int64(9004) {
		t.Fatalf("order id arg = %#v, want 9004", args[0])
	}
	if args[1] != domain.DigitalEntitlementStatusRevoked {
		t.Fatalf("status arg = %#v, want %s", args[1], domain.DigitalEntitlementStatusRevoked)
	}
	if args[2] != revokedAt {
		t.Fatalf("revoked at arg = %#v, want %v", args[2], revokedAt)
	}
	if args[3] != int64(7001) {
		t.Fatalf("refund id arg = %#v, want 7001", args[3])
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
