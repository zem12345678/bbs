package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRefreshOrderFromCurrentProductsUsesLockedProductSnapshot(t *testing.T) {
	db := &orderProductSnapshotQueryer{
		products: map[int64]domain.Product{
			202: {
				ID:           202,
				SKU:          "VIP-MONTH",
				Title:        "会员月卡",
				Category:     "digital",
				GrantType:    " Membership ",
				GrantKey:     " VIP-MONTH ",
				PriceCredits: 300,
				Stock:        5,
				Status:       domain.ProductStatusActive,
			},
		},
	}

	order, err := refreshOrderFromCurrentProducts(context.Background(), db, domain.Order{
		UserID:          7,
		OriginalCredits: 100,
		TotalCredits:    100,
		Items: []domain.OrderItem{
			{ProductID: 202, SKU: "STALE", Title: "旧会员", Category: "digital", GrantType: "theme", GrantKey: "theme-pro", Quantity: 2, UnitPriceCredits: 50, SubtotalCredits: 100},
		},
	})
	if err != nil {
		t.Fatalf("refreshOrderFromCurrentProducts() error = %v", err)
	}
	if order.OriginalCredits != 600 || order.TotalCredits != 600 || order.DiscountCredits != 0 || order.CouponID != 0 {
		t.Fatalf("order totals = original:%d total:%d discount:%d coupon:%d, want current 600/600 no discount", order.OriginalCredits, order.TotalCredits, order.DiscountCredits, order.CouponID)
	}
	if len(order.Items) != 1 {
		t.Fatalf("order items = %d, want 1", len(order.Items))
	}
	item := order.Items[0]
	if item.SKU != "VIP-MONTH" || item.Title != "会员月卡" || item.GrantType != "membership" || item.GrantKey != "vip-month" || item.UnitPriceCredits != 300 || item.SubtotalCredits != 600 {
		t.Fatalf("refreshed item = %+v, want current product snapshot", item)
	}
	if len(db.queries) != 1 || !strings.Contains(db.queries[0], "ORDER BY id") || !strings.Contains(db.queries[0], "FOR UPDATE") {
		t.Fatalf("product query = %+v, want ordered row lock", db.queries)
	}
	if len(db.args) != 1 || len(db.args[0]) != 1 {
		t.Fatalf("product query args = %+v, want one product id batch", db.args)
	}
	productIDs, ok := db.args[0][0].([]int64)
	if !ok || len(productIDs) != 1 || productIDs[0] != 202 {
		t.Fatalf("product query ids = %+v, want [202]", db.args[0][0])
	}
}

func TestRefreshOrderFromCurrentProductsLocksProductsWithOneSortedBatchQuery(t *testing.T) {
	db := &orderProductSnapshotQueryer{
		products: map[int64]domain.Product{
			1: {ID: 1, SKU: "A", Title: "A", Category: "digital", PriceCredits: 100, Stock: 1, Status: domain.ProductStatusActive},
			2: {ID: 2, SKU: "B", Title: "B", Category: "digital", PriceCredits: 200, Stock: 1, Status: domain.ProductStatusActive},
		},
	}

	_, err := refreshOrderFromCurrentProducts(context.Background(), db, domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 2, Quantity: 1},
			{ProductID: 1, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("refreshOrderFromCurrentProducts() error = %v", err)
	}
	if len(db.queries) != 1 || len(db.args) != 1 || len(db.args[0]) != 1 {
		t.Fatalf("product lock queries/args = %+v/%+v, want one batched query", db.queries, db.args)
	}
	productIDs, ok := db.args[0][0].([]int64)
	if !ok || len(productIDs) != 2 || productIDs[0] != 1 || productIDs[1] != 2 {
		t.Fatalf("product lock ids = %+v, want [1 2]", db.args[0][0])
	}
}

