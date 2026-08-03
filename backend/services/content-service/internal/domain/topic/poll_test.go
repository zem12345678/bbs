package topic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewNormalizesPoll(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	topic, err := New(1001, CreateCmd{
		Slug: "poll-topic", Title: "Poll", Body: "Body", AuthorID: 42,
		Poll: &PollInput{Enabled: true, Multiple: true, Choices: []string{" First ", "Second"}, ExpiresAt: &expiresAt},
	})

	require.NoError(t, err)
	require.NotNil(t, topic.Poll)
	require.True(t, topic.Poll.Multiple)
	require.Equal(t, []PollChoice{{Index: 0, Text: "First"}, {Index: 1, Text: "Second"}}, topic.Poll.Choices)
}

func TestNewRejectsInvalidPoll(t *testing.T) {
	tests := []struct {
		name  string
		input PollInput
		err   error
	}{
		{name: "too few", input: PollInput{Enabled: true, Choices: []string{"one"}}, err: ErrPollChoicesInvalid},
		{name: "blank", input: PollInput{Enabled: true, Choices: []string{"one", " "}}, err: ErrPollChoiceInvalid},
		{name: "duplicate", input: PollInput{Enabled: true, Choices: []string{"One", " one "}}, err: ErrPollChoiceDuplicate},
		{name: "expired", input: PollInput{Enabled: true, Choices: []string{"one", "two"}, ExpiresAt: timePointer(time.Now().Add(-time.Minute))}, err: ErrPollExpiryInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(1001, CreateCmd{Slug: "poll-topic", Title: "Poll", Body: "Body", AuthorID: 42, Poll: &tt.input})
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestUpdateWithoutPollInputPreservesPoll(t *testing.T) {
	topic, err := New(1001, CreateCmd{
		Slug: "poll-topic", Title: "Poll", Body: "Body", AuthorID: 42,
		Poll: &PollInput{Enabled: true, Choices: []string{"one", "two"}},
	})
	require.NoError(t, err)

	err = topic.Update(UpdateCmd{Title: "Updated", Body: "Body"})

	require.NoError(t, err)
	require.NotNil(t, topic.Poll)
	require.Len(t, topic.Poll.Choices, 2)
}

func timePointer(value time.Time) *time.Time { return &value }
