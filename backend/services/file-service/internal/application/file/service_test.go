package file

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"

	domain "file-service/internal/domain/file"
)

func newTestService(repo domain.Repository, charger CreditCharger) *Service {
	return NewService(repo, charger, &membershipEntitlementStub{active: true}, newTestTopicReader(repo))
}

func newTestTopicReader(repo domain.Repository) *topicReaderStub {
	if memory, ok := repo.(*memoryRepository); ok {
		return newPublishedTopicReader(memory.attachment.OwnerID)
	}
	return newPublishedTopicReader(0)
}

func TestAuthorizeDownloadChargesOnlyOnce(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(101, 9, 7))
	charger := &captureCharger{}
	service := newTestService(repo, charger)
	service.now = func() time.Time { return time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC) }

	first, err := service.AuthorizeDownload(context.Background(), 101, 42)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if first.AlreadyAuthorized || first.ChargedCredits != 7 {
		t.Fatalf("first authorization = %+v, want a new 7-credit authorization", first)
	}
	if len(charger.transfers) != 1 {
		t.Fatalf("credit transfers = %d, want 1", len(charger.transfers))
	}
	if command := charger.transfers[0]; command.SourceEventID != "attachment-download:101:42" || command.Amount != 7 || command.PayerUserID != 42 || command.PayeeUserID != 9 || command.DebitReason != "attachment_download" || command.CreditReason != "attachment_sale" {
		t.Fatalf("unexpected transfer command: %+v", command)
	}

	second, err := service.AuthorizeDownload(context.Background(), 101, 42)
	if err != nil {
		t.Fatalf("second AuthorizeDownload() error = %v", err)
	}
	if !second.AlreadyAuthorized || second.ChargedCredits != 0 {
		t.Fatalf("second authorization = %+v, want an already-authorized free retry", second)
	}
	if len(charger.transfers) != 1 {
		t.Fatalf("credit transfers after retry = %d, want 1", len(charger.transfers))
	}
}

func TestAuthorizeDownloadMembershipEnforcement(t *testing.T) {
	t.Run("blocks new paid sale after author membership ends", func(t *testing.T) {
		repo := newMemoryRepository(activeAttachment(115, 9, 7))
		reader := &membershipEntitlementStub{}
		charger := &captureCharger{}
		service := NewService(repo, charger, reader, newPublishedTopicReader(9))

		_, err := service.AuthorizeDownload(context.Background(), 115, 42)
		if err != domain.ErrPaidAttachmentSalesMembershipInactive {
			t.Fatalf("AuthorizeDownload() error = %v, want inactive author membership", err)
		}
		if len(repo.downloads) != 0 {
			t.Fatalf("download records = %+v, want none after rejected sale", repo.downloads)
		}
		if len(charger.transfers) != 0 {
			t.Fatalf("credit transfers = %d, want none after rejected sale", len(charger.transfers))
		}
		if len(reader.userIDs) != 1 || reader.userIDs[0] != 9 {
			t.Fatalf("membership checks = %+v, want author 9", reader.userIDs)
		}
	})

	t.Run("fails closed when author membership cannot be checked", func(t *testing.T) {
		repo := newMemoryRepository(activeAttachment(118, 9, 7))
		reader := &membershipEntitlementStub{err: errors.New("mall unavailable")}
		charger := &captureCharger{}
		service := NewService(repo, charger, reader, newPublishedTopicReader(9))

		_, err := service.AuthorizeDownload(context.Background(), 118, 42)
		if err != domain.ErrMembershipServiceUnavailable {
			t.Fatalf("AuthorizeDownload() error = %v, want membership service unavailable", err)
		}
		if len(repo.downloads) != 0 || len(charger.transfers) != 0 {
			t.Fatalf("rejected sale changed downloads=%+v transfers=%+v", repo.downloads, charger.transfers)
		}
	})

	t.Run("keeps completed purchases downloadable after author membership ends", func(t *testing.T) {
		repo := newMemoryRepository(activeAttachment(116, 9, 7))
		reader := &membershipEntitlementStub{active: true}
		charger := &captureCharger{}
		service := NewService(repo, charger, reader, newPublishedTopicReader(9))

		if _, err := service.AuthorizeDownload(context.Background(), 116, 42); err != nil {
			t.Fatalf("initial AuthorizeDownload() error = %v", err)
		}
		reader.active = false
		retry, err := service.AuthorizeDownload(context.Background(), 116, 42)
		if err != nil {
			t.Fatalf("completed purchase retry error = %v", err)
		}
		if !retry.AlreadyAuthorized || retry.ChargedCredits != 0 {
			t.Fatalf("completed purchase retry = %+v, want already authorized", retry)
		}
		if len(charger.transfers) != 1 {
			t.Fatalf("credit transfers = %d, want only the initial sale", len(charger.transfers))
		}
		if len(reader.userIDs) != 1 {
			t.Fatalf("membership checks = %+v, want only the initial sale check", reader.userIDs)
		}
	})

	t.Run("blocks pending paid authorization after author membership ends", func(t *testing.T) {
		repo := newMemoryRepository(activeAttachment(117, 9, 7))
		reader := &membershipEntitlementStub{active: true}
		charger := &captureCharger{errors: []error{domain.ErrCreditServiceUnavailable}}
		service := NewService(repo, charger, reader, newPublishedTopicReader(9))

		_, err := service.AuthorizeDownload(context.Background(), 117, 42)
		if err != domain.ErrCreditServiceUnavailable {
			t.Fatalf("initial AuthorizeDownload() error = %v, want credit service unavailable", err)
		}
		reader.active = false
		_, err = service.AuthorizeDownload(context.Background(), 117, 42)
		if err != domain.ErrPaidAttachmentSalesMembershipInactive {
			t.Fatalf("pending authorization retry error = %v, want inactive author membership", err)
		}
		if download := repo.downloads[downloadKey(117, 42)]; download.Status != domain.DownloadStatusPending || download.ChargedCredits != 7 {
			t.Fatalf("pending download = %+v, want unchanged paid pending authorization", download)
		}
		if len(charger.transfers) != 1 {
			t.Fatalf("credit transfers = %d, want no retry after membership ends", len(charger.transfers))
		}
	})
}

