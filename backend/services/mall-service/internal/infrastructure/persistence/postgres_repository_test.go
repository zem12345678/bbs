package persistence

import (
	"context"
	"errors"
	"reflect"
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

func TestScanOrdersBatchLoadsItemsAndEntitlements(t *testing.T) {
	parentRows := &batchRows{scans: []func(dest ...any) error{
		func(dest ...any) error {
			*dest[0].(*int64) = 101
			return nil
		},
		func(dest ...any) error {
			*dest[0].(*int64) = 102
			return nil
		},
	}}
	db := &orderDetailsBatchQueryer{parentRows: parentRows}

	orders, total, err := scanOrders(context.Background(), db, parentRows, 2)
	if err != nil {
		t.Fatalf("scanOrders() error = %v", err)
	}
	if !parentRows.closed {
		t.Fatal("order rows must be closed before loading batched details")
	}
	if total != 2 || len(orders) != 2 || len(orders[0].Items) != 1 || len(orders[1].Items) != 1 || len(orders[1].DigitalEntitlements) != 1 {
		t.Fatalf("batched orders = %+v, want two item-loaded orders and one entitlement", orders)
	}
	if orders[0].Items[0].ProductID != 1001 || orders[1].Items[0].ProductID != 1002 || orders[1].DigitalEntitlements[0].ProductID != 1002 {
		t.Fatalf("batched order details = %+v, want details attached to their order", orders)
	}
	if db.itemQueries != 1 || db.entitlementQueries != 1 {
		t.Fatalf("detail queries = items:%d entitlements:%d, want 1/1", db.itemQueries, db.entitlementQueries)
	}
	wantArgs := []any{[]int64{101, 102}}
	if !reflect.DeepEqual(db.itemArgs, wantArgs) || !reflect.DeepEqual(db.entitlementArgs, wantArgs) {
		t.Fatalf("batch detail args = items:%#v entitlements:%#v, want %#v", db.itemArgs, db.entitlementArgs, wantArgs)
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
			*dest[0].(*int64) = 1
			*dest[1].(*int64) = 2
			*dest[3].(*string) = "product"
			*dest[4].(*string) = "physical"
			*dest[7].(*int32) = 1
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

type orderDetailsBatchQueryer struct {
	parentRows         *batchRows
	itemQueries        int
	entitlementQueries int
	itemArgs           []any
	entitlementArgs    []any
}

func (q *orderDetailsBatchQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (q *orderDetailsBatchQueryer) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM mall_order_items") {
		if !q.parentRows.closed {
			return nil, errors.New("parent rows are still open")
		}
		q.itemQueries++
		q.itemArgs = append([]any(nil), args...)
		return &batchRows{scans: []func(dest ...any) error{
			func(dest ...any) error {
				*dest[0].(*int64) = 101
				*dest[1].(*int64) = 1001
				*dest[2].(*string) = "PRODUCT-ONE"
				*dest[3].(*string) = "Product one"
				*dest[4].(*string) = "physical"
				*dest[7].(*int32) = 1
				return nil
			},
			func(dest ...any) error {
				*dest[0].(*int64) = 102
				*dest[1].(*int64) = 1002
				*dest[2].(*string) = "VIP-MONTH"
				*dest[3].(*string) = "VIP month"
				*dest[4].(*string) = "digital"
				*dest[5].(*string) = "membership"
				*dest[6].(*string) = "vip-month"
				*dest[7].(*int32) = 1
				return nil
			},
		}}, nil
	}
	if strings.Contains(sql, "FROM mall_digital_entitlements") {
		q.entitlementQueries++
		q.entitlementArgs = append([]any(nil), args...)
		return &batchRows{scans: []func(dest ...any) error{
			func(dest ...any) error {
				*dest[0].(*int64) = 501
				*dest[1].(*int64) = 102
				*dest[2].(*string) = "ORDER-102"
				*dest[3].(*int64) = 7
				*dest[4].(*int64) = 1002
				*dest[5].(*string) = "VIP-MONTH"
				*dest[6].(*string) = "VIP month"
				*dest[7].(*int32) = 1
				*dest[8].(*string) = "BBS-ENTITLEMENT"
				*dest[9].(*string) = "membership"
				*dest[10].(*string) = "vip-month"
				*dest[13].(*string) = "ACTIVE"
				return nil
			},
		}}, nil
	}
	return nil, errors.New("unexpected query")
}

func (q *orderDetailsBatchQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return errorRow{}
}

type batchRows struct {
	scans  []func(dest ...any) error
	index  int
	closed bool
}

func (r *batchRows) Close() {
	r.closed = true
}

func (*batchRows) Err() error {
	return nil
}

func (*batchRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (*batchRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *batchRows) Next() bool {
	if r.index >= len(r.scans) {
		r.Close()
		return false
	}
	r.index++
	return true
}

func (r *batchRows) Scan(dest ...any) error {
	if r.index <= 0 || r.index > len(r.scans) {
		return errors.New("scan called without a current row")
	}
	return r.scans[r.index-1](dest...)
}

func (*batchRows) Values() ([]any, error) {
	return nil, nil
}

func (*batchRows) RawValues() [][]byte {
	return nil
}

func (*batchRows) Conn() *pgx.Conn {
	return nil
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
