import { toNumber } from "./formatters.js";

export function listItems(data) {
  if (Array.isArray(data?.items)) return data.items;
  if (Array.isArray(data?.list)) return data.list;
  return [];
}

export function listTotal(data, fallbackItems) {
  const items = fallbackItems ?? listItems(data);
  return toNumber(data?.total ?? data?.count, items.length);
}

export function unreadCount(data) {
  return toNumber(data?.count ?? data?.unread_count ?? data?.unreadCount);
}

export function notificationRead(item) {
  return Boolean(item?.read || item?.read_at || item?.readAt);
}

export function creditBalance(data) {
  const balance = data?.balance ?? null;
  return balance && typeof balance === "object" ? balance : null;
}