func TestAuthorizeDownloadKeepsPendingRecordForStableRetry(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(102, 9, 11))
	charger := &captureCharger{errors: []error{domain.ErrInsufficientCredits}}
	service := newTestService(repo, charger)

	_, err := service.AuthorizeDownload(context.Background(), 102, 42)
	if err != domain.ErrInsufficientCredits {
		t.Fatalf("first AuthorizeDownload() error = %v, want insufficient credits", err)
	}
	pending := repo.downloads[downloadKey(102, 42)]
	if pending.Status != domain.DownloadStatusPending || pending.SourceEventID != "attachment-download:102:42" {
		t.Fatalf("pending download = %+v", pending)
	}
	if _, err := service.UpdateAttachmentPrice(context.Background(), 102, 9, 17); err != nil {
		t.Fatalf("UpdateAttachmentPrice() error = %v", err)
	}

	authorization, err := service.AuthorizeDownload(context.Background(), 102, 42)
	if err != nil {
		t.Fatalf("retry AuthorizeDownload() error = %v", err)
	}
	if authorization.AlreadyAuthorized || authorization.ChargedCredits != 11 {
		t.Fatalf("retry authorization = %+v", authorization)
	}
	if len(charger.transfers) != 2 {
		t.Fatalf("credit transfers = %d, want 2 attempts", len(charger.transfers))
	}
	if charger.transfers[0].SourceEventID != charger.transfers[1].SourceEventID || charger.transfers[1].PayeeUserID != 9 {
		t.Fatalf("retry transfers = %+v, want stable owner settlement", charger.transfers)
	}
}

func TestAuthorizeDownloadDoesNotDebitAfterAttachmentIsArchivedBeforeCompletion(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(109, 9, 11))
	repo.archiveBeforeAuthorization = true
	charger := &captureCharger{}
	service := newTestService(repo, charger)

	_, err := service.AuthorizeDownload(context.Background(), 109, 42)
	if err != domain.ErrAttachmentArchived {
		t.Fatalf("AuthorizeDownload() error = %v, want archived attachment", err)
	}
	if len(charger.transfers) != 0 {
		t.Fatalf("credit transfers = %d, want none", len(charger.transfers))
	}
	if download := repo.downloads[downloadKey(109, 42)]; download.Status != domain.DownloadStatusPending {
		t.Fatalf("download after archived completion = %+v", download)
	}
}

func TestAuthorizeDownloadAllowsAuthorizedPurchaseAfterAttachmentArchive(t *testing.T) {
	attachment := activeAttachment(111, 9, 11)
	attachment.Status = domain.AttachmentStatusArchived
	repo := newMemoryRepository(attachment)
	authorizedAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	repo.downloads[downloadKey(attachment.ID, 42)] = domain.Download{
		AttachmentID:   attachment.ID,
		UserID:         42,
		Status:         domain.DownloadStatusAuthorized,
		SourceEventID:  attachmentDownloadEventID(attachment.ID, 42),
		ChargedCredits: 11,
		CreatedAt:      authorizedAt.Add(-time.Minute),
		AuthorizedAt:   &authorizedAt,
	}
	charger := &captureCharger{}

	authorization, err := newTestService(repo, charger).AuthorizeDownload(context.Background(), attachment.ID, 42)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if !authorization.AlreadyAuthorized || authorization.Attachment.Status != domain.AttachmentStatusArchived {
		t.Fatalf("authorization = %+v, want archived already-authorized purchase", authorization)
	}
	if len(charger.transfers) != 0 {
		t.Fatalf("credit transfers = %d, want none", len(charger.transfers))
	}
}

