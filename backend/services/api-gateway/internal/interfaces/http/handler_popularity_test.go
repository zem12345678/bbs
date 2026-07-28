package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/chatpb"
	"api-gateway/internal/clients"
	"api-gateway/internal/popularity"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListPopularChatRoomsReturnsRedisRankedSafePreviews(t *testing.T) {
	gin.SetMode(gin.TestMode)
	popularityStore := &fakePopularityStore{
		chatEntries: []popularity.Entry{{Key: "ABCD1234", Score: 3}},
	}
	chatClient := &chatHTTPClient{lookupRoomResponse: &chatpb.RoomDetailsResponse{Details: &chatpb.RoomDetails{
		Room:        &chatpb.Room{RoomNo: "ABCD1234", Name: "Lounge", Status: 1},
		MemberCount: 12,
	}}}
	h := NewHandler(&clients.Clients{Chat: chatClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetPopularityStore(popularityStore)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/chat/popular?limit=5", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Items []popularChatRoomView `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, []popularChatRoomView{{
		RoomNo: "ABCD1234", Name: "Lounge", MemberCount: "12", Score: "3",
	}}, envelope.Data.Items)
}

func TestRecordLinkVisitWritesResourcePopularity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	popularityStore := &fakePopularityStore{}
	h := NewHandler(nil, "Authorization", "Bearer", testJWTSecret)
	h.SetPopularityStore(popularityStore)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/links/42/visit", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{42}, popularityStore.resourceRecords)
}

func TestListPopularResourcesUsesRedisOrderAndActiveLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	popularityStore := &fakePopularityStore{
		resourceEntries: []popularity.Entry{{Key: "2", Score: 5}, {Key: "1", Score: 3}, {Key: "3", Score: 1}},
	}
	adminClient := &popularAdminClient{links: []*adminpb.LinkInfo{
		{Id: 1, Key: "docs", Title: "Docs", Url: "https://docs.example.test", Description: "Docs", Status: 2},
		{Id: 2, Key: "tool", Title: "Tool", Url: "https://tool.example.test", Description: "Tool", Status: 2},
		{Id: 3, Key: "disabled", Title: "Disabled", Url: "https://disabled.example.test", Status: 1},
	}}
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetPopularityStore(popularityStore)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/links/popular?limit=5", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Items []popularResourceView `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, []popularResourceView{
		{ID: 2, Key: "tool", Title: "Tool", URL: "https://tool.example.test", Description: "Tool", Score: "5"},
		{ID: 1, Key: "docs", Title: "Docs", URL: "https://docs.example.test", Description: "Docs", Score: "3"},
	}, envelope.Data.Items)
}

func TestListPopularResourcesFindsRankedLinkBeyondFirstPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	popularityStore := &fakePopularityStore{
		resourceEntries: []popularity.Entry{{Key: "150", Score: 7}},
	}
	adminClient := &popularAdminClient{}
	for id := int64(1); id <= 100; id++ {
		adminClient.links = append(adminClient.links, &adminpb.LinkInfo{
			Id: id, Key: "filler", Title: "Filler", Url: "https://filler.example.test", Status: 2,
		})
	}
	adminClient.links = append(adminClient.links, &adminpb.LinkInfo{
		Id: 150, Key: "deep", Title: "Deep Link", Url: "https://deep.example.test", Description: "Paged", Status: 2,
	})
	h := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetPopularityStore(popularityStore)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/links/popular?limit=5", nil))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var envelope struct {
		Data struct {
			Items []popularResourceView `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, []popularResourceView{
		{ID: 150, Key: "deep", Title: "Deep Link", URL: "https://deep.example.test", Description: "Paged", Score: "7"},
	}, envelope.Data.Items)
	require.Len(t, adminClient.requests, 2)
	require.Equal(t, int32(0), adminClient.requests[0].GetOffset())
	require.Equal(t, int32(100), adminClient.requests[1].GetOffset())
}

type fakePopularityStore struct {
	chatRecords     []string
	resourceRecords []int64
	chatEntries     []popularity.Entry
	resourceEntries []popularity.Entry
}

func (s *fakePopularityStore) RecordChatRoomActivity(_ context.Context, roomNo string) error {
	s.chatRecords = append(s.chatRecords, roomNo)
	return nil
}

func (s *fakePopularityStore) ListChatRooms(context.Context, int) ([]popularity.Entry, error) {
	return s.chatEntries, nil
}

func (s *fakePopularityStore) RecordResourceVisit(_ context.Context, linkID int64) error {
	s.resourceRecords = append(s.resourceRecords, linkID)
	return nil
}

func (s *fakePopularityStore) ListResources(context.Context, int) ([]popularity.Entry, error) {
	return s.resourceEntries, nil
}

type popularAdminClient struct {
	adminpb.AdminServiceClient
	links    []*adminpb.LinkInfo
	requests []*adminpb.ListLinksRequest
}

func (c *popularAdminClient) ListLinks(_ context.Context, req *adminpb.ListLinksRequest, _ ...grpc.CallOption) (*adminpb.LinkListResponse, error) {
	c.requests = append(c.requests, req)
	filtered := make([]*adminpb.LinkInfo, 0, len(c.links))
	for _, link := range c.links {
		if link != nil && (req.GetStatus() <= 0 || link.GetStatus() == req.GetStatus()) {
			filtered = append(filtered, link)
		}
	}
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := int(req.GetOffset())
	if offset < 0 {
		offset = 0
	}
	if offset >= len(filtered) {
		return &adminpb.LinkListResponse{Items: []*adminpb.LinkInfo{}, Total: int64(len(filtered))}, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return &adminpb.LinkListResponse{Items: filtered[offset:end], Total: int64(len(filtered))}, nil
}
