package http

import (
	"context"
	stdhttp "net/http"
	"sort"
	"strconv"
	"strings"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/notificationpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const misskeyNotificationDefaultLimit int32 = 10

type misskeyNotificationsRequest struct {
	Limit        *int32    `json:"limit"`
	SinceID      jsonInt64 `json:"sinceId"`
	UntilID      jsonInt64 `json:"untilId"`
	MarkAsRead   *bool     `json:"markAsRead"`
	IncludeTypes *[]string `json:"includeTypes"`
	ExcludeTypes *[]string `json:"excludeTypes"`
}

type misskeyNotificationBase struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Type      string `json:"type"`
}

type misskeyActorNotification struct {
	misskeyNotificationBase
	User   misskeyUserLite `json:"user"`
	UserID string          `json:"userId"`
}

type misskeyNoteNotification struct {
	misskeyActorNotification
	Note misskeyConversationNote `json:"note"`
}

type misskeyReactionNotification struct {
	misskeyNoteNotification
	Reaction string `json:"reaction"`
}

type misskeyFollowRequestAcceptedNotification struct {
	misskeyActorNotification
	Message *string `json:"message"`
}

type misskeyExportCompletedNotification struct {
	misskeyNotificationBase
	ExportedEntity string `json:"exportedEntity"`
	FileID         string `json:"fileId"`
}

type misskeyAppNotification struct {
	misskeyNotificationBase
	Body   string  `json:"body"`
	Header *string `json:"header"`
	Icon   *string `json:"icon"`
}

type misskeyGroupedReactionNotification struct {
	ID        string                                  `json:"id"`
	CreatedAt string                                  `json:"createdAt"`
	Type      string                                  `json:"type"`
	Note      misskeyConversationNote                 `json:"note"`
	Reactions []misskeyGroupedReactionNotificationRef `json:"reactions"`
}

type misskeyGroupedReactionNotificationRef struct {
	User     misskeyUserLite `json:"user"`
	Reaction string          `json:"reaction"`
}

type misskeyGroupedFollowNotification struct {
	ID        string            `json:"id"`
	CreatedAt string            `json:"createdAt"`
	Type      string            `json:"type"`
	Users     []misskeyUserLite `json:"users"`
}

type misskeyNotificationView struct {
	id        int64
	createdAt string
	typeName  string
	output    any
	actor     *misskeyUserLite
	note      *misskeyConversationNote
	reaction  string
}

var misskeyNotificationRequestTypes = map[string]struct{}{
	"note": {}, "follow": {}, "mention": {}, "reply": {}, "renote": {}, "quote": {}, "reaction": {}, "pollEnded": {}, "edited": {},
	"receiveFollowRequest": {}, "followRequestAccepted": {}, "roleAssigned": {}, "chatRoomInvitationReceived": {}, "achievementEarned": {},
	"exportCompleted": {}, "importCompleted": {}, "login": {}, "createToken": {}, "scheduledNoteFailed": {}, "scheduledNotePosted": {},
	"app": {}, "test": {}, "sharedAccessGranted": {}, "sharedAccessRevoked": {}, "sharedAccessLogin": {}, "pollVote": {}, "groupInvited": {},
}

var misskeyNotificationInternalTypes = map[string][]string{
	"note":                  {"comment"},
	"follow":                {"follow"},
	"reply":                 {"reply", "qa_answer_accepted"},
	"reaction":              {"like", "favorite"},
	"receiveFollowRequest":  {"follow_request_received"},
	"followRequestAccepted": {"follow_request_accepted"},
	"exportCompleted":       {"export_completed"},
	"app": {
		"system", "mall_refund_approved", "mall_refund_rejected", "mall_digital_entitlement_revoked", "mall_order_paid",
		"mall_order_shipped", "mall_order_completed", "mall_review_published", "mall_review_hidden",
	},
}

func (h *Handler) listNotificationsCompat(c *gin.Context) {
	h.listMisskeyNotifications(c, false)
}

