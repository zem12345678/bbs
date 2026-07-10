import { toId } from "./formatters";

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
    return entityId ? `/dashboard/orders?order_id=${encodeURIComponent(entityId)}` : "/dashboard/orders";
  }
  if (entityType === "mall_product") {
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
  if (entityType === "mall_order") return "查看订单";
  if (entityType === "mall_product") return "查看商品";
  return "查看";
}
