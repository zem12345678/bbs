import { toId } from "./formatters";

export function notificationTarget(item) {
  const entityType = item?.entity_type || item?.entityType || "";
  const entityId = toId(item?.entity_id ?? item?.entityId);
  const actorId = toId(item?.actor_id ?? item?.actorId);

  if (entityType === "topic" && entityId) {
    return `/topic/${entityId}`;
  }
  if (entityType === "article" && entityId) {
    return `/article/${entityId}`;
  }
  if (entityType === "user") {
    const userId = actorId || entityId;
    return userId ? `/user/${userId}` : "";
  }
  if ((item?.type || "") === "follow" && actorId) {
    return `/user/${actorId}`;
  }
  return "";
}

export function notificationTargetLabel(item) {
  const entityType = item?.entity_type || item?.entityType || "";
  if (entityType === "topic") return "查看话题";
  if (entityType === "article") return "查看文章";
  if (entityType === "user" || (item?.type || "") === "follow") return "查看用户";
  return "查看";
}
