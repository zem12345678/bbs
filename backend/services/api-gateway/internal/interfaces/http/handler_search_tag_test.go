package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/searchpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSearchNotesByTagAliasesReturnBareMixedProjections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := []string{"/notes/search-by-tag", "/api/notes/search-by-tag", "/api/v1/notes/search-by-tag"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			searchClient := &fakeSearchByTagClient{resp: &searchpb.SearchNotesByTagResponse{Items: []*searchpb.SearchNoteHit{
				{Kind: "article", Article: &searchpb.ArticleDocument{Id: 9007199254740993, AuthorId: 41, LikeCount: 7, CommentCount: 3}},
				{Kind: "topic", Topic: &searchpb.TopicDocument{Id: 9007199254740995, AuthorId: 42, LikeCount: 5, CommentCount: 2}},
			}}}
			contentClient := &fakeSearchVisibilityContentClient{
				articles: map[int64]*contentpb.ArticleInfo{9007199254740993: {Id: 9007199254740993, Title: "article title", Body: "article body", Tags: []string{"go"}, AuthorId: 41, Status: contentStatusPublished, CreatedAt: 1700000000000, UpdatedAt: 1700000001000, ViewCount: 11}},
				topics:   map[int64]*contentpb.TopicInfo{9007199254740995: {Id: 9007199254740995, Title: "topic title", Body: "topic body", Tags: []string{"go", "search"}, AuthorId: 42, Status: contentStatusPublished, CreatedAt: 1700000002000, UpdatedAt: 1700000003000, ViewCount: 13, ChannelId: 88, Poll: &contentpb.TopicPollInfo{Multiple: true, Choices: []*contentpb.TopicPollChoiceInfo{{Text: "yes", Votes: 4, Selected: true}}}}},
			}
			h := NewHandler(&clients.Clients{Search: searchClient, Content: contentClient}, "Authorization", "Bearer", testJWTSecret)
			router := gin.New()
			NewInitControllers(h)(router)

			body := `{"tag":" Go ","query":[[" Search "]],"reply":null,"renote":null,"poll":null,"withFiles":false,"scope":null,"sinceId":"9007199254740000","untilId":"9007199254740999","limit":2}`
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(body)))

			require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
			require.NotNil(t, searchClient.req)
			require.Equal(t, "go", searchClient.req.GetTag())
			require.Equal(t, int64(9007199254740000), searchClient.req.GetSinceId())
			require.Equal(t, int64(9007199254740999), searchClient.req.GetUntilId())
			require.Equal(t, int32(2), searchClient.req.GetLimit())
			require.Equal(t, []string{"search"}, searchClient.req.GetQuery()[0].GetTags())

			var notes []struct {
				Kind    string                   `json:"kind"`
				Article *searchArticleProjection `json:"article"`
				Topic   *searchTopicProjection   `json:"topic"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &notes))
			require.Len(t, notes, 2)
			require.Equal(t, "article", notes[0].Kind)
			require.Equal(t, "9007199254740993", notes[0].Article.ID)
			require.Equal(t, "41", notes[0].Article.AuthorID)
			require.Equal(t, "article body", notes[0].Article.ContentExcerpt)
			require.Equal(t, "7", notes[0].Article.LikeCount)
			require.Equal(t, "topic", notes[1].Kind)
			require.Equal(t, "9007199254740995", notes[1].Topic.ID)
			require.Equal(t, "42", notes[1].Topic.AuthorID)
			require.Equal(t, "13", notes[1].Topic.ViewCount)
			require.NotContains(t, recorder.Body.String(), `"data"`)
		})
	}
}

func TestSearchNotesByTagAcceptsQueryAndDefaultsLimit(t *testing.T) {
	searchClient := &fakeSearchByTagClient{resp: &searchpb.SearchNotesByTagResponse{}}
	h := NewHandler(&clients.Clients{Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/notes/search-by-tag", strings.NewReader(`{"query":[["Go","数据库"],["Search"]]}`)))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(10), searchClient.req.GetLimit())
	require.Empty(t, searchClient.req.GetTag())
	require.Equal(t, []string{"go", "数据库"}, searchClient.req.GetQuery()[0].GetTags())
	require.JSONEq(t, `[]`, recorder.Body.String())
}

func TestSearchNotesByTagRejectsInvalidParametersBeforeRateLimitAndRPC(t *testing.T) {
	tooManyGroups := make([]string, 9)
	for index := range tooManyGroups {
		tooManyGroups[index] = `["go"]`
	}
	tooManyTags := make([]string, 9)
	for index := range tooManyTags {
		tooManyTags[index] = `"go"`
	}
	tests := map[string]string{
		"missing tag and query": `{}`,
		"blank tag":             `{"tag":"  "}`,
		"null tag":              `{"tag":null}`,
		"long unicode tag":      `{"tag":"` + strings.Repeat("界", 129) + `"}`,
		"null query":            `{"query":null}`,
		"empty query":           `{"query":[]}`,
		"too many groups":       `{"query":[` + strings.Join(tooManyGroups, ",") + `]}`,
		"empty group":           `{"query":[[]]}`,
		"too many group tags":   `{"query":[[` + strings.Join(tooManyTags, ",") + `]]}`,
		"blank query tag":       `{"query":[[" "]]}`,
		"long query tag":        `{"query":[["` + strings.Repeat("界", 129) + `"]]}`,
		"null limit":            `{"tag":"go","limit":null}`,
		"zero limit":            `{"tag":"go","limit":0}`,
		"large limit":           `{"tag":"go","limit":101}`,
		"fractional limit":      `{"tag":"go","limit":1.5}`,
		"zero since id":         `{"tag":"go","sinceId":"0"}`,
		"negative since id":     `{"tag":"go","sinceId":"-1"}`,
		"null since id":         `{"tag":"go","sinceId":null}`,
		"invalid until id":      `{"tag":"go","untilId":"nope"}`,
		"invalid scope":         `{"tag":"go","scope":"everywhere"}`,
		"unsupported scope":     `{"tag":"go","scope":"local"}`,
		"null with files":       `{"tag":"go","withFiles":null}`,
		"with files":            `{"tag":"go","withFiles":true}`,
		"reply false":           `{"tag":"go","reply":false}`,
		"renote true":           `{"tag":"go","renote":true}`,
		"poll false":            `{"tag":"go","poll":false}`,
		"unknown field":         `{"tag":"go","unexpected":true}`,
		"multiple json values":  `{"tag":"go"}{"tag":"rust"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			searchClient := &fakeSearchByTagClient{}
			limiter := &searchRateLimitStub{}
			h := NewHandler(&clients.Clients{Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
			h.SetSearchRateLimits(SearchRateLimits{Content: limiter})
			router := gin.New()
			NewInitControllers(h)(router)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/notes/search-by-tag", strings.NewReader(body)))

			require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), `"legacy_code":"INVALID_PARAM"`)
			require.Nil(t, searchClient.req)
			require.Empty(t, limiter.keys)
		})
	}
}

