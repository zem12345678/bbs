package http

import (
	"net"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func TestNewOptionsUsesConfiguredHost(t *testing.T) {
	v := viper.New()
	v.Set("service.name", "bbs-api-gateway")
	v.Set("service.httpPort", 18080)
	v.Set("http.host", "0.0.0.0")

	options, err := NewOptions(v, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if options.Host != "0.0.0.0" {
		t.Fatalf("host = %q, want 0.0.0.0", options.Host)
	}
}

func TestValidateTrustedProxies(t *testing.T) {
	if err := validateTrustedProxies(nil); err != nil {
		t.Fatalf("empty trusted proxies should disable forwarded IP headers: %v", err)
	}
	if err := validateTrustedProxies([]string{"10.0.0.0/8", "127.0.0.1"}); err != nil {
		t.Fatalf("valid trusted proxies: %v", err)
	}
	if err := validateTrustedProxies([]string{"not-a-proxy"}); err == nil {
		t.Fatal("expected invalid trusted proxy to be rejected")
	}
}

func TestEmptyTrustedProxiesUsesRemoteAddress(t *testing.T) {
	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	router.GET("/", func(c *gin.Context) {
		c.String(stdhttp.StatusOK, c.ClientIP())
	})

	request := httptest.NewRequest(stdhttp.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.20:12345"
	request.Header.Set("X-Forwarded-For", "198.51.100.10")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Body.String() != "203.0.113.20" {
		t.Fatalf("client IP = %q, want remote address", recorder.Body.String())
	}
}

func TestApplyCORSRequiresExplicitOrigins(t *testing.T) {
	router := gin.New()
	applyCORS(router, nil)
	router.GET("/", func(c *gin.Context) { c.Status(stdhttp.StatusNoContent) })

	request := httptest.NewRequest(stdhttp.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("allow origin = %q, want no CORS header", origin)
	}
}

func TestApplyCORSAllowsConfiguredOrigin(t *testing.T) {
	router := gin.New()
	applyCORS(router, []string{"https://bbs.example.com"})
	router.GET("/", func(c *gin.Context) { c.Status(stdhttp.StatusNoContent) })

	request := httptest.NewRequest(stdhttp.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://bbs.example.com")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "https://bbs.example.com" {
		t.Fatalf("allow origin = %q", origin)
	}
}

func TestRegisterPprofRequiresExplicitEnablement(t *testing.T) {
	disabled := gin.New()
	registerPprof(disabled, false)
	if hasRoute(disabled, "/debug/pprof/") {
		t.Fatal("pprof route registered while disabled")
	}

	enabled := gin.New()
	registerPprof(enabled, true)
	if !hasRoute(enabled, "/debug/pprof/") {
		t.Fatal("pprof route missing while enabled")
	}
}

func hasRoute(router *gin.Engine, path string) bool {
	for _, route := range router.Routes() {
		if strings.EqualFold(route.Path, path) {
			return true
		}
	}
	return false
}

func TestStartReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	server, err := New(&Options{Host: "127.0.0.1", Port: port}, zap.NewNop(), gin.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err == nil {
		t.Fatal("expected occupied port to fail during Start")
	}
}