func (h *Handler) listGroupedNotificationsCompat(c *gin.Context) {
	h.listMisskeyNotifications(c, true)
}

func (h *Handler) listMisskeyNotifications(c *gin.Context, grouped bool) {
	var req misskeyNotificationsRequest
	if !bindJSON(c, &req) {
		return
	}
	limit := misskeyNotificationDefaultLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > 100 || req.SinceID.Int64() < 0 || req.UntilID.Int64() < 0 {
		writeError(c, stdhttp.StatusBadRequest, "limit must be between 1 and 100 and cursors must be non-negative", "invalid_argument")
		return
	}
	includeTypes, includeSet, excludeTypes, excludeSet, valid := misskeyNotificationFilters(req.IncludeTypes, req.ExcludeTypes)
	if !valid {
		writeError(c, stdhttp.StatusBadRequest, "invalid notification type filter", "invalid_argument")
		return
	}
	limiter := h.notificationRateLimits.List
	action := notificationRateLimitList
	if grouped {
		limiter = h.notificationRateLimits.Grouped
		action = notificationRateLimitGrouped
	}
	if !h.allowNotificationRateLimit(c, limiter, action) {
		return
	}
	if includeSet && len(includeTypes) == 0 {
		c.JSON(stdhttp.StatusOK, []any{})
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.ListNotificationsCompat(ctx, &notificationpb.ListNotificationsCompatRequest{
		UserId:          currentUserID(c),
		Limit:           limit,
		SinceId:         req.SinceID.Int64(),
		UntilId:         req.UntilID.Int64(),
		IncludeTypes:    includeTypes,
		ExcludeTypes:    excludeTypes,
		IncludeTypesSet: includeSet,
		ExcludeTypesSet: excludeSet,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if grouped && len(resp.GetItems()) == 0 {
		c.JSON(stdhttp.StatusOK, []any{})
		return
	}
	resolver := newMisskeyNotificationResolver(h, ctx)
	items, err := resolver.views(resp.GetItems())
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if req.MarkAsRead == nil || *req.MarkAsRead {
		// The reference endpoint treats this as best-effort; a completed read is
		// still useful when the asynchronous read marker is temporarily unavailable.
		_, _ = h.clients.Notification.MarkAllRead(ctx, &notificationpb.MarkAllReadRequest{UserId: currentUserID(c)})
	}
	if grouped {
		c.JSON(stdhttp.StatusOK, groupMisskeyNotifications(items, req.SinceID.Int64() > 0 && req.UntilID.Int64() == 0))
		return
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item.output)
	}
	c.JSON(stdhttp.StatusOK, out)
}

func misskeyNotificationFilters(include, exclude *[]string) ([]string, bool, []string, bool, bool) {
	if include != nil {
		values, valid := translateMisskeyNotificationTypes(*include)
		return values, true, nil, false, valid
	}
	if exclude != nil {
		values, valid := translateMisskeyNotificationTypes(*exclude)
		return nil, false, values, true, valid
	}
	return nil, false, nil, false, true
}

func translateMisskeyNotificationTypes(values []string) ([]string, bool) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, valid := misskeyNotificationRequestTypes[value]; !valid {
			return nil, false
		}
		for _, internalType := range misskeyNotificationInternalTypes[value] {
			if _, exists := seen[internalType]; exists {
				continue
			}
			seen[internalType] = struct{}{}
			result = append(result, internalType)
		}
	}
	return result, true
}

type misskeyNotificationResolver struct {
	h            *Handler
	ctx          context.Context
	users        map[int64]*userpb.UserInfo
	contentNotes map[string]*misskeyConversationNote
	commentNotes map[int64]*misskeyConversationNote
}

func newMisskeyNotificationResolver(h *Handler, ctx context.Context) *misskeyNotificationResolver {
	return &misskeyNotificationResolver{
		h: h, ctx: ctx, users: make(map[int64]*userpb.UserInfo), contentNotes: make(map[string]*misskeyConversationNote), commentNotes: make(map[int64]*misskeyConversationNote),
	}
}

