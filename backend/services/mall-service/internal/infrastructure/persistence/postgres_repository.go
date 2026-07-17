package persistence

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

const (
	cartAdvisoryLockBase    int64 = 4200000000000
	addressAdvisoryLockBase int64 = 4300000000000
)

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) EnsureSchema(ctx context.Context) error {
	for _, statement := range schemaStatements {
		if _, err := r.pool.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) ListProducts(ctx context.Context, query domain.ProductListQuery) ([]domain.Product, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	keyword := strings.TrimSpace(query.Keyword)
	category := strings.TrimSpace(query.Category)
	total, err := r.countProducts(ctx, category, keyword, domain.ProductStatusActive)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductSQL()+`
		WHERE status = $1
		  AND ($2 = '' OR category = $2)
		  AND ($3 = '' OR title ILIKE '%' || $3 || '%' OR description ILIKE '%' || $3 || '%' OR sku ILIKE '%' || $3 || '%')
		ORDER BY sort ASC, created_at DESC, id DESC
		LIMIT $4 OFFSET $5`,
		string(domain.ProductStatusActive),
		category,
		keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProducts(rows, total)
}

func (r *PostgresRepository) ListProductCategories(ctx context.Context, query domain.ProductCategoryListQuery) ([]domain.ProductCategory, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	total, err := r.countProductCategories(ctx, "", domain.ProductCategoryStatusActive)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductCategorySQL()+`
		WHERE c.status = $1
		ORDER BY c.sort ASC, c.created_at DESC, c.id DESC
		LIMIT $2 OFFSET $3`,
		string(domain.ProductCategoryStatusActive),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProductCategories(rows, total)
}

func (r *PostgresRepository) GetProduct(ctx context.Context, productID int64) (domain.Product, error) {
	return scanProduct(r.pool.QueryRow(ctx, selectProductSQL()+` WHERE id = $1`, productID))
}

func (r *PostgresRepository) AdminListProducts(ctx context.Context, query domain.ProductListQuery) ([]domain.Product, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	keyword := strings.TrimSpace(query.Keyword)
	category := strings.TrimSpace(query.Category)
	status := query.Status
	total, err := r.countProducts(ctx, category, keyword, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductSQL()+`
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR category = $2)
		  AND ($3 = '' OR title ILIKE '%' || $3 || '%' OR description ILIKE '%' || $3 || '%' OR sku ILIKE '%' || $3 || '%')
		ORDER BY sort ASC, created_at DESC, id DESC
		LIMIT $4 OFFSET $5`,
		string(status),
		category,
		keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProducts(rows, total)
}

func (r *PostgresRepository) AdminListProductCategories(ctx context.Context, query domain.ProductCategoryListQuery) ([]domain.ProductCategory, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	keyword := strings.TrimSpace(query.Keyword)
	status := query.Status
	total, err := r.countProductCategories(ctx, keyword, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductCategorySQL()+`
		WHERE ($1 = '' OR c.status = $1)
		  AND ($2 = '' OR c.slug ILIKE '%' || $2 || '%' OR c.name ILIKE '%' || $2 || '%' OR c.description ILIKE '%' || $2 || '%')
		ORDER BY c.sort ASC, c.created_at DESC, c.id DESC
		LIMIT $3 OFFSET $4`,
		string(status),
		keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProductCategories(rows, total)
}

func (r *PostgresRepository) AdminMallOverview(ctx context.Context, lowStockThreshold int64) (domain.MallOverview, error) {
	if lowStockThreshold <= 0 {
		lowStockThreshold = 10
	}
	var overview domain.MallOverview
	if err := r.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*),
		  COUNT(*) FILTER (WHERE status = $2),
		  COUNT(*) FILTER (WHERE status = $2 AND stock <= $1),
		  COALESCE(SUM(stock), 0),
		  COALESCE(SUM(sales_count), 0)
		FROM mall_products`,
		lowStockThreshold,
		string(domain.ProductStatusActive),
	).Scan(
		&overview.ProductTotal,
		&overview.ActiveProductTotal,
		&overview.LowStockTotal,
		&overview.StockTotal,
		&overview.SalesCountTotal,
	); err != nil {
		return domain.MallOverview{}, err
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*),
		  COUNT(*) FILTER (WHERE status IN ('PAID', 'SHIPPED', 'COMPLETED', 'REFUNDED')),
		  COALESCE(SUM(total_credits) FILTER (WHERE status IN ('PAID', 'SHIPPED', 'COMPLETED', 'REFUNDED')), 0),
		  COUNT(*) FILTER (WHERE created_at >= date_trunc('day', NOW())),
		  COALESCE(SUM(total_credits) FILTER (WHERE paid_at >= date_trunc('day', NOW()) AND status IN ('PAID', 'SHIPPED', 'COMPLETED', 'REFUNDED')), 0),
		  COUNT(*) FILTER (WHERE status = 'PAID')
		FROM mall_orders`,
	).Scan(
		&overview.OrderTotal,
		&overview.PaidOrderTotal,
		&overview.RevenueCreditsTotal,
		&overview.TodayOrderTotal,
		&overview.TodayRevenueCredits,
		&overview.PendingShipmentTotal,
	); err != nil {
		return domain.MallOverview{}, err
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status IN ('REQUESTED', 'PROCESSING')),
		  COALESCE(SUM(amount_credits) FILTER (WHERE status IN ('REQUESTED', 'PROCESSING')), 0),
		  COALESCE(SUM(amount_credits) FILTER (WHERE status = 'APPROVED'), 0)
		FROM mall_refund_requests`,
	).Scan(
		&overview.PendingRefundTotal,
		&overview.PendingRefundCreditsTotal,
		&overview.RefundedCreditsTotal,
	); err != nil {
		return domain.MallOverview{}, err
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(amount_credits) FILTER (WHERE status = 'SUCCEEDED'), 0),
		  COUNT(*) FILTER (WHERE status = 'FAILED'),
		  COALESCE(SUM(amount_credits) FILTER (WHERE status = 'FAILED'), 0)
		FROM mall_payments`,
	).Scan(
		&overview.SucceededPaymentCreditsTotal,
		&overview.FailedPaymentTotal,
		&overview.FailedPaymentCreditsTotal,
	); err != nil {
		return domain.MallOverview{}, err
	}
	overview.NetRevenueCreditsTotal = overview.RevenueCreditsTotal - overview.RefundedCreditsTotal
	pendingOutboxTotal, err := r.CountPendingOutboxEvents(ctx)
	if err != nil {
		return domain.MallOverview{}, err
	}
	overview.PendingOutboxTotal = int64(pendingOutboxTotal)
	outboxCounts, err := r.statusCounts(ctx, `
		SELECT status, COUNT(*)
		FROM mall_outbox_events
		WHERE status IN ('pending', 'publishing', 'failed', 'dead_letter')
		GROUP BY status
		ORDER BY status ASC`)
	if err != nil {
		return domain.MallOverview{}, err
	}
	overview.OutboxStatusCounts = outboxCounts
	var outboxLastErrorAt sql.NullTime
	if err := r.pool.QueryRow(ctx, `
		SELECT last_error, updated_at
		FROM mall_outbox_events
		WHERE last_error <> ''
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1`).Scan(&overview.OutboxLastError, &outboxLastErrorAt); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.MallOverview{}, err
	}
	if outboxLastErrorAt.Valid {
		overview.OutboxLastErrorAt = &outboxLastErrorAt.Time
	}
	var outboxNextAttemptAt sql.NullTime
	if err := r.pool.QueryRow(ctx, `
		SELECT MIN(next_attempt_at)
		FROM mall_outbox_events
		WHERE status = 'failed'
		  AND next_attempt_at IS NOT NULL`).Scan(&outboxNextAttemptAt); err != nil {
		return domain.MallOverview{}, err
	}
	if outboxNextAttemptAt.Valid {
		overview.OutboxNextAttemptAt = &outboxNextAttemptAt.Time
	}

	financeAnomalies, financeAnomalyTotal, err := r.financeAnomalies(ctx, 5)
	if err != nil {
		return domain.MallOverview{}, err
	}
	overview.FinanceAnomalies = financeAnomalies
	overview.FinanceAnomalyTotal = financeAnomalyTotal

	orderCounts, err := r.statusCounts(ctx, `SELECT status, COUNT(*) FROM mall_orders GROUP BY status ORDER BY status ASC`)
	if err != nil {
		return domain.MallOverview{}, err
	}
	overview.OrderStatusCounts = orderCounts
	refundCounts, err := r.statusCounts(ctx, `SELECT status, COUNT(*) FROM mall_refund_requests GROUP BY status ORDER BY status ASC`)
	if err != nil {
		return domain.MallOverview{}, err
	}
	overview.RefundStatusCounts = refundCounts

	lowStockRows, err := r.pool.Query(ctx, selectProductSQL()+`
		WHERE status = $2
		  AND stock <= $1
		ORDER BY stock ASC, sales_count DESC, updated_at DESC, id DESC
		LIMIT 5`,
		lowStockThreshold,
		string(domain.ProductStatusActive),
	)
	if err != nil {
		return domain.MallOverview{}, err
	}
	defer lowStockRows.Close()
	lowStockProducts, _, err := scanProducts(lowStockRows, 0)
	if err != nil {
		return domain.MallOverview{}, err
	}
	overview.LowStockProducts = lowStockProducts

	topSellingRows, err := r.pool.Query(ctx, selectProductSQL()+`
		WHERE status = $1
		ORDER BY sales_count DESC, updated_at DESC, id DESC
		LIMIT 5`,
		string(domain.ProductStatusActive),
	)
	if err != nil {
		return domain.MallOverview{}, err
	}
	defer topSellingRows.Close()
	topSellingProducts, _, err := scanProducts(topSellingRows, 0)
	if err != nil {
		return domain.MallOverview{}, err
	}
	overview.TopSellingProducts = topSellingProducts
	return overview, nil
}

func (r *PostgresRepository) AdminListOutboxRequeueAudits(ctx context.Context, query domain.OutboxRequeueAuditListQuery) ([]domain.OutboxRequeueAudit, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	eventID := strings.TrimSpace(query.EventID)
	aggregateType := strings.TrimSpace(query.AggregateType)

	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_outbox_requeue_audits
		WHERE ($1 = '' OR event_id = $1)
		  AND ($2 = '' OR aggregate_type = $2)
		  AND ($3 = 0 OR aggregate_id = $3)`,
		eventID,
		aggregateType,
		query.AggregateID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, aggregate_type, aggregate_id, previous_status, previous_attempts, previous_error, operator_id, requeued_at
		FROM mall_outbox_requeue_audits
		WHERE ($1 = '' OR event_id = $1)
		  AND ($2 = '' OR aggregate_type = $2)
		  AND ($3 = 0 OR aggregate_id = $3)
		ORDER BY requeued_at DESC, id DESC
		LIMIT $4 OFFSET $5`,
		eventID,
		aggregateType,
		query.AggregateID,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanOutboxRequeueAudits(rows, total)
}

func (r *PostgresRepository) AdminCreateProductCategory(ctx context.Context, category domain.ProductCategory) (domain.ProductCategory, error) {
	created, err := scanProductCategory(r.pool.QueryRow(ctx, `
		INSERT INTO mall_product_categories (slug, name, description, status, sort, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, slug, name, description, status, sort, 0::BIGINT AS product_count, created_at, updated_at`,
		category.Slug,
		category.Name,
		category.Description,
		string(category.Status),
		category.Sort,
		category.CreatedAt,
		category.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ProductCategory{}, domain.ErrDuplicateReference
		}
		return domain.ProductCategory{}, err
	}
	return created, nil
}

func (r *PostgresRepository) AdminUpdateProductCategory(ctx context.Context, category domain.ProductCategory) (domain.ProductCategory, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ProductCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := scanProductCategory(tx.QueryRow(ctx, selectProductCategorySQL()+` WHERE c.id = $1 FOR UPDATE`, category.ID))
	if err != nil {
		return domain.ProductCategory{}, err
	}
	if err := ensureProductCategoryChangeAllowed(ctx, tx, existing, category); err != nil {
		return domain.ProductCategory{}, err
	}
	updated, err := scanProductCategory(tx.QueryRow(ctx, `
		UPDATE mall_product_categories
		SET slug = $2,
		    name = $3,
		    description = $4,
		    status = $5,
		    sort = $6,
		    updated_at = $7
		WHERE id = $1
		RETURNING id, slug, name, description, status, sort,
		  (SELECT COUNT(*) FROM mall_products p WHERE p.category = mall_product_categories.slug) AS product_count,
		  created_at, updated_at`,
		category.ID,
		category.Slug,
		category.Name,
		category.Description,
		string(category.Status),
		category.Sort,
		category.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ProductCategory{}, domain.ErrDuplicateReference
		}
		return domain.ProductCategory{}, err
	}
	if existing.Slug != updated.Slug {
		if _, err := tx.Exec(ctx, `UPDATE mall_products SET category = $2, updated_at = $3 WHERE category = $1`, existing.Slug, updated.Slug, category.UpdatedAt); err != nil {
			return domain.ProductCategory{}, err
		}
		updated.ProductCount = existing.ProductCount
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ProductCategory{}, err
	}
	return updated, nil
}

func (r *PostgresRepository) ListProductReviews(ctx context.Context, query domain.ProductReviewListQuery) ([]domain.ProductReview, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	total, err := r.countProductReviews(ctx, query.ProductID, 0, domain.ProductReviewStatusPublished)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductReviewSQL()+`
		WHERE r.product_id = $1
		  AND r.status = $2
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $3 OFFSET $4`,
		query.ProductID,
		string(domain.ProductReviewStatusPublished),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProductReviews(rows, total)
}

func (r *PostgresRepository) ListUserProductReviews(ctx context.Context, query domain.ProductReviewListQuery) ([]domain.ProductReview, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	total, err := r.countProductReviews(ctx, query.ProductID, query.UserID, query.Status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductReviewSQL()+`
		WHERE r.user_id = $1
		  AND ($2::BIGINT = 0 OR r.product_id = $2::BIGINT)
		  AND ($3 = '' OR r.status = $3)
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $4 OFFSET $5`,
		query.UserID,
		query.ProductID,
		string(query.Status),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProductReviews(rows, total)
}

func (r *PostgresRepository) CreateProductReview(ctx context.Context, review domain.ProductReview) (domain.ProductReview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ProductReview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, review.OrderID)
	if err != nil {
		return domain.ProductReview{}, err
	}
	created, err := createProductReviewForOrder(ctx, tx, review, order)
	if err != nil {
		return domain.ProductReview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ProductReview{}, err
	}
	return created, nil
}

func createProductReviewForOrder(ctx context.Context, db queryer, review domain.ProductReview, order domain.Order) (domain.ProductReview, error) {
	if order.UserID != review.UserID {
		return domain.ProductReview{}, domain.ErrOrderOwnerMismatch
	}
	if order.Status != domain.OrderStatusCompleted {
		return domain.ProductReview{}, domain.ErrInvalidOrderState
	}
	product, err := scanProduct(db.QueryRow(ctx, selectProductSQL()+` WHERE id = $1`, review.ProductID))
	if err != nil {
		return domain.ProductReview{}, err
	}
	if product.Status != domain.ProductStatusActive {
		return domain.ProductReview{}, domain.ErrProductNotFound
	}
	refundRequested, err := orderHasRefundRequest(ctx, db, review.OrderID)
	if err != nil {
		return domain.ProductReview{}, err
	}
	if refundRequested {
		return domain.ProductReview{}, domain.ErrInvalidOrderState
	}
	var included bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mall_order_items WHERE order_id = $1 AND product_id = $2)`, review.OrderID, review.ProductID).Scan(&included); err != nil {
		return domain.ProductReview{}, err
	}
	if !included {
		return domain.ProductReview{}, domain.ErrInvalidOrderState
	}

	created, err := scanProductReview(db.QueryRow(ctx, `
		WITH inserted AS (
		  INSERT INTO mall_product_reviews (product_id, order_id, user_id, rating, content, status, created_at, updated_at)
		  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		  RETURNING id, product_id, order_id, user_id, rating, content, status, created_at, updated_at
		)
		SELECT i.id, i.product_id, p.sku, p.title, i.order_id, i.user_id, i.rating, i.content, i.status, i.created_at, i.updated_at
		FROM inserted i
		JOIN mall_products p ON p.id = i.product_id`,
		review.ProductID,
		review.OrderID,
		review.UserID,
		review.Rating,
		review.Content,
		string(review.Status),
		review.CreatedAt,
		review.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ProductReview{}, domain.ErrDuplicateReference
		}
		return domain.ProductReview{}, err
	}
	return created, nil
}

func (r *PostgresRepository) GetProductReview(ctx context.Context, reviewID int64) (domain.ProductReview, error) {
	return scanProductReview(r.pool.QueryRow(ctx, selectProductReviewSQL()+`
		WHERE r.id = $1`,
		reviewID,
	))
}

func (r *PostgresRepository) AdminListProductReviews(ctx context.Context, query domain.ProductReviewListQuery) ([]domain.ProductReview, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	status := query.Status
	total, err := r.countProductReviews(ctx, query.ProductID, query.UserID, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductReviewSQL()+`
		WHERE ($1::BIGINT = 0 OR r.product_id = $1::BIGINT)
		  AND ($2::BIGINT = 0 OR r.user_id = $2::BIGINT)
		  AND ($3 = '' OR r.status = $3)
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $4 OFFSET $5`,
		query.ProductID,
		query.UserID,
		string(status),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProductReviews(rows, total)
}

func (r *PostgresRepository) AdminUpdateProductReviewStatus(ctx context.Context, reviewID int64, reviewStatus domain.ProductReviewStatus, updatedAt time.Time, event domain.OutboxEvent) (domain.ProductReview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ProductReview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	review, err := scanProductReview(tx.QueryRow(ctx, `
		WITH updated AS (
		  UPDATE mall_product_reviews
		  SET status = $2,
		      updated_at = $3
		  WHERE id = $1
		  RETURNING id, product_id, order_id, user_id, rating, content, status, created_at, updated_at
		)
		SELECT u.id, u.product_id, p.sku, p.title, u.order_id, u.user_id, u.rating, u.content, u.status, u.created_at, u.updated_at
		FROM updated u
		JOIN mall_products p ON p.id = u.product_id`,
		reviewID,
		string(reviewStatus),
		updatedAt,
	))
	if err != nil {
		return domain.ProductReview{}, err
	}
	if event.EventID != "" {
		if err := insertOutboxEvent(ctx, tx, event); err != nil {
			return domain.ProductReview{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ProductReview{}, err
	}
	return review, nil
}

func (r *PostgresRepository) CreateProduct(ctx context.Context, product domain.Product, operatorID string) (domain.Product, error) {
	product = normalizeProductGrantContract(product)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureProductCategoryUsable(ctx, tx, product); err != nil {
		return domain.Product{}, err
	}
	created, err := scanProduct(tx.QueryRow(ctx, `
		INSERT INTO mall_products (
		  sku, title, description, category, cover_url, grant_type, grant_key, price_credits, stock, sales_count, status, sort, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, 0, $10, $11, $12, $13
		)
		RETURNING `+selectProductColumns(),
		product.SKU,
		product.Title,
		product.Description,
		product.Category,
		product.CoverURL,
		product.GrantType,
		product.GrantKey,
		product.PriceCredits,
		product.Stock,
		string(product.Status),
		product.Sort,
		product.CreatedAt,
		product.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Product{}, domain.ErrDuplicateReference
		}
		return domain.Product{}, err
	}
	if err := insertProductStockLog(ctx, tx, domain.ProductStockLog{
		ProductID:     created.ID,
		SKU:           created.SKU,
		Title:         created.Title,
		Delta:         created.Stock,
		BeforeStock:   0,
		AfterStock:    created.Stock,
		Reason:        domain.StockChangeReasonProductCreated,
		ReferenceType: domain.StockReferenceProduct,
		ReferenceID:   created.ID,
		OperatorType:  domain.OrderStatusOperatorAdmin,
		OperatorID:    strings.TrimSpace(operatorID),
		Note:          "商品创建初始库存",
		CreatedAt:     created.CreatedAt,
	}); err != nil {
		return domain.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Product{}, err
	}
	return created, nil
}

func (r *PostgresRepository) UpdateProduct(ctx context.Context, product domain.Product, operatorID string) (domain.Product, error) {
	product = normalizeProductGrantContract(product)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := scanProduct(tx.QueryRow(ctx, selectProductSQL()+` WHERE id = $1 FOR UPDATE`, product.ID))
	if err != nil {
		return domain.Product{}, err
	}
	if err := ensureProductGrantMutable(ctx, tx, existing, product); err != nil {
		return domain.Product{}, err
	}
	if err := ensureProductCategoryUsable(ctx, tx, product); err != nil {
		return domain.Product{}, err
	}
	updated, err := scanProduct(tx.QueryRow(ctx, `
		UPDATE mall_products
		SET sku = $2,
		    title = $3,
		    description = $4,
		    category = $5,
		    cover_url = $6,
		    grant_type = $7,
		    grant_key = $8,
		    price_credits = $9,
		    stock = $10,
		    status = $11,
		    sort = $12,
		    updated_at = $13
		WHERE id = $1
		RETURNING `+selectProductColumns(),
		product.ID,
		product.SKU,
		product.Title,
		product.Description,
		product.Category,
		product.CoverURL,
		product.GrantType,
		product.GrantKey,
		product.PriceCredits,
		product.Stock,
		string(product.Status),
		product.Sort,
		product.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Product{}, domain.ErrDuplicateReference
		}
		return domain.Product{}, err
	}
	if updated.Stock != existing.Stock {
		if err := insertProductStockLog(ctx, tx, domain.ProductStockLog{
			ProductID:     updated.ID,
			SKU:           updated.SKU,
			Title:         updated.Title,
			Delta:         updated.Stock - existing.Stock,
			BeforeStock:   existing.Stock,
			AfterStock:    updated.Stock,
			Reason:        domain.StockChangeReasonManualAdjustment,
			ReferenceType: domain.StockReferenceProduct,
			ReferenceID:   updated.ID,
			OperatorType:  domain.OrderStatusOperatorAdmin,
			OperatorID:    strings.TrimSpace(operatorID),
			Note:          "运营端调整商品库存",
			CreatedAt:     updated.UpdatedAt,
		}); err != nil {
			return domain.Product{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Product{}, err
	}
	return updated, nil
}

func ensureProductCategoryUsable(ctx context.Context, db queryer, product domain.Product) error {
	category := strings.TrimSpace(product.Category)
	if category == "" {
		return domain.ErrProductCategoryNotFound
	}
	var status string
	err := db.QueryRow(ctx, `
		SELECT status
		FROM mall_product_categories
		WHERE slug = $1
		FOR SHARE`,
		category,
	).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrProductCategoryNotFound
		}
		return err
	}
	if product.Status == domain.ProductStatusActive && domain.ProductCategoryStatus(status) != domain.ProductCategoryStatusActive {
		return domain.ErrProductCategoryUnavailable
	}
	return nil
}

func ensureProductCategoryChangeAllowed(ctx context.Context, db queryer, existing, next domain.ProductCategory) error {
	if existing.Slug != next.Slug {
		var referencedProducts int64
		if err := db.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM mall_products
			WHERE category = $1`,
			existing.Slug,
		).Scan(&referencedProducts); err != nil {
			return err
		}
		if referencedProducts > 0 {
			return domain.ErrProductCategorySlugLocked
		}
	}
	if next.Status == domain.ProductCategoryStatusActive {
		return nil
	}
	var activeProducts int64
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_products
		WHERE category = $1
		  AND status = $2`,
		existing.Slug,
		string(domain.ProductStatusActive),
	).Scan(&activeProducts); err != nil {
		return err
	}
	if activeProducts > 0 {
		return domain.ErrProductCategoryLocked
	}
	return nil
}

func ensureProductGrantMutable(ctx context.Context, db queryer, existing, next domain.Product) error {
	grantChanged := productGrantChanged(existing, next)
	fulfillmentChanged := productFulfillmentChanged(existing, next)
	if !grantChanged && !fulfillmentChanged {
		return nil
	}
	var locked bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM mall_order_items oi
		  JOIN mall_orders o ON o.id = oi.order_id
		  WHERE oi.product_id = $1
		    AND o.status IN ($2, $3, $4, $5)
		  LIMIT 1
		)`,
		existing.ID,
		string(domain.OrderStatusPaid),
		string(domain.OrderStatusShipped),
		string(domain.OrderStatusCompleted),
		string(domain.OrderStatusRefunded),
	).Scan(&locked); err != nil {
		return err
	}
	if locked {
		if fulfillmentChanged {
			return domain.ErrProductFulfillmentLocked
		}
		return domain.ErrProductGrantLocked
	}
	return nil
}

func productGrantChanged(existing, next domain.Product) bool {
	existing = normalizeProductGrantContract(existing)
	next = normalizeProductGrantContract(next)
	return existing.GrantType != next.GrantType || existing.GrantKey != next.GrantKey
}

func normalizeProductGrant(product domain.Product) (string, string) {
	grantType := normalizeProductGrantField(product.GrantType)
	grantKey := normalizeProductGrantField(product.GrantKey)
	if grantType == "" && grantKey != "" {
		grantType = digitalGrantTypeForKey(grantKey)
	}
	return grantType, grantKey
}

func normalizeProductGrantContract(product domain.Product) domain.Product {
	grantType, grantKey := normalizeProductGrant(product)
	if strings.EqualFold(strings.TrimSpace(product.Category), "digital") {
		if grantKey == "" {
			grantKey = normalizeProductGrantField(product.SKU)
		}
		if grantType == "" {
			grantType = digitalGrantTypeForKey(grantKey)
		}
	}
	product.GrantType = grantType
	product.GrantKey = grantKey
	return product
}

