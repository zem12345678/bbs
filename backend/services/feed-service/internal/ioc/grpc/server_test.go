package grpc

import (
	"testing"

	"feed-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9113)
	v.Set("grpc.server.serviceName", "bbs-feed-service")
	v.Set("grpc.server.internalAuthToken", " feed-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "feed-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
