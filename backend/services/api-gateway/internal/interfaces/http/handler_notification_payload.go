package http

import (
	"strconv"

	"api-gateway/api/proto/notificationpb"
)

type notificationPayloadView struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	ActorID    any    `json:"actor_id"`
	EntityType string `json:"entity_type"`
	EntityID   any    `json:"entity_id"`
	SourceID   any    `json:"source_id"`
	Read       bool   `json:"read"`
	CreatedAt  int64  `json:"created_at"`
	ReadAt     int64  `json:"read_at"`
}

func toNotificationPayload(item *notificationpb.Notification) notificationPayloadView {
	if item == nil {
		return notificationPayloadView{}
	}
	return notificationPayloadView{
		ID: strconv.FormatInt(item.GetId(), 10), UserID: strconv.FormatInt(item.GetUserId(), 10),
		Type: item.GetType(), Title: item.GetTitle(), Content: item.GetContent(),
		ActorID: nullableEntityID(item.GetActorId()), EntityType: item.GetEntityType(),
		EntityID: nullableEntityID(item.GetEntityId()), SourceID: nullableEntityID(item.GetSourceId()),
		Read: item.GetRead(), CreatedAt: item.GetCreatedAt(), ReadAt: item.GetReadAt(),
	}
}
