package grpc

import (
	"context"
	"errors"
	"time"

	pb "mall-service/api/proto/mallpb"
	app "mall-service/internal/application/mall"
	domain "mall-service/internal/domain/mall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedMallServiceServer
	service *app.Service
}

func NewHandler(service *app.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HealthCheck(context.Context, *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Status: "SERVING", Version: "local"}, nil
}

func (h *Handler) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	items, total, err := h.service.ListProducts(ctx, app.ListProductsCommand{
		Limit:    int(req.GetLimit()),
		Offset:   int(req.GetOffset()),
		Keyword:  req.GetKeyword(),
		Category: req.GetCategory(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductsResponse{Items: productsToPB(items), Total: total}, nil
}

func (h *Handler) ListProductCategories(ctx context.Context, req *pb.ListProductCategoriesRequest) (*pb.ListProductCategoriesResponse, error) {
	items, total, err := h.service.ListProductCategories(ctx, app.ListProductCategoriesCommand{
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductCategoriesResponse{Items: productCategoriesToPB(items), Total: total}, nil
}

func (h *Handler) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	product, err := h.service.GetProduct(ctx, req.GetId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.GetProductResponse{Product: productToPB(product)}, nil
}

func (h *Handler) ListProductReviews(ctx context.Context, req *pb.ListProductReviewsRequest) (*pb.ListProductReviewsResponse, error) {
	items, total, err := h.service.ListProductReviews(ctx, app.ListProductReviewsCommand{
		ProductID: req.GetProductId(),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductReviewsResponse{Items: productReviewsToPB(items), Total: total}, nil
}

func (h *Handler) ListUserProductReviews(ctx context.Context, req *pb.ListUserProductReviewsRequest) (*pb.ListProductReviewsResponse, error) {
	items, total, err := h.service.ListUserProductReviews(ctx, app.ListUserProductReviewsCommand{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
		Status:    productReviewStatusFromPB(req.GetStatus()),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductReviewsResponse{Items: productReviewsToPB(items), Total: total}, nil
}

func (h *Handler) CreateProductReview(ctx context.Context, req *pb.CreateProductReviewRequest) (*pb.ProductReviewResponse, error) {
	review, err := h.service.CreateProductReview(ctx, app.CreateProductReviewCommand{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
		OrderID:   req.GetOrderId(),
		Rating:    req.GetRating(),
		Content:   req.GetContent(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductReviewResponse{Review: productReviewToPB(review)}, nil
}

func (h *Handler) ListProductFavorites(ctx context.Context, req *pb.ListProductFavoritesRequest) (*pb.ListProductFavoritesResponse, error) {
	items, total, err := h.service.ListProductFavorites(ctx, app.ListProductFavoritesCommand{
		UserID: req.GetUserId(),
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductFavoritesResponse{Items: productFavoritesToPB(items), Total: total}, nil
}

func (h *Handler) IsProductFavorite(ctx context.Context, req *pb.ProductFavoriteStateRequest) (*pb.ProductFavoriteStateResponse, error) {
	favorited, err := h.service.IsProductFavorite(ctx, app.ProductFavoriteCommand{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductFavoriteStateResponse{Favorited: favorited}, nil
}

func (h *Handler) AddProductFavorite(ctx context.Context, req *pb.ProductFavoriteRequest) (*pb.ProductFavoriteStateResponse, error) {
	_, err := h.service.AddProductFavorite(ctx, app.ProductFavoriteCommand{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductFavoriteStateResponse{Favorited: true}, nil
}

func (h *Handler) RemoveProductFavorite(ctx context.Context, req *pb.ProductFavoriteRequest) (*pb.ProductFavoriteStateResponse, error) {
	_, err := h.service.RemoveProductFavorite(ctx, app.ProductFavoriteCommand{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductFavoriteStateResponse{Favorited: false}, nil
}

func (h *Handler) ListCoupons(ctx context.Context, req *pb.ListCouponsRequest) (*pb.ListCouponsResponse, error) {
	items, total, err := h.service.ListCoupons(ctx, app.ListCouponsCommand{
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListCouponsResponse{Items: couponsToPB(items), Total: total}, nil
}

func (h *Handler) ClaimCoupon(ctx context.Context, req *pb.ClaimCouponRequest) (*pb.CouponUsageResponse, error) {
	usage, duplicate, err := h.service.ClaimCoupon(ctx, app.ClaimCouponCommand{
		UserID:   req.GetUserId(),
		CouponID: req.GetCouponId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CouponUsageResponse{Usage: couponUsageToPB(usage), Duplicate: duplicate}, nil
}

func (h *Handler) ListUserCouponUsages(ctx context.Context, req *pb.ListUserCouponUsagesRequest) (*pb.ListCouponUsagesResponse, error) {
	items, total, err := h.service.ListUserCouponUsages(ctx, app.ListUserCouponUsagesCommand{
		UserID: req.GetUserId(),
		Status: couponUsageStatusFromPB(req.GetStatus()),
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListCouponUsagesResponse{Items: couponUsagesToPB(items), Total: total}, nil
}

func (h *Handler) AdminListProducts(ctx context.Context, req *pb.AdminListProductsRequest) (*pb.ListProductsResponse, error) {
	items, total, err := h.service.AdminListProducts(ctx, app.AdminListProductsCommand{
		Limit:    int(req.GetLimit()),
		Offset:   int(req.GetOffset()),
		Keyword:  req.GetKeyword(),
		Category: req.GetCategory(),
		Status:   productStatusFromPB(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductsResponse{Items: productsToPB(items), Total: total}, nil
}

func (h *Handler) AdminListProductCategories(ctx context.Context, req *pb.AdminListProductCategoriesRequest) (*pb.ListProductCategoriesResponse, error) {
	items, total, err := h.service.AdminListProductCategories(ctx, app.AdminListProductCategoriesCommand{
		Limit:   int(req.GetLimit()),
		Offset:  int(req.GetOffset()),
		Keyword: req.GetKeyword(),
		Status:  productCategoryStatusFromPB(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductCategoriesResponse{Items: productCategoriesToPB(items), Total: total}, nil
}

func (h *Handler) AdminListProductReviews(ctx context.Context, req *pb.AdminListProductReviewsRequest) (*pb.ListProductReviewsResponse, error) {
	items, total, err := h.service.AdminListProductReviews(ctx, app.AdminListProductReviewsCommand{
		ProductID: req.GetProductId(),
		UserID:    req.GetUserId(),
		Status:    productReviewStatusFromPB(req.GetStatus()),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductReviewsResponse{Items: productReviewsToPB(items), Total: total}, nil
}

func (h *Handler) AdminUpdateProductReviewStatus(ctx context.Context, req *pb.AdminUpdateProductReviewStatusRequest) (*pb.ProductReviewResponse, error) {
	review, err := h.service.AdminUpdateProductReviewStatus(ctx, app.AdminUpdateProductReviewStatusCommand{
		ID:     req.GetId(),
		Status: productReviewStatusFromPB(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductReviewResponse{Review: productReviewToPB(review)}, nil
}

func (h *Handler) AdminListCoupons(ctx context.Context, req *pb.AdminListCouponsRequest) (*pb.ListCouponsResponse, error) {
	items, total, err := h.service.AdminListCoupons(ctx, app.AdminListCouponsCommand{
		Limit:   int(req.GetLimit()),
		Offset:  int(req.GetOffset()),
		Keyword: req.GetKeyword(),
		Status:  couponStatusFromPB(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListCouponsResponse{Items: couponsToPB(items), Total: total}, nil
}

func (h *Handler) AdminListCouponUsages(ctx context.Context, req *pb.AdminListCouponUsagesRequest) (*pb.ListCouponUsagesResponse, error) {
	items, total, err := h.service.AdminListCouponUsages(ctx, app.AdminListCouponUsagesCommand{
		CouponID: req.GetCouponId(),
		UserID:   req.GetUserId(),
		Status:   couponUsageStatusFromPB(req.GetStatus()),
		Limit:    int(req.GetLimit()),
		Offset:   int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListCouponUsagesResponse{Items: couponUsagesToPB(items), Total: total}, nil
}

func (h *Handler) AdminCreateCoupon(ctx context.Context, req *pb.AdminSaveCouponRequest) (*pb.CouponResponse, error) {
	coupon, err := h.service.AdminCreateCoupon(ctx, saveCouponCommandFromPB(req))
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CouponResponse{Coupon: couponToPB(coupon)}, nil
}

func (h *Handler) AdminUpdateCoupon(ctx context.Context, req *pb.AdminSaveCouponRequest) (*pb.CouponResponse, error) {
	coupon, err := h.service.AdminUpdateCoupon(ctx, saveCouponCommandFromPB(req))
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CouponResponse{Coupon: couponToPB(coupon)}, nil
}

func (h *Handler) AdminCreateProductCategory(ctx context.Context, req *pb.AdminSaveProductCategoryRequest) (*pb.ProductCategoryResponse, error) {
	category, err := h.service.AdminCreateProductCategory(ctx, saveProductCategoryCommandFromPB(req))
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductCategoryResponse{Category: productCategoryToPB(category)}, nil
}

func (h *Handler) AdminUpdateProductCategory(ctx context.Context, req *pb.AdminSaveProductCategoryRequest) (*pb.ProductCategoryResponse, error) {
	category, err := h.service.AdminUpdateProductCategory(ctx, saveProductCategoryCommandFromPB(req))
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductCategoryResponse{Category: productCategoryToPB(category)}, nil
}

func (h *Handler) AdminMallOverview(ctx context.Context, req *pb.AdminMallOverviewRequest) (*pb.AdminMallOverviewResponse, error) {
	overview, err := h.service.AdminMallOverview(ctx, app.AdminMallOverviewCommand{
		LowStockThreshold: req.GetLowStockThreshold(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.AdminMallOverviewResponse{Overview: mallOverviewToPB(overview)}, nil
}

func (h *Handler) AdminCreateProduct(ctx context.Context, req *pb.AdminCreateProductRequest) (*pb.ProductResponse, error) {
	product, err := h.service.CreateProduct(ctx, app.CreateProductCommand{
		SKU:          req.GetSku(),
		Title:        req.GetTitle(),
		Description:  req.GetDescription(),
		Category:     req.GetCategory(),
		CoverURL:     req.GetCoverUrl(),
		GrantType:    req.GetGrantType(),
		GrantKey:     req.GetGrantKey(),
		PriceCredits: req.GetPriceCredits(),
		Stock:        req.GetStock(),
		Status:       productStatusFromPB(req.GetStatus()),
		Sort:         req.GetSort(),
		OperatorID:   req.GetOperatorId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductResponse{Product: productToPB(product)}, nil
}

func (h *Handler) AdminUpdateProduct(ctx context.Context, req *pb.AdminUpdateProductRequest) (*pb.ProductResponse, error) {
	product, err := h.service.UpdateProduct(ctx, app.UpdateProductCommand{
		ID:           req.GetId(),
		SKU:          req.GetSku(),
		Title:        req.GetTitle(),
		Description:  req.GetDescription(),
		Category:     req.GetCategory(),
		CoverURL:     req.GetCoverUrl(),
		GrantType:    req.GetGrantType(),
		GrantKey:     req.GetGrantKey(),
		PriceCredits: req.GetPriceCredits(),
		Stock:        req.GetStock(),
		Status:       productStatusFromPB(req.GetStatus()),
		Sort:         req.GetSort(),
		OperatorID:   req.GetOperatorId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ProductResponse{Product: productToPB(product)}, nil
}

func (h *Handler) AdminListProductStockLogs(ctx context.Context, req *pb.AdminListProductStockLogsRequest) (*pb.ListProductStockLogsResponse, error) {
	items, total, err := h.service.AdminListProductStockLogs(ctx, app.AdminListProductStockLogsCommand{
		ProductID: req.GetProductId(),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
		Reason:    req.GetReason(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListProductStockLogsResponse{Items: productStockLogsToPB(items), Total: total}, nil
}

func (h *Handler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	items := make([]domain.CreateOrderItem, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		items = append(items, domain.CreateOrderItem{ProductID: item.GetProductId(), Quantity: item.GetQuantity()})
	}
	result, err := h.service.CreateOrder(ctx, app.CreateOrderCommand{
		IdempotencyKey: req.GetIdempotencyKey(),
		UserID:         req.GetUserId(),
		Items:          items,
		CouponCode:     req.GetCouponCode(),
		Receiver:       req.GetReceiver(),
		Phone:          req.GetPhone(),
		Address:        req.GetAddress(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CreateOrderResponse{Order: orderToPB(result.Order), Duplicate: result.Duplicate}, nil
}

func (h *Handler) CheckoutCart(ctx context.Context, req *pb.CheckoutCartRequest) (*pb.CreateOrderResponse, error) {
	result, err := h.service.CheckoutCart(ctx, app.CheckoutCartCommand{
		IdempotencyKey: req.GetIdempotencyKey(),
		UserID:         req.GetUserId(),
		CouponCode:     req.GetCouponCode(),
		Receiver:       req.GetReceiver(),
		Phone:          req.GetPhone(),
		Address:        req.GetAddress(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CreateOrderResponse{Order: orderToPB(result.Order), Duplicate: result.Duplicate}, nil
}

func (h *Handler) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	var (
		order domain.Order
		err   error
	)
	if req.GetUserId() > 0 {
		order, err = h.service.GetUserOrder(ctx, req.GetId(), req.GetUserId())
	} else {
		order, err = h.service.GetOrder(ctx, req.GetId())
	}
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.GetOrderResponse{Order: orderToPB(order)}, nil
}

func (h *Handler) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	items, total, err := h.service.ListOrders(ctx, app.ListOrdersCommand{
		UserID: req.GetUserId(),
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
		Status: orderStatusFromPB(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListOrdersResponse{Items: ordersToPB(items), Total: total}, nil
}

func (h *Handler) ListUserDigitalEntitlements(ctx context.Context, req *pb.ListUserDigitalEntitlementsRequest) (*pb.ListDigitalEntitlementsResponse, error) {
	items, total, err := h.service.ListDigitalEntitlements(ctx, app.ListDigitalEntitlementsCommand{
		UserID:    req.GetUserId(),
		Status:    req.GetStatus(),
		GrantType: req.GetGrantType(),
		GrantKey:  req.GetGrantKey(),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListDigitalEntitlementsResponse{Items: digitalEntitlementsToPB(items), Total: total}, nil
}

func (h *Handler) AdminListDigitalEntitlements(ctx context.Context, req *pb.AdminListDigitalEntitlementsRequest) (*pb.ListDigitalEntitlementsResponse, error) {
	items, total, err := h.service.AdminListDigitalEntitlements(ctx, app.ListDigitalEntitlementsCommand{
		UserID:    req.GetUserId(),
		Status:    req.GetStatus(),
		GrantType: req.GetGrantType(),
		GrantKey:  req.GetGrantKey(),
		Keyword:   req.GetKeyword(),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListDigitalEntitlementsResponse{Items: digitalEntitlementsToPB(items), Total: total}, nil
}

func (h *Handler) AdminRevokeDigitalEntitlement(ctx context.Context, req *pb.AdminRevokeDigitalEntitlementRequest) (*pb.DigitalEntitlementResponse, error) {
	entitlement, err := h.service.AdminRevokeDigitalEntitlement(ctx, app.AdminRevokeDigitalEntitlementCommand{
		ID:         req.GetId(),
		OperatorID: req.GetOperatorId(),
		Reason:     req.GetReason(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.DigitalEntitlementResponse{Entitlement: digitalEntitlementToPB(entitlement)}, nil
}

func (h *Handler) ListReviewableOrders(ctx context.Context, req *pb.ListReviewableOrdersRequest) (*pb.ListOrdersResponse, error) {
	items, total, err := h.service.ListReviewableOrders(ctx, app.ListReviewableOrdersCommand{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListOrdersResponse{Items: ordersToPB(items), Total: total}, nil
}

func (h *Handler) AdminListOrders(ctx context.Context, req *pb.AdminListOrdersRequest) (*pb.ListOrdersResponse, error) {
	items, total, err := h.service.AdminListOrders(ctx, app.AdminListOrdersCommand{
		UserID:  req.GetUserId(),
		Limit:   int(req.GetLimit()),
		Offset:  int(req.GetOffset()),
		Keyword: req.GetKeyword(),
		Status:  orderStatusFromPB(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListOrdersResponse{Items: ordersToPB(items), Total: total}, nil
}

func (h *Handler) AdminListOrderPayments(ctx context.Context, req *pb.AdminListOrderPaymentsRequest) (*pb.ListOrderPaymentsResponse, error) {
	items, total, err := h.service.AdminListOrderPayments(ctx, app.AdminListOrderPaymentsCommand{
		UserID:  req.GetUserId(),
		Limit:   int(req.GetLimit()),
		Offset:  int(req.GetOffset()),
		Keyword: req.GetKeyword(),
		Status:  orderStatusFromPB(pb.OrderStatus(req.GetStatus())),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListOrderPaymentsResponse{Items: paymentsToPB(items), Total: total}, nil
}

func (h *Handler) PayOrder(ctx context.Context, req *pb.PayOrderRequest) (*pb.PayOrderResponse, error) {
	order, err := h.service.PayOrder(ctx, app.PayOrderCommand{
		OrderID:        req.GetOrderId(),
		UserID:         req.GetUserId(),
		PaymentMethod:  req.GetPaymentMethod(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.PayOrderResponse{Order: orderToPB(order)}, nil
}

func (h *Handler) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	order, err := h.service.CancelOrder(ctx, app.CancelOrderCommand{OrderID: req.GetOrderId(), UserID: req.GetUserId()})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CancelOrderResponse{Order: orderToPB(order)}, nil
}

func (h *Handler) ConfirmOrder(ctx context.Context, req *pb.ConfirmOrderRequest) (*pb.OrderResponse, error) {
	order, err := h.service.ConfirmOrder(ctx, app.ConfirmOrderCommand{OrderID: req.GetOrderId(), UserID: req.GetUserId()})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.OrderResponse{Order: orderToPB(order)}, nil
}

func (h *Handler) CloseExpiredOrders(ctx context.Context, req *pb.CloseExpiredOrdersRequest) (*pb.CloseExpiredOrdersResponse, error) {
	orders, err := h.service.CloseExpiredOrders(ctx, app.CloseExpiredOrdersCommand{
		ExpireAfter: time.Duration(req.GetExpireAfterSeconds()) * time.Second,
		Limit:       int(req.GetLimit()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CloseExpiredOrdersResponse{Items: ordersToPB(orders), Total: int64(len(orders))}, nil
}

func (h *Handler) RecoverStalePayingOrders(ctx context.Context, req *pb.RecoverStalePayingOrdersRequest) (*pb.RecoverStalePayingOrdersResponse, error) {
	result, err := h.service.RecoverStalePayingOrders(ctx, app.RecoverStalePayingOrdersCommand{
		StaleAfter: time.Duration(req.GetStaleAfterSeconds()) * time.Second,
		Limit:      int(req.GetLimit()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.RecoverStalePayingOrdersResponse{
		Recovered: int64(result.Recovered),
		Failed:    int64(result.Failed),
	}, nil
}

func (h *Handler) AdminRequeueOutboxEvents(ctx context.Context, req *pb.AdminRequeueOutboxEventsRequest) (*pb.AdminRequeueOutboxEventsResponse, error) {
	result, err := h.service.AdminRequeueOutboxEvents(ctx, app.AdminRequeueOutboxEventsCommand{
		Statuses:   req.GetStatuses(),
		Limit:      int(req.GetLimit()),
		OperatorID: req.GetOperatorId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.AdminRequeueOutboxEventsResponse{Requeued: result.Requeued, EventIds: result.EventIDs}, nil
}

func (h *Handler) AdminListOutboxRequeueAudits(ctx context.Context, req *pb.AdminListOutboxRequeueAuditsRequest) (*pb.AdminListOutboxRequeueAuditsResponse, error) {
	result, err := h.service.AdminListOutboxRequeueAudits(ctx, app.AdminListOutboxRequeueAuditsCommand{
		Limit:         int(req.GetLimit()),
		Offset:        int(req.GetOffset()),
		EventID:       req.GetEventId(),
		AggregateType: req.GetAggregateType(),
		AggregateID:   req.GetAggregateId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.AdminListOutboxRequeueAuditsResponse{
		Items: outboxRequeueAuditsToPB(result.Items),
		Total: result.Total,
	}, nil
}

func (h *Handler) AdminUpdateOrderStatus(ctx context.Context, req *pb.AdminUpdateOrderStatusRequest) (*pb.OrderResponse, error) {
	order, err := h.service.AdminUpdateOrderStatus(ctx, app.AdminUpdateOrderStatusCommand{
		OrderID:         req.GetOrderId(),
		Status:          orderStatusFromPB(req.GetStatus()),
		OperatorID:      req.GetOperatorId(),
		ShippingCarrier: req.GetShippingCarrier(),
		TrackingNo:      req.GetTrackingNo(),
		Note:            req.GetNote(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.OrderResponse{Order: orderToPB(order)}, nil
}

func (h *Handler) ListOrderStatusLogs(ctx context.Context, req *pb.ListOrderStatusLogsRequest) (*pb.ListOrderStatusLogsResponse, error) {
	var (
		items []domain.OrderStatusLog
		err   error
	)
	if req.GetUserId() > 0 {
		items, err = h.service.ListUserOrderStatusLogs(ctx, req.GetOrderId(), req.GetUserId())
	} else {
		items, err = h.service.ListOrderStatusLogs(ctx, req.GetOrderId())
	}
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListOrderStatusLogsResponse{Items: orderStatusLogsToPB(items)}, nil
}

func (h *Handler) ListOrderPayments(ctx context.Context, req *pb.ListOrderPaymentsRequest) (*pb.ListOrderPaymentsResponse, error) {
	var (
		items []domain.Payment
		err   error
	)
	if req.GetUserId() > 0 {
		items, err = h.service.ListUserOrderPayments(ctx, req.GetOrderId(), req.GetUserId())
	} else {
		items, err = h.service.ListOrderPayments(ctx, req.GetOrderId())
	}
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListOrderPaymentsResponse{Items: paymentsToPB(items), Total: int64(len(items))}, nil
}

func (h *Handler) ListCartItems(ctx context.Context, req *pb.ListCartItemsRequest) (*pb.CartResponse, error) {
	items, total, err := h.service.ListCartItems(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CartResponse{Items: cartItemsToPB(items), Total: total}, nil
}

func (h *Handler) SetCartItem(ctx context.Context, req *pb.SetCartItemRequest) (*pb.CartResponse, error) {
	items, total, err := h.service.SetCartItem(ctx, app.SetCartItemCommand{
		UserID:    req.GetUserId(),
		ProductID: req.GetProductId(),
		Quantity:  req.GetQuantity(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CartResponse{Items: cartItemsToPB(items), Total: total}, nil
}

func (h *Handler) RemoveCartItem(ctx context.Context, req *pb.RemoveCartItemRequest) (*pb.CartResponse, error) {
	items, total, err := h.service.RemoveCartItem(ctx, req.GetUserId(), req.GetProductId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CartResponse{Items: cartItemsToPB(items), Total: total}, nil
}

func (h *Handler) ClearCart(ctx context.Context, req *pb.ClearCartRequest) (*pb.CartResponse, error) {
	items, total, err := h.service.ClearCart(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.CartResponse{Items: cartItemsToPB(items), Total: total}, nil
}

func (h *Handler) ListAddresses(ctx context.Context, req *pb.ListAddressesRequest) (*pb.ListAddressesResponse, error) {
	items, total, err := h.service.ListAddresses(ctx, app.ListAddressesCommand{
		UserID: req.GetUserId(),
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListAddressesResponse{Items: addressesToPB(items), Total: total}, nil
}

func (h *Handler) CreateAddress(ctx context.Context, req *pb.CreateAddressRequest) (*pb.AddressResponse, error) {
	address, err := h.service.CreateAddress(ctx, app.SaveAddressCommand{
		UserID:     req.GetUserId(),
		Receiver:   req.GetReceiver(),
		Phone:      req.GetPhone(),
		Province:   req.GetProvince(),
		City:       req.GetCity(),
		District:   req.GetDistrict(),
		Detail:     req.GetDetail(),
		PostalCode: req.GetPostalCode(),
		IsDefault:  req.GetIsDefault(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.AddressResponse{Address: addressToPB(address)}, nil
}

func (h *Handler) UpdateAddress(ctx context.Context, req *pb.UpdateAddressRequest) (*pb.AddressResponse, error) {
	address, err := h.service.UpdateAddress(ctx, app.SaveAddressCommand{
		ID:         req.GetId(),
		UserID:     req.GetUserId(),
		Receiver:   req.GetReceiver(),
		Phone:      req.GetPhone(),
		Province:   req.GetProvince(),
		City:       req.GetCity(),
		District:   req.GetDistrict(),
		Detail:     req.GetDetail(),
		PostalCode: req.GetPostalCode(),
		IsDefault:  req.GetIsDefault(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.AddressResponse{Address: addressToPB(address)}, nil
}

func (h *Handler) DeleteAddress(ctx context.Context, req *pb.DeleteAddressRequest) (*pb.DeleteAddressResponse, error) {
	deleted, err := h.service.DeleteAddress(ctx, req.GetUserId(), req.GetId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.DeleteAddressResponse{Deleted: deleted}, nil
}

func (h *Handler) SetDefaultAddress(ctx context.Context, req *pb.SetDefaultAddressRequest) (*pb.AddressResponse, error) {
	address, err := h.service.SetDefaultAddress(ctx, req.GetUserId(), req.GetId())
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.AddressResponse{Address: addressToPB(address)}, nil
}

func (h *Handler) CreateRefundRequest(ctx context.Context, req *pb.CreateRefundRequestRequest) (*pb.RefundRequestResponse, error) {
	refund, duplicate, err := h.service.CreateRefundRequest(ctx, app.CreateRefundRequestCommand{
		OrderID: req.GetOrderId(),
		UserID:  req.GetUserId(),
		Reason:  req.GetReason(),
		Note:    req.GetNote(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.RefundRequestResponse{Refund: refundRequestToPB(refund), Duplicate: duplicate}, nil
}

func (h *Handler) CancelRefundRequest(ctx context.Context, req *pb.CancelRefundRequestRequest) (*pb.RefundRequestResponse, error) {
	refund, duplicate, err := h.service.CancelRefundRequest(ctx, app.CancelRefundRequestCommand{
		RefundID: req.GetRefundId(),
		UserID:   req.GetUserId(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.RefundRequestResponse{Refund: refundRequestToPB(refund), Duplicate: duplicate}, nil
}

func (h *Handler) ListRefundRequests(ctx context.Context, req *pb.ListRefundRequestsRequest) (*pb.ListRefundRequestsResponse, error) {
	items, total, err := h.service.ListRefundRequests(ctx, app.ListRefundRequestsCommand{
		UserID: req.GetUserId(),
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
		Status: refundStatusFromPB(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListRefundRequestsResponse{Items: refundRequestsToPB(items), Total: total}, nil
}

func (h *Handler) AdminListRefundRequests(ctx context.Context, req *pb.AdminListRefundRequestsRequest) (*pb.ListRefundRequestsResponse, error) {
	items, total, err := h.service.AdminListRefundRequests(ctx, app.AdminListRefundRequestsCommand{
		UserID:  req.GetUserId(),
		Limit:   int(req.GetLimit()),
		Offset:  int(req.GetOffset()),
		Status:  refundStatusFromPB(req.GetStatus()),
		Keyword: req.GetKeyword(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.ListRefundRequestsResponse{Items: refundRequestsToPB(items), Total: total}, nil
}

func (h *Handler) AdminReviewRefundRequest(ctx context.Context, req *pb.AdminReviewRefundRequestRequest) (*pb.RefundRequestResponse, error) {
	refund, err := h.service.AdminReviewRefundRequest(ctx, app.AdminReviewRefundRequestCommand{
		RefundID:     req.GetRefundId(),
		Approved:     req.GetApproved(),
		OperatorID:   req.GetOperatorId(),
		AdminNote:    req.GetAdminNote(),
		RestoreStock: req.GetRestoreStock(),
	})
	if err != nil {
		return nil, toStatusError(err)
	}
	return &pb.RefundRequestResponse{Refund: refundRequestToPB(refund)}, nil
}

func saveCouponCommandFromPB(req *pb.AdminSaveCouponRequest) app.SaveCouponCommand {
	return app.SaveCouponCommand{
		ID:              req.GetId(),
		Code:            req.GetCode(),
		Name:            req.GetName(),
		Description:     req.GetDescription(),
		DiscountCredits: req.GetDiscountCredits(),
		MinOrderCredits: req.GetMinOrderCredits(),
		TotalQuota:      req.GetTotalQuota(),
		PerUserLimit:    req.GetPerUserLimit(),
		Status:          couponStatusFromPB(req.GetStatus()),
		StartsAt:        timeFromMillis(req.GetStartsAt()),
		EndsAt:          timeFromMillis(req.GetEndsAt()),
	}
}

func saveProductCategoryCommandFromPB(req *pb.AdminSaveProductCategoryRequest) app.SaveProductCategoryCommand {
	return app.SaveProductCategoryCommand{
		ID:          req.GetId(),
		Slug:        req.GetSlug(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Status:      productCategoryStatusFromPB(req.GetStatus()),
		Sort:        req.GetSort(),
	}
}

func timeFromMillis(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	t := time.UnixMilli(value).UTC()
	return &t
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, domain.ErrProductNotFound), errors.Is(err, domain.ErrOrderNotFound), errors.Is(err, domain.ErrRefundNotFound), errors.Is(err, domain.ErrAddressNotFound), errors.Is(err, domain.ErrCouponNotFound), errors.Is(err, domain.ErrProductCategoryNotFound), errors.Is(err, domain.ErrProductReviewNotFound), errors.Is(err, domain.ErrDigitalEntitlementNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrOrderOwnerMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrProductUnavailable), errors.Is(err, domain.ErrProductGrantLocked), errors.Is(err, domain.ErrProductFulfillmentLocked), errors.Is(err, domain.ErrProductCategoryUnavailable), errors.Is(err, domain.ErrProductCategoryLocked), errors.Is(err, domain.ErrProductCategorySlugLocked), errors.Is(err, domain.ErrInvalidOrderState), errors.Is(err, domain.ErrInsufficientStock), errors.Is(err, domain.ErrInsufficientCredits), errors.Is(err, domain.ErrUnsupportedPayment), errors.Is(err, domain.ErrCouponUnavailable), errors.Is(err, domain.ErrCouponTermsLocked), errors.Is(err, domain.ErrMembershipRefundUnavailable), errors.Is(err, domain.ErrPendingMembershipOrderExists), errors.Is(err, domain.ErrActiveThemeEntitlementExists), errors.Is(err, domain.ErrPendingThemeOrderExists), errors.Is(err, domain.ErrDuplicateThemeGrantInOrder), errors.Is(err, domain.ErrActiveBadgeEntitlementExists), errors.Is(err, domain.ErrPendingBadgeOrderExists), errors.Is(err, domain.ErrDuplicateBadgeGrantInOrder):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrDuplicateReference):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.InvalidArgument, err.Error())
	}
}
