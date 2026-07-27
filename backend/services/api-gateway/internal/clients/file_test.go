package clients

import (
	"context"
	"testing"
)

func TestFileInternalAuthCredentials(t *testing.T) {
	credentials := fileInternalAuthCredentials{token: "file-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[fileInternalAuthMetadataKey]; got != "file-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("file internal credential must support the configured local insecure transport")
	}
}
