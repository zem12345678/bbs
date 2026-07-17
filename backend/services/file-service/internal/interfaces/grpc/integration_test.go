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
	"google.golang.org/grpc/credentials/insecure"
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

	archived, err := client.ArchiveAttachment(ctx, &pb.ArchiveAttachmentRequest{AttachmentId: attachment.GetId(), OwnerId: ownerID})
	if err != nil {
		t.Fatalf("ArchiveAttachment() error = %v", err)
	}
	if archived.GetAttachment().GetStatus() != "ARCHIVED" {
		t.Fatalf("archived attachment = %+v", archived.GetAttachment())
	}

	listed, err = client.ListTopicAttachments(ctx, &pb.ListTopicAttachmentsRequest{TopicId: topicID})
	if err != nil {
		t.Fatalf("ListTopicAttachments() after archive error = %v", err)
	}
	if len(listed.GetItems()) != 0 {
		t.Fatalf("active attachments after archive = %+v", listed.GetItems())
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
}
