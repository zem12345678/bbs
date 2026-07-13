package http

import (
	"context"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestCaptureAdminRequestBodyRedactsJSONAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"username":"admin","password":"Admin123!","newPwd":"New123!"}`
	c := newTestContextWithBody(stdhttp.MethodPost, "application/json", body)

	got := captureAdminRequestBody(c)

	require.JSONEq(t, `{"username":"admin","password":"[REDACTED]","newPwd":"[REDACTED]"}`, got)
	restored, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(restored))
}

func TestCaptureAdminRequestBodyRedactsNestedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"profile":{"old_password":"old","items":[{"access_token":"token","name":"visible"}]},"clientSecret":"secret","normal":"kept"}`
	c := newTestContextWithBody(stdhttp.MethodPatch, "application/json", body)

	got := captureAdminRequestBody(c)

	require.JSONEq(t, `{"profile":{"old_password":"[REDACTED]","items":[{"access_token":"[REDACTED]","name":"visible"}]},"clientSecret":"[REDACTED]","normal":"kept"}`, got)
	require.NotContains(t, got, `:"token"`)
	require.NotContains(t, got, `:"secret"`)
	require.Contains(t, got, "visible")
}

func TestCaptureAdminRequestBodyRedactsSensitiveSettingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"key":"auth.github.client_secret","value":"github-secret","value_type":"password","group":"auth"}`
	c := newTestContextWithBody(stdhttp.MethodPut, "application/json", body)

	got := captureAdminRequestBody(c)

	require.JSONEq(t, `{"key":"auth.github.client_secret","value":"[REDACTED]","value_type":"password","group":"auth"}`, got)
	require.NotContains(t, got, "github-secret")
	require.Contains(t, got, "auth.github.client_secret")
}

func TestCaptureAdminRequestBodyKeepsNormalSettingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"key":"site_name","value":"BBS Community","value_type":"string","group":"site"}`
	c := newTestContextWithBody(stdhttp.MethodPut, "application/json", body)

	got := captureAdminRequestBody(c)

	require.JSONEq(t, body, got)
	require.Contains(t, got, "BBS Community")
}

func TestCaptureAdminRequestBodyRedactsFormBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newTestContextWithBody(stdhttp.MethodPost, "application/x-www-form-urlencoded", "username=admin&password=Admin123%21&access_token=abc")

	got := captureAdminRequestBody(c)

	require.Contains(t, got, "username=admin")
	require.Contains(t, got, "password=%5BREDACTED%5D")
	require.Contains(t, got, "access_token=%5BREDACTED%5D")
	require.NotContains(t, got, "Admin123")
	require.NotContains(t, got, "abc")
}

func TestCaptureAdminRequestBodySkipsMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := "raw multipart body"
	c := newTestContextWithBody(stdhttp.MethodPost, "multipart/form-data; boundary=test", body)

	got := captureAdminRequestBody(c)

	require.Equal(t, "[multipart/form-data]", got)
	restored, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(restored))
}

func TestCaptureAdminRequestBodyTruncatesLoggedBodyOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.Repeat("a", maxLoggedRequestBody+32)
	c := newTestContextWithBody(stdhttp.MethodPost, "text/plain", body)

	got := captureAdminRequestBody(c)

	require.Len(t, got, maxLoggedRequestBody+len("...[truncated]"))
	require.True(t, strings.HasSuffix(got, "...[truncated]"))
	restored, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(restored))
}

func TestListEmailLogsUsesSystemPaginationAndTablePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminClient := &fakeEmailLogAdminClient{}
	handler := NewHandler(&clients.Clients{Admin: adminClient}, "Authorization", "Bearer", testJWTSecret)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/email-logs?page=2&page_size=3&query=welcome&status=1", nil)

	handler.listEmailLogs(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.NotNil(t, adminClient.req)
	require.Equal(t, int32(1), adminClient.req.GetStatus())
	require.Equal(t, "welcome", adminClient.req.GetQuery())
	require.Equal(t, int32(3), adminClient.req.GetLimit())
	require.Equal(t, int32(3), adminClient.req.GetOffset())

	var envelope struct {
		Data struct {
			Items       []map[string]any `json:"items"`
			List        []map[string]any `json:"list"`
			Total       int              `json:"total"`
			CurrentPage int              `json:"currentPage"`
			PageSize    int              `json:"pageSize"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, 9, envelope.Data.Total)
	require.Equal(t, 2, envelope.Data.CurrentPage)
	require.Equal(t, 3, envelope.Data.PageSize)
	require.Len(t, envelope.Data.Items, 1)
	require.Len(t, envelope.Data.List, 1)
	require.Equal(t, "buyer@example.com", envelope.Data.Items[0]["to"])
	require.Equal(t, "welcome", envelope.Data.Items[0]["templateKey"])
	require.Equal(t, "welcome", envelope.Data.Items[0]["template_key"])
	require.Equal(t, envelope.Data.Items[0]["id"], envelope.Data.List[0]["id"])
}

func newTestContextWithBody(method string, contentType string, body string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, "/api/v1/admin/system/users", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	c.Request = req
	return c
}

type fakeEmailLogAdminClient struct {
	adminpb.AdminServiceClient
	req *adminpb.ListEmailLogsRequest
}

func (f *fakeEmailLogAdminClient) ListEmailLogs(_ context.Context, req *adminpb.ListEmailLogsRequest, _ ...grpc.CallOption) (*adminpb.EmailLogListResponse, error) {
	f.req = req
	return &adminpb.EmailLogListResponse{
		Items: []*adminpb.EmailLogInfo{
			{
				Id:          7,
				To:          "buyer@example.com",
				Subject:     "Welcome",
				TemplateKey: "welcome",
				Provider:    "smtp",
				Status:      1,
				CreatedAt:   1700000000000,
				UpdatedAt:   1700000001000,
			},
		},
		Total: 9,
	}, nil
}
