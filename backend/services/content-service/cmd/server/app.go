package server

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"content-service/internal/infrastructure/persistence"
	contentgrpc "content-service/internal/interfaces/grpc"
	"content-service/internal/interfaces/grpc/pb/contentpb"
	"content-service/internal/support/config"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/gorm"
)

type App struct {
	cfg      *config.Config
	server   *grpc.Server
	listener net.Listener
	redis    *redis.Client
	db       *gorm.DB
}

func NewApp(cfg *config.Config, server *grpc.Server, rdb *redis.Client, db *gorm.DB) (*App, error) {
	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(cfg.Service.GRPCPort))
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return &App{cfg: cfg, server: server, listener: lis, redis: rdb, db: db}, nil
}

func NewGRPCServer(handler *contentgrpc.Handler) *grpc.Server {
	server := grpc.NewServer()
	contentpb.RegisterContentServiceServer(server, handler)
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	return server
}

func (a *App) Start() error {
	go func() {
		fmt.Printf("%s listening on %s\n", a.cfg.Service.Name, a.listener.Addr().String())
		if err := a.server.Serve(a.listener); err != nil {
			fmt.Printf("grpc server stopped: %v\n", err)
		}
	}()
	return nil
}

func (a *App) AwaitSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	a.Stop()
}

func (a *App) Stop() {
	if a.server != nil {
		a.server.GracefulStop()
	}
}

func (a *App) Close() {
	if a.redis != nil {
		_ = a.redis.Close()
	}
	_ = persistence.CloseDB(a.db)
}
