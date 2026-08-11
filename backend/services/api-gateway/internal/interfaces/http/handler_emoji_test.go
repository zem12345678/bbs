package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPublicEmojiCompatibilityRoutesReturnSimpleAndDetailedContracts(t *testing.T) {
	client := &emojiHandlerAdminClient{item: emojiTestInfo()}
	router := emojiHandlerTestRouter(client)

	for _, path := range []string{"/emojis", "/api/emojis"} {
		list := httptest.NewRecorder()
		router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, list.Code, list.Body.String())
		var listPayload struct {
			Emojis []map[string]any `json:"emojis"`
		}
		require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listPayload))
		require.Len(t, listPayload.Emojis, 1)
		require.Equal(t, "party", listPayload.Emojis[0]["name"])
		require.NotContains(t, listPayload.Emojis[0], "id")
		require.NotContains(t, list.Body.String(), `"data"`)
	}

	show := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/emoji", strings.NewReader(`{"name":"party"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(show, request)
	require.Equal(t, http.StatusOK, show.Code, show.Body.String())
	var emoji struct {
		ID   string  `json:"id"`
		Host *string `json:"host"`
	}
	require.NoError(t, json.Unmarshal(show.Body.Bytes(), &emoji))
	require.Equal(t, "42", emoji.ID)
	require.Nil(t, emoji.Host)
	require.NotContains(t, show.Body.String(), `"data"`)
	require.Equal(t, "party", client.getRequest.GetName())

	canonical := httptest.NewRecorder()
	router.ServeHTTP(canonical, httptest.NewRequest(http.MethodGet, "/api/v1/emojis", nil))
	require.Equal(t, http.StatusOK, canonical.Code, canonical.Body.String())
	var canonicalPayload struct {
		Data struct {
			Emojis []map[string]any `json:"emojis"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(canonical.Body.Bytes(), &canonicalPayload))
	require.Len(t, canonicalPayload.Data.Emojis, 1)
}

func TestPublicEmojiCompatibilityListLoadsEveryPage(t *testing.T) {
	items := make([]*adminpb.EmojiInfo, 1001)
	for index := range items {
		items[index] = &adminpb.EmojiInfo{Id: int64(index + 1), Name: "emoji" + strconv.Itoa(index+1), Url: "http://example.test/emoji.png"}
	}
	client := &emojiHandlerAdminClient{items: items}
	router := emojiHandlerTestRouter(client)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/emojis", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var payload struct {
		Emojis []map[string]any `json:"emojis"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Emojis, 1001)
	require.Len(t, client.listRequests, 2)
	require.Equal(t, int32(1000), client.listRequests[1].GetOffset())
}

func TestAdminEmojiCreateMapsUploadedFileID(t *testing.T) {
	client := &emojiHandlerAdminClient{permissions: []string{"governance:create_emoji"}, item: emojiTestInfo()}
	router := emojiHandlerTestRouter(client)

	for _, path := range []string{"/admin/emoji/add", "/api/admin/emoji/add", "/api/v1/admin/emoji/add"} {
		recorder := emojiAdminRequest(router, http.MethodPost, path, `{"name":"party","fileId":"abc.webp","aliases":["celebrate"],"isSensitive":true}`)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var payload emojiView
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		require.Equal(t, "42", payload.ID)
		require.NotContains(t, recorder.Body.String(), `"data"`)
	}
	require.Equal(t, "party", client.createRequest.GetName())
	require.Equal(t, "http://example.test/uploads/emojis/abc.webp", client.createRequest.GetUrl())
	require.Equal(t, "image/webp", client.createRequest.GetContentType())
	require.Equal(t, []string{"celebrate"}, client.createRequest.GetAliases())
	require.True(t, client.createRequest.GetIsSensitive())

	canonical := emojiAdminRequest(router, http.MethodPost, "/api/v1/admin/emojis", `{"name":"party","fileId":"abc.webp"}`)
	require.Equal(t, http.StatusOK, canonical.Code, canonical.Body.String())
	var canonicalPayload struct {
		Data emojiView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(canonical.Body.Bytes(), &canonicalPayload))
	require.Equal(t, "42", canonicalPayload.Data.ID)
}

