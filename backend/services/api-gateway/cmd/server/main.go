package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"api-gateway/internal/clients"
	httpiface "api-gateway/internal/interfaces/http"
	iochttp "api-gateway/internal/ioc/http"
	ioclogger "api-gateway/internal/ioc/logger"
	ioctrace "api-gateway/internal/ioc/trace"

	"github.com/spf13/viper"
)

type config struct {
	Service struct {
		Name     string
		HTTPPort int
	}
	Auth struct {
		TokenHeader string
		TokenPrefix string
		JWTSecret   string
	}
	Upstreams clients.Options
}

func main() {
	configPath := flag.String("config", "configs/config.yaml", "config file path")
	flag.Parse()

	v, cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		log.Fatalf("load logger options: %v", err)
	}
	appLogger, err := ioclogger.New(logOptions)
	if err != nil {
		log.Fatalf("create logger: %v", err)
	}
	zapLogger := appLogger.GetZapLogger()

	traceOptions, err := ioctrace.NewOptions(v, zapLogger)
	if err != nil {
		log.Fatalf("load trace options: %v", err)
	}
	tracer, err := ioctrace.New(traceOptions)
	if err != nil {
		log.Fatalf("create tracer: %v", err)
	}

	httpOptions, err := iochttp.NewOptions(v, zapLogger)
	if err != nil {
		log.Fatalf("load http options: %v", err)
	}
	bbsClients, err := clients.New(cfg.Upstreams)
	if err != nil {
		log.Fatalf("create grpc clients: %v", err)
	}
	defer func() { _ = bbsClients.Close() }()

	handler := httpiface.NewHandler(bbsClients, cfg.Auth.TokenHeader, cfg.Auth.TokenPrefix, cfg.Auth.JWTSecret)
	router := iochttp.NewRouter(httpOptions, zapLogger, httpiface.NewInitControllers(handler), tracer)
	server, err := iochttp.New(httpOptions, zapLogger, router)
	if err != nil {
		log.Fatalf("create http server: %v", err)
	}
	server.Application(cfg.Service.Name)
	if err := server.Start(); err != nil {
		log.Fatalf("start http server: %v", err)
	}

	waitForShutdown(server)
}

func loadConfig(path string) (*viper.Viper, *config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, nil, err
	}
	var cfg config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, nil, err
	}
	if cfg.Service.Name == "" {
		cfg.Service.Name = "api-gateway"
	}
	if cfg.Service.HTTPPort == 0 {
		cfg.Service.HTTPPort = 8080
		v.Set("service.httpPort", cfg.Service.HTTPPort)
	}
	if cfg.Auth.TokenHeader == "" {
		cfg.Auth.TokenHeader = "Authorization"
	}
	if cfg.Auth.TokenPrefix == "" {
		cfg.Auth.TokenPrefix = "Bearer"
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "bbs-local-dev-secret"
	}
	if cfg.Upstreams.Admin == "" {
		cfg.Upstreams.Admin = "127.0.0.1:9114"
	}
	if cfg.Upstreams.User == "" {
		cfg.Upstreams.User = "127.0.0.1:9102"
	}
	if cfg.Upstreams.Content == "" {
		cfg.Upstreams.Content = "127.0.0.1:9103"
	}
	if cfg.Upstreams.Comment == "" {
		cfg.Upstreams.Comment = "127.0.0.1:9104"
	}
	if cfg.Upstreams.Reaction == "" {
		cfg.Upstreams.Reaction = "127.0.0.1:9105"
	}
	if cfg.Upstreams.Search == "" {
		cfg.Upstreams.Search = "127.0.0.1:9106"
	}
	if cfg.Upstreams.Feed == "" {
		cfg.Upstreams.Feed = "127.0.0.1:9113"
	}
	if cfg.Upstreams.Credit == "" {
		cfg.Upstreams.Credit = "127.0.0.1:9107"
	}
	if cfg.Upstreams.Notification == "" {
		cfg.Upstreams.Notification = "127.0.0.1:9108"
	}
	return v, &cfg, nil
}

func waitForShutdown(server *iochttp.Server) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	if err := server.Stop(); err != nil {
		log.Printf("stop http server: %v", err)
	}
}
