package grpc

import (
	"testing"

	"notification-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9108)
	v.Set("grpc.server.serviceName", "bbs-notification-service")
	v.Set("grpc.server.internalAuthToken", " notification-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "notification-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