func TestAdminEmojiUpdatePreservesPatchPresence(t *testing.T) {
	client := &emojiHandlerAdminClient{permissions: []string{"governance:update_emoji"}, item: emojiTestInfo()}
	router := emojiHandlerTestRouter(client)

	recorder := emojiAdminRequest(router, http.MethodPost, "/api/v1/admin/emoji/update", `{"id":"42","category":null,"license":null,"aliases":[],"localOnly":false,"roleIdsThatCanBeUsedThisEmojiAsReaction":[]}`)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), client.updateRequest.GetId())
	require.Nil(t, client.updateRequest.Name)
	require.True(t, client.updateRequest.GetClearCategory())
	require.True(t, client.updateRequest.GetClearLicense())
	require.NotNil(t, client.updateRequest.GetAliases())
	require.Empty(t, client.updateRequest.GetAliases().GetItems())
	require.NotNil(t, client.updateRequest.LocalOnly)
	require.False(t, client.updateRequest.GetLocalOnly())
	require.NotNil(t, client.updateRequest.GetRoleIdsThatCanBeUsedThisEmojiAsReaction())
}

func TestAdminEmojiCompatUpdateCanSelectByName(t *testing.T) {
	client := &emojiHandlerAdminClient{permissions: []string{"governance:update_emoji"}, item: emojiTestInfo()}
	router := emojiHandlerTestRouter(client)

	recorder := emojiAdminRequest(router, http.MethodPost, "/admin/emoji/update", `{"name":"party","license":"CC0"}`)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Equal(t, "party", client.getRequest.GetName())
	require.Equal(t, int64(42), client.updateRequest.GetId())
	require.Equal(t, "CC0", client.updateRequest.GetLicense())
}

func TestAdminEmojiBulkAliasesAndNullableFields(t *testing.T) {
	item := emojiTestInfo()
	item.Aliases = []string{"celebrate"}
	client := &emojiHandlerAdminClient{permissions: []string{"governance:update_emoji", "governance:delete_emoji"}, item: item}
	router := emojiHandlerTestRouter(client)

	add := emojiAdminRequest(router, http.MethodPost, "/api/admin/emoji/add-aliases-bulk", `{"ids":["42"],"aliases":["party-time"]}`)
	require.Equal(t, http.StatusNoContent, add.Code, add.Body.String())
	require.Equal(t, []string{"celebrate", "party-time"}, client.updateRequest.GetAliases().GetItems())

	category := emojiAdminRequest(router, http.MethodPost, "/admin/emoji/set-category-bulk", `{"ids":["42"],"category":null}`)
	require.Equal(t, http.StatusNoContent, category.Code, category.Body.String())
	require.True(t, client.updateRequest.GetClearCategory())

	remove := emojiAdminRequest(router, http.MethodPost, "/admin/emoji/remove-aliases-bulk", `{"ids":["42"],"aliases":["celebrate"]}`)
	require.Equal(t, http.StatusNoContent, remove.Code, remove.Body.String())
	require.Empty(t, client.updateRequest.GetAliases().GetItems())

	set := emojiAdminRequest(router, http.MethodPost, "/api/admin/emoji/set-aliases-bulk", `{"ids":["42"],"aliases":["fresh"]}`)
	require.Equal(t, http.StatusNoContent, set.Code, set.Body.String())
	require.Equal(t, []string{"fresh"}, client.updateRequest.GetAliases().GetItems())

	license := emojiAdminRequest(router, http.MethodPost, "/admin/emoji/set-license-bulk", `{"ids":["42"],"license":null}`)
	require.Equal(t, http.StatusNoContent, license.Code, license.Body.String())
	require.True(t, client.updateRequest.GetClearLicense())

	deleted := emojiAdminRequest(router, http.MethodPost, "/admin/emoji/delete-bulk", `{"ids":["42"]}`)
	require.Equal(t, http.StatusNoContent, deleted.Code, deleted.Body.String())
	require.Equal(t, int64(42), client.deleteRequest.GetId())
}