func TestAuthorizeDownloadRejectsArchivedAttachmentWithoutAuthorizedPurchase(t *testing.T) {
	attachment := activeAttachment(112, 9, 11)
	attachment.Status = domain.AttachmentStatusArchived
	charger := &captureCharger{}

	_, err := newTestService(newMemoryRepository(attachment), charger).AuthorizeDownload(context.Background(), attachment.ID, 42)
	if err != domain.ErrAttachmentArchived {
		t.Fatalf("AuthorizeDownload() error = %v, want archived attachment", err)
	}
	if len(charger.transfers) != 0 {
		t.Fatalf("credit transfers = %d, want none", len(charger.transfers))
	}
}

func TestAuthorizeDownloadReportsConcurrentAuthorizationAsAlreadyAuthorized(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(110, 9, 11))
	repo.authorizeBeforeCompletion = true
	charger := &captureCharger{}
	service := newTestService(repo, charger)

	authorization, err := service.AuthorizeDownload(context.Background(), 110, 42)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if !authorization.AlreadyAuthorized || authorization.ChargedCredits != 0 {
		t.Fatalf("authorization = %+v, want already authorized without charge", authorization)
	}
	if len(charger.transfers) != 0 {
		t.Fatalf("credit transfers = %d, want none", len(charger.transfers))
	}
}

func TestUpdateAttachmentPriceKeepsAuthorizedDownloadAvailable(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(107, 9, 7))
	charger := &captureCharger{}
	service := newTestService(repo, charger)

	first, err := service.AuthorizeDownload(context.Background(), 107, 42)
	if err != nil {
		t.Fatalf("first AuthorizeDownload() error = %v", err)
	}
	if first.ChargedCredits != 7 {
		t.Fatalf("first authorization charge = %d, want 7", first.ChargedCredits)
	}
	updated, err := service.UpdateAttachmentPrice(context.Background(), 107, 9, 13)
	if err != nil {
		t.Fatalf("UpdateAttachmentPrice() error = %v", err)
	}
	if updated.PriceCredits != 13 {
		t.Fatalf("updated attachment price = %d, want 13", updated.PriceCredits)
	}

	retry, err := service.AuthorizeDownload(context.Background(), 107, 42)
	if err != nil {
		t.Fatalf("authorized buyer retry error = %v", err)
	}
	if !retry.AlreadyAuthorized || retry.ChargedCredits != 0 {
		t.Fatalf("authorized buyer retry = %+v", retry)
	}
	newBuyer, err := service.AuthorizeDownload(context.Background(), 107, 43)
	if err != nil {
		t.Fatalf("new buyer authorization error = %v", err)
	}
	if newBuyer.AlreadyAuthorized || newBuyer.ChargedCredits != 13 {
		t.Fatalf("new buyer authorization = %+v, want a 13-credit authorization", newBuyer)
	}
	if got := repo.downloads[downloadKey(107, 42)].ChargedCredits; got != 7 {
		t.Fatalf("authorized buyer charge snapshot = %d, want 7", got)
	}
	if len(charger.transfers) != 2 {
		t.Fatalf("credit transfers = %d, want 2", len(charger.transfers))
	}
}

func TestUpdateAttachmentPriceRequiresActiveOwner(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(108, 9, 7))
	service := newTestService(repo, &captureCharger{})
	if _, err := service.UpdateAttachmentPrice(context.Background(), 108, 42, 13); err != domain.ErrAttachmentOwnerMismatch {
		t.Fatalf("non-owner UpdateAttachmentPrice() error = %v, want owner mismatch", err)
	}
	repo.attachment.Status = domain.AttachmentStatusArchived
	if _, err := service.UpdateAttachmentPrice(context.Background(), 108, 9, 13); err != domain.ErrAttachmentArchived {
		t.Fatalf("archived UpdateAttachmentPrice() error = %v, want archived attachment", err)
	}
	if _, err := service.UpdateAttachmentPrice(context.Background(), 108, 9, -1); err != domain.ErrInvalidAttachment {
		t.Fatalf("negative UpdateAttachmentPrice() error = %v, want invalid attachment", err)
	}
}

func TestAuthorizeDownloadAllowsOwnerWithoutDebit(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(103, 42, 13))
	charger := &captureCharger{}
	service := newTestService(repo, charger)

	authorization, err := service.AuthorizeDownload(context.Background(), 103, 42)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if authorization.AlreadyAuthorized || authorization.ChargedCredits != 0 {
		t.Fatalf("owner authorization = %+v", authorization)
	}
	if len(charger.transfers) != 0 {
		t.Fatalf("owner transfers = %d, want none", len(charger.transfers))
	}
}

