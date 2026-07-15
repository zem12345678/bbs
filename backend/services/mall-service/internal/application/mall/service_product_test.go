package mall

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestCommandToProductRejectsGrantTypeWithoutGrantKey(t *testing.T) {
	_, err := commandToProduct(CreateProductCommand{
		SKU:          "VIP-MISSING-KEY",
		Title:        "会员权益",
		Category:     "digital",
		GrantType:    "membership",
		PriceCredits: 120,
		Stock:        10,
		Status:       domain.ProductStatusActive,
	})
	if err == nil || !strings.Contains(err.Error(), "grant_key is required") {
		t.Fatalf("commandToProduct() error = %v, want grant_key validation", err)
	}
}

func TestCommandToProductInfersGrantTypeFromGrantKey(t *testing.T) {
	product, err := commandToProduct(CreateProductCommand{
		SKU:          "VIP-MONTH",
		Title:        "会员权益",
		Category:     "digital",
		GrantKey:     " vip-month ",
		PriceCredits: 120,
		Stock:        10,
		Status:       domain.ProductStatusActive,
	})
	if err != nil {
		t.Fatalf("commandToProduct() error = %v", err)
	}
	if product.GrantType != "membership" || product.GrantKey != "vip-month" {
		t.Fatalf("grant = (%q, %q), want membership/vip-month", product.GrantType, product.GrantKey)
	}
}

func TestGetProductRequiresActiveProduct(t *testing.T) {
	repo := &orderRepoStub{products: map[int64]domain.Product{
		101: {ID: 101, Status: domain.ProductStatusDraft},
	}}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.GetProduct(context.Background(), 101)

	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("GetProduct() error = %v, want %v", err, domain.ErrProductNotFound)
	}
}

func TestGetProductReturnsActiveProduct(t *testing.T) {
	repo := &orderRepoStub{products: map[int64]domain.Product{
		101: {ID: 101, Title: "Theme Pro", Status: domain.ProductStatusActive},
	}}
	svc := NewService(repo, nil, time.Minute)

	product, err := svc.GetProduct(context.Background(), 101)

	if err != nil {
		t.Fatalf("GetProduct() error = %v", err)
	}
	if product.ID != 101 || product.Status != domain.ProductStatusActive {
		t.Fatalf("product = %+v, want active product 101", product)
	}
}

func TestListProductReviewsRequiresActiveProduct(t *testing.T) {
	repo := &productReviewRepoStub{
		product: domain.Product{ID: 101, Status: domain.ProductStatusArchived},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.ListProductReviews(context.Background(), ListProductReviewsCommand{ProductID: 101})

	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("ListProductReviews() error = %v, want %v", err, domain.ErrProductNotFound)
	}
	if repo.listReviewsCalls != 0 {
		t.Fatalf("ListProductReviews() repo calls = %d, want 0", repo.listReviewsCalls)
	}
}

func TestListProductReviewsForActiveProductUsesPublishedFilter(t *testing.T) {
	repo := &productReviewRepoStub{
		product: domain.Product{ID: 101, Status: domain.ProductStatusActive},
		reviews: []domain.ProductReview{
			{ID: 9001, ProductID: 101, Status: domain.ProductReviewStatusPublished},
		},
		total: 1,
	}
	svc := NewService(repo, nil, time.Minute)

	reviews, total, err := svc.ListProductReviews(context.Background(), ListProductReviewsCommand{ProductID: 101, Limit: 5, Offset: 2})

	if err != nil {
		t.Fatalf("ListProductReviews() error = %v", err)
	}
	if total != 1 || len(reviews) != 1 || reviews[0].ID != 9001 {
		t.Fatalf("reviews total=%d items=%+v, want one review", total, reviews)
	}
	if repo.listReviewsQuery.ProductID != 101 || repo.listReviewsQuery.Status != domain.ProductReviewStatusPublished {
		t.Fatalf("query = %+v, want product 101 published reviews", repo.listReviewsQuery)
	}
	if repo.listReviewsQuery.Limit != 5 || repo.listReviewsQuery.Offset != 2 {
		t.Fatalf("pagination = %d/%d, want 5/2", repo.listReviewsQuery.Limit, repo.listReviewsQuery.Offset)
	}
}

type productReviewRepoStub struct {
	domain.Repository
	product          domain.Product
	productErr       error
	reviews          []domain.ProductReview
	total            int64
	listReviewsQuery domain.ProductReviewListQuery
	listReviewsCalls int
}

func (r *productReviewRepoStub) GetProduct(_ context.Context, productID int64) (domain.Product, error) {
	if r.productErr != nil {
		return domain.Product{}, r.productErr
	}
	if r.product.ID != productID {
		return domain.Product{}, domain.ErrProductNotFound
	}
	return r.product, nil
}

func (r *productReviewRepoStub) ListProductReviews(_ context.Context, query domain.ProductReviewListQuery) ([]domain.ProductReview, int64, error) {
	r.listReviewsCalls++
	r.listReviewsQuery = query
	return r.reviews, r.total, nil
}
