package grpc

import (
	"testing"

	"content-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9103)
	v.Set("grpc.server.serviceName", "bbs-content-service")
	v.Set("grpc.server.internalAuthToken", " content-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "content-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