func TestAdminEmojiV2RouteFiltersSortsAndUsesOneBasedPages(t *testing.T) {
	category := "memes"
	items := []*adminpb.EmojiInfo{
		{Id: 41, Name: "zeta", Url: "http://example.test/zeta.webp", OriginalUrl: "http://example.test/zeta.webp", Category: &category, LocalOnly: true, UpdatedAt: 1700000000000},
		{Id: 42, Name: "alpha", Url: "http://example.test/alpha.webp", OriginalUrl: "http://example.test/alpha.webp", Category: &category, LocalOnly: true, UpdatedAt: 1700000001000, RoleIdsThatCanBeUsedThisEmojiAsReaction: []string{"7"}},
		{Id: 43, Name: "other", Url: "http://example.test/other.webp", OriginalUrl: "http://example.test/other.webp", LocalOnly: false, UpdatedAt: 1700000002000},
	}
	client := &emojiHandlerAdminClient{permissions: []string{"governance:list_emojis"}, items: items, roles: []*adminpb.RoleInfo{{Id: 7, Name: "Moderators"}}}
	router := emojiHandlerTestRouter(client)

	recorder := emojiAdminRequest(router, http.MethodPost, "/api/v2/admin/emoji/list", `{"query":{"category":"MEMES","localOnly":true},"limit":1,"page":1,"sortKeys":["+name"]}`)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var payload struct {
		Emojis   []emojiV2View `json:"emojis"`
		Count    int           `json:"count"`
		AllCount int           `json:"allCount"`
		AllPages int           `json:"allPages"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, 1, payload.Count)
	require.Equal(t, 2, payload.AllCount)
	require.Equal(t, 2, payload.AllPages)
	require.Equal(t, "alpha", payload.Emojis[0].Name)
	require.Equal(t, []emojiReactionRoleView{{ID: "7", Name: "Moderators"}}, payload.Emojis[0].RoleIDsThatCanBeUsedThisEmojiAsReaction)
	require.NotContains(t, recorder.Body.String(), `"data"`)
	require.NotNil(t, client.listRequest.GetActor())

	invalid := emojiAdminRequest(router, http.MethodPost, "/api/v2/admin/emoji/list", `{"sortKeys":["name"]}`)
	require.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())
}

func TestAdminEmojiV2RoleNameLookupDoesNotRequireRolePermission(t *testing.T) {
	item := emojiTestInfo()
	item.RoleIdsThatCanBeUsedThisEmojiAsReaction = []string{"7"}
	client := &emojiHandlerAdminClient{
		permissions:  []string{"governance:list_emojis"},
		item:         item,
		listRolesErr: status.Error(codes.PermissionDenied, "role permission denied"),
	}
	router := emojiHandlerTestRouter(client)

	recorder := emojiAdminRequest(router, http.MethodPost, "/api/v2/admin/emoji/list", `{}`)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var payload struct {
		Emojis []emojiV2View `json:"emojis"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Emojis, 1)
	require.Equal(t, []emojiReactionRoleView{{ID: "7", Name: ""}}, payload.Emojis[0].RoleIDsThatCanBeUsedThisEmojiAsReaction)
}

func TestAdminEmojiCompatibilityListUsesCursorAndRawArray(t *testing.T) {
	items := []*adminpb.EmojiInfo{
		{Id: 41, Name: "one", Url: "http://example.test/one.png"},
		{Id: 42, Name: "two", Url: "http://example.test/two.png"},
		{Id: 43, Name: "three", Url: "http://example.test/three.png"},
		{Id: 44, Name: "four", Url: "http://example.test/four.png"},
	}
	client := &emojiHandlerAdminClient{permissions: []string{"governance:list_emojis"}, items: items}
	router := emojiHandlerTestRouter(client)

	for _, path := range []string{"/admin/emoji/list", "/api/admin/emoji/list", "/api/v1/admin/emoji/list"} {
		recorder := emojiAdminRequest(router, http.MethodPost, path, `{"untilId":"44","limit":2}`)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var payload []emojiView
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		require.Equal(t, []string{"43", "42"}, []string{payload[0].ID, payload[1].ID})
		require.True(t, strings.HasPrefix(strings.TrimSpace(recorder.Body.String()), "["))
	}

	since := emojiAdminRequest(router, http.MethodPost, "/admin/emoji/list", `{"sinceId":"41","limit":2}`)
	require.Equal(t, http.StatusOK, since.Code, since.Body.String())
	var sincePayload []emojiView
	require.NoError(t, json.Unmarshal(since.Body.Bytes(), &sincePayload))
	require.Equal(t, []string{"42", "43"}, []string{sincePayload[0].ID, sincePayload[1].ID})
}

func TestAdminEmojiUploadAllowsUpdateOnlyPermission(t *testing.T) {
	client := &emojiHandlerAdminClient{permissions: []string{"governance:update_emoji"}}
	store := &fakeImageStore{objects: make(map[string]fakeImageObject)}
	router := emojiHandlerTestRouterWithStore(client, store)

	recorder := performImageUpload(t, router, "/api/v1/admin/uploads/emoji", "replacement.png")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, store.objects, 1)
}

