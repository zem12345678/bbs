package mall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	domain "mall-service/internal/domain/mall"

	"github.com/google/uuid"
)

const (
	OrderPaidEventType          = "mall.order.paid.v1"
	OrderShippedEventType       = "mall.order.shipped.v1"
	OrderCompletedEventType     = "mall.order.completed.v1"
	RefundApprovedEventType     = "mall.refund.approved.v1"
	RefundRejectedEventType     = "mall.refund.rejected.v1"
	ReviewPublishedEventType    = "mall.product_review.published.v1"
	ReviewHiddenEventType       = "mall.product_review.hidden.v1"
	EntitlementRevokedEventType = "mall.digital_entitlement.revoked.v1"
)

const (
	DefaultOrderExpireAfter   = 30 * time.Minute
	DefaultOrderExpireLimit   = 100
	DefaultOutboxRequeueLimit = 100
)

type CreditDebitCommand struct {
	UserID        int64
	Amount        int64
	Reason        string
	Description   string
	SourceEventID string
	SourceType    string
	SourceID      int64
}

type CreditAdjustCommand struct {
	UserID        int64
	Delta         int64
	Reason        string
	Description   string
	SourceEventID string
	SourceType    string
	SourceID      int64
}

type CreditCharger interface {
	DebitCredits(ctx context.Context, command CreditDebitCommand) error
	AdjustCredits(ctx context.Context, command CreditAdjustCommand) error
}

type Service struct {
	repo             domain.Repository
	charger          CreditCharger
	orderExpireAfter time.Duration
	now              func() time.Time
}

type ListProductsCommand struct {
	Limit    int
	Offset   int
	Keyword  string
	Category string
}

type ListProductCategoriesCommand struct {
	Limit  int
	Offset int
}

type ListProductReviewsCommand struct {
	ProductID int64
	Limit     int
	Offset    int
}

type ListUserProductReviewsCommand struct {
	UserID    int64
	ProductID int64
	Status    domain.ProductReviewStatus
	Limit     int
	Offset    int
}

type AdminListProductsCommand struct {
	Limit    int
	Offset   int
	Keyword  string
	Category string
	Status   domain.ProductStatus
}

type AdminListProductCategoriesCommand struct {
	Limit   int
	Offset  int
	Keyword string
	Status  domain.ProductCategoryStatus
}

type AdminListProductReviewsCommand struct {
	ProductID int64
	UserID    int64
	Status    domain.ProductReviewStatus
	Limit     int
	Offset    int
}

type CreateProductCommand struct {
	SKU          string
	Title        string
	Description  string
	Category     string
	CoverURL     string
	GrantType    string
	GrantKey     string
	PriceCredits int64
	Stock        int64
	Status       domain.ProductStatus
	Sort         int32
	OperatorID   string
}

type UpdateProductCommand struct {
	ID           int64
	SKU          string
	Title        string
	Description  string
	Category     string
	CoverURL     string
	GrantType    string
	GrantKey     string
	PriceCredits int64
	Stock        int64
	Status       domain.ProductStatus
	Sort         int32
	OperatorID   string
}

type SaveProductCategoryCommand struct {
	ID          int64
	Slug        string
	Name        string
	Description string
	Status      domain.ProductCategoryStatus
	Sort        int32
}

type CreateProductReviewCommand struct {
	UserID    int64
	ProductID int64
	OrderID   int64
	Rating    int32
	Content   string
}

type AdminUpdateProductReviewStatusCommand struct {
	ID     int64
	Status domain.ProductReviewStatus
}

type AdminListProductStockLogsCommand struct {
	ProductID int64
	Limit     int
	Offset    int
	Reason    string
}

type ListCouponsCommand struct {
	Limit  int
	Offset int
}

type ClaimCouponCommand struct {
	UserID   int64
	CouponID int64
}

type ListUserCouponUsagesCommand struct {
	UserID int64
	Status domain.CouponUsageStatus
	Limit  int
	Offset int
}

type AdminListCouponsCommand struct {
	Limit   int
	Offset  int
	Keyword string
	Status  domain.CouponStatus
}

type AdminListCouponUsagesCommand struct {
	CouponID int64
	UserID   int64
	Status   domain.CouponUsageStatus
	Limit    int
	Offset   int
}

type SaveCouponCommand struct {
	ID              int64
	Code            string
	Name            string
	Description     string
	DiscountCredits int64
	MinOrderCredits int64
	TotalQuota      int64
	PerUserLimit    int64
	Status          domain.CouponStatus
	StartsAt        *time.Time
	EndsAt          *time.Time
}

type CreateOrderCommand struct {
	IdempotencyKey string
	UserID         int64
	Items          []domain.CreateOrderItem
	CouponCode     string
	Receiver       string
	Phone          string
	Address        string
}

type CheckoutCartCommand struct {
	IdempotencyKey string
	UserID         int64
	CouponCode     string
	Receiver       string
	Phone          string
	Address        string
}

type CreateOrderResult struct {
	Order     domain.Order
	Duplicate bool
}

type ListOrdersCommand struct {
	UserID int64
	Limit  int
	Offset int
	Status domain.OrderStatus
}

type ListDigitalEntitlementsCommand struct {
	UserID    int64
	Status    string
	GrantType string
	GrantKey  string
	Keyword   string
	Limit     int
	Offset    int
}

type AdminRevokeDigitalEntitlementCommand struct {
	ID         int64
	OperatorID string
	Reason     string
}

type ListReviewableOrdersCommand struct {
	UserID    int64
	ProductID int64
	Limit     int
	Offset    int
}

type SetCartItemCommand struct {
	UserID    int64
	ProductID int64
	Quantity  int32
}

type ProductFavoriteCommand struct {
	UserID    int64
	ProductID int64
}

type ListProductFavoritesCommand struct {
	UserID int64
	Limit  int
	Offset int
}

type AdminListOrdersCommand struct {
	UserID  int64
	Limit   int
	Offset  int
	Keyword string
	Status  domain.OrderStatus
}

type PayOrderCommand struct {
	OrderID        int64
	UserID         int64
	PaymentMethod  string
	IdempotencyKey string
}

type CancelOrderCommand struct {
	OrderID int64
	UserID  int64
}

type ConfirmOrderCommand struct {
	OrderID int64
	UserID  int64
}

type CloseExpiredOrdersCommand struct {
	ExpireAfter time.Duration
	Limit       int
}

type RecoverStalePayingOrdersCommand struct {
	StaleAfter time.Duration
	Limit      int
}

type RecoverStalePayingOrdersResult struct {
	Recovered int
	Failed    int
}

type AdminUpdateOrderStatusCommand struct {
	OrderID         int64
	Status          domain.OrderStatus
	OperatorID      string
	ShippingCarrier string
	TrackingNo      string
	Note            string
}

type CreateRefundRequestCommand struct {
	OrderID int64
	UserID  int64
	Reason  string
	Note    string
}

type CancelRefundRequestCommand struct {
	RefundID int64
	UserID   int64
}

type ListAddressesCommand struct {
	UserID int64
	Limit  int
	Offset int
}

type SaveAddressCommand struct {
	ID         int64
	UserID     int64
	Receiver   string
	Phone      string
	Province   string
	City       string
	District   string
	Detail     string
	PostalCode string
	IsDefault  bool
}

type ListRefundRequestsCommand struct {
	UserID int64
	Limit  int
	Offset int
	Status domain.RefundStatus
}

type AdminListRefundRequestsCommand struct {
	UserID  int64
	Limit   int
	Offset  int
	Status  domain.RefundStatus
	Keyword string
}

type AdminReviewRefundRequestCommand struct {
	RefundID     int64
	Approved     bool
	OperatorID   string
	AdminNote    string
	RestoreStock bool
}

type AdminMallOverviewCommand struct {
	LowStockThreshold int64
}

type AdminListOutboxRequeueAuditsCommand struct {
	Limit         int
	Offset        int
	EventID       string
	AggregateType string
	AggregateID   int64
}

type AdminListOutboxRequeueAuditsResult struct {
	Items []domain.OutboxRequeueAudit
	Total int64
}

type AdminRequeueOutboxEventsCommand struct {
	Statuses   []string
	Limit      int
	OperatorID string
}

type AdminRequeueOutboxEventsResult struct {
	Requeued int64
	EventIDs []string
}

func NewService(repo domain.Repository, charger CreditCharger, orderExpireAfter time.Duration) *Service {
	if orderExpireAfter <= 0 {
		orderExpireAfter = DefaultOrderExpireAfter
	}
	return &Service{repo: repo, charger: charger, orderExpireAfter: orderExpireAfter, now: time.Now}
}

func (s *Service) SetClockForTest(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *Service) ListProducts(ctx context.Context, cmd ListProductsCommand) ([]domain.Product, int64, error) {
	return s.repo.ListProducts(ctx, domain.ProductListQuery{
		Limit:    domain.NormalizeListLimit(cmd.Limit),
		Offset:   domain.NormalizeOffset(cmd.Offset),
		Keyword:  strings.TrimSpace(cmd.Keyword),
		Category: strings.TrimSpace(cmd.Category),
		Status:   domain.ProductStatusActive,
	})
}

func (s *Service) ListProductCategories(ctx context.Context, cmd ListProductCategoriesCommand) ([]domain.ProductCategory, int64, error) {
	return s.repo.ListProductCategories(ctx, domain.ProductCategoryListQuery{
		Limit:  domain.NormalizeListLimit(cmd.Limit),
		Offset: domain.NormalizeOffset(cmd.Offset),
		Status: domain.ProductCategoryStatusActive,
	})
}

func (s *Service) ListProductReviews(ctx context.Context, cmd ListProductReviewsCommand) ([]domain.ProductReview, int64, error) {
	if cmd.ProductID <= 0 {
		return nil, 0, errors.New("product id is required")
	}
	if _, err := s.GetProduct(ctx, cmd.ProductID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListProductReviews(ctx, domain.ProductReviewListQuery{
		ProductID: cmd.ProductID,
		Status:    domain.ProductReviewStatusPublished,
		Limit:     domain.NormalizeListLimit(cmd.Limit),
		Offset:    domain.NormalizeOffset(cmd.Offset),
	})
}

func (s *Service) ListUserProductReviews(ctx context.Context, cmd ListUserProductReviewsCommand) ([]domain.ProductReview, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListUserProductReviews(ctx, domain.ProductReviewListQuery{
		ProductID: cmd.ProductID,
		UserID:    cmd.UserID,
		Status:    domain.NormalizeProductReviewStatus(cmd.Status),
		Limit:     domain.NormalizeListLimit(cmd.Limit),
		Offset:    domain.NormalizeOffset(cmd.Offset),
	})
}

func (s *Service) GetProduct(ctx context.Context, id int64) (domain.Product, error) {
	if id <= 0 {
		return domain.Product{}, errors.New("product id is required")
	}
	product, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		return domain.Product{}, err
	}
	if product.Status != domain.ProductStatusActive {
		return domain.Product{}, domain.ErrProductNotFound
	}
	return product, nil
}

