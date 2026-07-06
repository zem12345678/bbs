package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
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
	configureEnv(v)
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, nil, err
	}
	applyEnvOverrides(v)
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

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_GATEWAY_SERVICE_NAME")
	bindEnv(v, "service.httpPort", "BBS_GATEWAY_SERVICE_HTTP_PORT")
	bindEnv(v, "auth.tokenHeader", "BBS_GATEWAY_AUTH_TOKEN_HEADER")
	bindEnv(v, "auth.tokenPrefix", "BBS_GATEWAY_AUTH_TOKEN_PREFIX")
	bindEnv(v, "auth.jwtSecret", "BBS_GATEWAY_AUTH_JWT_SECRET")
	bindEnv(v, "log.filename", "BBS_GATEWAY_LOG_FILENAME")
	bindEnv(v, "log.level", "BBS_GATEWAY_LOG_LEVEL")
	bindEnv(v, "log.stdout", "BBS_GATEWAY_LOG_STDOUT")
	bindEnv(v, "trace.grpcEndpoint", "BBS_GATEWAY_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.serviceName", "BBS_GATEWAY_TRACE_SERVICE_NAME")
	bindEnv(v, "trace.version", "BBS_GATEWAY_TRACE_VERSION")
	bindEnv(v, "trace.env", "BBS_GATEWAY_TRACE_ENV")
	bindEnv(v, "cors.allowedOrigins", "BBS_GATEWAY_CORS_ALLOWED_ORIGINS")
	bindEnv(v, "upstreams.admin", "BBS_GATEWAY_UPSTREAMS_ADMIN")
	bindEnv(v, "upstreams.user", "BBS_GATEWAY_UPSTREAMS_USER")
	bindEnv(v, "upstreams.content", "BBS_GATEWAY_UPSTREAMS_CONTENT")
	bindEnv(v, "upstreams.comment", "BBS_GATEWAY_UPSTREAMS_COMMENT")
	bindEnv(v, "upstreams.reaction", "BBS_GATEWAY_UPSTREAMS_REACTION")
	bindEnv(v, "upstreams.search", "BBS_GATEWAY_UPSTREAMS_SEARCH")
	bindEnv(v, "upstreams.feed", "BBS_GATEWAY_UPSTREAMS_FEED")
	bindEnv(v, "upstreams.credit", "BBS_GATEWAY_UPSTREAMS_CREDIT")
	bindEnv(v, "upstreams.notification", "BBS_GATEWAY_UPSTREAMS_NOTIFICATION")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_CORS_ALLOWED_ORIGINS")); value != "" {
		v.Set("cors.allowedOrigins", splitCommaSeparated(value))
	}
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func waitForShutdown(server *iochttp.Server) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	if err := server.Stop(); err != nil {
		log.Printf("stop http server: %v", err)
	}
}
