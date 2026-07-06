import { http } from "@/utils/http";

type ApiEnvelope<T> = {
  code: number;
  message: string;
  data: T;
};

export type EntityRef = {
  entity_type?: string;
  entityType?: string;
  entity_id?: number;
  entityId?: number;
};

export type AdminReport = {
  id: number;
  entity?: EntityRef;
  reporter_id?: number;
  reporterId?: number;
  reason: string;
  description: string;
  status: number;
  handled_by?: number;
  handledBy?: number;
  handled_at?: number;
  handledAt?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminReportList = {
  items: AdminReport[];
  total: number;
};

export type AdminUser = {
  id: number;
  username: string;
  email: string;
  nickname: string;
  avatar_url?: string;
  avatarUrl?: string;
  bio?: string;
  status: number;
  follower_count?: number;
  followerCount?: number;
  following_count?: number;
  followingCount?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
  last_login_at?: number;
  lastLoginAt?: number;
  locked_flag?: number;
  lockedFlag?: number;
  roles?: string[];
};

export type AdminRole = {
  id: number;
  name: string;
  key: string;
  status: string;
  sort: number;
  admin: boolean;
  remark: string;
  permissions: string[];
};

export type AdminUserList = {
  items: AdminUser[];
  total: number;
};

export type AdminRoleList = {
  items: AdminRole[];
  total: number;
};

export type AdminArticle = {
  id: number;
  slug: string;
  title: string;
  summary: string;
  body: string;
  cover_url?: string;
  coverUrl?: string;
  tags: string[];
  author_id?: number;
  authorId?: number;
  status: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
  published_at?: number;
  publishedAt?: number;
};

export type AdminArticleList = {
  items: AdminArticle[];
  total: number;
};

export type AdminTopic = {
  id: number;
  slug: string;
  type: string;
  title: string;
  body: string;
  tags: string[];
  author_id?: number;
  authorId?: number;
  status: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
  published_at?: number;
  publishedAt?: number;
};

export type AdminTopicList = {
  items: AdminTopic[];
  total: number;
};

export type AdminComment = {
  id: number;
  entity_type?: string;
  entityType?: string;
  entity_id?: number;
  entityId?: number;
  root_id?: number;
  rootId?: number;
  parent_id?: number;
  parentId?: number;
  author_id?: number;
  authorId?: number;
  content: string;
  status: number;
  reply_count?: number;
  replyCount?: number;
  like_count?: number;
  likeCount?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminCommentList = {
  items: AdminComment[];
  total: number;
};

export type AdminOverviewMetric = {
  key: string;
  name: string;
  value: number;
  percent: string;
  data: number[];
};

export type AdminOverviewChart = {
  labels: string[];
  previous: {
    contentData: number[];
    governanceData: number[];
  };
  current: {
    contentData: number[];
    governanceData: number[];
  };
};

export type AdminOverviewProgress = {
  label?: string;
  week?: string;
  percentage: number;
  duration: number;
  color: string;
};

export type AdminOverviewDaily = {
  id: number;
  date: string;
  newUsers: number;
  newArticles: number;
  newTopics: number;
  newComments: number;
  reports: number;
  pendingReports: number;
};

export type AdminOverviewActivity = {
  type: string;
  summary: string;
  detail: string;
  timestamp: number;
  date: string;
};

export type AdminOverview = {
  metrics: AdminOverviewMetric[];
  chart: AdminOverviewChart;
  progress: AdminOverviewProgress[];
  daily: AdminOverviewDaily[];
  latest: AdminOverviewActivity[];
};

export const getAdminOverview = () => {
  return http.request<ApiEnvelope<AdminOverview>>(
    "get",
    "/api/v1/admin/overview"
  );
};

export const listAdminReports = (params: {
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminReportList>>(
    "get",
    "/api/v1/admin/reports",
    { params }
  );
};

export const auditAdminReport = (id: number, status: number) => {
  return http.request<ApiEnvelope<{ report: AdminReport }>>(
    "post",
    `/api/v1/admin/reports/${id}/audit`,
    { data: { status } }
  );
};

export const muteAdminUser = (id: number) => {
  return http.request<ApiEnvelope<{ user: AdminUser }>>(
    "post",
    `/api/v1/admin/users/${id}/mute`
  );
};

export const unmuteAdminUser = (id: number) => {
  return http.request<ApiEnvelope<{ user: AdminUser }>>(
    "post",
    `/api/v1/admin/users/${id}/unmute`
  );
};

export const listGovernanceUsers = (params: {
  query?: string;
  status?: number;
  page: number;
  page_size: number;
}) => {
  return http.request<ApiEnvelope<AdminUserList>>(
    "get",
    "/api/v1/admin/users",
    { params }
  );
};

export const listAdminArticles = (params: {
  status?: number;
  tag?: string;
  author_id?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminArticleList>>(
    "get",
    "/api/v1/admin/articles",
    { params }
  );
};

export const hideAdminArticle = (id: number) => {
  return http.request<ApiEnvelope<{ article: AdminArticle }>>(
    "post",
    `/api/v1/admin/articles/${id}/hide`
  );
};

export const archiveAdminArticle = (id: number) => {
  return http.request<ApiEnvelope<{ article: AdminArticle }>>(
    "post",
    `/api/v1/admin/articles/${id}/archive`
  );
};

export const listAdminTopics = (params: {
  status?: number;
  type?: string;
  tag?: string;
  author_id?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminTopicList>>(
    "get",
    "/api/v1/admin/topics",
    { params }
  );
};

export const hideAdminTopic = (id: number) => {
  return http.request<ApiEnvelope<{ topic: AdminTopic }>>(
    "post",
    `/api/v1/admin/topics/${id}/hide`
  );
};

export const archiveAdminTopic = (id: number) => {
  return http.request<ApiEnvelope<{ topic: AdminTopic }>>(
    "post",
    `/api/v1/admin/topics/${id}/archive`
  );
};

export const listAdminComments = (params: {
  entity_type?: string;
  entity_id?: number;
  author_id?: number;
  status?: number;
  page: number;
  page_size: number;
}) => {
  return http.request<ApiEnvelope<AdminCommentList>>(
    "get",
    "/api/v1/admin/comments",
    { params }
  );
};

export const hideAdminComment = (id: number) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "post",
    `/api/v1/admin/comments/${id}/hide`
  );
};