func (s *Service) ListProductFavorites(ctx context.Context, cmd ListProductFavoritesCommand) ([]domain.ProductFavorite, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListProductFavorites(ctx, cmd.UserID, domain.NormalizeListLimit(cmd.Limit), domain.NormalizeOffset(cmd.Offset))
}

func (s *Service) IsProductFavorite(ctx context.Context, cmd ProductFavoriteCommand) (bool, error) {
	if cmd.UserID <= 0 {
		return false, errors.New("user id is required")
	}
	if cmd.ProductID <= 0 {
		return false, errors.New("product id is required")
	}
	if _, err := s.GetProduct(ctx, cmd.ProductID); err != nil {
		return false, err
	}
	return s.repo.IsProductFavorite(ctx, cmd.UserID, cmd.ProductID)
}

func (s *Service) AddProductFavorite(ctx context.Context, cmd ProductFavoriteCommand) (bool, error) {
	if cmd.UserID <= 0 {
		return false, errors.New("user id is required")
	}
	if cmd.ProductID <= 0 {
		return false, errors.New("product id is required")
	}
	product, err := s.repo.GetProduct(ctx, cmd.ProductID)
	if err != nil {
		return false, err
	}
	if product.Status != domain.ProductStatusActive {
		return false, domain.ErrProductUnavailable
	}
	return s.repo.AddProductFavorite(ctx, cmd.UserID, cmd.ProductID, s.now())
}

func (s *Service) RemoveProductFavorite(ctx context.Context, cmd ProductFavoriteCommand) (bool, error) {
	if cmd.UserID <= 0 {
		return false, errors.New("user id is required")
	}
	if cmd.ProductID <= 0 {
		return false, errors.New("product id is required")
	}
	return s.repo.RemoveProductFavorite(ctx, cmd.UserID, cmd.ProductID)
}

func (s *Service) AdminListProducts(ctx context.Context, cmd AdminListProductsCommand) ([]domain.Product, int64, error) {
	return s.repo.AdminListProducts(ctx, domain.ProductListQuery{
		Limit:    domain.NormalizeListLimit(cmd.Limit),
		Offset:   domain.NormalizeOffset(cmd.Offset),
		Keyword:  strings.TrimSpace(cmd.Keyword),
		Category: strings.TrimSpace(cmd.Category),
		Status:   cmd.Status,
	})
}

func (s *Service) AdminListProductCategories(ctx context.Context, cmd AdminListProductCategoriesCommand) ([]domain.ProductCategory, int64, error) {
	return s.repo.AdminListProductCategories(ctx, domain.ProductCategoryListQuery{
		Limit:   domain.NormalizeListLimit(cmd.Limit),
		Offset:  domain.NormalizeOffset(cmd.Offset),
		Keyword: strings.TrimSpace(cmd.Keyword),
		Status:  domain.NormalizeProductCategoryStatus(cmd.Status),
	})
}

func (s *Service) AdminListProductReviews(ctx context.Context, cmd AdminListProductReviewsCommand) ([]domain.ProductReview, int64, error) {
	return s.repo.AdminListProductReviews(ctx, domain.ProductReviewListQuery{
		ProductID: cmd.ProductID,
		UserID:    cmd.UserID,
		Status:    domain.NormalizeProductReviewStatus(cmd.Status),
		Limit:     domain.NormalizeListLimit(cmd.Limit),
		Offset:    domain.NormalizeOffset(cmd.Offset),
	})
}

func (s *Service) AdminMallOverview(ctx context.Context, cmd AdminMallOverviewCommand) (domain.MallOverview, error) {
	threshold := cmd.LowStockThreshold
	if threshold <= 0 {
		threshold = 10
	}
	return s.repo.AdminMallOverview(ctx, threshold)
}

func (s *Service) AdminListOutboxRequeueAudits(ctx context.Context, cmd AdminListOutboxRequeueAuditsCommand) (AdminListOutboxRequeueAuditsResult, error) {
	items, total, err := s.repo.AdminListOutboxRequeueAudits(ctx, domain.OutboxRequeueAuditListQuery{
		Limit:         domain.NormalizeListLimit(cmd.Limit),
		Offset:        domain.NormalizeOffset(cmd.Offset),
		EventID:       strings.TrimSpace(cmd.EventID),
		AggregateType: strings.TrimSpace(cmd.AggregateType),
		AggregateID:   cmd.AggregateID,
	})
	if err != nil {
		return AdminListOutboxRequeueAuditsResult{}, err
	}
	return AdminListOutboxRequeueAuditsResult{Items: items, Total: total}, nil
}

func (s *Service) AdminRequeueOutboxEvents(ctx context.Context, cmd AdminRequeueOutboxEventsCommand) (AdminRequeueOutboxEventsResult, error) {
	statuses, err := normalizeOutboxRequeueStatuses(cmd.Statuses)
	if err != nil {
		return AdminRequeueOutboxEventsResult{}, err
	}
	limit := cmd.Limit
	if limit <= 0 {
		limit = DefaultOutboxRequeueLimit
	}
	if limit > domain.MaxListLimit {
		limit = domain.MaxListLimit
	}
	requeued, err := s.repo.RequeueOutboxEvents(ctx, statuses, limit, normalizeOperatorID(cmd.OperatorID), s.now().UTC())
	if err != nil {
		return AdminRequeueOutboxEventsResult{}, err
	}
	return AdminRequeueOutboxEventsResult{Requeued: requeued.Requeued, EventIDs: requeued.EventIDs}, nil
}

func (s *Service) CreateProduct(ctx context.Context, cmd CreateProductCommand) (domain.Product, error) {
	product, err := commandToProduct(cmd)
	if err != nil {
		return domain.Product{}, err
	}
	now := s.now().UTC()
	product.CreatedAt = now
	product.UpdatedAt = now
	return s.repo.CreateProduct(ctx, product, normalizeOperatorID(cmd.OperatorID))
}

func normalizeOutboxRequeueStatuses(statuses []string) ([]string, error) {
	if len(statuses) == 0 {
		return []string{"failed", "dead_letter"}, nil
	}
	seen := make(map[string]struct{}, len(statuses))
	normalized := make([]string, 0, len(statuses))
	for _, status := range statuses {
		switch trimmed := strings.ToLower(strings.TrimSpace(status)); trimmed {
		case "":
			continue
		case "failed", "dead_letter":
			if _, ok := seen[trimmed]; !ok {
				seen[trimmed] = struct{}{}
				normalized = append(normalized, trimmed)
			}
		default:
			return nil, domain.ErrInvalidOutboxStatus
		}
	}
	if len(normalized) == 0 {
		return nil, domain.ErrInvalidOutboxStatus
	}
	return normalized, nil
}

func (s *Service) UpdateProduct(ctx context.Context, cmd UpdateProductCommand) (domain.Product, error) {
	if cmd.ID <= 0 {
		return domain.Product{}, errors.New("product id is required")
	}
	product, err := productFromCommand(CreateProductCommand{
		SKU:          cmd.SKU,
		Title:        cmd.Title,
		Description:  cmd.Description,
		Category:     cmd.Category,
		CoverURL:     cmd.CoverURL,
		GrantType:    cmd.GrantType,
		GrantKey:     cmd.GrantKey,
		PriceCredits: cmd.PriceCredits,
		Stock:        cmd.Stock,
		Status:       cmd.Status,
		Sort:         cmd.Sort,
	})
	if err != nil {
		return domain.Product{}, err
	}
	if err := validateProductGrantContract(product); err != nil {
		existing, existingErr := s.repo.GetProduct(ctx, cmd.ID)
		if existingErr != nil {
			return domain.Product{}, existingErr
		}
		if !isLegacyUnsupportedThemeArchival(existing, product) {
			return domain.Product{}, err
		}
	}
	product.ID = cmd.ID
	product.UpdatedAt = s.now().UTC()
	return s.repo.UpdateProduct(ctx, product, normalizeOperatorID(cmd.OperatorID))
}

func (s *Service) AdminCreateProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (domain.ProductCategory, error) {
	category, err := productCategoryFromCommand(cmd, s.now().UTC())
	if err != nil {
		return domain.ProductCategory{}, err
	}
	return s.repo.AdminCreateProductCategory(ctx, category)
}

func (s *Service) AdminUpdateProductCategory(ctx context.Context, cmd SaveProductCategoryCommand) (domain.ProductCategory, error) {
	if cmd.ID <= 0 {
		return domain.ProductCategory{}, errors.New("product category id is required")
	}
	category, err := productCategoryFromCommand(cmd, s.now().UTC())
	if err != nil {
		return domain.ProductCategory{}, err
	}
	category.ID = cmd.ID
	return s.repo.AdminUpdateProductCategory(ctx, category)
}

