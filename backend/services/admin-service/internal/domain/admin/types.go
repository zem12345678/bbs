package admin

import "errors"

type Action string

const (
	ActionListReports                  Action = "list_reports"
	ActionAuditReport                  Action = "audit_report"
	ActionListUsers                    Action = "list_users"
	ActionMuteUser                     Action = "mute_user"
	ActionUnmuteUser                   Action = "unmute_user"
	ActionListUserFileCapacity         Action = "list_user_file_capacity"
	ActionUpdateUserFileCapacity       Action = "update_user_file_capacity"
	ActionListArticles                 Action = "list_articles"
	ActionPublishArticle               Action = "publish_article"
	ActionHideArticle                  Action = "hide_article"
	ActionArchiveArticle               Action = "archive_article"
	ActionListTopics                   Action = "list_topics"
	ActionPublishTopic                 Action = "publish_topic"
	ActionHideTopic                    Action = "hide_topic"
	ActionArchiveTopic                 Action = "archive_topic"
	ActionListCategories               Action = "list_categories"
	ActionCreateCategory               Action = "create_category"
	ActionUpdateCategory               Action = "update_category"
	ActionDeleteCategory               Action = "delete_category"
	ActionListComments                 Action = "list_comments"
	ActionHideComment                  Action = "hide_comment"
	ActionRestoreComment               Action = "restore_comment"
	ActionListAdminUsers               Action = "list_admin_users"
	ActionCreateAdminUser              Action = "create_admin_user"
	ActionListRoles                    Action = "list_roles"
	ActionAssignRoles                  Action = "assign_roles"
	ActionListBadges                   Action = "list_badges"
	ActionCreateBadge                  Action = "create_badge"
	ActionUpdateBadge                  Action = "update_badge"
	ActionDeleteBadge                  Action = "delete_badge"
	ActionListLevels                   Action = "list_levels"
	ActionCreateLevel                  Action = "create_level"
	ActionUpdateLevel                  Action = "update_level"
	ActionDeleteLevel                  Action = "delete_level"
	ActionListForbiddenWords           Action = "list_forbidden_words"
	ActionCreateForbiddenWord          Action = "create_forbidden_word"
	ActionUpdateForbiddenWord          Action = "update_forbidden_word"
	ActionDeleteForbiddenWord          Action = "delete_forbidden_word"
	ActionListEmailLogs                Action = "list_email_logs"
	ActionListLoginLogs                Action = "list_login_logs"
	ActionListOperationLogs            Action = "list_operation_logs"
	ActionListSettings                 Action = "list_settings"
	ActionUpdateSetting                Action = "update_setting"
	ActionListInviteCodes              Action = "list_invite_codes"
	ActionCreateInviteCodes            Action = "create_invite_codes"
	ActionRevokeInviteCode             Action = "revoke_invite_code"
	ActionListLinks                    Action = "list_links"
	ActionCreateLink                   Action = "create_link"
	ActionUpdateLink                   Action = "update_link"
	ActionDeleteLink                   Action = "delete_link"
	ActionListTasks                    Action = "list_tasks"
	ActionCreateTask                   Action = "create_task"
	ActionUpdateTask                   Action = "update_task"
	ActionDeleteTask                   Action = "delete_task"
	ActionListUserCredits              Action = "list_user_credits"
	ActionAdjustUserCredits            Action = "adjust_user_credits"
	ActionListMallProductCategories    Action = "list_product_categories"
	ActionCreateMallProductCategory    Action = "create_product_category"
	ActionUpdateMallProductCategory    Action = "update_product_category"
	ActionListMallProducts             Action = "list_products"
	ActionCreateMallProduct            Action = "create_product"
	ActionUpdateMallProduct            Action = "update_product"
	ActionListMallProductReviews       Action = "list_product_reviews"
	ActionUpdateMallProductReview      Action = "update_product_review"
	ActionListMallCoupons              Action = "list_coupons"
	ActionListMallCouponUsages         Action = "list_coupon_usages"
	ActionCreateMallCoupon             Action = "create_coupon"
	ActionUpdateMallCoupon             Action = "update_coupon"
	ActionListMallOrders               Action = "list_orders"
	ActionListMallDigitalEntitlements  Action = "list_digital_entitlements"
	ActionRevokeMallDigitalEntitlement Action = "revoke_digital_entitlement"
	ActionCloseExpiredMall             Action = "close_expired_orders"
	ActionRecoverPayingMallOrders      Action = "recover_paying_orders"
	ActionRequeueMallOutboxEvents      Action = "requeue_outbox_events"
	ActionUpdateMallOrder              Action = "update_order_status"
	ActionListMallOrderLogs            Action = "list_order_logs"
	ActionListMallPayments             Action = "list_order_payments"
	ActionListMallRefunds              Action = "list_refunds"
	ActionReviewMallRefunds            Action = "review_refunds"
	ResourceGovernance                        = "governance"
	ResourceMall                              = "mall"
	UserStatusActive                   int32  = 1
	UserStatusMuted                    int32  = 2
	AdminStatusActive                  int32  = 1
	ReportStatusPending                int32  = 1
	ReportStatusResolved               int32  = 2
	ReportTargetActionHide                    = "hide"
)

