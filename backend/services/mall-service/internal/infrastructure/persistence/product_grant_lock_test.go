package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestEnsureProductGrantMutableSkipsUnchangedGrant(t *testing.T) {
	db := &productGrantLockQueryer{locked: true}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, GrantKey: "vip-month"},
		domain.Product{ID: 101, GrantType: " Membership ", GrantKey: " VIP-MONTH "},
	)
	if err != nil {
		t.Fatalf("ensureProductGrantMutable() error = %v, want nil", err)
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0 for unchanged grant", db.queryRows)
	}
}

func TestEnsureProductGrantMutableAllowsUnsoldGrantChange(t *testing.T) {
	db := &productGrantLockQueryer{}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, GrantType: "membership", GrantKey: "vip-month"},
		domain.Product{ID: 101, GrantType: "theme", GrantKey: "theme-pro"},
	)
	if err != nil {
		t.Fatalf("ensureProductGrantMutable() error = %v, want nil", err)
	}
	if db.queryRows != 1 {
		t.Fatalf("QueryRow() calls = %d, want 1", db.queryRows)
	}
	query := db.query
	for _, want := range []string{
		"mall_order_items",
		"mall_orders",
		"oi.product_id = $1",
		"o.status IN ($2, $3, $4, $5)",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("grant lock query = %q, want %q", query, want)
		}
	}
	wantArgs := []any{
		int64(101),
		string(domain.OrderStatusPaid),
		string(domain.OrderStatusShipped),
		string(domain.OrderStatusCompleted),
		string(domain.OrderStatusRefunded),
	}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("grant lock args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("grant lock arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestEnsureProductGrantMutableBlocksSoldGrantChange(t *testing.T) {
	db := &productGrantLockQueryer{locked: true}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, GrantType: "membership", GrantKey: "vip-month"},
		domain.Product{ID: 101, GrantType: "membership", GrantKey: "vip-year"},
	)
	if !errors.Is(err, domain.ErrProductGrantLocked) {
		t.Fatalf("ensureProductGrantMutable() error = %v, want product grant locked", err)
	}
}

func TestEnsureProductGrantMutableAllowsUnsoldFulfillmentChange(t *testing.T) {
	db := &productGrantLockQueryer{}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, Category: "goods"},
		domain.Product{ID: 101, Category: "digital"},
	)
	if err != nil {
		t.Fatalf("ensureProductGrantMutable() error = %v, want nil", err)
	}
	if db.queryRows != 1 {
		t.Fatalf("QueryRow() calls = %d, want 1", db.queryRows)
	}
}

func TestEnsureProductGrantMutableBlocksSoldFulfillmentChange(t *testing.T) {
	db := &productGrantLockQueryer{locked: true}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, Category: "goods"},
		domain.Product{ID: 101, Category: "digital"},
	)
	if !errors.Is(err, domain.ErrProductFulfillmentLocked) {
		t.Fatalf("ensureProductGrantMutable() error = %v, want product fulfillment locked", err)
	}
}

func TestOpenThemeOrderExistsQueriesPendingPayingThemeGrant(t *testing.T) {
	db := &productGrantLockQueryer{locked: true}

	exists, err := openThemeOrderExists(context.Background(), db, 7, " Theme-Pro ")
	if err != nil {
		t.Fatalf("openThemeOrderExists() error = %v", err)
	}
	if !exists {
		t.Fatal("openThemeOrderExists() = false, want true")
	}
	for _, want := range []string{
		"mall_orders o",
		"mall_order_items oi",
		"o.status IN ($2, $3)",
		"LOWER(TRIM(COALESCE(oi.grant_type, ''))) = $4",
		"LOWER(TRIM(COALESCE(oi.grant_key, ''))) = $5",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("open theme order query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{
		int64(7),
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		"theme",
		"theme-pro",
	}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("open theme order args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("open theme order arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestOpenThemeOrderExistsSkipsInvalidInput(t *testing.T) {
	db := &productGrantLockQueryer{}

	exists, err := openThemeOrderExists(context.Background(), db, 7, " ")
	if err != nil {
		t.Fatalf("openThemeOrderExists() error = %v", err)
	}
	if exists {
		t.Fatal("openThemeOrderExists() = true, want false")
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0", db.queryRows)
	}
}

type productGrantLockQueryer struct {
	locked    bool
	query     string
	args      []any
	queryRows int
}

func (q *productGrantLockQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *productGrantLockQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *productGrantLockQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryRows++
	q.query = query
	q.args = args
	return productGrantLockScanRow{locked: q.locked}
}

type productGrantLockScanRow struct {
	locked bool
}

func (r productGrantLockScanRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}
	locked, ok := dest[0].(*bool)
	if !ok {
		return errors.New("expected bool scan destination")
	}
	*locked = r.locked
	return nil
}
