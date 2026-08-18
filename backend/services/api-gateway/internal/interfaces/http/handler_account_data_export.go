package http

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"api-gateway/api/proto/filepb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

const (
	accountDataExportPageSize = int32(100)
	accountDataExportTimeout  = 15 * time.Minute
)

var errAccountDataStoredObjectUnavailable = errors.New("account data stored object unavailable")

type accountDataUserRecord struct {
	ID                     string `json:"id"`
	Username               string `json:"username"`
	Email                  string `json:"email"`
	Status                 int32  `json:"status"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
	LastLoginAt            string `json:"lastLoginAt,omitempty"`
	EmailVerified          bool   `json:"emailVerified"`
	EmailVerifiedAt        string `json:"emailVerifiedAt,omitempty"`
	AccountState           string `json:"accountState"`
	FollowApprovalRequired bool   `json:"followApprovalRequired"`
}

type accountDataProfileRecord struct {
	UserID        string `json:"userId"`
	Name          string `json:"name"`
	AvatarURL     string `json:"avatarUrl"`
	Bio           string `json:"bio"`
	BackgroundURL string `json:"backgroundUrl"`
	ProfileTheme  string `json:"profileTheme"`
}

type accountDataLoginEventRecord struct {
	SessionID     string `json:"sessionId,omitempty"`
	IPAddress     string `json:"ipAddress"`
	UserAgent     string `json:"userAgent"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failureReason,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

type accountDataDriveRecord struct {
	FileName string                     `json:"fileName"`
	File     accountDataDriveFileRecord `json:"file"`
}

type accountDataDriveFileRecord struct {
	ID          string  `json:"id"`
	CreatedAt   string  `json:"createdAt"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Size        int64   `json:"size"`
	IsSensitive bool    `json:"isSensitive"`
	Comment     string  `json:"comment"`
	FolderID    *string `json:"folderId"`
	URL         string  `json:"url"`
	Status      string  `json:"status"`
	BizType     string  `json:"bizType"`
}

type accountDataAttachmentRecord struct {
	FileName   string  `json:"fileName"`
	ID         string  `json:"id"`
	TopicID    string  `json:"topicId"`
	CreatedAt  string  `json:"createdAt"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Size       int64   `json:"size"`
	Price      int64   `json:"priceCredits"`
	Status     string  `json:"status"`
	ArchivedAt *string `json:"archivedAt"`
}

func (h *Handler) exportAccountData(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.User == nil || h.clients.UserSessions == nil ||
		h.clients.Content == nil || h.clients.File == nil || h.clients.Reaction == nil ||
		h.clients.UserSafety == nil || h.clients.UserLists == nil || h.clients.UserAntennas == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "account data export dependencies unavailable", "service_unavailable")
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "account data", filenamePrefix: "data-request", exportedEntity: "data",
		extension: ".zip", contentType: "application/zip", gate: h.accountDataExportGate,
		buildArtifact: h.buildAccountDataExportArtifact, timeout: accountDataExportTimeout,
	})
}