func TestAdminEmojiUpdateCleansUnreferencedManagedImage(t *testing.T) {
	item := emojiTestInfo()
	item.Url = "http://example.test/uploads/emojis/old.png"
	item.OriginalUrl = item.Url
	client := &emojiHandlerAdminClient{permissions: []string{"governance:update_emoji"}, item: item}
	store := &fakeImageStore{objects: map[string]fakeImageObject{
		"uploads/emojis/old.png": {data: testPNGImage, contentType: "image/png"},
		"uploads/emojis/new.png": {data: testPNGImage, contentType: "image/png"},
	}}
	router := emojiHandlerTestRouterWithStore(client, store)

	recorder := emojiAdminRequest(router, http.MethodPatch, "/api/v1/admin/emojis/42", `{"fileId":"new.png"}`)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotContains(t, store.objects, "uploads/emojis/old.png")
	require.Contains(t, store.objects, "uploads/emojis/new.png")
}

func TestAdminEmojiDeletePreservesManagedImageStillInUse(t *testing.T) {
	sharedURL := "http://example.test/uploads/emojis/shared.png"
	items := []*adminpb.EmojiInfo{
		{Id: 42, Name: "party", Url: sharedURL, OriginalUrl: sharedURL},
		{Id: 43, Name: "party2", Url: sharedURL, OriginalUrl: sharedURL},
	}
	client := &emojiHandlerAdminClient{permissions: []string{"governance:delete_emoji"}, items: items}
	store := &fakeImageStore{objects: map[string]fakeImageObject{
		"uploads/emojis/shared.png": {data: testPNGImage, contentType: "image/png"},
	}}
	router := emojiHandlerTestRouterWithStore(client, store)

	recorder := emojiAdminRequest(router, http.MethodDelete, "/api/v1/admin/emojis/42", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, store.objects, "uploads/emojis/shared.png")
}

func TestAdminEmojiDeleteCleansUnreferencedManagedImage(t *testing.T) {
	item := emojiTestInfo()
	item.Url = "http://example.test/uploads/emojis/delete.png"
	item.OriginalUrl = item.Url
	client := &emojiHandlerAdminClient{permissions: []string{"governance:delete_emoji"}, item: item}
	store := &fakeImageStore{objects: map[string]fakeImageObject{
		"uploads/emojis/delete.png": {data: testPNGImage, contentType: "image/png"},
	}}
	router := emojiHandlerTestRouterWithStore(client, store)

	recorder := emojiAdminRequest(router, http.MethodDelete, "/api/v1/admin/emojis/42", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotContains(t, store.objects, "uploads/emojis/delete.png")
}

func TestAdminEmojiErrorsUseRPCStatusMapping(t *testing.T) {
	client := &emojiHandlerAdminClient{
		permissions: []string{"governance:create_emoji"}, item: emojiTestInfo(),
		createErr: status.Error(codes.AlreadyExists, "emoji name already exists"),
	}
	router := emojiHandlerTestRouter(client)
	recorder := emojiAdminRequest(router, http.MethodPost, "/api/v1/admin/emoji/add", `{"name":"party","url":"https://example.test/party.webp"}`)
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())

	unsafe := emojiAdminRequest(router, http.MethodPost, "/api/v1/admin/emoji/add", `{"name":"party","url":"https://user:secret@example.test/party.webp"}`)
	require.Equal(t, http.StatusBadRequest, unsafe.Code, unsafe.Body.String())
}

func TestAdminEmojiRoutesRequireDedicatedPermission(t *testing.T) {
	client := &emojiHandlerAdminClient{item: emojiTestInfo()}
	router := emojiHandlerTestRouter(client)
	recorder := emojiAdminRequest(router, http.MethodGet, "/api/v1/admin/emojis", "")
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Nil(t, client.listRequest)
}

