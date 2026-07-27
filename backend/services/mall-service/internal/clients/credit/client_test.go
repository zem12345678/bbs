package credit

import (
	"context"
	"testing"
)

func TestServiceNameNormalizesLegacyCreditService(t *testing.T) {
	if got := serviceName("credit-service"); got != "bbs-credit-service" {
		t.Fatalf("serviceName = %q", got)
	}
}

func TestInternalAuthCredentials(t *testing.T) {
	credentials := internalAuthCredentials{token: "credit-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[internalAuthMetadataKey]; got != "credit-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("credit internal credential must support the configured local insecure transport")
	}
}
