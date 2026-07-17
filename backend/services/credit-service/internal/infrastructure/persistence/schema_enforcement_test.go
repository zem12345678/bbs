package persistence

import "strings"
import "testing"

func TestEnsureSchemaEnforcesCreditLedgerInvariants(t *testing.T) {
	for _, want := range []string{
		"credit_balances_nonnegative_check",
		"credit_ledger_snapshot_check",
		"delta <> 0",
		"credit_reservations_lifecycle_check",
		"status = 'ACTIVE' AND settled_at IS NULL",
		"status IN ('RELEASED', 'SETTLED') AND settled_at IS NOT NULL",
	} {
		if !strings.Contains(schemaSQL, want) {
			t.Fatalf("schemaSQL missing credit invariant %q", want)
		}
	}
}
