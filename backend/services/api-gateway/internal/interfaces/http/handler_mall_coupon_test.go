package http

import (
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

func TestListMyMallCouponsForwardsCurrentUserAndFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallCouponClient{listUsagesResponse: &mallpb.ListCouponUsagesResponse{
		Items: []*mallpb.CouponUsage{{Id: 501, CouponId: 77, UserId: 42, Status: mallpb.CouponUsageStatus_COUPON_USAGE_STATUS_CLAIMED}},
		Total: 1,
	}}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallCouponContext(http.MethodGet, "/api/v1/mall/coupons/mine?status=4&limit=8&offset=16", 0, 42)
	h.listMyMallCoupons(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.listUsagesReq)
	require.Equal(t, int64(42), mallClient.listUsagesReq.GetUserId())
	require.Equal(t, mallpb.CouponUsageStatus_COUPON_USAGE_STATUS_CLAIMED, mallClient.listUsagesReq.GetStatus())
	require.Equal(t, int32(8), mallClient.listUsagesReq.GetLimit())
	require.Equal(t, int32(16), mallClient.listUsagesReq.GetOffset())

	var envelope struct {
		Data struct {
			Items []struct {
				Id     int64 `json:"id"`
				Status int32 `json:"status"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, int64(501), envelope.Data.Items[0].Id)
	require.Equal(t, int32(mallpb.CouponUsageStatus_COUPON_USAGE_STATUS_CLAIMED), envelope.Data.Items[0].Status)
	require.Equal(t, int64(1), envelope.Data.Total)
}

func TestClaimMallCouponForwardsCurrentUserAndCouponID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallCouponClient{claimResponse: &mallpb.CouponUsageResponse{
		Usage:     &mallpb.CouponUsage{Id: 501, CouponId: 77, UserId: 42, Status: mallpb.CouponUsageStatus_COUPON_USAGE_STATUS_CLAIMED},
		Duplicate: false,
	}}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallCouponContext(http.MethodPost, "/api/v1/mall/coupons/77/claim", 77, 42)
	h.claimMallCoupon(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.claimReq)
	require.Equal(t, int64(42), mallClient.claimReq.GetUserId())
	require.Equal(t, int64(77), mallClient.claimReq.GetCouponId())

	var envelope struct {
		Data struct {
			Usage struct {
				Id       int64 `json:"id"`
				CouponID int64 `json:"coupon_id"`
				Status   int32 `json:"status"`
			} `json:"usage"`
			Duplicate bool `json:"duplicate"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, int64(501), envelope.Data.Usage.Id)
	require.Equal(t, int64(77), envelope.Data.Usage.CouponID)
	require.Equal(t, int32(mallpb.CouponUsageStatus_COUPON_USAGE_STATUS_CLAIMED), envelope.Data.Usage.Status)
	require.False(t, envelope.Data.Duplicate)
}

func TestClaimMallCouponForwardsCurrentUserAndMapsUnavailableError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeMallCouponClient{claimErr: status.Error(codes.FailedPrecondition, "coupon unavailable")}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	c, recorder := newMallCouponContext(http.MethodPost, "/api/v1/mall/coupons/77/claim", 77, 42)
	h.claimMallCoupon(c)

	require.Equal(t, http.StatusPreconditionFailed, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.claimReq)
	require.Equal(t, int64(42), mallClient.claimReq.GetUserId())
	require.Equal(t, int64(77), mallClient.claimReq.GetCouponId())

	var envelope struct {
		Message string         `json:"message"`
		Meta    map[string]any `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "coupon unavailable", envelope.Message)
	require.Equal(t, codes.FailedPrecondition.String(), envelope.Meta["legacy_code"])
}

func newMallCouponContext(method string, rawURL string, couponID int64, userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	if couponID > 0 {
		c.Params = gin.Params{{Key: "id", Value: "77"}}
	}
	c.Set("user_id", userID)
	c.Request = httptest.NewRequest(method, rawURL, nil)
	return c, recorder
}

type fakeMallCouponClient struct {
	mallpb.MallServiceClient
	listUsagesReq      *mallpb.ListUserCouponUsagesRequest
	listUsagesResponse *mallpb.ListCouponUsagesResponse
	claimReq           *mallpb.ClaimCouponRequest
	claimErr           error
	claimResponse      *mallpb.CouponUsageResponse
}

func (f *fakeMallCouponClient) ListUserCouponUsages(_ context.Context, req *mallpb.ListUserCouponUsagesRequest, _ ...grpc.CallOption) (*mallpb.ListCouponUsagesResponse, error) {
	f.listUsagesReq = req
	if f.listUsagesResponse != nil {
		return f.listUsagesResponse, nil
	}
	return &mallpb.ListCouponUsagesResponse{}, nil
}

func (f *fakeMallCouponClient) ClaimCoupon(_ context.Context, req *mallpb.ClaimCouponRequest, _ ...grpc.CallOption) (*mallpb.CouponUsageResponse, error) {
	f.claimReq = req
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	if f.claimResponse != nil {
		return f.claimResponse, nil
	}
	return &mallpb.CouponUsageResponse{}, nil
}
