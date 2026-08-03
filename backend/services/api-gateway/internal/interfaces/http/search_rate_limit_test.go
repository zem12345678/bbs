package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSearchRateLimitsBlockBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		path        string
		limits      SearchRateLimits
		expectedKey string
		assertNoRPC func(*testing.T, *fakeSearchVisibilityClient, *fakeUserClient)
	}{
		{
			name:        "articles share content limiter",
			path:        "/api/v1/search/articles?q=codx",
			limits:      SearchRateLimits{Content: &searchRateLimitStub{limited: true}},
			expectedKey: searchRateLimitKey(searchRateLimitContent, "203.0.113.20"),
			assertNoRPC: func(t *testing.T, searchClient *fakeSearchVisibilityClient, _ *fakeUserClient) {
				require.Nil(t, searchClient.articleReq)
			},
		},
		{
			name:        "topics share content limiter",
			path:        "/api/v1/search/topics?q=paymnt",
			limits:      SearchRateLimits{Content: &searchRateLimitStub{limited: true}},
			expectedKey: searchRateLimitKey(searchRateLimitContent, "203.0.113.20"),
			assertNoRPC: func(t *testing.T, searchClient *fakeSearchVisibilityClient, _ *fakeUserClient) {
				require.Nil(t, searchClient.topicReq)
			},
		},
		{
			name:        "users use separate limiter",
			path:        "/api/v1/search/users?q=ali",
			limits:      SearchRateLimits{User: &searchRateLimitStub{limited: true}},
			expectedKey: searchRateLimitKey(searchRateLimitUser, "203.0.113.20"),
			assertNoRPC: func(t *testing.T, searchClient *fakeSearchVisibilityClient, userClient *fakeUserClient) {
				require.Nil(t, searchClient.userReq)
				require.Zero(t, userClient.listUsersCalls)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchClient := &fakeSearchVisibilityClient{}
			userClient := &fakeUserClient{}
			h := NewHandler(&clients.Clients{Search: searchClient, User: userClient}, "Authorization", "Bearer", testJWTSecret)
			h.SetSearchRateLimits(tt.limits)
			router := gin.New()
			NewInitControllers(h)(router)

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(stdhttp.MethodGet, tt.path, nil)
			req.RemoteAddr = "203.0.113.20:12345"
			router.ServeHTTP(recorder, req)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
			limiter := tt.limits.Content
			if limiter == nil {
				limiter = tt.limits.User
			}
			require.Equal(t, []string{tt.expectedKey}, limiter.(*searchRateLimitStub).keys)
			tt.assertNoRPC(t, searchClient, userClient)
		})
	}
}

func TestSearchRateLimitReturnsUnavailableWhenRedisFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
	h.SetSearchRateLimits(SearchRateLimits{Content: &searchRateLimitStub{err: errors.New("redis unavailable")}})
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/search/articles?q=codx", nil)
	req.RemoteAddr = "203.0.113.20:12345"
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "search rate limiter unavailable")
}

func TestSearchRateLimitKeyIsNormalizedAndDoesNotExposeIP(t *testing.T) {
	key := searchRateLimitKey(searchRateLimitContent, " 203.0.113.20 ")
	require.Equal(t, searchRateLimitKey(searchRateLimitContent, "203.0.113.20"), key)
	require.NotContains(t, key, "203.0.113.20")
	require.True(t, strings.HasPrefix(key, "rate:search:content:ip:"))
}

type searchRateLimitStub struct {
	err     error
	keys    []string
	limited bool
}

func (l *searchRateLimitStub) Limit(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	return l.limited, l.err
}
