package credit

import (
	"context"
	"testing"

	"github.com/spf13/viper"
)

func TestInternalAuthCredentials(t *testing.T) {
	credentials := internalAuthCredentials{token: "credit-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[internalAuthMetadataKey]; got != "credit-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("credit internal credential must support the configured local insecure transport")
	}
}

func TestNewClientRequiresInternalAuthToken(t *testing.T) {
	_, err := NewClient(viper.New())
	if err == nil {
		t.Fatal("NewClient() error = nil, want missing credit internal auth token")
	}
}
