package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	creditpb "file-service/api/proto/creditpb"
	pb "file-service/api/proto/filepb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestFileServiceRejectsUnavailableTopicsIntegration(t *testing.T) {
	address := os.Getenv("BBS_FILE_INTEGRATION_ADDR")
	if address == "" {
		t.Skip("set BBS_FILE_INTEGRATION_ADDR to run against a live file-service")
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("connect file-service: %v", err)
	}
	defer conn.Close()
	client := pb.NewFileServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stamp := time.Now().UnixNano()
	topicID := stamp
	ownerID := stamp + 1
	for _, priceCredits := range []int64{0, 1} {
		if _, err := client.CreateAttachment(ctx, &pb.CreateAttachmentRequest{
			TopicId:      topicID,
			OwnerId:      ownerID,
			ObjectKey:    fmt.Sprintf("integration/%d/guide-%d.pdf", stamp, priceCredits),
			OriginalName: "guide.pdf",
			ContentType:  "application/pdf",
			SizeBytes:    128,
			PriceCredits: priceCredits,
		}); status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("CreateAttachment(price=%d) error = %v, want FailedPrecondition", priceCredits, err)
		}
	}
	if _, err := client.ListTopicAttachments(ctx, &pb.ListTopicAttachmentsRequest{TopicId: topicID}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("ListTopicAttachments() error = %v, want FailedPrecondition", err)
	}
}

