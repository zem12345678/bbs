package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListMallProductReviewsForwardsProductAndPaging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewContext(http.MethodGet, "/api/v1/mall/products/77/reviews?limit=7&offset=14", 77, 0)
	h.listMallProductReviews(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.listProductReviewsReq)
	require.Equal(t, int64(77), mallClient.listProductReviewsReq.GetProductId())
	require.Equal(t, int32(7), mallClient.listProductReviewsReq.GetLimit())
	require.Equal(t, int32(14), mallClient.listProductReviewsReq.GetOffset())

	var envelope struct {
		Data struct {
			Items []struct {
				Id      int64  `json:"id"`
				Content string `json:"content"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(501), envelope.Data.Items[0].Id)
	require.Equal(t, "public review", envelope.Data.Items[0].Content)
}

func TestCreateMallProductReviewForwardsCurrentUserAndPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	h := NewHandler(&clients.Clients{Mall: mallClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewJSONContext(http.MethodPost, "/api/v1/mall/products/77/reviews", 77, 42, `{"order_id":88,"rating":4,"content":"兑换体验不错"}`)
	h.createMallProductReview(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.createReviewReq)
	require.Equal(t, int64(42), mallClient.createReviewReq.GetUserId())
	require.Equal(t, int64(77), mallClient.createReviewReq.GetProductId())
	require.Equal(t, int64(88), mallClient.createReviewReq.GetOrderId())
	require.Equal(t, int32(4), mallClient.createReviewReq.GetRating())
	require.Equal(t, "兑换体验不错", mallClient.createReviewReq.GetContent())
}

func TestCreateMallProductReviewAcceptsQuotedInt64OrderID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	h := NewHandler(&clients.Clients{Mall: mallClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewJSONContext(http.MethodPost, "/api/v1/mall/products/77/reviews", 77, 42, `{"order_id":"339000000000000012","rating":4,"content":"大整数订单评价"}`)
	h.createMallProductReview(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.createReviewReq)
	require.Equal(t, int64(339000000000000012), mallClient.createReviewReq.GetOrderId())
}

func TestCreateMallProductReviewRejectsMutedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusMuted}}}
	h := NewHandler(&clients.Clients{Mall: mallClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewJSONContext(http.MethodPost, "/api/v1/mall/products/77/reviews", 77, 42, `{"order_id":88,"rating":4,"content":"禁言用户不能评价"}`)
	h.createMallProductReview(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "user_muted")
	require.Nil(t, mallClient.createReviewReq)
}

func TestCreateMallProductReviewRejectsUnverifiedUserWhenEmailGateEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	h := NewHandler(&clients.Clients{
		Mall:  mallClient,
		User:  userClient,
		Admin: fakeAuthSettingsAdminClient{items: []*adminpb.SettingInfo{authSetting("auth.email_verification.required", "true")}},
	}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewJSONContext(http.MethodPost, "/api/v1/mall/products/77/reviews", 77, 42, `{"order_id":88,"rating":4,"content":"未验证用户不能评价"}`)
	h.createMallProductReview(c)

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "email_not_verified")
	require.Nil(t, mallClient.createReviewReq)
}

func TestCreateMallProductReviewMapsDuplicateReference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{
		createReviewErr: status.Error(codes.AlreadyExists, "duplicate reference"),
	}
	userClient := &fakeUserClient{userResponse: &userpb.UserResponse{User: &userpb.UserInfo{Id: 42, Status: userStatusActive}}}
	h := NewHandler(&clients.Clients{Mall: mallClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewJSONContext(http.MethodPost, "/api/v1/mall/products/77/reviews", 77, 42, `{"order_id":88,"rating":4,"content":"重复评价"}`)
	h.createMallProductReview(c)

	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.createReviewReq)

	var envelope struct {
		Message string         `json:"message"`
		Meta    map[string]any `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "duplicate reference", envelope.Message)
	require.Equal(t, codes.AlreadyExists.String(), envelope.Meta["legacy_code"])
}

func TestListMyMallProductReviewsForwardsCurrentUserAndFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewContext(http.MethodGet, "/api/v1/mall/reviews?product_id=77&status=2&limit=9&offset=3", 0, 42)
	h.listMyMallProductReviews(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.listUserReviewsReq)
	require.Equal(t, int64(42), mallClient.listUserReviewsReq.GetUserId())
	require.Equal(t, int64(77), mallClient.listUserReviewsReq.GetProductId())
	require.Equal(t, mallpb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_PUBLISHED, mallClient.listUserReviewsReq.GetStatus())
	require.Equal(t, int32(9), mallClient.listUserReviewsReq.GetLimit())
	require.Equal(t, int32(3), mallClient.listUserReviewsReq.GetOffset())
}

