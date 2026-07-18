package mall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestCreateOrderAllowsDigitalProductWithoutShippingAddress(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			101: {
				ID:           101,
				Title:        "数字专栏",
				Category:     "digital",
				GrantType:    "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 120,
				Stock:        50,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "digital-order",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 101, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if repo.createOrderCalls != 1 {
		t.Fatalf("CreateOrder() calls = %d, want 1", repo.createOrderCalls)
	}
	if got := strings.TrimSpace(result.Order.Receiver); got != "" {
		t.Fatalf("CreateOrder() receiver = %q, want empty", got)
	}
	if got := strings.TrimSpace(result.Order.Phone); got != "" {
		t.Fatalf("CreateOrder() phone = %q, want empty", got)
	}
	if got := strings.TrimSpace(result.Order.Address); got != "" {
		t.Fatalf("CreateOrder() address = %q, want empty", got)
	}
	if got := result.Order.Items[0].Category; got != "digital" {
		t.Fatalf("CreateOrder() item category = %q, want digital", got)
	}
	if got := result.Order.Items[0].GrantType; got != "badge" {
		t.Fatalf("CreateOrder() item grant type = %q, want badge", got)
	}
	if got := result.Order.Items[0].GrantKey; got != "badge-founder" {
		t.Fatalf("CreateOrder() item grant key = %q, want badge-founder", got)
	}
}

func TestCreateOrderAllowsGrantedProductWithoutShippingAddress(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			102: {
				ID:           102,
				Title:        "创始会员徽章",
				Category:     "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 99,
				Stock:        20,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "granted-order",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 102, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if repo.createOrderCalls != 1 {
		t.Fatalf("CreateOrder() calls = %d, want 1", repo.createOrderCalls)
	}
	if got := strings.TrimSpace(result.Order.Receiver); got != "" {
		t.Fatalf("CreateOrder() receiver = %q, want empty", got)
	}
	if got := result.Order.Items[0].Category; got != "badge" {
		t.Fatalf("CreateOrder() item category = %q, want badge", got)
	}
	if got := result.Order.Items[0].GrantType; got != "badge" {
		t.Fatalf("CreateOrder() item grant type = %q, want inferred badge", got)
	}
	if got := result.Order.Items[0].GrantKey; got != "badge-founder" {
		t.Fatalf("CreateOrder() item grant key = %q, want badge-founder", got)
	}
}

func TestCreateOrderRejectsDuplicateActiveThemeEntitlement(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			103: {
				ID:           103,
				Title:        "高级主题",
				Category:     "digital",
				GrantType:    "theme",
				GrantKey:     "theme-pro",
				PriceCredits: 188,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "theme", GrantKey: "theme-pro", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-theme",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 103, Quantity: 1},
		},
	})
	if !errors.Is(err, domain.ErrActiveThemeEntitlementExists) {
		t.Fatalf("CreateOrder() error = %v, want ErrActiveThemeEntitlementExists", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
	if repo.listDigitalEntitlementsQuery.UserID != 7 || repo.listDigitalEntitlementsQuery.Status != domain.DigitalEntitlementStatusActive || repo.listDigitalEntitlementsQuery.GrantType != "theme" || repo.listDigitalEntitlementsQuery.GrantKey != "theme-pro" {
		t.Fatalf("ListDigitalEntitlements() query = %+v, want active theme/theme-pro for user 7", repo.listDigitalEntitlementsQuery)
	}
}

func TestCreateOrderRejectsDuplicateThemeGrantQuantity(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			103: {
				ID:           103,
				Title:        "高级主题",
				Category:     "digital",
				GrantType:    "theme",
				GrantKey:     "theme-pro",
				PriceCredits: 188,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order: domain.Order{UserID: 7},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-theme-quantity",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 103, Quantity: 2},
		},
	})
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("CreateOrder() error = %v, want ErrDuplicateThemeGrantInOrder", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
	if repo.listDigitalEntitlementsCalls != 0 || repo.openDigitalGrantOrderExistsCalls != 0 {
		t.Fatalf("entitlement checks = list:%d open:%d, want 0/0 after local duplicate theme rejection", repo.listDigitalEntitlementsCalls, repo.openDigitalGrantOrderExistsCalls)
	}
}

func TestCreateOrderRejectsDuplicateThemeGrantAcrossProducts(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			103: {
				ID:           103,
				Title:        "高级主题",
				Category:     "digital",
				GrantType:    "theme",
				GrantKey:     "theme-pro",
				PriceCredits: 188,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
			104: {
				ID:           104,
				Title:        "高级主题礼包",
				Category:     "digital",
				GrantType:    "theme",
				GrantKey:     "theme-pro",
				PriceCredits: 188,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order: domain.Order{UserID: 7},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-theme-products",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 103, Quantity: 1},
			{ProductID: 104, Quantity: 1},
		},
	})
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("CreateOrder() error = %v, want ErrDuplicateThemeGrantInOrder", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
}

func TestCreateOrderRejectsDuplicatePendingThemeOrder(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			104: {
				ID:           104,
				Title:        "高级主题",
				Category:     "digital",
				GrantType:    "theme",
				GrantKey:     "theme-pro",
				PriceCredits: 188,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order:                       domain.Order{UserID: 7},
		openDigitalGrantOrderExists: true,
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-pending-theme",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 104, Quantity: 1},
		},
	})
	if !errors.Is(err, domain.ErrPendingThemeOrderExists) {
		t.Fatalf("CreateOrder() error = %v, want ErrPendingThemeOrderExists", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderUserID != 7 || repo.openDigitalGrantOrderGrantType != "theme" || repo.openDigitalGrantOrderGrantKey != "theme-pro" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d user=%d grant=%q/%q, want one query for user 7 theme/theme-pro", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderUserID, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestCreateOrderRejectsDuplicatePendingMembershipOrder(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			106: {
				ID:           106,
				Title:        "会员月卡",
				Category:     "digital",
				GrantType:    "membership",
				GrantKey:     "vip-month",
				PriceCredits: 300,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order:                       domain.Order{UserID: 7},
		openDigitalGrantOrderExists: true,
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-pending-membership",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 106, Quantity: 1},
		},
	})
	if !errors.Is(err, domain.ErrPendingMembershipOrderExists) {
		t.Fatalf("CreateOrder() error = %v, want ErrPendingMembershipOrderExists", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
	if repo.listDigitalEntitlementsCalls != 0 {
		t.Fatalf("ListDigitalEntitlements() calls = %d, want 0 because active membership can renew", repo.listDigitalEntitlementsCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderUserID != 7 || repo.openDigitalGrantOrderGrantType != "membership" || repo.openDigitalGrantOrderGrantKey != "vip-month" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d user=%d grant=%q/%q, want one query for user 7 membership/vip-month", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderUserID, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestCreateOrderAllowsActiveMembershipRenewal(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			106: {
				ID:           106,
				Title:        "会员月卡",
				Category:     "digital",
				GrantType:    "membership",
				GrantKey:     "vip-month",
				PriceCredits: 300,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "membership", GrantKey: "vip-month", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "membership-renewal",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 106, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v, want nil", err)
	}
	if result.Order.ID == 0 || repo.createOrderCalls != 1 {
		t.Fatalf("CreateOrder() result=%+v calls=%d, want one created renewal order", result.Order, repo.createOrderCalls)
	}
	if repo.listDigitalEntitlementsCalls != 0 {
		t.Fatalf("ListDigitalEntitlements() calls = %d, want 0 because active membership renewal is allowed", repo.listDigitalEntitlementsCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderGrantType != "membership" || repo.openDigitalGrantOrderGrantKey != "vip-month" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d grant=%q/%q, want membership/vip-month", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestSetCartItemRejectsUnavailableOrInsufficientStockProduct(t *testing.T) {
	tests := []struct {
		name     string
		product  domain.Product
		quantity int32
		wantErr  error
	}{
		{
			name:     "unavailable",
			product:  domain.Product{ID: 201, Title: "下架权益", Category: "digital", GrantType: "membership", GrantKey: "vip-month", PriceCredits: 300, Stock: 5, Status: domain.ProductStatusDraft},
			quantity: 1,
			wantErr:  domain.ErrProductUnavailable,
		},
		{
			name:     "insufficient stock",
			product:  domain.Product{ID: 202, Title: "库存不足权益", Category: "digital", GrantType: "membership", GrantKey: "vip-month", PriceCredits: 300, Stock: 1, Status: domain.ProductStatusActive},
			quantity: 2,
			wantErr:  domain.ErrInsufficientStock,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &orderRepoStub{products: map[int64]domain.Product{tt.product.ID: tt.product}}
			svc := NewService(repo, nil, time.Minute)

			_, _, err := svc.SetCartItem(context.Background(), SetCartItemCommand{
				UserID:    7,
				ProductID: tt.product.ID,
				Quantity:  tt.quantity,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SetCartItem() error = %v, want %v", err, tt.wantErr)
			}
			if repo.setCartItemCalls != 0 {
				t.Fatalf("SetCartItem() repo calls = %d, want 0", repo.setCartItemCalls)
			}
		})
	}
}

func TestSetCartItemRejectsDuplicateBadgeQuantity(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			205: {
				ID:           205,
				Title:        "创始会员徽章",
				Category:     "digital",
				GrantType:    "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 80,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.SetCartItem(context.Background(), SetCartItemCommand{
		UserID:    7,
		ProductID: 205,
		Quantity:  2,
	})
	if !errors.Is(err, domain.ErrDuplicateBadgeGrantInOrder) {
		t.Fatalf("SetCartItem() error = %v, want ErrDuplicateBadgeGrantInOrder", err)
	}
	if repo.setCartItemCalls != 0 || repo.listDigitalEntitlementsCalls != 0 || repo.openDigitalGrantOrderExistsCalls != 0 {
		t.Fatalf("repo calls set=%d list=%d open=%d, want 0/0/0 after local duplicate badge rejection", repo.setCartItemCalls, repo.listDigitalEntitlementsCalls, repo.openDigitalGrantOrderExistsCalls)
	}
}

func TestSetCartItemRejectsDuplicateBadgeGrantAlreadyInCart(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			206: {
				ID:           206,
				Title:        "徽章礼包 B",
				Category:     "digital",
				GrantType:    "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 80,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           204,
					Title:        "徽章礼包 A",
					Category:     "digital",
					GrantType:    "badge",
					GrantKey:     "badge-founder",
					PriceCredits: 80,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 1,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.SetCartItem(context.Background(), SetCartItemCommand{
		UserID:    7,
		ProductID: 206,
		Quantity:  1,
	})
	if !errors.Is(err, domain.ErrDuplicateBadgeGrantInOrder) {
		t.Fatalf("SetCartItem() error = %v, want ErrDuplicateBadgeGrantInOrder", err)
	}
	if repo.setCartItemCalls != 0 {
		t.Fatalf("SetCartItem() repo calls = %d, want 0", repo.setCartItemCalls)
	}
}

func TestSetCartItemRejectsDuplicatePendingMembershipOrder(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			207: {
				ID:           207,
				Title:        "会员月卡",
				Category:     "digital",
				GrantType:    "membership",
				GrantKey:     "vip-month",
				PriceCredits: 300,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		openDigitalGrantOrderExists: true,
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.SetCartItem(context.Background(), SetCartItemCommand{
		UserID:    7,
		ProductID: 207,
		Quantity:  1,
	})
	if !errors.Is(err, domain.ErrPendingMembershipOrderExists) {
		t.Fatalf("SetCartItem() error = %v, want ErrPendingMembershipOrderExists", err)
	}
	if repo.setCartItemCalls != 0 || repo.listDigitalEntitlementsCalls != 0 {
		t.Fatalf("repo calls set=%d list=%d, want 0/0", repo.setCartItemCalls, repo.listDigitalEntitlementsCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderGrantType != "membership" || repo.openDigitalGrantOrderGrantKey != "vip-month" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d grant=%q/%q, want membership/vip-month", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestSetCartItemAllowsActiveMembershipRenewal(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			208: {
				ID:           208,
				Title:        "会员月卡",
				Category:     "digital",
				GrantType:    "membership",
				GrantKey:     "vip-month",
				PriceCredits: 300,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "membership", GrantKey: "vip-month", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.SetCartItem(context.Background(), SetCartItemCommand{
		UserID:    7,
		ProductID: 208,
		Quantity:  2,
	})
	if err != nil {
		t.Fatalf("SetCartItem() error = %v, want nil", err)
	}
	if repo.setCartItemCalls != 1 || repo.setCartItemUserID != 7 || repo.setCartItemProductID != 208 || repo.setCartItemQuantity != 2 {
		t.Fatalf("SetCartItem() calls=%d user=%d product=%d quantity=%d, want one membership renewal cart write", repo.setCartItemCalls, repo.setCartItemUserID, repo.setCartItemProductID, repo.setCartItemQuantity)
	}
	if repo.listDigitalEntitlementsCalls != 0 || repo.openDigitalGrantOrderExistsCalls != 1 {
		t.Fatalf("repo checks list=%d open=%d, want 0/1 for active membership renewal", repo.listDigitalEntitlementsCalls, repo.openDigitalGrantOrderExistsCalls)
	}
}

func TestCreateOrderRejectsDuplicateActiveBadgeEntitlement(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			105: {
				ID:           105,
				Title:        "创始会员徽章",
				Category:     "digital",
				GrantType:    "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 80,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "badge", GrantKey: "badge-founder", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-badge",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 105, Quantity: 1},
		},
	})
	if !errors.Is(err, domain.ErrActiveBadgeEntitlementExists) {
		t.Fatalf("CreateOrder() error = %v, want ErrActiveBadgeEntitlementExists", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
	if repo.listDigitalEntitlementsQuery.UserID != 7 || repo.listDigitalEntitlementsQuery.Status != domain.DigitalEntitlementStatusActive || repo.listDigitalEntitlementsQuery.GrantType != "badge" || repo.listDigitalEntitlementsQuery.GrantKey != "badge-founder" {
		t.Fatalf("ListDigitalEntitlements() query = %+v, want active badge/badge-founder for user 7", repo.listDigitalEntitlementsQuery)
	}
}

func TestCreateOrderRejectsDuplicateBadgeGrantQuantity(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			105: {
				ID:           105,
				Title:        "创始会员徽章",
				Category:     "digital",
				GrantType:    "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 80,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order: domain.Order{UserID: 7},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-badge-quantity",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 105, Quantity: 2},
		},
	})
	if !errors.Is(err, domain.ErrDuplicateBadgeGrantInOrder) {
		t.Fatalf("CreateOrder() error = %v, want ErrDuplicateBadgeGrantInOrder", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
	if repo.listDigitalEntitlementsCalls != 0 || repo.openDigitalGrantOrderExistsCalls != 0 {
		t.Fatalf("entitlement checks = list:%d open:%d, want 0/0 after local duplicate badge rejection", repo.listDigitalEntitlementsCalls, repo.openDigitalGrantOrderExistsCalls)
	}
}

func TestCreateOrderRejectsDuplicatePendingBadgeOrder(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			105: {
				ID:           105,
				Title:        "创始会员徽章",
				Category:     "digital",
				GrantType:    "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 80,
				Stock:        10,
				Status:       domain.ProductStatusActive,
			},
		},
		order:                       domain.Order{UserID: 7},
		openDigitalGrantOrderExists: true,
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "duplicate-pending-badge",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 105, Quantity: 1},
		},
	})
	if !errors.Is(err, domain.ErrPendingBadgeOrderExists) {
		t.Fatalf("CreateOrder() error = %v, want ErrPendingBadgeOrderExists", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderUserID != 7 || repo.openDigitalGrantOrderGrantType != "badge" || repo.openDigitalGrantOrderGrantKey != "badge-founder" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d user=%d grant=%q/%q, want one query for user 7 badge/badge-founder", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderUserID, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestCreateOrderRequiresShippingAddressForPhysicalProduct(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			201: {
				ID:           201,
				Title:        "实体周边",
				Category:     "books",
				PriceCredits: 80,
				Stock:        12,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "physical-order",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 201, Quantity: 1},
		},
	})
	if err == nil {
		t.Fatal("CreateOrder() error = nil, want shipping validation error")
	}
	if !strings.Contains(err.Error(), "receiver is required") {
		t.Fatalf("CreateOrder() error = %q, want receiver validation", err.Error())
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
}

