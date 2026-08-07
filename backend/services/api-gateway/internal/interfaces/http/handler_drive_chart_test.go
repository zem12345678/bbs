package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/filepb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestDriveChartGETUsesQueryAndReturnsDeltaPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &driveChartHTTPClient{response: &filepb.DriveChartResponse{
		Local: &filepb.DriveChartSeries{
			TotalCount: []int64{10, 11}, TotalSize: []float64{20.5, 22},
			IncCount: []int64{2, 3}, IncSize: []float64{1.25, 2.5},
			DecCount: []int64{0, 1}, DecSize: []float64{0, 0.75},
		},
		Remote: &filepb.DriveChartSeries{
			IncCount: []int64{0, 0}, IncSize: []float64{0, 0},
			DecCount: []int64{0, 0}, DecSize: []float64{0, 0},
		},
	}}
	router := driveChartTestRouter(client)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/drive?span=day&limit=2&offset=0", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, client.requests, 1)
	require.Equal(t, "day", client.requests[0].GetSpan())
	require.Equal(t, int32(2), client.requests[0].GetLimit())
	require.NotNil(t, client.requests[0].Offset)
	require.Equal(t, int64(0), client.requests[0].GetOffset())
	require.Zero(t, client.requests[0].GetOwnerId())
	require.JSONEq(t, `{
		"local":{"incCount":[2,3],"incSize":[1.25,2.5],"decCount":[0,1],"decSize":[0,0.75]},
		"remote":{"incCount":[0,0],"incSize":[0,0],"decCount":[0,0],"decSize":[0,0]}
	}`, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "totalCount")
}

func TestDriveChartPOSTUsesJSONAndDefaultsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &driveChartHTTPClient{}
	router := driveChartTestRouter(client)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/charts/drive", strings.NewReader(`{"span":"hour","offset":0}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, client.requests, 1)
	require.Equal(t, "hour", client.requests[0].GetSpan())
	require.Equal(t, defaultDriveChartLimit, client.requests[0].GetLimit())
	require.NotNil(t, client.requests[0].Offset)
	require.Equal(t, int64(0), client.requests[0].GetOffset())
	require.JSONEq(t, `{
		"local":{"incCount":[],"incSize":[],"decCount":[],"decSize":[]},
		"remote":{"incCount":[],"incSize":[],"decCount":[],"decSize":[]}
	}`, recorder.Body.String())
}

func TestUserDriveChartGETMapsUserIDAndReturnsLocalTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &driveChartHTTPClient{response: &filepb.DriveChartResponse{Local: &filepb.DriveChartSeries{
		TotalCount: []int64{5}, TotalSize: []float64{10.125},
		IncCount: []int64{2}, IncSize: []float64{3.5},
		DecCount: []int64{1}, DecSize: []float64{0.25},
	}}}
	router := driveChartTestRouter(client)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/user/drive?span=day&userId=9223372036854775807&limit=500&offset=8640000000000000", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, client.requests, 1)
	require.Equal(t, int64(9223372036854775807), client.requests[0].GetOwnerId())
	require.Equal(t, maxDriveChartLimit, client.requests[0].GetLimit())
	require.Equal(t, maxDriveChartOffset, client.requests[0].GetOffset())
	require.JSONEq(t, `{
		"totalCount":[5],"totalSize":[10.125],
		"incCount":[2],"incSize":[3.5],"decCount":[1],"decSize":[0.25]
	}`, recorder.Body.String())
}

func TestUserDriveChartPOSTMapsUserIDAndReturnsEmptyArrays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &driveChartHTTPClient{}
	router := driveChartTestRouter(client)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/charts/user/drive", strings.NewReader(`{"span":"hour","userId":"42"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, client.requests, 1)
	require.Equal(t, int64(42), client.requests[0].GetOwnerId())
	require.JSONEq(t, `{
		"totalCount":[],"totalSize":[],"incCount":[],"incSize":[],
		"decCount":[],"decSize":[]
	}`, recorder.Body.String())
}

func TestDriveChartsRejectInvalidParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "missing span", method: stdhttp.MethodGet, path: "/charts/drive"},
		{name: "unknown span", method: stdhttp.MethodGet, path: "/charts/drive?span=week"},
		{name: "invalid limit", method: stdhttp.MethodGet, path: "/charts/drive?span=day&limit=nope"},
		{name: "zero limit", method: stdhttp.MethodGet, path: "/charts/drive?span=day&limit=0"},
		{name: "large limit", method: stdhttp.MethodGet, path: "/charts/drive?span=day&limit=501"},
		{name: "negative offset", method: stdhttp.MethodGet, path: "/charts/drive?span=day&offset=-1"},
		{name: "large offset", method: stdhttp.MethodGet, path: "/charts/drive?span=day&offset=8640000000000001"},
		{name: "invalid offset", method: stdhttp.MethodGet, path: "/charts/drive?span=day&offset=nope"},
		{name: "missing user", method: stdhttp.MethodGet, path: "/charts/user/drive?span=day"},
		{name: "zero user", method: stdhttp.MethodGet, path: "/charts/user/drive?span=day&userId=0"},
		{name: "negative user", method: stdhttp.MethodGet, path: "/charts/user/drive?span=day&userId=-1"},
		{name: "invalid user", method: stdhttp.MethodGet, path: "/charts/user/drive?span=day&userId=user"},
		{name: "overflow user", method: stdhttp.MethodGet, path: "/charts/user/drive?span=day&userId=9223372036854775808"},
		{name: "invalid json", method: stdhttp.MethodPost, path: "/charts/drive", body: `{"span":`},
		{name: "numeric json user", method: stdhttp.MethodPost, path: "/charts/user/drive", body: `{"span":"day","userId":42}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &driveChartHTTPClient{}
			router := driveChartTestRouter(client)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.method == stdhttp.MethodPost {
				request.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Empty(t, client.requests)
		})
	}
}

func driveChartTestRouter(client filepb.FileServiceClient) *gin.Engine {
	h := NewHandler(&clients.Clients{File: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type driveChartHTTPClient struct {
	filepb.FileServiceClient
	requests []*filepb.DriveChartRequest
	response *filepb.DriveChartResponse
}

func (f *driveChartHTTPClient) GetDriveChart(_ context.Context, request *filepb.DriveChartRequest, _ ...grpc.CallOption) (*filepb.DriveChartResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, nil
}
