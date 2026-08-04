package persistence

var accountErasureSchemaStatements = []string{
	`CREATE SEQUENCE IF NOT EXISTS mall_erased_user_id_seq AS BIGINT
	  INCREMENT BY -1 MINVALUE -9223372036854775808 MAXVALUE -1 START WITH -1`,
	`CREATE TABLE IF NOT EXISTS mall_erased_users (
	  user_id BIGINT PRIMARY KEY,
	  anonymized_user_id BIGINT NOT NULL UNIQUE,
	  deletion_job_id BIGINT NOT NULL,
	  policy_version INTEGER NOT NULL,
	  anonymized_orders BIGINT NOT NULL DEFAULT 0,
	  anonymized_payments BIGINT NOT NULL DEFAULT 0,
	  anonymized_refunds BIGINT NOT NULL DEFAULT 0,
	  anonymized_coupon_usages BIGINT NOT NULL DEFAULT 0,
	  closed_orders BIGINT NOT NULL DEFAULT 0,
	  failed_payments BIGINT NOT NULL DEFAULT 0,
	  released_coupon_usages BIGINT NOT NULL DEFAULT 0,
	  canceled_refunds BIGINT NOT NULL DEFAULT 0,
	  revoked_entitlements BIGINT NOT NULL DEFAULT 0,
	  redacted_reviews BIGINT NOT NULL DEFAULT 0,
	  deleted_addresses BIGINT NOT NULL DEFAULT 0,
	  deleted_cart_items BIGINT NOT NULL DEFAULT 0,
	  deleted_favorites BIGINT NOT NULL DEFAULT 0,
	  deleted_coupon_claims BIGINT NOT NULL DEFAULT 0,
	  suppressed_outbox_events BIGINT NOT NULL DEFAULT 0,
	  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	  completed_at TIMESTAMPTZ,
	  CONSTRAINT mall_erased_users_identity_check CHECK (
	    user_id > 0 AND anonymized_user_id < 0 AND deletion_job_id > 0 AND policy_version > 0
	  ),
	  CONSTRAINT mall_erased_users_counts_check CHECK (
	    anonymized_orders >= 0 AND anonymized_payments >= 0
	    AND anonymized_refunds >= 0 AND anonymized_coupon_usages >= 0
	    AND closed_orders >= 0 AND failed_payments >= 0
	    AND released_coupon_usages >= 0 AND canceled_refunds >= 0
	    AND revoked_entitlements >= 0 AND redacted_reviews >= 0
	    AND deleted_addresses >= 0 AND deleted_cart_items >= 0
	    AND deleted_favorites >= 0 AND deleted_coupon_claims >= 0
	    AND suppressed_outbox_events >= 0
	  )
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_erased_users_job ON mall_erased_users (deletion_job_id)`,
	`DO $$
	 BEGIN
	   IF EXISTS (
	     SELECT 1 FROM pg_constraint
	     WHERE conname = 'mall_orders_lifecycle_check'
	       AND conrelid = 'mall_orders'::regclass
	       AND pg_get_constraintdef(oid) NOT LIKE '%user_id <> 0%'
	   ) THEN
	     ALTER TABLE mall_orders DROP CONSTRAINT mall_orders_lifecycle_check;
	   END IF;
	   IF NOT EXISTS (
	     SELECT 1 FROM pg_constraint
	     WHERE conname = 'mall_orders_lifecycle_check'
	       AND conrelid = 'mall_orders'::regclass
	   ) THEN
	     ALTER TABLE mall_orders
	     ADD CONSTRAINT mall_orders_lifecycle_check
	     CHECK (
	       user_id <> 0
	       AND status = UPPER(TRIM(status))
	       AND status IN ('PENDING_PAYMENT', 'PAYING', 'PAID', 'CANCELED', 'SHIPPED', 'COMPLETED', 'CLOSED', 'REFUNDED')
	       AND updated_at >= created_at
	       AND (
	         (status IN ('PENDING_PAYMENT', 'PAYING', 'CANCELED', 'CLOSED') AND paid_at IS NULL AND shipped_at IS NULL AND completed_at IS NULL)
	         OR (status = 'PAID' AND paid_at IS NOT NULL AND shipped_at IS NULL AND completed_at IS NULL)
	         OR (status = 'SHIPPED' AND paid_at IS NOT NULL AND shipped_at IS NOT NULL AND completed_at IS NULL)
	         OR (status = 'COMPLETED' AND paid_at IS NOT NULL AND completed_at IS NOT NULL)
	         OR (status = 'REFUNDED' AND paid_at IS NOT NULL)
	       )
	       AND (paid_at IS NULL OR paid_at >= created_at)
	       AND (shipped_at IS NULL OR (paid_at IS NOT NULL AND shipped_at >= paid_at))
	       AND (completed_at IS NULL OR (paid_at IS NOT NULL AND completed_at >= paid_at AND (shipped_at IS NULL OR completed_at >= shipped_at)))
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF EXISTS (
	     SELECT 1 FROM pg_constraint
	     WHERE conname = 'mall_product_reviews_lifecycle_check'
	       AND conrelid = 'mall_product_reviews'::regclass
	       AND pg_get_constraintdef(oid) NOT LIKE '%user_id <> 0%'
	   ) THEN
	     ALTER TABLE mall_product_reviews DROP CONSTRAINT mall_product_reviews_lifecycle_check;
	   END IF;
	   IF NOT EXISTS (
	     SELECT 1 FROM pg_constraint
	     WHERE conname = 'mall_product_reviews_lifecycle_check'
	       AND conrelid = 'mall_product_reviews'::regclass
	   ) THEN
	     ALTER TABLE mall_product_reviews
	     ADD CONSTRAINT mall_product_reviews_lifecycle_check
	     CHECK (
	       user_id <> 0
	       AND status = UPPER(TRIM(status))
	       AND status IN ('PENDING', 'PUBLISHED', 'HIDDEN')
	       AND BTRIM(content) <> ''
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 DECLARE
	   item RECORD;
	 BEGIN
	   FOR item IN SELECT * FROM (VALUES
	     ('mall_digital_entitlements', 'mall_digital_entitlements_order_user_fkey'),
	     ('mall_payments', 'mall_payments_order_user_fkey'),
	     ('mall_coupon_usages', 'mall_coupon_usages_order_user_fkey'),
	     ('mall_coupon_usages', 'mall_coupon_usages_order_coupon_snapshot_fkey'),
	     ('mall_refund_requests', 'mall_refund_requests_order_user_fkey'),
	     ('mall_refund_requests', 'mall_refund_requests_order_snapshot_fkey'),
	     ('mall_digital_entitlements', 'mall_digital_entitlements_refund_order_user_fkey'),
	     ('mall_product_reviews', 'mall_product_reviews_order_user_fkey')
	   ) AS constraints_to_defer(table_name, constraint_name)
	   LOOP
	     IF EXISTS (
	       SELECT 1 FROM pg_constraint
	       WHERE conname = item.constraint_name
	         AND conrelid = item.table_name::regclass
	         AND (NOT condeferrable OR NOT condeferred)
	     ) THEN
	       EXECUTE format(
	         'ALTER TABLE %I ALTER CONSTRAINT %I DEFERRABLE INITIALLY DEFERRED',
	         item.table_name,
	         item.constraint_name
	       );
	     END IF;
	   END LOOP;
	 END $$`,
	`CREATE OR REPLACE FUNCTION mall_reject_erased_identity()
	 RETURNS TRIGGER
	 LANGUAGE plpgsql
	 AS $$
	 DECLARE
	   old_identity_id BIGINT;
	   new_identity_id BIGINT;
	   identity_id BIGINT;
	   lock_user_id BIGINT;
	 BEGIN
	   old_identity_id := NULLIF(to_jsonb(OLD)->>TG_ARGV[0], '')::BIGINT;
	   new_identity_id := NULLIF(to_jsonb(NEW)->>TG_ARGV[0], '')::BIGINT;
	   FOREACH identity_id IN ARRAY ARRAY[old_identity_id, new_identity_id]
	   LOOP
	     IF identity_id IS NULL OR identity_id = 0 THEN
	       CONTINUE;
	     END IF;
	     lock_user_id := identity_id;
	     IF identity_id < 0 THEN
	       SELECT user_id INTO lock_user_id
	       FROM mall_erased_users
	       WHERE anonymized_user_id = identity_id;
	       IF lock_user_id IS NULL THEN
	         CONTINUE;
	       END IF;
	     END IF;
	     IF NOT pg_try_advisory_xact_lock(hashtextextended('bbs-mall-user:' || lock_user_id::TEXT, 0)) THEN
	       RAISE EXCEPTION 'mall account erased' USING ERRCODE = 'P0001';
	     END IF;
	   END LOOP;
	   IF current_setting('bbs.mall_erasure', true) = 'on' THEN
	     RETURN NEW;
	   END IF;
	   IF EXISTS (
	     SELECT 1 FROM mall_erased_users
	     WHERE user_id IN (old_identity_id, new_identity_id)
	        OR anonymized_user_id IN (old_identity_id, new_identity_id)
	   ) THEN
	     RAISE EXCEPTION 'mall account erased' USING ERRCODE = 'P0001';
	   END IF;
	   RETURN NEW;
	 END;
	 $$`,
	`DO $$
	 DECLARE
	   item RECORD;
	 BEGIN
	   FOR item IN SELECT * FROM (VALUES
	     ('mall_orders', 'user_id', 'mall_orders_erased_user_guard'),
	     ('mall_digital_entitlements', 'user_id', 'mall_digital_entitlements_erased_user_guard'),
	     ('mall_payments', 'user_id', 'mall_payments_erased_user_guard'),
	     ('mall_cart_items', 'user_id', 'mall_cart_items_erased_user_guard'),
	     ('mall_product_favorites', 'user_id', 'mall_product_favorites_erased_user_guard'),
	     ('mall_coupon_usages', 'user_id', 'mall_coupon_usages_erased_user_guard'),
	     ('mall_addresses', 'user_id', 'mall_addresses_erased_user_guard'),
	     ('mall_refund_requests', 'user_id', 'mall_refund_requests_erased_user_guard'),
	     ('mall_product_reviews', 'user_id', 'mall_product_reviews_erased_user_guard')
	   ) AS guards(table_name, column_name, trigger_name)
	   LOOP
	     IF NOT EXISTS (
	       SELECT 1 FROM pg_trigger
	       WHERE tgname = item.trigger_name
	         AND tgrelid = item.table_name::regclass
	         AND NOT tgisinternal
	     ) THEN
	       EXECUTE format(
	         'CREATE TRIGGER %I BEFORE INSERT OR UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION mall_reject_erased_identity(%L)',
	         item.trigger_name,
	         item.table_name,
	         item.column_name
	       );
	     END IF;
	   END LOOP;
	 END $$`,
}
