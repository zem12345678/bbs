package persistence

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWithDigitalEntitlementsAddsTraceableDeliveryFieldsToOutboxPayload(t *testing.T) {
	event := domain.OutboxEvent{Payload: []byte(`{"event_id":"evt-1","event_type":"mall.order.paid.v1"}`)}
	updated, err := withDigitalEntitlements(event, []domain.DigitalEntitlement{{
		ProductID: 101,
		SKU:       "BADGE-FOUNDER",
		Title:     "创始会员徽章",
		Code:      "BBS-ENTITLEMENT",
		GrantType: "badge",
		GrantKey:  "badge-founder",
		Status:    domain.DigitalEntitlementStatusActive,
	}})
	if err != nil {
		t.Fatalf("withDigitalEntitlements() error = %v", err)
	}
	if updated.PayloadJSON != string(updated.Payload) {
		t.Fatalf("PayloadJSON = %q, want payload %q", updated.PayloadJSON, updated.Payload)
	}
	var payload struct {
		DigitalEntitlements []struct {
			FulfillmentCode string `json:"fulfillment_code"`
			GrantType       string `json:"grant_type"`
			GrantKey        string `json:"grant_key"`
		} `json:"digital_entitlements"`
	}
	if err := json.Unmarshal(updated.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if len(payload.DigitalEntitlements) != 1 {
		t.Fatalf("digital_entitlements = %+v, want one item", payload.DigitalEntitlements)
	}
	item := payload.DigitalEntitlements[0]
	if item.FulfillmentCode != "BBS-ENTITLEMENT" || item.GrantType != "badge" || item.GrantKey != "badge-founder" {
		t.Fatalf("digital entitlement = %+v, want traceable delivery fields", item)
	}
}

func TestIssueDigitalEntitlementsInsertsFulfillmentCode(t *testing.T) {
	issuedAt := time.Date(2026, 7, 12, 12, 30, 0, 0, time.UTC)
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, issuedAt); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 1 {
		t.Fatalf("Exec() calls = %d, want 1", len(db.execArgs))
	}
	args := db.execArgs[0]
	if args[0] != order.ID || args[1] != order.Items[0].ProductID || args[2] != order.UserID {
		t.Fatalf("Exec() identity args = %+v", args[:3])
	}
	code, ok := args[6].(string)
	if !ok || !strings.HasPrefix(code, "BBS-") {
		t.Fatalf("fulfillment code = %#v, want BBS- prefix", args[6])
	}
	if args[7] != "membership" {
		t.Fatalf("grant type arg = %#v, want membership", args[7])
	}
	if args[8] != "vip-month" {
		t.Fatalf("grant key arg = %#v, want vip-month", args[8])
	}
	if args[9] != domain.DigitalEntitlementStatusActive {
		t.Fatalf("status arg = %#v, want %s", args[9], domain.DigitalEntitlementStatusActive)
	}
	if args[10] != issuedAt {
		t.Fatalf("issued at arg = %#v, want %v", args[10], issuedAt)
	}
	if args[11] != issuedAt.Add(membershipEntitlementDuration) {
		t.Fatalf("expires at arg = %#v, want %v", args[11], issuedAt.Add(membershipEntitlementDuration))
	}
	query := db.execQueries[0]
	for _, expected := range []string{
		"pg_advisory_xact_lock",
		"CONCAT($3::BIGINT::text, ':', LOWER($8), ':', LOWER($9))",
		"SELECT MAX(existing.expires_at)",
		"existing.user_id = $3::BIGINT",
		"LOWER(TRIM(COALESCE(existing.grant_type, ''))) = $8",
		"LOWER(TRIM(COALESCE(existing.grant_key, ''))) = $9",
		"existing.status = 'ACTIVE'",
		"existing.revoked_at IS NULL",
		"existing.expires_at > $11::timestamptz",
		"($12::timestamptz - $11::timestamptz)",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("issuance query = %q, want %q for atomic membership renewal", query, expected)
		}
	}
}

