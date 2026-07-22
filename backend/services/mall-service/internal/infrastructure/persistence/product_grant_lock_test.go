package persistence

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestEnsureProductGrantMutableSkipsUnchangedGrant(t *testing.T) {
	db := &productGrantLockQueryer{locked: true}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, GrantKey: "vip-month"},
		domain.Product{ID: 101, GrantType: " Membership ", GrantKey: " VIP-MONTH "},
	)
	if err != nil {
		t.Fatalf("ensureProductGrantMutable() error = %v, want nil", err)
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0 for unchanged grant", db.queryRows)
	}
}

func TestEnsureProductGrantMutableAllowsUnsoldGrantChange(t *testing.T) {
	db := &productGrantLockQueryer{}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, GrantType: "membership", GrantKey: "vip-month"},
		domain.Product{ID: 101, GrantType: "theme", GrantKey: "theme-pro"},
	)
	if err != nil {
		t.Fatalf("ensureProductGrantMutable() error = %v, want nil", err)
	}
	if db.queryRows != 1 {
		t.Fatalf("QueryRow() calls = %d, want 1", db.queryRows)
	}
	query := db.query
	for _, want := range []string{
		"mall_order_items",
		"mall_orders",
		"oi.product_id = $1",
		"o.status IN ($2, $3, $4, $5)",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("grant lock query = %q, want %q", query, want)
		}
	}
	wantArgs := []any{
		int64(101),
		string(domain.OrderStatusPaid),
		string(domain.OrderStatusShipped),
		string(domain.OrderStatusCompleted),
		string(domain.OrderStatusRefunded),
	}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("grant lock args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("grant lock arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestEnsureProductGrantMutableBlocksSoldGrantChange(t *testing.T) {
	db := &productGrantLockQueryer{locked: true}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, GrantType: "membership", GrantKey: "vip-month"},
		domain.Product{ID: 101, GrantType: "membership", GrantKey: "vip-year"},
	)
	if !errors.Is(err, domain.ErrProductGrantLocked) {
		t.Fatalf("ensureProductGrantMutable() error = %v, want product grant locked", err)
	}
}

func TestEnsureProductGrantMutableAllowsUnsoldFulfillmentChange(t *testing.T) {
	db := &productGrantLockQueryer{}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, Category: "goods"},
		domain.Product{ID: 101, Category: "digital"},
	)
	if err != nil {
		t.Fatalf("ensureProductGrantMutable() error = %v, want nil", err)
	}
	if db.queryRows != 1 {
		t.Fatalf("QueryRow() calls = %d, want 1", db.queryRows)
	}
}

func TestEnsureProductGrantMutableBlocksSoldFulfillmentChange(t *testing.T) {
	db := &productGrantLockQueryer{locked: true}

	err := ensureProductGrantMutable(context.Background(), db,
		domain.Product{ID: 101, Category: "goods"},
		domain.Product{ID: 101, Category: "digital"},
	)
	if !errors.Is(err, domain.ErrProductFulfillmentLocked) {
		t.Fatalf("ensureProductGrantMutable() error = %v, want product fulfillment locked", err)
	}
}

func TestOpenDigitalGrantOrderGrantsQueriesPendingPayingGrants(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExists: true}

	matches, err := openDigitalGrantOrderGrants(context.Background(), db, 7, 0, []ownedDigitalGrant{{grantType: " Badge ", grantKey: " Badge-Founder "}})
	if err != nil {
		t.Fatalf("openDigitalGrantOrderGrants() error = %v", err)
	}
	if len(matches) != 1 || matches[0].grantType != "badge" || matches[0].grantKey != "badge-founder" {
		t.Fatalf("openDigitalGrantOrderGrants() = %+v, want badge/badge-founder", matches)
	}
	for _, want := range []string{
		"mall_orders o",
		"mall_order_items oi",
		"unnest($2::TEXT[], $3::TEXT[])",
		"o.status IN ($4, $5)",
		"LOWER(TRIM(COALESCE(oi.grant_type, ''))) = requested.grant_type",
		"LOWER(TRIM(COALESCE(oi.grant_key, ''))) = requested.grant_key",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("open theme order query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{
		int64(7),
		[]string{"badge"},
		[]string{"badge-founder"},
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		int64(0),
	}
	assertProductGrantLockArgs(t, "open digital grant order", db.args, wantArgs)
}

