package user

import (
	"strings"
	"unicode/utf8"
)

const MaxUserMemoRunes = 2048

// NormalizeUserMemo trims the value while preserving the user's line breaks
// and other content. Empty values mean that the memo should be deleted.
func NormalizeUserMemo(value string) string {
	return strings.TrimSpace(value)
}

func ValidUserMemo(value string) bool {
	return utf8.RuneCountInString(value) <= MaxUserMemoRunes
}
