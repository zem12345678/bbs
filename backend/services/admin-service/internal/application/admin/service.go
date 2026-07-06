package admin

import (
	"context"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
)

type Authorizer interface {
	Authorize(ctx context.Context, actor domain.Actor, action domain.Action) error
	Reload(ctx context.Context) error
}

type ReportGateway interface {
	ListReports(ctx context.Context, status int32, entityType string, limit int32, offset int32) (domain.ReportList, error)
	AuditReport(ctx context.Context, id int64, status int32, handlerID int64) (domain.Report, error)
}

type UserGateway interface {
	ListUsers(ctx context.Context, query string, status int32, page int32, pageSize int32) (domain.UserList, error)
	UpdateStatus(ctx context.Context, userID int64, status int32) (domain.User, error)
}

type ContentGateway interface {
	ListArticles(ctx context.Context, status int32, tag string, authorID int64, limit int32, offset int32) (domain.ArticleList, error)
	PublishArticle(ctx context.Context, id int64) (domain.Article, error)
	HideArticle(ctx context.Context, id int64) (domain.Article, error)
	ArchiveArticle(ctx context.Context, id int64) (domain.Article, error)
	ListTopics(ctx context.Context, status int32, typ string, tag string, authorID int64, categoryID int64, limit int32, offset int32) (domain.TopicList, error)
	PublishTopic(ctx context.Context, id int64) (domain.Topic, error)
	HideTopic(ctx context.Context, id int64) (domain.Topic, error)
	ArchiveTopic(ctx context.Context, id int64) (domain.Topic, error)
	ListCategories(ctx context.Context, status int32, limit int32, offset int32) (domain.CategoryList, error)
	CreateCategory(ctx context.Context, command domain.UpsertCategoryCommand) (domain.Category, error)
	UpdateCategory(ctx context.Context, command domain.UpsertCategoryCommand) (domain.Category, error)
	DeleteCategory(ctx context.Context, id int64) error
}

type CommentGateway interface {
	ListComments(ctx context.Context, entityType string, entityID int64, authorID int64, status int32, page int32, pageSize int32) (domain.CommentList, error)
	HideComment(ctx context.Context, id int64, actorID int64) error
	RestoreComment(ctx context.Context, id int64, actorID int64) error
}

type AuthStore interface {
	FindAdminUserByAccount(ctx context.Context, account string) (domain.AdminUser, error)
	FindAdminUserByID(ctx context.Context, id int64) (domain.AdminUser, error)
	UpdateAdminProfile(ctx context.Context, command domain.UpdateAdminProfileCommand) (domain.AdminUser, error)
	RoleKeysByUserID(ctx context.Context, userID int64) ([]string, error)
	PermissionsByRoleKeys(ctx context.Context, roles []string) ([]string, error)
	UpdateAdminLastLogin(ctx context.Context, userID int64, loginIP string) error
}

type RBACStore interface {
	ListAdminUsers(ctx context.Context, query string, limit int32, offset int32) (domain.AdminUserList, error)
	CreateAdminUser(ctx context.Context, command domain.CreateAdminUserCommand, passwordHash string) (domain.AdminUser, error)
	ListRoles(ctx context.Context) (domain.RoleList, error)
	AssignRoles(ctx context.Context, userID int64, roleKeys []string) (domain.AdminUser, error)
}

