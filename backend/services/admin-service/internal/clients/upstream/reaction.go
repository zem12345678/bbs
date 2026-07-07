package upstream

import (
	"context"

	"admin/api/proto/reactionpb"
	domain "admin/internal/domain/admin"
)

func (c *Clients) ListReports(ctx context.Context, status int32, entityType string, limit int32, offset int32) (domain.ReportList, error) {
	resp, err := c.reaction.ListReports(ctx, &reactionpb.ListReportsRequest{Status: status, EntityType: entityType, Limit: limit, Offset: offset})
	if err != nil {
		return domain.ReportList{}, err
	}
	items := make([]domain.Report, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainReport(item))
	}
	return domain.ReportList{Items: items, Total: resp.GetTotal()}, nil
}

func (c *Clients) GetReport(ctx context.Context, id int64) (domain.Report, error) {
	resp, err := c.reaction.GetReport(ctx, &reactionpb.GetReportRequest{Id: id})
	if err != nil {
		return domain.Report{}, err
	}
	return toDomainReport(resp.GetReport()), nil
}

func (c *Clients) AuditReport(ctx context.Context, id int64, status int32, handlerID int64, auditNote string, targetAction string) (domain.Report, error) {
	resp, err := c.reaction.AuditReport(ctx, &reactionpb.AuditReportRequest{Id: id, Status: status, HandlerId: handlerID, AuditNote: auditNote, TargetAction: targetAction})
	if err != nil {
		return domain.Report{}, err
	}
	return toDomainReport(resp.GetReport()), nil
}

func toDomainReport(r *reactionpb.ReportInfo) domain.Report {
	if r == nil {
		return domain.Report{}
	}
	var entity domain.EntityRef
	if r.GetEntity() != nil {
		entity = domain.EntityRef{EntityType: r.GetEntity().GetEntityType(), EntityID: r.GetEntity().GetEntityId()}
	}
	return domain.Report{
		ID:           r.GetId(),
		Entity:       entity,
		ReporterID:   r.GetReporterId(),
		Reason:       r.GetReason(),
		Description:  r.GetDescription(),
		Status:       r.GetStatus(),
		HandledBy:    r.GetHandledBy(),
		HandledAt:    r.GetHandledAt(),
		AuditNote:    r.GetAuditNote(),
		TargetAction: r.GetTargetAction(),
		CreatedAt:    r.GetCreatedAt(),
		UpdatedAt:    r.GetUpdatedAt(),
	}
}