func TestCreateOrderRejectsQuantityAboveStockAfterMergingItems(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			202: {
				ID:           202,
				Title:        "数字专栏",
				Category:     "digital",
				PriceCredits: 80,
				Stock:        3,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "oversold-direct-order",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 202, Quantity: 2},
			{ProductID: 202, Quantity: 2},
		},
	})
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("CreateOrder() error = %v, want insufficient stock", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
}

func TestCreateOrderDoesNotReuseAnotherUsersIdempotencyKey(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			203: {ID: 203, Title: "数字专栏", Category: "digital", PriceCredits: 80, Stock: 3, Status: domain.ProductStatusActive},
		},
		idempotencyOrders: map[string]domain.Order{
			"7:shared-key": {ID: 7001, UserID: 7, IdempotencyKey: "shared-key", Receiver: "用户一"},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "shared-key",
		UserID:         8,
		Items:          []domain.CreateOrderItem{{ProductID: 203, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if result.Duplicate {
		t.Fatal("CreateOrder() reused another user's order")
	}
	if result.Order.UserID != 8 {
		t.Fatalf("CreateOrder() user = %d, want 8", result.Order.UserID)
	}
	if repo.createOrderCalls != 1 {
		t.Fatalf("CreateOrder() calls = %d, want 1", repo.createOrderCalls)
	}
}

func TestCreateOrderReturnsExistingMatchingIdempotencyRequest(t *testing.T) {
	repo := &orderRepoStub{
		idempotencyOrders: map[string]domain.Order{
			"7:retry-key": {
				ID:             7002,
				UserID:         7,
				IdempotencyKey: "retry-key",
				CouponCode:     "SAVE10",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.1 Road",
				Items:          []domain.OrderItem{{ProductID: 203, Quantity: 3}},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "retry-key",
		UserID:         7,
		CouponCode:     " save10 ",
		Receiver:       " Alice ",
		Phone:          " 13800138000 ",
		Address:        " No.1 Road ",
		Items: []domain.CreateOrderItem{
			{ProductID: 203, Quantity: 1},
			{ProductID: 203, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if !result.Duplicate || result.Order.ID != 7002 {
		t.Fatalf("CreateOrder() result = %+v, want duplicate order 7002", result)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
}

func TestCreateOrderRejectsMismatchedIdempotencyRequest(t *testing.T) {
	tests := []struct {
		name    string
		command CreateOrderCommand
	}{
		{
			name: "different item",
			command: CreateOrderCommand{
				IdempotencyKey: "retry-key",
				UserID:         7,
				CouponCode:     "SAVE10",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.1 Road",
				Items:          []domain.CreateOrderItem{{ProductID: 204, Quantity: 1}},
			},
		},
		{
			name: "different coupon",
			command: CreateOrderCommand{
				IdempotencyKey: "retry-key",
				UserID:         7,
				CouponCode:     "SAVE20",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.1 Road",
				Items:          []domain.CreateOrderItem{{ProductID: 203, Quantity: 1}},
			},
		},
		{
			name: "different address",
			command: CreateOrderCommand{
				IdempotencyKey: "retry-key",
				UserID:         7,
				CouponCode:     "SAVE10",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.2 Road",
				Items:          []domain.CreateOrderItem{{ProductID: 203, Quantity: 1}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &orderRepoStub{
				idempotencyOrders: map[string]domain.Order{
					"7:retry-key": {
						ID:             7002,
						UserID:         7,
						IdempotencyKey: "retry-key",
						CouponCode:     "SAVE10",
						Receiver:       "Alice",
						Phone:          "13800138000",
						Address:        "No.1 Road",
						Items:          []domain.OrderItem{{ProductID: 203, Quantity: 1}},
					},
				},
			}
			svc := NewService(repo, nil, time.Minute)

			_, err := svc.CreateOrder(context.Background(), test.command)
			if !errors.Is(err, domain.ErrDuplicateReference) {
				t.Fatalf("CreateOrder() error = %v, want duplicate reference", err)
			}
			if repo.createOrderCalls != 0 {
				t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
			}
		})
	}
}

func TestCheckoutCartReturnsExistingMatchingIdempotencyRequestAfterCartCleared(t *testing.T) {
	repo := &orderRepoStub{
		idempotencyOrders: map[string]domain.Order{
			"7:cart-retry": {
				ID:             8001,
				UserID:         7,
				IdempotencyKey: "cart-retry",
				CouponCode:     "CART10",
				Items:          []domain.OrderItem{{ProductID: 301, Quantity: 1}},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "cart-retry",
		UserID:         7,
		CouponCode:     " cart10 ",
	})
	if err != nil {
		t.Fatalf("CheckoutCart() error = %v", err)
	}
	if !result.Duplicate || result.Order.ID != 8001 {
		t.Fatalf("CheckoutCart() result = %+v, want duplicate order 8001", result)
	}
	if repo.listCartItemsCalls != 1 || repo.createOrderFromCartCalls != 0 {
		t.Fatalf("repo calls list=%d create=%d, want 1/0", repo.listCartItemsCalls, repo.createOrderFromCartCalls)
	}
}

func TestCheckoutCartRejectsMismatchedIdempotencyRequestAfterCartCleared(t *testing.T) {
	base := CheckoutCartCommand{
		IdempotencyKey: "cart-retry",
		UserID:         7,
		CouponCode:     "CART10",
		Receiver:       "Alice",
		Phone:          "13800138000",
		Address:        "No.1 Road",
	}
	tests := []struct {
		name   string
		mutate func(*CheckoutCartCommand)
	}{
		{
			name: "coupon",
			mutate: func(command *CheckoutCartCommand) {
				command.CouponCode = "DIFFERENT"
			},
		},
		{
			name: "receiver",
			mutate: func(command *CheckoutCartCommand) {
				command.Receiver = "Bob"
			},
		},
		{
			name: "phone",
			mutate: func(command *CheckoutCartCommand) {
				command.Phone = "13900139000"
			},
		},
		{
			name: "address",
			mutate: func(command *CheckoutCartCommand) {
				command.Address = "No.2 Road"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &orderRepoStub{
				idempotencyOrders: map[string]domain.Order{
					"7:cart-retry": {
						ID:             8004,
						UserID:         7,
						IdempotencyKey: "cart-retry",
						CouponCode:     "CART10",
						Receiver:       "Alice",
						Phone:          "13800138000",
						Address:        "No.1 Road",
						Items:          []domain.OrderItem{{ProductID: 301, Quantity: 1}},
					},
				},
			}
			svc := NewService(repo, nil, time.Minute)
			command := base
			test.mutate(&command)

			_, err := svc.CheckoutCart(context.Background(), command)
			if !errors.Is(err, domain.ErrDuplicateReference) {
				t.Fatalf("CheckoutCart() error = %v, want duplicate reference", err)
			}
			if repo.listCartItemsCalls != 1 || repo.createOrderFromCartCalls != 0 {
				t.Fatalf("repo calls list=%d create=%d, want 1/0", repo.listCartItemsCalls, repo.createOrderFromCartCalls)
			}
		})
	}
}

func TestCheckoutCartReturnsExistingMatchingIdempotencyCart(t *testing.T) {
	repo := &orderRepoStub{
		idempotencyOrders: map[string]domain.Order{
			"7:cart-retry": {
				ID:             8002,
				UserID:         7,
				IdempotencyKey: "cart-retry",
				CouponCode:     "CART10",
				Receiver:       "Alice",
				Phone:          "13800138000",
				Address:        "No.1 Road",
				Items:          []domain.OrderItem{{ProductID: 301, Quantity: 3}},
			},
		},
		cartItems: []domain.CartItem{
			{
				Product:  domain.Product{ID: 301, Title: "Physical Book", Category: "goods", PriceCredits: 50, Stock: 10, Status: domain.ProductStatusActive},
				Quantity: 3,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "cart-retry",
		UserID:         7,
		CouponCode:     " cart10 ",
		Receiver:       " Alice ",
		Phone:          " 13800138000 ",
		Address:        " No.1 Road ",
	})
	if err != nil {
		t.Fatalf("CheckoutCart() error = %v", err)
	}
	if !result.Duplicate || result.Order.ID != 8002 {
		t.Fatalf("CheckoutCart() result = %+v, want duplicate order 8002", result)
	}
	if repo.listCartItemsCalls != 1 || repo.createOrderFromCartCalls != 0 {
		t.Fatalf("repo calls list=%d create=%d, want 1/0", repo.listCartItemsCalls, repo.createOrderFromCartCalls)
	}
}

func TestCheckoutCartRejectsMismatchedIdempotencyCart(t *testing.T) {
	repo := &orderRepoStub{
		idempotencyOrders: map[string]domain.Order{
			"7:cart-retry": {
				ID:             8003,
				UserID:         7,
				IdempotencyKey: "cart-retry",
				Items:          []domain.OrderItem{{ProductID: 301, Quantity: 1}},
			},
		},
		cartItems: []domain.CartItem{
			{
				Product:  domain.Product{ID: 302, Title: "Different Item", Category: "digital", PriceCredits: 50, Stock: 10, Status: domain.ProductStatusActive},
				Quantity: 1,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "cart-retry",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrDuplicateReference) {
		t.Fatalf("CheckoutCart() error = %v, want duplicate reference", err)
	}
	if repo.listCartItemsCalls != 1 || repo.createOrderFromCartCalls != 0 {
		t.Fatalf("repo calls list=%d create=%d, want 1/0", repo.listCartItemsCalls, repo.createOrderFromCartCalls)
	}
}

func TestCheckoutCartRequiresShippingWhenAnyItemNeedsDelivery(t *testing.T) {
	t.Run("digital only", func(t *testing.T) {
		repo := &orderRepoStub{
			cartItems: []domain.CartItem{
				{
					Product: domain.Product{
						ID:           301,
						Title:        "数字专栏",
						Category:     "digital",
						PriceCredits: 60,
						Stock:        100,
						Status:       domain.ProductStatusActive,
					},
					Quantity: 2,
				},
			},
		}
		svc := NewService(repo, nil, time.Minute)

		result, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
			IdempotencyKey: "digital-cart",
			UserID:         7,
		})
		if err != nil {
			t.Fatalf("CheckoutCart() error = %v", err)
		}
		if repo.createOrderFromCartCalls != 1 {
			t.Fatalf("CheckoutCart() calls = %d, want 1", repo.createOrderFromCartCalls)
		}
		if got := strings.TrimSpace(result.Order.Receiver); got != "" {
			t.Fatalf("CheckoutCart() receiver = %q, want empty", got)
		}
	})

	t.Run("granted product", func(t *testing.T) {
		repo := &orderRepoStub{
			cartItems: []domain.CartItem{
				{
					Product: domain.Product{
						ID:           302,
						Title:        "会员月卡",
						Category:     "membership",
						GrantType:    "membership",
						GrantKey:     "vip-month",
						PriceCredits: 120,
						Stock:        100,
						Status:       domain.ProductStatusActive,
					},
					Quantity: 1,
				},
			},
		}
		svc := NewService(repo, nil, time.Minute)

		result, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
			IdempotencyKey: "granted-cart",
			UserID:         7,
		})
		if err != nil {
			t.Fatalf("CheckoutCart() error = %v", err)
		}
		if repo.createOrderFromCartCalls != 1 {
			t.Fatalf("CheckoutCart() calls = %d, want 1", repo.createOrderFromCartCalls)
		}
		if got := strings.TrimSpace(result.Order.Receiver); got != "" {
			t.Fatalf("CheckoutCart() receiver = %q, want empty", got)
		}
		if got := result.Order.Items[0].GrantType; got != "membership" {
			t.Fatalf("CheckoutCart() grant type = %q, want membership", got)
		}
	})

	t.Run("mixed cart", func(t *testing.T) {
		repo := &orderRepoStub{
			cartItems: []domain.CartItem{
				{
					Product: domain.Product{
						ID:           401,
						Title:        "数字专栏",
						Category:     "digital",
						PriceCredits: 60,
						Stock:        100,
						Status:       domain.ProductStatusActive,
					},
					Quantity: 1,
				},
				{
					Product: domain.Product{
						ID:           402,
						Title:        "实体周边",
						Category:     "merch",
						PriceCredits: 90,
						Stock:        100,
						Status:       domain.ProductStatusActive,
					},
					Quantity: 1,
				},
			},
		}
		svc := NewService(repo, nil, time.Minute)

		_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
			IdempotencyKey: "mixed-cart",
			UserID:         7,
		})
		if err == nil {
			t.Fatal("CheckoutCart() error = nil, want shipping validation error")
		}
		if !strings.Contains(err.Error(), "receiver is required") {
			t.Fatalf("CheckoutCart() error = %q, want receiver validation", err.Error())
		}
		if repo.createOrderFromCartCalls != 0 {
			t.Fatalf("CheckoutCart() calls = %d, want 0", repo.createOrderFromCartCalls)
		}
	})
}

func TestCheckoutCartRejectsDuplicateActiveThemeEntitlement(t *testing.T) {
	repo := &orderRepoStub{
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           304,
					Title:        "高级主题",
					Category:     "digital",
					GrantType:    "theme",
					GrantKey:     "theme-pro",
					PriceCredits: 188,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 1,
			},
		},
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "theme", GrantKey: "theme-pro", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "duplicate-theme-cart",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrActiveThemeEntitlementExists) {
		t.Fatalf("CheckoutCart() error = %v, want ErrActiveThemeEntitlementExists", err)
	}
	if repo.createOrderFromCartCalls != 0 {
		t.Fatalf("CreateOrderFromCart() calls = %d, want 0", repo.createOrderFromCartCalls)
	}
}

func TestCheckoutCartRejectsDuplicateThemeGrantQuantity(t *testing.T) {
	repo := &orderRepoStub{
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           304,
					Title:        "高级主题",
					Category:     "digital",
					GrantType:    "theme",
					GrantKey:     "theme-pro",
					PriceCredits: 188,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 2,
			},
		},
		order: domain.Order{UserID: 7},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "duplicate-theme-cart",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("CheckoutCart() error = %v, want ErrDuplicateThemeGrantInOrder", err)
	}
	if repo.createOrderFromCartCalls != 0 {
		t.Fatalf("CreateOrderFromCart() calls = %d, want 0", repo.createOrderFromCartCalls)
	}
	if repo.listDigitalEntitlementsCalls != 0 || repo.openDigitalGrantOrderExistsCalls != 0 {
		t.Fatalf("entitlement checks = list:%d open:%d, want 0/0 after local duplicate theme rejection", repo.listDigitalEntitlementsCalls, repo.openDigitalGrantOrderExistsCalls)
	}
}

func TestCheckoutCartRejectsDuplicatePendingThemeOrder(t *testing.T) {
	repo := &orderRepoStub{
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           305,
					Title:        "高级主题",
					Category:     "digital",
					GrantType:    "theme",
					GrantKey:     "theme-pro",
					PriceCredits: 188,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 1,
			},
		},
		order:                       domain.Order{UserID: 7},
		openDigitalGrantOrderExists: true,
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "duplicate-pending-theme-cart",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrPendingThemeOrderExists) {
		t.Fatalf("CheckoutCart() error = %v, want ErrPendingThemeOrderExists", err)
	}
	if repo.createOrderFromCartCalls != 0 {
		t.Fatalf("CreateOrderFromCart() calls = %d, want 0", repo.createOrderFromCartCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderUserID != 7 || repo.openDigitalGrantOrderGrantType != "theme" || repo.openDigitalGrantOrderGrantKey != "theme-pro" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d user=%d grant=%q/%q, want one query for user 7 theme/theme-pro", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderUserID, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestCheckoutCartRejectsDuplicatePendingMembershipOrder(t *testing.T) {
	repo := &orderRepoStub{
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           307,
					Title:        "会员月卡",
					Category:     "digital",
					GrantType:    "membership",
					GrantKey:     "vip-month",
					PriceCredits: 300,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 1,
			},
		},
		order:                       domain.Order{UserID: 7},
		openDigitalGrantOrderExists: true,
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "duplicate-pending-membership-cart",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrPendingMembershipOrderExists) {
		t.Fatalf("CheckoutCart() error = %v, want ErrPendingMembershipOrderExists", err)
	}
	if repo.createOrderFromCartCalls != 0 {
		t.Fatalf("CreateOrderFromCart() calls = %d, want 0", repo.createOrderFromCartCalls)
	}
	if repo.listDigitalEntitlementsCalls != 0 {
		t.Fatalf("ListDigitalEntitlements() calls = %d, want 0 because active membership can renew", repo.listDigitalEntitlementsCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderUserID != 7 || repo.openDigitalGrantOrderGrantType != "membership" || repo.openDigitalGrantOrderGrantKey != "vip-month" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d user=%d grant=%q/%q, want one query for user 7 membership/vip-month", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderUserID, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestCheckoutCartRejectsDuplicateActiveBadgeEntitlement(t *testing.T) {
	repo := &orderRepoStub{
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           306,
					Title:        "创始会员徽章",
					Category:     "digital",
					GrantType:    "badge",
					GrantKey:     "badge-founder",
					PriceCredits: 80,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 1,
			},
		},
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "badge", GrantKey: "badge-founder", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "duplicate-badge-cart",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrActiveBadgeEntitlementExists) {
		t.Fatalf("CheckoutCart() error = %v, want ErrActiveBadgeEntitlementExists", err)
	}
	if repo.createOrderFromCartCalls != 0 {
		t.Fatalf("CreateOrderFromCart() calls = %d, want 0", repo.createOrderFromCartCalls)
	}
}

func TestCheckoutCartRejectsDuplicateBadgeGrantQuantity(t *testing.T) {
	repo := &orderRepoStub{
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           306,
					Title:        "创始会员徽章",
					Category:     "digital",
					GrantType:    "badge",
					GrantKey:     "badge-founder",
					PriceCredits: 80,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 2,
			},
		},
		order: domain.Order{UserID: 7},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "duplicate-badge-quantity-cart",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrDuplicateBadgeGrantInOrder) {
		t.Fatalf("CheckoutCart() error = %v, want ErrDuplicateBadgeGrantInOrder", err)
	}
	if repo.createOrderFromCartCalls != 0 {
		t.Fatalf("CreateOrderFromCart() calls = %d, want 0", repo.createOrderFromCartCalls)
	}
	if repo.listDigitalEntitlementsCalls != 0 || repo.openDigitalGrantOrderExistsCalls != 0 {
		t.Fatalf("entitlement checks = list:%d open:%d, want 0/0 after local duplicate badge rejection", repo.listDigitalEntitlementsCalls, repo.openDigitalGrantOrderExistsCalls)
	}
}

