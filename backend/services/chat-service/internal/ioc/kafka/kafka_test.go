package kafka

import (
	"testing"

	"github.com/segmentio/kafka-go/sasl/scram"
)

func TestScramAlgorithmNormalizesConfiguredValue(t *testing.T) {
	tests := []struct {
		configured ScramAlgorithm
		want       scram.Algorithm
	}{
		{configured: SHA256, want: scram.SHA256},
		{configured: "sha256", want: scram.SHA256},
		{configured: " SHA256 ", want: scram.SHA256},
		{configured: SHA512, want: scram.SHA512},
	}

	for _, test := range tests {
		t.Run(string(test.configured), func(t *testing.T) {
			if got := scramAlgorithm(test.configured).Name(); got != test.want.Name() {
				t.Fatalf("scramAlgorithm(%q) = %q, want %q", test.configured, got, test.want.Name())
			}
		})
	}
}
