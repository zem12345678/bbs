import { http } from "@/utils/http";
import {
  toPureResult,
  toPureTableResult,
  unwrapGatewayData,
  type GatewayEnvelope
} from "./client";

type Query = Record<string, any>;

type ListResult<T> = {
  items?: T[];
  list?: T[];
  flat_items?: T[];
  flatList?: T[];
  total?: number;
  page?: number;
  size?: number;
  currentPage?: number;
  pageSize?: number;
};

type UserDTO = {
  id: number;
  username: string;
  nickname?: string;
  phone?: string;
  avatar?: string;
  avatar_url?: string;
  email?: string;
  sex?: number;
  role_id?: number;
  roleId?: number;
  role_ids?: number[];
  roleIds?: number[];
  dept?: {
    id?: number;
    name?: string;
  };
  dept_id?: number;
  deptId?: number;
  post_id?: number;
  postId?: number;
  status: number;
  locked_flag?: number;
  created_at?: number;
  createdAt?: number;
  createTime?: number;
  updated_at?: number;
};

type RoleDTO = {
  id: number;
  key?: string;
  code?: string;
  name: string;
  remark?: string;
  sort?: number;
  admin?: boolean;
  status: string | number;
  menu_ids?: number[];
  menuIds?: number[];
  permissions?: string[];
  created_at?: number;
  createTime?: number;
};

type MenuDTO = {
  id: number;
  parent_id?: number;
  parentId?: number;
  name: string;
  title?: string;
  path?: string;
  component?: string;
  icon?: string;
  type?: string;
  menuType?: number;
  permission?: string;
  auths?: string;
  sort?: number;
  rank?: number;
  visible?: string;
  is_hide?: string;
  status?: string;
  remark?: string;
  showLink?: boolean;
  children?: MenuDTO[];
};

type DepartmentDTO = {
  id: number;
  parent_id?: number;
  parentId?: number;
  path?: string;
  name: string;
  sort?: number;
  leader?: string;
  principal?: string;
  phone?: string;
  email?: string;
  status: number;
  remark?: string;
  created_at?: number;
  createdAt?: number;
  createTime?: number;
  updated_at?: number;
  updatedAt?: number;
  updateTime?: number;
  children?: DepartmentDTO[];
};

export const getUserList = (data: Query = {}) => {
  return list<UserDTO>("/api/v1/admin/system/users", data, normalizeUser).then(
    result => toPureTableResult(result)
  );
};

export const getAllRoleList = () => {
  return list<RoleDTO>(
    "/api/v1/admin/system/roles",
    { page_size: 200 },
    normalizeRole
  ).then(result =>
    toPureResult(
      result.items.map(role => ({
        id: role.id,
        name: role.name,
        code: role.code,
        label: role.name,
        value: role.id
      }))
    )
  );
};

export const getRoleIds = async (data: Query = {}) => {
  const userId = data.userId ?? data.user_id ?? data.id;
  if (!userId) return toPureResult([]);
  const user = await getOne<Record<string, any>>(
    `/api/v1/admin/system/users/${userId}`
  );
  return toPureResult(user.role_ids ?? user.roleIds ?? []);
};

export const getRoleList = (data: Query = {}) => {
  return list<RoleDTO>("/api/v1/admin/system/roles", data, normalizeRole).then(
    result => toPureTableResult(result)
  );
};

export const getMenuList = (data: Query = {}) => {
  return list<MenuDTO>("/api/v1/admin/system/menus", data, normalizeMenu).then(
    result => toPureResult(result.items)
  );
};

export const getDeptList = (data: Query = {}) => {
  return list<DepartmentDTO>(
    "/api/v1/admin/system/depts",
    data,
    normalizeDepartment
  ).then(result => toPureResult(result.items));
};

export const getRoleMenu = () => getMenuList({ page_size: 500 });

export const getRoleMenuIds = (data: Query = {}) => {
  const roleId = data.id ?? data.role_id;
  if (!roleId) return Promise.resolve(toPureResult([]));
  return http
    .request<GatewayEnvelope<number[]>>(
      "get",
      `/api/v1/admin/system/roles/${roleId}/menu-ids`
    )
    .then(response => toPureResult(unwrapGatewayData(response) ?? []));
};

export const updateRoleMenuIds = (roleId: number, menuIds: number[]) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "put",
      `/api/v1/admin/system/roles/${roleId}/menus`,
      {
        data: {
          menu_ids: menuIds
        }
      }
    )
    .then(response => toPureResult(unwrapGatewayData(response)));
};