func TestOpenDigitalGrantOrderGrantsSkipsInvalidInput(t *testing.T) {
	db := &productGrantLockQueryer{}

	matches, err := openDigitalGrantOrderGrants(context.Background(), db, 7, 0, []ownedDigitalGrant{{grantType: "badge", grantKey: " "}})
	if err != nil {
		t.Fatalf("openDigitalGrantOrderGrants() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("openDigitalGrantOrderGrants() = %+v, want no matches", matches)
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0", db.queryRows)
	}
}

func TestActiveDigitalEntitlementGrantsAppliesMembershipExpiry(t *testing.T) {
	db := &productGrantLockQueryer{}

	_, err := activeDigitalEntitlementGrants(context.Background(), db, 7, []ownedDigitalGrant{{grantType: " Membership ", grantKey: " VIP-MONTH "}})
	if err != nil {
		t.Fatalf("activeDigitalEntitlementGrants() error = %v", err)
	}
	if !strings.Contains(db.query, "requested.grant_type = 'membership' AND de.expires_at IS NOT NULL AND de.expires_at > NOW()") {
		t.Fatalf("active entitlement query = %q, want membership to require future expiry", db.query)
	}
	if !strings.Contains(db.query, "UPPER(TRIM(COALESCE(de.status, ''))) = $4") {
		t.Fatalf("active entitlement query = %q, want normalized status filter", db.query)
	}
	wantArgs := []any{
		int64(7),
		[]string{"membership"},
		[]string{"vip-month"},
		domain.DigitalEntitlementStatusActive,
	}
	assertProductGrantLockArgs(t, "active entitlement", db.args, wantArgs)
}

func TestActiveDigitalEntitlementGrantsAllowsPerpetualNonMembershipGrant(t *testing.T) {
	db := &productGrantLockQueryer{}

	_, err := activeDigitalEntitlementGrants(context.Background(), db, 7, []ownedDigitalGrant{{grantType: " Theme ", grantKey: " Theme-Pro "}})
	if err != nil {
		t.Fatalf("activeDigitalEntitlementGrants() error = %v", err)
	}
	if !strings.Contains(db.query, "requested.grant_type <> 'membership' AND (de.expires_at IS NULL OR de.expires_at > NOW())") {
		t.Fatalf("active entitlement query = %q, want non-membership grants to allow no expiry", db.query)
	}
	wantArgs := []any{
		int64(7),
		[]string{"theme"},
		[]string{"theme-pro"},
		domain.DigitalEntitlementStatusActive,
	}
	assertProductGrantLockArgs(t, "active entitlement", db.args, wantArgs)
}

func TestOpenDigitalGrantOrderGrantsExcludingSkipsCurrentOrder(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExistsExcluding: true}

	matches, err := openDigitalGrantOrderGrants(context.Background(), db, 7, 9001, []ownedDigitalGrant{{grantType: " Membership ", grantKey: " VIP-MONTH "}})
	if err != nil {
		t.Fatalf("openDigitalGrantOrderGrants() error = %v", err)
	}
	if len(matches) != 1 || matches[0].grantType != "membership" || matches[0].grantKey != "vip-month" {
		t.Fatalf("openDigitalGrantOrderGrants() = %+v, want membership/vip-month", matches)
	}
	for _, want := range []string{
		"mall_orders o",
		"mall_order_items oi",
		"o.status IN ($4, $5)",
		"LOWER(TRIM(COALESCE(oi.grant_type, ''))) = requested.grant_type",
		"LOWER(TRIM(COALESCE(oi.grant_key, ''))) = requested.grant_key",
		"($6::BIGINT = 0 OR o.id <> $6::BIGINT)",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("excluded open grant order query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{
		int64(7),
		[]string{"membership"},
		[]string{"vip-month"},
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		int64(9001),
	}
	assertProductGrantLockArgs(t, "excluded open digital grant order", db.args, wantArgs)
}

func TestPrepareOwnedDigitalGrantOrderCreationLocksAndBlocksActiveThemeEntitlement(t *testing.T) {
	db := &productGrantLockQueryer{activeDigitalEntitlementExists: true}
	order := domain.Order{
		UserID:         7,
		IdempotencyKey: "theme-order",
		Items: []domain.OrderItem{
			{GrantType: " Theme ", GrantKey: " Theme-Pro "},
		},
	}

	_, duplicate, err := prepareOwnedDigitalGrantOrderCreation(context.Background(), db, order)
	if !errors.Is(err, domain.ErrActiveThemeEntitlementExists) {
		t.Fatalf("prepareOwnedDigitalGrantOrderCreation() error = %v, want active theme entitlement", err)
	}
	if duplicate {
		t.Fatal("prepareOwnedDigitalGrantOrderCreation() duplicate = true, want false")
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	if !strings.Contains(db.ownedDigitalGrantLockQuery, "pg_advisory_xact_lock") || !strings.Contains(db.ownedDigitalGrantLockQuery, "unnest($2::TEXT[], $3::TEXT[])") || !strings.Contains(db.ownedDigitalGrantLockQuery, "CONCAT($1::BIGINT::text, ':', requested.grant_type, ':', requested.grant_key)") {
		t.Fatalf("advisory lock query = %q, want entitlement-compatible key", db.ownedDigitalGrantLockQuery)
	}
	assertProductGrantLockArgs(t, "advisory lock", db.ownedDigitalGrantLockArgs, []any{int64(7), []string{"theme"}, []string{"theme-pro"}})
	if db.openDigitalGrantOrderQueryRows != 0 {
		t.Fatalf("open digital grant order checks = %d, want 0 after active entitlement block", db.openDigitalGrantOrderQueryRows)
	}
}

func TestPrepareOwnedDigitalGrantOrderCreationRejectsDuplicateThemeGrantBeforeLock(t *testing.T) {
	db := &productGrantLockQueryer{}
	order := domain.Order{
		UserID:         7,
		IdempotencyKey: "theme-order",
		Items: []domain.OrderItem{
			{GrantType: "theme", GrantKey: "theme-pro", Quantity: 2},
		},
	}

	_, duplicate, err := prepareOwnedDigitalGrantOrderCreation(context.Background(), db, order)
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("prepareOwnedDigitalGrantOrderCreation() error = %v, want duplicate theme grant", err)
	}
	if duplicate {
		t.Fatal("prepareOwnedDigitalGrantOrderCreation() duplicate = true, want false")
	}
	if db.ownedDigitalGrantLockQueryRows != 0 {
		t.Fatalf("advisory lock queries = %d, want 0 before locking", db.ownedDigitalGrantLockQueryRows)
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0 before entitlement/order checks", db.queryRows)
	}
}

func TestPrepareOwnedDigitalGrantOrderCreationBlocksOpenThemeOrder(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExists: true}
	order := domain.Order{
		UserID:         7,
		IdempotencyKey: "theme-order",
		Items: []domain.OrderItem{
			{GrantType: "theme", GrantKey: "theme-pro"},
		},
	}

	_, duplicate, err := prepareOwnedDigitalGrantOrderCreation(context.Background(), db, order)
	if !errors.Is(err, domain.ErrPendingThemeOrderExists) {
		t.Fatalf("prepareOwnedDigitalGrantOrderCreation() error = %v, want pending theme order", err)
	}
	if duplicate {
		t.Fatal("prepareOwnedDigitalGrantOrderCreation() duplicate = true, want false")
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	if db.activeDigitalEntitlementQueryRows != 1 {
		t.Fatalf("active entitlement checks = %d, want 1", db.activeDigitalEntitlementQueryRows)
	}
	if db.openDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("open digital grant order checks = %d, want 1", db.openDigitalGrantOrderQueryRows)
	}
}

