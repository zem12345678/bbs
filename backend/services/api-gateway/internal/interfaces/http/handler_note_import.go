package http

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"strings"
	"time"
	"unicode/utf8"

	"api-gateway/api/proto/contentpb"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	noteImportMaxBytes      = int64(50 << 20)
	noteImportMaxEntryBytes = int64(50 << 20)
	noteImportTimeout       = 2 * time.Minute
	noteImportTitleMaxRunes = 160
	noteImportBodyMaxRunes  = 500000
)

var noteImportSources = map[string]struct{}{
	"Misskey":   {},
	"Mastodon":  {},
	"Pleroma":   {},
	"Twitter":   {},
	"Instagram": {},
	"Facebook":  {},
}

type noteImportRequest struct {
	FileID string `json:"fileId"`
	Type   string `json:"type"`
}

type noteImportPoll struct {
	Multiple  bool
	Choices   []string
	ExpiresAt time.Time
}

type noteImportRecord struct {
	ID         string
	Text       string
	CW         string
	Visibility string
	Tags       []string
	Poll       *noteImportPoll
}

type noteImportResult struct {
	Imported int `json:"imported"`
	Drafts   int `json:"drafts"`
	Skipped  int `json:"skipped"`
}

func (h *Handler) importNotes(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.Content == nil || h.clients.File == nil || h.clients.User == nil || h.attachments == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "note import dependencies unavailable", "service_unavailable")
		return
	}
	var request noteImportRequest
	if !bindJSON(c, &request) {
		return
	}
	fileID, ok := parseImportFileID(c, request.FileID)
	if !ok {
		return
	}
	source := strings.TrimSpace(request.Type)
	if _, ok := noteImportSources[source]; !ok {
		writeError(c, stdhttp.StatusBadRequest, "unsupported note import source", "invalid_argument")
		return
	}
	ownerID := currentUserID(c)
	if !h.allowNoteImportRateLimit(c, ownerID) {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), noteImportTimeout)
	defer cancel()
	payload, ok := h.readOwnedImportFile(c, ctx, ownerID, fileID, noteImportMaxBytes)
	if !ok {
		return
	}
	records, err := parseNoteImportRecords(payload, source)
	if err != nil {
		writeError(c, stdhttp.StatusBadRequest, "invalid note import file", "bad_request")
		return
	}
	if len(records) == 0 {
		c.JSON(stdhttp.StatusOK, noteImportResult{})
		return
	}
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	result, err := h.applyNoteImport(ctx, ownerID, records)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, result)
}

func (h *Handler) applyNoteImport(ctx context.Context, ownerID int64, records []noteImportRecord) (noteImportResult, error) {
	result := noteImportResult{}
	stamp := time.Now().UnixNano()
	for index, record := range records {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		text := strings.TrimSpace(record.Text)
		if text == "" || utf8.RuneCountInString(text) > noteImportBodyMaxRunes {
			result.Skipped++
			continue
		}
		slug := noteImportSlug(ownerID, record, index, stamp)
		title := noteImportTitle(text)
		poll := noteImportTopicPoll(record.Poll)
		if poll != nil {
			response, err := h.clients.Content.CreateTopic(ctx, &contentpb.CreateTopicRequest{
				Slug: slug, Type: "topic", Title: title, Body: text, Tags: record.Tags,
				AuthorId: ownerID, CategoryId: 1, Poll: poll,
			})
			if err != nil {
				if noteImportInfrastructureError(err) {
					return result, err
				}
				result.Skipped++
				continue
			}
			topic := response.GetTopic()
			if topic == nil || topic.GetId() <= 0 {
				result.Skipped++
				continue
			}
			result.Imported++
			if record.Visibility == "public" {
				if _, err := h.clients.Content.PublishTopic(ctx, &contentpb.TopicIDRequest{Id: topic.GetId()}); err != nil {
					if noteImportInfrastructureError(err) {
						return result, err
					}
					result.Drafts++
				}
			} else {
				result.Drafts++
			}
			continue
		}

		response, err := h.clients.Content.CreateArticle(ctx, &contentpb.CreateArticleRequest{
			Slug: slug, Title: title, Summary: truncateNoteImportText(record.CW, 180), Body: text,
			Tags: record.Tags, AuthorId: ownerID,
		})
		if err != nil {
			if noteImportInfrastructureError(err) {
				return result, err
			}
			result.Skipped++
			continue
		}
		article := response.GetArticle()
		if article == nil || article.GetId() <= 0 {
			result.Skipped++
			continue
		}
		result.Imported++
		if record.Visibility == "public" {
			if _, err := h.clients.Content.PublishArticle(ctx, &contentpb.ArticleIDRequest{Id: article.GetId()}); err != nil {
				if noteImportInfrastructureError(err) {
					return result, err
				}
				result.Drafts++
			}
		} else {
			result.Drafts++
		}
	}
	return result, nil
}

