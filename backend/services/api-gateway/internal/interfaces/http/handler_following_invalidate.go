package http

import (
	stdhttp "net/http"
	"strconv"
	"strings"

	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

type followingInvalidateRequest struct {
	UserID jsonInt64 `json:"userId"`
}

type misskeyUserLite struct {
	ID                  string            `json:"id"`
	Name                *string           `json:"name"`
	Username            string            `json:"username"`
	Host                *string           `json:"host"`
	CreatedAt           string            `json:"createdAt"`
	UpdatedAt           *string           `json:"updatedAt"`
	LastFetchedAt       *string           `json:"lastFetchedAt"`
	Approved            bool              `json:"approved"`
	Description         *string           `json:"description"`
	AvatarURL           *string           `json:"avatarUrl"`
	AvatarBlurhash      *string           `json:"avatarBlurhash"`
	AvatarDecorations   []any             `json:"avatarDecorations"`
	NoIndex             bool              `json:"noindex"`
	EnableRSS           bool              `json:"enableRss"`
	MandatoryCW         *string           `json:"mandatoryCW"`
	IsSilenced          bool              `json:"isSilenced"`
	BypassSilence       bool              `json:"bypassSilence"`
	FollowersCount      int64             `json:"followersCount"`
	FollowingCount      int64             `json:"followingCount"`
	NotesCount          int64             `json:"notesCount"`
	LevelScore          int64             `json:"levelScore"`
	Level               int64             `json:"level"`
	LevelProgress       int64             `json:"levelProgress"`
	LevelProgressRate   int64             `json:"levelProgressRate"`
	LevelTotalScore     int64             `json:"levelTotalScore"`
	LevelTitle          string            `json:"levelTitle"`
	LevelColor          string            `json:"levelColor"`
	Emojis              map[string]string `json:"emojis"`
	OnlineStatus        string            `json:"onlineStatus"`
	AttributionDomains  []string          `json:"attributionDomains"`
	Birthday            *string           `json:"birthday"`
	FollowingVisibility string            `json:"followingVisibility"`
	FollowersVisibility string            `json:"followersVisibility"`
	IsLocked            bool              `json:"isLocked"`
}

func (h *Handler) removeFollower(c *gin.Context) {
	followerID, ok := pathInt64(c, "followerId")
	if !ok {
		return
	}
	h.invalidateFollowing(c, followerID, false)
}

func (h *Handler) invalidateFollowingCompat(c *gin.Context) {
	var req followingInvalidateRequest
	if !bindJSON(c, &req) {
		return
	}
	followerID := req.UserID.Int64()
	if followerID <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "userId must be a positive integer", "bad_request")
		return
	}
	h.invalidateFollowing(c, followerID, true)
}

func (h *Handler) invalidateFollowing(c *gin.Context, followerID int64, compatibility bool) {
	currentUser := currentUserID(c)
	if followerID == currentUser {
		writeError(c, stdhttp.StatusBadRequest, "follower cannot be yourself", "invalid_argument")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.User.Unfollow(ctx, &userpb.FollowRequest{FollowerId: followerID, FolloweeId: currentUser}); err != nil {
		writeRPCError(c, err)
		return
	}
	result, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: followerID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	user := result.GetUser()
	if user == nil {
		writeError(c, stdhttp.StatusInternalServerError, "user service returned an empty user", "internal_error")
		return
	}
	h.sanitizeUserProfileTheme(ctx, user)
	if compatibility {
		c.JSON(stdhttp.StatusOK, toMisskeyUserLite(user))
		return
	}
	response.Success(c, publicUserResponse{Success: true, User: toPublicUserView(user)})
}

func toMisskeyUserLite(user *userpb.UserInfo) misskeyUserLite {
	return misskeyUserLite{
		ID: strconv.FormatInt(user.GetId(), 10), Name: optionalMisskeyText(user.GetNickname()), Username: user.GetUsername(),
		CreatedAt: formatUnixMilli(user.GetCreatedAt()), UpdatedAt: formatUnixMilliPointer(user.GetUpdatedAt()), Approved: true,
		Description: optionalMisskeyText(user.GetBio()), AvatarURL: optionalMisskeyText(user.GetAvatarUrl()),
		AvatarDecorations: []any{}, EnableRSS: true, IsSilenced: user.GetStatus() == userStatusMuted,
		FollowersCount: user.GetFollowerCount(), FollowingCount: user.GetFollowingCount(), Level: 1, LevelTotalScore: 10000,
		LevelTitle: "Community member", LevelColor: "gray", Emojis: map[string]string{}, OnlineStatus: "unknown", AttributionDomains: []string{},
		Birthday:            optionalMisskeyText(user.GetBirthday()),
		FollowingVisibility: profileVisibilityOrPublic(user.GetFollowingVisibility()),
		FollowersVisibility: profileVisibilityOrPublic(user.GetFollowersVisibility()),
		IsLocked:            user.GetFollowApprovalRequired(),
	}
}

func optionalMisskeyText(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
