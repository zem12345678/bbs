package clients

import (
	"context"
	"testing"
)

func TestChatInternalAuthCredentials(t *testing.T) {
	credentials := chatInternalAuthCredentials{token: "chat-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[chatInternalAuthMetadataKey]; got != "chat-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("chat internal credential must support the configured local insecure transport")
	}
}
