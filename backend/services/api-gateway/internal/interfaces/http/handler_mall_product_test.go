package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/mallpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestCreateAdminMallProductForwardsGrantFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mallClient := &fakeAdminMallProductClient{}
	h := NewHandler(&clients.Clients{Mall: mallClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("admin_id", int64(99))
	c.Set("admin_username", "operator")
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/mall/products",
		strings.NewReader(`{"sku":"BADGE-FOUNDER","title":"创始会员徽章","category":"digital","grant_type":"badge","grant_key":"badge-founder","price_credits":80,"stock":9999,"status":2,"sort":10,"operator_id":"forged-admin"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.createAdminMallProduct(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, mallClient.createReq)
	require.Equal(t, "BADGE-FOUNDER", mallClient.createReq.GetSku())
	require.Equal(t, "badge", mallClient.createReq.GetGrantType())
	require.Equal(t, "badge-founder", mallClient.createReq.GetGrantKey())
	require.Equal(t, "99", mallClient.createReq.GetOperatorId())
}

func TestMallProductsPayloadIncludesGrantFields(t *testing.T) {
	payload := mallProductsPayload([]*mallpb.Product{{
		Id:        1001,
		Sku:       "badge-founder",
		Title:     "创始会员徽章",
		Category:  "digital",
		GrantType: "badge",
		GrantKey:  "badge-founder",
		Status:    mallpb.ProductStatus_PRODUCT_STATUS_ACTIVE,
	}})

	require.Len(t, payload, 1)
	require.Equal(t, "badge", payload[0]["grant_type"])
	require.Equal(t, "badge-founder", payload[0]["grant_key"])
}

type fakeAdminMallProductClient struct {
	mallpb.MallServiceClient
	createReq *mallpb.AdminCreateProductRequest
}

func (f *fakeAdminMallProductClient) AdminCreateProduct(_ context.Context, req *mallpb.AdminCreateProductRequest, _ ...grpc.CallOption) (*mallpb.ProductResponse, error) {
	f.createReq = req
	return &mallpb.ProductResponse{
		Product: &mallpb.Product{
			Id:        1001,
			Sku:       req.GetSku(),
			Title:     req.GetTitle(),
			GrantType: req.GetGrantType(),
			GrantKey:  req.GetGrantKey(),
			Status:    req.GetStatus(),
		},
	}, nil
}