func normalizeProductGrantField(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func productFulfillmentChanged(existing, next domain.Product) bool {
	return productRequiresShippingForFulfillment(existing) != productRequiresShippingForFulfillment(next)
}

func productRequiresShippingForFulfillment(product domain.Product) bool {
	if strings.EqualFold(strings.TrimSpace(product.Category), "digital") {
		return false
	}
	grantType, grantKey := normalizeProductGrant(product)
	return grantType == "" && grantKey == ""
}

func (r *PostgresRepository) AdminListProductStockLogs(ctx context.Context, query domain.ProductStockLogQuery) ([]domain.ProductStockLog, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	reason := strings.TrimSpace(query.Reason)
	total, err := r.countProductStockLogs(ctx, query.ProductID, reason)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectProductStockLogSQL()+`
		WHERE ($1::BIGINT = 0 OR product_id = $1::BIGINT)
		  AND ($2 = '' OR reason = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`,
		query.ProductID,
		reason,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProductStockLogs(rows, total)
}

func (r *PostgresRepository) ListAvailableCoupons(ctx context.Context, limit, offset int, now time.Time) ([]domain.Coupon, int64, error) {
	limit = domain.NormalizeListLimit(limit)
	offset = domain.NormalizeOffset(offset)
	total, err := r.countAvailableCoupons(ctx, now)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectCouponSQL()+`
		WHERE c.status = $1
		  AND (c.starts_at IS NULL OR c.starts_at <= $2)
		  AND (c.ends_at IS NULL OR c.ends_at > $2)
		  AND (
		    c.total_quota = 0
		    OR (
		      SELECT COUNT(*)
		      FROM mall_coupon_usages u
		      WHERE u.coupon_id = c.id
		        AND u.status <> $3
		    ) < c.total_quota
		  )
		ORDER BY c.discount_credits DESC, c.min_order_credits ASC, c.created_at DESC, c.id DESC
		LIMIT $4 OFFSET $5`,
		string(domain.CouponStatusActive),
		now,
		string(domain.CouponUsageStatusReleased),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanCoupons(rows, total)
}

func (r *PostgresRepository) AdminListCoupons(ctx context.Context, query domain.CouponListQuery) ([]domain.Coupon, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	keyword := strings.TrimSpace(query.Keyword)
	status := domain.NormalizeCouponStatus(query.Status)
	total, err := r.countCoupons(ctx, keyword, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectCouponSQL()+`
		WHERE ($1 = '' OR c.status = $1)
		  AND `+couponListKeywordCondition(2)+`
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $3 OFFSET $4`,
		string(status),
		keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanCoupons(rows, total)
}

func (r *PostgresRepository) AdminCreateCoupon(ctx context.Context, coupon domain.Coupon) (domain.Coupon, error) {
	created, err := scanCoupon(r.pool.QueryRow(ctx, `
		INSERT INTO mall_coupons (
		  code, name, description, discount_credits, min_order_credits, total_quota, per_user_limit, status, starts_at, ends_at, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		RETURNING `+selectCouponColumns(),
		coupon.Code,
		coupon.Name,
		coupon.Description,
		coupon.DiscountCredits,
		coupon.MinOrderCredits,
		coupon.TotalQuota,
		coupon.PerUserLimit,
		string(coupon.Status),
		coupon.StartsAt,
		coupon.EndsAt,
		coupon.CreatedAt,
		coupon.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Coupon{}, domain.ErrDuplicateReference
		}
		return domain.Coupon{}, err
	}
	return created, nil
}

func (r *PostgresRepository) AdminUpdateCoupon(ctx context.Context, coupon domain.Coupon) (domain.Coupon, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Coupon{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := scanCoupon(tx.QueryRow(ctx, selectCouponSQL()+` WHERE c.id = $1 FOR UPDATE`, coupon.ID))
	if err != nil {
		return domain.Coupon{}, err
	}
	if err := ensureCouponTermsMutable(ctx, tx, existing, coupon); err != nil {
		return domain.Coupon{}, err
	}
	updated, err := scanCoupon(tx.QueryRow(ctx, `
		UPDATE mall_coupons
		SET code = $2,
		    name = $3,
		    description = $4,
		    discount_credits = $5,
		    min_order_credits = $6,
		    total_quota = $7,
		    per_user_limit = $8,
		    status = $9,
		    starts_at = $10,
		    ends_at = $11,
		    updated_at = $12
		WHERE id = $1
		RETURNING `+selectCouponColumns(),
		coupon.ID,
		coupon.Code,
		coupon.Name,
		coupon.Description,
		coupon.DiscountCredits,
		coupon.MinOrderCredits,
		coupon.TotalQuota,
		coupon.PerUserLimit,
		string(coupon.Status),
		coupon.StartsAt,
		coupon.EndsAt,
		coupon.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, domain.ErrCouponNotFound) {
			return domain.Coupon{}, err
		}
		if isUniqueViolation(err) {
			return domain.Coupon{}, domain.ErrDuplicateReference
		}
		return domain.Coupon{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Coupon{}, err
	}
	return updated, nil
}

func ensureCouponTermsMutable(ctx context.Context, db queryer, existing, next domain.Coupon) error {
	if !couponTermsChanged(existing, next) {
		return nil
	}
	var locked bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM mall_coupon_usages
		  WHERE coupon_id = $1
		    AND status <> $2
		  LIMIT 1
		)`,
		existing.ID,
		string(domain.CouponUsageStatusReleased),
	).Scan(&locked); err != nil {
		return err
	}
	if locked {
		return domain.ErrCouponTermsLocked
	}
	return nil
}

func couponTermsChanged(existing, next domain.Coupon) bool {
	return normalizeCouponTermText(existing.Code) != normalizeCouponTermText(next.Code) ||
		existing.DiscountCredits != next.DiscountCredits ||
		existing.MinOrderCredits != next.MinOrderCredits ||
		existing.TotalQuota != next.TotalQuota ||
		existing.PerUserLimit != next.PerUserLimit ||
		existing.Status != next.Status ||
		!nullableTimesEqual(existing.StartsAt, next.StartsAt) ||
		!nullableTimesEqual(existing.EndsAt, next.EndsAt)
}

func normalizeCouponTermText(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func nullableTimesEqual(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func (r *PostgresRepository) AdminListCouponUsages(ctx context.Context, query domain.CouponUsageListQuery) ([]domain.CouponUsage, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	status := domain.NormalizeCouponUsageStatus(query.Status)
	total, err := r.countCouponUsages(ctx, query.CouponID, query.UserID, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		`+selectCouponUsageWithCouponSQL()+`
		WHERE ($1::BIGINT = 0 OR u.coupon_id = $1::BIGINT)
		  AND ($2::BIGINT = 0 OR u.user_id = $2::BIGINT)
		  AND ($3 = '' OR u.status = $3)
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT $4 OFFSET $5`,
		query.CouponID,
		query.UserID,
		string(status),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanCouponUsages(rows, total)
}

func (r *PostgresRepository) ClaimCoupon(ctx context.Context, userID int64, couponID int64, claimedAt time.Time) (domain.CouponUsage, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.CouponUsage{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	coupon, err := lockCouponForClaim(ctx, tx, couponID)
	if err != nil {
		return domain.CouponUsage{}, false, err
	}
	if !couponClaimable(coupon, claimedAt) {
		return domain.CouponUsage{}, false, domain.ErrCouponUnavailable
	}

	existing, err := scanCouponUsageWithCoupon(tx.QueryRow(ctx, `
		`+selectCouponUsageWithCouponSQL()+`
		WHERE u.coupon_id = $1
		  AND u.user_id = $2::BIGINT
		  AND u.status = $3
		ORDER BY u.created_at ASC, u.id ASC
		LIMIT 1
		FOR UPDATE OF u`,
		couponID,
		userID,
		string(domain.CouponUsageStatusClaimed),
	))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return domain.CouponUsage{}, false, err
		}
		return existing, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.CouponUsage{}, false, err
	}

	var totalUses int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_coupon_usages
		WHERE coupon_id = $1
		  AND status <> $2`,
		couponID,
		string(domain.CouponUsageStatusReleased),
	).Scan(&totalUses); err != nil {
		return domain.CouponUsage{}, false, err
	}
	if coupon.TotalQuota > 0 && totalUses >= coupon.TotalQuota {
		return domain.CouponUsage{}, false, domain.ErrCouponUnavailable
	}
	var userUses int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_coupon_usages
		WHERE coupon_id = $1
		  AND user_id = $2::BIGINT
		  AND status <> $3`,
		couponID,
		userID,
		string(domain.CouponUsageStatusReleased),
	).Scan(&userUses); err != nil {
		return domain.CouponUsage{}, false, err
	}
	if coupon.PerUserLimit > 0 && userUses >= coupon.PerUserLimit {
		return domain.CouponUsage{}, false, domain.ErrCouponUnavailable
	}

	var claimedID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO mall_coupon_usages (
		  coupon_id, code, user_id, order_id, status, discount_credits, created_at, updated_at
		) VALUES (
		  $1, $2, $3, NULL, $4, $5, $6, $6
		)
		RETURNING id`,
		coupon.ID,
		coupon.Code,
		userID,
		string(domain.CouponUsageStatusClaimed),
		coupon.DiscountCredits,
		claimedAt,
	).Scan(&claimedID); err != nil {
		return domain.CouponUsage{}, false, err
	}
	claimed, err := scanCouponUsageWithCoupon(tx.QueryRow(ctx, `
		`+selectCouponUsageWithCouponSQL()+`
		WHERE u.id = $1`, claimedID))
	if err != nil {
		return domain.CouponUsage{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.CouponUsage{}, false, err
	}
	return claimed, false, nil
}

func (r *PostgresRepository) ListCouponUsagesByUser(ctx context.Context, query domain.CouponUsageListQuery) ([]domain.CouponUsage, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	status := domain.NormalizeCouponUsageStatus(query.Status)
	total, err := r.countCouponUsages(ctx, 0, query.UserID, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		`+selectCouponUsageWithCouponSQL()+`
		WHERE u.user_id = $1::BIGINT
		  AND ($2 = '' OR u.status = $2)
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT $3 OFFSET $4`,
		query.UserID,
		string(status),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanCouponUsages(rows, total)
}

func (r *PostgresRepository) CreateOrder(ctx context.Context, order domain.Order) (domain.Order, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOrderIdempotencyKey(ctx, tx, order.UserID, order.IdempotencyKey); err != nil {
		return domain.Order{}, false, err
	}
	if existing, err := getOrderByIdempotencyKey(ctx, tx, order.UserID, order.IdempotencyKey); err == nil {
		existing, duplicate, err := idempotentExistingOrder(existing, order)
		if err != nil {
			return domain.Order{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, false, err
		}
		return existing, duplicate, nil
	} else if !errors.Is(err, domain.ErrOrderNotFound) {
		return domain.Order{}, false, err
	}
	if existing, duplicate, err := prepareOwnedDigitalGrantOrderCreation(ctx, tx, order); err != nil {
		return domain.Order{}, false, err
	} else if duplicate {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, false, err
		}
		return existing, true, nil
	}

	saved, duplicate, err := createOrderInTx(ctx, tx, order)
	if err != nil {
		return domain.Order{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, false, err
	}
	return saved, duplicate, nil
}

func (r *PostgresRepository) CreateOrderFromCart(ctx context.Context, order domain.Order) (domain.Order, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockOrderIdempotencyKey(ctx, tx, order.UserID, order.IdempotencyKey); err != nil {
		return domain.Order{}, false, err
	}
	if existing, err := getOrderByIdempotencyKey(ctx, tx, order.UserID, order.IdempotencyKey); err == nil {
		existing, duplicate, err := idempotentExistingOrder(existing, order)
		if err != nil {
			return domain.Order{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, false, err
		}
		return existing, duplicate, nil
	} else if !errors.Is(err, domain.ErrOrderNotFound) {
		return domain.Order{}, false, err
	}
	if err := lockUserCart(ctx, tx, order.UserID); err != nil {
		return domain.Order{}, false, err
	}
	if err := verifyCartMatchesOrder(ctx, tx, order.UserID, order.Items); err != nil {
		return domain.Order{}, false, err
	}
	if existing, duplicate, err := prepareOwnedDigitalGrantOrderCreation(ctx, tx, order); err != nil {
		return domain.Order{}, false, err
	} else if duplicate {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, false, err
		}
		return existing, true, nil
	}
	saved, duplicate, err := createOrderInTx(ctx, tx, order)
	if err != nil {
		return domain.Order{}, false, err
	}
	if !duplicate {
		if _, err := tx.Exec(ctx, `DELETE FROM mall_cart_items WHERE user_id = $1::BIGINT`, order.UserID); err != nil {
			return domain.Order{}, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, false, err
	}
	return saved, duplicate, nil
}

func lockOrderIdempotencyKey(ctx context.Context, db queryer, userID int64, idempotencyKey string) error {
	key := strings.TrimSpace(idempotencyKey)
	if userID <= 0 || key == "" {
		return nil
	}
	_, err := db.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended(CONCAT($1::BIGINT::text, ':', $2::TEXT), 0))`,
		userID,
		key,
	)
	return err
}

type ownedDigitalGrant struct {
	grantType string
	grantKey  string
}

func prepareOwnedDigitalGrantOrderCreation(ctx context.Context, db queryer, order domain.Order) (domain.Order, bool, error) {
	if err := ensureSingleOwnedDigitalGrantPerOrder(order.Items); err != nil {
		return domain.Order{}, false, err
	}
	openOrderGrants := openOrderProtectedDigitalGrantsForOrderItems(order.Items)
	if len(openOrderGrants) == 0 {
		return domain.Order{}, false, nil
	}
	for _, grant := range openOrderGrants {
		if err := lockOwnedDigitalGrant(ctx, db, order.UserID, grant.grantType, grant.grantKey); err != nil {
			return domain.Order{}, false, err
		}
	}
	if existing, err := getOrderByIdempotencyKey(ctx, db, order.UserID, order.IdempotencyKey); err == nil {
		return idempotentExistingOrder(existing, order)
	} else if !errors.Is(err, domain.ErrOrderNotFound) {
		return domain.Order{}, false, err
	}
	for _, grant := range ownedDigitalGrantsForOrderItems(order.Items) {
		active, err := activeDigitalEntitlementExists(ctx, db, order.UserID, grant.grantType, grant.grantKey)
		if err != nil {
			return domain.Order{}, false, err
		}
		if active {
			return domain.Order{}, false, activeOwnedDigitalGrantEntitlementError(grant.grantType)
		}
	}
	for _, grant := range openOrderGrants {
		openOrder, err := openDigitalGrantOrderExists(ctx, db, order.UserID, grant.grantType, grant.grantKey)
		if err != nil {
			return domain.Order{}, false, err
		}
		if openOrder {
			return domain.Order{}, false, pendingOwnedDigitalGrantOrderError(grant.grantType)
		}
	}
	return domain.Order{}, false, nil
}

func ensureNoOtherOpenDigitalGrantOrdersForPayment(ctx context.Context, db queryer, order domain.Order) error {
	if err := ensureSingleOwnedDigitalGrantPerOrder(order.Items); err != nil {
		return err
	}
	openOrderGrants := openOrderProtectedDigitalGrantsForOrderItems(order.Items)
	for _, grant := range openOrderGrants {
		if err := lockOwnedDigitalGrant(ctx, db, order.UserID, grant.grantType, grant.grantKey); err != nil {
			return err
		}
	}
	for _, grant := range ownedDigitalGrantsForOrderItems(order.Items) {
		active, err := activeDigitalEntitlementExists(ctx, db, order.UserID, grant.grantType, grant.grantKey)
		if err != nil {
			return err
		}
		if active {
			return activeOwnedDigitalGrantEntitlementError(grant.grantType)
		}
	}
	for _, grant := range openOrderGrants {
		openOrder, err := openDigitalGrantOrderExistsExcluding(ctx, db, order.UserID, order.ID, grant.grantType, grant.grantKey)
		if err != nil {
			return err
		}
		if openOrder {
			return pendingOwnedDigitalGrantOrderError(grant.grantType)
		}
	}
	return nil
}

func ensureSingleOwnedDigitalGrantPerOrder(items []domain.OrderItem) error {
	seen := make(map[string]struct{})
	for _, item := range items {
		grantType, grantKey := digitalGrantForItem(item)
		if !isSingleOwnedDigitalGrantType(grantType) || grantKey == "" {
			continue
		}
		if item.Quantity > 1 {
			return duplicateOwnedDigitalGrantInOrderError(grantType)
		}
		if item.Quantity <= 0 {
			continue
		}
		key := grantType + ":" + grantKey
		if _, ok := seen[key]; ok {
			return duplicateOwnedDigitalGrantInOrderError(grantType)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func ownedDigitalGrantsForOrderItems(items []domain.OrderItem) []ownedDigitalGrant {
	return digitalGrantsForOrderItems(items, isSingleOwnedDigitalGrantType)
}

func openOrderProtectedDigitalGrantsForOrderItems(items []domain.OrderItem) []ownedDigitalGrant {
	return digitalGrantsForOrderItems(items, isOpenOrderProtectedDigitalGrantType)
}

func digitalGrantsForOrderItems(items []domain.OrderItem, include func(string) bool) []ownedDigitalGrant {
	seen := make(map[string]struct{})
	grants := make([]ownedDigitalGrant, 0)
	for _, item := range items {
		grantType, grantKey := digitalGrantForItem(item)
		if !include(grantType) || grantKey == "" {
			continue
		}
		key := grantType + ":" + grantKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		grants = append(grants, ownedDigitalGrant{grantType: grantType, grantKey: grantKey})
	}
	sort.Slice(grants, func(i, j int) bool {
		if grants[i].grantType == grants[j].grantType {
			return grants[i].grantKey < grants[j].grantKey
		}
		return grants[i].grantType < grants[j].grantType
	})
	return grants
}

func lockOwnedDigitalGrant(ctx context.Context, db queryer, userID int64, grantType, grantKey string) error {
	_, err := db.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended(CONCAT($1::BIGINT::text, ':', LOWER($2), ':', LOWER($3)), 0))`,
		userID,
		strings.ToLower(strings.TrimSpace(grantType)),
		strings.ToLower(strings.TrimSpace(grantKey)),
	)
	return err
}

func isSingleOwnedDigitalGrantType(grantType string) bool {
	return grantType == "theme" || grantType == "badge"
}

func isOpenOrderProtectedDigitalGrantType(grantType string) bool {
	return isSingleOwnedDigitalGrantType(grantType) || grantType == "membership"
}

func activeOwnedDigitalGrantEntitlementError(grantType string) error {
	if grantType == "badge" {
		return domain.ErrActiveBadgeEntitlementExists
	}
	return domain.ErrActiveThemeEntitlementExists
}

func pendingOwnedDigitalGrantOrderError(grantType string) error {
	if grantType == "membership" {
		return domain.ErrPendingMembershipOrderExists
	}
	if grantType == "badge" {
		return domain.ErrPendingBadgeOrderExists
	}
	return domain.ErrPendingThemeOrderExists
}

func duplicateOwnedDigitalGrantInOrderError(grantType string) error {
	if grantType == "badge" {
		return domain.ErrDuplicateBadgeGrantInOrder
	}
	return domain.ErrDuplicateThemeGrantInOrder
}

func createOrderInTx(ctx context.Context, tx pgx.Tx, order domain.Order) (domain.Order, bool, error) {
	var err error
	order, err = refreshOrderFromCurrentProducts(ctx, tx, order)
	if err != nil {
		return domain.Order{}, false, err
	}
	if existing, duplicate, err := prepareOwnedDigitalGrantOrderCreation(ctx, tx, order); err != nil {
		return domain.Order{}, false, err
	} else if duplicate {
		return existing, true, nil
	}
	if strings.TrimSpace(order.CouponCode) != "" {
		order, err = applyCouponToOrderInTx(ctx, tx, order)
		if err != nil {
			return domain.Order{}, false, err
		}
	} else {
		order.DiscountCredits = 0
		order.CouponID = 0
		order.CouponCode = ""
		order.TotalCredits = order.OriginalCredits
	}
	var insertedID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO mall_orders (
		  order_no, idempotency_key, user_id, original_credits, discount_credits, total_credits, coupon_id, coupon_code, status, receiver, phone, address, payment_method, paid_at, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, '', NULL, $13, $14
		)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING
		RETURNING id`,
		order.OrderNo,
		order.IdempotencyKey,
		order.UserID,
		order.OriginalCredits,
		order.DiscountCredits,
		order.TotalCredits,
		nullCouponID(order.CouponID),
		order.CouponCode,
		string(order.Status),
		order.Receiver,
		order.Phone,
		order.Address,
		order.CreatedAt,
		order.UpdatedAt,
	).Scan(&insertedID)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := getOrderByIdempotencyKey(ctx, tx, order.UserID, order.IdempotencyKey)
		if getErr != nil {
			return domain.Order{}, false, getErr
		}
		return idempotentExistingOrder(existing, order)
	}
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Order{}, false, domain.ErrDuplicateReference
		}
		return domain.Order{}, false, err
	}
	order.ID = insertedID
	if order.CouponID > 0 {
		if err := insertCouponUsage(ctx, tx, order); err != nil {
			return domain.Order{}, false, err
		}
	}

	for _, item := range stockDeductionItems(order.Items) {
		if err := decrementProductStock(ctx, tx, item.ProductID, item.Quantity, domain.StockChangeReasonOrderCreated, domain.StockReferenceOrder, order.ID, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", order.UserID), "下单锁定库存", order.CreatedAt); err != nil {
			return domain.Order{}, false, err
		}
	}
	for _, item := range order.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO mall_order_items (
			  order_id, product_id, sku, title, category, grant_type, grant_key, quantity, unit_price_credits, subtotal_credits
			) VALUES (
			  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			)`,
			order.ID,
			item.ProductID,
			item.SKU,
			item.Title,
			item.Category,
			item.GrantType,
			item.GrantKey,
			item.Quantity,
			item.UnitPriceCredits,
			item.SubtotalCredits,
		); err != nil {
			return domain.Order{}, false, err
		}
	}
	if err := insertOrderStatusLog(ctx, tx, order.ID, "", domain.OrderStatusPendingPayment, domain.OrderStatusReasonCreated, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", order.UserID), "", order.CreatedAt); err != nil {
		return domain.Order{}, false, err
	}
	saved, err := getOrder(ctx, tx, order.ID)
	return saved, false, err
}

func refreshOrderFromCurrentProducts(ctx context.Context, db queryer, order domain.Order) (domain.Order, error) {
	products, err := lockCurrentOrderProducts(ctx, db, order.Items)
	if err != nil {
		return domain.Order{}, err
	}
	refreshedItems := make([]domain.OrderItem, 0, len(order.Items))
	total := int64(0)
	requiresShipping := false
	for _, item := range order.Items {
		product, ok := products[item.ProductID]
		if !ok {
			return domain.Order{}, domain.ErrProductNotFound
		}
		if product.Status != domain.ProductStatusActive {
			return domain.Order{}, domain.ErrProductUnavailable
		}
		if item.Quantity <= 0 {
			return domain.Order{}, domain.ErrInvalidOrderState
		}
		if int64(item.Quantity) > product.Stock {
			return domain.Order{}, domain.ErrInsufficientStock
		}
		if productRequiresShippingForFulfillment(product) {
			requiresShipping = true
		}
		subtotal, nextTotal, err := addPersistedOrderSubtotal(total, product.PriceCredits, item.Quantity)
		if err != nil {
			return domain.Order{}, err
		}
		total = nextTotal
		refreshed := orderItemForCurrentProduct(product, item.Quantity)
		refreshed.SubtotalCredits = subtotal
		refreshedItems = append(refreshedItems, refreshed)
	}
	if err := ensureSingleOwnedDigitalGrantPerOrder(refreshedItems); err != nil {
		return domain.Order{}, err
	}
	if requiresShipping && (strings.TrimSpace(order.Receiver) == "" || strings.TrimSpace(order.Phone) == "" || strings.TrimSpace(order.Address) == "") {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	order.Items = refreshedItems
	order.OriginalCredits = total
	order.DiscountCredits = 0
	order.CouponID = 0
	order.TotalCredits = total
	return order, nil
}

func lockCurrentOrderProducts(ctx context.Context, db queryer, items []domain.OrderItem) (map[int64]domain.Product, error) {
	if len(items) == 0 {
		return nil, domain.ErrInvalidOrderState
	}
	productIDs := make([]int64, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.ProductID <= 0 {
			return nil, domain.ErrInvalidOrderState
		}
		if _, ok := seen[item.ProductID]; ok {
			continue
		}
		seen[item.ProductID] = struct{}{}
		productIDs = append(productIDs, item.ProductID)
	}
	sort.Slice(productIDs, func(i, j int) bool {
		return productIDs[i] < productIDs[j]
	})
	products := make(map[int64]domain.Product, len(productIDs))
	for _, productID := range productIDs {
		product, err := scanProduct(db.QueryRow(ctx, selectProductSQL()+` WHERE id = $1 FOR UPDATE`, productID))
		if err != nil {
			return nil, err
		}
		products[productID] = product
	}
	return products, nil
}

func orderItemForCurrentProduct(product domain.Product, quantity int32) domain.OrderItem {
	product = normalizeProductGrantContract(product)
	return domain.OrderItem{
		ProductID:        product.ID,
		SKU:              product.SKU,
		Title:            product.Title,
		Category:         strings.TrimSpace(product.Category),
		GrantType:        product.GrantType,
		GrantKey:         product.GrantKey,
		Quantity:         quantity,
		UnitPriceCredits: product.PriceCredits,
	}
}

func addPersistedOrderSubtotal(total, unitPrice int64, quantity int32) (int64, int64, error) {
	if unitPrice < 0 {
		return 0, 0, errors.New("product price must be non-negative")
	}
	if quantity <= 0 {
		return 0, 0, domain.ErrInvalidOrderState
	}
	count := int64(quantity)
	if unitPrice > math.MaxInt64/count {
		return 0, 0, errors.New("order amount is too large")
	}
	subtotal := unitPrice * count
	if total > math.MaxInt64-subtotal {
		return 0, 0, errors.New("order amount is too large")
	}
	return subtotal, total + subtotal, nil
}

func idempotentExistingOrder(existing domain.Order, requested domain.Order) (domain.Order, bool, error) {
	if !domain.OrderMatchesIdempotencyRequest(existing, requested) {
		return domain.Order{}, false, domain.ErrDuplicateReference
	}
	return existing, true, nil
}

func lockCouponForClaim(ctx context.Context, tx pgx.Tx, couponID int64) (domain.Coupon, error) {
	var coupon domain.Coupon
	var status string
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	err := tx.QueryRow(ctx, `
		SELECT c.id, c.code, c.name, c.description, c.discount_credits, c.min_order_credits, c.total_quota, c.per_user_limit,
		  (
		    SELECT COUNT(*)
		    FROM mall_coupon_usages u
		    WHERE u.coupon_id = c.id
		      AND u.status <> $2
		  ) AS claimed_count,
		  (
		    SELECT COUNT(*)
		    FROM mall_coupon_usages u
		    WHERE u.coupon_id = c.id
		      AND u.status = $3
		  ) AS used_count,
		  c.status, c.starts_at, c.ends_at, c.created_at, c.updated_at
		FROM mall_coupons c
		WHERE c.id = $1
		FOR UPDATE`,
		couponID,
		string(domain.CouponUsageStatusReleased),
		string(domain.CouponUsageStatusUsed),
	).Scan(
		&coupon.ID,
		&coupon.Code,
		&coupon.Name,
		&coupon.Description,
		&coupon.DiscountCredits,
		&coupon.MinOrderCredits,
		&coupon.TotalQuota,
		&coupon.PerUserLimit,
		&coupon.ClaimedCount,
		&coupon.UsedCount,
		&status,
		&startsAt,
		&endsAt,
		&coupon.CreatedAt,
		&coupon.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Coupon{}, domain.ErrCouponNotFound
		}
		return domain.Coupon{}, err
	}
	coupon.Status = domain.CouponStatus(status)
	if startsAt.Valid {
		coupon.StartsAt = &startsAt.Time
	}
	if endsAt.Valid {
		coupon.EndsAt = &endsAt.Time
	}
	return coupon, nil
}

func couponClaimable(coupon domain.Coupon, now time.Time) bool {
	if coupon.Status != domain.CouponStatusActive || coupon.DiscountCredits <= 0 {
		return false
	}
	if coupon.StartsAt != nil && coupon.StartsAt.After(now) {
		return false
	}
	if coupon.EndsAt != nil && !coupon.EndsAt.After(now) {
		return false
	}
	return true
}