func (h *Handler) buildAccountDataExportArtifact(ctx context.Context, userID int64) (userExportArtifact, error) {
	host, err := h.exportAccountHost()
	if err != nil {
		return userExportArtifact{}, err
	}
	userResponse, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: userID})
	if err != nil {
		return userExportArtifact{}, err
	}
	user := userResponse.GetUser()
	if user == nil || user.GetId() != userID {
		return userExportArtifact{}, errors.New("account data export user unavailable")
	}

	exportedAt := time.Now().UTC()
	notes, err := h.buildNoteExport(ctx, userID)
	if err != nil {
		return userExportArtifact{}, err
	}
	followingCSV, err := h.buildFollowingExport(ctx, userID, followingExportRequest{ExcludeMuting: true, ExcludeInactive: true})
	if err != nil {
		return userExportArtifact{}, err
	}
	followers, err := h.accountDataFollowers(ctx, userID, host)
	if err != nil {
		return userExportArtifact{}, err
	}
	mutingCSV, err := h.buildSafetyRelationExport(ctx, userID, false)
	if err != nil {
		return userExportArtifact{}, err
	}
	blockingCSV, err := h.buildSafetyRelationExport(ctx, userID, true)
	if err != nil {
		return userExportArtifact{}, err
	}
	favorites, err := h.buildFavoriteExport(ctx, userID)
	if err != nil {
		return userExportArtifact{}, err
	}
	antennas, err := h.buildAntennaExport(ctx, userID)
	if err != nil {
		return userExportArtifact{}, err
	}
	lists, err := h.buildUserListsExport(ctx, userID)
	if err != nil {
		return userExportArtifact{}, err
	}
	loginEvents, err := h.accountDataLoginEvents(ctx, userID)
	if err != nil {
		return userExportArtifact{}, err
	}
	files, err := h.accountDataFiles(ctx, userID)
	if err != nil {
		return userExportArtifact{}, err
	}
	attachments, err := h.accountDataAttachments(ctx, userID)
	if err != nil {
		return userExportArtifact{}, err
	}

	temp, err := os.CreateTemp("", "bbs-account-data-*.zip")
	if err != nil {
		return userExportArtifact{}, err
	}
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}
	fail := func(err error) (userExportArtifact, error) {
		cleanup()
		return userExportArtifact{}, err
	}

	archive := zip.NewWriter(temp)
	userRecord := accountDataUserRecord{
		ID: strconv.FormatInt(user.GetId(), 10), Username: user.GetUsername(), Email: user.GetEmail(), Status: user.GetStatus(),
		CreatedAt: accountDataTimestamp(user.GetCreatedAt()), UpdatedAt: accountDataTimestamp(user.GetUpdatedAt()),
		LastLoginAt: accountDataOptionalTimestamp(user.GetLastLoginAt()), EmailVerified: user.GetEmailVerified(),
		EmailVerifiedAt: accountDataOptionalTimestamp(user.GetEmailVerifiedAt()), AccountState: user.GetAccountState(),
		FollowApprovalRequired: user.GetFollowApprovalRequired(),
	}
	profileRecord := accountDataProfileRecord{
		UserID: strconv.FormatInt(user.GetId(), 10), Name: user.GetNickname(), AvatarURL: user.GetAvatarUrl(),
		Bio: user.GetBio(), BackgroundURL: user.GetBackgroundUrl(), ProfileTheme: user.GetProfileTheme(),
	}
	entries := []struct {
		name string
		key  string
		data any
		raw  json.RawMessage
	}{
		{name: "user.json", key: "user", data: []accountDataUserRecord{userRecord}},
		{name: "profile.json", key: "profile", data: []accountDataProfileRecord{profileRecord}},
		{name: "ips.json", key: "ips", data: loginEvents},
		{name: "notes.json", key: "notes", raw: notes},
		{name: "followings.json", key: "followings", data: accountDataAccountsFromCSV(followingCSV)},
		{name: "followers.json", key: "followers", data: followers},
		{name: "mutings.json", key: "mutings", data: accountDataAccountsFromCSV(mutingCSV)},
		{name: "blockings.json", key: "blockings", data: accountDataAccountsFromCSV(blockingCSV)},
		{name: "favorites.json", key: "favorites", raw: favorites},
		{name: "antennas.json", key: "antennas", raw: antennas},
	}
	for _, entry := range entries {
		payload, marshalErr := accountDataEnvelope(host, exportedAt, entry.key, entry.data, entry.raw)
		if marshalErr != nil {
			_ = archive.Close()
			return fail(marshalErr)
		}
		if writeErr := accountDataWriteZipBytes(archive, entry.name, payload, exportedAt); writeErr != nil {
			_ = archive.Close()
			return fail(writeErr)
		}
	}

	driveRecords := make([]accountDataDriveRecord, 0, len(files))
	for _, file := range files {
		fileName := accountDataArchiveFileName(file)
		driveRecords = append(driveRecords, accountDataDriveRecord{
			FileName: fileName,
			File: accountDataDriveFileRecord{
				ID: strconv.FormatInt(file.GetId(), 10), CreatedAt: accountDataTimestamp(file.GetCreatedAt()),
				Name: file.GetOriginalName(), Type: file.GetContentType(), Size: file.GetSizeBytes(),
				IsSensitive: file.GetIsSensitive(), Comment: file.GetComment(), FolderID: accountDataFolderID(file.GetFolderId()),
				URL:    h.publicBaseURL + "/api/v1/files/" + strconv.FormatInt(file.GetId(), 10) + "/download",
				Status: file.GetStatus(), BizType: file.GetBizType(),
			},
		})
		if copyErr := h.accountDataWriteStoredFile(ctx, archive, file, fileName, exportedAt); copyErr != nil {
			if ctx.Err() != nil {
				_ = archive.Close()
				return fail(ctx.Err())
			}
			if !errors.Is(copyErr, errAccountDataStoredObjectUnavailable) {
				_ = archive.Close()
				return fail(copyErr)
			}
			// Keep metadata when a historical object is unavailable, matching the
			// reference export's best-effort drive file copy behavior.
		}
	}
	drivePayload, err := accountDataEnvelope(host, exportedAt, "drive", driveRecords, nil)
	if err != nil {
		_ = archive.Close()
		return fail(err)
	}
	if err := accountDataWriteZipBytes(archive, "drive.json", drivePayload, exportedAt); err != nil {
		_ = archive.Close()
		return fail(err)
	}
	attachmentRecords := make([]accountDataAttachmentRecord, 0, len(attachments))
	for _, attachment := range attachments {
		fileName := accountDataArchiveAttachmentName(attachment)
		attachmentRecords = append(attachmentRecords, accountDataAttachmentRecord{
			FileName: fileName, ID: strconv.FormatInt(attachment.GetId(), 10), TopicID: strconv.FormatInt(attachment.GetTopicId(), 10),
			CreatedAt: accountDataTimestamp(attachment.GetCreatedAt()), Name: attachment.GetOriginalName(), Type: attachment.GetContentType(),
			Size: attachment.GetSizeBytes(), Price: attachment.GetPriceCredits(), Status: attachment.GetStatus(),
			ArchivedAt: accountDataOptionalTimestampPointer(attachment.GetArchivedAt()),
		})
		copyErr := h.accountDataWriteStoredObject(ctx, archive, attachment.GetObjectKey(), "attachments/"+fileName, exportedAt)
		if copyErr != nil {
			if ctx.Err() != nil {
				_ = archive.Close()
				return fail(ctx.Err())
			}
			if !errors.Is(copyErr, errAccountDataStoredObjectUnavailable) {
				_ = archive.Close()
				return fail(copyErr)
			}
		}
	}
	attachmentPayload, err := accountDataEnvelope(host, exportedAt, "attachments", attachmentRecords, nil)
	if err != nil {
		_ = archive.Close()
		return fail(err)
	}
	if err := accountDataWriteZipBytes(archive, "attachments.json", attachmentPayload, exportedAt); err != nil {
		_ = archive.Close()
		return fail(err)
	}
	if err := accountDataWriteZipBytes(archive, "lists.csv", lists, exportedAt); err != nil {
		_ = archive.Close()
		return fail(err)
	}
	if err := archive.Close(); err != nil {
		return fail(err)
	}
	info, err := temp.Stat()
	if err != nil {
		return fail(err)
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	return userExportArtifact{reader: temp, size: info.Size(), cleanup: cleanup}, nil
}