func TestAuthorizeDownloadRetriesTransferWithStableEvent(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(113, 9, 7))
	charger := &captureCharger{errors: []error{domain.ErrCreditServiceUnavailable}}
	service := newTestService(repo, charger)

	_, err := service.AuthorizeDownload(context.Background(), 113, 42)
	if err != domain.ErrCreditServiceUnavailable {
		t.Fatalf("first AuthorizeDownload() error = %v, want credit service unavailable", err)
	}
	if download := repo.downloads[downloadKey(113, 42)]; download.Status != domain.DownloadStatusPending {
		t.Fatalf("download after transfer failure = %+v, want pending", download)
	}

	authorization, err := service.AuthorizeDownload(context.Background(), 113, 42)
	if err != nil {
		t.Fatalf("retry AuthorizeDownload() error = %v", err)
	}
	if authorization.AlreadyAuthorized || authorization.ChargedCredits != 7 {
		t.Fatalf("retry authorization = %+v, want a new 7-credit authorization", authorization)
	}
	if len(charger.transfers) != 2 {
		t.Fatalf("credit transfer attempts = %d, want 2", len(charger.transfers))
	}
	if charger.transfers[0].SourceEventID != charger.transfers[1].SourceEventID {
		t.Fatalf("retry event ids are not stable: transfers=%+v", charger.transfers)
	}
	if command := charger.transfers[1]; command.PayerUserID != 42 || command.PayeeUserID != 9 || command.Amount != 7 || command.CreditReason != "attachment_sale" {
		t.Fatalf("retry transfer command = %+v", command)
	}
}

func TestAuthorizeDownloadSkipsSettlementForFreeAttachment(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(114, 9, 0))
	charger := &captureCharger{}

	authorization, err := newTestService(repo, charger).AuthorizeDownload(context.Background(), 114, 42)
	if err != nil {
		t.Fatalf("AuthorizeDownload() error = %v", err)
	}
	if authorization.AlreadyAuthorized || authorization.ChargedCredits != 0 {
		t.Fatalf("free authorization = %+v", authorization)
	}
	if len(charger.transfers) != 0 {
		t.Fatalf("free attachment transfers = %d, want none", len(charger.transfers))
	}
}

func TestCreateAttachmentMembershipEnforcement(t *testing.T) {
	command := CreateAttachmentCommand{
		TopicID:      1,
		OwnerID:      2,
		ObjectKey:    "topics/1/guide.pdf",
		OriginalName: "guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    1,
		PriceCredits: 5,
	}

	t.Run("paid attachment requires active membership", func(t *testing.T) {
		repo := newMemoryRepository(domain.Attachment{})
		reader := &membershipEntitlementStub{}
		_, err := NewService(repo, &captureCharger{}, reader, newPublishedTopicReader(command.OwnerID)).CreateAttachment(context.Background(), command)
		if err != domain.ErrMembershipEntitlementRequired {
			t.Fatalf("CreateAttachment() error = %v, want membership entitlement required", err)
		}
		if len(reader.userIDs) != 1 || reader.userIDs[0] != command.OwnerID {
			t.Fatalf("membership checks = %+v, want owner %d", reader.userIDs, command.OwnerID)
		}
		if repo.attachment.ObjectKey != "" {
			t.Fatalf("attachment persisted without membership = %+v", repo.attachment)
		}
	})

	t.Run("membership lookup failure fails closed", func(t *testing.T) {
		reader := &membershipEntitlementStub{err: errors.New("mall unavailable")}
		_, err := NewService(newMemoryRepository(domain.Attachment{}), &captureCharger{}, reader, newPublishedTopicReader(command.OwnerID)).CreateAttachment(context.Background(), command)
		if err != domain.ErrMembershipServiceUnavailable {
			t.Fatalf("CreateAttachment() error = %v, want membership service unavailable", err)
		}
	})

	t.Run("free attachment skips membership lookup", func(t *testing.T) {
		repo := newMemoryRepository(domain.Attachment{})
		reader := &membershipEntitlementStub{err: errors.New("mall unavailable")}
		created, err := NewService(repo, &captureCharger{}, reader, newPublishedTopicReader(command.OwnerID)).CreateAttachment(context.Background(), CreateAttachmentCommand{
			TopicID:      command.TopicID,
			OwnerID:      command.OwnerID,
			ObjectKey:    "topics/1/free-guide.pdf",
			OriginalName: command.OriginalName,
			ContentType:  command.ContentType,
			SizeBytes:    command.SizeBytes,
		})
		if err != nil {
			t.Fatalf("CreateAttachment() error = %v", err)
		}
		if created.PriceCredits != 0 || len(reader.userIDs) != 0 {
			t.Fatalf("free attachment = %+v, membership checks = %+v", created, reader.userIDs)
		}
	})
}

