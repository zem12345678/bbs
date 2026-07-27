package comment

import (
	"context"
	"testing"

	"github.com/spf13/viper"
)

func TestInternalAuthCredentials(t *testing.T) {
	credentials := internalAuthCredentials{token: "comment-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[internalAuthMetadataKey]; got != "comment-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("comment internal credential must support the configured local insecure transport")
	}
}

func TestNewClientRequiresInternalAuthToken(t *testing.T) {
	_, err := NewClient(nil, viper.New())
	if err == nil || err.Error() != "comment internal auth token required" {
		t.Fatalf("NewClient() error = %v", err)
	}
}