func TestFileServiceRejectsArchivedTopicIntegration(t *testing.T) {
	address := os.Getenv("BBS_FILE_INTEGRATION_ADDR")
	gatewayBase := strings.TrimRight(strings.TrimSpace(os.Getenv("BBS_GATEWAY_INTEGRATION_BASE")), "/")
	if address == "" || gatewayBase == "" {
		t.Skip("set BBS_FILE_INTEGRATION_ADDR and BBS_GATEWAY_INTEGRATION_BASE to run against live services")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stamp := time.Now().UnixNano()
	author := registerAttachmentIntegrationUser(t, ctx, gatewayBase, stamp)
	topicID := createPublishedAttachmentIntegrationTopic(t, ctx, gatewayBase, author, stamp)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("connect file-service: %v", err)
	}
	defer conn.Close()
	client := pb.NewFileServiceClient(conn)
	created, err := client.CreateAttachment(ctx, &pb.CreateAttachmentRequest{
		TopicId:      topicID,
		OwnerId:      author.UserID,
		ObjectKey:    fmt.Sprintf("integration/%d/topic-archive-guide.pdf", stamp),
		OriginalName: "topic-archive-guide.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    128,
	})
	if err != nil {
		t.Fatalf("CreateAttachment() on published topic error = %v", err)
	}
	attachmentID := created.GetAttachment().GetId()
	if attachmentID <= 0 {
		t.Fatalf("created attachment = %+v", created.GetAttachment())
	}
	listed, err := client.ListTopicAttachments(ctx, &pb.ListTopicAttachmentsRequest{TopicId: topicID})
	if err != nil || len(listed.GetItems()) != 1 || listed.GetItems()[0].GetId() != attachmentID {
		t.Fatalf("ListTopicAttachments() before archive = %+v, %v", listed.GetItems(), err)
	}

	archiveAttachmentIntegrationTopic(t, ctx, gatewayBase, author, topicID)
	if _, err := client.CreateAttachment(ctx, &pb.CreateAttachmentRequest{
		TopicId:      topicID,
		OwnerId:      author.UserID,
		ObjectKey:    fmt.Sprintf("integration/%d/after-topic-archive.pdf", stamp),
		OriginalName: "after-topic-archive.pdf",
		ContentType:  "application/pdf",
		SizeBytes:    128,
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("CreateAttachment() after topic archive error = %v, want FailedPrecondition", err)
	}
	if _, err := client.ListTopicAttachments(ctx, &pb.ListTopicAttachmentsRequest{TopicId: topicID}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("ListTopicAttachments() after topic archive error = %v, want FailedPrecondition", err)
	}
	if _, err := client.UpdateAttachmentPrice(ctx, &pb.UpdateAttachmentPriceRequest{AttachmentId: attachmentID, OwnerId: author.UserID, PriceCredits: 0}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("UpdateAttachmentPrice() after topic archive error = %v, want FailedPrecondition", err)
	}
	if _, err := client.AuthorizeAttachmentDownload(ctx, &pb.AuthorizeAttachmentDownloadRequest{AttachmentId: attachmentID, UserId: author.UserID}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("AuthorizeAttachmentDownload() after topic archive error = %v, want FailedPrecondition", err)
	}
	if _, err := client.ArchiveAttachment(ctx, &pb.ArchiveAttachmentRequest{AttachmentId: attachmentID, OwnerId: author.UserID}); err != nil {
		t.Fatalf("ArchiveAttachment() cleanup error = %v", err)
	}
}

type attachmentIntegrationAuthor struct {
	UserID      int64
	AccessToken string
}

func registerAttachmentIntegrationUser(t *testing.T, ctx context.Context, gatewayBase string, stamp int64) attachmentIntegrationAuthor {
	t.Helper()
	username := fmt.Sprintf("fta%d", stamp)
	var payload struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	attachmentIntegrationRequest(t, ctx, stdhttp.MethodPost, gatewayBase+"/api/v1/auth/register", "", map[string]any{
		"username": username,
		"email":    username + "@example.com",
		"password": "Password123!",
		"nickname": "File Topic Integration",
	}, &payload)
	if payload.User.ID <= 0 || strings.TrimSpace(payload.AccessToken) == "" {
		t.Fatalf("registration response = %+v", payload)
	}
	return attachmentIntegrationAuthor{UserID: payload.User.ID, AccessToken: payload.AccessToken}
}

func createPublishedAttachmentIntegrationTopic(t *testing.T, ctx context.Context, gatewayBase string, author attachmentIntegrationAuthor, stamp int64) int64 {
	t.Helper()
	var categories struct {
		Items []struct {
			ID   int64  `json:"id"`
			Slug string `json:"slug"`
		} `json:"items"`
	}
	attachmentIntegrationRequest(t, ctx, stdhttp.MethodGet, gatewayBase+"/api/v1/categories?status=2&limit=20&offset=0", "", nil, &categories)
	categoryID := int64(0)
	for _, category := range categories.Items {
		if category.Slug == "general" {
			categoryID = category.ID
			break
		}
	}
	if categoryID <= 0 {
		t.Fatalf("general category not found: %+v", categories.Items)
	}
	var created struct {
		Topic struct {
			ID int64 `json:"id"`
		} `json:"topic"`
	}
	attachmentIntegrationRequest(t, ctx, stdhttp.MethodPost, gatewayBase+"/api/v1/topics", author.AccessToken, map[string]any{
		"slug":        fmt.Sprintf("file-topic-access-%d", stamp),
		"type":        "topic",
		"title":       fmt.Sprintf("File topic access %d", stamp),
		"body":        "A published topic for file-service integration coverage.",
		"tags":        []string{"attachments", "integration"},
		"category_id": categoryID,
		"publish":     true,
	}, &created)
	if created.Topic.ID <= 0 {
		t.Fatalf("topic creation response = %+v", created)
	}
	return created.Topic.ID
}

func archiveAttachmentIntegrationTopic(t *testing.T, ctx context.Context, gatewayBase string, author attachmentIntegrationAuthor, topicID int64) {
	t.Helper()
	attachmentIntegrationRequest(t, ctx, stdhttp.MethodDelete, fmt.Sprintf("%s/api/v1/topics/%d", gatewayBase, topicID), author.AccessToken, nil, nil)
}

func attachmentIntegrationRequest(t *testing.T, ctx context.Context, method, url, accessToken string, body any, target any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := stdhttp.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(accessToken) != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	response, err := stdhttp.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer response.Body.Close()
	encoded, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, url, err)
	}
	var envelope struct {
		Code    int64           `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("decode %s %s response: %v, body=%s", method, url, err, encoded)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || envelope.Code != 0 {
		t.Fatalf("%s %s failed: status=%d code=%d message=%s body=%s", method, url, response.StatusCode, envelope.Code, envelope.Message, encoded)
	}
	if target != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			t.Fatalf("decode %s %s data: %v, body=%s", method, url, err, encoded)
		}
	}
}

func TestFileServiceIntegration(t *testing.T) {
	address := os.Getenv("BBS_FILE_INTEGRATION_ADDR")
	creditAddress := os.Getenv("BBS_CREDIT_INTEGRATION_ADDR")
	memberOwnerID, err := strconv.ParseInt(os.Getenv("BBS_FILE_INTEGRATION_MEMBER_OWNER_ID"), 10, 64)
	topicID, topicErr := strconv.ParseInt(os.Getenv("BBS_FILE_INTEGRATION_TOPIC_ID"), 10, 64)
	if address == "" || creditAddress == "" || err != nil || topicErr != nil || memberOwnerID <= 0 || topicID <= 0 {
		t.Skip("set BBS_FILE_INTEGRATION_ADDR, BBS_CREDIT_INTEGRATION_ADDR, BBS_FILE_INTEGRATION_MEMBER_OWNER_ID, and a published BBS_FILE_INTEGRATION_TOPIC_ID owned by that member to run the paid live flow")
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
	ownerID := memberOwnerID
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
	if len(sales.GetItems()) != 2 || sales.GetTotal() != 2 || sales.GetTotalEarnedCredits() != 22 {
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
	if len(buyerSales.GetItems()) != 0 || buyerSales.GetTotal() != 0 || buyerSales.GetTotalEarnedCredits() != 0 {
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
	if len(sales.GetItems()) != 2 || sales.GetTotal() != 2 || sales.GetTotalEarnedCredits() != 22 {
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
