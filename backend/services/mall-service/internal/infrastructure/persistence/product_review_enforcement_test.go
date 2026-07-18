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

func TestCreateProductReviewForOrderRejectsOwnershipAndState(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	review := domain.ProductReview{
		ProductID: 101,
		OrderID:   9001,
		UserID:    7,
		Rating:    5,
		Content:   "很好用",
		Status:    domain.ProductReviewStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := createProductReviewForOrder(context.Background(), &productReviewQueryer{}, review, domain.Order{ID: 9001, UserID: 8, Status: domain.OrderStatusCompleted}); !errors.Is(err, domain.ErrOrderOwnerMismatch) {
		t.Fatalf("owner mismatch error = %v, want owner mismatch", err)
	}
	if _, err := createProductReviewForOrder(context.Background(), &productReviewQueryer{}, review, domain.Order{ID: 9001, UserID: 7, Status: domain.OrderStatusPaid}); !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("state error = %v, want invalid order state", err)
	}
}

func TestCreateProductReviewForOrderRequiresProductAndOrderItem(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 30, 0, 0, time.UTC)
	review := domain.ProductReview{
		ProductID: 101,
		OrderID:   9001,
		UserID:    7,
		Rating:    5,
		Content:   "很好用",
		Status:    domain.ProductReviewStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Status: domain.OrderStatusCompleted,
	}
	product := domain.Product{ID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Status: domain.ProductStatusActive, CreatedAt: now, UpdatedAt: now}

	if _, err := createProductReviewForOrder(context.Background(), &productReviewQueryer{productErr: pgx.ErrNoRows}, review, order); !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("missing product error = %v, want product not found", err)
	}
	if _, err := createProductReviewForOrder(context.Background(), &productReviewQueryer{product: product}, review, order); !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("missing order item error = %v, want invalid order state", err)
	}
}

func TestCreateProductReviewForOrderRequiresActiveProduct(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 45, 0, 0, time.UTC)
	review := domain.ProductReview{
		ProductID: 101,
		OrderID:   9001,
		UserID:    7,
		Rating:    5,
		Content:   "很好用",
		Status:    domain.ProductReviewStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Status: domain.OrderStatusCompleted,
	}
	db := &productReviewQueryer{
		product: domain.Product{ID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Status: domain.ProductStatusArchived, CreatedAt: now, UpdatedAt: now},
	}

	_, err := createProductReviewForOrder(context.Background(), db, review, order)

	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("inactive product error = %v, want product not found", err)
	}
	if db.queryRowCount != 1 {
		t.Fatalf("QueryRow() calls = %d, want 1 before order item and insert checks", db.queryRowCount)
	}
}

func TestCreateProductReviewForOrderRejectsOrderWithRefundRequest(t *testing.T) {
	now := time.Date(2026, 7, 18, 4, 40, 0, 0, time.UTC)
	review := domain.ProductReview{
		ProductID: 101,
		OrderID:   9001,
		UserID:    7,
		Rating:    5,
		Content:   "很好用",
		Status:    domain.ProductReviewStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	order := domain.Order{ID: 9001, UserID: 7, Status: domain.OrderStatusCompleted}
	db := &productReviewQueryer{
		product:         domain.Product{ID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Status: domain.ProductStatusActive, CreatedAt: now, UpdatedAt: now},
		included:        true,
		refundRequested: true,
		createdReview: domain.ProductReview{
			ID: 8001,
		},
	}

	created, err := createProductReviewForOrder(context.Background(), db, review, order)

	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("createProductReviewForOrder() error = %v, want invalid order state", err)
	}
	if created.ID != 0 {
		t.Fatalf("createProductReviewForOrder() created %+v, want no review", created)
	}
}

func TestEnsureProductReviewPublicationAllowedRejectsBlockedOrders(t *testing.T) {
	tests := []struct {
		name            string
		orderStatus     domain.OrderStatus
		refundRequested bool
	}{
		{
			name:        "refunded order",
			orderStatus: domain.OrderStatusRefunded,
		},
		{
			name:            "open refund request",
			orderStatus:     domain.OrderStatusCompleted,
			refundRequested: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := &productReviewPublicationQueryer{
				orderID:         9001,
				orderStatus:     test.orderStatus,
				refundRequested: test.refundRequested,
			}

			err := ensureProductReviewPublicationAllowed(context.Background(), db, 8001)

			if !errors.Is(err, domain.ErrInvalidOrderState) {
				t.Fatalf("ensureProductReviewPublicationAllowed() error = %v, want invalid order state", err)
			}
		})
	}
}

