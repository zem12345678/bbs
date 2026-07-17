package persistence

import (
	"context"
	"errors"
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

func TestOpenDigitalGrantOrderExistsQueriesPendingPayingGrant(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExists: true}

	exists, err := openDigitalGrantOrderExists(context.Background(), db, 7, " Badge ", " Badge-Founder ")
	if err != nil {
		t.Fatalf("openDigitalGrantOrderExists() error = %v", err)
	}
	if !exists {
		t.Fatal("openDigitalGrantOrderExists() = false, want true")
	}
	for _, want := range []string{
		"mall_orders o",
		"mall_order_items oi",
		"o.status IN ($2, $3)",
		"LOWER(TRIM(COALESCE(oi.grant_type, ''))) = $4",
		"LOWER(TRIM(COALESCE(oi.grant_key, ''))) = $5",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("open theme order query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{
		int64(7),
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		"badge",
		"badge-founder",
	}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("open digital grant order args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("open digital grant order arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestOpenDigitalGrantOrderExistsSkipsInvalidInput(t *testing.T) {
	db := &productGrantLockQueryer{}

	exists, err := openDigitalGrantOrderExists(context.Background(), db, 7, "badge", " ")
	if err != nil {
		t.Fatalf("openDigitalGrantOrderExists() error = %v", err)
	}
	if exists {
		t.Fatal("openDigitalGrantOrderExists() = true, want false")
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0", db.queryRows)
	}
}

func TestActiveDigitalEntitlementExistsRequiresMembershipExpiry(t *testing.T) {
	db := &productGrantLockQueryer{}

	_, err := activeDigitalEntitlementExists(context.Background(), db, 7, " Membership ", " VIP-MONTH ")
	if err != nil {
		t.Fatalf("activeDigitalEntitlementExists() error = %v", err)
	}
	if !strings.Contains(db.query, "de.expires_at IS NOT NULL") || !strings.Contains(db.query, "de.expires_at > NOW()") {
		t.Fatalf("active entitlement query = %q, want membership to require future expiry", db.query)
	}
	if strings.Contains(db.query, "de.expires_at IS NULL OR de.expires_at > NOW()") {
		t.Fatalf("active entitlement query = %q, should not allow perpetual membership", db.query)
	}
	if !strings.Contains(db.query, "UPPER(TRIM(COALESCE(de.status, ''))) = $4") {
		t.Fatalf("active entitlement query = %q, want normalized status filter", db.query)
	}
	wantArgs := []any{
		int64(7),
		"membership",
		"vip-month",
		domain.DigitalEntitlementStatusActive,
	}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("active entitlement args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("active entitlement arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestActiveDigitalEntitlementExistsAllowsPerpetualNonMembershipGrant(t *testing.T) {
	db := &productGrantLockQueryer{}

	_, err := activeDigitalEntitlementExists(context.Background(), db, 7, " Theme ", " Theme-Pro ")
	if err != nil {
		t.Fatalf("activeDigitalEntitlementExists() error = %v", err)
	}
	if !strings.Contains(db.query, "de.expires_at IS NULL OR de.expires_at > NOW()") {
		t.Fatalf("active entitlement query = %q, want non-membership grants to allow no expiry", db.query)
	}
	if strings.Contains(db.query, "de.expires_at IS NOT NULL") {
		t.Fatalf("active entitlement query = %q, should not require expiry for theme grants", db.query)
	}
	wantArgs := []any{
		int64(7),
		"theme",
		"theme-pro",
		domain.DigitalEntitlementStatusActive,
	}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("active entitlement args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("active entitlement arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestOpenDigitalGrantOrderExistsExcludingSkipsCurrentOrder(t *testing.T) {
	db := &productGrantLockQueryer{openDigitalGrantOrderExistsExcluding: true}

	exists, err := openDigitalGrantOrderExistsExcluding(context.Background(), db, 7, 9001, " Membership ", " VIP-MONTH ")
	if err != nil {
		t.Fatalf("openDigitalGrantOrderExistsExcluding() error = %v", err)
	}
	if !exists {
		t.Fatal("openDigitalGrantOrderExistsExcluding() = false, want true")
	}
	for _, want := range []string{
		"mall_orders o",
		"mall_order_items oi",
		"o.status IN ($2, $3)",
		"LOWER(TRIM(COALESCE(oi.grant_type, ''))) = $4",
		"LOWER(TRIM(COALESCE(oi.grant_key, ''))) = $5",
		"o.id <> $6",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("excluded open grant order query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{
		int64(7),
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		"membership",
		"vip-month",
		int64(9001),
	}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("excluded open digital grant order args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("excluded open digital grant order arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
	}
	if !strings.Contains(db.execQuery, "pg_advisory_xact_lock") || !strings.Contains(db.execQuery, "CONCAT($1::BIGINT::text, ':', LOWER($2), ':', LOWER($3))") {
		t.Fatalf("advisory lock query = %q, want entitlement-compatible key", db.execQuery)
	}
	wantArgs := []any{int64(7), "theme", "theme-pro"}
	for i := range wantArgs {
		if db.execArgs[i] != wantArgs[i] {
			t.Fatalf("advisory lock arg %d = %#v, want %#v", i, db.execArgs[i], wantArgs[i])
		}
	}
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
	if db.execCalls != 0 {
		t.Fatalf("Exec() calls = %d, want 0 before advisory lock", db.execCalls)
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
	}
	if db.activeDigitalEntitlementQueryRows != 1 {
		t.Fatalf("active entitlement checks = %d, want 1", db.activeDigitalEntitlementQueryRows)
	}
	if db.openDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("open digital grant order checks = %d, want 1", db.openDigitalGrantOrderQueryRows)
	}
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
	}
	wantArgs := []any{int64(7), "membership", "vip-month"}
	for i := range wantArgs {
		if db.execArgs[i] != wantArgs[i] {
			t.Fatalf("advisory lock arg %d = %#v, want %#v", i, db.execArgs[i], wantArgs[i])
		}
	}
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
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
				if db.execCalls != 1 {
					t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
				}
				wantArgs := []any{int64(7), "badge", "badge-founder"}
				for i := range wantArgs {
					if db.execArgs[i] != wantArgs[i] {
						t.Fatalf("advisory lock arg %d = %#v, want %#v", i, db.execArgs[i], wantArgs[i])
					}
				}
			} else if db.execCalls != 0 {
				t.Fatalf("Exec() calls = %d, want 0 before advisory lock", db.execCalls)
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
	}
	wantArgs := []any{int64(7), "membership", "vip-month"}
	for i := range wantArgs {
		if db.execArgs[i] != wantArgs[i] {
			t.Fatalf("advisory lock arg %d = %#v, want %#v", i, db.execArgs[i], wantArgs[i])
		}
	}
	if db.activeDigitalEntitlementQueryRows != 0 {
		t.Fatalf("active entitlement checks = %d, want 0 because active membership can renew", db.activeDigitalEntitlementQueryRows)
	}
	if db.excludedOpenDigitalGrantOrderQueryRows != 1 {
		t.Fatalf("excluded open order checks = %d, want 1", db.excludedOpenDigitalGrantOrderQueryRows)
	}
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
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
	if db.execCalls != 0 {
		t.Fatalf("Exec() calls = %d, want 0 before advisory lock", db.execCalls)
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
	if db.execCalls != 0 {
		t.Fatalf("Exec() calls = %d, want 0 before advisory lock", db.execCalls)
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
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
	if db.execCalls != 1 {
		t.Fatalf("Exec() calls = %d, want one advisory lock", db.execCalls)
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
	openDigitalGrantOrderQueryRows         int
	excludedOpenDigitalGrantOrderQueryRows int
	execQuery                              string
	execArgs                               []any
	execCalls                              int
}

func (q *productGrantLockQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	q.execCalls++
	q.execQuery = query
	q.execArgs = args
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
	case strings.Contains(query, "mall_digital_entitlements"):
		q.activeDigitalEntitlementQueryRows++
		return productGrantLockScanRow{locked: q.activeDigitalEntitlementExists}
	case len(args) == 6 && strings.Contains(query, "mall_order_items") && strings.Contains(query, "o.id <> $6"):
		q.excludedOpenDigitalGrantOrderQueryRows++
		return productGrantLockScanRow{locked: q.openDigitalGrantOrderExistsExcluding}
	case len(args) == 5 && strings.Contains(query, "mall_order_items") && strings.Contains(query, "LOWER(TRIM(COALESCE(oi.grant_type, ''))) = $4"):
		q.openDigitalGrantOrderQueryRows++
		return productGrantLockScanRow{locked: q.openDigitalGrantOrderExists}
	}
	return productGrantLockScanRow{locked: q.locked}
}

type productGrantLockScanRow struct {
	locked bool
}

func (r productGrantLockScanRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}
	locked, ok := dest[0].(*bool)
	if !ok {
		return errors.New("expected bool scan destination")
	}
	*locked = r.locked
	return nil
}

type scanErrorRow struct {
	err error
}

func (r scanErrorRow) Scan(...any) error {
	return r.err
}
