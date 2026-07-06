import { http } from "@/utils/http";
import type { EntityId } from "@/utils/entityId";

type ApiEnvelope<T> = {
  code: number;
  message: string;
  data: T;
};

export type EntityRef = {
  entity_type?: string;
  entityType?: string;
  entity_id?: EntityId;
  entityId?: EntityId;
};

export type AdminReport = {
  id: EntityId;
  entity?: EntityRef;
  reporter_id?: EntityId;
  reporterId?: EntityId;
  reason: string;
  description: string;
  status: number;
  handled_by?: EntityId;
  handledBy?: EntityId;
  handled_at?: number;
  handledAt?: number;
  audit_note?: string;
  auditNote?: string;
  target_action?: string;
  targetAction?: string;
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
  id: EntityId;
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
  id: EntityId;
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
  id: EntityId;
  slug: string;
  title: string;
  summary: string;
  body: string;
  cover_url?: string;
  coverUrl?: string;
  tags: string[];
  author_id?: EntityId;
  authorId?: EntityId;
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
  id: EntityId;
  slug: string;
  type: string;
  title: string;
  body: string;
  tags: string[];
  author_id?: EntityId;
  authorId?: EntityId;
  category_id?: EntityId;
  categoryId?: EntityId;
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

export type AdminCategory = {
  id: EntityId;
  slug: string;
  name: string;
  description?: string;
  sort: number;
  status: number;
  topic_count?: number;
  topicCount?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminCategoryList = {
  items: AdminCategory[];
  total: number;
};

export type AdminCategoryPayload = {
  slug: string;
  name: string;
  description?: string;
  status: number;
  sort: number;
};

export type AdminComment = {
  id: EntityId;
  entity_type?: string;
  entityType?: string;
  entity_id?: EntityId;
  entityId?: EntityId;
  root_id?: EntityId;
  rootId?: EntityId;
  parent_id?: EntityId;
  parentId?: EntityId;
  author_id?: EntityId;
  authorId?: EntityId;
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

export type AdminForbiddenWord = {
  id: EntityId;
  word: string;
  scene: string;
  action: string;
  replacement?: string;
  description?: string;
  status: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminForbiddenWordList = {
  items: AdminForbiddenWord[];
  total: number;
};

export type AdminForbiddenWordPayload = {
  word: string;
  scene: string;
  action: string;
  replacement?: string;
  description?: string;
  status: number;
};

export type AdminSetting = {
  id: EntityId;
  key: string;
  value: string;
  group: string;
  value_type?: string;
  valueType?: string;
  description?: string;
  status: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminSettingList = {
  items: AdminSetting[];
  total: number;
  settings?: Record<string, string>;
};

export type AdminSettingPayload = {
  key: string;
  value: string;
  group: string;
  value_type: string;
  description?: string;
  status: number;
};

export type AdminLink = {
  id: EntityId;
  key: string;
  title: string;
  url?: string;
  URL?: string;
  description?: string;
  status: number;
  sort: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminLinkList = {
  items: AdminLink[];
  total: number;
};

export type AdminLinkPayload = {
  key: string;
  title: string;
  url: string;
  description?: string;
  status: number;
  sort: number;
};

export type AdminTask = {
  id: EntityId;
  key: string;
  title: string;
  description?: string;
  reward_points?: number;
  rewardPoints?: number;
  status: number;
  sort: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminTaskList = {
  items: AdminTask[];
  total: number;
};

export type AdminTaskPayload = {
  key: string;
  title: string;
  description?: string;
  reward_points: number;
  status: number;
  sort: number;
};

export type AdminBadge = {
  id: EntityId;
  key: string;
  name: string;
  description?: string;
  icon_url?: string;
  iconUrl?: string;
  rule_type?: string;
  ruleType?: string;
  rule_value?: number;
  ruleValue?: number;
  status: number;
  sort: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminBadgeList = {
  items: AdminBadge[];
  total: number;
};

export type AdminBadgePayload = {
  key: string;
  name: string;
  description?: string;
  icon_url?: string;
  rule_type: string;
  rule_value: number;
  status: number;
  sort: number;
};

export type AdminLevel = {
  id: EntityId;
  key: string;
  name: string;
  description?: string;
  min_score?: number;
  minScore?: number;
  max_score?: number;
  maxScore?: number;
  status: number;
  sort: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminLevelList = {
  items: AdminLevel[];
  total: number;
};

export type AdminLevelPayload = {
  key: string;
  name: string;
  description?: string;
  min_score: number;
  max_score: number;
  status: number;
  sort: number;
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
  id: EntityId;
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
  entity_type?: string;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminReportList>>(
    "get",
    "/api/v1/admin/reports",
    { params }
  );
};

export const auditAdminReport = (
  id: EntityId,
  status: number,
  auditNote = "",
  targetAction = ""
) => {
  return http.request<ApiEnvelope<{ report: AdminReport }>>(
    "post",
    `/api/v1/admin/reports/${id}/audit`,
    { data: { status, audit_note: auditNote, target_action: targetAction } }
  );
};

export const muteAdminUser = (id: EntityId) => {
  return http.request<ApiEnvelope<{ user: AdminUser }>>(
    "post",
    `/api/v1/admin/users/${id}/mute`
  );
};

export const unmuteAdminUser = (id: EntityId) => {
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
  author_id?: EntityId;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminArticleList>>(
    "get",
    "/api/v1/admin/articles",
    { params }
  );
};

export const hideAdminArticle = (id: EntityId) => {
  return http.request<ApiEnvelope<{ article: AdminArticle }>>(
    "post",
    `/api/v1/admin/articles/${id}/hide`
  );
};

export const publishAdminArticle = (id: EntityId) => {
  return http.request<ApiEnvelope<{ article: AdminArticle }>>(
    "post",
    `/api/v1/admin/articles/${id}/publish`
  );
};

export const archiveAdminArticle = (id: EntityId) => {
  return http.request<ApiEnvelope<{ article: AdminArticle }>>(
    "post",
    `/api/v1/admin/articles/${id}/archive`
  );
};

export const listAdminTopics = (params: {
  status?: number;
  type?: string;
  tag?: string;
  author_id?: EntityId;
  category_id?: EntityId;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminTopicList>>(
    "get",
    "/api/v1/admin/topics",
    { params }
  );
};

export const hideAdminTopic = (id: EntityId) => {
  return http.request<ApiEnvelope<{ topic: AdminTopic }>>(
    "post",
    `/api/v1/admin/topics/${id}/hide`
  );
};

export const publishAdminTopic = (id: EntityId) => {
  return http.request<ApiEnvelope<{ topic: AdminTopic }>>(
    "post",
    `/api/v1/admin/topics/${id}/publish`
  );
};

export const archiveAdminTopic = (id: EntityId) => {
  return http.request<ApiEnvelope<{ topic: AdminTopic }>>(
    "post",
    `/api/v1/admin/topics/${id}/archive`
  );
};

export const listAdminCategories = (params: {
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminCategoryList>>(
    "get",
    "/api/v1/admin/categories",
    { params }
  );
};

export const createAdminCategory = (data: AdminCategoryPayload) => {
  return http.request<ApiEnvelope<{ category: AdminCategory }>>(
    "post",
    "/api/v1/admin/categories",
    { data }
  );
};

export const updateAdminCategory = (id: EntityId, data: AdminCategoryPayload) => {
  return http.request<ApiEnvelope<{ category: AdminCategory }>>(
    "put",
    `/api/v1/admin/categories/${id}`,
    { data }
  );
};

export const deleteAdminCategory = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "delete",
    `/api/v1/admin/categories/${id}`
  );
};

export const listAdminComments = (params: {
  entity_type?: string;
  entity_id?: EntityId;
  author_id?: EntityId;
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

export const hideAdminComment = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "post",
    `/api/v1/admin/comments/${id}/hide`
  );
};

export const restoreAdminComment = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "post",
    `/api/v1/admin/comments/${id}/restore`
  );
};

export const listAdminForbiddenWords = (params: {
  status?: number;
  query?: string;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminForbiddenWordList>>(
    "get",
    "/api/v1/admin/forbidden-words",
    { params }
  );
};

export const createAdminForbiddenWord = (data: AdminForbiddenWordPayload) => {
  return http.request<ApiEnvelope<{ word: AdminForbiddenWord }>>(
    "post",
    "/api/v1/admin/forbidden-words",
    { data }
  );
};

export const updateAdminForbiddenWord = (
  id: EntityId,
  data: AdminForbiddenWordPayload
) => {
  return http.request<ApiEnvelope<{ word: AdminForbiddenWord }>>(
    "put",
    `/api/v1/admin/forbidden-words/${id}`,
    { data }
  );
};

export const deleteAdminForbiddenWord = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "delete",
    `/api/v1/admin/forbidden-words/${id}`
  );
};

export const listAdminSettings = (params: {
  group?: string;
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminSettingList>>(
    "get",
    "/api/v1/admin/settings",
    { params }
  );
};

export const updateAdminSetting = (key: string, data: AdminSettingPayload) => {
  return http.request<ApiEnvelope<{ setting: AdminSetting }>>(
    "put",
    `/api/v1/admin/settings/${encodeURIComponent(key)}`,
    { data }
  );
};

export const listAdminLinks = (params: {
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminLinkList>>(
    "get",
    "/api/v1/admin/links",
    { params }
  );
};

export const createAdminLink = (data: AdminLinkPayload) => {
  return http.request<ApiEnvelope<{ link: AdminLink }>>(
    "post",
    "/api/v1/admin/links",
    { data }
  );
};

export const updateAdminLink = (id: EntityId, data: AdminLinkPayload) => {
  return http.request<ApiEnvelope<{ link: AdminLink }>>(
    "put",
    `/api/v1/admin/links/${id}`,
    { data }
  );
};

export const deleteAdminLink = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "delete",
    `/api/v1/admin/links/${id}`
  );
};

export const listAdminTasks = (params: {
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminTaskList>>(
    "get",
    "/api/v1/admin/tasks",
    { params }
  );
};

export const createAdminTask = (data: AdminTaskPayload) => {
  return http.request<ApiEnvelope<{ task: AdminTask }>>(
    "post",
    "/api/v1/admin/tasks",
    { data }
  );
};

export const updateAdminTask = (id: EntityId, data: AdminTaskPayload) => {
  return http.request<ApiEnvelope<{ task: AdminTask }>>(
    "put",
    `/api/v1/admin/tasks/${id}`,
    { data }
  );
};

export const deleteAdminTask = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "delete",
    `/api/v1/admin/tasks/${id}`
  );
};

export const listAdminBadges = (params: {
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminBadgeList>>(
    "get",
    "/api/v1/admin/badges",
    { params }
  );
};

export const createAdminBadge = (data: AdminBadgePayload) => {
  return http.request<ApiEnvelope<{ badge: AdminBadge }>>(
    "post",
    "/api/v1/admin/badges",
    { data }
  );
};

export const updateAdminBadge = (id: EntityId, data: AdminBadgePayload) => {
  return http.request<ApiEnvelope<{ badge: AdminBadge }>>(
    "put",
    `/api/v1/admin/badges/${id}`,
    { data }
  );
};

export const deleteAdminBadge = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "delete",
    `/api/v1/admin/badges/${id}`
  );
};

export const listAdminLevels = (params: {
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminLevelList>>(
    "get",
    "/api/v1/admin/levels",
    { params }
  );
};

export const createAdminLevel = (data: AdminLevelPayload) => {
  return http.request<ApiEnvelope<{ level: AdminLevel }>>(
    "post",
    "/api/v1/admin/levels",
    { data }
  );
};

export const updateAdminLevel = (id: EntityId, data: AdminLevelPayload) => {
  return http.request<ApiEnvelope<{ level: AdminLevel }>>(
    "put",
    `/api/v1/admin/levels/${id}`,
    { data }
  );
};

export const deleteAdminLevel = (id: EntityId) => {
  return http.request<ApiEnvelope<{ success: boolean; message: string }>>(
    "delete",
    `/api/v1/admin/levels/${id}`
  );
};