func TestUpdateAttachmentPriceMembershipEnforcement(t *testing.T) {
	t.Run("positive price requires active membership", func(t *testing.T) {
		repo := newMemoryRepository(activeAttachment(120, 9, 0))
		reader := &membershipEntitlementStub{}
		_, err := NewService(repo, &captureCharger{}, reader, newPublishedTopicReader(9)).UpdateAttachmentPrice(context.Background(), 120, 9, 5)
		if err != domain.ErrMembershipEntitlementRequired {
			t.Fatalf("UpdateAttachmentPrice() error = %v, want membership entitlement required", err)
		}
		if repo.attachment.PriceCredits != 0 || len(reader.userIDs) != 1 || reader.userIDs[0] != 9 {
			t.Fatalf("attachment = %+v, membership checks = %+v", repo.attachment, reader.userIDs)
		}
	})

	t.Run("membership lookup failure fails closed", func(t *testing.T) {
		reader := &membershipEntitlementStub{err: errors.New("mall unavailable")}
		_, err := NewService(newMemoryRepository(activeAttachment(121, 9, 0)), &captureCharger{}, reader, newPublishedTopicReader(9)).UpdateAttachmentPrice(context.Background(), 121, 9, 5)
		if err != domain.ErrMembershipServiceUnavailable {
			t.Fatalf("UpdateAttachmentPrice() error = %v, want membership service unavailable", err)
		}
	})

	t.Run("lowering to free skips membership lookup", func(t *testing.T) {
		repo := newMemoryRepository(activeAttachment(122, 9, 5))
		reader := &membershipEntitlementStub{err: errors.New("mall unavailable")}
		updated, err := NewService(repo, &captureCharger{}, reader, newPublishedTopicReader(9)).UpdateAttachmentPrice(context.Background(), 122, 9, 0)
		if err != nil {
			t.Fatalf("UpdateAttachmentPrice() error = %v", err)
		}
		if updated.PriceCredits != 0 || len(reader.userIDs) != 0 {
			t.Fatalf("updated attachment = %+v, membership checks = %+v", updated, reader.userIDs)
		}
	})
}

func TestAttachmentTopicAccessEnforcement(t *testing.T) {
	command := CreateAttachmentCommand{
		TopicID:      8,
		OwnerID:      9,
		ObjectKey:    "topics/8/guide.pdf",
		OriginalName: "guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    1,
	}

	t.Run("create requires the topic owner", func(t *testing.T) {
		repo := newMemoryRepository(domain.Attachment{})
		membership := &membershipEntitlementStub{active: true}
		_, err := NewService(repo, &captureCharger{}, membership, newPublishedTopicReader(42)).CreateAttachment(context.Background(), command)
		if err != domain.ErrAttachmentTopicOwnerMismatch {
			t.Fatalf("CreateAttachment() error = %v, want attachment topic owner mismatch", err)
		}
		if repo.attachment.ObjectKey != "" || len(membership.userIDs) != 0 {
			t.Fatalf("attachment = %+v, membership checks = %+v", repo.attachment, membership.userIDs)
		}
	})

	t.Run("create requires a published topic", func(t *testing.T) {
		topics := newPublishedTopicReader(command.OwnerID)
		topics.status = 4
		_, err := NewService(newMemoryRepository(domain.Attachment{}), &captureCharger{}, &membershipEntitlementStub{active: true}, topics).CreateAttachment(context.Background(), command)
		if err != domain.ErrAttachmentTopicUnavailable {
			t.Fatalf("CreateAttachment() error = %v, want attachment topic unavailable", err)
		}
	})

	t.Run("content lookup failure fails closed", func(t *testing.T) {
		topics := newPublishedTopicReader(command.OwnerID)
		topics.err = errors.New("content unavailable")
		_, err := NewService(newMemoryRepository(domain.Attachment{}), &captureCharger{}, &membershipEntitlementStub{active: true}, topics).CreateAttachment(context.Background(), command)
		if err != domain.ErrContentServiceUnavailable {
			t.Fatalf("CreateAttachment() error = %v, want content service unavailable", err)
		}
	})

	t.Run("unpublished topic blocks list, price update, and download", func(t *testing.T) {
		repo := newMemoryRepository(activeAttachment(123, command.OwnerID, 7))
		topics := newPublishedTopicReader(command.OwnerID)
		topics.status = 4
		charger := &captureCharger{}
		service := NewService(repo, charger, &membershipEntitlementStub{active: true}, topics)

		if _, err := service.ListTopicAttachments(context.Background(), command.TopicID); err != domain.ErrAttachmentTopicUnavailable {
			t.Fatalf("ListTopicAttachments() error = %v, want attachment topic unavailable", err)
		}
		if _, err := service.UpdateAttachmentPrice(context.Background(), 123, command.OwnerID, 0); err != domain.ErrAttachmentTopicUnavailable {
			t.Fatalf("UpdateAttachmentPrice() error = %v, want attachment topic unavailable", err)
		}
		if repo.attachment.PriceCredits != 7 {
			t.Fatalf("attachment price after rejected update = %d, want 7", repo.attachment.PriceCredits)
		}
		if _, err := service.AuthorizeDownload(context.Background(), 123, 42); err != domain.ErrAttachmentTopicUnavailable {
			t.Fatalf("AuthorizeDownload() error = %v, want attachment topic unavailable", err)
		}
		if len(charger.transfers) != 0 {
			t.Fatalf("credit transfers = %d, want none", len(charger.transfers))
		}
	})
}

