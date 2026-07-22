package persistence

import "testing"

func TestLeaderboardMemberRoundTripPreservesNumericOrder(t *testing.T) {
	for _, userID := range []int64{1, 7, 42, 9223372036854775807} {
		member := leaderboardMember(userID)
		parsed, ok := parseLeaderboardMember(member)
		if !ok || parsed != userID {
			t.Fatalf("leaderboard member %q parsed as %d, %v", member, parsed, ok)
		}
	}
	if leaderboardMember(42) >= leaderboardMember(100) {
		t.Fatalf("leaderboard members must sort numerically: %q >= %q", leaderboardMember(42), leaderboardMember(100))
	}
}

func TestParseLeaderboardMemberRejectsInvalidValues(t *testing.T) {
	for _, member := range []string{"", "u:42", "u:0000000000000000000", "x:0000000000000000042"} {
		if _, ok := parseLeaderboardMember(member); ok {
			t.Fatalf("invalid leaderboard member %q was accepted", member)
		}
	}
}