func (r *misskeyNotificationResolver) views(items []*notificationpb.Notification) ([]misskeyNotificationView, error) {
	result := make([]misskeyNotificationView, 0, len(items))
	for _, item := range items {
		view, include, err := r.view(item)
		if err != nil {
			return nil, err
		}
		if include {
			result = append(result, view)
		}
	}
	return result, nil
}

func (r *misskeyNotificationResolver) view(item *notificationpb.Notification) (misskeyNotificationView, bool, error) {
	if item == nil || item.GetId() <= 0 {
		return misskeyNotificationView{}, false, nil
	}
	base := misskeyNotificationBase{ID: strconv.FormatInt(item.GetId(), 10), CreatedAt: formatUnixMilli(item.GetCreatedAt())}
	actorNotification := func(notificationType string) (misskeyNotificationView, bool, error) {
		user, found, err := r.user(item.GetActorId())
		if err != nil || !found {
			return misskeyNotificationView{}, false, err
		}
		actor := toMisskeyUserLite(user)
		base.Type = notificationType
		output := misskeyActorNotification{misskeyNotificationBase: base, User: actor, UserID: strconv.FormatInt(item.GetActorId(), 10)}
		return misskeyNotificationView{id: item.GetId(), createdAt: base.CreatedAt, typeName: notificationType, output: output, actor: &actor}, true, nil
	}
	noteNotification := func(notificationType string) (misskeyNotificationView, bool, error) {
		note, found, err := r.commentNote(item.GetSourceId())
		if err != nil || !found {
			return misskeyNotificationView{}, false, err
		}
		user, found, err := r.user(item.GetActorId())
		if err != nil || !found {
			return misskeyNotificationView{}, false, err
		}
		actor := toMisskeyUserLite(user)
		base.Type = notificationType
		output := misskeyNoteNotification{
			misskeyActorNotification: misskeyActorNotification{misskeyNotificationBase: base, User: actor, UserID: strconv.FormatInt(item.GetActorId(), 10)},
			Note:                     *note,
		}
		return misskeyNotificationView{id: item.GetId(), createdAt: base.CreatedAt, typeName: notificationType, output: output, actor: &actor, note: note}, true, nil
	}
	switch item.GetType() {
	case "follow":
		return actorNotification("follow")
	case "follow_request_received":
		return actorNotification("receiveFollowRequest")
	case "follow_request_accepted":
		user, found, err := r.user(item.GetActorId())
		if err != nil || !found {
			return misskeyNotificationView{}, false, err
		}
		actor := toMisskeyUserLite(user)
		base.Type = "followRequestAccepted"
		return misskeyNotificationView{
			id: item.GetId(), createdAt: base.CreatedAt, typeName: base.Type, actor: &actor,
			output: misskeyFollowRequestAcceptedNotification{
				misskeyActorNotification: misskeyActorNotification{misskeyNotificationBase: base, User: actor, UserID: strconv.FormatInt(item.GetActorId(), 10)},
				Message:                  optionalMisskeyText(item.GetContent()),
			},
		}, true, nil
	case "comment":
		return noteNotification("note")
	case "reply", "qa_answer_accepted":
		return noteNotification("reply")
	case "like", "favorite":
		note, found, err := r.contentNote(item.GetEntityType(), item.GetEntityId())
		if err != nil || !found {
			return misskeyNotificationView{}, false, err
		}
		user, found, err := r.user(item.GetActorId())
		if err != nil || !found {
			return misskeyNotificationView{}, false, err
		}
		actor := toMisskeyUserLite(user)
		reaction := ":thumbsup:"
		if item.GetType() == "favorite" {
			reaction = ":star:"
		}
		base.Type = "reaction"
		output := misskeyReactionNotification{
			misskeyNoteNotification: misskeyNoteNotification{
				misskeyActorNotification: misskeyActorNotification{misskeyNotificationBase: base, User: actor, UserID: strconv.FormatInt(item.GetActorId(), 10)},
				Note:                     *note,
			},
			Reaction: reaction,
		}
		return misskeyNotificationView{id: item.GetId(), createdAt: base.CreatedAt, typeName: base.Type, output: output, actor: &actor, note: note, reaction: reaction}, true, nil
	case "export_completed":
		if item.GetEntityType() != "file" || item.GetEntityId() <= 0 {
			return misskeyNotificationView{}, false, nil
		}
		base.Type = "exportCompleted"
		return misskeyNotificationView{
			id: item.GetId(), createdAt: base.CreatedAt, typeName: base.Type,
			output: misskeyExportCompletedNotification{misskeyNotificationBase: base, ExportedEntity: exportedEntityFromNotificationTitle(item.GetTitle()), FileID: strconv.FormatInt(item.GetEntityId(), 10)},
		}, true, nil
	default:
		base.Type = "app"
		return misskeyNotificationView{
			id: item.GetId(), createdAt: base.CreatedAt, typeName: base.Type,
			output: misskeyAppNotification{misskeyNotificationBase: base, Body: item.GetContent(), Header: optionalMisskeyText(item.GetTitle()), Icon: nil},
		}, true, nil
	}
}