func applyCouponToOrderInTx(ctx context.Context, tx queryer, order domain.Order) (domain.Order, error) {
	code := strings.ToUpper(strings.TrimSpace(order.CouponCode))
	if code == "" {
		return order, nil
	}
	var coupon domain.Coupon
	var status string
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	if err := tx.QueryRow(ctx, `
		SELECT id, code, name, discount_credits, min_order_credits, total_quota, per_user_limit, status, starts_at, ends_at
		FROM mall_coupons
		WHERE lower(code) = lower($1)
		FOR UPDATE`,
		code,
	).Scan(
		&coupon.ID,
		&coupon.Code,
		&coupon.Name,
		&coupon.DiscountCredits,
		&coupon.MinOrderCredits,
		&coupon.TotalQuota,
		&coupon.PerUserLimit,
		&status,
		&startsAt,
		&endsAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrCouponNotFound
		}
		return domain.Order{}, err
	}
	coupon.Status = domain.CouponStatus(status)
	if startsAt.Valid {
		coupon.StartsAt = &startsAt.Time
	}
	if endsAt.Valid {
		coupon.EndsAt = &endsAt.Time
	}
	now := order.CreatedAt
	if coupon.Status != domain.CouponStatusActive ||
		(coupon.StartsAt != nil && coupon.StartsAt.After(now)) ||
		(coupon.EndsAt != nil && !coupon.EndsAt.After(now)) ||
		order.OriginalCredits < coupon.MinOrderCredits ||
		coupon.DiscountCredits <= 0 {
		return domain.Order{}, domain.ErrCouponUnavailable
	}
	var claimedUsageID int64
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM mall_coupon_usages
		WHERE coupon_id = $1
		  AND user_id = $2::BIGINT
		  AND status = $3
		ORDER BY created_at ASC, id ASC
		LIMIT 1
		FOR UPDATE`,
		coupon.ID,
		order.UserID,
		string(domain.CouponUsageStatusClaimed),
	).Scan(&claimedUsageID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, err
	}
	hasClaimedUsage := claimedUsageID > 0
	var totalUses int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_coupon_usages
		WHERE coupon_id = $1
		  AND status <> $2`,
		coupon.ID,
		string(domain.CouponUsageStatusReleased),
	).Scan(&totalUses); err != nil {
		return domain.Order{}, err
	}
	if coupon.TotalQuota > 0 && totalUses >= coupon.TotalQuota && !hasClaimedUsage {
		return domain.Order{}, domain.ErrCouponUnavailable
	}
	var userUses int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_coupon_usages
		WHERE coupon_id = $1
		  AND user_id = $2::BIGINT
		  AND status <> $3`,
		coupon.ID,
		order.UserID,
		string(domain.CouponUsageStatusReleased),
	).Scan(&userUses); err != nil {
		return domain.Order{}, err
	}
	if coupon.PerUserLimit > 0 && userUses >= coupon.PerUserLimit && !hasClaimedUsage {
		return domain.Order{}, domain.ErrCouponUnavailable
	}
	discount := coupon.DiscountCredits
	if discount > order.OriginalCredits {
		discount = order.OriginalCredits
	}
	order.CouponID = coupon.ID
	order.CouponCode = coupon.Code
	order.CouponUsageID = claimedUsageID
	order.DiscountCredits = discount
	order.TotalCredits = order.OriginalCredits - discount
	return order, nil
}

func insertCouponUsage(ctx context.Context, db queryer, order domain.Order) error {
	if order.CouponUsageID > 0 {
		tag, err := db.Exec(ctx, `
			UPDATE mall_coupon_usages
			SET order_id = $2,
			    status = $3,
			    discount_credits = $4,
			    updated_at = $5
			WHERE id = $1
			  AND user_id = $6::BIGINT
			  AND coupon_id = $7
			  AND status = $8
			  AND order_id IS NULL`,
			order.CouponUsageID,
			order.ID,
			string(domain.CouponUsageStatusReserved),
			order.DiscountCredits,
			order.CreatedAt,
			order.UserID,
			order.CouponID,
			string(domain.CouponUsageStatusClaimed),
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrCouponUnavailable
		}
		return nil
	}
	tag, err := db.Exec(ctx, `
		INSERT INTO mall_coupon_usages (
		  coupon_id, code, user_id, order_id, status, discount_credits, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, $7
		)
		ON CONFLICT (order_id) DO NOTHING`,
		order.CouponID,
		order.CouponCode,
		order.UserID,
		order.ID,
		string(domain.CouponUsageStatusReserved),
		order.DiscountCredits,
		order.CreatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCouponUnavailable
	}
	return nil
}

func markCouponUsageUsed(ctx context.Context, db queryer, orderID int64, usedAt time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE mall_coupon_usages
		SET status = $2,
		    used_at = $3,
		    updated_at = $3
		WHERE order_id = $1
		  AND status = $4`,
		orderID,
		string(domain.CouponUsageStatusUsed),
		usedAt,
		string(domain.CouponUsageStatusReserved),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCouponUnavailable
	}
	return nil
}

func releaseCouponUsage(ctx context.Context, db queryer, orderID int64, releasedAt time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE mall_coupon_usages
		SET status = $2,
		    released_at = $3,
		    updated_at = $3
		WHERE order_id = $1
		  AND status = $4`,
		orderID,
		string(domain.CouponUsageStatusReleased),
		releasedAt,
		string(domain.CouponUsageStatusReserved),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCouponUnavailable
	}
	return nil
}

func nullCouponID(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (r *PostgresRepository) GetOrder(ctx context.Context, orderID int64) (domain.Order, error) {
	return getOrder(ctx, r.pool, orderID)
}

func (r *PostgresRepository) GetOrderByIdempotencyKey(ctx context.Context, userID int64, idempotencyKey string) (domain.Order, error) {
	return getOrderByIdempotencyKey(ctx, r.pool, userID, idempotencyKey)
}

func (r *PostgresRepository) OpenDigitalGrantOrderExists(ctx context.Context, userID int64, grantType, grantKey string) (bool, error) {
	return openDigitalGrantOrderExists(ctx, r.pool, userID, grantType, grantKey)
}

func (r *PostgresRepository) ListOrdersByUser(ctx context.Context, query domain.OrderListQuery) ([]domain.Order, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	status := domain.NormalizeOrderStatus(query.Status)
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM mall_orders WHERE user_id = $1::BIGINT AND ($2 = '' OR status = $2)`, query.UserID, string(status)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectOrderSQL()+`
		WHERE user_id = $1::BIGINT
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`,
		query.UserID,
		string(status),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanOrders(ctx, r.pool, rows, total)
}

