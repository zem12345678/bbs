package grpc

import (
	"context"
	"errors"

	app "admin/internal/application/admin"
	domain "admin/internal/domain/admin"
	pb "admin/internal/interfaces/grpc/pb/adminpb"

	stdgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedAdminServiceServer
	service *app.Service
}

func NewHandler(service *app.Service) *Handler {
	return &Handler{service: service}
}

func NewInitServers(h *Handler) func(*stdgrpc.Server) {
	return func(s *stdgrpc.Server) {
		pb.RegisterAdminServiceServer(s, h)
	}
}

func (h *Handler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	session, err := h.service.Login(ctx, req.GetAccount(), req.GetPassword(), req.GetLoginIp(), req.GetUserAgent())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AuthResponse{
		Success:     true,
		Message:     "ok",
		AccessToken: session.Token.AccessToken,
		ExpiresAt:   session.Token.ExpiresAt,
		User:        toPbAdminUser(session.Profile.User),
		Roles:       session.Profile.Roles,
		Permissions: session.Profile.Permissions,
	}, nil
}

func (h *Handler) GetProfile(ctx context.Context, req *pb.ProfileRequest) (*pb.ProfileResponse, error) {
	profile, err := h.service.GetProfile(ctx, req.GetAccessToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ProfileResponse{
		User:        toPbAdminUser(profile.User),
		Roles:       profile.Roles,
		Permissions: profile.Permissions,
	}, nil
}

func (h *Handler) ListReports(ctx context.Context, req *pb.ListReportsRequest) (*pb.ReportListResponse, error) {
	result, err := h.service.ListReports(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ReportListResponse{Items: toPbReports(result.Items), Total: result.Total}, nil
}

func (h *Handler) AuditReport(ctx context.Context, req *pb.AuditReportRequest) (*pb.ReportResponse, error) {
	report, err := h.service.AuditReport(ctx, toActor(req.GetActor()), req.GetId(), req.GetStatus())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ReportResponse{Success: true, Message: "ok", Report: toPbReport(report)}, nil
}

func (h *Handler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.UserListResponse, error) {
	result, err := h.service.ListUsers(ctx, toActor(req.GetActor()), req.GetQuery(), req.GetStatus(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListResponse{Items: toPbUsers(result.Items), Total: result.Total}, nil
}

func (h *Handler) MuteUser(ctx context.Context, req *pb.UserStatusRequest) (*pb.UserResponse, error) {
	user, err := h.service.MuteUser(ctx, toActor(req.GetActor()), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserResponse{Success: true, Message: "ok", User: toPbUser(user)}, nil
}

func (h *Handler) UnmuteUser(ctx context.Context, req *pb.UserStatusRequest) (*pb.UserResponse, error) {
	user, err := h.service.UnmuteUser(ctx, toActor(req.GetActor()), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserResponse{Success: true, Message: "ok", User: toPbUser(user)}, nil
}

func (h *Handler) ListArticles(ctx context.Context, req *pb.ListArticlesRequest) (*pb.ArticleListResponse, error) {
	result, err := h.service.ListArticles(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetTag(), req.GetAuthorId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleListResponse{Items: toPbArticles(result.Items), Total: result.Total}, nil
}

func (h *Handler) HideArticle(ctx context.Context, req *pb.ArticleStatusRequest) (*pb.ArticleResponse, error) {
	article, err := h.service.HideArticle(ctx, toActor(req.GetActor()), req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPbArticle(article)}, nil
}

func (h *Handler) ArchiveArticle(ctx context.Context, req *pb.ArticleStatusRequest) (*pb.ArticleResponse, error) {
	article, err := h.service.ArchiveArticle(ctx, toActor(req.GetActor()), req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPbArticle(article)}, nil
}

func (h *Handler) ListTopics(ctx context.Context, req *pb.ListTopicsRequest) (*pb.TopicListResponse, error) {
	result, err := h.service.ListTopics(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetType(), req.GetTag(), req.GetAuthorId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicListResponse{Items: toPbTopics(result.Items), Total: result.Total}, nil
}

func (h *Handler) HideTopic(ctx context.Context, req *pb.TopicStatusRequest) (*pb.TopicResponse, error) {
	topic, err := h.service.HideTopic(ctx, toActor(req.GetActor()), req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(topic)}, nil
}

func (h *Handler) ArchiveTopic(ctx context.Context, req *pb.TopicStatusRequest) (*pb.TopicResponse, error) {
	topic, err := h.service.ArchiveTopic(ctx, toActor(req.GetActor()), req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(topic)}, nil
}

func (h *Handler) ListComments(ctx context.Context, req *pb.ListCommentsRequest) (*pb.CommentListResponse, error) {
	result, err := h.service.ListComments(ctx, toActor(req.GetActor()), req.GetEntityType(), req.GetEntityId(), req.GetAuthorId(), req.GetStatus(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CommentListResponse{Items: toPbComments(result.Items), Total: result.Total}, nil
}

func (h *Handler) HideComment(ctx context.Context, req *pb.CommentStatusRequest) (*pb.SimpleResponse, error) {
	if err := h.service.HideComment(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListAdminUsers(ctx context.Context, req *pb.ListAdminUsersRequest) (*pb.AdminUserListResponse, error) {
	result, err := h.service.ListAdminUsers(ctx, toActor(req.GetActor()), req.GetQuery(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AdminUserListResponse{Items: toPbAdminUsers(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateAdminUser(ctx context.Context, req *pb.CreateAdminUserRequest) (*pb.AdminUserResponse, error) {
	user, err := h.service.CreateAdminUser(ctx, toActor(req.GetActor()), domain.CreateAdminUserCommand{
		Username: req.GetUsername(),
		Email:    req.GetEmail(),
		Phone:    req.GetPhone(),
		Password: req.GetPassword(),
		Nickname: req.GetNickname(),
		RoleKeys: req.GetRoleKeys(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AdminUserResponse{Success: true, Message: "ok", User: toPbAdminUser(user)}, nil
}

func (h *Handler) ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.RoleListResponse, error) {
	result, err := h.service.ListRoles(ctx, toActor(req.GetActor()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.RoleListResponse{Items: toPbRoles(result.Items), Total: result.Total}, nil
}

func (h *Handler) AssignRoles(ctx context.Context, req *pb.AssignRolesRequest) (*pb.AdminUserResponse, error) {
	user, err := h.service.AssignRoles(ctx, toActor(req.GetActor()), req.GetUserId(), req.GetRoleKeys())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AdminUserResponse{Success: true, Message: "ok", User: toPbAdminUser(user)}, nil
}

func (h *Handler) ListBadges(ctx context.Context, req *pb.ListBadgesRequest) (*pb.BadgeListResponse, error) {
	result, err := h.service.ListBadges(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.BadgeListResponse{Items: toPbBadges(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateBadge(ctx context.Context, req *pb.UpsertBadgeRequest) (*pb.BadgeResponse, error) {
	badge, err := h.service.CreateBadge(ctx, toActor(req.GetActor()), domain.UpsertBadgeCommand{
		Key:         req.GetKey(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		IconURL:     req.GetIconUrl(),
		RuleType:    req.GetRuleType(),
		RuleValue:   req.GetRuleValue(),
		Status:      req.GetStatus(),
		Sort:        req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.BadgeResponse{Success: true, Message: "ok", Badge: toPbBadge(badge)}, nil
}

func (h *Handler) UpdateBadge(ctx context.Context, req *pb.UpsertBadgeRequest) (*pb.BadgeResponse, error) {
	badge, err := h.service.UpdateBadge(ctx, toActor(req.GetActor()), domain.UpsertBadgeCommand{
		ID:          req.GetId(),
		Key:         req.GetKey(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		IconURL:     req.GetIconUrl(),
		RuleType:    req.GetRuleType(),
		RuleValue:   req.GetRuleValue(),
		Status:      req.GetStatus(),
		Sort:        req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.BadgeResponse{Success: true, Message: "ok", Badge: toPbBadge(badge)}, nil
}

func (h *Handler) DeleteBadge(ctx context.Context, req *pb.BadgeIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteBadge(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListLevels(ctx context.Context, req *pb.ListLevelsRequest) (*pb.LevelListResponse, error) {
	result, err := h.service.ListLevels(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.LevelListResponse{Items: toPbLevels(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateLevel(ctx context.Context, req *pb.UpsertLevelRequest) (*pb.LevelResponse, error) {
	level, err := h.service.CreateLevel(ctx, toActor(req.GetActor()), domain.UpsertLevelCommand{
		Key:         req.GetKey(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		MinScore:    req.GetMinScore(),
		MaxScore:    req.GetMaxScore(),
		Status:      req.GetStatus(),
		Sort:        req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.LevelResponse{Success: true, Message: "ok", Level: toPbLevel(level)}, nil
}

func (h *Handler) UpdateLevel(ctx context.Context, req *pb.UpsertLevelRequest) (*pb.LevelResponse, error) {
	level, err := h.service.UpdateLevel(ctx, toActor(req.GetActor()), domain.UpsertLevelCommand{
		ID:          req.GetId(),
		Key:         req.GetKey(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		MinScore:    req.GetMinScore(),
		MaxScore:    req.GetMaxScore(),
		Status:      req.GetStatus(),
		Sort:        req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.LevelResponse{Success: true, Message: "ok", Level: toPbLevel(level)}, nil
}

func (h *Handler) DeleteLevel(ctx context.Context, req *pb.LevelIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteLevel(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListForbiddenWords(ctx context.Context, req *pb.ListForbiddenWordsRequest) (*pb.ForbiddenWordListResponse, error) {
	result, err := h.service.ListForbiddenWords(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetQuery(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ForbiddenWordListResponse{Items: toPbForbiddenWords(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateForbiddenWord(ctx context.Context, req *pb.UpsertForbiddenWordRequest) (*pb.ForbiddenWordResponse, error) {
	word, err := h.service.CreateForbiddenWord(ctx, toActor(req.GetActor()), domain.UpsertForbiddenWordCommand{
		Word:        req.GetWord(),
		Scene:       req.GetScene(),
		Action:      req.GetAction(),
		Replacement: req.GetReplacement(),
		Description: req.GetDescription(),
		Status:      req.GetStatus(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ForbiddenWordResponse{Success: true, Message: "ok", Word: toPbForbiddenWord(word)}, nil
}

func (h *Handler) UpdateForbiddenWord(ctx context.Context, req *pb.UpsertForbiddenWordRequest) (*pb.ForbiddenWordResponse, error) {
	word, err := h.service.UpdateForbiddenWord(ctx, toActor(req.GetActor()), domain.UpsertForbiddenWordCommand{
		ID:          req.GetId(),
		Word:        req.GetWord(),
		Scene:       req.GetScene(),
		Action:      req.GetAction(),
		Replacement: req.GetReplacement(),
		Description: req.GetDescription(),
		Status:      req.GetStatus(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ForbiddenWordResponse{Success: true, Message: "ok", Word: toPbForbiddenWord(word)}, nil
}

func (h *Handler) DeleteForbiddenWord(ctx context.Context, req *pb.ForbiddenWordIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteForbiddenWord(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListSettings(ctx context.Context, req *pb.ListSettingsRequest) (*pb.SettingListResponse, error) {
	result, err := h.service.ListSettings(ctx, toActor(req.GetActor()), req.GetGroup(), req.GetStatus(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SettingListResponse{Items: toPbSettings(result.Items), Total: result.Total}, nil
}

func (h *Handler) UpdateSetting(ctx context.Context, req *pb.UpsertSettingRequest) (*pb.SettingResponse, error) {
	setting, err := h.service.UpdateSetting(ctx, toActor(req.GetActor()), domain.UpsertSettingCommand{
		ID:          req.GetId(),
		Key:         req.GetKey(),
		Value:       req.GetValue(),
		Group:       req.GetGroup(),
		ValueType:   req.GetValueType(),
		Description: req.GetDescription(),
		Status:      req.GetStatus(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SettingResponse{Success: true, Message: "ok", Setting: toPbSetting(setting)}, nil
}

func (h *Handler) ListEmailLogs(ctx context.Context, req *pb.ListEmailLogsRequest) (*pb.EmailLogListResponse, error) {
	result, err := h.service.ListEmailLogs(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetQuery(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.EmailLogListResponse{Items: toPbEmailLogs(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListLoginLogs(ctx context.Context, req *pb.ListLoginLogsRequest) (*pb.LoginLogListResponse, error) {
	result, err := h.service.ListLoginLogs(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetQuery(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.LoginLogListResponse{Items: toPbLoginLogs(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListOperationLogs(ctx context.Context, req *pb.ListOperationLogsRequest) (*pb.OperationLogListResponse, error) {
	result, err := h.service.ListOperationLogs(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetQuery(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.OperationLogListResponse{Items: toPbOperationLogs(result.Items), Total: result.Total}, nil
}

func (h *Handler) RecordOperationLog(ctx context.Context, req *pb.RecordOperationLogRequest) (*pb.SimpleResponse, error) {
	if err := h.service.RecordOperationLog(ctx, toActor(req.GetActor()), domain.RecordOperationLogCommand{
		Title:         req.GetTitle(),
		BusinessType:  req.GetBusinessType(),
		Method:        req.GetMethod(),
		RequestMethod: req.GetRequestMethod(),
		OperatorType:  req.GetOperatorType(),
		URL:           req.GetUrl(),
		IP:            req.GetIp(),
		Params:        req.GetParams(),
		Status:        req.GetStatus(),
		Result:        req.GetResult(),
		Remark:        req.GetRemark(),
		LatencyTime:   req.GetLatencyTime(),
		UserAgent:     req.GetUserAgent(),
	}); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListLinks(ctx context.Context, req *pb.ListLinksRequest) (*pb.LinkListResponse, error) {
	result, err := h.service.ListLinks(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.LinkListResponse{Items: toPbLinks(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateLink(ctx context.Context, req *pb.UpsertLinkRequest) (*pb.LinkResponse, error) {
	link, err := h.service.CreateLink(ctx, toActor(req.GetActor()), domain.UpsertLinkCommand{
		Key:         req.GetKey(),
		Title:       req.GetTitle(),
		URL:         req.GetUrl(),
		Description: req.GetDescription(),
		Status:      req.GetStatus(),
		Sort:        req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.LinkResponse{Success: true, Message: "ok", Link: toPbLink(link)}, nil
}

func (h *Handler) UpdateLink(ctx context.Context, req *pb.UpsertLinkRequest) (*pb.LinkResponse, error) {
	link, err := h.service.UpdateLink(ctx, toActor(req.GetActor()), domain.UpsertLinkCommand{
		ID:          req.GetId(),
		Key:         req.GetKey(),
		Title:       req.GetTitle(),
		URL:         req.GetUrl(),
		Description: req.GetDescription(),
		Status:      req.GetStatus(),
		Sort:        req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.LinkResponse{Success: true, Message: "ok", Link: toPbLink(link)}, nil
}

func (h *Handler) DeleteLink(ctx context.Context, req *pb.LinkIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteLink(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.TaskListResponse, error) {
	result, err := h.service.ListTasks(ctx, toActor(req.GetActor()), req.GetStatus(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TaskListResponse{Items: toPbTasks(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateTask(ctx context.Context, req *pb.UpsertTaskRequest) (*pb.TaskResponse, error) {
	task, err := h.service.CreateTask(ctx, toActor(req.GetActor()), domain.UpsertTaskCommand{
		Key:          req.GetKey(),
		Title:        req.GetTitle(),
		Description:  req.GetDescription(),
		RewardPoints: req.GetRewardPoints(),
		Status:       req.GetStatus(),
		Sort:         req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TaskResponse{Success: true, Message: "ok", Task: toPbTask(task)}, nil
}

func (h *Handler) UpdateTask(ctx context.Context, req *pb.UpsertTaskRequest) (*pb.TaskResponse, error) {
	task, err := h.service.UpdateTask(ctx, toActor(req.GetActor()), domain.UpsertTaskCommand{
		ID:           req.GetId(),
		Key:          req.GetKey(),
		Title:        req.GetTitle(),
		Description:  req.GetDescription(),
		RewardPoints: req.GetRewardPoints(),
		Status:       req.GetStatus(),
		Sort:         req.GetSort(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TaskResponse{Success: true, Message: "ok", Task: toPbTask(task)}, nil
}

func (h *Handler) DeleteTask(ctx context.Context, req *pb.TaskIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteTask(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func toActor(a *pb.Actor) domain.Actor {
	if a == nil {
		return domain.Actor{}
	}
	return domain.Actor{ID: a.GetId(), Username: a.GetUsername()}
}

func toPbAdminUser(u domain.AdminUser) *pb.AdminUserInfo {
	return &pb.AdminUserInfo{
		Id:         u.ID,
		Username:   u.Username,
		Email:      u.Email,
		Nickname:   u.Nickname,
		AvatarUrl:  u.AvatarURL,
		Status:     u.Status,
		LockedFlag: u.LockedFlag,
		Roles:      u.Roles,
	}
}

func toPbAdminUsers(items []domain.AdminUser) []*pb.AdminUserInfo {
	out := make([]*pb.AdminUserInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbAdminUser(item))
	}
	return out
}

func toPbRoles(items []domain.Role) []*pb.RoleInfo {
	out := make([]*pb.RoleInfo, 0, len(items))
	for _, item := range items {
		out = append(out, &pb.RoleInfo{
			Id:          item.ID,
			Name:        item.Name,
			Key:         item.Key,
			Status:      item.Status,
			Sort:        item.Sort,
			Admin:       item.Admin,
			Remark:      item.Remark,
			Permissions: item.Permissions,
		})
	}
	return out
}

func toPbReports(items []domain.Report) []*pb.ReportInfo {
	out := make([]*pb.ReportInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbReport(item))
	}
	return out
}

func toPbArticles(items []domain.Article) []*pb.ArticleInfo {
	out := make([]*pb.ArticleInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbArticle(item))
	}
	return out
}

func toPbTopics(items []domain.Topic) []*pb.TopicInfo {
	out := make([]*pb.TopicInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbTopic(item))
	}
	return out
}

func toPbComments(items []domain.Comment) []*pb.CommentInfo {
	out := make([]*pb.CommentInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbComment(item))
	}
	return out
}

func toPbUsers(items []domain.User) []*pb.UserInfo {
	out := make([]*pb.UserInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbUser(item))
	}
	return out
}

func toPbBadges(items []domain.Badge) []*pb.BadgeInfo {
	out := make([]*pb.BadgeInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbBadge(item))
	}
	return out
}

func toPbLevels(items []domain.Level) []*pb.LevelInfo {
	out := make([]*pb.LevelInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbLevel(item))
	}
	return out
}

func toPbForbiddenWords(items []domain.ForbiddenWord) []*pb.ForbiddenWordInfo {
	out := make([]*pb.ForbiddenWordInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbForbiddenWord(item))
	}
	return out
}

func toPbSettings(items []domain.Setting) []*pb.SettingInfo {
	out := make([]*pb.SettingInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbSetting(item))
	}
	return out
}

func toPbEmailLogs(items []domain.EmailLog) []*pb.EmailLogInfo {
	out := make([]*pb.EmailLogInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbEmailLog(item))
	}
	return out
}

func toPbLoginLogs(items []domain.LoginLog) []*pb.LoginLogInfo {
	out := make([]*pb.LoginLogInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbLoginLog(item))
	}
	return out
}

func toPbOperationLogs(items []domain.OperationLog) []*pb.OperationLogInfo {
	out := make([]*pb.OperationLogInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbOperationLog(item))
	}
	return out
}

func toPbLinks(items []domain.Link) []*pb.LinkInfo {
	out := make([]*pb.LinkInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbLink(item))
	}
	return out
}

func toPbTasks(items []domain.Task) []*pb.TaskInfo {
	out := make([]*pb.TaskInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbTask(item))
	}
	return out
}

func toPbReport(r domain.Report) *pb.ReportInfo {
	return &pb.ReportInfo{
		Id:          r.ID,
		Entity:      &pb.EntityRef{EntityType: r.Entity.EntityType, EntityId: r.Entity.EntityID},
		ReporterId:  r.ReporterID,
		Reason:      r.Reason,
		Description: r.Description,
		Status:      r.Status,
		HandledBy:   r.HandledBy,
		HandledAt:   r.HandledAt,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toPbArticle(a domain.Article) *pb.ArticleInfo {
	return &pb.ArticleInfo{
		Id:          a.ID,
		Slug:        a.Slug,
		Title:       a.Title,
		Summary:     a.Summary,
		Body:        a.Body,
		CoverUrl:    a.CoverURL,
		Tags:        a.Tags,
		AuthorId:    a.AuthorID,
		Status:      a.Status,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
		PublishedAt: a.PublishedAt,
	}
}

func toPbTopic(t domain.Topic) *pb.TopicInfo {
	return &pb.TopicInfo{
		Id:          t.ID,
		Slug:        t.Slug,
		Type:        t.Type,
		Title:       t.Title,
		Body:        t.Body,
		Tags:        t.Tags,
		AuthorId:    t.AuthorID,
		Status:      t.Status,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		PublishedAt: t.PublishedAt,
	}
}

func toPbComment(c domain.Comment) *pb.CommentInfo {
	return &pb.CommentInfo{
		Id:         c.ID,
		EntityType: c.EntityType,
		EntityId:   c.EntityID,
		RootId:     c.RootID,
		ParentId:   c.ParentID,
		AuthorId:   c.AuthorID,
		Content:    c.Content,
		Status:     c.Status,
		ReplyCount: c.ReplyCount,
		LikeCount:  c.LikeCount,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func toPbUser(u domain.User) *pb.UserInfo {
	return &pb.UserInfo{
		Id:             u.ID,
		Username:       u.Username,
		Email:          u.Email,
		Nickname:       u.Nickname,
		AvatarUrl:      u.AvatarURL,
		Bio:            u.Bio,
		Status:         u.Status,
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
		LastLoginAt:    u.LastLoginAt,
	}
}

func toPbBadge(badge domain.Badge) *pb.BadgeInfo {
	return &pb.BadgeInfo{
		Id:          badge.ID,
		Key:         badge.Key,
		Name:        badge.Name,
		Description: badge.Description,
		IconUrl:     badge.IconURL,
		RuleType:    badge.RuleType,
		RuleValue:   badge.RuleValue,
		Status:      badge.Status,
		Sort:        badge.Sort,
		CreatedAt:   badge.CreatedAt,
		UpdatedAt:   badge.UpdatedAt,
	}
}

func toPbLevel(level domain.Level) *pb.LevelInfo {
	return &pb.LevelInfo{
		Id:          level.ID,
		Key:         level.Key,
		Name:        level.Name,
		Description: level.Description,
		MinScore:    level.MinScore,
		MaxScore:    level.MaxScore,
		Status:      level.Status,
		Sort:        level.Sort,
		CreatedAt:   level.CreatedAt,
		UpdatedAt:   level.UpdatedAt,
	}
}

func toPbForbiddenWord(word domain.ForbiddenWord) *pb.ForbiddenWordInfo {
	return &pb.ForbiddenWordInfo{
		Id:          word.ID,
		Word:        word.Word,
		Scene:       word.Scene,
		Action:      word.Action,
		Replacement: word.Replacement,
		Description: word.Description,
		Status:      word.Status,
		CreatedAt:   word.CreatedAt,
		UpdatedAt:   word.UpdatedAt,
	}
}

func toPbSetting(setting domain.Setting) *pb.SettingInfo {
	return &pb.SettingInfo{
		Id:          setting.ID,
		Key:         setting.Key,
		Value:       setting.Value,
		Group:       setting.Group,
		ValueType:   setting.ValueType,
		Description: setting.Description,
		Status:      setting.Status,
		CreatedAt:   setting.CreatedAt,
		UpdatedAt:   setting.UpdatedAt,
	}
}

func toPbEmailLog(log domain.EmailLog) *pb.EmailLogInfo {
	return &pb.EmailLogInfo{
		Id:          log.ID,
		To:          log.To,
		Subject:     log.Subject,
		TemplateKey: log.TemplateKey,
		Provider:    log.Provider,
		Status:      log.Status,
		Error:       log.Error,
		CreatedAt:   log.CreatedAt,
		UpdatedAt:   log.UpdatedAt,
	}
}

func toPbLoginLog(log domain.LoginLog) *pb.LoginLogInfo {
	return &pb.LoginLogInfo{
		Id:        log.ID,
		Username:  log.Username,
		Status:    log.Status,
		Ip:        log.IP,
		Location:  log.Location,
		Browser:   log.Browser,
		Os:        log.OS,
		Platform:  log.Platform,
		Message:   log.Message,
		Remark:    log.Remark,
		LoginTime: log.LoginTime,
	}
}

func toPbOperationLog(log domain.OperationLog) *pb.OperationLogInfo {
	return &pb.OperationLogInfo{
		Id:            log.ID,
		Title:         log.Title,
		BusinessType:  log.BusinessType,
		Method:        log.Method,
		RequestMethod: log.RequestMethod,
		OperatorType:  log.OperatorType,
		OperatorName:  log.OperatorName,
		DeptName:      log.DeptName,
		Url:           log.URL,
		Ip:            log.IP,
		Location:      log.Location,
		Params:        log.Params,
		Status:        log.Status,
		OperationTime: log.OperationTime,
		Result:        log.Result,
		Remark:        log.Remark,
		LatencyTime:   log.LatencyTime,
		UserAgent:     log.UserAgent,
	}
}

func toPbLink(link domain.Link) *pb.LinkInfo {
	return &pb.LinkInfo{
		Id:          link.ID,
		Key:         link.Key,
		Title:       link.Title,
		Url:         link.URL,
		Description: link.Description,
		Status:      link.Status,
		Sort:        link.Sort,
		CreatedAt:   link.CreatedAt,
		UpdatedAt:   link.UpdatedAt,
	}
}

func toPbTask(task domain.Task) *pb.TaskInfo {
	return &pb.TaskInfo{
		Id:           task.ID,
		Key:          task.Key,
		Title:        task.Title,
		Description:  task.Description,
		RewardPoints: task.RewardPoints,
		Status:       task.Status,
		Sort:         task.Sort,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	code := codes.Internal
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrInvalidToken):
		code = codes.Unauthenticated
	case errors.Is(err, domain.ErrAdminDisabled):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrPermissionDenied),
		errors.Is(err, domain.ErrProtectedSystemRole):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrAdminUserExists):
		code = codes.AlreadyExists
	case errors.Is(err, domain.ErrInvalidActor),
		errors.Is(err, domain.ErrInvalidArticleID),
		errors.Is(err, domain.ErrInvalidAdminUserID),
		errors.Is(err, domain.ErrInvalidBadge),
		errors.Is(err, domain.ErrInvalidBadgeID),
		errors.Is(err, domain.ErrInvalidCommentID),
		errors.Is(err, domain.ErrInvalidForbiddenWord),
		errors.Is(err, domain.ErrInvalidForbiddenWordID),
		errors.Is(err, domain.ErrInvalidLevel),
		errors.Is(err, domain.ErrInvalidLevelID),
		errors.Is(err, domain.ErrInvalidLink),
		errors.Is(err, domain.ErrInvalidLinkID),
		errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrInvalidRoleKeys),
		errors.Is(err, domain.ErrInvalidReportID),
		errors.Is(err, domain.ErrInvalidSetting),
		errors.Is(err, domain.ErrInvalidSettingID),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidSystemDept),
		errors.Is(err, domain.ErrInvalidSystemMenu),
		errors.Is(err, domain.ErrInvalidSystemRole),
		errors.Is(err, domain.ErrInvalidSystemUser),
		errors.Is(err, domain.ErrInvalidTask),
		errors.Is(err, domain.ErrInvalidTaskID),
		errors.Is(err, domain.ErrInvalidTopicID),
		errors.Is(err, domain.ErrInvalidUserID):
		code = codes.InvalidArgument
	}
	return status.Error(code, err.Error())
}
