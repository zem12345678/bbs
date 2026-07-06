package article

type Status int32

const (
	StatusDraft     Status = 1
	StatusPublished Status = 2
	StatusHidden    Status = 3
	StatusArchived  Status = 4
)

func (s Status) CanReadPublicly() bool {
	return s == StatusPublished
}
