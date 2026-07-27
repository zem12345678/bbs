package http

import (
	"api-gateway/internal/ioc/trace"
	"api-gateway/pkg/http/middleware/accesslog"
	"api-gateway/pkg/http/middleware/i18n"
	ginprometheus "api-gateway/pkg/http/middleware/metric/prometheus"
	"api-gateway/pkg/http/middleware/recovery"
	"api-gateway/pkg/http/middleware/validator"
	"api-gateway/pkg/network"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

type Options struct {
	Port               int
	Host               string
	Mode               string
	ServerName         string
	CORSAllowedOrigins []string
	TrustedProxies     []string
	PprofEnabled       bool
}

type Server struct {
	o          *Options
	app        string
	host       string
	port       int
	logger     *zap.Logger
	router     *gin.Engine
	httpServer http.Server
}

func NewOptions(v *viper.Viper, logger *zap.Logger) (*Options, error) {
	var (
		err error
		o   = new(Options)
	)
	if err = v.UnmarshalKey("http", &o); err != nil {
		return nil, errors.Wrap(err, "unmarshal http option error")
	}
	o.Port = v.GetInt("service.httpPort")
	if o.Port == 0 {
		o.Port = 8080
	}
	if o.ServerName == "" {
		o.ServerName = v.GetString("service.name")
	}
	o.CORSAllowedOrigins = v.GetStringSlice("cors.allowedOrigins")
	o.TrustedProxies = v.GetStringSlice("http.trustedProxies")
	o.PprofEnabled = v.GetBool("http.pprofEnabled")
	if err := validateTrustedProxies(o.TrustedProxies); err != nil {
		return nil, errors.Wrap(err, "invalid http trusted proxies")
	}
	if o.Host == "" {
		o.Host = "127.0.0.1"
	}
	if o.Mode == "" {
		o.Mode = gin.DebugMode
	}
	logger.Info("load http options success", zap.Any("http options", o))
	return o, err
}

type InitControllers func(r *gin.Engine)

func NewRouter(o *Options, logger *zap.Logger, init InitControllers, tracer *trace.TracerProvider) *gin.Engine {
	//defer func() {
	//	if err := tracer.HttpTracerProvider.Shutdown(context.Background()); err != nil {
	//		logger.Error("Error shutting down tracer provider: %v", zap.Error(err))
	//	}
	//}()
	gin.SetMode(o.Mode)
	r := gin.New()
	if err := r.SetTrustedProxies(o.TrustedProxies); err != nil {
		logger.Error("configure trusted proxies failed", zap.Error(err))
	}
	r.Use(otelgin.Middleware(fmt.Sprintf("%s:%s:%d", o.ServerName, o.Host, o.Port), otelgin.WithTracerProvider(tracer.TracerProvider)))
	applyCORS(r, o.CORSAllowedOrigins)
	//国际化
	r.Use(i18n.GinI18nLocalize())
	//参数验证器
	r.Use(validator.TransactionMiddleware())
	// panic之后自动恢复
	r.Use(gin.Recovery())
	r.Use(recovery.Recovery(logger))
	// 日志格式化
	r.Use(accesslog.GinZap(logger, time.RFC3339, true))
	//panic日志格式化
	r.Use(ginzap.RecoveryWithZap(logger, true))
	// 添加prometheus 监控
	r.Use(ginprometheus.New(r).Middleware())
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	registerPprof(r, o.PprofEnabled)
	init(r)
	return r
}

// applyCORS installs CORS only for explicitly configured browser origins.
// An absent configuration must not silently expose authenticated APIs to every
// origin; same-origin requests do not need the middleware.
func applyCORS(r *gin.Engine, allowedOrigins []string) {
	if len(allowedOrigins) == 0 {
		return
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Length",
			"Content-Type",
			"Authorization",
			"X-Requested-With",
			"Accept",
		},
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * time.Hour,
	}))
}

func registerPprof(r *gin.Engine, enabled bool) {
	if enabled {
		pprof.Register(r)
	}
}

func validateTrustedProxies(proxies []string) error {
	return gin.New().SetTrustedProxies(proxies)
}

func New(o *Options, logger *zap.Logger, router *gin.Engine) (*Server, error) {
	return &Server{
		logger: logger.With(zap.String("type", "http.server")),
		router: router,
		o:      o,
	}, nil
}

// Application set app name
func (s *Server) Application(name string) {
	s.app = name
}

func (s *Server) Start() error {
	s.port = s.o.Port
	if s.port == 0 {
		s.port = network.GetAvailablePort()
	}
	s.host = s.o.Host
	if s.host == "" {
		// return errors.New("get local ipv4 error")
		s.host = network.GetLocalIP4()
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.logger.Info("http server starting ...", zap.String("addr", addr))

	s.httpServer = http.Server{Addr: addr, Handler: s.router}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "listen http server")
	}
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server runtime error", zap.Error(err))
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	s.logger.Info("stopping http server")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5) // 平滑关闭,等待5秒钟处理
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "shutdown http server error")
	}

	return nil
}

// ProviderSet dependency injection
var ProviderSet = wire.NewSet(New, NewRouter, NewOptions)