func TestPrepareOwnedDigitalGrantOrderCreationBatchesMultiGrantChecks(t *testing.T) {
	db := &productGrantLockQueryer{}
	order := domain.Order{
		UserID:         7,
		IdempotencyKey: "multi-grant-order",
		Items: []domain.OrderItem{
			{GrantType: "theme", GrantKey: "theme-pro", Quantity: 1},
			{GrantType: "badge", GrantKey: "badge-founder", Quantity: 1},
			{GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
		},
	}

	_, duplicate, err := prepareOwnedDigitalGrantOrderCreation(context.Background(), db, order)
	if err != nil {
		t.Fatalf("prepareOwnedDigitalGrantOrderCreation() error = %v", err)
	}
	if duplicate {
		t.Fatal("prepareOwnedDigitalGrantOrderCreation() duplicate = true, want false")
	}
	if db.ownedDigitalGrantLockQueryRows != 1 || db.activeDigitalEntitlementQueryRows != 1 || db.openDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("multi-grant queries = lock:%d active:%d open:%d, want 1/1/1", db.ownedDigitalGrantLockQueryRows, db.activeDigitalEntitlementQueryRows, db.openDigitalGrantOrderQueryRows)
	}
	assertProductGrantLockArgs(t, "multi-grant lock", db.ownedDigitalGrantLockArgs, []any{
		int64(7),
		[]string{"badge", "membership", "theme"},
		[]string{"badge-founder", "vip-month", "theme-pro"},
	})
	assertProductGrantLockArgs(t, "multi-grant active entitlement", db.activeDigitalEntitlementArgs, []any{
		int64(7),
		[]string{"badge", "theme"},
		[]string{"badge-founder", "theme-pro"},
		domain.DigitalEntitlementStatusActive,
	})
	assertProductGrantLockArgs(t, "multi-grant open order", db.openDigitalGrantOrderArgs, []any{
		int64(7),
		[]string{"badge", "membership", "theme"},
		[]string{"badge-founder", "vip-month", "theme-pro"},
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		int64(0),
	})
}

