package grpc

import (
	"chat-service/internal/ioc/discovery"
	"chat-service/internal/ioc/trace"
	"chat-service/pkg/grpc/middleware/recovery"
	"chat-service/pkg/logger"
	"chat-service/pkg/network"
	"chat-service/pkg/ratelimit"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type ServerOptions struct {
	Host              string
	AdvertiseHost     string
	Port              int
	EtcdAddr          []string
	ServiceName       string
	Timeout           time.Duration
	InternalAuthToken string
	TLS               ServerTLSOptions
}

type Server struct {
	o            *ServerOptions
	app          string
	host         string
	port         int
	logger       logger.Logger
	server       *grpc.Server
	mu           sync.Mutex
	registration interface{ Stop() }
}

func NewServerOptions(v *viper.Viper, l logger.Logger) (*ServerOptions, error) {
	var o ServerOptions
	if err := v.UnmarshalKey("grpc.server", &o); err != nil {
		return nil, errors.Wrap(err, "unmarshal grpc server option error")
	}
	o.Host = strings.TrimSpace(v.GetString("grpc.server.host"))
	o.AdvertiseHost = strings.TrimSpace(v.GetString("grpc.server.advertiseHost"))
	o.Port = v.GetInt("grpc.server.port")
	o.EtcdAddr = v.GetStringSlice("grpc.server.etcdAddr")
	o.ServiceName = v.GetString("grpc.server.serviceName")
	o.Timeout = v.GetDuration("grpc.server.timeout")
	o.InternalAuthToken = strings.TrimSpace(v.GetString("grpc.server.internalAuthToken"))
	o.TLS = ServerTLSOptions{
		Enabled:      v.GetBool("grpc.server.tls.enabled"),
		CertFile:     strings.TrimSpace(v.GetString("grpc.server.tls.certFile")),
		KeyFile:      strings.TrimSpace(v.GetString("grpc.server.tls.keyFile")),
		ClientCAFile: strings.TrimSpace(v.GetString("grpc.server.tls.clientCAFile")),
	}
	normalizeServerOptions(&o, v, "bbs-chat-service", "chat-service")
	l.Info("load grpc options success",
		logger.String("host", o.Host),
		logger.String("advertise_host", o.AdvertiseHost),
		logger.Int("port", o.Port),
		logger.String("service", o.ServiceName),
	)
	return &o, nil
}

func normalizeServerOptions(o *ServerOptions, v *viper.Viper, fallback string, legacy ...string) {
	if o.ServiceName == "" {
		o.ServiceName = v.GetString("service.name")
	}
	for _, name := range legacy {
		if o.ServiceName == name {
			o.ServiceName = fallback
			break
		}
	}
	if o.ServiceName == "" {
		o.ServiceName = fallback
	}
	if o.Port == 0 {
		o.Port = v.GetInt("service.grpcPort")
	}
}

type InitServers func(server *grpc.Server)

func NewServer(o *ServerOptions, l logger.Logger, init InitServers, tracer *trace.TracerProvider, rateLimiter ratelimit.Limiter) (*Server, error) {
	if rateLimiter == nil {
		return nil, errors.New("grpc rate limiter is required")
	}
	prometheusExporter, err := prometheus.New()
	if err != nil {
		l.Error("failed to create prometheus exporter", logger.Error(err))
		return nil, errors.Wrap(err, "create prometheus exporter")
	}
	meterProvider := metric.NewMeterProvider(metric.WithReader(prometheusExporter))
	grpc_prometheus.EnableHandlingTimeHistogram()

	unaryInts := []grpc.UnaryServerInterceptor{
		recovery.UnaryRecoverInterceptor(), // Recovery 中间件置顶
		newInternalAuthUnaryServerInterceptor(o.InternalAuthToken),
		newServiceRateLimitUnaryServerInterceptor(rateLimiter),
		grpc_ctxtags.UnaryServerInterceptor(),
		grpc_prometheus.UnaryServerInterceptor,
		grpc_zap.UnaryServerInterceptor(l.GetZapLogger()),
	}

	streamInts := []grpc.StreamServerInterceptor{
		recovery.StreamRecoverInterceptor(), // Recovery 中间件置顶
		newInternalAuthStreamServerInterceptor(o.InternalAuthToken),
		grpc_ctxtags.StreamServerInterceptor(),
		grpc_prometheus.StreamServerInterceptor,
		grpc_zap.StreamServerInterceptor(l.GetZapLogger()),
	}

	serverOptions := []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInts...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInts...)),
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(tracer.TracerProvider),
			otelgrpc.WithMeterProvider(meterProvider),
		)),
	}
	if o.TLS.Enabled {
		tlsConfig, err := newServerTLSConfig(o.TLS)
		if err != nil {
			return nil, err
		}
		serverOptions = append(serverOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	gs := grpc.NewServer(serverOptions...)
	init(gs)
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())

	return &Server{
		o:      o,
		logger: l.With(logger.String("type", o.ServiceName)),
		server: gs,
	}, nil
}

