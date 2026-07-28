package popularity

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestStoreRanksChatRoomsByActivity(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewStore(client)

	require.NoError(t, store.RecordChatRoomActivity(context.Background(), "ab12cd3e"))
	require.NoError(t, store.RecordChatRoomActivity(context.Background(), "AB12CD3E"))
	require.NoError(t, store.RecordChatRoomActivity(context.Background(), "ZX90QWER"))

	entries, err := store.ListChatRooms(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, []Entry{
		{Key: "AB12CD3E", Score: 2},
		{Key: "ZX90QWER", Score: 1},
	}, entries)
}

func TestStoreRanksResourcesByVisits(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewStore(client)

	require.NoError(t, store.RecordResourceVisit(context.Background(), 9))
	require.NoError(t, store.RecordResourceVisit(context.Background(), 7))
	require.NoError(t, store.RecordResourceVisit(context.Background(), 9))

	entries, err := store.ListResources(context.Background(), 5)
	require.NoError(t, err)
	require.Equal(t, []Entry{
		{Key: "9", Score: 2},
		{Key: "7", Score: 1},
	}, entries)
}
