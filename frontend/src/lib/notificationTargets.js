import { toId } from "./formatters.js";

const MALL_ENTITY_TYPES = new Set(["mall_order", "mall_product"]);
const MALL_NOTIFICATION_PREFIX = "mall_";

const MALL_NOTIFICATION_GROUP_META = {
  order: { value: "order", label: "订单", description: "支付、发货与完成提醒" },
  refund: { value: "refund", label: "售后", description: "退款审核与处理结果" },
  review: { value: "review", label: "评价", description: "商品评价审核结果" },
  entitlement: { value: "entitlement", label: "权益", description: "数字权益发放与撤销" },
  other: { value: "other", label: "商城", description: "其他商城通知" }
};

export const MALL_NOTIFICATION_GROUPS = [
  MALL_NOTIFICATION_GROUP_META.order,
  MALL_NOTIFICATION_GROUP_META.refund,
  MALL_NOTIFICATION_GROUP_META.review,
  MALL_NOTIFICATION_GROUP_META.entitlement
];

export const NOTIFICATION_FILTERS = [
  { value: "all", label: "全部" },
  { value: "mall", label: "商城" }
];

export function notificationTarget(item) {
  const entityType = item?.entity_type || item?.entityType || "";
  const entityId = toId(item?.entity_id ?? item?.entityId);
  const actorId = toId(item?.actor_id ?? item?.actorId);
  const sourceId = toId(item?.source_id ?? item?.sourceId);
  const type = item?.type || "";
  const commentHash = (type === "comment" || type === "reply") && sourceId ? `#comment-${sourceId}` : "";

  if (entityType === "topic" && entityId) {
    return `/topic/${entityId}${commentHash}`;
  }
  if (entityType === "article" && entityId) {
    return `/article/${entityId}${commentHash}`;
  }
  if (entityType === "user") {
    const userId = actorId || entityId;
    return userId ? `/user/${userId}` : "";
  }
  if (entityType === "mall_order") {
    if (mallNotificationGroup(item) === "refund") {
      const params = new URLSearchParams();
      if (sourceId) params.set("refund_id", sourceId);
      if (entityId) params.set("order_id", entityId);
      const query = params.toString();
      return query ? `/dashboard/refunds?${query}` : "/dashboard/refunds";
    }
    if (mallNotificationGroup(item) === "entitlement") {
      return sourceId ? `/dashboard/entitlements?entitlement_id=${encodeURIComponent(sourceId)}` : "/dashboard/entitlements";
    }
    return entityId ? `/dashboard/orders?order_id=${encodeURIComponent(entityId)}` : "/dashboard/orders";
  }
  if (entityType === "mall_product") {
    if (mallNotificationGroup(item) === "review") {
      const params = new URLSearchParams();
      if (sourceId) params.set("review_id", sourceId);
      if (entityId) params.set("product_id", entityId);
      const query = params.toString();
      return query ? `/dashboard/reviews?${query}` : "/dashboard/reviews";
    }
    return entityId ? `/shop?product_id=${encodeURIComponent(entityId)}` : "/shop";
  }
  if ((item?.type || "") === "follow" && actorId) {
    return `/user/${actorId}`;
  }
  return "";
}

export function notificationTargetLabel(item) {
  const entityType = item?.entity_type || item?.entityType || "";
  const type = item?.type || "";
  if (type === "reply") return "查看回复";
  if (type === "comment") return "查看评论";
  if (entityType === "topic") return "查看话题";
  if (entityType === "article") return "查看文章";
  if (entityType === "user" || (item?.type || "") === "follow") return "查看用户";
  if (entityType === "mall_order") {
    if (mallNotificationGroup(item) === "refund") return "查看售后";
    if (mallNotificationGroup(item) === "entitlement") return "查看权益";
    return "查看订单";
  }
  if (entityType === "mall_product") return mallNotificationGroup(item) === "review" ? "查看评价" : "查看商品";
  return "查看";
}

export function isMallNotification(item) {
  const entityType = item?.entity_type || item?.entityType || "";
  const type = item?.type || "";
  return MALL_ENTITY_TYPES.has(entityType) || type.startsWith(MALL_NOTIFICATION_PREFIX);
}

export function mallNotificationGroup(item) {
  const entityType = item?.entity_type || item?.entityType || "";
  const type = item?.type || "";
  if (type.startsWith("mall_refund_")) return "refund";
  if (type.startsWith("mall_review_") || type.startsWith("mall_product_review_")) return "review";
  if (type.startsWith("mall_digital_entitlement_")) return "entitlement";
  if (type.startsWith("mall_order_")) return "order";
  if (entityType === "mall_product") return "review";
  if (entityType === "mall_order") return "order";
  return "other";
}

export function notificationGroupLabel(item) {
  if (isMallNotification(item)) {
    return MALL_NOTIFICATION_GROUP_META[mallNotificationGroup(item)]?.label || MALL_NOTIFICATION_GROUP_META.other.label;
  }
  const type = item?.type || "";
  if (type === "system") return "系统";
  if (type === "reply") return "回复";
  if (type === "comment") return "评论";
  if (type === "like") return "点赞";
  if (type === "favorite") return "收藏";
  if (type === "follow") return "关注";
  return "互动";
}

export function notificationToneClass(item) {
  if (isMallNotification(item)) {
    return `type-mall-${mallNotificationGroup(item)}`;
  }
  return `type-${item?.type || "default"}`;
}

export function filterNotifications(items = [], filter = "all") {
  if (filter === "mall") {
    return items.filter(isMallNotification);
  }
  return items;
}

export function summarizeNotifications(items = []) {
  const summary = {
    total: items.length,
    unread: 0,
    mall: {
      total: 0,
      unread: 0,
      groups: {
        order: emptyNotificationCount(),
        refund: emptyNotificationCount(),
        review: emptyNotificationCount(),
        entitlement: emptyNotificationCount(),
        other: emptyNotificationCount()
      }
    }
  };

  items.forEach((item) => {
    const unread = !notificationIsRead(item);
    if (unread) {
      summary.unread += 1;
    }
    if (!isMallNotification(item)) {
      return;
    }
    const group = mallNotificationGroup(item);
    const groupSummary = summary.mall.groups[group] || summary.mall.groups.other;
    summary.mall.total += 1;
    groupSummary.total += 1;
    if (unread) {
      summary.mall.unread += 1;
      groupSummary.unread += 1;
    }
  });

  return summary;
}

function emptyNotificationCount() {
  return { total: 0, unread: 0 };
}

function notificationIsRead(item) {
  return Boolean(item?.read || item?.read_at || item?.readAt);
}
