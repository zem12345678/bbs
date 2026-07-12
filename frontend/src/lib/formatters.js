export function toNumber(value, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

export function compactNumber(value, fallback = 0) {
  const number = toNumber(value, fallback);
  if (Math.abs(number) < 1000) {
    return String(number);
  }
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 1, notation: "compact" }).format(number);
}

export function toId(value) {
  if (value === undefined || value === null || value === "") {
    return "";
  }
  return String(value);
}

export function sameId(left, right) {
  const leftId = toId(left);
  const rightId = toId(right);
  return Boolean(leftId && rightId && leftId === rightId);
}

export function timeAgo(seconds) {
  const timestamp = toNumber(seconds);
  if (!timestamp) {
    return "刚刚";
  }
  const diff = Math.max(1, Math.floor((Date.now() - timestamp * 1000) / 1000));
  if (diff < 60) return "刚刚";
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
  if (diff < 2592000) return `${Math.floor(diff / 86400)}天前`;
  return new Date(timestamp * 1000).toLocaleDateString("zh-CN");
}

export function timeAgoMillis(value) {
  const timestamp = toNumber(value);
  if (!timestamp) {
    return "刚刚";
  }
  return timeAgo(timestamp > 100000000000 ? Math.floor(timestamp / 1000) : timestamp);
}

export function creditReasonLabel(reason) {
  const labels = {
    welcome: "注册奖励",
    follow_user: "关注用户",
    article_published: "发布文章",
    comment_created: "发表评论",
    like_given: "点赞文章",
    favorite_given: "收藏文章",
    article_commented: "收到评论",
    article_liked: "收到点赞",
    article_favorited: "收到收藏",
    qa_answer_accepted: "回答被采纳"
  };
  return labels[reason] || reason || "积分变更";
}

export function creditEntryMeta(entry) {
  const delta = toNumber(entry?.delta);
  const prefix = delta > 0 ? "+" : "";
  return `${prefix}${delta} · ${timeAgo(entry?.created_at || entry?.createdAt)}`;
}
