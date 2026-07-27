package grpc

import (
	"testing"

	"reaction-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9105)
	v.Set("grpc.server.serviceName", "bbs-reaction-service")
	v.Set("grpc.server.internalAuthToken", " reaction-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "reaction-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