func TestCreateAttachmentRejectsUnsafeOriginalName(t *testing.T) {
	service := newTestService(newMemoryRepository(domain.Attachment{}), &captureCharger{})
	_, err := service.CreateAttachment(context.Background(), CreateAttachmentCommand{
		TopicID:      1,
		OwnerID:      2,
		ObjectKey:    "topics/1/report.pdf",
		OriginalName: "../report.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    1,
	})
	if err != domain.ErrInvalidAttachment {
		t.Fatalf("CreateAttachment() error = %v, want invalid attachment", err)
	}
}

func TestGetAttachmentReturnsArchivedMetadata(t *testing.T) {
	attachment := activeAttachment(104, 7, 0)
	attachment.Status = domain.AttachmentStatusArchived
	service := newTestService(newMemoryRepository(attachment), &captureCharger{})

	got, err := service.GetAttachment(context.Background(), attachment.ID)
	if err != nil {
		t.Fatalf("GetAttachment() error = %v", err)
	}
	if got.Status != domain.AttachmentStatusArchived {
		t.Fatalf("GetAttachment() status = %s, want archived", got.Status)
	}
}

func TestListUserAttachmentDownloadsReturnsOnlyAuthorizedDownloads(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(105, 9, 7))
	authorizedAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	repo.downloads[downloadKey(105, 42)] = domain.Download{
		AttachmentID:   105,
		UserID:         42,
		Status:         domain.DownloadStatusAuthorized,
		ChargedCredits: 7,
		CreatedAt:      authorizedAt.Add(-time.Minute),
		AuthorizedAt:   &authorizedAt,
	}
	repo.downloads[downloadKey(106, 42)] = domain.Download{
		AttachmentID: 106,
		UserID:       42,
		Status:       domain.DownloadStatusPending,
		CreatedAt:    authorizedAt,
	}
	repo.downloads[downloadKey(105, 99)] = domain.Download{
		AttachmentID: 105,
		UserID:       99,
		Status:       domain.DownloadStatusAuthorized,
		CreatedAt:    authorizedAt,
		AuthorizedAt: &authorizedAt,
	}

	downloads, err := newTestService(repo, &captureCharger{}).ListUserAttachmentDownloads(context.Background(), 42, 20, 0)
	if err != nil {
		t.Fatalf("ListUserAttachmentDownloads() error = %v", err)
	}
	if downloads.Total != 1 || len(downloads.Items) != 1 {
		t.Fatalf("downloads = %+v, want one authorized current-user record", downloads)
	}
	if downloads.Items[0].Attachment.ID != 105 || downloads.Items[0].ChargedCredits != 7 || downloads.Items[0].Status != domain.DownloadStatusAuthorized {
		t.Fatalf("download = %+v", downloads.Items[0])
	}
}

func TestListUserAttachmentDownloadsRejectsInvalidPage(t *testing.T) {
	service := newTestService(newMemoryRepository(activeAttachment(106, 9, 7)), &captureCharger{})
	if _, err := service.ListUserAttachmentDownloads(context.Background(), 42, 0, 0); err != domain.ErrInvalidDownload {
		t.Fatalf("ListUserAttachmentDownloads() error = %v, want invalid attachment download", err)
	}
}

func TestListUserAttachmentSalesReturnsOnlyPaidAuthorizedOwnerSales(t *testing.T) {
	repo := newMemoryRepository(activeAttachment(107, 9, 7))
	authorizedAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	repo.downloads[downloadKey(107, 42)] = domain.Download{
		AttachmentID:   107,
		UserID:         42,
		Status:         domain.DownloadStatusAuthorized,
		ChargedCredits: 7,
		CreatedAt:      authorizedAt.Add(-time.Minute),
		AuthorizedAt:   &authorizedAt,
	}
	laterAuthorizedAt := authorizedAt.Add(time.Minute)
	repo.downloads[downloadKey(107, 44)] = domain.Download{
		AttachmentID:   107,
		UserID:         44,
		Status:         domain.DownloadStatusAuthorized,
		ChargedCredits: 11,
		CreatedAt:      laterAuthorizedAt.Add(-time.Minute),
		AuthorizedAt:   &laterAuthorizedAt,
	}
	repo.downloads[downloadKey(107, 9)] = domain.Download{
		AttachmentID: 107,
		UserID:       9,
		Status:       domain.DownloadStatusAuthorized,
		CreatedAt:    authorizedAt,
		AuthorizedAt: &authorizedAt,
	}
	repo.downloads[downloadKey(107, 43)] = domain.Download{
		AttachmentID:   107,
		UserID:         43,
		Status:         domain.DownloadStatusPending,
		ChargedCredits: 7,
		CreatedAt:      authorizedAt,
	}
	repo.downloads[downloadKey(108, 44)] = domain.Download{
		AttachmentID:   108,
		UserID:         44,
		Status:         domain.DownloadStatusAuthorized,
		ChargedCredits: 7,
		CreatedAt:      authorizedAt,
		AuthorizedAt:   &authorizedAt,
	}

	service := newTestService(repo, &captureCharger{})
	sales, err := service.ListUserAttachmentSales(context.Background(), 9, 1, 0)
	if err != nil {
		t.Fatalf("ListUserAttachmentSales() error = %v", err)
	}
	if len(sales.Items) != 1 {
		t.Fatalf("sales = %+v, want one paid authorized owner sale", sales)
	}
	if sales.Total != 2 || sales.TotalEarnedCredits != 18 {
		t.Fatalf("sales summary = %+v, want total 2 and earned credits 18", sales)
	}
	if sales.Items[0].Attachment.ID != 107 || sales.Items[0].EarnedCredits != 11 || !sales.Items[0].SoldAt.Equal(laterAuthorizedAt) {
		t.Fatalf("sale = %+v", sales.Items[0])
	}

	otherOwnerSales, err := service.ListUserAttachmentSales(context.Background(), 42, 20, 0)
	if err != nil {
		t.Fatalf("ListUserAttachmentSales() for non-owner error = %v", err)
	}
	if len(otherOwnerSales.Items) != 0 || otherOwnerSales.Total != 0 || otherOwnerSales.TotalEarnedCredits != 0 {
		t.Fatalf("sales for non-owner = %+v, want none", otherOwnerSales)
	}
}

