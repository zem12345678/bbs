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