func TestEnsureProductReviewPublicationAllowedLocksCompletedOrder(t *testing.T) {
	db := &productReviewPublicationQueryer{
		orderID:     9001,
		orderStatus: domain.OrderStatusCompleted,
	}

	if err := ensureProductReviewPublicationAllowed(context.Background(), db, 8001); err != nil {
		t.Fatalf("ensureProductReviewPublicationAllowed() error = %v", err)
	}
	for _, want := range []string{
		"FROM mall_product_reviews r",
		"JOIN mall_orders o ON o.id = r.order_id",
		"FOR UPDATE OF o",
	} {
		if !strings.Contains(db.publicationQuery, want) {
			t.Fatalf("publication query = %q, want %q", db.publicationQuery, want)
		}
	}
	if len(db.publicationArgs) != 1 || db.publicationArgs[0] != int64(8001) {
		t.Fatalf("publication args = %+v, want [8001]", db.publicationArgs)
	}
}

func TestEnsureProductReviewPublicationAllowedMapsMissingReview(t *testing.T) {
	db := &productReviewPublicationQueryer{orderErr: pgx.ErrNoRows}

	err := ensureProductReviewPublicationAllowed(context.Background(), db, 8001)

	if !errors.Is(err, domain.ErrProductReviewNotFound) {
		t.Fatalf("ensureProductReviewPublicationAllowed() error = %v, want product review not found", err)
	}
}