type SystemStore interface {
	ListSystemUsers(ctx context.Context, query string, status int32, page int32, pageSize int32) (domain.SystemUserList, error)
	CreateSystemUser(ctx context.Context, command domain.UpsertSystemUserCommand, passwordHash string) (domain.SystemUser, error)
	UpdateSystemUser(ctx context.Context, command domain.UpsertSystemUserCommand) (domain.SystemUser, error)
	DeleteSystemUser(ctx context.Context, id int64) error
	ResetSystemUserPassword(ctx context.Context, id int64, passwordHash string) (domain.SystemUser, error)
	AssignSystemUserRoles(ctx context.Context, userID int64, roleIDs []int64) (domain.SystemUser, error)
	ListSystemRoles(ctx context.Context, query string, status string, page int32, pageSize int32) (domain.SystemRoleList, error)
	CreateSystemRole(ctx context.Context, command domain.UpsertSystemRoleCommand) (domain.SystemRole, error)
	UpdateSystemRole(ctx context.Context, command domain.UpsertSystemRoleCommand) (domain.SystemRole, error)
	DeleteSystemRole(ctx context.Context, id int64) error
	AssignSystemRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) (domain.SystemRole, error)
	ListSystemMenus(ctx context.Context, query string, status string) (domain.SystemMenuList, error)
	ListCurrentSystemMenus(ctx context.Context, userID int64) (domain.SystemMenuList, error)
	CreateSystemMenu(ctx context.Context, command domain.UpsertSystemMenuCommand) (domain.SystemMenu, error)
	UpdateSystemMenu(ctx context.Context, command domain.UpsertSystemMenuCommand) (domain.SystemMenu, error)
	DeleteSystemMenu(ctx context.Context, id int64) error
	ListSystemDepts(ctx context.Context, query string, status int32) (domain.SystemDeptList, error)
	CreateSystemDept(ctx context.Context, command domain.UpsertSystemDeptCommand) (domain.SystemDept, error)
	UpdateSystemDept(ctx context.Context, command domain.UpsertSystemDeptCommand) (domain.SystemDept, error)
	DeleteSystemDept(ctx context.Context, id int64) error
}

type OperationStore interface {
	ListBadges(ctx context.Context, status int32, limit int32, offset int32) (domain.BadgeList, error)
	UpsertBadge(ctx context.Context, command domain.UpsertBadgeCommand) (domain.Badge, error)
	DeleteBadge(ctx context.Context, id int64) error
	ListLevels(ctx context.Context, status int32, limit int32, offset int32) (domain.LevelList, error)
	UpsertLevel(ctx context.Context, command domain.UpsertLevelCommand) (domain.Level, error)
	DeleteLevel(ctx context.Context, id int64) error
	ListForbiddenWords(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.ForbiddenWordList, error)
	UpsertForbiddenWord(ctx context.Context, command domain.UpsertForbiddenWordCommand) (domain.ForbiddenWord, error)
	DeleteForbiddenWord(ctx context.Context, id int64) error
	ListSettings(ctx context.Context, group string, status int32, limit int32, offset int32) (domain.SettingList, error)
	UpsertSetting(ctx context.Context, command domain.UpsertSettingCommand) (domain.Setting, error)
	ListEmailLogs(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.EmailLogList, error)
	ListLoginLogs(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.LoginLogList, error)
	RecordLoginLog(ctx context.Context, command domain.RecordLoginLogCommand) error
	CountRecentLoginFailures(ctx context.Context, username string, ip string, since time.Time) (int64, error)
	ListOperationLogs(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.OperationLogList, error)
	RecordOperationLog(ctx context.Context, command domain.RecordOperationLogCommand) error
	ListLinks(ctx context.Context, status int32, limit int32, offset int32) (domain.LinkList, error)
	UpsertLink(ctx context.Context, command domain.UpsertLinkCommand) (domain.Link, error)
	DeleteLink(ctx context.Context, id int64) error
	ListTasks(ctx context.Context, status int32, limit int32, offset int32) (domain.TaskList, error)
	UpsertTask(ctx context.Context, command domain.UpsertTaskCommand) (domain.Task, error)
	DeleteTask(ctx context.Context, id int64) error
}

