package clients

import (
	"context"
	"testing"
)

func TestUserInternalAuthCredentials(t *testing.T) {
	credentials := userInternalAuthCredentials{token: "user-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[userInternalAuthMetadataKey]; got != "user-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("user internal credential must support the configured local insecure transport")
	}
}
