package grpc

import (
	"context"

	pb "admin/api/proto/adminpb"
	domain "admin/internal/domain/admin"
)

func (h *Handler) ListSystemUsers(ctx context.Context, req *pb.ListSystemUsersRequest) (*pb.SystemUserListResponse, error) {
	result, err := h.service.ListSystemUsers(ctx, toActor(req.GetActor()), req.GetQuery(), req.GetStatus(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemUserListResponse{Items: toPbSystemUsers(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateSystemUser(ctx context.Context, req *pb.UpsertSystemUserRequest) (*pb.SystemUserResponse, error) {
	user, err := h.service.CreateSystemUser(ctx, toActor(req.GetActor()), toSystemUserCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemUserResponse{Success: true, Message: "ok", User: toPbSystemUser(user)}, nil
}

func (h *Handler) UpdateSystemUser(ctx context.Context, req *pb.UpsertSystemUserRequest) (*pb.SystemUserResponse, error) {
	user, err := h.service.UpdateSystemUser(ctx, toActor(req.GetActor()), toSystemUserCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemUserResponse{Success: true, Message: "ok", User: toPbSystemUser(user)}, nil
}

func (h *Handler) DeleteSystemUser(ctx context.Context, req *pb.SystemUserIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteSystemUser(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ResetSystemUserPassword(ctx context.Context, req *pb.ResetSystemUserPasswordRequest) (*pb.SystemUserResponse, error) {
	user, err := h.service.ResetSystemUserPassword(ctx, toActor(req.GetActor()), req.GetId(), req.GetPassword())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemUserResponse{Success: true, Message: "ok", User: toPbSystemUser(user)}, nil
}

func (h *Handler) AssignSystemUserRoles(ctx context.Context, req *pb.AssignSystemUserRolesRequest) (*pb.SystemUserResponse, error) {
	user, err := h.service.AssignSystemUserRoles(ctx, toActor(req.GetActor()), req.GetUserId(), req.GetRoleIds())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemUserResponse{Success: true, Message: "ok", User: toPbSystemUser(user)}, nil
}

func (h *Handler) ListSystemRoles(ctx context.Context, req *pb.ListSystemRolesRequest) (*pb.SystemRoleListResponse, error) {
	result, err := h.service.ListSystemRoles(ctx, toActor(req.GetActor()), req.GetQuery(), req.GetStatus(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemRoleListResponse{Items: toPbSystemRoles(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateSystemRole(ctx context.Context, req *pb.UpsertSystemRoleRequest) (*pb.SystemRoleResponse, error) {
	role, err := h.service.CreateSystemRole(ctx, toActor(req.GetActor()), toSystemRoleCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemRoleResponse{Success: true, Message: "ok", Role: toPbSystemRole(role)}, nil
}

func (h *Handler) UpdateSystemRole(ctx context.Context, req *pb.UpsertSystemRoleRequest) (*pb.SystemRoleResponse, error) {
	role, err := h.service.UpdateSystemRole(ctx, toActor(req.GetActor()), toSystemRoleCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemRoleResponse{Success: true, Message: "ok", Role: toPbSystemRole(role)}, nil
}

func (h *Handler) DeleteSystemRole(ctx context.Context, req *pb.SystemRoleIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteSystemRole(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) AssignSystemRoleMenus(ctx context.Context, req *pb.AssignSystemRoleMenusRequest) (*pb.SystemRoleResponse, error) {
	role, err := h.service.AssignSystemRoleMenus(ctx, toActor(req.GetActor()), req.GetRoleId(), req.GetMenuIds())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemRoleResponse{Success: true, Message: "ok", Role: toPbSystemRole(role)}, nil
}

func (h *Handler) ListSystemMenus(ctx context.Context, req *pb.ListSystemMenusRequest) (*pb.SystemMenuListResponse, error) {
	result, err := h.service.ListSystemMenus(ctx, toActor(req.GetActor()), req.GetQuery(), req.GetStatus())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemMenuListResponse{Items: toPbSystemMenus(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListCurrentSystemMenus(ctx context.Context, req *pb.CurrentSystemMenusRequest) (*pb.SystemMenuListResponse, error) {
	result, err := h.service.ListCurrentSystemMenus(ctx, toActor(req.GetActor()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemMenuListResponse{Items: toPbSystemMenus(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateSystemMenu(ctx context.Context, req *pb.UpsertSystemMenuRequest) (*pb.SystemMenuResponse, error) {
	menu, err := h.service.CreateSystemMenu(ctx, toActor(req.GetActor()), toSystemMenuCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemMenuResponse{Success: true, Message: "ok", Menu: toPbSystemMenu(menu)}, nil
}

func (h *Handler) UpdateSystemMenu(ctx context.Context, req *pb.UpsertSystemMenuRequest) (*pb.SystemMenuResponse, error) {
	menu, err := h.service.UpdateSystemMenu(ctx, toActor(req.GetActor()), toSystemMenuCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemMenuResponse{Success: true, Message: "ok", Menu: toPbSystemMenu(menu)}, nil
}

func (h *Handler) DeleteSystemMenu(ctx context.Context, req *pb.SystemMenuIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteSystemMenu(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListSystemDepts(ctx context.Context, req *pb.ListSystemDeptsRequest) (*pb.SystemDeptListResponse, error) {
	result, err := h.service.ListSystemDepts(ctx, toActor(req.GetActor()), req.GetQuery(), req.GetStatus())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemDeptListResponse{Items: toPbSystemDepts(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateSystemDept(ctx context.Context, req *pb.UpsertSystemDeptRequest) (*pb.SystemDeptResponse, error) {
	dept, err := h.service.CreateSystemDept(ctx, toActor(req.GetActor()), toSystemDeptCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemDeptResponse{Success: true, Message: "ok", Dept: toPbSystemDept(dept)}, nil
}

func (h *Handler) UpdateSystemDept(ctx context.Context, req *pb.UpsertSystemDeptRequest) (*pb.SystemDeptResponse, error) {
	dept, err := h.service.UpdateSystemDept(ctx, toActor(req.GetActor()), toSystemDeptCommand(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SystemDeptResponse{Success: true, Message: "ok", Dept: toPbSystemDept(dept)}, nil
}

func (h *Handler) DeleteSystemDept(ctx context.Context, req *pb.SystemDeptIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteSystemDept(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func toSystemUserCommand(req *pb.UpsertSystemUserRequest) domain.UpsertSystemUserCommand {
	return domain.UpsertSystemUserCommand{
		ID:        req.GetId(),
		Username:  req.GetUsername(),
		Nickname:  req.GetNickname(),
		Email:     req.GetEmail(),
		Phone:     req.GetPhone(),
		Password:  req.GetPassword(),
		AvatarURL: req.GetAvatarUrl(),
		Status:    req.GetStatus(),
		DeptID:    req.GetDeptId(),
		PostID:    req.GetPostId(),
		RoleIDs:   req.GetRoleIds(),
	}
}

func toSystemRoleCommand(req *pb.UpsertSystemRoleRequest) domain.UpsertSystemRoleCommand {
	return domain.UpsertSystemRoleCommand{
		ID:        req.GetId(),
		Name:      req.GetName(),
		Key:       req.GetKey(),
		Status:    req.GetStatus(),
		Sort:      req.GetSort(),
		Admin:     req.GetAdmin(),
		DataScope: req.GetDataScope(),
		Remark:    req.GetRemark(),
	}
}

func toSystemMenuCommand(req *pb.UpsertSystemMenuRequest) domain.UpsertSystemMenuCommand {
	return domain.UpsertSystemMenuCommand{
		ID:         req.GetId(),
		ParentID:   req.GetParentId(),
		Name:       req.GetName(),
		Title:      req.GetTitle(),
		Icon:       req.GetIcon(),
		Path:       req.GetPath(),
		Component:  req.GetComponent(),
		Type:       req.GetType(),
		Permission: req.GetPermission(),
		Status:     req.GetStatus(),
		Visible:    req.GetVisible(),
		IsHide:     req.GetIsHide(),
		Sort:       req.GetSort(),
		Remark:     req.GetRemark(),
	}
}

func toSystemDeptCommand(req *pb.UpsertSystemDeptRequest) domain.UpsertSystemDeptCommand {
	return domain.UpsertSystemDeptCommand{
		ID:       req.GetId(),
		ParentID: req.GetParentId(),
		Name:     req.GetName(),
		Sort:     req.GetSort(),
		Leader:   req.GetLeader(),
		Phone:    req.GetPhone(),
		Email:    req.GetEmail(),
		Status:   req.GetStatus(),
	}
}

func toPbSystemUser(user domain.SystemUser) *pb.SystemUserInfo {
	return &pb.SystemUserInfo{
		Id:         user.ID,
		Username:   user.Username,
		Nickname:   user.Nickname,
		Email:      user.Email,
		Phone:      user.Phone,
		AvatarUrl:  user.AvatarURL,
		Status:     user.Status,
		LockedFlag: user.LockedFlag,
		RoleId:     user.RoleID,
		DeptId:     user.DeptID,
		PostId:     user.PostID,
		RoleIds:    user.RoleIDs,
		Roles:      user.Roles,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}
}

func toPbSystemUsers(items []domain.SystemUser) []*pb.SystemUserInfo {
	out := make([]*pb.SystemUserInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbSystemUser(item))
	}
	return out
}

func toPbSystemRole(role domain.SystemRole) *pb.SystemRoleInfo {
	return &pb.SystemRoleInfo{
		Id:          role.ID,
		Name:        role.Name,
		Key:         role.Key,
		Status:      role.Status,
		Sort:        role.Sort,
		Admin:       role.Admin,
		DataScope:   role.DataScope,
		Remark:      role.Remark,
		MenuIds:     role.MenuIDs,
		Permissions: role.Permissions,
	}
}

func toPbSystemRoles(items []domain.SystemRole) []*pb.SystemRoleInfo {
	out := make([]*pb.SystemRoleInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbSystemRole(item))
	}
	return out
}

func toPbSystemMenu(menu domain.SystemMenu) *pb.SystemMenuInfo {
	return &pb.SystemMenuInfo{
		Id:         menu.ID,
		ParentId:   menu.ParentID,
		Name:       menu.Name,
		Title:      menu.Title,
		Icon:       menu.Icon,
		Path:       menu.Path,
		Component:  menu.Component,
		Type:       menu.Type,
		Permission: menu.Permission,
		Status:     menu.Status,
		Visible:    menu.Visible,
		IsHide:     menu.IsHide,
		Sort:       menu.Sort,
		Remark:     menu.Remark,
	}
}

func toPbSystemMenus(items []domain.SystemMenu) []*pb.SystemMenuInfo {
	out := make([]*pb.SystemMenuInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbSystemMenu(item))
	}
	return out
}

func toPbSystemDept(dept domain.SystemDept) *pb.SystemDeptInfo {
	return &pb.SystemDeptInfo{
		Id:        dept.ID,
		ParentId:  dept.ParentID,
		Path:      dept.Path,
		Name:      dept.Name,
		Sort:      dept.Sort,
		Leader:    dept.Leader,
		Phone:     dept.Phone,
		Email:     dept.Email,
		Status:    dept.Status,
		CreatedAt: dept.CreatedAt,
		UpdatedAt: dept.UpdatedAt,
	}
}

func toPbSystemDepts(items []domain.SystemDept) []*pb.SystemDeptInfo {
	out := make([]*pb.SystemDeptInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbSystemDept(item))
	}
	return out
}
