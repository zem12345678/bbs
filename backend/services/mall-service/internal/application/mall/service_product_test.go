package mall

import (
	"strings"
	"testing"

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
