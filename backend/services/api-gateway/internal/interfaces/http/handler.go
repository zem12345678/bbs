package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"api-gateway/internal/clients"
	"api-gateway/internal/clients/pb/adminpb"
	"api-gateway/internal/clients/pb/commentpb"
	"api-gateway/internal/clients/pb/contentpb"
	"api-gateway/internal/clients/pb/creditpb"
	"api-gateway/internal/clients/pb/feedpb"
	"api-gateway/internal/clients/pb/notificationpb"
	"api-gateway/internal/clients/pb/reactionpb"
	"api-gateway/internal/clients/pb/searchpb"
	"api-gateway/internal/clients/pb/userpb"
	iochttp "api-gateway/internal/ioc/http"
	"api-gateway/pkg/exception"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const requestTimeout = 10 * time.Second
const (
	userStatusMuted int32 = 2
)

type Handler struct {
	clients     *clients.Clients
	tokenHeader string
	tokenPrefix string
	jwtSecret   []byte
}

func NewHandler(clients *clients.Clients, tokenHeader string, tokenPrefix string, jwtSecret string) *Handler {
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	if tokenPrefix == "" {
		tokenPrefix = "Bearer"
	}
	return &Handler{clients: clients, tokenHeader: tokenHeader, tokenPrefix: tokenPrefix, jwtSecret: []byte(jwtSecret)}
}

