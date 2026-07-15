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

func TestIsProductFavoriteRequiresActiveProduct(t *testing.T) {
	repo := &productReviewRepoStub{
		product: domain.Product{ID: 101, Status: domain.ProductStatusDraft},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.IsProductFavorite(context.Background(), ProductFavoriteCommand{UserID: 42, ProductID: 101})

	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("IsProductFavorite() error = %v, want %v", err, domain.ErrProductNotFound)
	}
	if repo.favoriteCalls != 0 {
		t.Fatalf("IsProductFavorite() repo calls = %d, want 0", repo.favoriteCalls)
	}
}

func TestIsProductFavoriteForActiveProductChecksFavorite(t *testing.T) {
	repo := &productReviewRepoStub{
		product:  domain.Product{ID: 101, Status: domain.ProductStatusActive},
		favorite: true,
	}
	svc := NewService(repo, nil, time.Minute)

	favorited, err := svc.IsProductFavorite(context.Background(), ProductFavoriteCommand{UserID: 42, ProductID: 101})

	if err != nil {
		t.Fatalf("IsProductFavorite() error = %v", err)
	}
	if !favorited {
		t.Fatal("IsProductFavorite() = false, want true")
	}
	if repo.favoriteUserID != 42 || repo.favoriteProductID != 101 {
		t.Fatalf("favorite lookup = %d/%d, want 42/101", repo.favoriteUserID, repo.favoriteProductID)
	}
}

func TestCreateProductReviewRequiresActiveProduct(t *testing.T) {
	repo := &productReviewRepoStub{
		product: domain.Product{ID: 101, Status: domain.ProductStatusArchived},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateProductReview(context.Background(), CreateProductReviewCommand{
		UserID:    42,
		ProductID: 101,
		OrderID:   9001,
		Rating:    5,
		Content:   "很好用",
	})

	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("CreateProductReview() error = %v, want %v", err, domain.ErrProductNotFound)
	}
	if repo.createReviewCalls != 0 {
		t.Fatalf("CreateProductReview() repo calls = %d, want 0", repo.createReviewCalls)
	}
}

func TestCreateProductReviewForActiveProductCreatesPendingReview(t *testing.T) {
	repo := &productReviewRepoStub{
		product: domain.Product{ID: 101, Status: domain.ProductStatusActive},
		createdReview: domain.ProductReview{
			ID:        8001,
			ProductID: 101,
			OrderID:   9001,
			UserID:    42,
			Status:    domain.ProductReviewStatusPending,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	review, err := svc.CreateProductReview(context.Background(), CreateProductReviewCommand{
		UserID:    42,
		ProductID: 101,
		OrderID:   9001,
		Rating:    5,
		Content:   " 很好用 ",
	})

	if err != nil {
		t.Fatalf("CreateProductReview() error = %v", err)
	}
	if review.ID != 8001 || repo.createReview.ProductID != 101 || repo.createReview.Status != domain.ProductReviewStatusPending {
		t.Fatalf("created review = %+v request=%+v, want pending product review", review, repo.createReview)
	}
	if repo.createReview.Content != "很好用" {
		t.Fatalf("review content = %q, want trimmed content", repo.createReview.Content)
	}
}

func TestListReviewableOrdersRequiresActiveProduct(t *testing.T) {
	repo := &productReviewRepoStub{
		product: domain.Product{ID: 101, Status: domain.ProductStatusArchived},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.ListReviewableOrders(context.Background(), ListReviewableOrdersCommand{UserID: 42, ProductID: 101})

	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("ListReviewableOrders() error = %v, want %v", err, domain.ErrProductNotFound)
	}
	if repo.reviewableCalls != 0 {
		t.Fatalf("ListReviewableOrders() repo calls = %d, want 0", repo.reviewableCalls)
	}
}

func TestListReviewableOrdersForActiveProductForwardsQuery(t *testing.T) {
	repo := &productReviewRepoStub{
		product: domain.Product{ID: 101, Status: domain.ProductStatusActive},
		reviewableOrders: []domain.Order{
			{ID: 9001, UserID: 42, Status: domain.OrderStatusCompleted},
		},
		reviewableTotal: 1,
	}
	svc := NewService(repo, nil, time.Minute)

	orders, total, err := svc.ListReviewableOrders(context.Background(), ListReviewableOrdersCommand{UserID: 42, ProductID: 101, Limit: 5, Offset: 2})

	if err != nil {
		t.Fatalf("ListReviewableOrders() error = %v", err)
	}
	if total != 1 || len(orders) != 1 || orders[0].ID != 9001 {
		t.Fatalf("orders total=%d items=%+v, want one reviewable order", total, orders)
	}
	if repo.reviewableProductID != 101 || repo.reviewableQuery.UserID != 42 {
		t.Fatalf("query = %+v product=%d, want user 42 product 101", repo.reviewableQuery, repo.reviewableProductID)
	}
	if repo.reviewableQuery.Limit != 5 || repo.reviewableQuery.Offset != 2 {
		t.Fatalf("pagination = %d/%d, want 5/2", repo.reviewableQuery.Limit, repo.reviewableQuery.Offset)
	}
}

type productReviewRepoStub struct {
	domain.Repository
	product             domain.Product
	productErr          error
	reviews             []domain.ProductReview
	total               int64
	listReviewsQuery    domain.ProductReviewListQuery
	listReviewsCalls    int
	favorite            bool
	favoriteCalls       int
	favoriteUserID      int64
	favoriteProductID   int64
	createReview        domain.ProductReview
	createdReview       domain.ProductReview
	createReviewCalls   int
	reviewableOrders    []domain.Order
	reviewableTotal     int64
	reviewableQuery     domain.OrderListQuery
	reviewableProductID int64
	reviewableCalls     int
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

func (r *productReviewRepoStub) IsProductFavorite(_ context.Context, userID int64, productID int64) (bool, error) {
	r.favoriteCalls++
	r.favoriteUserID = userID
	r.favoriteProductID = productID
	return r.favorite, nil
}

func (r *productReviewRepoStub) CreateProductReview(_ context.Context, review domain.ProductReview) (domain.ProductReview, error) {
	r.createReviewCalls++
	r.createReview = review
	if r.createdReview.ID == 0 {
		return review, nil
	}
	return r.createdReview, nil
}

func (r *productReviewRepoStub) ListReviewableOrders(_ context.Context, query domain.OrderListQuery, productID int64) ([]domain.Order, int64, error) {
	r.reviewableCalls++
	r.reviewableQuery = query
	r.reviewableProductID = productID
	return r.reviewableOrders, r.reviewableTotal, nil
}
