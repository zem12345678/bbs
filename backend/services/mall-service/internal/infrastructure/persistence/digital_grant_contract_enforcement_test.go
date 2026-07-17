package persistence

import (
	"strings"
	"testing"

	domain "mall-service/internal/domain/mall"
)

func TestNormalizeProductGrantContractForDigitalProducts(t *testing.T) {
	tests := []struct {
		name      string
		product   domain.Product
		grantType string
		grantKey  string
	}{
		{
			name:      "generic digital product uses SKU",
			product:   domain.Product{Category: "digital", SKU: " E2E-DIRECT "},
			grantType: "digital",
			grantKey:  "e2e-direct",
		},
		{
			name:      "digital product infers membership type",
			product:   domain.Product{Category: "digital", SKU: "VIP-MONTH", GrantKey: " VIP-MONTH "},
			grantType: "membership",
			grantKey:  "vip-month",
		},
		{
			name:      "physical product keeps empty grant",
			product:   domain.Product{Category: "physical", SKU: "STICKER-PACK"},
			grantType: "",
			grantKey:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeProductGrantContract(tt.product)
			if got.GrantType != tt.grantType || got.GrantKey != tt.grantKey {
				t.Fatalf("normalized grant = %q/%q, want %q/%q", got.GrantType, got.GrantKey, tt.grantType, tt.grantKey)
			}
		})
	}
}

func TestOrderItemForCurrentProductPersistsDigitalGrantContract(t *testing.T) {
	item := orderItemForCurrentProduct(domain.Product{ID: 101, SKU: "E2E-DIRECT", Category: "digital"}, 1)
	if item.GrantType != "digital" || item.GrantKey != "e2e-direct" {
		t.Fatalf("order item grant = %q/%q, want digital/e2e-direct", item.GrantType, item.GrantKey)
	}
}

func TestDigitalGrantContractSchemaEnforcesOrderSnapshots(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_products_digital_grant_contract_check",
		"mall_order_items_digital_grant_contract_check",
		"idx_mall_order_items_entitlement_grant",
		"mall_digital_entitlements_order_grant_fkey",
		"FOREIGN KEY (order_id, product_id, grant_type, grant_key) REFERENCES mall_order_items(order_id, product_id, grant_type, grant_key) ON DELETE CASCADE NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing digital grant contract enforcement %q", want)
		}
	}
}
