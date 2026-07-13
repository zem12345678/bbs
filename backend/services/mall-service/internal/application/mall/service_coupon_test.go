package mall

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestAdminCreateCouponNormalizesDefaultsAndTimestamps(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	startsAt := now.Add(time.Hour)
	endsAt := now.Add(48 * time.Hour)
	repo := &couponRepoStub{}
	svc := NewService(repo, nil, time.Minute)
	svc.SetClockForTest(func() time.Time { return now })

	coupon, err := svc.AdminCreateCoupon(context.Background(), SaveCouponCommand{
		Code:            " spring10 ",
		Name:            " 新人券 ",
		Description:     "  下单立减  ",
		DiscountCredits: 10,
		MinOrderCredits: 100,
		TotalQuota:      1000,
		PerUserLimit:    1,
		StartsAt:        &startsAt,
		EndsAt:          &endsAt,
	})
	if err != nil {
		t.Fatalf("AdminCreateCoupon() error = %v", err)
	}
	if repo.createCouponCalls != 1 {
		t.Fatalf("AdminCreateCoupon() calls = %d, want 1", repo.createCouponCalls)
	}

	saved := repo.createdCoupon
	if saved.Code != "SPRING10" {
		t.Fatalf("coupon code = %q, want normalized SPRING10", saved.Code)
	}
	if saved.Name != "新人券" || saved.Description != "下单立减" {
		t.Fatalf("coupon text = %q/%q, want trimmed values", saved.Name, saved.Description)
	}
	if saved.Status != domain.CouponStatusDraft {
		t.Fatalf("coupon status = %q, want draft default", saved.Status)
	}
	if !saved.CreatedAt.Equal(now) || !saved.UpdatedAt.Equal(now) {
		t.Fatalf("coupon timestamps = %s/%s, want %s", saved.CreatedAt, saved.UpdatedAt, now)
	}
	if saved.StartsAt == nil || !saved.StartsAt.Equal(startsAt) || saved.EndsAt == nil || !saved.EndsAt.Equal(endsAt) {
		t.Fatalf("coupon window = %v/%v, want %s/%s", saved.StartsAt, saved.EndsAt, startsAt, endsAt)
	}
	if coupon.ID != 8001 || coupon.Code != "SPRING10" {
		t.Fatalf("returned coupon = %+v, want persisted normalized coupon", coupon)
	}
}

func TestAdminCreateCouponValidatesCouponFields(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	badEnd := now

	tests := []struct {
		name    string
		command SaveCouponCommand
		wantErr string
	}{
		{
			name:    "blank code",
			command: SaveCouponCommand{Name: "新人券", DiscountCredits: 10},
			wantErr: "coupon code is required",
		},
		{
			name:    "embedded whitespace in code",
			command: SaveCouponCommand{Code: "SUM MER", Name: "新人券", DiscountCredits: 10},
			wantErr: "coupon code must not contain whitespace",
		},
		{
			name:    "non-positive discount",
			command: SaveCouponCommand{Code: "SUMMER", Name: "新人券"},
			wantErr: "discount_credits must be positive",
		},
		{
			name: "invalid time window",
			command: SaveCouponCommand{
				Code:            "SUMMER",
				Name:            "新人券",
				DiscountCredits: 10,
				StartsAt:        &now,
				EndsAt:          &badEnd,
			},
			wantErr: "coupon end time must be after start time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &couponRepoStub{}
			svc := NewService(repo, nil, time.Minute)

			_, err := svc.AdminCreateCoupon(context.Background(), tt.command)
			if err == nil {
				t.Fatal("AdminCreateCoupon() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("AdminCreateCoupon() error = %q, want %q", err.Error(), tt.wantErr)
			}
			if repo.createCouponCalls != 0 {
				t.Fatalf("AdminCreateCoupon() calls = %d, want 0", repo.createCouponCalls)
			}
		})
	}
}

func TestClaimCouponValidatesAndDelegates(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
	repo := &couponRepoStub{
		claimedUsage: domain.CouponUsage{
			ID:       9001,
			CouponID: 77,
			UserID:   42,
			Status:   domain.CouponUsageStatusClaimed,
		},
		claimDuplicate: true,
	}
	svc := NewService(repo, nil, time.Minute)
	svc.SetClockForTest(func() time.Time { return now })

	usage, duplicate, err := svc.ClaimCoupon(context.Background(), ClaimCouponCommand{
		UserID:   42,
		CouponID: 77,
	})
	if err != nil {
		t.Fatalf("ClaimCoupon() error = %v", err)
	}
	if !duplicate {
		t.Fatal("ClaimCoupon() duplicate = false, want true")
	}
	if usage.ID != 9001 || usage.UserID != 42 || usage.CouponID != 77 {
		t.Fatalf("ClaimCoupon() usage = %+v, want repo usage", usage)
	}
	if repo.claimCalls != 1 || repo.claimUserID != 42 || repo.claimCouponID != 77 || !repo.claimedAt.Equal(now) {
		t.Fatalf("ClaimCoupon() delegated user=%d coupon=%d at=%s calls=%d", repo.claimUserID, repo.claimCouponID, repo.claimedAt, repo.claimCalls)
	}

	for _, cmd := range []ClaimCouponCommand{{CouponID: 77}, {UserID: 42}} {
		if _, _, err := svc.ClaimCoupon(context.Background(), cmd); err == nil {
			t.Fatalf("ClaimCoupon(%+v) error = nil, want validation error", cmd)
		}
	}
	if repo.claimCalls != 1 {
		t.Fatalf("ClaimCoupon() calls after invalid requests = %d, want 1", repo.claimCalls)
	}
}

