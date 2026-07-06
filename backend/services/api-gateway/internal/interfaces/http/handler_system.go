package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"api-gateway/internal/clients/pb/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

type upsertSystemUserRequest struct {
	Username  string  `json:"username"`
	Nickname  string  `json:"nickname"`
	Email     string  `json:"email"`
	Phone     string  `json:"phone"`
	Password  string  `json:"password"`
	AvatarURL string  `json:"avatar_url"`
	Status    int32   `json:"status"`
	DeptID    int64   `json:"dept_id"`
	PostID    int64   `json:"post_id"`
	RoleIDs   []int64 `json:"role_ids"`
}

type resetSystemUserPasswordRequest struct {
	Password string `json:"password"`
	NewPwd   string `json:"newPwd"`
}

type assignSystemUserRolesRequest struct {
	RoleIDs []int64 `json:"role_ids"`
	IDs     []int64 `json:"ids"`
}

type upsertSystemRoleRequest struct {
	Name      string `json:"name"`
	Key       string `json:"key"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	Sort      int32  `json:"sort"`
	Admin     bool   `json:"admin"`
	DataScope string `json:"data_scope"`
	Remark    string `json:"remark"`
}

type assignSystemRoleMenusRequest struct {
	MenuIDs []int64 `json:"menu_ids"`
	IDs     []int64 `json:"ids"`
}

type upsertSystemMenuRequest struct {
	ParentID   int64  `json:"parent_id"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Icon       string `json:"icon"`
	Path       string `json:"path"`
	Component  string `json:"component"`
	Type       string `json:"type"`
	MenuType   int32  `json:"menuType"`
	Permission string `json:"permission"`
	Auths      string `json:"auths"`
	Status     string `json:"status"`
	Visible    string `json:"visible"`
	IsHide     string `json:"is_hide"`
	Sort       int32  `json:"sort"`
	Rank       int32  `json:"rank"`
	Remark     string `json:"remark"`
	ShowLink   *bool  `json:"showLink"`
}