func TestPrepareOwnedDigitalGrantOrderCreationBlocksOpenMembershipOrder(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExists: true}
	order := domain.Order{
		UserID:         7,
		IdempotencyKey: "membership-order",
		Items: []domain.OrderItem{
			{GrantType: " Membership ", GrantKey: " VIP-MONTH ", Quantity: 2},
		},
	}

	_, duplicate, err := prepareOwnedDigitalGrantOrderCreation(context.Background(), db, order)
	if !errors.Is(err, domain.ErrPendingMembershipOrderExists) {
		t.Fatalf("prepareOwnedDigitalGrantOrderCreation() error = %v, want pending membership order", err)
	}
	if duplicate {
		t.Fatal("prepareOwnedDigitalGrantOrderCreation() duplicate = true, want false")
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	assertProductGrantLockArgs(t, "advisory lock", db.ownedDigitalGrantLockArgs, []any{int64(7), []string{"membership"}, []string{"vip-month"}})
	if db.activeDigitalEntitlementQueryRows != 0 {
		t.Fatalf("active entitlement checks = %d, want 0 because active membership can renew", db.activeDigitalEntitlementQueryRows)
	}
	if db.openDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("open digital grant order checks = %d, want 1", db.openDigitalGrantOrderQueryRows)
	}
}

func TestPrepareOwnedDigitalGrantOrderCreationAllowsActiveMembershipRenewal(t *testing.T) {
	db := &productGrantLockQueryer{activeDigitalEntitlementExists: true}
	order := domain.Order{
		UserID:         7,
		IdempotencyKey: "membership-renewal",
		Items: []domain.OrderItem{
			{GrantType: "membership", GrantKey: "vip-month", Quantity: 2},
		},
	}

	_, duplicate, err := prepareOwnedDigitalGrantOrderCreation(context.Background(), db, order)
	if err != nil {
		t.Fatalf("prepareOwnedDigitalGrantOrderCreation() error = %v, want nil", err)
	}
	if duplicate {
		t.Fatal("prepareOwnedDigitalGrantOrderCreation() duplicate = true, want false")
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	if db.activeDigitalEntitlementQueryRows != 0 {
		t.Fatalf("active entitlement checks = %d, want 0 because active membership can renew", db.activeDigitalEntitlementQueryRows)
	}
	if db.openDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("open digital grant order checks = %d, want 1", db.openDigitalGrantOrderQueryRows)
	}
}

