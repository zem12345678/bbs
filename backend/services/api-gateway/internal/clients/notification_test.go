package clients

import (
	"context"
	"testing"
)

func TestNotificationInternalAuthCredentials(t *testing.T) {
	credentials := notificationInternalAuthCredentials{token: "notification-internal-token"}
	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[notificationInternalAuthMetadataKey]; got != "notification-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("notification internal credential must support the configured local insecure transport")
	}
}
