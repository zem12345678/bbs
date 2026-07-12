package credit

import "testing"

func TestServiceNameNormalizesLegacyCreditService(t *testing.T) {
	if got := serviceName("credit-service"); got != "bbs-credit-service" {
		t.Fatalf("serviceName = %q", got)
	}
}
