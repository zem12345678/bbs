package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	chatapp "chat-service/internal/application/chat"
	"chat-service/internal/config"
	"chat-service/internal/infrastructure/persistence"
	interfacesgrpc "chat-service/internal/interfaces/grpc"
	"chat-service/internal/platform/discovery"
	platformpostgres "chat-service/internal/platform/postgres"
	"chat-service/pkg/snowflake"

	"go.uber.org/zap"
	stdgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type Server struct {
	config *config.Config
	logger *zap.Logger
}

func NewServer(cfg *config.Config, logger *zap.Logger) *Server {
	return &Server{config: cfg, logger: logger}
}

func (s *Server) Run(ctx context.Context) error {
	pool, err := platformpostgres.Open(ctx, s.config.Postgres.DSN, s.config.Postgres.MaxOpenConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	ids, err := snowflake.New(s.config.Snowflake.WorkerID)
	if err != nil {
		return err
	}
	repository := persistence.NewPostgresRepository(pool)
	service := chatapp.NewService(repository, ids)
	handler := interfacesgrpc.NewHandler(service)

	grpcServer := stdgrpc.NewServer(stdgrpc.ChainUnaryInterceptor(recoveryInterceptor(s.logger)))
	interfacesgrpc.Register(grpcServer, handler)
	healthServer := health.NewServer()
	healthServer.SetServingStatus(s.config.GRPC.Server.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	listenAddress := net.JoinHostPort(s.config.GRPC.Server.Host, strconv.Itoa(s.config.GRPC.Server.Port))
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("listen for chat grpc: %w", err)
	}
	defer listener.Close()

	registrar, err := discovery.New(s.config.GRPC.Server.EtcdAddr, s.logger)
	if err != nil {
		return err
	}
	advertiseHost := strings.TrimSpace(s.config.GRPC.Server.AdvertiseHost)
	if advertiseHost == "" {
		advertiseHost = localIPv4()
	}
	advertiseAddress := net.JoinHostPort(advertiseHost, strconv.Itoa(s.config.GRPC.Server.Port))
	if err := registrar.Start(ctx, s.config.GRPC.Server.ServiceName, advertiseAddress); err != nil {
		_ = registrar.Close()
		return fmt.Errorf("register chat grpc service: %w", err)
	}
	defer registrar.Close()

	s.logger.Info("chat grpc server started",
		zap.String("listen", listenAddress),
		zap.String("advertise", advertiseAddress),
		zap.Int64("snowflake_worker_id", s.config.Snowflake.WorkerID),
	)
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- grpcServer.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, stdgrpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve chat grpc: %w", err)
	case <-ctx.Done():
		healthServer.SetServingStatus(s.config.GRPC.Server.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			grpcServer.Stop()
		}
		return nil
	}
}

func recoveryInterceptor(logger *zap.Logger) stdgrpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *stdgrpc.UnaryServerInfo, handler stdgrpc.UnaryHandler) (response any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("chat grpc panic",
					zap.String("method", info.FullMethod),
					zap.Any("panic", recovered),
					zap.ByteString("stack", debug.Stack()),
				)
				err = status.Error(codes.Internal, "chat service internal error")
			}
		}()
		return handler(ctx, request)
	}
}

func localIPv4() string {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, networkInterface := range interfaces {
			if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, addressErr := networkInterface.Addrs()
			if addressErr != nil {
				continue
			}
			for _, address := range addresses {
				var ip net.IP
				switch value := address.(type) {
				case *net.IPNet:
					ip = value.IP
				case *net.IPAddr:
					ip = value.IP
				}
				if ipv4 := ip.To4(); ipv4 != nil && !ipv4.IsLoopback() {
					return ipv4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}
