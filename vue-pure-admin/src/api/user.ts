import { http } from "@/utils/http";
import { getToken } from "@/utils/auth";

type ApiEnvelope<T> = {
  http_code?: number;
  code: number;
  message: string;
  data: T;
};

type AdminUser = {
  id: number;
  username: string;
  email: string;
  nickname: string;
  phone?: string;
  avatar?: string;
  avatar_url: string;
  avatarUrl?: string;
  bio?: string;
  description?: string;
  status: number;
  locked_flag: number;
};

type AdminAuthData = {
  success: boolean;
  message: string;
  access_token: string;
  expires_at: number;
  user: AdminUser;
  roles: Array<string>;
  permissions: Array<string>;
};

type AdminProfileData = {
  user: AdminUser;
  roles: Array<string>;
  permissions: Array<string>;
};

type AdminProfileUpdateData = {
  success?: boolean;
  message?: string;
  user: AdminUser;
};

export type UserResult = {
  code: number;
  message: string;
  data: {
    /** 头像 */
    avatar: string;
    /** 用户名 */
    username: string;
    /** 昵称 */
    nickname: string;
    /** 当前登录用户的角色 */
    roles: Array<string>;
    /** 按钮级别权限 */
    permissions: Array<string>;
    /** `token` */
    accessToken: string;
    /** 用于调用刷新`accessToken`的接口时所需的`token` */
    refreshToken: string;
    /** `accessToken`的过期时间（格式'xxxx/xx/xx xx:xx:xx'） */
    expires: Date;
  };
};

export type RefreshTokenResult = {
  code: number;
  message: string;
  data: {
    /** `token` */
    accessToken: string;
    /** 用于调用刷新`accessToken`的接口时所需的`token` */
    refreshToken: string;
    /** `accessToken`的过期时间（格式'xxxx/xx/xx xx:xx:xx'） */
    expires: Date;
  };
};

export type UserInfo = {
  /** 头像 */
  avatar: string;
  /** 用户名 */
  username: string;
  /** 昵称 */
  nickname: string;
  /** 邮箱 */
  email: string;
  /** 联系电话 */
  phone: string;
  /** 简介 */
  description: string;
};

export type UserInfoResult = {
  code: number;
  message: string;
  data: UserInfo;
};

type ResultTable = {
  code: number;
  message: string;
  data?: {
    /** 列表数据 */
    list: Array<any>;
    /** 总条目数 */
    total?: number;
    /** 每页显示条目个数 */
    pageSize?: number;
    /** 当前页数 */
    currentPage?: number;
  };
};

type ListPayload<T> = {
  items?: T[];
  list?: T[];
  total?: number;
  page?: number;
  size?: number;
  currentPage?: number;
  pageSize?: number;
};

/** 登录 */
export const getLogin = async (data?: {
  username?: string;
  password?: string;
}) => {
  const response = await http.request<ApiEnvelope<AdminAuthData>>(
    "post",
    "/api/v1/admin/auth/login",
    {
      data: {
        account: data?.username,
        password: data?.password
      }
    }
  );
  return toUserResult(response);
};

/** 刷新`token` */
export const refreshTokenApi = async (_data?: object) => {
  const token = getToken();
  return {
    code: token?.accessToken ? 0 : 401,
    message: token?.accessToken ? "success" : "missing token",
    data: {
      accessToken: token?.accessToken ?? "",
      refreshToken: token?.refreshToken ?? "",
      expires: new Date(token?.expires ?? 0)
    }
  } satisfies RefreshTokenResult;
};

/** 账户设置-个人信息 */
export const getMine = async () => {
  const response = await http.request<ApiEnvelope<AdminProfileData>>(
    "get",
    "/api/v1/admin/auth/profile"
  );
  return {
    code: response.code,
    message: response.message,
    data: toUserInfo(response.data.user)
  } satisfies UserInfoResult;
};

