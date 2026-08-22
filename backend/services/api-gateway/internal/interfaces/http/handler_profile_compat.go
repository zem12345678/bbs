package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

// profileCompatRequest contains the profile fields that BBS persists and that
// Misskey-compatible clients commonly send. RawMessage preserves omitted vs
// explicit null values for nullable fields.
type profileCompatRequest struct {
	Name                json.RawMessage `json:"name"`
	Description         json.RawMessage `json:"description"`
	Birthday            json.RawMessage `json:"birthday"`
	FollowingVisibility json.RawMessage `json:"followingVisibility"`
	FollowersVisibility json.RawMessage `json:"followersVisibility"`
}

func (h *Handler) updateProfileCompat(c *gin.Context) {
	var request profileCompatRequest
	if !decodeSensitiveAccountCompatRequest(c, &request) {
		writeSensitiveAccountCompatInvalidParam(c)
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	currentResponse, err := h.clients.User.GetUser(ctx, &userpb.UserIDRequest{Id: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	current := currentResponse.GetUser()
	if current == nil {
		writeError(c, stdhttp.StatusBadGateway, "user service returned an empty user", "internal_error")
		return
	}

	name, nameSet, nameValid := nullableJSONText(request.Name)
	if !nameValid || (nameSet && (name == nil || strings.TrimSpace(*name) == "")) {
		writeSensitiveAccountCompatInvalidParam(c)
		return
	}
	description, descriptionSet, descriptionValid := nullableJSONText(request.Description)
	if !descriptionValid {
		writeSensitiveAccountCompatInvalidParam(c)
		return
	}
	birthday, birthdaySet, birthdayValid := nullableJSONText(request.Birthday)
	if !birthdayValid {
		writeSensitiveAccountCompatInvalidParam(c)
		return
	}
	followingVisibility, followingSet, followingValid := nullableJSONText(request.FollowingVisibility)
	followersVisibility, followersSet, followersValid := nullableJSONText(request.FollowersVisibility)
	if !followingValid || !followersValid || (followingSet && followingVisibility == nil) || (followersSet && followersVisibility == nil) {
		writeSensitiveAccountCompatInvalidParam(c)
		return
	}

	nickname := current.GetNickname()
	if nameSet {
		nickname = *name
	}
	bio := current.GetBio()
	if descriptionSet {
		if description == nil {
			bio = ""
		} else {
			bio = *description
		}
	}
	update := &userpb.UpdateProfileRequest{
		Id: current.GetId(), Nickname: nickname, AvatarUrl: current.GetAvatarUrl(),
		BackgroundUrl: current.GetBackgroundUrl(), ProfileTheme: current.GetProfileTheme(), Bio: bio,
	}
	if birthdaySet {
		if birthday == nil {
			cleared := ""
			update.Birthday = &cleared
		} else {
			update.Birthday = birthday
		}
	}
	if followingSet {
		update.FollowingVisibility = followingVisibility
	}
	if followersSet {
		update.FollowersVisibility = followersVisibility
	}
	updatedResponse, err := h.clients.User.UpdateProfile(ctx, update)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	updated := updatedResponse.GetUser()
	if updated == nil {
		writeError(c, stdhttp.StatusBadGateway, "user service returned an empty user", "internal_error")
		return
	}
	h.sanitizeUserProfileTheme(ctx, updated)
	c.JSON(stdhttp.StatusOK, toMisskeyUserLite(updated))
}
