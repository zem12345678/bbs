package grpc

import (
	"testing"

	"credit-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9107)
	v.Set("grpc.server.serviceName", "bbs-credit-service")
	v.Set("grpc.server.internalAuthToken", " credit-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "credit-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
