package clients

import (
	"context"
	"testing"
)

func TestCommentInternalAuthCredentials(t *testing.T) {
	credentials := commentInternalAuthCredentials{token: "comment-internal-token"}
	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[commentInternalAuthMetadataKey]; got != "comment-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("comment internal credential must support the configured local insecure transport")
	}
}
