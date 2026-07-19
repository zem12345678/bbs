package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/creditpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestTaskRoutesExposeOnlySupportedTasksAndUseServerConfiguredReward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeTaskAdminClient{items: []*adminpb.TaskInfo{
		{Id: 8, Key: taskKeyDailyCheckIn, Title: "每日签到", Description: "完成签到后领取奖励", RewardPoints: 12, Status: 2},
		{Id: 9, Key: taskKeyFirstTopic, Title: "发布第一条话题", Description: "发布后领取奖励", RewardPoints: 20, Status: 2},
		{Id: 10, Key: taskKeyFirstComment, Title: "完成一次评论", Description: "评论后领取奖励", RewardPoints: 10, Status: 2},
		{Id: 11, Key: "first-comment", Title: "旧版首评任务", RewardPoints: 99999, Status: 2},
	}}
	creditClient := &fakeTaskCreditClient{}
	h := NewHandler(&clients.Clients{Admin: adminClient, Credit: creditClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "member"})

	publicRecorder := httptest.NewRecorder()
	router.ServeHTTP(publicRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
	require.Equal(t, http.StatusOK, publicRecorder.Code, publicRecorder.Body.String())
	var publicEnvelope struct {
		Data struct {
			Items []taskView `json:"items"`
			Total int        `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(publicRecorder.Body.Bytes(), &publicEnvelope))
	require.Len(t, publicEnvelope.Data.Items, 3)
	require.Equal(t, []string{taskKeyDailyCheckIn, taskKeyFirstTopic, taskKeyFirstComment}, []string{publicEnvelope.Data.Items[0].Key, publicEnvelope.Data.Items[1].Key, publicEnvelope.Data.Items[2].Key})
	require.Equal(t, 3, publicEnvelope.Data.Total)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/me", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(listRecorder, listRequest)
	require.Equal(t, http.StatusOK, listRecorder.Code, listRecorder.Body.String())
	require.NotNil(t, adminClient.lastListRequest())
	require.Nil(t, adminClient.lastListRequest().GetActor())
	require.EqualValues(t, 2, adminClient.lastListRequest().GetStatus())
	require.EqualValues(t, 100, adminClient.lastListRequest().GetLimit())
	require.Len(t, creditClient.statusRequests, 3)
	require.EqualValues(t, 42, creditClient.statusRequests[0].GetUserId())
	require.EqualValues(t, 8, creditClient.statusRequests[0].GetTaskId())
	require.Equal(t, taskKeyDailyCheckIn, creditClient.statusRequests[0].GetTaskKey())
	require.EqualValues(t, 42, creditClient.statusRequests[1].GetUserId())
	require.EqualValues(t, 9, creditClient.statusRequests[1].GetTaskId())
	require.Equal(t, taskKeyFirstTopic, creditClient.statusRequests[1].GetTaskKey())
	require.EqualValues(t, 42, creditClient.statusRequests[2].GetUserId())
	require.EqualValues(t, 10, creditClient.statusRequests[2].GetTaskId())
	require.Equal(t, taskKeyFirstComment, creditClient.statusRequests[2].GetTaskKey())

	claimRecorder := httptest.NewRecorder()
	claimRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/9/claim", strings.NewReader(`{"reward_credits":99999,"task_key":"daily_check_in"}`))
	claimRequest.Header.Set("Content-Type", "application/json")
	claimRequest.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(claimRecorder, claimRequest)
	require.Equal(t, http.StatusOK, claimRecorder.Code, claimRecorder.Body.String())
	require.Len(t, creditClient.claimRequests, 1)
	require.EqualValues(t, 42, creditClient.claimRequests[0].GetUserId())
	require.EqualValues(t, 9, creditClient.claimRequests[0].GetTaskId())
	require.Equal(t, taskKeyFirstTopic, creditClient.claimRequests[0].GetTaskKey())
	require.EqualValues(t, 20, creditClient.claimRequests[0].GetRewardCredits())
	require.Equal(t, "发布第一条话题", creditClient.claimRequests[0].GetTaskTitle())

	unsupportedRecorder := httptest.NewRecorder()
	unsupportedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/11/claim", nil)
	unsupportedRequest.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(unsupportedRecorder, unsupportedRequest)
	require.Equal(t, http.StatusNotFound, unsupportedRecorder.Code, unsupportedRecorder.Body.String())
	require.Len(t, creditClient.claimRequests, 1)
}

func TestTaskClaimRouteRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{Admin: &fakeTaskAdminClient{}, Credit: &fakeTaskCreditClient{}}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/8/claim", nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
}

type fakeTaskAdminClient struct {
	adminpb.AdminServiceClient
	items    []*adminpb.TaskInfo
	requests []*adminpb.ListTasksRequest
}

func (f *fakeTaskAdminClient) ListTasks(_ context.Context, req *adminpb.ListTasksRequest, _ ...grpc.CallOption) (*adminpb.TaskListResponse, error) {
	f.requests = append(f.requests, req)
	return &adminpb.TaskListResponse{Items: f.items, Total: int64(len(f.items))}, nil
}

func (f *fakeTaskAdminClient) lastListRequest() *adminpb.ListTasksRequest {
	if len(f.requests) == 0 {
		return nil
	}
	return f.requests[len(f.requests)-1]
}

type fakeTaskCreditClient struct {
	creditpb.CreditServiceClient
	statusRequests []*creditpb.GetTaskClaimStatusRequest
	claimRequests  []*creditpb.ClaimTaskRequest
}

func (f *fakeTaskCreditClient) GetTaskClaimStatus(_ context.Context, req *creditpb.GetTaskClaimStatusRequest, _ ...grpc.CallOption) (*creditpb.TaskClaimStatusResponse, error) {
	f.statusRequests = append(f.statusRequests, req)
	return &creditpb.TaskClaimStatusResponse{Status: &creditpb.TaskClaimStatus{
		TaskId:    req.GetTaskId(),
		TaskKey:   req.GetTaskKey(),
		Cycle:     "2026-07-20",
		Completed: true,
		Claimed:   false,
	}}, nil
}

func (f *fakeTaskCreditClient) ClaimTask(_ context.Context, req *creditpb.ClaimTaskRequest, _ ...grpc.CallOption) (*creditpb.ClaimTaskResponse, error) {
	f.claimRequests = append(f.claimRequests, req)
	return &creditpb.ClaimTaskResponse{
		Status: &creditpb.TaskClaimStatus{
			TaskId:    req.GetTaskId(),
			TaskKey:   req.GetTaskKey(),
			Cycle:     "2026-07-20",
			Completed: true,
			Claimed:   true,
		},
		Balance: &creditpb.Balance{UserId: req.GetUserId(), Total: 117},
		Ledger:  &creditpb.LedgerEntry{UserId: req.GetUserId(), Delta: req.GetRewardCredits(), Reason: "daily_check_in_task"},
	}, nil
}
