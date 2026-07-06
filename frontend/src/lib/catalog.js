import { toNumber } from "./formatters";

export function normalizeTagsResponse(data) {
  return (data?.items || [])
    .map((item) => ({
      name: item?.name || "",
      count: toNumber(item?.count)
    }))
    .filter((item) => item.name);
}

export function normalizeCategoriesResponse(data) {
  return (data?.items || [])
    .map((item) => {
      const hasTopicCount = item?.topic_count !== undefined || item?.topicCount !== undefined;
      return {
        id: toNumber(item?.id),
        slug: item?.slug || "",
        name: item?.name || "",
        description: item?.description || "",
        topicCount: hasTopicCount ? toNumber(item?.topic_count ?? item?.topicCount) : 0,
        topicCountKnown: hasTopicCount,
        sort: toNumber(item?.sort)
      };
    })
    .filter((item) => item.id > 0 && item.name)
    .sort((a, b) => a.sort - b.sort || a.id - b.id);
}
