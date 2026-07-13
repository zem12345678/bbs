package persistence

import (
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