func (h *Handler) accountDataFollowers(ctx context.Context, userID int64, host string) ([]string, error) {
	result := make([]string, 0)
	var afterID int64
	for {
		response, err := h.clients.User.ListFollowers(ctx, &userpb.ListFollowsRequest{
			UserId: userID, PageSize: accountDataExportPageSize, AfterUserId: afterID, AscendingByUserId: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, follower := range items {
			if follower == nil || follower.GetId() <= afterID || strings.TrimSpace(follower.GetUsername()) == "" {
				return nil, errors.New("invalid account data follower")
			}
			afterID = follower.GetId()
			result = append(result, follower.GetUsername()+"@"+host)
		}
		if len(items) < int(accountDataExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) accountDataLoginEvents(ctx context.Context, userID int64) ([]accountDataLoginEventRecord, error) {
	result := make([]accountDataLoginEventRecord, 0)
	var afterID int64
	for {
		response, err := h.clients.UserSessions.ListLoginEvents(ctx, &userpb.ListLoginEventsRequest{
			UserId: userID, Limit: accountDataExportPageSize, AfterId: afterID, AscendingById: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			id, parseErr := strconv.ParseInt(item.GetId(), 10, 64)
			if parseErr != nil || id <= afterID || item.GetUserId() != userID {
				return nil, errors.New("invalid account data login event")
			}
			afterID = id
			result = append(result, accountDataLoginEventRecord{
				SessionID: item.GetSessionId(), IPAddress: item.GetIpAddress(), UserAgent: item.GetUserAgent(),
				Success: item.GetSuccess(), FailureReason: item.GetFailureReason(), CreatedAt: accountDataTimestamp(item.GetCreatedAt()),
			})
		}
		if len(items) < int(accountDataExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) accountDataFiles(ctx context.Context, userID int64) ([]*filepb.File, error) {
	result := make([]*filepb.File, 0)
	var afterID int64
	for {
		response, err := h.clients.File.ListFiles(ctx, &filepb.ListFilesRequest{
			OwnerId: userID, Limit: accountDataExportPageSize, AfterId: afterID, AscendingById: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, file := range items {
			if file == nil || file.GetId() <= afterID || file.GetOwnerId() != userID || strings.TrimSpace(file.GetObjectKey()) == "" {
				return nil, errors.New("invalid account data file")
			}
			afterID = file.GetId()
			result = append(result, file)
		}
		if len(items) < int(accountDataExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) accountDataAttachments(ctx context.Context, userID int64) ([]*filepb.Attachment, error) {
	result := make([]*filepb.Attachment, 0)
	var afterID int64
	for {
		response, err := h.clients.File.ListOwnedAttachments(ctx, &filepb.ListOwnedAttachmentsRequest{
			OwnerId: userID, AfterId: afterID, Limit: accountDataExportPageSize, AscendingById: true,
		})
		if err != nil {
			return nil, err
		}
		items := response.GetItems()
		if len(items) == 0 {
			break
		}
		for _, attachment := range items {
			if attachment == nil || attachment.GetId() <= afterID || attachment.GetOwnerId() != userID || strings.TrimSpace(attachment.GetObjectKey()) == "" {
				return nil, errors.New("invalid account data attachment")
			}
			afterID = attachment.GetId()
			result = append(result, attachment)
		}
		if len(items) < int(accountDataExportPageSize) {
			break
		}
	}
	return result, nil
}

func (h *Handler) accountDataWriteStoredFile(ctx context.Context, archive *zip.Writer, file *filepb.File, fileName string, modified time.Time) error {
	return h.accountDataWriteStoredObject(ctx, archive, file.GetObjectKey(), "files/"+fileName, modified)
}

func (h *Handler) accountDataWriteStoredObject(ctx context.Context, archive *zip.Writer, objectKey, archiveName string, modified time.Time) error {
	object, _, err := h.attachments.Open(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("%w: %v", errAccountDataStoredObjectUnavailable, err)
	}
	defer object.Close()
	temp, err := os.CreateTemp("", "bbs-account-data-object-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}()
	if _, err := io.Copy(temp, object); err != nil {
		return fmt.Errorf("%w: %v", errAccountDataStoredObjectUnavailable, err)
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	header := &zip.FileHeader{Name: archiveName, Method: zip.Store, Modified: modified}
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, temp)
	return err
}

func accountDataEnvelope(host string, exportedAt time.Time, key string, value any, raw json.RawMessage) ([]byte, error) {
	if raw == nil {
		var err error
		raw, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	if !json.Valid(raw) {
		return nil, errors.New("invalid account data JSON")
	}
	hostJSON, _ := json.Marshal(host)
	timeJSON, _ := json.Marshal(exportedAt.Format(noteExportTimestampLayout))
	keyJSON, _ := json.Marshal(key)
	return []byte(fmt.Sprintf(`{"metaVersion":1,"host":%s,"exportedAt":%s,%s:%s}`, hostJSON, timeJSON, keyJSON, raw)), nil
}

func accountDataWriteZipBytes(archive *zip.Writer, name string, payload []byte, modified time.Time) error {
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: modified})
	if err != nil {
		return err
	}
	_, err = entry.Write(payload)
	return err
}

func accountDataAccountsFromCSV(payload []byte) []string {
	lines := strings.Split(strings.ReplaceAll(string(payload), "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if value := strings.TrimSpace(line); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func accountDataArchiveFileName(file *filepb.File) string {
	return strconv.FormatInt(file.GetId(), 10) + "-" + accountDataSafeArchiveName(file.GetOriginalName())
}

func accountDataArchiveAttachmentName(attachment *filepb.Attachment) string {
	return strconv.FormatInt(attachment.GetId(), 10) + "-" + accountDataSafeArchiveName(attachment.GetOriginalName())
}

func accountDataSafeArchiveName(value string) string {
	name := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(value))
	if name == "" || name == "." || name == ".." {
		name = "file"
	}
	return name
}

func accountDataFolderID(value int64) *string {
	if value <= 0 {
		return nil
	}
	result := strconv.FormatInt(value, 10)
	return &result
}

func accountDataTimestamp(value int64) string {
	return time.UnixMilli(value).UTC().Format(noteExportTimestampLayout)
}

func accountDataOptionalTimestamp(value int64) string {
	if value <= 0 {
		return ""
	}
	return accountDataTimestamp(value)
}

func accountDataOptionalTimestampPointer(value int64) *string {
	if value <= 0 {
		return nil
	}
	result := accountDataTimestamp(value)
	return &result
}
