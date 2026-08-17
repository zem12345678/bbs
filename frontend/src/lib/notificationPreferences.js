export const NOTIFICATION_PREFERENCES = [
  { type: "system", label: "系统通知", description: "平台维护和运营通知" },
  { type: "export_completed", label: "数据导出", description: "个人数据导出文件生成完成" },
  { type: "follow", label: "新增关注", description: "有人关注你的账号" },
  { type: "follow_request_received", label: "关注申请", description: "有人申请关注你的账号" },
  { type: "follow_request_accepted", label: "申请通过", description: "你的关注申请已被接受" },
  { type: "comment", label: "内容评论", description: "有人评论你的文章或话题" },
  { type: "reply", label: "评论回复", description: "有人回复你的评论" },
  { type: "like", label: "点赞", description: "有人点赞你的内容" },
  { type: "favorite", label: "收藏", description: "有人收藏你的内容" },
  { type: "qa_answer_accepted", label: "回答被采纳", description: "你的回答被采纳并获得积分" },
  { type: "mall_order_paid", label: "订单支付", description: "商城订单支付完成" },
  { type: "mall_order_shipped", label: "订单发货", description: "商城订单已发货" },
  { type: "mall_order_completed", label: "订单完成", description: "商城订单已完成" },
  { type: "mall_refund_approved", label: "售后通过", description: "商城退款申请已通过" },
  { type: "mall_refund_rejected", label: "售后拒绝", description: "商城退款申请未通过" },
  { type: "mall_digital_entitlement_revoked", label: "权益撤销", description: "数字权益被撤销" },
  { type: "mall_review_published", label: "评价展示", description: "商品评价已展示" },
  { type: "mall_review_hidden", label: "评价隐藏", description: "商品评价被隐藏" }
];

export function defaultNotificationPreferences() {
  return NOTIFICATION_PREFERENCES.map(({ type }) => ({ type, enabled: true }));
}

export function mergeNotificationPreferences(items) {
  const overrides = new Map();
  for (const item of Array.isArray(items) ? items : []) {
    const type = String(item?.type || item?.notification_type || "").trim();
    if (type) overrides.set(type, item?.enabled !== false);
  }

  const preferences = defaultNotificationPreferences().map((item) => ({
    ...item,
    enabled: overrides.has(item.type) ? overrides.get(item.type) : item.enabled
  }));
  const knownTypes = new Set(preferences.map((item) => item.type));
  for (const [type, enabled] of overrides) {
    if (!knownTypes.has(type)) preferences.push({ type, enabled });
  }
  return preferences;
}
