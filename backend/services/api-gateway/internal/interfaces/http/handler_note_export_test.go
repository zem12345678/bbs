package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/filepb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestBuildNoteExportPaginatesAndPreservesTopicFiles(t *testing.T) {
	articles := make([]*contentpb.ArticleInfo, 0, 101)
	for i := int64(1); i <= 101; i++ {
		id := 1000 + i*2
		body := "article-" + strconv.FormatInt(id, 10)
		title := "unused"
		if i == 1 {
			body = ""
			title = "article title"
		}
		articles = append(articles, &contentpb.ArticleInfo{
			Id: id, Title: title, Body: body, AuthorId: 42,
			Status: int32(i%4 + 1), CreatedAt: 1700000000000 + i,
		})
	}
	topic := &contentpb.TopicInfo{
		Id: 1003, Title: "topic title", AuthorId: 42, Status: 4, CreatedAt: 1700000000123,
		Poll: &contentpb.TopicPollInfo{
			Multiple: true, ExpiresAt: 1800000000123,
			Choices: []*contentpb.TopicPollChoiceInfo{{Text: "A", Votes: 2}, {Text: "B", Votes: 0}},
		},
	}
	content := &noteExportContentStub{
		articles: articles,
		topics:   []*contentpb.TopicInfo{topic},
		topicDetails: map[int64]*contentpb.TopicInfo{
			topic.GetId(): topic,
		},
	}
	fileClient := &noteExportFileStub{
		fakeUserFileClient: &fakeUserFileClient{},
		attachments: map[int64][]*filepb.Attachment{
			topic.GetId(): {{
				Id: 9001, TopicId: topic.GetId(), OwnerId: 42, OriginalName: "guide.pdf",
				ContentType: "application/pdf", SizeBytes: 2048, CreatedAt: 1700000000456,
			}},
		},
	}
	h := NewHandler(&clients.Clients{Content: content, File: fileClient}, "Authorization", "Bearer", testJWTSecret)
	h.SetPublicBaseURL("https://bbs.example.com")

	payload, err := h.buildNoteExport(context.Background(), 42)

	require.NoError(t, err)
	var records []noteExportRecord
	require.NoError(t, json.Unmarshal(payload, &records))
	require.Len(t, records, 102)
	require.Equal(t, "1002", records[0].ID)
	require.Equal(t, "article title", records[0].Text)
	require.Equal(t, "1003", records[1].ID)
	require.Equal(t, "1004", records[2].ID)
	require.Equal(t, "2023-11-14T22:13:20.001Z", records[0].CreatedAt)
	require.Equal(t, "2023-11-14T22:13:20.123Z", records[1].CreatedAt)
	require.Equal(t, "topic title", records[1].Text)
	require.Equal(t, []string{"A", "B"}, records[1].Poll.Choices)
	require.Equal(t, []int64{2, 0}, records[1].Poll.Votes)
	require.Equal(t, "2027-01-15T08:00:00.123Z", *records[1].Poll.ExpiresAt)
	require.Equal(t, []string{"9001"}, records[1].FileIDs)
	require.Len(t, records[1].Files, 1)
	require.Equal(t, "guide.pdf", records[1].Files[0].Name)
	require.Equal(t, "application/pdf", records[1].Files[0].Type)
	require.EqualValues(t, 2048, records[1].Files[0].Size)
	require.Equal(t, "2023-11-14T22:13:20.456Z", records[1].Files[0].CreatedAt)
	require.Equal(t, "https://bbs.example.com/api/v1/attachments/9001/download", records[1].Files[0].URL)
	require.Empty(t, records[1].Files[0].Properties)
	require.Nil(t, records[1].Files[0].Folder)
	require.Nil(t, records[1].Files[0].UserID)
	require.Nil(t, records[1].Files[0].User)
	require.Equal(t, []int64{0, 1200}, content.articleAfterIDs)
	require.Equal(t, []int64{0}, content.topicAfterIDs)
	require.Equal(t, []int32{0, 0}, content.articleStatuses)
	require.Equal(t, []int32{0}, content.topicStatuses)
	require.Equal(t, []*filepb.ListOwnedTopicAttachmentsRequest{{TopicId: 1003, OwnerId: 42}}, fileClient.listRequests)

	var raw []map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	require.Contains(t, raw[0], "poll")
	require.Nil(t, raw[0]["poll"])
	require.Contains(t, raw[0], "files")
	require.NotContains(t, raw[0], "user")
}

func TestBuildNoteExportReturnsEmptyArray(t *testing.T) {
	h := NewHandler(&clients.Clients{
		Content: &noteExportContentStub{},
		File:    &noteExportFileStub{fakeUserFileClient: &fakeUserFileClient{}},
	}, "Authorization", "Bearer", testJWTSecret)

	payload, err := h.buildNoteExport(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, "[]", string(payload))
}

func TestExportNotesRegistersJSONAndCompletionNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fileClient := &noteExportFileStub{fakeUserFileClient: &fakeUserFileClient{
		createResp: &filepb.FileResponse{File: &filepb.File{Id: 9020, OwnerId: 42}},
	}}
	notifications := &clipExportNotificationStub{}
	store := newFakeUserFileStore()
	permit := &clipExportPermitStub{}
	h := NewHandlerWithAttachmentStore(&clients.Clients{
		Content: &noteExportContentStub{}, File: fileClient, NotificationInternal: notifications,
	}, "Authorization", "Bearer", testJWTSecret, store)
	h.SetNoteExportGate(&clipExportGateStub{permit: permit})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodPost, "/i/export-notes", strings.NewReader(`{}`))
	c.Set("user_id", int64(42))
	h.exportNotes(c)

	require.Equal(t, stdhttp.StatusNoContent, c.Writer.Status(), recorder.Body.String())
	require.NotNil(t, fileClient.createReq)
	require.Equal(t, "exports", fileClient.createReq.GetBizType())
	require.Equal(t, "application/json", fileClient.createReq.GetContentType())
	require.True(t, strings.HasPrefix(fileClient.createReq.GetOriginalName(), "notes-"))
	require.True(t, strings.HasSuffix(fileClient.createReq.GetOriginalName(), ".json"))
	require.EqualValues(t, 2, fileClient.createReq.GetSizeBytes())
	require.True(t, permit.committed)
	require.Equal(t, "note", notifications.req.GetExportedEntity())
	require.EqualValues(t, 9020, notifications.req.GetFileId())
	require.Len(t, store.objects, 1)
	for _, object := range store.objects {
		require.Equal(t, "[]", string(object.data))
		require.Equal(t, "application/json", object.contentType)
	}
}

func TestNoteExportRoutesRequireInteractiveAuthAndApplyRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newHandler := func() *Handler {
		h := NewHandlerWithAttachmentStore(&clients.Clients{
			Content: &noteExportContentStub{},
			File:    &noteExportFileStub{fakeUserFileClient: &fakeUserFileClient{}},
		}, "Authorization", "Bearer", testJWTSecret, newFakeUserFileStore())
		h.SetNoteExportGate(&clipExportGateStub{err: errExportRateLimited})
		return h
	}
	for _, path := range []string{"/i/export-notes", "/api/i/export-notes", "/api/v1/i/export-notes"} {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			NewInitControllers(newHandler())(router)
			request := httptest.NewRequest(stdhttp.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{"sub": "42"}))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			require.Equal(t, stdhttp.StatusTooManyRequests, recorder.Code, recorder.Body.String())
		})
	}

	router := gin.New()
	NewInitControllers(newHandler())(router)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/i/export-notes", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+signedAuthToken(t, jwt.MapClaims{
		"sub": "42", "jti": "note-export-api-token", "exp": time.Now().Add(time.Hour).Unix(),
		credentialVersionClaim: "0", "token_type": apiTokenType, "scopes": []string{"read"},
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code, recorder.Body.String())
}

type noteExportContentStub struct {
	contentpb.ContentServiceClient
	articles        []*contentpb.ArticleInfo
	topics          []*contentpb.TopicInfo
	topicDetails    map[int64]*contentpb.TopicInfo
	articleAfterIDs []int64
	topicAfterIDs   []int64
	articleStatuses []int32
	topicStatuses   []int32
}

func (s *noteExportContentStub) ListArticles(_ context.Context, req *contentpb.ListArticlesRequest, _ ...grpc.CallOption) (*contentpb.ArticleListResponse, error) {
	s.articleAfterIDs = append(s.articleAfterIDs, req.GetAfterId())
	s.articleStatuses = append(s.articleStatuses, req.GetStatus())
	items := make([]*contentpb.ArticleInfo, 0)
	for _, item := range s.articles {
		if item.GetAuthorId() == req.GetAuthorId() && item.GetId() > req.GetAfterId() {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetId() < items[j].GetId() })
	if len(items) > int(req.GetLimit()) {
		items = items[:req.GetLimit()]
	}
	return &contentpb.ArticleListResponse{Items: items, Total: int64(len(s.articles))}, nil
}

func (s *noteExportContentStub) ListTopics(_ context.Context, req *contentpb.ListTopicsRequest, _ ...grpc.CallOption) (*contentpb.TopicListResponse, error) {
	s.topicAfterIDs = append(s.topicAfterIDs, req.GetAfterId())
	s.topicStatuses = append(s.topicStatuses, req.GetStatus())
	items := make([]*contentpb.TopicInfo, 0)
	for _, item := range s.topics {
		if item.GetAuthorId() == req.GetAuthorId() && item.GetId() > req.GetAfterId() {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetId() < items[j].GetId() })
	if len(items) > int(req.GetLimit()) {
		items = items[:req.GetLimit()]
	}
	return &contentpb.TopicListResponse{Items: items, Total: int64(len(s.topics))}, nil
}

func (s *noteExportContentStub) GetTopic(_ context.Context, req *contentpb.GetTopicRequest, _ ...grpc.CallOption) (*contentpb.TopicResponse, error) {
	return &contentpb.TopicResponse{Topic: s.topicDetails[req.GetId()]}, nil
}

type noteExportFileStub struct {
	*fakeUserFileClient
	attachments  map[int64][]*filepb.Attachment
	listRequests []*filepb.ListOwnedTopicAttachmentsRequest
}

func (s *noteExportFileStub) ListOwnedTopicAttachments(_ context.Context, req *filepb.ListOwnedTopicAttachmentsRequest, _ ...grpc.CallOption) (*filepb.AttachmentListResponse, error) {
	s.listRequests = append(s.listRequests, req)
	return &filepb.AttachmentListResponse{Items: s.attachments[req.GetTopicId()]}, nil
}
