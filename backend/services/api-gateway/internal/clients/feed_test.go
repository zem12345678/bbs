package clients

import (
	"context"
	"testing"
)

func TestFeedInternalAuthCredentials(t *testing.T) {
	credentials := feedInternalAuthCredentials{token: "feed-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[feedInternalAuthMetadataKey]; got != "feed-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("feed internal credential must support the configured local insecure transport")
	}
}
