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