func TestDigitalEntitlementExpiresAtOnlyAppliesToMembership(t *testing.T) {
	issuedAt := time.Date(2026, 7, 12, 12, 30, 0, 0, time.UTC)
	expiresAt := digitalEntitlementExpiresAt("membership", issuedAt)
	if expiresAt == nil || !expiresAt.Equal(issuedAt.Add(membershipEntitlementDuration)) {
		t.Fatalf("membership expiresAt = %v, want %v", expiresAt, issuedAt.Add(membershipEntitlementDuration))
	}
	if got := digitalEntitlementExpiresAt("badge", issuedAt); got != nil {
		t.Fatalf("badge expiresAt = %v, want nil", got)
	}
}

func TestNormalizeDigitalEntitlementStatusAcceptsEffectiveExpired(t *testing.T) {
	if got := normalizeDigitalEntitlementStatus(" expired "); got != domain.DigitalEntitlementStatusExpired {
		t.Fatalf("normalizeDigitalEntitlementStatus(expired) = %q, want %s", got, domain.DigitalEntitlementStatusExpired)
	}
}

func TestDigitalEntitlementListStatusConditionFiltersEffectiveExpiry(t *testing.T) {
	active := digitalEntitlementListStatusCondition("de", domain.DigitalEntitlementStatusActive)
	if !strings.Contains(active, "UPPER(TRIM(COALESCE(de.status, ''))) = 'ACTIVE'") {
		t.Fatalf("ACTIVE condition = %q, want explicit active status filter", active)
	}
	if !strings.Contains(active, "LOWER(TRIM(COALESCE(de.grant_type, ''))) = 'membership'") ||
		!strings.Contains(active, "de.expires_at IS NOT NULL") {
		t.Fatalf("ACTIVE condition = %q, want membership expiry requirement", active)
	}
	if !strings.Contains(active, "de.expires_at IS NULL OR de.expires_at > NOW()") {
		t.Fatalf("ACTIVE condition = %q, want future-or-empty expiry filter for non-membership grants", active)
	}
	expired := digitalEntitlementListStatusCondition("de", domain.DigitalEntitlementStatusExpired)
	if !strings.Contains(expired, "de.expires_at IS NOT NULL") || !strings.Contains(expired, "de.expires_at <= NOW()") {
		t.Fatalf("EXPIRED condition = %q, want elapsed expiry filter", expired)
	}
	revoked := digitalEntitlementListStatusCondition("de", domain.DigitalEntitlementStatusRevoked)
	if strings.Contains(revoked, "expires_at") {
		t.Fatalf("REVOKED condition = %q, should not filter by expiry", revoked)
	}
}

func TestDigitalEntitlementListGrantConditionFiltersGrantColumns(t *testing.T) {
	plain := digitalEntitlementListGrantCondition("", 2, 3)
	if !strings.Contains(plain, "LOWER(TRIM(COALESCE(grant_type, ''))) = $2") {
		t.Fatalf("plain grant condition = %q, want explicit grant type filter", plain)
	}
	if !strings.Contains(plain, "LOWER(TRIM(COALESCE(grant_key, ''))) = $3") {
		t.Fatalf("plain grant condition = %q, want explicit grant key filter", plain)
	}

	aliased := digitalEntitlementListGrantCondition("de", 2, 3)
	if !strings.Contains(aliased, "LOWER(TRIM(COALESCE(de.grant_type, ''))) = $2") {
		t.Fatalf("aliased grant condition = %q, want aliased explicit grant type filter", aliased)
	}
	if !strings.Contains(aliased, "LOWER(TRIM(COALESCE(de.grant_key, ''))) = $3") {
		t.Fatalf("aliased grant condition = %q, want aliased explicit grant key filter", aliased)
	}
}

func TestDigitalEntitlementListKeywordConditionCoversLedgerFields(t *testing.T) {
	condition := digitalEntitlementListKeywordCondition(4)
	for _, want := range []string{
		"de.user_id::TEXT = $4",
		"de.order_id::TEXT = $4",
		"de.refund_id::TEXT = $4",
		"o.order_no ILIKE '%' || $4 || '%'",
		"de.fulfillment_code ILIKE '%' || $4 || '%'",
		"COALESCE(de.grant_key, '') ILIKE '%' || $4 || '%'",
	} {
		if !strings.Contains(condition, want) {
			t.Fatalf("keyword condition = %q, want %q", condition, want)
		}
	}
}

