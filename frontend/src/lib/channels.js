import { listItems, listTotal } from "./apiShapes.js";
import { sameId, toId, toNumber } from "./formatters.js";

const DEFAULT_CHANNEL_COLOR = "#1683f7";

export function channelColor(value) {
  const color = String(value || "").trim();
  return /^#[0-9a-f]{6}$/i.test(color) ? color : DEFAULT_CHANNEL_COLOR;
}

export function normalizeChannel(item) {
  if (!item || typeof item !== "object") return null;
  const id = toId(item.id);
  if (!id) return null;
  return {
    ...item,
    id,
    owner_id: toId(item.owner_id),
    category_id: toId(item.category_id),
    name: String(item.name || "").trim(),
    description: String(item.description || "").trim(),
    color: channelColor(item.color),
    followers_count: toNumber(item.followers_count),
    topics_count: toNumber(item.topics_count),
    is_archived: Boolean(item.is_archived),
    is_featured: Boolean(item.is_featured),
    is_following: Boolean(item.is_following),
    is_favorited: Boolean(item.is_favorited)
  };
}

export function channelItems(data) {
  return listItems(data).map(normalizeChannel).filter(Boolean);
}

export function channelList(data) {
  const items = channelItems(data);
  return { items, total: listTotal(data, items) };
}

export function channelFromResponse(data) {
  return normalizeChannel(data?.channel || data);
}

export function channelCategories(data) {
  return listItems(data).map((item) => ({
    ...item,
    id: toId(item?.category_id ?? item?.id),
    name: String(item?.name || "").trim(),
    channels_count: toNumber(item?.channels_count),
    followers_count: toNumber(item?.followers_count),
    topics_count: toNumber(item?.topics_count)
  })).filter((item) => item.id && item.name);
}

export function ownsChannel(channel, user) {
  return sameId(channel?.owner_id, user?.id);
}