var (
	ErrInvalidActor           = errors.New("invalid admin actor")
	ErrPermissionDenied       = errors.New("admin permission denied")
	ErrInvalidUserID          = errors.New("invalid user id")
	ErrInvalidReportID        = errors.New("invalid report id")
	ErrInvalidReportAction    = errors.New("invalid report action")
	ErrInvalidArticleID       = errors.New("invalid article id")
	ErrInvalidTopicID         = errors.New("invalid topic id")
	ErrInvalidCategoryID      = errors.New("invalid category id")
	ErrInvalidCommentID       = errors.New("invalid comment id")
	ErrInvalidAdminUserID     = errors.New("invalid admin user id")
	ErrInvalidBadgeID         = errors.New("invalid badge id")
	ErrInvalidForbiddenWordID = errors.New("invalid forbidden word id")
	ErrInvalidLevelID         = errors.New("invalid level id")
	ErrInvalidLinkID          = errors.New("invalid link id")
	ErrInvalidSettingID       = errors.New("invalid setting id")
	ErrInvalidTaskID          = errors.New("invalid task id")
	ErrInvalidRoleKeys        = errors.New("invalid role keys")
	ErrInvalidPassword        = errors.New("密码必须为 8-64 位，且同时包含字母、数字和特殊字符，不能包含空白字符")
	ErrInvalidBadge           = errors.New("invalid badge")
	ErrInvalidForbiddenWord   = errors.New("invalid forbidden word")
	ErrInvalidLevel           = errors.New("invalid level")
	ErrInvalidLink            = errors.New("invalid link")
	ErrInvalidSetting         = errors.New("invalid setting")
	ErrInvalidTask            = errors.New("invalid task")
	ErrInvalidStatus          = errors.New("invalid status")
	ErrTaskDefinitionsManaged = errors.New("task definitions are managed by the system")
	ErrInvalidCategory        = errors.New("invalid category")
	ErrInvalidCredentials     = errors.New("invalid admin credentials")
	ErrTooManyLoginAttempts   = errors.New("登录失败次数过多，请 15 分钟后再试")
	ErrInvalidAdminProfile    = errors.New("invalid admin profile")
	ErrAdminDisabled          = errors.New("admin account disabled")
	ErrInvalidToken           = errors.New("invalid admin token")
	ErrAdminUserExists        = errors.New("用户名、邮箱或手机号已存在")
)

type Actor struct {
	ID       int64
	Username string
}

func (a Actor) Validate() error {
	if a.ID <= 0 || a.Username == "" {
		return ErrInvalidActor
	}
	return nil
}

type EntityRef struct {
	EntityType string
	EntityID   int64
}

