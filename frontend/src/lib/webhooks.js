export const WEBHOOK_EVENT_OPTIONS = Object.freeze([
  { value: "mention", label: "提及" },
  { value: "unfollow", label: "取消关注" },
  { value: "follow", label: "关注用户" },
  { value: "followed", label: "被关注" },
  { value: "note", label: "内容发布" },
  { value: "reply", label: "回复" },
  { value: "renote", label: "转发" },
  { value: "reaction", label: "互动" },
  { value: "edited", label: "内容编辑" }
]);

const VALID_EVENTS = new Set(WEBHOOK_EVENT_OPTIONS.map((item) => item.value));

export function normalizeWebhookList(data) {
  const rawItems = Array.isArray(data) ? data : Array.isArray(data?.items) ? data.items : [];
  const items = rawItems.map(normalizeWebhook).filter((item) => item.id);
  return {
    items,
    total: Math.max(items.length, nonNegativeNumber(data?.total))
  };
}

export function normalizeWebhook(data) {
  const source = data?.webhook || data || {};
  const rawEvents = Array.isArray(source.on) ? source.on : Array.isArray(source.events) ? source.events : [];
  return {
    id: String(source.id || ""),
    name: String(source.name || "").trim(),
    url: String(source.url || "").trim(),
    events: [...new Set(rawEvents.map(normalizeEvent).filter(Boolean))],
    active: source.active !== false,
    latestSentAt: timestampSeconds(source.latest_sent_at ?? source.latestSentAt),
    latestStatus: nonNegativeNumber(source.latest_status ?? source.latestStatus),
    createdAt: timestampSeconds(source.created_at ?? source.createdAt),
    updatedAt: timestampSeconds(source.updated_at ?? source.updatedAt)
  };
}

export function webhookEventLabel(eventType) {
  const normalized = normalizeEvent(eventType);
  return WEBHOOK_EVENT_OPTIONS.find((item) => item.value === normalized)?.label || String(eventType || "未知事件");
}

export function webhookTime(value) {
  const seconds = timestampSeconds(value);
  if (!seconds) return "暂无";
  return new Date(seconds * 1000).toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

export function validWebhookURL(value) {
  try {
    const parsed = new URL(value);
    if (parsed.protocol === "https:") return true;
    const host = parsed.hostname.toLowerCase().replace(/\.$/, "");
    return parsed.protocol === "http:" && (host === "localhost" || host.endsWith(".localhost") || host === "127.0.0.1" || host === "::1" || host === "[::1]");
  } catch {
    return false;
  }
}

function normalizeEvent(value) {
  const normalized = String(value || "").trim().toLowerCase();
  return VALID_EVENTS.has(normalized) ? normalized : "";
}

function timestampSeconds(value) {
  const number = Number(value);
  if (Number.isFinite(number) && number > 0) return number >= 1_000_000_000_000 ? Math.floor(number / 1000) : number;
  const milliseconds = Date.parse(String(value || ""));
  return Number.isFinite(milliseconds) && milliseconds > 0 ? Math.floor(milliseconds / 1000) : 0;
}

function nonNegativeNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) && number >= 0 ? number : 0;
}
