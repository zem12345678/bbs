package tracing

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestProcessHookDoesNotRecordCommandArgumentsOrResults(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	hook := &Hook{tracer: provider.Tracer(instrumentationName)}

	ctx := context.Background()
	cmd := redis.NewStringCmd(ctx, "get", "chat:ws-ticket:sensitive-token")
	cmd.SetVal("sensitive-message-body")
	if err := hook.ProcessHook(func(context.Context, redis.Cmder) error { return nil })(ctx, cmd); err != nil {
		t.Fatalf("process hook: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	for _, attr := range spans[0].Attributes() {
		switch string(attr.Key) {
		case "db.statement", "db.result":
			t.Fatalf("sensitive Redis attribute %q was recorded", attr.Key)
		}
	}
}
