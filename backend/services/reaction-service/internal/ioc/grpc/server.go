package grpc

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"reaction-service/internal/ioc/discovery"
	"reaction-service/internal/ioc/trace"
	"reaction-service/pkg/grpc/middleware/recovery"
	"reaction-service/pkg/logger"
	"reaction-service/pkg/network"
	"syscall"
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
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type ServerOptions struct {
	Port        int
	EtcdAddr    []string
	ServiceName string
	Timeout     time.Duration
}

type Server struct {
	o      *ServerOptions
	app    string
	host   string
	port   int
	logger logger.Logger
	server *grpc.Server
}

func NewServerOptions(v *viper.Viper, l logger.Logger) (*ServerOptions, error) {
	var o ServerOptions
	if err := v.UnmarshalKey("grpc.server", &o); err != nil {
		return nil, errors.Wrap(err, "unmarshal grpc server option error")
	}
	normalizeServerOptions(&o, v, "bbs-reaction-service", "reaction-service")
	l.Info("load grpc options success", logger.Any("grpc options", o))
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

func NewServer(o *ServerOptions, l logger.Logger, init InitServers, tracer *trace.TracerProvider) (*Server, error) {
	prometheusExporter, err := prometheus.New()
	if err != nil {
		l.Error("failed to create prometheus exporter", logger.Error(err))
		return nil, errors.Wrap(err, "create prometheus exporter")
	}
	meterProvider := metric.NewMeterProvider(metric.WithReader(prometheusExporter))
	grpc_prometheus.EnableHandlingTimeHistogram()

	unaryInts := []grpc.UnaryServerInterceptor{
		recovery.UnaryRecoverInterceptor(), // Recovery 中间件置顶
		grpc_ctxtags.UnaryServerInterceptor(),
		grpc_prometheus.UnaryServerInterceptor,
		grpc_zap.UnaryServerInterceptor(l.GetZapLogger()),
	}

	streamInts := []grpc.StreamServerInterceptor{
		recovery.StreamRecoverInterceptor(), // Recovery 中间件置顶
		grpc_ctxtags.StreamServerInterceptor(),
		grpc_prometheus.StreamServerInterceptor,
		grpc_zap.StreamServerInterceptor(l.GetZapLogger()),
	}

	gs := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInts...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInts...)),
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(tracer.TracerProvider),
			otelgrpc.WithMeterProvider(meterProvider),
		)),
	)
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

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		sig := <-ch
		s.logger.Info("received shutdown signal", logger.String("signal", sig.String()))
		etcdRegister.Stop()
		s.server.GracefulStop()
		//if err := etcdRegister.Unregister(); err != nil {
		//	s.logger.Error("failed to unregister from etcd", logger.Error(err))
		//}
	}()

	return nil
}

func (s *Server) Start() error {
	s.port = s.o.Port
	if s.port == 0 {
		s.port = network.GetAvailablePort()
		s.o.Port = s.port // 保持配置一致性
	}

	s.host = network.GetLocalIP4()
	if s.host == "" {
		return errors.New("get local ipv4 error")
	}
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "create listener failed")
	}
	//defer lis.Close()
	// 注意：不能 defer lis.Close()。Serve 在下方 goroutine 异步运行，
	// Start 返回后 listener 仍需存活；关闭交由 GracefulStop。

	if err := s.registerService(addr); err != nil {
		return errors.Wrap(err, "service registration failed")
	}

	s.logger.Info("grpc server starting",
		logger.String("addr", addr),
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

// Stop  停止GRPC服务
func (s *Server) Stop() error {
	s.logger.Info("grpc server stopping",
		logger.String("service", s.o.ServiceName),
		logger.Int("port", s.port),
	)
	s.server.GracefulStop()
	return nil
}
