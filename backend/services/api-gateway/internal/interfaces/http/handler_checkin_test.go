package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/creditpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestCheckInRoutesUseAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	creditClient := &fakeCheckInCreditClient{}
	h := NewHandler(&clients.Clients{Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "member"})

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/credits/check-in", nil)
	statusRequest.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(statusRecorder, statusRequest)

	require.Equal(t, http.StatusOK, statusRecorder.Code, statusRecorder.Body.String())
	require.NotNil(t, creditClient.statusReq)
	require.EqualValues(t, 42, creditClient.statusReq.GetUserId())
	var statusEnvelope struct {
		Data creditpb.CheckInStatusResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(statusRecorder.Body.Bytes(), &statusEnvelope))
	require.False(t, statusEnvelope.Data.GetCheckedIn())
	require.EqualValues(t, 5, statusEnvelope.Data.GetRewardCredits())

	checkInRecorder := httptest.NewRecorder()
	checkInRequest := httptest.NewRequest(http.MethodPost, "/api/v1/credits/check-in", nil)
	checkInRequest.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(checkInRecorder, checkInRequest)

	require.Equal(t, http.StatusOK, checkInRecorder.Code, checkInRecorder.Body.String())
	require.NotNil(t, creditClient.checkInReq)
	require.EqualValues(t, 42, creditClient.checkInReq.GetUserId())
	var checkInEnvelope struct {
		Data creditpb.CheckInResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(checkInRecorder.Body.Bytes(), &checkInEnvelope))
	require.Equal(t, "2026-07-19", checkInEnvelope.Data.GetCheckIn().GetLatestDay())
	require.EqualValues(t, 5, checkInEnvelope.Data.GetLedger().GetDelta())
}

func TestCheckInRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	creditClient := &fakeCheckInCreditClient{}
	h := NewHandler(&clients.Clients{Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/credits/check-in", nil))

	require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	require.Nil(t, creditClient.checkInReq)
}

type fakeCheckInCreditClient struct {
	creditpb.CreditServiceClient
	statusReq  *creditpb.GetCheckInStatusRequest
	checkInReq *creditpb.CheckInRequest
}

func (f *fakeCheckInCreditClient) GetCheckInStatus(_ context.Context, req *creditpb.GetCheckInStatusRequest, _ ...grpc.CallOption) (*creditpb.CheckInStatusResponse, error) {
	f.statusReq = req
	return &creditpb.CheckInStatusResponse{
		CheckIn:       &creditpb.DailyCheckIn{UserId: req.GetUserId(), LatestDay: "2026-07-18", ConsecutiveDays: 3},
		CheckedIn:     false,
		RewardCredits: 5,
	}, nil
}

func (f *fakeCheckInCreditClient) CheckIn(_ context.Context, req *creditpb.CheckInRequest, _ ...grpc.CallOption) (*creditpb.CheckInResponse, error) {
	f.checkInReq = req
	return &creditpb.CheckInResponse{
		CheckIn:       &creditpb.DailyCheckIn{UserId: req.GetUserId(), LatestDay: "2026-07-19", ConsecutiveDays: 4},
		Balance:       &creditpb.Balance{UserId: req.GetUserId(), Total: 105},
		Ledger:        &creditpb.LedgerEntry{UserId: req.GetUserId(), Delta: 5, Reason: "daily_check_in"},
		RewardCredits: 5,
	}, nil
}
