package persistence

import (
	"testing"
	"time"

	channelDomain "content-service/internal/domain/channel"
	topicDomain "content-service/internal/domain/topic"
)

func TestChannelPersistenceMappingPreservesOptionalCategoryAndAggregates(t *testing.T) {
	postedAt := time.Now().UTC()
	row := channelViewPO{
		ID:              1,
		OwnerID:         2,
		Name:            "name",
		Color:           channelDomain.DefaultColor,
		FollowersCount:  3,
		TopicsCount:     4,
		LastPostedAt:    &postedAt,
		ViewerFollowing: true,
		ViewerFavorited: true,
	}
	entity := channelToEntity(&row)
	if entity.CategoryID != 0 || entity.FollowersCount != 3 || entity.TopicsCount != 4 {
		t.Fatalf("unexpected mapped channel: %#v", entity)
	}
	if !entity.ViewerFollowing || !entity.ViewerFavorited || entity.LastPostedAt == nil {
		t.Fatalf("viewer aggregates were not preserved: %#v", entity)
	}
	if channelToPO(entity).CategoryID != nil {
		t.Fatal("uncategorized channel must persist a NULL category_id")
	}
}

func TestTopicPersistenceMappingPreservesOptionalChannel(t *testing.T) {
	topic := &topicDomain.Topic{ID: 1, ChannelID: 9}
	po := topicToPO(topic)
	if po.ChannelID == nil || *po.ChannelID != 9 {
		t.Fatalf("channel_id PO = %v, want 9", po.ChannelID)
	}
	mapped := topicToEntity(&po)
	if mapped.ChannelID != 9 {
		t.Fatalf("mapped ChannelID = %d, want 9", mapped.ChannelID)
	}

	topic.ChannelID = 0
	po = topicToPO(topic)
	if po.ChannelID != nil {
		t.Fatalf("zero channel_id must persist as NULL, got %v", *po.ChannelID)
	}
}