func TestCheckoutCartRejectsDuplicatePendingBadgeOrder(t *testing.T) {
	repo := &orderRepoStub{
		cartItems: []domain.CartItem{
			{
				Product: domain.Product{
					ID:           306,
					Title:        "创始会员徽章",
					Category:     "digital",
					GrantType:    "badge",
					GrantKey:     "badge-founder",
					PriceCredits: 80,
					Stock:        10,
					Status:       domain.ProductStatusActive,
				},
				Quantity: 1,
			},
		},
		order:                       domain.Order{UserID: 7},
		openDigitalGrantOrderExists: true,
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
		IdempotencyKey: "duplicate-pending-badge-cart",
		UserID:         7,
	})
	if !errors.Is(err, domain.ErrPendingBadgeOrderExists) {
		t.Fatalf("CheckoutCart() error = %v, want ErrPendingBadgeOrderExists", err)
	}
	if repo.createOrderFromCartCalls != 0 {
		t.Fatalf("CreateOrderFromCart() calls = %d, want 0", repo.createOrderFromCartCalls)
	}
	if repo.openDigitalGrantOrderExistsCalls != 1 || repo.openDigitalGrantOrderUserID != 7 || repo.openDigitalGrantOrderGrantType != "badge" || repo.openDigitalGrantOrderGrantKey != "badge-founder" {
		t.Fatalf("OpenDigitalGrantOrderExists() calls=%d user=%d grant=%q/%q, want one query for user 7 badge/badge-founder", repo.openDigitalGrantOrderExistsCalls, repo.openDigitalGrantOrderUserID, repo.openDigitalGrantOrderGrantType, repo.openDigitalGrantOrderGrantKey)
	}
}

func TestCreateOrderRejectsAmountOverflow(t *testing.T) {
	for _, test := range []struct {
		name     string
		products map[int64]domain.Product
		items    []domain.CreateOrderItem
	}{
		{
			name: "subtotal multiplication",
			products: map[int64]domain.Product{
				501: {ID: 501, Title: "高价数字权益", Category: "digital", PriceCredits: math.MaxInt64/2 + 1, Stock: 2, Status: domain.ProductStatusActive},
			},
			items: []domain.CreateOrderItem{{ProductID: 501, Quantity: 2}},
		},
		{
			name: "order total accumulation",
			products: map[int64]domain.Product{
				502: {ID: 502, Title: "高价数字权益", Category: "digital", PriceCredits: math.MaxInt64 - 10, Stock: 1, Status: domain.ProductStatusActive},
				503: {ID: 503, Title: "数字权益", Category: "digital", PriceCredits: 20, Stock: 1, Status: domain.ProductStatusActive},
			},
			items: []domain.CreateOrderItem{{ProductID: 502, Quantity: 1}, {ProductID: 503, Quantity: 1}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &orderRepoStub{products: test.products}
			svc := NewService(repo, nil, time.Minute)

			_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
				IdempotencyKey: "overflow-direct-" + test.name,
				UserID:         7,
				Items:          test.items,
			})
			if err == nil || !strings.Contains(err.Error(), "order amount is too large") {
				t.Fatalf("CreateOrder() error = %v, want amount overflow", err)
			}
			if repo.createOrderCalls != 0 {
				t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
			}
		})
	}
}

func TestCheckoutCartRejectsAmountOverflow(t *testing.T) {
	for _, test := range []struct {
		name  string
		items []domain.CartItem
	}{
		{
			name: "subtotal multiplication",
			items: []domain.CartItem{{
				Product:  domain.Product{ID: 601, Title: "高价数字权益", Category: "digital", PriceCredits: math.MaxInt64/2 + 1, Stock: 2, Status: domain.ProductStatusActive},
				Quantity: 2,
			}},
		},
		{
			name: "order total accumulation",
			items: []domain.CartItem{
				{Product: domain.Product{ID: 602, Title: "高价数字权益", Category: "digital", PriceCredits: math.MaxInt64 - 10, Stock: 1, Status: domain.ProductStatusActive}, Quantity: 1},
				{Product: domain.Product{ID: 603, Title: "数字权益", Category: "digital", PriceCredits: 20, Stock: 1, Status: domain.ProductStatusActive}, Quantity: 1},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &orderRepoStub{cartItems: test.items}
			svc := NewService(repo, nil, time.Minute)

			_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
				IdempotencyKey: "overflow-cart-" + test.name,
				UserID:         7,
			})
			if err == nil || !strings.Contains(err.Error(), "order amount is too large") {
				t.Fatalf("CheckoutCart() error = %v, want amount overflow", err)
			}
			if repo.createOrderFromCartCalls != 0 {
				t.Fatalf("CheckoutCart() calls = %d, want 0", repo.createOrderFromCartCalls)
			}
		})
	}
}

func TestCreateRefundRequestRequiresUserNote(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           501,
			OrderNo:      "M501",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaid,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.CreateRefundRequest(context.Background(), CreateRefundRequestCommand{
		OrderID: 501,
		UserID:  7,
		Reason:  "quality",
		Note:    " 短 ",
	})
	if err == nil {
		t.Fatal("CreateRefundRequest() error = nil, want note validation error")
	}
	if !strings.Contains(err.Error(), "refund note must be at least 4 characters") {
		t.Fatalf("CreateRefundRequest() error = %q, want note validation", err.Error())
	}
	if repo.createRefundRequestCalls != 0 {
		t.Fatalf("CreateRefundRequest() calls = %d, want 0", repo.createRefundRequestCalls)
	}
}

func TestCreateRefundRequestTrimsAndPersistsUserNote(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           601,
			OrderNo:      "M601",
			UserID:       7,
			TotalCredits: 200,
			Status:       domain.OrderStatusCompleted,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	refund, duplicate, err := svc.CreateRefundRequest(context.Background(), CreateRefundRequestCommand{
		OrderID: 601,
		UserID:  7,
		Reason:  "after_sale",
		Note:    "  包装破损影响使用  ",
	})
	if err != nil {
		t.Fatalf("CreateRefundRequest() error = %v", err)
	}
	if duplicate {
		t.Fatal("CreateRefundRequest() duplicate = true, want false")
	}
	if repo.createRefundRequestCalls != 1 {
		t.Fatalf("CreateRefundRequest() calls = %d, want 1", repo.createRefundRequestCalls)
	}
	if refund.UserNote != "包装破损影响使用" {
		t.Fatalf("CreateRefundRequest() note = %q, want trimmed note", refund.UserNote)
	}
}

func TestCreateRefundRequestRejectsMembershipOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           603,
			OrderNo:      "M603",
			UserID:       7,
			TotalCredits: 300,
			Status:       domain.OrderStatusPaid,
			Items: []domain.OrderItem{
				{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.CreateRefundRequest(context.Background(), CreateRefundRequestCommand{
		OrderID: 603,
		UserID:  7,
		Reason:  "after_sale",
		Note:    "会员订单售后申请",
	})
	if !errors.Is(err, domain.ErrMembershipRefundUnavailable) {
		t.Fatalf("CreateRefundRequest() error = %v, want ErrMembershipRefundUnavailable", err)
	}
	if repo.createRefundRequestCalls != 0 {
		t.Fatalf("CreateRefundRequest() calls = %d, want 0", repo.createRefundRequestCalls)
	}
}

func TestAdminReviewRefundRequestRejectsMembershipOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           604,
			OrderNo:      "M604",
			UserID:       7,
			TotalCredits: 300,
			Status:       domain.OrderStatusPaid,
			Items: []domain.OrderItem{
				{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
			},
		},
		refund: domain.RefundRequest{
			ID:            704,
			OrderID:       604,
			OrderNo:       "M604",
			UserID:        7,
			AmountCredits: 300,
			Status:        domain.RefundStatusRequested,
		},
	}
	svc := NewService(repo, &creditChargerStub{}, time.Minute)

	_, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID:     704,
		Approved:     true,
		OperatorID:   "ops",
		AdminNote:    "会员订单不可退款",
		RestoreStock: true,
	})
	if !errors.Is(err, domain.ErrMembershipRefundUnavailable) {
		t.Fatalf("AdminReviewRefundRequest() error = %v, want ErrMembershipRefundUnavailable", err)
	}
	if repo.startRefundApprovalCalls != 0 {
		t.Fatalf("StartRefundApproval() calls = %d, want 0", repo.startRefundApprovalCalls)
	}
	if repo.completeRefundApprovalCalls != 0 {
		t.Fatalf("CompleteRefundApproval() calls = %d, want 0", repo.completeRefundApprovalCalls)
	}
}

func TestCreateRefundRequestReturnsExistingDuplicateRequest(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           602,
			OrderNo:      "M602",
			UserID:       7,
			TotalCredits: 180,
			Status:       domain.OrderStatusPaid,
		},
		refund: domain.RefundRequest{
			ID:            7602,
			OrderID:       602,
			OrderNo:       "M602",
			UserID:        7,
			AmountCredits: 180,
			Status:        domain.RefundStatusRequested,
			Reason:        "quality_issue",
			UserNote:      "原申请说明",
		},
	}
	svc := NewService(repo, nil, time.Minute)

	refund, duplicate, err := svc.CreateRefundRequest(context.Background(), CreateRefundRequestCommand{
		OrderID: 602,
		UserID:  7,
		Reason:  "after_sale",
		Note:    "  新的重复申请说明  ",
	})
	if err != nil {
		t.Fatalf("CreateRefundRequest() error = %v", err)
	}
	if !duplicate {
		t.Fatal("CreateRefundRequest() duplicate = false, want true")
	}
	if repo.createRefundRequestCalls != 1 {
		t.Fatalf("CreateRefundRequest() calls = %d, want 1", repo.createRefundRequestCalls)
	}
	if refund.ID != 7602 || refund.UserNote != "原申请说明" {
		t.Fatalf("CreateRefundRequest() refund = %+v, want existing refund", refund)
	}
}

func TestAdminReviewRefundRequestRetriesProcessingRefund(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     601,
			UserID: 7,
		},
		refund: domain.RefundRequest{
			ID:            701,
			OrderID:       601,
			OrderNo:       "M701",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusProcessing,
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	refund, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID:     701,
		Approved:     true,
		OperatorID:   "ops",
		AdminNote:    "重试退款",
		RestoreStock: true,
	})
	if err != nil {
		t.Fatalf("AdminReviewRefundRequest() error = %v", err)
	}
	if refund.Status != domain.RefundStatusApproved {
		t.Fatalf("AdminReviewRefundRequest() status = %q, want approved", refund.Status)
	}
	if repo.startRefundApprovalCalls != 1 {
		t.Fatalf("StartRefundApproval() calls = %d, want 1", repo.startRefundApprovalCalls)
	}
	if repo.completeRefundApprovalCalls != 1 {
		t.Fatalf("CompleteRefundApproval() calls = %d, want 1", repo.completeRefundApprovalCalls)
	}
	if charger.adjustCalls != 1 {
		t.Fatalf("AdjustCredits() calls = %d, want 1", charger.adjustCalls)
	}
	if charger.adjustCommand.UserID != 7 || charger.adjustCommand.Delta != 200 {
		t.Fatalf("AdjustCredits() command = %+v, want user 7 delta 200", charger.adjustCommand)
	}
	if charger.adjustCommand.SourceEventID != "mall.refund:701" {
		t.Fatalf("AdjustCredits() source event = %q, want mall.refund:701", charger.adjustCommand.SourceEventID)
	}
}

func TestAdminReviewRefundRequestRetriesFailedCompletionWithStableCreditSourceEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     602,
			UserID: 7,
		},
		refund: domain.RefundRequest{
			ID:            702,
			OrderID:       602,
			OrderNo:       "M702",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusRequested,
		},
		completeRefundApprovalFailures: 1,
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)
	command := AdminReviewRefundRequestCommand{
		RefundID:     702,
		Approved:     true,
		OperatorID:   "ops",
		AdminNote:    "退款审批",
		RestoreStock: true,
	}

	if _, err := svc.AdminReviewRefundRequest(context.Background(), command); err == nil {
		t.Fatal("first AdminReviewRefundRequest() error = nil, want completion error")
	}
	if repo.refund.Status != domain.RefundStatusProcessing {
		t.Fatalf("refund status after failed completion = %q, want processing", repo.refund.Status)
	}

	refund, err := svc.AdminReviewRefundRequest(context.Background(), command)
	if err != nil {
		t.Fatalf("retry AdminReviewRefundRequest() error = %v", err)
	}
	if refund.Status != domain.RefundStatusApproved {
		t.Fatalf("retry refund status = %q, want approved", refund.Status)
	}
	if repo.completeRefundApprovalCalls != 2 {
		t.Fatalf("CompleteRefundApproval() calls = %d, want 2", repo.completeRefundApprovalCalls)
	}
	if charger.adjustCalls != 2 {
		t.Fatalf("AdjustCredits() calls = %d, want 2", charger.adjustCalls)
	}
	for i, adjust := range charger.adjustCommands {
		if adjust.SourceEventID != "mall.refund:702" {
			t.Fatalf("AdjustCredits() call %d source event = %q, want mall.refund:702", i+1, adjust.SourceEventID)
		}
	}

	if _, err := svc.AdminReviewRefundRequest(context.Background(), command); err != nil {
		t.Fatalf("settled AdminReviewRefundRequest() error = %v", err)
	}
	if repo.completeRefundApprovalCalls != 2 || charger.adjustCalls != 2 {
		t.Fatalf("settled refund retried completion=%d adjustments=%d, want 2 each", repo.completeRefundApprovalCalls, charger.adjustCalls)
	}
}

