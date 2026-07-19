package grpc

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	creditpb "file-service/api/proto/creditpb"
	pb "file-service/api/proto/filepb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestFileServiceIntegration(t *testing.T) {
	address := os.Getenv("BBS_FILE_INTEGRATION_ADDR")
	creditAddress := os.Getenv("BBS_CREDIT_INTEGRATION_ADDR")
	if address == "" || creditAddress == "" {
		t.Skip("set BBS_FILE_INTEGRATION_ADDR and BBS_CREDIT_INTEGRATION_ADDR to run against live services")
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("connect file-service: %v", err)
	}
	defer conn.Close()
	client := pb.NewFileServiceClient(conn)
	creditConn, err := grpc.NewClient(creditAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("connect credit-service: %v", err)
	}
	defer creditConn.Close()
	credits := creditpb.NewCreditServiceClient(creditConn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stamp := time.Now().UnixNano()
	topicID := stamp
	ownerID := stamp + 1
	buyerID := stamp + 2
	newBuyerID := stamp + 3
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        buyerID,
		Delta:         20,
		Reason:        "file_integration_seed",
		Description:   "file-service integration test seed",
		SourceEventId: fmt.Sprintf("file-integration-seed:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("seed buyer credits: %v", err)
	}
	created, err := client.CreateAttachment(ctx, &pb.CreateAttachmentRequest{
		TopicId:      topicID,
		OwnerId:      ownerID,
		ObjectKey:    fmt.Sprintf("integration/%d/guide.pdf", stamp),
		OriginalName: "guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    128,
		PriceCredits: 9,
	})
	if err != nil {
		t.Fatalf("CreateAttachment() error = %v", err)
	}
	attachment := created.GetAttachment()
	if attachment.GetId() <= 0 || attachment.GetStatus() != "ACTIVE" {
		t.Fatalf("created attachment = %+v", attachment)
	}

	listed, err := client.ListTopicAttachments(ctx, &pb.ListTopicAttachmentsRequest{TopicId: topicID})
	if err != nil {
		t.Fatalf("ListTopicAttachments() error = %v", err)
	}
	if len(listed.GetItems()) != 1 || listed.GetItems()[0].GetId() != attachment.GetId() {
		t.Fatalf("listed attachments = %+v", listed.GetItems())
	}

	authorized, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachment.GetId(), UserId: ownerID})
	if err != nil {
		t.Fatalf("AuthorizeAttachmentDownload() error = %v", err)
	}
	if authorized.GetAlreadyAuthorized() || authorized.GetChargedCredits() != 0 {
		t.Fatalf("owner download authorization = %+v", authorized)
	}

	paid, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachment.GetId(), UserId: buyerID})
	if err != nil {
		t.Fatalf("paid AuthorizeAttachmentDownload() error = %v", err)
	}
	if paid.GetAlreadyAuthorized() || paid.GetChargedCredits() != 9 {
		t.Fatalf("paid download authorization = %+v", paid)
	}
	balance, err := credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: buyerID})
	if err != nil {
		t.Fatalf("GetBalance() after paid download error = %v", err)
	}
	if balance.GetBalance().GetTotal() != 11 {
		t.Fatalf("buyer balance after paid download = %d, want 11", balance.GetBalance().GetTotal())
	}
	ownerBalance, err := credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: ownerID})
	if err != nil {
		t.Fatalf("GetBalance() for owner after paid download error = %v", err)
	}
	if ownerBalance.GetBalance().GetTotal() != 9 {
		t.Fatalf("owner balance after paid download = %d, want 9", ownerBalance.GetBalance().GetTotal())
	}

	retry, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachment.GetId(), UserId: buyerID})
	if err != nil {
		t.Fatalf("retry AuthorizeAttachmentDownload() error = %v", err)
	}
	if !retry.GetAlreadyAuthorized() || retry.GetChargedCredits() != 0 {
		t.Fatalf("paid retry authorization = %+v", retry)
	}
	balance, err = credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: buyerID})
	if err != nil {
		t.Fatalf("GetBalance() after paid retry error = %v", err)
	}
	if balance.GetBalance().GetTotal() != 11 {
		t.Fatalf("buyer balance after paid retry = %d, want 11", balance.GetBalance().GetTotal())
	}
	ownerBalance, err = credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: ownerID})
	if err != nil {
		t.Fatalf("GetBalance() for owner after paid retry error = %v", err)
	}
	if ownerBalance.GetBalance().GetTotal() != 9 {
		t.Fatalf("owner balance after paid retry = %d, want 9", ownerBalance.GetBalance().GetTotal())
	}
	updated, err := client.UpdateAttachmentPrice(ctx, &pb.UpdateAttachmentPriceRequest{
		AttachmentId: attachment.GetId(),
		OwnerId:      ownerID,
		PriceCredits: 13,
	})
	if err != nil {
		t.Fatalf("UpdateAttachmentPrice() error = %v", err)
	}
	if updated.GetAttachment().GetPriceCredits() != 13 || updated.GetAttachment().GetStatus() != "ACTIVE" {
		t.Fatalf("updated attachment = %+v", updated.GetAttachment())
	}
	if _, err := client.UpdateAttachmentPrice(ctx, &pb.UpdateAttachmentPriceRequest{
		AttachmentId: attachment.GetId(),
		OwnerId:      buyerID,
		PriceCredits: 1,
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-owner UpdateAttachmentPrice() error = %v, want PermissionDenied", err)
	}
	updatedRetry, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachment.GetId(), UserId: buyerID})
	if err != nil {
		t.Fatalf("retry after price update error = %v", err)
	}
	if !updatedRetry.GetAlreadyAuthorized() || updatedRetry.GetChargedCredits() != 0 {
		t.Fatalf("retry after price update = %+v", updatedRetry)
	}
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        newBuyerID,
		Delta:         20,
		Reason:        "file_integration_seed",
		Description:   "file-service integration test seed",
		SourceEventId: fmt.Sprintf("file-integration-new-buyer-seed:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("seed new buyer credits: %v", err)
	}
	newPaid, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachment.GetId(), UserId: newBuyerID})
	if err != nil {
		t.Fatalf("new buyer AuthorizeAttachmentDownload() error = %v", err)
	}
	if newPaid.GetAlreadyAuthorized() || newPaid.GetChargedCredits() != 13 {
		t.Fatalf("new buyer download authorization = %+v", newPaid)
	}
	newBuyerBalance, err := credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: newBuyerID})
	if err != nil {
		t.Fatalf("GetBalance() for new buyer error = %v", err)
	}
	if newBuyerBalance.GetBalance().GetTotal() != 7 {
		t.Fatalf("new buyer balance after paid download = %d, want 7", newBuyerBalance.GetBalance().GetTotal())
	}
	ownerBalance, err = credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: ownerID})
	if err != nil {
		t.Fatalf("GetBalance() for owner after second sale error = %v", err)
	}
	if ownerBalance.GetBalance().GetTotal() != 22 {
		t.Fatalf("owner balance after second sale = %d, want 22", ownerBalance.GetBalance().GetTotal())
	}

	transferPayerID := stamp + 5
	transferPayeeID := stamp + 6
	transferEventID := fmt.Sprintf("file-integration-transfer:%d", stamp)
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        transferPayerID,
		Delta:         10,
		Reason:        "file_integration_seed",
		Description:   "file-service integration transfer seed",
		SourceEventId: fmt.Sprintf("file-integration-transfer-seed:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("seed transfer payer credits: %v", err)
	}
	transferRequest := &creditpb.TransferCreditsRequest{
		PayerUserId:       transferPayerID,
		PayeeUserId:       transferPayeeID,
		Amount:            6,
		DebitReason:       "file_integration_transfer_debit",
		DebitDescription:  "file-service integration transfer debit",
		CreditReason:      "file_integration_transfer_credit",
		CreditDescription: "file-service integration transfer credit",
		SourceEventId:     transferEventID,
		SourceType:        "file_integration_test",
		SourceId:          topicID,
	}
	if _, err := credits.TransferCredits(ctx, transferRequest); err != nil {
		t.Fatalf("TransferCredits() error = %v", err)
	}
	if _, err := credits.TransferCredits(ctx, transferRequest); err != nil {
		t.Fatalf("duplicate TransferCredits() error = %v", err)
	}
	transferPayerBalance, err := credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: transferPayerID})
	if err != nil {
		t.Fatalf("GetBalance() for transfer payer error = %v", err)
	}
	transferPayeeBalance, err := credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: transferPayeeID})
	if err != nil {
		t.Fatalf("GetBalance() for transfer payee error = %v", err)
	}
	if transferPayerBalance.GetBalance().GetTotal() != 4 || transferPayeeBalance.GetBalance().GetTotal() != 6 {
		t.Fatalf("transfer balances = payer:%d payee:%d, want 4/6", transferPayerBalance.GetBalance().GetTotal(), transferPayeeBalance.GetBalance().GetTotal())
	}
	if _, err := credits.TransferCredits(ctx, &creditpb.TransferCreditsRequest{
		PayerUserId:       transferPayerID,
		PayeeUserId:       transferPayeeID,
		Amount:            5,
		DebitReason:       transferRequest.DebitReason,
		DebitDescription:  transferRequest.DebitDescription,
		CreditReason:      transferRequest.CreditReason,
		CreditDescription: transferRequest.CreditDescription,
		SourceEventId:     transferEventID,
		SourceType:        transferRequest.SourceType,
		SourceId:          transferRequest.SourceId,
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mismatched TransferCredits() error = %v, want FailedPrecondition", err)
	}
	transferPayerBalance, err = credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: transferPayerID})
	if err != nil {
		t.Fatalf("GetBalance() for transfer payer after mismatch error = %v", err)
	}
	transferPayeeBalance, err = credits.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: transferPayeeID})
	if err != nil {
		t.Fatalf("GetBalance() for transfer payee after mismatch error = %v", err)
	}
	if transferPayerBalance.GetBalance().GetTotal() != 4 || transferPayeeBalance.GetBalance().GetTotal() != 6 {
		t.Fatalf("mismatched transfer changed balances to payer:%d payee:%d, want 4/6", transferPayerBalance.GetBalance().GetTotal(), transferPayeeBalance.GetBalance().GetTotal())
	}

	downloads, err := client.ListUserAttachmentDownloads(ctx, &pb.ListUserAttachmentDownloadsRequest{UserId: buyerID, Limit: 10})
	if err != nil {
		t.Fatalf("ListUserAttachmentDownloads() error = %v", err)
	}
	if len(downloads.GetItems()) != 1 {
		t.Fatalf("buyer download history = %+v", downloads.GetItems())
	}
	download := downloads.GetItems()[0]
	if download.GetAttachment().GetId() != attachment.GetId() || download.GetChargedCredits() != 9 || download.GetStatus() != "AUTHORIZED" || download.GetAuthorizedAt() == 0 {
		t.Fatalf("buyer download history record = %+v", download)
	}
	if download.GetAttachment().GetPriceCredits() != 13 {
		t.Fatalf("buyer attachment current price = %d, want 13", download.GetAttachment().GetPriceCredits())
	}
	sales, err := client.ListUserAttachmentSales(ctx, &pb.ListUserAttachmentSalesRequest{UserId: ownerID, Limit: 10})
	if err != nil {
		t.Fatalf("ListUserAttachmentSales() error = %v", err)
	}
	if len(sales.GetItems()) != 2 {
		t.Fatalf("owner attachment sale history = %+v", sales.GetItems())
	}
	earnedCredits := map[int64]bool{}
	for _, sale := range sales.GetItems() {
		if sale.GetAttachment().GetId() != attachment.GetId() || sale.GetSoldAt() == 0 {
			t.Fatalf("owner attachment sale record = %+v", sale)
		}
		earnedCredits[sale.GetEarnedCredits()] = true
	}
	if !earnedCredits[9] || !earnedCredits[13] {
		t.Fatalf("owner attachment sale credits = %+v, want 9 and 13", earnedCredits)
	}
	buyerSales, err := client.ListUserAttachmentSales(ctx, &pb.ListUserAttachmentSalesRequest{UserId: buyerID, Limit: 10})
	if err != nil {
		t.Fatalf("ListUserAttachmentSales() for buyer error = %v", err)
	}
	if len(buyerSales.GetItems()) != 0 {
		t.Fatalf("buyer attachment sale history = %+v, want none", buyerSales.GetItems())
	}

	archived, err := client.ArchiveAttachment(ctx, &pb.ArchiveAttachmentRequest{AttachmentId: attachment.GetId(), OwnerId: ownerID})
	if err != nil {
		t.Fatalf("ArchiveAttachment() error = %v", err)
	}
	if archived.GetAttachment().GetStatus() != "ARCHIVED" {
		t.Fatalf("archived attachment = %+v", archived.GetAttachment())
	}
	archivedMetadata, err := client.GetAttachment(ctx, &pb.GetAttachmentRequest{AttachmentId: attachment.GetId()})
	if err != nil {
		t.Fatalf("GetAttachment() after archive error = %v", err)
	}
	if archivedMetadata.GetAttachment().GetStatus() != "ARCHIVED" {
		t.Fatalf("archived attachment metadata = %+v", archivedMetadata.GetAttachment())
	}
	retryAfterArchive, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachment.GetId(), UserId: buyerID})
	if err != nil {
		t.Fatalf("authorized buyer retry after archive error = %v", err)
	}
	if !retryAfterArchive.GetAlreadyAuthorized() || retryAfterArchive.GetChargedCredits() != 0 {
		t.Fatalf("authorized buyer retry after archive = %+v", retryAfterArchive)
	}
	if _, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachment.GetId(), UserId: stamp + 4}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unpaid buyer authorization after archive error = %v, want FailedPrecondition", err)
	}
	if _, err := client.UpdateAttachmentPrice(ctx, &pb.UpdateAttachmentPriceRequest{
		AttachmentId: attachment.GetId(),
		OwnerId:      ownerID,
		PriceCredits: 1,
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("archived UpdateAttachmentPrice() error = %v, want FailedPrecondition", err)
	}

	listed, err = client.ListTopicAttachments(ctx, &pb.ListTopicAttachmentsRequest{TopicId: topicID})
	if err != nil {
		t.Fatalf("ListTopicAttachments() after archive error = %v", err)
	}
	if len(listed.GetItems()) != 0 {
		t.Fatalf("active attachments after archive = %+v", listed.GetItems())
	}
	downloads, err = client.ListUserAttachmentDownloads(ctx, &pb.ListUserAttachmentDownloadsRequest{UserId: buyerID, Limit: 10})
	if err != nil {
		t.Fatalf("ListUserAttachmentDownloads() after archive error = %v", err)
	}
	if len(downloads.GetItems()) != 1 || downloads.GetItems()[0].GetAttachment().GetStatus() != "ARCHIVED" {
		t.Fatalf("buyer archived download history = %+v", downloads.GetItems())
	}
	sales, err = client.ListUserAttachmentSales(ctx, &pb.ListUserAttachmentSalesRequest{UserId: ownerID, Limit: 10})
	if err != nil {
		t.Fatalf("ListUserAttachmentSales() after archive error = %v", err)
	}
	if len(sales.GetItems()) != 2 {
		t.Fatalf("owner archived attachment sale history = %+v", sales.GetItems())
	}
	for _, sale := range sales.GetItems() {
		if sale.GetAttachment().GetStatus() != "ARCHIVED" {
			t.Fatalf("owner archived attachment sale record = %+v", sale)
		}
	}
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        buyerID,
		Delta:         -11,
		Reason:        "file_integration_cleanup",
		Description:   "file-service integration test cleanup",
		SourceEventId: fmt.Sprintf("file-integration-cleanup:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("cleanup buyer credits: %v", err)
	}
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        newBuyerID,
		Delta:         -7,
		Reason:        "file_integration_cleanup",
		Description:   "file-service integration test cleanup",
		SourceEventId: fmt.Sprintf("file-integration-new-buyer-cleanup:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("cleanup new buyer credits: %v", err)
	}
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        ownerID,
		Delta:         -22,
		Reason:        "file_integration_cleanup",
		Description:   "file-service integration test cleanup",
		SourceEventId: fmt.Sprintf("file-integration-owner-cleanup:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("cleanup owner credits: %v", err)
	}
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        transferPayerID,
		Delta:         -4,
		Reason:        "file_integration_cleanup",
		Description:   "file-service integration test cleanup",
		SourceEventId: fmt.Sprintf("file-integration-transfer-payer-cleanup:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("cleanup transfer payer credits: %v", err)
	}
	if _, err := credits.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        transferPayeeID,
		Delta:         -6,
		Reason:        "file_integration_cleanup",
		Description:   "file-service integration test cleanup",
		SourceEventId: fmt.Sprintf("file-integration-transfer-payee-cleanup:%d", stamp),
		SourceType:    "file_integration_test",
		SourceId:      topicID,
	}); err != nil {
		t.Fatalf("cleanup transfer payee credits: %v", err)
	}
}
