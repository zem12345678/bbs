package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

func TestCouponUsageStateUpdateRequiresAffectedRow(t *testing.T) {
	now := time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC)

	for _, tt := range []struct {
		name string
		run  func(context.Context, queryer, int64, time.Time) error
	}{
		{
			name: "mark used",
			run:  markCouponUsageUsed,
		},
		{
			name: "release",
			run:  releaseCouponUsage,
		},
	} {
		t.Run(tt.name+"/missing row", func(t *testing.T) {
			err := tt.run(context.Background(), &couponUsageStateQueryer{tag: pgconn.NewCommandTag("UPDATE 0")}, 501, now)
			if !errors.Is(err, domain.ErrCouponUnavailable) {
				t.Fatalf("%s() error = %v, want coupon unavailable", tt.name, err)
			}
		})
		t.Run(tt.name+"/updated row", func(t *testing.T) {
			err := tt.run(context.Background(), &couponUsageStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")}, 501, now)
			if err != nil {
				t.Fatalf("%s() error = %v, want nil", tt.name, err)
			}
		})
	}
}

func TestInsertCouponUsageRequiresReservedUsageRow(t *testing.T) {
	now := time.Date(2026, 7, 16, 11, 30, 0, 0, time.UTC)
	order := domain.Order{
		ID:              501,
		UserID:          42,
		CouponID:        77,
		CouponCode:      "SAVE10",
		DiscountCredits: 10,
		CreatedAt:       now,
	}

	err := insertCouponUsage(context.Background(), &couponUsageStateQueryer{tag: pgconn.NewCommandTag("INSERT 0 0")}, order)
	if !errors.Is(err, domain.ErrCouponUnavailable) {
		t.Fatalf("insertCouponUsage() error = %v, want coupon unavailable", err)
	}

	err = insertCouponUsage(context.Background(), &couponUsageStateQueryer{tag: pgconn.NewCommandTag("INSERT 0 1")}, order)
	if err != nil {
		t.Fatalf("insertCouponUsage() error = %v, want nil", err)
	}
}

func TestInsertCouponUsageRequiresClaimedUsageUpdate(t *testing.T) {
	now := time.Date(2026, 7, 16, 11, 45, 0, 0, time.UTC)
	order := domain.Order{
		ID:              501,
		UserID:          42,
		CouponID:        77,
		CouponUsageID:   9001,
		CouponCode:      "SAVE10",
		DiscountCredits: 10,
		CreatedAt:       now,
	}

	err := insertCouponUsage(context.Background(), &couponUsageStateQueryer{tag: pgconn.NewCommandTag("UPDATE 0")}, order)
	if !errors.Is(err, domain.ErrCouponUnavailable) {
		t.Fatalf("insertCouponUsage() error = %v, want coupon unavailable", err)
	}

	err = insertCouponUsage(context.Background(), &couponUsageStateQueryer{tag: pgconn.NewCommandTag("UPDATE 1")}, order)
	if err != nil {
		t.Fatalf("insertCouponUsage() error = %v, want nil", err)
	}
}

func TestCouponListKeywordConditionCoversCouponID(t *testing.T) {
	condition := couponListKeywordCondition(2)
	for _, want := range []string{
		"c.id::TEXT = $2",
		"c.code ILIKE '%' || $2 || '%'",
		"c.name ILIKE '%' || $2 || '%'",
		"c.description ILIKE '%' || $2 || '%'",
	} {
		if !strings.Contains(condition, want) {
			t.Fatalf("coupon keyword condition = %q, want %q", condition, want)
		}
	}
}

func TestEnsureCouponTermsMutableSkipsNonTermChanges(t *testing.T) {
	now := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	endsAt := now.Add(24 * time.Hour)
	db := &couponTermLockQueryer{locked: true}

	err := ensureCouponTermsMutable(context.Background(), db,
		domain.Coupon{ID: 77, Code: "SAVE10", Name: "旧名称", Description: "旧说明", DiscountCredits: 10, MinOrderCredits: 100, TotalQuota: 1000, PerUserLimit: 1, Status: domain.CouponStatusActive, EndsAt: &endsAt},
		domain.Coupon{ID: 77, Code: " save10 ", Name: "新名称", Description: "新说明", DiscountCredits: 10, MinOrderCredits: 100, TotalQuota: 1000, PerUserLimit: 1, Status: domain.CouponStatusActive, EndsAt: &endsAt},
	)
	if err != nil {
		t.Fatalf("ensureCouponTermsMutable() error = %v, want nil", err)
	}
	if db.queryRows != 0 {
		t.Fatalf("QueryRow() calls = %d, want 0 for non-term changes", db.queryRows)
	}
}

func TestEnsureCouponTermsMutableAllowsUnusedTermChange(t *testing.T) {
	db := &couponTermLockQueryer{}

	err := ensureCouponTermsMutable(context.Background(), db,
		domain.Coupon{ID: 77, Code: "SAVE10", DiscountCredits: 10, Status: domain.CouponStatusActive},
		domain.Coupon{ID: 77, Code: "SAVE20", DiscountCredits: 20, Status: domain.CouponStatusActive},
	)
	if err != nil {
		t.Fatalf("ensureCouponTermsMutable() error = %v, want nil", err)
	}
	if db.queryRows != 1 {
		t.Fatalf("QueryRow() calls = %d, want 1", db.queryRows)
	}
	query := db.query
	for _, want := range []string{
		"mall_coupon_usages",
		"coupon_id = $1",
		"status <> $2",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("coupon terms lock query = %q, want %q", query, want)
		}
	}
	wantArgs := []any{int64(77), string(domain.CouponUsageStatusReleased)}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("coupon terms lock args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("coupon terms lock arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestEnsureCouponTermsMutableBlocksIssuedTermChange(t *testing.T) {
	db := &couponTermLockQueryer{locked: true}

	err := ensureCouponTermsMutable(context.Background(), db,
		domain.Coupon{ID: 77, Code: "SAVE10", DiscountCredits: 10, Status: domain.CouponStatusActive},
		domain.Coupon{ID: 77, Code: "SAVE10", DiscountCredits: 10, Status: domain.CouponStatusArchived},
	)
	if !errors.Is(err, domain.ErrCouponTermsLocked) {
		t.Fatalf("ensureCouponTermsMutable() error = %v, want coupon terms locked", err)
	}
}

func TestCouponSchemaEnforcesNormalizedUniqueCodes(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"idx_mall_coupons_code_ci",
		"LOWER(TRIM(code))",
		"mall_coupons_code_normalized_check",
		"code = UPPER(TRIM(code))",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing coupon code constraint %q", want)
		}
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

type couponUsageStateQueryer struct {
	tag pgconn.CommandTag
}

func (q *couponUsageStateQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return q.tag, nil
}

func (q *couponUsageStateQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *couponUsageStateQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	return couponScanRow{err: errors.New("unexpected query row")}
}

type couponTermLockQueryer struct {
	locked    bool
	query     string
	args      []any
	queryRows int
}

func (q *couponTermLockQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *couponTermLockQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *couponTermLockQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryRows++
	q.query = query
	q.args = args
	return couponTermLockScanRow{locked: q.locked}
}

type couponTermLockScanRow struct {
	locked bool
}

func (r couponTermLockScanRow) Scan(dest ...any) error {
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
