package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListCategoriesForcesEnabledStatus(t *testing.T) {
	contentClient := &fakeCategoryContentClient{
		listResp: &contentpb.CategoryListResponse{Items: []*contentpb.CategoryInfo{{Id: 1, Status: categoryStatusEnabled}}},
	}
	h := NewHandler(&clients.Clients{Content: contentClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/categories?status=1&limit=7&offset=3", nil)

	h.listCategories(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.listReq)
	require.Equal(t, categoryStatusEnabled, contentClient.listReq.GetStatus())
	require.EqualValues(t, 7, contentClient.listReq.GetLimit())
	require.EqualValues(t, 3, contentClient.listReq.GetOffset())
}

func TestGetCategoryRequiresEnabledCategory(t *testing.T) {
	contentClient := &fakeCategoryContentClient{
		category: &contentpb.CategoryInfo{Id: 1, Status: 1},
	}
	h := NewHandler(&clients.Clients{Content: contentClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/categories/1", nil)

	h.getCategory(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.categoryReq)
}

func TestGetCategoryForwardsEnabledCategory(t *testing.T) {
	contentClient := &fakeCategoryContentClient{
		category: &contentpb.CategoryInfo{Id: 1, Status: categoryStatusEnabled},
	}
	h := NewHandler(&clients.Clients{Content: contentClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/categories/1", nil)

	h.getCategory(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.categoryReq)
}

type fakeCategoryContentClient struct {
	contentpb.ContentServiceClient
	listReq     *contentpb.ListCategoriesRequest
	categoryReq *contentpb.CategoryIDRequest
	listResp    *contentpb.CategoryListResponse
	category    *contentpb.CategoryInfo
}

func (f *fakeCategoryContentClient) ListCategories(_ context.Context, req *contentpb.ListCategoriesRequest, _ ...grpc.CallOption) (*contentpb.CategoryListResponse, error) {
	f.listReq = req
	if f.listResp != nil {
		return f.listResp, nil
	}
	return &contentpb.CategoryListResponse{Items: []*contentpb.CategoryInfo{}}, nil
}

func (f *fakeCategoryContentClient) GetCategory(_ context.Context, req *contentpb.CategoryIDRequest, _ ...grpc.CallOption) (*contentpb.CategoryResponse, error) {
	f.categoryReq = req
	return &contentpb.CategoryResponse{Success: true, Message: "ok", Category: f.category}, nil
}
