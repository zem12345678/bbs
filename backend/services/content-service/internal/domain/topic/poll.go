package topic

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MinPollChoices     = 2
	MaxPollChoices     = 10
	MaxPollChoiceRunes = 80
)

type PollChoice struct {
	Index int32
	Text  string
	Votes int64
}

type Poll struct {
	Multiple    bool
	Choices     []PollChoice
	ExpiresAt   *time.Time
	TotalVoters int64
	MyChoices   []int32
}

type PollInput struct {
	Enabled   bool
	Multiple  bool
	Choices   []string
	ExpiresAt *time.Time
}

func normalizePollInput(input *PollInput, now time.Time) (*Poll, error) {
	if input == nil || !input.Enabled {
		return nil, nil
	}
	if len(input.Choices) < MinPollChoices || len(input.Choices) > MaxPollChoices {
		return nil, ErrPollChoicesInvalid
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
		return nil, ErrPollExpiryInvalid
	}

	seen := make(map[string]struct{}, len(input.Choices))
	choices := make([]PollChoice, 0, len(input.Choices))
	for index, value := range input.Choices {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > MaxPollChoiceRunes {
			return nil, ErrPollChoiceInvalid
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return nil, ErrPollChoiceDuplicate
		}
		seen[key] = struct{}{}
		choices = append(choices, PollChoice{Index: int32(index), Text: value})
	}

	return &Poll{
		Multiple:  input.Multiple,
		Choices:   choices,
		ExpiresAt: input.ExpiresAt,
	}, nil
}
