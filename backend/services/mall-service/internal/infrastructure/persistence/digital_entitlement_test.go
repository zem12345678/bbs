package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
	if args[0] != order.ID || args[1] != order.UserID {
		t.Fatalf("Exec() identity args = %+v", args[:2])
	}
	if got, want := args[2], []int64{order.Items[0].ProductID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("product IDs = %#v, want %#v", got, want)
	}
	if got, want := args[5], []string{"membership"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant types = %#v, want %#v", got, want)
	}
	if got, want := args[6], []string{"vip-month"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant keys = %#v, want %#v", got, want)
	}
	codes, ok := args[7].([]string)
	if !ok || len(codes) != 1 || !strings.HasPrefix(codes[0], "BBS-") {
		t.Fatalf("fulfillment codes = %#v, want one BBS- code", args[7])
	}
	if args[8] != domain.DigitalEntitlementStatusActive {
		t.Fatalf("status arg = %#v, want %s", args[8], domain.DigitalEntitlementStatusActive)
	}
	if args[9] != issuedAt || args[10] != issuedAt.Add(membershipEntitlementDuration) {
		t.Fatalf("issued and membership expiry args = %#v, want %v / %v", args[9:], issuedAt, issuedAt.Add(membershipEntitlementDuration))
	}
	if len(args) != 11 {
		t.Fatalf("batch issuance args = %+v, want 11 args", args)
	}
	if db.queryRows != 1 || !reflect.DeepEqual(db.queryArgs, []any{order.UserID}) {
		t.Fatalf("membership expiry lock = calls:%d args:%#v, want 1/%#v", db.queryRows, db.queryArgs, []any{order.UserID})
	}
	for _, expected := range []string{
		"pg_advisory_xact_lock",
		"CONCAT($1::BIGINT::text, ':membership')",
	} {
		if !strings.Contains(db.query, expected) {
			t.Fatalf("membership expiry lock query = %q, want %q", db.query, expected)
		}
	}
	query := db.execQueries[0]
	for _, expected := range []string{
		"FROM unnest(",
		"WITH ORDINALITY",
		"$3::BIGINT[]",
		"COUNT(*) FILTER (WHERE grant_type = 'membership')",
		"COALESCE(MAX(existing.expires_at), $10::TIMESTAMPTZ)",
		"existing.user_id = $2::BIGINT",
		"LOWER(TRIM(COALESCE(existing.grant_type, ''))) = 'membership'",
		"UPPER(TRIM(COALESCE(existing.status, ''))) = $9",
		"existing.revoked_at IS NULL",
		"existing.expires_at > $10::TIMESTAMPTZ",
		"($11::TIMESTAMPTZ - $10::TIMESTAMPTZ) * scheduled.membership_ordinal",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("issuance query = %q, want %q for atomic membership renewal", query, expected)
		}
	}
}