func TestListMallReviewableOrdersForwardsCurrentUserAndProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallReviewClient{}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallReviewContext(http.MethodGet, "/api/v1/mall/products/77/reviewable-orders?limit=5&offset=10", 77, 42)
	h.listMallReviewableOrders(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.listReviewableOrdersReq)
	require.Equal(t, int64(42), mallClient.listReviewableOrdersReq.GetUserId())
	require.Equal(t, int64(77), mallClient.listReviewableOrdersReq.GetProductId())
	require.Equal(t, int32(5), mallClient.listReviewableOrdersReq.GetLimit())
	require.Equal(t, int32(10), mallClient.listReviewableOrdersReq.GetOffset())
}

func newMallReviewContext(method string, rawURL string, productID int64, userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	if productID > 0 {
		c.Params = gin.Params{{Key: "id", Value: "77"}}
	}
	if userID > 0 {
		c.Set("user_id", userID)
	}
	c.Request = httptest.NewRequest(method, rawURL, nil)
	return c, recorder
}

func newMallReviewJSONContext(method string, rawURL string, productID int64, userID int64, body string) (*gin.Context, *httptest.ResponseRecorder) {
	c, recorder := newMallReviewContext(method, rawURL, productID, userID)
	c.Request = httptest.NewRequest(method, rawURL, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

type fakeMallReviewClient struct {
	mallpb.MallServiceClient
	listProductReviewsReq   *mallpb.ListProductReviewsRequest
	createReviewReq         *mallpb.CreateProductReviewRequest
	createReviewErr         error
	listUserReviewsReq      *mallpb.ListUserProductReviewsRequest
	listReviewableOrdersReq *mallpb.ListReviewableOrdersRequest
}

func (f *fakeMallReviewClient) ListProductReviews(_ context.Context, req *mallpb.ListProductReviewsRequest, _ ...grpc.CallOption) (*mallpb.ListProductReviewsResponse, error) {
	f.listProductReviewsReq = req
	return &mallpb.ListProductReviewsResponse{
		Items: []*mallpb.ProductReview{
			{Id: 501, ProductId: req.GetProductId(), UserId: 42, Rating: 5, Content: "public review", Status: mallpb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_PUBLISHED},
		},
		Total: 1,
	}, nil
}

func (f *fakeMallReviewClient) CreateProductReview(_ context.Context, req *mallpb.CreateProductReviewRequest, _ ...grpc.CallOption) (*mallpb.ProductReviewResponse, error) {
	f.createReviewReq = req
	if f.createReviewErr != nil {
		return nil, f.createReviewErr
	}
	return &mallpb.ProductReviewResponse{
		Review: &mallpb.ProductReview{
			Id:        502,
			ProductId: req.GetProductId(),
			OrderId:   req.GetOrderId(),
			UserId:    req.GetUserId(),
			Rating:    req.GetRating(),
			Content:   req.GetContent(),
			Status:    mallpb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_PENDING,
		},
	}, nil
}

func (f *fakeMallReviewClient) ListUserProductReviews(_ context.Context, req *mallpb.ListUserProductReviewsRequest, _ ...grpc.CallOption) (*mallpb.ListProductReviewsResponse, error) {
	f.listUserReviewsReq = req
	return &mallpb.ListProductReviewsResponse{
		Items: []*mallpb.ProductReview{
			{Id: 503, ProductId: req.GetProductId(), UserId: req.GetUserId(), Rating: 4, Content: "my review", Status: req.GetStatus()},
		},
		Total: 1,
	}, nil
}

func (f *fakeMallReviewClient) ListReviewableOrders(_ context.Context, req *mallpb.ListReviewableOrdersRequest, _ ...grpc.CallOption) (*mallpb.ListOrdersResponse, error) {
	f.listReviewableOrdersReq = req
	return &mallpb.ListOrdersResponse{
		Items: []*mallpb.Order{
			{Id: 88, UserId: req.GetUserId(), Status: mallpb.OrderStatus_ORDER_STATUS_COMPLETED},
		},
		Total: 1,
	}, nil
}
