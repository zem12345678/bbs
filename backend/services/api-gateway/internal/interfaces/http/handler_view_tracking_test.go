package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/contentpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetArticleTracksView(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		article: &contentpb.ArticleInfo{Id: 2001, Status: contentStatusPublished},
	}
	h := NewHandler(&clients.Clients{Content: contentClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "2001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/articles/2001", nil)

	h.getArticle(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.articleReq)
	require.True(t, contentClient.articleReq.GetTrackView())
}

func TestGetTopicTracksView(t *testing.T) {
	contentClient := &fakeCommentTargetContentClient{
		topic: &contentpb.TopicInfo{Id: 1001, Status: contentStatusPublished},
	}
	h := NewHandler(&clients.Clients{Content: contentClient}, "Authorization", "Bearer", testJWTSecret)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "1001"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topics/1001", nil)

	h.getTopic(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, contentClient.topicReq)
	require.True(t, contentClient.topicReq.GetTrackView())
}