type upsertSystemDeptRequest struct {
	ParentID  int64  `json:"parent_id"`
	Name      string `json:"name"`
	Sort      int32  `json:"sort"`
	Leader    string `json:"leader"`
	Principal string `json:"principal"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Status    int32  `json:"status"`
	Remark    string `json:"remark"`
}

type systemMenuNode struct {
	ID         int64             `json:"id"`
	ParentID   int64             `json:"parent_id"`
	ParentId   int64             `json:"parentId"`
	Name       string            `json:"name"`
	Title      string            `json:"title"`
	Icon       string            `json:"icon"`
	Path       string            `json:"path"`
	Component  string            `json:"component"`
	Type       string            `json:"type"`
	MenuType   int32             `json:"menuType"`
	Permission string            `json:"permission"`
	Auths      string            `json:"auths"`
	Status     string            `json:"status"`
	Visible    string            `json:"visible"`
	IsHide     string            `json:"is_hide"`
	IsHideText string            `json:"isHide"`
	Sort       int32             `json:"sort"`
	Rank       int32             `json:"rank"`
	Remark     string            `json:"remark"`
	Redirect   string            `json:"redirect"`
	ExtraIcon  string            `json:"extraIcon"`
	EnterTrans string            `json:"enterTransition"`
	LeaveTrans string            `json:"leaveTransition"`
	ActivePath string            `json:"activePath"`
	FrameSrc   string            `json:"frameSrc"`
	FrameLoad  bool              `json:"frameLoading"`
	KeepAlive  bool              `json:"keepAlive"`
	HiddenTag  bool              `json:"hiddenTag"`
	FixedTag   bool              `json:"fixedTag"`
	ShowLink   bool              `json:"showLink"`
	ShowParent bool              `json:"showParent"`
	Children   []*systemMenuNode `json:"children,omitempty"`
}

type systemDeptNode struct {
	ID             int64             `json:"id"`
	ParentID       int64             `json:"parent_id"`
	ParentId       int64             `json:"parentId"`
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	Sort           int32             `json:"sort"`
	Leader         string            `json:"leader"`
	Principal      string            `json:"principal"`
	Phone          string            `json:"phone"`
	Email          string            `json:"email"`
	Status         int32             `json:"status"`
	Type           int32             `json:"type"`
	CreateAt       int64             `json:"createTime"`
	CreatedAt      int64             `json:"created_at"`
	CreatedAtCamel int64             `json:"createdAt"`
	UpdateAt       int64             `json:"updateTime"`
	UpdatedAt      int64             `json:"updated_at"`
	UpdatedAtCamel int64             `json:"updatedAt"`
	Remark         string            `json:"remark"`
	Children       []*systemDeptNode `json:"children,omitempty"`
}

func (h *Handler) listSystemUsers(c *gin.Context) {
	page, pageSize := systemPage(c)
	actor := currentActor(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSystemUsers(ctx, &adminpb.ListSystemUsersRequest{
		Actor:    actor,
		Query:    firstQuery(c, "query", "username", "phone"),
		Status:   queryInt32(c, "status", 0),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	deptNames, err := h.systemDeptNames(ctx, actor)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, systemTablePayload(toHTTPSystemUsers(resp.GetItems(), deptNames), resp.GetTotal(), page, pageSize))
}

func (h *Handler) getSystemUser(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSystemUsers(ctx, &adminpb.ListSystemUsersRequest{
		Actor:    currentActor(c),
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	for _, user := range resp.GetItems() {
		if user.GetId() == id {
			deptNames, err := h.systemDeptNames(ctx, currentActor(c))
			if err != nil {
				writeRPCError(c, err)
				return
			}
			response.Success(c, toHTTPSystemUser(user, deptNames))
			return
		}
	}
	writeError(c, http.StatusNotFound, "system user not found", "not_found")
}

func (h *Handler) createSystemUser(c *gin.Context) {
	var req upsertSystemUserRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateSystemUser(ctx, toPbUpsertSystemUserRequest(currentActor(c), 0, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"user": toHTTPSystemUser(resp.GetUser(), nil), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) updateSystemUser(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertSystemUserRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateSystemUser(ctx, toPbUpsertSystemUserRequest(currentActor(c), id, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"user": toHTTPSystemUser(resp.GetUser(), nil), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) deleteSystemUser(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteSystemUser(ctx, &adminpb.SystemUserIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) resetSystemUserPassword(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req resetSystemUserPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		password = strings.TrimSpace(req.NewPwd)
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ResetSystemUserPassword(ctx, &adminpb.ResetSystemUserPasswordRequest{
		Actor:    currentActor(c),
		Id:       id,
		Password: password,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"user": toHTTPSystemUser(resp.GetUser(), nil), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) assignSystemUserRoles(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req assignSystemUserRolesRequest
	if !bindJSON(c, &req) {
		return
	}
	roleIDs := req.RoleIDs
	if len(roleIDs) == 0 {
		roleIDs = req.IDs
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.AssignSystemUserRoles(ctx, &adminpb.AssignSystemUserRolesRequest{
		Actor:   currentActor(c),
		UserId:  id,
		RoleIds: roleIDs,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"user": toHTTPSystemUser(resp.GetUser(), nil), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) listSystemRoles(c *gin.Context) {
	page, pageSize := systemPage(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSystemRoles(ctx, &adminpb.ListSystemRolesRequest{
		Actor:    currentActor(c),
		Query:    firstQuery(c, "query", "name", "code"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, systemTablePayload(toHTTPSystemRoles(resp.GetItems()), resp.GetTotal(), page, pageSize))
}

func (h *Handler) createSystemRole(c *gin.Context) {
	var req upsertSystemRoleRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateSystemRole(ctx, toPbUpsertSystemRoleRequest(currentActor(c), 0, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"role": toHTTPSystemRole(resp.GetRole()), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) updateSystemRole(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertSystemRoleRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateSystemRole(ctx, toPbUpsertSystemRoleRequest(currentActor(c), id, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"role": toHTTPSystemRole(resp.GetRole()), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) deleteSystemRole(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteSystemRole(ctx, &adminpb.SystemRoleIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) getSystemRoleMenuIDs(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSystemRoles(ctx, &adminpb.ListSystemRolesRequest{
		Actor:    currentActor(c),
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	for _, role := range resp.GetItems() {
		if role.GetId() == id {
			response.Success(c, role.GetMenuIds())
			return
		}
	}
	writeError(c, http.StatusNotFound, "system role not found", "not_found")
}

func (h *Handler) getSystemRolePermissions(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSystemRoles(ctx, &adminpb.ListSystemRolesRequest{
		Actor:    currentActor(c),
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	for _, role := range resp.GetItems() {
		if role.GetId() == id {
			response.Success(c, gin.H{"role_id": id, "menu_ids": role.GetMenuIds(), "permissions": role.GetPermissions()})
			return
		}
	}
	writeError(c, http.StatusNotFound, "system role not found", "not_found")
}

func (h *Handler) assignSystemRoleMenus(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req assignSystemRoleMenusRequest
	if !bindJSON(c, &req) {
		return
	}
	menuIDs := req.MenuIDs
	if len(menuIDs) == 0 {
		menuIDs = req.IDs
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.AssignSystemRoleMenus(ctx, &adminpb.AssignSystemRoleMenusRequest{
		Actor:   currentActor(c),
		RoleId:  id,
		MenuIds: menuIDs,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"role": toHTTPSystemRole(resp.GetRole()), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) listSystemMenus(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSystemMenus(ctx, &adminpb.ListSystemMenusRequest{
		Actor:  currentActor(c),
		Query:  firstQuery(c, "query", "title", "name"),
		Status: c.Query("status"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	flat := toHTTPSystemMenus(resp.GetItems())
	tree := buildSystemMenuTree(flat)
	response.Success(c, gin.H{"items": tree, "list": tree, "flat_items": flat, "flatList": flat, "total": resp.GetTotal()})
}

func (h *Handler) listCurrentAdminMenus(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListCurrentSystemMenus(ctx, &adminpb.CurrentSystemMenusRequest{
		Actor: currentActor(c),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	flat := routeSystemMenuNodes(toHTTPSystemMenus(resp.GetItems()))
	tree := buildSystemMenuTree(flat)
	response.Success(c, gin.H{"items": tree, "list": tree, "flat_items": flat, "flatList": flat, "total": len(flat)})
}

func (h *Handler) createSystemMenu(c *gin.Context) {
	var req upsertSystemMenuRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateSystemMenu(ctx, toPbUpsertSystemMenuRequest(currentActor(c), 0, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"menu": toHTTPSystemMenu(resp.GetMenu()), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) updateSystemMenu(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertSystemMenuRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateSystemMenu(ctx, toPbUpsertSystemMenuRequest(currentActor(c), id, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"menu": toHTTPSystemMenu(resp.GetMenu()), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) deleteSystemMenu(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteSystemMenu(ctx, &adminpb.SystemMenuIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) listSystemDepts(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListSystemDepts(ctx, &adminpb.ListSystemDeptsRequest{
		Actor:  currentActor(c),
		Query:  firstQuery(c, "query", "name"),
		Status: queryInt32(c, "status", 0),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	flat := toHTTPSystemDepts(resp.GetItems())
	tree := buildSystemDeptTree(flat)
	response.Success(c, gin.H{"items": tree, "list": tree, "flat_items": flat, "flatList": flat, "total": resp.GetTotal()})
}

func (h *Handler) createSystemDept(c *gin.Context) {
	var req upsertSystemDeptRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateSystemDept(ctx, toPbUpsertSystemDeptRequest(currentActor(c), 0, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"dept": toHTTPSystemDept(resp.GetDept()), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) updateSystemDept(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	var req upsertSystemDeptRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.UpdateSystemDept(ctx, toPbUpsertSystemDeptRequest(currentActor(c), id, req))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, gin.H{"dept": toHTTPSystemDept(resp.GetDept()), "success": resp.GetSuccess(), "message": resp.GetMessage()})
}

func (h *Handler) deleteSystemDept(c *gin.Context) {
	id, ok := pathInt64(c, "id")
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.DeleteSystemDept(ctx, &adminpb.SystemDeptIDRequest{Actor: currentActor(c), Id: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func toPbUpsertSystemUserRequest(actor *adminpb.Actor, id int64, req upsertSystemUserRequest) *adminpb.UpsertSystemUserRequest {
	return &adminpb.UpsertSystemUserRequest{
		Actor:     actor,
		Id:        id,
		Username:  req.Username,
		Nickname:  req.Nickname,
		Email:     req.Email,
		Phone:     req.Phone,
		Password:  req.Password,
		AvatarUrl: req.AvatarURL,
		Status:    req.Status,
		DeptId:    req.DeptID,
		PostId:    req.PostID,
		RoleIds:   req.RoleIDs,
	}
}

func toPbUpsertSystemRoleRequest(actor *adminpb.Actor, id int64, req upsertSystemRoleRequest) *adminpb.UpsertSystemRoleRequest {
	key := strings.TrimSpace(req.Key)
	if key == "" {
		key = strings.TrimSpace(req.Code)
	}
	statusValue := strings.TrimSpace(req.Status)
	if statusValue == "" {
		statusValue = "1"
	}
	dataScope := strings.TrimSpace(req.DataScope)
	if dataScope == "" {
		dataScope = "1"
	}
	return &adminpb.UpsertSystemRoleRequest{
		Actor:     actor,
		Id:        id,
		Name:      req.Name,
		Key:       key,
		Status:    statusValue,
		Sort:      req.Sort,
		Admin:     req.Admin,
		DataScope: dataScope,
		Remark:    req.Remark,
	}
}

func toPbUpsertSystemMenuRequest(actor *adminpb.Actor, id int64, req upsertSystemMenuRequest) *adminpb.UpsertSystemMenuRequest {
	permission := strings.TrimSpace(req.Permission)
	if permission == "" {
		permission = strings.TrimSpace(req.Auths)
	}
	sort := req.Sort
	if sort == 0 {
		sort = req.Rank
	}
	visible, isHide := req.Visible, req.IsHide
	if req.ShowLink != nil {
		if *req.ShowLink {
			visible, isHide = "0", "0"
		} else {
			visible, isHide = "1", "1"
		}
	}
	return &adminpb.UpsertSystemMenuRequest{
		Actor:      actor,
		Id:         id,
		ParentId:   req.ParentID,
		Name:       req.Name,
		Title:      req.Title,
		Icon:       req.Icon,
		Path:       req.Path,
		Component:  req.Component,
		Type:       systemMenuType(req.Type, req.MenuType),
		Permission: permission,
		Status:     defaultHTTPString(req.Status, "0"),
		Visible:    defaultHTTPString(visible, "0"),
		IsHide:     defaultHTTPString(isHide, "0"),
		Sort:       sort,
		Remark:     req.Remark,
	}
}

func toPbUpsertSystemDeptRequest(actor *adminpb.Actor, id int64, req upsertSystemDeptRequest) *adminpb.UpsertSystemDeptRequest {
	leader := strings.TrimSpace(req.Leader)
	if leader == "" {
		leader = strings.TrimSpace(req.Principal)
	}
	return &adminpb.UpsertSystemDeptRequest{
		Actor:    actor,
		Id:       id,
		ParentId: req.ParentID,
		Name:     req.Name,
		Sort:     req.Sort,
		Leader:   leader,
		Phone:    req.Phone,
		Email:    req.Email,
		Status:   req.Status,
	}
}

func toHTTPSystemUser(user *adminpb.SystemUserInfo, deptNames map[int64]string) gin.H {
	if user == nil {
		return gin.H{}
	}
	deptName := deptNames[user.GetDeptId()]
	return gin.H{
		"id":          user.GetId(),
		"username":    user.GetUsername(),
		"nickname":    user.GetNickname(),
		"email":       user.GetEmail(),
		"phone":       user.GetPhone(),
		"avatar":      user.GetAvatarUrl(),
		"avatar_url":  user.GetAvatarUrl(),
		"avatarUrl":   user.GetAvatarUrl(),
		"status":      user.GetStatus(),
		"locked_flag": user.GetLockedFlag(),
		"lockedFlag":  user.GetLockedFlag(),
		"role_id":     user.GetRoleId(),
		"roleId":      user.GetRoleId(),
		"dept_id":     user.GetDeptId(),
		"deptId":      user.GetDeptId(),
		"post_id":     user.GetPostId(),
		"postId":      user.GetPostId(),
		"role_ids":    user.GetRoleIds(),
		"roleIds":     user.GetRoleIds(),
		"roles":       user.GetRoles(),
		"dept":        gin.H{"id": user.GetDeptId(), "name": deptName},
		"createTime":  user.GetCreatedAt(),
		"created_at":  user.GetCreatedAt(),
		"createdAt":   user.GetCreatedAt(),
		"updated_at":  user.GetUpdatedAt(),
		"updatedAt":   user.GetUpdatedAt(),
	}
}

func toHTTPSystemUsers(items []*adminpb.SystemUserInfo, deptNames map[int64]string) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, toHTTPSystemUser(item, deptNames))
	}
	return out
}

func toHTTPSystemRole(role *adminpb.SystemRoleInfo) gin.H {
	if role == nil {
		return gin.H{}
	}
	status := role.GetStatus()
	return gin.H{
		"id":          role.GetId(),
		"name":        role.GetName(),
		"key":         role.GetKey(),
		"code":        role.GetKey(),
		"status":      statusToInt(status),
		"status_text": status,
		"sort":        role.GetSort(),
		"admin":       role.GetAdmin(),
		"data_scope":  role.GetDataScope(),
		"dataScope":   role.GetDataScope(),
		"remark":      role.GetRemark(),
		"menu_ids":    role.GetMenuIds(),
		"menuIds":     role.GetMenuIds(),
		"permissions": role.GetPermissions(),
	}
}

func toHTTPSystemRoles(items []*adminpb.SystemRoleInfo) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, toHTTPSystemRole(item))
	}
	return out
}

func toHTTPSystemMenu(menu *adminpb.SystemMenuInfo) *systemMenuNode {
	if menu == nil {
		return nil
	}
	return &systemMenuNode{
		ID:         menu.GetId(),
		ParentID:   menu.GetParentId(),
		ParentId:   menu.GetParentId(),
		Name:       menu.GetName(),
		Title:      menu.GetTitle(),
		Icon:       menu.GetIcon(),
		Path:       menu.GetPath(),
		Component:  menu.GetComponent(),
		Type:       menu.GetType(),
		MenuType:   templateMenuType(menu.GetType()),
		Permission: menu.GetPermission(),
		Auths:      menu.GetPermission(),
		Status:     menu.GetStatus(),
		Visible:    menu.GetVisible(),
		IsHide:     menu.GetIsHide(),
		IsHideText: menu.GetIsHide(),
		Sort:       menu.GetSort(),
		Rank:       menu.GetSort(),
		Remark:     menu.GetRemark(),
		FrameLoad:  true,
		ShowLink:   menu.GetVisible() != "1" && menu.GetIsHide() != "1",
	}
}

func toHTTPSystemMenus(items []*adminpb.SystemMenuInfo) []*systemMenuNode {
	out := make([]*systemMenuNode, 0, len(items))
	for _, item := range items {
		if converted := toHTTPSystemMenu(item); converted != nil {
			out = append(out, converted)
		}
	}
	return out
}

func routeSystemMenuNodes(items []*systemMenuNode) []*systemMenuNode {
	out := make([]*systemMenuNode, 0, len(items))
	for _, item := range items {
		if item == nil || item.MenuType == 3 {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(item.Type)) {
		case "F", "B":
			continue
		default:
			out = append(out, item)
		}
	}
	return out
}

func toHTTPSystemDept(dept *adminpb.SystemDeptInfo) *systemDeptNode {
	if dept == nil {
		return nil
	}
	return &systemDeptNode{
		ID:             dept.GetId(),
		ParentID:       dept.GetParentId(),
		ParentId:       dept.GetParentId(),
		Path:           dept.GetPath(),
		Name:           dept.GetName(),
		Sort:           dept.GetSort(),
		Leader:         dept.GetLeader(),
		Principal:      dept.GetLeader(),
		Phone:          dept.GetPhone(),
		Email:          dept.GetEmail(),
		Status:         dept.GetStatus(),
		Type:           3,
		CreateAt:       dept.GetCreatedAt(),
		CreatedAt:      dept.GetCreatedAt(),
		CreatedAtCamel: dept.GetCreatedAt(),
		UpdateAt:       dept.GetUpdatedAt(),
		UpdatedAt:      dept.GetUpdatedAt(),
		UpdatedAtCamel: dept.GetUpdatedAt(),
	}
}

func toHTTPSystemDepts(items []*adminpb.SystemDeptInfo) []*systemDeptNode {
	out := make([]*systemDeptNode, 0, len(items))
	for _, item := range items {
		if converted := toHTTPSystemDept(item); converted != nil {
			out = append(out, converted)
		}
	}
	return out
}

func buildSystemMenuTree(items []*systemMenuNode) []*systemMenuNode {
	byID := make(map[int64]*systemMenuNode, len(items))
	roots := make([]*systemMenuNode, 0, len(items))
	for _, item := range items {
		item.Children = nil
		byID[item.ID] = item
	}
	for _, item := range items {
		if parent, ok := byID[item.ParentID]; ok && item.ParentID != item.ID {
			parent.Children = append(parent.Children, item)
			continue
		}
		roots = append(roots, item)
	}
	return roots
}

func buildSystemDeptTree(items []*systemDeptNode) []*systemDeptNode {
	byID := make(map[int64]*systemDeptNode, len(items))
	roots := make([]*systemDeptNode, 0, len(items))
	for _, item := range items {
		item.Children = nil
		byID[item.ID] = item
	}
	for _, item := range items {
		if parent, ok := byID[item.ParentID]; ok && item.ParentID != item.ID {
			parent.Children = append(parent.Children, item)
			continue
		}
		roots = append(roots, item)
	}
	return roots
}

func (h *Handler) systemDeptNames(ctx context.Context, actor *adminpb.Actor) (map[int64]string, error) {
	resp, err := h.clients.Admin.ListSystemDepts(ctx, &adminpb.ListSystemDeptsRequest{
		Actor: actor,
	})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(resp.GetItems()))
	for _, dept := range resp.GetItems() {
		if dept.GetId() > 0 {
			names[dept.GetId()] = dept.GetName()
		}
	}
	return names, nil
}

func systemTablePayload(items any, total int64, page int32, pageSize int32) gin.H {
	return gin.H{
		"items":       items,
		"list":        items,
		"total":       total,
		"currentPage": page,
		"pageSize":    pageSize,
	}
}

func systemPage(c *gin.Context) (int32, int32) {
	page := queryInt32(c, "page", queryInt32(c, "currentPage", 1))
	pageSize := queryInt32(c, "page_size", queryInt32(c, "pageSize", 10))
	return page, pageSize
}

func firstQuery(c *gin.Context, names ...string) string {
	for _, name := range names {
		value := strings.TrimSpace(c.Query(name))
		if value != "" {
			return value
		}
	}
	return ""
}

func statusToInt(status string) int32 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(status), 10, 32)
	if err != nil {
		return 0
	}
	return int32(parsed)
}

func systemMenuType(value string, menuType int32) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	switch menuType {
	case 3:
		return "F"
	case 1:
		return "I"
	case 2:
		return "L"
	default:
		return "C"
	}
}

func templateMenuType(value string) int32 {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "F", "B":
		return 3
	case "I":
		return 1
	case "L":
		return 2
	default:
		return 0
	}
}

func defaultHTTPString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