func (r *PostgresRepository) ListReviewableOrders(ctx context.Context, query domain.OrderListQuery, productID int64) ([]domain.Order, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	total, err := r.countReviewableOrders(ctx, query.UserID, productID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectOrderSQL()+`
		WHERE user_id = $1::BIGINT
		  AND status = $2
		  AND EXISTS (
		    SELECT 1
		    FROM mall_order_items oi
		    WHERE oi.order_id = mall_orders.id
		      AND oi.product_id = $3::BIGINT
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM mall_refund_requests rr
		    WHERE rr.order_id = mall_orders.id
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM mall_product_reviews r
		    WHERE r.order_id = mall_orders.id
		      AND r.product_id = $3::BIGINT
		      AND r.user_id = $1::BIGINT
		  )
		ORDER BY completed_at DESC NULLS LAST, created_at DESC, id DESC
		LIMIT $4 OFFSET $5`,
		query.UserID,
		string(domain.OrderStatusCompleted),
		productID,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanOrders(ctx, r.pool, rows, total)
}

func (r *PostgresRepository) AdminListOrders(ctx context.Context, query domain.OrderListQuery) ([]domain.Order, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	keyword := strings.TrimSpace(query.Keyword)
	status := domain.NormalizeOrderStatus(query.Status)
	total, err := r.countOrders(ctx, query.UserID, keyword, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectOrderSQL()+`
		WHERE ($1::BIGINT = 0 OR user_id = $1::BIGINT)
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR id::TEXT = $3 OR order_no ILIKE '%' || $3 || '%' OR idempotency_key ILIKE '%' || $3 || '%' OR coupon_code ILIKE '%' || $3 || '%' OR receiver ILIKE '%' || $3 || '%' OR phone ILIKE '%' || $3 || '%')
		ORDER BY created_at DESC, id DESC
		LIMIT $4 OFFSET $5`,
		query.UserID,
		string(status),
		keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanOrders(ctx, r.pool, rows, total)
}

func (r *PostgresRepository) ListDigitalEntitlements(ctx context.Context, query domain.DigitalEntitlementListQuery) ([]domain.DigitalEntitlement, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	status := normalizeDigitalEntitlementStatus(query.Status)
	grantType := strings.ToLower(strings.TrimSpace(query.GrantType))
	grantKey := strings.ToLower(strings.TrimSpace(query.GrantKey))
	keyword := strings.TrimSpace(query.Keyword)
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_digital_entitlements de
		JOIN mall_orders o ON o.id = de.order_id
		WHERE ($1::BIGINT = 0 OR de.user_id = $1::BIGINT)`+digitalEntitlementListGrantCondition("de", 2, 3)+digitalEntitlementListStatusCondition("de", status)+digitalEntitlementListKeywordCondition(4),
		query.UserID,
		grantType,
		grantKey,
		keyword,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT de.id,
		       de.order_id,
		       o.order_no,
		       de.user_id,
		       de.product_id,
		       de.sku,
		       de.title,
		       de.quantity,
		       de.fulfillment_code,
		       COALESCE(de.grant_type, ''),
		       COALESCE(de.grant_key, ''),
		       de.issued_at,
		       de.expires_at,
		       COALESCE(de.status, ''),
		       de.revoked_at,
		       de.refund_id,
		       COALESCE(de.revoked_by, ''),
		       COALESCE(de.revoke_reason, '')
		FROM mall_digital_entitlements de
		JOIN mall_orders o ON o.id = de.order_id
		WHERE ($1::BIGINT = 0 OR de.user_id = $1::BIGINT)`+digitalEntitlementListGrantCondition("de", 2, 3)+digitalEntitlementListStatusCondition("de", status)+digitalEntitlementListKeywordCondition(4)+`
		ORDER BY de.issued_at DESC, de.id DESC
		LIMIT $5 OFFSET $6`,
		query.UserID,
		grantType,
		grantKey,
		keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.DigitalEntitlement, 0)
	for rows.Next() {
		item, err := scanDigitalEntitlement(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *PostgresRepository) GetDigitalEntitlement(ctx context.Context, entitlementID int64) (domain.DigitalEntitlement, error) {
	item, err := scanDigitalEntitlement(r.pool.QueryRow(ctx, `
		SELECT de.id,
		       de.order_id,
		       o.order_no,
		       de.user_id,
		       de.product_id,
		       de.sku,
		       de.title,
		       de.quantity,
		       de.fulfillment_code,
		       COALESCE(de.grant_type, ''),
		       COALESCE(de.grant_key, ''),
		       de.issued_at,
		       de.expires_at,
		       COALESCE(de.status, ''),
		       de.revoked_at,
		       de.refund_id,
		       COALESCE(de.revoked_by, ''),
		       COALESCE(de.revoke_reason, '')
		FROM mall_digital_entitlements de
		JOIN mall_orders o ON o.id = de.order_id
		WHERE de.id = $1`,
		entitlementID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DigitalEntitlement{}, domain.ErrDigitalEntitlementNotFound
		}
		return domain.DigitalEntitlement{}, err
	}
	return item, nil
}

func (r *PostgresRepository) AdminRevokeDigitalEntitlement(ctx context.Context, entitlementID int64, operatorID string, reason string, revokedAt time.Time, event domain.OutboxEvent) (domain.DigitalEntitlement, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DigitalEntitlement{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, err := scanDigitalEntitlement(tx.QueryRow(ctx, `
		SELECT de.id,
		       de.order_id,
		       o.order_no,
		       de.user_id,
		       de.product_id,
		       de.sku,
		       de.title,
		       de.quantity,
		       de.fulfillment_code,
		       COALESCE(de.grant_type, ''),
		       COALESCE(de.grant_key, ''),
		       de.issued_at,
		       de.expires_at,
		       COALESCE(de.status, ''),
		       de.revoked_at,
		       de.refund_id,
		       COALESCE(de.revoked_by, ''),
		       COALESCE(de.revoke_reason, '')
		FROM mall_digital_entitlements de
		JOIN mall_orders o ON o.id = de.order_id
		WHERE de.id = $1
		FOR UPDATE OF de`,
		entitlementID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DigitalEntitlement{}, domain.ErrDigitalEntitlementNotFound
		}
		return domain.DigitalEntitlement{}, err
	}
	if item.Status == domain.DigitalEntitlementStatusRevoked || item.RevokedAt != nil {
		item.Status = domain.DigitalEntitlementStatusRevoked
		if err := tx.Commit(ctx); err != nil {
			return domain.DigitalEntitlement{}, err
		}
		return item, nil
	}
	if err := markDigitalEntitlementRevoked(ctx, tx, entitlementID, operatorID, reason, revokedAt); err != nil {
		return domain.DigitalEntitlement{}, err
	}
	item.Status = domain.DigitalEntitlementStatusRevoked
	item.RevokedAt = &revokedAt
	item.RevokedBy = operatorID
	item.RevokeReason = reason
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return domain.DigitalEntitlement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DigitalEntitlement{}, err
	}
	return item, nil
}

func (r *PostgresRepository) BeginOrderPayment(ctx context.Context, orderID, userID int64, paymentMethod, idempotencyKey string, now time.Time) (domain.Order, domain.Payment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	if order.UserID != userID {
		return domain.Order{}, domain.Payment{}, domain.ErrOrderOwnerMismatch
	}
	if order.Status == domain.OrderStatusPaid || order.Status == domain.OrderStatusCompleted {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, domain.Payment{}, err
		}
		return order, domain.Payment{}, nil
	}
	if order.Status != domain.OrderStatusPendingPayment && order.Status != domain.OrderStatusPaying {
		return domain.Order{}, domain.Payment{}, domain.ErrInvalidOrderState
	}
	if err := ensureNoOtherOpenDigitalGrantOrdersForPayment(ctx, tx, order); err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	if order.Status == domain.OrderStatusPaying {
		payment, err := getPaymentByProviderKey(ctx, tx, paymentMethod, idempotencyKey)
		if err != nil {
			return domain.Order{}, domain.Payment{}, err
		}
		if payment.OrderID != order.ID || payment.UserID != userID || payment.Status != domain.PaymentStatusPending {
			return domain.Order{}, domain.Payment{}, domain.ErrInvalidOrderState
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, domain.Payment{}, err
		}
		return order, payment, nil
	}

	payment, err := insertPendingPayment(ctx, tx, order, paymentMethod, idempotencyKey, now)
	if err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE mall_orders
		SET status = $3,
		    payment_method = $4,
		    updated_at = $5
		WHERE id = $1
		  AND user_id = $2::BIGINT
		  AND status = $6`,
		order.ID,
		userID,
		string(domain.OrderStatusPaying),
		paymentMethod,
		now,
		string(domain.OrderStatusPendingPayment),
	)
	if err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.Payment{}, domain.ErrInvalidOrderState
	}
	if err := insertOrderStatusLog(ctx, tx, order.ID, domain.OrderStatusPendingPayment, domain.OrderStatusPaying, domain.OrderStatusReasonPaying, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", userID), "", now); err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	paying, err := getOrder(ctx, tx, order.ID)
	if err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, domain.Payment{}, err
	}
	return paying, payment, nil
}

func (r *PostgresRepository) CompleteOrderPayment(ctx context.Context, orderID, userID, paymentID int64, paidAt time.Time, event domain.OutboxEvent) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	payment, err := getPaymentForUpdate(ctx, tx, paymentID)
	if err != nil {
		return domain.Order{}, err
	}
	if err := validatePaymentForOrder(payment, order, userID); err != nil {
		return domain.Order{}, err
	}
	if !canCompleteOrderPayment(order.Status, payment.Status) {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	if order.Status == domain.OrderStatusPaid || order.Status == domain.OrderStatusCompleted {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, err
		}
		return order, nil
	}
	if order.Status != domain.OrderStatusPaying {
		return domain.Order{}, domain.ErrInvalidOrderState
	}

	completeDigitalOrder := isDigitalOnlyOrder(order)
	issueEntitlements := orderHasDigitalEntitlementItems(order)
	nextStatus := domain.OrderStatusPaid
	if completeDigitalOrder {
		nextStatus = domain.OrderStatusCompleted
	}
	tag, err := tx.Exec(ctx, `
		UPDATE mall_orders
		SET status = $3,
		    paid_at = $4,
		    completed_at = CASE WHEN $5 THEN $4 ELSE completed_at END,
		    updated_at = $4
		WHERE id = $1
		  AND user_id = $2::BIGINT
		  AND status = $6`,
		orderID,
		userID,
		string(nextStatus),
		paidAt,
		completeDigitalOrder,
		string(domain.OrderStatusPaying),
	)
	if err != nil {
		return domain.Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	if err := markPaymentSucceeded(ctx, tx, paymentID, orderID, userID, order.TotalCredits, paidAt); err != nil {
		return domain.Order{}, err
	}
	if issueEntitlements {
		if err := issueDigitalEntitlements(ctx, tx, order, paidAt); err != nil {
			return domain.Order{}, err
		}
		if err := loadDigitalEntitlements(ctx, tx, &order); err != nil {
			return domain.Order{}, err
		}
		event, err = withDigitalEntitlements(event, order.DigitalEntitlements)
		if err != nil {
			return domain.Order{}, err
		}
	}
	if order.CouponID > 0 {
		if err := markCouponUsageUsed(ctx, tx, orderID, paidAt); err != nil {
			return domain.Order{}, err
		}
	}
	for _, item := range order.Items {
		if err := incrementProductSales(ctx, tx, item.ProductID, item.Quantity, paidAt); err != nil {
			return domain.Order{}, err
		}
	}
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return domain.Order{}, err
	}
	reason := domain.OrderStatusReasonPaid
	note := ""
	if completeDigitalOrder {
		reason = domain.OrderStatusReasonCompleted
	}
	if issueEntitlements {
		note = "数字权益已发放"
	}
	if err := insertOrderStatusLog(ctx, tx, orderID, domain.OrderStatusPaying, nextStatus, reason, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", userID), note, paidAt); err != nil {
		return domain.Order{}, err
	}
	paid, err := getOrder(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return paid, nil
}

func (r *PostgresRepository) FailOrderPayment(ctx context.Context, orderID, userID, paymentID int64, reason string, failedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return domain.ErrOrderOwnerMismatch
	}
	payment, err := getPaymentForUpdate(ctx, tx, paymentID)
	if err != nil {
		return err
	}
	if err := validatePaymentForOrder(payment, order, userID); err != nil {
		return err
	}
	if order.Status != domain.OrderStatusPaying {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return nil
	}
	if err := markPaymentFailed(ctx, tx, paymentID, orderID, userID, order.TotalCredits, reason, failedAt); err != nil {
		return err
	}
	if err := reopenOrderAfterPaymentFailure(ctx, tx, orderID, userID, failedAt); err != nil {
		return err
	}
	if err := insertOrderStatusLog(ctx, tx, orderID, domain.OrderStatusPaying, domain.OrderStatusPendingPayment, domain.OrderStatusReasonPaymentFailed, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", userID), reason, failedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListStalePayingOrders(ctx context.Context, startedBefore time.Time, limit int) ([]domain.PayingOrderPayment, error) {
	if limit <= 0 {
		limit = domain.DefaultListLimit
	}
	if limit > domain.MaxListLimit {
		limit = domain.MaxListLimit
	}
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.user_id, p.id, p.provider, p.idempotency_key
		FROM mall_orders o
		JOIN LATERAL (
		  SELECT id, provider, idempotency_key, created_at
		  FROM mall_payments
		  WHERE order_id = o.id
		    AND status = $2
		  ORDER BY created_at ASC, id ASC
		  LIMIT 1
		) p ON TRUE
		WHERE o.status = $1
		  AND p.created_at <= $3
		ORDER BY p.created_at ASC, o.id ASC
		LIMIT $4`,
		string(domain.OrderStatusPaying),
		string(domain.PaymentStatusPending),
		startedBefore,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PayingOrderPayment, 0, limit)
	for rows.Next() {
		var item domain.PayingOrderPayment
		if err := rows.Scan(&item.OrderID, &item.UserID, &item.PaymentID, &item.Provider, &item.IdempotencyKey); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CancelOrder(ctx context.Context, orderID, userID int64, canceledAt time.Time) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	if order.Status == domain.OrderStatusCanceled {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, err
		}
		return order, nil
	}
	if order.Status != domain.OrderStatusPendingPayment {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	tag, err := tx.Exec(ctx, `
		UPDATE mall_orders
		SET status = $3,
		    updated_at = $4
		WHERE id = $1
		  AND user_id = $2::BIGINT
		  AND status = $5`,
		orderID,
		userID,
		string(domain.OrderStatusCanceled),
		canceledAt,
		string(domain.OrderStatusPendingPayment),
	)
	if err != nil {
		return domain.Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	for _, item := range order.Items {
		if err := releaseProductStock(ctx, tx, item.ProductID, item.Quantity, domain.StockChangeReasonOrderCanceled, domain.StockReferenceOrder, order.ID, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", userID), "用户取消订单释放库存", canceledAt); err != nil {
			return domain.Order{}, err
		}
	}
	if order.CouponID > 0 {
		if err := releaseCouponUsage(ctx, tx, orderID, canceledAt); err != nil {
			return domain.Order{}, err
		}
	}
	if err := insertOrderStatusLog(ctx, tx, orderID, domain.OrderStatusPendingPayment, domain.OrderStatusCanceled, domain.OrderStatusReasonCanceled, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", userID), "", canceledAt); err != nil {
		return domain.Order{}, err
	}
	canceled, err := getOrder(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return canceled, nil
}

func (r *PostgresRepository) ConfirmOrder(ctx context.Context, orderID, userID int64, completedAt time.Time, event domain.OutboxEvent) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	if order.Status == domain.OrderStatusCompleted {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, err
		}
		return order, nil
	}
	if order.Status != domain.OrderStatusShipped {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	refundRequested, err := orderHasRefundRequest(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if refundRequested {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	tag, err := tx.Exec(ctx, `
		UPDATE mall_orders
		SET status = $3,
		    completed_at = COALESCE(completed_at, $4),
		    updated_at = $4
		WHERE id = $1
		  AND user_id = $2::BIGINT
		  AND status = $5`,
		orderID,
		userID,
		string(domain.OrderStatusCompleted),
		completedAt,
		string(domain.OrderStatusShipped),
	)
	if err != nil {
		return domain.Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	if err := insertOrderStatusLog(ctx, tx, orderID, domain.OrderStatusShipped, domain.OrderStatusCompleted, domain.OrderStatusReasonCompleted, domain.OrderStatusOperatorUser, fmt.Sprintf("%d", userID), "用户确认收货", completedAt); err != nil {
		return domain.Order{}, err
	}
	completed, err := getOrder(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return completed, nil
}

func (r *PostgresRepository) CloseExpiredOrder(ctx context.Context, orderID, userID int64, expireBefore time.Time, closedAt time.Time) (domain.Order, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, false, err
	}
	if order.UserID != userID {
		return domain.Order{}, false, domain.ErrOrderOwnerMismatch
	}
	if !isOrderExpiredForClose(order, expireBefore) {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, false, err
		}
		return order, false, nil
	}
	closed, ok, err := closeExpiredOrderInTx(ctx, tx, order, closedAt)
	if err != nil {
		return domain.Order{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, false, err
	}
	return closed, ok, nil
}

func (r *PostgresRepository) CloseExpiredOrders(ctx context.Context, expireBefore time.Time, limit int, closedAt time.Time) ([]domain.Order, error) {
	if limit <= 0 {
		limit = domain.DefaultListLimit
	}
	if limit > domain.MaxListLimit {
		limit = domain.MaxListLimit
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM mall_orders
		WHERE status = $1
		  AND created_at <= $2
		ORDER BY created_at ASC, id ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED`,
		string(domain.OrderStatusPendingPayment),
		expireBefore,
		limit,
	)
	if err != nil {
		return nil, err
	}
	orderIDs := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		orderIDs = append(orderIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	closedOrders := make([]domain.Order, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order, err := getOrder(ctx, tx, orderID)
		if err != nil {
			return nil, err
		}
		closed, ok, err := closeExpiredOrderInTx(ctx, tx, order, closedAt)
		if err != nil {
			return nil, err
		}
		if ok {
			closedOrders = append(closedOrders, closed)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return closedOrders, nil
}

func isOrderExpiredForClose(order domain.Order, expireBefore time.Time) bool {
	switch order.Status {
	case domain.OrderStatusPendingPayment:
		return !order.CreatedAt.After(expireBefore)
	default:
		return false
	}
}

func closeExpiredOrderInTx(ctx context.Context, tx pgx.Tx, order domain.Order, closedAt time.Time) (domain.Order, bool, error) {
	if order.Status != domain.OrderStatusPendingPayment {
		return order, false, nil
	}
	tag, err := tx.Exec(ctx, `
		UPDATE mall_orders
		SET status = $2,
		    updated_at = $3
		WHERE id = $1
		  AND status = $4`,
		order.ID,
		string(domain.OrderStatusClosed),
		closedAt,
		string(domain.OrderStatusPendingPayment),
	)
	if err != nil {
		return domain.Order{}, false, err
	}
	if tag.RowsAffected() == 0 {
		return order, false, nil
	}
	for _, item := range order.Items {
		if err := releaseProductStock(ctx, tx, item.ProductID, item.Quantity, domain.StockChangeReasonOrderExpired, domain.StockReferenceOrder, order.ID, domain.OrderStatusOperatorAdmin, "system", "订单超时释放库存", closedAt); err != nil {
			return domain.Order{}, false, err
		}
	}
	if order.CouponID > 0 {
		if err := releaseCouponUsage(ctx, tx, order.ID, closedAt); err != nil {
			return domain.Order{}, false, err
		}
	}
	if err := insertOrderStatusLog(ctx, tx, order.ID, order.Status, domain.OrderStatusClosed, domain.OrderStatusReasonExpired, domain.OrderStatusOperatorAdmin, "system", "订单超时未支付，系统自动关闭", closedAt); err != nil {
		return domain.Order{}, false, err
	}
	closed, err := getOrder(ctx, tx, order.ID)
	if err != nil {
		return domain.Order{}, false, err
	}
	return closed, true, nil
}

func (r *PostgresRepository) AdminUpdateOrderStatus(ctx context.Context, orderID int64, nextStatus domain.OrderStatus, operatorID string, fulfillment domain.OrderFulfillment, note string, changedAt time.Time, event domain.OutboxEvent) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.Status == nextStatus {
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, err
		}
		return order, nil
	}
	if !isAllowedAdminOrderTransition(order.Status, nextStatus) {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	shippingCarrier := strings.TrimSpace(fulfillment.ShippingCarrier)
	trackingNo := strings.TrimSpace(fulfillment.TrackingNo)
	if err := validatePersistedAdminOrderFulfillment(order, nextStatus, domain.OrderFulfillment{ShippingCarrier: shippingCarrier, TrackingNo: trackingNo}, note); err != nil {
		return domain.Order{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE mall_orders
		SET status = $2,
		    shipping_carrier = CASE WHEN $5 <> '' THEN $5 ELSE shipping_carrier END,
		    tracking_no = CASE WHEN $6 <> '' THEN $6 ELSE tracking_no END,
		    shipped_at = CASE WHEN $2 = $7 AND shipped_at IS NULL THEN $3 ELSE shipped_at END,
		    completed_at = CASE WHEN $2 = $8 AND completed_at IS NULL THEN $3 ELSE completed_at END,
		    updated_at = $3
		WHERE id = $1
		  AND status = $4`,
		orderID,
		string(nextStatus),
		changedAt,
		string(order.Status),
		shippingCarrier,
		trackingNo,
		string(domain.OrderStatusShipped),
		string(domain.OrderStatusCompleted),
	)
	if err != nil {
		return domain.Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	if err := insertOrderStatusLog(ctx, tx, orderID, order.Status, nextStatus, adminOrderTransitionReason(nextStatus), domain.OrderStatusOperatorAdmin, operatorID, note, changedAt); err != nil {
		return domain.Order{}, err
	}
	updated, err := getOrder(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return domain.Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return updated, nil
}

func (r *PostgresRepository) ListOrderStatusLogs(ctx context.Context, orderID int64) ([]domain.OrderStatusLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, order_id, from_status, to_status, reason, operator_type, operator_id, note, created_at
		FROM mall_order_status_logs
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := make([]domain.OrderStatusLog, 0)
	for rows.Next() {
		log, err := scanOrderStatusLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (r *PostgresRepository) ListOrderPayments(ctx context.Context, orderID int64) ([]domain.Payment, error) {
	rows, err := r.pool.Query(ctx, selectPaymentSQL()+`
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	payments := make([]domain.Payment, 0)
	for rows.Next() {
		payment, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return payments, rows.Err()
}

func (r *PostgresRepository) ListCartItems(ctx context.Context, userID int64) ([]domain.CartItem, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_cart_items ci
		JOIN mall_products p ON p.id = ci.product_id
		WHERE ci.user_id = $1::BIGINT
		  AND `+cartActiveProductCondition("p"),
		userID,
		string(domain.ProductStatusActive),
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT ci.user_id, ci.quantity, ci.created_at, ci.updated_at, `+prefixedProductColumns("p")+`
		FROM mall_cart_items ci
		JOIN mall_products p ON p.id = ci.product_id
		WHERE ci.user_id = $1::BIGINT
		  AND `+cartActiveProductCondition("p")+`
		ORDER BY ci.updated_at DESC, ci.created_at DESC, ci.product_id DESC`,
		userID,
		string(domain.ProductStatusActive),
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanCartItems(rows, total)
}

func cartActiveProductCondition(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return "status = $2"
	}
	return fmt.Sprintf("%s.status = $2", alias)
}

func (r *PostgresRepository) SetCartItem(ctx context.Context, userID int64, productID int64, quantity int32, updatedAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUserCart(ctx, tx, userID); err != nil {
		return err
	}
	product, err := scanProduct(tx.QueryRow(ctx, selectProductSQL()+` WHERE id = $1`, productID))
	if err != nil {
		return err
	}
	if product.Status != domain.ProductStatusActive {
		return domain.ErrProductUnavailable
	}
	if int64(quantity) > product.Stock {
		return domain.ErrInsufficientStock
	}
	if err := validateCartOwnedDigitalGrantWrite(ctx, tx, userID, productID, product, quantity); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO mall_cart_items (user_id, product_id, quantity, created_at, updated_at)
		VALUES ($1::BIGINT, $2, $3, $4, $4)
		ON CONFLICT (user_id, product_id)
		DO UPDATE SET quantity = EXCLUDED.quantity,
		              updated_at = EXCLUDED.updated_at`,
		userID,
		productID,
		quantity,
		updatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validateCartOwnedDigitalGrantWrite(ctx context.Context, db queryer, userID int64, productID int64, product domain.Product, quantity int32) error {
	candidate := cartOrderItemForProduct(product, quantity)
	candidateItems := []domain.OrderItem{candidate}
	if err := ensureSingleOwnedDigitalGrantPerOrder(candidateItems); err != nil {
		return err
	}

	openOrderGrants := openOrderProtectedDigitalGrantsForOrderItems(candidateItems)
	if len(openOrderGrants) == 0 {
		return nil
	}
	for _, grant := range openOrderGrants {
		if err := lockOwnedDigitalGrant(ctx, db, userID, grant.grantType, grant.grantKey); err != nil {
			return err
		}
	}
	for _, grant := range ownedDigitalGrantsForOrderItems(candidateItems) {
		active, err := activeDigitalEntitlementExists(ctx, db, userID, grant.grantType, grant.grantKey)
		if err != nil {
			return err
		}
		if active {
			return activeOwnedDigitalGrantEntitlementError(grant.grantType)
		}
	}
	for _, grant := range openOrderGrants {
		openOrder, err := openDigitalGrantOrderExists(ctx, db, userID, grant.grantType, grant.grantKey)
		if err != nil {
			return err
		}
		if openOrder {
			return pendingOwnedDigitalGrantOrderError(grant.grantType)
		}
	}

	existingItems, err := cartOrderItemsForGrantValidation(ctx, db, userID, productID)
	if err != nil {
		return err
	}
	return validateCartOwnedDigitalGrantComposition(existingItems, candidate)
}

func validateCartOwnedDigitalGrantComposition(existingItems []domain.OrderItem, candidate domain.OrderItem) error {
	candidateItems := []domain.OrderItem{candidate}
	if err := ensureSingleOwnedDigitalGrantPerOrder(candidateItems); err != nil {
		return err
	}
	if len(existingItems) == 0 {
		return nil
	}
	items := make([]domain.OrderItem, 0, len(existingItems)+1)
	items = append(items, existingItems...)
	items = append(items, candidate)
	return ensureSingleOwnedDigitalGrantPerOrder(items)
}

func cartOrderItemsForGrantValidation(ctx context.Context, db queryer, userID int64, excludedProductID int64) ([]domain.OrderItem, error) {
	rows, err := db.Query(ctx, `
		SELECT ci.product_id, p.sku, p.title, p.category, p.grant_type, p.grant_key, ci.quantity, p.price_credits
		FROM mall_cart_items ci
		JOIN mall_products p ON p.id = ci.product_id
		WHERE ci.user_id = $1::BIGINT
		  AND ci.product_id <> $2
		  AND p.status = $3
		ORDER BY ci.product_id ASC
		FOR UPDATE OF ci`,
		userID,
		excludedProductID,
		string(domain.ProductStatusActive),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.OrderItem, 0)
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(
			&item.ProductID,
			&item.SKU,
			&item.Title,
			&item.Category,
			&item.GrantType,
			&item.GrantKey,
			&item.Quantity,
			&item.UnitPriceCredits,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func cartOrderItemForProduct(product domain.Product, quantity int32) domain.OrderItem {
	grantType, grantKey := normalizeProductGrant(product)
	return domain.OrderItem{
		ProductID:        product.ID,
		SKU:              product.SKU,
		Title:            product.Title,
		Category:         strings.TrimSpace(product.Category),
		GrantType:        grantType,
		GrantKey:         grantKey,
		Quantity:         quantity,
		UnitPriceCredits: product.PriceCredits,
	}
}

func (r *PostgresRepository) RemoveCartItem(ctx context.Context, userID int64, productID int64) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUserCart(ctx, tx, userID); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM mall_cart_items WHERE user_id = $1::BIGINT AND product_id = $2`, userID, productID)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PostgresRepository) ClearCart(ctx context.Context, userID int64) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUserCart(ctx, tx, userID); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM mall_cart_items WHERE user_id = $1::BIGINT`, userID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *PostgresRepository) ListProductFavorites(ctx context.Context, userID int64, limit, offset int) ([]domain.ProductFavorite, int64, error) {
	limit = domain.NormalizeListLimit(limit)
	offset = domain.NormalizeOffset(offset)
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_product_favorites f
		JOIN mall_products p ON p.id = f.product_id
		WHERE f.user_id = $1::BIGINT
		  AND p.status = $2`,
		userID,
		string(domain.ProductStatusActive),
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT f.user_id, f.created_at, `+prefixedProductColumns("p")+`
		FROM mall_product_favorites f
		JOIN mall_products p ON p.id = f.product_id
		WHERE f.user_id = $1::BIGINT
		  AND p.status = $2
		ORDER BY f.created_at DESC, f.product_id DESC
		LIMIT $3 OFFSET $4`,
		userID,
		string(domain.ProductStatusActive),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanProductFavorites(rows, total)
}

func (r *PostgresRepository) IsProductFavorite(ctx context.Context, userID int64, productID int64) (bool, error) {
	return isProductFavorite(ctx, r.pool, userID, productID)
}

func isProductFavorite(ctx context.Context, db queryer, userID int64, productID int64) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM mall_product_favorites f
		  JOIN mall_products p ON p.id = f.product_id
		  WHERE f.user_id = $1::BIGINT
		    AND f.product_id = $2
		    AND p.status = $3
		)`,
		userID,
		productID,
		string(domain.ProductStatusActive),
	).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) AddProductFavorite(ctx context.Context, userID int64, productID int64, createdAt time.Time) (bool, error) {
	return addProductFavorite(ctx, r.pool, userID, productID, createdAt)
}

func addProductFavorite(ctx context.Context, db queryer, userID int64, productID int64, createdAt time.Time) (bool, error) {
	var productExists bool
	var activeProductExists bool
	var inserted bool
	err := db.QueryRow(ctx, `
		WITH product_state AS (
		  SELECT id, status
		  FROM mall_products
		  WHERE id = $2
		),
		active_product AS (
		  SELECT id
		  FROM product_state
		  WHERE status = $4
		),
		inserted AS (
		  INSERT INTO mall_product_favorites (user_id, product_id, created_at)
		  SELECT $1::BIGINT, id, $3
		  FROM active_product
		  ON CONFLICT (user_id, product_id) DO NOTHING
		  RETURNING 1
		)
		SELECT
		  EXISTS (SELECT 1 FROM product_state),
		  EXISTS (SELECT 1 FROM active_product),
		  EXISTS (SELECT 1 FROM inserted)`,
		userID,
		productID,
		createdAt,
		string(domain.ProductStatusActive),
	).Scan(&productExists, &activeProductExists, &inserted)
	if err != nil {
		return false, err
	}
	if !productExists {
		return false, domain.ErrProductNotFound
	}
	if !activeProductExists {
		return false, domain.ErrProductUnavailable
	}
	return inserted, nil
}

func (r *PostgresRepository) RemoveProductFavorite(ctx context.Context, userID int64, productID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM mall_product_favorites WHERE user_id = $1::BIGINT AND product_id = $2`, userID, productID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func lockUserCart(ctx context.Context, db queryer, userID int64) error {
	_, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock($1::BIGINT)`, cartAdvisoryLockBase+userID)
	return err
}

func lockUserAddresses(ctx context.Context, db queryer, userID int64) error {
	_, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock($1::BIGINT)`, addressAdvisoryLockBase+userID)
	return err
}

func verifyCartMatchesOrder(ctx context.Context, db queryer, userID int64, items []domain.OrderItem) error {
	rows, err := db.Query(ctx, `
		SELECT product_id, quantity
		FROM mall_cart_items
		WHERE user_id = $1::BIGINT
		ORDER BY product_id ASC
		FOR UPDATE`,
		userID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	cartQuantities := make(map[int64]int32, len(items))
	for rows.Next() {
		var productID int64
		var quantity int32
		if err := rows.Scan(&productID, &quantity); err != nil {
			return err
		}
		cartQuantities[productID] = quantity
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(cartQuantities) != len(items) {
		return domain.ErrInvalidOrderState
	}
	for _, item := range items {
		if cartQuantities[item.ProductID] != item.Quantity {
			return domain.ErrInvalidOrderState
		}
	}
	return nil
}

func (r *PostgresRepository) ListAddresses(ctx context.Context, userID int64, limit, offset int) ([]domain.Address, int64, error) {
	limit = domain.NormalizeListLimit(limit)
	offset = domain.NormalizeOffset(offset)
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM mall_addresses WHERE user_id = $1::BIGINT`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectAddressSQL()+`
		WHERE user_id = $1::BIGINT
		ORDER BY is_default DESC, updated_at DESC, id DESC
		LIMIT $2 OFFSET $3`,
		userID,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanAddresses(rows, total)
}

func (r *PostgresRepository) CreateAddress(ctx context.Context, address domain.Address) (domain.Address, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Address{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUserAddresses(ctx, tx, address.UserID); err != nil {
		return domain.Address{}, err
	}
	isDefault := address.IsDefault
	if !isDefault {
		var count int64
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM mall_addresses WHERE user_id = $1::BIGINT`, address.UserID).Scan(&count); err != nil {
			return domain.Address{}, err
		}
		isDefault = count == 0
	}
	if isDefault {
		if err := clearDefaultAddress(ctx, tx, address.UserID); err != nil {
			return domain.Address{}, err
		}
	}
	address.IsDefault = isDefault
	saved, err := scanAddress(tx.QueryRow(ctx, `
		INSERT INTO mall_addresses (
		  user_id, receiver, phone, province, city, district, detail, postal_code, is_default, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		RETURNING `+selectAddressColumns(),
		address.UserID,
		address.Receiver,
		address.Phone,
		address.Province,
		address.City,
		address.District,
		address.Detail,
		address.PostalCode,
		address.IsDefault,
		address.CreatedAt,
		address.UpdatedAt,
	))
	if err != nil {
		return domain.Address{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Address{}, err
	}
	return saved, nil
}

func (r *PostgresRepository) UpdateAddress(ctx context.Context, address domain.Address) (domain.Address, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Address{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUserAddresses(ctx, tx, address.UserID); err != nil {
		return domain.Address{}, err
	}
	var existingDefault bool
	if err := tx.QueryRow(ctx, `SELECT is_default FROM mall_addresses WHERE id = $1 AND user_id = $2::BIGINT FOR UPDATE`, address.ID, address.UserID).Scan(&existingDefault); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Address{}, domain.ErrAddressNotFound
		}
		return domain.Address{}, err
	}
	if address.IsDefault {
		if err := clearDefaultAddress(ctx, tx, address.UserID); err != nil {
			return domain.Address{}, err
		}
	}
	updated, err := scanAddress(tx.QueryRow(ctx, `
		UPDATE mall_addresses
		SET receiver = $3,
		    phone = $4,
		    province = $5,
		    city = $6,
		    district = $7,
		    detail = $8,
		    postal_code = $9,
		    is_default = $10,
		    updated_at = $11
		WHERE id = $1
		  AND user_id = $2::BIGINT
		RETURNING `+selectAddressColumns(),
		address.ID,
		address.UserID,
		address.Receiver,
		address.Phone,
		address.Province,
		address.City,
		address.District,
		address.Detail,
		address.PostalCode,
		address.IsDefault,
		address.UpdatedAt,
	))
	if err != nil {
		return domain.Address{}, err
	}
	if existingDefault && !updated.IsDefault {
		if err := ensureAnyDefaultAddress(ctx, tx, address.UserID, updated.ID, address.UpdatedAt); err != nil {
			return domain.Address{}, err
		}
		updated.IsDefault = true
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Address{}, err
	}
	return updated, nil
}

func (r *PostgresRepository) DeleteAddress(ctx context.Context, userID, addressID int64) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUserAddresses(ctx, tx, userID); err != nil {
		return false, err
	}
	var wasDefault bool
	if err := tx.QueryRow(ctx, `SELECT is_default FROM mall_addresses WHERE id = $1 AND user_id = $2::BIGINT FOR UPDATE`, addressID, userID).Scan(&wasDefault); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, domain.ErrAddressNotFound
		}
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM mall_addresses WHERE id = $1 AND user_id = $2::BIGINT`, addressID, userID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, domain.ErrAddressNotFound
	}
	if wasDefault {
		if err := promoteLatestDefaultAddress(ctx, tx, userID, time.Now().UTC()); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *PostgresRepository) SetDefaultAddress(ctx context.Context, userID, addressID int64, updatedAt time.Time) (domain.Address, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Address{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUserAddresses(ctx, tx, userID); err != nil {
		return domain.Address{}, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mall_addresses WHERE id = $1 AND user_id = $2::BIGINT)`, addressID, userID).Scan(&exists); err != nil {
		return domain.Address{}, err
	}
	if !exists {
		return domain.Address{}, domain.ErrAddressNotFound
	}
	if err := clearDefaultAddress(ctx, tx, userID); err != nil {
		return domain.Address{}, err
	}
	updated, err := scanAddress(tx.QueryRow(ctx, `
		UPDATE mall_addresses
		SET is_default = true,
		    updated_at = $3
		WHERE id = $1
		  AND user_id = $2::BIGINT
		RETURNING `+selectAddressColumns(),
		addressID,
		userID,
		updatedAt,
	))
	if err != nil {
		return domain.Address{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Address{}, err
	}
	return updated, nil
}

func (r *PostgresRepository) CreateRefundRequest(ctx context.Context, request domain.RefundRequest) (domain.RefundRequest, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RefundRequest{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := getOrderForUpdate(ctx, tx, request.OrderID)
	if err != nil {
		return domain.RefundRequest{}, false, err
	}
	request, err = refundRequestForLockedOrder(request, order)
	if err != nil {
		return domain.RefundRequest{}, false, err
	}
	created, duplicate, err := insertRefundRequest(ctx, tx, request)
	if err != nil {
		return domain.RefundRequest{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RefundRequest{}, false, err
	}
	return created, duplicate, nil
}

func refundRequestForLockedOrder(request domain.RefundRequest, order domain.Order) (domain.RefundRequest, error) {
	if order.UserID != request.UserID {
		return domain.RefundRequest{}, domain.ErrOrderOwnerMismatch
	}
	if !isRefundableOrderStatus(order.Status) {
		return domain.RefundRequest{}, domain.ErrInvalidOrderState
	}
	if orderContainsMembershipGrant(order) {
		return domain.RefundRequest{}, domain.ErrMembershipRefundUnavailable
	}
	request.OrderID = order.ID
	request.OrderNo = order.OrderNo
	request.UserID = order.UserID
	request.AmountCredits = order.TotalCredits
	request.Status = domain.RefundStatusRequested
	return request, nil
}

func insertRefundRequest(ctx context.Context, db queryer, request domain.RefundRequest) (domain.RefundRequest, bool, error) {
	created, err := scanRefundRequest(db.QueryRow(ctx, `
		INSERT INTO mall_refund_requests (
		  order_id, order_no, user_id, amount_credits, status, reason, user_note, admin_note, restore_stock, operator_id, requested_at, reviewed_at, refunded_at, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, '', false, '', $8, NULL, NULL, $9, $10
		)
		ON CONFLICT (order_id) DO NOTHING
		RETURNING `+selectRefundRequestColumns(),
		request.OrderID,
		request.OrderNo,
		request.UserID,
		request.AmountCredits,
		string(request.Status),
		request.Reason,
		request.UserNote,
		request.RequestedAt,
		request.CreatedAt,
		request.UpdatedAt,
	))
	if errors.Is(err, domain.ErrRefundNotFound) {
		existing, getErr := getRefundRequestByOrderID(ctx, db, request.OrderID)
		return existing, true, getErr
	}
	if err != nil {
		return domain.RefundRequest{}, false, err
	}
	return created, false, nil
}

func (r *PostgresRepository) GetRefundRequest(ctx context.Context, refundID int64) (domain.RefundRequest, error) {
	return getRefundRequest(ctx, r.pool, refundID)
}

func (r *PostgresRepository) ListRefundRequests(ctx context.Context, query domain.RefundListQuery) ([]domain.RefundRequest, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	status := domain.NormalizeRefundStatus(query.Status)
	total, err := r.countRefundRequests(ctx, query.UserID, status, "")
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectRefundRequestSQL()+`
		WHERE user_id = $1::BIGINT
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`,
		query.UserID,
		string(status),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanRefundRequests(rows, total)
}

func (r *PostgresRepository) AdminListRefundRequests(ctx context.Context, query domain.RefundListQuery) ([]domain.RefundRequest, int64, error) {
	limit := domain.NormalizeListLimit(query.Limit)
	offset := domain.NormalizeOffset(query.Offset)
	status := domain.NormalizeRefundStatus(query.Status)
	keyword := strings.TrimSpace(query.Keyword)
	total, err := r.countRefundRequests(ctx, query.UserID, status, keyword)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, selectRefundRequestSQL()+`
		WHERE ($1::BIGINT = 0 OR user_id = $1::BIGINT)
		  AND ($2 = '' OR status = $2)
		  AND `+refundRequestKeywordCondition(3)+`
		ORDER BY created_at DESC, id DESC
		LIMIT $4 OFFSET $5`,
		query.UserID,
		string(status),
		keyword,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanRefundRequests(rows, total)
}

func (r *PostgresRepository) StartRefundApproval(ctx context.Context, refundID int64, operatorID, adminNote string, restoreStock bool, reviewedAt time.Time) (domain.RefundRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	refund, err := getRefundRequestForUpdate(ctx, tx, refundID)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	switch refund.Status {
	case domain.RefundStatusApproved, domain.RefundStatusRejected:
		if err := tx.Commit(ctx); err != nil {
			return domain.RefundRequest{}, err
		}
		return refund, nil
	case domain.RefundStatusRequested, domain.RefundStatusProcessing:
	default:
		return domain.RefundRequest{}, domain.ErrInvalidOrderState
	}
	order, err := getOrderForUpdate(ctx, tx, refund.OrderID)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if orderContainsMembershipGrant(order) {
		return domain.RefundRequest{}, domain.ErrMembershipRefundUnavailable
	}
	if refund.Status == domain.RefundStatusProcessing {
		if err := tx.Commit(ctx); err != nil {
			return domain.RefundRequest{}, err
		}
		return refund, nil
	}
	if !isRefundableOrderStatus(order.Status) {
		return domain.RefundRequest{}, domain.ErrInvalidOrderState
	}
	updated, err := scanRefundRequest(tx.QueryRow(ctx, `
		UPDATE mall_refund_requests
		SET status = $2,
		    admin_note = $3,
		    restore_stock = $4,
		    operator_id = $5,
		    reviewed_at = $6,
		    updated_at = $6
		WHERE id = $1
		  AND status = $7
		RETURNING `+selectRefundRequestColumns(),
		refundID,
		string(domain.RefundStatusProcessing),
		adminNote,
		restoreStock,
		operatorID,
		reviewedAt,
		string(domain.RefundStatusRequested),
	))
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RefundRequest{}, err
	}
	return updated, nil
}

func (r *PostgresRepository) CompleteRefundApproval(ctx context.Context, refundID int64, reviewedAt time.Time, event domain.OutboxEvent) (domain.RefundRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	refund, err := getRefundRequestForUpdate(ctx, tx, refundID)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if refund.Status == domain.RefundStatusApproved {
		order, err := getOrderForUpdate(ctx, tx, refund.OrderID)
		if err != nil {
			return domain.RefundRequest{}, err
		}
		if err := revokeRefundableDigitalEntitlementsForRefund(ctx, tx, order, refund.ID, reviewedAt); err != nil {
			return domain.RefundRequest{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.RefundRequest{}, err
		}
		return refund, nil
	}
	if refund.Status != domain.RefundStatusProcessing {
		return domain.RefundRequest{}, domain.ErrInvalidOrderState
	}
	order, err := getOrderForUpdate(ctx, tx, refund.OrderID)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if !isRefundableOrderStatus(order.Status) && order.Status != domain.OrderStatusRefunded {
		return domain.RefundRequest{}, domain.ErrInvalidOrderState
	}
	if orderContainsMembershipGrant(order) {
		return domain.RefundRequest{}, domain.ErrMembershipRefundUnavailable
	}
	updated, err := scanRefundRequest(tx.QueryRow(ctx, `
		UPDATE mall_refund_requests
		SET status = $2,
		    refunded_at = $3,
		    reviewed_at = COALESCE(reviewed_at, $3),
		    updated_at = $3
		WHERE id = $1
		  AND status = $4
		RETURNING `+selectRefundRequestColumns(),
		refundID,
		string(domain.RefundStatusApproved),
		reviewedAt,
		string(domain.RefundStatusProcessing),
	))
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if err := revokeRefundableDigitalEntitlementsForRefund(ctx, tx, order, updated.ID, reviewedAt); err != nil {
		return domain.RefundRequest{}, err
	}
	if order.Status != domain.OrderStatusRefunded {
		if err := markOrderRefunded(ctx, tx, order.ID, order.Status, reviewedAt); err != nil {
			return domain.RefundRequest{}, err
		}
		if updated.RestoreStock {
			for _, item := range order.Items {
				if err := releaseProductStock(ctx, tx, item.ProductID, item.Quantity, domain.StockChangeReasonRefundRestored, domain.StockReferenceRefund, updated.ID, domain.OrderStatusOperatorAdmin, updated.OperatorID, "售后审核通过恢复库存", reviewedAt); err != nil {
					return domain.RefundRequest{}, err
				}
			}
		}
		if err := insertOrderStatusLog(ctx, tx, order.ID, order.Status, domain.OrderStatusRefunded, domain.OrderStatusReasonRefunded, domain.OrderStatusOperatorAdmin, updated.OperatorID, updated.AdminNote, reviewedAt); err != nil {
			return domain.RefundRequest{}, err
		}
	}
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return domain.RefundRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RefundRequest{}, err
	}
	return updated, nil
}

func (r *PostgresRepository) RejectRefundRequest(ctx context.Context, refundID int64, operatorID, adminNote string, reviewedAt time.Time, event domain.OutboxEvent) (domain.RefundRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	refund, err := getRefundRequestForUpdate(ctx, tx, refundID)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if rejectable, err := refundRequestRejectable(refund); err != nil {
		return domain.RefundRequest{}, err
	} else if !rejectable {
		if err := tx.Commit(ctx); err != nil {
			return domain.RefundRequest{}, err
		}
		return refund, nil
	}
	updated, err := scanRefundRequest(tx.QueryRow(ctx, `
		UPDATE mall_refund_requests
		SET status = $2,
		    admin_note = $3,
		    operator_id = $4,
		    reviewed_at = $5,
		    updated_at = $5
		WHERE id = $1
		  AND status = $6
		RETURNING `+selectRefundRequestColumns(),
		refundID,
		string(domain.RefundStatusRejected),
		adminNote,
		operatorID,
		reviewedAt,
		string(domain.RefundStatusRequested),
	))
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return domain.RefundRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RefundRequest{}, err
	}
	return updated, nil
}

func refundRequestRejectable(refund domain.RefundRequest) (bool, error) {
	switch refund.Status {
	case domain.RefundStatusRejected:
		return false, nil
	case domain.RefundStatusRequested:
		return true, nil
	default:
		return false, domain.ErrInvalidOrderState
	}
}

func (r *PostgresRepository) CountPendingOutboxEvents(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_outbox_events
		WHERE status IN ('pending', 'failed')
		  AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
		  AND (lease_expires_at IS NULL OR lease_expires_at <= NOW())`).Scan(&count)
	return count, err
}

func (r *PostgresRepository) RequeueOutboxEvents(ctx context.Context, statuses []string, limit int, operatorID string, requeuedAt time.Time) (domain.OutboxRequeueResult, error) {
	if len(statuses) == 0 {
		return domain.OutboxRequeueResult{}, nil
	}
	if limit <= 0 {
		limit = domain.DefaultListLimit
	}
	if requeuedAt.IsZero() {
		requeuedAt = time.Now().UTC()
	}
	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
		  SELECT event_id, aggregate_type, aggregate_id, status, attempts, last_error
		  FROM mall_outbox_events
		  WHERE status = ANY($1::text[])
		  ORDER BY updated_at ASC, created_at ASC
		  LIMIT $2
		  FOR UPDATE SKIP LOCKED
		), requeued AS (
		UPDATE mall_outbox_events e
		SET status = 'pending',
		    attempts = 0,
		    lease_owner = '',
		    lease_expires_at = NULL,
		    last_error = '',
		    next_attempt_at = NULL,
		    updated_at = $3
		FROM picked
		WHERE e.event_id = picked.event_id
		RETURNING e.event_id
		), audited AS (
		INSERT INTO mall_outbox_requeue_audits (
		  event_id, aggregate_type, aggregate_id, previous_status, previous_attempts, previous_error, operator_id, requeued_at
		)
		SELECT p.event_id, p.aggregate_type, p.aggregate_id, p.status, p.attempts, p.last_error, $4, $3
		FROM picked p
		JOIN requeued q ON q.event_id = p.event_id
		)
		SELECT p.event_id
		FROM picked p
		JOIN requeued q ON q.event_id = p.event_id
		ORDER BY p.event_id`,
		statuses,
		limit,
		requeuedAt,
		operatorID,
	)
	if err != nil {
		return domain.OutboxRequeueResult{}, err
	}
	defer rows.Close()
	result := domain.OutboxRequeueResult{}
	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return domain.OutboxRequeueResult{}, err
		}
		result.EventIDs = append(result.EventIDs, eventID)
	}
	if err := rows.Err(); err != nil {
		return domain.OutboxRequeueResult{}, err
	}
	result.Requeued = int64(len(result.EventIDs))
	return result, nil
}

func (r *PostgresRepository) ClaimPendingOutboxEvents(ctx context.Context, owner string, limit int, leaseDuration time.Duration) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		limit = domain.DefaultListLimit
	}
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
		  SELECT event_id
		  FROM mall_outbox_events
		  WHERE status IN ('pending', 'failed')
		    AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
		    AND (lease_expires_at IS NULL OR lease_expires_at <= NOW())
		  ORDER BY created_at ASC
		  LIMIT $1
		  FOR UPDATE SKIP LOCKED
		)
		UPDATE mall_outbox_events e
		SET status = 'publishing',
		    lease_owner = $2,
		    lease_expires_at = NOW() + ($3 * interval '1 second'),
		    attempts = attempts + 1,
		    updated_at = NOW()
		FROM picked
		WHERE e.event_id = picked.event_id
		RETURNING e.event_id, e.aggregate_type, e.aggregate_id, e.event_type, e.message_key, e.payload_json::text, e.attempts, e.created_at`,
		limit,
		owner,
		int64(leaseDuration.Seconds()),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]domain.OutboxEvent, 0)
	for rows.Next() {
		var event domain.OutboxEvent
		if err := rows.Scan(&event.EventID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.MessageKey, &event.PayloadJSON, &event.Attempt, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Payload = []byte(event.PayloadJSON)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *PostgresRepository) MarkOutboxEventPublished(ctx context.Context, eventID string, owner string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE mall_outbox_events
		SET status = 'published',
		    lease_owner = '',
		    lease_expires_at = NULL,
		    last_error = '',
		    published_at = NOW(),
		    updated_at = NOW()
		WHERE event_id = $1
		  AND lease_owner = $2
		  AND status = 'publishing'`,
		eventID,
		owner,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOutboxEventNotFound
	}
	return nil
}

func (r *PostgresRepository) MarkOutboxEventFailed(ctx context.Context, eventID string, owner string, message string, nextAttemptAt time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE mall_outbox_events
		SET status = 'failed',
		    lease_owner = '',
		    lease_expires_at = NULL,
		    last_error = $3,
		    next_attempt_at = $4,
		    updated_at = NOW()
		WHERE event_id = $1
		  AND lease_owner = $2
		  AND status = 'publishing'`,
		eventID,
		owner,
		message,
		nextAttemptAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOutboxEventNotFound
	}
	return nil
}

func (r *PostgresRepository) MarkOutboxEventDeadLetter(ctx context.Context, eventID string, owner string, message string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE mall_outbox_events
		SET status = 'dead_letter',
		    lease_owner = '',
		    lease_expires_at = NULL,
		    last_error = $3,
		    next_attempt_at = NULL,
		    updated_at = NOW()
		WHERE event_id = $1
		  AND lease_owner = $2
		  AND status = 'publishing'`,
		eventID,
		owner,
		message,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOutboxEventNotFound
	}
	return nil
}

func scanProducts(rows pgx.Rows, total int64) ([]domain.Product, int64, error) {
	products := make([]domain.Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, product)
	}
	return products, total, rows.Err()
}

