package persistence

import (
	"context"
	"fmt"
	"time"

	domain "comment-service/internal/domain/comment"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
)

type activeUsersChartRow struct {
	Start         time.Time `bson:"_id"`
	WriterUserIDs []int64   `bson:"writerUserIds"`
}

func (r *Repository) GetActiveUsersChart(ctx context.Context, q domain.NoteChartQuery) (domain.ActiveUsersChart, error) {
	firstStart, lastStart, step, unit := noteChartWindow(q, time.Now().UTC())
	pipeline := drivermongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"createdAt": bson.M{"$gte": firstStart, "$lt": lastStart.Add(step)}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "start", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
					{Key: "date", Value: "$createdAt"}, {Key: "unit", Value: unit}, {Key: "timezone", Value: "UTC"},
				}}}},
				{Key: "authorId", Value: "$authorId"},
			}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_id.start", Value: 1}, {Key: "_id.authorId", Value: 1}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$_id.start"},
			{Key: "writerUserIds", Value: bson.D{{Key: "$push", Value: "$_id.authorId"}}},
		}}},
	}
	cursor, err := r.comments().Aggregate(ctx, pipeline)
	if err != nil {
		return domain.ActiveUsersChart{}, fmt.Errorf("aggregate active comment writers: %w", err)
	}
	defer cursor.Close(ctx)
	var rows []activeUsersChartRow
	if err := cursor.All(ctx, &rows); err != nil {
		return domain.ActiveUsersChart{}, fmt.Errorf("decode active comment writers: %w", err)
	}
	byStart := make(map[int64][]int64, len(rows))
	for _, row := range rows {
		byStart[row.Start.UTC().UnixMilli()] = row.WriterUserIDs
	}
	buckets := make([]domain.ActiveUsersChartBucket, 0, q.Limit)
	for index := 0; index < q.Limit; index++ {
		start := lastStart.Add(-time.Duration(index) * step)
		writerUserIDs := byStart[start.UnixMilli()]
		if writerUserIDs == nil {
			writerUserIDs = []int64{}
		}
		buckets = append(buckets, domain.ActiveUsersChartBucket{WriterUserIDs: writerUserIDs})
	}
	return domain.ActiveUsersChart{Buckets: buckets}, nil
}

var _ domain.ActiveUsersChartRepository = (*Repository)(nil)