func TestAdminReviewRefundRequestIncludesRevokedDigitalEntitlementsInEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      603,
			OrderNo: "M703",
			UserID:  7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ProductID: 101,
					SKU:       "BADGE-FOUNDER",
					Title:     "创始会员徽章",
					Quantity:  1,
					Code:      "BBS-ENTITLEMENT",
					GrantType: "badge",
					GrantKey:  "badge-founder",
					Status:    domain.DigitalEntitlementStatusActive,
				},
			},
		},
		refund: domain.RefundRequest{
			ID:            703,
			OrderID:       603,
			OrderNo:       "M703",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusRequested,
		},
	}
	svc := NewService(repo, &creditChargerStub{}, time.Minute)

	if _, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID: 703,
		Approved: true,
	}); err != nil {
		t.Fatalf("AdminReviewRefundRequest() error = %v", err)
	}

	var payload refundReviewedEventPayload
	if err := json.Unmarshal(repo.completeRefundEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal refund event: %v", err)
	}
	if payload.EventType != RefundApprovedEventType || payload.RefundID != 703 || payload.OrderID != 603 {
		t.Fatalf("payload = %+v, want refund approval fields", payload)
	}
	if len(payload.DigitalEntitlements) != 1 {
		t.Fatalf("digital entitlements = %+v, want one entitlement", payload.DigitalEntitlements)
	}
	entitlement := payload.DigitalEntitlements[0]
	if entitlement.Status != domain.DigitalEntitlementStatusRevoked || entitlement.RefundID != 703 {
		t.Fatalf("entitlement state = %+v, want revoked refund 703", entitlement)
	}
	if entitlement.GrantKey != "badge-founder" || entitlement.FulfillmentCode != "BBS-ENTITLEMENT" {
		t.Fatalf("entitlement grant/code = %+v, want grant and fulfillment code", entitlement)
	}
}

func TestAdminReviewRefundRequestOnlyIncludesActiveDigitalEntitlementRevocations(t *testing.T) {
	revokedAt := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	expiredAt := time.Now().Add(-time.Hour)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      605,
			OrderNo: "M705",
			UserID:  7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ProductID: 101,
					SKU:       "BADGE-FOUNDER",
					Title:     "创始徽章",
					Quantity:  1,
					Code:      "BBS-ACTIVE",
					GrantType: "badge",
					GrantKey:  "badge-founder",
					Status:    domain.DigitalEntitlementStatusActive,
				},
				{
					ProductID: 102,
					SKU:       "THEME-PRO",
					Title:     "高级主题包",
					Quantity:  1,
					Code:      "BBS-REVOKED",
					GrantType: "theme",
					GrantKey:  "theme-pro",
					Status:    domain.DigitalEntitlementStatusRevoked,
					RevokedAt: &revokedAt,
				},
				{
					ProductID: 104,
					SKU:       "EXPIRED-BADGE",
					Title:     "已过期徽章",
					Quantity:  1,
					Code:      "BBS-EXPIRED",
					GrantType: "badge",
					GrantKey:  "badge-expired",
					Status:    domain.DigitalEntitlementStatusActive,
					ExpiresAt: &expiredAt,
				},
				{
					ProductID: 103,
					SKU:       "DIRTY",
					Title:     "历史脏数据",
					Quantity:  1,
					Code:      "BBS-DIRTY",
					GrantType: "digital",
					GrantKey:  "legacy",
				},
			},
		},
		refund: domain.RefundRequest{
			ID:            705,
			OrderID:       605,
			OrderNo:       "M705",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusRequested,
		},
	}
	svc := NewService(repo, &creditChargerStub{}, time.Minute)

	if _, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID: 705,
		Approved: true,
	}); err != nil {
		t.Fatalf("AdminReviewRefundRequest() error = %v", err)
	}

	var payload refundReviewedEventPayload
	if err := json.Unmarshal(repo.completeRefundEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal refund event: %v", err)
	}
	if len(payload.DigitalEntitlements) != 1 {
		t.Fatalf("digital entitlements = %+v, want only active entitlement", payload.DigitalEntitlements)
	}
	entitlement := payload.DigitalEntitlements[0]
	if entitlement.FulfillmentCode != "BBS-ACTIVE" || entitlement.Status != domain.DigitalEntitlementStatusRevoked || entitlement.RefundID != 705 {
		t.Fatalf("entitlement = %+v, want active entitlement revoked by refund 705", entitlement)
	}
}

func TestAdminReviewRefundRequestIncludesRejectAdminNoteInEvent(t *testing.T) {
	repo := &orderRepoStub{
		refund: domain.RefundRequest{
			ID:            704,
			OrderID:       604,
			OrderNo:       "M704",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusRequested,
			Reason:        "quality_issue",
		},
	}
	svc := NewService(repo, &creditChargerStub{}, time.Minute)

	if _, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID:   704,
		Approved:   false,
		OperatorID: "ops",
		AdminNote:  "凭证不足，暂不支持退款",
	}); err != nil {
		t.Fatalf("AdminReviewRefundRequest() error = %v", err)
	}

	var payload refundReviewedEventPayload
	if err := json.Unmarshal(repo.rejectRefundEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal refund reject event: %v", err)
	}
	if payload.EventType != RefundRejectedEventType || payload.RefundID != 704 || payload.Status != string(domain.RefundStatusRejected) {
		t.Fatalf("payload = %+v, want refund rejection fields", payload)
	}
	if payload.AdminNote != "凭证不足，暂不支持退款" {
		t.Fatalf("payload admin note = %q, want reject note", payload.AdminNote)
	}
	if payload.Reason != "quality_issue" {
		t.Fatalf("payload reason = %q, want quality_issue", payload.Reason)
	}
}

func TestAdminReviewRefundRequestRejectsProcessingRefundRejection(t *testing.T) {
	repo := &orderRepoStub{
		refund: domain.RefundRequest{
			ID:            801,
			OrderID:       601,
			OrderNo:       "M801",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusProcessing,
		},
	}
	svc := NewService(repo, &creditChargerStub{}, time.Minute)

	_, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID: 801,
		Approved: false,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminReviewRefundRequest() error = %v, want invalid order state", err)
	}
	if repo.rejectRefundRequestCalls != 0 {
		t.Fatalf("RejectRefundRequest() calls = %d, want 0", repo.rejectRefundRequestCalls)
	}
}

func TestAdminReviewRefundRequestCompletesZeroCreditRefundWithoutCharger(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           804,
			OrderNo:      "M804",
			UserID:       7,
			TotalCredits: 0,
			Status:       domain.OrderStatusPaid,
		},
		refund: domain.RefundRequest{
			ID:            804,
			OrderID:       804,
			OrderNo:       "M804",
			UserID:        7,
			AmountCredits: 0,
			Status:        domain.RefundStatusRequested,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	refund, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID: 804,
		Approved: true,
	})
	if err != nil {
		t.Fatalf("AdminReviewRefundRequest() error = %v", err)
	}
	if refund.Status != domain.RefundStatusApproved {
		t.Fatalf("refund status = %q, want approved", refund.Status)
	}
	if repo.startRefundApprovalCalls != 1 || repo.completeRefundApprovalCalls != 1 {
		t.Fatalf("refund calls start=%d complete=%d, want 1/1", repo.startRefundApprovalCalls, repo.completeRefundApprovalCalls)
	}
}

func TestPayOrderClosesExpiredOrderWithinPaymentTransaction(t *testing.T) {
	now := time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           810,
			OrderNo:      "M810",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPendingPayment,
			CreatedAt:    now.Add(-time.Minute),
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)
	svc.SetClockForTest(func() time.Time { return now })

	closed, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        810,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-810",
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("PayOrder() error = %v, want invalid order state", err)
	}
	if closed.Status != domain.OrderStatusClosed {
		t.Fatalf("PayOrder() order status = %q, want closed", closed.Status)
	}
	if repo.order.Status != domain.OrderStatusClosed {
		t.Fatalf("order status = %q, want closed", repo.order.Status)
	}
	if repo.beginOrderPaymentCalls != 1 || !repo.beginOrderPaymentExpireBefore.Equal(now.Add(-time.Minute)) {
		t.Fatalf("BeginOrderPayment() calls/expireBefore = %d/%s, want 1/%s", repo.beginOrderPaymentCalls, repo.beginOrderPaymentExpireBefore, now.Add(-time.Minute))
	}
	if repo.closeExpiredOrderCalls != 0 {
		t.Fatalf("CloseExpiredOrder() calls = %d, want 0 because payment closes atomically", repo.closeExpiredOrderCalls)
	}
	if charger.debitCalls != 0 || repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("payment side effects debit=%d completion=%d, want 0/0", charger.debitCalls, repo.completeOrderPaymentCalls)
	}
}

func TestPayOrderRetriesFailedCompletionWithStableCreditSourceEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           811,
			OrderNo:      "M811",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPendingPayment,
		},
		completeOrderPaymentFailures: 1,
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)
	command := PayOrderCommand{
		OrderID:        811,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-811",
	}

	if _, err := svc.PayOrder(context.Background(), command); err == nil {
		t.Fatal("first PayOrder() error = nil, want completion error")
	}
	if repo.order.Status != domain.OrderStatusPaying {
		t.Fatalf("order status after failed completion = %q, want paying", repo.order.Status)
	}
	if repo.payment.Status != domain.PaymentStatusPending {
		t.Fatalf("payment status after failed completion = %q, want pending", repo.payment.Status)
	}

	order, err := svc.PayOrder(context.Background(), command)
	if err != nil {
		t.Fatalf("retry PayOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusPaid {
		t.Fatalf("retry order status = %q, want paid", order.Status)
	}
	if repo.beginOrderPaymentCalls != 2 {
		t.Fatalf("BeginOrderPayment() calls = %d, want 2", repo.beginOrderPaymentCalls)
	}
	if repo.completeOrderPaymentCalls != 2 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 2", repo.completeOrderPaymentCalls)
	}
	if charger.debitCalls != 2 {
		t.Fatalf("DebitCredits() calls = %d, want 2", charger.debitCalls)
	}
	for i, debit := range charger.debitCommands {
		if debit.SourceEventID != "mall.order.pay:811:pay-811" {
			t.Fatalf("DebitCredits() call %d source event = %q, want mall.order.pay:811:pay-811", i+1, debit.SourceEventID)
		}
	}

	if _, err := svc.PayOrder(context.Background(), command); err != nil {
		t.Fatalf("settled PayOrder() error = %v", err)
	}
	if repo.completeOrderPaymentCalls != 2 || charger.debitCalls != 2 {
		t.Fatalf("settled order retried completion=%d debits=%d, want 2 each", repo.completeOrderPaymentCalls, charger.debitCalls)
	}
}

func TestPayOrderMarksInteractiveDebitFailureFailed(t *testing.T) {
	debitErr := domain.ErrInsufficientCredits
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           816,
			OrderNo:      "M816",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPendingPayment,
		},
	}
	charger := &creditChargerStub{debitErr: debitErr}
	svc := NewService(repo, charger, time.Minute)

	_, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        816,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-816",
	})
	if !errors.Is(err, debitErr) {
		t.Fatalf("PayOrder() error = %v, want debit error", err)
	}
	if repo.beginOrderPaymentCalls != 1 {
		t.Fatalf("BeginOrderPayment() calls = %d, want 1", repo.beginOrderPaymentCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
	if repo.failOrderPaymentCalls != 1 {
		t.Fatalf("FailOrderPayment() calls = %d, want 1", repo.failOrderPaymentCalls)
	}
	if repo.order.Status != domain.OrderStatusPendingPayment {
		t.Fatalf("order status after debit failure = %q, want pending payment", repo.order.Status)
	}
	if repo.payment.Status != domain.PaymentStatusFailed {
		t.Fatalf("payment status after debit failure = %q, want failed", repo.payment.Status)
	}
	if repo.failPaymentReason != debitErr.Error() {
		t.Fatalf("payment failure reason = %q, want %q", repo.failPaymentReason, debitErr.Error())
	}
	if charger.debitCommand.SourceEventID != "mall.order.pay:816:pay-816" {
		t.Fatalf("DebitCredits() source event = %q, want mall.order.pay:816:pay-816", charger.debitCommand.SourceEventID)
	}
}

func TestPayOrderKeepsAmbiguousDebitFailurePayingAndResumesOriginalAttempt(t *testing.T) {
	debitErr := errors.New("credit service unavailable")
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           818,
			OrderNo:      "M818",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPendingPayment,
		},
	}
	charger := &creditChargerStub{debitErr: debitErr}
	svc := NewService(repo, charger, time.Minute)
	firstCommand := PayOrderCommand{
		OrderID:        818,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-818-original",
	}

	if _, err := svc.PayOrder(context.Background(), firstCommand); !errors.Is(err, debitErr) {
		t.Fatalf("first PayOrder() error = %v, want debit error", err)
	}
	if repo.order.Status != domain.OrderStatusPaying || repo.payment.Status != domain.PaymentStatusPending {
		t.Fatalf("payment state after ambiguous debit error = order:%q payment:%q, want paying/pending", repo.order.Status, repo.payment.Status)
	}
	if repo.failOrderPaymentCalls != 0 {
		t.Fatalf("FailOrderPayment() calls = %d, want 0", repo.failOrderPaymentCalls)
	}

	charger.debitErr = nil
	paid, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        818,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-818-browser-retry",
	})
	if err != nil {
		t.Fatalf("retry PayOrder() error = %v", err)
	}
	if paid.Status != domain.OrderStatusPaid {
		t.Fatalf("retry PayOrder() status = %q, want paid", paid.Status)
	}
	if repo.failOrderPaymentCalls != 0 || repo.completeOrderPaymentCalls != 1 {
		t.Fatalf("payment calls fail=%d complete=%d, want 0/1", repo.failOrderPaymentCalls, repo.completeOrderPaymentCalls)
	}
	if charger.debitCalls != 2 {
		t.Fatalf("DebitCredits() calls = %d, want 2", charger.debitCalls)
	}
	for i, debit := range charger.debitCommands {
		if debit.SourceEventID != "mall.order.pay:818:pay-818-original" {
			t.Fatalf("DebitCredits() call %d source event = %q, want original payment key", i+1, debit.SourceEventID)
		}
	}
}

func TestPayOrderCompletesZeroCreditOrderWithoutCharger(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           817,
			OrderNo:      "M817",
			UserID:       7,
			TotalCredits: 0,
			Status:       domain.OrderStatusPendingPayment,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        817,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-817",
	})
	if err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusPaid {
		t.Fatalf("order status = %q, want paid", order.Status)
	}
	if repo.beginOrderPaymentCalls != 1 || repo.completeOrderPaymentCalls != 1 || repo.failOrderPaymentCalls != 0 {
		t.Fatalf("payment calls begin=%d complete=%d fail=%d, want 1/1/0", repo.beginOrderPaymentCalls, repo.completeOrderPaymentCalls, repo.failOrderPaymentCalls)
	}
}

func TestPayOrderRejectsPendingMembershipOrderBeforeDebit(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           823,
			OrderNo:      "M823",
			UserID:       7,
			TotalCredits: 300,
			Status:       domain.OrderStatusPendingPayment,
			Items: []domain.OrderItem{
				{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
			},
		},
		beginOrderPaymentErr: domain.ErrPendingMembershipOrderExists,
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	_, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        823,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-823",
	})
	if !errors.Is(err, domain.ErrPendingMembershipOrderExists) {
		t.Fatalf("PayOrder() error = %v, want ErrPendingMembershipOrderExists", err)
	}
	if repo.beginOrderPaymentCalls != 1 {
		t.Fatalf("BeginOrderPayment() calls = %d, want 1", repo.beginOrderPaymentCalls)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
}

func TestPayOrderReturnsCompletedOrderWithoutDuplicateDebit(t *testing.T) {
	completedAt := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           814,
			OrderNo:      "M814",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusCompleted,
			PaidAt:       &completedAt,
			CompletedAt:  &completedAt,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ProductID: 101,
					SKU:       "VIP-MONTH",
					Title:     "会员月卡",
					Quantity:  1,
					Code:      "BBS-EXISTING",
					IssuedAt:  completedAt,
				},
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	order, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        814,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-814",
	})
	if err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("PayOrder() status = %q, want completed", order.Status)
	}
	if len(order.DigitalEntitlements) != 1 || order.DigitalEntitlements[0].Code != "BBS-EXISTING" {
		t.Fatalf("PayOrder() entitlements = %+v, want existing entitlement", order.DigitalEntitlements)
	}
	if repo.beginOrderPaymentCalls != 1 {
		t.Fatalf("BeginOrderPayment() calls = %d, want 1", repo.beginOrderPaymentCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
}

func TestPayOrderRejectsDuplicateActiveThemeBeforeDebit(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           817,
			OrderNo:      "M817",
			UserID:       7,
			TotalCredits: 188,
			Status:       domain.OrderStatusPendingPayment,
			Items: []domain.OrderItem{
				{ProductID: 103, SKU: "theme-pro", Title: "高级主题", Category: "digital", GrantType: "theme", GrantKey: "theme-pro", Quantity: 1},
			},
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "theme", GrantKey: "theme-pro", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	_, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        817,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-817",
	})
	if !errors.Is(err, domain.ErrActiveThemeEntitlementExists) {
		t.Fatalf("PayOrder() error = %v, want ErrActiveThemeEntitlementExists", err)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
	if repo.failOrderPaymentCalls != 1 {
		t.Fatalf("FailOrderPayment() calls = %d, want 1", repo.failOrderPaymentCalls)
	}
	if repo.order.Status != domain.OrderStatusPendingPayment {
		t.Fatalf("order status after duplicate theme = %q, want pending payment", repo.order.Status)
	}
	if repo.failPaymentReason != domain.ErrActiveThemeEntitlementExists.Error() {
		t.Fatalf("payment failure reason = %q, want %q", repo.failPaymentReason, domain.ErrActiveThemeEntitlementExists.Error())
	}
}