func scanProductCategories(rows pgx.Rows, total int64) ([]domain.ProductCategory, int64, error) {
	categories := make([]domain.ProductCategory, 0)
	for rows.Next() {
		category, err := scanProductCategory(rows)
		if err != nil {
			return nil, 0, err
		}
		categories = append(categories, category)
	}
	return categories, total, rows.Err()
}

func scanProductReviews(rows pgx.Rows, total int64) ([]domain.ProductReview, int64, error) {
	reviews := make([]domain.ProductReview, 0)
	for rows.Next() {
		review, err := scanProductReview(rows)
		if err != nil {
			return nil, 0, err
		}
		reviews = append(reviews, review)
	}
	return reviews, total, rows.Err()
}

func scanOrders(ctx context.Context, db queryer, rows pgx.Rows, total int64) ([]domain.Order, int64, error) {
	orders := make([]domain.Order, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		if err := loadOrderItems(ctx, db, &order); err != nil {
			return nil, 0, err
		}
		orders = append(orders, order)
	}
	return orders, total, rows.Err()
}

func scanRefundRequests(rows pgx.Rows, total int64) ([]domain.RefundRequest, int64, error) {
	refunds := make([]domain.RefundRequest, 0)
	for rows.Next() {
		refund, err := scanRefundRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		refunds = append(refunds, refund)
	}
	return refunds, total, rows.Err()
}

func scanAddresses(rows pgx.Rows, total int64) ([]domain.Address, int64, error) {
	addresses := make([]domain.Address, 0)
	for rows.Next() {
		address, err := scanAddress(rows)
		if err != nil {
			return nil, 0, err
		}
		addresses = append(addresses, address)
	}
	return addresses, total, rows.Err()
}

func scanCartItems(rows pgx.Rows, total int64) ([]domain.CartItem, int64, error) {
	items := make([]domain.CartItem, 0)
	for rows.Next() {
		item, err := scanCartItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func scanProductFavorites(rows pgx.Rows, total int64) ([]domain.ProductFavorite, int64, error) {
	items := make([]domain.ProductFavorite, 0)
	for rows.Next() {
		item, err := scanProductFavorite(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func scanCoupons(rows pgx.Rows, total int64) ([]domain.Coupon, int64, error) {
	items := make([]domain.Coupon, 0)
	for rows.Next() {
		item, err := scanCoupon(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func scanCouponUsages(rows pgx.Rows, total int64) ([]domain.CouponUsage, int64, error) {
	items := make([]domain.CouponUsage, 0)
	for rows.Next() {
		item, err := scanCouponUsageWithCoupon(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func scanProductStockLogs(rows pgx.Rows, total int64) ([]domain.ProductStockLog, int64, error) {
	logs := make([]domain.ProductStockLog, 0)
	for rows.Next() {
		log, err := scanProductStockLog(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}

func scanFinanceAnomalies(rows pgx.Rows) ([]domain.FinanceAnomaly, error) {
	items := make([]domain.FinanceAnomaly, 0)
	for rows.Next() {
		item, err := scanFinanceAnomaly(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanOutboxRequeueAudits(rows pgx.Rows, total int64) ([]domain.OutboxRequeueAudit, int64, error) {
	audits := make([]domain.OutboxRequeueAudit, 0)
	for rows.Next() {
		audit, err := scanOutboxRequeueAudit(rows)
		if err != nil {
			return nil, 0, err
		}
		audits = append(audits, audit)
	}
	return audits, total, rows.Err()
}

func (r *PostgresRepository) statusCounts(ctx context.Context, statement string) ([]domain.StatusCount, error) {
	rows, err := r.pool.Query(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make([]domain.StatusCount, 0)
	for rows.Next() {
		var item domain.StatusCount
		if err := rows.Scan(&item.Status, &item.Count); err != nil {
			return nil, err
		}
		counts = append(counts, item)
	}
	return counts, rows.Err()
}

func (r *PostgresRepository) financeAnomalies(ctx context.Context, limit int) ([]domain.FinanceAnomaly, int64, error) {
	if limit <= 0 {
		limit = 5
	}
	var total int64
	if err := r.pool.QueryRow(ctx, financeAnomalyCTE()+`
		SELECT COUNT(*)
		FROM anomalies`,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, financeAnomalyCTE()+`
		SELECT issue_type, order_id, order_no, user_id, order_status, order_total_credits, succeeded_payment_credits, refunded_credits, difference_credits, updated_at
		FROM anomalies
		ORDER BY updated_at DESC, order_id DESC, issue_type ASC
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanFinanceAnomalies(rows)
	return items, total, err
}

func financeAnomalyCTE() string {
	return `
		WITH payment_totals AS (
		  SELECT
		    order_id,
		    COALESCE(SUM(amount_credits) FILTER (WHERE status = 'SUCCEEDED'), 0) AS succeeded_payment_credits,
		    MAX(updated_at) FILTER (WHERE status = 'SUCCEEDED') AS payment_updated_at
		  FROM mall_payments
		  GROUP BY order_id
		), refund_totals AS (
		  SELECT
		    order_id,
		    COALESCE(SUM(amount_credits) FILTER (WHERE status = 'APPROVED'), 0) AS refunded_credits,
		    MAX(updated_at) FILTER (WHERE status = 'APPROVED') AS refund_updated_at
		  FROM mall_refund_requests
		  GROUP BY order_id
		), order_money AS (
		  SELECT
		    o.id AS order_id,
		    o.order_no,
		    o.user_id,
		    o.status AS order_status,
		    o.total_credits AS order_total_credits,
		    COALESCE(p.succeeded_payment_credits, 0) AS succeeded_payment_credits,
		    COALESCE(r.refunded_credits, 0) AS refunded_credits,
		    GREATEST(
		      o.updated_at,
		      COALESCE(p.payment_updated_at, o.updated_at),
		      COALESCE(r.refund_updated_at, o.updated_at)
		    ) AS updated_at
		  FROM mall_orders o
		  LEFT JOIN payment_totals p ON p.order_id = o.id
		  LEFT JOIN refund_totals r ON r.order_id = o.id
		), anomalies AS (
		  SELECT
		    'PAYMENT_MISMATCH' AS issue_type,
		    order_id,
		    order_no,
		    user_id,
		    order_status,
		    order_total_credits,
		    succeeded_payment_credits,
		    refunded_credits,
		    succeeded_payment_credits - order_total_credits AS difference_credits,
		    updated_at
		  FROM order_money
		  WHERE order_status IN ('PAID', 'SHIPPED', 'COMPLETED', 'REFUNDED')
		    AND succeeded_payment_credits <> order_total_credits
		  UNION ALL
		  SELECT
		    'REFUND_EXCEEDS_PAYMENT' AS issue_type,
		    order_id,
		    order_no,
		    user_id,
		    order_status,
		    order_total_credits,
		    succeeded_payment_credits,
		    refunded_credits,
		    refunded_credits - succeeded_payment_credits AS difference_credits,
		    updated_at
		  FROM order_money
		  WHERE refunded_credits > succeeded_payment_credits
		)`
}

func (r *PostgresRepository) countProducts(ctx context.Context, category string, keyword string, status domain.ProductStatus) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_products
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR category = $2)
		  AND ($3 = '' OR title ILIKE '%' || $3 || '%' OR description ILIKE '%' || $3 || '%' OR sku ILIKE '%' || $3 || '%')`,
		string(status),
		category,
		keyword,
	).Scan(&total)
	return total, err
}

func (r *PostgresRepository) countProductCategories(ctx context.Context, keyword string, status domain.ProductCategoryStatus) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_product_categories
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR slug ILIKE '%' || $2 || '%' OR name ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')`,
		string(status),
		strings.TrimSpace(keyword),
	).Scan(&total)
	return total, err
}

func (r *PostgresRepository) countProductReviews(ctx context.Context, productID int64, userID int64, reviewStatus domain.ProductReviewStatus) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_product_reviews
		WHERE ($1::BIGINT = 0 OR product_id = $1::BIGINT)
		  AND ($2::BIGINT = 0 OR user_id = $2::BIGINT)
		  AND ($3 = '' OR status = $3)`,
		productID,
		userID,
		string(reviewStatus),
	).Scan(&total)
	return total, err
}

func (r *PostgresRepository) countAvailableCoupons(ctx context.Context, now time.Time) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_coupons c
		WHERE c.status = $1
		  AND (c.starts_at IS NULL OR c.starts_at <= $2)
		  AND (c.ends_at IS NULL OR c.ends_at > $2)
		  AND (
		    c.total_quota = 0
		    OR (
		      SELECT COUNT(*)
		      FROM mall_coupon_usages u
		      WHERE u.coupon_id = c.id
		        AND u.status <> $3
		    ) < c.total_quota
		  )`,
		string(domain.CouponStatusActive),
		now,
		string(domain.CouponUsageStatusReleased),
	).Scan(&total)
	return total, err
}

func (r *PostgresRepository) countCoupons(ctx context.Context, keyword string, status domain.CouponStatus) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_coupons c
		WHERE ($1 = '' OR c.status = $1)
		  AND `+couponListKeywordCondition(2),
		string(status),
		strings.TrimSpace(keyword),
	).Scan(&total)
	return total, err
}

func couponListKeywordCondition(keywordParam int) string {
	keyword := fmt.Sprintf("$%d", keywordParam)
	return fmt.Sprintf("(%[1]s = '' OR c.id::TEXT = %[1]s OR c.code ILIKE '%%' || %[1]s || '%%' OR c.name ILIKE '%%' || %[1]s || '%%' OR c.description ILIKE '%%' || %[1]s || '%%')", keyword)
}

func (r *PostgresRepository) countCouponUsages(ctx context.Context, couponID int64, userID int64, status domain.CouponUsageStatus) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_coupon_usages
		WHERE ($1::BIGINT = 0 OR coupon_id = $1::BIGINT)
		  AND ($2::BIGINT = 0 OR user_id = $2::BIGINT)
		  AND ($3 = '' OR status = $3)`,
		couponID,
		userID,
		string(status),
	).Scan(&total)
	return total, err
}

func (r *PostgresRepository) countOrders(ctx context.Context, userID int64, keyword string, status domain.OrderStatus) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_orders
		WHERE ($1::BIGINT = 0 OR user_id = $1::BIGINT)
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR id::TEXT = $3 OR order_no ILIKE '%' || $3 || '%' OR idempotency_key ILIKE '%' || $3 || '%' OR coupon_code ILIKE '%' || $3 || '%' OR receiver ILIKE '%' || $3 || '%' OR phone ILIKE '%' || $3 || '%')`,
		userID,
		string(status),
		keyword,
	).Scan(&total)
	return total, err
}

func (r *PostgresRepository) countReviewableOrders(ctx context.Context, userID int64, productID int64) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_orders o
		WHERE o.user_id = $1::BIGINT
		  AND o.status = $2
		  AND EXISTS (
		    SELECT 1
		    FROM mall_order_items oi
		    WHERE oi.order_id = o.id
		      AND oi.product_id = $3::BIGINT
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM mall_refund_requests rr
		    WHERE rr.order_id = o.id
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM mall_product_reviews r
		    WHERE r.order_id = o.id
		      AND r.product_id = $3::BIGINT
		      AND r.user_id = $1::BIGINT
		  )`,
		userID,
		string(domain.OrderStatusCompleted),
		productID,
	).Scan(&total)
	return total, err
}

func openDigitalGrantOrderExists(ctx context.Context, db queryer, userID int64, grantType, grantKey string) (bool, error) {
	normalizedGrantType := strings.ToLower(strings.TrimSpace(grantType))
	normalizedGrantKey := strings.ToLower(strings.TrimSpace(grantKey))
	if userID <= 0 || normalizedGrantType == "" || normalizedGrantKey == "" {
		return false, nil
	}
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM mall_orders o
		  JOIN mall_order_items oi ON oi.order_id = o.id
		  WHERE o.user_id = $1::BIGINT
		    AND o.status IN ($2, $3)
		    AND LOWER(TRIM(COALESCE(oi.grant_type, ''))) = $4
		    AND LOWER(TRIM(COALESCE(oi.grant_key, ''))) = $5
		)`,
		userID,
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		normalizedGrantType,
		normalizedGrantKey,
	).Scan(&exists)
	return exists, err
}

func openDigitalGrantOrderExistsExcluding(ctx context.Context, db queryer, userID, excludedOrderID int64, grantType, grantKey string) (bool, error) {
	normalizedGrantType := strings.ToLower(strings.TrimSpace(grantType))
	normalizedGrantKey := strings.ToLower(strings.TrimSpace(grantKey))
	if userID <= 0 || excludedOrderID <= 0 || normalizedGrantType == "" || normalizedGrantKey == "" {
		return false, nil
	}
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM mall_orders o
		  JOIN mall_order_items oi ON oi.order_id = o.id
		  WHERE o.user_id = $1::BIGINT
		    AND o.status IN ($2, $3)
		    AND LOWER(TRIM(COALESCE(oi.grant_type, ''))) = $4
		    AND LOWER(TRIM(COALESCE(oi.grant_key, ''))) = $5
		    AND o.id <> $6
		)`,
		userID,
		string(domain.OrderStatusPendingPayment),
		string(domain.OrderStatusPaying),
		normalizedGrantType,
		normalizedGrantKey,
		excludedOrderID,
	).Scan(&exists)
	return exists, err
}

func activeDigitalEntitlementExists(ctx context.Context, db queryer, userID int64, grantType, grantKey string) (bool, error) {
	normalizedGrantType := strings.ToLower(strings.TrimSpace(grantType))
	normalizedGrantKey := strings.ToLower(strings.TrimSpace(grantKey))
	if userID <= 0 || normalizedGrantType == "" || normalizedGrantKey == "" {
		return false, nil
	}
	expiryCondition := `AND (de.expires_at IS NULL OR de.expires_at > NOW())`
	if normalizedGrantType == "membership" {
		expiryCondition = `AND de.expires_at IS NOT NULL AND de.expires_at > NOW()`
	}
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM mall_digital_entitlements de
		  WHERE de.user_id = $1::BIGINT
		    AND LOWER(TRIM(COALESCE(de.grant_type, ''))) = $2
		    AND LOWER(TRIM(COALESCE(de.grant_key, ''))) = $3
		    AND UPPER(TRIM(COALESCE(de.status, ''))) = $4
		    AND de.revoked_at IS NULL
		    `+expiryCondition+`
		)`,
		userID,
		normalizedGrantType,
		normalizedGrantKey,
		domain.DigitalEntitlementStatusActive,
	).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) countRefundRequests(ctx context.Context, userID int64, status domain.RefundStatus, keyword string) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_refund_requests
		WHERE ($1::BIGINT = 0 OR user_id = $1::BIGINT)
		  AND ($2 = '' OR status = $2)
		  AND `+refundRequestKeywordCondition(3),
		userID,
		string(status),
		keyword,
	).Scan(&total)
	return total, err
}

func refundRequestKeywordCondition(keywordParam int) string {
	keyword := fmt.Sprintf("$%d", keywordParam)
	return fmt.Sprintf("(%[1]s = '' OR id::TEXT = %[1]s OR order_id::TEXT = %[1]s OR order_no ILIKE '%%' || %[1]s || '%%' OR reason ILIKE '%%' || %[1]s || '%%' OR user_note ILIKE '%%' || %[1]s || '%%' OR admin_note ILIKE '%%' || %[1]s || '%%' OR operator_id ILIKE '%%' || %[1]s || '%%')", keyword)
}

func (r *PostgresRepository) countProductStockLogs(ctx context.Context, productID int64, reason string) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM mall_product_stock_logs
		WHERE ($1::BIGINT = 0 OR product_id = $1::BIGINT)
		  AND ($2 = '' OR reason = $2)`,
		productID,
		reason,
	).Scan(&total)
	return total, err
}

func getOrder(ctx context.Context, db queryer, orderID int64) (domain.Order, error) {
	order, err := scanOrder(db.QueryRow(ctx, selectOrderSQL()+` WHERE id = $1`, orderID))
	if err != nil {
		return domain.Order{}, err
	}
	if err := loadOrderItems(ctx, db, &order); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func getOrderForUpdate(ctx context.Context, db queryer, orderID int64) (domain.Order, error) {
	order, err := scanOrder(db.QueryRow(ctx, selectOrderSQL()+` WHERE id = $1 FOR UPDATE`, orderID))
	if err != nil {
		return domain.Order{}, err
	}
	if err := loadOrderItems(ctx, db, &order); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func getOrderByIdempotencyKey(ctx context.Context, db queryer, userID int64, idempotencyKey string) (domain.Order, error) {
	order, err := scanOrder(db.QueryRow(ctx, selectOrderSQL()+` WHERE user_id = $1::BIGINT AND idempotency_key = $2`, userID, idempotencyKey))
	if err != nil {
		return domain.Order{}, err
	}
	if err := loadOrderItems(ctx, db, &order); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func getRefundRequest(ctx context.Context, db queryer, refundID int64) (domain.RefundRequest, error) {
	return scanRefundRequest(db.QueryRow(ctx, selectRefundRequestSQL()+` WHERE id = $1`, refundID))
}

func getRefundRequestForUpdate(ctx context.Context, db queryer, refundID int64) (domain.RefundRequest, error) {
	return scanRefundRequest(db.QueryRow(ctx, selectRefundRequestSQL()+` WHERE id = $1 FOR UPDATE`, refundID))
}

func getRefundRequestByOrderID(ctx context.Context, db queryer, orderID int64) (domain.RefundRequest, error) {
	return scanRefundRequest(db.QueryRow(ctx, selectRefundRequestSQL()+` WHERE order_id = $1`, orderID))
}

func orderHasRefundRequest(ctx context.Context, db queryer, orderID int64) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mall_refund_requests WHERE order_id = $1)`, orderID).Scan(&exists)
	return exists, err
}

func isRefundableOrderStatus(status domain.OrderStatus) bool {
	switch status {
	case domain.OrderStatusPaid, domain.OrderStatusShipped, domain.OrderStatusCompleted:
		return true
	default:
		return false
	}
}

func orderContainsMembershipGrant(order domain.Order) bool {
	for _, item := range order.Items {
		if isMembershipGrant(item.GrantType, item.GrantKey) {
			return true
		}
	}
	for _, entitlement := range order.DigitalEntitlements {
		if isMembershipGrant(entitlement.GrantType, entitlement.GrantKey) {
			return true
		}
	}
	return false
}

func isMembershipGrant(grantType, grantKey string) bool {
	normalized := strings.ToLower(strings.TrimSpace(grantType))
	if normalized == "" {
		normalized = digitalGrantTypeForKey(grantKey)
	}
	return normalized == "membership"
}

func loadOrderItems(ctx context.Context, db queryer, order *domain.Order) error {
	rows, err := db.Query(ctx, `
		SELECT oi.product_id, oi.sku, oi.title,
		       COALESCE(NULLIF(oi.category, ''), p.category, ''),
		       COALESCE(NULLIF(oi.grant_type, ''), p.grant_type, ''),
		       COALESCE(NULLIF(oi.grant_key, ''), p.grant_key, ''),
		       oi.quantity, oi.unit_price_credits, oi.subtotal_credits
		FROM mall_order_items oi
		LEFT JOIN mall_products p ON p.id = oi.product_id
		WHERE oi.order_id = $1
		ORDER BY oi.product_id ASC`,
		order.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]domain.OrderItem, 0)
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ProductID, &item.SKU, &item.Title, &item.Category, &item.GrantType, &item.GrantKey, &item.Quantity, &item.UnitPriceCredits, &item.SubtotalCredits); err != nil {
			return err
		}
		items = append(items, item)
	}
	order.Items = items
	if err := rows.Err(); err != nil {
		return err
	}
	return loadDigitalEntitlements(ctx, db, order)
}

func isDigitalOnlyOrder(order domain.Order) bool {
	if len(order.Items) == 0 {
		return false
	}
	for _, item := range order.Items {
		if orderItemRequiresShipping(item) {
			return false
		}
	}
	return true
}

func orderHasDigitalEntitlementItems(order domain.Order) bool {
	for _, item := range order.Items {
		if !orderItemRequiresShipping(item) {
			return true
		}
	}
	return false
}

func orderItemRequiresShipping(item domain.OrderItem) bool {
	if strings.EqualFold(strings.TrimSpace(item.Category), "digital") {
		return false
	}
	return !orderItemHasDigitalGrant(item)
}

func orderItemHasDigitalGrant(item domain.OrderItem) bool {
	if strings.TrimSpace(item.GrantKey) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(item.GrantType)) {
	case "badge", "theme", "membership", "digital":
		return true
	default:
		return false
	}
}

const membershipEntitlementDuration = 30 * 24 * time.Hour

func issueDigitalEntitlements(ctx context.Context, db queryer, order domain.Order, issuedAt time.Time) error {
	for _, item := range order.Items {
		if orderItemRequiresShipping(item) {
			continue
		}
		grantType, grantKey := digitalGrantForItem(item)
		expiresAt := digitalEntitlementExpiresAt(grantType, issuedAt)
		for unit := int32(0); unit < item.Quantity; unit++ {
			for attempt := 0; attempt < 3; attempt++ {
				code, err := newDigitalEntitlementCode()
				if err != nil {
					return err
				}
				_, err = db.Exec(ctx, `
					WITH entitlement_lock AS (
						SELECT pg_advisory_xact_lock(hashtextextended(CONCAT($3::BIGINT::text, ':', LOWER($8), ':', LOWER($9)), 0))
					)
					INSERT INTO mall_digital_entitlements (order_id, product_id, user_id, sku, title, quantity, fulfillment_code, grant_type, grant_key, status, issued_at, expires_at, created_at)
					SELECT
						$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
						CASE
							WHEN $12::timestamptz IS NULL THEN NULL
							ELSE GREATEST(
								$11::timestamptz,
								COALESCE((
									SELECT MAX(existing.expires_at)
									FROM mall_digital_entitlements existing
									WHERE existing.user_id = $3::BIGINT
									  AND LOWER(TRIM(COALESCE(existing.grant_type, ''))) = $8
									  AND LOWER(TRIM(COALESCE(existing.grant_key, ''))) = $9
									  AND UPPER(TRIM(COALESCE(existing.status, ''))) = $10
									  AND existing.revoked_at IS NULL
									  AND existing.expires_at > $11::timestamptz
								), $11::timestamptz)
							) + ($12::timestamptz - $11::timestamptz)
						END,
						$11
					FROM entitlement_lock`,
					order.ID, item.ProductID, order.UserID, item.SKU, item.Title, int32(1), code, grantType, grantKey, domain.DigitalEntitlementStatusActive, issuedAt, nullableTime(expiresAt),
				)
				if err == nil {
					break
				}
				if !isUniqueViolation(err) || attempt == 2 {
					return err
				}
			}
		}
	}
	return nil
}