/** 账户设置-更新个人信息 */
export const updateMine = async (data: Partial<UserInfo>) => {
  const response = await http.request<ApiEnvelope<AdminProfileUpdateData>>(
    "put",
    "/api/v1/admin/auth/profile",
    {
      data: {
        nickname: data.nickname,
        email: data.email,
        phone: data.phone,
        bio: data.description,
        description: data.description,
        avatar_url: data.avatar
      }
    }
  );
  return {
    code: response.code,
    message: response.message,
    data: toUserInfo(response.data.user, [], data.description ?? "")
  } satisfies UserInfoResult;
};

/** 账户设置-修改当前管理员密码 */
export const changeMinePassword = async (data: {
  oldPassword: string;
  newPassword: string;
}) => {
  const response = await http.request<ApiEnvelope<AdminProfileData>>(
    "put",
    "/api/v1/admin/auth/password",
    {
      data: {
        old_password: data.oldPassword,
        new_password: data.newPassword
      }
    }
  );
  return {
    code: response.code,
    message: response.message,
    data: toUserInfo(response.data.user)
  } satisfies UserInfoResult;
};

/** 账户设置-上传头像 */
export const uploadMineAvatar = async (data: FormData) => {
  const response = await http.request<
    ApiEnvelope<{ url?: string; avatar_url?: string; path?: string }>
  >(
    "post",
    "/api/v1/admin/uploads/avatar",
    {
      data,
      headers: {
        "Content-Type": "multipart/form-data"
      }
    }
  );
  return {
    code: response.code,
    message: response.message,
    data: {
      url: response.data.url ?? response.data.avatar_url ?? response.data.path ?? ""
    }
  };
};

/** 账户设置-个人安全日志 */
export const getMineLogs = async (data: Record<string, any> = {}) => {
  const currentPage = data.currentPage ?? data.page ?? 1;
  const pageSize = data.pageSize ?? data.page_size ?? 10;
  const username = getToken()?.username ?? "";
  const response = await http.request<ApiEnvelope<ListPayload<any>>>(
    "get",
    "/api/v1/admin/login-logs",
    {
      params: {
        ...data,
        query: data.query ?? data.username ?? username,
        page: currentPage,
        page_size: pageSize
      }
    }
  );
  const result = response.data ?? {};
  const items = result.items ?? result.list ?? [];
  return {
    code: response.code,
    message: response.message,
    data: {
      list: items.map(normalizeMineLog),
      total: Number(result.total ?? items.length),
      pageSize: Number(result.pageSize ?? result.size ?? pageSize),
      currentPage: Number(result.currentPage ?? result.page ?? currentPage)
    }
  } satisfies ResultTable;
};

function toUserResult(response: ApiEnvelope<AdminAuthData>): UserResult {
  const auth = response.data;
  const user = auth.user;
  const accessToken = auth.access_token;
  return {
    code: response.code,
    message: response.message,
    data: {
      avatar: user?.avatar_url ?? "",
      username: user?.username ?? "",
      nickname: user?.nickname || user?.username || "",
      roles: auth.roles ?? [],
      permissions: auth.permissions ?? [],
      accessToken,
      refreshToken: accessToken,
      expires: new Date((auth.expires_at || 0) * 1000)
    }
  };
}

function normalizeMineLog(log: Record<string, any>) {
  const time =
    log.operatingTime ?? log.loginTime ?? log.login_time ?? log.created_at ?? 0;
  return {
    ...log,
    summary: log.summary ?? log.behavior ?? log.message ?? log.msg ?? "登录日志",
    ip: log.ip ?? log.ipaddr ?? "",
    address: log.address ?? log.location ?? log.loginLocation ?? "",
    system: log.system ?? log.os ?? "",
    browser: log.browser ?? "",
    status: Number(log.status ?? 0),
    operatingTime: time,
    loginTime: time
  };
}

function toUserInfo(
  user: AdminUser,
  _permissions: string[] = [],
  description = ""
): UserInfo {
  const bio = user?.bio ?? user?.description ?? description;
  return {
    avatar: user?.avatar_url ?? user?.avatarUrl ?? user?.avatar ?? "",
    username: user?.username ?? "",
    nickname: user?.nickname || user?.username || "",
    email: user?.email ?? "",
    phone: user?.phone ?? "",
    description: bio
  };
}
