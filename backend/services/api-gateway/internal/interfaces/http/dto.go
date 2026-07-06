package http

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type adminLoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type updateProfileRequest struct {
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatar_url"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Bio         string `json:"bio"`
	Description string `json:"description"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
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
	Slug       string   `json:"slug"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Tags       []string `json:"tags"`
	CategoryID int64    `json:"category_id"`
	Publish    bool     `json:"publish"`
}

type updateTopicRequest struct {
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Tags       []string `json:"tags"`
	CategoryID int64    `json:"category_id"`
}

type createCommentRequest struct {
	ParentID int64  `json:"parent_id"`
	Content  string `json:"content"`
}

type autocompleteTagsRequest struct {
	Query string `json:"query"`
	Limit int32  `json:"limit"`
}

type submitReportRequest struct {
	Reason      string `json:"reason"`
	Description string `json:"description"`
}

type auditReportRequest struct {
	Status    int32  `json:"status"`
	AuditNote string `json:"audit_note"`
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
