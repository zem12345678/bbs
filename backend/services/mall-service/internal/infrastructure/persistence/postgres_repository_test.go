package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestScanOrdersReleasesRowsBeforeLoadingItems(t *testing.T) {
	parentRows := &singleRowRows{hasRow: true, scan: func(dest ...any) error {
		*dest[0].(*int64) = 1
		return nil
	}}
	db := &orderItemsQueryer{parentRows: parentRows}

	orders, total, err := scanOrders(context.Background(), db, parentRows, 1)
	if err != nil {
		t.Fatalf("scan orders: %v", err)
	}
	if !parentRows.closed {
		t.Fatal("order rows must be closed before loading order items")
	}
	if total != 1 || len(orders) != 1 || len(orders[0].Items) != 1 {
		t.Fatalf("unexpected orders result: total=%d orders=%+v", total, orders)
	}
}

type orderItemsQueryer struct {
	parentRows *singleRowRows
}

func (q *orderItemsQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (q *orderItemsQueryer) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM mall_order_items") {
		if !q.parentRows.closed {
			return nil, errors.New("parent rows are still open")
		}
		return &singleRowRows{hasRow: true, scan: func(dest ...any) error {
			*dest[0].(*int64) = 2
			*dest[2].(*string) = "product"
			*dest[3].(*string) = "physical"
			*dest[6].(*int32) = 1
			return nil
		}}, nil
	}
	if strings.Contains(sql, "FROM mall_digital_entitlements") {
		return &singleRowRows{}, nil
	}
	return nil, errors.New("unexpected query")
}

func (q *orderItemsQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return errorRow{}
}

type singleRowRows struct {
	hasRow  bool
	yielded bool
	closed  bool
	scan    func(dest ...any) error
}

func (r *singleRowRows) Close() {
	r.closed = true
}

func (r *singleRowRows) Err() error {
	return nil
}

func (r *singleRowRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *singleRowRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *singleRowRows) Next() bool {
	if !r.hasRow || r.yielded {
		r.closed = true
		return false
	}
	r.yielded = true
	return true
}

func (r *singleRowRows) Scan(dest ...any) error {
	return r.scan(dest...)
}

func (r *singleRowRows) Values() ([]any, error) {
	return nil, nil
}

func (r *singleRowRows) RawValues() [][]byte {
	return nil
}

func (r *singleRowRows) Conn() *pgx.Conn {
	return nil
}

type errorRow struct{}

func (errorRow) Scan(...any) error {
	return errors.New("unexpected query row")
}
