package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeAntennaClient struct {
	userpb.UserServiceClient
	antenna       *userpb.AntennaInfo
	updateRequest *userpb.UpdateAntennaRequest
}

func (f *fakeAntennaClient) ListAntennas(context.Context, *userpb.ListAntennasRequest, ...grpc.CallOption) (*userpb.AntennaListResponse, error) {
	return &userpb.AntennaListResponse{Items: []*userpb.AntennaInfo{f.antenna}, Total: 1}, nil
}
func (f *fakeAntennaClient) CreateAntenna(context.Context, *userpb.CreateAntennaRequest, ...grpc.CallOption) (*userpb.AntennaInfoResponse, error) {
	return &userpb.AntennaInfoResponse{Antenna: f.antenna}, nil
}

func (f *fakeAntennaClient) UpdateAntenna(_ context.Context, request *userpb.UpdateAntennaRequest, _ ...grpc.CallOption) (*userpb.AntennaInfoResponse, error) {
	f.updateRequest = request
	return &userpb.AntennaInfoResponse{Antenna: f.antenna}, nil
}
func (f *fakeAntennaClient) DeleteAntenna(context.Context, *userpb.DeleteAntennaRequest, ...grpc.CallOption) (*userpb.SimpleResponse, error) {
	return &userpb.SimpleResponse{Success: true}, nil
}
func (f *fakeAntennaClient) GetAntenna(context.Context, *userpb.GetAntennaRequest, ...grpc.CallOption) (*userpb.AntennaInfoResponse, error) {
	return &userpb.AntennaInfoResponse{Antenna: f.antenna}, nil
}

type fakeFilteredFeedClient struct {
	feedpb.FeedServiceClient
	request *feedpb.FilteredFeedRequest
}

type fakeAntennaUsernameClient struct {
	userpb.UserServiceClient
	err error
}

func (f *fakeAntennaUsernameClient) GetUserByUsername(context.Context, *userpb.UsernameRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	return nil, f.err
}

type fakeAntennaSafetyClient struct {
	userpb.UserServiceClient
	err error
}

func (f *fakeAntennaSafetyClient) ListBlockedUsers(context.Context, *userpb.ListUserRelationsRequest, ...grpc.CallOption) (*userpb.UserListResponse, error) {
	return nil, f.err
}

func (f *fakeFilteredFeedClient) ListFiltered(_ context.Context, request *feedpb.FilteredFeedRequest, _ ...grpc.CallOption) (*feedpb.FeedListResponse, error) {
	f.request = request
	return &feedpb.FeedListResponse{Items: []*feedpb.FeedItem{{Id: 41, Title: "matched"}}}, nil
}

func TestListAntennasReturnsCanonicalEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&clients.Clients{UserAntennas: &fakeAntennaClient{antenna: &userpb.AntennaInfo{Id: 41, OwnerId: 7, Name: "Backend", Source: "all", CreatedAt: 1720000000000, IsActive: true}}}, "Authorization", "Bearer", testJWTSecret)
	c := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(c)
	ctx.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/antennas", nil)
	ctx.Set("user_id", int64(7))
	h.listAntennas(ctx)
	require.Equal(t, stdhttp.StatusOK, c.Code)
	var envelope struct {
		Data struct {
			Items []antennaPayload `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(c.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "41", envelope.Data.Items[0].ID)
	require.Equal(t, "2024-07-03T09:46:40Z", envelope.Data.Items[0].CreatedAt)
}

func TestUpdateAntennaDistinguishesMissingAndNullUserListID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &fakeAntennaClient{antenna: &userpb.AntennaInfo{Id: 41, OwnerId: 7, Name: "Backend", Source: "list", UserListId: 99, Keywords: []*userpb.AntennaKeywordGroup{{Terms: []string{"go"}}}}}
	h := NewHandler(&clients.Clients{UserAntennas: client}, "Authorization", "Bearer", testJWTSecret)

	for _, testCase := range []struct {
		name       string
		body       string
		expectedID int64
	}{
		{name: "missing keeps current value", body: `{"antennaId":"41","name":"Updated"}`, expectedID: 99},
		{name: "null clears current value", body: `{"antennaId":"41","src":"all","userListId":null}`, expectedID: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(stdhttp.MethodPost, "/antennas/update", strings.NewReader(testCase.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set("user_id", int64(7))

			h.updateAntenna(ctx)

			require.Equal(t, stdhttp.StatusOK, recorder.Code)
			require.NotNil(t, client.updateRequest)
			require.Equal(t, testCase.expectedID, client.updateRequest.GetUserListId())
		})
	}
}

func TestAntennaNotesPassesConfiguredFilterToFeed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	feed := &fakeFilteredFeedClient{}
	h := NewHandler(&clients.Clients{UserAntennas: &fakeAntennaClient{antenna: &userpb.AntennaInfo{Id: 41, OwnerId: 7, Name: "Backend", Source: "all", Keywords: []*userpb.AntennaKeywordGroup{{Terms: []string{"go"}}}}}, FeedFiltered: feed}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/antennas/41/notes?limit=5", nil)
	ctx.Set("user_id", int64(7))
	ctx.Params = gin.Params{{Key: "antennaId", Value: "41"}}
	h.antennaNotes(ctx)
	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.NotNil(t, feed.request)
	require.Equal(t, int32(5), feed.request.GetLimit())
	require.Equal(t, []string{"go"}, feed.request.GetKeywords()[0].GetTerms())
}

func TestAntennaNotesRestrictsEmptyUserSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	feed := &fakeFilteredFeedClient{}
	h := NewHandler(&clients.Clients{
		User:         &fakeAntennaUsernameClient{err: status.Error(codes.NotFound, "user not found")},
		UserAntennas: &fakeAntennaClient{antenna: &userpb.AntennaInfo{Id: 41, OwnerId: 7, Source: "users", Users: []string{"missing"}}},
		FeedFiltered: feed,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/antennas/41/notes", nil)
	ctx.Set("user_id", int64(7))
	ctx.Params = gin.Params{{Key: "antennaId", Value: "41"}}

	h.antennaNotes(ctx)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.NotNil(t, feed.request)
	require.True(t, feed.request.GetRestrictAuthors())
	require.Empty(t, feed.request.GetAuthorIds())
}

func TestAntennaNotesFailsClosedWhenUsernameLookupIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	feed := &fakeFilteredFeedClient{}
	h := NewHandler(&clients.Clients{
		User:         &fakeAntennaUsernameClient{err: status.Error(codes.Unavailable, "user service unavailable")},
		UserAntennas: &fakeAntennaClient{antenna: &userpb.AntennaInfo{Id: 41, OwnerId: 7, Source: "users", Users: []string{"alice"}}},
		FeedFiltered: feed,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/antennas/41/notes", nil)
	ctx.Set("user_id", int64(7))
	ctx.Params = gin.Params{{Key: "antennaId", Value: "41"}}

	h.antennaNotes(ctx)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code)
	require.Nil(t, feed.request)
}

func TestAntennaNotesFailsClosedWhenHiddenUsersCannotBeLoaded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	feed := &fakeFilteredFeedClient{}
	h := NewHandler(&clients.Clients{
		UserSafety:   &fakeAntennaSafetyClient{err: status.Error(codes.Unavailable, "user service unavailable")},
		UserAntennas: &fakeAntennaClient{antenna: &userpb.AntennaInfo{Id: 41, OwnerId: 7, Source: "all"}},
		FeedFiltered: feed,
	}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/users/me/antennas/41/notes", nil)
	ctx.Set("user_id", int64(7))
	ctx.Params = gin.Params{{Key: "antennaId", Value: "41"}}

	h.antennaNotes(ctx)

	require.Equal(t, stdhttp.StatusServiceUnavailable, recorder.Code)
	require.Nil(t, feed.request)
}
