package article

import (
	"context"
	"errors"
)

const (
	NoteChartSpanHour = "hour"
	NoteChartSpanDay  = "day"

	DefaultNoteChartLimit    = 30
	MaxNoteChartLimit        = 500
	MaxNoteChartOffsetMillis = int64(8640000000000000)
)

var (
	ErrNoteChartSpanInvalid                  = errors.New("note chart span invalid")
	ErrNoteChartLimitInvalid                 = errors.New("note chart limit invalid")
	ErrNoteChartOffsetInvalid                = errors.New("note chart offset invalid")
	ErrNoteChartUserInvalid                  = errors.New("note chart user invalid")
	ErrNoteChartRepositoryUnavailable        = errors.New("note chart repository unavailable")
	ErrActiveUsersChartRepositoryUnavailable = errors.New("active users chart repository unavailable")
)

type NoteChartQuery struct {
	Span   string
	Limit  int
	Offset *int64
	UserID int64
}

type NoteChartDiffs struct {
	Normal   []int64
	Reply    []int64
	Renote   []int64
	WithFile []int64
}

type NoteChartSeries struct {
	Total []int64
	Inc   []int64
	Dec   []int64
	Diffs NoteChartDiffs
}

type NoteChart struct {
	Local  NoteChartSeries
	Remote NoteChartSeries
}

type NoteChartRepository interface {
	GetNoteChart(context.Context, NoteChartQuery) (NoteChart, error)
}
