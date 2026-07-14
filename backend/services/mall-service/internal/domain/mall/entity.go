package mall

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	DefaultListLimit = 20
	MaxListLimit     = 100
)

type ProductStatus string

const (
	ProductStatusDraft    ProductStatus = "DRAFT"
	ProductStatusActive   ProductStatus = "ACTIVE"
	ProductStatusArchived ProductStatus = "ARCHIVED"
)

type ProductCategoryStatus string

const (
	ProductCategoryStatusDraft    ProductCategoryStatus = "DRAFT"
	ProductCategoryStatusActive   ProductCategoryStatus = "ACTIVE"
	ProductCategoryStatusArchived ProductCategoryStatus = "ARCHIVED"
)

type ProductReviewStatus string

const (
	ProductReviewStatusPending   ProductReviewStatus = "PENDING"
	ProductReviewStatusPublished ProductReviewStatus = "PUBLISHED"
	ProductReviewStatusHidden    ProductReviewStatus = "HIDDEN"
)

type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "PENDING_PAYMENT"
	OrderStatusPaying         OrderStatus = "PAYING"
	OrderStatusPaid           OrderStatus = "PAID"
	OrderStatusCanceled       OrderStatus = "CANCELED"
	OrderStatusShipped        OrderStatus = "SHIPPED"
	OrderStatusCompleted      OrderStatus = "COMPLETED"
	OrderStatusClosed         OrderStatus = "CLOSED"
	OrderStatusRefunded       OrderStatus = "REFUNDED"
)

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "PENDING"
	PaymentStatusSucceeded PaymentStatus = "SUCCEEDED"
	PaymentStatusFailed    PaymentStatus = "FAILED"
)

type RefundStatus string

const (
	RefundStatusRequested  RefundStatus = "REQUESTED"
	RefundStatusProcessing RefundStatus = "PROCESSING"
	RefundStatusApproved   RefundStatus = "APPROVED"
	RefundStatusRejected   RefundStatus = "REJECTED"
)

type CouponStatus string

const (
	CouponStatusDraft    CouponStatus = "DRAFT"
	CouponStatusActive   CouponStatus = "ACTIVE"
	CouponStatusArchived CouponStatus = "ARCHIVED"
)

type CouponUsageStatus string

const (
	CouponUsageStatusClaimed  CouponUsageStatus = "CLAIMED"
	CouponUsageStatusReserved CouponUsageStatus = "RESERVED"
	CouponUsageStatusUsed     CouponUsageStatus = "USED"
	CouponUsageStatusReleased CouponUsageStatus = "RELEASED"
)

const (
	PaymentProviderCredits = "credits"

	OrderStatusReasonCreated       = "created"
	OrderStatusReasonPaying        = "paying"
	OrderStatusReasonPaid          = "paid"
	OrderStatusReasonPaymentFailed = "payment_failed"
	OrderStatusReasonCanceled      = "canceled_by_user"
	OrderStatusReasonExpired       = "expired"
	OrderStatusReasonShipped       = "shipped"
	OrderStatusReasonCompleted     = "completed"
	OrderStatusReasonRefunded      = "refunded"

	OrderStatusOperatorUser  = "user"
	OrderStatusOperatorAdmin = "admin"

	StockChangeReasonProductCreated   = "product_created"
	StockChangeReasonManualAdjustment = "manual_adjustment"
	StockChangeReasonOrderCreated     = "order_created"
	StockChangeReasonOrderCanceled    = "order_canceled"
	StockChangeReasonOrderExpired     = "order_expired"
	StockChangeReasonRefundRestored   = "refund_restored"

	StockReferenceProduct = "product"
	StockReferenceOrder   = "order"
	StockReferenceRefund  = "refund"
)