export const createUser = (data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "post",
      "/api/v1/admin/system/users",
      { data: toUserPayload(data) }
    )
    .then(response =>
      toPureResult(normalizeUser(unwrapEntity<UserDTO>(response, "user")))
    );
};

export const updateUser = (id: number, data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "put",
      `/api/v1/admin/system/users/${id}`,
      { data: toUserPayload(data) }
    )
    .then(response =>
      toPureResult(normalizeUser(unwrapEntity<UserDTO>(response, "user")))
    );
};

export const deleteUser = (id: number) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "delete",
      `/api/v1/admin/system/users/${id}`
    )
    .then(response => toPureResult(unwrapGatewayData(response)));
};

export const resetUserPassword = (id: number, password: string) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "put",
      `/api/v1/admin/system/users/${id}/password`,
      { data: { password } }
    )
    .then(response => toPureResult(unwrapGatewayData(response)));
};

export const updateUserRole = (id: number, roleId: number) => {
  return updateUserRoles(id, roleId ? [roleId] : []);
};

export const updateUserRoles = (id: number, roleIds: number[]) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "put",
      `/api/v1/admin/system/users/${id}/roles`,
      { data: { role_ids: roleIds } }
    )
    .then(response => toPureResult(unwrapGatewayData(response)));
};

export const createRole = (data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "post",
      "/api/v1/admin/system/roles",
      { data: toRolePayload(data) }
    )
    .then(response =>
      toPureResult(normalizeRole(unwrapEntity<RoleDTO>(response, "role")))
    );
};

export const updateRole = (id: number, data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "put",
      `/api/v1/admin/system/roles/${id}`,
      { data: toRolePayload(data) }
    )
    .then(response =>
      toPureResult(normalizeRole(unwrapEntity<RoleDTO>(response, "role")))
    );
};

export const deleteRole = (id: number) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "delete",
      `/api/v1/admin/system/roles/${id}`
    )
    .then(response => toPureResult(unwrapGatewayData(response)));
};

export const createMenu = (data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "post",
      "/api/v1/admin/system/menus",
      { data: toMenuPayload(data) }
    )
    .then(response =>
      toPureResult(normalizeMenu(unwrapEntity<MenuDTO>(response, "menu")))
    );
};

export const updateMenu = (id: number, data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "put",
      `/api/v1/admin/system/menus/${id}`,
      { data: toMenuPayload(data) }
    )
    .then(response =>
      toPureResult(normalizeMenu(unwrapEntity<MenuDTO>(response, "menu")))
    );
};

export const deleteMenu = (id: number) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "delete",
      `/api/v1/admin/system/menus/${id}`
    )
    .then(response => toPureResult(unwrapGatewayData(response)));
};

export const createDepartment = (data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "post",
      "/api/v1/admin/system/depts",
      { data: toDepartmentPayload(data) }
    )
    .then(response =>
      toPureResult(
        normalizeDepartment(unwrapEntity<DepartmentDTO>(response, "dept"))
      )
    );
};

export const updateDepartment = (id: number, data: Query) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "put",
      `/api/v1/admin/system/depts/${id}`,
      { data: toDepartmentPayload(data) }
    )
    .then(response =>
      toPureResult(
        normalizeDepartment(unwrapEntity<DepartmentDTO>(response, "dept"))
      )
    );
};

export const deleteDepartment = (id: number) => {
  return http
    .request<GatewayEnvelope<Record<string, any>>>(
      "delete",
      `/api/v1/admin/system/depts/${id}`
    )
    .then(response => toPureResult(unwrapGatewayData(response)));
};

export const getLoginLogsList = (data: Query = {}) => {
  return list<Record<string, any>>(
    "/api/v1/admin/login-logs",
    data,
    normalizeLoginLog
  ).then(result => toPureTableResult(result));
};

export const getOperationLogsList = (data: Query = {}) => {
  return list<Record<string, any>>(
    "/api/v1/admin/operation-logs",
    data,
    normalizeOperationLog
  ).then(result => toPureTableResult(result));
};

export const getEmailLogsList = (data: Query = {}) => {
  return list<Record<string, any>>(
    "/api/v1/admin/email-logs",
    data,
    normalizeEmailLog
  ).then(result => toPureTableResult(result));
};

