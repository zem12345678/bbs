package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/creditpb"
	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/mallpb"
	"api-gateway/api/proto/notificationpb"
	"api-gateway/api/proto/reactionpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/internal/clients"
	iochttp "api-gateway/internal/ioc/http"
	"api-gateway/internal/popularity"
	realtimechat "api-gateway/internal/realtime/chat"
	"api-gateway/internal/storage"
	"api-gateway/pkg/exception"
	"api-gateway/pkg/http/response"
	"api-gateway/pkg/ratelimit"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const requestTimeout = 10 * time.Second
const (
	maxUploadedImageSize         = int64(5 << 20)
	imageTransferTimeout         = 30 * time.Second
	userStatusActive       int32 = 1
	userStatusMuted        int32 = 2
	contentStatusPublished int32 = 2
	categoryStatusEnabled  int32 = 2
	maxExactIntegerFloat64       = 1<<53 - 1
)

const digitalEntitlementStatusActive = "ACTIVE"
const digitalEntitlementGrantTypeMembership = "membership"
const digitalEntitlementLookupLimit int32 = 20
const digitalEntitlementBatchUserLookupLimit = 100
const adminDigitalEntitlementOrderIDFilterLimit = 100
const publicUserBatchLookupLimit = 100
const (
	taskKeyDailyCheckIn = "daily_check_in"
	taskKeyFirstTopic   = "first_topic"
	taskKeyFirstComment = "first_comment"
)
const membershipBountyRequiredMessage = "membership entitlement required for bounty QA topics"
const paidAttachmentMembershipRequiredMessage = "membership entitlement required for paid attachments"
const profileBackgroundMembershipRequiredMessage = "profile background membership entitlement required"
const (
	profileThemeDefault = "default"
	profileThemePro     = "theme-pro"
)

type Handler struct {
	clients                            *clients.Clients
	tokenHeader                        string
	tokenPrefix                        string
	jwtSecret                          []byte
	publicBaseURL                      string
	attachments                        storage.ObjectStore
	objectCleanup                      uploadedObjectCleaner
	chatRealtime                       *realtimechat.Service
	chatTicketLimit                    ratelimit.Limiter
	chatTicketRetryAfterSeconds        int
	chatCreateRoomLimit                ratelimit.Limiter
	chatJoinLimit                      ratelimit.Limiter
	chatSendLimit                      ratelimit.Limiter
	chatReadLimit                      ratelimit.Limiter
	authRateLimits                     AuthRateLimits
	searchRateLimits                   SearchRateLimits
	fileUploadLimit                    ratelimit.Limiter
	antennaImportLimit                 ratelimit.Limiter
	blockingImportLimit                ratelimit.Limiter
	mutingImportLimit                  ratelimit.Limiter
	followingImportLimit               ratelimit.Limiter
	userListImportLimit                ratelimit.Limiter
	noteImportLimit                    ratelimit.Limiter
	accountDataExportGate              ExportGate
	clipExportGate                     ExportGate
	favoriteExportGate                 ExportGate
	antennaExportGate                  ExportGate
	blockingExportGate                 ExportGate
	followingExportGate                ExportGate
	muteExportGate                     ExportGate
	noteExportGate                     ExportGate
	userListExportGate                 ExportGate
	tokenRevocations                   TokenRevocationStore
	credentialVersions                 CredentialVersionStore
	popularity                         popularityStore
	requireCredentialVersionValidation bool
}

type uploadedObjectCleaner interface {
	Delete(context.Context, string) error
}

type popularityStore interface {
	RecordChatRoomActivity(context.Context, string) error
	ListChatRooms(context.Context, int) ([]popularity.Entry, error)
	RecordResourceVisit(context.Context, int64) error
	ListResources(context.Context, int) ([]popularity.Entry, error)
}