func TestPayOrderRejectsDuplicateThemeGrantInOrderBeforeDebit(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           819,
			OrderNo:      "M819",
			UserID:       7,
			TotalCredits: 376,
			Status:       domain.OrderStatusPendingPayment,
			Items: []domain.OrderItem{
				{ProductID: 103, SKU: "theme-pro", Title: "高级主题", Category: "digital", GrantType: "theme", GrantKey: "theme-pro", Quantity: 2},
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	_, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        819,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-819",
	})
	if !errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) {
		t.Fatalf("PayOrder() error = %v, want ErrDuplicateThemeGrantInOrder", err)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
	if repo.failOrderPaymentCalls != 1 {
		t.Fatalf("FailOrderPayment() calls = %d, want 1", repo.failOrderPaymentCalls)
	}
	if repo.failPaymentReason != domain.ErrDuplicateThemeGrantInOrder.Error() {
		t.Fatalf("payment failure reason = %q, want %q", repo.failPaymentReason, domain.ErrDuplicateThemeGrantInOrder.Error())
	}
}

func TestPayOrderRejectsDuplicateActiveBadgeBeforeDebit(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           820,
			OrderNo:      "M820",
			UserID:       7,
			TotalCredits: 80,
			Status:       domain.OrderStatusPendingPayment,
			Items: []domain.OrderItem{
				{ProductID: 105, SKU: "badge-founder", Title: "创始会员徽章", Category: "digital", GrantType: "badge", GrantKey: "badge-founder", Quantity: 1},
			},
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "badge", GrantKey: "badge-founder", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	_, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        820,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-820",
	})
	if !errors.Is(err, domain.ErrActiveBadgeEntitlementExists) {
		t.Fatalf("PayOrder() error = %v, want ErrActiveBadgeEntitlementExists", err)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
	if repo.failOrderPaymentCalls != 1 {
		t.Fatalf("FailOrderPayment() calls = %d, want 1", repo.failOrderPaymentCalls)
	}
	if repo.failPaymentReason != domain.ErrActiveBadgeEntitlementExists.Error() {
		t.Fatalf("payment failure reason = %q, want %q", repo.failPaymentReason, domain.ErrActiveBadgeEntitlementExists.Error())
	}
}

func TestPayOrderRejectsDuplicateBadgeGrantInOrderBeforeDebit(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           821,
			OrderNo:      "M821",
			UserID:       7,
			TotalCredits: 160,
			Status:       domain.OrderStatusPendingPayment,
			Items: []domain.OrderItem{
				{ProductID: 105, SKU: "badge-founder", Title: "创始会员徽章", Category: "digital", GrantType: "badge", GrantKey: "badge-founder", Quantity: 2},
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	_, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        821,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-821",
	})
	if !errors.Is(err, domain.ErrDuplicateBadgeGrantInOrder) {
		t.Fatalf("PayOrder() error = %v, want ErrDuplicateBadgeGrantInOrder", err)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
	if repo.failOrderPaymentCalls != 1 {
		t.Fatalf("FailOrderPayment() calls = %d, want 1", repo.failOrderPaymentCalls)
	}
	if repo.failPaymentReason != domain.ErrDuplicateBadgeGrantInOrder.Error() {
		t.Fatalf("payment failure reason = %q, want %q", repo.failPaymentReason, domain.ErrDuplicateBadgeGrantInOrder.Error())
	}
}

func TestPayOrderIssuesDigitalEntitlementsForMixedOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           815,
			OrderNo:      "M815",
			UserID:       7,
			TotalCredits: 180,
			Status:       domain.OrderStatusPendingPayment,
			Items: []domain.OrderItem{
				{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
				{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	order, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        815,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-815",
	})
	if err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusPaid {
		t.Fatalf("PayOrder() status = %q, want paid for mixed order", order.Status)
	}
	if len(order.DigitalEntitlements) != 1 {
		t.Fatalf("PayOrder() digital entitlements = %+v, want 1 membership grant", order.DigitalEntitlements)
	}
	if order.DigitalEntitlements[0].GrantType != "membership" || order.DigitalEntitlements[0].GrantKey != "vip-month" {
		t.Fatalf("PayOrder() entitlement = %+v, want membership/vip-month", order.DigitalEntitlements[0])
	}
	if order.DigitalEntitlements[0].Status != domain.DigitalEntitlementStatusActive {
		t.Fatalf("PayOrder() entitlement status = %q, want active", order.DigitalEntitlements[0].Status)
	}
	if order.DigitalEntitlements[0].ExpiresAt == nil || order.DigitalEntitlements[0].ExpiresAt.Before(time.Now()) {
		t.Fatalf("PayOrder() entitlement expires_at = %v, want future timestamp", order.DigitalEntitlements[0].ExpiresAt)
	}
	if repo.completeOrderPaymentCalls != 1 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 1", repo.completeOrderPaymentCalls)
	}
	if charger.debitCalls != 1 {
		t.Fatalf("DebitCredits() calls = %d, want 1", charger.debitCalls)
	}
}

func TestListDigitalEntitlementsReturnsUserGrants(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	repo := &orderRepoStub{
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{ID: 501, OrderID: 9001, ProductID: 101, GrantType: "membership", GrantKey: "vip-month", Status: domain.DigitalEntitlementStatusActive, ExpiresAt: &expiresAt},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)

	items, total, err := service.ListDigitalEntitlements(context.Background(), ListDigitalEntitlementsCommand{
		UserID:    7,
		Status:    domain.DigitalEntitlementStatusActive,
		GrantType: "membership",
		GrantKey:  "vip-month",
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListDigitalEntitlements() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("ListDigitalEntitlements() = total %d items %d, want 1 item", total, len(items))
	}
	if items[0].GrantType != "membership" || items[0].GrantKey != "vip-month" {
		t.Fatalf("grant = (%q, %q), want (membership, vip-month)", items[0].GrantType, items[0].GrantKey)
	}
	if repo.listDigitalEntitlementsQuery.GrantType != "membership" || repo.listDigitalEntitlementsQuery.GrantKey != "vip-month" {
		t.Fatalf("query grant = (%q, %q), want (membership, vip-month)", repo.listDigitalEntitlementsQuery.GrantType, repo.listDigitalEntitlementsQuery.GrantKey)
	}
}

func TestListDigitalEntitlementsHidesDirtyMembershipWithoutExpiry(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{ID: 501, OrderID: 9001, ProductID: 101, GrantType: "membership", GrantKey: "vip-month", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)

	items, total, err := service.ListDigitalEntitlements(context.Background(), ListDigitalEntitlementsCommand{
		UserID:    7,
		Status:    domain.DigitalEntitlementStatusActive,
		GrantType: "membership",
		GrantKey:  "vip-month",
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListDigitalEntitlements() error = %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("ListDigitalEntitlements() = total %d items %d, want no dirty membership grant", total, len(items))
	}
}

func TestAdminListDigitalEntitlementsAllowsAllUsersAndKeyword(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			DigitalEntitlements: []domain.DigitalEntitlement{
				{ID: 502, OrderID: 9002, UserID: 43, ProductID: 102, GrantType: "theme", GrantKey: "theme-pro", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)

	items, total, err := service.AdminListDigitalEntitlements(context.Background(), ListDigitalEntitlementsCommand{
		Status:    domain.DigitalEntitlementStatusActive,
		GrantType: "theme",
		Keyword:   "theme-pro",
		Limit:     20,
	})
	if err != nil {
		t.Fatalf("AdminListDigitalEntitlements() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("AdminListDigitalEntitlements() = total %d items %d, want 1 item", total, len(items))
	}
	if repo.listDigitalEntitlementsQuery.UserID != 0 {
		t.Fatalf("query user id = %d, want all users", repo.listDigitalEntitlementsQuery.UserID)
	}
	if repo.listDigitalEntitlementsQuery.Keyword != "theme-pro" {
		t.Fatalf("query keyword = %q, want theme-pro", repo.listDigitalEntitlementsQuery.Keyword)
	}
}

func TestAdminRevokeDigitalEntitlementEmitsOutboxEvent(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 30, 0, 0, time.UTC)
	expiresAt := now.Add(30 * 24 * time.Hour)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      9006,
			OrderNo: "MO9006",
			UserID:  43,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ID:        503,
					OrderID:   9006,
					OrderNo:   "MO9006",
					UserID:    43,
					ProductID: 102,
					SKU:       "VIP-MONTH",
					Title:     "会员月卡",
					Code:      "BBS-VIP-503",
					GrantType: "membership",
					GrantKey:  "vip-month",
					Status:    domain.DigitalEntitlementStatusActive,
					ExpiresAt: &expiresAt,
				},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)
	service.now = func() time.Time { return now }

	entitlement, err := service.AdminRevokeDigitalEntitlement(context.Background(), AdminRevokeDigitalEntitlementCommand{
		ID:         503,
		OperatorID: "42",
		Reason:     "risk review",
	})
	if err != nil {
		t.Fatalf("AdminRevokeDigitalEntitlement() error = %v", err)
	}
	if entitlement.Status != domain.DigitalEntitlementStatusRevoked || entitlement.RevokedBy != "42" || entitlement.RevokeReason != "risk review" {
		t.Fatalf("revoked entitlement = %+v", entitlement)
	}
	if repo.adminRevokeEvent.EventType != EntitlementRevokedEventType {
		t.Fatalf("event type = %q, want %q", repo.adminRevokeEvent.EventType, EntitlementRevokedEventType)
	}
	if repo.adminRevokeEvent.AggregateType != "mall_digital_entitlement" || repo.adminRevokeEvent.AggregateID != 503 || repo.adminRevokeEvent.MessageKey != "43" {
		t.Fatalf("event metadata = %+v", repo.adminRevokeEvent)
	}
	var payload digitalEntitlementRevokedEventPayload
	if err := json.Unmarshal(repo.adminRevokeEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if payload.EntitlementID != 503 || payload.OrderID != 9006 || payload.UserID != 43 || payload.FulfillmentCode != "BBS-VIP-503" || payload.GrantType != "membership" || payload.GrantKey != "vip-month" || payload.OperatorID != "42" || payload.Reason != "risk review" {
		t.Fatalf("payload = %+v, want revoked entitlement details", payload)
	}
}

func TestAdminRevokeDigitalEntitlementSkipsAlreadyRevokedGrant(t *testing.T) {
	revokedAt := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      9007,
			OrderNo: "MO9007",
			UserID:  43,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ID:           504,
					OrderID:      9007,
					OrderNo:      "MO9007",
					UserID:       43,
					ProductID:    101,
					SKU:          "VIP-MONTH",
					Title:        "会员月卡",
					Code:         "BBS-VIP-504",
					GrantType:    "membership",
					GrantKey:     "vip-month",
					Status:       domain.DigitalEntitlementStatusRevoked,
					RevokedAt:    &revokedAt,
					RevokedBy:    "admin-1",
					RevokeReason: "first review",
				},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)

	entitlement, err := service.AdminRevokeDigitalEntitlement(context.Background(), AdminRevokeDigitalEntitlementCommand{
		ID:         504,
		OperatorID: "admin-7",
		Reason:     "duplicate review",
	})
	if err != nil {
		t.Fatalf("AdminRevokeDigitalEntitlement() error = %v", err)
	}
	if entitlement.RevokedBy != "admin-1" || entitlement.RevokeReason != "first review" {
		t.Fatalf("revoked metadata = %q/%q, want original metadata", entitlement.RevokedBy, entitlement.RevokeReason)
	}
	if repo.adminRevokeDigitalEntitlementCalls != 0 {
		t.Fatalf("AdminRevokeDigitalEntitlement repo calls = %d, want 0", repo.adminRevokeDigitalEntitlementCalls)
	}
	if len(repo.adminRevokeEvent.Payload) != 0 {
		t.Fatal("AdminRevokeDigitalEntitlement emitted duplicate outbox event")
	}
}

func TestAdminRevokeDigitalEntitlementSkipsGrantWithRevokedAt(t *testing.T) {
	revokedAt := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      9008,
			OrderNo: "MO9008",
			UserID:  43,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ID:           505,
					OrderID:      9008,
					OrderNo:      "MO9008",
					UserID:       43,
					ProductID:    101,
					SKU:          "VIP-MONTH",
					Title:        "会员月卡",
					Code:         "BBS-VIP-505",
					GrantType:    "membership",
					GrantKey:     "vip-month",
					Status:       domain.DigitalEntitlementStatusActive,
					RevokedAt:    &revokedAt,
					RevokedBy:    "admin-1",
					RevokeReason: "first review",
				},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)

	entitlement, err := service.AdminRevokeDigitalEntitlement(context.Background(), AdminRevokeDigitalEntitlementCommand{
		ID:         505,
		OperatorID: "admin-7",
		Reason:     "duplicate review",
	})
	if err != nil {
		t.Fatalf("AdminRevokeDigitalEntitlement() error = %v", err)
	}
	if entitlement.Status != domain.DigitalEntitlementStatusRevoked || entitlement.RevokedBy != "admin-1" || entitlement.RevokeReason != "first review" {
		t.Fatalf("revoked metadata = status:%q by:%q reason:%q, want original revoked grant", entitlement.Status, entitlement.RevokedBy, entitlement.RevokeReason)
	}
	if repo.adminRevokeDigitalEntitlementCalls != 0 {
		t.Fatalf("AdminRevokeDigitalEntitlement repo calls = %d, want 0", repo.adminRevokeDigitalEntitlementCalls)
	}
	if len(repo.adminRevokeEvent.Payload) != 0 {
		t.Fatal("AdminRevokeDigitalEntitlement emitted duplicate outbox event")
	}
}

func TestAdminRevokeDigitalEntitlementRejectsExpiredGrant(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 30, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      9009,
			OrderNo: "MO9009",
			UserID:  43,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ID:        506,
					OrderID:   9009,
					OrderNo:   "MO9009",
					UserID:    43,
					ProductID: 101,
					SKU:       "VIP-MONTH",
					Title:     "会员月卡",
					Code:      "BBS-VIP-506",
					GrantType: "membership",
					GrantKey:  "vip-month",
					Status:    domain.DigitalEntitlementStatusActive,
					ExpiresAt: &expiredAt,
				},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)
	service.now = func() time.Time { return now }

	_, err := service.AdminRevokeDigitalEntitlement(context.Background(), AdminRevokeDigitalEntitlementCommand{
		ID:         506,
		OperatorID: "admin-7",
		Reason:     "risk review",
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminRevokeDigitalEntitlement() error = %v, want invalid order state", err)
	}
	if repo.adminRevokeDigitalEntitlementCalls != 0 {
		t.Fatalf("AdminRevokeDigitalEntitlement repo calls = %d, want 0", repo.adminRevokeDigitalEntitlementCalls)
	}
	if len(repo.adminRevokeEvent.Payload) != 0 {
		t.Fatal("AdminRevokeDigitalEntitlement emitted expired entitlement revoke event")
	}
}

func TestNewOrderPaidEventUsesUserMessageKeyAndPayload(t *testing.T) {
	paidAt := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)
	event, err := newOrderPaidEvent(domain.Order{
		ID:           811,
		OrderNo:      "M811",
		UserID:       7,
		TotalCredits: 120,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1, UnitPriceCredits: 120, SubtotalCredits: 120},
		},
	}, domain.Payment{
		ID:       9101,
		Provider: domain.PaymentProviderCredits,
	}, paidAt)
	if err != nil {
		t.Fatalf("newOrderPaidEvent() error = %v", err)
	}
	if event.EventType != OrderPaidEventType {
		t.Fatalf("event type = %q, want %q", event.EventType, OrderPaidEventType)
	}
	if event.MessageKey != "7" {
		t.Fatalf("message key = %q, want user id", event.MessageKey)
	}
	var payload orderPaidEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OrderID != 811 || payload.UserID != 7 || payload.TotalCredits != 120 || payload.PaymentID != 9101 {
		t.Fatalf("payload = %+v, want paid order fields", payload)
	}
	if payload.PaymentMethod != domain.PaymentProviderCredits {
		t.Fatalf("payment method = %q, want credits", payload.PaymentMethod)
	}
	if len(payload.Items) != 1 || payload.Items[0].ProductID != 101 || payload.Items[0].Quantity != 1 {
		t.Fatalf("payload items = %+v, want one paid item", payload.Items)
	}
	if payload.Items[0].Category != "digital" {
		t.Fatalf("payload item category = %q, want digital", payload.Items[0].Category)
	}
}

