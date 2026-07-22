package persistence

import "strings"
import "testing"

func TestEnsureSchemaEnforcesCreditLedgerInvariants(t *testing.T) {
	for _, want := range []string{
		"credit_balances_nonnegative_check",
		"idx_credit_balances_leaderboard",
		"credit_leaderboard_state",
		"credit_balances_leaderboard_revision",
		"credit_ledger_snapshot_check",
		"delta <> 0",
		"credit_reservations_lifecycle_check",
		"status = 'ACTIVE' AND settled_at IS NULL",
		"status IN ('RELEASED', 'SETTLED') AND settled_at IS NOT NULL",
		"check_ins_valid_check",
		"consecutive_days > 0",
	} {
		if !strings.Contains(schemaSQL, want) {
			t.Fatalf("schemaSQL missing credit invariant %q", want)
		}
	}
	if strings.Contains(schemaSQL, "DROP TRIGGER IF EXISTS credit_balances_leaderboard_revision") {
		t.Fatal("schemaSQL must not remove the leaderboard revision trigger during service startup")
	}
}
