package reaction

import "time"

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() int64
}

type baseEvent struct {
	occurredAt time.Time
}

func newBaseEvent() baseEvent {
	return baseEvent{occurredAt: time.Now()}
}

func (e baseEvent) OccurredAt() time.Time { return e.occurredAt }

type EntityReactionEvent struct {
	baseEvent
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	UserID     int64  `json:"user_id"`
	Count      int64  `json:"count"`
	Changed    bool   `json:"changed"`
	eventName  string
}

func NewLikedEvent(ref EntityRef, userID, count int64, changed bool) EntityReactionEvent {
	return newEntityReactionEvent("reaction.liked.v1", ref, userID, count, changed)
}

func NewUnlikedEvent(ref EntityRef, userID, count int64, changed bool) EntityReactionEvent {
	return newEntityReactionEvent("reaction.unliked.v1", ref, userID, count, changed)
}

func NewFavoritedEvent(ref EntityRef, userID, count int64, changed bool) EntityReactionEvent {
	return newEntityReactionEvent("reaction.favorited.v1", ref, userID, count, changed)
}

func NewUnfavoritedEvent(ref EntityRef, userID, count int64, changed bool) EntityReactionEvent {
	return newEntityReactionEvent("reaction.unfavorited.v1", ref, userID, count, changed)
}

func newEntityReactionEvent(name string, ref EntityRef, userID, count int64, changed bool) EntityReactionEvent {
	return EntityReactionEvent{
		baseEvent:  newBaseEvent(),
		EntityType: string(ref.Type),
		EntityID:   ref.ID,
		UserID:     userID,
		Count:      count,
		Changed:    changed,
		eventName:  name,
	}
}

func (e EntityReactionEvent) EventName() string  { return e.eventName }
func (e EntityReactionEvent) AggregateID() int64 { return e.EntityID }

type ReportSubmittedEvent struct {
	baseEvent
	ReportID    int64  `json:"report_id"`
	EntityType  string `json:"entity_type"`
	EntityID    int64  `json:"entity_id"`
	ReporterID  int64  `json:"reporter_id"`
	Reason      string `json:"reason"`
	Description string `json:"description"`
}

func NewReportSubmittedEvent(report *Report) ReportSubmittedEvent {
	if report == nil {
		return ReportSubmittedEvent{baseEvent: newBaseEvent()}
	}
	return ReportSubmittedEvent{
		baseEvent:   newBaseEvent(),
		ReportID:    report.ID,
		EntityType:  string(report.Entity.Type),
		EntityID:    report.Entity.ID,
		ReporterID:  report.ReporterID,
		Reason:      report.Reason,
		Description: report.Description,
	}
}

func (e ReportSubmittedEvent) EventName() string  { return "reaction.reported.v1" }
func (e ReportSubmittedEvent) AggregateID() int64 { return e.ReportID }