func TestRecoverStalePayingOrdersRetriesWithStableCreditSourceEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           812,
			OrderNo:      "M812",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaying,
		},
		payment: domain.Payment{
			ID:             9102,
			OrderID:        812,
			UserID:         7,
			AmountCredits:  120,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-812",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{
			{
				OrderID:        812,
				UserID:         7,
				PaymentID:      9102,
				Provider:       domain.PaymentProviderCredits,
				IdempotencyKey: "pay-812",
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{
		StaleAfter: time.Minute,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 1 || result.Failed != 0 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=1 failed=0", result)
	}
	if repo.order.Status != domain.OrderStatusPaid {
		t.Fatalf("order status = %q, want paid", repo.order.Status)
	}
	if repo.beginOrderPaymentCalls != 1 || repo.completeOrderPaymentCalls != 1 {
		t.Fatalf("payment calls begin=%d complete=%d, want 1 each", repo.beginOrderPaymentCalls, repo.completeOrderPaymentCalls)
	}
	if repo.failOrderPaymentCalls != 0 {
		t.Fatalf("FailOrderPayment() calls = %d, want 0", repo.failOrderPaymentCalls)
	}
	if charger.debitCalls != 1 {
		t.Fatalf("DebitCredits() calls = %d, want 1", charger.debitCalls)
	}
	if charger.debitCommand.SourceEventID != "mall.order.pay:812:pay-812" {
		t.Fatalf("DebitCredits() source event = %q, want mall.order.pay:812:pay-812", charger.debitCommand.SourceEventID)
	}
}

func TestRecoverStalePayingOrdersKeepsPayingWhenDebitFails(t *testing.T) {
	debitErr := errors.New("credit service unavailable")
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           813,
			OrderNo:      "M813",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaying,
		},
		payment: domain.Payment{
			ID:             9103,
			OrderID:        813,
			UserID:         7,
			AmountCredits:  120,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-813",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{
			{
				OrderID:        813,
				UserID:         7,
				PaymentID:      9103,
				Provider:       domain.PaymentProviderCredits,
				IdempotencyKey: "pay-813",
			},
		},
	}
	charger := &creditChargerStub{debitErr: debitErr}
	svc := NewService(repo, charger, time.Minute)

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{
		StaleAfter: time.Minute,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 0 || result.Failed != 1 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=0 failed=1", result)
	}
	if repo.order.Status != domain.OrderStatusPaying {
		t.Fatalf("order status = %q, want paying", repo.order.Status)
	}
	if repo.payment.Status != domain.PaymentStatusPending {
		t.Fatalf("payment status = %q, want pending", repo.payment.Status)
	}
	if repo.failOrderPaymentCalls != 0 {
		t.Fatalf("FailOrderPayment() calls = %d, want 0", repo.failOrderPaymentCalls)
	}
}

func TestRecoverStalePayingOrdersReopensOrderWhenCreditsAreInsufficient(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           814,
			OrderNo:      "M814",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaying,
		},
		payment: domain.Payment{
			ID:             9105,
			OrderID:        814,
			UserID:         7,
			AmountCredits:  120,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-814",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{{
			OrderID:        814,
			UserID:         7,
			PaymentID:      9105,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-814",
		}},
	}
	svc := NewService(repo, &creditChargerStub{debitErr: domain.ErrInsufficientCredits}, time.Minute)

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{StaleAfter: time.Minute, Limit: 10})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 0 || result.Failed != 1 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=0 failed=1", result)
	}
	if repo.order.Status != domain.OrderStatusPendingPayment || repo.payment.Status != domain.PaymentStatusFailed {
		t.Fatalf("payment recovery state = order:%q payment:%q, want pending payment/failed", repo.order.Status, repo.payment.Status)
	}
	if repo.failOrderPaymentCalls != 1 || repo.failPaymentReason != domain.ErrInsufficientCredits.Error() {
		t.Fatalf("FailOrderPayment() calls/reason = %d/%q, want 1/%q", repo.failOrderPaymentCalls, repo.failPaymentReason, domain.ErrInsufficientCredits.Error())
	}
}

func TestRecoverStalePayingOrdersClosesExpiredOrderWhenCreditsAreInsufficient(t *testing.T) {
	now := time.Date(2026, time.July, 18, 14, 0, 0, 0, time.UTC)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           815,
			OrderNo:      "M815",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaying,
		},
		payment: domain.Payment{
			ID:             9106,
			OrderID:        815,
			UserID:         7,
			AmountCredits:  120,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-815",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{{
			OrderID:        815,
			UserID:         7,
			PaymentID:      9106,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-815",
		}},
		closeExpiredOrderClosesOnCall: 1,
	}
	svc := NewService(repo, &creditChargerStub{debitErr: domain.ErrInsufficientCredits}, time.Minute)
	svc.SetClockForTest(func() time.Time { return now })

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{StaleAfter: time.Minute, Limit: 10})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 0 || result.Failed != 1 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=0 failed=1", result)
	}
	if repo.order.Status != domain.OrderStatusClosed || repo.payment.Status != domain.PaymentStatusFailed {
		t.Fatalf("payment recovery state = order:%q payment:%q, want closed/failed", repo.order.Status, repo.payment.Status)
	}
	if repo.closeExpiredOrderCalls != 1 || !repo.closeExpiredOrderExpireBefore.Equal(now.Add(-time.Minute)) {
		t.Fatalf("CloseExpiredOrder() calls/expireBefore = %d/%s, want 1/%s", repo.closeExpiredOrderCalls, repo.closeExpiredOrderExpireBefore, now.Add(-time.Minute))
	}
}

func TestRecoverStalePayingOrdersMarksDuplicateActiveThemeFailed(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           818,
			OrderNo:      "M818",
			UserID:       7,
			TotalCredits: 188,
			Status:       domain.OrderStatusPaying,
			Items: []domain.OrderItem{
				{ProductID: 103, SKU: "theme-pro", Title: "高级主题", Category: "digital", GrantType: "theme", GrantKey: "theme-pro", Quantity: 1},
			},
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "theme", GrantKey: "theme-pro", Status: domain.DigitalEntitlementStatusActive},
			},
		},
		payment: domain.Payment{
			ID:             9104,
			OrderID:        818,
			UserID:         7,
			AmountCredits:  188,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-818",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{
			{
				OrderID:        818,
				UserID:         7,
				PaymentID:      9104,
				Provider:       domain.PaymentProviderCredits,
				IdempotencyKey: "pay-818",
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{
		StaleAfter: time.Minute,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 0 || result.Failed != 1 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=0 failed=1", result)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
	if repo.failOrderPaymentCalls != 1 {
		t.Fatalf("FailOrderPayment() calls = %d, want 1", repo.failOrderPaymentCalls)
	}
	if repo.order.Status != domain.OrderStatusPendingPayment {
		t.Fatalf("order status = %q, want pending payment", repo.order.Status)
	}
	if repo.payment.Status != domain.PaymentStatusFailed {
		t.Fatalf("payment status = %q, want failed", repo.payment.Status)
	}
	if repo.failPaymentReason != domain.ErrActiveThemeEntitlementExists.Error() {
		t.Fatalf("payment failure reason = %q, want %q", repo.failPaymentReason, domain.ErrActiveThemeEntitlementExists.Error())
	}
}

func TestRecoverStalePayingOrdersMarksDuplicateActiveBadgeFailed(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           822,
			OrderNo:      "M822",
			UserID:       7,
			TotalCredits: 80,
			Status:       domain.OrderStatusPaying,
			Items: []domain.OrderItem{
				{ProductID: 105, SKU: "badge-founder", Title: "创始会员徽章", Category: "digital", GrantType: "badge", GrantKey: "badge-founder", Quantity: 1},
			},
			DigitalEntitlements: []domain.DigitalEntitlement{
				{UserID: 7, GrantType: "badge", GrantKey: "badge-founder", Status: domain.DigitalEntitlementStatusActive},
			},
		},
		payment: domain.Payment{
			ID:             9105,
			OrderID:        822,
			UserID:         7,
			AmountCredits:  80,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-822",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{
			{
				OrderID:        822,
				UserID:         7,
				PaymentID:      9105,
				Provider:       domain.PaymentProviderCredits,
				IdempotencyKey: "pay-822",
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{
		StaleAfter: time.Minute,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 0 || result.Failed != 1 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=0 failed=1", result)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
	if repo.failOrderPaymentCalls != 1 {
		t.Fatalf("FailOrderPayment() calls = %d, want 1", repo.failOrderPaymentCalls)
	}
	if repo.order.Status != domain.OrderStatusPendingPayment {
		t.Fatalf("order status = %q, want pending payment", repo.order.Status)
	}
	if repo.payment.Status != domain.PaymentStatusFailed {
		t.Fatalf("payment status = %q, want failed", repo.payment.Status)
	}
	if repo.failPaymentReason != domain.ErrActiveBadgeEntitlementExists.Error() {
		t.Fatalf("payment failure reason = %q, want %q", repo.failPaymentReason, domain.ErrActiveBadgeEntitlementExists.Error())
	}
}

func TestAdminRequeueOutboxEventsNormalizesStatusesAndLimit(t *testing.T) {
	repo := &orderRepoStub{requeueOutboxResult: domain.OutboxRequeueResult{Requeued: 4, EventIDs: []string{"evt-1", "evt-2"}}}
	svc := NewService(repo, nil, time.Minute)
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	svc.SetClockForTest(func() time.Time { return now })

	result, err := svc.AdminRequeueOutboxEvents(context.Background(), AdminRequeueOutboxEventsCommand{
		Statuses:   []string{" failed ", "DEAD_LETTER", "failed"},
		Limit:      domain.MaxListLimit + 1,
		OperatorID: " 42 ",
	})
	if err != nil {
		t.Fatalf("AdminRequeueOutboxEvents() error = %v", err)
	}
	if result.Requeued != 4 {
		t.Fatalf("AdminRequeueOutboxEvents() requeued = %d, want 4", result.Requeued)
	}
	if repo.requeueOutboxCalls != 1 {
		t.Fatalf("RequeueOutboxEvents() calls = %d, want 1", repo.requeueOutboxCalls)
	}
	if len(repo.requeueOutboxStatuses) != 2 || repo.requeueOutboxStatuses[0] != "failed" || repo.requeueOutboxStatuses[1] != "dead_letter" {
		t.Fatalf("RequeueOutboxEvents() statuses = %v, want [failed dead_letter]", repo.requeueOutboxStatuses)
	}
	if repo.requeueOutboxLimit != domain.MaxListLimit {
		t.Fatalf("RequeueOutboxEvents() limit = %d, want %d", repo.requeueOutboxLimit, domain.MaxListLimit)
	}
	if !repo.requeueOutboxAt.Equal(now) {
		t.Fatalf("RequeueOutboxEvents() at = %s, want %s", repo.requeueOutboxAt, now)
	}
	if repo.requeueOutboxOperatorID != "42" {
		t.Fatalf("RequeueOutboxEvents() operator = %q, want 42", repo.requeueOutboxOperatorID)
	}
	if len(result.EventIDs) != 2 || result.EventIDs[0] != "evt-1" {
		t.Fatalf("AdminRequeueOutboxEvents() event IDs = %v, want [evt-1 evt-2]", result.EventIDs)
	}
}

func TestAdminRequeueOutboxEventsDefaultsStatusesAndLimit(t *testing.T) {
	repo := &orderRepoStub{requeueOutboxResult: domain.OutboxRequeueResult{Requeued: 2}}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.AdminRequeueOutboxEvents(context.Background(), AdminRequeueOutboxEventsCommand{})
	if err != nil {
		t.Fatalf("AdminRequeueOutboxEvents() error = %v", err)
	}
	if result.Requeued != 2 {
		t.Fatalf("AdminRequeueOutboxEvents() requeued = %d, want 2", result.Requeued)
	}
	if repo.requeueOutboxCalls != 1 {
		t.Fatalf("RequeueOutboxEvents() calls = %d, want 1", repo.requeueOutboxCalls)
	}
	if len(repo.requeueOutboxStatuses) != 2 || repo.requeueOutboxStatuses[0] != "failed" || repo.requeueOutboxStatuses[1] != "dead_letter" {
		t.Fatalf("RequeueOutboxEvents() statuses = %v, want [failed dead_letter]", repo.requeueOutboxStatuses)
	}
	if repo.requeueOutboxLimit != DefaultOutboxRequeueLimit {
		t.Fatalf("RequeueOutboxEvents() limit = %d, want %d", repo.requeueOutboxLimit, DefaultOutboxRequeueLimit)
	}
	if repo.requeueOutboxOperatorID != "admin" {
		t.Fatalf("RequeueOutboxEvents() operator = %q, want admin", repo.requeueOutboxOperatorID)
	}
}

func TestAdminRequeueOutboxEventsRejectsUnsupportedStatus(t *testing.T) {
	repo := &orderRepoStub{}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.AdminRequeueOutboxEvents(context.Background(), AdminRequeueOutboxEventsCommand{
		Statuses: []string{"published"},
	})
	if !errors.Is(err, domain.ErrInvalidOutboxStatus) {
		t.Fatalf("AdminRequeueOutboxEvents() error = %v, want invalid outbox status", err)
	}
	if repo.requeueOutboxCalls != 0 {
		t.Fatalf("RequeueOutboxEvents() calls = %d, want 0", repo.requeueOutboxCalls)
	}
}

func TestAdminListOutboxRequeueAuditsNormalizesQuery(t *testing.T) {
	requeuedAt := time.Date(2026, 7, 13, 15, 30, 0, 0, time.UTC)
	repo := &orderRepoStub{
		listOutboxRequeueAuditsItems: []domain.OutboxRequeueAudit{
			{ID: 7, EventID: "evt-7", AggregateType: "order", AggregateID: 9001, PreviousStatus: "dead_letter", PreviousAttempts: 5, PreviousError: "publisher down", OperatorID: "42", RequeuedAt: requeuedAt},
		},
		listOutboxRequeueAuditsTotal: 1,
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.AdminListOutboxRequeueAudits(context.Background(), AdminListOutboxRequeueAuditsCommand{
		Limit:         domain.MaxListLimit + 10,
		Offset:        -5,
		EventID:       " evt-7 ",
		AggregateType: " order ",
		AggregateID:   9001,
	})
	if err != nil {
		t.Fatalf("AdminListOutboxRequeueAudits() error = %v", err)
	}
	if len(result.Items) != 1 || result.Total != 1 {
		t.Fatalf("AdminListOutboxRequeueAudits() = %+v, want one audit", result)
	}
	if repo.listOutboxRequeueAuditsCalls != 1 {
		t.Fatalf("AdminListOutboxRequeueAudits() repository calls = %d, want 1", repo.listOutboxRequeueAuditsCalls)
	}
	query := repo.listOutboxRequeueAuditsQuery
	if query.Limit != domain.MaxListLimit || query.Offset != 0 {
		t.Fatalf("query page = limit %d offset %d, want %d/0", query.Limit, query.Offset, domain.MaxListLimit)
	}
	if query.EventID != "evt-7" || query.AggregateType != "order" || query.AggregateID != 9001 {
		t.Fatalf("query filters = %+v, want evt-7/order/9001", query)
	}
}

func TestAdminUpdateOrderStatusRequiresTrackingForShippedPhysicalOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: physicalPaidOrder(901),
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID: 901,
		Status:  domain.OrderStatusShipped,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminUpdateOrderStatus() error = %v, want invalid order state", err)
	}
	if repo.adminUpdateOrderStatusCalls != 0 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 0", repo.adminUpdateOrderStatusCalls)
	}
}

func TestAdminUpdateOrderStatusRequiresTrackingForPhysicalOrderItem(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      905,
			OrderNo: "M905",
			UserID:  7,
			Status:  domain.OrderStatusPaid,
			Items: []domain.OrderItem{
				{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID: 905,
		Status:  domain.OrderStatusShipped,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminUpdateOrderStatus() error = %v, want invalid order state", err)
	}
	if repo.adminUpdateOrderStatusCalls != 0 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 0", repo.adminUpdateOrderStatusCalls)
	}
}

func TestAdminUpdateOrderStatusAllowsShippedPhysicalOrderWithTracking(t *testing.T) {
	repo := &orderRepoStub{
		order: physicalPaidOrder(902),
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID:         902,
		Status:          domain.OrderStatusShipped,
		OperatorID:      "ops",
		ShippingCarrier: " 顺丰 ",
		TrackingNo:      " SF123 ",
		Note:            " 已发货 ",
	})
	if err != nil {
		t.Fatalf("AdminUpdateOrderStatus() error = %v", err)
	}
	if order.Status != domain.OrderStatusShipped {
		t.Fatalf("AdminUpdateOrderStatus() status = %q, want shipped", order.Status)
	}
	if repo.adminUpdateOrderStatusCalls != 1 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 1", repo.adminUpdateOrderStatusCalls)
	}
	if repo.adminFulfillment.ShippingCarrier != "顺丰" || repo.adminFulfillment.TrackingNo != "SF123" {
		t.Fatalf("AdminUpdateOrderStatus() fulfillment = %+v, want trimmed carrier/tracking", repo.adminFulfillment)
	}
	if repo.adminNote != "已发货" {
		t.Fatalf("AdminUpdateOrderStatus() note = %q, want trimmed note", repo.adminNote)
	}
}

