package persistence

import (
	"context"
	"errors"
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
	product       domain.Product
	included      bool
	createdReview domain.ProductReview
	productErr    error
	insertErr     error
	queryRowCount int
}

func (q *productReviewQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *productReviewQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *productReviewQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	q.queryRowCount++
	switch q.queryRowCount {
	case 1:
		if q.productErr != nil {
			return productReviewScanRow{err: q.productErr}
		}
		return productReviewScanRow{values: productValues(q.product)}
	case 2:
		return productReviewScanRow{values: []any{q.included}}
	case 3:
		if q.insertErr != nil {
			return productReviewScanRow{err: q.insertErr}
		}
		return productReviewScanRow{values: productReviewValues(q.createdReview)}
	default:
		return productReviewScanRow{err: errors.New("unexpected product review query")}
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
