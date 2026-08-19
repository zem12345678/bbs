package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	largeCollectionID       int64 = 9007199254740993
	largeCollectionUserID   int64 = 9007199254740995
	largeCollectionItemID   int64 = 9007199254740997
	largeCollectionEntityID int64 = 9007199254740999
)

func TestCreateCollectionTrimsFieldsAndStringifiesBIGINTIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{createResp: &reactionpb.CollectionResponse{
		Success: true, Message: "ok",
		Collection: &reactionpb.CollectionInfo{
			Id: largeCollectionID, UserId: largeCollectionUserID, Name: "Research",
			Description: "Useful references", IsPublic: true, ItemCount: 3,
			CreatedAt: 1800000000000, UpdatedAt: 1800000001000,
		},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := collectionContext(http.MethodPost, "/api/v1/users/me/collections", `{"name":"  Research  ","description":"  Useful references  ","is_public":true}`)
	c.Set("user_id", largeCollectionUserID)

	h.createCurrentUserCollection(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, reaction.createReq)
	require.Equal(t, largeCollectionUserID, reaction.createReq.GetUserId())
	require.Equal(t, "Research", reaction.createReq.GetName())
	require.Equal(t, "Useful references", reaction.createReq.GetDescription())
	var envelope struct {
		Data struct {
			Collection collectionView `json:"collection"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9007199254740993", envelope.Data.Collection.ID)
	require.Equal(t, "9007199254740995", envelope.Data.Collection.UserID)
	require.Equal(t, int64(1800000000000), envelope.Data.Collection.CreatedAt)
}

func TestCreateAndUpdateCollectionValidateName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)

	createRecorder, createContext := collectionContext(http.MethodPost, "/api/v1/users/me/collections", `{"name":"   "}`)
	h.createCurrentUserCollection(createContext)
	require.Equal(t, http.StatusBadRequest, createRecorder.Code)
	require.Nil(t, reaction.createReq)

	updateRecorder, updateContext := collectionContext(http.MethodPut, "/api/v1/users/me/collections/9", `{"name":"   "}`)
	updateContext.Params = gin.Params{{Key: "id", Value: "9"}}
	h.updateCurrentUserCollection(updateContext)
	require.Equal(t, http.StatusBadRequest, updateRecorder.Code)
	require.Nil(t, reaction.updateReq)

	longRecorder, longContext := collectionContext(http.MethodPost, "/api/v1/users/me/collections", `{"name":"Saved","description":"`+strings.Repeat("界", 2049)+`"}`)
	h.createCurrentUserCollection(longContext)
	require.Equal(t, http.StatusBadRequest, longRecorder.Code)
}

func TestUpdateAndDeleteCollectionForwardOwnerAndStringPathID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{updateResp: &reactionpb.CollectionResponse{
		Success: true, Collection: &reactionpb.CollectionInfo{Id: largeCollectionID, UserId: 42, Name: "Updated"},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)

	updateRecorder, updateContext := collectionContext(http.MethodPut, "/api/v1/users/me/collections/9007199254740993", `{"name":"Updated","description":"desc","is_public":false}`)
	updateContext.Params = gin.Params{{Key: "id", Value: "9007199254740993"}}
	updateContext.Set("user_id", int64(42))
	h.updateCurrentUserCollection(updateContext)
	require.Equal(t, http.StatusOK, updateRecorder.Code, updateRecorder.Body.String())
	require.Equal(t, largeCollectionID, reaction.updateReq.GetId())
	require.Equal(t, int64(42), reaction.updateReq.GetUserId())

	deleteRecorder, deleteContext := collectionContext(http.MethodDelete, "/api/v1/users/me/collections/9007199254740993", "")
	deleteContext.Params = gin.Params{{Key: "id", Value: "9007199254740993"}}
	deleteContext.Set("user_id", int64(42))
	h.deleteCurrentUserCollection(deleteContext)
	require.Equal(t, http.StatusOK, deleteRecorder.Code, deleteRecorder.Body.String())
	require.Equal(t, largeCollectionID, reaction.deleteReq.GetId())
	require.Equal(t, int64(42), reaction.deleteReq.GetUserId())
}

func TestListCollectionsForwardsOwnerPaginationAndStringifiesIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{listResp: &reactionpb.ListCollectionsResponse{
		Total: 1, Items: []*reactionpb.CollectionInfo{{Id: largeCollectionID, UserId: largeCollectionUserID, Name: "Saved"}},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := collectionContext(http.MethodGet, "/api/v1/users/me/collections?limit=999&offset=-5", "")
	c.Set("user_id", largeCollectionUserID)

	h.listCurrentUserCollections(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, largeCollectionUserID, reaction.listReq.GetUserId())
	require.Equal(t, int32(100), reaction.listReq.GetLimit())
	require.Equal(t, int32(0), reaction.listReq.GetOffset())
	var envelope struct {
		Data struct {
			Items []collectionView `json:"items"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9007199254740993", envelope.Data.Items[0].ID)
}

func TestAddCollectionItemAcceptsStringEntityIDAndRequiresPublishedContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &fakeCommentTargetContentClient{article: &contentpb.ArticleInfo{Id: largeCollectionEntityID, Status: contentStatusPublished}}
	reaction := &collectionHTTPClient{}
	h := NewHandler(&clients.Clients{Content: content, Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := collectionContext(http.MethodPost, "/api/v1/users/me/collections/9007199254740993/items", `{"entity_type":"ARTICLE","entity_id":"9007199254740999"}`)
	c.Params = gin.Params{{Key: "id", Value: "9007199254740993"}}
	c.Set("user_id", int64(42))

	h.addCurrentUserCollectionItem(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, content.articleReq)
	require.NotNil(t, reaction.addReq)
	require.Equal(t, largeCollectionID, reaction.addReq.GetCollectionId())
	require.Equal(t, largeCollectionEntityID, reaction.addReq.GetEntity().GetEntityId())
	require.Equal(t, "article", reaction.addReq.GetEntity().GetEntityType())
}

func TestAddCollectionItemRejectsUnpublishedContentBeforeReactionRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := &fakeCommentTargetContentClient{topic: &contentpb.TopicInfo{Id: 77, Status: 1}}
	reaction := &collectionHTTPClient{}
	h := NewHandler(&clients.Clients{Content: content, Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := collectionContext(http.MethodPost, "/api/v1/users/me/collections/9/items", `{"entity_type":"topic","entity_id":77}`)
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	c.Set("user_id", int64(42))

	h.addCurrentUserCollectionItem(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Nil(t, reaction.addReq)
}

func TestRemoveCollectionItemDoesNotRequireContentLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := collectionContext(http.MethodDelete, "/api/v1/users/me/collections/9/items", `{"entity_type":"topic","entity_id":"77"}`)
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	c.Set("user_id", int64(42))

	h.removeCurrentUserCollectionItem(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, reaction.removeReq)
	require.Equal(t, int64(42), reaction.removeReq.GetUserId())
}

func TestListCollectionItemsForwardsOwnerAndStringifiesAllIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{itemsResp: &reactionpb.CollectionItemsResponse{
		Total: 1,
		Items: []*reactionpb.CollectionItemInfo{{
			Id: largeCollectionItemID, CollectionId: largeCollectionID,
			Entity: &reactionpb.EntityRef{EntityType: "article", EntityId: largeCollectionEntityID}, CreatedAt: 1800000000000,
		}},
	}}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := collectionContext(http.MethodGet, "/api/v1/users/me/collections/9007199254740993/items?entity_type=ARTICLE&limit=8&offset=4", "")
	c.Params = gin.Params{{Key: "id", Value: "9007199254740993"}}
	c.Set("user_id", int64(42))

	h.listCurrentUserCollectionItems(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), reaction.itemsReq.GetUserId())
	require.Equal(t, largeCollectionID, reaction.itemsReq.GetCollectionId())
	require.Equal(t, "article", reaction.itemsReq.GetEntityType())
	var envelope struct {
		Data struct {
			Items []collectionItemView `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "9007199254740997", envelope.Data.Items[0].ID)
	require.Equal(t, "9007199254740993", envelope.Data.Items[0].CollectionID)
	require.Equal(t, "9007199254740999", envelope.Data.Items[0].Entity.EntityID)
}

func TestCollectionItemValidationRejectsInvalidEntityTypeAndNormalizesPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)

	invalidRecorder, invalidContext := collectionContext(http.MethodPost, "/api/v1/users/me/collections/9/items", `{"entity_type":"comment","entity_id":7}`)
	invalidContext.Params = gin.Params{{Key: "id", Value: "9"}}
	h.addCurrentUserCollectionItem(invalidContext)
	require.Equal(t, http.StatusBadRequest, invalidRecorder.Code)
	require.Nil(t, reaction.addReq)

	listRecorder, listContext := collectionContext(http.MethodGet, "/api/v1/users/me/collections/9/items?limit=0&offset=-2", "")
	listContext.Params = gin.Params{{Key: "id", Value: "9"}}
	listContext.Set("user_id", int64(42))
	h.listCurrentUserCollectionItems(listContext)
	require.Equal(t, http.StatusOK, listRecorder.Code, listRecorder.Body.String())
	require.Equal(t, collectionDefaultLimit, reaction.itemsReq.GetLimit())
	require.Equal(t, int32(0), reaction.itemsReq.GetOffset())
}

func TestCollectionRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/users/me/collections", ""},
		{http.MethodPost, "/api/v1/users/me/collections", `{"name":"Saved"}`},
		{http.MethodPut, "/api/v1/users/me/collections/9", `{"name":"Saved"}`},
		{http.MethodDelete, "/api/v1/users/me/collections/9", ""},
		{http.MethodGet, "/api/v1/users/me/collections/9/items", ""},
		{http.MethodPost, "/api/v1/users/me/collections/9/items", `{"entity_type":"article","entity_id":7}`},
		{http.MethodDelete, "/api/v1/users/me/collections/9/items", `{"entity_type":"article","entity_id":7}`},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, tc.method+" "+tc.path+" "+recorder.Body.String())
	}
}

func TestCollectionListCannotChooseAnotherOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	token := signedAuthToken(t, jwt.MapClaims{"sub": "42", "username": "alice"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/collections?user_id=99", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), reaction.listReq.GetUserId())
}

func TestCollectionHandlerMapsBackendOwnershipDenial(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reaction := &collectionHTTPClient{err: status.Error(codes.NotFound, "collection not found")}
	h := NewHandler(&clients.Clients{Reaction: reaction}, "Authorization", "Bearer", testJWTSecret)
	recorder, c := collectionContext(http.MethodGet, "/api/v1/users/me/collections/9/items", "")
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	c.Set("user_id", int64(42))

	h.listCurrentUserCollectionItems(c)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
}

func collectionContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	return recorder, c
}

type collectionHTTPClient struct {
	reactionpb.ReactionServiceClient
	createReq  *reactionpb.CreateCollectionRequest
	updateReq  *reactionpb.UpdateCollectionRequest
	deleteReq  *reactionpb.DeleteCollectionRequest
	listReq    *reactionpb.ListCollectionsRequest
	addReq     *reactionpb.CollectionItemRequest
	removeReq  *reactionpb.CollectionItemRequest
	itemsReq   *reactionpb.ListCollectionItemsRequest
	createResp *reactionpb.CollectionResponse
	updateResp *reactionpb.CollectionResponse
	listResp   *reactionpb.ListCollectionsResponse
	itemsResp  *reactionpb.CollectionItemsResponse
	err        error
}

func (f *collectionHTTPClient) CreateCollection(_ context.Context, req *reactionpb.CreateCollectionRequest, _ ...grpc.CallOption) (*reactionpb.CollectionResponse, error) {
	f.createReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.createResp != nil {
		return f.createResp, nil
	}
	return &reactionpb.CollectionResponse{Success: true, Collection: &reactionpb.CollectionInfo{Id: 1, UserId: req.GetUserId(), Name: req.GetName()}}, nil
}

func (f *collectionHTTPClient) UpdateCollection(_ context.Context, req *reactionpb.UpdateCollectionRequest, _ ...grpc.CallOption) (*reactionpb.CollectionResponse, error) {
	f.updateReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.updateResp != nil {
		return f.updateResp, nil
	}
	return &reactionpb.CollectionResponse{Success: true, Collection: &reactionpb.CollectionInfo{Id: req.GetId(), UserId: req.GetUserId(), Name: req.GetName()}}, nil
}

func (f *collectionHTTPClient) DeleteCollection(_ context.Context, req *reactionpb.DeleteCollectionRequest, _ ...grpc.CallOption) (*reactionpb.CollectionActionResponse, error) {
	f.deleteReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &reactionpb.CollectionActionResponse{Success: true, Changed: true}, nil
}

func (f *collectionHTTPClient) ListCollections(_ context.Context, req *reactionpb.ListCollectionsRequest, _ ...grpc.CallOption) (*reactionpb.ListCollectionsResponse, error) {
	f.listReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.listResp != nil {
		return f.listResp, nil
	}
	return &reactionpb.ListCollectionsResponse{}, nil
}

func (f *collectionHTTPClient) AddCollectionItem(_ context.Context, req *reactionpb.CollectionItemRequest, _ ...grpc.CallOption) (*reactionpb.CollectionActionResponse, error) {
	f.addReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &reactionpb.CollectionActionResponse{Success: true, Changed: true}, nil
}

func (f *collectionHTTPClient) RemoveCollectionItem(_ context.Context, req *reactionpb.CollectionItemRequest, _ ...grpc.CallOption) (*reactionpb.CollectionActionResponse, error) {
	f.removeReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &reactionpb.CollectionActionResponse{Success: true, Changed: true}, nil
}

func (f *collectionHTTPClient) ListCollectionItems(_ context.Context, req *reactionpb.ListCollectionItemsRequest, _ ...grpc.CallOption) (*reactionpb.CollectionItemsResponse, error) {
	f.itemsReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.itemsResp != nil {
		return f.itemsResp, nil
	}
	return &reactionpb.CollectionItemsResponse{}, nil
}