type PasswordVerifier interface {
	Verify(hash string, password string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type TokenManager interface {
	Issue(user domain.AdminUser, roles []string) (domain.AdminToken, error)
	Parse(accessToken string) (domain.TokenClaims, error)
}

type Service struct {
	auth      Authorizer
	authStore AuthStore
	rbacStore RBACStore
	system    SystemStore
	passwords PasswordVerifier
	hasher    PasswordHasher
	tokens    TokenManager
	reports   ReportGateway
	users     UserGateway
	content   ContentGateway
	comments  CommentGateway
	ops       OperationStore
}

func NewService(auth Authorizer, authStore AuthStore, rbacStore RBACStore, system SystemStore, ops OperationStore, passwords PasswordVerifier, hasher PasswordHasher, tokens TokenManager, reports ReportGateway, users UserGateway, content ContentGateway, comments CommentGateway) *Service {
	return &Service{auth: auth, authStore: authStore, rbacStore: rbacStore, system: system, ops: ops, passwords: passwords, hasher: hasher, tokens: tokens, reports: reports, users: users, content: content, comments: comments}
}

func (s *Service) Login(ctx context.Context, account string, password string, loginIP string, userAgent string) (domain.AdminSession, error) {
	account = strings.ToLower(strings.TrimSpace(account))
	if account == "" || password == "" {
		s.recordLoginAttempt(ctx, account, loginIP, userAgent, 0, "账号或密码为空")
		return domain.AdminSession{}, domain.ErrInvalidCredentials
	}
	if limited, err := s.tooManyLoginFailures(ctx, account, loginIP); err != nil {
		return domain.AdminSession{}, err
	} else if limited {
		s.recordLoginAttempt(ctx, account, loginIP, userAgent, 0, "登录失败次数过多，请稍后再试")
		return domain.AdminSession{}, domain.ErrTooManyLoginAttempts
	}
	user, err := s.authStore.FindAdminUserByAccount(ctx, account)
	if err != nil {
		s.recordLoginAttempt(ctx, account, loginIP, userAgent, 0, "账号不存在或密码错误")
		return domain.AdminSession{}, s.loginFailureError(ctx, account, loginIP)
	}
	if !user.CanLogin() {
		s.recordLoginAttempt(ctx, account, loginIP, userAgent, 0, "账号已禁用或锁定")
		return domain.AdminSession{}, domain.ErrAdminDisabled
	}
	if err := s.passwords.Verify(user.PasswordHash, password); err != nil {
		s.recordLoginAttempt(ctx, account, loginIP, userAgent, 0, "账号不存在或密码错误")
		return domain.AdminSession{}, s.loginFailureError(ctx, account, loginIP)
	}
	profile, err := s.profileForUser(ctx, user)
	if err != nil {
		return domain.AdminSession{}, err
	}
	token, err := s.tokens.Issue(user, profile.Roles)
	if err != nil {
		return domain.AdminSession{}, err
	}
	if err := s.authStore.UpdateAdminLastLogin(ctx, user.ID, loginIP); err != nil {
		return domain.AdminSession{}, err
	}
	s.recordLoginAttempt(ctx, account, loginIP, userAgent, 1, "登录成功")
	return domain.AdminSession{Profile: profile, Token: token}, nil
}

func (s *Service) GetProfile(ctx context.Context, accessToken string) (domain.AdminProfile, error) {
	claims, err := s.tokens.Parse(strings.TrimSpace(accessToken))
	if err != nil {
		return domain.AdminProfile{}, err
	}
	user, err := s.authStore.FindAdminUserByID(ctx, claims.UserID)
	if err != nil {
		return domain.AdminProfile{}, err
	}
	if !user.CanLogin() {
		return domain.AdminProfile{}, domain.ErrAdminDisabled
	}
	return s.profileForUser(ctx, user)
}

func (s *Service) UpdateProfile(ctx context.Context, actor domain.Actor, command domain.UpdateAdminProfileCommand) (domain.AdminProfile, error) {
	if err := actor.Validate(); err != nil {
		return domain.AdminProfile{}, err
	}
	nickname := strings.TrimSpace(command.Nickname)
	if nickname == "" {
		nickname = actor.Username
	}
	command.UserID = actor.ID
	command.Nickname = nickname
	user, err := s.authStore.UpdateAdminProfile(ctx, command)
	if err != nil {
		return domain.AdminProfile{}, err
	}
	if !user.CanLogin() {
		return domain.AdminProfile{}, domain.ErrAdminDisabled
	}
	return s.profileForUser(ctx, user)
}

func (s *Service) ListReports(ctx context.Context, actor domain.Actor, status int32, entityType string, limit int32, offset int32) (domain.ReportList, error) {
	if err := actor.Validate(); err != nil {
		return domain.ReportList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListReports); err != nil {
		return domain.ReportList{}, err
	}
	return s.reports.ListReports(ctx, status, entityType, limit, offset)
}

func (s *Service) AuditReport(ctx context.Context, actor domain.Actor, id int64, status int32) (domain.Report, error) {
	if err := actor.Validate(); err != nil {
		return domain.Report{}, err
	}
	if id <= 0 {
		return domain.Report{}, domain.ErrInvalidReportID
	}
	if status <= 0 {
		return domain.Report{}, domain.ErrInvalidStatus
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionAuditReport); err != nil {
		return domain.Report{}, err
	}
	return s.reports.AuditReport(ctx, id, status, actor.ID)
}

func (s *Service) ListUsers(ctx context.Context, actor domain.Actor, query string, status int32, page int32, pageSize int32) (domain.UserList, error) {
	if err := actor.Validate(); err != nil {
		return domain.UserList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListUsers); err != nil {
		return domain.UserList{}, err
	}
	return s.users.ListUsers(ctx, query, status, page, pageSize)
}

func (s *Service) MuteUser(ctx context.Context, actor domain.Actor, userID int64) (domain.User, error) {
	return s.updateUserStatus(ctx, actor, userID, domain.UserStatusMuted, domain.ActionMuteUser)
}

func (s *Service) UnmuteUser(ctx context.Context, actor domain.Actor, userID int64) (domain.User, error) {
	return s.updateUserStatus(ctx, actor, userID, domain.UserStatusActive, domain.ActionUnmuteUser)
}

func (s *Service) ListArticles(ctx context.Context, actor domain.Actor, status int32, tag string, authorID int64, limit int32, offset int32) (domain.ArticleList, error) {
	if err := actor.Validate(); err != nil {
		return domain.ArticleList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListArticles); err != nil {
		return domain.ArticleList{}, err
	}
	return s.content.ListArticles(ctx, status, tag, authorID, limit, offset)
}

func (s *Service) HideArticle(ctx context.Context, actor domain.Actor, id int64) (domain.Article, error) {
	return s.updateArticleStatus(ctx, actor, id, domain.ActionHideArticle, s.content.HideArticle)
}

func (s *Service) PublishArticle(ctx context.Context, actor domain.Actor, id int64) (domain.Article, error) {
	return s.updateArticleStatus(ctx, actor, id, domain.ActionPublishArticle, s.content.PublishArticle)
}

func (s *Service) ArchiveArticle(ctx context.Context, actor domain.Actor, id int64) (domain.Article, error) {
	return s.updateArticleStatus(ctx, actor, id, domain.ActionArchiveArticle, s.content.ArchiveArticle)
}

func (s *Service) ListTopics(ctx context.Context, actor domain.Actor, status int32, typ string, tag string, authorID int64, categoryID int64, limit int32, offset int32) (domain.TopicList, error) {
	if err := actor.Validate(); err != nil {
		return domain.TopicList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListTopics); err != nil {
		return domain.TopicList{}, err
	}
	return s.content.ListTopics(ctx, status, typ, tag, authorID, categoryID, limit, offset)
}

func (s *Service) HideTopic(ctx context.Context, actor domain.Actor, id int64) (domain.Topic, error) {
	return s.updateTopicStatus(ctx, actor, id, domain.ActionHideTopic, s.content.HideTopic)
}

func (s *Service) PublishTopic(ctx context.Context, actor domain.Actor, id int64) (domain.Topic, error) {
	return s.updateTopicStatus(ctx, actor, id, domain.ActionPublishTopic, s.content.PublishTopic)
}

func (s *Service) ArchiveTopic(ctx context.Context, actor domain.Actor, id int64) (domain.Topic, error) {
	return s.updateTopicStatus(ctx, actor, id, domain.ActionArchiveTopic, s.content.ArchiveTopic)
}

func (s *Service) ListCategories(ctx context.Context, actor domain.Actor, status int32, limit int32, offset int32) (domain.CategoryList, error) {
	if err := actor.Validate(); err != nil {
		return domain.CategoryList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListCategories); err != nil {
		return domain.CategoryList{}, err
	}
	return s.content.ListCategories(ctx, status, limit, offset)
}

func (s *Service) CreateCategory(ctx context.Context, actor domain.Actor, command domain.UpsertCategoryCommand) (domain.Category, error) {
	if err := actor.Validate(); err != nil {
		return domain.Category{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateCategory); err != nil {
		return domain.Category{}, err
	}
	command.ID = 0
	if strings.TrimSpace(command.Slug) == "" || strings.TrimSpace(command.Name) == "" {
		return domain.Category{}, domain.ErrInvalidCategory
	}
	return s.content.CreateCategory(ctx, command)
}

func (s *Service) UpdateCategory(ctx context.Context, actor domain.Actor, command domain.UpsertCategoryCommand) (domain.Category, error) {
	if err := actor.Validate(); err != nil {
		return domain.Category{}, err
	}
	if command.ID <= 0 {
		return domain.Category{}, domain.ErrInvalidCategoryID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateCategory); err != nil {
		return domain.Category{}, err
	}
	if strings.TrimSpace(command.Slug) == "" || strings.TrimSpace(command.Name) == "" {
		return domain.Category{}, domain.ErrInvalidCategory
	}
	return s.content.UpdateCategory(ctx, command)
}

func (s *Service) DeleteCategory(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidCategoryID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteCategory); err != nil {
		return err
	}
	return s.content.DeleteCategory(ctx, id)
}

func (s *Service) ListComments(ctx context.Context, actor domain.Actor, entityType string, entityID int64, authorID int64, status int32, page int32, pageSize int32) (domain.CommentList, error) {
	if err := actor.Validate(); err != nil {
		return domain.CommentList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListComments); err != nil {
		return domain.CommentList{}, err
	}
	return s.comments.ListComments(ctx, entityType, entityID, authorID, status, page, pageSize)
}

func (s *Service) HideComment(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidCommentID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionHideComment); err != nil {
		return err
	}
	return s.comments.HideComment(ctx, id, actor.ID)
}

func (s *Service) RestoreComment(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidCommentID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionRestoreComment); err != nil {
		return err
	}
	return s.comments.RestoreComment(ctx, id, actor.ID)
}

