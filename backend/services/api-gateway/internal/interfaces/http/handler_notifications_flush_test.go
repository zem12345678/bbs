package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/api/proto/notificationpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestFlushNotificationsRoutesRequireAuthAndForwardUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &flushNotificationsClient{}
	h := NewHandler(&clients.Clients{Notification: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	for _, path := range []string{"/api/v1/notifications/flush", "/api/i/notifications/flush", "/i/notifications/flush"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodPost, path, nil)
		request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
		router.ServeHTTP(recorder, request)

		require.Equal(t, stdhttp.StatusNoContent, recorder.Code, path+": "+recorder.Body.String())
		require.Empty(t, recorder.Body.String())
		require.Equal(t, int64(42), client.request.GetUserId())
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/notifications/flush", nil))
	require.Equal(t, stdhttp.StatusUnauthorized, unauthorized.Code)

	for _, path := range []string{"/api/v1/notifications/flush", "/api/i/notifications/flush", "/i/notifications/flush"} {
		readOnly := httptest.NewRecorder()
		readRequest := httptest.NewRequest(stdhttp.MethodPost, path, nil)
		readRequest.Header.Set("Authorization", "Bearer "+flushNotificationsAPIToken(t, "read"))
		router.ServeHTTP(readOnly, readRequest)
		require.Equal(t, stdhttp.StatusForbidden, readOnly.Code, path+": "+readOnly.Body.String())

		write := httptest.NewRecorder()
		writeRequest := httptest.NewRequest(stdhttp.MethodPost, path, nil)
		writeRequest.Header.Set("Authorization", "Bearer "+flushNotificationsAPIToken(t, "write"))
		router.ServeHTTP(write, writeRequest)
		require.Equal(t, stdhttp.StatusNoContent, write.Code, path+": "+write.Body.String())
	}
}

func flushNotificationsAPIToken(t *testing.T, scope string) string {
	t.Helper()
	return signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "notification-flush-" + scope, "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: credentialVersionInitial, "token_type": apiTokenType, "scopes": []string{scope},
	})
}

type flushNotificationsClient struct {
	notificationpb.NotificationServiceClient
	request *notificationpb.FlushRequest
}

func (c *flushNotificationsClient) Flush(_ context.Context, request *notificationpb.FlushRequest, _ ...grpc.CallOption) (*notificationpb.MutationResponse, error) {
	c.request = request
	return &notificationpb.MutationResponse{Success: true, Message: "ok"}, nil
}