func TestListUserCouponUsagesNormalizesQuery(t *testing.T) {
	repo := &couponRepoStub{}
	svc := NewService(repo, nil, time.Minute)

	if _, _, err := svc.ListUserCouponUsages(context.Background(), ListUserCouponUsagesCommand{UserID: 0}); err == nil {
		t.Fatal("ListUserCouponUsages() error = nil, want user validation error")
	}

	_, total, err := svc.ListUserCouponUsages(context.Background(), ListUserCouponUsagesCommand{
		UserID: 42,
		Status: domain.CouponUsageStatus("UNKNOWN"),
		Limit:  -1,
		Offset: -5,
	})
	if err != nil {
		t.Fatalf("ListUserCouponUsages() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("ListUserCouponUsages() total = %d, want 2", total)
	}
	if repo.listUserCouponUsagesCalls != 1 {
		t.Fatalf("ListCouponUsagesByUser() calls = %d, want 1", repo.listUserCouponUsagesCalls)
	}
	query := repo.listUserCouponUsagesQuery
	if query.UserID != 42 || query.Status != "" || query.Limit != domain.DefaultListLimit || query.Offset != 0 {
		t.Fatalf("coupon usage query = %+v, want normalized user/status/pagination", query)
	}
}

func TestCreateOrderNormalizesCouponCode(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			101: {
				ID:           101,
				Title:        "数字专栏",
				Category:     "digital",
				PriceCredits: 120,
				Stock:        50,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "coupon-order",
		UserID:         7,
		Items:          []domain.CreateOrderItem{{ProductID: 101, Quantity: 2}},
		CouponCode:     " spring10 ",
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if result.Order.CouponCode != "SPRING10" {
		t.Fatalf("CreateOrder() coupon code = %q, want SPRING10", result.Order.CouponCode)
	}
	if result.Order.OriginalCredits != 240 || result.Order.TotalCredits != 240 {
		t.Fatalf("CreateOrder() totals = original %d total %d, want 240/240 before repository discount", result.Order.OriginalCredits, result.Order.TotalCredits)
	}
}

func TestCheckoutCartNormalizesCouponCode(t *testing.T) {
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
		IdempotencyKey: "coupon-cart",
		UserID:         7,
		CouponCode:     " cart10 ",
	})
	if err != nil {
		t.Fatalf("CheckoutCart() error = %v", err)
	}
	if result.Order.CouponCode != "CART10" {
		t.Fatalf("CheckoutCart() coupon code = %q, want CART10", result.Order.CouponCode)
	}
	if result.Order.OriginalCredits != 120 || result.Order.TotalCredits != 120 {
		t.Fatalf("CheckoutCart() totals = original %d total %d, want 120/120 before repository discount", result.Order.OriginalCredits, result.Order.TotalCredits)
	}
}

type couponRepoStub struct {
	domain.Repository

	createCouponCalls         int
	createdCoupon             domain.Coupon
	claimCalls                int
	claimUserID               int64
	claimCouponID             int64
	claimedAt                 time.Time
	claimedUsage              domain.CouponUsage
	claimDuplicate            bool
	listUserCouponUsagesCalls int
	listUserCouponUsagesQuery domain.CouponUsageListQuery
}

func (r *couponRepoStub) AdminCreateCoupon(_ context.Context, coupon domain.Coupon) (domain.Coupon, error) {
	r.createCouponCalls++
	r.createdCoupon = coupon
	coupon.ID = 8001
	return coupon, nil
}

func (r *couponRepoStub) ClaimCoupon(_ context.Context, userID int64, couponID int64, claimedAt time.Time) (domain.CouponUsage, bool, error) {
	r.claimCalls++
	r.claimUserID = userID
	r.claimCouponID = couponID
	r.claimedAt = claimedAt
	return r.claimedUsage, r.claimDuplicate, nil
}

func (r *couponRepoStub) ListCouponUsagesByUser(_ context.Context, query domain.CouponUsageListQuery) ([]domain.CouponUsage, int64, error) {
	r.listUserCouponUsagesCalls++
	r.listUserCouponUsagesQuery = query
	return []domain.CouponUsage{
		{ID: 1, CouponID: 77, UserID: query.UserID},
		{ID: 2, CouponID: 88, UserID: query.UserID},
	}, 2, nil
}