func parseNoteImportRecords(payload []byte, source string) ([]noteImportRecord, error) {
	data := payload
	var err error
	switch source {
	case "Misskey":
		data, err = noteImportArchiveEntry(payload, "notes.json")
		if err != nil {
			return nil, err
		}
		return parseMisskeyNoteImport(data)
	case "Mastodon", "Pleroma":
		data, err = noteImportArchiveEntry(payload, "outbox.json")
		if err != nil {
			return nil, err
		}
		return parseActivityPubNoteImport(data)
	case "Twitter":
		data, err = noteImportArchiveEntry(payload, "data/tweets.js", "tweets.js")
		if err != nil {
			return nil, err
		}
		return parseTwitterNoteImport(data)
	case "Instagram":
		data, err = noteImportArchiveEntry(payload, "content/posts_1.json", "posts_1.json")
		if err != nil {
			return nil, err
		}
		return parseInstagramNoteImport(data)
	case "Facebook":
		data, err = noteImportArchiveEntry(payload, "your_posts__check_ins__photos_and_videos_1.json")
		if err != nil {
			return nil, err
		}
		return parseFacebookNoteImport(data)
	default:
		return nil, errors.New("unsupported note import source")
	}
}

func noteImportArchiveEntry(payload []byte, candidates ...string) ([]byte, error) {
	if len(payload) < 2 || payload[0] != 'P' || payload[1] != 'K' {
		return payload, nil
	}
	archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, err
	}
	for _, entry := range archive.File {
		name := strings.ToLower(strings.ReplaceAll(entry.Name, "\\", "/"))
		for _, candidate := range candidates {
			candidate = strings.ToLower(strings.TrimPrefix(candidate, "/"))
			if name != candidate && !strings.HasSuffix(name, "/"+candidate) {
				continue
			}
			if entry.FileInfo().IsDir() || int64(entry.UncompressedSize64) > noteImportMaxEntryBytes {
				return nil, errors.New("note import archive entry too large")
			}
			reader, err := entry.Open()
			if err != nil {
				return nil, err
			}
			data, readErr := io.ReadAll(io.LimitReader(reader, noteImportMaxEntryBytes+1))
			_ = reader.Close()
			if readErr != nil {
				return nil, readErr
			}
			if int64(len(data)) > noteImportMaxEntryBytes {
				return nil, errors.New("note import archive entry too large")
			}
			return data, nil
		}
	}
	return nil, errors.New("note import archive entry not found")
}

type misskeyNoteImportJSON struct {
	ID         string          `json:"id"`
	Text       *string         `json:"text"`
	CW         *string         `json:"cw"`
	Visibility string          `json:"visibility"`
	Tags       []string        `json:"tags"`
	RenoteID   json.RawMessage `json:"renoteId"`
	Poll       *struct {
		Multiple  bool            `json:"multiple"`
		Choices   []string        `json:"choices"`
		ExpiresAt json.RawMessage `json:"expiresAt"`
	} `json:"poll"`
}

func parseMisskeyNoteImport(payload []byte) ([]noteImportRecord, error) {
	var notes []misskeyNoteImportJSON
	if err := json.Unmarshal(payload, &notes); err != nil {
		var wrapper struct {
			Notes []misskeyNoteImportJSON `json:"notes"`
		}
		if wrapperErr := json.Unmarshal(payload, &wrapper); wrapperErr != nil {
			return nil, err
		}
		notes = wrapper.Notes
	}
	result := make([]noteImportRecord, 0, len(notes))
	for _, note := range notes {
		if rawImportIdentifier(note.RenoteID) != "" || note.Text == nil {
			continue
		}
		text := strings.TrimSpace(*note.Text)
		if text == "" {
			continue
		}
		record := noteImportRecord{
			ID: note.ID, Text: text, CW: strings.TrimSpace(valueOrEmpty(note.CW)),
			Visibility: normalizeNoteImportVisibility(note.Visibility), Tags: normalizeNoteImportTags(note.Tags),
		}
		if note.Poll != nil {
			record.Poll = parseNoteImportPoll(note.Poll.Multiple, note.Poll.Choices, note.Poll.ExpiresAt)
		}
		result = appendNoteImportRecord(result, record)
	}
	return result, nil
}

