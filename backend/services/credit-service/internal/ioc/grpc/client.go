package grpc

import (
	"context"
	"credit-service/internal/ioc/discovery"
	"credit-service/internal/ioc/trace"
	"credit-service/pkg/grpc/middleware/exception"
	"credit-service/pkg/logger"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/balancer/roundrobin"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
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

	prometheusExporter, err := initPrometheusExporter()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(metric.WithReader(prometheusExporter))
	grpc_prometheus.EnableClientHandlingTimeHistogram()

	streamInts := []grpc.StreamClientInterceptor{
		grpc_prometheus.StreamClientInterceptor,
		grpc_zap.StreamClientInterceptor(l.GetZapLogger()),
	}
	unaryInts := []grpc.UnaryClientInterceptor{
		grpc_prometheus.UnaryClientInterceptor,
		grpc_zap.UnaryClientInterceptor(l.GetZapLogger()),
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
			otelgrpc.WithMeterProvider(meterProvider),
		)),
	)
	o.logger = l.With(logger.String("type", o.ServerName))
	//o.logger = logger.WithOptions(logger.String("type", o.ServerName))
	return o, nil
}

func initPrometheusExporter() (*prometheus.Exporter, error) {
	var once sync.Once
	var exporter *prometheus.Exporter
	var initErr error

	once.Do(func() {
		exporter, initErr = prometheus.New()
	})
	return exporter, initErr
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
	resolver.Register(etcdRegister)

	_, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()

	addr := fmt.Sprintf("%s:///%s", etcdRegister.Scheme(), service)
	conn, err := grpc.NewClient(addr, o.GrpcDialOptions...)
	if err != nil {
		return nil, errors.Wrap(err, "grpc dial error")
	}
	return conn, nil
}