type captureCharger struct {
	transfers []CreditTransferCommand
	errors    []error
}

func (c *captureCharger) TransferCredits(_ context.Context, command CreditTransferCommand) error {
	c.transfers = append(c.transfers, command)
	if len(c.errors) == 0 {
		return nil
	}
	err := c.errors[0]
	c.errors = c.errors[1:]
	return err
}

type membershipEntitlementStub struct {
	active  bool
	err     error
	userIDs []int64
}

func (s *membershipEntitlementStub) HasActiveMembership(_ context.Context, userID int64) (bool, error) {
	s.userIDs = append(s.userIDs, userID)
	return s.active, s.err
}

type topicReaderStub struct {
	authorID int64
	status   int32
	err      error
	topicIDs []int64
}

func newPublishedTopicReader(authorID int64) *topicReaderStub {
	return &topicReaderStub{authorID: authorID, status: topicStatusPublished}
}

func (s *topicReaderStub) GetTopic(_ context.Context, topicID int64) (Topic, error) {
	s.topicIDs = append(s.topicIDs, topicID)
	if s.err != nil {
		return Topic{}, s.err
	}
	return Topic{ID: topicID, AuthorID: s.authorID, Status: s.status}, nil
}

type memoryRepository struct {
	attachment                 domain.Attachment
	downloads                  map[string]domain.Download
	archiveBeforeAuthorization bool
	authorizeBeforeCompletion  bool
}

func newMemoryRepository(attachment domain.Attachment) *memoryRepository {
	return &memoryRepository{attachment: attachment, downloads: make(map[string]domain.Download)}
}

func (r *memoryRepository) EnsureSchema(context.Context) error { return nil }

func (r *memoryRepository) CreateAttachment(_ context.Context, attachment domain.Attachment) (domain.Attachment, error) {
	r.attachment = attachment
	return attachment, nil
}

func (r *memoryRepository) ListTopicAttachments(_ context.Context, topicID int64) ([]domain.Attachment, error) {
	if r.attachment.TopicID != topicID || r.attachment.Status != domain.AttachmentStatusActive {
		return nil, nil
	}
	return []domain.Attachment{r.attachment}, nil
}

func (r *memoryRepository) ListUserAttachmentDownloads(_ context.Context, userID int64, limit, offset int32) (domain.AttachmentDownloadList, error) {
	result := domain.AttachmentDownloadList{Items: make([]domain.AttachmentDownload, 0)}
	downloads := make([]domain.AttachmentDownload, 0)
	for _, download := range r.downloads {
		if download.UserID != userID || download.Status != domain.DownloadStatusAuthorized {
			continue
		}
		downloads = append(downloads, domain.AttachmentDownload{
			Attachment:     r.attachment,
			Status:         download.Status,
			ChargedCredits: download.ChargedCredits,
			CreatedAt:      download.CreatedAt,
			AuthorizedAt:   download.AuthorizedAt,
		})
	}
	sort.Slice(downloads, func(i, j int) bool {
		return downloads[i].CreatedAt.After(downloads[j].CreatedAt)
	})
	result.Total = int64(len(downloads))
	start := int(offset)
	if start >= len(downloads) {
		return result, nil
	}
	end := start + int(limit)
	if end > len(downloads) {
		end = len(downloads)
	}
	result.Items = downloads[start:end]
	return result, nil
}