type taskView struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	RewardPoints int64  `json:"reward_points"`
	Status       int32  `json:"status"`
	Sort         int32  `json:"sort"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Cycle        string `json:"cycle,omitempty"`
	Completed    bool   `json:"completed"`
	Claimed      bool   `json:"claimed"`
	Claimable    bool   `json:"claimable"`
}

type creditLeaderboardUserView struct {
	ID            int64  `json:"id"`
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	AvatarURL     string `json:"avatar_url"`
	BackgroundURL string `json:"background_url,omitempty"`
	ProfileTheme  string `json:"profile_theme"`
}

type creditLeaderboardView struct {
	Rank   int32                     `json:"rank"`
	UserID int64                     `json:"user_id"`
	Total  int64                     `json:"total"`
	User   creditLeaderboardUserView `json:"user"`
}

// publicUserView is the allowlist for anonymous user-profile responses.
// Keep account, authentication, and moderation fields in the authenticated
// user-service response only.
type publicUserView struct {
	ID                     int64  `json:"id,omitempty"`
	Username               string `json:"username,omitempty"`
	Nickname               string `json:"nickname,omitempty"`
	AvatarURL              string `json:"avatar_url,omitempty"`
	Bio                    string `json:"bio,omitempty"`
	FollowerCount          int64  `json:"follower_count,omitempty"`
	FollowingCount         int64  `json:"following_count,omitempty"`
	BackgroundURL          string `json:"background_url,omitempty"`
	ProfileTheme           string `json:"profile_theme,omitempty"`
	FollowApprovalRequired bool   `json:"follow_approval_required,omitempty"`
}

type publicUserResponse struct {
	Success bool            `json:"success,omitempty"`
	Message string          `json:"message,omitempty"`
	User    *publicUserView `json:"user,omitempty"`
}

type publicUserListResponse struct {
	Items []*publicUserView `json:"items,omitempty"`
	Total int64             `json:"total,omitempty"`
}

func toPublicUserView(user *userpb.UserInfo) *publicUserView {
	if user == nil {
		return nil
	}
	if state := strings.ToLower(strings.TrimSpace(user.GetAccountState())); state == "deletion_pending" || state == "anonymized" {
		return &publicUserView{
			ID:           user.GetId(),
			Username:     "deleted",
			Nickname:     "已注销用户",
			ProfileTheme: profileThemeDefault,
		}
	}
	return &publicUserView{
		ID:                     user.GetId(),
		Username:               user.GetUsername(),
		Nickname:               user.GetNickname(),
		AvatarURL:              user.GetAvatarUrl(),
		Bio:                    user.GetBio(),
		FollowerCount:          user.GetFollowerCount(),
		FollowingCount:         user.GetFollowingCount(),
		BackgroundURL:          user.GetBackgroundUrl(),
		ProfileTheme:           user.GetProfileTheme(),
		FollowApprovalRequired: user.GetFollowApprovalRequired(),
	}
}

func toPublicUserResponse(resp *userpb.UserResponse) publicUserResponse {
	return publicUserResponse{
		Success: resp.GetSuccess(),
		Message: resp.GetMessage(),
		User:    toPublicUserView(resp.GetUser()),
	}
}

func toPublicUserListResponse(resp *userpb.UserListResponse) publicUserListResponse {
	items := resp.GetItems()
	publicItems := make([]*publicUserView, len(items))
	for index, user := range items {
		publicItems[index] = toPublicUserView(user)
	}
	return publicUserListResponse{Items: publicItems, Total: resp.GetTotal()}
}

func NewHandler(clients *clients.Clients, tokenHeader string, tokenPrefix string, jwtSecret string) *Handler {
	return NewHandlerWithAttachmentStore(clients, tokenHeader, tokenPrefix, jwtSecret, nil)
}

func NewHandlerWithTokenRevocationStore(
	clients *clients.Clients,
	tokenHeader string,
	tokenPrefix string,
	jwtSecret string,
	tokenRevocations TokenRevocationStore,
) *Handler {
	return NewHandlerWithRealtimeAndRateLimitsAndTokenRevocationStore(
		clients, tokenHeader, tokenPrefix, jwtSecret, nil, nil, nil, nil, tokenRevocations,
	)
}

// NewHandlerWithCredentialVersionStore enables the production credential-version
// check without requiring unrelated realtime dependencies in focused callers.
func NewHandlerWithCredentialVersionStore(
	clients *clients.Clients,
	tokenHeader string,
	tokenPrefix string,
	jwtSecret string,
	credentialVersions CredentialVersionStore,
) *Handler {
	return NewHandlerWithRealtimeAndRateLimitsAndTokenSecurityStores(
		clients, tokenHeader, tokenPrefix, jwtSecret, nil, nil, nil, nil, nil, credentialVersions,
	)
}

func NewHandlerWithAttachmentStore(clients *clients.Clients, tokenHeader string, tokenPrefix string, jwtSecret string, attachments storage.ObjectStore) *Handler {
	return NewHandlerWithRealtime(clients, tokenHeader, tokenPrefix, jwtSecret, attachments, nil)
}

func NewHandlerWithRealtime(clients *clients.Clients, tokenHeader string, tokenPrefix string, jwtSecret string, attachments storage.ObjectStore, realtime *realtimechat.Service) *Handler {
	return NewHandlerWithRealtimeAndRateLimits(clients, tokenHeader, tokenPrefix, jwtSecret, attachments, realtime, nil, nil)
}

// SetChatTicketLimit configures the shared limiter used before issuing a
// single-use chat WebSocket ticket. It is a setter to keep focused HTTP tests
// and callers that do not provision realtime dependencies lightweight.
func (h *Handler) SetChatTicketLimit(limiter ratelimit.Limiter) {
	h.chatTicketLimit = limiter
}

// SetPublicBaseURL configures the externally reachable HTTPS/HTTP origin used
// in absolute API URLs. Production supplies this from trusted deployment
// configuration rather than request-controlled forwarding headers.
func (h *Handler) SetPublicBaseURL(baseURL string) {
	h.publicBaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

// SetUploadedObjectCleaner configures durable compensation for objects whose
// metadata creation was rejected deterministically. Focused HTTP tests may
// omit it; production wires the Redis-backed cleanup queue.
func (h *Handler) SetUploadedObjectCleaner(cleaner uploadedObjectCleaner) {
	h.objectCleanup = cleaner
}

// SetChatTicketRetryAfter configures a conservative Retry-After value for a
// saturated ticket window. Waiting the full interval avoids immediately
// retrying a still-full distributed sliding window.
func (h *Handler) SetChatTicketRetryAfter(interval time.Duration) {
	if interval <= 0 {
		h.chatTicketRetryAfterSeconds = 0
		return
	}
	h.chatTicketRetryAfterSeconds = max(1, int(math.Ceil(interval.Seconds())))
}

// SetChatReadLimit configures the shared limiter used before accepting a chat
// read-position update. It intentionally shares its Redis key with the
// WebSocket command path, so clients cannot bypass the limit by switching
// transports.
func (h *Handler) SetChatReadLimit(limiter ratelimit.Limiter) {
	h.chatReadLimit = limiter
}

// SetChatCreateRoomLimit configures the user-scoped limiter for new chat
// rooms. Room creation is a durable, higher-cost operation and deliberately
// has a separate budget from joining existing rooms.
func (h *Handler) SetChatCreateRoomLimit(limiter ratelimit.Limiter) {
	h.chatCreateRoomLimit = limiter
}

func (h *Handler) SetPopularityStore(store popularityStore) {
	h.popularity = store
}

func (h *Handler) SetClipExportGate(gate ClipExportGate) {
	h.clipExportGate = gate
}

func (h *Handler) SetFavoriteExportGate(gate ExportGate) {
	h.favoriteExportGate = gate
}

func (h *Handler) SetAntennaExportGate(gate ExportGate) {
	h.antennaExportGate = gate
}

func (h *Handler) SetAccountDataExportGate(gate ExportGate) {
	h.accountDataExportGate = gate
}

func (h *Handler) SetBlockingExportGate(gate ExportGate) {
	h.blockingExportGate = gate
}

func (h *Handler) SetFollowingExportGate(gate ExportGate) {
	h.followingExportGate = gate
}

func (h *Handler) SetMuteExportGate(gate ExportGate) {
	h.muteExportGate = gate
}

func (h *Handler) SetNoteExportGate(gate ExportGate) {
	h.noteExportGate = gate
}

func (h *Handler) SetUserListExportGate(gate ExportGate) {
	h.userListExportGate = gate
}

func NewHandlerWithRealtimeAndRateLimits(
	clients *clients.Clients,
	tokenHeader string,
	tokenPrefix string,
	jwtSecret string,
	attachments storage.ObjectStore,
	realtime *realtimechat.Service,
	chatJoinLimit ratelimit.Limiter,
	chatSendLimit ratelimit.Limiter,
) *Handler {
	return NewHandlerWithRealtimeAndRateLimitsAndTokenRevocationStore(
		clients, tokenHeader, tokenPrefix, jwtSecret, attachments, realtime, chatJoinLimit, chatSendLimit, nil,
	)
}

func NewHandlerWithRealtimeAndRateLimitsAndTokenRevocationStore(
	clients *clients.Clients,
	tokenHeader string,
	tokenPrefix string,
	jwtSecret string,
	attachments storage.ObjectStore,
	realtime *realtimechat.Service,
	chatJoinLimit ratelimit.Limiter,
	chatSendLimit ratelimit.Limiter,
	tokenRevocations TokenRevocationStore,
) *Handler {
	return newHandler(
		clients, tokenHeader, tokenPrefix, jwtSecret, attachments, realtime, chatJoinLimit, chatSendLimit,
		tokenRevocations, nil, false,
	)
}

// NewHandlerWithRealtimeAndRateLimitsAndTokenSecurityStores is the production
// constructor: it requires both per-token logout revocation and per-user
// credential-version validation backed by the shared Redis client.
func NewHandlerWithRealtimeAndRateLimitsAndTokenSecurityStores(
	clients *clients.Clients,
	tokenHeader string,
	tokenPrefix string,
	jwtSecret string,
	attachments storage.ObjectStore,
	realtime *realtimechat.Service,
	chatJoinLimit ratelimit.Limiter,
	chatSendLimit ratelimit.Limiter,
	tokenRevocations TokenRevocationStore,
	credentialVersions CredentialVersionStore,
) *Handler {
	return newHandler(
		clients, tokenHeader, tokenPrefix, jwtSecret, attachments, realtime, chatJoinLimit, chatSendLimit,
		tokenRevocations, credentialVersions, true,
	)
}

func newHandler(
	clients *clients.Clients,
	tokenHeader string,
	tokenPrefix string,
	jwtSecret string,
	attachments storage.ObjectStore,
	realtime *realtimechat.Service,
	chatJoinLimit ratelimit.Limiter,
	chatSendLimit ratelimit.Limiter,
	tokenRevocations TokenRevocationStore,
	credentialVersions CredentialVersionStore,
	requireCredentialVersionValidation bool,
) *Handler {
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	if tokenPrefix == "" {
		tokenPrefix = "Bearer"
	}
	handler := &Handler{
		clients: clients, tokenHeader: tokenHeader, tokenPrefix: tokenPrefix,
		jwtSecret: []byte(jwtSecret), attachments: attachments, chatRealtime: realtime,
		chatJoinLimit: chatJoinLimit, chatSendLimit: chatSendLimit, tokenRevocations: tokenRevocations,
		credentialVersions: credentialVersions, requireCredentialVersionValidation: requireCredentialVersionValidation,
	}
	if realtime != nil {
		realtime.SetSessionValidator(handler)
	}
	return handler
}

func NewInitControllers(h *Handler) iochttp.InitControllers {
	return func(r *gin.Engine) {
		r.GET("/healthz", h.health)
		r.GET("/robots.txt", h.robots)
		r.GET("/sitemap.xml", h.sitemapIndex)
		r.GET("/sitemaps/:name", h.sitemapPage)
		r.GET("/uploads/:kind/:name", h.serveUploadedImage)
		r.GET("/charts/users", h.userChart)
		r.POST("/charts/users", h.userChart)
		r.GET("/charts/user/following", h.userFollowingChart)
		r.POST("/charts/user/following", h.userFollowingChart)
		r.GET("/charts/notes", h.notesChart)
		r.POST("/charts/notes", h.notesChart)
		r.GET("/charts/user/notes", h.userNotesChart)
		r.POST("/charts/user/notes", h.userNotesChart)
		r.GET("/charts/active-users", h.requireAdminAuth(), h.requireAdminPermission("governance:list_users"), h.activeUsersChart)
		r.POST("/charts/active-users", h.requireAdminAuth(), h.requireAdminPermission("governance:list_users"), h.activeUsersChart)
		r.GET("/charts/drive", h.driveChart)
		r.POST("/charts/drive", h.driveChart)
		r.GET("/charts/user/drive", h.userDriveChart)
		r.POST("/charts/user/drive", h.userDriveChart)
		r.POST("/api/tokens/create", h.requireAuth(), h.requireInteractiveAuth(), h.createAPIToken)
		r.POST("/api/tokens/list", h.requireAuthScope("read"), h.listAPITokens)
		r.POST("/api/tokens/revoke", h.requireAuth(), h.requireInteractiveAuth(), h.revokeAPIToken)
		for _, prefix := range []string{"/api/i", "/i"} {
			compatibility := r.Group(prefix)
			compatibility.POST("/webhooks/list", h.requireAuthScope("read"), h.listWebhooks)
			compatibility.POST("/webhooks/create", h.requireAuthScope("write"), h.createWebhook)
			compatibility.POST("/webhooks/show", h.requireAuthScope("read"), h.showWebhook)
			compatibility.POST("/webhooks/update", h.requireAuthScope("write"), h.updateWebhook)
			compatibility.POST("/webhooks/delete", h.requireAuthScope("write"), h.deleteWebhook)
			compatibility.POST("/webhooks/test", h.requireAuthScope("read"), h.testWebhook)
			compatibility.POST("/antennas/create", h.requireAuthScope("write"), h.createAntenna)
			compatibility.POST("/antennas/delete", h.requireAuthScope("write"), h.deleteAntenna)
			compatibility.POST("/antennas/list", h.requireAuthScope("read"), h.listAntennas)
			compatibility.POST("/antennas/notes", h.requireAuthScope("read"), h.antennaNotes)
			compatibility.POST("/antennas/show", h.requireAuthScope("read"), h.showAntenna)
			compatibility.POST("/antennas/update", h.requireAuthScope("write"), h.updateAntenna)
		}
		for _, prefix := range []string{"/api", ""} {
			compatibility := r.Group(prefix + "/antennas")
			compatibility.POST("/create", h.requireAuthScope("write"), h.createAntenna)
			compatibility.POST("/delete", h.requireAuthScope("write"), h.deleteAntenna)
			compatibility.POST("/list", h.requireAuthScope("read"), h.listAntennas)
			compatibility.POST("/notes", h.requireAuthScope("read"), h.antennaNotes)
			compatibility.POST("/show", h.requireAuthScope("read"), h.showAntenna)
			compatibility.POST("/update", h.requireAuthScope("write"), h.updateAntenna)
		}
		for _, prefix := range []string{"/api", ""} {
			r.POST(prefix+"/following/invalidate", h.requireAuthScope("write"), h.invalidateFollowingCompat)
		}
		for _, prefix := range []string{"/api", ""} {
			r.POST(prefix+"/notes/conversation", h.notesConversationCompat)
			clips := r.Group(prefix + "/clips")
			clips.POST("/create", h.requireAuthScope("write"), h.createClip)
			clips.POST("/update", h.requireAuthScope("write"), h.updateClip)
			clips.POST("/delete", h.requireAuthScope("write"), h.deleteClip)
			clips.POST("/list", h.requireAuthScope("read"), h.listClips)
			clips.POST("/show", h.optionalAuth(), h.showClip)
			clips.POST("/add-note", h.requireAuthScope("write"), h.addClipNote)
			clips.POST("/remove-note", h.requireAuthScope("write"), h.removeClipNote)
			clips.POST("/notes", h.optionalAuth(), h.listClipNotes)
			clips.POST("/favorite", h.requireAuthScope("write"), h.mutateClipFavorite)
			clips.POST("/unfavorite", h.requireAuthScope("write"), h.mutateClipFavorite)
			clips.POST("/my-favorites", h.requireAuthScope("read"), h.listFavoriteClips)
			r.POST(prefix+"/users/clips", h.optionalAuth(), h.listPublicClips)
			r.POST(prefix+"/notes/clips", h.optionalAuth(), h.listNoteClips)
			r.POST(prefix+"/i/export-antennas", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportAntennas)
			r.POST(prefix+"/i/export-blocking", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportBlocking)
			r.POST(prefix+"/i/export-clips", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportClips)
			r.POST(prefix+"/i/export-data", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportAccountData)
			r.POST(prefix+"/i/export-favorites", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportFavorites)
			r.POST(prefix+"/i/export-following", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportFollowing)
			r.POST(prefix+"/i/export-mute", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportMute)
			r.POST(prefix+"/i/export-notes", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportNotes)
			r.POST(prefix+"/i/export-user-lists", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportUserLists)
			r.POST(prefix+"/i/import-antennas", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importAntennas)
			r.POST(prefix+"/i/import-blocking", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importBlocking)
			r.POST(prefix+"/i/import-muting", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importMuting)
			r.POST(prefix+"/i/import-following", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importFollowing)
			r.POST(prefix+"/i/import-user-lists", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importUserLists)
			r.POST(prefix+"/i/import-notes", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importNotes)
		}
		for _, prefix := range []string{"/api", ""} {
			r.GET(prefix+"/emoji", h.getEmojiCompat)
			r.POST(prefix+"/emoji", h.getEmojiCompat)
			r.GET(prefix+"/emojis", h.listEmojisCompat)
			r.POST(prefix+"/emojis", h.listEmojisCompat)
			adminEmoji := r.Group(prefix + "/admin/emoji")
			adminEmoji.POST("/add", h.requireAdminAuth(), h.requireAdminPermission("governance:create_emoji"), h.createAdminEmojiCompat)
			adminEmoji.POST("/add-aliases-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.addAdminEmojiAliasesBulk)
			adminEmoji.POST("/update", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.updateAdminEmojiCompat)
			adminEmoji.POST("/delete", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_emoji"), h.deleteAdminEmojiCompat)
			adminEmoji.POST("/delete-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_emoji"), h.deleteAdminEmojisBulk)
			adminEmoji.POST("/list", h.requireAdminAuth(), h.requireAdminPermission("governance:list_emojis"), h.listAdminEmojisCompat)
			adminEmoji.POST("/remove-aliases-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.removeAdminEmojiAliasesBulk)
			adminEmoji.POST("/set-aliases-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.setAdminEmojiAliasesBulk)
			adminEmoji.POST("/set-category-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.setAdminEmojiCategoryBulk)
			adminEmoji.POST("/set-license-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.setAdminEmojiLicenseBulk)
			r.POST(prefix+"/v2/admin/emoji/list", h.requireAdminAuth(), h.requireAdminPermission("governance:list_emojis"), h.listAdminEmojisV2)
		}
	api := r.Group("/api/v1")
	api.POST("/clips/create", h.requireAuthScope("write"), h.createClip)
	api.POST("/clips/update", h.requireAuthScope("write"), h.updateClip)
	api.POST("/clips/delete", h.requireAuthScope("write"), h.deleteClip)
	api.POST("/clips/list", h.requireAuthScope("read"), h.listClips)
	api.POST("/clips/show", h.optionalAuth(), h.showClip)
	api.POST("/clips/add-note", h.requireAuthScope("write"), h.addClipNote)
	api.POST("/clips/remove-note", h.requireAuthScope("write"), h.removeClipNote)
	api.POST("/clips/notes", h.optionalAuth(), h.listClipNotes)
	api.POST("/clips/favorite", h.requireAuthScope("write"), h.mutateClipFavorite)
	api.POST("/clips/unfavorite", h.requireAuthScope("write"), h.mutateClipFavorite)
	api.POST("/clips/my-favorites", h.requireAuthScope("read"), h.listFavoriteClips)
	api.POST("/users/clips", h.optionalAuth(), h.listPublicClips)
	api.POST("/notes/clips", h.optionalAuth(), h.listNoteClips)
	api.POST("/i/export-antennas", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportAntennas)
	api.POST("/i/export-blocking", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportBlocking)
	api.POST("/i/export-clips", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportClips)
	api.POST("/i/export-data", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportAccountData)
	api.POST("/i/export-favorites", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportFavorites)
	api.POST("/i/export-following", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportFollowing)
	api.POST("/i/export-mute", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportMute)
	api.POST("/i/export-notes", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportNotes)
	api.POST("/i/export-user-lists", h.requireAuthScope("read"), h.requireInteractiveAuth(), h.exportUserLists)
	api.POST("/i/import-antennas", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importAntennas)
	api.POST("/i/import-blocking", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importBlocking)
	api.POST("/i/import-muting", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importMuting)
	api.POST("/i/import-following", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importFollowing)
	api.POST("/i/import-user-lists", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importUserLists)
	api.POST("/i/import-notes", h.requireAuthScope("write"), h.requireInteractiveAuth(), h.importNotes)
		api.GET("/auth/config", h.authConfig)
		api.GET("/site-config", h.siteConfig)
		api.GET("/ping", h.instancePing)
		api.POST("/ping", h.instancePing)
		api.GET("/meta", h.instanceMeta)
		api.POST("/meta", h.instanceMeta)
		api.GET("/server-info", h.instanceServerInfo)
		api.POST("/server-info", h.instanceServerInfo)
		api.GET("/stats", h.instanceStats)
		api.POST("/stats", h.instanceStats)
		api.GET("/charts/users", h.userChart)
		api.POST("/charts/users", h.userChart)
		api.GET("/charts/user/following", h.userFollowingChart)
		api.POST("/charts/user/following", h.userFollowingChart)
		api.GET("/charts/notes", h.notesChart)
		api.POST("/charts/notes", h.notesChart)
		api.GET("/charts/user/notes", h.userNotesChart)
		api.POST("/charts/user/notes", h.userNotesChart)
		api.GET("/charts/active-users", h.requireAdminAuth(), h.requireAdminPermission("governance:list_users"), h.activeUsersChart)
		api.POST("/charts/active-users", h.requireAdminAuth(), h.requireAdminPermission("governance:list_users"), h.activeUsersChart)
		api.GET("/charts/drive", h.driveChart)
		api.POST("/charts/drive", h.driveChart)
		api.GET("/charts/user/drive", h.userDriveChart)
		api.POST("/charts/user/drive", h.userDriveChart)
		api.GET("/announcements", h.optionalAuth(), h.listAnnouncements)
		api.POST("/announcements", h.optionalAuth(), h.listAnnouncements)
		api.GET("/announcements/:id", h.optionalAuth(), h.getAnnouncement)
		api.POST("/announcements/show", h.optionalAuth(), h.showAnnouncement)
		api.POST("/i/read-announcement", h.requireAuth(), h.readAnnouncement)
		api.POST("/tokens/create", h.requireAuth(), h.requireInteractiveAuth(), h.createAPIToken)
		api.POST("/tokens/list", h.requireAuthScope("read"), h.listAPITokens)
		api.POST("/tokens/revoke", h.requireAuth(), h.requireInteractiveAuth(), h.revokeAPIToken)
		api.POST("/auth/register", h.register)
		api.POST("/auth/login", h.login)
		api.POST("/auth/login/mfa", h.completeMFALogin)
		api.POST("/auth/login/mfa/passkey/options", h.beginPasskeyMFALogin)
		api.POST("/auth/login/mfa/passkey", h.completePasskeyMFALogin)
		api.POST("/auth/passkeys/options", h.beginPasswordlessPasskeyLogin)
		api.POST("/auth/passkeys/login", h.completePasswordlessPasskeyLogin)
		api.POST("/auth/logout", h.requireAuth(), h.logout)
		api.POST("/auth/password/forgot", h.requestPasswordReset)
		api.POST("/auth/password/reset", h.resetPassword)
		api.POST("/auth/email/verification", h.requireAuth(), h.requestEmailVerification)
		api.POST("/auth/email/verify", h.verifyEmail)
		api.GET("/auth/oauth/:provider/start", h.oauthStart)
		api.GET("/auth/oauth/:provider/callback", h.oauthCallback)
		api.POST("/uploads/images", h.requireAuth(), h.uploadImage)
		api.GET("/emoji", h.getEmoji)
		api.POST("/emoji", h.getEmoji)
		api.GET("/emojis", h.listEmojis)
		api.POST("/emojis", h.listEmojis)
		api.POST("/files", h.requireAuth(), h.uploadFile)
		api.GET("/files", h.requireAuth(), h.listFiles)
		api.GET("/files/usage", h.requireAuth(), h.getFileUsage)
		api.GET("/file-folders", h.requireAuth(), h.listFileFolders)
		api.POST("/file-folders", h.requireAuth(), h.createFileFolder)
		api.PUT("/file-folders/:id", h.requireAuth(), h.updateFileFolder)
		api.DELETE("/file-folders/:id", h.requireAuth(), h.deleteFileFolder)
		api.GET("/files/:id/download", h.requireAuth(), h.downloadFile)
		api.GET("/files/:id", h.requireAuth(), h.getFile)
		api.PATCH("/files/:id", h.requireAuth(), h.updateFile)
		api.DELETE("/files/:id", h.requireAuth(), h.deleteFile)
		api.POST("/admin/auth/login", h.adminLogin)
		api.POST("/admin/auth/refresh", h.adminRefresh)
		api.POST("/admin/auth/logout", h.requireAdminAuth(), h.adminLogout)
		api.GET("/admin/auth/profile", h.requireAdminAuth(), h.adminProfile)
		api.PUT("/admin/auth/profile", h.requireAdminAuth(), h.updateAdminProfile)
		api.PUT("/admin/auth/password", h.requireAdminAuth(), h.changeAdminPassword)
		api.GET("/admin/auth/menus", h.requireAdminAuth(), h.listCurrentAdminMenus)
		api.GET("/admin/overview", h.requireAdminAuth(), h.requireAdminPermission("system:view_dashboard"), h.adminOverview)
		api.POST("/admin/uploads/avatar", h.requireAdminAuth(), h.uploadAdminAvatar)
		api.POST("/admin/uploads/emoji", h.requireAdminAuth(), h.requireAdminAnyPermission("governance:create_emoji", "governance:update_emoji"), h.uploadAdminEmoji)
		api.GET("/admin/emojis", h.requireAdminAuth(), h.requireAdminPermission("governance:list_emojis"), h.listAdminEmojis)
		api.POST("/admin/emojis", h.requireAdminAuth(), h.requireAdminPermission("governance:create_emoji"), h.createAdminEmoji)
		api.PATCH("/admin/emojis/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.updateAdminEmoji)
		api.DELETE("/admin/emojis/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_emoji"), h.deleteAdminEmoji)
		api.POST("/admin/emoji/add", h.requireAdminAuth(), h.requireAdminPermission("governance:create_emoji"), h.createAdminEmojiCompat)
		api.POST("/admin/emoji/add-aliases-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.addAdminEmojiAliasesBulk)
		api.POST("/admin/emoji/update", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.updateAdminEmojiCompat)
		api.POST("/admin/emoji/delete", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_emoji"), h.deleteAdminEmojiCompat)
		api.POST("/admin/emoji/delete-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_emoji"), h.deleteAdminEmojisBulk)
		api.POST("/admin/emoji/list", h.requireAdminAuth(), h.requireAdminPermission("governance:list_emojis"), h.listAdminEmojisCompat)
		api.POST("/admin/emoji/remove-aliases-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.removeAdminEmojiAliasesBulk)
		api.POST("/admin/emoji/set-aliases-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.setAdminEmojiAliasesBulk)
		api.POST("/admin/emoji/set-category-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.setAdminEmojiCategoryBulk)
		api.POST("/admin/emoji/set-license-bulk", h.requireAdminAuth(), h.requireAdminPermission("governance:update_emoji"), h.setAdminEmojiLicenseBulk)
		api.POST("/v2/admin/emoji/list", h.requireAdminAuth(), h.requireAdminPermission("governance:list_emojis"), h.listAdminEmojisV2)
		api.GET("/users/me", h.requireAuth(), h.getMe)
		api.GET("/users/me/mfa", h.requireAuth(), h.getMFAStatus)
		api.POST("/users/me/mfa/totp/enrollment", h.requireAuth(), h.beginTOTPEnrollment)
		api.POST("/users/me/mfa/totp/confirm", h.requireAuth(), h.confirmTOTPEnrollment)
		api.POST("/users/me/mfa/recovery-codes", h.requireAuth(), h.regenerateMFARecoveryCodes)
		api.DELETE("/users/me/mfa/totp", h.requireAuth(), h.disableTOTP)
		api.GET("/users/me/passkeys", h.requireAuth(), h.listPasskeys)
		api.POST("/users/me/passkeys/registration/options", h.requireAuth(), h.beginPasskeyRegistration)
		api.POST("/users/me/passkeys/registration/verify", h.requireAuth(), h.finishPasskeyRegistration)
		api.PUT("/users/me/passkeys/passwordless", h.requireAuth(), h.setPasskeyPasswordless)
		api.PUT("/users/me/passkeys/:credentialId", h.requireAuth(), h.updatePasskey)
		api.DELETE("/users/me/passkeys/:credentialId", h.requireAuth(), h.deletePasskey)
		api.GET("/users/me/account-lifecycle", h.requireAuth(), h.getAccountLifecycle)
		api.GET("/users/me/sessions", h.requireAuth(), h.listCurrentUserSessions)
		api.GET("/users/me/sessions/:sessionId", h.requireAuth(), h.getCurrentUserSession)
		api.DELETE("/users/me/sessions/:sessionId", h.requireAuth(), h.revokeCurrentUserSession)
		api.GET("/users/me/login-events", h.requireAuth(), h.listCurrentUserLoginEvents)
		api.POST("/users/me/api-tokens", h.requireAuth(), h.requireInteractiveAuth(), h.createAPIToken)
		api.GET("/users/me/api-tokens", h.requireAuthScope("read"), h.listAPITokens)
		api.DELETE("/users/me/api-tokens/:tokenId", h.requireAuth(), h.requireInteractiveAuth(), h.revokeAPIToken)
		api.GET("/users/me/webhooks", h.requireAuthScope("read"), h.listWebhooks)
		api.POST("/users/me/webhooks", h.requireAuthScope("write"), h.createWebhook)
		api.GET("/users/me/webhooks/:webhookId", h.requireAuthScope("read"), h.showWebhook)
		api.PUT("/users/me/webhooks/:webhookId", h.requireAuthScope("write"), h.updateWebhook)
		api.DELETE("/users/me/webhooks/:webhookId", h.requireAuthScope("write"), h.deleteWebhook)
		api.POST("/users/me/webhooks/:webhookId/test", h.requireAuthScope("read"), h.testWebhook)
		api.GET("/users/me/antennas", h.requireAuthScope("read"), h.listAntennas)
		api.POST("/users/me/antennas", h.requireAuthScope("write"), h.createAntenna)
		api.GET("/users/me/antennas/:antennaId", h.requireAuthScope("read"), h.showAntenna)
		api.PUT("/users/me/antennas/:antennaId", h.requireAuthScope("write"), h.updateAntenna)
		api.DELETE("/users/me/antennas/:antennaId", h.requireAuthScope("write"), h.deleteAntenna)
		api.GET("/users/me/antennas/:antennaId/notes", h.requireAuthScope("read"), h.antennaNotes)
		api.POST("/users/me/deletion-requests", h.requireAuth(), h.requestAccountDeletion)
		api.GET("/users/me/articles", h.requireAuth(), h.listCurrentUserArticles)
		api.GET("/users/me/topics", h.requireAuth(), h.listCurrentUserTopics)
		api.GET("/users/me/blocked", h.requireAuth(), h.listBlockedUsers)
		api.GET("/users/me/muted", h.requireAuth(), h.listMutedUsers)
		api.GET("/users/me/lists", h.requireAuth(), h.listCurrentUserLists)
		api.POST("/users/me/lists", h.requireAuth(), h.createUserList)
		api.GET("/users/me/favorite-lists", h.requireAuth(), h.listFavoriteUserLists)
		api.GET("/users/me/follow-requests", h.requireAuth(), h.listReceivedFollowRequests)
		api.GET("/users/me/follow-requests/sent", h.requireAuth(), h.listSentFollowRequests)
		api.POST("/users/me/follow-requests/:requesterId/accept", h.requireAuth(), h.acceptFollowRequest)
		api.POST("/users/me/follow-requests/:requesterId/reject", h.requireAuth(), h.rejectFollowRequest)
		api.PUT("/users/me/settings/follow-approval", h.requireAuth(), h.setFollowApprovalRequired)
		api.GET("/users/current/likes", h.requireAuth(), h.listCurrentUserLikes)
		api.GET("/users/current/favorites", h.requireAuth(), h.listCurrentUserFavorites)
		api.GET("/users/me/collections", h.requireAuth(), h.listCurrentUserCollections)
		api.POST("/users/me/collections", h.requireAuth(), h.createCurrentUserCollection)
		api.PUT("/users/me/collections/:id", h.requireAuth(), h.updateCurrentUserCollection)
		api.DELETE("/users/me/collections/:id", h.requireAuth(), h.deleteCurrentUserCollection)
		api.GET("/users/me/collections/:id/items", h.requireAuth(), h.listCurrentUserCollectionItems)
		api.POST("/users/me/collections/:id/items", h.requireAuth(), h.addCurrentUserCollectionItem)
		api.DELETE("/users/me/collections/:id/items", h.requireAuth(), h.removeCurrentUserCollectionItem)
		api.PUT("/users/me", h.requireAuth(), h.updateMe)
		api.POST("/users/me/password", h.requireAuth(), h.changePassword)
		api.POST("/users/me/avatar", h.requireAuth(), h.uploadUserAvatar)
		api.GET("/users/by-username/:username", h.getUserByUsername)
		api.GET("/users/batch", h.listUsersByIDs)
		api.GET("/users/:id/lists", h.optionalAuth(), h.listUserLists)
		api.GET("/users/:id/badges", h.listUserBadges)
		api.GET("/levels", h.listLevels)
		api.GET("/users/:id", h.getUser)
		api.GET("/users/:id/followers", h.listFollowers)
		api.GET("/users/:id/following", h.listFollowing)
		api.DELETE("/users/me/followers/:followerId", h.requireAuthScope("write"), h.removeFollower)
		api.POST("/users/:id/follow", h.requireAuth(), h.follow)
		api.DELETE("/users/:id/follow", h.requireAuth(), h.unfollow)
		api.POST("/users/:id/follow/cancel", h.requireAuth(), h.cancelFollowRequest)
		api.GET("/users/:id/following-state", h.requireAuth(), h.isFollowing)
		api.GET("/users/:id/safety-state", h.requireAuth(), h.getUserSafetyState)
		api.POST("/users/:id/block", h.requireAuth(), h.blockUser)
		api.DELETE("/users/:id/block", h.requireAuth(), h.unblockUser)
		api.POST("/users/:id/mute", h.requireAuth(), h.muteUserRelation)
		api.DELETE("/users/:id/mute", h.requireAuth(), h.unmuteUserRelation)
		api.GET("/user-lists/:id", h.optionalAuth(), h.getUserList)
		api.PUT("/user-lists/:id", h.requireAuth(), h.updateUserList)
		api.DELETE("/user-lists/:id", h.requireAuth(), h.deleteUserList)
		api.GET("/user-lists/:id/members", h.optionalAuth(), h.listUserListMembers)
		api.POST("/user-lists/:id/members", h.requireAuth(), h.addUserListMember)
		api.DELETE("/user-lists/:id/members", h.requireAuth(), h.removeUserListMember)
		api.POST("/user-lists/:id/copy", h.requireAuth(), h.copyUserList)
		api.POST("/user-lists/:id/favorite", h.requireAuth(), h.favoriteUserList)
		api.DELETE("/user-lists/:id/favorite", h.requireAuth(), h.unfavoriteUserList)
		api.GET("/user-lists/:id/feed", h.optionalAuth(), h.userListFeed)
		h.registerChatRoutes(api)

		api.POST("/topics", h.requireAuth(), h.createTopic)
		api.GET("/topics", h.listTopics)
		api.GET("/topics/:id", h.optionalAuth(), h.getTopic)
		api.GET("/topics/:id/attachments", h.listTopicAttachments)
		api.POST("/topics/:id/attachments", h.requireAuth(), h.uploadTopicAttachment)
		api.GET("/topics/:id/edit-source", h.requireAuth(), h.getEditableTopic)
		api.PUT("/topics/:id", h.requireAuth(), h.updateTopic)
		api.POST("/topics/:id/publish", h.requireAuth(), h.publishTopic)
		api.DELETE("/topics/:id", h.requireAuth(), h.archiveTopic)
		api.GET("/attachments/downloads", h.requireAuth(), h.listUserAttachmentDownloads)
		api.GET("/attachments/sales", h.requireAuth(), h.listUserAttachmentSales)
		api.GET("/attachments/:id/download", h.requireAuth(), h.downloadTopicAttachment)
		api.PATCH("/attachments/:id", h.requireAuth(), h.updateTopicAttachmentPrice)
		api.DELETE("/attachments/:id", h.requireAuth(), h.archiveTopicAttachment)
		api.POST("/topics/:id/comments", h.requireAuth(), h.createTopicComment)
		api.GET("/topics/:id/comments", h.listTopicComments)
		api.POST("/topics/:id/comments/:commentId/accept", h.requireAuth(), h.acceptTopicComment)
		api.POST("/topics/:id/comments/:commentId/unaccept", h.requireAuth(), h.unacceptTopicComment)
		api.POST("/topics/:id/like", h.requireAuth(), h.likeTopic)
		api.DELETE("/topics/:id/like", h.requireAuth(), h.unlikeTopic)
		api.POST("/topics/:id/favorite", h.requireAuth(), h.favoriteTopic)
		api.DELETE("/topics/:id/favorite", h.requireAuth(), h.unfavoriteTopic)
		api.POST("/topics/:id/report", h.requireAuth(), h.reportTopic)
		api.GET("/topics/:id/reactions", h.getTopicReactions)
		api.POST("/topics/:id/poll/votes", h.requireAuth(), h.voteTopicPoll)
		api.GET("/channels", h.optionalAuth(), h.listChannels)
		api.POST("/channels", h.requireAuth(), h.createChannel)
		api.GET("/channels/categories", h.optionalAuth(), h.listChannelCategories)
		api.GET("/channels/featured", h.optionalAuth(), h.listFeaturedChannels)
		api.GET("/channels/owned", h.requireAuth(), h.listOwnedChannels)
		api.GET("/channels/followed", h.requireAuth(), h.listFollowedChannels)
		api.GET("/channels/favorites", h.requireAuth(), h.listFavoriteChannels)
		api.GET("/channels/:id/topics", h.optionalAuth(), h.listChannelTopics)
		api.POST("/channels/:id/follow", h.requireAuth(), h.followChannel)
		api.DELETE("/channels/:id/follow", h.requireAuth(), h.unfollowChannel)
		api.POST("/channels/:id/favorite", h.requireAuth(), h.favoriteChannel)
		api.DELETE("/channels/:id/favorite", h.requireAuth(), h.unfavoriteChannel)
		api.GET("/channels/:id", h.optionalAuth(), h.getChannel)
		api.PUT("/channels/:id", h.requireAuth(), h.updateChannel)
		api.DELETE("/channels/:id", h.requireAuth(), h.archiveChannel)
		api.GET("/categories", h.listCategories)
		api.GET("/categories/:id", h.getCategory)
		api.GET("/links", h.listLinks)
		api.GET("/links/popular", h.listPopularResources)
		api.POST("/links/:id/visit", h.recordLinkVisit)
		api.GET("/tasks", h.listTasks)
		api.GET("/tasks/me", h.requireAuth(), h.listCurrentUserTasks)
		api.POST("/tasks/:id/claim", h.requireAuth(), h.claimTask)

		api.POST("/articles", h.requireAuth(), h.createArticle)
		api.GET("/articles", h.listArticles)
		api.GET("/feed", h.feedArticles)
		api.GET("/articles/:id", h.getArticle)
		api.GET("/articles/:id/edit-source", h.requireAuth(), h.getEditableArticle)
		api.PUT("/articles/:id", h.requireAuth(), h.updateArticle)
		api.POST("/articles/:id/publish", h.requireAuth(), h.publishArticle)
		api.POST("/articles/:id/hide", h.requireAuth(), h.hideArticle)
		api.POST("/articles/:id/archive", h.requireAuth(), h.archiveArticle)
		api.DELETE("/articles/:id", h.requireAuth(), h.archiveArticle)
		api.GET("/tags", h.listTags)
		api.POST("/tags/autocomplete", h.autocompleteTags)
		api.GET("/hashtags/list", h.listHashtags)
		api.POST("/hashtags/list", h.listHashtags)
		api.GET("/hashtags/search", h.searchHashtags)
		api.POST("/hashtags/search", h.searchHashtags)
		api.GET("/hashtags/show", h.showHashtag)
		api.POST("/hashtags/show", h.showHashtag)
		api.GET("/hashtags/trend", h.trendingHashtags)
		api.POST("/hashtags/trend", h.trendingHashtags)
		api.GET("/hashtags/users", h.listHashtagUsers)
		api.POST("/hashtags/users", h.listHashtagUsers)

		api.POST("/articles/:id/comments", h.requireAuth(), h.createComment)
		api.GET("/articles/:id/comments", h.listComments)
		api.GET("/comments/:id", h.getComment)
		api.GET("/comments/:id/conversation", h.getCommentConversation)
		api.GET("/comments/:id/replies", h.listReplies)
		api.DELETE("/comments/:id", h.requireAuth(), h.deleteComment)
		api.POST("/comments/:id/report", h.requireAuth(), h.reportComment)

		api.POST("/articles/:id/like", h.requireAuth(), h.likeArticle)
		api.DELETE("/articles/:id/like", h.requireAuth(), h.unlikeArticle)
		api.POST("/articles/:id/favorite", h.requireAuth(), h.favoriteArticle)
		api.DELETE("/articles/:id/favorite", h.requireAuth(), h.unfavoriteArticle)
		api.POST("/articles/:id/report", h.requireAuth(), h.reportArticle)
		api.GET("/articles/:id/reactions", h.getArticleReactions)
		api.GET("/search/articles", h.searchArticles)
		api.GET("/search/topics", h.searchTopics)
		api.GET("/search/users", h.searchUsers)

		api.GET("/notifications", h.requireAuth(), h.listNotifications)
		api.GET("/notifications/unread-count", h.requireAuth(), h.countUnreadNotifications)
		api.POST("/notifications/read-all", h.requireAuth(), h.markAllNotificationsRead)
		api.POST("/notifications/:id/read", h.requireAuth(), h.markNotificationRead)
		api.GET("/users/me/notification-preferences", h.requireAuth(), h.getNotificationPreferences)
		api.PUT("/users/me/notification-preferences", h.requireAuth(), h.updateNotificationPreferences)
		api.GET("/sw/config", h.webPushConfig)
		api.POST("/sw/register", h.requireAuth(), h.registerWebPushSubscription)
		api.POST("/sw/show-registration", h.requireAuth(), h.showWebPushSubscription)
		api.POST("/sw/unregister", h.requireAuth(), h.unregisterWebPushSubscription)

		api.GET("/credits/balance", h.requireAuth(), h.getCreditBalance)
		api.GET("/credits/ledger", h.requireAuth(), h.listCreditLedger)
		api.GET("/credits/leaderboard", h.listCreditLeaderboard)
		api.GET("/credits/check-in", h.requireAuth(), h.getCheckInStatus)
		api.POST("/credits/check-in", h.requireAuth(), h.checkIn)
		api.GET("/admin/credits/users/:id/balance", h.requireAdminAuth(), h.requireAdminPermission("governance:list_user_credits"), h.getAdminUserCreditBalance)
		api.GET("/admin/credits/users/:id/ledger", h.requireAdminAuth(), h.requireAdminPermission("governance:list_user_credits"), h.listAdminUserCreditLedger)
		api.POST("/admin/credits/users/:id/adjust", h.requireAdminAuth(), h.requireAdminPermission("governance:adjust_user_credits"), h.adjustAdminUserCredits)
		api.GET("/mall/products", h.listMallProducts)
		api.GET("/mall/categories", h.listMallProductCategories)
		api.GET("/mall/products/:id/reviews", h.listMallProductReviews)
		api.POST("/mall/products/:id/reviews", h.requireAuth(), h.createMallProductReview)
		api.GET("/mall/products/:id/reviewable-orders", h.requireAuth(), h.listMallReviewableOrders)
		api.GET("/mall/products/:id", h.getMallProduct)
		api.GET("/mall/reviews", h.requireAuth(), h.listMyMallProductReviews)
		api.GET("/mall/coupons", h.listMallCoupons)
		api.GET("/mall/coupons/mine", h.requireAuth(), h.listMyMallCoupons)
		api.POST("/mall/coupons/:id/claim", h.requireAuth(), h.claimMallCoupon)
		api.GET("/mall/favorites", h.requireAuth(), h.listMallProductFavorites)
		api.GET("/mall/products/:id/favorite", h.requireAuth(), h.getMallProductFavoriteState)
		api.POST("/mall/products/:id/favorite", h.requireAuth(), h.addMallProductFavorite)
		api.DELETE("/mall/products/:id/favorite", h.requireAuth(), h.removeMallProductFavorite)
		api.GET("/mall/cart", h.requireAuth(), h.listMallCart)
		api.PUT("/mall/cart/items/:id", h.requireAuth(), h.setMallCartItem)
		api.DELETE("/mall/cart/items/:id", h.requireAuth(), h.removeMallCartItem)
		api.POST("/mall/cart/checkout", h.requireAuth(), h.checkoutMallCart)
		api.DELETE("/mall/cart", h.requireAuth(), h.clearMallCart)
		api.GET("/mall/addresses", h.requireAuth(), h.listMallAddresses)
		api.POST("/mall/addresses", h.requireAuth(), h.createMallAddress)
		api.PUT("/mall/addresses/:id", h.requireAuth(), h.updateMallAddress)
		api.DELETE("/mall/addresses/:id", h.requireAuth(), h.deleteMallAddress)
		api.POST("/mall/addresses/:id/default", h.requireAuth(), h.setDefaultMallAddress)
		api.POST("/mall/orders", h.requireAuth(), h.createMallOrder)
		api.GET("/mall/orders", h.requireAuth(), h.listMallOrders)
		api.GET("/mall/digital-entitlements", h.requireAuth(), h.listMallDigitalEntitlements)
		api.GET("/mall/orders/:id", h.requireAuth(), h.getMallOrder)
		api.GET("/mall/orders/:id/logs", h.requireAuth(), h.listMallOrderLogs)
		api.GET("/mall/orders/:id/payments", h.requireAuth(), h.listMallOrderPayments)
		api.POST("/mall/orders/:id/pay", h.requireAuth(), h.payMallOrder)
		api.POST("/mall/orders/:id/cancel", h.requireAuth(), h.cancelMallOrder)
		api.POST("/mall/orders/:id/confirm", h.requireAuth(), h.confirmMallOrder)
		api.POST("/mall/orders/:id/refunds", h.requireAuth(), h.createMallRefundRequest)
		api.POST("/mall/refunds/:id/cancel", h.requireAuth(), h.cancelMallRefundRequest)
		api.GET("/mall/refunds", h.requireAuth(), h.listMallRefundRequests)

		api.GET("/admin/reports", h.requireAdminAuth(), h.requireAdminPermission("governance:list_reports"), h.listReports)
		api.POST("/admin/reports/:id/audit", h.requireAdminAuth(), h.requireAdminPermission("governance:audit_report"), h.auditReport)
		api.GET("/admin/users", h.requireAdminAuth(), h.requireAdminPermission("governance:list_users"), h.listGovernanceUsers)
		api.GET("/admin/users/:id/file-capacity", h.requireAdminAuth(), h.requireAdminPermission("governance:list_user_file_capacity"), h.getAdminUserFileCapacity)
		api.PUT("/admin/users/:id/file-capacity", h.requireAdminAuth(), h.requireAdminPermission("governance:list_user_file_capacity"), h.requireAdminPermission("governance:update_user_file_capacity"), h.updateAdminUserFileCapacity)
		api.POST("/admin/users/:id/mute", h.requireAdminAuth(), h.requireAdminPermission("governance:mute_user"), h.muteUser)
		api.POST("/admin/users/:id/unmute", h.requireAdminAuth(), h.requireAdminPermission("governance:unmute_user"), h.unmuteUser)
		api.GET("/admin/channels", h.requireAdminAuth(), h.requireAdminPermission("governance:list_channels"), h.listAdminChannels)
		api.PUT("/admin/channels/:id/featured", h.requireAdminAuth(), h.requireAdminPermission("governance:feature_channel"), h.setAdminChannelFeatured)
		api.PUT("/admin/channels/:id/archived", h.requireAdminAuth(), h.setAdminChannelArchived)
		api.GET("/admin/categories", h.requireAdminAuth(), h.requireAdminPermission("governance:list_categories"), h.listAdminCategories)
		api.POST("/admin/categories", h.requireAdminAuth(), h.requireAdminPermission("governance:create_category"), h.createAdminCategory)
		api.PUT("/admin/categories/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:update_category"), h.updateAdminCategory)
		api.DELETE("/admin/categories/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_category"), h.deleteAdminCategory)
		api.GET("/admin/badges", h.requireAdminAuth(), h.requireAdminPermission("governance:list_badges"), h.listAdminBadges)
		api.POST("/admin/badges", h.requireAdminAuth(), h.requireAdminPermission("governance:create_badge"), h.createAdminBadge)
		api.PUT("/admin/badges/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:update_badge"), h.updateAdminBadge)
		api.DELETE("/admin/badges/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_badge"), h.deleteAdminBadge)
		api.GET("/admin/levels", h.requireAdminAuth(), h.requireAdminPermission("governance:list_levels"), h.listAdminLevels)
		api.POST("/admin/levels", h.requireAdminAuth(), h.requireAdminPermission("governance:create_level"), h.createAdminLevel)
		api.PUT("/admin/levels/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:update_level"), h.updateAdminLevel)
		api.DELETE("/admin/levels/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_level"), h.deleteAdminLevel)
		api.GET("/admin/links", h.requireAdminAuth(), h.requireAdminPermission("governance:list_links"), h.listAdminLinks)
		api.POST("/admin/links", h.requireAdminAuth(), h.requireAdminPermission("governance:create_link"), h.createAdminLink)
		api.PUT("/admin/links/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:update_link"), h.updateAdminLink)
		api.DELETE("/admin/links/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_link"), h.deleteAdminLink)
		api.POST("/admin/ad/list", h.requireAdminAuth(), h.requireAdminPermission("governance:list_ads"), h.listAdminAds)
		api.POST("/admin/ad/create", h.requireAdminAuth(), h.requireAdminPermission("governance:create_ad"), h.createAdminAd)
		api.POST("/admin/ad/update", h.requireAdminAuth(), h.requireAdminPermission("governance:update_ad"), h.updateAdminAd)
		api.POST("/admin/ad/delete", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_ad"), h.deleteAdminAd)
		api.GET("/admin/tasks", h.requireAdminAuth(), h.requireAdminPermission("governance:list_tasks"), h.listAdminTasks)
		api.POST("/admin/tasks", h.requireAdminAuth(), h.requireAdminPermission("governance:create_task"), h.createAdminTask)
		api.PUT("/admin/tasks/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:update_task"), h.updateAdminTask)
		api.DELETE("/admin/tasks/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_task"), h.deleteAdminTask)
		api.GET("/admin/forbidden-words", h.requireAdminAuth(), h.requireAdminPermission("governance:list_forbidden_words"), h.listForbiddenWords)
		api.POST("/admin/forbidden-words", h.requireAdminAuth(), h.requireAdminPermission("governance:create_forbidden_word"), h.createForbiddenWord)
		api.PUT("/admin/forbidden-words/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:update_forbidden_word"), h.updateForbiddenWord)
		api.DELETE("/admin/forbidden-words/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_forbidden_word"), h.deleteForbiddenWord)
		api.GET("/admin/settings", h.requireAdminAuth(), h.requireAdminPermission("governance:list_settings"), h.listSettings)
		api.PUT("/admin/settings/:key", h.requireAdminAuth(), h.requireAdminPermission("governance:update_setting"), h.updateSetting)
		api.POST("/admin/announcements/list", h.requireAdminAuth(), h.requireAdminPermission("governance:list_announcements"), h.listAdminAnnouncements)
		api.POST("/admin/announcements/create", h.requireAdminAuth(), h.requireAdminPermission("governance:create_announcement"), h.createAdminAnnouncement)
		api.POST("/admin/announcements/update", h.requireAdminAuth(), h.requireAdminPermission("governance:update_announcement"), h.updateAdminAnnouncement)
		api.POST("/admin/announcements/delete", h.requireAdminAuth(), h.requireAdminPermission("governance:delete_announcement"), h.deleteAdminAnnouncement)
		api.GET("/admin/invites", h.requireAdminAuth(), h.requireAdminPermission("governance:list_invite_codes"), h.listAdminInviteCodes)
		api.POST("/admin/invites", h.requireAdminAuth(), h.requireAdminPermission("governance:create_invite_codes"), h.createAdminInviteCodes)
		api.DELETE("/admin/invites/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:revoke_invite_code"), h.revokeAdminInviteCode)
		api.GET("/admin/email-logs", h.requireAdminAuth(), h.requireAdminPermission("governance:list_email_logs"), h.listEmailLogs)
		api.GET("/admin/login-logs", h.requireAdminAuth(), h.requireAdminPermission("system:list_login_logs"), h.listLoginLogs)
		api.GET("/admin/operation-logs", h.requireAdminAuth(), h.requireAdminPermission("system:list_operation_logs"), h.listOperationLogs)
		api.GET("/admin/articles", h.requireAdminAuth(), h.requireAdminPermission("governance:list_articles"), h.listAdminArticles)
		api.GET("/admin/articles/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:list_articles"), h.getAdminArticle)
		api.POST("/admin/articles/:id/publish", h.requireAdminAuth(), h.requireAdminPermission("governance:publish_article"), h.publishAdminArticle)
		api.POST("/admin/articles/:id/hide", h.requireAdminAuth(), h.requireAdminPermission("governance:hide_article"), h.hideAdminArticle)
		api.POST("/admin/articles/:id/archive", h.requireAdminAuth(), h.requireAdminPermission("governance:archive_article"), h.archiveAdminArticle)
		api.GET("/admin/topics", h.requireAdminAuth(), h.requireAdminPermission("governance:list_topics"), h.listAdminTopics)
		api.GET("/admin/topics/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:list_topics"), h.getAdminTopic)
		api.POST("/admin/topics/:id/publish", h.requireAdminAuth(), h.requireAdminPermission("governance:publish_topic"), h.publishAdminTopic)
		api.POST("/admin/topics/:id/hide", h.requireAdminAuth(), h.requireAdminPermission("governance:hide_topic"), h.hideAdminTopic)
		api.POST("/admin/topics/:id/archive", h.requireAdminAuth(), h.requireAdminPermission("governance:archive_topic"), h.archiveAdminTopic)
		api.GET("/admin/comments", h.requireAdminAuth(), h.requireAdminPermission("governance:list_comments"), h.listAdminComments)
		api.GET("/admin/comments/:id", h.requireAdminAuth(), h.requireAdminPermission("governance:list_comments"), h.getAdminComment)
		api.POST("/admin/comments/:id/hide", h.requireAdminAuth(), h.requireAdminPermission("governance:hide_comment"), h.hideAdminComment)
		api.POST("/admin/comments/:id/restore", h.requireAdminAuth(), h.requireAdminPermission("governance:restore_comment"), h.restoreAdminComment)
		api.GET("/admin/mall/overview", h.requireAdminAuth(), h.requireAdminPermission("mall:list_orders"), h.adminMallOverview)
		api.GET("/admin/mall/finance-anomalies", h.requireAdminAuth(), h.requireAdminPermission("mall:list_orders"), h.listAdminMallFinanceAnomalies)
		api.GET("/admin/mall/categories", h.requireAdminAuth(), h.requireAdminPermission("mall:list_product_categories"), h.listAdminMallProductCategories)
		api.POST("/admin/mall/categories", h.requireAdminAuth(), h.requireAdminPermission("mall:create_product_category"), h.createAdminMallProductCategory)
		api.PUT("/admin/mall/categories/:id", h.requireAdminAuth(), h.requireAdminPermission("mall:update_product_category"), h.updateAdminMallProductCategory)
		api.GET("/admin/mall/products", h.requireAdminAuth(), h.requireAdminPermission("mall:list_products"), h.listAdminMallProducts)
		api.GET("/admin/mall/products/:id/stock-logs", h.requireAdminAuth(), h.requireAdminPermission("mall:list_products"), h.listAdminMallProductStockLogs)
		api.POST("/admin/mall/products", h.requireAdminAuth(), h.requireAdminPermission("mall:create_product"), h.createAdminMallProduct)
		api.PUT("/admin/mall/products/:id", h.requireAdminAuth(), h.requireAdminPermission("mall:update_product"), h.updateAdminMallProduct)
		api.GET("/admin/mall/reviews", h.requireAdminAuth(), h.requireAdminPermission("mall:list_product_reviews"), h.listAdminMallProductReviews)
		api.PUT("/admin/mall/reviews/:id/status", h.requireAdminAuth(), h.requireAdminPermission("mall:update_product_review"), h.updateAdminMallProductReviewStatus)
		api.GET("/admin/mall/coupons", h.requireAdminAuth(), h.requireAdminPermission("mall:list_coupons"), h.listAdminMallCoupons)
		api.GET("/admin/mall/coupons/:id/usages", h.requireAdminAuth(), h.requireAdminPermission("mall:list_coupon_usages"), h.listAdminMallCouponUsages)
		api.POST("/admin/mall/coupons", h.requireAdminAuth(), h.requireAdminPermission("mall:create_coupon"), h.createAdminMallCoupon)
		api.PUT("/admin/mall/coupons/:id", h.requireAdminAuth(), h.requireAdminPermission("mall:update_coupon"), h.updateAdminMallCoupon)
		api.GET("/admin/mall/digital-entitlements", h.requireAdminAuth(), h.requireAdminPermission("mall:list_digital_entitlements"), h.listAdminMallDigitalEntitlements)
		api.POST("/admin/mall/digital-entitlements/:id/revoke", h.requireAdminAuth(), h.requireAdminPermission("mall:revoke_digital_entitlement"), h.revokeAdminMallDigitalEntitlement)
		api.GET("/admin/mall/orders", h.requireAdminAuth(), h.requireAdminPermission("mall:list_orders"), h.listAdminMallOrders)
		api.GET("/admin/mall/payments", h.requireAdminAuth(), h.requireAdminPermission("mall:list_order_payments"), h.listAdminMallPayments)
		api.POST("/admin/mall/orders/expire", h.requireAdminAuth(), h.requireAdminPermission("mall:close_expired_orders"), h.closeAdminExpiredMallOrders)
		api.POST("/admin/mall/orders/recover-paying", h.requireAdminAuth(), h.requireAdminPermission("mall:recover_paying_orders"), h.recoverAdminStalePayingMallOrders)
		api.POST("/admin/mall/outbox/requeue", h.requireAdminAuth(), h.requireAdminPermission("mall:requeue_outbox_events"), h.requeueAdminMallOutboxEvents)
		api.GET("/admin/mall/outbox/requeue-audits", h.requireAdminAuth(), h.requireAdminPermission("mall:requeue_outbox_events"), h.listAdminMallOutboxRequeueAudits)
		api.PUT("/admin/mall/orders/:id/status", h.requireAdminAuth(), h.requireAdminPermission("mall:update_order_status"), h.updateAdminMallOrderStatus)
		api.GET("/admin/mall/orders/:id/logs", h.requireAdminAuth(), h.requireAdminPermission("mall:list_order_logs"), h.listAdminMallOrderLogs)
		api.GET("/admin/mall/orders/:id/payments", h.requireAdminAuth(), h.requireAdminPermission("mall:list_order_payments"), h.listAdminMallOrderPayments)
		api.GET("/admin/mall/refunds", h.requireAdminAuth(), h.requireAdminPermission("mall:list_refunds"), h.listAdminMallRefundRequests)
		api.POST("/admin/mall/refunds/:id/review", h.requireAdminAuth(), h.requireAdminPermission("mall:review_refunds"), h.reviewAdminMallRefundRequest)
		api.GET("/admin/rbac/users", h.requireAdminAuth(), h.requireAdminPermission("governance:list_admin_users"), h.listAdminUsers)
		api.POST("/admin/rbac/users", h.requireAdminAuth(), h.requireAdminPermission("governance:create_admin_user"), h.createAdminUser)
		api.GET("/admin/rbac/roles", h.requireAdminAuth(), h.requireAdminPermission("governance:list_roles"), h.listAdminRoles)
		api.PUT("/admin/rbac/users/:id/roles", h.requireAdminAuth(), h.requireAdminPermission("governance:assign_roles"), h.assignAdminRoles)
		api.GET("/admin/system/users", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_users"), h.listSystemUsers)
		api.GET("/admin/system/users/:id", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_users"), h.getSystemUser)
		api.POST("/admin/system/users", h.requireAdminAuth(), h.requireAdminPermission("system:create_system_user"), h.createSystemUser)
		api.PUT("/admin/system/users/:id", h.requireAdminAuth(), h.requireAdminPermission("system:update_system_user"), h.updateSystemUser)
		api.DELETE("/admin/system/users/:id", h.requireAdminAuth(), h.requireAdminPermission("system:delete_system_user"), h.deleteSystemUser)
		api.PUT("/admin/system/users/:id/password", h.requireAdminAuth(), h.requireAdminPermission("system:reset_system_user_password"), h.resetSystemUserPassword)
		api.PUT("/admin/system/users/:id/roles", h.requireAdminAuth(), h.requireAdminPermission("system:assign_system_user_roles"), h.assignSystemUserRoles)
		api.GET("/admin/system/roles", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_roles"), h.listSystemRoles)
		api.POST("/admin/system/roles", h.requireAdminAuth(), h.requireAdminPermission("system:create_system_role"), h.createSystemRole)
		api.PUT("/admin/system/roles/:id", h.requireAdminAuth(), h.requireAdminPermission("system:update_system_role"), h.updateSystemRole)
		api.DELETE("/admin/system/roles/:id", h.requireAdminAuth(), h.requireAdminPermission("system:delete_system_role"), h.deleteSystemRole)
		api.GET("/admin/system/roles/:id/menu-ids", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_roles"), h.getSystemRoleMenuIDs)
		api.GET("/admin/system/roles/:id/permissions", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_roles"), h.getSystemRolePermissions)
		api.PUT("/admin/system/roles/:id/menus", h.requireAdminAuth(), h.requireAdminPermission("system:assign_system_role_menus"), h.assignSystemRoleMenus)
		api.PUT("/admin/system/roles/:id/permissions", h.requireAdminAuth(), h.requireAdminPermission("system:assign_system_role_menus"), h.assignSystemRoleMenus)
		api.GET("/admin/system/menus", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_menus"), h.listSystemMenus)
		api.POST("/admin/system/menus", h.requireAdminAuth(), h.requireAdminPermission("system:create_system_menu"), h.createSystemMenu)
		api.PUT("/admin/system/menus/:id", h.requireAdminAuth(), h.requireAdminPermission("system:update_system_menu"), h.updateSystemMenu)
		api.DELETE("/admin/system/menus/:id", h.requireAdminAuth(), h.requireAdminPermission("system:delete_system_menu"), h.deleteSystemMenu)
		api.GET("/admin/system/depts", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_depts"), h.listSystemDepts)
		api.POST("/admin/system/depts", h.requireAdminAuth(), h.requireAdminPermission("system:create_system_dept"), h.createSystemDept)
		api.PUT("/admin/system/depts/:id", h.requireAdminAuth(), h.requireAdminPermission("system:update_system_dept"), h.updateSystemDept)
		api.DELETE("/admin/system/depts/:id", h.requireAdminAuth(), h.requireAdminPermission("system:delete_system_dept"), h.deleteSystemDept)
		api.GET("/admin/system/departments", h.requireAdminAuth(), h.requireAdminPermission("system:list_system_depts"), h.listSystemDepts)
		api.POST("/admin/system/departments", h.requireAdminAuth(), h.requireAdminPermission("system:create_system_dept"), h.createSystemDept)
		api.PUT("/admin/system/departments/:id", h.requireAdminAuth(), h.requireAdminPermission("system:update_system_dept"), h.updateSystemDept)
		api.DELETE("/admin/system/departments/:id", h.requireAdminAuth(), h.requireAdminPermission("system:delete_system_dept"), h.deleteSystemDept)
		api.POST("/admin/notifications/system", h.requireAdminAuth(), h.requireAdminPermission("system:send_system_notification"), h.sendSystemNotification)
		api.POST("/admin/search/rebuild", h.requireAdminAuth(), h.requireAdminPermission("system:rebuild_search"), h.startSearchRebuild)
		api.GET("/admin/search/rebuild", h.requireAdminAuth(), h.requireAdminPermission("system:view_search_rebuild"), h.getSearchRebuildStatus)
	}
}

func (h *Handler) health(c *gin.Context) {
	response.Success(c, gin.H{"status": "ok"})
}

func (h *Handler) register(c *gin.Context) {
	var req registerRequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Register, authRateLimitRegister, req.Email) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	settings, err := h.loadAuthSettings(ctx, false)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	mode := registrationModeFromSettings(settings)
	if !settingBool(settings, "auth.password.enabled", true) || mode == registrationModeClosed {
		writeError(c, http.StatusForbidden, "password registration disabled", "permission_denied")
		return
	}
	inviteCode := ""
	requireInvite := mode == registrationModeInviteOnly
	if requireInvite {
		inviteCode = strings.TrimSpace(req.InviteCode)
	}
	resp, err := h.clients.User.Register(ctx, &userpb.RegisterRequest{
		Username:      req.Username,
		Email:         req.Email,
		Password:      req.Password,
		Nickname:      req.Nickname,
		InviteCode:    inviteCode,
		RequireInvite: requireInvite,
		Client:        sessionClientInfo(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeAuthResponseProfileTheme(ctx, resp)
	response.Success(c, resp)
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.Login, authRateLimitLogin, req.Account) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if resp, handled := h.tryWebmasterLogin(c, ctx, req); handled {
		if resp != nil {
			h.sanitizeAuthResponseProfileTheme(ctx, resp)
			response.Success(c, resp)
		}
		return
	}
	if !h.passwordLoginEnabled(ctx) {
		writeError(c, http.StatusForbidden, "password login disabled", "permission_denied")
		return
	}
	resp, err := h.clients.User.Login(ctx, &userpb.LoginRequest{Account: req.Account, Password: req.Password, Client: sessionClientInfo(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeAuthResponseProfileTheme(ctx, resp)
	response.Success(c, resp)
}

func (h *Handler) logout(c *gin.Context) {
	accessToken, err := h.authTokenFromRequest(c)
	if err != nil {
		writeAuthenticationError(c, err)
		return
	}
	expiresAt, err := h.authTokenExpiry(accessToken)
	if err != nil {
		writeAuthenticationError(c, err)
		return
	}
	if h.tokenRevocations == nil {
		writeAuthenticationError(c, errTokenRevocationUnavailable)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if err := h.tokenRevocations.Revoke(ctx, accessToken, expiresAt); err != nil {
		writeAuthenticationError(c, tokenRevocationUnavailableError(err))
		return
	}
	response.Success(c, gin.H{"logged_out": true})
}

func (h *Handler) requestPasswordReset(c *gin.Context) {
	var req passwordResetRequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.PasswordReset, authRateLimitPasswordReset, req.Email) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.passwordLoginEnabled(ctx) {
		writeError(c, http.StatusForbidden, "password login disabled", "permission_denied")
		return
	}
	resp, err := h.clients.User.RequestPasswordReset(ctx, &userpb.PasswordResetRequest{Email: req.Email})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	payload := gin.H{
		"accepted":   resp.GetAccepted(),
		"expires_at": resp.GetExpiresAt(),
	}
	response.Success(c, payload)
}

func (h *Handler) resetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.PasswordResetConfirm, authRateLimitPasswordResetConfirm, req.Token) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.passwordLoginEnabled(ctx) {
		writeError(c, http.StatusForbidden, "password login disabled", "permission_denied")
		return
	}
	resp, err := h.clients.User.ResetPassword(ctx, &userpb.ResetPasswordRequest{Token: req.Token, NewPassword: req.NewPassword})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) requestEmailVerification(c *gin.Context) {
	userID := currentUserID(c)
	if !h.allowAuthRateLimit(c, h.authRateLimits.EmailVerification, authRateLimitEmailVerification, authUserRateLimitSubject(userID)) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.RequestEmailVerification(ctx, &userpb.EmailVerificationRequest{UserId: userID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	payload := gin.H{
		"accepted":         resp.GetAccepted(),
		"expires_at":       resp.GetExpiresAt(),
		"already_verified": resp.GetAlreadyVerified(),
	}
	response.Success(c, payload)
}

func (h *Handler) verifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.VerifyEmail(ctx, &userpb.VerifyEmailRequest{Token: req.Token})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) adminLogin(c *gin.Context) {
	var req adminLoginRequest
	if !bindJSON(c, &req) {
		return
	}
	if !h.allowAuthRateLimit(c, h.authRateLimits.AdminLogin, authRateLimitAdminLogin, req.Account) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.Login(ctx, &adminpb.LoginRequest{
		Account:   req.Account,
		Password:  req.Password,
		LoginIp:   c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) adminRefresh(c *gin.Context) {
	var req adminRefreshRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.Refresh(ctx, &adminpb.RefreshTokenRequest{RefreshToken: req.RefreshToken})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) adminLogout(c *gin.Context) {
	accessToken, err := h.authTokenFromRequest(c)
	if err != nil {
		writeAuthenticationError(c, err)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.Logout(ctx, &adminpb.LogoutRequest{AccessToken: accessToken})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) adminProfile(c *gin.Context) {
	if profile, ok := c.Get("admin_profile"); ok {
		if adminProfile, ok := profile.(*adminpb.ProfileResponse); ok {
			response.Success(c, adminProfilePayload(adminProfile))
			return
		}
		response.Success(c, profile)
		return
	}
	writeError(c, http.StatusUnauthorized, "missing admin profile", "unauthorized")
}

func (h *Handler) updateAdminProfile(c *gin.Context) {
	var req updateProfileRequest
	if !bindJSON(c, &req) {
		return
	}
	actor := currentActor(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	bio := strings.TrimSpace(req.Bio)
	if bio == "" {
		bio = strings.TrimSpace(req.Description)
	}
	resp, err := h.clients.Admin.UpdateProfile(ctx, &adminpb.UpdateProfileRequest{
		Actor:     actor,
		Nickname:  req.Nickname,
		Email:     req.Email,
		Phone:     req.Phone,
		AvatarUrl: req.AvatarURL,
		Bio:       bio,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, adminProfilePayload(resp))
}

func (h *Handler) changeAdminPassword(c *gin.Context) {
	var req changePasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ChangePassword(ctx, &adminpb.ChangePasswordRequest{
		Actor:       currentActor(c),
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, adminProfilePayload(resp))
}

func (h *Handler) uploadAdminAvatar(c *gin.Context) {
	payload, ok := h.saveUploadedImage(c, "avatars")
	if !ok {
		return
	}
	payload["avatar_url"] = payload["url"]
	response.Success(c, payload)
}

func (h *Handler) uploadAdminEmoji(c *gin.Context) {
	payload, ok := h.saveUploadedImage(c, "emojis")
	if !ok {
		return
	}
	path, _ := payload["path"].(string)
	payload["fileId"] = filepath.Base(path)
	payload["file_id"] = filepath.Base(path)
	response.Success(c, payload)
}

func (h *Handler) uploadUserAvatar(c *gin.Context) {
	if !h.allowFileUploadRateLimit(c, currentUserID(c)) {
		return
	}
	payload, ok := h.saveUploadedImage(c, "avatars")
	if !ok {
		return
	}
	payload["avatar_url"] = payload["url"]
	response.Success(c, payload)
}

func (h *Handler) uploadImage(c *gin.Context) {
	if !h.allowFileUploadRateLimit(c, currentUserID(c)) {
		return
	}
	payload, ok := h.saveUploadedImage(c, "images")
	if !ok {
		return
	}
	payload["image_url"] = payload["url"]
	response.Success(c, payload)
}

func (h *Handler) saveUploadedImage(c *gin.Context, folder string) (gin.H, bool) {
	if !h.hasAttachmentStore(c) {
		return nil, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadedImageSize)
	file, err := c.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "missing image file", "bad_request")
		return nil, false
	}
	if file.Size <= 0 || file.Size > maxUploadedImageSize {
		writeError(c, http.StatusBadRequest, "image file size must be between 1 byte and 5 MiB", "bad_request")
		return nil, false
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedAvatarExt(ext) {
		writeError(c, http.StatusBadRequest, "image file type is not supported", "bad_request")
		return nil, false
	}
	name, err := uploadedAvatarName(ext)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "create image name failed", "internal_error")
		return nil, false
	}
	folder = strings.Trim(strings.TrimSpace(folder), `/\`)
	if folder == "" {
		folder = "images"
	}
	reader, err := file.Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "open image file failed", "bad_request")
		return nil, false
	}
	defer reader.Close()
	head := make([]byte, 512)
	n, readErr := io.ReadFull(reader, head)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(c, http.StatusBadRequest, "read image file failed", "bad_request")
		return nil, false
	}
	if !matchesImageContentType(ext, http.DetectContentType(head[:n])) {
		writeError(c, http.StatusBadRequest, "image content does not match its file type", "bad_request")
		return nil, false
	}
	objectKey := "uploads/" + folder + "/" + name
	transferCtx, cancel := context.WithTimeout(c.Request.Context(), imageTransferTimeout)
	err = h.attachments.Upload(transferCtx, objectKey, io.MultiReader(bytes.NewReader(head[:n]), reader), file.Size, imageContentType(ext))
	cancel()
	if err != nil {
		writeError(c, http.StatusBadGateway, "store image failed", "storage_unavailable")
		return nil, false
	}
	fileID, ok := h.registerUploadedImage(c, folder, file.Filename, imageContentType(ext), file.Size, objectKey)
	if !ok {
		return nil, false
	}
	path := "/" + objectKey
	url := h.publicURL(c, path)
	payload := gin.H{"url": url, "path": path}
	if fileID > 0 {
		payload["file_id"] = strconv.FormatInt(fileID, 10)
		payload["file_url"] = h.publicURL(c, "/api/v1/files/"+strconv.FormatInt(fileID, 10)+"/download")
	}
	return payload, true
}

func (h *Handler) serveUploadedImage(c *gin.Context) {
	if !h.hasAttachmentStore(c) {
		return
	}
	objectKey, contentType, ok := publicImageObject(c.Param("kind"), c.Param("name"))
	if !ok {
		writeError(c, http.StatusNotFound, "uploaded image not found", "not_found")
		return
	}
	transferCtx, cancel := context.WithTimeout(c.Request.Context(), imageTransferTimeout)
	defer cancel()
	object, info, err := h.attachments.Open(transferCtx, objectKey)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			writeError(c, http.StatusNotFound, "uploaded image not found", "not_found")
			return
		}
		writeError(c, http.StatusBadGateway, "image storage unavailable", "storage_unavailable")
		return
	}
	defer object.Close()
	c.DataFromReader(http.StatusOK, info.Size, contentType, object, map[string]string{
		"Cache-Control":          "public, max-age=31536000, immutable",
		"X-Content-Type-Options": "nosniff",
	})
}

func publicImageObject(kind string, name string) (string, string, bool) {
	if (kind != "avatars" && kind != "images" && kind != "emojis") || name == "" || strings.ContainsAny(name, `/\\`) {
		return "", "", false
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !allowedAvatarExt(ext) {
		return "", "", false
	}
	return "uploads/" + kind + "/" + name, imageContentType(ext), true
}

func imageContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func matchesImageContentType(ext string, contentType string) bool {
	return imageContentType(ext) == contentType
}

func (h *Handler) publicURL(c *gin.Context, path string) string {
	if h != nil && h.publicBaseURL != "" {
		return h.publicBaseURL + path
	}
	return publicRequestURL(c, path)
}

func (h *Handler) getUser(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileTheme(ctx, resp.GetUser())
	response.Success(c, toPublicUserResponse(resp))
}

func (h *Handler) getUserByUsername(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.GetUserByUsername(ctx, &userpb.UsernameRequest{Username: c.Param("username")})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileTheme(ctx, resp.GetUser())
	response.Success(c, toPublicUserResponse(resp))
}

func (h *Handler) listUsersByIDs(c *gin.Context) {
	ids, ok := queryPositiveInt64CSV(c, "ids", publicUserBatchLookupLimit)
	if !ok || len(ids) == 0 {
		writeError(c, http.StatusBadRequest, "ids must contain up to 100 positive user IDs", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
		Ids:      ids,
		Page:     1,
		PageSize: int32(len(ids)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileThemes(ctx, resp.GetItems())
	response.Success(c, toPublicUserListResponse(resp))
}

func normalizeProfileTheme(value string) string {
	theme := strings.ToLower(strings.TrimSpace(value))
	if theme == "" {
		return profileThemeDefault
	}
	return theme
}

func validProfileTheme(value string) bool {
	switch normalizeProfileTheme(value) {
	case profileThemeDefault, profileThemePro:
		return true
	default:
		return false
	}
}

func profileThemeRequiresEntitlement(value string) bool {
	return normalizeProfileTheme(value) == profileThemePro
}

func profileBackgroundRequiresEntitlement(value string) bool {
	return strings.TrimSpace(value) != ""
}

func (h *Handler) sanitizeUserProfileTheme(ctx context.Context, user *userpb.UserInfo) {
	if user == nil {
		return
	}
	if profileBackgroundRequiresEntitlement(user.GetBackgroundUrl()) {
		allowed, err := h.userHasActiveDigitalEntitlement(ctx, user.GetId(), digitalEntitlementGrantTypeMembership, "")
		if err != nil || !allowed {
			user.BackgroundUrl = ""
		}
	}
	theme := normalizeProfileTheme(user.GetProfileTheme())
	if !validProfileTheme(theme) || !profileThemeRequiresEntitlement(theme) {
		user.ProfileTheme = profileThemeDefault
		return
	}
	allowed, err := h.userHasActiveDigitalEntitlement(ctx, user.GetId(), "theme", theme)
	if err != nil || !allowed {
		user.ProfileTheme = profileThemeDefault
		return
	}
	user.ProfileTheme = theme
}

func (h *Handler) sanitizeUserProfileThemes(ctx context.Context, users []*userpb.UserInfo) {
	backgroundUserIDs := make([]int64, 0, len(users))
	themeUserIDs := make([]int64, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		if profileBackgroundRequiresEntitlement(user.GetBackgroundUrl()) {
			backgroundUserIDs = append(backgroundUserIDs, user.GetId())
		}
		theme := normalizeProfileTheme(user.GetProfileTheme())
		if !validProfileTheme(theme) || !profileThemeRequiresEntitlement(theme) {
			user.ProfileTheme = profileThemeDefault
			continue
		}
		themeUserIDs = append(themeUserIDs, user.GetId())
	}
	membershipUsers, membershipErr := h.listActiveEntitlementUserIDs(ctx, backgroundUserIDs, digitalEntitlementGrantTypeMembership, "")
	themeUsers, themeErr := h.listActiveEntitlementUserIDs(ctx, themeUserIDs, "theme", profileThemePro)
	for _, user := range users {
		if user == nil {
			continue
		}
		if profileBackgroundRequiresEntitlement(user.GetBackgroundUrl()) {
			if membershipErr != nil || !membershipUsers[user.GetId()] {
				user.BackgroundUrl = ""
			}
		}
		theme := normalizeProfileTheme(user.GetProfileTheme())
		if !validProfileTheme(theme) || !profileThemeRequiresEntitlement(theme) {
			user.ProfileTheme = profileThemeDefault
			continue
		}
		if themeErr != nil || !themeUsers[user.GetId()] {
			user.ProfileTheme = profileThemeDefault
			continue
		}
		user.ProfileTheme = theme
	}
}

func (h *Handler) listActiveEntitlementUserIDs(ctx context.Context, userIDs []int64, grantType string, grantKey string) (map[int64]bool, error) {
	active := make(map[int64]bool)
	requested := make(map[int64]bool, len(userIDs))
	orderedUserIDs := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 || requested[userID] {
			continue
		}
		requested[userID] = true
		orderedUserIDs = append(orderedUserIDs, userID)
	}
	if len(orderedUserIDs) == 0 {
		return active, nil
	}
	if h.clients == nil || h.clients.Mall == nil {
		return nil, status.Error(codes.Unavailable, "mall service unavailable")
	}
	for start := 0; start < len(orderedUserIDs); start += digitalEntitlementBatchUserLookupLimit {
		end := start + digitalEntitlementBatchUserLookupLimit
		if end > len(orderedUserIDs) {
			end = len(orderedUserIDs)
		}
		resp, err := h.clients.Mall.ListActiveEntitlementUserIDs(ctx, &mallpb.ListActiveEntitlementUserIDsRequest{
			UserIds:   orderedUserIDs[start:end],
			GrantType: grantType,
			GrantKey:  grantKey,
		})
		if err != nil {
			return nil, err
		}
		for _, userID := range resp.GetUserIds() {
			if requested[userID] {
				active[userID] = true
			}
		}
	}
	return active, nil
}

func (h *Handler) sanitizeAuthResponseProfileTheme(ctx context.Context, resp *userpb.AuthResponse) {
	if resp == nil {
		return
	}
	h.sanitizeUserProfileTheme(ctx, resp.GetUser())
}

func (h *Handler) userHasActiveDigitalEntitlement(ctx context.Context, userID int64, grantType string, grantKey string) (bool, error) {
	if h.clients == nil || h.clients.Mall == nil {
		return false, status.Error(codes.Unavailable, "mall service unavailable")
	}
	grantType = strings.ToLower(strings.TrimSpace(grantType))
	grantKey = strings.ToLower(strings.TrimSpace(grantKey))
	now := time.Now()
	limit := digitalEntitlementLookupLimit
	offset := int32(0)
	for {
		resp, err := h.clients.Mall.ListUserDigitalEntitlements(ctx, &mallpb.ListUserDigitalEntitlementsRequest{
			UserId:    userID,
			Status:    digitalEntitlementStatusActive,
			GrantType: grantType,
			GrantKey:  grantKey,
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			return false, err
		}
		for _, entitlement := range resp.GetItems() {
			if !digitalEntitlementIsActive(entitlement, now) {
				continue
			}
			if strings.ToLower(strings.TrimSpace(entitlement.GetGrantType())) != grantType {
				continue
			}
			entitlementGrantKey := strings.ToLower(strings.TrimSpace(entitlement.GetGrantKey()))
			if entitlementGrantKey == "" {
				continue
			}
			if grantKey != "" && entitlementGrantKey != grantKey {
				continue
			}
			if grantType == digitalEntitlementGrantTypeMembership && entitlement.GetExpiresAt() <= now.UnixMilli() {
				continue
			}
			return true, nil
		}
		if int32(len(resp.GetItems())) < limit {
			break
		}
		offset += limit
	}
	return false, nil
}

func digitalEntitlementIsActive(entitlement *mallpb.DigitalEntitlement, now time.Time) bool {
	if entitlement == nil || entitlement.GetRevokedAt() > 0 {
		return false
	}
	statusText := strings.ToUpper(strings.TrimSpace(entitlement.GetStatus()))
	if statusText != digitalEntitlementStatusActive {
		return false
	}
	expiresAt := entitlement.GetExpiresAt()
	return expiresAt <= 0 || expiresAt > now.UnixMilli()
}

func (h *Handler) listUserBadges(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	badges, err := h.listActiveBadgeDefinitions(ctx)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := buildUserBadges(resp.GetUser(), badges)
	if h.clients.Mall != nil {
		entitlements, err := h.listActiveBadgeEntitlements(ctx, id)
		if err == nil {
			items = mergeDigitalBadgeEntitlements(items, badges, entitlements)
		}
		entitlements, err = h.listActiveMembershipEntitlements(ctx, id)
		if err == nil {
			items = mergeMembershipEntitlementBadge(items, entitlements)
		}
	}
	total := len(items)
	items = paginateBadgeRows(items, int(queryInt32(c, "limit", 20)), int(queryInt32(c, "offset", 0)))
	response.Success(c, gin.H{"items": items, "total": total})
}

func (h *Handler) listActiveBadgeDefinitions(ctx context.Context) ([]*adminpb.BadgeInfo, error) {
	const limit int32 = 100
	items := make([]*adminpb.BadgeInfo, 0)
	for offset := int32(0); ; offset += limit {
		resp, err := h.clients.Admin.ListBadges(ctx, &adminpb.ListBadgesRequest{
			Status: 2,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		page := resp.GetItems()
		items = append(items, page...)
		if int32(len(page)) < limit {
			break
		}
	}
	return items, nil
}

func (h *Handler) listActiveBadgeEntitlements(ctx context.Context, userID int64) ([]*mallpb.DigitalEntitlement, error) {
	return h.listActiveDigitalEntitlements(ctx, userID, "badge")
}

func (h *Handler) listActiveMembershipEntitlements(ctx context.Context, userID int64) ([]*mallpb.DigitalEntitlement, error) {
	return h.listActiveDigitalEntitlements(ctx, userID, digitalEntitlementGrantTypeMembership)
}

func (h *Handler) listActiveDigitalEntitlements(ctx context.Context, userID int64, grantType string) ([]*mallpb.DigitalEntitlement, error) {
	const limit int32 = 100
	items := make([]*mallpb.DigitalEntitlement, 0)
	for offset := int32(0); ; offset += limit {
		resp, err := h.clients.Mall.ListUserDigitalEntitlements(ctx, &mallpb.ListUserDigitalEntitlementsRequest{
			UserId:    userID,
			Status:    digitalEntitlementStatusActive,
			GrantType: grantType,
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			return nil, err
		}
		page := resp.GetItems()
		items = append(items, page...)
		if int32(len(page)) < limit {
			break
		}
	}
	return items, nil
}

func (h *Handler) listLevels(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListLevels(ctx, &adminpb.ListLevelsRequest{
		Status: 2,
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getMe(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileTheme(ctx, resp.GetUser())
	response.Success(c, resp)
}

func (h *Handler) listCurrentUserLikes(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListLikes(ctx, &reactionpb.ListLikesRequest{
		UserId:     currentUserID(c),
		EntityType: c.Query("entity_type"),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listCurrentUserFavorites(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Reaction.ListFavorites(ctx, &reactionpb.ListFavoritesRequest{
		UserId:     currentUserID(c),
		EntityType: c.Query("entity_type"),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateMe(c *gin.Context) {
	var req updateProfileRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if profileBackgroundRequiresEntitlement(req.BackgroundURL) {
		allowed, err := h.userHasActiveDigitalEntitlement(ctx, currentUserID(c), digitalEntitlementGrantTypeMembership, "")
		if err != nil {
			writeRPCError(c, err)
			return
		}
		if !allowed {
			writeError(c, http.StatusForbidden, profileBackgroundMembershipRequiredMessage, "permission_denied")
			return
		}
	}
	profileTheme := ""
	if req.ProfileTheme != nil {
		profileTheme = normalizeProfileTheme(*req.ProfileTheme)
		if !validProfileTheme(profileTheme) {
			writeError(c, http.StatusBadRequest, "invalid profile theme", "invalid_argument")
			return
		}
		if profileThemeRequiresEntitlement(profileTheme) {
			allowed, err := h.userHasActiveDigitalEntitlement(ctx, currentUserID(c), "theme", profileTheme)
			if err != nil {
				writeRPCError(c, err)
				return
			}
			if !allowed {
				writeError(c, http.StatusForbidden, "profile theme entitlement required", "permission_denied")
				return
			}
		}
	}
	resp, err := h.clients.User.UpdateProfile(ctx, &userpb.UpdateProfileRequest{
		Id:            currentUserID(c),
		Nickname:      req.Nickname,
		AvatarUrl:     req.AvatarURL,
		BackgroundUrl: req.BackgroundURL,
		ProfileTheme:  profileTheme,
		Bio:           req.Bio,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileTheme(ctx, resp.GetUser())
	response.Success(c, resp)
}

func (h *Handler) changePassword(c *gin.Context) {
	var req changePasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.ChangePassword(ctx, &userpb.ChangePasswordRequest{Id: currentUserID(c), OldPassword: req.OldPassword, NewPassword: req.NewPassword})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) follow(c *gin.Context) {
	followeeID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.Follow(ctx, &userpb.FollowRequest{FollowerId: currentUserID(c), FolloweeId: followeeID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) unfollow(c *gin.Context) {
	followeeID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.Unfollow(ctx, &userpb.FollowRequest{FollowerId: currentUserID(c), FolloweeId: followeeID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) isFollowing(c *gin.Context) {
	followeeID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.IsFollowing(ctx, &userpb.FollowRequest{FollowerId: currentUserID(c), FolloweeId: followeeID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listFollowers(c *gin.Context) {
	h.listFollows(c, true)
}

func (h *Handler) listFollowing(c *gin.Context) {
	h.listFollows(c, false)
}

func (h *Handler) listFollows(c *gin.Context, followers bool) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &userpb.ListFollowsRequest{UserId: id, Page: queryInt32(c, "page", 1), PageSize: queryInt32(c, "page_size", 20)}
	var (
		resp *userpb.UserListResponse
		err  error
	)
	if followers {
		resp, err = h.clients.User.ListFollowers(ctx, req)
	} else {
		resp, err = h.clients.User.ListFollowing(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileThemes(ctx, resp.GetItems())
	response.Success(c, toPublicUserListResponse(resp))
}

type followRequestView struct {
	ID          int64           `json:"id"`
	RequesterID int64           `json:"requester_id"`
	TargetID    int64           `json:"target_id"`
	CreatedAt   int64           `json:"created_at"`
	Counterpart *publicUserView `json:"counterpart,omitempty"`
}

type followRequestListView struct {
	Items []*followRequestView `json:"items"`
	Total int64                `json:"total"`
}

func toFollowRequestListView(resp *userpb.FollowRequestListResponse) followRequestListView {
	items := make([]*followRequestView, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, &followRequestView{
			ID:          item.GetId(),
			RequesterID: item.GetRequesterId(),
			TargetID:    item.GetTargetId(),
			CreatedAt:   item.GetCreatedAt(),
			Counterpart: toPublicUserView(item.GetCounterpart()),
		})
	}
	return followRequestListView{Items: items, Total: resp.GetTotal()}
}

func (h *Handler) listReceivedFollowRequests(c *gin.Context) {
	h.listPendingFollowRequests(c, true)
}

func (h *Handler) listSentFollowRequests(c *gin.Context) {
	h.listPendingFollowRequests(c, false)
}

func (h *Handler) listPendingFollowRequests(c *gin.Context, received bool) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &userpb.ListFollowRequestsRequest{
		ActorId:  currentUserID(c),
		Page:     queryInt32(c, "page", 1),
		PageSize: queryInt32(c, "page_size", 20),
	}
	var (
		resp *userpb.FollowRequestListResponse
		err  error
	)
	if received {
		resp, err = h.clients.User.ListReceivedFollowRequests(ctx, req)
	} else {
		resp, err = h.clients.User.ListSentFollowRequests(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	users := make([]*userpb.UserInfo, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		if item.GetCounterpart() != nil {
			users = append(users, item.GetCounterpart())
		}
	}
	h.sanitizeUserProfileThemes(ctx, users)
	response.Success(c, toFollowRequestListView(resp))
}

func (h *Handler) acceptFollowRequest(c *gin.Context) {
	h.resolveFollowRequest(c, true)
}

func (h *Handler) rejectFollowRequest(c *gin.Context) {
	h.resolveFollowRequest(c, false)
}

func (h *Handler) resolveFollowRequest(c *gin.Context, accept bool) {
	requesterID, ok := pathInt64(c, "requesterId")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &userpb.FollowRequestActionRequest{ActorId: currentUserID(c), CounterpartId: requesterID}
	var (
		resp *userpb.SimpleResponse
		err  error
	)
	if accept {
		resp, err = h.clients.User.AcceptFollowRequest(ctx, req)
	} else {
		resp, err = h.clients.User.RejectFollowRequest(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) cancelFollowRequest(c *gin.Context) {
	targetID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.CancelFollowRequest(ctx, &userpb.FollowRequestActionRequest{
		ActorId:       currentUserID(c),
		CounterpartId: targetID,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

type setFollowApprovalRequest struct {
	Required bool `json:"required"`
}

func (h *Handler) setFollowApprovalRequired(c *gin.Context) {
	var body setFollowApprovalRequest
	if !bindJSON(c, &body) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.SetFollowApprovalRequired(ctx, &userpb.SetFollowApprovalRequest{
		UserId:   currentUserID(c),
		Required: body.Required,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createTopic(c *gin.Context) {
	var req createTopicRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if req.Publish {
		if !h.ensureCurrentUserCanPost(c, ctx) {
			return
		}
	} else if !h.ensureCurrentUserCanCreateContent(c, ctx) {
		return
	}
	if req.Publish && topicRequiresMembership(req.Type, req.BountyScore) {
		if !h.ensureCurrentUserHasMembershipBountyEntitlement(c, ctx) {
			return
		}
	}
	resp, err := h.clients.Content.CreateTopic(ctx, &contentpb.CreateTopicRequest{
		Slug: req.Slug, Type: req.Type, Title: req.Title, Body: req.Body, Tags: req.Tags, AuthorId: currentUserID(c), CategoryId: req.CategoryID.Int64(), ChannelId: req.ChannelID.Int64(), BountyScore: req.BountyScore, Poll: topicPollInput(req.Poll),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if req.Publish && resp.GetTopic() != nil {
		resp, err = h.clients.Content.PublishTopic(ctx, &contentpb.TopicIDRequest{Id: resp.GetTopic().GetId()})
		if err != nil {
			writeRPCError(c, err)
			return
		}
	}
	response.Success(c, resp)
}

func topicRequiresMembership(topicType string, bountyScore int64) bool {
	return strings.EqualFold(strings.TrimSpace(topicType), "qa") && bountyScore > 0
}

func topicBountyChangeRequiresMembership(topic *contentpb.TopicInfo, bountyScore int64) bool {
	if topic == nil || topic.GetStatus() != contentStatusPublished {
		return false
	}
	currentBounty := topic.GetBountyScore()
	return topicRequiresMembership(topic.GetType(), currentBounty) || topicRequiresMembership(topic.GetType(), bountyScore)
}

func (h *Handler) updateTopic(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req updateTopicRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	topic, ok := h.requireTopicOwner(c, ctx, id)
	if !ok {
		return
	}
	if topic.GetStatus() == contentStatusPublished {
		if !h.ensureCurrentUserCanPost(c, ctx) {
			return
		}
	} else if !h.ensureCurrentUserCanCreateContent(c, ctx) {
		return
	}
	if topicBountyChangeRequiresMembership(topic, req.BountyScore) {
		if !h.ensureCurrentUserHasMembershipBountyEntitlement(c, ctx) {
			return
		}
	}
	resp, err := h.clients.Content.UpdateTopic(ctx, &contentpb.UpdateTopicRequest{Id: id, Title: req.Title, Body: req.Body, Tags: req.Tags, CategoryId: req.CategoryID.Int64(), ChannelId: req.ChannelID.Int64(), BountyScore: req.BountyScore, Poll: topicPollInput(req.Poll)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) publishTopic(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	topic, ok := h.requireTopicOwner(c, ctx, id)
	if !ok {
		return
	}
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	if topicRequiresMembership(topic.GetType(), topic.GetBountyScore()) {
		if !h.ensureCurrentUserHasMembershipBountyEntitlement(c, ctx) {
			return
		}
	}
	resp, err := h.clients.Content.PublishTopic(ctx, &contentpb.TopicIDRequest{Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) archiveTopic(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, ok := h.requireTopicOwner(c, ctx, id); !ok {
		return
	}
	resp, err := h.clients.Content.ArchiveTopic(ctx, &contentpb.TopicIDRequest{Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ensureCurrentUserHasMembershipBountyEntitlement(c *gin.Context, ctx context.Context) bool {
	return h.ensureCurrentUserHasMembershipEntitlement(c, ctx, membershipBountyRequiredMessage)
}

func (h *Handler) ensureCurrentUserHasMembershipPaidAttachmentEntitlement(c *gin.Context, ctx context.Context) bool {
	return h.ensureCurrentUserHasMembershipEntitlement(c, ctx, paidAttachmentMembershipRequiredMessage)
}

func (h *Handler) ensureCurrentUserHasMembershipEntitlement(c *gin.Context, ctx context.Context, requiredMessage string) bool {
	allowed, err := h.userHasActiveDigitalEntitlement(ctx, currentUserID(c), digitalEntitlementGrantTypeMembership, "")
	if err != nil {
		writeRPCError(c, err)
		return false
	}
	if !allowed {
		writeError(c, http.StatusForbidden, requiredMessage, "permission_denied")
		return false
	}
	return true
}

func (h *Handler) getTopic(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}, TrackView: true, ViewerUserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetTopic() == nil || resp.GetTopic().GetStatus() != contentStatusPublished {
		writeError(c, http.StatusNotFound, "topic not found", "not_found")
		return
	}
	response.Success(c, resp)
}

func topicPollInput(input *topicPollRequest) *contentpb.TopicPollInput {
	if input == nil {
		return nil
	}
	return &contentpb.TopicPollInput{Enabled: input.Enabled, Multiple: input.Multiple, Choices: input.Choices, ExpiresAt: input.ExpiresAt.Int64()}
}

func (h *Handler) voteTopicPoll(c *gin.Context) {
	topicID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req voteTopicPollRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.VoteTopicPoll(ctx, &contentpb.VoteTopicPollRequest{TopicId: topicID, UserId: currentUserID(c), Choices: req.Choices})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getEditableTopic(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	topic, ok := h.requireTopicOwner(c, ctx, id)
	if !ok {
		return
	}
	response.Success(c, gin.H{"topic": topic})
}

func (h *Handler) listTopics(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{
		Status:     contentStatusPublished,
		Type:       c.Query("type"),
		Tag:        c.Query("tag"),
		AuthorId:   queryInt64(c, "author_id", 0),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
		CategoryId: queryInt64(c, "category_id", 0),
		ChannelId:  queryInt64(c, "channel_id", 0),
		Sort:       c.Query("sort"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listCurrentUserTopics(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{
		Status:     queryInt32(c, "status", 0),
		Type:       c.Query("type"),
		Tag:        c.Query("tag"),
		AuthorId:   currentUserID(c),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
		CategoryId: queryInt64(c, "category_id", 0),
		ChannelId:  queryInt64(c, "channel_id", 0),
		Sort:       c.Query("sort"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createChannel(c *gin.Context) {
	var req channelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.CreateChannel(ctx, &contentpb.CreateChannelRequest{
		OwnerId: currentUserID(c), CategoryId: req.CategoryID.Int64(), Name: req.Name, Description: req.Description, Color: req.Color,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateChannel(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req channelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.UpdateChannel(ctx, &contentpb.UpdateChannelRequest{
		Id: id, ActorId: currentUserID(c), CategoryId: req.CategoryID.Int64(), Name: req.Name, Description: req.Description, Color: req.Color,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) archiveChannel(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ArchiveChannel(ctx, &contentpb.ArchiveChannelRequest{Id: id, ActorId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getChannel(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.GetChannel(ctx, &contentpb.GetChannelRequest{Id: id, ViewerUserId: currentUserID(c), IncludeArchived: true})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listChannels(c *gin.Context) {
	h.listChannelsBy(c, 0, 0, 0, false, false)
}

func (h *Handler) listFeaturedChannels(c *gin.Context) {
	h.listChannelsBy(c, 0, 0, 0, true, false)
}

func (h *Handler) listOwnedChannels(c *gin.Context) {
	h.listChannelsBy(c, currentUserID(c), 0, 0, false, true)
}

func (h *Handler) listFollowedChannels(c *gin.Context) {
	h.listChannelsBy(c, 0, currentUserID(c), 0, false, false)
}

func (h *Handler) listFavoriteChannels(c *gin.Context) {
	h.listChannelsBy(c, 0, 0, currentUserID(c), false, false)
}

func (h *Handler) listChannelsBy(c *gin.Context, ownerID, followerUserID, favoritedUserID int64, featured, includeArchived bool) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListChannels(ctx, &contentpb.ListChannelsRequest{
		Query:           c.Query("q"),
		CategoryId:      queryInt64(c, "category_id", 0),
		Uncategorized:   queryBool(c, "uncategorized", false),
		OwnerId:         ownerID,
		FollowerUserId:  followerUserID,
		FavoritedUserId: favoritedUserID,
		ViewerUserId:    currentUserID(c),
		Featured:        featured,
		IncludeArchived: includeArchived,
		Limit:           queryInt32(c, "limit", 20),
		Offset:          queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listChannelCategories(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListChannelCategories(ctx, &contentpb.ListChannelCategoriesRequest{})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listChannelTopics(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.Content.GetChannel(ctx, &contentpb.GetChannelRequest{Id: id, ViewerUserId: currentUserID(c), IncludeArchived: true}); err != nil {
		writeRPCError(c, err)
		return
	}
	resp, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{
		Status: contentStatusPublished, ChannelId: id, Limit: queryInt32(c, "limit", 20), Offset: queryInt32(c, "offset", 0), Sort: c.Query("sort"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) followChannel(c *gin.Context) {
	h.channelUserAction(c, func(ctx context.Context, req *contentpb.ChannelUserRequest) (*contentpb.ChannelActionResponse, error) {
		return h.clients.Content.FollowChannel(ctx, req)
	})
}

func (h *Handler) unfollowChannel(c *gin.Context) {
	h.channelUserAction(c, func(ctx context.Context, req *contentpb.ChannelUserRequest) (*contentpb.ChannelActionResponse, error) {
		return h.clients.Content.UnfollowChannel(ctx, req)
	})
}

func (h *Handler) favoriteChannel(c *gin.Context) {
	h.channelUserAction(c, func(ctx context.Context, req *contentpb.ChannelUserRequest) (*contentpb.ChannelActionResponse, error) {
		return h.clients.Content.FavoriteChannel(ctx, req)
	})
}

func (h *Handler) unfavoriteChannel(c *gin.Context) {
	h.channelUserAction(c, func(ctx context.Context, req *contentpb.ChannelUserRequest) (*contentpb.ChannelActionResponse, error) {
		return h.clients.Content.UnfavoriteChannel(ctx, req)
	})
}

func (h *Handler) channelUserAction(c *gin.Context, action func(context.Context, *contentpb.ChannelUserRequest) (*contentpb.ChannelActionResponse, error)) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := action(ctx, &contentpb.ChannelUserRequest{ChannelId: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listCategories(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListCategories(ctx, &contentpb.ListCategoriesRequest{
		Status: categoryStatusEnabled,
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getCategory(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.GetCategory(ctx, &contentpb.CategoryIDRequest{Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetCategory() == nil || resp.GetCategory().GetStatus() != categoryStatusEnabled {
		writeError(c, http.StatusNotFound, "category not found", "not_found")
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listLinks(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListLinks(ctx, &adminpb.ListLinksRequest{
		Status: 2,
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listTasks(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	tasks, err := h.listEnabledClaimableTasks(ctx)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := pageTasks(tasks, queryInt32(c, "limit", 20), queryInt32(c, "offset", 0))
	response.Success(c, gin.H{"items": toTaskViews(items, nil), "total": len(tasks)})
}

func (h *Handler) listCurrentUserTasks(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	tasks, err := h.listEnabledClaimableTasks(ctx)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := pageTasks(tasks, queryInt32(c, "limit", 20), queryInt32(c, "offset", 0))
	views := make([]taskView, 0, len(items))
	if len(items) > 0 {
		claimStatusInputs := make([]*creditpb.TaskClaimStatusInput, 0, len(items))
		for _, task := range items {
			claimStatusInputs = append(claimStatusInputs, &creditpb.TaskClaimStatusInput{
				TaskId:  task.GetId(),
				TaskKey: task.GetKey(),
			})
		}
		claimStatuses, err := h.clients.Credit.ListTaskClaimStatuses(ctx, &creditpb.ListTaskClaimStatusesRequest{
			UserId: currentUserID(c),
			Tasks:  claimStatusInputs,
		})
		if err != nil {
			writeRPCError(c, err)
			return
		}
		if len(claimStatuses.GetItems()) != len(items) {
			writeRPCError(c, status.Error(codes.Internal, "credit task status response mismatch"))
			return
		}
		for i, task := range items {
			claimStatus := claimStatuses.GetItems()[i]
			if claimStatus == nil || claimStatus.GetTaskId() != task.GetId() || claimStatus.GetTaskKey() != task.GetKey() {
				writeRPCError(c, status.Error(codes.Internal, "credit task status response mismatch"))
				return
			}
			views = append(views, toTaskView(task, claimStatus))
		}
	}
	response.Success(c, gin.H{"items": views, "total": len(tasks)})
}

func (h *Handler) claimTask(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	tasks, err := h.listEnabledClaimableTasks(ctx)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	task := taskByID(tasks, id)
	if task == nil {
		writeError(c, http.StatusNotFound, "task not found", "not_found")
		return
	}
	claim, err := h.clients.Credit.ClaimTask(ctx, &creditpb.ClaimTaskRequest{
		UserId:        currentUserID(c),
		TaskId:        task.GetId(),
		TaskKey:       task.GetKey(),
		RewardCredits: task.GetRewardPoints(),
		TaskTitle:     task.GetTitle(),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"task": toTaskView(task, claim.GetStatus()), "claim": claim})
}

func (h *Handler) listEnabledClaimableTasks(ctx context.Context) ([]*adminpb.TaskInfo, error) {
	resp, err := h.clients.Admin.ListTasks(ctx, &adminpb.ListTasksRequest{Status: 2, Limit: 100})
	if err != nil {
		return nil, err
	}
	tasks := make([]*adminpb.TaskInfo, 0, len(resp.GetItems()))
	for _, task := range resp.GetItems() {
		if isClaimableTask(task) {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func isClaimableTask(task *adminpb.TaskInfo) bool {
	if task == nil || task.GetRewardPoints() <= 0 {
		return false
	}
	switch task.GetKey() {
	case taskKeyDailyCheckIn, taskKeyFirstTopic, taskKeyFirstComment:
		return true
	default:
		return false
	}
}

func pageTasks(tasks []*adminpb.TaskInfo, limit, offset int32) []*adminpb.TaskInfo {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	start := int(offset)
	if start >= len(tasks) {
		return []*adminpb.TaskInfo{}
	}
	end := start + int(limit)
	if end > len(tasks) {
		end = len(tasks)
	}
	return tasks[start:end]
}

func taskByID(tasks []*adminpb.TaskInfo, id int64) *adminpb.TaskInfo {
	for _, task := range tasks {
		if task.GetId() == id {
			return task
		}
	}
	return nil
}

func toTaskViews(tasks []*adminpb.TaskInfo, claimStatus *creditpb.TaskClaimStatus) []taskView {
	items := make([]taskView, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, toTaskView(task, claimStatus))
	}
	return items
}

func toTaskView(task *adminpb.TaskInfo, claimStatus *creditpb.TaskClaimStatus) taskView {
	if task == nil {
		return taskView{}
	}
	item := taskView{
		ID:           task.GetId(),
		Key:          task.GetKey(),
		Title:        task.GetTitle(),
		Description:  task.GetDescription(),
		RewardPoints: task.GetRewardPoints(),
		Status:       task.GetStatus(),
		Sort:         task.GetSort(),
		CreatedAt:    task.GetCreatedAt(),
		UpdatedAt:    task.GetUpdatedAt(),
	}
	if claimStatus == nil {
		return item
	}
	item.Cycle = claimStatus.GetCycle()
	item.Completed = claimStatus.GetCompleted()
	item.Claimed = claimStatus.GetClaimed()
	item.Claimable = item.Completed && !item.Claimed
	return item
}

func (h *Handler) createTopicComment(c *gin.Context) {
	h.createEntityComment(c, "topic")
}

func (h *Handler) listTopicComments(c *gin.Context) {
	h.listEntityComments(c, "topic")
}

func (h *Handler) acceptTopicComment(c *gin.Context) {
	topicID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	commentID, ok := pathInt64(c, "commentId")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, ok := h.requireTopicOwner(c, ctx, topicID); !ok {
		return
	}
	resp, err := h.clients.Content.AcceptTopicComment(ctx, &contentpb.AcceptTopicCommentRequest{TopicId: topicID, CommentId: commentID, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) unacceptTopicComment(c *gin.Context) {
	topicID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	commentID, ok := pathInt64(c, "commentId")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, ok := h.requireTopicOwner(c, ctx, topicID); !ok {
		return
	}
	resp, err := h.clients.Content.UnacceptTopicComment(ctx, &contentpb.UnacceptTopicCommentRequest{TopicId: topicID, CommentId: commentID, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) likeTopic(c *gin.Context)       { h.reactEntity(c, "topic", "like") }
func (h *Handler) unlikeTopic(c *gin.Context)     { h.reactEntity(c, "topic", "unlike") }
func (h *Handler) favoriteTopic(c *gin.Context)   { h.reactEntity(c, "topic", "favorite") }
func (h *Handler) unfavoriteTopic(c *gin.Context) { h.reactEntity(c, "topic", "unfavorite") }

func (h *Handler) getTopicReactions(c *gin.Context) {
	h.getEntityReactions(c, "topic")
}

func (h *Handler) reportTopic(c *gin.Context) {
	h.reportEntity(c, "topic")
}

func (h *Handler) createArticle(c *gin.Context) {
	var req createArticleRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if req.Publish {
		if !h.ensureCurrentUserCanPost(c, ctx) {
			return
		}
	} else if !h.ensureCurrentUserCanCreateContent(c, ctx) {
		return
	}
	resp, err := h.clients.Content.CreateArticle(ctx, &contentpb.CreateArticleRequest{
		Slug: req.Slug, Title: req.Title, Summary: req.Summary, Body: req.Body, CoverUrl: req.CoverURL, Tags: req.Tags, AuthorId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if req.Publish && resp.GetArticle() != nil {
		resp, err = h.clients.Content.PublishArticle(ctx, &contentpb.ArticleIDRequest{Id: resp.GetArticle().GetId()})
		if err != nil {
			writeRPCError(c, err)
			return
		}
	}
	response.Success(c, resp)
}

func (h *Handler) updateArticle(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req updateArticleRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	article, ok := h.requireArticleOwner(c, ctx, id)
	if !ok {
		return
	}
	if article.GetStatus() == contentStatusPublished {
		if !h.ensureCurrentUserCanPost(c, ctx) {
			return
		}
	} else if !h.ensureCurrentUserCanCreateContent(c, ctx) {
		return
	}
	resp, err := h.clients.Content.UpdateArticle(ctx, &contentpb.UpdateArticleRequest{Id: id, Title: req.Title, Summary: req.Summary, Body: req.Body, CoverUrl: req.CoverURL, Tags: req.Tags})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) publishArticle(c *gin.Context) { h.articleStatus(c, "publish") }
func (h *Handler) hideArticle(c *gin.Context)    { h.articleStatus(c, "hide") }
func (h *Handler) archiveArticle(c *gin.Context) { h.articleStatus(c, "archive") }

func (h *Handler) articleStatus(c *gin.Context, action string) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, ok := h.requireArticleOwner(c, ctx, id); !ok {
		return
	}
	if action == "publish" && !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	req := &contentpb.ArticleIDRequest{Id: id}
	var (
		resp *contentpb.ArticleResponse
		err  error
	)
	switch action {
	case "publish":
		resp, err = h.clients.Content.PublishArticle(ctx, req)
	case "hide":
		resp, err = h.clients.Content.HideArticle(ctx, req)
	default:
		resp, err = h.clients.Content.ArchiveArticle(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getArticle(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}, TrackView: true})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetArticle() == nil || resp.GetArticle().GetStatus() != contentStatusPublished {
		writeError(c, http.StatusNotFound, "article not found", "not_found")
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getEditableArticle(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	article, ok := h.requireArticleOwner(c, ctx, id)
	if !ok {
		return
	}
	response.Success(c, gin.H{"article": article})
}

func (h *Handler) listArticles(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{
		Status: contentStatusPublished, Tag: c.Query("tag"), AuthorId: queryInt64(c, "author_id", 0), Limit: queryInt32(c, "limit", 20), Offset: queryInt32(c, "offset", 0), Sort: c.Query("sort"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listCurrentUserArticles(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{
		Status:   queryInt32(c, "status", 0),
		Tag:      c.Query("tag"),
		AuthorId: currentUserID(c),
		Limit:    queryInt32(c, "limit", 20),
		Offset:   queryInt32(c, "offset", 0),
		Sort:     c.Query("sort"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) feedArticles(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	limit := normalizeFeedLimit(queryInt32(c, "limit", 20))
	offset := normalizeFeedOffset(queryInt32(c, "offset", 0))
	req := &feedpb.ListFeedRequest{Limit: limit, Offset: offset}
	var (
		resp *feedpb.FeedListResponse
		err  error
	)
	switch strings.ToLower(strings.TrimSpace(c.Query("sort"))) {
	case "hot":
		resp, err = h.clients.Feed.ListHot(ctx, req)
	case "active", "recent-replies", "updated":
		resp, err = h.clients.Feed.ListActive(ctx, req)
	case "follow", "following":
		var identity authIdentity
		identity, err = h.authIdentityFromRequest(c)
		if err != nil {
			writeAuthenticationError(c, err)
			return
		}
		resp, err = h.listFollowingFeed(ctx, identity.userID, limit, offset)
	default:
		resp, err = h.clients.Feed.ListLatest(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listFollowingFeed(ctx context.Context, userID int64, limit, offset int32) (*feedpb.FeedListResponse, error) {
	followedIDs, err := h.followingIDSet(ctx, userID)
	if err != nil {
		return nil, err
	}
	hiddenIDs, err := h.hiddenUserIDSet(ctx, userID)
	if err != nil {
		return nil, err
	}
	for hiddenID := range hiddenIDs {
		delete(followedIDs, hiddenID)
	}
	if len(followedIDs) == 0 {
		return &feedpb.FeedListResponse{Items: []*feedpb.FeedItem{}}, nil
	}
	authorIDs := make([]int64, 0, len(followedIDs))
	for authorID := range followedIDs {
		authorIDs = append(authorIDs, authorID)
	}
	return h.clients.Feed.ListLatest(ctx, &feedpb.ListFeedRequest{Limit: limit, Offset: offset, AuthorIds: authorIDs})
}

func (h *Handler) followingIDSet(ctx context.Context, userID int64) (map[int64]struct{}, error) {
	const pageSize int32 = 100
	ids := make(map[int64]struct{})
	for page := int32(1); ; page++ {
		resp, err := h.clients.User.ListFollowing(ctx, &userpb.ListFollowsRequest{UserId: userID, Page: page, PageSize: pageSize})
		if err != nil {
			return nil, err
		}
		items := resp.GetItems()
		for _, item := range items {
			if item != nil && item.GetId() > 0 {
				ids[item.GetId()] = struct{}{}
			}
		}
		total := resp.GetTotal()
		if len(items) < int(pageSize) || (total > 0 && int64(page)*int64(pageSize) >= total) {
			break
		}
	}
	return ids, nil
}

func (h *Handler) hiddenUserIDSet(ctx context.Context, userID int64) (map[int64]struct{}, error) {
	ids := make(map[int64]struct{})
	if h.clients.UserSafety == nil {
		return ids, nil
	}
	load := func(blocked bool) error {
		const pageSize int32 = 100
		for page := int32(1); ; page++ {
			req := &userpb.ListUserRelationsRequest{ActorId: userID, Page: page, PageSize: pageSize}
			var (
				resp *userpb.UserListResponse
				err  error
			)
			if blocked {
				resp, err = h.clients.UserSafety.ListBlockedUsers(ctx, req)
			} else {
				resp, err = h.clients.UserSafety.ListMutedUsers(ctx, req)
			}
			if err != nil {
				return err
			}
			items := resp.GetItems()
			for _, item := range items {
				if item != nil && item.GetId() > 0 {
					ids[item.GetId()] = struct{}{}
				}
			}
			if len(items) < int(pageSize) || (resp.GetTotal() > 0 && int64(page)*int64(pageSize) >= resp.GetTotal()) {
				break
			}
		}
		return nil
	}
	if err := load(true); err != nil {
		return nil, err
	}
	if err := load(false); err != nil {
		return nil, err
	}
	return ids, nil
}

func normalizeFeedLimit(value int32) int32 {
	if value <= 0 {
		return 20
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeFeedOffset(value int32) int32 {
	if value < 0 {
		return 0
	}
	return value
}

func (h *Handler) createComment(c *gin.Context) {
	h.createEntityComment(c, "article")
}

func (h *Handler) createEntityComment(c *gin.Context, entityType string) {
	entityID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req createCommentRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	if !h.requirePublishedContentTarget(c, ctx, entityType, entityID) {
		return
	}
	resp, err := h.clients.Comment.CreateComment(ctx, &commentpb.CreateCommentRequest{EntityType: entityType, EntityId: entityID, ParentId: req.ParentID.Int64(), AuthorId: currentUserID(c), Content: req.Content})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) requirePublishedContentTarget(c *gin.Context, ctx context.Context, entityType string, entityID int64) bool {
	switch entityType {
	case "topic":
		resp, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: entityID}})
		if err != nil {
			writeRPCError(c, err)
			return false
		}
		if resp.GetTopic() == nil || resp.GetTopic().GetStatus() != contentStatusPublished {
			writeError(c, http.StatusNotFound, "topic not found", "not_found")
			return false
		}
		return true
	case "article":
		resp, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: entityID}})
		if err != nil {
			writeRPCError(c, err)
			return false
		}
		if resp.GetArticle() == nil || resp.GetArticle().GetStatus() != contentStatusPublished {
			writeError(c, http.StatusNotFound, "article not found", "not_found")
			return false
		}
		return true
	default:
		writeError(c, http.StatusBadRequest, "invalid comment target", "invalid_argument")
		return false
	}
}

func (h *Handler) listComments(c *gin.Context) {
	h.listEntityComments(c, "article")
}

func (h *Handler) listEntityComments(c *gin.Context, entityType string) {
	entityID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.requirePublishedContentTarget(c, ctx, entityType, entityID) {
		return
	}
	resp, err := h.clients.Comment.ListComments(ctx, &commentpb.ListCommentsRequest{EntityType: entityType, EntityId: entityID, Page: queryInt32(c, "page", 1), PageSize: queryInt32(c, "page_size", 20)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getComment(c *gin.Context) {
	commentID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Comment.GetComment(ctx, &commentpb.GetCommentRequest{Id: commentID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetComment() == nil || resp.GetComment().GetStatus() != 1 {
		writeError(c, http.StatusNotFound, "comment not found", "not_found")
		return
	}
	if !h.requirePublishedContentTarget(c, ctx, resp.GetComment().GetEntityType(), resp.GetComment().GetEntityId()) {
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listReplies(c *gin.Context) {
	rootID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	root, ok := h.requireVisibleComment(c, ctx, rootID)
	if !ok {
		return
	}
	if !h.requirePublishedContentTarget(c, ctx, root.GetEntityType(), root.GetEntityId()) {
		return
	}
	resp, err := h.clients.Comment.ListReplies(ctx, &commentpb.ListRepliesRequest{RootId: rootID, Page: queryInt32(c, "page", 1), PageSize: queryInt32(c, "page_size", 20)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) requireVisibleComment(c *gin.Context, ctx context.Context, commentID int64) (*commentpb.CommentInfo, bool) {
	resp, err := h.clients.Comment.GetComment(ctx, &commentpb.GetCommentRequest{Id: commentID})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	comment := resp.GetComment()
	if comment == nil || comment.GetStatus() != 1 {
		writeError(c, http.StatusNotFound, "comment not found", "not_found")
		return nil, false
	}
	return comment, true
}

func (h *Handler) deleteComment(c *gin.Context) {
	commentID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Comment.DeleteComment(ctx, &commentpb.DeleteCommentRequest{Id: commentID, ActorId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) likeArticle(c *gin.Context)       { h.reactArticle(c, "like") }
func (h *Handler) unlikeArticle(c *gin.Context)     { h.reactArticle(c, "unlike") }
func (h *Handler) favoriteArticle(c *gin.Context)   { h.reactArticle(c, "favorite") }
func (h *Handler) unfavoriteArticle(c *gin.Context) { h.reactArticle(c, "unfavorite") }

func (h *Handler) reactArticle(c *gin.Context, action string) {
	h.reactEntity(c, "article", action)
}

func (h *Handler) reactEntity(c *gin.Context, entityType string, action string) {
	entityID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	switch action {
	case "like", "favorite":
		if !h.requirePublishedContentTarget(c, ctx, entityType, entityID) {
			return
		}
	}
	req := &reactionpb.ReactRequest{Entity: &reactionpb.EntityRef{EntityType: entityType, EntityId: entityID}, UserId: currentUserID(c)}
	var (
		resp *reactionpb.ReactResponse
		err  error
	)
	switch action {
	case "like":
		resp, err = h.clients.Reaction.Like(ctx, req)
	case "unlike":
		resp, err = h.clients.Reaction.Unlike(ctx, req)
	case "favorite":
		resp, err = h.clients.Reaction.Favorite(ctx, req)
	default:
		resp, err = h.clients.Reaction.Unfavorite(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getArticleReactions(c *gin.Context) {
	h.getEntityReactions(c, "article")
}

func (h *Handler) getEntityReactions(c *gin.Context, entityType string) {
	entityID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.requirePublishedContentTarget(c, ctx, entityType, entityID) {
		return
	}
	resp, err := h.clients.Reaction.GetCounts(ctx, &reactionpb.EntityRequest{Entity: &reactionpb.EntityRef{EntityType: entityType, EntityId: entityID}})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) reportArticle(c *gin.Context) {
	h.reportEntity(c, "article")
}

func (h *Handler) reportComment(c *gin.Context) {
	h.reportEntity(c, "comment")
}

func (h *Handler) reportEntity(c *gin.Context, entityType string) {
	entityID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req submitReportRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.requireReportTarget(c, ctx, entityType, entityID) {
		return
	}
	resp, err := h.clients.Reaction.SubmitReport(ctx, &reactionpb.SubmitReportRequest{
		Entity:      &reactionpb.EntityRef{EntityType: entityType, EntityId: entityID},
		ReporterId:  currentUserID(c),
		Reason:      req.Reason,
		Description: req.Description,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) requireReportTarget(c *gin.Context, ctx context.Context, entityType string, entityID int64) bool {
	switch entityType {
	case "article", "topic":
		return h.requirePublishedContentTarget(c, ctx, entityType, entityID)
	case "comment":
		comment, ok := h.requireVisibleComment(c, ctx, entityID)
		if !ok {
			return false
		}
		return h.requirePublishedContentTarget(c, ctx, comment.GetEntityType(), comment.GetEntityId())
	default:
		writeError(c, http.StatusBadRequest, "invalid report target", "invalid_argument")
		return false
	}
}

func (h *Handler) listNotifications(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.ListNotifications(ctx, &notificationpb.ListNotificationsRequest{
		UserId:     currentUserID(c),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
		UnreadOnly: queryBool(c, "unread_only", false),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]notificationPayloadView, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toNotificationPayload(item))
	}
	response.Success(c, gin.H{"items": items, "total": resp.GetTotal(), "unread_count": resp.GetUnreadCount()})
}

func (h *Handler) countUnreadNotifications(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.CountUnread(ctx, &notificationpb.CountUnreadRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) markNotificationRead(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.MarkRead(ctx, &notificationpb.MarkReadRequest{UserId: currentUserID(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) markAllNotificationsRead(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.MarkAllRead(ctx, &notificationpb.MarkAllReadRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getNotificationPreferences(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.GetPreferences(ctx, &notificationpb.GetPreferencesRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

type updateNotificationPreferencesRequest struct {
	Items []struct {
		Type    string `json:"type"`
		Enabled bool   `json:"enabled"`
	} `json:"items"`
}

func (h *Handler) updateNotificationPreferences(c *gin.Context) {
	var req updateNotificationPreferencesRequest
	if !bindJSON(c, &req) {
		return
	}
	items := make([]*notificationpb.NotificationPreference, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, &notificationpb.NotificationPreference{Type: item.Type, Enabled: item.Enabled})
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Notification.UpdatePreferences(ctx, &notificationpb.UpdatePreferencesRequest{
		UserId: currentUserID(c),
		Items:  items,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listReports(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListReports(ctx, &adminpb.ListReportsRequest{
		Actor:      currentActor(c),
		Status:     queryInt32(c, "status", 0),
		EntityType: c.Query("entity_type"),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) auditReport(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req auditReportRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.AuditReport(ctx, &adminpb.AuditReportRequest{
		Actor:        currentActor(c),
		Id:           id,
		Status:       req.Status,
		AuditNote:    req.AuditNote,
		TargetAction: req.TargetAction,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listGovernanceUsers(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListUsers(ctx, &adminpb.ListUsersRequest{
		Actor:    currentActor(c),
		Query:    c.Query("query"),
		Status:   queryInt32(c, "status", 0),
		Page:     queryInt32(c, "page", 1),
		PageSize: queryInt32(c, "page_size", 20),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) muteUser(c *gin.Context) {
	h.updateUserStatus(c, true)
}

func (h *Handler) unmuteUser(c *gin.Context) {
	h.updateUserStatus(c, false)
}

func (h *Handler) updateUserStatus(c *gin.Context, muted bool) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &adminpb.UserStatusRequest{Actor: currentActor(c), UserId: id}
	var (
		resp *adminpb.UserResponse
		err  error
	)
	if muted {
		resp, err = h.clients.Admin.MuteUser(ctx, req)
	} else {
		resp, err = h.clients.Admin.UnmuteUser(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminArticles(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListArticles(ctx, &adminpb.ListArticlesRequest{
		Actor:    currentActor(c),
		Status:   queryInt32(c, "status", 0),
		Tag:      c.Query("tag"),
		AuthorId: queryInt64(c, "author_id", 0),
		Limit:    queryInt32(c, "limit", 20),
		Offset:   queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminChannels(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) > 100 {
		writeError(c, http.StatusBadRequest, "q must be at most 100 characters", "bad_request")
		return
	}
	categoryID := int64(0)
	if raw, exists := c.GetQuery("category_id"); exists {
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || value < 0 {
			writeError(c, http.StatusBadRequest, "category_id must be a non-negative integer", "bad_request")
			return
		}
		categoryID = value
	}
	archivedStatus, ok := adminChannelArchivedStatus(c)
	if !ok {
		return
	}
	limit := int32(20)
	if raw, exists := c.GetQuery("limit"); exists {
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
		if err != nil || value < 1 || value > 100 {
			writeError(c, http.StatusBadRequest, "limit must be between 1 and 100", "bad_request")
			return
		}
		limit = int32(value)
	}
	offset := int32(0)
	if raw, exists := c.GetQuery("offset"); exists {
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
		if err != nil || value < 0 {
			writeError(c, http.StatusBadRequest, "offset must be a non-negative integer", "bad_request")
			return
		}
		offset = int32(value)
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListChannels(ctx, &adminpb.ListChannelsRequest{
		Actor:          currentActor(c),
		Query:          query,
		CategoryId:     categoryID,
		ArchivedStatus: archivedStatus,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) setAdminChannelFeatured(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req setAdminChannelFeaturedRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Featured == nil {
		writeError(c, http.StatusBadRequest, "featured is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.SetChannelFeatured(ctx, &adminpb.ChannelFeaturedRequest{
		Actor: currentActor(c), Id: id, Featured: *req.Featured,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) setAdminChannelArchived(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req setAdminChannelArchivedRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Archived == nil {
		writeError(c, http.StatusBadRequest, "archived is required", "bad_request")
		return
	}
	permission := "governance:restore_channel"
	if *req.Archived {
		permission = "governance:archive_channel"
	}
	profile, ok := c.Get("admin_profile")
	adminProfile, validProfile := profile.(*adminpb.ProfileResponse)
	if !ok || !validProfile || !adminProfileHasPermission(adminProfile, permission) {
		writeError(c, http.StatusForbidden, "admin permission denied", "permission_denied")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.SetChannelArchived(ctx, &adminpb.ChannelArchivedRequest{
		Actor: currentActor(c), Id: id, Archived: *req.Archived,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func adminChannelArchivedStatus(c *gin.Context) (int32, bool) {
	raw, exists := c.GetQuery("archived_status")
	if !exists {
		return 0, true
	}
	raw = strings.TrimSpace(raw)
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < 0 || value > 2 {
		writeError(c, http.StatusBadRequest, "archived_status must be 0, 1, or 2", "bad_request")
		return 0, false
	}
	return int32(value), true
}

func (h *Handler) getAdminArticle(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminCategories(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListCategories(ctx, &adminpb.ListCategoriesRequest{
		Actor:  currentActor(c),
		Status: queryInt32(c, "status", 0),
		Limit:  queryInt32(c, "limit", 50),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminCategory(c *gin.Context) {
	var req upsertAdminCategoryRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateCategory(ctx, &adminpb.UpsertCategoryRequest{
		Actor:       currentActor(c),
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminCategory(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertAdminCategoryRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateCategory(ctx, &adminpb.UpsertCategoryRequest{
		Actor:       currentActor(c),
		Id:          id,
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteAdminCategory(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteCategory(ctx, &adminpb.CategoryIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminBadges(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListBadges(ctx, &adminpb.ListBadgesRequest{
		Actor:  currentActor(c),
		Status: queryInt32(c, "status", 0),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminBadge(c *gin.Context) {
	var req upsertAdminBadgeRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateBadge(ctx, &adminpb.UpsertBadgeRequest{
		Actor:       currentActor(c),
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		IconUrl:     req.IconURL,
		RuleType:    req.RuleType,
		RuleValue:   req.RuleValue,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminBadge(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertAdminBadgeRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateBadge(ctx, &adminpb.UpsertBadgeRequest{
		Actor:       currentActor(c),
		Id:          id,
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		IconUrl:     req.IconURL,
		RuleType:    req.RuleType,
		RuleValue:   req.RuleValue,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteAdminBadge(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteBadge(ctx, &adminpb.BadgeIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminLevels(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListLevels(ctx, &adminpb.ListLevelsRequest{
		Actor:  currentActor(c),
		Status: queryInt32(c, "status", 0),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminLevel(c *gin.Context) {
	var req upsertAdminLevelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateLevel(ctx, &adminpb.UpsertLevelRequest{
		Actor:       currentActor(c),
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		MinScore:    req.MinScore,
		MaxScore:    req.MaxScore,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminLevel(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertAdminLevelRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateLevel(ctx, &adminpb.UpsertLevelRequest{
		Actor:       currentActor(c),
		Id:          id,
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		MinScore:    req.MinScore,
		MaxScore:    req.MaxScore,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteAdminLevel(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteLevel(ctx, &adminpb.LevelIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminLinks(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListLinks(ctx, &adminpb.ListLinksRequest{
		Actor:  currentActor(c),
		Status: queryInt32(c, "status", 0),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminLink(c *gin.Context) {
	var req upsertAdminLinkRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateLink(ctx, &adminpb.UpsertLinkRequest{
		Actor:       currentActor(c),
		Key:         req.Key,
		Title:       req.Title,
		Url:         req.URL,
		Description: req.Description,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminLink(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertAdminLinkRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateLink(ctx, &adminpb.UpsertLinkRequest{
		Actor:       currentActor(c),
		Id:          id,
		Key:         req.Key,
		Title:       req.Title,
		Url:         req.URL,
		Description: req.Description,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteAdminLink(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteLink(ctx, &adminpb.LinkIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminTasks(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListTasks(ctx, &adminpb.ListTasksRequest{
		Actor:  currentActor(c),
		Status: queryInt32(c, "status", 0),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminTask(c *gin.Context) {
	var req upsertAdminTaskRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateTask(ctx, &adminpb.UpsertTaskRequest{
		Actor:        currentActor(c),
		Key:          req.Key,
		Title:        req.Title,
		Description:  req.Description,
		RewardPoints: req.RewardPoints,
		Status:       req.Status,
		Sort:         req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminTask(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertAdminTaskRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateTask(ctx, &adminpb.UpsertTaskRequest{
		Actor:        currentActor(c),
		Id:           id,
		Key:          req.Key,
		Title:        req.Title,
		Description:  req.Description,
		RewardPoints: req.RewardPoints,
		Status:       req.Status,
		Sort:         req.Sort,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteAdminTask(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteTask(ctx, &adminpb.TaskIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listForbiddenWords(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListForbiddenWords(ctx, &adminpb.ListForbiddenWordsRequest{
		Actor:  currentActor(c),
		Status: queryInt32(c, "status", 0),
		Query:  c.Query("query"),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createForbiddenWord(c *gin.Context) {
	var req upsertForbiddenWordRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateForbiddenWord(ctx, &adminpb.UpsertForbiddenWordRequest{
		Actor:       currentActor(c),
		Word:        req.Word,
		Scene:       req.Scene,
		Action:      req.Action,
		Replacement: req.Replacement,
		Description: req.Description,
		Status:      req.Status,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateForbiddenWord(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertForbiddenWordRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateForbiddenWord(ctx, &adminpb.UpsertForbiddenWordRequest{
		Actor:       currentActor(c),
		Id:          id,
		Word:        req.Word,
		Scene:       req.Scene,
		Action:      req.Action,
		Replacement: req.Replacement,
		Description: req.Description,
		Status:      req.Status,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteForbiddenWord(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteForbiddenWord(ctx, &adminpb.ForbiddenWordIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listSettings(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSettings(ctx, &adminpb.ListSettingsRequest{
		Actor:  currentActor(c),
		Group:  c.Query("group"),
		Status: queryInt32(c, "status", 0),
		Limit:  queryInt32(c, "limit", 100),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	settings := gin.H{}
	for _, item := range resp.GetItems() {
		settings[item.GetKey()] = item.GetValue()
	}
	response.Success(c, gin.H{"items": resp.GetItems(), "total": resp.GetTotal(), "settings": settings})
}

func (h *Handler) updateSetting(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	var req upsertSettingRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Key == "" {
		req.Key = key
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateSetting(ctx, &adminpb.UpsertSettingRequest{
		Actor:       currentActor(c),
		Key:         req.Key,
		Value:       req.Value,
		Group:       req.Group,
		ValueType:   req.ValueType,
		Description: req.Description,
		Status:      req.Status,
		ClearValue:  req.ClearValue,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listEmailLogs(c *gin.Context) {
	page, pageSize := systemPage(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListEmailLogs(ctx, &adminpb.ListEmailLogsRequest{
		Actor:  currentActor(c),
		Status: queryInt32(c, "status", 0),
		Query:  c.Query("query"),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, systemTablePayload(toHTTPEmailLogs(resp.GetItems()), resp.GetTotal(), page, pageSize))
}

func (h *Handler) hideAdminArticle(c *gin.Context) {
	h.updateAdminArticleStatus(c, "hide")
}

func (h *Handler) publishAdminArticle(c *gin.Context) {
	h.updateAdminArticleStatus(c, "publish")
}

func (h *Handler) archiveAdminArticle(c *gin.Context) {
	h.updateAdminArticleStatus(c, "archive")
}

func (h *Handler) updateAdminArticleStatus(c *gin.Context, action string) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &adminpb.ArticleStatusRequest{Actor: currentActor(c), Id: id}
	var (
		resp *adminpb.ArticleResponse
		err  error
	)
	switch action {
	case "publish":
		resp, err = h.clients.Admin.PublishArticle(ctx, req)
	case "hide":
		resp, err = h.clients.Admin.HideArticle(ctx, req)
	default:
		resp, err = h.clients.Admin.ArchiveArticle(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminTopics(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListTopics(ctx, &adminpb.ListTopicsRequest{
		Actor:      currentActor(c),
		Status:     queryInt32(c, "status", 0),
		Type:       c.Query("type"),
		Tag:        c.Query("tag"),
		AuthorId:   queryInt64(c, "author_id", 0),
		CategoryId: queryInt64(c, "category_id", 0),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getAdminTopic(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) hideAdminTopic(c *gin.Context) {
	h.updateAdminTopicStatus(c, "hide")
}

func (h *Handler) publishAdminTopic(c *gin.Context) {
	h.updateAdminTopicStatus(c, "publish")
}

func (h *Handler) archiveAdminTopic(c *gin.Context) {
	h.updateAdminTopicStatus(c, "archive")
}

func (h *Handler) updateAdminTopicStatus(c *gin.Context, action string) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	req := &adminpb.TopicStatusRequest{Actor: currentActor(c), Id: id}
	var (
		resp *adminpb.TopicResponse
		err  error
	)
	switch action {
	case "publish":
		resp, err = h.clients.Admin.PublishTopic(ctx, req)
	case "hide":
		resp, err = h.clients.Admin.HideTopic(ctx, req)
	default:
		resp, err = h.clients.Admin.ArchiveTopic(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminComments(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListComments(ctx, &adminpb.ListCommentsRequest{
		Actor:      currentActor(c),
		EntityType: c.Query("entity_type"),
		EntityId:   queryInt64(c, "entity_id", 0),
		AuthorId:   queryInt64(c, "author_id", 0),
		Status:     queryInt32(c, "status", -1),
		Page:       queryInt32(c, "page", 1),
		PageSize:   queryInt32(c, "page_size", 20),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getAdminComment(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Comment.GetComment(ctx, &commentpb.GetCommentRequest{Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) hideAdminComment(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.HideComment(ctx, &adminpb.CommentStatusRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) restoreAdminComment(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.RestoreComment(ctx, &adminpb.CommentStatusRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminUsers(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListAdminUsers(ctx, &adminpb.ListAdminUsersRequest{
		Actor:  currentActor(c),
		Query:  c.Query("query"),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminUser(c *gin.Context) {
	var req createAdminUserRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateAdminUser(ctx, &adminpb.CreateAdminUserRequest{
		Actor:    currentActor(c),
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
		Nickname: req.Nickname,
		RoleKeys: req.RoleKeys,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminRoles(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListRoles(ctx, &adminpb.ListRolesRequest{Actor: currentActor(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) assignAdminRoles(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req assignRolesRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.AssignRoles(ctx, &adminpb.AssignRolesRequest{
		Actor:    currentActor(c),
		UserId:   id,
		RoleKeys: req.RoleKeys,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getCreditBalance(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listCreditLedger(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.ListLedger(ctx, &creditpb.ListLedgerRequest{
		UserId: currentUserID(c),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listCreditLeaderboard(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.ListLeaderboard(ctx, &creditpb.ListLeaderboardRequest{
		Limit: queryInt32(c, "limit", 10),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	entries := resp.GetItems()
	if len(entries) == 0 {
		response.Success(c, gin.H{"items": []creditLeaderboardView{}})
		return
	}
	userIDs := make([]int64, 0, len(entries))
	seenUserIDs := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		userID := entry.GetUserId()
		if userID <= 0 {
			continue
		}
		if _, exists := seenUserIDs[userID]; exists {
			continue
		}
		seenUserIDs[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	if len(userIDs) == 0 {
		response.Success(c, gin.H{"items": []creditLeaderboardView{}})
		return
	}
	users, err := h.clients.User.ListUsers(ctx, &userpb.ListUsersRequest{
		Ids:      userIDs,
		Status:   userStatusActive,
		Page:     1,
		PageSize: int32(len(userIDs)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.sanitizeUserProfileThemes(ctx, users.GetItems())
	usersByID := make(map[int64]*userpb.UserInfo, len(users.GetItems()))
	for _, user := range users.GetItems() {
		if user != nil && user.GetId() > 0 {
			usersByID[user.GetId()] = user
		}
	}
	items := make([]creditLeaderboardView, 0, len(entries))
	for _, entry := range entries {
		user := usersByID[entry.GetUserId()]
		if user == nil {
			continue
		}
		items = append(items, creditLeaderboardView{
			Rank:   entry.GetRank(),
			UserID: entry.GetUserId(),
			Total:  entry.GetTotal(),
			User: creditLeaderboardUserView{
				ID:            user.GetId(),
				Username:      user.GetUsername(),
				Nickname:      user.GetNickname(),
				AvatarURL:     user.GetAvatarUrl(),
				BackgroundURL: user.GetBackgroundUrl(),
				ProfileTheme:  user.GetProfileTheme(),
			},
		})
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) getCheckInStatus(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.GetCheckInStatus(ctx, &creditpb.GetCheckInStatusRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) checkIn(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.CheckIn(ctx, &creditpb.CheckInRequest{UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getAdminUserCreditBalance(c *gin.Context) {
	userID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.GetBalance(ctx, &creditpb.GetBalanceRequest{UserId: userID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminUserCreditLedger(c *gin.Context) {
	userID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.ListLedger(ctx, &creditpb.ListLedgerRequest{
		UserId: userID,
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) adjustAdminUserCredits(c *gin.Context) {
	userID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminAdjustCreditsRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Delta == 0 {
		writeError(c, http.StatusBadRequest, "credit delta must not be zero", "bad_request")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "admin_adjustment"
	}
	sourceEventID := strings.TrimSpace(req.SourceEventID)
	if sourceEventID == "" {
		sourceEventID = adminCreditSourceEventID(userID)
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Credit.AdjustCredits(ctx, &creditpb.AdjustCreditsRequest{
		UserId:        userID,
		Delta:         req.Delta,
		Reason:        reason,
		Description:   strings.TrimSpace(req.Description),
		SourceEventId: sourceEventID,
		SourceType:    "admin_adjustment",
		SourceId:      currentActor(c).GetId(),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

type mallCreateOrderRequest struct {
	IdempotencyKey          string                 `json:"idempotency_key"`
	UserID                  int64                  `json:"user_id"`
	Items                   []mallOrderItemRequest `json:"items"`
	ExpectedOriginalCredits *int64                 `json:"expected_original_credits"`
	CouponCode              string                 `json:"coupon_code"`
	Receiver                string                 `json:"receiver"`
	Phone                   string                 `json:"phone"`
	Address                 string                 `json:"address"`
}

type mallOrderItemRequest struct {
	ProductID jsonInt64 `json:"product_id"`
	Quantity  int32     `json:"quantity"`
}

type mallPayOrderRequest struct {
	PaymentMethod  string `json:"payment_method"`
	IdempotencyKey string `json:"idempotency_key"`
}

type mallCartItemRequest struct {
	Quantity int32 `json:"quantity"`
}

type mallRefundRequest struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

type mallProductReviewRequest struct {
	OrderID jsonInt64 `json:"order_id"`
	Rating  int32     `json:"rating"`
	Content string    `json:"content"`
}

type mallAddressRequest struct {
	Receiver   string `json:"receiver"`
	Phone      string `json:"phone"`
	Province   string `json:"province"`
	City       string `json:"city"`
	District   string `json:"district"`
	Detail     string `json:"detail"`
	PostalCode string `json:"postal_code"`
	IsDefault  bool   `json:"is_default"`
}

type adminMallProductRequest struct {
	SKU          string               `json:"sku"`
	Title        string               `json:"title"`
	Description  string               `json:"description"`
	Category     string               `json:"category"`
	CoverURL     string               `json:"cover_url"`
	GrantType    string               `json:"grant_type"`
	GrantKey     string               `json:"grant_key"`
	PriceCredits int64                `json:"price_credits"`
	Stock        int64                `json:"stock"`
	Status       mallpb.ProductStatus `json:"status"`
	Sort         int32                `json:"sort"`
	OperatorID   string               `json:"operator_id"`
}

type adminMallProductCategoryRequest struct {
	Slug        string                       `json:"slug"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Status      mallpb.ProductCategoryStatus `json:"status"`
	Sort        int32                        `json:"sort"`
}

