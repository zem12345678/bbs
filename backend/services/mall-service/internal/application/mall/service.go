package mall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/google/uuid"
)

const (
	OrderPaidEventType      = "mall.order.paid.v1"
	OrderShippedEventType   = "mall.order.shipped.v1"
	OrderCompletedEventType = "mall.order.completed.v1"
	RefundApprovedEventType = "mall.refund.approved.v1"
	RefundRejectedEventType = "mall.refund.rejected.v1"
)

const (
	DefaultOrderExpireAfter = 30 * time.Minute
	DefaultOrderExpireLimit = 100
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

type CreateProductCommand struct {
	SKU          string
	Title        string
	Description  string
	Category     string
	CoverURL     string
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

type CloseExpiredOrdersCommand struct {
	ExpireAfter time.Duration
	Limit       int
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

func (s *Service) GetProduct(ctx context.Context, id int64) (domain.Product, error) {
	if id <= 0 {
		return domain.Product{}, errors.New("product id is required")
	}
	return s.repo.GetProduct(ctx, id)
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

func (s *Service) AdminMallOverview(ctx context.Context, cmd AdminMallOverviewCommand) (domain.MallOverview, error) {
	threshold := cmd.LowStockThreshold
	if threshold <= 0 {
		threshold = 10
	}
	return s.repo.AdminMallOverview(ctx, threshold)
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

func (s *Service) UpdateProduct(ctx context.Context, cmd UpdateProductCommand) (domain.Product, error) {
	if cmd.ID <= 0 {
		return domain.Product{}, errors.New("product id is required")
	}
	product, err := commandToProduct(CreateProductCommand{
		SKU:          cmd.SKU,
		Title:        cmd.Title,
		Description:  cmd.Description,
		Category:     cmd.Category,
		CoverURL:     cmd.CoverURL,
		PriceCredits: cmd.PriceCredits,
		Stock:        cmd.Stock,
		Status:       cmd.Status,
		Sort:         cmd.Sort,
	})
	if err != nil {
		return domain.Product{}, err
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
	return domain.Product{
		SKU:          sku,
		Title:        title,
		Description:  strings.TrimSpace(cmd.Description),
		Category:     strings.TrimSpace(cmd.Category),
		CoverURL:     strings.TrimSpace(cmd.CoverURL),
		PriceCredits: cmd.PriceCredits,
		Stock:        cmd.Stock,
		Status:       domain.NormalizeProductStatus(cmd.Status),
		Sort:         cmd.Sort,
	}, nil
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
	if existing, err := s.repo.GetOrderByIdempotencyKey(ctx, idempotencyKey); err == nil {
		return CreateOrderResult{Order: existing, Duplicate: true}, nil
	} else if !errors.Is(err, domain.ErrOrderNotFound) {
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
		if productRequiresShipping(product) {
			requiresShipping = true
		}
		subtotal := product.PriceCredits * int64(item.Quantity)
		total += subtotal
		orderItems = append(orderItems, domain.OrderItem{
			ProductID:        product.ID,
			SKU:              product.SKU,
			Title:            product.Title,
			Quantity:         item.Quantity,
			UnitPriceCredits: product.PriceCredits,
			SubtotalCredits:  subtotal,
		})
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
	saved, duplicate, err := s.repo.CreateOrder(ctx, order)
	if err != nil {
		return CreateOrderResult{}, err
	}
	return CreateOrderResult{Order: saved, Duplicate: duplicate}, nil
}

func (s *Service) CheckoutCart(ctx context.Context, cmd CheckoutCartCommand) (CreateOrderResult, error) {
	idempotencyKey, err := domain.NormalizeRequired(cmd.IdempotencyKey, "idempotency key")
	if err != nil {
		return CreateOrderResult{}, err
	}
	if existing, err := s.repo.GetOrderByIdempotencyKey(ctx, idempotencyKey); err == nil {
		return CreateOrderResult{Order: existing, Duplicate: true}, nil
	} else if !errors.Is(err, domain.ErrOrderNotFound) {
		return CreateOrderResult{}, err
	}
	if cmd.UserID <= 0 {
		return CreateOrderResult{}, errors.New("user id is required")
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
		subtotal := product.PriceCredits * int64(item.Quantity)
		total += subtotal
		orderItems = append(orderItems, domain.OrderItem{
			ProductID:        product.ID,
			SKU:              product.SKU,
			Title:            product.Title,
			Quantity:         item.Quantity,
			UnitPriceCredits: product.PriceCredits,
			SubtotalCredits:  subtotal,
		})
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

func productRequiresShipping(product domain.Product) bool {
	return !strings.EqualFold(strings.TrimSpace(product.Category), "digital")
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

func (s *Service) GetOrder(ctx context.Context, orderID int64) (domain.Order, error) {
	if orderID <= 0 {
		return domain.Order{}, errors.New("order id is required")
	}
	return s.repo.GetOrder(ctx, orderID)
}

func (s *Service) ListOrders(ctx context.Context, cmd ListOrdersCommand) ([]domain.Order, int64, error) {
	if cmd.UserID <= 0 {
		return nil, 0, errors.New("user id is required")
	}
	return s.repo.ListOrdersByUser(ctx, domain.OrderListQuery{
		UserID: cmd.UserID,
		Limit:  domain.NormalizeListLimit(cmd.Limit),
		Offset: domain.NormalizeOffset(cmd.Offset),
	})
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

	now := s.now().UTC()
	if s.orderExpireAfter > 0 {
		closed, ok, err := s.repo.CloseExpiredOrder(ctx, cmd.OrderID, cmd.UserID, now.Add(-s.orderExpireAfter), now)
		if err != nil {
			return domain.Order{}, err
		}
		if ok {
			return closed, domain.ErrInvalidOrderState
		}
	}
	order, payment, err := s.repo.BeginOrderPayment(ctx, cmd.OrderID, cmd.UserID, paymentMethod, idempotencyKey, now)
	if err != nil {
		return domain.Order{}, err
	}
	if order.Status == domain.OrderStatusPaid {
		return order, nil
	}
	if s.charger == nil {
		_ = s.repo.FailOrderPayment(ctx, order.ID, order.UserID, payment.ID, "credit charger not configured", s.now().UTC())
		return domain.Order{}, errors.New("credit charger not configured")
	}
	if order.TotalCredits > 0 {
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
			_ = s.repo.FailOrderPayment(ctx, order.ID, order.UserID, payment.ID, err.Error(), s.now().UTC())
			return domain.Order{}, err
		}
	}
	paidAt := s.now().UTC()
	event, err := newOrderPaidEvent(order, payment, paidAt)
	if err != nil {
		_ = s.repo.FailOrderPayment(ctx, order.ID, order.UserID, payment.ID, err.Error(), s.now().UTC())
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
	event, err := newOrderStatusUpdatedEvent(order, orderStatusUpdatedEventType(nextStatus), nextStatus, domain.OrderFulfillment{
		ShippingCarrier: strings.TrimSpace(cmd.ShippingCarrier),
		TrackingNo:      strings.TrimSpace(cmd.TrackingNo),
	}, strings.TrimSpace(cmd.Note), operatorID, s.now().UTC())
	if err != nil {
		return domain.Order{}, err
	}
	return s.repo.AdminUpdateOrderStatus(ctx, cmd.OrderID, nextStatus, operatorID, domain.OrderFulfillment{
		ShippingCarrier: strings.TrimSpace(cmd.ShippingCarrier),
		TrackingNo:      strings.TrimSpace(cmd.TrackingNo),
	}, strings.TrimSpace(cmd.Note), event.CreatedAt, event)
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
	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		reason = "after_sale"
	}
	now := s.now().UTC()
	return s.repo.CreateRefundRequest(ctx, domain.RefundRequest{
		OrderID:       order.ID,
		OrderNo:       order.OrderNo,
		UserID:        order.UserID,
		AmountCredits: order.TotalCredits,
		Status:        domain.RefundStatusRequested,
		Reason:        reason,
		UserNote:      strings.TrimSpace(cmd.Note),
		RequestedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
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
	if !cmd.Approved {
		event, err := newRefundReviewedEvent(refund, RefundRejectedEventType, domain.RefundStatusRejected, now)
		if err != nil {
			return domain.RefundRequest{}, err
		}
		return s.repo.RejectRefundRequest(ctx, cmd.RefundID, operatorID, adminNote, now, event)
	}
	refund, err = s.repo.StartRefundApproval(ctx, cmd.RefundID, operatorID, adminNote, cmd.RestoreStock, now)
	if err != nil {
		return domain.RefundRequest{}, err
	}
	if refund.Status == domain.RefundStatusApproved || refund.Status == domain.RefundStatusRejected {
		return refund, nil
	}
	if s.charger == nil {
		return domain.RefundRequest{}, errors.New("credit charger not configured")
	}
	if refund.AmountCredits > 0 {
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
	event, err := newRefundReviewedEvent(refund, RefundApprovedEventType, domain.RefundStatusApproved, now)
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

func refundSourceEventID(refundID int64) string {
	return fmt.Sprintf("mall.refund:%d", refundID)
}

func (s *Service) ListOrderStatusLogs(ctx context.Context, orderID int64) ([]domain.OrderStatusLog, error) {
	if orderID <= 0 {
		return nil, errors.New("order id is required")
	}
	return s.repo.ListOrderStatusLogs(ctx, orderID)
}

func (s *Service) ListOrderPayments(ctx context.Context, orderID int64) ([]domain.Payment, error) {
	if orderID <= 0 {
		return nil, errors.New("order id is required")
	}
	return s.repo.ListOrderPayments(ctx, orderID)
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
	EventID          string `json:"event_id"`
	EventType        string `json:"event_type"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	RefundID         int64  `json:"refund_id"`
	OrderID          int64  `json:"order_id"`
	OrderNo          string `json:"order_no"`
	UserID           int64  `json:"user_id"`
	AmountCredits    int64  `json:"amount_credits"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	AdminNote        string `json:"admin_note"`
	RestoreStock     bool   `json:"restore_stock"`
}

func newRefundReviewedEvent(refund domain.RefundRequest, eventType string, status domain.RefundStatus, occurredAt time.Time) (domain.OutboxEvent, error) {
	eventID := uuid.NewString()
	payload, err := json.Marshal(refundReviewedEventPayload{
		EventID:          eventID,
		EventType:        eventType,
		OccurredAtUnixMs: occurredAt.UnixMilli(),
		RefundID:         refund.ID,
		OrderID:          refund.OrderID,
		OrderNo:          refund.OrderNo,
		UserID:           refund.UserID,
		AmountCredits:    refund.AmountCredits,
		Status:           string(status),
		Reason:           refund.Reason,
		AdminNote:        refund.AdminNote,
		RestoreStock:     refund.RestoreStock,
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

func newOrderPaidEvent(order domain.Order, payment domain.Payment, paidAt time.Time) (domain.OutboxEvent, error) {
	eventID := uuid.NewString()
	items := make([]orderPaidEventItemDTO, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, orderPaidEventItemDTO{
			ProductID:        item.ProductID,
			SKU:              item.SKU,
			Title:            item.Title,
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
		MessageKey:    fmt.Sprintf("%d", order.ID),
		PayloadJSON:   string(payload),
		Payload:       payload,
		CreatedAt:     paidAt,
	}, nil
}

func newOrderNo(now time.Time) string {
	return fmt.Sprintf("MO%s%s", now.Format("20060102150405"), strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
}