func TestPrepareOwnedDigitalGrantOrderCreationBlocksDuplicateBadgeGrants(t *testing.T) {
	for _, test := range []struct {
		name   string
		db     productGrantLockQueryer
		item   domain.OrderItem
		err    error
		locked bool
	}{
		{
			name:   "active entitlement",
			db:     productGrantLockQueryer{activeDigitalEntitlementExists: true},
			item:   domain.OrderItem{GrantType: " Badge ", GrantKey: " Badge-Founder ", Quantity: 1},
			err:    domain.ErrActiveBadgeEntitlementExists,
			locked: true,
		},
		{
			name:   "open order",
			db:     productGrantLockQueryer{openDigitalGrantOrderExists: true},
			item:   domain.OrderItem{GrantType: "badge", GrantKey: "badge-founder", Quantity: 1},
			err:    domain.ErrPendingBadgeOrderExists,
			locked: true,
		},
		{
			name: "same order quantity",
			item: domain.OrderItem{GrantType: "badge", GrantKey: "badge-founder", Quantity: 2},
			err:  domain.ErrDuplicateBadgeGrantInOrder,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := test.db
			order := domain.Order{
				UserID:         7,
				IdempotencyKey: "badge-order",
				Items:          []domain.OrderItem{test.item},
			}

			_, duplicate, err := prepareOwnedDigitalGrantOrderCreation(context.Background(), &db, order)
			if !errors.Is(err, test.err) {
				t.Fatalf("prepareOwnedDigitalGrantOrderCreation() error = %v, want %v", err, test.err)
			}
			if duplicate {
				t.Fatal("prepareOwnedDigitalGrantOrderCreation() duplicate = true, want false")
			}
			if test.locked {
				if db.ownedDigitalGrantLockQueryRows != 1 {
					t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
				}
				assertProductGrantLockArgs(t, "advisory lock", db.ownedDigitalGrantLockArgs, []any{int64(7), []string{"badge"}, []string{"badge-founder"}})
			} else if db.ownedDigitalGrantLockQueryRows != 0 {
				t.Fatalf("advisory lock queries = %d, want 0 before locking", db.ownedDigitalGrantLockQueryRows)
			}
		})
	}
}

func TestEnsureNoOtherOpenDigitalGrantOrdersForPaymentBlocksPendingMembershipOrder(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExistsExcluding: true}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{GrantType: " Membership ", GrantKey: " VIP-MONTH ", Quantity: 2},
		},
	}

	err := ensureNoOtherOpenDigitalGrantOrdersForPayment(context.Background(), db, order)
	if !errors.Is(err, domain.ErrPendingMembershipOrderExists) {
		t.Fatalf("ensureNoOtherOpenDigitalGrantOrdersForPayment() error = %v, want pending membership order", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	assertProductGrantLockArgs(t, "advisory lock", db.ownedDigitalGrantLockArgs, []any{int64(7), []string{"membership"}, []string{"vip-month"}})
	if db.activeDigitalEntitlementQueryRows != 0 {
		t.Fatalf("active entitlement checks = %d, want 0 because active membership can renew", db.activeDigitalEntitlementQueryRows)
	}
	if db.excludedOpenDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("excluded open order checks = %d, want 1", db.excludedOpenDigitalGrantOrderQueryRows)
	}
}

