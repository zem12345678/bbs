package persistence

import (
	"strings"
	"testing"
)

func TestOrderItemSchemaEnforcesSnapshotFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_order_items_snapshot_check",
		"BTRIM(sku) <> ''",
		"BTRIM(title) <> ''",
		"BTRIM(category) <> ''",
		"grant_type = LOWER(TRIM(grant_type))",
		"grant_key = LOWER(TRIM(grant_key))",
		"grant_type = '' AND grant_key = ''",
		"grant_type IN ('badge', 'theme', 'membership', 'digital')",
		"grant_key <> ''",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order item snapshot constraint %q", want)
		}
	}
}
