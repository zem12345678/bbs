package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/mallpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetMallOrderRequiresOrderOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{order: &mallpb.Order{Id: 88, UserId: 42, OrderNo: "O-88"}}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodGet, "/api/v1/mall/orders/88", 88, 42)
	h.getMallOrder(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(88), mallClient.getOrderReq.GetId())

	var envelope struct {
		Data struct {
			Order struct {
				Id      int64  `json:"id"`
				OrderNo string `json:"order_no"`
			} `json:"order"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(88), envelope.Data.Order.Id)
	require.Equal(t, "O-88", envelope.Data.Order.OrderNo)
}

func TestGetMallOrderRejectsOtherUserOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{order: &mallpb.Order{Id: 88, UserId: 99}}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodGet, "/api/v1/mall/orders/88", 88, 42)
	h.getMallOrder(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
}

func TestListMallOrderLogsRequiresOrderOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{
		order: &mallpb.Order{Id: 88, UserId: 42},
		logs: []*mallpb.OrderStatusLog{
			{Id: 501, OrderId: 88, ToStatus: mallpb.OrderStatus_ORDER_STATUS_PAID, Reason: "paid"},
		},
	}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodGet, "/api/v1/mall/orders/88/logs", 88, 42)
	h.listMallOrderLogs(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.True(t, mallClient.logsCalled)
	require.Equal(t, int64(88), mallClient.logsReq.GetOrderId())

	var envelope struct {
		Data struct {
			Items []struct {
				Id     int64  `json:"id"`
				Reason string `json:"reason"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(501), envelope.Data.Items[0].Id)
	require.Equal(t, "paid", envelope.Data.Items[0].Reason)
}

func TestListMallOrderLogsRejectsOtherUserOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{order: &mallpb.Order{Id: 88, UserId: 99}}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodGet, "/api/v1/mall/orders/88/logs", 88, 42)
	h.listMallOrderLogs(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.False(t, mallClient.logsCalled)
}