type Report struct {
	ID           int64
	Entity       EntityRef
	ReporterID   int64
	Reason       string
	Description  string
	Status       int32
	HandledBy    int64
	HandledAt    int64
	AuditNote    string
	TargetAction string
	CreatedAt    int64
	UpdatedAt    int64
}

type ReportList struct {
	Items []Report
	Total int64
}

type User struct {
	ID             int64
	Username       string
	Email          string
	Nickname       string
	AvatarURL      string
	BackgroundURL  string
	Bio            string
	Status         int32
	FollowerCount  int64
	FollowingCount int64
	CreatedAt      int64
	UpdatedAt      int64
	LastLoginAt    int64
}

type UserList struct {
	Items []User
	Total int64
}

type Article struct {
	ID          int64
	Slug        string
	Title       string
	Summary     string
	Body        string
	CoverURL    string
	Tags        []string
	AuthorID    int64
	Status      int32
	CreatedAt   int64
	UpdatedAt   int64
	PublishedAt int64
}

type ArticleList struct {
	Items []Article
	Total int64
}

type Topic struct {
	ID          int64
	Slug        string
	Type        string
	Title       string
	Body        string
	Tags        []string
	AuthorID    int64
	CategoryID  int64
	Status      int32
	CreatedAt   int64
	UpdatedAt   int64
	PublishedAt int64
}

type TopicList struct {
	Items []Topic
	Total int64
}

type Category struct {
	ID          int64
	Slug        string
	Name        string
	Description string
	Sort        int32
	Status      int32
	TopicCount  int64
	CreatedAt   int64
	UpdatedAt   int64
}

type CategoryList struct {
	Items []Category
	Total int64
}

type UpsertCategoryCommand struct {
	ID          int64
	Slug        string
	Name        string
	Description string
	Sort        int32
	Status      int32
}

type Comment struct {
	ID         int64
	EntityType string
	EntityID   int64
	RootID     int64
	ParentID   int64
	AuthorID   int64
	Content    string
	Status     int32
	ReplyCount int64
	LikeCount  int64
	CreatedAt  int64
	UpdatedAt  int64
}

type CommentList struct {
	Items []Comment
	Total int64
}

type AdminUser struct {
	ID           int64
	Username     string
	Email        string
	Phone        string
	Nickname     string
	AvatarURL    string
	Bio          string
	Status       int32
	LockedFlag   int32
	PasswordHash string
	Roles        []string
}

func (u AdminUser) CanLogin() bool {
	return u.ID > 0 && u.Status == AdminStatusActive && u.LockedFlag != 1
}

type AdminProfile struct {
	User        AdminUser
	Roles       []string
	Permissions []string
}

type AdminToken struct {
	AccessToken      string
	ExpiresAt        int64
	RefreshToken     string
	RefreshExpiresAt int64
}

type AdminSession struct {
	Profile AdminProfile
	Token   AdminToken
}

type UpdateAdminProfileCommand struct {
	UserID    int64
	Nickname  string
	Email     string
	Phone     string
	AvatarURL string
	Bio       string
}

type TokenClaims struct {
	UserID    int64
	Username  string
	SessionID string
}

type Role struct {
	ID          int64
	Name        string
	Key         string
	Status      string
	Sort        int32
	Admin       bool
	Remark      string
	Permissions []string
}

type RoleList struct {
	Items []Role
	Total int64
}

type AdminUserList struct {
	Items []AdminUser
	Total int64
}

type CreateAdminUserCommand struct {
	Username string
	Email    string
	Phone    string
	Password string
	Nickname string
	RoleKeys []string
}

type Badge struct {
	ID          int64
	Key         string
	Name        string
	Description string
	IconURL     string
	RuleType    string
	RuleValue   int64
	Status      int32
	Sort        int32
	CreatedAt   int64
	UpdatedAt   int64
}

type BadgeList struct {
	Items []Badge
	Total int64
}

