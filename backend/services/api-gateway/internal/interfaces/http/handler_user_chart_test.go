package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestUserChartGETUsesQueryAndReturnsRawCompatibilityPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &userChartHTTPClient{response: &userpb.UserChartResponse{
		Local:  &userpb.UserChartSeries{Total: []int64{9, 8}, Inc: []int64{2, 1}, Dec: []int64{0, 1}},
		Remote: &userpb.UserChartSeries{Total: []int64{0, 0}, Inc: []int64{0, 0}, Dec: []int64{0, 0}},
	}}
	router := userChartTestRouter(client)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/users?span=day&limit=2&offset=0", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "day", client.request.GetSpan())
	require.Equal(t, int32(2), client.request.GetLimit())
	require.NotNil(t, client.request.Offset)
	require.Equal(t, int64(0), client.request.GetOffset())

	var payload struct {
		Local struct {
			Total []int64 `json:"total"`
			Inc   []int64 `json:"inc"`
			Dec   []int64 `json:"dec"`
		} `json:"local"`
		Remote struct {
			Total []int64 `json:"total"`
			Inc   []int64 `json:"inc"`
			Dec   []int64 `json:"dec"`
		} `json:"remote"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, []int64{9, 8}, payload.Local.Total)
	require.Equal(t, []int64{2, 1}, payload.Local.Inc)
	require.Equal(t, []int64{0, 0}, payload.Remote.Total)
	require.NotContains(t, recorder.Body.String(), `"data"`)
}

func TestUserChartPOSTUsesJSONAndDefaultsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &userChartHTTPClient{}
	router := userChartTestRouter(client)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/charts/users", strings.NewReader(`{"span":"hour","offset":0}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "hour", client.request.GetSpan())
	require.Equal(t, defaultUserChartLimit, client.request.GetLimit())
	require.NotNil(t, client.request.Offset)
	require.Equal(t, int64(0), client.request.GetOffset())
	require.JSONEq(t, `{"local":{"total":[],"inc":[],"dec":[]},"remote":{"total":[],"inc":[],"dec":[]}}`, recorder.Body.String())
}

func TestUserChartAcceptsMaximumLimitAndOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &userChartHTTPClient{response: &userpb.UserChartResponse{}}
	router := userChartTestRouter(client)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/users?span=day&limit=500&offset=8640000000000000", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(500), client.request.GetLimit())
	require.NotNil(t, client.request.Offset)
	require.Equal(t, maxUserChartOffset, client.request.GetOffset())
}

func TestUserChartRejectsInvalidParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "missing span", method: stdhttp.MethodGet, path: "/charts/users"},
		{name: "unknown span", method: stdhttp.MethodGet, path: "/charts/users?span=week"},
		{name: "invalid limit", method: stdhttp.MethodGet, path: "/charts/users?span=day&limit=nope"},
		{name: "zero limit", method: stdhttp.MethodGet, path: "/charts/users?span=day&limit=0"},
		{name: "large limit", method: stdhttp.MethodGet, path: "/charts/users?span=day&limit=501"},
		{name: "negative offset", method: stdhttp.MethodGet, path: "/charts/users?span=day&offset=-1"},
		{name: "large offset", method: stdhttp.MethodGet, path: "/charts/users?span=day&offset=8640000000000001"},
		{name: "invalid offset", method: stdhttp.MethodGet, path: "/charts/users?span=day&offset=nope"},
		{name: "invalid json", method: stdhttp.MethodPost, path: "/charts/users", body: `{"span":`},
		{name: "invalid json limit", method: stdhttp.MethodPost, path: "/charts/users", body: `{"span":"day","limit":0}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &userChartHTTPClient{response: &userpb.UserChartResponse{}}
			router := userChartTestRouter(client)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.method == stdhttp.MethodPost {
				request.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Nil(t, client.request)
		})
	}
}

func userChartTestRouter(client clients.UserChartClient) *gin.Engine {
	h := NewHandler(&clients.Clients{UserCharts: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type userChartHTTPClient struct {
	request  *userpb.UserChartRequest
	response *userpb.UserChartResponse
}

func (f *userChartHTTPClient) GetUserChart(_ context.Context, request *userpb.UserChartRequest, _ ...grpc.CallOption) (*userpb.UserChartResponse, error) {
	f.request = request
	return f.response, nil
}