func NewInitControllers(h *Handler) iochttp.InitControllers {
	return func(r *gin.Engine) {
		r.GET("/healthz", h.health)
		r.Static("/uploads", "uploads")
		api := r.Group("/api/v1")
		api.POST("/auth/register", h.register)
		api.POST("/auth/login", h.login)
		api.POST("/admin/auth/login", h.adminLogin)
		api.GET("/admin/auth/profile", h.requireAdminAuth(), h.adminProfile)
		api.PUT("/admin/auth/profile", h.requireAdminAuth(), h.updateAdminProfile)
		api.GET("/admin/auth/menus", h.requireAdminAuth(), h.listCurrentAdminMenus)
		api.GET("/admin/overview", h.requireAdminAuth(), h.adminOverview)
		api.POST("/admin/uploads/avatar", h.requireAdminAuth(), h.uploadAdminAvatar)
		api.GET("/users/me", h.requireAuth(), h.getMe)
		api.GET("/users/current/likes", h.requireAuth(), h.listCurrentUserLikes)
		api.GET("/users/current/favorites", h.requireAuth(), h.listCurrentUserFavorites)
		api.PUT("/users/me", h.requireAuth(), h.updateMe)
		api.POST("/users/me/password", h.requireAuth(), h.changePassword)
		api.GET("/users/by-username/:username", h.getUserByUsername)
		api.GET("/users/:id/badges", h.listUserBadges)
		api.GET("/users/:id", h.getUser)
		api.GET("/users/:id/followers", h.listFollowers)
		api.GET("/users/:id/following", h.listFollowing)
		api.POST("/users/:id/follow", h.requireAuth(), h.follow)
		api.DELETE("/users/:id/follow", h.requireAuth(), h.unfollow)
		api.GET("/users/:id/following-state", h.requireAuth(), h.isFollowing)

		api.POST("/topics", h.requireAuth(), h.createTopic)
		api.GET("/topics", h.listTopics)
		api.GET("/topics/:id", h.getTopic)
		api.PUT("/topics/:id", h.requireAuth(), h.updateTopic)
		api.POST("/topics/:id/publish", h.requireAuth(), h.publishTopic)
		api.DELETE("/topics/:id", h.requireAuth(), h.archiveTopic)
		api.POST("/topics/:id/comments", h.requireAuth(), h.createTopicComment)
		api.GET("/topics/:id/comments", h.listTopicComments)
		api.POST("/topics/:id/like", h.requireAuth(), h.likeTopic)
		api.DELETE("/topics/:id/like", h.requireAuth(), h.unlikeTopic)
		api.POST("/topics/:id/favorite", h.requireAuth(), h.favoriteTopic)
		api.DELETE("/topics/:id/favorite", h.requireAuth(), h.unfavoriteTopic)
		api.POST("/topics/:id/report", h.requireAuth(), h.reportTopic)
		api.GET("/topics/:id/reactions", h.getTopicReactions)
		api.GET("/categories", h.listCategories)
		api.GET("/categories/:id", h.getCategory)
		api.GET("/links", h.listLinks)
		api.GET("/tasks", h.listTasks)

		api.POST("/articles", h.requireAuth(), h.createArticle)
		api.GET("/articles", h.listArticles)
		api.GET("/feed", h.feedArticles)
		api.GET("/articles/:id", h.getArticle)
		api.PUT("/articles/:id", h.requireAuth(), h.updateArticle)
		api.POST("/articles/:id/publish", h.requireAuth(), h.publishArticle)
		api.POST("/articles/:id/hide", h.requireAuth(), h.hideArticle)
		api.POST("/articles/:id/archive", h.requireAuth(), h.archiveArticle)
		api.DELETE("/articles/:id", h.requireAuth(), h.archiveArticle)
		api.GET("/tags", h.listTags)
		api.POST("/tags/autocomplete", h.autocompleteTags)

		api.POST("/articles/:id/comments", h.requireAuth(), h.createComment)
		api.GET("/articles/:id/comments", h.listComments)
		api.GET("/comments/:id/replies", h.listReplies)
		api.DELETE("/comments/:id", h.requireAuth(), h.deleteComment)

		api.POST("/articles/:id/like", h.requireAuth(), h.likeArticle)
		api.DELETE("/articles/:id/like", h.requireAuth(), h.unlikeArticle)
		api.POST("/articles/:id/favorite", h.requireAuth(), h.favoriteArticle)
		api.DELETE("/articles/:id/favorite", h.requireAuth(), h.unfavoriteArticle)
		api.POST("/articles/:id/report", h.requireAuth(), h.reportArticle)
		api.GET("/articles/:id/reactions", h.getArticleReactions)
		api.GET("/search/articles", h.searchArticles)
		api.GET("/search/topics", h.searchTopics)

		api.GET("/notifications", h.requireAuth(), h.listNotifications)
		api.GET("/notifications/unread-count", h.requireAuth(), h.countUnreadNotifications)
		api.POST("/notifications/read-all", h.requireAuth(), h.markAllNotificationsRead)
		api.POST("/notifications/:id/read", h.requireAuth(), h.markNotificationRead)

		api.GET("/credits/balance", h.requireAuth(), h.getCreditBalance)
		api.GET("/credits/ledger", h.requireAuth(), h.listCreditLedger)

		api.GET("/admin/reports", h.requireAdminAuth(), h.listReports)
		api.POST("/admin/reports/:id/audit", h.requireAdminAuth(), h.auditReport)
		api.GET("/admin/users", h.requireAdminAuth(), h.listGovernanceUsers)
		api.POST("/admin/users/:id/mute", h.requireAdminAuth(), h.muteUser)
		api.POST("/admin/users/:id/unmute", h.requireAdminAuth(), h.unmuteUser)
		api.GET("/admin/categories", h.requireAdminAuth(), h.listAdminCategories)
		api.POST("/admin/categories", h.requireAdminAuth(), h.createAdminCategory)
		api.PUT("/admin/categories/:id", h.requireAdminAuth(), h.updateAdminCategory)
		api.DELETE("/admin/categories/:id", h.requireAdminAuth(), h.deleteAdminCategory)
		api.GET("/admin/badges", h.requireAdminAuth(), h.listAdminBadges)
		api.POST("/admin/badges", h.requireAdminAuth(), h.createAdminBadge)
		api.PUT("/admin/badges/:id", h.requireAdminAuth(), h.updateAdminBadge)
		api.DELETE("/admin/badges/:id", h.requireAdminAuth(), h.deleteAdminBadge)
		api.GET("/admin/levels", h.requireAdminAuth(), h.listAdminLevels)
		api.POST("/admin/levels", h.requireAdminAuth(), h.createAdminLevel)
		api.PUT("/admin/levels/:id", h.requireAdminAuth(), h.updateAdminLevel)
		api.DELETE("/admin/levels/:id", h.requireAdminAuth(), h.deleteAdminLevel)
		api.GET("/admin/links", h.requireAdminAuth(), h.listAdminLinks)
		api.POST("/admin/links", h.requireAdminAuth(), h.createAdminLink)
		api.PUT("/admin/links/:id", h.requireAdminAuth(), h.updateAdminLink)
		api.DELETE("/admin/links/:id", h.requireAdminAuth(), h.deleteAdminLink)
		api.GET("/admin/tasks", h.requireAdminAuth(), h.listAdminTasks)
		api.POST("/admin/tasks", h.requireAdminAuth(), h.createAdminTask)
		api.PUT("/admin/tasks/:id", h.requireAdminAuth(), h.updateAdminTask)
		api.DELETE("/admin/tasks/:id", h.requireAdminAuth(), h.deleteAdminTask)
		api.GET("/admin/forbidden-words", h.requireAdminAuth(), h.listForbiddenWords)
		api.POST("/admin/forbidden-words", h.requireAdminAuth(), h.createForbiddenWord)
		api.PUT("/admin/forbidden-words/:id", h.requireAdminAuth(), h.updateForbiddenWord)
		api.DELETE("/admin/forbidden-words/:id", h.requireAdminAuth(), h.deleteForbiddenWord)
		api.GET("/admin/settings", h.requireAdminAuth(), h.listSettings)
		api.PUT("/admin/settings/:key", h.requireAdminAuth(), h.updateSetting)
		api.GET("/admin/email-logs", h.requireAdminAuth(), h.listEmailLogs)
		api.GET("/admin/login-logs", h.requireAdminAuth(), h.listLoginLogs)
		api.GET("/admin/operation-logs", h.requireAdminAuth(), h.listOperationLogs)
		api.GET("/admin/articles", h.requireAdminAuth(), h.listAdminArticles)
		api.POST("/admin/articles/:id/publish", h.requireAdminAuth(), h.publishAdminArticle)
		api.POST("/admin/articles/:id/hide", h.requireAdminAuth(), h.hideAdminArticle)
		api.POST("/admin/articles/:id/archive", h.requireAdminAuth(), h.archiveAdminArticle)
		api.GET("/admin/topics", h.requireAdminAuth(), h.listAdminTopics)
		api.POST("/admin/topics/:id/publish", h.requireAdminAuth(), h.publishAdminTopic)
		api.POST("/admin/topics/:id/hide", h.requireAdminAuth(), h.hideAdminTopic)
		api.POST("/admin/topics/:id/archive", h.requireAdminAuth(), h.archiveAdminTopic)
		api.GET("/admin/comments", h.requireAdminAuth(), h.listAdminComments)
		api.POST("/admin/comments/:id/hide", h.requireAdminAuth(), h.hideAdminComment)
		api.POST("/admin/comments/:id/restore", h.requireAdminAuth(), h.restoreAdminComment)
		api.GET("/admin/rbac/users", h.requireAdminAuth(), h.listAdminUsers)
		api.POST("/admin/rbac/users", h.requireAdminAuth(), h.createAdminUser)
		api.GET("/admin/rbac/roles", h.requireAdminAuth(), h.listAdminRoles)
		api.PUT("/admin/rbac/users/:id/roles", h.requireAdminAuth(), h.assignAdminRoles)
		api.GET("/admin/system/users", h.requireAdminAuth(), h.listSystemUsers)
		api.GET("/admin/system/users/:id", h.requireAdminAuth(), h.getSystemUser)
		api.POST("/admin/system/users", h.requireAdminAuth(), h.createSystemUser)
		api.PUT("/admin/system/users/:id", h.requireAdminAuth(), h.updateSystemUser)
		api.DELETE("/admin/system/users/:id", h.requireAdminAuth(), h.deleteSystemUser)
		api.PUT("/admin/system/users/:id/password", h.requireAdminAuth(), h.resetSystemUserPassword)
		api.PUT("/admin/system/users/:id/roles", h.requireAdminAuth(), h.assignSystemUserRoles)
		api.GET("/admin/system/roles", h.requireAdminAuth(), h.listSystemRoles)
		api.POST("/admin/system/roles", h.requireAdminAuth(), h.createSystemRole)
		api.PUT("/admin/system/roles/:id", h.requireAdminAuth(), h.updateSystemRole)
		api.DELETE("/admin/system/roles/:id", h.requireAdminAuth(), h.deleteSystemRole)
		api.GET("/admin/system/roles/:id/menu-ids", h.requireAdminAuth(), h.getSystemRoleMenuIDs)
		api.GET("/admin/system/roles/:id/permissions", h.requireAdminAuth(), h.getSystemRolePermissions)
		api.PUT("/admin/system/roles/:id/menus", h.requireAdminAuth(), h.assignSystemRoleMenus)
		api.PUT("/admin/system/roles/:id/permissions", h.requireAdminAuth(), h.assignSystemRoleMenus)
		api.GET("/admin/system/menus", h.requireAdminAuth(), h.listSystemMenus)
		api.POST("/admin/system/menus", h.requireAdminAuth(), h.createSystemMenu)
		api.PUT("/admin/system/menus/:id", h.requireAdminAuth(), h.updateSystemMenu)
		api.DELETE("/admin/system/menus/:id", h.requireAdminAuth(), h.deleteSystemMenu)
		api.GET("/admin/system/depts", h.requireAdminAuth(), h.listSystemDepts)
		api.POST("/admin/system/depts", h.requireAdminAuth(), h.createSystemDept)
		api.PUT("/admin/system/depts/:id", h.requireAdminAuth(), h.updateSystemDept)
		api.DELETE("/admin/system/depts/:id", h.requireAdminAuth(), h.deleteSystemDept)
		api.GET("/admin/system/departments", h.requireAdminAuth(), h.listSystemDepts)
		api.POST("/admin/system/departments", h.requireAdminAuth(), h.createSystemDept)
		api.PUT("/admin/system/departments/:id", h.requireAdminAuth(), h.updateSystemDept)
		api.DELETE("/admin/system/departments/:id", h.requireAdminAuth(), h.deleteSystemDept)
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
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.Register(ctx, &userpb.RegisterRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		Nickname: req.Nickname,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.Login(ctx, &userpb.LoginRequest{Account: req.Account, Password: req.Password})
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

func (h *Handler) uploadAdminAvatar(c *gin.Context) {
	const maxAvatarSize = int64(5 << 20)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarSize)
	file, err := c.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "missing avatar file", "bad_request")
		return
	}
	if file.Size <= 0 || file.Size > maxAvatarSize {
		writeError(c, http.StatusBadRequest, "avatar file size must be between 1 byte and 5 MiB", "bad_request")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedAvatarExt(ext) {
		writeError(c, http.StatusBadRequest, "avatar file type is not supported", "bad_request")
		return
	}
	name, err := uploadedAvatarName(ext)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "create avatar name failed", "internal_error")
		return
	}
	dir := filepath.Join("uploads", "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(c, http.StatusInternalServerError, "create avatar directory failed", "internal_error")
		return
	}
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		writeError(c, http.StatusInternalServerError, "save avatar failed", "internal_error")
		return
	}
	path := "/uploads/avatars/" + name
	response.Success(c, gin.H{"url": publicRequestURL(c, path), "path": path, "avatar_url": publicRequestURL(c, path)})
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
	response.Success(c, resp)
}