func TestSearchNotesByTagUsesOptionalAuth(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantCalls  int
	}{
		{name: "anonymous", wantStatus: stdhttp.StatusOK, wantCalls: 1},
		{name: "valid api token", token: userMemoScopedToken(t, "read"), wantStatus: stdhttp.StatusOK, wantCalls: 1},
		{name: "write only api token", token: userMemoScopedToken(t, "write"), wantStatus: stdhttp.StatusForbidden, wantCalls: 0},
		{name: "invalid token", token: "not-a-token", wantStatus: stdhttp.StatusUnauthorized, wantCalls: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchClient := &fakeSearchByTagClient{resp: &searchpb.SearchNotesByTagResponse{}}
			h := NewHandler(&clients.Clients{Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
			router := gin.New()
			NewInitControllers(h)(router)
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/notes/search-by-tag", strings.NewReader(`{"tag":"go"}`))
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			router.ServeHTTP(recorder, req)
			require.Equal(t, tt.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, tt.wantCalls, searchClient.calls)
		})
	}
}

func TestSearchNotesByTagSharesContentRateLimit(t *testing.T) {
	searchClient := &fakeSearchByTagClient{}
	limiter := &searchRateLimitStub{limited: true}
	h := NewHandler(&clients.Clients{Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetSearchRateLimits(SearchRateLimits{Content: limiter})
	router := gin.New()
	NewInitControllers(h)(router)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/notes/search-by-tag", strings.NewReader(`{"tag":"go"}`))
	req.RemoteAddr = "203.0.113.20:54321"
	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	require.Zero(t, searchClient.calls)
	require.Equal(t, []string{searchRateLimitKey(searchRateLimitContent, "203.0.113.20")}, limiter.keys)
}

func TestSearchNotesByTagMapsBackendInvalidArgumentToInvalidParam(t *testing.T) {
	searchClient := &fakeSearchByTagClient{err: status.Error(codes.InvalidArgument, "SEARCH_TAG_FILTER_UNSUPPORTED")}
	h := NewHandler(&clients.Clients{Search: searchClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/notes/search-by-tag", strings.NewReader(`{"tag":"go"}`)))

	require.Equal(t, stdhttp.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"legacy_code":"INVALID_PARAM"`)
}

func TestSearchNotesByTagFiltersStaleResults(t *testing.T) {
	searchClient := &fakeSearchByTagClient{resp: &searchpb.SearchNotesByTagResponse{Items: []*searchpb.SearchNoteHit{
		{Kind: "article", Article: &searchpb.ArticleDocument{Id: 1, AuthorId: 10}},
		{Kind: "article", Article: &searchpb.ArticleDocument{Id: 2, AuthorId: 20}},
		{Kind: "topic", Topic: &searchpb.TopicDocument{Id: 3, AuthorId: 30}},
	}}}
	contentClient := &fakeSearchVisibilityContentClient{
		articles: map[int64]*contentpb.ArticleInfo{
			1: {Id: 1, AuthorId: 10, Status: contentStatusArchived},
			2: {Id: 2, AuthorId: 20, Status: contentStatusPublished, Body: "inactive author", CreatedAt: 1},
		},
		topics: map[int64]*contentpb.TopicInfo{3: {Id: 3, AuthorId: 30, Status: contentStatusPublished, Body: "visible", CreatedAt: 1}},
	}
	h := NewHandler(&clients.Clients{Search: searchClient, Content: contentClient}, "Authorization", "Bearer", testJWTSecret)
	router := gin.New()
	NewInitControllers(h)(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/notes/search-by-tag", bytes.NewBufferString(`{"tag":"go"}`)))

	require.Equal(t, stdhttp.StatusOK, recorder.Code, recorder.Body.String())
	var notes []searchNoteProjection
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &notes))
	require.Len(t, notes, 2)
	require.Equal(t, "2", notes[0].Article.ID)
	require.Equal(t, "3", notes[1].Topic.ID)
}

type fakeSearchByTagClient struct {
	searchpb.SearchServiceClient
	req   *searchpb.SearchNotesByTagRequest
	resp  *searchpb.SearchNotesByTagResponse
	err   error
	calls int
}

func (f *fakeSearchByTagClient) SearchNotesByTag(_ context.Context, req *searchpb.SearchNotesByTagRequest, _ ...grpc.CallOption) (*searchpb.SearchNotesByTagResponse, error) {
	f.calls++
	f.req = req
	return f.resp, f.err
}
