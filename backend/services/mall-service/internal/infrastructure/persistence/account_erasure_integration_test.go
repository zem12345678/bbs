package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountErasurePostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_MALL_PG_SMOKE") != "1" {
		t.Skip("set BBS_MALL_PG_SMOKE=1 to run the PostgreSQL account-erasure test")
	}
	dsn := os.Getenv("BBS_MALL_PG_DSN")
	if dsn == "" {
		t.Skip("set BBS_MALL_PG_DSN to run the PostgreSQL account-erasure test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewPostgresRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure mall schema: %v", err)
	}

	base := time.Now().UnixMilli() * 1000
	userID, otherUserID := base+1, base+2
	jobID := base + 3
	productID, couponID := base+10, base+11
	pendingOrderID, completedOrderID := base+20, base+21
	pendingPaymentID, succeededPaymentID := base+30, base+31
	claimedUsageID, reservedUsageID := base+40, base+41
	addressID, refundID := base+50, base+51
	entitlementID, reviewID := base+60, base+61
	eventID := uuid.NewString()
	suffix := strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:12]
	productSKU := "ERASURE-" + suffix
	couponCode := "ERASE" + suffix
	pendingOrderNo := "MO-ERASE-P-" + suffix
	completedOrderNo := "MO-ERASE-C-" + suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	createdAt := now.Add(-time.Hour)
	paidAt := createdAt.Add(10 * time.Minute)
	completedAt := paidAt.Add(10 * time.Minute)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_outbox_events WHERE event_id = $1`, eventID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_product_reviews WHERE id = $1`, reviewID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_digital_entitlements WHERE id = $1`, entitlementID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_refund_requests WHERE id = $1`, refundID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_coupon_usages WHERE id = ANY($1::BIGINT[])`, []int64{claimedUsageID, reservedUsageID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_payments WHERE id = ANY($1::BIGINT[])`, []int64{pendingPaymentID, succeededPaymentID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_order_status_logs WHERE order_id = ANY($1::BIGINT[])`, []int64{pendingOrderID, completedOrderID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_product_stock_logs WHERE product_id = $1`, productID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_order_items WHERE order_id = ANY($1::BIGINT[])`, []int64{pendingOrderID, completedOrderID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_orders WHERE id = ANY($1::BIGINT[])`, []int64{pendingOrderID, completedOrderID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_addresses WHERE id = $1 OR user_id = $2`, addressID, otherUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_cart_items WHERE user_id IN ($1, $2)`, userID, otherUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_product_favorites WHERE user_id IN ($1, $2)`, userID, otherUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_coupons WHERE id = $1`, couponID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_products WHERE id = $1`, productID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM mall_erased_users WHERE user_id = $1`, userID)
	})

	if _, err := pool.Exec(ctx, `
INSERT INTO mall_products(
  id, sku, title, description, category, cover_url, grant_type, grant_key,
  price_credits, stock, sales_count, status, sort, created_at, updated_at
)
VALUES($1, $2, 'Erasure product', '', 'digital', '', 'digital', $3, 100, 9, 1, 'ACTIVE', 0, $4, $4)
`, productID, productSKU, strings.ToLower(productSKU), createdAt); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_coupons(
  id, code, name, description, discount_credits, min_order_credits,
  total_quota, per_user_limit, status, created_at, updated_at
)
VALUES($1, $2, 'Erasure coupon', '', 10, 0, 100, 10, 'ACTIVE', $3, $3)
`, couponID, couponCode, createdAt); err != nil {
		t.Fatalf("seed coupon: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_orders(
  id, order_no, idempotency_key, user_id, original_credits, discount_credits,
  total_credits, coupon_id, coupon_code, status, receiver, phone, address,
  payment_method, paid_at, completed_at, created_at, updated_at
)
VALUES
  ($1, $2, $3, $4, 100, 10, 90, $5, $6, 'PENDING_PAYMENT', 'Private Receiver', '13800000000', 'Private Address', 'credits', NULL, NULL, $7, $7),
  ($8, $9, $10, $4, 100, 0, 100, NULL, '', 'COMPLETED', 'Historical Receiver', '13900000000', 'Historical Address', 'credits', $11, $12, $7, $12)
`, pendingOrderID, pendingOrderNo, "order-pending-"+suffix, userID, couponID, couponCode,
		createdAt, completedOrderID, completedOrderNo, "order-completed-"+suffix, paidAt, completedAt); err != nil {
		t.Fatalf("seed orders: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_order_items(
  order_id, product_id, sku, title, category, grant_type, grant_key,
  quantity, unit_price_credits, subtotal_credits
)
VALUES
  ($1, $2, $3, 'Erasure product', 'digital', 'digital', $4, 1, 100, 100),
  ($5, $2, $3, 'Erasure product', 'digital', 'digital', $4, 1, 100, 100)
`, pendingOrderID, productID, productSKU, strings.ToLower(productSKU), completedOrderID); err != nil {
		t.Fatalf("seed order items: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_payments(
  id, order_id, user_id, amount_credits, provider, idempotency_key, status,
  provider_trade_no, failure_reason, paid_at, created_at, updated_at
)
VALUES
  ($1, $2, $3, 90, 'credits', $4, 'PENDING', '', '', NULL, $5, $5),
  ($6, $7, $3, 100, 'credits', $8, 'SUCCEEDED', $9, '', $10, $5, $10)
`, pendingPaymentID, pendingOrderID, userID, "payment-pending-"+suffix, createdAt,
		succeededPaymentID, completedOrderID, "payment-completed-"+suffix, "credit-trade-"+suffix, paidAt); err != nil {
		t.Fatalf("seed payments: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_coupon_usages(
  id, coupon_id, code, user_id, order_id, status, discount_credits,
  created_at, used_at, released_at, updated_at
)
VALUES
  ($1, $2, $3, $4, NULL, 'CLAIMED', 10, $5, NULL, NULL, $5),
  ($6, $2, $3, $4, $7, 'RESERVED', 10, $5, NULL, NULL, $5)
`, claimedUsageID, couponID, couponCode, userID, createdAt, reservedUsageID, pendingOrderID); err != nil {
		t.Fatalf("seed coupon usages: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_addresses(
  id, user_id, receiver, phone, province, city, district, detail,
  postal_code, is_default, created_at, updated_at
)
VALUES($1, $2, 'Private Receiver', '13800000000', 'P', 'C', 'D', 'Private Detail', '100000', true, $3, $3)
`, addressID, userID, createdAt); err != nil {
		t.Fatalf("seed address: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_cart_items(user_id, product_id, quantity, created_at, updated_at)
VALUES($1, $2, 1, $3, $3)
`, userID, productID, createdAt); err != nil {
		t.Fatalf("seed cart: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_product_favorites(user_id, product_id, created_at)
VALUES($1, $2, $3)
`, userID, productID, createdAt); err != nil {
		t.Fatalf("seed favorite: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_refund_requests(
  id, order_id, order_no, user_id, amount_credits, status, reason, user_note,
  admin_note, restore_stock, operator_id, requested_at, reviewed_at,
  refunded_at, canceled_at, created_at, updated_at
)
VALUES($1, $2, $3, $4, 100, 'REQUESTED', 'private reason', 'private note', '', false, '', $5, NULL, NULL, NULL, $5, $5)
`, refundID, completedOrderID, completedOrderNo, userID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("seed refund: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_digital_entitlements(
  id, order_id, product_id, user_id, sku, title, quantity, fulfillment_code,
  grant_type, grant_key, status, issued_at, expires_at, revoked_at, refund_id,
  revoked_by, revoke_reason, created_at
)
VALUES($1, $2, $3, $4, $5, 'Erasure product', 1, $6, 'digital', $7, 'ACTIVE', $8, NULL, NULL, NULL, '', '', $8)
`, entitlementID, completedOrderID, productID, userID, productSKU, "private-code-"+suffix,
		strings.ToLower(productSKU), paidAt); err != nil {
		t.Fatalf("seed entitlement: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_product_reviews(
  id, product_id, order_id, user_id, rating, content, status, created_at, updated_at
)
VALUES($1, $2, $3, $4, 5, 'private review', 'PUBLISHED', $5, $5)
`, reviewID, productID, completedOrderID, userID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("seed review: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_order_status_logs(
  order_id, from_status, to_status, reason, operator_type, operator_id, note, created_at
)
VALUES($1, '', 'PENDING_PAYMENT', 'created', 'user', $2::BIGINT::TEXT, 'private status note', $3)
`, pendingOrderID, userID, createdAt); err != nil {
		t.Fatalf("seed order log: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_product_stock_logs(
  product_id, sku, title, delta, before_stock, after_stock, reason,
  reference_type, reference_id, operator_type, operator_id, note, created_at
)
VALUES($1, $2, 'Erasure product', -1, 10, 9, 'order_created', 'order', $3, 'user', $4::BIGINT::TEXT, 'private stock note', $5)
`, productID, productSKU, pendingOrderID, userID, createdAt); err != nil {
		t.Fatalf("seed stock log: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_outbox_events(
  event_id, aggregate_type, aggregate_id, event_type, message_key, payload_json,
  status, attempts, created_at, updated_at
)
VALUES($1, 'mall_product_review', $2, 'mall.product_review.published.v1', $3::BIGINT::TEXT,
       jsonb_build_object('event_id', $1::TEXT, 'event_type', 'mall.product_review.published.v1', 'user_id', $3::BIGINT, 'content', 'private review'),
       'pending', 0, $4, $4)
`, eventID, reviewID, userID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}

	got, err := repo.EraseUserData(ctx, userID, jobID, 1)
	if err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	want := domain.AccountErasureResult{
		AnonymizedOrders: 2, AnonymizedPayments: 2, AnonymizedRefunds: 1,
		AnonymizedCouponUsages: 1, ClosedOrders: 1, FailedPayments: 1,
		ReleasedCouponUsages: 1, CanceledRefunds: 1, RevokedEntitlements: 1,
		RedactedReviews: 1, DeletedAddresses: 1, DeletedCartItems: 1,
		DeletedFavorites: 1, DeletedCouponClaims: 1, SuppressedOutboxEvents: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EraseUserData() = %+v, want %+v", got, want)
	}

	var anonymizedUserID int64
	var storedPolicy int32
	if err := pool.QueryRow(ctx, `
SELECT anonymized_user_id, policy_version
FROM mall_erased_users WHERE user_id = $1
`, userID).Scan(&anonymizedUserID, &storedPolicy); err != nil {
		t.Fatalf("load erasure receipt: %v", err)
	}
	if anonymizedUserID >= 0 || storedPolicy != 1 {
		t.Fatalf("erasure identity = %d policy = %d, want negative identity and policy 1", anonymizedUserID, storedPolicy)
	}
	for _, table := range []string{
		"mall_orders", "mall_payments", "mall_coupon_usages", "mall_refund_requests",
		"mall_digital_entitlements", "mall_product_reviews", "mall_addresses",
		"mall_cart_items", "mall_product_favorites",
	} {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = $1", table)
		if err := pool.QueryRow(ctx, query, userID).Scan(&count); err != nil {
			t.Fatalf("count original user rows in %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("original user rows in %s = %d, want 0", table, count)
		}
	}

	var orderStatus, receiver, phone, address, orderKey string
	if err := pool.QueryRow(ctx, `
SELECT status, receiver, phone, address, idempotency_key
FROM mall_orders WHERE id = $1
`, pendingOrderID).Scan(&orderStatus, &receiver, &phone, &address, &orderKey); err != nil {
		t.Fatalf("load redacted order: %v", err)
	}
	if orderStatus != string(domain.OrderStatusClosed) || receiver != "" || phone != "" || address != "" || !strings.HasPrefix(orderKey, "account-erased-order:") {
		t.Fatalf("redacted order = status %q receiver %q phone %q address %q key %q", orderStatus, receiver, phone, address, orderKey)
	}
	var stock int64
	if err := pool.QueryRow(ctx, `SELECT stock FROM mall_products WHERE id = $1`, productID).Scan(&stock); err != nil {
		t.Fatalf("load restored product stock: %v", err)
	}
	if stock != 10 {
		t.Fatalf("restored product stock = %d, want 10", stock)
	}
	var paymentStatus, paymentFailure string
	if err := pool.QueryRow(ctx, `SELECT status, failure_reason FROM mall_payments WHERE id = $1`, pendingPaymentID).Scan(&paymentStatus, &paymentFailure); err != nil {
		t.Fatalf("load failed payment: %v", err)
	}
	if paymentStatus != string(domain.PaymentStatusFailed) || paymentFailure != "account-erased" {
		t.Fatalf("failed payment = status %q reason %q", paymentStatus, paymentFailure)
	}
	var couponStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM mall_coupon_usages WHERE id = $1`, reservedUsageID).Scan(&couponStatus); err != nil {
		t.Fatalf("load released coupon: %v", err)
	}
	if couponStatus != string(domain.CouponUsageStatusReleased) {
		t.Fatalf("coupon status = %q, want RELEASED", couponStatus)
	}
	var refundStatus, refundReason, refundNote string
	if err := pool.QueryRow(ctx, `SELECT status, reason, user_note FROM mall_refund_requests WHERE id = $1`, refundID).Scan(&refundStatus, &refundReason, &refundNote); err != nil {
		t.Fatalf("load canceled refund: %v", err)
	}
	if refundStatus != string(domain.RefundStatusCanceled) || refundReason != "" || refundNote != "" {
		t.Fatalf("refund = status %q reason %q note %q", refundStatus, refundReason, refundNote)
	}
	var entitlementStatus, fulfillmentCode, revokedBy string
	if err := pool.QueryRow(ctx, `
SELECT status, fulfillment_code, revoked_by
FROM mall_digital_entitlements WHERE id = $1
`, entitlementID).Scan(&entitlementStatus, &fulfillmentCode, &revokedBy); err != nil {
		t.Fatalf("load revoked entitlement: %v", err)
	}
	if entitlementStatus != domain.DigitalEntitlementStatusRevoked || !strings.HasPrefix(fulfillmentCode, "account-erased-entitlement:") || revokedBy != "account-erasure" {
		t.Fatalf("entitlement = status %q code %q revoked_by %q", entitlementStatus, fulfillmentCode, revokedBy)
	}
	var reviewStatus, reviewContent string
	if err := pool.QueryRow(ctx, `SELECT status, content FROM mall_product_reviews WHERE id = $1`, reviewID).Scan(&reviewStatus, &reviewContent); err != nil {
		t.Fatalf("load redacted review: %v", err)
	}
	if reviewStatus != string(domain.ProductReviewStatusHidden) || reviewContent != "account-erased" {
		t.Fatalf("review = status %q content %q", reviewStatus, reviewContent)
	}
	var outboxStatus, outboxUserID, outboxContent string
	if err := pool.QueryRow(ctx, `
SELECT status, COALESCE(payload_json->>'user_id', ''), COALESCE(payload_json->>'content', '')
FROM mall_outbox_events WHERE event_id = $1
`, eventID).Scan(&outboxStatus, &outboxUserID, &outboxContent); err != nil {
		t.Fatalf("load suppressed outbox: %v", err)
	}
	if outboxStatus != "published" || outboxUserID != "" || outboxContent != "" {
		t.Fatalf("outbox = status %q user %q content %q", outboxStatus, outboxUserID, outboxContent)
	}

	replayed, err := repo.EraseUserData(ctx, userID, jobID, 1)
	if err != nil || !reflect.DeepEqual(replayed, want) {
		t.Fatalf("replayed EraseUserData() = %+v, %v; want %+v", replayed, err, want)
	}
	upgraded, err := repo.EraseUserData(ctx, userID, jobID+1, 2)
	if err != nil || !reflect.DeepEqual(upgraded, want) {
		t.Fatalf("upgraded EraseUserData() = %+v, %v; want %+v", upgraded, err, want)
	}
	var upgradedIdentity int64
	if err := pool.QueryRow(ctx, `SELECT anonymized_user_id FROM mall_erased_users WHERE user_id = $1`, userID).Scan(&upgradedIdentity); err != nil {
		t.Fatalf("load upgraded erasure identity: %v", err)
	}
	if upgradedIdentity != anonymizedUserID {
		t.Fatalf("upgraded anonymized identity = %d, want stable %d", upgradedIdentity, anonymizedUserID)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO mall_addresses(user_id, receiver, phone, detail, created_at, updated_at)
VALUES($1, 'late', 'late', 'late', $2, $2)
`, userID, now)
	assertMallErasedWriteError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mall_orders SET receiver = 'late' WHERE id = $1`, completedOrderID)
	assertMallErasedWriteError(t, err)
	if _, err := pool.Exec(ctx, `
INSERT INTO mall_addresses(user_id, receiver, phone, detail, created_at, updated_at)
VALUES($1, 'other', 'other', 'other', $2, $2)
`, otherUserID, now); err != nil {
		t.Fatalf("write for active user after erasure: %v", err)
	}
}

func assertMallErasedWriteError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("late write error = nil, want mall account erased")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || pgErr.Message != "mall account erased" {
		t.Fatalf("late write error = %v, want P0001 mall account erased", err)
	}
}
