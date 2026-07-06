package trace

import (
	"context"

	"github.com/google/wire"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.uber.org/zap"
)

type TracerProvider struct {
	TracerProvider *sdktrace.TracerProvider
}

type Options struct {
	GrpcEndpoint string `toml:"grpcEndpoint" json:"grpcEndpoint" yaml:"grpcEndpoint" env:"TRACE_PROVIDER_GRPCENDPOINT"`
	ServiceName  string `toml:"serviceName" json:"serviceName" yaml:"serviceName" env:"TRACE_PROVIDER_SERVICENAME"`
	Version      string `toml:"version" json:"version" yaml:"version" env:"TRACE_PROVIDER_VERSION"`
	Env          string `toml:"env" json:"env" yaml:"env" env:"TRACE_PROVIDER_ENV"`
}

func NewOptions(v *viper.Viper, logger *zap.Logger) (*Options, error) {
	var (
		err error
		o   = new(Options)
	)
	if err = v.UnmarshalKey("trace", &o); err != nil {
		return nil, errors.Wrap(err, "unmarshal opentelemetry jaeger configuration error")
	}
	logger.Info("load opentelemetry jaeger configuration success", zap.Any("opentelemetry jaeger options", o))
	return o, nil
}

func New(o *Options) (*TracerProvider, error) {
	ctx := context.Background()
	tracerProvider := new(TracerProvider)
	if o.GrpcEndpoint == "" {
		return nil, errors.New("opentelemetry jaeger config error")
	}
	if o.GrpcEndpoint != "" {
		exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(o.GrpcEndpoint))
		if err != nil {
			panic(err)
		}

		res, err := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceName(o.ServiceName),
				semconv.ServiceVersion(o.Version),
				attribute.String("env", o.Env),
			), resource.WithOS(), resource.WithHost())
		if err != nil {
			res = resource.Default()
			zap.L().Error("trace resource missing value", zap.Error(err))
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
		)
		metricExporter, err := prometheus.New()
		if err != nil {
			return nil, err
		}
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricExporter),
		)
		otel.SetMeterProvider(mp)
		otel.SetTracerProvider(tp)
		b3Propagator := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
			b3Propagator),
		)
		tracerProvider.TracerProvider = tp
	}
	return tracerProvider, nil
}

var ProviderSet = wire.NewSet(New, NewOptions)
