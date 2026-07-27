package clients

import (
	"context"
	"testing"
)

func TestContentInternalAuthCredentials(t *testing.T) {
	credentials := contentInternalAuthCredentials{token: "content-internal-token"}
	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[contentInternalAuthMetadataKey]; got != "content-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("content internal credential must support the configured local insecure transport")
	}
}
