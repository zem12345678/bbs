package persistence

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestApplyCouponToOrderRejectsUnavailableCoupons(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	futureStart := now.Add(time.Hour)
	expiredEnd := now.Add(-time.Second)

	tests := []struct {
		name     string
		coupon   couponRow
		order    domain.Order
		totalUse int64
		userUse  int64
	}{
		{
			name:   "inactive",
			coupon: couponRow{status: domain.CouponStatusDraft, discount: 10},
		},
		{
			name:   "future window",
			coupon: couponRow{status: domain.CouponStatusActive, discount: 10, startsAt: &futureStart},
		},
		{
			name:   "expired window",
			coupon: couponRow{status: domain.CouponStatusActive, discount: 10, endsAt: &expiredEnd},
		},
		{
			name:   "below minimum order",
			coupon: couponRow{status: domain.CouponStatusActive, discount: 10, minOrder: 200},
		},
		{
			name:   "non-positive discount",
			coupon: couponRow{status: domain.CouponStatusActive},
		},
		{
			name:     "total quota exhausted",
			coupon:   couponRow{status: domain.CouponStatusActive, discount: 10, totalQuota: 1},
			totalUse: 1,
		},
		{
			name:    "per user quota exhausted",
			coupon:  couponRow{status: domain.CouponStatusActive, discount: 10, perUserLimit: 1},
			userUse: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := tt.order
			if order.UserID == 0 {
				order.UserID = 42
			}
			if order.OriginalCredits == 0 {
				order.OriginalCredits = 100
			}
			order.CouponCode = "SAVE10"
			order.CreatedAt = now

			_, err := applyCouponToOrderInTx(context.Background(), &couponApplyQueryer{
				coupon:    tt.coupon.withDefaults(now),
				totalUses: tt.totalUse,
				userUses:  tt.userUse,
			}, order)
			if !errors.Is(err, domain.ErrCouponUnavailable) {
				t.Fatalf("applyCouponToOrderInTx() error = %v, want coupon unavailable", err)
			}
		})
	}
}

func TestApplyCouponToOrderReservesExistingClaimedUsageAtQuota(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 30, 0, 0, time.UTC)
	order, err := applyCouponToOrderInTx(context.Background(), &couponApplyQueryer{
		coupon: couponRow{
			id:           77,
			code:         "SAVE10",
			status:       domain.CouponStatusActive,
			discount:     25,
			totalQuota:   1,
			perUserLimit: 1,
		}.withDefaults(now),
		claimedUsageID: 9001,
		totalUses:      1,
		userUses:       1,
	}, domain.Order{
		UserID:          42,
		OriginalCredits: 100,
		CouponCode:      " save10 ",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("applyCouponToOrderInTx() error = %v", err)
	}
	if order.CouponID != 77 || order.CouponUsageID != 9001 || order.CouponCode != "SAVE10" {
		t.Fatalf("coupon snapshot = id %d usage %d code %q, want claimed usage applied", order.CouponID, order.CouponUsageID, order.CouponCode)
	}
	if order.DiscountCredits != 25 || order.TotalCredits != 75 {
		t.Fatalf("discounted totals = discount %d total %d, want 25/75", order.DiscountCredits, order.TotalCredits)
	}
}

type couponRow struct {
	id           int64
	code         string
	status       domain.CouponStatus
	discount     int64
	minOrder     int64
	totalQuota   int64
	perUserLimit int64
	startsAt     *time.Time
	endsAt       *time.Time
}

func (r couponRow) withDefaults(now time.Time) couponRow {
	if r.id == 0 {
		r.id = 77
	}
	if r.code == "" {
		r.code = "SAVE10"
	}
	if r.endsAt == nil {
		endsAt := now.Add(time.Hour)
		r.endsAt = &endsAt
	}
	return r
}

type couponApplyQueryer struct {
	coupon         couponRow
	claimedUsageID int64
	totalUses      int64
	userUses       int64
	queryRows      int
}

func (q *couponApplyQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *couponApplyQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *couponApplyQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	q.queryRows++
	switch q.queryRows {
	case 1:
		return couponScanRow{values: q.coupon.values()}
	case 2:
		if q.claimedUsageID <= 0 {
			return couponScanRow{err: pgx.ErrNoRows}
		}
		return couponScanRow{values: []any{q.claimedUsageID}}
	case 3:
		return couponScanRow{values: []any{q.totalUses}}
	case 4:
		return couponScanRow{values: []any{q.userUses}}
	default:
		return couponScanRow{err: errors.New("unexpected coupon query")}
	}
}

func (r couponRow) values() []any {
	var startsAt sql.NullTime
	if r.startsAt != nil {
		startsAt = sql.NullTime{Time: *r.startsAt, Valid: true}
	}
	var endsAt sql.NullTime
	if r.endsAt != nil {
		endsAt = sql.NullTime{Time: *r.endsAt, Valid: true}
	}
	return []any{
		r.id,
		r.code,
		"coupon",
		r.discount,
		r.minOrder,
		r.totalQuota,
		r.perUserLimit,
		string(r.status),
		startsAt,
		endsAt,
	}
}

type couponScanRow struct {
	values []any
	err    error
}

func (r couponScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return testScanner(r.values).Scan(dest...)
}
