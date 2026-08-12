package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClipCompatibilityUnavailableEndpointsAreExplicit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		handler gin.HandlerFunc
	}{
		{name: "public list", handler: (&Handler{}).publicClipsUnavailable},
		{name: "note list", handler: (&Handler{}).noteClipsUnavailable},
		{name: "export", handler: (&Handler{}).exportClipsUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(stdhttp.MethodPost, "/", nil)
			tc.handler(ctx)
			require.Equal(t, stdhttp.StatusNotImplemented, recorder.Code)
			require.Contains(t, recorder.Body.String(), "not_implemented")
		})
	}
}