func TestDigitalEntitlementSchemaDoesNotPromoteDirtyRows(t *testing.T) {
	forbidden := []string{
		"UPDATE mall_digital_entitlements SET grant_key",
		"UPDATE mall_digital_entitlements SET grant_type",
		"UPDATE mall_digital_entitlements SET status = 'ACTIVE'",
		"grant_type TEXT NOT NULL DEFAULT 'digital'",
		"status TEXT NOT NULL DEFAULT 'ACTIVE'",
	}
	seenGrantTypeDefault := false
	seenStatusDefault := false
	for _, statement := range schemaStatements {
		normalized := strings.Join(strings.Fields(statement), " ")
		if !strings.Contains(normalized, "mall_digital_entitlements") {
			continue
		}
		for _, blocked := range forbidden {
			if strings.Contains(normalized, blocked) {
				t.Fatalf("schema statement promotes dirty entitlement rows: %s", statement)
			}
		}
		if strings.Contains(normalized, "ALTER TABLE mall_digital_entitlements ALTER COLUMN grant_type SET DEFAULT ''") {
			seenGrantTypeDefault = true
		}
		if strings.Contains(normalized, "ALTER TABLE mall_digital_entitlements ALTER COLUMN status SET DEFAULT ''") {
			seenStatusDefault = true
		}
	}
	if !seenGrantTypeDefault || !seenStatusDefault {
		t.Fatalf("schema statements should force blank grant_type/status defaults, seen grant_type=%v status=%v", seenGrantTypeDefault, seenStatusDefault)
	}
}

func TestIssueDigitalEntitlementsRetriesFulfillmentCodeCollision(t *testing.T) {
	db := &digitalEntitlementQueryer{
		execErrors: []error{&pgconn.PgError{Code: "23505"}},
	}
	order := domain.Order{
		ID:     9002,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 2 {
		t.Fatalf("Exec() calls = %d, want retry after unique violation", len(db.execArgs))
	}
}

func TestIssueDigitalEntitlementsIssuesOneCodePerUnit(t *testing.T) {
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9005,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 3},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 3 {
		t.Fatalf("Exec() calls = %d, want one entitlement per unit", len(db.execArgs))
	}
	codes := make(map[string]bool, len(db.execArgs))
	for i, args := range db.execArgs {
		if args[5] != int32(1) {
			t.Fatalf("Exec() call %d quantity = %#v, want 1 for a traceable entitlement instance", i+1, args[5])
		}
		code, ok := args[6].(string)
		if !ok || !strings.HasPrefix(code, "BBS-") {
			t.Fatalf("Exec() call %d fulfillment code = %#v, want BBS- prefix", i+1, args[6])
		}
		if codes[code] {
			t.Fatalf("Exec() call %d reused fulfillment code %q", i+1, code)
		}
		codes[code] = true
	}
}

func TestIssueDigitalEntitlementsSkipsPhysicalItems(t *testing.T) {
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9003,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 0 {
		t.Fatalf("Exec() calls = %d, want 0 for physical item", len(db.execArgs))
	}
}

func TestIssueDigitalEntitlementsIncludesGrantedNonDigitalCategory(t *testing.T) {
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9006,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 303, SKU: "BADGE-FOUNDER", Title: "创始会员徽章", Category: "badge", GrantType: "badge", GrantKey: "badge-founder", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 1 {
		t.Fatalf("Exec() calls = %d, want 1 for granted item", len(db.execArgs))
	}
	args := db.execArgs[0]
	if args[7] != "badge" || args[8] != "badge-founder" {
		t.Fatalf("grant args = (%#v, %#v), want (badge, badge-founder)", args[7], args[8])
	}
}

