package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAccountLifecycleRouteUsesJWTCurrentUserAndReturnsStringIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const userID int64 = 9_007_199_254_740_993
	client := &accountLifecycleHTTPClient{getResponse: &userpb.AccountLifecycleResponse{
		UserId:       9_223_372_036_854_775_000,
		State:        "deletion_pending",
		StateVersion: 9_007_199_254_740_995,
		ActiveDeletionJob: &userpb.AccountDeletionJobInfo{
			Id:             9_223_372_036_854_774_000,
			Status:         "pending",
			PolicyVersion:  1,
			CompletedSteps: 2,
			TotalSteps:     10,
		},
	}}
	h := NewHandler(&clients.Clients{UserAccountLifecycle: client}, "Authorization", "Bearer", testJWTSecret)
	router := accountLifecycleTestRouter(h)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "9007199254740993", "username": "alice"})

	recorder := performAccountLifecycleRequest(router, stdhttp.MethodGet, "/api/v1/users/me/account-lifecycle", "", token)

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, client.getRequest)
	require.Equal(t, userID, client.getRequest.GetId())
	var envelope struct {
		Data struct {
			UserID            string `json:"user_id"`
			StateVersion      string `json:"state_version"`
			ActiveDeletionJob struct {
				ID string `json:"id"`
			} `json:"active_deletion_job"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9223372036854775000", envelope.Data.UserID)
	require.Equal(t, "9007199254740995", envelope.Data.StateVersion)
	require.Equal(t, "9223372036854774000", envelope.Data.ActiveDeletionJob.ID)
}

func TestRequestAccountDeletionReturnsAcceptedAndForwardsCurrentUserCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const userID int64 = 332_578_385_639_251_969
	client := &accountLifecycleHTTPClient{deleteResponse: &userpb.AccountLifecycleResponse{
		UserId:       userID,
		State:        "deletion_pending",
		StateVersion: 2,
		ActiveDeletionJob: &userpb.AccountDeletionJobInfo{
			Id:         9_223_372_036_854_773_000,
			Status:     "pending",
			TotalSteps: 10,
		},
	}}
	h := NewHandler(&clients.Clients{UserAccountLifecycle: client}, "Authorization", "Bearer", testJWTSecret)
	router := accountLifecycleTestRouter(h)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "332578385639251969"})
	body := `{"user_id":"1","password":"secret-password","code":"backup-code"}`

	recorder := performAccountLifecycleRequest(router, stdhttp.MethodPost, "/api/v1/users/me/deletion-requests", body, token)

	require.Equal(t, stdhttp.StatusAccepted, recorder.Code, recorder.Body.String())
	require.NotNil(t, client.deleteRequest)
	require.Equal(t, userID, client.deleteRequest.GetUserId())
	require.Equal(t, "secret-password", client.deleteRequest.GetPassword())
	require.Equal(t, "backup-code", client.deleteRequest.GetCode())
	var envelope struct {
		HTTPCode int `json:"http_code"`
		Data     struct {
			UserID            string `json:"user_id"`
			ActiveDeletionJob struct {
				ID string `json:"id"`
			} `json:"active_deletion_job"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, stdhttp.StatusAccepted, envelope.HTTPCode)
	require.Equal(t, "332578385639251969", envelope.Data.UserID)
	require.Equal(t, "9223372036854773000", envelope.Data.ActiveDeletionJob.ID)
}

func TestAccountLifecycleHandlersMapUnavailableAndLifecycleErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "77"})
	tests := []struct {
		name       string
		clients    *clients.Clients
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "client unavailable",
			clients:    nil,
			method:     stdhttp.MethodGet,
			path:       "/api/v1/users/me/account-lifecycle",
			wantStatus: stdhttp.StatusServiceUnavailable,
			wantCode:   "service_unavailable",
		},
		{
			name: "upstream unavailable",
			clients: &clients.Clients{UserAccountLifecycle: &accountLifecycleHTTPClient{
				getErr: status.Error(codes.Unavailable, "account lifecycle store unavailable"),
			}},
			method:     stdhttp.MethodGet,
			path:       "/api/v1/users/me/account-lifecycle",
			wantStatus: stdhttp.StatusServiceUnavailable,
			wantCode:   codes.Unavailable.String(),
		},
		{
			name: "protected account",
			clients: &clients.Clients{UserAccountLifecycle: &accountLifecycleHTTPClient{
				deleteErr: status.Error(codes.FailedPrecondition, "protected account cannot be deleted"),
			}},
			method:     stdhttp.MethodPost,
			path:       "/api/v1/users/me/deletion-requests",
			body:       `{"password":"secret-password"}`,
			wantStatus: stdhttp.StatusPreconditionFailed,
			wantCode:   codes.FailedPrecondition.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.clients, "Authorization", "Bearer", testJWTSecret)
			router := accountLifecycleTestRouter(h)

			recorder := performAccountLifecycleRequest(router, tt.method, tt.path, tt.body, token)

			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			var envelope struct {
				Meta map[string]any `json:"meta"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
			require.Equal(t, tt.wantCode, envelope.Meta["legacy_code"])
		})
	}
}

func accountLifecycleTestRouter(handler *Handler) *gin.Engine {
	router := gin.New()
	NewInitControllers(handler)(router)
	return router
}

func performAccountLifecycleRequest(router stdhttp.Handler, method string, path string, body string, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

type accountLifecycleHTTPClient struct {
	getRequest     *userpb.UserIDRequest
	getResponse    *userpb.AccountLifecycleResponse
	getErr         error
	deleteRequest  *userpb.RequestAccountDeletionRequest
	deleteResponse *userpb.AccountLifecycleResponse
	deleteErr      error
}

func (c *accountLifecycleHTTPClient) GetAccountLifecycle(_ context.Context, req *userpb.UserIDRequest, _ ...grpc.CallOption) (*userpb.AccountLifecycleResponse, error) {
	c.getRequest = req
	return c.getResponse, c.getErr
}

func (c *accountLifecycleHTTPClient) RequestAccountDeletion(_ context.Context, req *userpb.RequestAccountDeletionRequest, _ ...grpc.CallOption) (*userpb.AccountLifecycleResponse, error) {
	c.deleteRequest = req
	return c.deleteResponse, c.deleteErr
}