func TestRefreshOrderFromCurrentProductsRejectsChangedExpectedOriginalCreditsBeforeWrites(t *testing.T) {
	expectedOriginalCredits := int64(100)
	tx := &priceChangedOrderTx{snapshot: orderProductSnapshotQueryer{
		products: map[int64]domain.Product{
			101: {ID: 101, SKU: "CURRENT", Title: "Current", Category: "digital", PriceCredits: 200, Stock: 5, Status: domain.ProductStatusActive},
		},
	}}

	_, _, err := createOrderInTx(context.Background(), tx, domain.Order{
		ExpectedOriginalCredits: &expectedOriginalCredits,
		Items:                   []domain.OrderItem{{ProductID: 101, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrOrderPriceChanged) {
		t.Fatalf("createOrderInTx() error = %v, want ErrOrderPriceChanged", err)
	}
	if tx.execCalls != 0 || tx.queryRowCalls != 0 {
		t.Fatalf("write calls = exec:%d query row:%d, want 0/0", tx.execCalls, tx.queryRowCalls)
	}
}

type priceChangedOrderTx struct {
	pgx.Tx
	snapshot      orderProductSnapshotQueryer
	execCalls     int
	queryRowCalls int
}

func (t *priceChangedOrderTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return t.snapshot.Query(ctx, query, args...)
}

func (t *priceChangedOrderTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	t.execCalls++
	return pgconn.CommandTag{}, errors.New("unexpected write")
}

func (t *priceChangedOrderTx) QueryRow(context.Context, string, ...any) pgx.Row {
	t.queryRowCalls++
	return errorRow{}
}

func TestRefreshOrderFromCurrentProductsRejectsInactiveProduct(t *testing.T) {
	db := &orderProductSnapshotQueryer{
		products: map[int64]domain.Product{
			202: {ID: 202, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", PriceCredits: 300, Stock: 5, Status: domain.ProductStatusDraft},
		},
	}

	_, err := refreshOrderFromCurrentProducts(context.Background(), db, domain.Order{
		Items: []domain.OrderItem{{ProductID: 202, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrProductUnavailable) {
		t.Fatalf("refreshOrderFromCurrentProducts() error = %v, want product unavailable", err)
	}
}

func TestRefreshOrderFromCurrentProductsRejectsCurrentDuplicateThemeGrant(t *testing.T) {
	db := &orderProductSnapshotQueryer{
		products: map[int64]domain.Product{
			101: {ID: 101, SKU: "THEME-A", Title: "主题 A", Category: "digital", GrantType: "theme", GrantKey: "theme-pro", PriceCredits: 100, Stock: 1, Status: domain.ProductStatusActive},
			102: {ID: 102, SKU: "THEME-B", Title: "主题 B", Category: "digital", GrantType: "theme", GrantKey: "theme-pro", PriceCredits: 100, Stock: 1, Status: domain.ProductStatusActive},
		},
	}

	_, err := refreshOrderFromCurrentProducts(context.Background(), db, domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 101, GrantType: "theme", GrantKey: "theme-a", Quantity: 1},
			{ProductID: 102, GrantType: "theme", GrantKey: "theme-b", Quantity: 1},
		},
	})
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("refreshOrderFromCurrentProducts() error = %v, want duplicate theme grant", err)
	}
}

func TestRefreshOrderFromCurrentProductsRequiresCurrentShippingAddress(t *testing.T) {
	db := &orderProductSnapshotQueryer{
		products: map[int64]domain.Product{
			303: {ID: 303, SKU: "BOOK", Title: "实体书", Category: "goods", PriceCredits: 80, Stock: 2, Status: domain.ProductStatusActive},
		},
	}

	_, err := refreshOrderFromCurrentProducts(context.Background(), db, domain.Order{
		Items: []domain.OrderItem{{ProductID: 303, Category: "digital", Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("refreshOrderFromCurrentProducts() error = %v, want invalid order state", err)
	}
}

type orderProductSnapshotQueryer struct {
	products map[int64]domain.Product
	queries  []string
	args     [][]any
}

func (q *orderProductSnapshotQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (q *orderProductSnapshotQueryer) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, query)
	q.args = append(q.args, args)
	if len(args) != 1 {
		return nil, errors.New("expected product ids arg")
	}
	productIDs, ok := args[0].([]int64)
	if !ok {
		return nil, errors.New("expected []int64 product ids")
	}
	products := make([]domain.Product, 0, len(productIDs))
	for _, productID := range productIDs {
		if product, ok := q.products[productID]; ok {
			products = append(products, product)
		}
	}
	return &orderProductSnapshotRows{products: products, index: -1}, nil
}

func (*orderProductSnapshotQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return orderProductSnapshotRow{err: errors.New("unexpected query row")}
}

type orderProductSnapshotRow struct {
	product domain.Product
	err     error
}

func (r orderProductSnapshotRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 15 {
		return errors.New("expected product scan destinations")
	}
	createdAt := r.product.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Unix(1, 0).UTC()
	}
	updatedAt := r.product.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	values := []any{
		r.product.ID,
		r.product.SKU,
		r.product.Title,
		r.product.Description,
		r.product.Category,
		r.product.CoverURL,
		r.product.GrantType,
		r.product.GrantKey,
		r.product.PriceCredits,
		r.product.Stock,
		r.product.SalesCount,
		string(r.product.Status),
		r.product.Sort,
		createdAt,
		updatedAt,
	}
	for i, value := range values {
		switch out := dest[i].(type) {
		case *int64:
			v, ok := value.(int64)
			if !ok {
				return errors.New("expected int64 value")
			}
			*out = v
		case *int32:
			v, ok := value.(int32)
			if !ok {
				return errors.New("expected int32 value")
			}
			*out = v
		case *string:
			v, ok := value.(string)
			if !ok {
				return errors.New("expected string value")
			}
			*out = v
		case *time.Time:
			v, ok := value.(time.Time)
			if !ok {
				return errors.New("expected time value")
			}
			*out = v
		default:
			return errors.New("unexpected scan destination")
		}
	}
	return nil
}

type orderProductSnapshotRows struct {
	products []domain.Product
	index    int
	closed   bool
}

func (r *orderProductSnapshotRows) Close() {
	r.closed = true
}

func (*orderProductSnapshotRows) Err() error {
	return nil
}

func (*orderProductSnapshotRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (*orderProductSnapshotRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *orderProductSnapshotRows) Next() bool {
	if r.index+1 >= len(r.products) {
		r.Close()
		return false
	}
	r.index++
	return true
}

func (r *orderProductSnapshotRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.products) {
		return errors.New("scan called without a current product row")
	}
	return orderProductSnapshotRow{product: r.products[r.index]}.Scan(dest...)
}

func (*orderProductSnapshotRows) Values() ([]any, error) {
	return nil, nil
}

func (*orderProductSnapshotRows) RawValues() [][]byte {
	return nil
}

func (*orderProductSnapshotRows) Conn() *pgx.Conn {
	return nil
}
