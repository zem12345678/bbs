package persistence

import (
	"context"
	"fmt"
	"time"

	domain "comment-service/internal/domain/comment"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
)

type noteChartBucket struct {
	Start time.Time `bson:"_id"`
	Value int64     `bson:"value"`
}

func (r *Repository) GetNoteChart(ctx context.Context, q domain.NoteChartQuery) (domain.NoteChart, error) {
	firstStart, lastStart, step, unit := noteChartWindow(q, time.Now().UTC())
	initialFilter := bson.M{"createdAt": bson.M{"$lt": firstStart}}
	if q.UserID > 0 {
		initialFilter["authorId"] = q.UserID
	}
	initial, err := r.comments().CountDocuments(ctx, initialFilter)
	if err != nil {
		return domain.NoteChart{}, fmt.Errorf("count initial comments: %w", err)
	}
	increments, err := r.noteChartBuckets(ctx, "createdAt", firstStart, lastStart.Add(step), unit, q.UserID)
	if err != nil {
		return domain.NoteChart{}, err
	}

	local := fixedNoteChartSeries(q.Limit)
	total := initial
	for index := 0; index < q.Limit; index++ {
		bucketStart := firstStart.Add(time.Duration(index) * step)
		inc := increments[bucketStart.UnixMilli()]
		total += inc
		output := q.Limit - 1 - index
		local.Total[output] = total
		local.Inc[output] = inc
		local.Diffs.Reply[output] = inc
	}
	return domain.NoteChart{Local: local, Remote: fixedNoteChartSeries(q.Limit)}, nil
}

func (r *Repository) noteChartBuckets(ctx context.Context, field string, start, end time.Time, unit string, userID int64) (map[int64]int64, error) {
	match := bson.M{field: bson.M{"$gte": start, "$lt": end}}
	if userID > 0 {
		match["authorId"] = userID
	}
	pipeline := drivermongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
				{Key: "date", Value: "$" + field}, {Key: "unit", Value: unit}, {Key: "timezone", Value: "UTC"},
			}}}},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cursor, err := r.comments().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate comment chart buckets: %w", err)
	}
	defer cursor.Close(ctx)
	var rows []noteChartBucket
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode comment chart buckets: %w", err)
	}
	result := make(map[int64]int64, len(rows))
	for _, row := range rows {
		result[row.Start.UTC().UnixMilli()] = row.Value
	}
	return result, nil
}

func noteChartWindow(q domain.NoteChartQuery, now time.Time) (time.Time, time.Time, time.Duration, string) {
	step := time.Hour
	unit := domain.NoteChartSpanHour
	if q.Span == domain.NoteChartSpanDay {
		step = 24 * time.Hour
		unit = domain.NoteChartSpanDay
	}
	reference := now.UTC()
	if q.Offset != nil {
		reference = time.UnixMilli(*q.Offset).UTC()
	}
	lastStart := reference.Truncate(step)
	if q.Offset != nil && !lastStart.Equal(reference) {
		lastStart = lastStart.Add(step)
	}
	firstStart := lastStart.Add(-time.Duration(q.Limit-1) * step)
	return firstStart, lastStart, step, unit
}

func fixedNoteChartSeries(length int) domain.NoteChartSeries {
	return domain.NoteChartSeries{
		Total: make([]int64, length), Inc: make([]int64, length), Dec: make([]int64, length),
		Diffs: domain.NoteChartDiffs{
			Normal: make([]int64, length), Reply: make([]int64, length),
			Renote: make([]int64, length), WithFile: make([]int64, length),
		},
	}
}

var _ domain.NoteChartRepository = (*Repository)(nil)