type activityPubTag struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func parseActivityPubNoteImport(payload []byte) ([]noteImportRecord, error) {
	var envelope struct {
		OrderedItems []json.RawMessage `json:"orderedItems"`
		Items        []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, err
	}
	items := envelope.OrderedItems
	if len(items) == 0 {
		items = envelope.Items
	}
	result := make([]noteImportRecord, 0, len(items))
	for _, raw := range items {
		var activity struct {
			Type   string          `json:"type"`
			Object json.RawMessage `json:"object"`
		}
		if json.Unmarshal(raw, &activity) != nil {
			continue
		}
		if activity.Type != "" && !strings.EqualFold(activity.Type, "Create") {
			continue
		}
		objectRaw := raw
		if len(activity.Object) > 0 && string(activity.Object) != "null" {
			objectRaw = activity.Object
		}
		var object struct {
			ID        string           `json:"id"`
			Type      string           `json:"type"`
			Content   string           `json:"content"`
			Name      string           `json:"name"`
			Summary   string           `json:"summary"`
			Sensitive bool             `json:"sensitive"`
			To        json.RawMessage  `json:"to"`
			CC        json.RawMessage  `json:"cc"`
			Tag       []activityPubTag `json:"tag"`
		}
		if json.Unmarshal(objectRaw, &object) != nil {
			continue
		}
		if object.Type != "" && !strings.EqualFold(object.Type, "Note") && !strings.EqualFold(object.Type, "Article") {
			continue
		}
		text := plainTextFromHTML(object.Content)
		if text == "" {
			text = strings.TrimSpace(object.Name)
		}
		if text == "" {
			continue
		}
		visibility := activityPubVisibility(object.To, object.CC)
		cw := ""
		if object.Sensitive {
			cw = strings.TrimSpace(object.Summary)
		}
		tags := make([]string, 0, len(object.Tag))
		for _, tag := range object.Tag {
			if strings.EqualFold(tag.Type, "Hashtag") {
				tags = append(tags, strings.TrimPrefix(strings.TrimSpace(tag.Name), "#"))
			}
		}
		result = appendNoteImportRecord(result, noteImportRecord{
			ID: object.ID, Text: text, CW: cw, Visibility: visibility,
			Tags: normalizeNoteImportTags(tags),
		})
	}
	return result, nil
}

