package grpc

import (
	"testing"

	"user-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9102)
	v.Set("grpc.server.serviceName", "bbs-user-service")
	v.Set("grpc.server.internalAuthToken", " user-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "user-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
