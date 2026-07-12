package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListLevelsUsesPublicEnabledLevelQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakePublicLevelsAdminClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/levels?limit=7&offset=3", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, adminClient.listLevelsCalls)
	require.Nil(t, adminClient.listLevelsReq.GetActor())
	require.Equal(t, int32(2), adminClient.listLevelsReq.GetStatus())
	require.Equal(t, int32(7), adminClient.listLevelsReq.GetLimit())
	require.Equal(t, int32(3), adminClient.listLevelsReq.GetOffset())

	var envelope struct {
		Data adminpb.LevelListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "lv1", envelope.Data.Items[0].GetKey())
}

type fakePublicLevelsAdminClient struct {
	adminpb.AdminServiceClient
	listLevelsCalls int
	listLevelsReq   *adminpb.ListLevelsRequest
}

func (f *fakePublicLevelsAdminClient) ListLevels(_ context.Context, req *adminpb.ListLevelsRequest, _ ...grpc.CallOption) (*adminpb.LevelListResponse, error) {
	f.listLevelsCalls++
	f.listLevelsReq = req
	return &adminpb.LevelListResponse{
		Items: []*adminpb.LevelInfo{
			{Id: 1, Key: "lv1", Name: "LV.1", MinScore: 0, MaxScore: 99, Status: 2},
		},
		Total: 1,
	}, nil
}
