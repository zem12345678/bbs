package persistence

import (
	"strings"
	"testing"
)

func TestProductSchemaEnforcesLifecycleFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_products_lifecycle_check",
		"BTRIM(sku) <> ''",
		"BTRIM(title) <> ''",
		"sales_count >= 0",
		"status = UPPER(TRIM(status))",
		"status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing product lifecycle constraint %q", want)
		}
	}
}

func TestProductCategorySchemaEnforcesLifecycleFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_product_categories_lifecycle_check",
		"BTRIM(slug) <> ''",
		"BTRIM(name) <> ''",
		"status = UPPER(TRIM(status))",
		"status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing product category lifecycle constraint %q", want)
		}
	}
}
