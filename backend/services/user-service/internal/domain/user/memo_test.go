package user

import (
	"strings"
	"testing"
)

func TestUserMemoNormalizationAndLimit(t *testing.T) {
	if got := NormalizeUserMemo("  line one\nline two  "); got != "line one\nline two" {
		t.Fatalf("NormalizeUserMemo() = %q", got)
	}
	if !ValidUserMemo(strings.Repeat("界", MaxUserMemoRunes)) {
		t.Fatal("memo at rune limit should be valid")
	}
	if ValidUserMemo(strings.Repeat("界", MaxUserMemoRunes+1)) {
		t.Fatal("memo over rune limit should be invalid")
	}
}
