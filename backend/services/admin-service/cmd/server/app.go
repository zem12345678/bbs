package server

import (
	"admin/internal/infrastructure/upstream"
	"admin/internal/interfaces/grpc/pb/adminpb"
	"admin/internal/support/config"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"admin/internal/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type App struct {
	cfg       *config.Config
	server    *grpc.Server
	listener  net.Listener
	upstreams *upstream.Clients
	db        *persistence.DB
}

func NewApp(cfg *config.Config, server *grpc.Server, upstreams *upstream.Clients, db *persistence.DB) (*App, error) {
	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(cfg.Service.GRPCPort))
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return &App{cfg: cfg, server: server, listener: lis, upstreams: upstreams, db: db}, nil
}

func NewGRPCServer(handler adminpb.AdminServiceServer) *grpc.Server {
	server := grpc.NewServer()
	adminpb.RegisterAdminServiceServer(server, handler)
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
	if a.upstreams != nil {
		_ = a.upstreams.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}
