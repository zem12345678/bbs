package persistence

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDecrementOrderProductStocksBatchesStockAndLogs(t *testing.T) {
	createdAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	db := &orderBatchWriteQueryer{expectedCount: 2, updatedCount: 2}
	order := domain.Order{
		ID:        9001,
		UserID:    7,
		CreatedAt: createdAt,
		Items: []domain.OrderItem{
			{ProductID: 102, Quantity: 3},
			{ProductID: 101, Quantity: 2},
		},
	}

	if err := decrementOrderProductStocks(context.Background(), db, order); err != nil {
		t.Fatalf("decrementOrderProductStocks() error = %v", err)
	}
	if db.queryRows != 1 || db.execCalls != 0 {
		t.Fatalf("batch stock writes = query:%d exec:%d, want 1/0", db.queryRows, db.execCalls)
	}
	for _, want := range []string{
		"unnest($1::BIGINT[], $2::BIGINT[])",
		"UPDATE mall_products AS product",
		"INSERT INTO mall_product_stock_logs",
		"SELECT (SELECT COUNT(*) FROM requested), COUNT(*) FROM updated",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("batch stock query = %q, want %q", db.query, want)
		}
	}
	assertOrderBatchArgs(t, "batch stock", db.queryArgs, []any{
		[]int64{101, 102},
		[]int64{2, 3},
		domain.StockChangeReasonOrderCreated,
		domain.StockReferenceOrder,
		int64(9001),
		domain.OrderStatusOperatorUser,
		"7",
		"下单锁定库存",
		createdAt,
		string(domain.ProductStatusActive),
	})
}

func TestDecrementOrderProductStocksRejectsIncompleteBatch(t *testing.T) {
	db := &orderBatchWriteQueryer{expectedCount: 2, updatedCount: 1}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, Quantity: 1},
			{ProductID: 102, Quantity: 1},
		},
	}

	err := decrementOrderProductStocks(context.Background(), db, order)
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("decrementOrderProductStocks() error = %v, want insufficient stock", err)
	}
	if db.queryRows != 1 {
		t.Fatalf("batch stock query rows = %d, want 1", db.queryRows)
	}
}

func TestInsertOrderItemsBatchesRows(t *testing.T) {
	db := &orderBatchWriteQueryer{}
	items := []domain.OrderItem{
		{
			ProductID:        102,
			SKU:              "VIP-MONTH",
			Title:            "会员月卡",
			Category:         "digital",
			GrantType:        "membership",
			GrantKey:         "vip-month",
			Quantity:         3,
			UnitPriceCredits: 100,
			SubtotalCredits:  300,
		},
		{
			ProductID:        101,
			SKU:              "THEME-PRO",
			Title:            "高级主题",
			Category:         "digital",
			GrantType:        "theme",
			GrantKey:         "theme-pro",
			Quantity:         1,
			UnitPriceCredits: 200,
			SubtotalCredits:  200,
		},
	}

	if err := insertOrderItems(context.Background(), db, 9001, items); err != nil {
		t.Fatalf("insertOrderItems() error = %v", err)
	}
	if db.execCalls != 1 || db.queryRows != 0 {
		t.Fatalf("batch order item writes = exec:%d query:%d, want 1/0", db.execCalls, db.queryRows)
	}
	if !strings.Contains(db.execQuery, "FROM unnest(") || !strings.Contains(db.execQuery, "$8::INTEGER[]") {
		t.Fatalf("batch order item query = %q, want unnest integer quantity", db.execQuery)
	}
	assertOrderBatchArgs(t, "batch order items", db.execArgs, []any{
		int64(9001),
		[]int64{102, 101},
		[]string{"VIP-MONTH", "THEME-PRO"},
		[]string{"会员月卡", "高级主题"},
		[]string{"digital", "digital"},
		[]string{"membership", "theme"},
		[]string{"vip-month", "theme-pro"},
		[]int32{3, 1},
		[]int64{100, 200},
		[]int64{300, 200},
	})
}

type orderBatchWriteQueryer struct {
	expectedCount int64
	updatedCount  int64
	query         string
	queryArgs     []any
	queryRows     int
	execQuery     string
	execArgs      []any
	execCalls     int
}

func (q *orderBatchWriteQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	q.execCalls++
	q.execQuery = query
	q.execArgs = append([]any(nil), args...)
	return pgconn.CommandTag{}, nil
}

func (q *orderBatchWriteQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *orderBatchWriteQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryRows++
	q.query = query
	q.queryArgs = append([]any(nil), args...)
	return orderBatchWriteRow{expectedCount: q.expectedCount, updatedCount: q.updatedCount}
}

type orderBatchWriteRow struct {
	expectedCount int64
	updatedCount  int64
}

func (r orderBatchWriteRow) Scan(dest ...any) error {
	if len(dest) != 2 {
		return errors.New("expected requested and updated counts")
	}
	expectedCount, expectedOK := dest[0].(*int64)
	updatedCount, updatedOK := dest[1].(*int64)
	if !expectedOK || !updatedOK {
		return errors.New("expected int64 count destinations")
	}
	*expectedCount = r.expectedCount
	*updatedCount = r.updatedCount
	return nil
}

func assertOrderBatchArgs(t *testing.T, label string, got, want []any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s args = %#v, want %#v", label, got, want)
	}
}
