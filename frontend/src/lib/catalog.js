import { toId, toNumber } from "./formatters.js";

export function normalizeTagsResponse(data) {
  return (data?.items || [])
    .map((item) => ({
      name: item?.name || "",
      count: toNumber(item?.count)
    }))
    .filter((item) => item.name);
}

export function normalizeHashtagsResponse(data) {
  return (data?.items || [])
    .map((item) => ({
      name: String(item?.tag ?? item?.name ?? "").replace(/^#/, "").trim(),
      count: toNumber(item?.count ?? item?.mentionedUsersCount ?? item?.mentioned_users_count)
    }))
    .filter((item) => item.name);
}

export function normalizeCategoriesResponse(data) {
  return (data?.items || [])
    .map((item) => {
      const hasTopicCount = item?.topic_count !== undefined || item?.topicCount !== undefined;
      return {
        id: toId(item?.id),
        slug: item?.slug || "",
        name: item?.name || "",
        description: item?.description || "",
        topicCount: hasTopicCount ? toNumber(item?.topic_count ?? item?.topicCount) : 0,
        topicCountKnown: hasTopicCount,
        sort: toNumber(item?.sort)
      };
    })
    .filter((item) => /^[1-9]\d*$/.test(item.id) && item.name)
    .sort((a, b) => a.sort - b.sort || a.id.localeCompare(b.id));
}
