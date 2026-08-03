import { toId } from "./formatters.js";

export function collectionEntityKey(entityType, entityId) {
  const type = String(entityType || "").trim().toLowerCase();
  const id = toId(entityId);
  return (type === "article" || type === "topic") && /^[1-9]\d*$/.test(id) ? `${type}:${id}` : "";
}

export function collectionItemEntity(item) {
  const entity = item?.entity || {};
  const entityType = entity.entity_type ?? entity.entityType ?? "";
  const entityId = entity.entity_id ?? entity.entityId ?? "";
  return {
    entityType: String(entityType || "").trim().toLowerCase(),
    entityId: toId(entityId)
  };
}

export function collectionItemKey(item) {
  const entity = collectionItemEntity(item);
  return collectionEntityKey(entity.entityType, entity.entityId);
}

export function collectionMembership(items = []) {
  return new Set(items.map(collectionItemKey).filter(Boolean));
}

export function collectionPostKey(post) {
  return collectionEntityKey(post?.kind, post?.id);
}