func (s *Service) CreateProductReview(ctx context.Context, cmd CreateProductReviewCommand) (domain.ProductReview, error) {
	if cmd.UserID <= 0 {
		return domain.ProductReview{}, errors.New("user id is required")
	}
	if cmd.ProductID <= 0 {
		return domain.ProductReview{}, errors.New("product id is required")
	}
	if cmd.OrderID <= 0 {
		return domain.ProductReview{}, errors.New("order id is required")
	}
	if _, err := s.GetProduct(ctx, cmd.ProductID); err != nil {
		return domain.ProductReview{}, err
	}
	if cmd.Rating < 1 || cmd.Rating > 5 {
		return domain.ProductReview{}, errors.New("rating must be between 1 and 5")
	}
	content, err := domain.NormalizeRequired(cmd.Content, "review content")
	if err != nil {
		return domain.ProductReview{}, err
	}
	if len([]rune(content)) > 1000 {
		return domain.ProductReview{}, errors.New("review content must be at most 1000 characters")
	}
	now := s.now().UTC()
	return s.repo.CreateProductReview(ctx, domain.ProductReview{
		ProductID: cmd.ProductID,
		OrderID:   cmd.OrderID,
		UserID:    cmd.UserID,
		Rating:    cmd.Rating,
		Content:   content,
		Status:    domain.ProductReviewStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *Service) AdminUpdateProductReviewStatus(ctx context.Context, cmd AdminUpdateProductReviewStatusCommand) (domain.ProductReview, error) {
	if cmd.ID <= 0 {
		return domain.ProductReview{}, errors.New("product review id is required")
	}
	status := domain.NormalizeProductReviewStatus(cmd.Status)
	if status == "" {
		return domain.ProductReview{}, errors.New("product review status is required")
	}
	existing, err := s.repo.GetProductReview(ctx, cmd.ID)
	if err != nil {
		return domain.ProductReview{}, err
	}
	now := s.now().UTC()
	var event domain.OutboxEvent
	if eventType := productReviewStatusEventType(status); eventType != "" && existing.Status != status {
		event, err = newProductReviewStatusEvent(existing, eventType, status, now)
		if err != nil {
			return domain.ProductReview{}, err
		}
	}
	return s.repo.AdminUpdateProductReviewStatus(ctx, cmd.ID, status, now, event)
}

func (s *Service) AdminListProductStockLogs(ctx context.Context, cmd AdminListProductStockLogsCommand) ([]domain.ProductStockLog, int64, error) {
	return s.repo.AdminListProductStockLogs(ctx, domain.ProductStockLogQuery{
		ProductID: cmd.ProductID,
		Limit:     domain.NormalizeListLimit(cmd.Limit),
		Offset:    domain.NormalizeOffset(cmd.Offset),
		Reason:    strings.TrimSpace(cmd.Reason),
	})
}

func (s *Service) ListCoupons(ctx context.Context, cmd ListCouponsCommand) ([]domain.Coupon, int64, error) {
	return s.repo.ListAvailableCoupons(ctx, domain.NormalizeListLimit(cmd.Limit), domain.NormalizeOffset(cmd.Offset), s.now().UTC())
}

func (s *Service) ClaimCoupon(ctx context.Context, cmd ClaimCouponCommand) (domain.CouponUsage, bool, error) {
	if cmd.UserID <= 0 {
		return domain.CouponUsage{}, false, errors.New("user id is required")
	}
	if cmd.CouponID <= 0 {
		return domain.CouponUsage{}, false, errors.New("coupon id is required")
	}
	return s.repo.ClaimCoupon(ctx, cmd.UserID, cmd.CouponID, s.now().UTC())
}

func (s *Service) ListUserCouponUsages(ctx context.Context, cmd ListUserCouponUsagesCommand) ([]domain.CouponUsage, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListCouponUsagesByUser(ctx, domain.CouponUsageListQuery{
		UserID: cmd.UserID,
		Status: domain.NormalizeCouponUsageStatus(cmd.Status),
		Limit:  domain.NormalizeListLimit(cmd.Limit),
		Offset: domain.NormalizeOffset(cmd.Offset),
	})
}

func (s *Service) AdminListCoupons(ctx context.Context, cmd AdminListCouponsCommand) ([]domain.Coupon, int64, error) {
	return s.repo.AdminListCoupons(ctx, domain.CouponListQuery{
		Limit:   domain.NormalizeListLimit(cmd.Limit),
		Offset:  domain.NormalizeOffset(cmd.Offset),
		Keyword: strings.TrimSpace(cmd.Keyword),
		Status:  domain.NormalizeCouponStatus(cmd.Status),
	})
}

func (s *Service) AdminListCouponUsages(ctx context.Context, cmd AdminListCouponUsagesCommand) ([]domain.CouponUsage, int64, error) {
	return s.repo.AdminListCouponUsages(ctx, domain.CouponUsageListQuery{
		CouponID: cmd.CouponID,
		UserID:   cmd.UserID,
		Status:   domain.NormalizeCouponUsageStatus(cmd.Status),
		Limit:    domain.NormalizeListLimit(cmd.Limit),
		Offset:   domain.NormalizeOffset(cmd.Offset),
	})
}

func (s *Service) AdminCreateCoupon(ctx context.Context, cmd SaveCouponCommand) (domain.Coupon, error) {
	coupon, err := couponFromCommand(cmd, s.now().UTC())
	if err != nil {
		return domain.Coupon{}, err
	}
	return s.repo.AdminCreateCoupon(ctx, coupon)
}

func (s *Service) AdminUpdateCoupon(ctx context.Context, cmd SaveCouponCommand) (domain.Coupon, error) {
	if cmd.ID <= 0 {
		return domain.Coupon{}, errors.New("coupon id is required")
	}
	coupon, err := couponFromCommand(cmd, s.now().UTC())
	if err != nil {
		return domain.Coupon{}, err
	}
	coupon.ID = cmd.ID
	return s.repo.AdminUpdateCoupon(ctx, coupon)
}

func normalizeOperatorID(operatorID string) string {
	trimmed := strings.TrimSpace(operatorID)
	if trimmed == "" {
		return "admin"
	}
	return trimmed
}

func commandToProduct(cmd CreateProductCommand) (domain.Product, error) {
	product, err := productFromCommand(cmd)
	if err != nil {
		return domain.Product{}, err
	}
	if err := validateProductGrantContract(product); err != nil {
		return domain.Product{}, err
	}
	return product, nil
}

func productFromCommand(cmd CreateProductCommand) (domain.Product, error) {
	sku, err := domain.NormalizeRequired(cmd.SKU, "sku")
	if err != nil {
		return domain.Product{}, err
	}
	title, err := domain.NormalizeRequired(cmd.Title, "title")
	if err != nil {
		return domain.Product{}, err
	}
	if cmd.PriceCredits < 0 {
		return domain.Product{}, errors.New("price_credits must be non-negative")
	}
	if cmd.Stock < 0 {
		return domain.Product{}, errors.New("stock must be non-negative")
	}
	category, err := domain.NormalizeRequired(normalizeProductCategorySlug(cmd.Category), "category")
	if err != nil {
		return domain.Product{}, err
	}
	if !isSafeProductCategorySlug(category) {
		return domain.Product{}, errors.New("category only allows letters, numbers, underscore, dot and dash")
	}
	grantType := normalizeDigitalGrantType(cmd.GrantType, cmd.GrantKey)
	if strings.TrimSpace(cmd.GrantType) != "" && grantType == "" {
		return domain.Product{}, errors.New("grant_type must be one of badge, theme, membership or digital")
	}
	grantKey := normalizeDigitalGrantKey(cmd.GrantKey)
	if grantType != "" && grantKey == "" {
		return domain.Product{}, errors.New("grant_key is required when grant_type is set")
	}
	return domain.Product{
		SKU:          sku,
		Title:        title,
		Description:  strings.TrimSpace(cmd.Description),
		Category:     category,
		CoverURL:     strings.TrimSpace(cmd.CoverURL),
		GrantType:    grantType,
		GrantKey:     grantKey,
		PriceCredits: cmd.PriceCredits,
		Stock:        cmd.Stock,
		Status:       domain.NormalizeProductStatus(cmd.Status),
		Sort:         cmd.Sort,
	}, nil
}

func validateProductGrantContract(product domain.Product) error {
	if product.GrantType == "theme" && product.GrantKey != "theme-pro" {
		return domain.ErrUnsupportedThemeGrantKey
	}
	return nil
}

func isLegacyUnsupportedThemeArchival(existing, next domain.Product) bool {
	existingGrantType := normalizeDigitalGrantType(existing.GrantType, existing.GrantKey)
	existingGrantKey := normalizeDigitalGrantKey(existing.GrantKey)
	return existing.Status != domain.ProductStatusArchived &&
		next.Status == domain.ProductStatusArchived &&
		existingGrantType == "theme" &&
		existingGrantKey != "theme-pro" &&
		next.GrantType == existingGrantType &&
		next.GrantKey == existingGrantKey
}

func normalizeDigitalGrantType(grantType string, grantKey ...string) string {
	normalized := strings.ToLower(strings.TrimSpace(grantType))
	if normalized == "" && len(grantKey) > 0 {
		normalized = inferDigitalGrantType(grantKey[0])
	}
	switch normalized {
	case "badge", "theme", "membership", "digital":
		return normalized
	default:
		return ""
	}
}

func normalizeDigitalGrantKey(grantKey string) string {
	return strings.ToLower(strings.TrimSpace(grantKey))
}

func inferDigitalGrantType(grantKey string) string {
	normalized := normalizeDigitalGrantKey(grantKey)
	switch {
	case strings.HasPrefix(normalized, "badge-"):
		return "badge"
	case strings.HasPrefix(normalized, "theme-"):
		return "theme"
	case strings.HasPrefix(normalized, "vip-"), strings.HasPrefix(normalized, "member-"), strings.Contains(normalized, "membership"):
		return "membership"
	case normalized != "":
		return "digital"
	default:
		return ""
	}
}

func productCategoryFromCommand(cmd SaveProductCategoryCommand, now time.Time) (domain.ProductCategory, error) {
	slug, err := domain.NormalizeRequired(normalizeProductCategorySlug(cmd.Slug), "product category slug")
	if err != nil {
		return domain.ProductCategory{}, err
	}
	if !isSafeProductCategorySlug(slug) {
		return domain.ProductCategory{}, errors.New("product category slug only allows letters, numbers, underscore, dot and dash")
	}
	name, err := domain.NormalizeRequired(cmd.Name, "product category name")
	if err != nil {
		return domain.ProductCategory{}, err
	}
	status := domain.NormalizeProductCategoryStatus(cmd.Status)
	if status == "" {
		status = domain.ProductCategoryStatusActive
	}
	return domain.ProductCategory{
		Slug:        slug,
		Name:        name,
		Description: strings.TrimSpace(cmd.Description),
		Status:      status,
		Sort:        cmd.Sort,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func normalizeProductCategorySlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isSafeProductCategorySlug(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func couponFromCommand(cmd SaveCouponCommand, now time.Time) (domain.Coupon, error) {
	code, err := domain.NormalizeRequired(normalizeCouponCodeInput(cmd.Code), "coupon code")
	if err != nil {
		return domain.Coupon{}, err
	}
	if strings.ContainsAny(code, " \t\r\n") {
		return domain.Coupon{}, errors.New("coupon code must not contain whitespace")
	}
	name, err := domain.NormalizeRequired(cmd.Name, "coupon name")
	if err != nil {
		return domain.Coupon{}, err
	}
	if cmd.DiscountCredits <= 0 {
		return domain.Coupon{}, errors.New("discount_credits must be positive")
	}
	if cmd.MinOrderCredits < 0 {
		return domain.Coupon{}, errors.New("min_order_credits must be non-negative")
	}
	if cmd.TotalQuota < 0 {
		return domain.Coupon{}, errors.New("total_quota must be non-negative")
	}
	if cmd.PerUserLimit < 0 {
		return domain.Coupon{}, errors.New("per_user_limit must be non-negative")
	}
	if cmd.StartsAt != nil && cmd.EndsAt != nil && !cmd.EndsAt.After(*cmd.StartsAt) {
		return domain.Coupon{}, errors.New("coupon end time must be after start time")
	}
	status := domain.NormalizeCouponStatus(cmd.Status)
	if status == "" {
		status = domain.CouponStatusDraft
	}
	return domain.Coupon{
		Code:            code,
		Name:            name,
		Description:     strings.TrimSpace(cmd.Description),
		DiscountCredits: cmd.DiscountCredits,
		MinOrderCredits: cmd.MinOrderCredits,
		TotalQuota:      cmd.TotalQuota,
		PerUserLimit:    cmd.PerUserLimit,
		Status:          status,
		StartsAt:        cmd.StartsAt,
		EndsAt:          cmd.EndsAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func normalizeCouponCodeInput(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (s *Service) CreateOrder(ctx context.Context, cmd CreateOrderCommand) (CreateOrderResult, error) {
	idempotencyKey, err := domain.NormalizeRequired(cmd.IdempotencyKey, "idempotency key")
	if err != nil {
		return CreateOrderResult{}, err
	}
	if cmd.UserID <= 0 {
		return CreateOrderResult{}, errors.New("user id is required")
	}
	if len(cmd.Items) == 0 {
		return CreateOrderResult{}, errors.New("order items are required")
	}
	normalizedItems, err := normalizeCreateOrderItems(cmd.Items)
	if err != nil {
		return CreateOrderResult{}, err
	}
	idempotencyRequest := createOrderIdempotencyRequest(cmd.UserID, idempotencyKey, normalizedItems, cmd.CouponCode, cmd.Receiver, cmd.Phone, cmd.Address)
	if existing, err := s.repo.GetOrderByIdempotencyKey(ctx, cmd.UserID, idempotencyKey); err == nil {
		if !domain.OrderMatchesIdempotencyRequest(existing, idempotencyRequest) {
			return CreateOrderResult{}, domain.ErrDuplicateReference
		}
		return CreateOrderResult{Order: existing, Duplicate: true}, nil
	} else if !errors.Is(err, domain.ErrOrderNotFound) {
		return CreateOrderResult{}, err
	}

	orderItems := make([]domain.OrderItem, 0, len(normalizedItems))
	total := int64(0)
	requiresShipping := false
	for _, item := range normalizedItems {
		product, err := s.repo.GetProduct(ctx, item.ProductID)
		if err != nil {
			return CreateOrderResult{}, err
		}
		if product.Status != domain.ProductStatusActive {
			return CreateOrderResult{}, domain.ErrProductUnavailable
		}
		if int64(item.Quantity) > product.Stock {
			return CreateOrderResult{}, domain.ErrInsufficientStock
		}
		if productRequiresShipping(product) {
			requiresShipping = true
		}
		subtotal, nextTotal, err := addOrderSubtotal(total, product.PriceCredits, item.Quantity)
		if err != nil {
			return CreateOrderResult{}, err
		}
		total = nextTotal
		orderItem := orderItemForProduct(product, item.Quantity)
		orderItem.SubtotalCredits = subtotal
		orderItems = append(orderItems, orderItem)
	}

	receiver := idempotencyRequest.Receiver
	phone := idempotencyRequest.Phone
	address := idempotencyRequest.Address
	if requiresShipping {
		if receiver == "" {
			return CreateOrderResult{}, errors.New("receiver is required")
		}
		if phone == "" {
			return CreateOrderResult{}, errors.New("phone is required")
		}
		if address == "" {
			return CreateOrderResult{}, errors.New("address is required")
		}
	}
	if err := s.ensureOwnedDigitalGrantOrderCanBeCreated(ctx, cmd.UserID, orderItems); err != nil {
		return CreateOrderResult{}, err
	}

	now := s.now().UTC()
	order := domain.Order{
		OrderNo:         newOrderNo(now),
		IdempotencyKey:  idempotencyKey,
		UserID:          cmd.UserID,
		Items:           orderItems,
		OriginalCredits: total,
		TotalCredits:    total,
		CouponCode:      idempotencyRequest.CouponCode,
		Status:          domain.OrderStatusPendingPayment,
		Receiver:        receiver,
		Phone:           phone,
		Address:         address,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	saved, duplicate, err := s.repo.CreateOrder(ctx, order)
	if err != nil {
		return CreateOrderResult{}, err
	}
	return CreateOrderResult{Order: saved, Duplicate: duplicate}, nil
}

func createOrderIdempotencyRequest(userID int64, idempotencyKey string, items []domain.CreateOrderItem, couponCode, receiver, phone, address string) domain.Order {
	orderItems := make([]domain.OrderItem, 0, len(items))
	for _, item := range items {
		orderItems = append(orderItems, domain.OrderItem{ProductID: item.ProductID, Quantity: item.Quantity})
	}
	return domain.Order{
		IdempotencyKey: idempotencyKey,
		UserID:         userID,
		Items:          orderItems,
		CouponCode:     normalizeCouponCodeInput(couponCode),
		Receiver:       strings.TrimSpace(receiver),
		Phone:          strings.TrimSpace(phone),
		Address:        strings.TrimSpace(address),
	}
}

func (s *Service) CheckoutCart(ctx context.Context, cmd CheckoutCartCommand) (CreateOrderResult, error) {
	idempotencyKey, err := domain.NormalizeRequired(cmd.IdempotencyKey, "idempotency key")
	if err != nil {
		return CreateOrderResult{}, err
	}
	if cmd.UserID <= 0 {
		return CreateOrderResult{}, errors.New("user id is required")
	}
	if existing, err := s.repo.GetOrderByIdempotencyKey(ctx, cmd.UserID, idempotencyKey); err == nil {
		cartItems, _, err := s.repo.ListCartItems(ctx, cmd.UserID)
		if err != nil {
			return CreateOrderResult{}, err
		}
		idempotencyRequest := existing
		idempotencyRequest.IdempotencyKey = idempotencyKey
		idempotencyRequest.UserID = cmd.UserID
		idempotencyRequest.CouponCode = normalizeCouponCodeInput(cmd.CouponCode)
		idempotencyRequest.Receiver = strings.TrimSpace(cmd.Receiver)
		idempotencyRequest.Phone = strings.TrimSpace(cmd.Phone)
		idempotencyRequest.Address = strings.TrimSpace(cmd.Address)
		if len(cartItems) > 0 {
			idempotencyRequest.Items = checkoutCartIdempotencyRequest(cmd.UserID, idempotencyKey, cartItems, cmd.CouponCode, cmd.Receiver, cmd.Phone, cmd.Address).Items
		}
		if !domain.OrderMatchesIdempotencyRequest(existing, idempotencyRequest) {
			return CreateOrderResult{}, domain.ErrDuplicateReference
		}
		return CreateOrderResult{Order: existing, Duplicate: true}, nil
	} else if !errors.Is(err, domain.ErrOrderNotFound) {
		return CreateOrderResult{}, err
	}
	cartItems, _, err := s.repo.ListCartItems(ctx, cmd.UserID)
	if err != nil {
		return CreateOrderResult{}, err
	}
	if len(cartItems) == 0 {
		return CreateOrderResult{}, errors.New("cart items are required")
	}

	orderItems := make([]domain.OrderItem, 0, len(cartItems))
	total := int64(0)
	requiresShipping := false
	for _, item := range cartItems {
		product := item.Product
		if product.Status != domain.ProductStatusActive {
			return CreateOrderResult{}, domain.ErrProductUnavailable
		}
		if item.Quantity <= 0 {
			return CreateOrderResult{}, errors.New("item quantity must be positive")
		}
		if int64(item.Quantity) > product.Stock {
			return CreateOrderResult{}, domain.ErrInsufficientStock
		}
		if productRequiresShipping(product) {
			requiresShipping = true
		}
		subtotal, nextTotal, err := addOrderSubtotal(total, product.PriceCredits, item.Quantity)
		if err != nil {
			return CreateOrderResult{}, err
		}
		total = nextTotal
		orderItem := orderItemForProduct(product, item.Quantity)
		orderItem.SubtotalCredits = subtotal
		orderItems = append(orderItems, orderItem)
	}

	receiver := strings.TrimSpace(cmd.Receiver)
	phone := strings.TrimSpace(cmd.Phone)
	address := strings.TrimSpace(cmd.Address)
	if requiresShipping {
		if receiver == "" {
			return CreateOrderResult{}, errors.New("receiver is required")
		}
		if phone == "" {
			return CreateOrderResult{}, errors.New("phone is required")
		}
		if address == "" {
			return CreateOrderResult{}, errors.New("address is required")
		}
	}
	if err := s.ensureOwnedDigitalGrantOrderCanBeCreated(ctx, cmd.UserID, orderItems); err != nil {
		return CreateOrderResult{}, err
	}

	now := s.now().UTC()
	order := domain.Order{
		OrderNo:         newOrderNo(now),
		IdempotencyKey:  idempotencyKey,
		UserID:          cmd.UserID,
		Items:           orderItems,
		OriginalCredits: total,
		TotalCredits:    total,
		CouponCode:      normalizeCouponCodeInput(cmd.CouponCode),
		Status:          domain.OrderStatusPendingPayment,
		Receiver:        receiver,
		Phone:           phone,
		Address:         address,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	saved, duplicate, err := s.repo.CreateOrderFromCart(ctx, order)
	if err != nil {
		return CreateOrderResult{}, err
	}
	return CreateOrderResult{Order: saved, Duplicate: duplicate}, nil
}

func checkoutCartIdempotencyRequest(userID int64, idempotencyKey string, cartItems []domain.CartItem, couponCode, receiver, phone, address string) domain.Order {
	orderItems := make([]domain.OrderItem, 0, len(cartItems))
	for _, item := range cartItems {
		orderItems = append(orderItems, domain.OrderItem{ProductID: item.Product.ID, Quantity: item.Quantity})
	}
	return domain.Order{
		IdempotencyKey: idempotencyKey,
		UserID:         userID,
		Items:          orderItems,
		CouponCode:     normalizeCouponCodeInput(couponCode),
		Receiver:       strings.TrimSpace(receiver),
		Phone:          strings.TrimSpace(phone),
		Address:        strings.TrimSpace(address),
	}
}

func productRequiresShipping(product domain.Product) bool {
	if strings.EqualFold(strings.TrimSpace(product.Category), "digital") {
		return false
	}
	return normalizeDigitalGrantType(product.GrantType, product.GrantKey) == ""
}

func orderItemForProduct(product domain.Product, quantity int32) domain.OrderItem {
	return domain.OrderItem{
		ProductID:        product.ID,
		SKU:              product.SKU,
		Title:            product.Title,
		Category:         strings.TrimSpace(product.Category),
		GrantType:        normalizeDigitalGrantType(product.GrantType, product.GrantKey),
		GrantKey:         normalizeDigitalGrantKey(product.GrantKey),
		Quantity:         quantity,
		UnitPriceCredits: product.PriceCredits,
	}
}

func addOrderSubtotal(total, unitPrice int64, quantity int32) (int64, int64, error) {
	if unitPrice < 0 {
		return 0, 0, errors.New("product price must be non-negative")
	}
	if quantity <= 0 {
		return 0, 0, errors.New("item quantity must be positive")
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

func normalizeCreateOrderItems(items []domain.CreateOrderItem) ([]domain.CreateOrderItem, error) {
	quantityByProductID := make(map[int64]int32, len(items))
	orderedProductIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item.ProductID <= 0 {
			return nil, errors.New("product id is required")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("item quantity must be positive")
		}
		if _, ok := quantityByProductID[item.ProductID]; !ok {
			orderedProductIDs = append(orderedProductIDs, item.ProductID)
		}
		if quantityByProductID[item.ProductID] > math.MaxInt32-item.Quantity {
			return nil, errors.New("item quantity is too large")
		}
		quantityByProductID[item.ProductID] += item.Quantity
	}

	normalized := make([]domain.CreateOrderItem, 0, len(orderedProductIDs))
	for _, productID := range orderedProductIDs {
		normalized = append(normalized, domain.CreateOrderItem{
			ProductID: productID,
			Quantity:  quantityByProductID[productID],
		})
	}
	return normalized, nil
}

type ownedDigitalGrant struct {
	grantType string
	grantKey  string
}

func (s *Service) ensureOwnedDigitalGrantOrderCanBeCreated(ctx context.Context, userID int64, items []domain.OrderItem) error {
	if err := ensureSingleOwnedDigitalGrantPerOrder(items); err != nil {
		return err
	}
	if err := s.ensureNoDuplicateActiveOwnedDigitalEntitlements(ctx, userID, items); err != nil {
		return err
	}
	for _, grant := range openOrderProtectedDigitalGrantsForItems(items) {
		exists, err := s.repo.OpenDigitalGrantOrderExists(ctx, userID, grant.grantType, grant.grantKey)
		if err != nil {
			return err
		}
		if exists {
			return pendingOwnedDigitalGrantOrderError(grant.grantType)
		}
	}
	return nil
}

func (s *Service) ensureNoDuplicateActiveOwnedDigitalEntitlements(ctx context.Context, userID int64, items []domain.OrderItem) error {
	if err := ensureSingleOwnedDigitalGrantPerOrder(items); err != nil {
		return err
	}
	for _, grant := range ownedDigitalGrantsForItems(items) {
		exists, err := s.activeDigitalEntitlementExists(ctx, userID, grant.grantType, grant.grantKey)
		if err != nil {
			return err
		}
		if exists {
			return activeOwnedDigitalGrantEntitlementError(grant.grantType)
		}
	}
	return nil
}

func ensureSingleOwnedDigitalGrantPerOrder(items []domain.OrderItem) error {
	seen := make(map[string]struct{})
	for _, item := range items {
		grantType := normalizeDigitalGrantType(item.GrantType, item.GrantKey)
		if !isSingleOwnedDigitalGrantType(grantType) {
			continue
		}
		grantKey := normalizeDigitalGrantKey(item.GrantKey)
		if grantKey == "" {
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

func ownedDigitalGrantsForItems(items []domain.OrderItem) []ownedDigitalGrant {
	return digitalGrantsForItems(items, isSingleOwnedDigitalGrantType)
}

func openOrderProtectedDigitalGrantsForItems(items []domain.OrderItem) []ownedDigitalGrant {
	return digitalGrantsForItems(items, isOpenOrderProtectedDigitalGrantType)
}

func digitalGrantsForItems(items []domain.OrderItem, include func(string) bool) []ownedDigitalGrant {
	seen := make(map[string]struct{})
	grants := make([]ownedDigitalGrant, 0)
	for _, item := range items {
		grantType := normalizeDigitalGrantType(item.GrantType, item.GrantKey)
		if !include(grantType) {
			continue
		}
		grantKey := normalizeDigitalGrantKey(item.GrantKey)
		if grantKey == "" {
			continue
		}
		key := grantType + ":" + grantKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		grants = append(grants, ownedDigitalGrant{grantType: grantType, grantKey: grantKey})
	}
	return grants
}

func (s *Service) activeDigitalEntitlementExists(ctx context.Context, userID int64, grantType, grantKey string) (bool, error) {
	items, total, err := s.repo.ListDigitalEntitlements(ctx, domain.DigitalEntitlementListQuery{
		UserID:    userID,
		Status:    domain.DigitalEntitlementStatusActive,
		GrantType: grantType,
		GrantKey:  grantKey,
		Limit:     1,
	})
	if err != nil {
		return false, err
	}
	return total > 0 || len(items) > 0, nil
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

func isOwnedDigitalGrantPaymentFailure(err error) bool {
	return errors.Is(err, domain.ErrActiveThemeEntitlementExists) ||
		errors.Is(err, domain.ErrDuplicateThemeGrantInOrder) ||
		errors.Is(err, domain.ErrActiveBadgeEntitlementExists) ||
		errors.Is(err, domain.ErrDuplicateBadgeGrantInOrder)
}

func (s *Service) GetOrder(ctx context.Context, orderID int64) (domain.Order, error) {
	if orderID <= 0 {
		return domain.Order{}, errors.New("order id is required")
	}
	return s.repo.GetOrder(ctx, orderID)
}

func (s *Service) GetUserOrder(ctx context.Context, orderID, userID int64) (domain.Order, error) {
	if userID <= 0 {
		return domain.Order{}, errors.New("user id is required")
	}
	order, err := s.GetOrder(ctx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	return order, nil
}

func (s *Service) ListOrders(ctx context.Context, cmd ListOrdersCommand) ([]domain.Order, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListOrdersByUser(ctx, domain.OrderListQuery{
		UserID: cmd.UserID,
		Limit:  domain.NormalizeListLimit(cmd.Limit),
		Offset: domain.NormalizeOffset(cmd.Offset),
		Status: domain.NormalizeOrderStatus(cmd.Status),
	})
}

func (s *Service) ListDigitalEntitlements(ctx context.Context, cmd ListDigitalEntitlementsCommand) ([]domain.DigitalEntitlement, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListDigitalEntitlements(ctx, domain.DigitalEntitlementListQuery{
		UserID:    cmd.UserID,
		Status:    cmd.Status,
		GrantType: cmd.GrantType,
		GrantKey:  cmd.GrantKey,
		Keyword:   cmd.Keyword,
		Limit:     domain.NormalizeListLimit(cmd.Limit),
		Offset:    domain.NormalizeOffset(cmd.Offset),
	})
}

func (s *Service) AdminListDigitalEntitlements(ctx context.Context, cmd ListDigitalEntitlementsCommand) ([]domain.DigitalEntitlement, int64, error) {
	return s.repo.ListDigitalEntitlements(ctx, domain.DigitalEntitlementListQuery{
		UserID:    cmd.UserID,
		Status:    cmd.Status,
		GrantType: cmd.GrantType,
		GrantKey:  cmd.GrantKey,
		Keyword:   cmd.Keyword,
		Limit:     domain.NormalizeListLimit(cmd.Limit),
		Offset:    domain.NormalizeOffset(cmd.Offset),
	})
}

func (s *Service) AdminRevokeDigitalEntitlement(ctx context.Context, cmd AdminRevokeDigitalEntitlementCommand) (domain.DigitalEntitlement, error) {
	if cmd.ID <= 0 {
		return domain.DigitalEntitlement{}, errors.New("digital entitlement id is required")
	}
	operatorID := strings.TrimSpace(cmd.OperatorID)
	if operatorID == "" {
		return domain.DigitalEntitlement{}, errors.New("operator id is required")
	}
	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		return domain.DigitalEntitlement{}, errors.New("revoke reason is required")
	}
	entitlement, err := s.repo.GetDigitalEntitlement(ctx, cmd.ID)
	if err != nil {
		return domain.DigitalEntitlement{}, err
	}
	if digitalEntitlementAlreadyRevoked(entitlement) {
		entitlement.Status = domain.DigitalEntitlementStatusRevoked
		return entitlement, nil
	}
	now := s.now().UTC()
	if !revocableDigitalEntitlement(entitlement, now) {
		return domain.DigitalEntitlement{}, domain.ErrInvalidOrderState
	}
	event, err := newDigitalEntitlementRevokedEvent(entitlement, operatorID, reason, now)
	if err != nil {
		return domain.DigitalEntitlement{}, err
	}
	return s.repo.AdminRevokeDigitalEntitlement(ctx, cmd.ID, operatorID, reason, now, event)
}

func (s *Service) ListReviewableOrders(ctx context.Context, cmd ListReviewableOrdersCommand) ([]domain.Order, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	if cmd.ProductID <= 0 {
		return nil, 0, errors.New("product id is required")
	}
	if _, err := s.GetProduct(ctx, cmd.ProductID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListReviewableOrders(ctx, domain.OrderListQuery{
		UserID: cmd.UserID,
		Limit:  domain.NormalizeListLimit(cmd.Limit),
		Offset: domain.NormalizeOffset(cmd.Offset),
	}, cmd.ProductID)
}

func (s *Service) ListCartItems(ctx context.Context, userID int64) ([]domain.CartItem, int64, error) {
	if userID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListCartItems(ctx, userID)
}

func (s *Service) SetCartItem(ctx context.Context, cmd SetCartItemCommand) ([]domain.CartItem, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	if cmd.ProductID <= 0 {
		return nil, 0, errors.New("product id is required")
	}
	if cmd.Quantity <= 0 {
		if _, err := s.repo.RemoveCartItem(ctx, cmd.UserID, cmd.ProductID); err != nil {
			return nil, 0, err
		}
		return s.repo.ListCartItems(ctx, cmd.UserID)
	}
	if cmd.Quantity > 999 {
		return nil, 0, errors.New("quantity must be less than or equal to 999")
	}
	product, err := s.repo.GetProduct(ctx, cmd.ProductID)
	if err != nil {
		return nil, 0, err
	}
	if product.Status != domain.ProductStatusActive {
		return nil, 0, domain.ErrProductUnavailable
	}
	if int64(cmd.Quantity) > product.Stock {
		return nil, 0, domain.ErrInsufficientStock
	}
	targetItem := orderItemForProduct(product, cmd.Quantity)
	if err := s.ensureOwnedDigitalGrantOrderCanBeCreated(ctx, cmd.UserID, []domain.OrderItem{targetItem}); err != nil {
		return nil, 0, err
	}
	currentItems, _, err := s.repo.ListCartItems(ctx, cmd.UserID)
	if err != nil {
		return nil, 0, err
	}
	candidateItems := make([]domain.OrderItem, 0, len(currentItems)+1)
	for _, item := range currentItems {
		itemProduct := item.Product
		if itemProduct.ID <= 0 || itemProduct.ID == cmd.ProductID || item.Quantity <= 0 {
			continue
		}
		candidateItems = append(candidateItems, orderItemForProduct(itemProduct, item.Quantity))
	}
	candidateItems = append(candidateItems, targetItem)
	if err := ensureSingleOwnedDigitalGrantPerOrder(candidateItems); err != nil {
		return nil, 0, err
	}
	if err := s.repo.SetCartItem(ctx, cmd.UserID, cmd.ProductID, cmd.Quantity, s.now().UTC()); err != nil {
		return nil, 0, err
	}
	return s.repo.ListCartItems(ctx, cmd.UserID)
}

func (s *Service) RemoveCartItem(ctx context.Context, userID int64, productID int64) ([]domain.CartItem, int64, error) {
	if userID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	if productID <= 0 {
		return nil, 0, errors.New("product id is required")
	}
	if _, err := s.repo.RemoveCartItem(ctx, userID, productID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListCartItems(ctx, userID)
}

func (s *Service) ClearCart(ctx context.Context, userID int64) ([]domain.CartItem, int64, error) {
	if userID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	if _, err := s.repo.ClearCart(ctx, userID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListCartItems(ctx, userID)
}

func (s *Service) AdminListOrders(ctx context.Context, cmd AdminListOrdersCommand) ([]domain.Order, int64, error) {
	return s.repo.AdminListOrders(ctx, domain.OrderListQuery{
		UserID:  cmd.UserID,
		Limit:   domain.NormalizeListLimit(cmd.Limit),
		Offset:  domain.NormalizeOffset(cmd.Offset),
		Keyword: strings.TrimSpace(cmd.Keyword),
		Status:  domain.NormalizeOrderStatus(cmd.Status),
	})
}

func (s *Service) PayOrder(ctx context.Context, cmd PayOrderCommand) (domain.Order, error) {
	return s.payOrder(ctx, cmd, true)
}

func (s *Service) payOrder(ctx context.Context, cmd PayOrderCommand, failPaymentOnDebitError bool) (domain.Order, error) {
	if cmd.OrderID <= 0 {
		return domain.Order{}, errors.New("order id is required")
	}
	if cmd.UserID <= 0 {
		return domain.Order{}, errors.New("user id is required")
	}
	paymentMethod := strings.TrimSpace(cmd.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = domain.PaymentProviderCredits
	}
	if paymentMethod != domain.PaymentProviderCredits {
		return domain.Order{}, domain.ErrUnsupportedPayment
	}
	idempotencyKey := strings.TrimSpace(cmd.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("mall-order:%d:%d", cmd.OrderID, cmd.UserID)
	}

	startedAt := s.now().UTC()
	order, payment, err := s.repo.BeginOrderPayment(ctx, cmd.OrderID, cmd.UserID, paymentMethod, idempotencyKey, startedAt.Add(-s.orderExpireAfter), startedAt)
	if err != nil {
		return order, err
	}
	if order.Status == domain.OrderStatusPaid || order.Status == domain.OrderStatusCompleted {
		return order, nil
	}
	if err := s.ensureNoDuplicateActiveOwnedDigitalEntitlements(ctx, order.UserID, order.Items); err != nil {
		if isOwnedDigitalGrantPaymentFailure(err) {
			_ = s.repo.FailOrderPayment(ctx, order.ID, order.UserID, payment.ID, err.Error(), s.now().UTC())
		}
		return domain.Order{}, err
	}
	if order.TotalCredits > 0 {
		if s.charger == nil {
			if failPaymentOnDebitError {
				_ = s.repo.FailOrderPayment(ctx, order.ID, order.UserID, payment.ID, "credit charger not configured", s.now().UTC())
			}
			return domain.Order{}, errors.New("credit charger not configured")
		}
		err = s.charger.DebitCredits(ctx, CreditDebitCommand{
			UserID:        order.UserID,
			Amount:        order.TotalCredits,
			Reason:        "mall_order_paid",
			Description:   fmt.Sprintf("兑换订单 %s", order.OrderNo),
			SourceEventID: paymentSourceEventID(order.ID, payment.IdempotencyKey),
			SourceType:    "mall_order",
			SourceID:      order.ID,
		})
		if err != nil {
			if failPaymentOnDebitError && errors.Is(err, domain.ErrInsufficientCredits) {
				_ = s.repo.FailOrderPayment(ctx, order.ID, order.UserID, payment.ID, err.Error(), s.now().UTC())
			}
			return domain.Order{}, err
		}
	}
	paidAt := s.now().UTC()
	event, err := newOrderPaidEvent(order, payment, paidAt)
	if err != nil {
		return domain.Order{}, err
	}
	return s.repo.CompleteOrderPayment(ctx, order.ID, order.UserID, payment.ID, paidAt, event)
}

func paymentSourceEventID(orderID int64, idempotencyKey string) string {
	return fmt.Sprintf("mall.order.pay:%d:%s", orderID, strings.TrimSpace(idempotencyKey))
}

func (s *Service) CancelOrder(ctx context.Context, cmd CancelOrderCommand) (domain.Order, error) {
	if cmd.OrderID <= 0 {
		return domain.Order{}, errors.New("order id is required")
	}
	if cmd.UserID <= 0 {
		return domain.Order{}, errors.New("user id is required")
	}
	return s.repo.CancelOrder(ctx, cmd.OrderID, cmd.UserID, s.now().UTC())
}

func (s *Service) ConfirmOrder(ctx context.Context, cmd ConfirmOrderCommand) (domain.Order, error) {
	if cmd.OrderID <= 0 {
		return domain.Order{}, errors.New("order id is required")
	}
	if cmd.UserID <= 0 {
		return domain.Order{}, errors.New("user id is required")
	}
	order, err := s.repo.GetOrder(ctx, cmd.OrderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.UserID != cmd.UserID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	if order.Status == domain.OrderStatusCompleted {
		return order, nil
	}
	if order.Status != domain.OrderStatusShipped {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	now := s.now().UTC()
	event, err := newOrderStatusUpdatedEvent(order, OrderCompletedEventType, domain.OrderStatusCompleted, domain.OrderFulfillment{}, "用户确认收货", fmt.Sprintf("%d", cmd.UserID), now)
	if err != nil {
		return domain.Order{}, err
	}
	return s.repo.ConfirmOrder(ctx, cmd.OrderID, cmd.UserID, event.CreatedAt, event)
}

func (s *Service) CloseExpiredOrders(ctx context.Context, cmd CloseExpiredOrdersCommand) ([]domain.Order, error) {
	expireAfter := cmd.ExpireAfter
	if expireAfter <= 0 {
		expireAfter = DefaultOrderExpireAfter
	}
	limit := cmd.Limit
	if limit <= 0 {
		limit = DefaultOrderExpireLimit
	}
	if limit > domain.MaxListLimit {
		limit = domain.MaxListLimit
	}
	now := s.now().UTC()
	return s.repo.CloseExpiredOrders(ctx, now.Add(-expireAfter), limit, now)
}

func (s *Service) RecoverStalePayingOrders(ctx context.Context, cmd RecoverStalePayingOrdersCommand) (RecoverStalePayingOrdersResult, error) {
	staleAfter := cmd.StaleAfter
	if staleAfter <= 0 {
		staleAfter = DefaultOrderExpireAfter
	}
	limit := cmd.Limit
	if limit <= 0 {
		limit = DefaultOrderExpireLimit
	}
	if limit > domain.MaxListLimit {
		limit = domain.MaxListLimit
	}
	now := s.now().UTC()
	candidates, err := s.repo.ListStalePayingOrders(ctx, now.Add(-staleAfter), limit)
	if err != nil {
		return RecoverStalePayingOrdersResult{}, err
	}
	var result RecoverStalePayingOrdersResult
	for _, candidate := range candidates {
		_, err := s.payOrder(ctx, PayOrderCommand{
			OrderID:        candidate.OrderID,
			UserID:         candidate.UserID,
			PaymentMethod:  candidate.Provider,
			IdempotencyKey: candidate.IdempotencyKey,
		}, false)
		if err != nil {
			if errors.Is(err, domain.ErrInsufficientCredits) {
				failedAt := s.now().UTC()
				if s.repo.FailOrderPayment(ctx, candidate.OrderID, candidate.UserID, candidate.PaymentID, err.Error(), failedAt) == nil && s.orderExpireAfter > 0 {
					_, _, _ = s.repo.CloseExpiredOrder(ctx, candidate.OrderID, candidate.UserID, failedAt.Add(-s.orderExpireAfter), failedAt)
				}
			}
			result.Failed++
			continue
		}
		result.Recovered++
	}
	return result, nil
}

func (s *Service) AdminUpdateOrderStatus(ctx context.Context, cmd AdminUpdateOrderStatusCommand) (domain.Order, error) {
	if cmd.OrderID <= 0 {
		return domain.Order{}, errors.New("order id is required")
	}
	nextStatus := domain.NormalizeOrderStatus(cmd.Status)
	if nextStatus != domain.OrderStatusShipped && nextStatus != domain.OrderStatusCompleted {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	operatorID := strings.TrimSpace(cmd.OperatorID)
	if operatorID == "" {
		operatorID = "admin"
	}
	order, err := s.repo.GetOrder(ctx, cmd.OrderID)
	if err != nil {
		return domain.Order{}, err
	}
	fulfillment := domain.OrderFulfillment{
		ShippingCarrier: strings.TrimSpace(cmd.ShippingCarrier),
		TrackingNo:      strings.TrimSpace(cmd.TrackingNo),
	}
	note := strings.TrimSpace(cmd.Note)
	if err := validateAdminOrderFulfillment(order, nextStatus, fulfillment, note); err != nil {
		return domain.Order{}, err
	}
	event, err := newOrderStatusUpdatedEvent(order, orderStatusUpdatedEventType(nextStatus), nextStatus, domain.OrderFulfillment{
		ShippingCarrier: fulfillment.ShippingCarrier,
		TrackingNo:      fulfillment.TrackingNo,
	}, note, operatorID, s.now().UTC())
	if err != nil {
		return domain.Order{}, err
	}
	return s.repo.AdminUpdateOrderStatus(ctx, cmd.OrderID, nextStatus, operatorID, fulfillment, note, event.CreatedAt, event)
}

func (s *Service) CreateRefundRequest(ctx context.Context, cmd CreateRefundRequestCommand) (domain.RefundRequest, bool, error) {
	if cmd.OrderID <= 0 {
		return domain.RefundRequest{}, false, errors.New("order id is required")
	}
	if cmd.UserID <= 0 {
		return domain.RefundRequest{}, false, errors.New("user id is required")
	}
	order, err := s.repo.GetOrder(ctx, cmd.OrderID)
	if err != nil {
		return domain.RefundRequest{}, false, err
	}
	if order.UserID != cmd.UserID {
		return domain.RefundRequest{}, false, domain.ErrOrderOwnerMismatch
	}
	if !isRefundableOrderStatus(order.Status) {
		return domain.RefundRequest{}, false, domain.ErrInvalidOrderState
	}
	if orderContainsMembershipGrant(order) {
		return domain.RefundRequest{}, false, domain.ErrMembershipRefundUnavailable
	}
	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		reason = "after_sale"
	}
	note, err := normalizeRefundNote(cmd.Note)
	if err != nil {
		return domain.RefundRequest{}, false, err
	}
	now := s.now().UTC()
	return s.repo.CreateRefundRequest(ctx, domain.RefundRequest{
		OrderID:       order.ID,
		OrderNo:       order.OrderNo,
		UserID:        order.UserID,
		AmountCredits: order.TotalCredits,
		Status:        domain.RefundStatusRequested,
		Reason:        reason,
		UserNote:      note,
		RequestedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
}

func (s *Service) CancelRefundRequest(ctx context.Context, cmd CancelRefundRequestCommand) (domain.RefundRequest, bool, error) {
	if cmd.RefundID <= 0 {
		return domain.RefundRequest{}, false, errors.New("refund id is required")
	}
	if cmd.UserID <= 0 {
		return domain.RefundRequest{}, false, errors.New("user id is required")
	}
	return s.repo.CancelRefundRequest(ctx, cmd.RefundID, cmd.UserID, s.now().UTC())
}

func (s *Service) ListRefundRequests(ctx context.Context, cmd ListRefundRequestsCommand) ([]domain.RefundRequest, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListRefundRequests(ctx, domain.RefundListQuery{
		UserID: cmd.UserID,
		Limit:  domain.NormalizeListLimit(cmd.Limit),
		Offset: domain.NormalizeOffset(cmd.Offset),
		Status: domain.NormalizeRefundStatus(cmd.Status),
	})
}

func (s *Service) AdminListRefundRequests(ctx context.Context, cmd AdminListRefundRequestsCommand) ([]domain.RefundRequest, int64, error) {
	return s.repo.AdminListRefundRequests(ctx, domain.RefundListQuery{
		UserID:  cmd.UserID,
		Limit:   domain.NormalizeListLimit(cmd.Limit),
		Offset:  domain.NormalizeOffset(cmd.Offset),
		Status:  domain.NormalizeRefundStatus(cmd.Status),
		Keyword: strings.TrimSpace(cmd.Keyword),
	})
}

func (s *Service) AdminReviewRefundRequest(ctx context.Context, cmd AdminReviewRefundRequestCommand) (domain.RefundRequest, error) {
	if cmd.RefundID <= 0 {
		return domain.RefundRequest{}, errors.New("refund id is required")
	}
	operatorID := strings.TrimSpace(cmd.OperatorID)
	if operatorID == "" {
		operatorID = "admin"
	}
	adminNote := strings.TrimSpace(cmd.AdminNote)
	now := s.now().UTC()
	refund, err := s.repo.GetRefundRequest(ctx, cmd.RefundID)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if refund.Status == domain.RefundStatusApproved || refund.Status == domain.RefundStatusRejected {
		return refund, nil
	}
	if !cmd.Approved && refund.Status != domain.RefundStatusRequested {
		return domain.RefundRequest{}, domain.ErrInvalidOrderState
	}
	if !cmd.Approved {
		reviewedRefund := refund
		reviewedRefund.OperatorID = operatorID
		reviewedRefund.AdminNote = adminNote
		reviewedRefund.ReviewedAt = &now
		event, err := newRefundReviewedEvent(reviewedRefund, RefundRejectedEventType, domain.RefundStatusRejected, nil, now)
		if err != nil {
			return domain.RefundRequest{}, err
		}
		return s.repo.RejectRefundRequest(ctx, cmd.RefundID, operatorID, adminNote, now, event)
	}
	order, err := s.repo.GetOrder(ctx, refund.OrderID)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if orderContainsMembershipGrant(order) {
		return domain.RefundRequest{}, domain.ErrMembershipRefundUnavailable
	}
	refund, err = s.repo.StartRefundApproval(ctx, cmd.RefundID, operatorID, adminNote, cmd.RestoreStock, now)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if refund.Status == domain.RefundStatusApproved || refund.Status == domain.RefundStatusRejected {
		return refund, nil
	}
	if refund.AmountCredits > 0 {
		if s.charger == nil {
			return domain.RefundRequest{}, errors.New("credit charger not configured")
		}
		if err := s.charger.AdjustCredits(ctx, CreditAdjustCommand{
			UserID:        refund.UserID,
			Delta:         refund.AmountCredits,
			Reason:        "mall_order_refund",
			Description:   fmt.Sprintf("订单 %s 售后退款", refund.OrderNo),
			SourceEventID: refundSourceEventID(refund.ID),
			SourceType:    "mall_refund",
			SourceID:      refund.ID,
		}); err != nil {
			return domain.RefundRequest{}, err
		}
	}
	event, err := newRefundReviewedEvent(refund, RefundApprovedEventType, domain.RefundStatusApproved, order.DigitalEntitlements, now)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	return s.repo.CompleteRefundApproval(ctx, cmd.RefundID, now, event)
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
		if normalizeDigitalGrantType(item.GrantType, item.GrantKey) == "membership" {
			return true
		}
	}
	for _, entitlement := range order.DigitalEntitlements {
		if normalizeDigitalGrantType(entitlement.GrantType, entitlement.GrantKey) == "membership" {
			return true
		}
	}
	return false
}

func validateAdminOrderFulfillment(order domain.Order, nextStatus domain.OrderStatus, fulfillment domain.OrderFulfillment, note string) error {
	if !orderRequiresShipping(order) {
		return nil
	}
	hasCarrierOrTracking := strings.TrimSpace(order.ShippingCarrier) != "" ||
		strings.TrimSpace(order.TrackingNo) != "" ||
		strings.TrimSpace(fulfillment.ShippingCarrier) != "" ||
		strings.TrimSpace(fulfillment.TrackingNo) != ""
	switch nextStatus {
	case domain.OrderStatusShipped:
		if !hasCarrierOrTracking {
			return fmt.Errorf("%w: shipping carrier or tracking number is required", domain.ErrInvalidOrderState)
		}
	case domain.OrderStatusCompleted:
		if !hasCarrierOrTracking && strings.TrimSpace(note) == "" {
			return fmt.Errorf("%w: fulfillment evidence is required", domain.ErrInvalidOrderState)
		}
	}
	return nil
}

func orderRequiresShipping(order domain.Order) bool {
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

func orderItemRequiresShipping(item domain.OrderItem) bool {
	if strings.EqualFold(strings.TrimSpace(item.Category), "digital") {
		return false
	}
	return normalizeDigitalGrantType(item.GrantType, item.GrantKey) == ""
}

func normalizeRefundNote(note string) (string, error) {
	trimmed := strings.TrimSpace(note)
	if utf8.RuneCountInString(trimmed) < 4 {
		return "", errors.New("refund note must be at least 4 characters")
	}
	return trimmed, nil
}

func refundSourceEventID(refundID int64) string {
	return fmt.Sprintf("mall.refund:%d", refundID)
}

func (s *Service) ListOrderStatusLogs(ctx context.Context, orderID int64) ([]domain.OrderStatusLog, error) {
	if orderID <= 0 {
		return nil, errors.New("order id is required")
	}
	return s.repo.ListOrderStatusLogs(ctx, orderID)
}

func (s *Service) ListUserOrderStatusLogs(ctx context.Context, orderID, userID int64) ([]domain.OrderStatusLog, error) {
	if _, err := s.GetUserOrder(ctx, orderID, userID); err != nil {
		return nil, err
	}
	return s.ListOrderStatusLogs(ctx, orderID)
}

func (s *Service) ListOrderPayments(ctx context.Context, orderID int64) ([]domain.Payment, error) {
	if orderID <= 0 {
		return nil, errors.New("order id is required")
	}
	return s.repo.ListOrderPayments(ctx, orderID)
}

func (s *Service) ListUserOrderPayments(ctx context.Context, orderID, userID int64) ([]domain.Payment, error) {
	if _, err := s.GetUserOrder(ctx, orderID, userID); err != nil {
		return nil, err
	}
	return s.ListOrderPayments(ctx, orderID)
}

func (s *Service) ListAddresses(ctx context.Context, cmd ListAddressesCommand) ([]domain.Address, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListAddresses(ctx, cmd.UserID, domain.NormalizeListLimit(cmd.Limit), domain.NormalizeOffset(cmd.Offset))
}

func (s *Service) CreateAddress(ctx context.Context, cmd SaveAddressCommand) (domain.Address, error) {
	address, err := commandToAddress(cmd)
	if err != nil {
		return domain.Address{}, err
	}
	now := s.now().UTC()
	address.CreatedAt = now
	address.UpdatedAt = now
	return s.repo.CreateAddress(ctx, address)
}

func (s *Service) UpdateAddress(ctx context.Context, cmd SaveAddressCommand) (domain.Address, error) {
	if cmd.ID <= 0 {
		return domain.Address{}, errors.New("address id is required")
	}
	address, err := commandToAddress(cmd)
	if err != nil {
		return domain.Address{}, err
	}
	address.ID = cmd.ID
	address.UpdatedAt = s.now().UTC()
	return s.repo.UpdateAddress(ctx, address)
}

func (s *Service) DeleteAddress(ctx context.Context, userID, addressID int64) (bool, error) {
	if userID <= 0 {
		return false, errors.New("user id is required")
	}
	if addressID <= 0 {
		return false, errors.New("address id is required")
	}
	return s.repo.DeleteAddress(ctx, userID, addressID)
}

func (s *Service) SetDefaultAddress(ctx context.Context, userID, addressID int64) (domain.Address, error) {
	if userID <= 0 {
		return domain.Address{}, errors.New("user id is required")
	}
	if addressID <= 0 {
		return domain.Address{}, errors.New("address id is required")
	}
	return s.repo.SetDefaultAddress(ctx, userID, addressID, s.now().UTC())
}

func commandToAddress(cmd SaveAddressCommand) (domain.Address, error) {
	receiver, err := domain.NormalizeRequired(cmd.Receiver, "receiver")
	if err != nil {
		return domain.Address{}, err
	}
	phone, err := domain.NormalizeRequired(cmd.Phone, "phone")
	if err != nil {
		return domain.Address{}, err
	}
	detail, err := domain.NormalizeRequired(cmd.Detail, "address detail")
	if err != nil {
		return domain.Address{}, err
	}
	if cmd.UserID <= 0 {
		return domain.Address{}, errors.New("user id is required")
	}
	return domain.Address{
		UserID:     cmd.UserID,
		Receiver:   receiver,
		Phone:      phone,
		Province:   strings.TrimSpace(cmd.Province),
		City:       strings.TrimSpace(cmd.City),
		District:   strings.TrimSpace(cmd.District),
		Detail:     detail,
		PostalCode: strings.TrimSpace(cmd.PostalCode),
		IsDefault:  cmd.IsDefault,
	}, nil
}

type orderPaidEventPayload struct {
	EventID          string                  `json:"event_id"`
	EventType        string                  `json:"event_type"`
	OccurredAtUnixMs int64                   `json:"occurred_at_unix_ms"`
	OrderID          int64                   `json:"order_id"`
	OrderNo          string                  `json:"order_no"`
	UserID           int64                   `json:"user_id"`
	TotalCredits     int64                   `json:"total_credits"`
	PaymentMethod    string                  `json:"payment_method"`
	PaymentID        int64                   `json:"payment_id"`
	Items            []orderPaidEventItemDTO `json:"items"`
}

type orderStatusUpdatedEventPayload struct {
	EventID          string `json:"event_id"`
	EventType        string `json:"event_type"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	OrderID          int64  `json:"order_id"`
	OrderNo          string `json:"order_no"`
	UserID           int64  `json:"user_id"`
	Status           string `json:"status"`
	ShippingCarrier  string `json:"shipping_carrier"`
	TrackingNo       string `json:"tracking_no"`
	OperatorID       string `json:"operator_id"`
	Note             string `json:"note"`
}

type orderPaidEventItemDTO struct {
	ProductID        int64  `json:"product_id"`
	SKU              string `json:"sku"`
	Title            string `json:"title"`
	Category         string `json:"category"`
	Quantity         int32  `json:"quantity"`
	UnitPriceCredits int64  `json:"unit_price_credits"`
	SubtotalCredits  int64  `json:"subtotal_credits"`
}

func orderStatusUpdatedEventType(status domain.OrderStatus) string {
	switch status {
	case domain.OrderStatusShipped:
		return OrderShippedEventType
	case domain.OrderStatusCompleted:
		return OrderCompletedEventType
	default:
		return ""
	}
}

func newOrderStatusUpdatedEvent(order domain.Order, eventType string, status domain.OrderStatus, fulfillment domain.OrderFulfillment, note, operatorID string, occurredAt time.Time) (domain.OutboxEvent, error) {
	if eventType == "" {
		return domain.OutboxEvent{}, domain.ErrInvalidOrderState
	}
	eventID := uuid.NewString()
	shippingCarrier := strings.TrimSpace(fulfillment.ShippingCarrier)
	if shippingCarrier == "" {
		shippingCarrier = order.ShippingCarrier
	}
	trackingNo := strings.TrimSpace(fulfillment.TrackingNo)
	if trackingNo == "" {
		trackingNo = order.TrackingNo
	}
	payload, err := json.Marshal(orderStatusUpdatedEventPayload{
		EventID:          eventID,
		EventType:        eventType,
		OccurredAtUnixMs: occurredAt.UnixMilli(),
		OrderID:          order.ID,
		OrderNo:          order.OrderNo,
		UserID:           order.UserID,
		Status:           string(status),
		ShippingCarrier:  shippingCarrier,
		TrackingNo:       trackingNo,
		OperatorID:       operatorID,
		Note:             strings.TrimSpace(note),
	})
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{
		EventID:       eventID,
		AggregateType: "mall_order",
		AggregateID:   order.ID,
		EventType:     eventType,
		MessageKey:    fmt.Sprintf("%d", order.UserID),
		PayloadJSON:   string(payload),
		Payload:       payload,
		CreatedAt:     occurredAt,
	}, nil
}

type refundReviewedEventPayload struct {
	EventID             string                                `json:"event_id"`
	EventType           string                                `json:"event_type"`
	OccurredAtUnixMs    int64                                 `json:"occurred_at_unix_ms"`
	RefundID            int64                                 `json:"refund_id"`
	OrderID             int64                                 `json:"order_id"`
	OrderNo             string                                `json:"order_no"`
	UserID              int64                                 `json:"user_id"`
	AmountCredits       int64                                 `json:"amount_credits"`
	Status              string                                `json:"status"`
	Reason              string                                `json:"reason"`
	AdminNote           string                                `json:"admin_note"`
	RestoreStock        bool                                  `json:"restore_stock"`
	DigitalEntitlements []refundReviewedDigitalEntitlementDTO `json:"digital_entitlements,omitempty"`
}

type refundReviewedDigitalEntitlementDTO struct {
	ProductID       int64  `json:"product_id"`
	SKU             string `json:"sku"`
	Title           string `json:"title"`
	Quantity        int32  `json:"quantity"`
	FulfillmentCode string `json:"fulfillment_code"`
	GrantType       string `json:"grant_type"`
	GrantKey        string `json:"grant_key"`
	Status          string `json:"status"`
	RefundID        int64  `json:"refund_id"`
}

type digitalEntitlementRevokedEventPayload struct {
	EventID          string `json:"event_id"`
	EventType        string `json:"event_type"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	EntitlementID    int64  `json:"entitlement_id"`
	OrderID          int64  `json:"order_id"`
	OrderNo          string `json:"order_no"`
	UserID           int64  `json:"user_id"`
	ProductID        int64  `json:"product_id"`
	SKU              string `json:"sku"`
	Title            string `json:"title"`
	FulfillmentCode  string `json:"fulfillment_code"`
	GrantType        string `json:"grant_type"`
	GrantKey         string `json:"grant_key"`
	Status           string `json:"status"`
	OperatorID       string `json:"operator_id"`
	Reason           string `json:"reason"`
}

type productReviewStatusEventPayload struct {
	EventID          string `json:"event_id"`
	EventType        string `json:"event_type"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	ReviewID         int64  `json:"review_id"`
	ProductID        int64  `json:"product_id"`
	ProductTitle     string `json:"product_title"`
	OrderID          int64  `json:"order_id"`
	UserID           int64  `json:"user_id"`
	Status           string `json:"status"`
}

func productReviewStatusEventType(status domain.ProductReviewStatus) string {
	switch status {
	case domain.ProductReviewStatusPublished:
		return ReviewPublishedEventType
	case domain.ProductReviewStatusHidden:
		return ReviewHiddenEventType
	default:
		return ""
	}
}

func newProductReviewStatusEvent(review domain.ProductReview, eventType string, status domain.ProductReviewStatus, occurredAt time.Time) (domain.OutboxEvent, error) {
	eventID := uuid.NewString()
	payload, err := json.Marshal(productReviewStatusEventPayload{
		EventID:          eventID,
		EventType:        eventType,
		OccurredAtUnixMs: occurredAt.UnixMilli(),
		ReviewID:         review.ID,
		ProductID:        review.ProductID,
		ProductTitle:     review.ProductTitle,
		OrderID:          review.OrderID,
		UserID:           review.UserID,
		Status:           string(status),
	})
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{
		EventID:       eventID,
		AggregateType: "mall_product_review",
		AggregateID:   review.ID,
		EventType:     eventType,
		MessageKey:    fmt.Sprintf("%d", review.UserID),
		PayloadJSON:   string(payload),
		Payload:       payload,
		CreatedAt:     occurredAt,
	}, nil
}

func newRefundReviewedEvent(refund domain.RefundRequest, eventType string, status domain.RefundStatus, digitalEntitlements []domain.DigitalEntitlement, occurredAt time.Time) (domain.OutboxEvent, error) {
	eventID := uuid.NewString()
	payload, err := json.Marshal(refundReviewedEventPayload{
		EventID:             eventID,
		EventType:           eventType,
		OccurredAtUnixMs:    occurredAt.UnixMilli(),
		RefundID:            refund.ID,
		OrderID:             refund.OrderID,
		OrderNo:             refund.OrderNo,
		UserID:              refund.UserID,
		AmountCredits:       refund.AmountCredits,
		Status:              string(status),
		Reason:              refund.Reason,
		AdminNote:           refund.AdminNote,
		RestoreStock:        refund.RestoreStock,
		DigitalEntitlements: refundReviewedDigitalEntitlementDTOs(refund.ID, status, digitalEntitlements, occurredAt),
	})
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{
		EventID:       eventID,
		AggregateType: "mall_refund",
		AggregateID:   refund.ID,
		EventType:     eventType,
		MessageKey:    fmt.Sprintf("%d", refund.UserID),
		PayloadJSON:   string(payload),
		Payload:       payload,
		CreatedAt:     occurredAt,
	}, nil
}

func refundReviewedDigitalEntitlementDTOs(refundID int64, status domain.RefundStatus, entitlements []domain.DigitalEntitlement, occurredAt time.Time) []refundReviewedDigitalEntitlementDTO {
	if len(entitlements) == 0 {
		return nil
	}
	items := make([]refundReviewedDigitalEntitlementDTO, 0, len(entitlements))
	entitlementStatus := domain.DigitalEntitlementStatusActive
	if status == domain.RefundStatusApproved {
		entitlementStatus = domain.DigitalEntitlementStatusRevoked
	}
	for _, item := range entitlements {
		if status == domain.RefundStatusApproved && !refundRevocableDigitalEntitlement(item, occurredAt) {
			continue
		}
		items = append(items, refundReviewedDigitalEntitlementDTO{
			ProductID:       item.ProductID,
			SKU:             item.SKU,
			Title:           item.Title,
			Quantity:        item.Quantity,
			FulfillmentCode: item.Code,
			GrantType:       item.GrantType,
			GrantKey:        item.GrantKey,
			Status:          entitlementStatus,
			RefundID:        refundID,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func refundRevocableDigitalEntitlement(item domain.DigitalEntitlement, now time.Time) bool {
	if !strings.EqualFold(strings.TrimSpace(item.Status), domain.DigitalEntitlementStatusActive) || item.RevokedAt != nil {
		return false
	}
	return item.ExpiresAt == nil || item.ExpiresAt.After(now)
}

func revocableDigitalEntitlement(item domain.DigitalEntitlement, now time.Time) bool {
	if !strings.EqualFold(strings.TrimSpace(item.Status), domain.DigitalEntitlementStatusActive) || item.RevokedAt != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(item.GrantType), "membership") {
		return item.ExpiresAt != nil && item.ExpiresAt.After(now)
	}
	return item.ExpiresAt == nil || item.ExpiresAt.After(now)
}

func digitalEntitlementAlreadyRevoked(item domain.DigitalEntitlement) bool {
	return strings.EqualFold(strings.TrimSpace(item.Status), domain.DigitalEntitlementStatusRevoked) || item.RevokedAt != nil
}

func newDigitalEntitlementRevokedEvent(entitlement domain.DigitalEntitlement, operatorID, reason string, occurredAt time.Time) (domain.OutboxEvent, error) {
	eventID := uuid.NewString()
	payload, err := json.Marshal(digitalEntitlementRevokedEventPayload{
		EventID:          eventID,
		EventType:        EntitlementRevokedEventType,
		OccurredAtUnixMs: occurredAt.UnixMilli(),
		EntitlementID:    entitlement.ID,
		OrderID:          entitlement.OrderID,
		OrderNo:          entitlement.OrderNo,
		UserID:           entitlement.UserID,
		ProductID:        entitlement.ProductID,
		SKU:              entitlement.SKU,
		Title:            entitlement.Title,
		FulfillmentCode:  entitlement.Code,
		GrantType:        entitlement.GrantType,
		GrantKey:         entitlement.GrantKey,
		Status:           domain.DigitalEntitlementStatusRevoked,
		OperatorID:       strings.TrimSpace(operatorID),
		Reason:           strings.TrimSpace(reason),
	})
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{
		EventID:       eventID,
		AggregateType: "mall_digital_entitlement",
		AggregateID:   entitlement.ID,
		EventType:     EntitlementRevokedEventType,
		MessageKey:    fmt.Sprintf("%d", entitlement.UserID),
		PayloadJSON:   string(payload),
		Payload:       payload,
		CreatedAt:     occurredAt,
	}, nil
}

func newOrderPaidEvent(order domain.Order, payment domain.Payment, paidAt time.Time) (domain.OutboxEvent, error) {
	eventID := uuid.NewString()
	items := make([]orderPaidEventItemDTO, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, orderPaidEventItemDTO{
			ProductID:        item.ProductID,
			SKU:              item.SKU,
			Title:            item.Title,
			Category:         item.Category,
			Quantity:         item.Quantity,
			UnitPriceCredits: item.UnitPriceCredits,
			SubtotalCredits:  item.SubtotalCredits,
		})
	}
	payload, err := json.Marshal(orderPaidEventPayload{
		EventID:          eventID,
		EventType:        OrderPaidEventType,
		OccurredAtUnixMs: paidAt.UnixMilli(),
		OrderID:          order.ID,
		OrderNo:          order.OrderNo,
		UserID:           order.UserID,
		TotalCredits:     order.TotalCredits,
		PaymentMethod:    payment.Provider,
		PaymentID:        payment.ID,
		Items:            items,
	})
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{
		EventID:       eventID,
		AggregateType: "mall_order",
		AggregateID:   order.ID,
		EventType:     OrderPaidEventType,
		MessageKey:    fmt.Sprintf("%d", order.UserID),
		PayloadJSON:   string(payload),
		Payload:       payload,
		CreatedAt:     paidAt,
	}, nil
}

func newOrderNo(now time.Time) string {
	return fmt.Sprintf("MO%s%s", now.Format("20060102150405"), strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
}
