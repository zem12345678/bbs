package grpc

import (
	"testing"

	"search-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9106)
	v.Set("grpc.server.serviceName", "bbs-search-service")
	v.Set("grpc.server.internalAuthToken", " search-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "search-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