func TestAdminUpdateOrderStatusRequiresEvidenceForCompletingPhysicalOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: physicalPaidOrder(903),
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID: 903,
		Status:  domain.OrderStatusCompleted,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminUpdateOrderStatus() error = %v, want invalid order state", err)
	}
	if repo.adminUpdateOrderStatusCalls != 0 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 0", repo.adminUpdateOrderStatusCalls)
	}
}

func TestAdminUpdateOrderStatusAllowsCompletingDigitalOrderWithoutFulfillment(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      904,
			OrderNo: "M904",
			UserID:  7,
			Status:  domain.OrderStatusPaid,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID: 904,
		Status:  domain.OrderStatusCompleted,
	})
	if err != nil {
		t.Fatalf("AdminUpdateOrderStatus() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("AdminUpdateOrderStatus() status = %q, want completed", order.Status)
	}
	if repo.adminUpdateOrderStatusCalls != 1 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 1", repo.adminUpdateOrderStatusCalls)
	}
}

func TestConfirmOrderCompletesShippedOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:              1001,
			OrderNo:         "M1001",
			UserID:          7,
			Status:          domain.OrderStatusShipped,
			ShippingCarrier: "顺丰",
			TrackingNo:      "SF1001",
		},
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.ConfirmOrder(context.Background(), ConfirmOrderCommand{
		OrderID: 1001,
		UserID:  7,
	})
	if err != nil {
		t.Fatalf("ConfirmOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("ConfirmOrder() status = %q, want completed", order.Status)
	}
	if repo.confirmOrderCalls != 1 {
		t.Fatalf("ConfirmOrder() calls = %d, want 1", repo.confirmOrderCalls)
	}
	if repo.confirmEvent.EventType != OrderCompletedEventType {
		t.Fatalf("ConfirmOrder() event type = %q, want %q", repo.confirmEvent.EventType, OrderCompletedEventType)
	}
}

func TestConfirmOrderRejectsUnshippedOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     1002,
			UserID: 7,
			Status: domain.OrderStatusPaid,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.ConfirmOrder(context.Background(), ConfirmOrderCommand{
		OrderID: 1002,
		UserID:  7,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("ConfirmOrder() error = %v, want invalid order state", err)
	}
	if repo.confirmOrderCalls != 0 {
		t.Fatalf("ConfirmOrder() calls = %d, want 0", repo.confirmOrderCalls)
	}
}

func TestConfirmOrderReturnsCompletedOrderWithoutDuplicateWrite(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     1003,
			UserID: 7,
			Status: domain.OrderStatusCompleted,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.ConfirmOrder(context.Background(), ConfirmOrderCommand{
		OrderID: 1003,
		UserID:  7,
	})
	if err != nil {
		t.Fatalf("ConfirmOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("ConfirmOrder() status = %q, want completed", order.Status)
	}
	if repo.confirmOrderCalls != 0 {
		t.Fatalf("ConfirmOrder() calls = %d, want 0", repo.confirmOrderCalls)
	}
}

func TestGetUserOrderRejectsOtherUserOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{ID: 1101, UserID: 7},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.GetUserOrder(context.Background(), 1101, 8)
	if !errors.Is(err, domain.ErrOrderOwnerMismatch) {
		t.Fatalf("GetUserOrder() error = %v, want owner mismatch", err)
	}
}

