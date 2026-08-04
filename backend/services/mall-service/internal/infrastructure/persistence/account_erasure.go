package persistence

import (
	"context"
	"database/sql"
	"errors"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
)

const mallUserAdvisoryLockSQL = `SELECT pg_advisory_xact_lock(hashtextextended('bbs-mall-user:' || $1::BIGINT::TEXT, 0))`

type accountErasureReceipt struct {
	AnonymizedUserID       int64
	AnonymizedOrders       int64
	AnonymizedPayments     int64
	AnonymizedRefunds      int64
	AnonymizedCouponUsages int64
	ClosedOrders           int64
	FailedPayments         int64
	ReleasedCouponUsages   int64
	CanceledRefunds        int64
	RevokedEntitlements    int64
	RedactedReviews        int64
	DeletedAddresses       int64
	DeletedCartItems       int64
	DeletedFavorites       int64
	DeletedCouponClaims    int64
	SuppressedOutboxEvents int64
	PolicyVersion          int32
	ErasedAt               time.Time
	CompletedAt            sql.NullTime
}

func (r *PostgresRepository) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	if r == nil || r.pool == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.AccountErasureResult{}, domain.ErrInvalidAccountErasure
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, mallUserAdvisoryLockSQL, userID); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if _, err := tx.Exec(ctx, `SET LOCAL bbs.mall_erasure = 'on'`); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		return domain.AccountErasureResult{}, err
	}

	receipt, found, err := loadAccountErasureReceipt(ctx, tx, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	if found && receipt.CompletedAt.Valid && policyVersion <= receipt.PolicyVersion {
		if err := tx.Commit(ctx); err != nil {
			return domain.AccountErasureResult{}, err
		}
		return receipt.result(), nil
	}
	if !found {
		err = tx.QueryRow(ctx, `
INSERT INTO mall_erased_users(user_id, deletion_job_id, policy_version, anonymized_user_id)
VALUES($1, $2, $3, nextval('mall_erased_user_id_seq'))
RETURNING anonymized_user_id, erased_at
`, userID, deletionJobID, policyVersion).Scan(&receipt.AnonymizedUserID, &receipt.ErasedAt)
		if err != nil {
			return domain.AccountErasureResult{}, err
		}
		receipt.PolicyVersion = policyVersion
	} else if policyVersion > receipt.PolicyVersion {
		if _, err := tx.Exec(ctx, `
UPDATE mall_erased_users
SET deletion_job_id = $2, policy_version = $3, completed_at = NULL
WHERE user_id = $1
`, userID, deletionJobID, policyVersion); err != nil {
			return domain.AccountErasureResult{}, err
		}
		receipt.PolicyVersion = policyVersion
	}

	operationAt := time.Now().UTC()
	suppressedOutbox, err := suppressUserOutboxEvents(ctx, tx, userID, receipt.AnonymizedUserID, operationAt)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	closedOrders, failedPayments, releasedCoupons, err := closeOpenOrdersForErasure(ctx, tx, userID, operationAt)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}

	deletedAddresses, err := tx.Exec(ctx, `DELETE FROM mall_addresses WHERE user_id = $1`, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	deletedCartItems, err := tx.Exec(ctx, `DELETE FROM mall_cart_items WHERE user_id = $1`, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	deletedFavorites, err := tx.Exec(ctx, `DELETE FROM mall_product_favorites WHERE user_id = $1`, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	deletedCouponClaims, err := tx.Exec(ctx, `
DELETE FROM mall_coupon_usages
WHERE user_id = $1 AND status = $2 AND order_id IS NULL
`, userID, string(domain.CouponUsageStatusClaimed))
	if err != nil {
		return domain.AccountErasureResult{}, err
	}

	if err := redactUserAuditLogs(ctx, tx, userID); err != nil {
		return domain.AccountErasureResult{}, err
	}
	canceledRefunds, err := tx.Exec(ctx, `
UPDATE mall_refund_requests
SET status = $2,
    reason = '',
    user_note = '',
    admin_note = '',
    restore_stock = false,
    operator_id = '',
    reviewed_at = NULL,
    refunded_at = NULL,
    canceled_at = COALESCE(canceled_at, $3),
    updated_at = GREATEST(updated_at, $3)
WHERE user_id = $1 AND status IN ($4, $5)
`, userID, string(domain.RefundStatusCanceled), operationAt,
		string(domain.RefundStatusRequested), string(domain.RefundStatusProcessing))
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	revokedEntitlements, err := tx.Exec(ctx, `
UPDATE mall_digital_entitlements
SET status = $2,
    revoked_at = COALESCE(revoked_at, $3),
    revoked_by = 'account-erasure',
    revoke_reason = 'account-erasure'
WHERE user_id = $1 AND status = $4
`, userID, domain.DigitalEntitlementStatusRevoked, operationAt, domain.DigitalEntitlementStatusActive)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	redactedReviews, err := tx.Exec(ctx, `
UPDATE mall_product_reviews
SET user_id = $2,
    content = 'account-erased',
    status = $3,
    updated_at = GREATEST(updated_at, $4)
WHERE user_id = $1
`, userID, receipt.AnonymizedUserID, string(domain.ProductReviewStatusHidden), operationAt)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE mall_digital_entitlements
SET user_id = $2,
    fulfillment_code = 'account-erased-entitlement:' || id::TEXT
WHERE user_id = $1
`, userID, receipt.AnonymizedUserID); err != nil {
		return domain.AccountErasureResult{}, err
	}
	anonymizedCouponUsages, err := tx.Exec(ctx, `UPDATE mall_coupon_usages SET user_id = $2 WHERE user_id = $1`, userID, receipt.AnonymizedUserID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	anonymizedPayments, err := tx.Exec(ctx, `
UPDATE mall_payments
SET user_id = $2,
    idempotency_key = 'account-erased-payment:' || id::TEXT,
    failure_reason = CASE WHEN status = 'FAILED' THEN 'account-erased' ELSE '' END,
    updated_at = GREATEST(updated_at, $3)
WHERE user_id = $1
`, userID, receipt.AnonymizedUserID, operationAt)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	anonymizedRefunds, err := tx.Exec(ctx, `
UPDATE mall_refund_requests
SET user_id = $2,
    reason = '',
    user_note = '',
    admin_note = '',
    updated_at = GREATEST(updated_at, $3)
WHERE user_id = $1
`, userID, receipt.AnonymizedUserID, operationAt)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	anonymizedOrders, err := tx.Exec(ctx, `
UPDATE mall_orders
SET user_id = $2,
    idempotency_key = 'account-erased-order:' || id::TEXT,
    receiver = '',
    phone = '',
    address = '',
    shipping_carrier = '',
    tracking_no = '',
    updated_at = GREATEST(updated_at, $3)
WHERE user_id = $1
`, userID, receipt.AnonymizedUserID, operationAt)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}

	receipt.AnonymizedOrders += anonymizedOrders.RowsAffected()
	receipt.AnonymizedPayments += anonymizedPayments.RowsAffected()
	receipt.AnonymizedRefunds += anonymizedRefunds.RowsAffected()
	receipt.AnonymizedCouponUsages += anonymizedCouponUsages.RowsAffected()
	receipt.ClosedOrders += closedOrders
	receipt.FailedPayments += failedPayments
	receipt.ReleasedCouponUsages += releasedCoupons
	receipt.CanceledRefunds += canceledRefunds.RowsAffected()
	receipt.RevokedEntitlements += revokedEntitlements.RowsAffected()
	receipt.RedactedReviews += redactedReviews.RowsAffected()
	receipt.DeletedAddresses += deletedAddresses.RowsAffected()
	receipt.DeletedCartItems += deletedCartItems.RowsAffected()
	receipt.DeletedFavorites += deletedFavorites.RowsAffected()
	receipt.DeletedCouponClaims += deletedCouponClaims.RowsAffected()
	receipt.SuppressedOutboxEvents += suppressedOutbox
	if _, err := tx.Exec(ctx, `
UPDATE mall_erased_users
SET anonymized_orders = $2,
    anonymized_payments = $3,
    anonymized_refunds = $4,
    anonymized_coupon_usages = $5,
    closed_orders = $6,
    failed_payments = $7,
    released_coupon_usages = $8,
    canceled_refunds = $9,
    revoked_entitlements = $10,
    redacted_reviews = $11,
    deleted_addresses = $12,
    deleted_cart_items = $13,
    deleted_favorites = $14,
    deleted_coupon_claims = $15,
    suppressed_outbox_events = $16,
    completed_at = $17
WHERE user_id = $1
`, userID, receipt.AnonymizedOrders, receipt.AnonymizedPayments, receipt.AnonymizedRefunds,
		receipt.AnonymizedCouponUsages, receipt.ClosedOrders, receipt.FailedPayments,
		receipt.ReleasedCouponUsages, receipt.CanceledRefunds, receipt.RevokedEntitlements,
		receipt.RedactedReviews, receipt.DeletedAddresses, receipt.DeletedCartItems,
		receipt.DeletedFavorites, receipt.DeletedCouponClaims, receipt.SuppressedOutboxEvents,
		operationAt); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccountErasureResult{}, err
	}
	return receipt.result(), nil
}

func loadAccountErasureReceipt(ctx context.Context, tx pgx.Tx, userID int64) (accountErasureReceipt, bool, error) {
	var receipt accountErasureReceipt
	err := tx.QueryRow(ctx, `
SELECT anonymized_user_id, anonymized_orders, anonymized_payments, anonymized_refunds,
       anonymized_coupon_usages, closed_orders, failed_payments, released_coupon_usages,
       canceled_refunds, revoked_entitlements, redacted_reviews, deleted_addresses,
       deleted_cart_items, deleted_favorites, deleted_coupon_claims,
       suppressed_outbox_events, policy_version, erased_at, completed_at
FROM mall_erased_users
WHERE user_id = $1
FOR UPDATE
`, userID).Scan(&receipt.AnonymizedUserID, &receipt.AnonymizedOrders, &receipt.AnonymizedPayments,
		&receipt.AnonymizedRefunds, &receipt.AnonymizedCouponUsages, &receipt.ClosedOrders,
		&receipt.FailedPayments, &receipt.ReleasedCouponUsages, &receipt.CanceledRefunds,
		&receipt.RevokedEntitlements, &receipt.RedactedReviews, &receipt.DeletedAddresses,
		&receipt.DeletedCartItems, &receipt.DeletedFavorites, &receipt.DeletedCouponClaims,
		&receipt.SuppressedOutboxEvents, &receipt.PolicyVersion, &receipt.ErasedAt, &receipt.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return accountErasureReceipt{}, false, nil
	}
	return receipt, err == nil, err
}

func closeOpenOrdersForErasure(ctx context.Context, tx pgx.Tx, userID int64, closedAt time.Time) (int64, int64, int64, error) {
	rows, err := tx.Query(ctx, `
SELECT id
FROM mall_orders
WHERE user_id = $1 AND status IN ($2, $3)
ORDER BY id
FOR UPDATE
`, userID, string(domain.OrderStatusPendingPayment), string(domain.OrderStatusPaying))
	if err != nil {
		return 0, 0, 0, err
	}
	orderIDs := make([]int64, 0)
	for rows.Next() {
		var orderID int64
		if err := rows.Scan(&orderID); err != nil {
			rows.Close()
			return 0, 0, 0, err
		}
		orderIDs = append(orderIDs, orderID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, 0, 0, err
	}

	var closedOrders, failedPayments, releasedCoupons int64
	for _, orderID := range orderIDs {
		order, err := getOrder(ctx, tx, orderID)
		if err != nil {
			return 0, 0, 0, err
		}
		failed, err := tx.Exec(ctx, `
UPDATE mall_payments
SET status = $2,
    failure_reason = 'account-erasure',
    provider_trade_no = '',
    paid_at = NULL,
    updated_at = $3
WHERE order_id = $1 AND status = $4
`, orderID, string(domain.PaymentStatusFailed), closedAt, string(domain.PaymentStatusPending))
		if err != nil {
			return 0, 0, 0, err
		}
		updated, err := tx.Exec(ctx, `
UPDATE mall_orders
SET status = $2, updated_at = $3
WHERE id = $1 AND status = $4
`, orderID, string(domain.OrderStatusClosed), closedAt, string(order.Status))
		if err != nil {
			return 0, 0, 0, err
		}
		if updated.RowsAffected() == 0 {
			continue
		}
		for _, item := range stockDeductionItems(order.Items) {
			if err := releaseProductStock(ctx, tx, item.ProductID, item.Quantity,
				domain.StockChangeReasonOrderExpired, domain.StockReferenceOrder, order.ID,
				domain.OrderStatusOperatorAdmin, "system", "account erasure closed order", closedAt); err != nil {
				return 0, 0, 0, err
			}
		}
		released, err := tx.Exec(ctx, `
UPDATE mall_coupon_usages
SET status = $2, released_at = $3, updated_at = $3
WHERE order_id = $1 AND status = $4
`, orderID, string(domain.CouponUsageStatusReleased), closedAt, string(domain.CouponUsageStatusReserved))
		if err != nil {
			return 0, 0, 0, err
		}
		if err := insertOrderStatusLog(ctx, tx, orderID, order.Status, domain.OrderStatusClosed,
			domain.OrderStatusReasonExpired, domain.OrderStatusOperatorAdmin, "system", "account erasure closed order", closedAt); err != nil {
			return 0, 0, 0, err
		}
		closedOrders++
		failedPayments += failed.RowsAffected()
		releasedCoupons += released.RowsAffected()
	}
	return closedOrders, failedPayments, releasedCoupons, nil
}

func suppressUserOutboxEvents(ctx context.Context, tx pgx.Tx, userID, anonymizedUserID int64, suppressedAt time.Time) (int64, error) {
	tag, err := tx.Exec(ctx, `
UPDATE mall_outbox_events event
SET payload_json = jsonb_build_object(
      'event_id', event.event_id,
      'event_type', event.event_type,
      'erased', TRUE
    ),
    message_key = $2::BIGINT::TEXT,
    status = 'published',
    lease_owner = '',
    lease_expires_at = NULL,
    last_error = '',
    next_attempt_at = NULL,
    published_at = COALESCE(event.published_at, $3),
    updated_at = GREATEST(event.updated_at, $3)
WHERE event.message_key = $1::BIGINT::TEXT
   OR event.payload_json->>'user_id' = $1::BIGINT::TEXT
   OR (event.aggregate_type = 'mall_order' AND EXISTS (
        SELECT 1 FROM mall_orders item WHERE item.id = event.aggregate_id AND item.user_id = $1
      ))
   OR (event.aggregate_type = 'mall_refund' AND EXISTS (
        SELECT 1 FROM mall_refund_requests item WHERE item.id = event.aggregate_id AND item.user_id = $1
      ))
   OR (event.aggregate_type = 'mall_product_review' AND EXISTS (
        SELECT 1 FROM mall_product_reviews item WHERE item.id = event.aggregate_id AND item.user_id = $1
      ))
   OR (event.aggregate_type = 'mall_digital_entitlement' AND EXISTS (
        SELECT 1 FROM mall_digital_entitlements item WHERE item.id = event.aggregate_id AND item.user_id = $1
      ))
`, userID, anonymizedUserID, suppressedAt)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func redactUserAuditLogs(ctx context.Context, tx pgx.Tx, userID int64) error {
	if _, err := tx.Exec(ctx, `
UPDATE mall_order_status_logs log
SET operator_id = CASE
      WHEN log.operator_type = 'user' THEN 'account-erased'
      ELSE log.operator_id
    END,
    note = ''
WHERE log.order_id IN (SELECT id FROM mall_orders WHERE user_id = $1)
`, userID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
UPDATE mall_product_stock_logs log
SET operator_id = CASE
      WHEN log.operator_type = 'user' THEN 'account-erased'
      ELSE log.operator_id
    END,
    note = ''
WHERE (log.operator_type = 'user' AND log.operator_id = $1::BIGINT::TEXT)
   OR (log.reference_type = 'order' AND log.reference_id IN (
        SELECT id FROM mall_orders WHERE user_id = $1
      ))
   OR (log.reference_type = 'refund' AND log.reference_id IN (
        SELECT id FROM mall_refund_requests WHERE user_id = $1
      ))
`, userID)
	return err
}

func (r accountErasureReceipt) result() domain.AccountErasureResult {
	return domain.AccountErasureResult{
		AnonymizedOrders:       r.AnonymizedOrders,
		AnonymizedPayments:     r.AnonymizedPayments,
		AnonymizedRefunds:      r.AnonymizedRefunds,
		AnonymizedCouponUsages: r.AnonymizedCouponUsages,
		ClosedOrders:           r.ClosedOrders,
		FailedPayments:         r.FailedPayments,
		ReleasedCouponUsages:   r.ReleasedCouponUsages,
		CanceledRefunds:        r.CanceledRefunds,
		RevokedEntitlements:    r.RevokedEntitlements,
		RedactedReviews:        r.RedactedReviews,
		DeletedAddresses:       r.DeletedAddresses,
		DeletedCartItems:       r.DeletedCartItems,
		DeletedFavorites:       r.DeletedFavorites,
		DeletedCouponClaims:    r.DeletedCouponClaims,
		SuppressedOutboxEvents: r.SuppressedOutboxEvents,
	}
}

var _ domain.AccountErasureRepository = (*PostgresRepository)(nil)
