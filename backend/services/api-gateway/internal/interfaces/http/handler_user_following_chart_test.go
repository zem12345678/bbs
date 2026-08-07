package http

import (
	"context"
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

func TestUserFollowingChartGETUsesQueryAndReturnsScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &userFollowingChartHTTPClient{response: &userpb.UserFollowingChartResponse{
		Local: &userpb.UserFollowingChartScope{
			Followings: &userpb.UserChartSeries{Total: []int64{1, 2}, Inc: []int64{1, 1}, Dec: []int64{0, 0}},
			Followers:  &userpb.UserChartSeries{Total: []int64{3, 2}, Inc: []int64{0, 0}, Dec: []int64{0, 1}},
		},
		Remote: &userpb.UserFollowingChartScope{
			Followings: &userpb.UserChartSeries{Total: []int64{0, 0}, Inc: []int64{0, 0}, Dec: []int64{0, 0}},
			Followers:  &userpb.UserChartSeries{Total: []int64{0, 0}, Inc: []int64{0, 0}, Dec: []int64{0, 0}},
		},
	}}
	router := userFollowingChartTestRouter(client)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/user/following?span=day&userId=42&limit=2&offset=0", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, client.requests, 1)
	require.Equal(t, "day", client.requests[0].GetSpan())
	require.Equal(t, int64(42), client.requests[0].GetUserId())
	require.Equal(t, int32(2), client.requests[0].GetLimit())
	require.NotNil(t, client.requests[0].Offset)
	require.Equal(t, int64(0), client.requests[0].GetOffset())
	require.JSONEq(t, `{
		"local":{
			"followings":{"total":[1,2],"inc":[1,1],"dec":[0,0]},
			"followers":{"total":[3,2],"inc":[0,0],"dec":[0,1]}
		},
		"remote":{
			"followings":{"total":[0,0],"inc":[0,0],"dec":[0,0]},
			"followers":{"total":[0,0],"inc":[0,0],"dec":[0,0]}
		}
	}`, recorder.Body.String())
}

func TestUserFollowingChartPOSTUsesJSONAndDefaultsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &userFollowingChartHTTPClient{}
	router := userFollowingChartTestRouter(client)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/charts/user/following", strings.NewReader(`{"span":"hour","userId":"9223372036854775807","offset":0}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, client.requests, 1)
	require.Equal(t, "hour", client.requests[0].GetSpan())
	require.Equal(t, int64(9223372036854775807), client.requests[0].GetUserId())
	require.Equal(t, defaultUserChartLimit, client.requests[0].GetLimit())
	require.NotNil(t, client.requests[0].Offset)
	require.Equal(t, int64(0), client.requests[0].GetOffset())
	require.JSONEq(t, `{
		"local":{"followings":{"total":[],"inc":[],"dec":[]},"followers":{"total":[],"inc":[],"dec":[]}},
		"remote":{"followings":{"total":[],"inc":[],"dec":[]},"followers":{"total":[],"inc":[],"dec":[]}}
	}`, recorder.Body.String())
}

func TestUserFollowingChartAcceptsMaximumLimitAndOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &userFollowingChartHTTPClient{}
	router := userFollowingChartTestRouter(client)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/charts/user/following?span=day&userId=1&limit=500&offset=8640000000000000", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, client.requests, 1)
	require.Equal(t, maxUserChartLimit, client.requests[0].GetLimit())
	require.NotNil(t, client.requests[0].Offset)
	require.Equal(t, maxUserChartOffset, client.requests[0].GetOffset())
}

func TestUserFollowingChartRejectsInvalidParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "missing span", method: stdhttp.MethodGet, path: "/charts/user/following?userId=1"},
		{name: "unknown span", method: stdhttp.MethodGet, path: "/charts/user/following?span=week&userId=1"},
		{name: "invalid limit", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=1&limit=nope"},
		{name: "zero limit", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=1&limit=0"},
		{name: "large limit", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=1&limit=501"},
		{name: "negative offset", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=1&offset=-1"},
		{name: "large offset", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=1&offset=8640000000000001"},
		{name: "invalid offset", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=1&offset=nope"},
		{name: "missing user", method: stdhttp.MethodGet, path: "/charts/user/following?span=day"},
		{name: "zero user", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=0"},
		{name: "negative user", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=-1"},
		{name: "invalid user", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=user"},
		{name: "overflow user", method: stdhttp.MethodGet, path: "/charts/user/following?span=day&userId=9223372036854775808"},
		{name: "invalid json", method: stdhttp.MethodPost, path: "/charts/user/following", body: `{"span":`},
		{name: "numeric json user", method: stdhttp.MethodPost, path: "/charts/user/following", body: `{"span":"day","userId":42}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &userFollowingChartHTTPClient{}
			router := userFollowingChartTestRouter(client)
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

func userFollowingChartTestRouter(client clients.UserFollowingChartClient) *gin.Engine {
	h := NewHandler(&clients.Clients{UserFollowingCharts: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	return router
}

type userFollowingChartHTTPClient struct {
	requests []*userpb.UserFollowingChartRequest
	response *userpb.UserFollowingChartResponse
}

func (f *userFollowingChartHTTPClient) GetUserFollowingChart(_ context.Context, request *userpb.UserFollowingChartRequest, _ ...grpc.CallOption) (*userpb.UserFollowingChartResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, nil
}