type UpsertBadgeCommand struct {
	ID          int64
	Key         string
	Name        string
	Description string
	IconURL     string
	RuleType    string
	RuleValue   int64
	Status      int32
	Sort        int32
}

type Level struct {
	ID          int64
	Key         string
	Name        string
	Description string
	MinScore    int64
	MaxScore    int64
	Status      int32
	Sort        int32
	CreatedAt   int64
	UpdatedAt   int64
}

type LevelList struct {
	Items []Level
	Total int64
}

type UpsertLevelCommand struct {
	ID          int64
	Key         string
	Name        string
	Description string
	MinScore    int64
	MaxScore    int64
	Status      int32
	Sort        int32
}

type ForbiddenWord struct {
	ID          int64
	Word        string
	Scene       string
	Action      string
	Replacement string
	Description string
	Status      int32
	CreatedAt   int64
	UpdatedAt   int64
}

type ForbiddenWordList struct {
	Items []ForbiddenWord
	Total int64
}

type UpsertForbiddenWordCommand struct {
	ID          int64
	Word        string
	Scene       string
	Action      string
	Replacement string
	Description string
	Status      int32
}

type Setting struct {
	ID          int64
	Key         string
	Value       string
	Group       string
	ValueType   string
	Description string
	Status      int32
	CreatedAt   int64
	UpdatedAt   int64
}

type SettingList struct {
	Items []Setting
	Total int64
}

type UpsertSettingCommand struct {
	ID            int64
	Key           string
	Value         string
	Group         string
	ValueType     string
	Description   string
	Status        int32
	PreserveValue bool
	ClearValue    bool
}

type EmailLog struct {
	ID          int64
	To          string
	Subject     string
	TemplateKey string
	Provider    string
	Status      int32
	Error       string
	CreatedAt   int64
	UpdatedAt   int64
}

type EmailLogList struct {
	Items []EmailLog
	Total int64
}

type LoginLog struct {
	ID        int64
	Username  string
	Status    int32
	IP        string
	Location  string
	Browser   string
	OS        string
	Platform  string
	Message   string
	Remark    string
	LoginTime int64
}

type LoginLogList struct {
	Items []LoginLog
	Total int64
}

type RecordLoginLogCommand struct {
	Username  string
	Status    int32
	IP        string
	UserAgent string
	Message   string
	Remark    string
}

type OperationLog struct {
	ID            int64
	Title         string
	BusinessType  string
	Method        string
	RequestMethod string
	OperatorType  string
	OperatorName  string
	DeptName      string
	URL           string
	IP            string
	Location      string
	Params        string
	Status        int32
	OperationTime int64
	Result        string
	Remark        string
	LatencyTime   string
	UserAgent     string
}

type OperationLogList struct {
	Items []OperationLog
	Total int64
}

type RecordOperationLogCommand struct {
	Title         string
	BusinessType  string
	Method        string
	RequestMethod string
	OperatorType  string
	OperatorName  string
	URL           string
	IP            string
	Params        string
	Status        int32
	Result        string
	Remark        string
	LatencyTime   string
	UserAgent     string
}

type Link struct {
	ID          int64
	Key         string
	Title       string
	URL         string
	Description string
	Status      int32
	Sort        int32
	CreatedAt   int64
	UpdatedAt   int64
}

type LinkList struct {
	Items []Link
	Total int64
}

type UpsertLinkCommand struct {
	ID          int64
	Key         string
	Title       string
	URL         string
	Description string
	Status      int32
	Sort        int32
}

type Task struct {
	ID           int64
	Key          string
	Title        string
	Description  string
	RewardPoints int64
	Status       int32
	Sort         int32
	CreatedAt    int64
	UpdatedAt    int64
}

type TaskList struct {
	Items []Task
	Total int64
}

type UpsertTaskCommand struct {
	ID           int64
	Key          string
	Title        string
	Description  string
	RewardPoints int64
	Status       int32
	Sort         int32
}