func (h *Handler) getUserByUsername(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.GetUserByUsername(ctx, &userpb.UsernameRequest{Username: c.Param("username")})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
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
	badges, err := h.clients.Admin.ListBadges(ctx, &adminpb.ListBadgesRequest{Status: 2, Limit: 100, Offset: 0})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := buildUserBadges(resp.GetUser(), badges.GetItems())
	total := len(items)
	items = paginateBadgeRows(items, int(queryInt32(c, "limit", 20)), int(queryInt32(c, "offset", 0)))
	response.Success(c, gin.H{"items": items, "total": total})
}

func (h *Handler) getMe(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
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
	resp, err := h.clients.User.UpdateProfile(ctx, &userpb.UpdateProfileRequest{Id: currentUserID(c), Nickname: req.Nickname, AvatarUrl: req.AvatarURL, Bio: req.Bio})
	if err != nil {
		writeRPCError(c, err)
		return
	}
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
	response.Success(c, resp)
}

func (h *Handler) createTopic(c *gin.Context) {
	var req createTopicRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
	}
	resp, err := h.clients.Content.CreateTopic(ctx, &contentpb.CreateTopicRequest{
		Slug: req.Slug, Type: req.Type, Title: req.Title, Body: req.Body, Tags: req.Tags, AuthorId: currentUserID(c), CategoryId: req.CategoryID,
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
	if _, ok := h.requireTopicOwner(c, ctx, id); !ok {
		return
	}
	resp, err := h.clients.Content.UpdateTopic(ctx, &contentpb.UpdateTopicRequest{Id: id, Title: req.Title, Body: req.Body, Tags: req.Tags, CategoryId: req.CategoryID})
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
	if _, ok := h.requireTopicOwner(c, ctx, id); !ok {
		return
	}
	if !h.ensureCurrentUserCanPost(c, ctx) {
		return
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

func (h *Handler) getTopic(c *gin.Context) {
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

func (h *Handler) listTopics(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{
		Status:     queryInt32(c, "status", 2),
		Type:       c.Query("type"),
		Tag:        c.Query("tag"),
		AuthorId:   queryInt64(c, "author_id", 0),
		Limit:      queryInt32(c, "limit", 20),
		Offset:     queryInt32(c, "offset", 0),
		CategoryId: queryInt64(c, "category_id", 0),
	})
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
		Status: queryInt32(c, "status", 2),
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
	resp, err := h.clients.Admin.ListTasks(ctx, &adminpb.ListTasksRequest{
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

func (h *Handler) createTopicComment(c *gin.Context) {
	h.createEntityComment(c, "topic")
}

func (h *Handler) listTopicComments(c *gin.Context) {
	h.listEntityComments(c, "topic")
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
	if !h.ensureCurrentUserCanPost(c, ctx) {
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
	if _, ok := h.requireArticleOwner(c, ctx, id); !ok {
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
	resp, err := h.clients.Content.GetArticle(ctx, &contentpb.GetArticleRequest{Key: &contentpb.GetArticleRequest_Id{Id: id}})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listArticles(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{
		Status: queryInt32(c, "status", 0), Tag: c.Query("tag"), AuthorId: queryInt64(c, "author_id", 0), Limit: queryInt32(c, "limit", 20), Offset: queryInt32(c, "offset", 0),
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
	req := &feedpb.ListFeedRequest{Limit: queryInt32(c, "limit", 20), Offset: queryInt32(c, "offset", 0)}
	var (
		resp *feedpb.FeedListResponse
		err  error
	)
	if c.Query("sort") == "hot" {
		resp, err = h.clients.Feed.ListHot(ctx, req)
	} else {
		resp, err = h.clients.Feed.ListLatest(ctx, req)
	}
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listTags(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListTags(ctx, &contentpb.ListTagsRequest{
		Limit: queryInt32(c, "limit", 12),
		Query: c.Query("q"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) autocompleteTags(c *gin.Context) {
	var req autocompleteTagsRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.AutocompleteTags(ctx, &contentpb.AutocompleteTagsRequest{
		Query: req.Query,
		Limit: req.Limit,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
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
	resp, err := h.clients.Comment.CreateComment(ctx, &commentpb.CreateCommentRequest{EntityType: entityType, EntityId: entityID, ParentId: req.ParentID, AuthorId: currentUserID(c), Content: req.Content})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
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
	resp, err := h.clients.Comment.ListComments(ctx, &commentpb.ListCommentsRequest{EntityType: entityType, EntityId: entityID, Page: queryInt32(c, "page", 1), PageSize: queryInt32(c, "page_size", 20)})
	if err != nil {
		writeRPCError(c, err)
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
	resp, err := h.clients.Comment.ListReplies(ctx, &commentpb.ListRepliesRequest{RootId: rootID, Page: queryInt32(c, "page", 1), PageSize: queryInt32(c, "page_size", 20)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
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

func (h *Handler) searchArticles(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		writeError(c, http.StatusBadRequest, "q is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Search.SearchArticles(ctx, &searchpb.SearchArticlesRequest{Keyword: keyword, Page: queryInt32(c, "page", 1), PageSize: queryInt32(c, "page_size", 20)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) searchTopics(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		writeError(c, http.StatusBadRequest, "q is required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Search.SearchTopics(ctx, &searchpb.SearchTopicsRequest{Keyword: keyword, Page: queryInt32(c, "page", 1), PageSize: queryInt32(c, "page_size", 20)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
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
	response.Success(c, resp)
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
		Actor:     currentActor(c),
		Id:        id,
		Status:    req.Status,
		AuditNote: req.AuditNote,
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
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listEmailLogs(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListEmailLogs(ctx, &adminpb.ListEmailLogsRequest{
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
	items := resp.GetItems()
	if items == nil {
		items = []*adminpb.EmailLogInfo{}
	}
	response.Success(c, gin.H{"items": items, "total": resp.GetTotal()})
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

func (h *Handler) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, err := h.authIdentityFromRequest(c)
		if err != nil {
			writeError(c, http.StatusUnauthorized, err.Error(), "unauthorized")
			c.Abort()
			return
		}
		c.Set("user_id", identity.userID)
		c.Set("username", identity.username)
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

type authIdentity struct {
	userID   int64
	username string
}

func (h *Handler) authIdentityFromRequest(c *gin.Context) (authIdentity, error) {
	header, err := h.authTokenFromRequest(c)
	if err != nil {
		return authIdentity{}, err
	}
	token, err := jwt.Parse(header, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return authIdentity{}, errors.New("invalid authorization token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return authIdentity{}, errors.New("invalid authorization claims")
	}
	identity := authIdentity{username: normalizedClaimString(claims, "username")}
	if value, ok := claims["user_id"].(float64); ok {
		identity.userID = int64(value)
		return identity, nil
	}
	if value, ok := claims["sub"].(string); ok {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return authIdentity{}, err
		}
		identity.userID = id
		return identity, nil
	}
	return authIdentity{}, errors.New("missing user id claim")
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

func (h *Handler) ensureCurrentUserCanPost(c *gin.Context, ctx context.Context) bool {
	resp, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return false
	}
	if resp.GetUser() == nil {
		writeError(c, http.StatusUnauthorized, "user not found", "unauthorized")
		return false
	}
	if resp.GetUser().GetStatus() == userStatusMuted {
		writeError(c, http.StatusForbidden, "user muted", "user_muted")
		return false
	}
	return true
}

func normalizedClaimString(claims jwt.MapClaims, key string) string {
	value, _ := claims[key].(string)
	return strings.ToLower(strings.TrimSpace(value))
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
