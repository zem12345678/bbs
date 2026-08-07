package persistence

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	domain "comment-service/internal/domain/comment"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestNoteChartRepositoryMongoIntegration(t *testing.T) {
	if os.Getenv("BBS_MONGO_SMOKE") != "1" {
		t.Skip("set BBS_MONGO_SMOKE=1 to run MongoDB note-chart test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(getenv("BBS_MONGO_URI", "mongodb://127.0.0.1:27017")))
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	repo := NewRepository(client.Database(getenv("BBS_MONGO_DATABASE", "bbs_comment")))

	base := time.Now().UnixNano()
	userID := base + 100
	anchor := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	hiddenAt := anchor.Add(-time.Hour + 30*time.Minute)
	rows := []commentDocument{
		{ID: base + 1, AuthorID: userID, CreatedAt: anchor.Add(-3 * time.Hour), UpdatedAt: anchor},
		{ID: base + 2, AuthorID: userID, CreatedAt: anchor.Add(-2*time.Hour + 15*time.Minute), UpdatedAt: anchor},
		{ID: base + 3, AuthorID: base + 200, CreatedAt: anchor.Add(-time.Hour + 15*time.Minute), UpdatedAt: anchor},
		{ID: base + 4, AuthorID: userID, CreatedAt: anchor.Add(20 * time.Minute), UpdatedAt: anchor, DeletedAt: &hiddenAt},
	}
	if _, err := repo.comments().InsertMany(ctx, rows); err != nil {
		t.Fatalf("insert comments: %v", err)
	}
	defer func() {
		_, _ = repo.comments().DeleteMany(context.Background(), bson.M{"_id": bson.M{"$gte": base + 1, "$lte": base + 4}})
	}()

	offset := anchor.UnixMilli()
	got, err := repo.GetNoteChart(ctx, domain.NoteChartQuery{Span: "hour", Limit: 3, Offset: &offset, UserID: userID})
	if err != nil {
		t.Fatalf("GetNoteChart() error = %v", err)
	}
	assertCommentNoteSeries(t, got.Local, []int64{3, 2, 2}, []int64{1, 0, 1})
	assertCommentNoteSeries(t, got.Remote, []int64{0, 0, 0}, []int64{0, 0, 0})
}

func assertCommentNoteSeries(t *testing.T, got domain.NoteChartSeries, total, inc []int64) {
	t.Helper()
	zeros := make([]int64, len(total))
	if !reflect.DeepEqual(got.Total, total) || !reflect.DeepEqual(got.Inc, inc) || !reflect.DeepEqual(got.Dec, zeros) ||
		!reflect.DeepEqual(got.Diffs.Normal, zeros) || !reflect.DeepEqual(got.Diffs.Reply, inc) ||
		!reflect.DeepEqual(got.Diffs.Renote, zeros) || !reflect.DeepEqual(got.Diffs.WithFile, zeros) {
		t.Fatalf("series = %+v, want total=%v inc=%v", got, total, inc)
	}
}
