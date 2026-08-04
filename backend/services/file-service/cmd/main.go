package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	pb "file-service/api/proto/filepb"
	fileapp "file-service/internal/application/file"
	contentclient "file-service/internal/clients/content"
	creditclient "file-service/internal/clients/credit"
	mallclient "file-service/internal/clients/mall"
	"file-service/internal/config"
	"file-service/internal/infrastructure/persistence"
	"file-service/internal/infrastructure/storage"
	interfacesgrpc "file-service/internal/interfaces/grpc"
	discovery "file-service/internal/ioc/discovery"
	iocgrpc "file-service/internal/ioc/grpc"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	root := &cobra.Command{Use: "file-service", SilenceUsage: true}
	root.AddCommand(newServerCommand(), newMigrateCommand())
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newServerCommand() *cobra.Command {
	var configFile string
	command := &cobra.Command{
		Use:   "server",
		Short: "Start file gRPC server",
		RunE: func(*cobra.Command, []string) error {
			return runServer(configFile)
		},
	}
	command.Flags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Start with the given configuration file")
	return command
}

func newMigrateCommand() *cobra.Command {
	var configFile string
	command := &cobra.Command{
		Use:   "migrate",
		Short: "Create or update file-service database schema",
		RunE: func(*cobra.Command, []string) error {
			return runMigrate(configFile)
		},
	}
	command.Flags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run with the given configuration file")
	return command
}

func runMigrate(configFile string) error {
	v, err := config.Load(configFile)
	if err != nil {
		return err
	}
	pool, err := newPostgresPool(context.Background(), v.GetString("postgres.dsn"), v.GetInt("postgres.max_open_conns"))
	if err != nil {
		return err
	}
	defer pool.Close()
	return persistence.NewPostgresRepository(pool).EnsureSchema(context.Background())
}

func runServer(configFile string) error {
	v, err := config.Load(configFile)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := newPostgresPool(ctx, v.GetString("postgres.dsn"), v.GetInt("postgres.max_open_conns"))
	if err != nil {
		return err
	}
	defer pool.Close()
	repo := persistence.NewPostgresRepository(pool)
	charger, err := creditclient.NewClient(v)
	if err != nil {
		return err
	}
	defer charger.Close()
	membershipEntitlements, err := mallclient.NewClient(v)
	if err != nil {
		return err
	}
	defer membershipEntitlements.Close()
	topics, err := contentclient.NewClient(v)
	if err != nil {
		return err
	}
	defer topics.Close()
	deleter, err := storage.NewMinIODeleter(v)
	if err != nil {
		return err
	}
	var objects fileapp.ObjectDeleter
	if deleter != nil {
		storageCtx, storageCancel := context.WithTimeout(ctx, 10*time.Second)
		err = deleter.EnsureReady(storageCtx)
		storageCancel()
		if err != nil {
			return err
		}
		objects = deleter
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(iocgrpc.NewInternalAuthUnaryServerInterceptor(v.GetString("grpc.server.internalAuthToken"))),
		grpc.StreamInterceptor(iocgrpc.NewInternalAuthStreamServerInterceptor(v.GetString("grpc.server.internalAuthToken"))),
	)
	pb.RegisterFileServiceServer(server, interfacesgrpc.NewHandler(fileapp.NewService(
		repo,
		charger,
		membershipEntitlements,
		topics,
		fileapp.WithAccountErasure(repo, objects),
		fileapp.WithFileCapacity(v.GetInt64("files.capacityBytes")),
	)))
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	port := v.GetInt("grpc.server.port")
	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	defer listener.Close()

	addr := net.JoinHostPort(localIPv4(), strconv.Itoa(port))
	registration, err := discovery.Register(ctx, v.GetStringSlice("grpc.server.etcdAddr"), v.GetString("grpc.server.serviceName"), addr)
	if err != nil {
		return err
	}
	defer registration.Close()
	go func() { _ = server.Serve(listener) }()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	server.GracefulStop()
	return nil
}

const defaultPostgresMaxOpenConns = 4

func newPostgresPool(ctx context.Context, dsn string, maxOpenConns int) (*pgxpool.Pool, error) {
	config, err := newPostgresPoolConfig(dsn, maxOpenConns)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, config)
}

func newPostgresPoolConfig(dsn string, maxOpenConns int) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if maxOpenConns <= 0 {
		maxOpenConns = defaultPostgresMaxOpenConns
	}
	config.MaxConns = int32(maxOpenConns)
	config.MinConns = 0
	return config, nil
}

func localIPv4() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && isIntranetIPv4(ip) {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

func isIntranetIPv4(ip net.IP) bool {
	parsed := ip.To4()
	if parsed == nil || parsed.IsLoopback() || (parsed[0] == 169 && parsed[1] == 254) {
		return false
	}
	return parsed[0] == 10 ||
		(parsed[0] == 172 && parsed[1] >= 16 && parsed[1] <= 31) ||
		(parsed[0] == 192 && parsed[1] == 168)
}
