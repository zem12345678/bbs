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

func TestActiveUsersChartRepositoryMongoIntegration(t *testing.T) {
	if os.Getenv("BBS_MONGO_SMOKE") != "1" {
		t.Skip("set BBS_MONGO_SMOKE=1 to run MongoDB active-users chart test")
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
	anchor := time.Date(2098, time.August, 7, 12, 0, 0, 0, time.UTC)
	rows := []commentDocument{
		{ID: base + 1, AuthorID: base + 101, CreatedAt: anchor.Add(-2*time.Hour + 5*time.Minute), UpdatedAt: anchor},
		{ID: base + 2, AuthorID: base + 101, CreatedAt: anchor.Add(-2*time.Hour + 25*time.Minute), UpdatedAt: anchor},
		{ID: base + 3, AuthorID: base + 101, CreatedAt: anchor.Add(-time.Hour + 5*time.Minute), UpdatedAt: anchor},
		{ID: base + 4, AuthorID: base + 102, CreatedAt: anchor.Add(-time.Hour + 25*time.Minute), UpdatedAt: anchor},
		{ID: base + 5, AuthorID: base + 103, CreatedAt: anchor.Add(15 * time.Minute), UpdatedAt: anchor},
	}
	if _, err := repo.comments().InsertMany(ctx, rows); err != nil {
		t.Fatalf("insert comments: %v", err)
	}
	defer func() {
		_, _ = repo.comments().DeleteMany(context.Background(), bson.M{"_id": bson.M{"$gte": base + 1, "$lte": base + 5}})
	}()

	offset := anchor.UnixMilli()
	got, err := repo.GetActiveUsersChart(ctx, domain.NoteChartQuery{Span: "hour", Limit: 3, Offset: &offset})
	if err != nil {
		t.Fatalf("GetActiveUsersChart() error = %v", err)
	}
	want := [][]int64{{base + 103}, {base + 101, base + 102}, {base + 101}}
	for index, bucket := range got.Buckets {
		if !reflect.DeepEqual(bucket.WriterUserIDs, want[index]) {
			t.Fatalf("bucket %d writer ids = %v, want %v", index, bucket.WriterUserIDs, want[index])
		}
	}
}