func TestEnsureNoOtherOpenDigitalGrantOrdersForPaymentBatchesMultiGrantChecks(t *testing.T) {
	db := &productGrantLockQueryer{}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{GrantType: "theme", GrantKey: "theme-pro", Quantity: 1},
			{GrantType: "badge", GrantKey: "badge-founder", Quantity: 1},
			{GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
		},
	}

	err := ensureNoOtherOpenDigitalGrantOrdersForPayment(context.Background(), db, order)
	if err != nil {
		t.Fatalf("ensureNoOtherOpenDigitalGrantOrdersForPayment() error = %v", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 1 || db.activeDigitalEntitlementQueryRows != 1 || db.excludedOpenDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("payment multi-grant queries = lock:%d active:%d open:%d, want 1/1/1", db.ownedDigitalGrantLockQueryRows, db.activeDigitalEntitlementQueryRows, db.excludedOpenDigitalGrantOrderQueryRows)
	}
	assertProductGrantLockArgs(t, "payment multi-grant lock", db.ownedDigitalGrantLockArgs, []any{
		int64(7),
		[]string{"badge", "membership", "theme"},
		[]string{"badge-founder", "vip-month", "theme-pro"},
	})
	assertProductGrantLockArgs(t, "payment multi-grant active entitlement", db.activeDigitalEntitlementArgs, []any{
		int64(7),
		[]string{"badge", "theme"},
		[]string{"badge-founder", "theme-pro"},
		domain.DigitalEntitlementStatusActive,
	})
	assertProductGrantLockArgs(t, "payment multi-grant open order", db.excludedOpenDigitalGrantOrderArgs, []any{
		int64(7),
		[]string{"badge", "membership", "theme"},
		[]string{"badge-founder", "vip-month", "theme-pro"},
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		int64(9001),
	})
}

func TestEnsureNoOtherOpenDigitalGrantOrdersForPaymentBlocksActiveThemeEntitlement(t *testing.T) {
	db := &productGrantLockQueryer{activeDigitalEntitlementExists: true}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{GrantType: " Theme ", GrantKey: " Theme-Pro ", Quantity: 1},
		},
	}

	err := ensureNoOtherOpenDigitalGrantOrdersForPayment(context.Background(), db, order)
	if !errors.Is(err, domain.ErrActiveThemeEntitlementExists) {
		t.Fatalf("ensureNoOtherOpenDigitalGrantOrdersForPayment() error = %v, want active theme entitlement", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	if db.activeDigitalEntitlementQueryRows != 1 {
		t.Fatalf("active entitlement checks = %d, want 1", db.activeDigitalEntitlementQueryRows)
	}
	if db.excludedOpenDigitalGrantOrderQueryRows != 0 {
		t.Fatalf("excluded open order checks = %d, want 0 after active entitlement block", db.excludedOpenDigitalGrantOrderQueryRows)
	}
}

func TestEnsureNoOtherOpenDigitalGrantOrdersForPaymentAllowsCurrentMembershipOrder(t *testing.T) {
	db := &productGrantLockQueryer{activeDigitalEntitlementExists: true}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{GrantType: "membership", GrantKey: "vip-month", Quantity: 2},
		},
	}

	err := ensureNoOtherOpenDigitalGrantOrdersForPayment(context.Background(), db, order)
	if err != nil {
		t.Fatalf("ensureNoOtherOpenDigitalGrantOrdersForPayment() error = %v, want nil", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	if db.activeDigitalEntitlementQueryRows != 0 {
		t.Fatalf("active entitlement checks = %d, want 0 because active membership can renew", db.activeDigitalEntitlementQueryRows)
	}
	if db.excludedOpenDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("excluded open order checks = %d, want 1", db.excludedOpenDigitalGrantOrderQueryRows)
	}
}

func TestEnsureNoOtherOpenDigitalGrantOrdersForPaymentRejectsDuplicateThemeGrantBeforeLock(t *testing.T) {
	db := &productGrantLockQueryer{}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{GrantType: "theme", GrantKey: "theme-pro", Quantity: 2},
		},
	}

	err := ensureNoOtherOpenDigitalGrantOrdersForPayment(context.Background(), db, order)
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("ensureNoOtherOpenDigitalGrantOrdersForPayment() error = %v, want duplicate theme grant", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 0 {
		t.Fatalf("advisory lock queries = %d, want 0 before locking", db.ownedDigitalGrantLockQueryRows)
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0 before open order checks", db.queryRows)
	}
}