func (r *misskeyNotificationResolver) user(id int64) (*userpb.UserInfo, bool, error) {
	if id <= 0 {
		return nil, false, nil
	}
	if user, exists := r.users[id]; exists {
		return user, user != nil, nil
	}
	resp, err := r.h.clients.User.GetUser(r.ctx, &userpb.UserIDRequest{Id: id})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			r.users[id] = nil
			return nil, false, nil
		}
		return nil, false, err
	}
	user := resp.GetUser()
	r.users[id] = user
	return user, user != nil, nil
}

func (r *misskeyNotificationResolver) contentNote(entityType string, id int64) (*misskeyConversationNote, bool, error) {
	if id <= 0 || (entityType != "article" && entityType != "topic") {
		return nil, false, nil
	}
	key := entityType + ":" + strconv.FormatInt(id, 10)
	if note, exists := r.contentNotes[key]; exists {
		return note, note != nil, nil
	}
	var text, title string
	var authorID, createdAt int64
	var tags []string
	if entityType == "topic" {
		resp, err := r.h.clients.Content.GetTopic(r.ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				r.contentNotes[key] = nil
				return nil, false, nil
			}
			return nil, false, err
		}
		topic := resp.GetTopic()
		if topic == nil || topic.GetStatus() != contentStatusPublished {
			r.contentNotes[key] = nil
			return nil, false, nil
		}
		text, title, authorID, createdAt, tags = topic.GetBody(), topic.GetTitle(), topic.GetAuthorId(), topic.GetCreatedAt(), topic.GetTags()
	} else {
		resp, err := r.h.clients.Content.GetArticle(r.ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				r.contentNotes[key] = nil
				return nil, false, nil
			}
			return nil, false, err
		}
		article := resp.GetArticle()
		if article == nil || article.GetStatus() != contentStatusPublished {
			r.contentNotes[key] = nil
			return nil, false, nil
		}
		text, title, authorID, createdAt, tags = article.GetBody(), article.GetTitle(), article.GetAuthorId(), article.GetCreatedAt(), article.GetTags()
	}
	user, found, err := r.user(authorID)
	if err != nil || !found {
		return nil, false, err
	}
	if strings.TrimSpace(text) == "" {
		text = title
	}
	value := strconv.FormatInt(id, 10)
	note := &misskeyConversationNote{
		ID: value, ThreadID: value, CreatedAt: formatUnixMilli(createdAt), Text: text, UserID: strconv.FormatInt(authorID, 10), User: toMisskeyUserLite(user), Visibility: "public",
		Mentions: []string{}, VisibleUserIDs: []string{}, FileIDs: []string{}, Files: []any{}, Tags: append([]string(nil), tags...), Emojis: map[string]string{}, ReactionEmojis: map[string]string{}, Reactions: map[string]int64{},
	}
	r.contentNotes[key] = note
	return note, true, nil
}

