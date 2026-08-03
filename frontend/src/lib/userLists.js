import { sameId, toId, toNumber } from "./formatters.js";

export const USER_LIST_NAME_MAX_LENGTH = 100;

export function normalizeUserList(item) {
  const source = item?.user_list || item?.userList || item || {};
  return {
    id: toId(source.id),
    ownerId: toId(source.owner_id ?? source.ownerId),
    name: String(source.name || "").trim(),
    isPublic: Boolean(source.is_public ?? source.isPublic),
    memberCount: toNumber(source.member_count ?? source.memberCount),
    favoriteCount: toNumber(source.favorite_count ?? source.favoriteCount),
    isFavorited: Boolean(source.is_favorited ?? source.isFavorited),
    createdAt: toNumber(source.created_at ?? source.createdAt),
    updatedAt: toNumber(source.updated_at ?? source.updatedAt)
  };
}

export function normalizeUserLists(items) {
  return (Array.isArray(items) ? items : [])
    .map(normalizeUserList)
    .filter((item) => item.id && item.name);
}

export function validateUserListName(value) {
  const name = String(value || "").trim();
  const length = Array.from(name).length;
  if (length < 1 || length > USER_LIST_NAME_MAX_LENGTH) {
    return { name, error: "列表名称需为 1 至 100 个字符" };
  }
  return { name, error: "" };
}

export function userListOwnedBy(item, userId) {
  return sameId(item?.ownerId ?? item?.owner_id, userId);
}

export function appendUniqueUserLists(current, incoming) {
  const result = [...current];
  const seen = new Set(current.map((item) => toId(item?.id)).filter(Boolean));
  incoming.forEach((item) => {
    const id = toId(item?.id);
    if (!id || seen.has(id)) return;
    seen.add(id);
    result.push(item);
  });
  return result;
}
