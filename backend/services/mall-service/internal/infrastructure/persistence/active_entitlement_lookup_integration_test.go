package persistence

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListActiveEntitlementUserIDsPostgresUsesSingleQuery(t *testing.T) {
	ctx, pool, counter := openActiveEntitlementLookupSmokePool(t)
	defer pool.Close()

	stamp := strings.ToLower(uuid.NewString())
	userID := int64(9_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	now := time.Now().UTC()
	_, _ = seedActiveMembershipEntitlement(t, ctx, pool, stamp, userID, now)

	counter.Reset()
	userIDs, err := NewPostgresRepository(pool).ListActiveEntitlementUserIDs(ctx, []int64{userID, userID + 1}, "membership", "")
	if err != nil {
		t.Fatalf("ListActiveEntitlementUserIDs() error = %v", err)
	}
	if len(userIDs) != 1 || userIDs[0] != userID {
		t.Fatalf("ListActiveEntitlementUserIDs() = %v, want [%d]", userIDs, userID)
	}
	if count := counter.Count(); count != 1 {
		t.Fatalf("active entitlement lookup query count = %d, want 1", count)
	}
}

func seedActiveMembershipEntitlement(t *testing.T, ctx context.Context, pool *pgxpool.Pool, stamp string, userID int64, now time.Time) (int64, int64) {
	t.Helper()
	grantKey := "vip-lookup-" + stamp
	sku := strings.ToUpper("LOOKUP-" + stamp)
	var productID, orderID int64
	t.Cleanup(func() {
		if orderID > 0 {
			_, _ = pool.Exec(context.Background(), "DELETE FROM mall_orders WHERE id = $1", orderID)
		}
		if productID > 0 {
			_, _ = pool.Exec(context.Background(), "DELETE FROM mall_products WHERE id = $1", productID)
		}
	})
	if err := pool.QueryRow(ctx, `
		INSERT INTO mall_products (
		  sku, title, description, category, cover_url, grant_type, grant_key,
		  price_credits, stock, status, sort, created_at, updated_at
		) VALUES ($1, 'Lookup membership', '', 'digital', '', 'membership', $2, 0, 1, 'ACTIVE', 0, $3, $3)
		RETURNING id`, sku, grantKey, now).Scan(&productID); err != nil {
		t.Fatalf("insert lookup product: %v", err)
	}

	orderNo := "LOOKUP-" + strings.ToUpper(stamp)
	if err := pool.QueryRow(ctx, `
		INSERT INTO mall_orders (
		  order_no, idempotency_key, user_id, original_credits, discount_credits, total_credits,
		  coupon_code, status, payment_method, paid_at, created_at, updated_at
		) VALUES ($1, $2, $3, 0, 0, 0, '', 'PAID', 'credits', $4, $4, $4)
		RETURNING id`, orderNo, "lookup-"+stamp, userID, now).Scan(&orderID); err != nil {
		t.Fatalf("insert lookup order: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO mall_order_items (
		  order_id, product_id, sku, title, category, grant_type, grant_key,
		  quantity, unit_price_credits, subtotal_credits
		) VALUES ($1, $2, $3, 'Lookup membership', 'digital', 'membership', $4, 1, 0, 0)`,
		orderID, productID, sku, grantKey); err != nil {
		t.Fatalf("insert lookup order item: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mall_digital_entitlements (
		  order_id, product_id, user_id, sku, title, quantity, fulfillment_code,
		  grant_type, grant_key, status, issued_at, expires_at, created_at
		) VALUES ($1, $2, $3, $4, 'Lookup membership', 1, $5, 'membership', $6, 'ACTIVE', $7, $8, $7)`,
		orderID, productID, userID, sku, "BBS-LOOKUP-"+strings.ToUpper(stamp), grantKey, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lookup entitlement: %v", err)
	}
	return productID, orderID
}

func openActiveEntitlementLookupSmokePool(t *testing.T) (context.Context, *pgxpool.Pool, *activeEntitlementQueryCounter) {
	t.Helper()
	if os.Getenv("BBS_MALL_PG_SMOKE") != "1" {
		t.Skip("set BBS_MALL_PG_SMOKE=1 to run PostgreSQL active entitlement lookup smoke test")
	}

	dsn := os.Getenv("BBS_MALL_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_mall_app:local_mall_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_mall"
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	counter := &activeEntitlementQueryCounter{}
	config.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	return context.Background(), pool, counter
}

type activeEntitlementQueryCounter struct {
	mu    sync.Mutex
	count int
}

func (c *activeEntitlementQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "SELECT DISTINCT de.user_id") {
		c.mu.Lock()
		c.count++
		c.mu.Unlock()
	}
	return ctx
}

func (*activeEntitlementQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {
}

func (c *activeEntitlementQueryCounter) Reset() {
	c.mu.Lock()
	c.count = 0
	c.mu.Unlock()
}

func (c *activeEntitlementQueryCounter) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}