type adminMallCouponRequest struct {
	Code            string              `json:"code"`
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	DiscountCredits int64               `json:"discount_credits"`
	MinOrderCredits int64               `json:"min_order_credits"`
	TotalQuota      int64               `json:"total_quota"`
	PerUserLimit    int64               `json:"per_user_limit"`
	Status          mallpb.CouponStatus `json:"status"`
	StartsAt        int64               `json:"starts_at"`
	EndsAt          int64               `json:"ends_at"`
}

type adminMallOrderStatusRequest struct {
	Status          mallpb.OrderStatus `json:"status"`
	OperatorID      string             `json:"operator_id"`
	ShippingCarrier string             `json:"shipping_carrier"`
	TrackingNo      string             `json:"tracking_no"`
	Note            string             `json:"note"`
}

type adminMallDigitalEntitlementRevokeRequest struct {
	OperatorID string `json:"operator_id"`
	Reason     string `json:"reason"`
}

type adminMallRefundReviewRequest struct {
	Approved     bool   `json:"approved"`
	AdminNote    string `json:"admin_note"`
	RestoreStock bool   `json:"restore_stock"`
}

type adminMallProductReviewStatusRequest struct {
	Status mallpb.ProductReviewStatus `json:"status"`
}

func (h *Handler) listMallProducts(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListProducts(ctx, &mallpb.ListProductsRequest{
		Limit:    queryInt32(c, "limit", 20),
		Offset:   queryInt32(c, "offset", 0),
		Keyword:  c.Query("keyword"),
		Category: c.Query("category"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallProductCategories(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListProductCategories(ctx, &mallpb.ListProductCategoriesRequest{
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallProductReviews(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListProductReviews(ctx, &mallpb.ListProductReviewsRequest{
		ProductId: id,
		Limit:     queryInt32(c, "limit", 20),
		Offset:    queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createMallProductReview(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req mallProductReviewRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	resp, err := h.clients.Mall.CreateProductReview(ctx, &mallpb.CreateProductReviewRequest{
		UserId:    currentUserID(c),
		ProductId: id,
		OrderId:   req.OrderID.Int64(),
		Rating:    req.Rating,
		Content:   req.Content,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMyMallProductReviews(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListUserProductReviews(ctx, &mallpb.ListUserProductReviewsRequest{
		UserId:    currentUserID(c),
		ProductId: queryInt64(c, "product_id", 0),
		Status:    mallpb.ProductReviewStatus(queryInt32(c, "status", 0)),
		Limit:     queryInt32(c, "limit", 20),
		Offset:    queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getMallProduct(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.GetProduct(ctx, &mallpb.GetProductRequest{Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallCoupons(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListCoupons(ctx, &mallpb.ListCouponsRequest{
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMyMallCoupons(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListUserCouponUsages(ctx, &mallpb.ListUserCouponUsagesRequest{
		UserId: currentUserID(c),
		Status: mallpb.CouponUsageStatus(queryInt32(c, "status", 0)),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) claimMallCoupon(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ClaimCoupon(ctx, &mallpb.ClaimCouponRequest{
		UserId:   currentUserID(c),
		CouponId: id,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallProductFavorites(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListProductFavorites(ctx, &mallpb.ListProductFavoritesRequest{
		UserId: currentUserID(c),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getMallProductFavoriteState(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.IsProductFavorite(ctx, &mallpb.ProductFavoriteStateRequest{
		UserId:    currentUserID(c),
		ProductId: id,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"favorited": resp.GetFavorited()})
}

func (h *Handler) addMallProductFavorite(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AddProductFavorite(ctx, &mallpb.ProductFavoriteRequest{
		UserId:    currentUserID(c),
		ProductId: id,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"favorited": resp.GetFavorited()})
}

func (h *Handler) removeMallProductFavorite(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.RemoveProductFavorite(ctx, &mallpb.ProductFavoriteRequest{
		UserId:    currentUserID(c),
		ProductId: id,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"favorited": resp.GetFavorited()})
}

func (h *Handler) listMallCart(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListCartItems(ctx, &mallpb.ListCartItemsRequest{
		UserId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) setMallCartItem(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req mallCartItemRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.SetCartItem(ctx, &mallpb.SetCartItemRequest{
		UserId:    currentUserID(c),
		ProductId: id,
		Quantity:  req.Quantity,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) removeMallCartItem(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.RemoveCartItem(ctx, &mallpb.RemoveCartItemRequest{
		UserId:    currentUserID(c),
		ProductId: id,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) clearMallCart(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ClearCart(ctx, &mallpb.ClearCartRequest{
		UserId: currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) checkoutMallCart(c *gin.Context) {
	var req mallCreateOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	if !validateMallExpectedOriginalCredits(c, req.ExpectedOriginalCredits) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.CheckoutCart(ctx, &mallpb.CheckoutCartRequest{
		IdempotencyKey:          req.IdempotencyKey,
		UserId:                  currentUserID(c),
		ExpectedOriginalCredits: req.ExpectedOriginalCredits,
		CouponCode:              req.CouponCode,
		Receiver:                req.Receiver,
		Phone:                   req.Phone,
		Address:                 req.Address,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallAddresses(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListAddresses(ctx, &mallpb.ListAddressesRequest{
		UserId: currentUserID(c),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createMallAddress(c *gin.Context) {
	var req mallAddressRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.CreateAddress(ctx, &mallpb.CreateAddressRequest{
		UserId:     currentUserID(c),
		Receiver:   req.Receiver,
		Phone:      req.Phone,
		Province:   req.Province,
		City:       req.City,
		District:   req.District,
		Detail:     req.Detail,
		PostalCode: req.PostalCode,
		IsDefault:  req.IsDefault,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateMallAddress(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req mallAddressRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.UpdateAddress(ctx, &mallpb.UpdateAddressRequest{
		Id:         id,
		UserId:     currentUserID(c),
		Receiver:   req.Receiver,
		Phone:      req.Phone,
		Province:   req.Province,
		City:       req.City,
		District:   req.District,
		Detail:     req.Detail,
		PostalCode: req.PostalCode,
		IsDefault:  req.IsDefault,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) deleteMallAddress(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.DeleteAddress(ctx, &mallpb.DeleteAddressRequest{Id: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) setDefaultMallAddress(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.SetDefaultAddress(ctx, &mallpb.SetDefaultAddressRequest{Id: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createMallOrder(c *gin.Context) {
	var req mallCreateOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	if !validateMallExpectedOriginalCredits(c, req.ExpectedOriginalCredits) {
		return
	}
	items := make([]*mallpb.CreateOrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, &mallpb.CreateOrderItem{ProductId: item.ProductID.Int64(), Quantity: item.Quantity})
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.CreateOrder(ctx, &mallpb.CreateOrderRequest{
		IdempotencyKey:          req.IdempotencyKey,
		UserId:                  currentUserID(c),
		Items:                   items,
		ExpectedOriginalCredits: req.ExpectedOriginalCredits,
		CouponCode:              req.CouponCode,
		Receiver:                req.Receiver,
		Phone:                   req.Phone,
		Address:                 req.Address,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func validateMallExpectedOriginalCredits(c *gin.Context, value *int64) bool {
	if value != nil && *value >= 0 {
		return true
	}
	writeError(c, http.StatusBadRequest, "expected_original_credits is required and must be non-negative", "bad_request")
	return false
}

func (h *Handler) listMallOrders(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListOrders(ctx, &mallpb.ListOrdersRequest{
		UserId: currentUserID(c),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
		Status: mallpb.OrderStatus(queryInt32(c, "status", 0)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallDigitalEntitlements(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListUserDigitalEntitlements(ctx, &mallpb.ListUserDigitalEntitlementsRequest{
		UserId:    currentUserID(c),
		Status:    c.Query("status"),
		GrantType: c.Query("grant_type"),
		GrantKey:  c.Query("grant_key"),
		Limit:     queryInt32(c, "limit", 20),
		Offset:    queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallReviewableOrders(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListReviewableOrders(ctx, &mallpb.ListReviewableOrdersRequest{
		UserId:    currentUserID(c),
		ProductId: id,
		Limit:     queryInt32(c, "limit", 20),
		Offset:    queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getMallOrder(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.GetOrder(ctx, &mallpb.GetOrderRequest{Id: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetOrder().GetUserId() != currentUserID(c) {
		writeError(c, http.StatusForbidden, "order does not belong to user", "permission_denied")
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallOrderLogs(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	orderResp, err := h.clients.Mall.GetOrder(ctx, &mallpb.GetOrderRequest{Id: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if orderResp.GetOrder().GetUserId() != currentUserID(c) {
		writeError(c, http.StatusForbidden, "order does not belong to user", "permission_denied")
		return
	}
	resp, err := h.clients.Mall.ListOrderStatusLogs(ctx, &mallpb.ListOrderStatusLogsRequest{OrderId: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallOrderPayments(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	orderResp, err := h.clients.Mall.GetOrder(ctx, &mallpb.GetOrderRequest{Id: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if orderResp.GetOrder().GetUserId() != currentUserID(c) {
		writeError(c, http.StatusForbidden, "order does not belong to user", "permission_denied")
		return
	}
	resp, err := h.clients.Mall.ListOrderPayments(ctx, &mallpb.ListOrderPaymentsRequest{OrderId: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) payMallOrder(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req mallPayOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.PayOrder(ctx, &mallpb.PayOrderRequest{
		OrderId:        id,
		UserId:         currentUserID(c),
		PaymentMethod:  req.PaymentMethod,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) cancelMallOrder(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.CancelOrder(ctx, &mallpb.CancelOrderRequest{OrderId: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) confirmMallOrder(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ConfirmOrder(ctx, &mallpb.ConfirmOrderRequest{OrderId: id, UserId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createMallRefundRequest(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req mallRefundRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.CreateRefundRequest(ctx, &mallpb.CreateRefundRequestRequest{
		OrderId: id,
		UserId:  currentUserID(c),
		Reason:  req.Reason,
		Note:    req.Note,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) cancelMallRefundRequest(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.CancelRefundRequest(ctx, &mallpb.CancelRefundRequestRequest{
		RefundId: id,
		UserId:   currentUserID(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listMallRefundRequests(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListRefundRequests(ctx, &mallpb.ListRefundRequestsRequest{
		UserId: currentUserID(c),
		Limit:  queryInt32(c, "limit", 20),
		Offset: queryInt32(c, "offset", 0),
		Status: mallpb.RefundStatus(queryInt32(c, "status", 0)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) adminMallOverview(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminMallOverview(ctx, &mallpb.AdminMallOverviewRequest{
		LowStockThreshold: queryInt64(c, "low_stock_threshold", 10),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, adminMallOverviewPayload(resp))
}

func (h *Handler) listAdminMallFinanceAnomalies(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListFinanceAnomalies(ctx, &mallpb.AdminListFinanceAnomaliesRequest{
		Limit:   queryInt32(c, "limit", 20),
		Offset:  queryInt32(c, "offset", 0),
		Keyword: c.Query("keyword"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{
		"items": mallFinanceAnomaliesPayload(resp.GetItems()),
		"total": resp.GetTotal(),
	})
}

func adminMallOverviewPayload(resp *mallpb.AdminMallOverviewResponse) gin.H {
	if resp == nil {
		return gin.H{"overview": mallOverviewPayload(nil)}
	}
	return gin.H{"overview": mallOverviewPayload(resp.GetOverview())}
}

func mallOverviewPayload(overview *mallpb.MallOverview) gin.H {
	return gin.H{
		"product_total":                   overview.GetProductTotal(),
		"active_product_total":            overview.GetActiveProductTotal(),
		"low_stock_total":                 overview.GetLowStockTotal(),
		"stock_total":                     overview.GetStockTotal(),
		"sales_count_total":               overview.GetSalesCountTotal(),
		"order_total":                     overview.GetOrderTotal(),
		"paid_order_total":                overview.GetPaidOrderTotal(),
		"revenue_credits_total":           overview.GetRevenueCreditsTotal(),
		"today_order_total":               overview.GetTodayOrderTotal(),
		"today_revenue_credits":           overview.GetTodayRevenueCredits(),
		"pending_shipment_total":          overview.GetPendingShipmentTotal(),
		"pending_refund_total":            overview.GetPendingRefundTotal(),
		"refunded_credits_total":          overview.GetRefundedCreditsTotal(),
		"succeeded_payment_credits_total": overview.GetSucceededPaymentCreditsTotal(),
		"failed_payment_total":            overview.GetFailedPaymentTotal(),
		"failed_payment_credits_total":    overview.GetFailedPaymentCreditsTotal(),
		"pending_refund_credits_total":    overview.GetPendingRefundCreditsTotal(),
		"net_revenue_credits_total":       overview.GetNetRevenueCreditsTotal(),
		"finance_anomaly_total":           overview.GetFinanceAnomalyTotal(),
		"finance_anomalies":               mallFinanceAnomaliesPayload(overview.GetFinanceAnomalies()),
		"order_status_counts":             mallStatusCountsPayload(overview.GetOrderStatusCounts()),
		"refund_status_counts":            mallStatusCountsPayload(overview.GetRefundStatusCounts()),
		"low_stock_products":              mallProductsPayload(overview.GetLowStockProducts()),
		"top_selling_products":            mallProductsPayload(overview.GetTopSellingProducts()),
		"pending_outbox_total":            overview.GetPendingOutboxTotal(),
		"outbox_status_counts":            mallStatusCountsPayload(overview.GetOutboxStatusCounts()),
		"outbox_last_error":               overview.GetOutboxLastError(),
		"outbox_last_error_at":            overview.GetOutboxLastErrorAt(),
		"outbox_next_attempt_at":          overview.GetOutboxNextAttemptAt(),
	}
}

func mallFinanceAnomaliesPayload(items []*mallpb.FinanceAnomaly) []gin.H {
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		result = append(result, gin.H{
			"issue_type":                item.GetIssueType(),
			"order_id":                  item.GetOrderId(),
			"order_no":                  item.GetOrderNo(),
			"user_id":                   item.GetUserId(),
			"order_status":              mallOrderStatusName(item.GetOrderStatus()),
			"order_total_credits":       item.GetOrderTotalCredits(),
			"succeeded_payment_credits": item.GetSucceededPaymentCredits(),
			"refunded_credits":          item.GetRefundedCredits(),
			"difference_credits":        item.GetDifferenceCredits(),
			"updated_at":                item.GetUpdatedAt(),
		})
	}
	return result
}

func mallOrderStatusName(status mallpb.OrderStatus) string {
	return strings.TrimPrefix(strings.ToUpper(status.String()), "ORDER_STATUS_")
}

func mallStatusCountsPayload(items []*mallpb.MallStatusCount) []gin.H {
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		result = append(result, gin.H{
			"status": item.GetStatus(),
			"count":  item.GetCount(),
		})
	}
	return result
}

func mallProductsPayload(items []*mallpb.Product) []gin.H {
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		result = append(result, gin.H{
			"id":            item.GetId(),
			"sku":           item.GetSku(),
			"title":         item.GetTitle(),
			"description":   item.GetDescription(),
			"category":      item.GetCategory(),
			"cover_url":     item.GetCoverUrl(),
			"grant_type":    item.GetGrantType(),
			"grant_key":     item.GetGrantKey(),
			"price_credits": item.GetPriceCredits(),
			"stock":         item.GetStock(),
			"sales_count":   item.GetSalesCount(),
			"status":        item.GetStatus(),
			"sort":          item.GetSort(),
			"created_at":    item.GetCreatedAt(),
			"updated_at":    item.GetUpdatedAt(),
		})
	}
	return result
}

func (h *Handler) listAdminMallProducts(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListProducts(ctx, &mallpb.AdminListProductsRequest{
		Limit:    queryInt32(c, "limit", 20),
		Offset:   queryInt32(c, "offset", 0),
		Keyword:  c.Query("keyword"),
		Category: c.Query("category"),
		Status:   mallpb.ProductStatus(queryInt32(c, "status", 0)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallProductCategories(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListProductCategories(ctx, &mallpb.AdminListProductCategoriesRequest{
		Limit:   queryInt32(c, "limit", 20),
		Offset:  queryInt32(c, "offset", 0),
		Keyword: c.Query("keyword"),
		Status:  mallpb.ProductCategoryStatus(queryInt32(c, "status", 0)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallProductReviews(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListProductReviews(ctx, &mallpb.AdminListProductReviewsRequest{
		ProductId: queryInt64(c, "product_id", 0),
		UserId:    queryInt64(c, "user_id", 0),
		Status:    mallpb.ProductReviewStatus(queryInt32(c, "status", 0)),
		Limit:     queryInt32(c, "limit", 20),
		Offset:    queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminMallProductReviewStatus(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminMallProductReviewStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminUpdateProductReviewStatus(ctx, &mallpb.AdminUpdateProductReviewStatusRequest{
		Id:     id,
		Status: req.Status,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminMallProduct(c *gin.Context) {
	var req adminMallProductRequest
	if !bindJSON(c, &req) {
		return
	}
	req.OperatorID = fmt.Sprintf("%d", currentActor(c).GetId())
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminCreateProduct(ctx, &mallpb.AdminCreateProductRequest{
		Sku:          req.SKU,
		Title:        req.Title,
		Description:  req.Description,
		Category:     req.Category,
		CoverUrl:     req.CoverURL,
		GrantType:    req.GrantType,
		GrantKey:     req.GrantKey,
		PriceCredits: req.PriceCredits,
		Stock:        req.Stock,
		Status:       req.Status,
		Sort:         req.Sort,
		OperatorId:   req.OperatorID,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminMallProductCategory(c *gin.Context) {
	var req adminMallProductCategoryRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminCreateProductCategory(ctx, adminMallProductCategoryPB(0, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminMallProductCategory(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminMallProductCategoryRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminUpdateProductCategory(ctx, adminMallProductCategoryPB(id, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminMallProduct(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminMallProductRequest
	if !bindJSON(c, &req) {
		return
	}
	req.OperatorID = fmt.Sprintf("%d", currentActor(c).GetId())
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminUpdateProduct(ctx, &mallpb.AdminUpdateProductRequest{
		Id:           id,
		Sku:          req.SKU,
		Title:        req.Title,
		Description:  req.Description,
		Category:     req.Category,
		CoverUrl:     req.CoverURL,
		GrantType:    req.GrantType,
		GrantKey:     req.GrantKey,
		PriceCredits: req.PriceCredits,
		Stock:        req.Stock,
		Status:       req.Status,
		Sort:         req.Sort,
		OperatorId:   req.OperatorID,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func adminMallProductCategoryPB(id int64, req adminMallProductCategoryRequest) *mallpb.AdminSaveProductCategoryRequest {
	return &mallpb.AdminSaveProductCategoryRequest{
		Id:          id,
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		Sort:        req.Sort,
	}
}

func (h *Handler) listAdminMallProductStockLogs(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListProductStockLogs(ctx, &mallpb.AdminListProductStockLogsRequest{
		ProductId: id,
		Limit:     queryInt32(c, "limit", 20),
		Offset:    queryInt32(c, "offset", 0),
		Reason:    c.Query("reason"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallCoupons(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListCoupons(ctx, &mallpb.AdminListCouponsRequest{
		Limit:   queryInt32(c, "limit", 20),
		Offset:  queryInt32(c, "offset", 0),
		Keyword: c.Query("keyword"),
		Status:  mallpb.CouponStatus(queryInt32(c, "status", 0)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallCouponUsages(c *gin.Context) {
	couponID, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListCouponUsages(ctx, &mallpb.AdminListCouponUsagesRequest{
		CouponId: couponID,
		UserId:   queryInt64(c, "user_id", 0),
		Status:   mallpb.CouponUsageStatus(queryInt32(c, "status", 0)),
		Limit:    queryInt32(c, "limit", 20),
		Offset:   queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) createAdminMallCoupon(c *gin.Context) {
	var req adminMallCouponRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminCreateCoupon(ctx, adminMallCouponPB(0, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminMallCoupon(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminMallCouponRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminUpdateCoupon(ctx, adminMallCouponPB(id, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func adminMallCouponPB(id int64, req adminMallCouponRequest) *mallpb.AdminSaveCouponRequest {
	return &mallpb.AdminSaveCouponRequest{
		Id:              id,
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		DiscountCredits: req.DiscountCredits,
		MinOrderCredits: req.MinOrderCredits,
		TotalQuota:      req.TotalQuota,
		PerUserLimit:    req.PerUserLimit,
		Status:          req.Status,
		StartsAt:        req.StartsAt,
		EndsAt:          req.EndsAt,
	}
}

func (h *Handler) listAdminMallOrders(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListOrders(ctx, &mallpb.AdminListOrdersRequest{
		UserId:  queryInt64(c, "user_id", 0),
		Limit:   queryInt32(c, "limit", 20),
		Offset:  queryInt32(c, "offset", 0),
		Keyword: c.Query("keyword"),
		Status:  mallpb.OrderStatus(queryInt32(c, "status", 0)),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallPayments(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListOrderPayments(ctx, &mallpb.AdminListOrderPaymentsRequest{
		UserId:  queryInt64(c, "user_id", 0),
		Limit:   queryInt32(c, "limit", 20),
		Offset:  queryInt32(c, "offset", 0),
		Keyword: c.Query("keyword"),
		Status:  queryInt32(c, "status", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallDigitalEntitlements(c *gin.Context) {
	orderIDs, ok := queryPositiveInt64CSV(c, "order_ids", adminDigitalEntitlementOrderIDFilterLimit)
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid order_ids", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListDigitalEntitlements(ctx, &mallpb.AdminListDigitalEntitlementsRequest{
		UserId:    queryInt64(c, "user_id", 0),
		Status:    c.Query("status"),
		GrantType: c.Query("grant_type"),
		GrantKey:  c.Query("grant_key"),
		Keyword:   c.Query("keyword"),
		OrderIds:  orderIDs,
		Limit:     queryInt32(c, "limit", 20),
		Offset:    queryInt32(c, "offset", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) revokeAdminMallDigitalEntitlement(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminMallDigitalEntitlementRevokeRequest
	if !bindJSON(c, &req) {
		return
	}
	req.OperatorID = fmt.Sprintf("%d", currentActor(c).GetId())
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminRevokeDigitalEntitlement(ctx, &mallpb.AdminRevokeDigitalEntitlementRequest{
		Id:         id,
		OperatorId: req.OperatorID,
		Reason:     req.Reason,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) closeAdminExpiredMallOrders(c *gin.Context) {
	var req adminCloseExpiredMallOrdersRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.CloseExpiredOrders(ctx, &mallpb.CloseExpiredOrdersRequest{
		ExpireAfterSeconds: req.ExpireAfterSeconds,
		Limit:              req.Limit,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) recoverAdminStalePayingMallOrders(c *gin.Context) {
	var req adminRecoverStalePayingMallOrdersRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.RecoverStalePayingOrders(ctx, &mallpb.RecoverStalePayingOrdersRequest{
		StaleAfterSeconds: req.StaleAfterSeconds,
		Limit:             req.Limit,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{
		"recovered": resp.GetRecovered(),
		"failed":    resp.GetFailed(),
	})
}

func (h *Handler) requeueAdminMallOutboxEvents(c *gin.Context) {
	var req adminRequeueMallOutboxRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminRequeueOutboxEvents(ctx, &mallpb.AdminRequeueOutboxEventsRequest{
		Statuses:   req.Statuses,
		Limit:      req.Limit,
		OperatorId: fmt.Sprintf("%d", currentActor(c).GetId()),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{
		"requeued":  resp.GetRequeued(),
		"event_ids": resp.GetEventIds(),
	})
}

func (h *Handler) listAdminMallOutboxRequeueAudits(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListOutboxRequeueAudits(ctx, &mallpb.AdminListOutboxRequeueAuditsRequest{
		Limit:         queryInt32(c, "limit", 10),
		Offset:        queryInt32(c, "offset", 0),
		EventId:       c.Query("event_id"),
		AggregateType: c.Query("aggregate_type"),
		AggregateId:   queryInt64(c, "aggregate_id", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) updateAdminMallOrderStatus(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminMallOrderStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	req.OperatorID = fmt.Sprintf("%d", currentActor(c).GetId())
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminUpdateOrderStatus(ctx, &mallpb.AdminUpdateOrderStatusRequest{
		OrderId:         id,
		Status:          req.Status,
		OperatorId:      req.OperatorID,
		ShippingCarrier: req.ShippingCarrier,
		TrackingNo:      req.TrackingNo,
		Note:            req.Note,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallOrderLogs(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListOrderStatusLogs(ctx, &mallpb.ListOrderStatusLogsRequest{OrderId: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallOrderPayments(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.ListOrderPayments(ctx, &mallpb.ListOrderPaymentsRequest{OrderId: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listAdminMallRefundRequests(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminListRefundRequests(ctx, &mallpb.AdminListRefundRequestsRequest{
		UserId:  queryInt64(c, "user_id", 0),
		Limit:   queryInt32(c, "limit", 20),
		Offset:  queryInt32(c, "offset", 0),
		Status:  mallpb.RefundStatus(queryInt32(c, "status", 0)),
		Keyword: c.Query("keyword"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) reviewAdminMallRefundRequest(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req adminMallRefundReviewRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Mall.AdminReviewRefundRequest(ctx, &mallpb.AdminReviewRefundRequestRequest{
		RefundId:     id,
		Approved:     req.Approved,
		OperatorId:   fmt.Sprintf("%d", currentActor(c).GetId()),
		AdminNote:    req.AdminNote,
		RestoreStock: req.RestoreStock,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) requireAuth() gin.HandlerFunc {
	return h.requireAuthScope("")
}

func (h *Handler) requireAuthScope(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, err := h.authIdentityFromRequestWithScope(c, requiredScope)
		if err != nil {
			writeAuthenticationError(c, err)
			c.Abort()
			return
		}
		c.Set("user_id", identity.userID)
		c.Set("username", identity.username)
		c.Set("session_id", identity.sessionID)
		c.Set("auth_token_type", identity.tokenType)
		c.Set("auth_scopes", identity.scopes)
		c.Next()
	}
}

func (h *Handler) optionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(c.GetHeader(h.tokenHeader)) == "" {
			c.Next()
			return
		}
		identity, err := h.authIdentityFromRequestWithScope(c, "")
		if err != nil {
			writeAuthenticationError(c, err)
			c.Abort()
			return
		}
		c.Set("user_id", identity.userID)
		c.Set("username", identity.username)
		c.Set("session_id", identity.sessionID)
		c.Set("auth_token_type", identity.tokenType)
		c.Set("auth_scopes", identity.scopes)
		c.Next()
	}
}

func (h *Handler) requireAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken, err := h.authTokenFromRequest(c)
		if err != nil {
			writeError(c, http.StatusUnauthorized, err.Error(), "unauthorized")
			c.Abort()
			return
		}
		ctx, cancel := rpcContext(c)
		defer cancel()
		profile, err := h.clients.Admin.GetProfile(ctx, &adminpb.ProfileRequest{AccessToken: accessToken})
		if err != nil {
			writeRPCError(c, err)
			c.Abort()
			return
		}
		if profile.GetUser() == nil {
			writeError(c, http.StatusUnauthorized, "admin profile not found", "unauthorized")
			c.Abort()
			return
		}
		c.Set("admin_id", profile.GetUser().GetId())
		c.Set("admin_username", profile.GetUser().GetUsername())
		c.Set("admin_profile", profile)
		started := time.Now()
		requestBody := captureAdminRequestBody(c)
		c.Next()
		h.recordAdminOperationLog(c, started, requestBody)
	}
}

func (h *Handler) requireAdminPermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permission = strings.ToLower(strings.TrimSpace(permission))
		if permission == "" {
			c.Next()
			return
		}
		value, ok := c.Get("admin_profile")
		if !ok {
			writeError(c, http.StatusUnauthorized, "admin profile not found", "unauthorized")
			c.Abort()
			return
		}
		profile, ok := value.(*adminpb.ProfileResponse)
		if !ok || profile.GetUser() == nil {
			writeError(c, http.StatusUnauthorized, "admin profile invalid", "unauthorized")
			c.Abort()
			return
		}
		if adminProfileHasPermission(profile, permission) {
			c.Next()
			return
		}
		writeError(c, http.StatusForbidden, "admin permission denied", "permission_denied")
		c.Abort()
	}
}

func (h *Handler) requireAdminAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get("admin_profile")
		if !ok {
			writeError(c, http.StatusUnauthorized, "admin profile not found", "unauthorized")
			c.Abort()
			return
		}
		profile, ok := value.(*adminpb.ProfileResponse)
		if !ok || profile.GetUser() == nil {
			writeError(c, http.StatusUnauthorized, "admin profile invalid", "unauthorized")
			c.Abort()
			return
		}
		for _, permission := range permissions {
			if adminProfileHasPermission(profile, permission) {
				c.Next()
				return
			}
		}
		writeError(c, http.StatusForbidden, "admin permission denied", "permission_denied")
		c.Abort()
	}
}

func adminProfileHasPermission(profile *adminpb.ProfileResponse, required string) bool {
	required = strings.ToLower(strings.TrimSpace(required))
	if required == "" {
		return true
	}
	for _, permission := range profile.GetPermissions() {
		permission = strings.ToLower(strings.TrimSpace(permission))
		switch {
		case permission == required:
			return true
		case permission == "*" || permission == "*:*" || permission == "*:*:*":
			return true
		case strings.HasSuffix(permission, ":*"):
			prefix := strings.TrimSuffix(permission, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}

type authIdentity struct {
	userID    int64
	username  string
	sessionID string
	tokenType string
	scopes    map[string]bool
}

func (h *Handler) authIdentityFromRequest(c *gin.Context) (authIdentity, error) {
	return h.authIdentityFromRequestWithScope(c, "")
}

func (h *Handler) authIdentityFromRequestWithScope(c *gin.Context, requiredScope string) (authIdentity, error) {
	accessToken, err := h.authTokenFromRequest(c)
	if err != nil {
		return authIdentity{}, err
	}
	claims, err := h.parseAuthToken(accessToken)
	if err != nil {
		return authIdentity{}, errors.New("invalid authorization token")
	}
	tokenType, scopes, err := parseAPITokenClaims(claims)
	if err != nil {
		return authIdentity{}, err
	}
	if h.tokenRevocations != nil {
		ctx, cancel := rpcContext(c)
		defer cancel()
		revoked, err := h.tokenRevocations.IsRevoked(ctx, accessToken)
		if err != nil {
			return authIdentity{}, tokenRevocationUnavailableError(err)
		}
		if revoked {
			return authIdentity{}, errors.New("authorization token revoked")
		}
	}
	sessionID := sessionIDClaim(claims)
	// Older tokens predate session tracking and carry no jti, so an empty
	// session id stays valid instead of locking those clients out.
	if sessionID != "" && h.tokenRevocations != nil {
		ctx, cancel := rpcContext(c)
		defer cancel()
		revoked, err := h.tokenRevocations.IsSessionRevoked(ctx, sessionID)
		if err != nil {
			return authIdentity{}, tokenRevocationUnavailableError(err)
		}
		if revoked {
			return authIdentity{}, errors.New("authorization session revoked")
		}
	}
	identity := authIdentity{username: normalizedClaimString(claims, "username"), sessionID: sessionID, tokenType: tokenType, scopes: scopes}
	if id, ok := claimInt64(claims, "sub", "user_id"); ok {
		if err := h.validateCredentialVersion(c, claims, id); err != nil {
			return authIdentity{}, err
		}
		identity.userID = id
		if err := authorizeAPITokenScope(c, tokenType, scopes, requiredScope); err != nil {
			return authIdentity{}, err
		}
		return identity, nil
	}
	return authIdentity{}, errors.New("missing user id claim")
}

func (h *Handler) parseAuthToken(accessToken string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid authorization token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid authorization claims")
	}
	return claims, nil
}

func (h *Handler) authTokenExpiry(accessToken string) (time.Time, error) {
	claims, err := h.parseAuthToken(accessToken)
	if err != nil {
		return time.Time{}, errors.New("invalid authorization token")
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil || !expiresAt.After(time.Now()) {
		return time.Time{}, errors.New("authorization token has no valid expiration")
	}
	return expiresAt.Time, nil
}

func tokenRevocationUnavailableError(err error) error {
	return fmt.Errorf("%w: %v", errTokenRevocationUnavailable, err)
}

func (h *Handler) validateCredentialVersion(c *gin.Context, claims jwt.MapClaims, userID int64) error {
	claimed, hasClaim := credentialVersionClaimValue(claims)
	ctx, cancel := rpcContext(c)
	defer cancel()
	return h.validateCredentialVersionValue(ctx, userID, claimed, hasClaim)
}

func (h *Handler) validateCredentialVersionValue(ctx context.Context, userID int64, claimed string, hasClaim bool) error {
	if !h.requireCredentialVersionValidation {
		return nil
	}
	if h.credentialVersions == nil {
		return errCredentialVersionUnavailable
	}
	current, err := h.credentialVersions.Current(ctx, userID)
	if err != nil {
		return credentialVersionUnavailableError(err)
	}
	current = strings.TrimSpace(current)
	if current == "" {
		return credentialVersionUnavailableError(errors.New("empty credential version"))
	}
	if current == credentialVersionInitial {
		// Tokens issued before cv was introduced remain usable only until the
		// user's first password credential change. Once Redis holds a non-initial
		// version, a missing or stale claim is rejected.
		if !hasClaim || claimed == credentialVersionInitial {
			return nil
		}
		return errors.New("authorization token credential version is invalid")
	}
	if !hasClaim || claimed != current {
		return errors.New("authorization token invalidated after credential change")
	}
	return nil
}

func credentialVersionClaimValue(claims jwt.MapClaims) (string, bool) {
	value, ok := claims[credentialVersionClaim]
	if !ok {
		return "", false
	}
	version, ok := value.(string)
	version = strings.TrimSpace(version)
	return version, ok && version != ""
}

func credentialVersionUnavailableError(err error) error {
	return fmt.Errorf("%w: %v", errCredentialVersionUnavailable, err)
}

func writeAuthenticationError(c *gin.Context, err error) {
	if errors.Is(err, errAPITokenScopeDenied) {
		writeError(c, http.StatusForbidden, err.Error(), "permission_denied")
		return
	}
	if errors.Is(err, errTokenRevocationUnavailable) || errors.Is(err, errCredentialVersionUnavailable) {
		writeError(c, http.StatusServiceUnavailable, "authorization service unavailable", "service_unavailable")
		return
	}
	writeError(c, http.StatusUnauthorized, err.Error(), "unauthorized")
}

func (h *Handler) authTokenFromRequest(c *gin.Context) (string, error) {
	header := strings.TrimSpace(c.GetHeader(h.tokenHeader))
	if header == "" {
		return "", errors.New("missing authorization token")
	}
	prefix := h.tokenPrefix
	if prefix != "" {
		expected := prefix + " "
		if !strings.HasPrefix(header, expected) {
			return "", errors.New("invalid authorization token")
		}
		header = strings.TrimSpace(strings.TrimPrefix(header, expected))
	}
	if header == "" {
		return "", errors.New("missing authorization token")
	}
	return header, nil
}

func currentUserID(c *gin.Context) int64 {
	value, _ := c.Get("user_id")
	id, _ := value.(int64)
	return id
}

// currentSessionID returns the session that authenticated this request, or an
// empty string for a legacy token that carries no jti claim.
func currentSessionID(c *gin.Context) string {
	value, _ := c.Get("session_id")
	sessionID, _ := value.(string)
	return sessionID
}

func currentUsername(c *gin.Context) string {
	value, _ := c.Get("username")
	username, _ := value.(string)
	return username
}

func currentActor(c *gin.Context) *adminpb.Actor {
	if value, ok := c.Get("admin_id"); ok {
		id, _ := value.(int64)
		usernameValue, _ := c.Get("admin_username")
		username, _ := usernameValue.(string)
		if id > 0 {
			return &adminpb.Actor{Id: id, Username: username}
		}
	}
	return &adminpb.Actor{Id: currentUserID(c), Username: currentUsername(c)}
}

func adminCreditSourceEventID(userID int64) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("admin-credit-%d-%d", userID, time.Now().UnixMilli())
	}
	return fmt.Sprintf("admin-credit-%d-%d-%s", userID, time.Now().UnixMilli(), hex.EncodeToString(buf))
}

func adminProfilePayload(profile *adminpb.ProfileResponse) gin.H {
	if profile == nil {
		return gin.H{}
	}
	return gin.H{
		"user":        toHTTPAdminUser(profile.GetUser()),
		"roles":       profile.GetRoles(),
		"permissions": profile.GetPermissions(),
	}
}

func toHTTPAdminUser(user *adminpb.AdminUserInfo) gin.H {
	if user == nil {
		return gin.H{}
	}
	return gin.H{
		"id":          user.GetId(),
		"username":    user.GetUsername(),
		"nickname":    user.GetNickname(),
		"email":       user.GetEmail(),
		"phone":       user.GetPhone(),
		"avatar":      user.GetAvatarUrl(),
		"avatar_url":  user.GetAvatarUrl(),
		"avatarUrl":   user.GetAvatarUrl(),
		"bio":         user.GetBio(),
		"description": user.GetBio(),
		"status":      user.GetStatus(),
		"locked_flag": user.GetLockedFlag(),
		"lockedFlag":  user.GetLockedFlag(),
		"roles":       user.GetRoles(),
	}
}

func allowedAvatarExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func uploadedAvatarName(ext string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), hex.EncodeToString(buf), ext), nil
}

func publicRequestURL(c *gin.Context, path string) string {
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host + path
}

func buildUserBadges(user *userpb.UserInfo, definitions []*adminpb.BadgeInfo) []gin.H {
	if user == nil || user.GetId() == 0 || len(definitions) == 0 {
		return []gin.H{}
	}
	badges := make([]gin.H, 0, len(definitions))
	for _, badge := range definitions {
		awardedAt, ok := badgeAwardedAt(user, badge)
		if !ok {
			continue
		}
		id := badge.GetKey()
		if id == "" {
			id = strconv.FormatInt(badge.GetId(), 10)
		}
		badges = append(badges, gin.H{
			"id":          id,
			"name":        badge.GetName(),
			"description": badge.GetDescription(),
			"icon_url":    badge.GetIconUrl(),
			"awarded_at":  awardedAt,
			"status":      "awarded",
			"rule_type":   badge.GetRuleType(),
			"rule_value":  badge.GetRuleValue(),
		})
	}
	return badges
}

func mergeDigitalBadgeEntitlements(items []gin.H, definitions []*adminpb.BadgeInfo, entitlements []*mallpb.DigitalEntitlement) []gin.H {
	if len(entitlements) == 0 {
		return items
	}
	definitionIndex := badgeDefinitionIndex(definitions)
	seen := make(map[string]struct{}, len(items)+len(entitlements))
	for _, item := range items {
		rememberBadgeKey(seen, badgeRowID(item))
	}
	merged := items
	for _, entitlement := range entitlements {
		key, ok := digitalBadgeEntitlementKey(entitlement)
		if !ok {
			continue
		}
		if _, exists := seen[badgeCanonicalKey(key)]; exists {
			continue
		}
		if _, exists := seen[badgeKey(key)]; exists {
			continue
		}
		definition := lookupBadgeDefinition(definitionIndex, key)
		merged = append(merged, digitalBadgeEntitlementRow(entitlement, definition, key))
		rememberBadgeKey(seen, key)
		if definition != nil {
			rememberBadgeKey(seen, definition.GetKey())
		}
	}
	return merged
}

func mergeMembershipEntitlementBadge(items []gin.H, entitlements []*mallpb.DigitalEntitlement) []gin.H {
	entitlement := latestActiveMembershipEntitlement(entitlements)
	if entitlement == nil {
		return items
	}
	id := membershipEntitlementBadgeID(entitlement)
	if id == "" {
		return items
	}
	for _, item := range items {
		if badgeRowID(item) == id {
			return items
		}
	}
	description := "已开通会员权益。"
	expiresAt := entitlement.GetExpiresAt()
	if expiresAt > 0 {
		description = fmt.Sprintf("已开通会员权益，当前有效至 %s。", time.UnixMilli(expiresAt).Format("2006-01-02"))
	}
	return append(items, gin.H{
		"id":               id,
		"name":             "会员",
		"description":      description,
		"icon_url":         "",
		"awarded_at":       entitlement.GetIssuedAt(),
		"status":           "awarded",
		"rule_type":        "digital_entitlement",
		"rule_value":       0,
		"source":           "digital_entitlement",
		"entitlement_id":   entitlement.GetId(),
		"order_id":         entitlement.GetOrderId(),
		"order_no":         entitlement.GetOrderNo(),
		"fulfillment_code": entitlement.GetFulfillmentCode(),
		"grant_type":       "membership",
		"grant_key":        badgeKey(entitlement.GetGrantKey()),
		"expires_at":       expiresAt,
	})
}

func membershipEntitlementBadgeID(entitlement *mallpb.DigitalEntitlement) string {
	key := badgeKey(entitlement.GetGrantKey())
	if key == "" {
		return ""
	}
	return "digital-membership-" + key
}

func latestActiveMembershipEntitlement(entitlements []*mallpb.DigitalEntitlement) *mallpb.DigitalEntitlement {
	now := time.Now()
	var latest *mallpb.DigitalEntitlement
	for _, entitlement := range entitlements {
		if !digitalEntitlementIsActive(entitlement, now) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(entitlement.GetGrantType())) != digitalEntitlementGrantTypeMembership {
			continue
		}
		if strings.TrimSpace(entitlement.GetGrantKey()) == "" {
			continue
		}
		if entitlement.GetExpiresAt() <= now.UnixMilli() {
			continue
		}
		if latest == nil || entitlement.GetExpiresAt() > latest.GetExpiresAt() {
			latest = entitlement
		}
	}
	return latest
}

func digitalBadgeEntitlementKey(entitlement *mallpb.DigitalEntitlement) (string, bool) {
	if !digitalEntitlementIsActive(entitlement, time.Now()) {
		return "", false
	}
	grantType := strings.ToLower(strings.TrimSpace(entitlement.GetGrantType()))
	if grantType != "badge" {
		return "", false
	}
	key := badgeKey(entitlement.GetGrantKey())
	if key == "" {
		return "", false
	}
	return key, true
}

func digitalBadgeEntitlementRow(entitlement *mallpb.DigitalEntitlement, definition *adminpb.BadgeInfo, key string) gin.H {
	id := key
	name := strings.TrimSpace(entitlement.GetTitle())
	description := "通过商城数字权益获得。"
	iconURL := ""
	ruleType := "digital_entitlement"
	var ruleValue int64
	if definition != nil {
		if definition.GetKey() != "" {
			id = definition.GetKey()
		} else if definition.GetId() > 0 {
			id = strconv.FormatInt(definition.GetId(), 10)
		}
		if strings.TrimSpace(definition.GetName()) != "" {
			name = definition.GetName()
		}
		if strings.TrimSpace(definition.GetDescription()) != "" {
			description = definition.GetDescription()
		}
		iconURL = definition.GetIconUrl()
		if strings.TrimSpace(definition.GetRuleType()) != "" {
			ruleType = definition.GetRuleType()
		}
		ruleValue = definition.GetRuleValue()
	}
	if name == "" {
		name = strings.TrimPrefix(key, "badge-")
		if name == "" {
			name = key
		}
	}
	return gin.H{
		"id":               id,
		"name":             name,
		"description":      description,
		"icon_url":         iconURL,
		"awarded_at":       entitlement.GetIssuedAt(),
		"status":           "awarded",
		"rule_type":        ruleType,
		"rule_value":       ruleValue,
		"source":           "digital_entitlement",
		"entitlement_id":   entitlement.GetId(),
		"order_id":         entitlement.GetOrderId(),
		"order_no":         entitlement.GetOrderNo(),
		"fulfillment_code": entitlement.GetFulfillmentCode(),
		"grant_type":       "badge",
		"grant_key":        key,
	}
}

func badgeDefinitionIndex(definitions []*adminpb.BadgeInfo) map[string]*adminpb.BadgeInfo {
	index := make(map[string]*adminpb.BadgeInfo, len(definitions)*2)
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		rememberBadgeDefinition(index, definition, definition.GetKey())
		if definition.GetId() > 0 {
			rememberBadgeDefinition(index, definition, strconv.FormatInt(definition.GetId(), 10))
		}
	}
	return index
}

func rememberBadgeDefinition(index map[string]*adminpb.BadgeInfo, definition *adminpb.BadgeInfo, raw string) {
	for _, key := range []string{badgeKey(raw), badgeCanonicalKey(raw)} {
		if key == "" {
			continue
		}
		if _, exists := index[key]; !exists {
			index[key] = definition
		}
	}
}

func lookupBadgeDefinition(index map[string]*adminpb.BadgeInfo, raw string) *adminpb.BadgeInfo {
	if definition := index[badgeKey(raw)]; definition != nil {
		return definition
	}
	return index[badgeCanonicalKey(raw)]
}

func rememberBadgeKey(seen map[string]struct{}, raw string) {
	for _, key := range []string{badgeKey(raw), badgeCanonicalKey(raw)} {
		if key != "" {
			seen[key] = struct{}{}
		}
	}
}

func badgeRowID(item gin.H) string {
	value, ok := item["id"]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func badgeKey(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func badgeCanonicalKey(raw string) string {
	key := badgeKey(raw)
	canonical := strings.TrimPrefix(key, "badge-")
	if canonical == "" {
		return key
	}
	return canonical
}

func badgeAwardedAt(user *userpb.UserInfo, badge *adminpb.BadgeInfo) (int64, bool) {
	if badge == nil || badge.GetStatus() != 2 {
		return 0, false
	}
	switch strings.TrimSpace(strings.ToLower(badge.GetRuleType())) {
	case "", "manual":
		return 0, false
	case "always", "account_created":
		return user.GetCreatedAt(), user.GetCreatedAt() > 0
	case "following_count":
		return user.GetCreatedAt(), user.GetFollowingCount() >= badge.GetRuleValue()
	case "follower_count":
		return user.GetCreatedAt(), user.GetFollowerCount() >= badge.GetRuleValue()
	default:
		return 0, false
	}
}

func paginateBadgeRows(items []gin.H, limit int, offset int) []gin.H {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset >= len(items) {
		return []gin.H{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func (h *Handler) requireTopicOwner(c *gin.Context, ctx context.Context, id int64) (*contentpb.TopicInfo, bool) {
	resp, err := h.clients.Content.GetTopic(ctx, &contentpb.GetTopicRequest{Key: &contentpb.GetTopicRequest_Id{Id: id}})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	topic := resp.GetTopic()
	if topic == nil {
		writeError(c, http.StatusNotFound, "topic not found", "not_found")
		return nil, false
	}
	if topic.GetAuthorId() != currentUserID(c) {
		writeError(c, http.StatusForbidden, "only the author can modify this topic", "permission_denied")
		return nil, false
	}
	return topic, true
}

func (h *Handler) requireArticleOwner(c *gin.Context, ctx context.Context, id int64) (*contentpb.ArticleInfo, bool) {
	resp, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	article := resp.GetArticle()
	if article == nil {
		writeError(c, http.StatusNotFound, "article not found", "not_found")
		return nil, false
	}
	if article.GetAuthorId() != currentUserID(c) {
		writeError(c, http.StatusForbidden, "only the author can modify this article", "permission_denied")
		return nil, false
	}
	return article, true
}

func (h *Handler) ensureCurrentUserCanCreateContent(c *gin.Context, ctx context.Context) bool {
	user, ok := h.currentUserForContentAction(c, ctx)
	if !ok {
		return false
	}
	return h.ensureUserNotMuted(c, user)
}

func (h *Handler) ensureCurrentUserCanPost(c *gin.Context, ctx context.Context) bool {
	user, ok := h.currentUserForContentAction(c, ctx)
	if !ok {
		return false
	}
	if !h.ensureUserNotMuted(c, user) {
		return false
	}
	if h.emailVerificationRequiredForPosting(ctx) && !userEmailVerified(user) {
		writeError(c, http.StatusForbidden, "请先完成邮箱验证后再发布或评论。", "email_not_verified")
		return false
	}
	return true
}

func (h *Handler) currentUserForContentAction(c *gin.Context, ctx context.Context) (*userpb.UserInfo, bool) {
	resp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return nil, false
	}
	if resp.GetUser() == nil {
		writeError(c, http.StatusUnauthorized, "user not found", "unauthorized")
		return nil, false
	}
	return resp.GetUser(), true
}

func (h *Handler) ensureUserNotMuted(c *gin.Context, user *userpb.UserInfo) bool {
	if user.GetStatus() == userStatusMuted {
		writeError(c, http.StatusForbidden, "user muted", "user_muted")
		return false
	}
	return true
}

func (h *Handler) emailVerificationRequiredForPosting(ctx context.Context) bool {
	if h == nil || h.clients == nil || h.clients.Admin == nil {
		return false
	}
	settings, err := h.loadAuthSettings(ctx, false)
	if err != nil {
		return false
	}
	return settingBool(settings, "auth.email_verification.required", false)
}

func userEmailVerified(user *userpb.UserInfo) bool {
	return user.GetEmailVerified() || user.GetEmailVerifiedAt() > 0
}

// sessionIDClaim reads the jti verbatim apart from surrounding whitespace.
// Session ids are matched byte for byte against the revocation store, so the
// value must not be case-folded even though it is hex today.
func sessionIDClaim(claims jwt.MapClaims) string {
	value, _ := claims["jti"].(string)
	return strings.TrimSpace(value)
}

func normalizedClaimString(claims jwt.MapClaims, key string) string {
	value, _ := claims[key].(string)
	return strings.ToLower(strings.TrimSpace(value))
}

func claimInt64(claims jwt.MapClaims, keys ...string) (int64, bool) {
	for _, key := range keys {
		value, exists := claims[key]
		if !exists {
			continue
		}
		if id, ok := claimValueInt64(value); ok {
			return id, true
		}
	}
	return 0, false
}

func claimValueInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return id, err == nil && id > 0
	case json.Number:
		id, err := v.Int64()
		return id, err == nil && id > 0
	case float64:
		if v <= 0 || v > maxExactIntegerFloat64 || math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
			return 0, false
		}
		return int64(v), true
	case int64:
		return v, v > 0
	case int:
		return int64(v), v > 0
	default:
		return 0, false
	}
}

func rpcContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), requestTimeout)
}

func bindJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body", "bad_request")
		return false
	}
	return true
}

func pathInt64(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		writeError(c, http.StatusBadRequest, "invalid "+name, "bad_request")
		return 0, false
	}
	return value, true
}

func queryInt32(c *gin.Context, name string, fallback int32) int32 {
	value := c.Query(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(parsed)
}

func queryInt64(c *gin.Context, name string, fallback int64) int64 {
	value := c.Query(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryPositiveInt64CSV(c *gin.Context, name string, max int) ([]int64, bool) {
	raw, exists := c.GetQuery(name)
	if !exists {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 0 || len(parts) > max {
		return nil, false
	}
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, false
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, false
	}
	return ids, true
}

func queryBool(c *gin.Context, name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(c.Query(name)))
	switch value {
	case "":
		return fallback
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}

func writeRPCError(c *gin.Context, err error) {
	st := status.Convert(err)
	code := st.Code()
	httpStatus := http.StatusBadGateway
	switch code {
	case codes.InvalidArgument:
		httpStatus = http.StatusBadRequest
	case codes.NotFound:
		httpStatus = http.StatusNotFound
	case codes.AlreadyExists:
		httpStatus = http.StatusConflict
	case codes.Aborted:
		httpStatus = http.StatusConflict
	case codes.FailedPrecondition:
		httpStatus = http.StatusPreconditionFailed
	case codes.Unauthenticated:
		httpStatus = http.StatusUnauthorized
	case codes.PermissionDenied:
		httpStatus = http.StatusForbidden
	case codes.Unavailable:
		httpStatus = http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		httpStatus = http.StatusGatewayTimeout
	case codes.ResourceExhausted:
		httpStatus = http.StatusTooManyRequests
	}
	writeError(c, httpStatus, st.Message(), code.String())
}

func writeError(c *gin.Context, httpStatus int, message string, legacyCode string) {
	apiErr := newHTTPException(httpStatus, message)
	if legacyCode != "" {
		apiErr.WithMeta("legacy_code", legacyCode)
	}
	response.Failed(c, apiErr)
}

func newHTTPException(httpStatus int, message string) *exception.ApiException {
	switch httpStatus {
	case http.StatusBadRequest:
		return exception.NewBadRequest("%s", message)
	case http.StatusUnauthorized:
		return exception.NewUnauthorized("%s", message)
	case http.StatusForbidden:
		return exception.NewPermissionDeny("%s", message)
	case http.StatusNotFound:
		return exception.NewNotFound("%s", message)
	case http.StatusConflict:
		return exception.NewConflict("%s", message)
	case http.StatusInternalServerError:
		return exception.NewInternalServerError("%s", message)
	default:
		return exception.NewApiException(httpStatus, http.StatusText(httpStatus)).WithMessage(message).WithHttpCode(httpStatus)
	}
}
