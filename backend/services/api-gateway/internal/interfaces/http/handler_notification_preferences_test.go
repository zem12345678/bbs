package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/notificationpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
)

func TestNotificationPreferenceRoutesUseAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &notificationPreferencesHTTPClient{
		getResponse:    &notificationpb.PreferencesResponse{Items: []*notificationpb.NotificationPreference{{Type: "comment", Enabled: true}}},
		updateResponse: &notificationpb.PreferencesResponse{Items: []*notificationpb.NotificationPreference{{Type: "comment", Enabled: false}}},
	}
	h := NewHandler(&clients.Clients{Notification: client}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "9223372036854770001"})

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/notification-preferences", nil)
	getRequest.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d; body=%s", getRecorder.Code, http.StatusOK, getRecorder.Body.String())
	}
	if client.getRequest.GetUserId() != 9223372036854770001 {
		t.Fatalf("GET user ID = %d", client.getRequest.GetUserId())
	}

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/notification-preferences", strings.NewReader(`{"items":[{"type":"comment","enabled":false}]}`))
	updateRequest.Header.Set("Authorization", "Bearer "+token)
	updateRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d; body=%s", updateRecorder.Code, http.StatusOK, updateRecorder.Body.String())
	}
	if client.updateRequest.GetUserId() != 9223372036854770001 {
		t.Fatalf("PUT user ID = %d", client.updateRequest.GetUserId())
	}
	if items := client.updateRequest.GetItems(); len(items) != 1 || items[0].GetType() != "comment" || items[0].GetEnabled() {
		t.Fatalf("PUT items = %+v", items)
	}
}

type notificationPreferencesHTTPClient struct {
	notificationpb.NotificationServiceClient
	getRequest     *notificationpb.GetPreferencesRequest
	updateRequest  *notificationpb.UpdatePreferencesRequest
	getResponse    *notificationpb.PreferencesResponse
	updateResponse *notificationpb.PreferencesResponse
}

func (c *notificationPreferencesHTTPClient) GetPreferences(_ context.Context, req *notificationpb.GetPreferencesRequest, _ ...grpc.CallOption) (*notificationpb.PreferencesResponse, error) {
	c.getRequest = req
	return c.getResponse, nil
}

func (c *notificationPreferencesHTTPClient) UpdatePreferences(_ context.Context, req *notificationpb.UpdatePreferencesRequest, _ ...grpc.CallOption) (*notificationpb.PreferencesResponse, error) {
	c.updateRequest = req
	return c.updateResponse, nil
}
