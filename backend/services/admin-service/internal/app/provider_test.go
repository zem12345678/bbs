package app

import "testing"

func TestServiceNameDefaultNormalizesLegacyNames(t *testing.T) {
	tests := map[string]string{
		"user-service":     "bbs-user-service",
		"reaction-service": "bbs-reaction-service",
		"content-service":  "bbs-content-service",
		"comment-service":  "bbs-comment-service",
	}

	for input, want := range tests {
		if got := ServiceNameDefault(input, "fallback"); got != want {
			t.Fatalf("ServiceNameDefault(%q) = %q, want %q", input, got, want)
		}
	}
}