func TestDigitalGrantForItemMapsKnownSKUPrefixes(t *testing.T) {
	tests := []struct {
		name      string
		sku       string
		grantType string
		grantKey  string
		productID int64
		wantType  string
		wantKey   string
	}{
		{name: "badge key", grantKey: "badge-founder", wantType: "badge", wantKey: "badge-founder"},
		{name: "theme key", grantKey: "theme-pro", wantType: "theme", wantKey: "theme-pro"},
		{name: "vip key", grantKey: "VIP-MONTH", wantType: "membership", wantKey: "vip-month"},
		{name: "vip sku fallback", sku: "VIP-MONTH", wantType: "digital", wantKey: "vip-month"},
		{name: "explicit grant", sku: "VIP-MONTH", grantType: "badge", grantKey: "badge-founder", wantType: "badge", wantKey: "badge-founder"},
		{name: "fallback product id", productID: 101, wantType: "digital", wantKey: "product:101"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotKey := digitalGrantForItem(domain.OrderItem{ProductID: tt.productID, SKU: tt.sku, GrantType: tt.grantType, GrantKey: tt.grantKey})
			if gotType != tt.wantType || gotKey != tt.wantKey {
				t.Fatalf("digitalGrantForItem() = (%q, %q), want (%q, %q)", gotType, gotKey, tt.wantType, tt.wantKey)
			}
		})
	}
}

func TestIsDigitalOnlyOrderUsesItemCategorySnapshot(t *testing.T) {
	order := domain.Order{
		Receiver: "Alice",
		Phone:    "13800000000",
		Address:  "上海市数字权益也可填写地址",
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1},
		},
	}

	if !isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = false, want true for digital item even when address fields are present")
	}
}

func TestIsDigitalOnlyOrderUsesGrantSnapshot(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 303, SKU: "BADGE-FOUNDER", Title: "创始会员徽章", Category: "badge", GrantType: "badge", GrantKey: "badge-founder", Quantity: 1},
		},
	}

	if !isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = false, want true for granted item")
	}
}

func TestIsDigitalOnlyOrderRequiresAllItemsToBeDigital(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1},
			{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
		},
	}

	if isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = true, want false when any item requires shipping")
	}
}

func TestIsDigitalOnlyOrderDoesNotTreatMissingCategoryAsDigital(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "LEGACY", Title: "历史订单项", Quantity: 1},
		},
	}

	if isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = true, want false when item category snapshot is missing")
	}
}

func TestRevokeDigitalEntitlementsForRefundMarksOrderEntitlementsRevoked(t *testing.T) {
	revokedAt := time.Date(2026, 7, 13, 9, 45, 0, 0, time.UTC)
	db := &digitalEntitlementQueryer{}

	if err := revokeDigitalEntitlementsForRefund(context.Background(), db, 9004, 7001, revokedAt); err != nil {
		t.Fatalf("revokeDigitalEntitlementsForRefund() error = %v", err)
	}
	if len(db.execArgs) != 1 {
		t.Fatalf("Exec() calls = %d, want 1", len(db.execArgs))
	}
	args := db.execArgs[0]
	if args[0] != int64(9004) {
		t.Fatalf("order id arg = %#v, want 9004", args[0])
	}
	if args[1] != domain.DigitalEntitlementStatusRevoked {
		t.Fatalf("status arg = %#v, want %s", args[1], domain.DigitalEntitlementStatusRevoked)
	}
	if args[2] != revokedAt {
		t.Fatalf("revoked at arg = %#v, want %v", args[2], revokedAt)
	}
	if args[3] != int64(7001) {
		t.Fatalf("refund id arg = %#v, want 7001", args[3])
	}
	query := db.execQueries[0]
	for _, expected := range []string{
		"WHERE order_id = $1",
		"UPPER(TRIM(COALESCE(status, ''))) = $5",
		"revoked_at IS NULL",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("revocation query = %q, want %q", query, expected)
		}
	}
	if strings.Contains(query, "status <> $2 OR revoked_at IS NULL OR refund_id IS NULL") {
		t.Fatalf("revocation query = %q, should not match unrelated entitlement rows", query)
	}
}

type digitalEntitlementQueryer struct {
	execArgs    [][]any
	execQueries []string
	execErrors  []error
}

func (q *digitalEntitlementQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	q.execQueries = append(q.execQueries, query)
	q.execArgs = append(q.execArgs, args)
	if len(q.execErrors) == 0 {
		return pgconn.CommandTag{}, nil
	}
	err := q.execErrors[0]
	q.execErrors = q.execErrors[1:]
	return pgconn.CommandTag{}, err
}

func (q *digitalEntitlementQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *digitalEntitlementQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}
