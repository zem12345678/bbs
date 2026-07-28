package popularity

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

const (
	chatRoomsKey = "bbs:popular:chat_rooms"
	resourcesKey = "bbs:popular:resources"
)

type Entry struct {
	Key   string
	Score int64
}

type ChatRoomRecorder interface {
	RecordChatRoomActivity(context.Context, string) error
}

type Store struct {
	redis redis.Cmdable
}

func NewStore(client redis.Cmdable) *Store {
	if client == nil {
		return nil
	}
	return &Store{redis: client}
}

func (s *Store) RecordChatRoomActivity(ctx context.Context, roomNo string) error {
	if s == nil || s.redis == nil {
		return nil
	}
	roomNo = strings.ToUpper(strings.TrimSpace(roomNo))
	if roomNo == "" {
		return nil
	}
	return s.redis.ZIncrBy(ctx, chatRoomsKey, 1, roomNo).Err()
}

func (s *Store) ListChatRooms(ctx context.Context, limit int) ([]Entry, error) {
	return s.list(ctx, chatRoomsKey, limit)
}

func (s *Store) RecordResourceVisit(ctx context.Context, linkID int64) error {
	if s == nil || s.redis == nil || linkID <= 0 {
		return nil
	}
	return s.redis.ZIncrBy(ctx, resourcesKey, 1, strconv.FormatInt(linkID, 10)).Err()
}

func (s *Store) ListResources(ctx context.Context, limit int) ([]Entry, error) {
	return s.list(ctx, resourcesKey, limit)
}

func (s *Store) list(ctx context.Context, key string, limit int) ([]Entry, error) {
	if s == nil || s.redis == nil {
		return []Entry{}, nil
	}
	limit = boundedLimit(limit)
	values, err := s.redis.ZRevRangeWithScores(ctx, key, 0, int64(limit)-1).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(values))
	for _, value := range values {
		member := strings.TrimSpace(fmt.Sprint(value.Member))
		if member == "" {
			continue
		}
		entries = append(entries, Entry{Key: member, Score: int64(math.Round(value.Score))})
	}
	return entries, nil
}

func boundedLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	if limit > 20 {
		return 20
	}
	return limit
}
