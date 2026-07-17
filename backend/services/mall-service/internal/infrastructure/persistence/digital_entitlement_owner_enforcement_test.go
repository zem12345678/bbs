package persistence

import (
	"strings"
	"testing"
)

func TestDigitalEntitlementSchemaEnforcesOrderOwner(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"idx_mall_orders_id_user",
		"mall_digital_entitlements_order_user_fkey",
		"FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing entitlement owner enforcement %q", want)
		}
	}
}