async function list<T>(
  url: string,
  query: Query,
  normalize: (item: T) => Record<string, any>
): Promise<{
  items: Record<string, any>[];
  total: number;
  currentPage: number;
  pageSize: number;
}> {
  const response = await http.request<GatewayEnvelope<ListResult<T>>>(
    "get",
    url,
    { params: toListParams(query) }
  );
  const result = unwrapGatewayData(response);
  return {
    items: (result.items ?? result.list ?? []).map(normalize),
    total: Number(result.total ?? 0),
    currentPage: Number(result.currentPage ?? result.page ?? 1),
    pageSize: Number(result.pageSize ?? result.size ?? 20)
  };
}

async function getOne<T>(url: string): Promise<T> {
  const response = await http.request<GatewayEnvelope<T>>("get", url);
  return unwrapGatewayData(response);
}

function unwrapEntity<T>(
  response: GatewayEnvelope<Record<string, any>>,
  key: string
): T {
  const data = unwrapGatewayData(response);
  return (data?.[key] ?? data) as T;
}

function toListParams(query: Query) {
  const currentPage = query.currentPage ?? query.page ?? 1;
  const pageSize = query.pageSize ?? query.page_size ?? 20;
  const keyword =
    query.keyword ??
    query.query ??
    query.username ??
    query.name ??
    query.title ??
    query.phone ??
    query.code ??
    "";
  return {
    ...query,
    query: keyword,
    page: currentPage,
    page_size: pageSize,
    status:
      query.status === "" || query.status === null || query.status === undefined
        ? undefined
        : query.status
  };
}

function normalizeUser(user: UserDTO) {
  const deptId = user.dept_id ?? user.deptId ?? 0;
  const deptName = user.dept?.name ?? "";
  const createdAt = user.createTime ?? user.created_at ?? user.createdAt;
  return {
    ...user,
    avatar: user.avatar ?? user.avatar_url ?? "",
    nickname: user.nickname || user.username,
    status: Number(user.status ?? 1),
    role_id: user.role_id ?? user.roleId ?? 0,
    roleId: user.role_id ?? user.roleId ?? 0,
    role_ids: user.role_ids ?? user.roleIds ?? [],
    roleIds: user.role_ids ?? user.roleIds ?? [],
    dept_id: deptId,
    deptId,
    createTime: createdAt,
    dept: {
      id: deptId,
      name: deptName || "-"
    }
  };
}

function normalizeRole(role: RoleDTO) {
  const key = role.key ?? role.code ?? "";
  return {
    ...role,
    key,
    code: key,
    status: Number(role.status ?? 1),
    sort: Number(role.sort ?? 0),
    createTime: role.createTime ?? role.created_at,
    menu_ids: role.menu_ids ?? role.menuIds ?? [],
    menuIds: role.menu_ids ?? role.menuIds ?? []
  };
}

function normalizeMenu(menu: MenuDTO): Record<string, any> {
  const parentId = menu.parent_id ?? menu.parentId ?? 0;
  const menuType = menu.menuType ?? toFrontendMenuType(menu.type);
  const sort = menu.sort ?? menu.rank ?? 99;
  return {
    ...menu,
    parent_id: parentId,
    parentId,
    menuType,
    auths: menu.auths ?? menu.permission ?? "",
    rank: sort,
    sort,
    frameLoading: (menu as any).frameLoading ?? true,
    keepAlive: (menu as any).keepAlive ?? false,
    hiddenTag: (menu as any).hiddenTag ?? false,
    fixedTag: (menu as any).fixedTag ?? false,
    showLink: menu.showLink ?? menu.visible !== "1",
    showParent: (menu as any).showParent ?? false,
    children: (menu.children ?? []).map(normalizeMenu)
  };
}

function normalizeDepartment(dept: DepartmentDTO): Record<string, any> {
  const parentId = dept.parent_id ?? dept.parentId ?? 0;
  const createdAt = dept.createTime ?? dept.created_at ?? dept.createdAt ?? 0;
  const updatedAt = dept.updateTime ?? dept.updated_at ?? dept.updatedAt ?? 0;
  return {
    ...dept,
    parent_id: parentId,
    parentId,
    principal: dept.principal ?? dept.leader ?? "",
    status: Number(dept.status ?? 1),
    createTime: createdAt,
    created_at: createdAt,
    createdAt,
    updateTime: updatedAt,
    updated_at: updatedAt,
    updatedAt,
    children: (dept.children ?? []).map(normalizeDepartment)
  };
}

