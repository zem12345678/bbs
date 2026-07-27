package clients

import (
	"context"
	"testing"
)

func TestAdminInternalAuthCredentials(t *testing.T) {
	credentials := adminInternalAuthCredentials{token: "admin-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[adminInternalAuthMetadataKey]; got != "admin-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("admin internal credential must support the configured local insecure transport")
	}
}
