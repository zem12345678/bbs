const DEFAULT_DISPLAY = "normal";
const DEFAULT_ICON = "info";

export function normalizeAnnouncementsResponse(data) {
  const items = Array.isArray(data?.items) ? data.items : Array.isArray(data) ? data : [];
  return items.map(normalizeAnnouncement).filter(Boolean);
}

export function normalizeAnnouncement(item) {
  if (!item || typeof item !== "object") return null;
  const id = String(item.id ?? item.announcement_id ?? "").trim();
  const title = String(item.title ?? "").trim();
  const text = String(item.text ?? item.content ?? "").trim();
  if (!id || !title || !text) return null;
  return {
    ...item,
    id,
    title,
    text,
    imageUrl: String(item.image_url ?? item.imageUrl ?? "").trim(),
    icon: normalizeAnnouncementIcon(item.icon),
    display: normalizeAnnouncementDisplay(item.display),
    active: item.active !== false,
    startsAt: toTimestamp(item.starts_at ?? item.startsAt),
    endsAt: toTimestamp(item.ends_at ?? item.endsAt),
    updatedAt: toTimestamp(item.updated_at ?? item.updatedAt),
  };
}

export function visibleAnnouncements(items, now = Date.now()) {
  return (Array.isArray(items) ? items : []).filter((item) => {
    if (!item?.active) return false;
    if (item.startsAt > 0 && now < item.startsAt) return false;
    if (item.endsAt > 0 && now > item.endsAt) return false;
    return true;
  });
}

export function announcementDismissalKey(item) {
  if (!item?.id) return "";
  const version = item.updatedAt || `${item.title || ""}\u0000${item.text || ""}`;
  return `${item.id}:${version}`;
}

function normalizeAnnouncementIcon(value) {
  const icon = String(value || "").trim().toLowerCase();
  return ["info", "warning", "error", "success"].includes(icon) ? icon : DEFAULT_ICON;
}

function normalizeAnnouncementDisplay(value) {
  const display = String(value || "").trim().toLowerCase();
  return ["normal", "banner", "dialog"].includes(display) ? display : DEFAULT_DISPLAY;
}

function toTimestamp(value) {
  const timestamp = Number(value);
  return Number.isFinite(timestamp) && timestamp > 0 ? timestamp : 0;
}
