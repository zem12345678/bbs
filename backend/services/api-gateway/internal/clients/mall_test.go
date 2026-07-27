package clients

import (
	"context"
	"testing"
)

func TestMallInternalAuthCredentials(t *testing.T) {
	credentials := mallInternalAuthCredentials{token: "mall-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[mallInternalAuthMetadataKey]; got != "mall-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("mall internal credential must support the configured local insecure transport")
	}
}
