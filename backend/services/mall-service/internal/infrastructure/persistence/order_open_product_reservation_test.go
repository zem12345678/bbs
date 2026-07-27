package persistence

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestEnsureNoOpenOrderProductReservationRejectsMatchingOpenOrder(t *testing.T) {
	db := &openOrderProductReservationQueryer{exists: true}
	err := ensureNoOpenOrderProductReservation(context.Background(), db, 7, []domain.OrderItem{
		{ProductID: 3},
		{ProductID: 1},
		{ProductID: 3},
	})
	if !errors.Is(err, domain.ErrPendingOrderProductExists) {
		t.Fatalf("ensureNoOpenOrderProductReservation() error = %v, want pending product reservation", err)
	}
	for _, want := range []string{
		"FROM mall_orders o",
		"JOIN mall_order_items oi ON oi.order_id = o.id",
		"o.user_id = $1::BIGINT",
		"oi.product_id = ANY($2::BIGINT[])",
		"o.status IN ($3, $4)",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("reservation query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{
		int64(7),
		[]int64{1, 3},
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
	}
	if !reflect.DeepEqual(db.args, wantArgs) {
		t.Fatalf("reservation query args = %#v, want %#v", db.args, wantArgs)
	}
}

func TestEnsureNoOpenOrderProductReservationAllowsWhenNoOpenOrderMatches(t *testing.T) {
	db := &openOrderProductReservationQueryer{}
	err := ensureNoOpenOrderProductReservation(context.Background(), db, 7, []domain.OrderItem{{ProductID: 2}})
	if err != nil {
		t.Fatalf("ensureNoOpenOrderProductReservation() error = %v", err)
	}
	if db.queryCalls != 1 {
		t.Fatalf("QueryRow() calls = %d, want 1", db.queryCalls)
	}
}

func TestUniqueOrderProductIDsRejectsInvalidItems(t *testing.T) {
	if _, err := uniqueOrderProductIDs(nil); !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("uniqueOrderProductIDs(nil) error = %v, want invalid order state", err)
	}
	if _, err := uniqueOrderProductIDs([]domain.OrderItem{{ProductID: 0}}); !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("uniqueOrderProductIDs(invalid) error = %v, want invalid order state", err)
	}
}

type openOrderProductReservationQueryer struct {
	exists     bool
	err        error
	query      string
	args       []any
	queryCalls int
}

func (*openOrderProductReservationQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (*openOrderProductReservationQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (q *openOrderProductReservationQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryCalls++
	q.query = query
	q.args = append([]any(nil), args...)
	return openOrderProductReservationRow{exists: q.exists, err: q.err}
}

type openOrderProductReservationRow struct {
	exists bool
	err    error
}

func (r openOrderProductReservationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("expected one exists destination")
	}
	exists, ok := dest[0].(*bool)
	if !ok {
		return errors.New("expected bool destination")
	}
	*exists = r.exists
	return nil
}