func (s *Service) ListAdminUsers(ctx context.Context, actor domain.Actor, query string, limit int32, offset int32) (domain.AdminUserList, error) {
	if err := actor.Validate(); err != nil {
		return domain.AdminUserList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListAdminUsers); err != nil {
		return domain.AdminUserList{}, err
	}
	return s.rbacStore.ListAdminUsers(ctx, query, limit, offset)
}

func (s *Service) CreateAdminUser(ctx context.Context, actor domain.Actor, command domain.CreateAdminUserCommand) (domain.AdminUser, error) {
	if err := actor.Validate(); err != nil {
		return domain.AdminUser{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateAdminUser); err != nil {
		return domain.AdminUser{}, err
	}
	if strings.TrimSpace(command.Username) == "" {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	if err := validatePasswordPolicy(command.Password); err != nil {
		return domain.AdminUser{}, err
	}
	passwordHash, err := s.hasher.Hash(command.Password)
	if err != nil {
		return domain.AdminUser{}, err
	}
	return s.rbacStore.CreateAdminUser(ctx, command, passwordHash)
}

func (s *Service) ListRoles(ctx context.Context, actor domain.Actor) (domain.RoleList, error) {
	if err := actor.Validate(); err != nil {
		return domain.RoleList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListRoles); err != nil {
		return domain.RoleList{}, err
	}
	return s.rbacStore.ListRoles(ctx)
}

func (s *Service) AssignRoles(ctx context.Context, actor domain.Actor, userID int64, roleKeys []string) (domain.AdminUser, error) {
	if err := actor.Validate(); err != nil {
		return domain.AdminUser{}, err
	}
	if userID <= 0 {
		return domain.AdminUser{}, domain.ErrInvalidAdminUserID
	}
	if len(roleKeys) == 0 {
		return domain.AdminUser{}, domain.ErrInvalidRoleKeys
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionAssignRoles); err != nil {
		return domain.AdminUser{}, err
	}
	return s.rbacStore.AssignRoles(ctx, userID, roleKeys)
}

func (s *Service) ListBadges(ctx context.Context, actor domain.Actor, status int32, limit int32, offset int32) (domain.BadgeList, error) {
	if actor.ID > 0 || actor.Username != "" {
		if err := actor.Validate(); err != nil {
			return domain.BadgeList{}, err
		}
		if err := s.auth.Authorize(ctx, actor, domain.ActionListBadges); err != nil {
			return domain.BadgeList{}, err
		}
	} else {
		status = 2
	}
	return s.ops.ListBadges(ctx, status, limit, offset)
}

func (s *Service) CreateBadge(ctx context.Context, actor domain.Actor, command domain.UpsertBadgeCommand) (domain.Badge, error) {
	if err := actor.Validate(); err != nil {
		return domain.Badge{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateBadge); err != nil {
		return domain.Badge{}, err
	}
	command.ID = 0
	if strings.TrimSpace(command.Name) == "" {
		return domain.Badge{}, domain.ErrInvalidBadge
	}
	return s.ops.UpsertBadge(ctx, command)
}

func (s *Service) UpdateBadge(ctx context.Context, actor domain.Actor, command domain.UpsertBadgeCommand) (domain.Badge, error) {
	if err := actor.Validate(); err != nil {
		return domain.Badge{}, err
	}
	if command.ID <= 0 {
		return domain.Badge{}, domain.ErrInvalidBadgeID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateBadge); err != nil {
		return domain.Badge{}, err
	}
	if strings.TrimSpace(command.Name) == "" {
		return domain.Badge{}, domain.ErrInvalidBadge
	}
	return s.ops.UpsertBadge(ctx, command)
}

func (s *Service) DeleteBadge(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidBadgeID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteBadge); err != nil {
		return err
	}
	return s.ops.DeleteBadge(ctx, id)
}

func (s *Service) ListLevels(ctx context.Context, actor domain.Actor, status int32, limit int32, offset int32) (domain.LevelList, error) {
	if actor.ID > 0 || actor.Username != "" {
		if err := actor.Validate(); err != nil {
			return domain.LevelList{}, err
		}
		if err := s.auth.Authorize(ctx, actor, domain.ActionListLevels); err != nil {
			return domain.LevelList{}, err
		}
	} else {
		status = 2
	}
	return s.ops.ListLevels(ctx, status, limit, offset)
}

func (s *Service) CreateLevel(ctx context.Context, actor domain.Actor, command domain.UpsertLevelCommand) (domain.Level, error) {
	if err := actor.Validate(); err != nil {
		return domain.Level{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateLevel); err != nil {
		return domain.Level{}, err
	}
	command.ID = 0
	if strings.TrimSpace(command.Name) == "" {
		return domain.Level{}, domain.ErrInvalidLevel
	}
	return s.ops.UpsertLevel(ctx, command)
}

func (s *Service) UpdateLevel(ctx context.Context, actor domain.Actor, command domain.UpsertLevelCommand) (domain.Level, error) {
	if err := actor.Validate(); err != nil {
		return domain.Level{}, err
	}
	if command.ID <= 0 {
		return domain.Level{}, domain.ErrInvalidLevelID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateLevel); err != nil {
		return domain.Level{}, err
	}
	if strings.TrimSpace(command.Name) == "" {
		return domain.Level{}, domain.ErrInvalidLevel
	}
	return s.ops.UpsertLevel(ctx, command)
}

func (s *Service) DeleteLevel(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidLevelID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteLevel); err != nil {
		return err
	}
	return s.ops.DeleteLevel(ctx, id)
}

func (s *Service) ListForbiddenWords(ctx context.Context, actor domain.Actor, status int32, query string, limit int32, offset int32) (domain.ForbiddenWordList, error) {
	if err := actor.Validate(); err != nil {
		return domain.ForbiddenWordList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListForbiddenWords); err != nil {
		return domain.ForbiddenWordList{}, err
	}
	return s.ops.ListForbiddenWords(ctx, status, query, limit, offset)
}

func (s *Service) CreateForbiddenWord(ctx context.Context, actor domain.Actor, command domain.UpsertForbiddenWordCommand) (domain.ForbiddenWord, error) {
	if err := actor.Validate(); err != nil {
		return domain.ForbiddenWord{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateForbiddenWord); err != nil {
		return domain.ForbiddenWord{}, err
	}
	command.ID = 0
	if strings.TrimSpace(command.Word) == "" {
		return domain.ForbiddenWord{}, domain.ErrInvalidForbiddenWord
	}
	return s.ops.UpsertForbiddenWord(ctx, command)
}

func (s *Service) UpdateForbiddenWord(ctx context.Context, actor domain.Actor, command domain.UpsertForbiddenWordCommand) (domain.ForbiddenWord, error) {
	if err := actor.Validate(); err != nil {
		return domain.ForbiddenWord{}, err
	}
	if command.ID <= 0 {
		return domain.ForbiddenWord{}, domain.ErrInvalidForbiddenWordID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateForbiddenWord); err != nil {
		return domain.ForbiddenWord{}, err
	}
	if strings.TrimSpace(command.Word) == "" {
		return domain.ForbiddenWord{}, domain.ErrInvalidForbiddenWord
	}
	return s.ops.UpsertForbiddenWord(ctx, command)
}

func (s *Service) DeleteForbiddenWord(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidForbiddenWordID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteForbiddenWord); err != nil {
		return err
	}
	return s.ops.DeleteForbiddenWord(ctx, id)
}

func (s *Service) ListSettings(ctx context.Context, actor domain.Actor, group string, status int32, limit int32, offset int32) (domain.SettingList, error) {
	if err := actor.Validate(); err != nil {
		return domain.SettingList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListSettings); err != nil {
		return domain.SettingList{}, err
	}
	return s.ops.ListSettings(ctx, group, status, limit, offset)
}

func (s *Service) UpdateSetting(ctx context.Context, actor domain.Actor, command domain.UpsertSettingCommand) (domain.Setting, error) {
	if err := actor.Validate(); err != nil {
		return domain.Setting{}, err
	}
	if command.ID <= 0 && strings.TrimSpace(command.Key) == "" {
		return domain.Setting{}, domain.ErrInvalidSettingID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateSetting); err != nil {
		return domain.Setting{}, err
	}
	if strings.TrimSpace(command.Key) == "" {
		return domain.Setting{}, domain.ErrInvalidSetting
	}
	return s.ops.UpsertSetting(ctx, command)
}

func (s *Service) ListEmailLogs(ctx context.Context, actor domain.Actor, status int32, query string, limit int32, offset int32) (domain.EmailLogList, error) {
	if err := actor.Validate(); err != nil {
		return domain.EmailLogList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListEmailLogs); err != nil {
		return domain.EmailLogList{}, err
	}
	return s.ops.ListEmailLogs(ctx, status, query, limit, offset)
}

func (s *Service) recordLoginAttempt(ctx context.Context, username string, ip string, userAgent string, status int32, message string) {
	if s == nil || s.ops == nil {
		return
	}
	_ = s.ops.RecordLoginLog(ctx, domain.RecordLoginLogCommand{
		Username:  username,
		Status:    status,
		IP:        ip,
		UserAgent: userAgent,
		Message:   message,
	})
}

func (s *Service) loginFailureError(ctx context.Context, username string, ip string) error {
	limited, err := s.tooManyLoginFailures(ctx, username, ip)
	if err != nil {
		return err
	}
	if limited {
		return domain.ErrTooManyLoginAttempts
	}
	return domain.ErrInvalidCredentials
}

func (s *Service) tooManyLoginFailures(ctx context.Context, username string, ip string) (bool, error) {
	if s == nil || s.ops == nil {
		return false, nil
	}
	return tooManyLoginFailures(ctx, s.ops, username, ip, time.Now())
}

func (s *Service) ListLinks(ctx context.Context, actor domain.Actor, status int32, limit int32, offset int32) (domain.LinkList, error) {
	if actor.ID > 0 || actor.Username != "" {
		if err := actor.Validate(); err != nil {
			return domain.LinkList{}, err
		}
		if err := s.auth.Authorize(ctx, actor, domain.ActionListLinks); err != nil {
			return domain.LinkList{}, err
		}
	} else {
		status = 2
	}
	return s.ops.ListLinks(ctx, status, limit, offset)
}

func (s *Service) CreateLink(ctx context.Context, actor domain.Actor, command domain.UpsertLinkCommand) (domain.Link, error) {
	if err := actor.Validate(); err != nil {
		return domain.Link{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateLink); err != nil {
		return domain.Link{}, err
	}
	command.ID = 0
	if strings.TrimSpace(command.Title) == "" || strings.TrimSpace(command.URL) == "" {
		return domain.Link{}, domain.ErrInvalidLink
	}
	return s.ops.UpsertLink(ctx, command)
}

func (s *Service) UpdateLink(ctx context.Context, actor domain.Actor, command domain.UpsertLinkCommand) (domain.Link, error) {
	if err := actor.Validate(); err != nil {
		return domain.Link{}, err
	}
	if command.ID <= 0 {
		return domain.Link{}, domain.ErrInvalidLinkID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateLink); err != nil {
		return domain.Link{}, err
	}
	if strings.TrimSpace(command.Title) == "" || strings.TrimSpace(command.URL) == "" {
		return domain.Link{}, domain.ErrInvalidLink
	}
	return s.ops.UpsertLink(ctx, command)
}

func (s *Service) DeleteLink(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidLinkID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteLink); err != nil {
		return err
	}
	return s.ops.DeleteLink(ctx, id)
}

func (s *Service) ListTasks(ctx context.Context, actor domain.Actor, status int32, limit int32, offset int32) (domain.TaskList, error) {
	if actor.ID > 0 || actor.Username != "" {
		if err := actor.Validate(); err != nil {
			return domain.TaskList{}, err
		}
		if err := s.auth.Authorize(ctx, actor, domain.ActionListTasks); err != nil {
			return domain.TaskList{}, err
		}
	} else {
		status = 2
	}
	return s.ops.ListTasks(ctx, status, limit, offset)
}

func (s *Service) CreateTask(ctx context.Context, actor domain.Actor, command domain.UpsertTaskCommand) (domain.Task, error) {
	if err := actor.Validate(); err != nil {
		return domain.Task{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateTask); err != nil {
		return domain.Task{}, err
	}
	command.ID = 0
	if strings.TrimSpace(command.Title) == "" {
		return domain.Task{}, domain.ErrInvalidTask
	}
	return s.ops.UpsertTask(ctx, command)
}

func (s *Service) UpdateTask(ctx context.Context, actor domain.Actor, command domain.UpsertTaskCommand) (domain.Task, error) {
	if err := actor.Validate(); err != nil {
		return domain.Task{}, err
	}
	if command.ID <= 0 {
		return domain.Task{}, domain.ErrInvalidTaskID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateTask); err != nil {
		return domain.Task{}, err
	}
	if strings.TrimSpace(command.Title) == "" {
		return domain.Task{}, domain.ErrInvalidTask
	}
	return s.ops.UpsertTask(ctx, command)
}

func (s *Service) DeleteTask(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidTaskID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteTask); err != nil {
		return err
	}
	return s.ops.DeleteTask(ctx, id)
}

func (s *Service) updateUserStatus(ctx context.Context, actor domain.Actor, userID int64, status int32, action domain.Action) (domain.User, error) {
	if err := actor.Validate(); err != nil {
		return domain.User{}, err
	}
	if userID <= 0 {
		return domain.User{}, domain.ErrInvalidUserID
	}
	if err := s.auth.Authorize(ctx, actor, action); err != nil {
		return domain.User{}, err
	}
	return s.users.UpdateStatus(ctx, userID, status)
}

func (s *Service) updateArticleStatus(ctx context.Context, actor domain.Actor, id int64, action domain.Action, fn func(context.Context, int64) (domain.Article, error)) (domain.Article, error) {
	if err := actor.Validate(); err != nil {
		return domain.Article{}, err
	}
	if id <= 0 {
		return domain.Article{}, domain.ErrInvalidArticleID
	}
	if err := s.auth.Authorize(ctx, actor, action); err != nil {
		return domain.Article{}, err
	}
	return fn(ctx, id)
}

func (s *Service) updateTopicStatus(ctx context.Context, actor domain.Actor, id int64, action domain.Action, fn func(context.Context, int64) (domain.Topic, error)) (domain.Topic, error) {
	if err := actor.Validate(); err != nil {
		return domain.Topic{}, err
	}
	if id <= 0 {
		return domain.Topic{}, domain.ErrInvalidTopicID
	}
	if err := s.auth.Authorize(ctx, actor, action); err != nil {
		return domain.Topic{}, err
	}
	return fn(ctx, id)
}

func (s *Service) profileForUser(ctx context.Context, user domain.AdminUser) (domain.AdminProfile, error) {
	roles, err := s.authStore.RoleKeysByUserID(ctx, user.ID)
	if err != nil {
		return domain.AdminProfile{}, err
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}
	permissions, err := s.authStore.PermissionsByRoleKeys(ctx, roles)
	if err != nil {
		return domain.AdminProfile{}, err
	}
	return domain.AdminProfile{User: user, Roles: roles, Permissions: permissions}, nil
}
