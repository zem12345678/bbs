package http

import "encoding/json"

type registerRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	InviteCode string `json:"invite_code"`
}

type loginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type completeMFALoginRequest struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

type beginTOTPEnrollmentRequest struct {
	Password    string `json:"password"`
	CurrentCode string `json:"current_code"`
}

type confirmTOTPEnrollmentRequest struct {
	Code string `json:"code"`
}

type mfaReauthenticateRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

type beginPasskeyRegistrationRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type finishPasskeyRegistrationRequest struct {
	Challenge  string          `json:"challenge"`
	Credential json.RawMessage `json:"credential"`
}

type updatePasskeyRequest struct {
	Name string `json:"name"`
}

type deletePasskeyRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

type setPasskeyPasswordlessRequest struct {
	Enabled  bool   `json:"enabled"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type requestAccountDeletionRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

type beginPasskeyMFARequest struct {
	MFAChallenge string `json:"mfa_challenge"`
}

type completePasskeyLoginRequest struct {
	Challenge  string          `json:"challenge"`
	Credential json.RawMessage `json:"credential"`
}

type adminLoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type adminRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type updateProfileRequest struct {
	Nickname            string          `json:"nickname"`
	AvatarURL           string          `json:"avatar_url"`
	BackgroundURL       string          `json:"background_url"`
	ProfileTheme        *string         `json:"profile_theme"`
	Email               string          `json:"email"`
	Phone               string          `json:"phone"`
	Bio                 string          `json:"bio"`
	Description         string          `json:"description"`
	Birthday            json.RawMessage `json:"birthday"`
	FollowingVisibility json.RawMessage `json:"following_visibility"`
	FollowersVisibility json.RawMessage `json:"followers_visibility"`
}

type updateUserMemoRequest struct {
	UserID jsonInt64       `json:"userId"`
	Memo   json.RawMessage `json:"memo"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
	MFACode     string `json:"mfa_code"`
}

type adminAdjustCreditsRequest struct {
	Delta         int64  `json:"delta"`
	Reason        string `json:"reason"`
	Description   string `json:"description"`
	SourceEventID string `json:"source_event_id"`
}

type adminCloseExpiredMallOrdersRequest struct {
	ExpireAfterSeconds int64 `json:"expire_after_seconds"`
	Limit              int32 `json:"limit"`
}

type adminRecoverStalePayingMallOrdersRequest struct {
	StaleAfterSeconds int64 `json:"stale_after_seconds"`
	Limit             int32 `json:"limit"`
}

type adminRequeueMallOutboxRequest struct {
	Statuses []string `json:"statuses"`
	Limit    int32    `json:"limit"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type createArticleRequest struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Body     string   `json:"body"`
	CoverURL string   `json:"cover_url"`
	Tags     []string `json:"tags"`
	Publish  bool     `json:"publish"`
}

type updateArticleRequest struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Body     string   `json:"body"`
	CoverURL string   `json:"cover_url"`
	Tags     []string `json:"tags"`
}

type createTopicRequest struct {
	Slug        string            `json:"slug"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Tags        []string          `json:"tags"`
	CategoryID  jsonInt64         `json:"category_id"`
	ChannelID   jsonInt64         `json:"channel_id"`
	BountyScore int64             `json:"bounty_score"`
	Publish     bool              `json:"publish"`
	Poll        *topicPollRequest `json:"poll"`
}

type updateTopicRequest struct {
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Tags        []string          `json:"tags"`
	CategoryID  jsonInt64         `json:"category_id"`
	ChannelID   jsonInt64         `json:"channel_id"`
	BountyScore int64             `json:"bounty_score"`
	Poll        *topicPollRequest `json:"poll"`
}

type channelRequest struct {
	CategoryID  jsonInt64 `json:"category_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
}

type setAdminChannelFeaturedRequest struct {
	Featured *bool `json:"featured"`
}

type setAdminChannelArchivedRequest struct {
	Archived *bool `json:"archived"`
}

type topicPollRequest struct {
	Enabled   bool      `json:"enabled"`
	Multiple  bool      `json:"multiple"`
	Choices   []string  `json:"choices"`
	ExpiresAt jsonInt64 `json:"expires_at"`
}

type voteTopicPollRequest struct {
	Choices []int32 `json:"choices"`
}

type updateAttachmentPriceRequest struct {
	PriceCredits *jsonInt64 `json:"price_credits"`
}

type createCommentRequest struct {
	ParentID jsonInt64 `json:"parent_id"`
	Content  string    `json:"content"`
}

type autocompleteTagsRequest struct {
	Query string `json:"query"`
	Limit int32  `json:"limit"`
}

type createCollectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type updateCollectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type collectionItemRequest struct {
	EntityType string    `json:"entity_type"`
	EntityID   jsonInt64 `json:"entity_id"`
}

type submitReportRequest struct {
	Reason      string `json:"reason"`
	Description string `json:"description"`
}

type auditReportRequest struct {
	Status       int32  `json:"status"`
	AuditNote    string `json:"audit_note"`
	TargetAction string `json:"target_action"`
}

type createAdminUserRequest struct {
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Password string   `json:"password"`
	Nickname string   `json:"nickname"`
	RoleKeys []string `json:"role_keys"`
}

type assignRolesRequest struct {
	RoleKeys []string `json:"role_keys"`
}

type upsertAdminCategoryRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int32  `json:"status"`
	Sort        int32  `json:"sort"`
}

type upsertAdminBadgeRequest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
	RuleType    string `json:"rule_type"`
	RuleValue   int64  `json:"rule_value"`
	Status      int32  `json:"status"`
	Sort        int32  `json:"sort"`
}

type upsertAdminLevelRequest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MinScore    int64  `json:"min_score"`
	MaxScore    int64  `json:"max_score"`
	Status      int32  `json:"status"`
	Sort        int32  `json:"sort"`
}

type upsertForbiddenWordRequest struct {
	Word        string `json:"word"`
	Scene       string `json:"scene"`
	Action      string `json:"action"`
	Replacement string `json:"replacement"`
	Description string `json:"description"`
	Status      int32  `json:"status"`
}

type upsertSettingRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Group       string `json:"group"`
	ValueType   string `json:"value_type"`
	Description string `json:"description"`
	Status      int32  `json:"status"`
	ClearValue  bool   `json:"clear_value"`
}

type createInviteCodesRequest struct {
	Count     int32     `json:"count"`
	ExpiresAt jsonInt64 `json:"expires_at"`
}

type upsertAdminLinkRequest struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Status      int32  `json:"status"`
	Sort        int32  `json:"sort"`
}

type upsertAdminTaskRequest struct {
	Key          string `json:"key"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	RewardPoints int64  `json:"reward_points"`
	Status       int32  `json:"status"`
	Sort         int32  `json:"sort"`
}
