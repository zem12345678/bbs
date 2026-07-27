package clients

import (
	"context"
	"testing"
)

func TestCreditInternalAuthCredentials(t *testing.T) {
	credentials := creditInternalAuthCredentials{token: "credit-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[creditInternalAuthMetadataKey]; got != "credit-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("credit internal credential must support the configured local insecure transport")
	}
}
