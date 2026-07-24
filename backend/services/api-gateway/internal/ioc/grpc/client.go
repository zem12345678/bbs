package grpc

import (
	"api-gateway/internal/ioc/discovery"
	"api-gateway/internal/ioc/trace"
	"api-gateway/pkg/grpc/middleware/exception"
	"api-gateway/pkg/logger"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

type ClientOptions struct {
	Timeout         time.Duration
	Tag             string
	GrpcDialOptions []grpc.DialOption
	logger          logger.Logger
	EtcdAddr        []string
	ServerName      string
	Secure          bool
}

type Client struct {
	o      *ClientOptions
	Logger *zap.Logger
}

type ClientOptional func(o *ClientOptions)

func NewClientOptions(v *viper.Viper, l logger.Logger, tracer *trace.TracerProvider) (*ClientOptions, error) {
	var (
		err error
		o   = new(ClientOptions)
	)
	if err = v.UnmarshalKey("grpc.client", o); err != nil {
		return nil, err
	}

	l.Info("load grpc.client options success", logger.Any("grpc.client options", o))

	grpc_prometheus.EnableClientHandlingTimeHistogram()

	streamInts := []grpc.StreamClientInterceptor{
		grpc_prometheus.StreamClientInterceptor,
		grpc_zap.StreamClientInterceptor(l.GetZapLogger(), grpc_zap.WithLevels(grpcClientCodeToLevel)),
	}
	unaryInts := []grpc.UnaryClientInterceptor{
		grpc_prometheus.UnaryClientInterceptor,
		grpc_zap.UnaryClientInterceptor(l.GetZapLogger(), grpc_zap.WithLevels(grpcClientCodeToLevel)),
	}

	secureCreds := grpc.WithTransportCredentials(grpcInsecure.NewCredentials())
	o.GrpcDialOptions = append(o.GrpcDialOptions,
		secureCreds,
		//grpc.WithBlock(),
		grpc.WithChainUnaryInterceptor(exception.NewUnaryClientInterceptor()),
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(streamInts...)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unaryInts...)),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(
			otelgrpc.WithTracerProvider(tracer.TracerProvider),
			otelgrpc.WithMeterProvider(tracer.MeterProvider),
		)),
	)
	o.logger = l.With(logger.String("type", o.ServerName))
	//o.logger = logger.WithOptions(logger.String("type", o.ServerName))
	return o, nil
}

func grpcClientCodeToLevel(code codes.Code) zapcore.Level {
	switch code {
	case codes.Internal, codes.Unavailable:
		return zap.ErrorLevel
	}

	level := grpc_zap.DefaultClientCodeToLevel(code)
	if level == zap.DebugLevel {
		return zap.InfoLevel
	}
	return level
}

func NewClient(o *ClientOptions) (*Client, error) {
	return &Client{
		o:      o,
		Logger: o.logger.GetZapLogger(),
	}, nil
}

func WithTimeout(d time.Duration) ClientOptional {
	return func(o *ClientOptions) {
		o.Timeout = d
	}
}

func WithTag(tag string) ClientOptional {
	return func(o *ClientOptions) {
		o.Tag = tag
	}
}

func WithEndpoint(endpoints []string) ClientOptional {
	return func(o *ClientOptions) {
		o.EtcdAddr = endpoints
	}
}

func WithLogger(logger logger.Logger) ClientOptional {
	return func(o *ClientOptions) {
		o.logger = logger
	}
}

func WithGrpcDialOptions(options ...grpc.DialOption) ClientOptional {
	return func(o *ClientOptions) {
		o.GrpcDialOptions = append(o.GrpcDialOptions, options...)
	}
}

func WithSecureConnection(secure bool) ClientOptional {
	return func(o *ClientOptions) {
		o.Secure = secure
	}
}

func (c *Client) Dial(service string, secure bool, options ...ClientOptional) (*grpc.ClientConn, error) {
	return c.dial(service, secure, options...)
}

func (c *Client) dial(service string, secure bool, options ...ClientOptional) (*grpc.ClientConn, error) {
	o := &ClientOptions{
		Timeout:         c.o.Timeout,
		Tag:             c.o.Tag,
		GrpcDialOptions: make([]grpc.DialOption, len(c.o.GrpcDialOptions)),
		EtcdAddr:        c.o.EtcdAddr,
		ServerName:      c.o.ServerName,
		logger:          c.o.logger,
		Secure:          secure,
	}
	copy(o.GrpcDialOptions, c.o.GrpcDialOptions)

	lbConfig := struct {
		LoadBalancingPolicy string `json:"LoadBalancingPolicy"`
	}{roundrobin.Name}
	configBytes, err := json.Marshal(lbConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal lbConfig")
	}
	options = append(options, WithGrpcDialOptions(
		grpc.WithDefaultServiceConfig(string(configBytes)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                2 * time.Minute,
			Timeout:             20 * time.Second,
			PermitWithoutStream: false,
		}),
	))

	for _, option := range options {
		option(o)
	}

	etcdRegister := discovery.NewResolver(o.EtcdAddr, o.logger)
	o.GrpcDialOptions = append(o.GrpcDialOptions, grpc.WithResolvers(etcdRegister))

	addr := fmt.Sprintf("%s:///%s", etcdRegister.Scheme(), service)
	conn, err := grpc.NewClient(addr, o.GrpcDialOptions...)
	if err != nil {
		return nil, errors.Wrap(err, "grpc dial error")
	}
	return conn, nil
}