func (r *misskeyNotificationResolver) commentNote(id int64) (*misskeyConversationNote, bool, error) {
	if id <= 0 {
		return nil, false, nil
	}
	if note, exists := r.commentNotes[id]; exists {
		return note, note != nil, nil
	}
	resp, err := r.h.clients.Comment.GetComment(r.ctx, &commentpb.GetCommentRequest{Id: id})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			r.commentNotes[id] = nil
			return nil, false, nil
		}
		return nil, false, err
	}
	comment := resp.GetComment()
	if comment == nil || comment.GetStatus() != 1 {
		r.commentNotes[id] = nil
		return nil, false, nil
	}
	user, found, err := r.user(comment.GetAuthorId())
	if err != nil || !found {
		return nil, false, err
	}
	note := misskeyConversationNoteFromProto(comment, user)
	r.commentNotes[id] = &note
	return &note, true, nil
}

func exportedEntityFromNotificationTitle(title string) string {
	switch strings.TrimSpace(title) {
	case "天线导出完成":
		return "antenna"
	case "屏蔽列表导出完成":
		return "blocking"
	case "Clip 导出完成":
		return "clip"
	case "收藏导出完成":
		return "favorite"
	case "关注列表导出完成":
		return "following"
	case "静音列表导出完成":
		return "muting"
	case "内容导出完成":
		return "note"
	case "用户列表导出完成":
		return "userList"
	default:
		return "data"
	}
}

func groupMisskeyNotifications(items []misskeyNotificationView, ascending bool) []any {
	grouped := make([]misskeyNotificationView, 0, len(items))
	reactionIndexByNoteID := make(map[string]int)
	for _, item := range items {
		if item.typeName == "reaction" && item.note != nil && item.actor != nil {
			index, exists := reactionIndexByNoteID[item.note.ID]
			if !exists {
				reactionIndexByNoteID[item.note.ID] = len(grouped)
				grouped = append(grouped, item)
				continue
			}
			previous := &grouped[index]
			switch value := previous.output.(type) {
			case misskeyReactionNotification:
				previous.output = misskeyGroupedReactionNotification{
					ID: strconv.FormatInt(item.id, 10), CreatedAt: value.CreatedAt, Type: "reaction:grouped", Note: value.Note,
					Reactions: []misskeyGroupedReactionNotificationRef{{User: value.User, Reaction: value.Reaction}, {User: *item.actor, Reaction: item.reaction}},
				}
			case misskeyGroupedReactionNotification:
				value.ID = strconv.FormatInt(item.id, 10)
				value.Reactions = append(value.Reactions, misskeyGroupedReactionNotificationRef{User: *item.actor, Reaction: item.reaction})
				previous.output = value
			}
			previous.id = item.id
			previous.typeName = "reaction:grouped"
			continue
		}
		if item.typeName == "follow" && item.actor != nil && len(grouped) > 0 {
			previous := &grouped[len(grouped)-1]
			if previous.typeName == "follow" || previous.typeName == "follow:grouped" {
				switch value := previous.output.(type) {
				case misskeyActorNotification:
					previous.output = misskeyGroupedFollowNotification{ID: strconv.FormatInt(item.id, 10), CreatedAt: value.CreatedAt, Type: "follow:grouped", Users: []misskeyUserLite{value.User, *item.actor}}
				case misskeyGroupedFollowNotification:
					value.ID = strconv.FormatInt(item.id, 10)
					value.Users = append(value.Users, *item.actor)
					previous.output = value
				}
				previous.id = item.id
				previous.typeName = "follow:grouped"
				continue
			}
		}
		grouped = append(grouped, item)
	}
	sort.SliceStable(grouped, func(i, j int) bool {
		if ascending {
			return grouped[i].id < grouped[j].id
		}
		return grouped[i].id > grouped[j].id
	})
	out := make([]any, 0, len(grouped))
	for _, item := range grouped {
		out = append(out, item.output)
	}
	return out
}