func TestListUserOrderStatusLogsRequiresOrderOwner(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{ID: 1102, UserID: 7},
		orderStatusLogs: []domain.OrderStatusLog{
			{ID: 501, OrderID: 1102, ToStatus: domain.OrderStatusPaid},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	items, err := svc.ListUserOrderStatusLogs(context.Background(), 1102, 7)
	if err != nil {
		t.Fatalf("ListUserOrderStatusLogs() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 501 {
		t.Fatalf("ListUserOrderStatusLogs() = %+v, want one log", items)
	}

	if _, err := svc.ListUserOrderStatusLogs(context.Background(), 1102, 8); !errors.Is(err, domain.ErrOrderOwnerMismatch) {
		t.Fatalf("ListUserOrderStatusLogs() mismatch error = %v, want owner mismatch", err)
	}
	if repo.listOrderStatusLogsCalls != 1 {
		t.Fatalf("ListOrderStatusLogs() calls = %d, want only owner call", repo.listOrderStatusLogsCalls)
	}
}

func TestListUserOrderPaymentsRequiresOrderOwner(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{ID: 1103, UserID: 7},
		orderPayments: []domain.Payment{
			{ID: 601, OrderID: 1103, UserID: 7, AmountCredits: 120},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	items, err := svc.ListUserOrderPayments(context.Background(), 1103, 7)
	if err != nil {
		t.Fatalf("ListUserOrderPayments() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 601 {
		t.Fatalf("ListUserOrderPayments() = %+v, want one payment", items)
	}

	if _, err := svc.ListUserOrderPayments(context.Background(), 1103, 8); !errors.Is(err, domain.ErrOrderOwnerMismatch) {
		t.Fatalf("ListUserOrderPayments() mismatch error = %v, want owner mismatch", err)
	}
	if repo.listOrderPaymentsCalls != 1 {
		t.Fatalf("ListOrderPayments() calls = %d, want only owner call", repo.listOrderPaymentsCalls)
	}
}

func TestCreateProductReviewStartsPendingReview(t *testing.T) {
	repo := &orderRepoStub{products: map[int64]domain.Product{
		101: {ID: 101, Status: domain.ProductStatusActive},
	}}
	svc := NewService(repo, nil, time.Minute)

	review, err := svc.CreateProductReview(context.Background(), CreateProductReviewCommand{
		UserID:    7,
		ProductID: 101,
		OrderID:   9001,
		Rating:    5,
		Content:   "  很好用，晒单图片见正文  ",
	})
	if err != nil {
		t.Fatalf("CreateProductReview() error = %v", err)
	}
	if repo.createProductReviewCalls != 1 {
		t.Fatalf("CreateProductReview() calls = %d, want 1", repo.createProductReviewCalls)
	}
	if review.Status != domain.ProductReviewStatusPending {
		t.Fatalf("CreateProductReview() status = %q, want pending", review.Status)
	}
	if review.Content != "很好用，晒单图片见正文" {
		t.Fatalf("CreateProductReview() content = %q, want trimmed content", review.Content)
	}
}

func TestAdminUpdateProductReviewStatusEmitsOutboxEvent(t *testing.T) {
	repo := &orderRepoStub{
		productReview: domain.ProductReview{
			ID:           9004,
			ProductID:    101,
			ProductTitle: "主题皮肤",
			OrderID:      9001,
			UserID:       7,
			Status:       domain.ProductReviewStatusPending,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	review, err := svc.AdminUpdateProductReviewStatus(context.Background(), AdminUpdateProductReviewStatusCommand{
		ID:     9004,
		Status: domain.ProductReviewStatusPublished,
	})
	if err != nil {
		t.Fatalf("AdminUpdateProductReviewStatus() error = %v", err)
	}
	if review.Status != domain.ProductReviewStatusPublished {
		t.Fatalf("review status = %q, want published", review.Status)
	}
	if repo.adminUpdateProductReviewStatusCalls != 1 {
		t.Fatalf("AdminUpdateProductReviewStatus() calls = %d, want 1", repo.adminUpdateProductReviewStatusCalls)
	}
	if repo.productReviewEvent.EventType != ReviewPublishedEventType {
		t.Fatalf("event type = %q, want %q", repo.productReviewEvent.EventType, ReviewPublishedEventType)
	}
	var payload productReviewStatusEventPayload
	if err := json.Unmarshal(repo.productReviewEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if payload.ReviewID != 9004 || payload.ProductID != 101 || payload.UserID != 7 || payload.Status != string(domain.ProductReviewStatusPublished) {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestAdminUpdateProductReviewStatusSkipsEventForUnchangedStatus(t *testing.T) {
	repo := &orderRepoStub{
		productReview: domain.ProductReview{
			ID:        9005,
			ProductID: 102,
			OrderID:   9002,
			UserID:    8,
			Status:    domain.ProductReviewStatusPublished,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	if _, err := svc.AdminUpdateProductReviewStatus(context.Background(), AdminUpdateProductReviewStatusCommand{
		ID:     9005,
		Status: domain.ProductReviewStatusPublished,
	}); err != nil {
		t.Fatalf("AdminUpdateProductReviewStatus() error = %v", err)
	}
	if repo.productReviewEvent.EventID != "" {
		t.Fatalf("event = %+v, want empty", repo.productReviewEvent)
	}
}

func physicalPaidOrder(id int64) domain.Order {
	return domain.Order{
		ID:       id,
		OrderNo:  "M-PHYSICAL",
		UserID:   7,
		Status:   domain.OrderStatusPaid,
		Receiver: "张三",
		Phone:    "13800000000",
		Address:  "上海市浦东新区测试路 1 号",
	}
}

func buildStubDigitalEntitlements(order domain.Order, issuedAt time.Time) []domain.DigitalEntitlement {
	entitlements := make([]domain.DigitalEntitlement, 0)
	for _, item := range order.Items {
		grantType := strings.ToLower(strings.TrimSpace(item.GrantType))
		grantKey := strings.ToLower(strings.TrimSpace(item.GrantKey))
		if !strings.EqualFold(strings.TrimSpace(item.Category), "digital") && grantType == "" && grantKey == "" {
			continue
		}
		if grantType == "" {
			grantType = "digital"
		}
		if grantKey == "" {
			grantKey = strings.ToLower(strings.TrimSpace(item.SKU))
		}
		for unit := int32(0); unit < item.Quantity; unit++ {
			entitlement := domain.DigitalEntitlement{
				OrderID:   order.ID,
				OrderNo:   order.OrderNo,
				UserID:    order.UserID,
				ProductID: item.ProductID,
				SKU:       item.SKU,
				Title:     item.Title,
				Quantity:  1,
				Code:      fmt.Sprintf("BBS-TEST-%d-%d", order.ID, len(entitlements)+1),
				GrantType: grantType,
				GrantKey:  grantKey,
				IssuedAt:  issuedAt,
				Status:    domain.DigitalEntitlementStatusActive,
			}
			if entitlement.GrantType == "membership" {
				expiresAt := issuedAt.Add(30 * 24 * time.Hour)
				entitlement.ExpiresAt = &expiresAt
			}
			entitlements = append(entitlements, entitlement)
		}
	}
	return entitlements
}

type orderRepoStub struct {
	domain.Repository

	products                            map[int64]domain.Product
	idempotencyOrders                   map[string]domain.Order
	cartItems                           []domain.CartItem
	listCartItemsCalls                  int
	order                               domain.Order
	orderStatusLogs                     []domain.OrderStatusLog
	orderPayments                       []domain.Payment
	refund                              domain.RefundRequest
	productReview                       domain.ProductReview
	createOrderCalls                    int
	createOrderFromCartCalls            int
	createRefundRequestCalls            int
	adminUpdateOrderStatusCalls         int
	adminUpdateProductReviewStatusCalls int
	listOrderStatusLogsCalls            int
	listOrderPaymentsCalls              int
	adminFulfillment                    domain.OrderFulfillment
	adminNote                           string
	confirmOrderCalls                   int
	confirmEvent                        domain.OutboxEvent
	createProductReviewCalls            int
	productReviewEvent                  domain.OutboxEvent
	startRefundApprovalCalls            int
	completeRefundApprovalCalls         int
	completeRefundApprovalFailures      int
	completeRefundEvent                 domain.OutboxEvent
	rejectRefundRequestCalls            int
	rejectRefundEvent                   domain.OutboxEvent
	payment                             domain.Payment
	stalePayingOrders                   []domain.PayingOrderPayment
	listStalePayingOrdersCalls          int
	stalePayingStartedBefore            time.Time
	stalePayingLimit                    int
	beginOrderPaymentCalls              int
	beginOrderPaymentErr                error
	beginOrderPaymentExpireBefore       time.Time
	completeOrderPaymentCalls           int
	completeOrderPaymentFailures        int
	failOrderPaymentCalls               int
	failPaymentReason                   string
	closeExpiredOrderCalls              int
	closeExpiredOrderClosesOnCall       int
	closeExpiredOrderExpireBefore       time.Time
	requeueOutboxCalls                  int
	requeueOutboxStatuses               []string
	requeueOutboxLimit                  int
	requeueOutboxAt                     time.Time
	requeueOutboxOperatorID             string
	requeueOutboxResult                 domain.OutboxRequeueResult
	listOutboxRequeueAuditsCalls        int
	listOutboxRequeueAuditsQuery        domain.OutboxRequeueAuditListQuery
	listOutboxRequeueAuditsItems        []domain.OutboxRequeueAudit
	listOutboxRequeueAuditsTotal        int64
	listDigitalEntitlementsQuery        domain.DigitalEntitlementListQuery
	listDigitalEntitlementsCalls        int
	setCartItemCalls                    int
	setCartItemUserID                   int64
	setCartItemProductID                int64
	setCartItemQuantity                 int32
	openDigitalGrantOrderExists         bool
	openDigitalGrantOrderExistsCalls    int
	openDigitalGrantOrderUserID         int64
	openDigitalGrantOrderGrantType      string
	openDigitalGrantOrderGrantKey       string
	adminRevokeEvent                    domain.OutboxEvent
	adminRevokeDigitalEntitlementCalls  int
}

func (r *orderRepoStub) GetOrderByIdempotencyKey(_ context.Context, userID int64, idempotencyKey string) (domain.Order, error) {
	if order, ok := r.idempotencyOrders[fmt.Sprintf("%d:%s", userID, idempotencyKey)]; ok {
		return order, nil
	}
	return domain.Order{}, domain.ErrOrderNotFound
}

func (r *orderRepoStub) OpenDigitalGrantOrderExists(_ context.Context, userID int64, grantType, grantKey string) (bool, error) {
	r.openDigitalGrantOrderExistsCalls++
	r.openDigitalGrantOrderUserID = userID
	r.openDigitalGrantOrderGrantType = grantType
	r.openDigitalGrantOrderGrantKey = grantKey
	return r.openDigitalGrantOrderExists, nil
}

func (r *orderRepoStub) GetProduct(_ context.Context, productID int64) (domain.Product, error) {
	if product, ok := r.products[productID]; ok {
		return product, nil
	}
	return domain.Product{}, domain.ErrProductNotFound
}

func (r *orderRepoStub) SetCartItem(_ context.Context, userID int64, productID int64, quantity int32, _ time.Time) error {
	r.setCartItemCalls++
	r.setCartItemUserID = userID
	r.setCartItemProductID = productID
	r.setCartItemQuantity = quantity
	return nil
}

func (r *orderRepoStub) CreateOrder(_ context.Context, order domain.Order) (domain.Order, bool, error) {
	r.createOrderCalls++
	order.ID = 9001
	return order, false, nil
}

func (r *orderRepoStub) ListCartItems(_ context.Context, _ int64) ([]domain.CartItem, int64, error) {
	r.listCartItemsCalls++
	return r.cartItems, int64(len(r.cartItems)), nil
}

func (r *orderRepoStub) CreateOrderFromCart(_ context.Context, order domain.Order) (domain.Order, bool, error) {
	r.createOrderFromCartCalls++
	order.ID = 9002
	return order, false, nil
}

func (r *orderRepoStub) GetOrder(_ context.Context, orderID int64) (domain.Order, error) {
	if r.order.ID == orderID {
		return r.order, nil
	}
	return domain.Order{}, domain.ErrOrderNotFound
}

func (r *orderRepoStub) ListOrderStatusLogs(_ context.Context, _ int64) ([]domain.OrderStatusLog, error) {
	r.listOrderStatusLogsCalls++
	return r.orderStatusLogs, nil
}

func (r *orderRepoStub) ListOrderPayments(_ context.Context, _ int64) ([]domain.Payment, error) {
	r.listOrderPaymentsCalls++
	return r.orderPayments, nil
}

func (r *orderRepoStub) ListDigitalEntitlements(_ context.Context, query domain.DigitalEntitlementListQuery) ([]domain.DigitalEntitlement, int64, error) {
	r.listDigitalEntitlementsCalls++
	r.listDigitalEntitlementsQuery = query
	items := make([]domain.DigitalEntitlement, 0, len(r.order.DigitalEntitlements))
	now := time.Now().UTC()
	for _, entitlement := range r.order.DigitalEntitlements {
		entitlementUserID := entitlement.UserID
		if entitlementUserID == 0 {
			entitlementUserID = r.order.UserID
		}
		if query.UserID > 0 && query.UserID != entitlementUserID {
			continue
		}
		if query.GrantType != "" && !strings.EqualFold(strings.TrimSpace(entitlement.GrantType), strings.TrimSpace(query.GrantType)) {
			continue
		}
		if query.GrantKey != "" && normalizeDigitalGrantKey(entitlement.GrantKey) != normalizeDigitalGrantKey(query.GrantKey) {
			continue
		}
		if query.Status != "" && !stubDigitalEntitlementMatchesStatus(entitlement, query.Status, now) {
			continue
		}
		if query.Keyword != "" && !stubDigitalEntitlementMatchesKeyword(entitlement, query.Keyword) {
			continue
		}
		items = append(items, entitlement)
	}
	total := int64(len(items))
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	if offset >= len(items) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], total, nil
}

func stubDigitalEntitlementMatchesStatus(entitlement domain.DigitalEntitlement, status string, now time.Time) bool {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	itemStatus := strings.ToUpper(strings.TrimSpace(entitlement.Status))
	switch normalized {
	case domain.DigitalEntitlementStatusActive:
		if itemStatus != domain.DigitalEntitlementStatusActive || entitlement.RevokedAt != nil {
			return false
		}
		if strings.EqualFold(strings.TrimSpace(entitlement.GrantType), "membership") {
			return entitlement.ExpiresAt != nil && entitlement.ExpiresAt.After(now)
		}
		return entitlement.ExpiresAt == nil || entitlement.ExpiresAt.After(now)
	case domain.DigitalEntitlementStatusExpired:
		return itemStatus == domain.DigitalEntitlementStatusExpired || (entitlement.RevokedAt == nil && entitlement.ExpiresAt != nil && !entitlement.ExpiresAt.After(now))
	case domain.DigitalEntitlementStatusRevoked:
		return itemStatus == domain.DigitalEntitlementStatusRevoked || entitlement.RevokedAt != nil
	default:
		return itemStatus == normalized
	}
}

func stubDigitalEntitlementMatchesKeyword(entitlement domain.DigitalEntitlement, keyword string) bool {
	normalized := strings.ToLower(strings.TrimSpace(keyword))
	if normalized == "" {
		return true
	}
	values := []string{entitlement.OrderNo, entitlement.SKU, entitlement.Title, entitlement.Code, entitlement.GrantType, entitlement.GrantKey}
	for _, value := range values {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), normalized) {
			return true
		}
	}
	return false
}

func (r *orderRepoStub) GetDigitalEntitlement(_ context.Context, entitlementID int64) (domain.DigitalEntitlement, error) {
	for _, entitlement := range r.order.DigitalEntitlements {
		if entitlement.ID == entitlementID {
			return entitlement, nil
		}
	}
	return domain.DigitalEntitlement{}, domain.ErrDigitalEntitlementNotFound
}

func (r *orderRepoStub) AdminRevokeDigitalEntitlement(_ context.Context, entitlementID int64, operatorID string, reason string, revokedAt time.Time, event domain.OutboxEvent) (domain.DigitalEntitlement, error) {
	r.adminRevokeDigitalEntitlementCalls++
	r.adminRevokeEvent = event
	for i, entitlement := range r.order.DigitalEntitlements {
		if entitlement.ID == entitlementID {
			entitlement.Status = domain.DigitalEntitlementStatusRevoked
			entitlement.RevokedAt = &revokedAt
			entitlement.RevokedBy = operatorID
			entitlement.RevokeReason = reason
			r.order.DigitalEntitlements[i] = entitlement
			return entitlement, nil
		}
	}
	return domain.DigitalEntitlement{}, domain.ErrDigitalEntitlementNotFound
}

func (r *orderRepoStub) CreateRefundRequest(_ context.Context, refund domain.RefundRequest) (domain.RefundRequest, bool, error) {
	r.createRefundRequestCalls++
	if r.refund.OrderID == refund.OrderID {
		return r.refund, true, nil
	}
	refund.ID = 9003
	return refund, false, nil
}

func (r *orderRepoStub) CreateProductReview(_ context.Context, review domain.ProductReview) (domain.ProductReview, error) {
	r.createProductReviewCalls++
	review.ID = 9004
	return review, nil
}

func (r *orderRepoStub) GetProductReview(_ context.Context, reviewID int64) (domain.ProductReview, error) {
	if r.productReview.ID == reviewID {
		return r.productReview, nil
	}
	return domain.ProductReview{}, domain.ErrProductReviewNotFound
}

func (r *orderRepoStub) AdminUpdateProductReviewStatus(_ context.Context, reviewID int64, status domain.ProductReviewStatus, updatedAt time.Time, event domain.OutboxEvent) (domain.ProductReview, error) {
	r.adminUpdateProductReviewStatusCalls++
	if r.productReview.ID != reviewID {
		return domain.ProductReview{}, domain.ErrProductReviewNotFound
	}
	r.productReview.Status = status
	r.productReview.UpdatedAt = updatedAt
	r.productReviewEvent = event
	return r.productReview, nil
}

func (r *orderRepoStub) AdminUpdateOrderStatus(_ context.Context, orderID int64, nextStatus domain.OrderStatus, _ string, fulfillment domain.OrderFulfillment, note string, changedAt time.Time, _ domain.OutboxEvent) (domain.Order, error) {
	r.adminUpdateOrderStatusCalls++
	if r.order.ID != orderID {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	r.adminFulfillment = fulfillment
	r.adminNote = note
	r.order.Status = nextStatus
	if fulfillment.ShippingCarrier != "" {
		r.order.ShippingCarrier = fulfillment.ShippingCarrier
	}
	if fulfillment.TrackingNo != "" {
		r.order.TrackingNo = fulfillment.TrackingNo
	}
	if nextStatus == domain.OrderStatusShipped {
		r.order.ShippedAt = &changedAt
	}
	if nextStatus == domain.OrderStatusCompleted {
		r.order.CompletedAt = &changedAt
	}
	return r.order, nil
}

func (r *orderRepoStub) ConfirmOrder(_ context.Context, orderID, userID int64, completedAt time.Time, event domain.OutboxEvent) (domain.Order, error) {
	r.confirmOrderCalls++
	r.confirmEvent = event
	if r.order.ID != orderID {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	if r.order.Status != domain.OrderStatusShipped {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	r.order.Status = domain.OrderStatusCompleted
	r.order.CompletedAt = &completedAt
	return r.order, nil
}

func (r *orderRepoStub) GetRefundRequest(_ context.Context, refundID int64) (domain.RefundRequest, error) {
	if r.refund.ID == refundID {
		return r.refund, nil
	}
	return domain.RefundRequest{}, domain.ErrRefundNotFound
}

func (r *orderRepoStub) StartRefundApproval(_ context.Context, refundID int64, operatorID, adminNote string, restoreStock bool, reviewedAt time.Time) (domain.RefundRequest, error) {
	r.startRefundApprovalCalls++
	if r.refund.ID != refundID {
		return domain.RefundRequest{}, domain.ErrRefundNotFound
	}
	if r.refund.Status == domain.RefundStatusRequested {
		r.refund.Status = domain.RefundStatusProcessing
		r.refund.OperatorID = operatorID
		r.refund.AdminNote = adminNote
		r.refund.RestoreStock = restoreStock
		r.refund.ReviewedAt = &reviewedAt
	}
	return r.refund, nil
}

func (r *orderRepoStub) CompleteRefundApproval(_ context.Context, refundID int64, reviewedAt time.Time, event domain.OutboxEvent) (domain.RefundRequest, error) {
	r.completeRefundApprovalCalls++
	r.completeRefundEvent = event
	if r.refund.ID != refundID {
		return domain.RefundRequest{}, domain.ErrRefundNotFound
	}
	if r.completeRefundApprovalFailures > 0 {
		r.completeRefundApprovalFailures--
		return domain.RefundRequest{}, errors.New("complete refund approval failed")
	}
	r.refund.Status = domain.RefundStatusApproved
	r.refund.RefundedAt = &reviewedAt
	return r.refund, nil
}

func (r *orderRepoStub) RejectRefundRequest(_ context.Context, refundID int64, operatorID, adminNote string, reviewedAt time.Time, event domain.OutboxEvent) (domain.RefundRequest, error) {
	r.rejectRefundRequestCalls++
	r.rejectRefundEvent = event
	if r.refund.ID != refundID {
		return domain.RefundRequest{}, domain.ErrRefundNotFound
	}
	r.refund.Status = domain.RefundStatusRejected
	r.refund.OperatorID = operatorID
	r.refund.AdminNote = adminNote
	r.refund.ReviewedAt = &reviewedAt
	return r.refund, nil
}

func (r *orderRepoStub) CloseExpiredOrder(_ context.Context, orderID, userID int64, expireBefore time.Time, closedAt time.Time) (domain.Order, bool, error) {
	r.closeExpiredOrderCalls++
	r.closeExpiredOrderExpireBefore = expireBefore
	if r.order.ID != orderID {
		return domain.Order{}, false, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, false, domain.ErrOrderOwnerMismatch
	}
	if r.closeExpiredOrderClosesOnCall > 0 && r.closeExpiredOrderCalls >= r.closeExpiredOrderClosesOnCall {
		r.order.Status = domain.OrderStatusClosed
		r.order.UpdatedAt = closedAt
		return r.order, true, nil
	}
	return r.order, false, nil
}

func (r *orderRepoStub) ListStalePayingOrders(_ context.Context, startedBefore time.Time, limit int) ([]domain.PayingOrderPayment, error) {
	r.listStalePayingOrdersCalls++
	r.stalePayingStartedBefore = startedBefore
	r.stalePayingLimit = limit
	return r.stalePayingOrders, nil
}

func (r *orderRepoStub) RequeueOutboxEvents(_ context.Context, statuses []string, limit int, operatorID string, requeuedAt time.Time) (domain.OutboxRequeueResult, error) {
	r.requeueOutboxCalls++
	r.requeueOutboxStatuses = append([]string(nil), statuses...)
	r.requeueOutboxLimit = limit
	r.requeueOutboxOperatorID = operatorID
	r.requeueOutboxAt = requeuedAt
	return r.requeueOutboxResult, nil
}

func (r *orderRepoStub) AdminListOutboxRequeueAudits(_ context.Context, query domain.OutboxRequeueAuditListQuery) ([]domain.OutboxRequeueAudit, int64, error) {
	r.listOutboxRequeueAuditsCalls++
	r.listOutboxRequeueAuditsQuery = query
	return r.listOutboxRequeueAuditsItems, r.listOutboxRequeueAuditsTotal, nil
}

func (r *orderRepoStub) BeginOrderPayment(_ context.Context, orderID, userID int64, paymentMethod, idempotencyKey string, expireBefore, startedAt time.Time) (domain.Order, domain.Payment, error) {
	r.beginOrderPaymentCalls++
	r.beginOrderPaymentExpireBefore = expireBefore
	if r.beginOrderPaymentErr != nil {
		return domain.Order{}, domain.Payment{}, r.beginOrderPaymentErr
	}
	if r.order.ID != orderID {
		return domain.Order{}, domain.Payment{}, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, domain.Payment{}, domain.ErrOrderOwnerMismatch
	}
	if r.order.Status == domain.OrderStatusPaid || r.order.Status == domain.OrderStatusCompleted {
		return r.order, domain.Payment{}, nil
	}
	if r.order.Status == domain.OrderStatusPendingPayment && !r.order.CreatedAt.IsZero() && !r.order.CreatedAt.After(expireBefore) {
		r.order.Status = domain.OrderStatusClosed
		r.order.UpdatedAt = startedAt
		return r.order, domain.Payment{}, domain.ErrInvalidOrderState
	}
	if r.order.Status != domain.OrderStatusPendingPayment && r.order.Status != domain.OrderStatusPaying {
		return domain.Order{}, domain.Payment{}, domain.ErrInvalidOrderState
	}
	if r.payment.ID == 0 {
		r.payment = domain.Payment{
			ID:             9101,
			OrderID:        orderID,
			UserID:         userID,
			AmountCredits:  r.order.TotalCredits,
			Provider:       paymentMethod,
			IdempotencyKey: idempotencyKey,
			Status:         domain.PaymentStatusPending,
			CreatedAt:      startedAt,
			UpdatedAt:      startedAt,
		}
	}
	r.order.Status = domain.OrderStatusPaying
	r.order.PaymentMethod = paymentMethod
	r.order.UpdatedAt = startedAt
	return r.order, r.payment, nil
}

func (r *orderRepoStub) CompleteOrderPayment(_ context.Context, orderID, userID, paymentID int64, paidAt time.Time, _ domain.OutboxEvent) (domain.Order, error) {
	r.completeOrderPaymentCalls++
	if r.order.ID != orderID {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	if r.payment.ID != paymentID {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	if r.completeOrderPaymentFailures > 0 {
		r.completeOrderPaymentFailures--
		return domain.Order{}, errors.New("complete order payment failed")
	}
	r.order.Status = domain.OrderStatusPaid
	r.order.PaidAt = &paidAt
	r.order.UpdatedAt = paidAt
	r.payment.Status = domain.PaymentStatusSucceeded
	r.payment.PaidAt = &paidAt
	r.payment.UpdatedAt = paidAt
	if len(r.order.DigitalEntitlements) == 0 {
		r.order.DigitalEntitlements = buildStubDigitalEntitlements(r.order, paidAt)
	}
	return r.order, nil
}

func (r *orderRepoStub) FailOrderPayment(_ context.Context, orderID, userID, paymentID int64, reason string, failedAt time.Time) error {
	r.failOrderPaymentCalls++
	if r.order.ID != orderID {
		return domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.ErrOrderOwnerMismatch
	}
	if r.payment.ID != paymentID {
		return domain.ErrInvalidOrderState
	}
	r.failPaymentReason = reason
	r.order.Status = domain.OrderStatusPendingPayment
	r.order.UpdatedAt = failedAt
	r.payment.Status = domain.PaymentStatusFailed
	r.payment.FailureReason = reason
	r.payment.UpdatedAt = failedAt
	return nil
}

type creditChargerStub struct {
	adjustCalls    int
	adjustCommand  CreditAdjustCommand
	adjustCommands []CreditAdjustCommand
	debitCalls     int
	debitCommand   CreditDebitCommand
	debitCommands  []CreditDebitCommand
	debitErr       error
}

func (c *creditChargerStub) DebitCredits(_ context.Context, command CreditDebitCommand) error {
	c.debitCalls++
	c.debitCommand = command
	c.debitCommands = append(c.debitCommands, command)
	return c.debitErr
}

func (c *creditChargerStub) AdjustCredits(_ context.Context, command CreditAdjustCommand) error {
	c.adjustCalls++
	c.adjustCommand = command
	c.adjustCommands = append(c.adjustCommands, command)
	return nil
}
