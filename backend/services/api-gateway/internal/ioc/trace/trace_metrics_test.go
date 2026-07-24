package trace_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	iocgrpc "api-gateway/internal/ioc/grpc"
	traceioc "api-gateway/internal/ioc/trace"
	"api-gateway/pkg/logger/types"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func TestGRPCClientReusesTraceMeterProviderForPrometheusScrape(t *testing.T) {
	registry := prometheus.NewRegistry()
	previousRegisterer := prometheus.DefaultRegisterer
	previousGatherer := prometheus.DefaultGatherer
	previousMeterProvider := otel.GetMeterProvider()
	previousTracerProvider := otel.GetTracerProvider()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = previousRegisterer
		prometheus.DefaultGatherer = previousGatherer
		otel.SetMeterProvider(previousMeterProvider)
		otel.SetTracerProvider(previousTracerProvider)
	})

	tracer, err := traceioc.New(&traceioc.Options{
		GrpcEndpoint: "127.0.0.1:4317", ServiceName: "metrics-test", Version: "test", Env: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	v.Set("grpc.client.serverName", "metrics-test")
	if _, err := iocgrpc.NewClientOptions(v, types.NewZapLogger(zap.NewNop()), tracer); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
