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

export type AdminReportTarget = Record<string, any>;

export type AdminUser = {
  id: EntityId;
  username: string;
  email: string;
  nickname: string;
  avatar_url?: string;
  avatarUrl?: string;
  background_url?: string;
  backgroundUrl?: string;
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
  clear_value?: boolean;
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

export type AdminCreditBalance = {
  user_id?: EntityId;
  userId?: EntityId;
  total: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminCreditLedger = {
  id: EntityId;
  user_id?: EntityId;
  userId?: EntityId;
  delta: number;
  balance_after?: number;
  balanceAfter?: number;
  reason: string;
  description?: string;
  source_event_id?: string;
  sourceEventId?: string;
  source_type?: string;
  sourceType?: string;
  source_id?: EntityId;
  sourceId?: EntityId;
  created_at?: number;
  createdAt?: number;
};

export type AdminCreditLedgerList = {
  items: AdminCreditLedger[];
  total: number;
  balance?: AdminCreditBalance;
};

export type AdminCreditAdjustPayload = {
  delta: number;
  reason?: string;
  description?: string;
  source_event_id?: string;
};

export type AdminCreditAdjustResult = {
  balance?: AdminCreditBalance;
  ledger?: AdminCreditLedger;
  duplicate?: boolean;
};

export type AdminMallProduct = {
  id: EntityId;
  sku: string;
  title: string;
  description?: string;
  category?: string;
  cover_url?: string;
  coverUrl?: string;
  price_credits?: number;
  priceCredits?: number;
  stock: number;
  sales_count?: number;
  salesCount?: number;
  status: number;
  sort: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallProductList = {
  items: AdminMallProduct[];
  total: number;
};

export type AdminMallProductCategory = {
  id: EntityId;
  slug: string;
  name: string;
  description?: string;
  status: number | string;
  sort: number;
  product_count?: number;
  productCount?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallProductCategoryList = {
  items: AdminMallProductCategory[];
  total: number;
};

export type AdminMallProductCategoryPayload = {
  slug: string;
  name: string;
  description?: string;
  status: number;
  sort: number;
};

export type AdminMallProductReview = {
  id: EntityId;
  product_id?: EntityId;
  productId?: EntityId;
  product_sku?: string;
  productSku?: string;
  product_title?: string;
  productTitle?: string;
  order_id?: EntityId;
  orderId?: EntityId;
  user_id?: EntityId;
  userId?: EntityId;
  rating: number;
  content?: string;
  status: number | string;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallProductReviewList = {
  items: AdminMallProductReview[];
  total: number;
};

export type AdminMallProductReviewStatusPayload = {
  status: number;
};

export type AdminMallProductStockLog = {
  id: EntityId;
  product_id?: EntityId;
  productId?: EntityId;
  sku?: string;
  title?: string;
  delta: number;
  before_stock?: number;
  beforeStock?: number;
  after_stock?: number;
  afterStock?: number;
  reason?: string;
  reference_type?: string;
  referenceType?: string;
  reference_id?: EntityId;
  referenceId?: EntityId;
  operator_type?: string;
  operatorType?: string;
  operator_id?: string;
  operatorId?: string;
  note?: string;
  created_at?: number;
  createdAt?: number;
};

export type AdminMallProductStockLogList = {
  items: AdminMallProductStockLog[];
  total: number;
};

export type AdminMallProductPayload = {
  sku: string;
  title: string;
  description?: string;
  category?: string;
  cover_url?: string;
  price_credits: number;
  stock: number;
  status: number;
  sort: number;
};

export type AdminMallCoupon = {
  id: EntityId;
  code: string;
  name: string;
  description?: string;
  discount_credits?: number;
  discountCredits?: number;
  min_order_credits?: number;
  minOrderCredits?: number;
  total_quota?: number;
  totalQuota?: number;
  per_user_limit?: number;
  perUserLimit?: number;
  claimed_count?: number;
  claimedCount?: number;
  used_count?: number;
  usedCount?: number;
  status: number | string;
  starts_at?: number;
  startsAt?: number;
  ends_at?: number;
  endsAt?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallCouponList = {
  items: AdminMallCoupon[];
  total: number;
};

export type AdminMallCouponUsage = {
  id: EntityId;
  coupon_id?: EntityId;
  couponId?: EntityId;
  code: string;
  user_id?: EntityId;
  userId?: EntityId;
  order_id?: EntityId;
  orderId?: EntityId;
  status: number | string;
  discount_credits?: number;
  discountCredits?: number;
  created_at?: number;
  createdAt?: number;
  used_at?: number;
  usedAt?: number;
  released_at?: number;
  releasedAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallCouponUsageList = {
  items: AdminMallCouponUsage[];
  total: number;
};

export type AdminMallCouponPayload = {
  code: string;
  name: string;
  description?: string;
  discount_credits: number;
  min_order_credits: number;
  total_quota: number;
  per_user_limit: number;
  status: number;
  starts_at?: number;
  ends_at?: number;
};

export type AdminMallStatusCount = {
  status: string;
  count: number;
};

export type AdminMallOverview = {
  product_total?: number;
  productTotal?: number;
  active_product_total?: number;
  activeProductTotal?: number;
  low_stock_total?: number;
  lowStockTotal?: number;
  stock_total?: number;
  stockTotal?: number;
  sales_count_total?: number;
  salesCountTotal?: number;
  order_total?: number;
  orderTotal?: number;
  paid_order_total?: number;
  paidOrderTotal?: number;
  revenue_credits_total?: number;
  revenueCreditsTotal?: number;
  today_order_total?: number;
  todayOrderTotal?: number;
  today_revenue_credits?: number;
  todayRevenueCredits?: number;
  pending_shipment_total?: number;
  pendingShipmentTotal?: number;
  pending_refund_total?: number;
  pendingRefundTotal?: number;
  refunded_credits_total?: number;
  refundedCreditsTotal?: number;
  order_status_counts?: AdminMallStatusCount[];
  orderStatusCounts?: AdminMallStatusCount[];
  refund_status_counts?: AdminMallStatusCount[];
  refundStatusCounts?: AdminMallStatusCount[];
  low_stock_products?: AdminMallProduct[];
  lowStockProducts?: AdminMallProduct[];
  top_selling_products?: AdminMallProduct[];
  topSellingProducts?: AdminMallProduct[];
};

export type AdminMallOrderItem = {
  product_id?: EntityId;
  productId?: EntityId;
  sku: string;
  title: string;
  quantity: number;
  unit_price_credits?: number;
  unitPriceCredits?: number;
  subtotal_credits?: number;
  subtotalCredits?: number;
};

export type AdminMallOrder = {
  id: EntityId;
  order_no?: string;
  orderNo?: string;
  idempotency_key?: string;
  idempotencyKey?: string;
  user_id?: EntityId;
  userId?: EntityId;
  items: AdminMallOrderItem[];
  original_credits?: number;
  originalCredits?: number;
  discount_credits?: number;
  discountCredits?: number;
  total_credits?: number;
  totalCredits?: number;
  coupon_id?: EntityId;
  couponId?: EntityId;
  coupon_code?: string;
  couponCode?: string;
  status: number;
  receiver?: string;
  phone?: string;
  address?: string;
  payment_method?: string;
  paymentMethod?: string;
  shipping_carrier?: string;
  shippingCarrier?: string;
  tracking_no?: string;
  trackingNo?: string;
  paid_at?: number;
  paidAt?: number;
  shipped_at?: number;
  shippedAt?: number;
  completed_at?: number;
  completedAt?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallOrderList = {
  items: AdminMallOrder[];
  total: number;
};

export type AdminMallCloseExpiredPayload = {
  expire_after_seconds?: number;
  limit?: number;
};

export type AdminMallCloseExpiredResult = {
  items: AdminMallOrder[];
  total: number;
};

export type AdminMallRecoverPayingPayload = {
  stale_after_seconds?: number;
  limit?: number;
};

export type AdminMallRecoverPayingResult = {
  recovered?: number;
  failed?: number;
};

export type AdminMallOrderStatusPayload = {
  status: number;
  shipping_carrier?: string;
  tracking_no?: string;
  note?: string;
  operator_id?: string;
};

export type AdminMallOrderStatusLog = {
  id: EntityId;
  order_id?: EntityId;
  orderId?: EntityId;
  from_status?: number;
  fromStatus?: number;
  to_status?: number;
  toStatus?: number;
  reason?: string;
  operator_type?: string;
  operatorType?: string;
  operator_id?: string;
  operatorId?: string;
  note?: string;
  created_at?: number;
  createdAt?: number;
};

export type AdminMallPayment = {
  id: EntityId;
  order_id?: EntityId;
  orderId?: EntityId;
  user_id?: EntityId;
  userId?: EntityId;
  amount_credits?: number;
  amountCredits?: number;
  provider?: string;
  idempotency_key?: string;
  idempotencyKey?: string;
  status: number;
  provider_trade_no?: string;
  providerTradeNo?: string;
  failure_reason?: string;
  failureReason?: string;
  paid_at?: number;
  paidAt?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallRefund = {
  id: EntityId;
  order_id?: EntityId;
  orderId?: EntityId;
  order_no?: string;
  orderNo?: string;
  user_id?: EntityId;
  userId?: EntityId;
  amount_credits?: number;
  amountCredits?: number;
  status: number;
  reason?: string;
  user_note?: string;
  userNote?: string;
  admin_note?: string;
  adminNote?: string;
  restore_stock?: boolean;
  restoreStock?: boolean;
  operator_id?: string;
  operatorId?: string;
  requested_at?: number;
  requestedAt?: number;
  reviewed_at?: number;
  reviewedAt?: number;
  refunded_at?: number;
  refundedAt?: number;
  created_at?: number;
  createdAt?: number;
  updated_at?: number;
  updatedAt?: number;
};

export type AdminMallRefundList = {
  items: AdminMallRefund[];
  total: number;
};

export type AdminMallRefundReviewPayload = {
  approved: boolean;
  admin_note?: string;
  restore_stock?: boolean;
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

export const getAdminReportTopicTarget = (id: EntityId) => {
  return http.request<ApiEnvelope<{ topic: AdminReportTarget }>>(
    "get",
    `/api/v1/admin/topics/${encodeURIComponent(String(id))}`
  );
};

export const getAdminReportArticleTarget = (id: EntityId) => {
  return http.request<ApiEnvelope<{ article: AdminReportTarget }>>(
    "get",
    `/api/v1/admin/articles/${encodeURIComponent(String(id))}`
  );
};

export const getAdminReportCommentTarget = (id: EntityId) => {
  return http.request<ApiEnvelope<{ comment: AdminReportTarget }>>(
    "get",
    `/api/v1/admin/comments/${encodeURIComponent(String(id))}`
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

export const getAdminUserCreditBalance = (userId: EntityId) => {
  return http.request<ApiEnvelope<{ balance: AdminCreditBalance }>>(
    "get",
    `/api/v1/admin/credits/users/${userId}/balance`
  );
};

export const listAdminUserCreditLedger = (
  userId: EntityId,
  params: {
    limit: number;
    offset: number;
  }
) => {
  return http.request<ApiEnvelope<AdminCreditLedgerList>>(
    "get",
    `/api/v1/admin/credits/users/${userId}/ledger`,
    { params }
  );
};

export const adjustAdminUserCredits = (
  userId: EntityId,
  data: AdminCreditAdjustPayload
) => {
  return http.request<ApiEnvelope<AdminCreditAdjustResult>>(
    "post",
    `/api/v1/admin/credits/users/${userId}/adjust`,
    { data }
  );
};

export const listAdminMallProducts = (params: {
  keyword?: string;
  category?: string;
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminMallProductList>>(
    "get",
    "/api/v1/admin/mall/products",
    { params }
  );
};

export const listAdminMallProductCategories = (params: {
  keyword?: string;
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminMallProductCategoryList>>(
    "get",
    "/api/v1/admin/mall/categories",
    { params }
  );
};

export const createAdminMallProductCategory = (
  data: AdminMallProductCategoryPayload
) => {
  return http.request<ApiEnvelope<{ category: AdminMallProductCategory }>>(
    "post",
    "/api/v1/admin/mall/categories",
    { data }
  );
};

export const updateAdminMallProductCategory = (
  id: EntityId,
  data: AdminMallProductCategoryPayload
) => {
  return http.request<ApiEnvelope<{ category: AdminMallProductCategory }>>(
    "put",
    `/api/v1/admin/mall/categories/${id}`,
    { data }
  );
};

export const listAdminMallProductReviews = (params: {
  product_id?: EntityId;
  user_id?: EntityId;
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminMallProductReviewList>>(
    "get",
    "/api/v1/admin/mall/reviews",
    { params }
  );
};

export const updateAdminMallProductReviewStatus = (
  id: EntityId,
  data: AdminMallProductReviewStatusPayload
) => {
  return http.request<ApiEnvelope<{ review: AdminMallProductReview }>>(
    "put",
    `/api/v1/admin/mall/reviews/${id}/status`,
    { data }
  );
};

export const createAdminMallProduct = (data: AdminMallProductPayload) => {
  return http.request<ApiEnvelope<{ product: AdminMallProduct }>>(
    "post",
    "/api/v1/admin/mall/products",
    { data }
  );
};

export const updateAdminMallProduct = (
  id: EntityId,
  data: AdminMallProductPayload
) => {
  return http.request<ApiEnvelope<{ product: AdminMallProduct }>>(
    "put",
    `/api/v1/admin/mall/products/${id}`,
    { data }
  );
};

export const listAdminMallProductStockLogs = (
  id: EntityId,
  params: {
    reason?: string;
    limit: number;
    offset: number;
  }
) => {
  return http.request<ApiEnvelope<AdminMallProductStockLogList>>(
    "get",
    `/api/v1/admin/mall/products/${id}/stock-logs`,
    { params }
  );
};

export const listAdminMallCoupons = (params: {
  keyword?: string;
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminMallCouponList>>(
    "get",
    "/api/v1/admin/mall/coupons",
    { params }
  );
};

export const listAdminMallCouponUsages = (
  id: EntityId,
  params: {
    user_id?: EntityId;
    status?: number;
    limit: number;
    offset: number;
  }
) => {
  return http.request<ApiEnvelope<AdminMallCouponUsageList>>(
    "get",
    `/api/v1/admin/mall/coupons/${id}/usages`,
    { params }
  );
};

export const createAdminMallCoupon = (data: AdminMallCouponPayload) => {
  return http.request<ApiEnvelope<{ coupon: AdminMallCoupon }>>(
    "post",
    "/api/v1/admin/mall/coupons",
    { data }
  );
};

export const updateAdminMallCoupon = (
  id: EntityId,
  data: AdminMallCouponPayload
) => {
  return http.request<ApiEnvelope<{ coupon: AdminMallCoupon }>>(
    "put",
    `/api/v1/admin/mall/coupons/${id}`,
    { data }
  );
};

export const getAdminMallOverview = (params: {
  low_stock_threshold?: number;
} = {}) => {
  return http.request<ApiEnvelope<{ overview: AdminMallOverview }>>(
    "get",
    "/api/v1/admin/mall/overview",
    { params }
  );
};

export const listAdminMallOrders = (params: {
  user_id?: EntityId;
  keyword?: string;
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminMallOrderList>>(
    "get",
    "/api/v1/admin/mall/orders",
    { params }
  );
};

export const closeAdminMallExpiredOrders = (
  data: AdminMallCloseExpiredPayload
) => {
  return http.request<ApiEnvelope<AdminMallCloseExpiredResult>>(
    "post",
    "/api/v1/admin/mall/orders/expire",
    { data }
  );
};

export const recoverAdminMallStalePayingOrders = (
  data: AdminMallRecoverPayingPayload
) => {
  return http.request<ApiEnvelope<AdminMallRecoverPayingResult>>(
    "post",
    "/api/v1/admin/mall/orders/recover-paying",
    { data }
  );
};

export const updateAdminMallOrderStatus = (
  id: EntityId,
  data: AdminMallOrderStatusPayload
) => {
  return http.request<ApiEnvelope<{ order: AdminMallOrder }>>(
    "put",
    `/api/v1/admin/mall/orders/${id}/status`,
    { data }
  );
};

export const listAdminMallOrderLogs = (id: EntityId) => {
  return http.request<ApiEnvelope<{ items: AdminMallOrderStatusLog[] }>>(
    "get",
    `/api/v1/admin/mall/orders/${id}/logs`
  );
};

export const listAdminMallOrderPayments = (id: EntityId) => {
  return http.request<ApiEnvelope<{ items: AdminMallPayment[] }>>(
    "get",
    `/api/v1/admin/mall/orders/${id}/payments`
  );
};

export const listAdminMallRefunds = (params: {
  user_id?: EntityId;
  keyword?: string;
  status?: number;
  limit: number;
  offset: number;
}) => {
  return http.request<ApiEnvelope<AdminMallRefundList>>(
    "get",
    "/api/v1/admin/mall/refunds",
    { params }
  );
};

export const reviewAdminMallRefund = (
  id: EntityId,
  data: AdminMallRefundReviewPayload
) => {
  return http.request<ApiEnvelope<{ refund: AdminMallRefund }>>(
    "post",
    `/api/v1/admin/mall/refunds/${id}/review`,
    { data }
  );
};
