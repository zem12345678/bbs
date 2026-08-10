package admin

import (
	"context"
	"errors"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestAuditReportHidesArticleTargetBeforeAudit(t *testing.T) {
	auth := &fakeAuditAuthorizer{}
	reports := &fakeAuditReportGateway{
		report: domain.Report{
			ID:     10,
			Status: domain.ReportStatusPending,
			Entity: domain.EntityRef{EntityType: "article", EntityID: 99},
		},
	}
	content := &fakeAuditContentGateway{}
	service := &Service{auth: auth, reports: reports, content: content}

	report, err := service.AuditReport(context.Background(), testAuditActor(), 10, domain.ReportStatusResolved, "handled", " hide ")
	if err != nil {
		t.Fatalf("AuditReport() error = %v", err)
	}
	if !reports.getCalled {
		t.Fatalf("report should be loaded before target action")
	}
	if content.hiddenArticleID != 99 {
		t.Fatalf("hidden article id = %d, want 99", content.hiddenArticleID)
	}
	if !reports.auditCalled {
		t.Fatalf("report audit should be persisted")
	}
	if reports.auditTargetAction != domain.ReportTargetActionHide {
		t.Fatalf("audit target action = %q, want hide", reports.auditTargetAction)
	}
	if report.TargetAction != domain.ReportTargetActionHide {
		t.Fatalf("report target action = %q, want hide", report.TargetAction)
	}
	if !auth.authorized(domain.ActionAuditReport) || !auth.authorized(domain.ActionHideArticle) {
		t.Fatalf("expected audit and hide article permissions, got %v", auth.actions)
	}
}

func TestAuditReportRejectsHideActionForRejectedStatus(t *testing.T) {
	reports := &fakeAuditReportGateway{}
	content := &fakeAuditContentGateway{}
	service := &Service{auth: &fakeAuditAuthorizer{}, reports: reports, content: content}

	_, err := service.AuditReport(context.Background(), testAuditActor(), 10, 3, "not violation", domain.ReportTargetActionHide)
	if !errors.Is(err, domain.ErrInvalidReportAction) {
		t.Fatalf("AuditReport() error = %v, want ErrInvalidReportAction", err)
	}
	if reports.getCalled || reports.auditCalled {
		t.Fatalf("report gateway should not be called for invalid action/status combination")
	}
	if content.hiddenArticleID != 0 {
		t.Fatalf("content should not be hidden for invalid action/status combination")
	}
}

func testAuditActor() domain.Actor {
	return domain.Actor{ID: 1, Username: "admin"}
}

type fakeAuditAuthorizer struct {
	actions []domain.Action
}

func (f *fakeAuditAuthorizer) Authorize(_ context.Context, _ domain.Actor, action domain.Action) error {
	f.actions = append(f.actions, action)
	return nil
}

func (f *fakeAuditAuthorizer) Reload(context.Context) error {
	return nil
}

func (f *fakeAuditAuthorizer) authorized(action domain.Action) bool {
	for _, item := range f.actions {
		if item == action {
			return true
		}
	}
	return false
}

type fakeAuditReportGateway struct {
	report            domain.Report
	getCalled         bool
	auditCalled       bool
	auditTargetAction string
}

func (f *fakeAuditReportGateway) ListReports(context.Context, int32, string, int32, int32) (domain.ReportList, error) {
	return domain.ReportList{}, nil
}

func (f *fakeAuditReportGateway) GetReport(context.Context, int64) (domain.Report, error) {
	f.getCalled = true
	return f.report, nil
}

func (f *fakeAuditReportGateway) AuditReport(_ context.Context, id int64, status int32, handlerID int64, auditNote string, targetAction string) (domain.Report, error) {
	f.auditCalled = true
	f.auditTargetAction = targetAction
	report := f.report
	report.ID = id
	report.Status = status
	report.HandledBy = handlerID
	report.AuditNote = auditNote
	report.TargetAction = targetAction
	return report, nil
}

type fakeAuditContentGateway struct {
	hiddenArticleID int64
	hiddenTopicID   int64
}

func (f *fakeAuditContentGateway) ListArticles(context.Context, int32, string, int64, int32, int32) (domain.ArticleList, error) {
	return domain.ArticleList{}, nil
}

func (f *fakeAuditContentGateway) PublishArticle(context.Context, int64) (domain.Article, error) {
	return domain.Article{}, nil
}

func (f *fakeAuditContentGateway) HideArticle(_ context.Context, id int64) (domain.Article, error) {
	f.hiddenArticleID = id
	return domain.Article{ID: id, Status: 3}, nil
}

func (f *fakeAuditContentGateway) ArchiveArticle(context.Context, int64) (domain.Article, error) {
	return domain.Article{}, nil
}

func (f *fakeAuditContentGateway) ListTopics(context.Context, int32, string, string, int64, int64, int32, int32) (domain.TopicList, error) {
	return domain.TopicList{}, nil
}

func (f *fakeAuditContentGateway) PublishTopic(context.Context, int64) (domain.Topic, error) {
	return domain.Topic{}, nil
}

func (f *fakeAuditContentGateway) HideTopic(_ context.Context, id int64) (domain.Topic, error) {
	f.hiddenTopicID = id
	return domain.Topic{ID: id, Status: 3}, nil
}

func (f *fakeAuditContentGateway) ArchiveTopic(context.Context, int64) (domain.Topic, error) {
	return domain.Topic{}, nil
}

func (f *fakeAuditContentGateway) ListChannels(context.Context, string, int64, int32, int32, int32) (domain.ChannelList, error) {
	return domain.ChannelList{}, nil
}

func (f *fakeAuditContentGateway) SetChannelFeatured(context.Context, int64, bool) (domain.Channel, error) {
	return domain.Channel{}, nil
}

func (f *fakeAuditContentGateway) SetChannelArchived(context.Context, int64, bool) (domain.Channel, error) {
	return domain.Channel{}, nil
}

func (f *fakeAuditContentGateway) ListCategories(context.Context, int32, int32, int32) (domain.CategoryList, error) {
	return domain.CategoryList{}, nil
}

func (f *fakeAuditContentGateway) CreateCategory(context.Context, domain.UpsertCategoryCommand) (domain.Category, error) {
	return domain.Category{}, nil
}

func (f *fakeAuditContentGateway) UpdateCategory(context.Context, domain.UpsertCategoryCommand) (domain.Category, error) {
	return domain.Category{}, nil
}

func (f *fakeAuditContentGateway) DeleteCategory(context.Context, int64) error {
	return nil
}