func digitalEntitlementExpiresAt(grantType string, issuedAt time.Time) *time.Time {
	if !strings.EqualFold(strings.TrimSpace(grantType), "membership") {
		return nil
	}
	expiresAt := issuedAt.Add(membershipEntitlementDuration)
	return &expiresAt
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func millisPtr(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}

func withDigitalEntitlements(event domain.OutboxEvent, entitlements []domain.DigitalEntitlement) (domain.OutboxEvent, error) {
	if len(entitlements) == 0 {
		return event, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return domain.OutboxEvent{}, err
	}
	items := make([]outboxDigitalEntitlement, 0, len(entitlements))
	for _, entitlement := range entitlements {
		items = append(items, outboxDigitalEntitlement{
			ProductID:       entitlement.ProductID,
			SKU:             entitlement.SKU,
			Title:           entitlement.Title,
			Quantity:        entitlement.Quantity,
			FulfillmentCode: entitlement.Code,
			GrantType:       entitlement.GrantType,
			GrantKey:        entitlement.GrantKey,
			Status:          entitlement.Status,
			RefundID:        entitlement.RefundID,
			ExpiresAt:       millisPtr(entitlement.ExpiresAt),
		})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	payload["digital_entitlements"] = encoded
	event.Payload, err = json.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	event.PayloadJSON = string(event.Payload)
	return event, nil
}

type outboxDigitalEntitlement struct {
	ProductID       int64  `json:"product_id"`
	SKU             string `json:"sku"`
	Title           string `json:"title"`
	Quantity        int32  `json:"quantity"`
	FulfillmentCode string `json:"fulfillment_code"`
	GrantType       string `json:"grant_type"`
	GrantKey        string `json:"grant_key"`
	Status          string `json:"status"`
	RefundID        int64  `json:"refund_id"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
}

func newDigitalEntitlementCode() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "BBS-" + strings.TrimRight(base32.StdEncoding.EncodeToString(bytes), "="), nil
}

func digitalGrantForItem(item domain.OrderItem) (string, string) {
	explicitGrantKey := strings.ToLower(strings.TrimSpace(item.GrantKey))
	grantKey := explicitGrantKey
	if explicitGrantKey == "" {
		grantKey = strings.ToLower(strings.TrimSpace(item.SKU))
	}
	if grantKey == "" {
		grantKey = fmt.Sprintf("product:%d", item.ProductID)
	}
	grantType := strings.ToLower(strings.TrimSpace(item.GrantType))
	if grantType == "" {
		if explicitGrantKey == "" {
			grantType = "digital"
		} else {
			grantType = digitalGrantTypeForKey(grantKey)
		}
	}
	if explicitGrantKey == "" && grantType != "digital" {
		grantType = "digital"
	}
	return grantType, grantKey
}

func digitalGrantTypeForKey(grantKey string) string {
	normalized := strings.ToLower(strings.TrimSpace(grantKey))
	switch {
	case strings.HasPrefix(normalized, "badge-"):
		return "badge"
	case strings.HasPrefix(normalized, "theme-"):
		return "theme"
	case strings.HasPrefix(normalized, "vip-"), strings.HasPrefix(normalized, "member-"), strings.Contains(normalized, "membership"):
		return "membership"
	default:
		return "digital"
	}
}

func normalizeDigitalEntitlementStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case domain.DigitalEntitlementStatusActive:
		return domain.DigitalEntitlementStatusActive
	case domain.DigitalEntitlementStatusExpired:
		return domain.DigitalEntitlementStatusExpired
	case domain.DigitalEntitlementStatusRevoked:
		return domain.DigitalEntitlementStatusRevoked
	default:
		return ""
	}
}

func digitalEntitlementListGrantCondition(alias string, grantTypeParam, grantKeyParam int) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	grantTypeExpr := fmt.Sprintf("LOWER(TRIM(COALESCE(%sgrant_type, '')))", prefix)
	grantKeyExpr := fmt.Sprintf("LOWER(TRIM(COALESCE(%sgrant_key, '')))", prefix)
	return fmt.Sprintf(`
		  AND ($%d = '' OR %s = $%d)
		  AND ($%d = '' OR %s = $%d)`, grantTypeParam, grantTypeExpr, grantTypeParam, grantKeyParam, grantKeyExpr, grantKeyParam)
}

func digitalEntitlementListStatusCondition(alias, status string) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	statusExpr := fmt.Sprintf("UPPER(TRIM(COALESCE(%sstatus, '')))", prefix)
	grantTypeExpr := fmt.Sprintf("LOWER(TRIM(COALESCE(%sgrant_type, '')))", prefix)
	expiresAtExpr := prefix + "expires_at"
	revokedAtExpr := prefix + "revoked_at"
	switch status {
	case domain.DigitalEntitlementStatusActive:
		return fmt.Sprintf(`
		  AND %s = '%s'
		  AND %s IS NULL
		  AND (
		    (%s = 'membership' AND %s IS NOT NULL AND %s > NOW())
		    OR (%s <> 'membership' AND (%s IS NULL OR %s > NOW()))
		  )`, statusExpr, domain.DigitalEntitlementStatusActive, revokedAtExpr, grantTypeExpr, expiresAtExpr, expiresAtExpr, grantTypeExpr, expiresAtExpr, expiresAtExpr)
	case domain.DigitalEntitlementStatusExpired:
		return fmt.Sprintf(`
		  AND %s = '%s'
		  AND %s IS NULL
		  AND %s IS NOT NULL
		  AND %s <= NOW()`, statusExpr, domain.DigitalEntitlementStatusActive, revokedAtExpr, expiresAtExpr, expiresAtExpr)
	case domain.DigitalEntitlementStatusRevoked:
		return fmt.Sprintf(`
		  AND (%s = '%s' OR %s IS NOT NULL)`, statusExpr, domain.DigitalEntitlementStatusRevoked, revokedAtExpr)
	default:
		return ""
	}
}

func digitalEntitlementListKeywordCondition(keywordParam int) string {
	return fmt.Sprintf(`
		  AND ($%d = ''
		       OR de.id::TEXT = $%d
		       OR de.user_id::TEXT = $%d
		       OR de.order_id::TEXT = $%d
		       OR de.product_id::TEXT = $%d
		       OR de.refund_id::TEXT = $%d
		       OR o.order_no ILIKE '%%' || $%d || '%%'
		       OR de.sku ILIKE '%%' || $%d || '%%'
		       OR de.title ILIKE '%%' || $%d || '%%'
		       OR de.fulfillment_code ILIKE '%%' || $%d || '%%'
		       OR COALESCE(de.grant_key, '') ILIKE '%%' || $%d || '%%')`,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
		keywordParam,
	)
}

func revokeDigitalEntitlementsForRefund(ctx context.Context, db queryer, orderID, refundID int64, revokedAt time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE mall_digital_entitlements
		SET status = $2,
		    revoked_at = COALESCE(revoked_at, $3),
		    refund_id = COALESCE(refund_id, $4)
		WHERE order_id = $1
		  AND UPPER(TRIM(COALESCE(status, ''))) = $5
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $6)`,
		orderID,
		domain.DigitalEntitlementStatusRevoked,
		revokedAt,
		refundID,
		domain.DigitalEntitlementStatusActive,
		revokedAt,
	)
	return err
}

func revokeRefundableDigitalEntitlementsForRefund(ctx context.Context, db queryer, order domain.Order, refundID int64, revokedAt time.Time) error {
	if orderContainsMembershipGrant(order) {
		return domain.ErrMembershipRefundUnavailable
	}
	return revokeDigitalEntitlementsForRefund(ctx, db, order.ID, refundID, revokedAt)
}

func markDigitalEntitlementRevoked(ctx context.Context, db queryer, entitlementID int64, operatorID string, reason string, revokedAt time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE mall_digital_entitlements
		SET status = $2,
		    revoked_at = $3,
		    revoked_by = $4,
		    revoke_reason = $5
		WHERE id = $1
		  AND UPPER(TRIM(COALESCE(status, ''))) = $6
		  AND revoked_at IS NULL
		  AND (
		    (LOWER(TRIM(COALESCE(grant_type, ''))) = 'membership' AND expires_at IS NOT NULL AND expires_at > $7)
		    OR (LOWER(TRIM(COALESCE(grant_type, ''))) <> 'membership' AND (expires_at IS NULL OR expires_at > $7))
		  )`,
		entitlementID,
		domain.DigitalEntitlementStatusRevoked,
		revokedAt,
		operatorID,
		reason,
		domain.DigitalEntitlementStatusActive,
		revokedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidOrderState
	}
	return nil
}

func loadDigitalEntitlements(ctx context.Context, db queryer, order *domain.Order) error {
	rows, err := db.Query(ctx, `
		SELECT id,
		       order_id,
		       $2,
		       user_id,
		       product_id,
		       sku,
		       title,
		       quantity,
		       fulfillment_code,
		       COALESCE(grant_type, ''),
		       COALESCE(grant_key, ''),
		       issued_at,
		       expires_at,
		       COALESCE(status, ''),
		       revoked_at,
		       refund_id,
		       COALESCE(revoked_by, ''),
		       COALESCE(revoke_reason, '')
		FROM mall_digital_entitlements
		WHERE order_id = $1
		ORDER BY product_id ASC`, order.ID, order.OrderNo)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]domain.DigitalEntitlement, 0)
	for rows.Next() {
		item, err := scanDigitalEntitlement(rows)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	order.DigitalEntitlements = items
	return rows.Err()
}

func scanDigitalEntitlement(row scanner) (domain.DigitalEntitlement, error) {
	var item domain.DigitalEntitlement
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	var refundID sql.NullInt64
	if err := row.Scan(
		&item.ID,
		&item.OrderID,
		&item.OrderNo,
		&item.UserID,
		&item.ProductID,
		&item.SKU,
		&item.Title,
		&item.Quantity,
		&item.Code,
		&item.GrantType,
		&item.GrantKey,
		&item.IssuedAt,
		&expiresAt,
		&item.Status,
		&revokedAt,
		&refundID,
		&item.RevokedBy,
		&item.RevokeReason,
	); err != nil {
		return domain.DigitalEntitlement{}, err
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		item.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		item.RevokedAt = &t
	}
	if refundID.Valid {
		item.RefundID = refundID.Int64
	}
	return item, nil
}

func decrementProductStock(ctx context.Context, db queryer, productID int64, quantity int32, reason string, referenceType string, referenceID int64, operatorType string, operatorID string, note string, updatedAt time.Time) error {
	if quantity <= 0 {
		return nil
	}
	var log domain.ProductStockLog
	err := db.QueryRow(ctx, `
		UPDATE mall_products
		SET stock = stock - $2,
		    updated_at = $3
		WHERE id = $1
		  AND status = $4
		  AND stock >= $2
		RETURNING id, sku, title, stock + $2, stock`,
		productID,
		quantity,
		updatedAt,
		string(domain.ProductStatusActive),
	).Scan(&log.ProductID, &log.SKU, &log.Title, &log.BeforeStock, &log.AfterStock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInsufficientStock
		}
		return err
	}
	log.Delta = -int64(quantity)
	log.Reason = reason
	log.ReferenceType = referenceType
	log.ReferenceID = referenceID
	log.OperatorType = operatorType
	log.OperatorID = strings.TrimSpace(operatorID)
	log.Note = note
	log.CreatedAt = updatedAt
	return insertProductStockLog(ctx, db, log)
}

func stockDeductionItems(items []domain.OrderItem) []domain.OrderItem {
	ordered := append([]domain.OrderItem(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].ProductID < ordered[j].ProductID
	})
	return ordered
}

func releaseProductStock(ctx context.Context, db queryer, productID int64, quantity int32, reason string, referenceType string, referenceID int64, operatorType string, operatorID string, note string, updatedAt time.Time) error {
	if quantity <= 0 {
		return nil
	}
	var log domain.ProductStockLog
	err := db.QueryRow(ctx, `
		UPDATE mall_products
		SET stock = stock + $2,
		    updated_at = $3
		WHERE id = $1
		RETURNING id, sku, title, stock - $2, stock`,
		productID,
		quantity,
		updatedAt,
	).Scan(&log.ProductID, &log.SKU, &log.Title, &log.BeforeStock, &log.AfterStock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrProductNotFound
		}
		return err
	}
	log.Delta = int64(quantity)
	log.Reason = reason
	log.ReferenceType = referenceType
	log.ReferenceID = referenceID
	log.OperatorType = operatorType
	log.OperatorID = strings.TrimSpace(operatorID)
	log.Note = note
	log.CreatedAt = updatedAt
	return insertProductStockLog(ctx, db, log)
}

func insertPendingPayment(ctx context.Context, db queryer, order domain.Order, provider, idempotencyKey string, now time.Time) (domain.Payment, error) {
	payment, err := scanPayment(db.QueryRow(ctx, `
		INSERT INTO mall_payments (
		  order_id, user_id, amount_credits, provider, idempotency_key, status, provider_trade_no, failure_reason, paid_at, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, '', '', NULL, $7, $7
		)
		ON CONFLICT (provider, idempotency_key) DO NOTHING
		RETURNING `+selectPaymentColumns(),
		order.ID,
		order.UserID,
		order.TotalCredits,
		provider,
		idempotencyKey,
		string(domain.PaymentStatusPending),
		now,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := getPaymentByProviderKey(ctx, db, provider, idempotencyKey)
		if getErr != nil {
			return domain.Payment{}, getErr
		}
		if existing.OrderID == order.ID && existing.UserID == order.UserID {
			switch existing.Status {
			case domain.PaymentStatusPending:
				return existing, nil
			case domain.PaymentStatusFailed:
				if _, updateErr := db.Exec(ctx, `
					UPDATE mall_payments
					SET status = $2,
					    provider_trade_no = '',
					    failure_reason = '',
					    paid_at = NULL,
					    updated_at = $3
					WHERE id = $1
					  AND status = $4`,
					existing.ID,
					string(domain.PaymentStatusPending),
					now,
					string(domain.PaymentStatusFailed),
				); updateErr != nil {
					return domain.Payment{}, updateErr
				}
				return getPaymentByProviderKey(ctx, db, provider, idempotencyKey)
			}
		}
		return domain.Payment{}, domain.ErrDuplicateReference
	}
	return payment, err
}

func getPaymentByProviderKey(ctx context.Context, db queryer, provider, idempotencyKey string) (domain.Payment, error) {
	return scanPayment(db.QueryRow(ctx, selectPaymentSQL()+` WHERE provider = $1 AND idempotency_key = $2`, provider, idempotencyKey))
}

func getPaymentForUpdate(ctx context.Context, db queryer, paymentID int64) (domain.Payment, error) {
	return scanPayment(db.QueryRow(ctx, selectPaymentSQL()+` WHERE id = $1 FOR UPDATE`, paymentID))
}

func validatePaymentForOrder(payment domain.Payment, order domain.Order, userID int64) error {
	if payment.OrderID != order.ID || payment.UserID != userID || payment.AmountCredits != order.TotalCredits {
		return domain.ErrInvalidOrderState
	}
	return nil
}

func canCompleteOrderPayment(orderStatus domain.OrderStatus, paymentStatus domain.PaymentStatus) bool {
	switch orderStatus {
	case domain.OrderStatusPaying:
		return paymentStatus == domain.PaymentStatusPending
	case domain.OrderStatusPaid, domain.OrderStatusCompleted:
		return paymentStatus == domain.PaymentStatusSucceeded
	default:
		return false
	}
}

func markPaymentSucceeded(ctx context.Context, db queryer, paymentID, orderID, userID, amountCredits int64, paidAt time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE mall_payments
		SET status = $2,
		    provider_trade_no = $3,
		    paid_at = $4,
		    updated_at = $4
		WHERE id = $1
		  AND order_id = $5
		  AND user_id = $6::BIGINT
		  AND amount_credits = $7
		  AND status = $8`,
		paymentID,
		string(domain.PaymentStatusSucceeded),
		fmt.Sprintf("credit-%d", paymentID),
		paidAt,
		orderID,
		userID,
		amountCredits,
		string(domain.PaymentStatusPending),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidOrderState
	}
	return nil
}

func markPaymentFailed(ctx context.Context, db queryer, paymentID, orderID, userID, amountCredits int64, reason string, failedAt time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE mall_payments
		SET status = $2,
		    failure_reason = $3,
		    updated_at = $4
		WHERE id = $1
		  AND order_id = $5
		  AND user_id = $6::BIGINT
		  AND amount_credits = $7
		  AND status = $8`,
		paymentID,
		string(domain.PaymentStatusFailed),
		reason,
		failedAt,
		orderID,
		userID,
		amountCredits,
		string(domain.PaymentStatusPending),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidOrderState
	}
	return nil
}

func incrementProductSales(ctx context.Context, db queryer, productID int64, quantity int32, updatedAt time.Time) error {
	if quantity <= 0 {
		return nil
	}
	tag, err := db.Exec(ctx, `
		UPDATE mall_products
		SET sales_count = sales_count + $2,
		    updated_at = $3
		WHERE id = $1`,
		productID,
		quantity,
		updatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

func reopenOrderAfterPaymentFailure(ctx context.Context, db queryer, orderID, userID int64, failedAt time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE mall_orders
		SET status = $2,
		    updated_at = $3
		WHERE id = $1
		  AND user_id = $4::BIGINT
		  AND status = $5`,
		orderID,
		string(domain.OrderStatusPendingPayment),
		failedAt,
		userID,
		string(domain.OrderStatusPaying),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidOrderState
	}
	return nil
}

func markOrderRefunded(ctx context.Context, db queryer, orderID int64, fromStatus domain.OrderStatus, reviewedAt time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE mall_orders
		SET status = $2,
		    updated_at = $3
		WHERE id = $1
		  AND status = $4`,
		orderID,
		string(domain.OrderStatusRefunded),
		reviewedAt,
		string(fromStatus),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidOrderState
	}
	return nil
}

func insertOutboxEvent(ctx context.Context, db queryer, event domain.OutboxEvent) error {
	_, err := db.Exec(ctx, `
		INSERT INTO mall_outbox_events (
		  event_id, aggregate_type, aggregate_id, event_type, message_key, payload_json, status, attempts, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, 'pending', 0, $7, $7
		)`,
		event.EventID,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.MessageKey,
		event.PayloadJSON,
		event.CreatedAt,
	)
	return err
}

func insertOrderStatusLog(ctx context.Context, db queryer, orderID int64, fromStatus domain.OrderStatus, toStatus domain.OrderStatus, reason string, operatorType string, operatorID string, note string, createdAt time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO mall_order_status_logs (
		  order_id, from_status, to_status, reason, operator_type, operator_id, note, created_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, $8
		)`,
		orderID,
		string(fromStatus),
		string(toStatus),
		reason,
		operatorType,
		operatorID,
		note,
		createdAt,
	)
	return err
}

func insertProductStockLog(ctx context.Context, db queryer, log domain.ProductStockLog) error {
	_, err := db.Exec(ctx, `
		INSERT INTO mall_product_stock_logs (
		  product_id, sku, title, delta, before_stock, after_stock, reason, reference_type, reference_id, operator_type, operator_id, note, created_at
		) VALUES (
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)`,
		log.ProductID,
		log.SKU,
		log.Title,
		log.Delta,
		log.BeforeStock,
		log.AfterStock,
		log.Reason,
		log.ReferenceType,
		log.ReferenceID,
		log.OperatorType,
		strings.TrimSpace(log.OperatorID),
		log.Note,
		log.CreatedAt,
	)
	return err
}

func clearDefaultAddress(ctx context.Context, db queryer, userID int64) error {
	_, err := db.Exec(ctx, `UPDATE mall_addresses SET is_default = false WHERE user_id = $1::BIGINT AND is_default = true`, userID)
	return err
}

func ensureAnyDefaultAddress(ctx context.Context, db queryer, userID, preferredAddressID int64, updatedAt time.Time) error {
	var count int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM mall_addresses WHERE user_id = $1::BIGINT AND is_default = true`, userID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	tag, err := db.Exec(ctx, `
		UPDATE mall_addresses
		SET is_default = true,
		    updated_at = $3
		WHERE id = $1
		  AND user_id = $2::BIGINT`,
		preferredAddressID,
		userID,
		updatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAddressNotFound
	}
	return nil
}

func promoteLatestDefaultAddress(ctx context.Context, db queryer, userID int64, updatedAt time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE mall_addresses
		SET is_default = true,
		    updated_at = $2
		WHERE id = (
		  SELECT id
		  FROM mall_addresses
		  WHERE user_id = $1::BIGINT
		  ORDER BY updated_at DESC, id DESC
		  LIMIT 1
		)`,
		userID,
		updatedAt,
	)
	return err
}

func isAllowedAdminOrderTransition(current domain.OrderStatus, next domain.OrderStatus) bool {
	switch next {
	case domain.OrderStatusShipped:
		return current == domain.OrderStatusPaid
	case domain.OrderStatusCompleted:
		return current == domain.OrderStatusPaid || current == domain.OrderStatusShipped
	default:
		return false
	}
}

func adminOrderTransitionReason(next domain.OrderStatus) string {
	switch next {
	case domain.OrderStatusShipped:
		return domain.OrderStatusReasonShipped
	case domain.OrderStatusCompleted:
		return domain.OrderStatusReasonCompleted
	default:
		return "admin_status_update"
	}
}

func validatePersistedAdminOrderFulfillment(order domain.Order, nextStatus domain.OrderStatus, fulfillment domain.OrderFulfillment, note string) error {
	if !persistedOrderRequiresShipping(order) {
		return nil
	}
	hasCarrierOrTracking := strings.TrimSpace(order.ShippingCarrier) != "" ||
		strings.TrimSpace(order.TrackingNo) != "" ||
		strings.TrimSpace(fulfillment.ShippingCarrier) != "" ||
		strings.TrimSpace(fulfillment.TrackingNo) != ""
	switch nextStatus {
	case domain.OrderStatusShipped:
		if !hasCarrierOrTracking {
			return domain.ErrInvalidOrderState
		}
	case domain.OrderStatusCompleted:
		if !hasCarrierOrTracking && strings.TrimSpace(note) == "" {
			return domain.ErrInvalidOrderState
		}
	}
	return nil
}

func persistedOrderRequiresShipping(order domain.Order) bool {
	if strings.TrimSpace(order.Receiver) != "" || strings.TrimSpace(order.Phone) != "" || strings.TrimSpace(order.Address) != "" {
		return true
	}
	for _, item := range order.Items {
		if orderItemRequiresShipping(item) {
			return true
		}
	}
	return false
}

func selectProductSQL() string {
	return `SELECT ` + selectProductColumns() + ` FROM mall_products`
}

func selectProductColumns() string {
	return `id, sku, title, description, category, cover_url, grant_type, grant_key, price_credits, stock, sales_count, status, sort, created_at, updated_at`
}

func selectProductCategorySQL() string {
	return `SELECT ` + selectProductCategoryColumns("c") + ` FROM mall_product_categories c`
}

func selectProductCategoryColumns(alias string) string {
	return alias + `.id, ` + alias + `.slug, ` + alias + `.name, ` + alias + `.description, ` + alias + `.status, ` + alias + `.sort, ` +
		`(SELECT COUNT(*) FROM mall_products p WHERE p.category = ` + alias + `.slug) AS product_count, ` +
		alias + `.created_at, ` + alias + `.updated_at`
}

func selectProductReviewSQL() string {
	return `
		SELECT r.id, r.product_id, p.sku, p.title, r.order_id, r.user_id, r.rating, r.content, r.status, r.created_at, r.updated_at
		FROM mall_product_reviews r
		JOIN mall_products p ON p.id = r.product_id`
}

func prefixedProductColumns(alias string) string {
	return alias + `.id, ` + alias + `.sku, ` + alias + `.title, ` + alias + `.description, ` + alias + `.category, ` + alias + `.cover_url, ` + alias + `.grant_type, ` + alias + `.grant_key, ` + alias + `.price_credits, ` + alias + `.stock, ` + alias + `.sales_count, ` + alias + `.status, ` + alias + `.sort, ` + alias + `.created_at, ` + alias + `.updated_at`
}

func selectOrderSQL() string {
	return `SELECT ` + selectOrderColumns() + ` FROM mall_orders`
}

func selectOrderColumns() string {
	return `id, order_no, idempotency_key, user_id, original_credits, discount_credits, total_credits, coupon_id, coupon_code, status, receiver, phone, address, payment_method, paid_at, created_at, updated_at, shipping_carrier, tracking_no, shipped_at, completed_at`
}

func selectPaymentSQL() string {
	return `SELECT ` + selectPaymentColumns() + ` FROM mall_payments`
}

func selectPaymentColumns() string {
	return `id, order_id, user_id, amount_credits, provider, idempotency_key, status, provider_trade_no, failure_reason, paid_at, created_at, updated_at`
}

func selectRefundRequestSQL() string {
	return `SELECT ` + selectRefundRequestColumns() + ` FROM mall_refund_requests`
}

func selectRefundRequestColumns() string {
	return `id, order_id, order_no, user_id, amount_credits, status, reason, user_note, admin_note, restore_stock, operator_id, requested_at, reviewed_at, refunded_at, created_at, updated_at`
}

func selectAddressSQL() string {
	return `SELECT ` + selectAddressColumns() + ` FROM mall_addresses`
}

func selectAddressColumns() string {
	return `id, user_id, receiver, phone, province, city, district, detail, postal_code, is_default, created_at, updated_at`
}

func selectProductStockLogSQL() string {
	return `SELECT ` + selectProductStockLogColumns() + ` FROM mall_product_stock_logs`
}

func selectProductStockLogColumns() string {
	return `id, product_id, sku, title, delta, before_stock, after_stock, reason, reference_type, reference_id, operator_type, operator_id, note, created_at`
}

func selectCouponSQL() string {
	return `
		SELECT c.id, c.code, c.name, c.description, c.discount_credits, c.min_order_credits, c.total_quota, c.per_user_limit,
		  (
		    SELECT COUNT(*)
		    FROM mall_coupon_usages u
		    WHERE u.coupon_id = c.id
		      AND u.status <> 'RELEASED'
		  ) AS claimed_count,
		  (
		    SELECT COUNT(*)
		    FROM mall_coupon_usages u
		    WHERE u.coupon_id = c.id
		      AND u.status = 'USED'
		  ) AS used_count,
		  c.status, c.starts_at, c.ends_at, c.created_at, c.updated_at
		FROM mall_coupons c`
}

func selectCouponColumns() string {
	return `id, code, name, description, discount_credits, min_order_credits, total_quota, per_user_limit, 0::BIGINT AS claimed_count, 0::BIGINT AS used_count, status, starts_at, ends_at, created_at, updated_at`
}

func selectCouponUsageColumns() string {
	return `id, coupon_id, code, user_id, order_id, status, discount_credits, created_at, used_at, released_at, updated_at`
}

func selectCouponUsageWithCouponSQL() string {
	return `
		SELECT u.id, u.coupon_id, u.code, u.user_id, u.order_id, u.status, u.discount_credits, u.created_at, u.used_at, u.released_at, u.updated_at,
		  c.id, c.code, c.name, c.description, c.discount_credits, c.min_order_credits, c.total_quota, c.per_user_limit,
		  (
		    SELECT COUNT(*)
		    FROM mall_coupon_usages counted
		    WHERE counted.coupon_id = c.id
		      AND counted.status <> 'RELEASED'
		  ) AS claimed_count,
		  (
		    SELECT COUNT(*)
		    FROM mall_coupon_usages counted
		    WHERE counted.coupon_id = c.id
		      AND counted.status = 'USED'
		  ) AS used_count,
		  c.status, c.starts_at, c.ends_at, c.created_at, c.updated_at
		FROM mall_coupon_usages u
		JOIN mall_coupons c ON c.id = u.coupon_id`
}

type queryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(row scanner) (domain.Product, error) {
	var product domain.Product
	var status string
	err := row.Scan(
		&product.ID,
		&product.SKU,
		&product.Title,
		&product.Description,
		&product.Category,
		&product.CoverURL,
		&product.GrantType,
		&product.GrantKey,
		&product.PriceCredits,
		&product.Stock,
		&product.SalesCount,
		&status,
		&product.Sort,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Product{}, domain.ErrProductNotFound
		}
		return domain.Product{}, err
	}
	product.Status = domain.ProductStatus(status)
	return product, nil
}

func scanProductCategory(row scanner) (domain.ProductCategory, error) {
	var category domain.ProductCategory
	var status string
	err := row.Scan(
		&category.ID,
		&category.Slug,
		&category.Name,
		&category.Description,
		&status,
		&category.Sort,
		&category.ProductCount,
		&category.CreatedAt,
		&category.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProductCategory{}, domain.ErrProductCategoryNotFound
		}
		return domain.ProductCategory{}, err
	}
	category.Status = domain.ProductCategoryStatus(status)
	return category, nil
}

