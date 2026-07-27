package clients

import (
	"context"
	"testing"
)

func TestSearchInternalAuthCredentials(t *testing.T) {
	credentials := searchInternalAuthCredentials{token: "search-internal-token"}
	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[searchInternalAuthMetadataKey]; got != "search-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("search internal credential must support the configured local insecure transport")
	}
}