func TestAdminEmojiListSerializesEmptyListsAsArrays(t *testing.T) {
	client := &emojiHandlerAdminClient{
		permissions: []string{"governance:list_emojis"},
		item:        &adminpb.EmojiInfo{Id: 42, Name: "party", Url: "http://example.test/party.webp"},
	}
	router := emojiHandlerTestRouter(client)
	recorder := emojiAdminRequest(router, http.MethodGet, "/api/v1/admin/emojis", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var payload struct {
		Data struct {
			Items []struct {
				Aliases                                 json.RawMessage `json:"aliases"`
				RoleIDsThatCanBeUsedThisEmojiAsReaction json.RawMessage `json:"roleIdsThatCanBeUsedThisEmojiAsReaction"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)
	require.JSONEq(t, `[]`, string(payload.Data.Items[0].Aliases))
	require.JSONEq(t, `[]`, string(payload.Data.Items[0].RoleIDsThatCanBeUsedThisEmojiAsReaction))
}

func emojiHandlerTestRouter(client adminpb.AdminServiceClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(&clients.Clients{Admin: client}, "Authorization", "Bearer", testJWTSecret)
	handler.SetPublicBaseURL("http://example.test")
	NewInitControllers(handler)(router)
	return router
}

func emojiHandlerTestRouterWithStore(client adminpb.AdminServiceClient, store *fakeImageStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandlerWithAttachmentStore(&clients.Clients{Admin: client}, "Authorization", "Bearer", testJWTSecret, store)
	handler.SetPublicBaseURL("http://example.test")
	NewInitControllers(handler)(router)
	return router
}

func emojiAdminRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer emoji-admin-token")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func emojiTestInfo() *adminpb.EmojiInfo {
	category := "general"
	return &adminpb.EmojiInfo{Id: 42, Name: "party", Url: "http://example.test/uploads/emojis/party.webp", OriginalUrl: "http://example.test/uploads/emojis/party.webp", ContentType: "image/webp", Category: &category, Aliases: []string{"celebrate"}, UpdatedAt: 1700000000000}
}

type emojiHandlerAdminClient struct {
	adminpb.AdminServiceClient
	permissions   []string
	item          *adminpb.EmojiInfo
	items         []*adminpb.EmojiInfo
	roles         []*adminpb.RoleInfo
	listRolesErr  error
	listRequest   *adminpb.ListEmojisRequest
	listRequests  []*adminpb.ListEmojisRequest
	getRequest    *adminpb.GetEmojiRequest
	createRequest *adminpb.CreateEmojiRequest
	updateRequest *adminpb.UpdateEmojiRequest
	deleteRequest *adminpb.EmojiIDRequest
	createErr     error
	updateErr     error
	deleteErr     error
}

func (c *emojiHandlerAdminClient) GetProfile(context.Context, *adminpb.ProfileRequest, ...grpc.CallOption) (*adminpb.ProfileResponse, error) {
	return &adminpb.ProfileResponse{User: &adminpb.AdminUserInfo{Id: 7, Username: "emoji-admin"}, Permissions: c.permissions}, nil
}

func (c *emojiHandlerAdminClient) RecordOperationLog(context.Context, *adminpb.RecordOperationLogRequest, ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	return &adminpb.SimpleResponse{Success: true}, nil
}

func (c *emojiHandlerAdminClient) ListEmojis(_ context.Context, req *adminpb.ListEmojisRequest, _ ...grpc.CallOption) (*adminpb.EmojiListResponse, error) {
	c.listRequest = req
	c.listRequests = append(c.listRequests, req)
	items := c.items
	if items == nil && c.item != nil {
		items = []*adminpb.EmojiInfo{c.item}
	}
	start := int(req.GetOffset())
	if start > len(items) {
		start = len(items)
	}
	end := start + int(req.GetLimit())
	if req.GetLimit() <= 0 || end > len(items) {
		end = len(items)
	}
	return &adminpb.EmojiListResponse{Items: items[start:end], Total: int64(len(items))}, nil
}

func (c *emojiHandlerAdminClient) ListRoles(context.Context, *adminpb.ListRolesRequest, ...grpc.CallOption) (*adminpb.RoleListResponse, error) {
	if c.listRolesErr != nil {
		return nil, c.listRolesErr
	}
	return &adminpb.RoleListResponse{Items: c.roles, Total: int64(len(c.roles))}, nil
}

func (c *emojiHandlerAdminClient) GetEmoji(_ context.Context, req *adminpb.GetEmojiRequest, _ ...grpc.CallOption) (*adminpb.EmojiResponse, error) {
	c.getRequest = req
	return &adminpb.EmojiResponse{Success: true, Emoji: c.item}, nil
}

func (c *emojiHandlerAdminClient) CreateEmoji(_ context.Context, req *adminpb.CreateEmojiRequest, _ ...grpc.CallOption) (*adminpb.EmojiResponse, error) {
	c.createRequest = req
	if c.createErr != nil {
		return nil, c.createErr
	}
	return &adminpb.EmojiResponse{Success: true, Emoji: c.item}, nil
}

func (c *emojiHandlerAdminClient) UpdateEmoji(_ context.Context, req *adminpb.UpdateEmojiRequest, _ ...grpc.CallOption) (*adminpb.EmojiResponse, error) {
	c.updateRequest = req
	if c.updateErr != nil {
		return nil, c.updateErr
	}
	return &adminpb.EmojiResponse{Success: true, Emoji: c.item}, nil
}

func (c *emojiHandlerAdminClient) DeleteEmoji(_ context.Context, req *adminpb.EmojiIDRequest, _ ...grpc.CallOption) (*adminpb.SimpleResponse, error) {
	c.deleteRequest = req
	if c.deleteErr != nil {
		return nil, c.deleteErr
	}
	return &adminpb.SimpleResponse{Success: true}, nil
}