func TestListMallOrderPaymentsRequiresOrderOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{
		order: &mallpb.Order{Id: 88, UserId: 42},
		payments: []*mallpb.Payment{
			{Id: 1001, OrderId: 88, UserId: 42, AmountCredits: 120, Status: mallpb.PaymentStatus_PAYMENT_STATUS_SUCCEEDED},
		},
	}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodGet, "/api/v1/mall/orders/88/payments", 88, 42)
	h.listMallOrderPayments(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.True(t, mallClient.paymentsCalled)
	require.Equal(t, int64(88), mallClient.paymentsReq.GetOrderId())

	var envelope struct {
		Data struct {
			Items []struct {
				Id            int64 `json:"id"`
				AmountCredits int64 `json:"amount_credits"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(1001), envelope.Data.Items[0].Id)
	require.Equal(t, int64(120), envelope.Data.Items[0].AmountCredits)
}

func TestListMallOrderPaymentsRejectsOtherUserOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{order: &mallpb.Order{Id: 88, UserId: 99}}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodGet, "/api/v1/mall/orders/88/payments", 88, 42)
	h.listMallOrderPayments(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.False(t, mallClient.paymentsCalled)
}

func TestListMallOrdersForwardsStatusFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodGet, "/api/v1/mall/orders?limit=30&offset=10&status=3", 0, 42)
	h.listMallOrders(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.listOrdersReq)
	require.Equal(t, int64(42), mallClient.listOrdersReq.GetUserId())
	require.Equal(t, int32(30), mallClient.listOrdersReq.GetLimit())
	require.Equal(t, int32(10), mallClient.listOrdersReq.GetOffset())
	require.Equal(t, mallpb.OrderStatus_ORDER_STATUS_PAID, mallClient.listOrdersReq.GetStatus())
}

func TestCreateMallOrderMapsFailedPreconditionMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{
		createOrderErr: status.Error(codes.FailedPrecondition, "商品库存不足"),
	}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderJSONContext(http.MethodPost, "/api/v1/mall/orders", 42, `{"items":[{"product_id":1001,"quantity":2}],"idempotency_key":"create-stock"}`)
	h.createMallOrder(c)

	require.Equal(t, http.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.createOrderReq)
	require.Equal(t, int64(42), mallClient.createOrderReq.GetUserId())
	require.Equal(t, int64(1001), mallClient.createOrderReq.GetItems()[0].GetProductId())

	var envelope struct {
		Message string         `json:"message"`
		Meta    map[string]any `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "商品库存不足", envelope.Message)
	require.Equal(t, codes.FailedPrecondition.String(), envelope.Meta["legacy_code"])
}

func TestPayMallOrderMapsFailedPreconditionMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{
		payOrderErr: status.Error(codes.FailedPrecondition, "积分不足"),
	}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderJSONContext(http.MethodPost, "/api/v1/mall/orders/88/pay", 42, `{"payment_method":"credits","idempotency_key":"pay-credits"}`)
	h.payMallOrder(c)

	require.Equal(t, http.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.payOrderReq)
	require.Equal(t, int64(88), mallClient.payOrderReq.GetOrderId())
	require.Equal(t, int64(42), mallClient.payOrderReq.GetUserId())

	var envelope struct {
		Message string         `json:"message"`
		Meta    map[string]any `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "积分不足", envelope.Message)
	require.Equal(t, codes.FailedPrecondition.String(), envelope.Meta["legacy_code"])
}

func TestConfirmMallOrderForwardsCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{confirmOrderResponse: &mallpb.Order{Id: 88, UserId: 42, Status: mallpb.OrderStatus_ORDER_STATUS_COMPLETED}}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderContext(http.MethodPost, "/api/v1/mall/orders/88/confirm", 88, 42)
	h.confirmMallOrder(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.True(t, mallClient.confirmOrderCalled)
	require.Equal(t, int64(88), mallClient.confirmOrderReq.GetOrderId())
	require.Equal(t, int64(42), mallClient.confirmOrderReq.GetUserId())
}

func TestCreateMallRefundRequestForwardsCurrentUserAndNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallOrderPaymentsClient{}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallOrderJSONContext(http.MethodPost, "/api/v1/mall/orders/88/refunds", 42, `{"reason":"quality","note":"包装破损影响使用"}`)
	h.createMallRefundRequest(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.True(t, mallClient.createRefundCalled)
	require.Equal(t, int64(88), mallClient.createRefundReq.GetOrderId())
	require.Equal(t, int64(42), mallClient.createRefundReq.GetUserId())
	require.Equal(t, "quality", mallClient.createRefundReq.GetReason())
	require.Equal(t, "包装破损影响使用", mallClient.createRefundReq.GetNote())
}

func newMallOrderContext(method string, rawURL string, _ int64, userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "88"}}
	c.Set("user_id", userID)
	c.Request = httptest.NewRequest(method, rawURL, nil)
	return c, recorder
}

func newMallOrderJSONContext(method string, rawURL string, userID int64, body string) (*gin.Context, *httptest.ResponseRecorder) {
	c, recorder := newMallOrderContext(method, rawURL, 88, userID)
	c.Request = httptest.NewRequest(method, rawURL, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

type fakeMallOrderPaymentsClient struct {
	mallpb.MallServiceClient
	order                *mallpb.Order
	logs                 []*mallpb.OrderStatusLog
	payments             []*mallpb.Payment
	getOrderReq          *mallpb.GetOrderRequest
	logsReq              *mallpb.ListOrderStatusLogsRequest
	logsCalled           bool
	paymentsReq          *mallpb.ListOrderPaymentsRequest
	paymentsCalled       bool
	listOrdersReq        *mallpb.ListOrdersRequest
	createOrderReq       *mallpb.CreateOrderRequest
	createOrderErr       error
	payOrderReq          *mallpb.PayOrderRequest
	payOrderErr          error
	confirmOrderReq      *mallpb.ConfirmOrderRequest
	confirmOrderCalled   bool
	confirmOrderResponse *mallpb.Order
	createRefundReq      *mallpb.CreateRefundRequestRequest
	createRefundCalled   bool
}

func (f *fakeMallOrderPaymentsClient) ListOrders(_ context.Context, req *mallpb.ListOrdersRequest, _ ...grpc.CallOption) (*mallpb.ListOrdersResponse, error) {
	f.listOrdersReq = req
	return &mallpb.ListOrdersResponse{Items: []*mallpb.Order{}, Total: 0}, nil
}

func (f *fakeMallOrderPaymentsClient) CreateOrder(_ context.Context, req *mallpb.CreateOrderRequest, _ ...grpc.CallOption) (*mallpb.CreateOrderResponse, error) {
	f.createOrderReq = req
	if f.createOrderErr != nil {
		return nil, f.createOrderErr
	}
	return &mallpb.CreateOrderResponse{Order: &mallpb.Order{Id: 88, UserId: req.GetUserId()}}, nil
}

func (f *fakeMallOrderPaymentsClient) GetOrder(_ context.Context, req *mallpb.GetOrderRequest, _ ...grpc.CallOption) (*mallpb.GetOrderResponse, error) {
	f.getOrderReq = req
	return &mallpb.GetOrderResponse{Order: f.order}, nil
}

func (f *fakeMallOrderPaymentsClient) ListOrderStatusLogs(_ context.Context, req *mallpb.ListOrderStatusLogsRequest, _ ...grpc.CallOption) (*mallpb.ListOrderStatusLogsResponse, error) {
	f.logsCalled = true
	f.logsReq = req
	return &mallpb.ListOrderStatusLogsResponse{Items: f.logs}, nil
}

func (f *fakeMallOrderPaymentsClient) ListOrderPayments(_ context.Context, req *mallpb.ListOrderPaymentsRequest, _ ...grpc.CallOption) (*mallpb.ListOrderPaymentsResponse, error) {
	f.paymentsCalled = true
	f.paymentsReq = req
	return &mallpb.ListOrderPaymentsResponse{Items: f.payments}, nil
}

func (f *fakeMallOrderPaymentsClient) PayOrder(_ context.Context, req *mallpb.PayOrderRequest, _ ...grpc.CallOption) (*mallpb.PayOrderResponse, error) {
	f.payOrderReq = req
	if f.payOrderErr != nil {
		return nil, f.payOrderErr
	}
	return &mallpb.PayOrderResponse{Order: &mallpb.Order{Id: req.GetOrderId(), UserId: req.GetUserId()}}, nil
}

func (f *fakeMallOrderPaymentsClient) CreateRefundRequest(_ context.Context, req *mallpb.CreateRefundRequestRequest, _ ...grpc.CallOption) (*mallpb.RefundRequestResponse, error) {
	f.createRefundCalled = true
	f.createRefundReq = req
	return &mallpb.RefundRequestResponse{
		Refund: &mallpb.RefundRequest{
			Id:       700,
			OrderId:  req.GetOrderId(),
			UserId:   req.GetUserId(),
			Status:   mallpb.RefundStatus_REFUND_STATUS_REQUESTED,
			Reason:   req.GetReason(),
			UserNote: req.GetNote(),
		},
	}, nil
}

func (f *fakeMallOrderPaymentsClient) ConfirmOrder(_ context.Context, req *mallpb.ConfirmOrderRequest, _ ...grpc.CallOption) (*mallpb.OrderResponse, error) {
	f.confirmOrderCalled = true
	f.confirmOrderReq = req
	order := f.confirmOrderResponse
	if order == nil {
		order = &mallpb.Order{Id: req.GetOrderId(), UserId: req.GetUserId(), Status: mallpb.OrderStatus_ORDER_STATUS_COMPLETED}
	}
	return &mallpb.OrderResponse{Order: order}, nil
}
