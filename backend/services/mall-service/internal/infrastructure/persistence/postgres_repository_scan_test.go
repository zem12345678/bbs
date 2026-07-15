package persistence

import (
	"database/sql"
	"fmt"
	domain "mall-service/internal/domain/mall"
	"reflect"
	"testing"
	"time"
)

func TestScanCartItemIncludesProductGrantFields(t *testing.T) {
	createdAt := time.Date(2026, 7, 13, 9, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)

	item, err := scanCartItem(testScanner{
		int64(7),
		int32(2),
		createdAt,
		updatedAt,
		int64(101),
		"BADGE-E2E",
		"E2E 徽章",
		"数字徽章商品",
		"digital",
		"/images/badge.svg",
		"badge",
		"badge-e2e",
		int64(80),
		int64(5),
		int64(1),
		string(domain.ProductStatusActive),
		int32(9),
		createdAt,
		updatedAt,
	})
	if err != nil {
		t.Fatalf("scanCartItem() error = %v", err)
	}
	if item.Product.GrantType != "badge" || item.Product.GrantKey != "badge-e2e" {
		t.Fatalf("grant fields = %q/%q, want badge/badge-e2e", item.Product.GrantType, item.Product.GrantKey)
	}
	if item.Product.PriceCredits != 80 || item.Product.Status != domain.ProductStatusActive {
		t.Fatalf("product scan shifted fields: price=%d status=%s", item.Product.PriceCredits, item.Product.Status)
	}
}

func TestScanProductFavoriteIncludesProductGrantFields(t *testing.T) {
	createdAt := time.Date(2026, 7, 13, 9, 45, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)

	item, err := scanProductFavorite(testScanner{
		int64(9),
		createdAt,
		int64(202),
		"THEME-PRO",
		"高级主题",
		"主题包商品",
		"digital",
		"/images/theme.svg",
		"theme",
		"theme-pro",
		int64(500),
		int64(10),
		int64(3),
		string(domain.ProductStatusActive),
		int32(12),
		createdAt,
		updatedAt,
	})
	if err != nil {
		t.Fatalf("scanProductFavorite() error = %v", err)
	}
	if item.Product.GrantType != "theme" || item.Product.GrantKey != "theme-pro" {
		t.Fatalf("grant fields = %q/%q, want theme/theme-pro", item.Product.GrantType, item.Product.GrantKey)
	}
	if item.Product.PriceCredits != 500 || item.Product.Status != domain.ProductStatusActive {
		t.Fatalf("product scan shifted fields: price=%d status=%s", item.Product.PriceCredits, item.Product.Status)
	}
}

func TestScanOutboxRequeueAuditIncludesTraceFields(t *testing.T) {
	requeuedAt := time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC)

	audit, err := scanOutboxRequeueAudit(testScanner{
		int64(7),
		"evt-7",
		"order",
		int64(9001),
		"dead_letter",
		int(5),
		"publisher down",
		"42",
		requeuedAt,
	})
	if err != nil {
		t.Fatalf("scanOutboxRequeueAudit() error = %v", err)
	}
	if audit.EventID != "evt-7" || audit.AggregateType != "order" || audit.AggregateID != 9001 {
		t.Fatalf("audit aggregate = %+v, want evt-7/order/9001", audit)
	}
	if audit.PreviousStatus != "dead_letter" || audit.PreviousAttempts != 5 || audit.PreviousError != "publisher down" {
		t.Fatalf("audit previous state = %+v, want dead_letter/5/publisher down", audit)
	}
	if audit.OperatorID != "42" || !audit.RequeuedAt.Equal(requeuedAt) {
		t.Fatalf("audit operator/time = %q/%s, want 42/%s", audit.OperatorID, audit.RequeuedAt, requeuedAt)
	}
}

func TestScanFinanceAnomalyIncludesMoneyFields(t *testing.T) {
	updatedAt := time.Date(2026, 7, 13, 17, 0, 0, 0, time.UTC)

	anomaly, err := scanFinanceAnomaly(testScanner{
		"PAYMENT_MISMATCH",
		int64(9001),
		"M202607130001",
		int64(42),
		"PAID",
		int64(120),
		int64(80),
		int64(10),
		int64(-40),
		updatedAt,
	})
	if err != nil {
		t.Fatalf("scanFinanceAnomaly() error = %v", err)
	}
	if anomaly.IssueType != "PAYMENT_MISMATCH" || anomaly.OrderID != 9001 || anomaly.OrderNo != "M202607130001" || anomaly.UserID != 42 {
		t.Fatalf("anomaly identity = %+v, want PAYMENT_MISMATCH/9001/M202607130001/42", anomaly)
	}
	if anomaly.OrderStatus != domain.OrderStatusPaid {
		t.Fatalf("anomaly order status = %s, want PAID", anomaly.OrderStatus)
	}
	if anomaly.OrderTotalCredits != 120 || anomaly.SucceededPaymentCredits != 80 || anomaly.RefundedCredits != 10 || anomaly.DifferenceCredits != -40 {
		t.Fatalf("anomaly credits = %+v, want total 120 paid 80 refunded 10 diff -40", anomaly)
	}
	if !anomaly.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("anomaly updated_at = %s, want %s", anomaly.UpdatedAt, updatedAt)
	}
}

func TestScanDigitalEntitlementPreservesBlankGrantAndStatus(t *testing.T) {
	issuedAt := time.Date(2026, 7, 13, 18, 0, 0, 0, time.UTC)

	item, err := scanDigitalEntitlement(testScanner{
		int64(501),
		int64(9001),
		"M202607130001",
		int64(42),
		int64(101),
		"VIP-MONTH",
		"会员月卡",
		int32(1),
		"BBS-ENTITLEMENT",
		"",
		"",
		issuedAt,
		sql.NullTime{},
		"",
		sql.NullTime{},
		sql.NullInt64{},
		"",
		"",
	})
	if err != nil {
		t.Fatalf("scanDigitalEntitlement() error = %v", err)
	}
	if item.GrantType != "" || item.GrantKey != "" || item.Status != "" {
		t.Fatalf("scanDigitalEntitlement() grant/status = (%q, %q, %q), want blanks preserved", item.GrantType, item.GrantKey, item.Status)
	}
}

type testScanner []any

func (s testScanner) Scan(dest ...any) error {
	if len(dest) != len(s) {
		return fmt.Errorf("destinations = %d, values = %d", len(dest), len(s))
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return fmt.Errorf("destination %d is not a non-nil pointer", i)
		}
		value := reflect.ValueOf(s[i])
		if !value.IsValid() {
			target.Elem().Set(reflect.Zero(target.Elem().Type()))
			continue
		}
		if value.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(value)
			continue
		}
		if value.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(value.Convert(target.Elem().Type()))
			continue
		}
		return fmt.Errorf("value %d has type %s, cannot assign to %s", i, value.Type(), target.Elem().Type())
	}
	return nil
}
