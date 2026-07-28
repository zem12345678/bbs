package persistence

import (
	"sort"
	"strings"
	"unicode"
)

type scoredUserRow struct {
	row   userPO
	score int
}

func searchUserRows(rows []userPO, query string, page, pageSize int) ([]userPO, int64) {
	query = normalizeSearchText(query)
	if query == "" {
		return nil, 0
	}
	scored := make([]scoredUserRow, 0, len(rows))
	for _, row := range rows {
		score := userSearchScore(query, row)
		if score > 0 {
			scored = append(scored, scoredUserRow{row: row, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		left, right := scored[i], scored[j]
		if left.score != right.score {
			return left.score > right.score
		}
		if !left.row.CreatedAt.Equal(right.row.CreatedAt) {
			return left.row.CreatedAt.After(right.row.CreatedAt)
		}
		return left.row.ID > right.row.ID
	})
	start := (page - 1) * pageSize
	if start >= len(scored) {
		return []userPO{}, int64(len(scored))
	}
	end := start + pageSize
	if end > len(scored) {
		end = len(scored)
	}
	out := make([]userPO, 0, end-start)
	for _, item := range scored[start:end] {
		out = append(out, item.row)
	}
	return out, int64(len(scored))
}

func userSearchScore(query string, row userPO) int {
	best := 0
	for _, field := range []string{row.Username, row.Nickname, row.Email} {
		best = max(best, fieldSearchScore(query, field))
	}
	return best
}

func fieldSearchScore(query string, field string) int {
	value := normalizeSearchText(field)
	if value == "" {
		return 0
	}
	if value == query {
		return 1000
	}
	if strings.HasPrefix(value, query) {
		return 930
	}
	if strings.Contains(value, query) {
		return 860
	}
	compactQuery := compactSearchText(query)
	compactValue := compactSearchText(value)
	if compactQuery == "" || compactValue == "" {
		return 0
	}
	if compactValue == compactQuery {
		return 980
	}
	if strings.HasPrefix(compactValue, compactQuery) {
		return 910
	}
	if strings.Contains(compactValue, compactQuery) {
		return 840
	}
	best := 0
	if score := fuzzySearchScore(compactQuery, compactValue, 720); score > best {
		best = score
	}
	for _, token := range searchTokens(value) {
		if token == compactQuery {
			best = max(best, 970)
			continue
		}
		if strings.HasPrefix(token, compactQuery) {
			best = max(best, 900)
			continue
		}
		if strings.Contains(token, compactQuery) {
			best = max(best, 820)
			continue
		}
		best = max(best, fuzzySearchScore(compactQuery, token, 700))
	}
	return best
}

func fuzzySearchScore(query, candidate string, base int) int {
	maxDistance := maxUserSearchDistance(len([]rune(query)))
	if maxDistance <= 0 {
		return 0
	}
	distance, ok := levenshteinWithin(query, candidate, maxDistance)
	if !ok {
		return 0
	}
	score := base - distance*35
	if score < 1 {
		return 1
	}
	return score
}

func maxUserSearchDistance(length int) int {
	switch {
	case length < 3:
		return 0
	case length <= 4:
		return 1
	case length <= 8:
		return 2
	default:
		return 3
	}
}

func normalizeSearchText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func compactSearchText(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func searchTokens(value string) []string {
	tokens := strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = compactSearchText(token)
		if token != "" {
			out = append(out, token)
		}
	}
	return out
}

func levenshteinWithin(left, right string, maxDistance int) (int, bool) {
	a := []rune(left)
	b := []rune(right)
	if len(a) == 0 {
		return len(b), len(b) <= maxDistance
	}
	if len(b) == 0 {
		return len(a), len(a) <= maxDistance
	}
	if abs(len(a)-len(b)) > maxDistance {
		return maxDistance + 1, false
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		rowMin := current[0]
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(
				previous[j]+1,
				min(current[j-1]+1, previous[j-1]+cost),
			)
			if current[j] < rowMin {
				rowMin = current[j]
			}
		}
		if rowMin > maxDistance {
			return maxDistance + 1, false
		}
		previous, current = current, previous
	}
	distance := previous[len(b)]
	return distance, distance <= maxDistance
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
