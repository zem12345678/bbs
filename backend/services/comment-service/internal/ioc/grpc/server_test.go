package grpc

import (
	"testing"

	"comment-service/pkg/logger"

	"github.com/spf13/viper"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9104)
	v.Set("grpc.server.serviceName", "bbs-comment-service")
	v.Set("grpc.server.internalAuthToken", " comment-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "comment-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}