func parseTwitterNoteImport(payload []byte) ([]noteImportRecord, error) {
	start := bytes.IndexByte(payload, '[')
	end := bytes.LastIndexByte(payload, ']')
	if start < 0 || end < start {
		return nil, errors.New("twitter notes array not found")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(payload[start:end+1], &items); err != nil {
		return nil, err
	}
	result := make([]noteImportRecord, 0, len(items))
	for index, raw := range items {
		var wrapper struct {
			Tweet json.RawMessage `json:"tweet"`
		}
		_ = json.Unmarshal(raw, &wrapper)
		if len(wrapper.Tweet) > 0 && string(wrapper.Tweet) != "null" {
			raw = wrapper.Tweet
		}
		var tweet struct {
			ID       string `json:"id_str"`
			FullText string `json:"full_text"`
			Text     string `json:"text"`
			Entities struct {
				Hashtags []struct {
					Text string `json:"text"`
				} `json:"hashtags"`
			} `json:"entities"`
		}
		if json.Unmarshal(raw, &tweet) != nil {
			continue
		}
		text := strings.TrimSpace(tweet.FullText)
		if text == "" {
			text = strings.TrimSpace(tweet.Text)
		}
		if text == "" {
			continue
		}
		tags := make([]string, 0, len(tweet.Entities.Hashtags))
		for _, tag := range tweet.Entities.Hashtags {
			tags = append(tags, tag.Text)
		}
		id := tweet.ID
		if id == "" {
			id = fmt.Sprintf("twitter-%d", index)
		}
		result = appendNoteImportRecord(result, noteImportRecord{ID: id, Text: text, Visibility: "public", Tags: normalizeNoteImportTags(tags)})
	}
	return result, nil
}

func parseInstagramNoteImport(payload []byte) ([]noteImportRecord, error) {
	var posts []struct {
		Title string `json:"title"`
		Media []struct {
			Title string `json:"title"`
		} `json:"media"`
	}
	if err := json.Unmarshal(payload, &posts); err != nil {
		return nil, err
	}
	result := make([]noteImportRecord, 0, len(posts))
	for index, post := range posts {
		text := post.Title
		if len(post.Media) == 1 {
			text = post.Media[0].Title
		}
		result = appendNoteImportRecord(result, noteImportRecord{
			ID: fmt.Sprintf("instagram-%d", index), Text: decodeMetaNoteImportText(text), Visibility: "public",
		})
	}
	return result, nil
}

func parseFacebookNoteImport(payload []byte) ([]noteImportRecord, error) {
	var posts []struct {
		Data []struct {
			Post string `json:"post"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &posts); err != nil {
		return nil, err
	}
	result := make([]noteImportRecord, 0, len(posts))
	for index, post := range posts {
		if len(post.Data) == 0 {
			continue
		}
		result = appendNoteImportRecord(result, noteImportRecord{
			ID: fmt.Sprintf("facebook-%d", index), Text: decodeMetaNoteImportText(post.Data[0].Post), Visibility: "public",
		})
	}
	return result, nil
}

func decodeMetaNoteImportText(value string) string {
	runes := []rune(value)
	data := make([]byte, len(runes))
	for index, current := range runes {
		if current > 255 {
			return strings.TrimSpace(value)
		}
		data[index] = byte(current)
	}
	if !utf8.Valid(data) {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(string(data))
}

func appendNoteImportRecord(result []noteImportRecord, record noteImportRecord) []noteImportRecord {
	if strings.TrimSpace(record.Text) == "" {
		return result
	}
	record.Visibility = normalizeNoteImportVisibility(record.Visibility)
	return append(result, record)
}

func parseNoteImportPoll(multiple bool, choices []string, expiresAt json.RawMessage) *noteImportPoll {
	seen := make(map[string]struct{}, len(choices))
	clean := make([]string, 0, len(choices))
	for _, choice := range choices {
		choice = truncateNoteImportText(strings.TrimSpace(choice), 80)
		key := strings.ToLower(choice)
		if choice == "" || key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, choice)
		if len(clean) == 10 {
			break
		}
	}
	if len(clean) < 2 {
		return nil
	}
	var expiry time.Time
	if len(expiresAt) > 0 && string(expiresAt) != "null" {
		var value string
		if json.Unmarshal(expiresAt, &value) == nil {
			expiry, _ = time.Parse(time.RFC3339Nano, value)
		}
	}
	return &noteImportPoll{Multiple: multiple, Choices: clean, ExpiresAt: expiry}
}

func noteImportTopicPoll(poll *noteImportPoll) *contentpb.TopicPollInput {
	if poll == nil || len(poll.Choices) < 2 {
		return nil
	}
	result := &contentpb.TopicPollInput{Enabled: true, Multiple: poll.Multiple, Choices: poll.Choices}
	if !poll.ExpiresAt.IsZero() && poll.ExpiresAt.After(time.Now()) {
		result.ExpiresAt = poll.ExpiresAt.UnixMilli()
	}
	return result
}

func noteImportInfrastructureError(err error) bool {
	switch status.Code(err) {
	case codes.Canceled, codes.DeadlineExceeded, codes.Unavailable, codes.ResourceExhausted, codes.Internal:
		return true
	default:
		return false
	}
}

func noteImportSlug(ownerID int64, record noteImportRecord, index int, stamp int64) string {
	seed := fmt.Sprintf("%d:%d:%s:%s:%d", ownerID, index, record.ID, record.Text, stamp)
	digest := sha256.Sum256([]byte(seed))
	return "imported-" + hex.EncodeToString(digest[:10])
}

func noteImportTitle(text string) string {
	text = strings.TrimSpace(text)
	if index := strings.IndexAny(text, "\r\n"); index >= 0 {
		text = text[:index]
	}
	if text == "" {
		return "Imported note"
	}
	return truncateNoteImportText(text, noteImportTitleMaxRunes)
}

func truncateNoteImportText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	return string([]rune(value)[:maxRunes])
}

func normalizeNoteImportVisibility(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "public":
		return "public"
	case "":
		return "public"
	case "home", "followers", "specified":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "specified"
	}
}

func normalizeNoteImportTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func rawImportIdentifier(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	var object struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &object) == nil {
		return strings.TrimSpace(object.ID)
	}
	return "present"
}

func activityPubVisibility(to, cc json.RawMessage) string {
	values := append(rawStringList(to), rawStringList(cc)...)
	for _, value := range values {
		if strings.Contains(value, "#Public") || strings.Contains(value, "/Public") {
			return "public"
		}
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), "/followers") {
			return "followers"
		}
	}
	return "specified"
}

func rawStringList(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return []string{single}
	}
	return nil
}

func plainTextFromHTML(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	tokenizer := html.NewTokenizer(strings.NewReader(value))
	var result strings.Builder
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return collapseNoteImportWhitespace(result.String())
			}
			return collapseNoteImportWhitespace(result.String())
		case html.TextToken:
			result.WriteString(string(tokenizer.Text()))
		case html.StartTagToken, html.EndTagToken:
			name, _ := tokenizer.TagName()
			switch string(name) {
			case "br", "p", "div", "li":
				result.WriteByte('\n')
			}
		}
	}
}

func collapseNoteImportWhitespace(value string) string {
	value = strings.TrimSpace(value)
	for strings.Contains(value, "\n\n") {
		value = strings.ReplaceAll(value, "\n\n", "\n")
	}
	return value
}