func TestHidePublishedProductReviewsForRefundOnlyTargetsPublishedReviews(t *testing.T) {
	now := time.Date(2026, 7, 18, 6, 20, 0, 0, time.UTC)
	db := &refundReviewStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")}

	if err := hidePublishedProductReviewsForRefund(context.Background(), db, 9001, now); err != nil {
		t.Fatalf("hidePublishedProductReviewsForRefund() error = %v", err)
	}
	for _, want := range []string{
		"UPDATE mall_product_reviews",
		"WHERE order_id = $1",
		"AND status = $4",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("hide review query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{int64(9001), string(domain.ProductReviewStatusHidden), now, string(domain.ProductReviewStatusPublished)}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("hide review args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("hide review arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestProductReviewSchemaHidesPublishedReviewsAfterApprovedRefunds(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"UPDATE mall_product_reviews review",
		"refund.order_id = review.order_id",
		"refund.status = 'APPROVED'",
		"review.status = 'PUBLISHED'",
		"status = 'HIDDEN'",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing approved refund review cleanup %q", want)
		}
	}
}

func TestCreateProductReviewForOrderMapsDuplicateReview(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	review := domain.ProductReview{
		ProductID: 101,
		OrderID:   9001,
		UserID:    7,
		Rating:    5,
		Content:   "很好用",
		Status:    domain.ProductReviewStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Status: domain.OrderStatusCompleted,
	}

	created, err := createProductReviewForOrder(context.Background(), &productReviewQueryer{
		product:   domain.Product{ID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Description: "", Category: "digital", Status: domain.ProductStatusActive, CreatedAt: now, UpdatedAt: now},
		included:  true,
		insertErr: &pgconn.PgError{Code: "23505"},
	}, review, order)
	if !errors.Is(err, domain.ErrDuplicateReference) {
		t.Fatalf("duplicate review error = %v, want duplicate reference", err)
	}
	if created.ID != 0 {
		t.Fatalf("duplicate review returned %+v, want zero value", created)
	}
}

func TestCreateProductReviewForOrderCreatesPendingReview(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 30, 0, 0, time.UTC)
	review := domain.ProductReview{
		ProductID: 101,
		OrderID:   9001,
		UserID:    7,
		Rating:    5,
		Content:   "很好用",
		Status:    domain.ProductReviewStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Status: domain.OrderStatusCompleted,
	}
	product := domain.Product{
		ID:           101,
		SKU:          "VIP-MONTH",
		Title:        "会员月卡",
		Description:  "数字权益",
		Category:     "digital",
		CoverURL:     "",
		GrantType:    "membership",
		GrantKey:     "vip-month",
		PriceCredits: 300,
		Stock:        100,
		SalesCount:   0,
		Status:       domain.ProductStatusActive,
		Sort:         1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	created, err := createProductReviewForOrder(context.Background(), &productReviewQueryer{
		product:  product,
		included: true,
		createdReview: domain.ProductReview{
			ID:           8001,
			ProductID:    101,
			ProductSKU:   "VIP-MONTH",
			ProductTitle: "会员月卡",
			OrderID:      9001,
			UserID:       7,
			Rating:       5,
			Content:      "很好用",
			Status:       domain.ProductReviewStatusPending,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}, review, order)
	if err != nil {
		t.Fatalf("createProductReviewForOrder() error = %v", err)
	}
	if created.ID != 8001 || created.Status != domain.ProductReviewStatusPending || created.Content != "很好用" {
		t.Fatalf("created review = %+v, want persisted pending review", created)
	}
}

type productReviewQueryer struct {
	product         domain.Product
	included        bool
	refundRequested bool
	createdReview   domain.ProductReview
	productErr      error
	insertErr       error
	queryRowCount   int
}

type productReviewPublicationQueryer struct {
	orderID          int64
	orderStatus      domain.OrderStatus
	refundRequested  bool
	orderErr         error
	publicationQuery string
	publicationArgs  []any
}

type refundReviewStateQueryer struct {
	tag   pgconn.CommandTag
	query string
	args  []any
}

func (q *refundReviewStateQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	q.query = query
	q.args = args
	return q.tag, nil
}

func (*refundReviewStateQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (*refundReviewStateQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (q *productReviewQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *productReviewQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *productReviewQueryer) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	q.queryRowCount++
	switch {
	case strings.Contains(query, "INSERT INTO mall_product_reviews"):
		if q.insertErr != nil {
			return productReviewScanRow{err: q.insertErr}
		}
		return productReviewScanRow{values: productReviewValues(q.createdReview)}
	case strings.Contains(query, "FROM mall_refund_requests"):
		return productReviewScanRow{values: []any{q.refundRequested}}
	case strings.Contains(query, "FROM mall_products"):
		if q.productErr != nil {
			return productReviewScanRow{err: q.productErr}
		}
		return productReviewScanRow{values: productValues(q.product)}
	case strings.Contains(query, "FROM mall_order_items"):
		return productReviewScanRow{values: []any{q.included}}
	default:
		return productReviewScanRow{err: errors.New("unexpected product review query")}
	}
}

func (*productReviewPublicationQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (*productReviewPublicationQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *productReviewPublicationQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	switch {
	case strings.Contains(query, "FROM mall_product_reviews r"):
		q.publicationQuery = query
		q.publicationArgs = args
		if q.orderErr != nil {
			return productReviewScanRow{err: q.orderErr}
		}
		return productReviewScanRow{values: []any{q.orderID, string(q.orderStatus)}}
	case strings.Contains(query, "FROM mall_refund_requests"):
		return productReviewScanRow{values: []any{q.refundRequested}}
	default:
		return productReviewScanRow{err: errors.New("unexpected product review publication query")}
	}
}

type productReviewScanRow struct {
	values []any
	err    error
}

func (r productReviewScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return testScanner(r.values).Scan(dest...)
}

func productValues(product domain.Product) []any {
	return []any{
		product.ID,
		product.SKU,
		product.Title,
		product.Description,
		product.Category,
		product.CoverURL,
		product.GrantType,
		product.GrantKey,
		product.PriceCredits,
		product.Stock,
		product.SalesCount,
		string(product.Status),
		product.Sort,
		product.CreatedAt,
		product.UpdatedAt,
	}
}

func productReviewValues(review domain.ProductReview) []any {
	return []any{
		review.ID,
		review.ProductID,
		review.ProductSKU,
		review.ProductTitle,
		review.OrderID,
		review.UserID,
		review.Rating,
		review.Content,
		string(review.Status),
		review.CreatedAt,
		review.UpdatedAt,
	}
}
