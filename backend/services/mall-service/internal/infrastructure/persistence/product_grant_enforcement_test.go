package persistence

import (
	"strings"
	"testing"
)

func TestProductSchemaEnforcesGrantContract(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_products_grant_contract_check",
		"grant_type = LOWER(TRIM(grant_type))",
		"grant_key = LOWER(TRIM(grant_key))",
		"grant_type = '' AND grant_key = ''",
		"grant_type IN ('badge', 'theme', 'membership', 'digital')",
		"grant_key <> ''",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing product grant constraint %q", want)
		}
	}
}

func TestProductSchemaBackfillsDigitalGrantsAtomically(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	backfill := "UPDATE mall_products"
	backfillAt := strings.Index(joined, backfill)
	if backfillAt < 0 {
		t.Fatalf("schemaStatements missing atomic digital grant backfill %q", backfill)
	}
	for _, want := range []string{
		"SET grant_key = CASE",
		"grant_type = CASE",
		"WHERE category = 'digital'",
		"AND (COALESCE(grant_key, '') = '' OR COALESCE(grant_type, '') = '')",
	} {
		if !strings.Contains(joined[backfillAt:], want) {
			t.Fatalf("atomic digital grant backfill missing %q", want)
		}
	}
	if contractAt := strings.Index(joined, "mall_products_grant_contract_check"); contractAt < 0 || backfillAt > contractAt {
		t.Fatal("atomic digital grant backfill must run before grant contract enforcement")
	}
}