// Application 服务应用
func (s *Server) Application(name string) {
	s.app = name
}

func (s *Server) registerService(addr string) error {
	etcdRegister := discovery.NewRegister(s.o.EtcdAddr, s.logger)
	node := discovery.Server{
		Name: s.o.ServiceName,
		Addr: addr,
	}
	if _, err := etcdRegister.Register(node, 10); err != nil {
		return errors.Wrap(err, "service register failed")
	}

	s.mu.Lock()
	s.registration = etcdRegister
	s.mu.Unlock()

	return nil
}

func (s *Server) Start() error {
	s.port = s.o.Port
	if s.port == 0 {
		s.port = network.GetAvailablePort()
		s.o.Port = s.port // 保持配置一致性
	}

	fallbackHost := ""
	if strings.TrimSpace(s.o.Host) == "" || strings.TrimSpace(s.o.AdvertiseHost) == "" {
		fallbackHost = network.GetLocalIP4()
	}
	addr, advertiseAddr, err := resolveServerAddresses(s.o.Host, s.o.AdvertiseHost, fallbackHost, s.port)
	if err != nil {
		return err
	}
	s.host, _, _ = net.SplitHostPort(addr)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "create listener failed")
	}
	//defer lis.Close()
	// 注意：不能 defer lis.Close()。Serve 在下方 goroutine 异步运行，
	// Start 返回后 listener 仍需存活；关闭交由 GracefulStop。

	if err := s.registerService(advertiseAddr); err != nil {
		_ = lis.Close()
		return errors.Wrap(err, "service registration failed")
	}

	s.logger.Info("grpc server starting",
		logger.String("addr", addr),
		logger.String("advertise_addr", advertiseAddr),
		logger.String("service", s.o.ServiceName),
	)

	go func() {
		if err := s.server.Serve(lis); err != nil {
			s.logger.Error("grpc server runtime error", logger.Error(err))
		}
	}()

	s.logger.Info(fmt.Sprintf("service started listen on %s", addr))
	return nil
}

func resolveServerAddresses(host, advertiseHost, fallbackHost string, port int) (string, string, error) {
	host = strings.TrimSpace(host)
	advertiseHost = strings.TrimSpace(advertiseHost)
	fallbackHost = strings.TrimSpace(fallbackHost)
	if host == "" {
		host = fallbackHost
	}
	if advertiseHost == "" {
		advertiseHost = fallbackHost
	}
	if host == "" || advertiseHost == "" {
		return "", "", errors.New("get local ipv4 error")
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port)), net.JoinHostPort(advertiseHost, fmt.Sprintf("%d", port)), nil
}

// Stop  停止GRPC服务
func (s *Server) Stop() error {
	s.logger.Info("grpc server stopping",
		logger.String("service", s.o.ServiceName),
		logger.Int("port", s.port),
	)
	s.mu.Lock()
	registration := s.registration
	s.registration = nil
	s.mu.Unlock()
	if registration != nil {
		registration.Stop()
	}
	s.server.GracefulStop()
	return nil
}