func scanProductReview(row scanner) (domain.ProductReview, error) {
	var review domain.ProductReview
	var status string
	err := row.Scan(
		&review.ID,
		&review.ProductID,
		&review.ProductSKU,
		&review.ProductTitle,
		&review.OrderID,
		&review.UserID,
		&review.Rating,
		&review.Content,
		&status,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProductReview{}, domain.ErrProductReviewNotFound
		}
		return domain.ProductReview{}, err
	}
	review.Status = domain.ProductReviewStatus(status)
	return review, nil
}

func scanOrder(row scanner) (domain.Order, error) {
	var order domain.Order
	var status string
	var paidAt sql.NullTime
	var shippedAt sql.NullTime
	var completedAt sql.NullTime
	var couponID sql.NullInt64
	err := row.Scan(
		&order.ID,
		&order.OrderNo,
		&order.IdempotencyKey,
		&order.UserID,
		&order.OriginalCredits,
		&order.DiscountCredits,
		&order.TotalCredits,
		&couponID,
		&order.CouponCode,
		&status,
		&order.Receiver,
		&order.Phone,
		&order.Address,
		&order.PaymentMethod,
		&paidAt,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.ShippingCarrier,
		&order.TrackingNo,
		&shippedAt,
		&completedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrOrderNotFound
		}
		return domain.Order{}, err
	}
	order.Status = domain.OrderStatus(status)
	if couponID.Valid {
		order.CouponID = couponID.Int64
	}
	if paidAt.Valid {
		order.PaidAt = &paidAt.Time
	}
	if shippedAt.Valid {
		order.ShippedAt = &shippedAt.Time
	}
	if completedAt.Valid {
		order.CompletedAt = &completedAt.Time
	}
	return order, nil
}

func scanPayment(row scanner) (domain.Payment, error) {
	var payment domain.Payment
	var status string
	var paidAt sql.NullTime
	err := row.Scan(
		&payment.ID,
		&payment.OrderID,
		&payment.UserID,
		&payment.AmountCredits,
		&payment.Provider,
		&payment.IdempotencyKey,
		&status,
		&payment.ProviderTradeNo,
		&payment.FailureReason,
		&paidAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Payment{}, domain.ErrInvalidOrderState
		}
		return domain.Payment{}, err
	}
	payment.Status = domain.PaymentStatus(status)
	if paidAt.Valid {
		payment.PaidAt = &paidAt.Time
	}
	return payment, nil
}

func scanRefundRequest(row scanner) (domain.RefundRequest, error) {
	var refund domain.RefundRequest
	var status string
	var reviewedAt sql.NullTime
	var refundedAt sql.NullTime
	err := row.Scan(
		&refund.ID,
		&refund.OrderID,
		&refund.OrderNo,
		&refund.UserID,
		&refund.AmountCredits,
		&status,
		&refund.Reason,
		&refund.UserNote,
		&refund.AdminNote,
		&refund.RestoreStock,
		&refund.OperatorID,
		&refund.RequestedAt,
		&reviewedAt,
		&refundedAt,
		&refund.CreatedAt,
		&refund.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefundRequest{}, domain.ErrRefundNotFound
		}
		return domain.RefundRequest{}, err
	}
	refund.Status = domain.RefundStatus(status)
	if reviewedAt.Valid {
		refund.ReviewedAt = &reviewedAt.Time
	}
	if refundedAt.Valid {
		refund.RefundedAt = &refundedAt.Time
	}
	return refund, nil
}

func scanAddress(row scanner) (domain.Address, error) {
	var address domain.Address
	err := row.Scan(
		&address.ID,
		&address.UserID,
		&address.Receiver,
		&address.Phone,
		&address.Province,
		&address.City,
		&address.District,
		&address.Detail,
		&address.PostalCode,
		&address.IsDefault,
		&address.CreatedAt,
		&address.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Address{}, domain.ErrAddressNotFound
		}
		return domain.Address{}, err
	}
	return address, nil
}

func scanCartItem(row scanner) (domain.CartItem, error) {
	var item domain.CartItem
	var status string
	err := row.Scan(
		&item.UserID,
		&item.Quantity,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Product.ID,
		&item.Product.SKU,
		&item.Product.Title,
		&item.Product.Description,
		&item.Product.Category,
		&item.Product.CoverURL,
		&item.Product.GrantType,
		&item.Product.GrantKey,
		&item.Product.PriceCredits,
		&item.Product.Stock,
		&item.Product.SalesCount,
		&status,
		&item.Product.Sort,
		&item.Product.CreatedAt,
		&item.Product.UpdatedAt,
	)
	if err != nil {
		return domain.CartItem{}, err
	}
	item.Product.Status = domain.ProductStatus(status)
	return item, nil
}

func scanProductFavorite(row scanner) (domain.ProductFavorite, error) {
	var item domain.ProductFavorite
	var status string
	err := row.Scan(
		&item.UserID,
		&item.CreatedAt,
		&item.Product.ID,
		&item.Product.SKU,
		&item.Product.Title,
		&item.Product.Description,
		&item.Product.Category,
		&item.Product.CoverURL,
		&item.Product.GrantType,
		&item.Product.GrantKey,
		&item.Product.PriceCredits,
		&item.Product.Stock,
		&item.Product.SalesCount,
		&status,
		&item.Product.Sort,
		&item.Product.CreatedAt,
		&item.Product.UpdatedAt,
	)
	if err != nil {
		return domain.ProductFavorite{}, err
	}
	item.Product.Status = domain.ProductStatus(status)
	return item, nil
}

func scanCoupon(row scanner) (domain.Coupon, error) {
	var coupon domain.Coupon
	var status string
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	err := row.Scan(
		&coupon.ID,
		&coupon.Code,
		&coupon.Name,
		&coupon.Description,
		&coupon.DiscountCredits,
		&coupon.MinOrderCredits,
		&coupon.TotalQuota,
		&coupon.PerUserLimit,
		&coupon.ClaimedCount,
		&coupon.UsedCount,
		&status,
		&startsAt,
		&endsAt,
		&coupon.CreatedAt,
		&coupon.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Coupon{}, domain.ErrCouponNotFound
		}
		return domain.Coupon{}, err
	}
	coupon.Status = domain.CouponStatus(status)
	if startsAt.Valid {
		coupon.StartsAt = &startsAt.Time
	}
	if endsAt.Valid {
		coupon.EndsAt = &endsAt.Time
	}
	return coupon, nil
}

func scanCouponUsage(row scanner) (domain.CouponUsage, error) {
	var usage domain.CouponUsage
	var status string
	var orderID sql.NullInt64
	var usedAt sql.NullTime
	var releasedAt sql.NullTime
	err := row.Scan(
		&usage.ID,
		&usage.CouponID,
		&usage.Code,
		&usage.UserID,
		&orderID,
		&status,
		&usage.DiscountCredits,
		&usage.CreatedAt,
		&usedAt,
		&releasedAt,
		&usage.UpdatedAt,
	)
	if err != nil {
		return domain.CouponUsage{}, err
	}
	usage.Status = domain.CouponUsageStatus(status)
	if orderID.Valid {
		usage.OrderID = orderID.Int64
	}
	if usedAt.Valid {
		usage.UsedAt = &usedAt.Time
	}
	if releasedAt.Valid {
		usage.ReleasedAt = &releasedAt.Time
	}
	return usage, nil
}

func scanCouponUsageWithCoupon(row scanner) (domain.CouponUsage, error) {
	var usage domain.CouponUsage
	var usageStatus string
	var orderID sql.NullInt64
	var usedAt sql.NullTime
	var releasedAt sql.NullTime
	var couponStatus string
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	err := row.Scan(
		&usage.ID,
		&usage.CouponID,
		&usage.Code,
		&usage.UserID,
		&orderID,
		&usageStatus,
		&usage.DiscountCredits,
		&usage.CreatedAt,
		&usedAt,
		&releasedAt,
		&usage.UpdatedAt,
		&usage.Coupon.ID,
		&usage.Coupon.Code,
		&usage.Coupon.Name,
		&usage.Coupon.Description,
		&usage.Coupon.DiscountCredits,
		&usage.Coupon.MinOrderCredits,
		&usage.Coupon.TotalQuota,
		&usage.Coupon.PerUserLimit,
		&usage.Coupon.ClaimedCount,
		&usage.Coupon.UsedCount,
		&couponStatus,
		&startsAt,
		&endsAt,
		&usage.Coupon.CreatedAt,
		&usage.Coupon.UpdatedAt,
	)
	if err != nil {
		return domain.CouponUsage{}, err
	}
	usage.Status = domain.CouponUsageStatus(usageStatus)
	if orderID.Valid {
		usage.OrderID = orderID.Int64
	}
	if usedAt.Valid {
		usage.UsedAt = &usedAt.Time
	}
	if releasedAt.Valid {
		usage.ReleasedAt = &releasedAt.Time
	}
	usage.Coupon.Status = domain.CouponStatus(couponStatus)
	if startsAt.Valid {
		usage.Coupon.StartsAt = &startsAt.Time
	}
	if endsAt.Valid {
		usage.Coupon.EndsAt = &endsAt.Time
	}
	return usage, nil
}

func scanProductStockLog(row scanner) (domain.ProductStockLog, error) {
	var log domain.ProductStockLog
	err := row.Scan(
		&log.ID,
		&log.ProductID,
		&log.SKU,
		&log.Title,
		&log.Delta,
		&log.BeforeStock,
		&log.AfterStock,
		&log.Reason,
		&log.ReferenceType,
		&log.ReferenceID,
		&log.OperatorType,
		&log.OperatorID,
		&log.Note,
		&log.CreatedAt,
	)
	return log, err
}

func scanFinanceAnomaly(row scanner) (domain.FinanceAnomaly, error) {
	var anomaly domain.FinanceAnomaly
	var status string
	err := row.Scan(
		&anomaly.IssueType,
		&anomaly.OrderID,
		&anomaly.OrderNo,
		&anomaly.UserID,
		&status,
		&anomaly.OrderTotalCredits,
		&anomaly.SucceededPaymentCredits,
		&anomaly.RefundedCredits,
		&anomaly.DifferenceCredits,
		&anomaly.UpdatedAt,
	)
	anomaly.OrderStatus = domain.OrderStatus(status)
	return anomaly, err
}

func scanOutboxRequeueAudit(row scanner) (domain.OutboxRequeueAudit, error) {
	var audit domain.OutboxRequeueAudit
	err := row.Scan(
		&audit.ID,
		&audit.EventID,
		&audit.AggregateType,
		&audit.AggregateID,
		&audit.PreviousStatus,
		&audit.PreviousAttempts,
		&audit.PreviousError,
		&audit.OperatorID,
		&audit.RequeuedAt,
	)
	return audit, err
}