func (r *memoryRepository) ListUserAttachmentSales(_ context.Context, userID int64, limit, offset int32) (domain.AttachmentSaleList, error) {
	result := domain.AttachmentSaleList{Items: make([]domain.AttachmentSale, 0)}
	if r.attachment.OwnerID != userID {
		return result, nil
	}
	sales := make([]domain.AttachmentSale, 0)
	for _, download := range r.downloads {
		if download.AttachmentID != r.attachment.ID || download.Status != domain.DownloadStatusAuthorized || download.ChargedCredits <= 0 || download.AuthorizedAt == nil {
			continue
		}
		sales = append(sales, domain.AttachmentSale{
			Attachment:    r.attachment,
			EarnedCredits: download.ChargedCredits,
			SoldAt:        *download.AuthorizedAt,
		})
	}
	sort.Slice(sales, func(i, j int) bool {
		return sales[i].SoldAt.After(sales[j].SoldAt)
	})
	result.Total = int64(len(sales))
	for _, sale := range sales {
		result.TotalEarnedCredits += sale.EarnedCredits
	}
	start := int(offset)
	if start >= len(sales) {
		return result, nil
	}
	end := start + int(limit)
	if end > len(sales) {
		end = len(sales)
	}
	result.Items = sales[start:end]
	return result, nil
}

func (r *memoryRepository) GetAttachment(context.Context, int64) (domain.Attachment, error) {
	if r.attachment.ID == 0 {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	return r.attachment, nil
}

func (r *memoryRepository) GetDownload(_ context.Context, attachmentID, userID int64) (domain.Download, bool, error) {
	download, found := r.downloads[downloadKey(attachmentID, userID)]
	return download, found, nil
}

func (r *memoryRepository) ArchiveAttachment(_ context.Context, attachmentID, ownerID int64, archivedAt time.Time) (domain.Attachment, error) {
	if r.attachment.ID != attachmentID {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	if r.attachment.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	r.attachment.Status = domain.AttachmentStatusArchived
	r.attachment.ArchivedAt = &archivedAt
	return r.attachment, nil
}

func (r *memoryRepository) UpdateAttachmentPrice(_ context.Context, attachmentID, ownerID, priceCredits int64, updatedAt time.Time) (domain.Attachment, error) {
	if r.attachment.ID != attachmentID {
		return domain.Attachment{}, domain.ErrAttachmentNotFound
	}
	if r.attachment.OwnerID != ownerID {
		return domain.Attachment{}, domain.ErrAttachmentOwnerMismatch
	}
	if r.attachment.Status != domain.AttachmentStatusActive {
		return domain.Attachment{}, domain.ErrAttachmentArchived
	}
	r.attachment.PriceCredits = priceCredits
	r.attachment.UpdatedAt = updatedAt
	return r.attachment, nil
}

func (r *memoryRepository) EnsureDownload(_ context.Context, attachmentID, userID int64, sourceEventID string, chargedCredits int64, createdAt time.Time) (domain.Download, error) {
	key := downloadKey(attachmentID, userID)
	if existing, ok := r.downloads[key]; ok {
		return existing, nil
	}
	download := domain.Download{
		AttachmentID:   attachmentID,
		UserID:         userID,
		Status:         domain.DownloadStatusPending,
		SourceEventID:  sourceEventID,
		ChargedCredits: chargedCredits,
		CreatedAt:      createdAt,
	}
	r.downloads[key] = download
	return download, nil
}

func (r *memoryRepository) CompleteDownloadAuthorization(ctx context.Context, attachmentID, userID int64, authorizedAt time.Time, settle func(context.Context) error) (domain.Download, bool, error) {
	if r.archiveBeforeAuthorization {
		r.archiveBeforeAuthorization = false
		r.attachment.Status = domain.AttachmentStatusArchived
	}
	if r.attachment.ID != attachmentID {
		return domain.Download{}, false, domain.ErrAttachmentNotFound
	}
	if r.attachment.Status != domain.AttachmentStatusActive {
		return domain.Download{}, false, domain.ErrAttachmentArchived
	}
	key := downloadKey(attachmentID, userID)
	download, ok := r.downloads[key]
	if !ok {
		return domain.Download{}, false, domain.ErrDownloadRecordMismatch
	}
	if r.authorizeBeforeCompletion {
		r.authorizeBeforeCompletion = false
		download.Status = domain.DownloadStatusAuthorized
		download.AuthorizedAt = &authorizedAt
		r.downloads[key] = download
	}
	if download.Status == domain.DownloadStatusAuthorized {
		return download, true, nil
	}
	if download.Status != domain.DownloadStatusPending {
		return domain.Download{}, false, domain.ErrDownloadRecordMismatch
	}
	if settle == nil {
		return domain.Download{}, false, domain.ErrCreditServiceUnavailable
	}
	if err := settle(ctx); err != nil {
		return domain.Download{}, false, err
	}
	download.Status = domain.DownloadStatusAuthorized
	download.AuthorizedAt = &authorizedAt
	r.downloads[key] = download
	return download, false, nil
}

func activeAttachment(id, ownerID, priceCredits int64) domain.Attachment {
	return domain.Attachment{
		ID:           id,
		TopicID:      8,
		OwnerID:      ownerID,
		ObjectKey:    "topics/8/attachment",
		OriginalName: "guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    256,
		PriceCredits: priceCredits,
		Status:       domain.AttachmentStatusActive,
	}
}

func downloadKey(attachmentID, userID int64) string {
	return strconv.FormatInt(attachmentID, 10) + ":" + strconv.FormatInt(userID, 10)
}