func TestValidateCartOwnedDigitalGrantWriteRejectsDuplicateThemeQuantityBeforeLock(t *testing.T) {
	db := &productGrantLockQueryer{}

	err := validateCartOwnedDigitalGrantWrite(context.Background(), db, 7, 101, domain.Product{
		ID:        101,
		GrantType: "theme",
		GrantKey:  "theme-pro",
	}, 2)
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("validateCartOwnedDigitalGrantWrite() error = %v, want duplicate theme grant", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 0 {
		t.Fatalf("advisory lock queries = %d, want 0 before locking", db.ownedDigitalGrantLockQueryRows)
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0 before entitlement/order checks", db.queryRows)
	}
}

func TestValidateCartOwnedDigitalGrantWriteBlocksActiveBadgeEntitlement(t *testing.T) {
	db := &productGrantLockQueryer{activeDigitalEntitlementExists: true}

	err := validateCartOwnedDigitalGrantWrite(context.Background(), db, 7, 101, domain.Product{
		ID:        101,
		GrantType: " Badge ",
		GrantKey:  " Badge-Founder ",
	}, 1)
	if !errors.Is(err, domain.ErrActiveBadgeEntitlementExists) {
		t.Fatalf("validateCartOwnedDigitalGrantWrite() error = %v, want active badge entitlement", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	if db.activeDigitalEntitlementQueryRows != 1 {
		t.Fatalf("active entitlement checks = %d, want 1", db.activeDigitalEntitlementQueryRows)
	}
	if db.openDigitalGrantOrderQueryRows != 0 {
		t.Fatalf("open digital grant order checks = %d, want 0 after active entitlement block", db.openDigitalGrantOrderQueryRows)
	}
}

func TestValidateCartOwnedDigitalGrantWriteBlocksOpenMembershipOrder(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExists: true}

	err := validateCartOwnedDigitalGrantWrite(context.Background(), db, 7, 101, domain.Product{
		ID:        101,
		GrantType: " Membership ",
		GrantKey:  " VIP-MONTH ",
	}, 3)
	if !errors.Is(err, domain.ErrPendingMembershipOrderExists) {
		t.Fatalf("validateCartOwnedDigitalGrantWrite() error = %v, want pending membership order", err)
	}
	if db.ownedDigitalGrantLockQueryRows != 1 {
		t.Fatalf("advisory lock queries = %d, want one", db.ownedDigitalGrantLockQueryRows)
	}
	if db.activeDigitalEntitlementQueryRows != 0 {
		t.Fatalf("active entitlement checks = %d, want 0 because active membership can renew", db.activeDigitalEntitlementQueryRows)
	}
	if db.openDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("open digital grant order checks = %d, want 1", db.openDigitalGrantOrderQueryRows)
	}
}

func TestValidateCartOwnedDigitalGrantCompositionRejectsDuplicateThemeAlreadyInCart(t *testing.T) {
	err := validateCartOwnedDigitalGrantComposition(
		[]domain.OrderItem{{ProductID: 101, GrantType: "theme", GrantKey: "theme-pro", Quantity: 1}},
		domain.OrderItem{ProductID: 102, GrantType: " Theme ", GrantKey: " Theme-Pro ", Quantity: 1},
	)
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("validateCartOwnedDigitalGrantComposition() error = %v, want duplicate theme grant", err)
	}
}

func TestValidateCartOwnedDigitalGrantCompositionAllowsMembershipRenewalCart(t *testing.T) {
	err := validateCartOwnedDigitalGrantComposition(
		[]domain.OrderItem{{ProductID: 101, GrantType: "membership", GrantKey: "vip-month", Quantity: 1}},
		domain.OrderItem{ProductID: 102, GrantType: " Membership ", GrantKey: " VIP-MONTH ", Quantity: 3},
	)
	if err != nil {
		t.Fatalf("validateCartOwnedDigitalGrantComposition() error = %v, want nil", err)
	}
}