func TestIssueDigitalEntitlementsSharesRenewalScopeAcrossMembershipGrantKeys(t *testing.T) {
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9007,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
			{ProductID: 102, SKU: "MEMBER-PRO", Title: "会员续期", Category: "digital", GrantType: "membership", GrantKey: "member-pro", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 1 {
		t.Fatalf("Exec() calls = %d, want one batched insert", len(db.execArgs))
	}
	args := db.execArgs[0]
	if got, want := args[5], []string{"membership", "membership"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant types = %#v, want %#v", got, want)
	}
	if got, want := args[6], []string{"vip-month", "member-pro"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant keys = %#v, want %#v", got, want)
	}
	if db.queryRows != 1 {
		t.Fatalf("membership expiry lock calls = %d, want 1", db.queryRows)
	}
}

func TestIssueDigitalEntitlementsSkipsMembershipExpiryLockForNonMembership(t *testing.T) {
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID:     9008,
		UserID: 7,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "THEME-PRO", Title: "高级主题", Category: "digital", GrantType: "theme", GrantKey: "theme-pro", Quantity: 1},
		},
	}

	if err := issueDigitalEntitlements(context.Background(), db, order, time.Now().UTC()); err != nil {
		t.Fatalf("issueDigitalEntitlements() error = %v", err)
	}
	if len(db.execArgs) != 1 || len(db.execArgs[0]) != 11 {
		t.Fatalf("batch issuance args = %+v, want one 11-argument call", db.execArgs)
	}
	if got, want := db.execArgs[0][5], []string{"theme"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant types = %#v, want %#v", got, want)
	}
	if got, want := db.execArgs[0][6], []string{"theme-pro"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant keys = %#v, want %#v", got, want)
	}
	if db.queryRows != 0 {
		t.Fatalf("membership expiry lock calls = %d, want 0 for non-membership", db.queryRows)
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

func TestRebaseMembershipEntitlementExpiryScheduleRemovesRevokedDurationWithoutExtendingLegacyRows(t *testing.T) {
	issuedAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		items []membershipEntitlementExpiry
		want  []time.Time
	}{
		{
			name: "removes earlier stacked membership duration",
			items: []membershipEntitlementExpiry{
				{ID: 2, IssuedAt: issuedAt.Add(time.Hour), ExistingExpiresAt: issuedAt.Add(60 * 24 * time.Hour)},
				{ID: 3, IssuedAt: issuedAt.Add(2 * time.Hour), ExistingExpiresAt: issuedAt.Add(90 * 24 * time.Hour)},
			},
			want: []time.Time{
				issuedAt.Add(time.Hour + membershipEntitlementDuration),
				issuedAt.Add(time.Hour + 2*membershipEntitlementDuration),
			},
		},
		{
			name: "does not extend legacy independent rows",
			items: []membershipEntitlementExpiry{
				{ID: 2, IssuedAt: issuedAt, ExistingExpiresAt: issuedAt.Add(membershipEntitlementDuration)},
				{ID: 3, IssuedAt: issuedAt.Add(time.Hour), ExistingExpiresAt: issuedAt.Add(time.Hour + membershipEntitlementDuration)},
			},
			want: []time.Time{
				issuedAt.Add(membershipEntitlementDuration),
				issuedAt.Add(time.Hour + membershipEntitlementDuration),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rebaseMembershipEntitlementExpirySchedule(tt.items)
			if len(got) != len(tt.want) {
				t.Fatalf("rebased items = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if !got[i].ExpiresAt.Equal(tt.want[i]) {
					t.Fatalf("rebased expiry %d = %v, want %v", i, got[i].ExpiresAt, tt.want[i])
				}
				if got[i].ExpiresAt.After(got[i].ExistingExpiresAt) {
					t.Fatalf("rebased expiry %d = %v extends existing expiry %v", i, got[i].ExpiresAt, got[i].ExistingExpiresAt)
				}
			}
		})
	}
}