var (
	ErrProductNotFound            = errors.New("product not found")
	ErrOrderNotFound              = errors.New("order not found")
	ErrOrderOwnerMismatch         = errors.New("order does not belong to user")
	ErrInvalidOrderState          = errors.New("invalid order state")
	ErrDuplicateReference         = errors.New("duplicate reference")
	ErrProductUnavailable         = errors.New("product unavailable")
	ErrInsufficientStock          = errors.New("insufficient stock")
	ErrInsufficientCredits        = errors.New("insufficient credits")
	ErrUnsupportedPayment         = errors.New("unsupported payment method")
	ErrOutboxEventNotFound        = errors.New("outbox event not found")
	ErrInvalidOutboxStatus        = errors.New("invalid outbox status")
	ErrRefundNotFound             = errors.New("refund request not found")
	ErrAddressNotFound            = errors.New("address not found")
	ErrCouponNotFound             = errors.New("coupon not found")
	ErrCouponUnavailable          = errors.New("coupon unavailable")
	ErrProductCategoryNotFound    = errors.New("product category not found")
	ErrProductReviewNotFound      = errors.New("product review not found")
	ErrDigitalEntitlementNotFound = errors.New("digital entitlement not found")
)

type Product struct {
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
	SalesCount   int64
	Status       ProductStatus
	Sort         int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ProductCategory struct {
	ID           int64
	Slug         string
	Name         string
	Description  string
	Status       ProductCategoryStatus
	Sort         int32
	ProductCount int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ProductReview struct {
	ID           int64
	ProductID    int64
	ProductSKU   string
	ProductTitle string
	OrderID      int64
	UserID       int64
	Rating       int32
	Content      string
	Status       ProductReviewStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Order struct {
	ID                  int64
	OrderNo             string
	IdempotencyKey      string
	UserID              int64
	Items               []OrderItem
	OriginalCredits     int64
	DiscountCredits     int64
	TotalCredits        int64
	CouponID            int64
	CouponCode          string
	CouponUsageID       int64
	Status              OrderStatus
	Receiver            string
	Phone               string
	Address             string
	PaymentMethod       string
	ShippingCarrier     string
	TrackingNo          string
	PaidAt              *time.Time
	ShippedAt           *time.Time
	CompletedAt         *time.Time
	DigitalEntitlements []DigitalEntitlement
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type OrderItem struct {
	ProductID        int64
	SKU              string
	Title            string
	Category         string
	GrantType        string
	GrantKey         string
	Quantity         int32
	UnitPriceCredits int64
	SubtotalCredits  int64
}

type DigitalEntitlement struct {
	ID           int64
	OrderID      int64
	OrderNo      string
	UserID       int64
	ProductID    int64
	SKU          string
	Title        string
	Quantity     int32
	Code         string
	GrantType    string
	GrantKey     string
	IssuedAt     time.Time
	ExpiresAt    *time.Time
	Status       string
	RevokedAt    *time.Time
	RefundID     int64
	RevokedBy    string
	RevokeReason string
}

const (
	DigitalEntitlementStatusActive  = "ACTIVE"
	DigitalEntitlementStatusExpired = "EXPIRED"
	DigitalEntitlementStatusRevoked = "REVOKED"
)

type DigitalEntitlementListQuery struct {
	UserID    int64
	Status    string
	GrantType string
	GrantKey  string
	Keyword   string
	Limit     int
	Offset    int
}

type CreateOrderItem struct {
	ProductID int64
	Quantity  int32
}

type Payment struct {
	ID              int64
	OrderID         int64
	UserID          int64
	AmountCredits   int64
	Provider        string
	IdempotencyKey  string
	Status          PaymentStatus
	ProviderTradeNo string
	FailureReason   string
	PaidAt          *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PayingOrderPayment struct {
	OrderID        int64
	UserID         int64
	PaymentID      int64
	Provider       string
	IdempotencyKey string
}

type OrderStatusLog struct {
	ID           int64
	OrderID      int64
	FromStatus   OrderStatus
	ToStatus     OrderStatus
	Reason       string
	OperatorType string
	OperatorID   string
	Note         string
	CreatedAt    time.Time
}

type RefundRequest struct {
	ID            int64
	OrderID       int64
	OrderNo       string
	UserID        int64
	AmountCredits int64
	Status        RefundStatus
	Reason        string
	UserNote      string
	AdminNote     string
	RestoreStock  bool
	OperatorID    string
	RequestedAt   time.Time
	ReviewedAt    *time.Time
	RefundedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Address struct {
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
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CartItem struct {
	UserID    int64
	Product   Product
	Quantity  int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProductFavorite struct {
	UserID    int64
	Product   Product
	CreatedAt time.Time
}

type Coupon struct {
	ID              int64
	Code            string
	Name            string
	Description     string
	DiscountCredits int64
	MinOrderCredits int64
	TotalQuota      int64
	PerUserLimit    int64
	ClaimedCount    int64
	UsedCount       int64
	Status          CouponStatus
	StartsAt        *time.Time
	EndsAt          *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CouponUsage struct {
	ID              int64
	CouponID        int64
	Code            string
	UserID          int64
	OrderID         int64
	Status          CouponUsageStatus
	DiscountCredits int64
	CreatedAt       time.Time
	UsedAt          *time.Time
	ReleasedAt      *time.Time
	UpdatedAt       time.Time
	Coupon          Coupon
}

type ProductStockLog struct {
	ID            int64
	ProductID     int64
	SKU           string
	Title         string
	Delta         int64
	BeforeStock   int64
	AfterStock    int64
	Reason        string
	ReferenceType string
	ReferenceID   int64
	OperatorType  string
	OperatorID    string
	Note          string
	CreatedAt     time.Time
}

type ProductListQuery struct {
	Limit    int
	Offset   int
	Keyword  string
	Category string
	Status   ProductStatus
}

type ProductCategoryListQuery struct {
	Limit   int
	Offset  int
	Keyword string
	Status  ProductCategoryStatus
}

type ProductReviewListQuery struct {
	ProductID int64
	UserID    int64
	Status    ProductReviewStatus
	Limit     int
	Offset    int
}

type OrderListQuery struct {
	UserID  int64
	Limit   int
	Offset  int
	Keyword string
	Status  OrderStatus
}

type RefundListQuery struct {
	UserID  int64
	Limit   int
	Offset  int
	Status  RefundStatus
	Keyword string
}

type ProductStockLogQuery struct {
	ProductID int64
	Limit     int
	Offset    int
	Reason    string
}

type CouponListQuery struct {
	Limit   int
	Offset  int
	Keyword string
	Status  CouponStatus
}

type CouponUsageListQuery struct {
	CouponID int64
	UserID   int64
	Status   CouponUsageStatus
	Limit    int
	Offset   int
}

type StatusCount struct {
	Status string
	Count  int64
}

type FinanceAnomaly struct {
	IssueType               string
	OrderID                 int64
	OrderNo                 string
	UserID                  int64
	OrderStatus             OrderStatus
	OrderTotalCredits       int64
	SucceededPaymentCredits int64
	RefundedCredits         int64
	DifferenceCredits       int64
	UpdatedAt               time.Time
}

type MallOverview struct {
	ProductTotal                 int64
	ActiveProductTotal           int64
	LowStockTotal                int64
	StockTotal                   int64
	SalesCountTotal              int64
	OrderTotal                   int64
	PaidOrderTotal               int64
	RevenueCreditsTotal          int64
	TodayOrderTotal              int64
	TodayRevenueCredits          int64
	PendingShipmentTotal         int64
	PendingRefundTotal           int64
	RefundedCreditsTotal         int64
	SucceededPaymentCreditsTotal int64
	FailedPaymentTotal           int64
	FailedPaymentCreditsTotal    int64
	PendingRefundCreditsTotal    int64
	NetRevenueCreditsTotal       int64
	PendingOutboxTotal           int64
	OrderStatusCounts            []StatusCount
	RefundStatusCounts           []StatusCount
	OutboxStatusCounts           []StatusCount
	OutboxLastError              string
	OutboxLastErrorAt            *time.Time
	OutboxNextAttemptAt          *time.Time
	FinanceAnomalyTotal          int64
	FinanceAnomalies             []FinanceAnomaly
	LowStockProducts             []Product
	TopSellingProducts           []Product
}

type OrderFulfillment struct {
	ShippingCarrier string
	TrackingNo      string
}

type OutboxEvent struct {
	EventID       string
	AggregateType string
	AggregateID   int64
	EventType     string
	MessageKey    string
	PayloadJSON   string
	Payload       []byte
	Attempt       int
	CreatedAt     time.Time
}

type OutboxRequeueResult struct {
	Requeued int64
	EventIDs []string
}

type OutboxRequeueAudit struct {
	ID               int64
	EventID          string
	AggregateType    string
	AggregateID      int64
	PreviousStatus   string
	PreviousAttempts int
	PreviousError    string
	OperatorID       string
	RequeuedAt       time.Time
}

type OutboxRequeueAuditListQuery struct {
	EventID       string
	AggregateType string
	AggregateID   int64
	Limit         int
	Offset        int
}

type Repository interface {
	EnsureSchema(ctx context.Context) error
	ListProducts(ctx context.Context, query ProductListQuery) ([]Product, int64, error)
	ListProductCategories(ctx context.Context, query ProductCategoryListQuery) ([]ProductCategory, int64, error)
	GetProduct(ctx context.Context, productID int64) (Product, error)
	AdminListProducts(ctx context.Context, query ProductListQuery) ([]Product, int64, error)
	AdminListProductCategories(ctx context.Context, query ProductCategoryListQuery) ([]ProductCategory, int64, error)
	AdminCreateProductCategory(ctx context.Context, category ProductCategory) (ProductCategory, error)
	AdminUpdateProductCategory(ctx context.Context, category ProductCategory) (ProductCategory, error)
	CreateProduct(ctx context.Context, product Product, operatorID string) (Product, error)
	UpdateProduct(ctx context.Context, product Product, operatorID string) (Product, error)
	AdminListProductStockLogs(ctx context.Context, query ProductStockLogQuery) ([]ProductStockLog, int64, error)
	ListProductReviews(ctx context.Context, query ProductReviewListQuery) ([]ProductReview, int64, error)
	ListUserProductReviews(ctx context.Context, query ProductReviewListQuery) ([]ProductReview, int64, error)
	CreateProductReview(ctx context.Context, review ProductReview) (ProductReview, error)
	GetProductReview(ctx context.Context, reviewID int64) (ProductReview, error)
	AdminListProductReviews(ctx context.Context, query ProductReviewListQuery) ([]ProductReview, int64, error)
	AdminUpdateProductReviewStatus(ctx context.Context, reviewID int64, status ProductReviewStatus, updatedAt time.Time, event OutboxEvent) (ProductReview, error)
	ListAvailableCoupons(ctx context.Context, limit, offset int, now time.Time) ([]Coupon, int64, error)
	AdminListCoupons(ctx context.Context, query CouponListQuery) ([]Coupon, int64, error)
	AdminCreateCoupon(ctx context.Context, coupon Coupon) (Coupon, error)
	AdminUpdateCoupon(ctx context.Context, coupon Coupon) (Coupon, error)
	AdminListCouponUsages(ctx context.Context, query CouponUsageListQuery) ([]CouponUsage, int64, error)
	ClaimCoupon(ctx context.Context, userID int64, couponID int64, claimedAt time.Time) (CouponUsage, bool, error)
	ListCouponUsagesByUser(ctx context.Context, query CouponUsageListQuery) ([]CouponUsage, int64, error)
	CreateOrder(ctx context.Context, order Order) (Order, bool, error)
	CreateOrderFromCart(ctx context.Context, order Order) (Order, bool, error)
	GetOrder(ctx context.Context, orderID int64) (Order, error)
	GetOrderByIdempotencyKey(ctx context.Context, userID int64, idempotencyKey string) (Order, error)
	ListOrdersByUser(ctx context.Context, query OrderListQuery) ([]Order, int64, error)
	ListReviewableOrders(ctx context.Context, query OrderListQuery, productID int64) ([]Order, int64, error)
	AdminListOrders(ctx context.Context, query OrderListQuery) ([]Order, int64, error)
	ListDigitalEntitlements(ctx context.Context, query DigitalEntitlementListQuery) ([]DigitalEntitlement, int64, error)
	AdminRevokeDigitalEntitlement(ctx context.Context, entitlementID int64, operatorID string, reason string, revokedAt time.Time) (DigitalEntitlement, error)
	BeginOrderPayment(ctx context.Context, orderID, userID int64, paymentMethod, idempotencyKey string, now time.Time) (Order, Payment, error)
	CompleteOrderPayment(ctx context.Context, orderID, userID, paymentID int64, paidAt time.Time, event OutboxEvent) (Order, error)
	FailOrderPayment(ctx context.Context, orderID, userID, paymentID int64, reason string, failedAt time.Time) error
	ListStalePayingOrders(ctx context.Context, startedBefore time.Time, limit int) ([]PayingOrderPayment, error)
	CancelOrder(ctx context.Context, orderID, userID int64, canceledAt time.Time) (Order, error)
	ConfirmOrder(ctx context.Context, orderID, userID int64, completedAt time.Time, event OutboxEvent) (Order, error)
	CloseExpiredOrder(ctx context.Context, orderID, userID int64, expireBefore time.Time, closedAt time.Time) (Order, bool, error)
	CloseExpiredOrders(ctx context.Context, expireBefore time.Time, limit int, closedAt time.Time) ([]Order, error)
	AdminUpdateOrderStatus(ctx context.Context, orderID int64, nextStatus OrderStatus, operatorID string, fulfillment OrderFulfillment, note string, changedAt time.Time, event OutboxEvent) (Order, error)
	ListOrderStatusLogs(ctx context.Context, orderID int64) ([]OrderStatusLog, error)
	ListOrderPayments(ctx context.Context, orderID int64) ([]Payment, error)
	ListCartItems(ctx context.Context, userID int64) ([]CartItem, int64, error)
	SetCartItem(ctx context.Context, userID int64, productID int64, quantity int32, updatedAt time.Time) error
	RemoveCartItem(ctx context.Context, userID int64, productID int64) (bool, error)
	ClearCart(ctx context.Context, userID int64) (int64, error)
	ListProductFavorites(ctx context.Context, userID int64, limit, offset int) ([]ProductFavorite, int64, error)
	IsProductFavorite(ctx context.Context, userID int64, productID int64) (bool, error)
	AddProductFavorite(ctx context.Context, userID int64, productID int64, createdAt time.Time) (bool, error)
	RemoveProductFavorite(ctx context.Context, userID int64, productID int64) (bool, error)
	ListAddresses(ctx context.Context, userID int64, limit, offset int) ([]Address, int64, error)
	CreateAddress(ctx context.Context, address Address) (Address, error)
	UpdateAddress(ctx context.Context, address Address) (Address, error)
	DeleteAddress(ctx context.Context, userID, addressID int64) (bool, error)
	SetDefaultAddress(ctx context.Context, userID, addressID int64, updatedAt time.Time) (Address, error)
	CreateRefundRequest(ctx context.Context, request RefundRequest) (RefundRequest, bool, error)
	GetRefundRequest(ctx context.Context, refundID int64) (RefundRequest, error)
	ListRefundRequests(ctx context.Context, query RefundListQuery) ([]RefundRequest, int64, error)
	AdminListRefundRequests(ctx context.Context, query RefundListQuery) ([]RefundRequest, int64, error)
	StartRefundApproval(ctx context.Context, refundID int64, operatorID, adminNote string, restoreStock bool, reviewedAt time.Time) (RefundRequest, error)
	CompleteRefundApproval(ctx context.Context, refundID int64, reviewedAt time.Time, event OutboxEvent) (RefundRequest, error)
	RejectRefundRequest(ctx context.Context, refundID int64, operatorID, adminNote string, reviewedAt time.Time, event OutboxEvent) (RefundRequest, error)
	AdminMallOverview(ctx context.Context, lowStockThreshold int64) (MallOverview, error)
	AdminListOutboxRequeueAudits(ctx context.Context, query OutboxRequeueAuditListQuery) ([]OutboxRequeueAudit, int64, error)
	CountPendingOutboxEvents(ctx context.Context) (int, error)
	RequeueOutboxEvents(ctx context.Context, statuses []string, limit int, operatorID string, requeuedAt time.Time) (OutboxRequeueResult, error)
	ClaimPendingOutboxEvents(ctx context.Context, owner string, limit int, leaseDuration time.Duration) ([]OutboxEvent, error)
	MarkOutboxEventPublished(ctx context.Context, eventID string, owner string) error
	MarkOutboxEventFailed(ctx context.Context, eventID string, owner string, message string, nextAttemptAt time.Time) error
	MarkOutboxEventDeadLetter(ctx context.Context, eventID string, owner string, message string) error
}

type OutboxPublisher interface {
	PublishOutboxEvent(ctx context.Context, event OutboxEvent) error
}

func NormalizeRequired(value string, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New(field + " is required")
	}
	return trimmed, nil
}

func NormalizeListLimit(value int) int {
	if value <= 0 {
		return DefaultListLimit
	}
	if value > MaxListLimit {
		return MaxListLimit
	}
	return value
}

func NormalizeOffset(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func NormalizeProductStatus(value ProductStatus) ProductStatus {
	switch value {
	case ProductStatusDraft:
		return ProductStatusDraft
	case ProductStatusArchived:
		return ProductStatusArchived
	default:
		return ProductStatusActive
	}
}

func NormalizeProductCategoryStatus(value ProductCategoryStatus) ProductCategoryStatus {
	switch value {
	case ProductCategoryStatusDraft, ProductCategoryStatusActive, ProductCategoryStatusArchived:
		return value
	default:
		return ""
	}
}

func NormalizeProductReviewStatus(value ProductReviewStatus) ProductReviewStatus {
	switch value {
	case ProductReviewStatusPending, ProductReviewStatusPublished, ProductReviewStatusHidden:
		return value
	default:
		return ""
	}
}

func NormalizeOrderStatus(value OrderStatus) OrderStatus {
	switch value {
	case OrderStatusPendingPayment, OrderStatusPaying, OrderStatusPaid, OrderStatusCanceled, OrderStatusShipped, OrderStatusCompleted, OrderStatusClosed, OrderStatusRefunded:
		return value
	default:
		return ""
	}
}

func NormalizeRefundStatus(value RefundStatus) RefundStatus {
	switch value {
	case RefundStatusRequested, RefundStatusProcessing, RefundStatusApproved, RefundStatusRejected:
		return value
	default:
		return ""
	}
}

func NormalizeCouponStatus(value CouponStatus) CouponStatus {
	switch value {
	case CouponStatusDraft, CouponStatusActive, CouponStatusArchived:
		return value
	default:
		return ""
	}
}

func NormalizeCouponUsageStatus(value CouponUsageStatus) CouponUsageStatus {
	switch value {
	case CouponUsageStatusClaimed, CouponUsageStatusReserved, CouponUsageStatusUsed, CouponUsageStatusReleased:
		return value
	default:
		return ""
	}
}