type productGrantLockQueryer struct {
	locked                                 bool
	activeDigitalEntitlementExists         bool
	openDigitalGrantOrderExists            bool
	openDigitalGrantOrderExistsExcluding   bool
	query                                  string
	args                                   []any
	queryRows                              int
	activeDigitalEntitlementQueryRows      int
	activeDigitalEntitlementArgs           []any
	openDigitalGrantOrderQueryRows         int
	openDigitalGrantOrderArgs              []any
	excludedOpenDigitalGrantOrderQueryRows int
	excludedOpenDigitalGrantOrderArgs      []any
	ownedDigitalGrantLockQueryRows         int
	ownedDigitalGrantLockQuery             string
	ownedDigitalGrantLockArgs              []any
}

func (q *productGrantLockQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *productGrantLockQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *productGrantLockQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryRows++
	q.query = query
	q.args = args
	switch {
	case strings.Contains(query, "idempotency_key"):
		return scanErrorRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "pg_advisory_xact_lock"):
		q.ownedDigitalGrantLockQueryRows++
		q.ownedDigitalGrantLockQuery = query
		q.ownedDigitalGrantLockArgs = args
		return productGrantLockScanRow{count: int64(len(productGrantLockGrantTypes(args)))}
	case strings.Contains(query, "ARRAY_AGG(matches.grant_type") && strings.Contains(query, "mall_digital_entitlements"):
		q.activeDigitalEntitlementQueryRows++
		q.activeDigitalEntitlementArgs = append([]any(nil), args...)
		if q.activeDigitalEntitlementExists {
			return productGrantLockScanRow{grantTypes: productGrantLockGrantTypes(args), grantKeys: productGrantLockGrantKeys(args)}
		}
		return productGrantLockScanRow{}
	case strings.Contains(query, "ARRAY_AGG(matches.grant_type") && strings.Contains(query, "mall_order_items"):
		if len(args) >= 6 {
			if excludedOrderID, _ := args[5].(int64); excludedOrderID > 0 {
				q.excludedOpenDigitalGrantOrderQueryRows++
				q.excludedOpenDigitalGrantOrderArgs = append([]any(nil), args...)
				if q.openDigitalGrantOrderExistsExcluding {
					return productGrantLockScanRow{grantTypes: productGrantLockGrantTypes(args), grantKeys: productGrantLockGrantKeys(args)}
				}
				return productGrantLockScanRow{}
			}
		}
		q.openDigitalGrantOrderQueryRows++
		q.openDigitalGrantOrderArgs = append([]any(nil), args...)
		if q.openDigitalGrantOrderExists {
			return productGrantLockScanRow{grantTypes: productGrantLockGrantTypes(args), grantKeys: productGrantLockGrantKeys(args)}
		}
		return productGrantLockScanRow{}
	}
	return productGrantLockScanRow{locked: q.locked}
}

type productGrantLockScanRow struct {
	locked     bool
	count      int64
	grantTypes []string
	grantKeys  []string
}

func (r productGrantLockScanRow) Scan(dest ...any) error {
	switch len(dest) {
	case 1:
		switch value := dest[0].(type) {
		case *bool:
			*value = r.locked
			return nil
		case *int64:
			*value = r.count
			return nil
		}
	case 2:
		grantTypes, typesOK := dest[0].(*[]string)
		grantKeys, keysOK := dest[1].(*[]string)
		if typesOK && keysOK {
			*grantTypes = append([]string(nil), r.grantTypes...)
			*grantKeys = append([]string(nil), r.grantKeys...)
			return nil
		}
	}
	return errors.New("unexpected scan destination")
}

func productGrantLockGrantTypes(args []any) []string {
	if len(args) < 2 {
		return nil
	}
	grantTypes, _ := args[1].([]string)
	return grantTypes
}

func productGrantLockGrantKeys(args []any) []string {
	if len(args) < 3 {
		return nil
	}
	grantKeys, _ := args[2].([]string)
	return grantKeys
}

func assertProductGrantLockArgs(t *testing.T, label string, got, want []any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s args = %#v, want %#v", label, got, want)
	}
}

type scanErrorRow struct {
	err error
}

func (r scanErrorRow) Scan(...any) error {
	return r.err
}