func TestUpdateMembershipEntitlementExpirationsBatchesRows(t *testing.T) {
	issuedAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	updates := []membershipEntitlementExpiry{
		{ID: 2, ExpiresAt: issuedAt.Add(membershipEntitlementDuration)},
		{ID: 3, ExpiresAt: issuedAt.Add(2 * membershipEntitlementDuration)},
	}
	db := &digitalEntitlementStateQueryer{tag: pgconn.NewCommandTag("UPDATE 2")}

	if err := updateMembershipEntitlementExpirations(context.Background(), db, updates); err != nil {
		t.Fatalf("updateMembershipEntitlementExpirations() error = %v", err)
	}
	if len(db.execQueries) != 1 {
		t.Fatalf("Exec() calls = %d, want one batch update", len(db.execQueries))
	}
	for _, want := range []string{
		"UPDATE mall_digital_entitlements AS entitlement",
		"FROM unnest($1::BIGINT[], $2::TIMESTAMPTZ[])",
		"SET expires_at = input.expires_at",
	} {
		if !strings.Contains(db.execQueries[0], want) {
			t.Fatalf("membership expiry update query = %q, want %q", db.execQueries[0], want)
		}
	}
	wantArgs := []any{[]int64{2, 3}, []time.Time{updates[0].ExpiresAt, updates[1].ExpiresAt}}
	if !reflect.DeepEqual(db.execArgs[0], wantArgs) {
		t.Fatalf("membership expiry update args = %#v, want %#v", db.execArgs[0], wantArgs)
	}

	err := updateMembershipEntitlementExpirations(context.Background(), &digitalEntitlementStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")}, updates)
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("updateMembershipEntitlementExpirations() error = %v, want invalid order state", err)
	}

	emptyDB := &digitalEntitlementStateQueryer{}
	if err := updateMembershipEntitlementExpirations(context.Background(), emptyDB, nil); err != nil {
		t.Fatalf("updateMembershipEntitlementExpirations() empty error = %v", err)
	}
	if len(emptyDB.execQueries) != 0 {
		t.Fatalf("empty Exec() calls = %d, want 0", len(emptyDB.execQueries))
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
	if !strings.Contains(active, "de.revoked_at IS NULL") {
		t.Fatalf("ACTIVE condition = %q, want revoked entitlements excluded", active)
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
	if !strings.Contains(expired, "de.revoked_at IS NULL") {
		t.Fatalf("EXPIRED condition = %q, want revoked entitlements excluded", expired)
	}
	revoked := digitalEntitlementListStatusCondition("de", domain.DigitalEntitlementStatusRevoked)
	if strings.Contains(revoked, "expires_at") {
		t.Fatalf("REVOKED condition = %q, should not filter by expiry", revoked)
	}
	if !strings.Contains(revoked, "UPPER(TRIM(COALESCE(de.status, ''))) = 'REVOKED'") || !strings.Contains(revoked, "de.revoked_at IS NOT NULL") {
		t.Fatalf("REVOKED condition = %q, want status or revoked_at filter", revoked)
	}
}

func TestActiveDigitalEntitlementUserIDsSQLRestrictsRequestedActiveGrants(t *testing.T) {
	query := activeDigitalEntitlementUserIDsSQL()
	for _, want := range []string{
		"SELECT DISTINCT de.user_id",
		"de.user_id = ANY($1::BIGINT[])",
		"LOWER(TRIM(COALESCE(de.grant_type, ''))) = $2",
		"LOWER(TRIM(COALESCE(de.grant_key, ''))) = $3",
		"BTRIM(COALESCE(de.grant_key, '')) <> ''",
		"UPPER(TRIM(COALESCE(de.status, ''))) = 'ACTIVE'",
		"de.revoked_at IS NULL",
		"de.expires_at IS NOT NULL",
		"ORDER BY de.user_id ASC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("active entitlement user ids query = %q, want %q", query, want)
		}
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

func TestDigitalEntitlementListOrderIDsConditionUsesArrayFilter(t *testing.T) {
	condition := digitalEntitlementListOrderIDsCondition(4)
	for _, want := range []string{
		"COALESCE(CARDINALITY($4::BIGINT[]), 0) = 0",
		"de.order_id = ANY($4::BIGINT[])",
	} {
		if !strings.Contains(condition, want) {
			t.Fatalf("order ids condition = %q, want %q", condition, want)
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

func TestDigitalEntitlementSchemaEnforcesNormalizedActiveGrants(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_digital_entitlements_grant_status_normalized_check",
		"grant_type = LOWER(TRIM(grant_type))",
		"grant_type <> ''",
		"grant_key = LOWER(TRIM(grant_key))",
		"grant_key <> ''",
		"status = UPPER(TRIM(status))",
		"status IN ('ACTIVE', 'REVOKED')",
		"mall_digital_entitlements_grant_type_check",
		"grant_type IN ('badge', 'theme', 'membership', 'digital')",
		"mall_digital_entitlements_membership_expiry_check",
		"LOWER(TRIM(grant_type)) <> 'membership'",
		"UPPER(TRIM(status)) <> 'ACTIVE'",
		"expires_at IS NOT NULL",
		"mall_digital_entitlements_lifecycle_check",
		"expires_at IS NULL OR expires_at >= issued_at",
		"status = 'ACTIVE' AND revoked_at IS NULL AND refund_id IS NULL",
		"status = 'REVOKED'",
		"refund_id IS NOT NULL OR (BTRIM(revoked_by) <> '' AND BTRIM(revoke_reason) <> '')",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing digital entitlement constraint %q", want)
		}
	}
}

func TestDigitalEntitlementSchemaBackfillsOnlyMissingPaidOrShippedPerpetualGrants(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"WITH existing_units AS",
		"o.status IN ('PAID', 'SHIPPED', 'COMPLETED')",
		"LOWER(TRIM(oi.grant_type)) IN ('badge', 'theme', 'digital')",
		"existing.grant_type = LOWER(TRIM(oi.grant_type))",
		"existing.grant_key = LOWER(TRIM(oi.grant_key))",
		"generate_series(legacy.fulfilled_quantity + 1, legacy.quantity)",
		"BBS-LEGACY-%s-%s-%s",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing legacy entitlement backfill guard %q", want)
		}
	}
	if strings.Contains(joined, "LOWER(TRIM(oi.grant_type)) IN ('badge', 'theme', 'membership', 'digital')") {
		t.Fatal("legacy entitlement backfill must not retroactively issue time-bound membership grants")
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
	if db.queryRows != 1 {
		t.Fatalf("membership expiry lock calls = %d, want one lock before retries", db.queryRows)
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
	if len(db.execArgs) != 1 {
		t.Fatalf("Exec() calls = %d, want one batch for all entitlement units", len(db.execArgs))
	}
	if db.queryRows != 1 {
		t.Fatalf("membership expiry lock calls = %d, want 1 for one membership batch", db.queryRows)
	}
	issuedCodes, ok := db.execArgs[0][7].([]string)
	if !ok || len(issuedCodes) != 3 {
		t.Fatalf("fulfillment codes = %#v, want three unit codes", db.execArgs[0][7])
	}
	codes := make(map[string]bool, len(issuedCodes))
	for i, code := range issuedCodes {
		if !strings.HasPrefix(code, "BBS-") {
			t.Fatalf("fulfillment code %d = %q, want BBS- prefix", i+1, code)
		}
		if codes[code] {
			t.Fatalf("fulfillment code %d reused %q", i+1, code)
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
	if db.queryRows != 0 {
		t.Fatalf("membership expiry lock calls = %d, want 0 for physical item", db.queryRows)
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
	if got, want := args[5], []string{"badge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant types = %#v, want %#v", got, want)
	}
	if got, want := args[6], []string{"badge-founder"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grant keys = %#v, want %#v", got, want)
	}
	if db.queryRows != 0 {
		t.Fatalf("membership expiry lock calls = %d, want 0 for non-membership", db.queryRows)
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

func TestOrderHasDigitalEntitlementItemsAllowsMixedOrders(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
			{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
		},
	}

	if !orderHasDigitalEntitlementItems(order) {
		t.Fatal("orderHasDigitalEntitlementItems() = false, want true for mixed digital and physical order")
	}
	if isDigitalOnlyOrder(order) {
		t.Fatal("isDigitalOnlyOrder() = true, want mixed order to remain shippable")
	}
}

func TestOrderHasDigitalEntitlementItemsRejectsPhysicalOnlyOrders(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
		},
	}

	if orderHasDigitalEntitlementItems(order) {
		t.Fatal("orderHasDigitalEntitlementItems() = true, want false for physical-only order")
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
	if args[5] != revokedAt {
		t.Fatalf("effective time arg = %#v, want %v", args[5], revokedAt)
	}
	query := db.execQueries[0]
	for _, expected := range []string{
		"WHERE order_id = $1",
		"UPPER(TRIM(COALESCE(status, ''))) = $5",
		"revoked_at IS NULL",
		"(expires_at IS NULL OR expires_at > $6)",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("revocation query = %q, want %q", query, expected)
		}
	}
	if strings.Contains(query, "status <> $2 OR revoked_at IS NULL OR refund_id IS NULL") {
		t.Fatalf("revocation query = %q, should not match unrelated entitlement rows", query)
	}
}

func TestRevokeRefundableDigitalEntitlementsForRefundRejectsMembershipOrder(t *testing.T) {
	revokedAt := time.Date(2026, 7, 16, 16, 0, 0, 0, time.UTC)
	db := &digitalEntitlementQueryer{}
	order := domain.Order{
		ID: 9005,
		Items: []domain.OrderItem{
			{ProductID: 101, GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
		},
	}

	err := revokeRefundableDigitalEntitlementsForRefund(context.Background(), db, order, 7002, revokedAt)
	if !errors.Is(err, domain.ErrMembershipRefundUnavailable) {
		t.Fatalf("revokeRefundableDigitalEntitlementsForRefund() error = %v, want membership refund unavailable", err)
	}
	if len(db.execArgs) != 0 {
		t.Fatalf("Exec() calls = %d, want 0 for membership refund guard", len(db.execArgs))
	}
}

func TestMarkDigitalEntitlementRevokedRequiresAffectedRows(t *testing.T) {
	revokedAt := time.Date(2026, 7, 16, 14, 0, 0, 0, time.UTC)

	t.Run("missing row", func(t *testing.T) {
		err := markDigitalEntitlementRevoked(context.Background(), &digitalEntitlementStateQueryer{tag: pgconn.NewCommandTag("UPDATE 0")}, 501, "admin-1", "risk review", revokedAt)
		if !errors.Is(err, domain.ErrInvalidOrderState) {
			t.Fatalf("markDigitalEntitlementRevoked() error = %v, want invalid order state", err)
		}
	})

	t.Run("updated row", func(t *testing.T) {
		db := &digitalEntitlementStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")}
		err := markDigitalEntitlementRevoked(context.Background(), db, 501, "admin-1", "risk review", revokedAt)
		if err != nil {
			t.Fatalf("markDigitalEntitlementRevoked() error = %v, want nil", err)
		}
		query := db.execQueries[0]
		for _, expected := range []string{
			"WHERE id = $1",
			"UPPER(TRIM(COALESCE(status, ''))) = $6",
			"revoked_at IS NULL",
			"LOWER(TRIM(COALESCE(grant_type, ''))) = 'membership'",
			"expires_at IS NOT NULL AND expires_at > $7",
			"LOWER(TRIM(COALESCE(grant_type, ''))) <> 'membership'",
			"(expires_at IS NULL OR expires_at > $7)",
		} {
			if !strings.Contains(query, expected) {
				t.Fatalf("revocation query = %q, want %q", query, expected)
			}
		}
		args := db.execArgs[0]
		if len(args) != 7 {
			t.Fatalf("revocation args = %+v, want 7 args", args)
		}
		if args[6] != revokedAt {
			t.Fatalf("effective time arg = %#v, want %v", args[6], revokedAt)
		}
	})
}

type digitalEntitlementQueryer struct {
	execArgs    [][]any
	execQueries []string
	execErrors  []error
	query       string
	queryArgs   []any
	queryRows   int
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

func (q *digitalEntitlementQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryRows++
	q.query = query
	q.queryArgs = append([]any(nil), args...)
	return digitalEntitlementLockRow{}
}

type digitalEntitlementLockRow struct{}

func (digitalEntitlementLockRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("expected membership lock count destination")
	}
	locks, ok := dest[0].(*int64)
	if !ok {
		return errors.New("expected membership lock count destination")
	}
	*locks = 1
	return nil
}

type digitalEntitlementStateQueryer struct {
	tag         pgconn.CommandTag
	execQueries []string
	execArgs    [][]any
}

func (q *digitalEntitlementStateQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	q.execQueries = append(q.execQueries, query)
	q.execArgs = append(q.execArgs, args)
	return q.tag, nil
}

func (q *digitalEntitlementStateQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *digitalEntitlementStateQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}
