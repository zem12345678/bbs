package topic

type Status int32

const (
	StatusDraft     Status = 1
	StatusPublished Status = 2
	StatusHidden    Status = 3
	StatusArchived  Status = 4
)

type Type string

const (
	TypeTopic Type = "topic"
	TypeTweet Type = "tweet"
)

func NormalizeType(value string) Type {
	switch Type(value) {
	case TypeTweet:
		return TypeTweet
	default:
		return TypeTopic
	}
}

func (s Status) CanReadPublicly() bool {
	return s == StatusPublished
}