function normalizeLoginLog(log: Record<string, any>) {
  return {
    ...log,
    ip: log.ip ?? log.ipaddr ?? "",
    address: log.address ?? log.location ?? log.loginLocation ?? "",
    system: log.system ?? log.os ?? "",
    browser: log.browser ?? "",
    status: Number(log.status ?? 0),
    behavior: log.behavior ?? log.message ?? log.msg ?? "",
    loginTime: log.loginTime ?? log.login_time ?? 0
  };
}

function normalizeOperationLog(log: Record<string, any>) {
  return {
    ...log,
    username: log.username ?? log.operatorName ?? log.operName ?? "",
    module: log.module ?? log.title ?? "",
    summary: log.summary ?? log.businessType ?? "",
    ip: log.ip ?? log.operIp ?? "",
    address: log.address ?? log.location ?? log.operLocation ?? "",
    system: log.system ?? "",
    browser: log.browser ?? "",
    status: Number(log.status ?? 0),
    operatingTime: log.operatingTime ?? log.operation_time ?? log.operTime ?? 0,
    takesTime: Number(log.takesTime ?? 0)
  };
}

function normalizeEmailLog(log: Record<string, any>) {
  return {
    ...log,
    to: log.to ?? log.mail_to ?? "",
    subject: log.subject ?? "",
    templateKey: log.templateKey ?? log.template_key ?? "",
    provider: log.provider ?? "",
    status: Number(log.status ?? 0),
    error: log.error ?? "",
    createdAt: log.createdAt ?? log.created_at ?? 0,
    updatedAt: log.updatedAt ?? log.updated_at ?? 0
  };
}

function toUserPayload(data: Query) {
  return {
    username: data.username,
    nickname: data.nickname ?? data.username,
    password: data.password,
    phone: data.phone ? String(data.phone) : "",
    avatar_url: data.avatar ?? data.avatar_url ?? "",
    email: data.email ?? "",
    dept_id: Number(data.dept_id ?? data.deptId ?? data.parentId ?? 0),
    post_id: Number(data.post_id ?? data.postId ?? 0),
    role_ids: data.role_ids ?? data.roleIds ?? [],
    status: Number(data.status ?? 1)
  };
}

function toRolePayload(data: Query) {
  return {
    key: data.key ?? data.code,
    name: data.name,
    remark: data.remark ?? "",
    data_scope: data.data_scope ?? data.dataScope ?? "1",
    sort: Number(data.sort ?? 0),
    admin: Boolean(data.admin ?? false),
    status: String(data.status ?? 1)
  };
}

function toMenuPayload(data: Query) {
  const type = toBackendMenuType(data.menuType, data.type);
  const isButton = type === "F";
  const showLink = isButton ? false : data.showLink !== false;
  return {
    parent_id: Number(data.parent_id ?? data.parentId ?? 0),
    name: isButton ? data.name || data.auths || data.title || "" : data.name ?? "",
    title: data.title ?? "",
    path: isButton ? "" : data.path ?? "",
    component: isButton ? "" : data.component ?? "",
    icon: isButton ? "" : data.icon ?? "",
    type,
    permission: data.permission ?? data.auths ?? "",
    visible: showLink ? "0" : "1",
    is_hide: showLink ? "0" : "1",
    remark: data.remark ?? "",
    sort: Number(data.sort ?? data.rank ?? 99),
    status: String(data.status ?? "0")
  };
}

function toDepartmentPayload(data: Query) {
  return {
    parent_id: Number(data.parent_id ?? data.parentId ?? 0),
    name: data.name ?? "",
    sort: Number(data.sort ?? 0),
    leader: data.leader ?? data.principal ?? "",
    phone: data.phone ? String(data.phone) : "",
    email: data.email ?? "",
    status: Number(data.status ?? 1)
  };
}

function toFrontendMenuType(type: unknown) {
  switch (String(type ?? "").toUpperCase()) {
    case "F":
    case "B":
      return 3;
    case "I":
      return 1;
    case "L":
      return 2;
    default:
      return 0;
  }
}

function toBackendMenuType(menuType: unknown, fallback: unknown) {
  if (typeof fallback === "string" && fallback) return fallback;
  switch (Number(menuType)) {
    case 3:
      return "F";
    case 1:
      return "I";
    case 2:
      return "L";
    default:
      return "C";
  }
}
