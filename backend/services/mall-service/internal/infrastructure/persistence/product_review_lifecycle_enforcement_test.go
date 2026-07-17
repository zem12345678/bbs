package persistence

import (
	"strings"
	"testing"
)

func TestProductReviewSchemaEnforcesLifecycleFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_product_reviews_lifecycle_check",
		"user_id > 0",
		"status = UPPER(TRIM(status))",
		"status IN ('PENDING', 'PUBLISHED', 'HIDDEN')",
		"BTRIM(content) <> ''",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing product review lifecycle constraint %q", want)
		}
	}
}
