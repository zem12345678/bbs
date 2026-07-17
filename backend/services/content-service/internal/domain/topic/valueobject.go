package topic

type Status int32

const (
	StatusDraft     Status = 1
	StatusPublished Status = 2
	StatusHidden    Status = 3
	StatusArchived  Status = 4
	StatusArchiving Status = 5
)

type Type string

const (
	TypeTopic Type = "topic"
	TypeTweet Type = "tweet"
	TypeQA    Type = "qa"
)

type QAStatus string

const (
	QAStatusOpen     QAStatus = "open"
	QAStatusResolved QAStatus = "resolved"
)

func NormalizeType(value string) Type {
	switch Type(value) {
	case TypeTweet:
		return TypeTweet
	case TypeQA:
		return TypeQA
	default:
		return TypeTopic
	}
}

func (s Status) CanReadPublicly() bool {
	return s == StatusPublished
}