func scanOrderStatusLog(row scanner) (domain.OrderStatusLog, error) {
	var log domain.OrderStatusLog
	var fromStatus string
	var toStatus string
	if err := row.Scan(&log.ID, &log.OrderID, &fromStatus, &toStatus, &log.Reason, &log.OperatorType, &log.OperatorID, &log.Note, &log.CreatedAt); err != nil {
		return domain.OrderStatusLog{}, err
	}
	log.FromStatus = domain.OrderStatus(fromStatus)
	log.ToStatus = domain.OrderStatus(toStatus)
	return log, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS mall_product_categories (
	  id BIGSERIAL PRIMARY KEY,
	  slug TEXT NOT NULL UNIQUE,
	  name TEXT NOT NULL,
	  description TEXT NOT NULL DEFAULT '',
	  status TEXT NOT NULL,
	  sort INTEGER NOT NULL DEFAULT 0,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_categories_status_sort_created ON mall_product_categories (status, sort ASC, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_categories_created ON mall_product_categories (created_at DESC, id DESC)`,
	`CREATE TABLE IF NOT EXISTS mall_products (
	  id BIGSERIAL PRIMARY KEY,
	  sku TEXT NOT NULL UNIQUE,
	  title TEXT NOT NULL,
	  description TEXT NOT NULL DEFAULT '',
	  category TEXT NOT NULL DEFAULT '',
	  cover_url TEXT NOT NULL DEFAULT '',
	  grant_type TEXT NOT NULL DEFAULT '',
	  grant_key TEXT NOT NULL DEFAULT '',
	  price_credits BIGINT NOT NULL CHECK (price_credits >= 0),
	  stock BIGINT NOT NULL CHECK (stock >= 0),
	  sales_count BIGINT NOT NULL DEFAULT 0,
	  status TEXT NOT NULL,
	  sort INTEGER NOT NULL DEFAULT 0,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL
	)`,
	`ALTER TABLE mall_products ADD COLUMN IF NOT EXISTS grant_type TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_products ADD COLUMN IF NOT EXISTS grant_key TEXT NOT NULL DEFAULT ''`,
	`UPDATE mall_products
	 SET grant_key = CASE
	       WHEN COALESCE(grant_key, '') = '' THEN LOWER(sku)
	       ELSE grant_key
	     END,
	     grant_type = CASE
	       WHEN COALESCE(grant_type, '') <> '' THEN grant_type
	       WHEN LOWER(CASE WHEN COALESCE(grant_key, '') = '' THEN sku ELSE grant_key END) LIKE 'badge-%' THEN 'badge'
	       WHEN LOWER(CASE WHEN COALESCE(grant_key, '') = '' THEN sku ELSE grant_key END) LIKE 'theme-%' THEN 'theme'
	       WHEN LOWER(CASE WHEN COALESCE(grant_key, '') = '' THEN sku ELSE grant_key END) LIKE 'vip-%'
	         OR LOWER(CASE WHEN COALESCE(grant_key, '') = '' THEN sku ELSE grant_key END) LIKE 'member-%'
	         OR LOWER(CASE WHEN COALESCE(grant_key, '') = '' THEN sku ELSE grant_key END) LIKE '%membership%' THEN 'membership'
	       ELSE 'digital'
	     END
	 WHERE category = 'digital'
	   AND (COALESCE(grant_key, '') = '' OR COALESCE(grant_type, '') = '')`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_categories_lifecycle_check'
	       AND conrelid = 'mall_product_categories'::regclass
	   ) THEN
	     ALTER TABLE mall_product_categories
	     ADD CONSTRAINT mall_product_categories_lifecycle_check
	     CHECK (
	       BTRIM(slug) <> ''
	       AND BTRIM(name) <> ''
	       AND status = UPPER(TRIM(status))
	       AND status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_products_lifecycle_check'
	       AND conrelid = 'mall_products'::regclass
	   ) THEN
	     ALTER TABLE mall_products
	     ADD CONSTRAINT mall_products_lifecycle_check
	     CHECK (
	       BTRIM(sku) <> ''
	       AND BTRIM(title) <> ''
	       AND sales_count >= 0
	       AND status = UPPER(TRIM(status))
	       AND status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_products_grant_contract_check'
	       AND conrelid = 'mall_products'::regclass
	   ) THEN
	     ALTER TABLE mall_products
	     ADD CONSTRAINT mall_products_grant_contract_check
	     CHECK (
	       grant_type = LOWER(TRIM(grant_type))
	       AND grant_key = LOWER(TRIM(grant_key))
	       AND (
	         (grant_type = '' AND grant_key = '')
	         OR (grant_type IN ('badge', 'theme', 'membership', 'digital') AND grant_key <> '')
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_products_digital_grant_contract_check'
	       AND conrelid = 'mall_products'::regclass
	   ) THEN
	     ALTER TABLE mall_products
	     ADD CONSTRAINT mall_products_digital_grant_contract_check
	     CHECK (
	       LOWER(TRIM(category)) <> 'digital'
	       OR (grant_type IN ('badge', 'theme', 'membership', 'digital') AND grant_key <> '')
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_products_status_sort_created ON mall_products (status, sort ASC, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_products_category ON mall_products (category)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_products_grant ON mall_products (grant_type, grant_key)`,
	`INSERT INTO mall_product_categories (slug, name, description, status, sort, created_at, updated_at)
	 VALUES
	  ('digital', '数字权益', '会员、主题、徽章等在线发放商品。', 'ACTIVE', 10, NOW(), NOW()),
	  ('badge', '徽章权益', '个人徽章类数字权益商品。', 'ACTIVE', 12, NOW(), NOW()),
	  ('theme', '主题权益', '个人主页主题类数字权益商品。', 'ACTIVE', 14, NOW(), NOW()),
	  ('physical', '实物周边', '贴纸、纪念品等需要配送的实物商品。', 'ACTIVE', 20, NOW(), NOW())
	 ON CONFLICT (slug) DO NOTHING`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_products_category_fkey'
	       AND conrelid = 'mall_products'::regclass
	   ) THEN
	     ALTER TABLE mall_products
	     ADD CONSTRAINT mall_products_category_fkey
	     FOREIGN KEY (category) REFERENCES mall_product_categories(slug) ON DELETE RESTRICT NOT VALID;
	   END IF;
	 END $$`,
	`CREATE TABLE IF NOT EXISTS mall_orders (
	  id BIGSERIAL PRIMARY KEY,
	  order_no TEXT NOT NULL UNIQUE,
	  idempotency_key TEXT NOT NULL,
	  user_id BIGINT NOT NULL,
	  original_credits BIGINT NOT NULL DEFAULT 0 CHECK (original_credits >= 0),
	  discount_credits BIGINT NOT NULL DEFAULT 0 CHECK (discount_credits >= 0),
	  total_credits BIGINT NOT NULL CHECK (total_credits >= 0),
	  coupon_id BIGINT,
	  coupon_code TEXT NOT NULL DEFAULT '',
	  status TEXT NOT NULL,
	  receiver TEXT NOT NULL DEFAULT '',
	  phone TEXT NOT NULL DEFAULT '',
	  address TEXT NOT NULL DEFAULT '',
	  payment_method TEXT NOT NULL DEFAULT '',
	  paid_at TIMESTAMPTZ,
	  shipping_carrier TEXT NOT NULL DEFAULT '',
	  tracking_no TEXT NOT NULL DEFAULT '',
	  shipped_at TIMESTAMPTZ,
	  completed_at TIMESTAMPTZ,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL
	)`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS shipping_carrier TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS tracking_no TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS shipped_at TIMESTAMPTZ`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS original_credits BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS discount_credits BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS coupon_id BIGINT`,
	`ALTER TABLE mall_orders ADD COLUMN IF NOT EXISTS coupon_code TEXT NOT NULL DEFAULT ''`,
	`UPDATE mall_orders SET original_credits = total_credits WHERE original_credits = 0 AND total_credits > 0`,
	`UPDATE mall_orders
	 SET shipped_at = updated_at
	 WHERE status = 'SHIPPED'
	   AND shipped_at IS NULL`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_orders_financial_snapshot_check'
	       AND conrelid = 'mall_orders'::regclass
	   ) THEN
	     ALTER TABLE mall_orders
	     ADD CONSTRAINT mall_orders_financial_snapshot_check
	     CHECK (
	       original_credits >= discount_credits
	       AND total_credits = original_credits - discount_credits
	       AND (
	         (coupon_id IS NULL AND coupon_code = '' AND discount_credits = 0)
	         OR (coupon_id IS NOT NULL AND coupon_code = UPPER(TRIM(coupon_code)) AND coupon_code <> '')
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_orders_lifecycle_check'
	       AND conrelid = 'mall_orders'::regclass
	   ) THEN
	     ALTER TABLE mall_orders
	     ADD CONSTRAINT mall_orders_lifecycle_check
	     CHECK (
	       user_id > 0
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
	`ALTER TABLE mall_orders DROP CONSTRAINT IF EXISTS mall_orders_idempotency_key_key`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_orders_user_idempotency_key ON mall_orders (user_id, idempotency_key)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_orders_id_user ON mall_orders (id, user_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_orders_refund_snapshot ON mall_orders (id, user_id, order_no, total_credits)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_orders_coupon_usage_snapshot ON mall_orders (id, user_id, coupon_id, coupon_code, discount_credits)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_orders_user_created ON mall_orders (user_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_orders_status_created ON mall_orders (status, created_at DESC, id DESC)`,
	`CREATE TABLE IF NOT EXISTS mall_order_items (
	  order_id BIGINT NOT NULL REFERENCES mall_orders(id) ON DELETE CASCADE,
	  product_id BIGINT NOT NULL REFERENCES mall_products(id),
	  sku TEXT NOT NULL,
	  title TEXT NOT NULL,
	  category TEXT NOT NULL DEFAULT '',
	  grant_type TEXT NOT NULL DEFAULT '',
	  grant_key TEXT NOT NULL DEFAULT '',
	  quantity INTEGER NOT NULL CHECK (quantity > 0),
	  unit_price_credits BIGINT NOT NULL CHECK (unit_price_credits >= 0),
	  subtotal_credits BIGINT NOT NULL CHECK (subtotal_credits >= 0),
	  PRIMARY KEY (order_id, product_id)
	)`,
	`ALTER TABLE mall_order_items ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_order_items ADD COLUMN IF NOT EXISTS grant_type TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_order_items ADD COLUMN IF NOT EXISTS grant_key TEXT NOT NULL DEFAULT ''`,
	`UPDATE mall_order_items oi
	 SET category = p.category
	 FROM mall_products p
	 WHERE oi.product_id = p.id
	   AND COALESCE(oi.category, '') = ''`,
	`UPDATE mall_order_items oi
	 SET grant_type = p.grant_type,
	     grant_key = p.grant_key
	 FROM mall_products p
	 WHERE oi.product_id = p.id
	   AND COALESCE(oi.category, '') = 'digital'
	   AND (COALESCE(oi.grant_type, '') = '' OR COALESCE(oi.grant_key, '') = '')`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_order_items_financial_snapshot_check'
	       AND conrelid = 'mall_order_items'::regclass
	   ) THEN
	     ALTER TABLE mall_order_items
	     ADD CONSTRAINT mall_order_items_financial_snapshot_check
	     CHECK (subtotal_credits = quantity::BIGINT * unit_price_credits) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_order_items_snapshot_check'
	       AND conrelid = 'mall_order_items'::regclass
	   ) THEN
	     ALTER TABLE mall_order_items
	     ADD CONSTRAINT mall_order_items_snapshot_check
	     CHECK (
	       BTRIM(sku) <> ''
	       AND BTRIM(title) <> ''
	       AND BTRIM(category) <> ''
	       AND grant_type = LOWER(TRIM(grant_type))
	       AND grant_key = LOWER(TRIM(grant_key))
	       AND (
	         (grant_type = '' AND grant_key = '')
	         OR (grant_type IN ('badge', 'theme', 'membership', 'digital') AND grant_key <> '')
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_order_items_digital_grant_contract_check'
	       AND conrelid = 'mall_order_items'::regclass
	   ) THEN
	     ALTER TABLE mall_order_items
	     ADD CONSTRAINT mall_order_items_digital_grant_contract_check
	     CHECK (
	       LOWER(TRIM(category)) <> 'digital'
	       OR (grant_type IN ('badge', 'theme', 'membership', 'digital') AND grant_key <> '')
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_order_items_product ON mall_order_items (product_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_order_items_entitlement_grant ON mall_order_items (order_id, product_id, grant_type, grant_key)`,
	`CREATE TABLE IF NOT EXISTS mall_digital_entitlements (
	  id BIGSERIAL PRIMARY KEY,
	  order_id BIGINT NOT NULL REFERENCES mall_orders(id) ON DELETE CASCADE,
	  product_id BIGINT NOT NULL REFERENCES mall_products(id),
	  user_id BIGINT NOT NULL,
	  sku TEXT NOT NULL,
	  title TEXT NOT NULL,
	  quantity INTEGER NOT NULL CHECK (quantity > 0),
	  fulfillment_code TEXT NOT NULL UNIQUE,
	  grant_type TEXT NOT NULL DEFAULT '',
	  grant_key TEXT NOT NULL DEFAULT '',
	  status TEXT NOT NULL DEFAULT '',
	  issued_at TIMESTAMPTZ NOT NULL,
	  expires_at TIMESTAMPTZ,
	  revoked_at TIMESTAMPTZ,
	  refund_id BIGINT,
	  revoked_by TEXT NOT NULL DEFAULT '',
	  revoke_reason TEXT NOT NULL DEFAULT '',
	  created_at TIMESTAMPTZ NOT NULL
	)`,
	`ALTER TABLE mall_digital_entitlements DROP CONSTRAINT IF EXISTS mall_digital_entitlements_order_id_product_id_key`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS grant_type TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_digital_entitlements ALTER COLUMN grant_type SET DEFAULT ''`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS grant_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_digital_entitlements ALTER COLUMN status SET DEFAULT ''`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS refund_id BIGINT`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS revoked_by TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE mall_digital_entitlements ADD COLUMN IF NOT EXISTS revoke_reason TEXT NOT NULL DEFAULT ''`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_order_user_fkey'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_order_user_fkey
	     FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_order_item_fkey'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_order_item_fkey
	     FOREIGN KEY (order_id, product_id) REFERENCES mall_order_items(order_id, product_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_order_grant_fkey'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_order_grant_fkey
	     FOREIGN KEY (order_id, product_id, grant_type, grant_key) REFERENCES mall_order_items(order_id, product_id, grant_type, grant_key) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`UPDATE mall_digital_entitlements
	 SET expires_at = issued_at + interval '30 days'
	 WHERE grant_type = 'membership'
	   AND expires_at IS NULL`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_grant_status_normalized_check'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_grant_status_normalized_check
	     CHECK (
	       grant_type = LOWER(TRIM(grant_type))
	       AND grant_type <> ''
	       AND grant_key = LOWER(TRIM(grant_key))
	       AND grant_key <> ''
	       AND status = UPPER(TRIM(status))
	       AND status IN ('ACTIVE', 'REVOKED')
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_grant_type_check'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_grant_type_check
	     CHECK (grant_type IN ('badge', 'theme', 'membership', 'digital')) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_lifecycle_check'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_lifecycle_check
	     CHECK (
	       (expires_at IS NULL OR expires_at >= issued_at)
	       AND (
	         (status = 'ACTIVE' AND revoked_at IS NULL AND refund_id IS NULL AND revoked_by = '' AND revoke_reason = '')
	         OR (
	           status = 'REVOKED'
	           AND revoked_at IS NOT NULL
	           AND (refund_id IS NOT NULL OR (BTRIM(revoked_by) <> '' AND BTRIM(revoke_reason) <> ''))
	         )
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_membership_expiry_check'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_membership_expiry_check
	     CHECK (
	       LOWER(TRIM(grant_type)) <> 'membership'
	       OR UPPER(TRIM(status)) <> 'ACTIVE'
	       OR revoked_at IS NOT NULL
	       OR expires_at IS NOT NULL
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_digital_entitlements_user_created ON mall_digital_entitlements (user_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_digital_entitlements_user_status_issued ON mall_digital_entitlements (user_id, status, issued_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_digital_entitlements_user_grant ON mall_digital_entitlements (user_id, grant_type, grant_key, status)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_digital_entitlements_user_grant_expiry ON mall_digital_entitlements (user_id, grant_type, grant_key, status, expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_digital_entitlements_order_status ON mall_digital_entitlements (order_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_digital_entitlements_order_product ON mall_digital_entitlements (order_id, product_id, id)`,
	`CREATE TABLE IF NOT EXISTS mall_payments (
	  id BIGSERIAL PRIMARY KEY,
	  order_id BIGINT NOT NULL REFERENCES mall_orders(id) ON DELETE CASCADE,
	  user_id BIGINT NOT NULL,
	  amount_credits BIGINT NOT NULL CHECK (amount_credits >= 0),
	  provider TEXT NOT NULL,
	  idempotency_key TEXT NOT NULL,
	  status TEXT NOT NULL,
	  provider_trade_no TEXT NOT NULL DEFAULT '',
	  failure_reason TEXT NOT NULL DEFAULT '',
	  paid_at TIMESTAMPTZ,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL,
	  UNIQUE (provider, idempotency_key)
	)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_payments_lifecycle_check'
	       AND conrelid = 'mall_payments'::regclass
	   ) THEN
	     ALTER TABLE mall_payments
	     ADD CONSTRAINT mall_payments_lifecycle_check
	     CHECK (
	       status = UPPER(TRIM(status))
	       AND (
	         (status = 'PENDING' AND paid_at IS NULL AND provider_trade_no = '' AND failure_reason = '')
	         OR (status = 'SUCCEEDED' AND paid_at IS NOT NULL AND provider_trade_no <> '' AND failure_reason = '')
	         OR (status = 'FAILED' AND paid_at IS NULL AND provider_trade_no = '' AND failure_reason <> '')
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_payments_order_user_fkey'
	       AND conrelid = 'mall_payments'::regclass
	   ) THEN
	     ALTER TABLE mall_payments
	     ADD CONSTRAINT mall_payments_order_user_fkey
	     FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_payments_order_created ON mall_payments (order_id, created_at ASC, id ASC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_payments_user_created ON mall_payments (user_id, created_at DESC, id DESC)`,
	`CREATE TABLE IF NOT EXISTS mall_cart_items (
	  user_id BIGINT NOT NULL,
	  product_id BIGINT NOT NULL REFERENCES mall_products(id) ON DELETE CASCADE,
	  quantity INTEGER NOT NULL CHECK (quantity > 0),
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL,
	  PRIMARY KEY (user_id, product_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_cart_items_user_updated ON mall_cart_items (user_id, updated_at DESC, product_id DESC)`,
	`CREATE TABLE IF NOT EXISTS mall_product_favorites (
	  user_id BIGINT NOT NULL,
	  product_id BIGINT NOT NULL REFERENCES mall_products(id) ON DELETE CASCADE,
	  created_at TIMESTAMPTZ NOT NULL,
	  PRIMARY KEY (user_id, product_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_favorites_user_created ON mall_product_favorites (user_id, created_at DESC, product_id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_favorites_product_created ON mall_product_favorites (product_id, created_at DESC)`,
	`CREATE TABLE IF NOT EXISTS mall_coupons (
	  id BIGSERIAL PRIMARY KEY,
	  code TEXT NOT NULL UNIQUE,
	  name TEXT NOT NULL,
	  description TEXT NOT NULL DEFAULT '',
	  discount_credits BIGINT NOT NULL CHECK (discount_credits > 0),
	  min_order_credits BIGINT NOT NULL DEFAULT 0 CHECK (min_order_credits >= 0),
	  total_quota BIGINT NOT NULL DEFAULT 0 CHECK (total_quota >= 0),
	  per_user_limit BIGINT NOT NULL DEFAULT 1 CHECK (per_user_limit >= 0),
	  status TEXT NOT NULL,
	  starts_at TIMESTAMPTZ,
	  ends_at TIMESTAMPTZ,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_coupons_status_window ON mall_coupons (status, starts_at, ends_at)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_coupons_created ON mall_coupons (created_at DESC, id DESC)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_coupons_code_ci ON mall_coupons (LOWER(TRIM(code)))`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_orders_coupon_id_fkey'
	       AND conrelid = 'mall_orders'::regclass
	   ) THEN
	     ALTER TABLE mall_orders
	     ADD CONSTRAINT mall_orders_coupon_id_fkey
	     FOREIGN KEY (coupon_id) REFERENCES mall_coupons(id) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_coupons_code_normalized_check'
	       AND conrelid = 'mall_coupons'::regclass
	   ) THEN
	     ALTER TABLE mall_coupons
	     ADD CONSTRAINT mall_coupons_code_normalized_check
	     CHECK (code = UPPER(TRIM(code)) AND code <> '') NOT VALID;
	   END IF;
	 END $$`,
	`CREATE TABLE IF NOT EXISTS mall_coupon_usages (
	  id BIGSERIAL PRIMARY KEY,
	  coupon_id BIGINT NOT NULL REFERENCES mall_coupons(id) ON DELETE CASCADE,
	  code TEXT NOT NULL,
	  user_id BIGINT NOT NULL,
	  order_id BIGINT REFERENCES mall_orders(id) ON DELETE CASCADE,
	  status TEXT NOT NULL,
	  discount_credits BIGINT NOT NULL CHECK (discount_credits >= 0),
	  created_at TIMESTAMPTZ NOT NULL,
	  used_at TIMESTAMPTZ,
	  released_at TIMESTAMPTZ,
	  updated_at TIMESTAMPTZ NOT NULL,
	  UNIQUE (order_id)
	)`,
	`ALTER TABLE mall_coupon_usages ALTER COLUMN order_id DROP NOT NULL`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_coupon_usages_lifecycle_check'
	       AND conrelid = 'mall_coupon_usages'::regclass
	   ) THEN
	     ALTER TABLE mall_coupon_usages
	     ADD CONSTRAINT mall_coupon_usages_lifecycle_check
	     CHECK (
	       status = UPPER(TRIM(status))
	       AND code = UPPER(TRIM(code))
	       AND code <> ''
	       AND (
	         (status = 'CLAIMED' AND order_id IS NULL AND used_at IS NULL AND released_at IS NULL)
	         OR (status = 'RESERVED' AND order_id IS NOT NULL AND used_at IS NULL AND released_at IS NULL)
	         OR (status = 'USED' AND order_id IS NOT NULL AND used_at IS NOT NULL AND released_at IS NULL)
	         OR (status = 'RELEASED' AND order_id IS NOT NULL AND used_at IS NULL AND released_at IS NOT NULL)
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_coupon_usages_order_user_fkey'
	       AND conrelid = 'mall_coupon_usages'::regclass
	   ) THEN
	     ALTER TABLE mall_coupon_usages
	     ADD CONSTRAINT mall_coupon_usages_order_user_fkey
	     FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_coupon_usages_order_coupon_snapshot_fkey'
	       AND conrelid = 'mall_coupon_usages'::regclass
	   ) THEN
	     ALTER TABLE mall_coupon_usages
	     ADD CONSTRAINT mall_coupon_usages_order_coupon_snapshot_fkey
	     FOREIGN KEY (order_id, user_id, coupon_id, code, discount_credits) REFERENCES mall_orders(id, user_id, coupon_id, coupon_code, discount_credits) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_coupon_usages_coupon_status ON mall_coupon_usages (coupon_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_coupon_usages_user_coupon ON mall_coupon_usages (user_id, coupon_id, status)`,
	`CREATE TABLE IF NOT EXISTS mall_addresses (
	  id BIGSERIAL PRIMARY KEY,
	  user_id BIGINT NOT NULL,
	  receiver TEXT NOT NULL,
	  phone TEXT NOT NULL,
	  province TEXT NOT NULL DEFAULT '',
	  city TEXT NOT NULL DEFAULT '',
	  district TEXT NOT NULL DEFAULT '',
	  detail TEXT NOT NULL,
	  postal_code TEXT NOT NULL DEFAULT '',
	  is_default BOOLEAN NOT NULL DEFAULT false,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL
	)`,
	`WITH ranked AS (
	  SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY updated_at DESC, id DESC) AS rn
	  FROM mall_addresses
	  WHERE is_default = true
	)
	UPDATE mall_addresses a
	SET is_default = false
	FROM ranked r
	WHERE a.id = r.id
	  AND r.rn > 1`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_addresses_one_default_per_user ON mall_addresses (user_id) WHERE is_default = true`,
	`CREATE INDEX IF NOT EXISTS idx_mall_addresses_user_default_updated ON mall_addresses (user_id, is_default DESC, updated_at DESC, id DESC)`,
	`CREATE TABLE IF NOT EXISTS mall_refund_requests (
	  id BIGSERIAL PRIMARY KEY,
	  order_id BIGINT NOT NULL REFERENCES mall_orders(id) ON DELETE CASCADE,
	  order_no TEXT NOT NULL,
	  user_id BIGINT NOT NULL,
	  amount_credits BIGINT NOT NULL CHECK (amount_credits >= 0),
	  status TEXT NOT NULL,
	  reason TEXT NOT NULL DEFAULT '',
	  user_note TEXT NOT NULL DEFAULT '',
	  admin_note TEXT NOT NULL DEFAULT '',
	  restore_stock BOOLEAN NOT NULL DEFAULT false,
	  operator_id TEXT NOT NULL DEFAULT '',
	  requested_at TIMESTAMPTZ NOT NULL,
	  reviewed_at TIMESTAMPTZ,
	  refunded_at TIMESTAMPTZ,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL,
	  UNIQUE (order_id)
	)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_refund_requests_lifecycle_check'
	       AND conrelid = 'mall_refund_requests'::regclass
	   ) THEN
	     ALTER TABLE mall_refund_requests
	     ADD CONSTRAINT mall_refund_requests_lifecycle_check
	     CHECK (
	       status = UPPER(TRIM(status))
	       AND (
	         (status = 'REQUESTED' AND operator_id = '' AND reviewed_at IS NULL AND refunded_at IS NULL AND restore_stock = false)
	         OR (status = 'PROCESSING' AND BTRIM(operator_id) <> '' AND reviewed_at IS NOT NULL AND refunded_at IS NULL)
	         OR (status = 'APPROVED' AND BTRIM(operator_id) <> '' AND reviewed_at IS NOT NULL AND refunded_at IS NOT NULL)
	         OR (status = 'REJECTED' AND BTRIM(operator_id) <> '' AND reviewed_at IS NOT NULL AND refunded_at IS NULL AND restore_stock = false)
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_refund_requests_order_user_fkey'
	       AND conrelid = 'mall_refund_requests'::regclass
	   ) THEN
	     ALTER TABLE mall_refund_requests
	     ADD CONSTRAINT mall_refund_requests_order_user_fkey
	     FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_refund_requests_order_snapshot_fkey'
	       AND conrelid = 'mall_refund_requests'::regclass
	   ) THEN
	     ALTER TABLE mall_refund_requests
	     ADD CONSTRAINT mall_refund_requests_order_snapshot_fkey
	     FOREIGN KEY (order_id, user_id, order_no, amount_credits) REFERENCES mall_orders(id, user_id, order_no, total_credits) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_mall_refund_requests_id_order_user ON mall_refund_requests (id, order_id, user_id)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_digital_entitlements_refund_order_user_fkey'
	       AND conrelid = 'mall_digital_entitlements'::regclass
	   ) THEN
	     ALTER TABLE mall_digital_entitlements
	     ADD CONSTRAINT mall_digital_entitlements_refund_order_user_fkey
	     FOREIGN KEY (refund_id, order_id, user_id) REFERENCES mall_refund_requests(id, order_id, user_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_refund_requests_user_created ON mall_refund_requests (user_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_refund_requests_status_created ON mall_refund_requests (status, created_at DESC, id DESC)`,
	`CREATE TABLE IF NOT EXISTS mall_order_status_logs (
	  id BIGSERIAL PRIMARY KEY,
	  order_id BIGINT NOT NULL REFERENCES mall_orders(id) ON DELETE CASCADE,
	  from_status TEXT NOT NULL DEFAULT '',
	  to_status TEXT NOT NULL,
	  reason TEXT NOT NULL,
	  operator_type TEXT NOT NULL DEFAULT '',
	  operator_id TEXT NOT NULL DEFAULT '',
	  note TEXT NOT NULL DEFAULT '',
	  created_at TIMESTAMPTZ NOT NULL
	)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_order_status_logs_normalized_check'
	       AND conrelid = 'mall_order_status_logs'::regclass
	   ) THEN
	     ALTER TABLE mall_order_status_logs
	     ADD CONSTRAINT mall_order_status_logs_normalized_check
	     CHECK (
	       (from_status = '' OR from_status IN ('PENDING_PAYMENT', 'PAYING', 'PAID', 'CANCELED', 'SHIPPED', 'COMPLETED', 'CLOSED', 'REFUNDED'))
	       AND to_status IN ('PENDING_PAYMENT', 'PAYING', 'PAID', 'CANCELED', 'SHIPPED', 'COMPLETED', 'CLOSED', 'REFUNDED')
	       AND BTRIM(reason) <> ''
	       AND operator_type IN ('user', 'admin')
	       AND BTRIM(operator_id) <> ''
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_order_status_logs_transition_check'
	       AND conrelid = 'mall_order_status_logs'::regclass
	   ) THEN
	     ALTER TABLE mall_order_status_logs
	     ADD CONSTRAINT mall_order_status_logs_transition_check
	     CHECK (
	       reason = LOWER(TRIM(reason))
	       AND (
	         (from_status = '' AND to_status = 'PENDING_PAYMENT' AND reason = 'created' AND operator_type = 'user')
	         OR (from_status = 'PENDING_PAYMENT' AND to_status = 'PAYING' AND reason = 'paying' AND operator_type = 'user')
	         OR (from_status = 'PAYING' AND to_status = 'PAID' AND reason = 'paid' AND operator_type = 'user')
	         OR (from_status = 'PAYING' AND to_status = 'COMPLETED' AND reason = 'completed' AND operator_type = 'user')
	         OR (from_status = 'PAYING' AND to_status = 'PENDING_PAYMENT' AND reason = 'payment_failed' AND operator_type = 'user')
	         OR (from_status = 'PENDING_PAYMENT' AND to_status = 'CANCELED' AND reason = 'canceled_by_user' AND operator_type = 'user')
	         OR (from_status IN ('PENDING_PAYMENT', 'PAYING') AND to_status = 'CLOSED' AND reason = 'expired' AND operator_type = 'admin')
	         OR (from_status = 'PAID' AND to_status = 'SHIPPED' AND reason = 'shipped' AND operator_type = 'admin')
	         OR (from_status IN ('PAID', 'SHIPPED') AND to_status = 'COMPLETED' AND reason = 'completed' AND operator_type = 'admin')
	         OR (from_status = 'SHIPPED' AND to_status = 'COMPLETED' AND reason = 'completed' AND operator_type = 'user')
	         OR (from_status IN ('PAID', 'SHIPPED', 'COMPLETED') AND to_status = 'REFUNDED' AND reason = 'refunded' AND operator_type = 'admin')
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_order_status_logs_order_created ON mall_order_status_logs (order_id, created_at ASC, id ASC)`,
	`CREATE TABLE IF NOT EXISTS mall_product_stock_logs (
	  id BIGSERIAL PRIMARY KEY,
	  product_id BIGINT NOT NULL REFERENCES mall_products(id) ON DELETE CASCADE,
	  sku TEXT NOT NULL DEFAULT '',
	  title TEXT NOT NULL DEFAULT '',
	  delta BIGINT NOT NULL,
	  before_stock BIGINT NOT NULL,
	  after_stock BIGINT NOT NULL CHECK (after_stock >= 0),
	  reason TEXT NOT NULL,
	  reference_type TEXT NOT NULL DEFAULT '',
	  reference_id BIGINT NOT NULL DEFAULT 0,
	  operator_type TEXT NOT NULL DEFAULT '',
	  operator_id TEXT NOT NULL DEFAULT '',
	  note TEXT NOT NULL DEFAULT '',
	  created_at TIMESTAMPTZ NOT NULL
	)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_stock_logs_snapshot_check'
	       AND conrelid = 'mall_product_stock_logs'::regclass
	   ) THEN
	     ALTER TABLE mall_product_stock_logs
	     ADD CONSTRAINT mall_product_stock_logs_snapshot_check
	     CHECK (after_stock = before_stock + delta) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_stock_logs_audit_contract_check'
	       AND conrelid = 'mall_product_stock_logs'::regclass
	   ) THEN
	     ALTER TABLE mall_product_stock_logs
	     ADD CONSTRAINT mall_product_stock_logs_audit_contract_check
	     CHECK (
	       BTRIM(sku) <> ''
	       AND BTRIM(title) <> ''
	       AND reason = LOWER(TRIM(reason))
	       AND reference_type = LOWER(TRIM(reference_type))
	       AND reference_id > 0
	       AND operator_type = LOWER(TRIM(operator_type))
	       AND BTRIM(operator_id) <> ''
	       AND (
	         (reason = 'product_created' AND reference_type = 'product' AND reference_id = product_id AND operator_type = 'admin' AND before_stock = 0 AND delta >= 0)
	         OR (reason = 'manual_adjustment' AND reference_type = 'product' AND reference_id = product_id AND operator_type = 'admin')
	         OR (reason = 'order_created' AND reference_type = 'order' AND operator_type = 'user' AND delta < 0)
	         OR (reason = 'order_canceled' AND reference_type = 'order' AND operator_type = 'user' AND delta > 0)
	         OR (reason = 'order_expired' AND reference_type = 'order' AND operator_type = 'admin' AND delta > 0)
	         OR (reason = 'refund_restored' AND reference_type = 'refund' AND operator_type = 'admin' AND delta > 0)
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`ALTER TABLE mall_product_stock_logs
	 ADD COLUMN IF NOT EXISTS order_reference_id BIGINT GENERATED ALWAYS AS (
	   CASE WHEN reference_type = 'order' THEN reference_id ELSE NULL END
	 ) STORED`,
	`ALTER TABLE mall_product_stock_logs
	 ADD COLUMN IF NOT EXISTS refund_reference_id BIGINT GENERATED ALWAYS AS (
	   CASE WHEN reference_type = 'refund' THEN reference_id ELSE NULL END
	 ) STORED`,
	`DO $$
	 BEGIN
	   IF EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_stock_logs_order_item_fkey'
	       AND conrelid = 'mall_product_stock_logs'::regclass
	       AND NOT (condeferrable AND condeferred)
	   ) THEN
	     ALTER TABLE mall_product_stock_logs
	     DROP CONSTRAINT mall_product_stock_logs_order_item_fkey;
	   END IF;
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_stock_logs_order_item_fkey'
	       AND conrelid = 'mall_product_stock_logs'::regclass
	   ) THEN
	     ALTER TABLE mall_product_stock_logs
	     ADD CONSTRAINT mall_product_stock_logs_order_item_fkey
	     FOREIGN KEY (order_reference_id, product_id) REFERENCES mall_order_items(order_id, product_id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_stock_logs_refund_fkey'
	       AND conrelid = 'mall_product_stock_logs'::regclass
	   ) THEN
	     ALTER TABLE mall_product_stock_logs
	     ADD CONSTRAINT mall_product_stock_logs_refund_fkey
	     FOREIGN KEY (refund_reference_id) REFERENCES mall_refund_requests(id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_stock_logs_product_created ON mall_product_stock_logs (product_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_stock_logs_reason_created ON mall_product_stock_logs (reason, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_stock_logs_reference ON mall_product_stock_logs (reference_type, reference_id)`,
	`CREATE TABLE IF NOT EXISTS mall_product_reviews (
	  id BIGSERIAL PRIMARY KEY,
	  product_id BIGINT NOT NULL REFERENCES mall_products(id) ON DELETE CASCADE,
	  order_id BIGINT NOT NULL REFERENCES mall_orders(id) ON DELETE CASCADE,
	  user_id BIGINT NOT NULL,
	  rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
	  content TEXT NOT NULL,
	  status TEXT NOT NULL,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL,
	  UNIQUE (user_id, order_id, product_id)
	)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_reviews_lifecycle_check'
	       AND conrelid = 'mall_product_reviews'::regclass
	   ) THEN
	     ALTER TABLE mall_product_reviews
	     ADD CONSTRAINT mall_product_reviews_lifecycle_check
	     CHECK (
	       user_id > 0
	       AND status = UPPER(TRIM(status))
	       AND status IN ('PENDING', 'PUBLISHED', 'HIDDEN')
	       AND BTRIM(content) <> ''
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_reviews_order_user_fkey'
	       AND conrelid = 'mall_product_reviews'::regclass
	   ) THEN
	     ALTER TABLE mall_product_reviews
	     ADD CONSTRAINT mall_product_reviews_order_user_fkey
	     FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_product_reviews_order_item_fkey'
	       AND conrelid = 'mall_product_reviews'::regclass
	   ) THEN
	     ALTER TABLE mall_product_reviews
	     ADD CONSTRAINT mall_product_reviews_order_item_fkey
	     FOREIGN KEY (order_id, product_id) REFERENCES mall_order_items(order_id, product_id) ON DELETE CASCADE NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_reviews_product_status_created ON mall_product_reviews (product_id, status, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_reviews_user_created ON mall_product_reviews (user_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_product_reviews_status_created ON mall_product_reviews (status, created_at DESC, id DESC)`,
	`INSERT INTO mall_product_stock_logs (
	  product_id, sku, title, delta, before_stock, after_stock, reason, reference_type, reference_id, operator_type, operator_id, note, created_at
	)
	SELECT p.id, p.sku, p.title, p.stock, 0, p.stock, 'product_created', 'product', p.id, 'admin', 'system', '历史商品初始化库存', p.created_at
	FROM mall_products p
	WHERE NOT EXISTS (
	  SELECT 1
	  FROM mall_product_stock_logs l
	  WHERE l.product_id = p.id
	    AND l.reason = 'product_created'
	    AND l.reference_type = 'product'
	    AND l.reference_id = p.id
	)`,
	`CREATE TABLE IF NOT EXISTS mall_outbox_events (
	  event_id TEXT PRIMARY KEY,
	  aggregate_type TEXT NOT NULL,
	  aggregate_id BIGINT NOT NULL,
	  event_type TEXT NOT NULL,
	  message_key TEXT NOT NULL DEFAULT '',
	  payload_json JSONB NOT NULL,
	  status TEXT NOT NULL,
	  attempts INTEGER NOT NULL DEFAULT 0,
	  lease_owner TEXT NOT NULL DEFAULT '',
	  lease_expires_at TIMESTAMPTZ,
	  last_error TEXT NOT NULL DEFAULT '',
	  next_attempt_at TIMESTAMPTZ,
	  published_at TIMESTAMPTZ,
	  created_at TIMESTAMPTZ NOT NULL,
	  updated_at TIMESTAMPTZ NOT NULL
	)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_outbox_events_lifecycle_check'
	       AND conrelid = 'mall_outbox_events'::regclass
	   ) THEN
	     ALTER TABLE mall_outbox_events
	     ADD CONSTRAINT mall_outbox_events_lifecycle_check
	     CHECK (
	       attempts >= 0
	       AND (
	         (status = 'pending' AND lease_owner = '' AND lease_expires_at IS NULL AND last_error = '' AND next_attempt_at IS NULL AND published_at IS NULL)
	         OR (status = 'publishing' AND BTRIM(lease_owner) <> '' AND lease_expires_at IS NOT NULL AND published_at IS NULL)
	         OR (status = 'failed' AND lease_owner = '' AND lease_expires_at IS NULL AND last_error <> '' AND next_attempt_at IS NOT NULL AND published_at IS NULL)
	         OR (status = 'dead_letter' AND lease_owner = '' AND lease_expires_at IS NULL AND last_error <> '' AND next_attempt_at IS NULL AND published_at IS NULL)
	         OR (status = 'published' AND lease_owner = '' AND lease_expires_at IS NULL AND last_error = '' AND published_at IS NOT NULL)
	       )
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_outbox_status_created ON mall_outbox_events (status, created_at ASC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_outbox_aggregate ON mall_outbox_events (aggregate_type, aggregate_id)`,
	`CREATE TABLE IF NOT EXISTS mall_outbox_requeue_audits (
	  id BIGSERIAL PRIMARY KEY,
	  event_id TEXT NOT NULL,
	  aggregate_type TEXT NOT NULL,
	  aggregate_id BIGINT NOT NULL,
	  previous_status TEXT NOT NULL,
	  previous_attempts INTEGER NOT NULL,
	  previous_error TEXT NOT NULL DEFAULT '',
	  operator_id TEXT NOT NULL,
	  requeued_at TIMESTAMPTZ NOT NULL
	)`,
	`DO $$
	 BEGIN
	   IF NOT EXISTS (
	     SELECT 1
	     FROM pg_constraint
	     WHERE conname = 'mall_outbox_requeue_audits_recovery_check'
	       AND conrelid = 'mall_outbox_requeue_audits'::regclass
	   ) THEN
	     ALTER TABLE mall_outbox_requeue_audits
	     ADD CONSTRAINT mall_outbox_requeue_audits_recovery_check
	     CHECK (
	       BTRIM(event_id) <> ''
	       AND BTRIM(aggregate_type) <> ''
	       AND aggregate_id > 0
	       AND previous_status = LOWER(TRIM(previous_status))
	       AND previous_status IN ('failed', 'dead_letter')
	       AND previous_attempts > 0
	       AND BTRIM(previous_error) <> ''
	       AND BTRIM(operator_id) <> ''
	     ) NOT VALID;
	   END IF;
	 END $$`,
	`CREATE INDEX IF NOT EXISTS idx_mall_outbox_requeue_audits_event ON mall_outbox_requeue_audits (event_id, requeued_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mall_outbox_requeue_audits_aggregate ON mall_outbox_requeue_audits (aggregate_type, aggregate_id, requeued_at DESC)`,
	`INSERT INTO mall_products (sku, title, description, category, cover_url, grant_type, grant_key, price_credits, stock, status, sort, created_at, updated_at)
	 VALUES
	  ('badge-founder', '创始会员徽章', '站长推荐的社区身份标识，适合早期活跃用户兑换。', 'digital', '/images/shop/badge-founder.svg', 'badge', 'badge-founder', 80, 9999, 'ACTIVE', 10, NOW(), NOW()),
	  ('vip-month', '会员月卡', '开通 30 天会员权益，解锁悬赏问答和会员身份展示。', 'digital', '/images/shop/vip-month.svg', 'membership', 'vip-month', 300, 1000, 'ACTIVE', 15, NOW(), NOW()),
	  ('theme-pro', '高级主题包', '解锁个人主页高级主题和资料卡装饰。', 'digital', '/images/shop/theme-pro.svg', 'theme', 'theme-pro', 500, 500, 'ACTIVE', 20, NOW(), NOW()),
	  ('sticker-pack', '社区贴纸包', '论坛周边贴纸，适合线下活动和社区纪念。', 'physical', '/images/shop/sticker-pack.svg', '', '', 120, 100, 'ACTIVE', 30, NOW(), NOW())
	 ON CONFLICT (sku) DO NOTHING`,
	`INSERT INTO mall_product_categories (slug, name, description, status, sort, created_at, updated_at)
	 SELECT DISTINCT p.category, p.category, '', 'ACTIVE', 1000, NOW(), NOW()
	 FROM mall_products p
	 WHERE COALESCE(p.category, '') <> ''
	 ON CONFLICT (slug) DO NOTHING`,
	`INSERT INTO mall_product_stock_logs (
	  product_id, sku, title, delta, before_stock, after_stock, reason, reference_type, reference_id, operator_type, operator_id, note, created_at
	)
	SELECT p.id, p.sku, p.title, p.stock, 0, p.stock, 'product_created', 'product', p.id, 'admin', 'system', '历史商品初始化库存', p.created_at
	FROM mall_products p
	WHERE NOT EXISTS (
	  SELECT 1
	  FROM mall_product_stock_logs l
	  WHERE l.product_id = p.id
	    AND l.reason = 'product_created'
	    AND l.reference_type = 'product'
	    AND l.reference_id = p.id
	)`,
}
