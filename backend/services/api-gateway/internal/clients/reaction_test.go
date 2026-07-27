package clients

import (
	"context"
	"testing"
)

func TestReactionInternalAuthCredentials(t *testing.T) {
	credentials := reactionInternalAuthCredentials{token: "reaction-internal-token"}
	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[reactionInternalAuthMetadataKey]; got != "reaction-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("reaction internal credential must support the configured local insecure transport")
	}
}
