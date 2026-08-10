import { safeExternalURL } from "./externalLinks.js";

const VERTICAL_PLACE = "vertical";
const MAX_WEEKDAY_MASK = 0b1111111;

export function normalizeAd(value) {
  const id = String(value?.id ?? "").trim();
  const url = safeExternalURL(value?.url);
  const imageUrl = safeExternalURL(value?.imageUrl);
  const place = String(value?.place ?? "").trim().toLowerCase();
  const ratio = Number(value?.ratio ?? 0);
  const dayOfWeek = Number(value?.dayOfWeek ?? 0);

  if (!id || !url || !imageUrl || !Number.isFinite(ratio) || ratio < 0) return null;
  if (!Number.isInteger(dayOfWeek) || dayOfWeek < 0 || dayOfWeek > MAX_WEEKDAY_MASK) return null;

  return { id, url, imageUrl, place, ratio, dayOfWeek };
}

export function stableAdRoll(now = new Date(), pathname = "") {
  const instant = now instanceof Date ? now : new Date(now);
  if (!Number.isFinite(instant.getTime())) return 0;

  const key = `${instant.toISOString().slice(0, 10)}:${String(pathname || "/")}`;
  let hash = 2166136261;
  for (let index = 0; index < key.length; index += 1) {
    hash ^= key.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0) / 4294967296;
}

export function selectSidebarAd(values, { now = new Date(), pathname = "", random } = {}) {
  const instant = now instanceof Date ? now : new Date(now);
  if (!Number.isFinite(instant.getTime())) return null;

  const weekdayBit = 1 << instant.getUTCDay();
  const eligible = (Array.isArray(values) ? values : [])
    .map(normalizeAd)
    .filter((ad) => ad && ad.place === VERTICAL_PLACE && (ad.dayOfWeek === 0 || (ad.dayOfWeek & weekdayBit) !== 0))
    .sort((left, right) => left.id.localeCompare(right.id));
  if (eligible.length === 0) return null;

  const weighted = eligible.filter((ad) => ad.ratio > 0);
  if (weighted.length === 0) return eligible[0];

  const total = weighted.reduce((sum, ad) => sum + ad.ratio, 0);
  const suppliedRoll = typeof random === "function" ? Number(random()) : Number(random);
  const fallbackRoll = stableAdRoll(instant, pathname);
  const roll = Number.isFinite(suppliedRoll) ? Math.min(Math.max(suppliedRoll, 0), 1 - Number.EPSILON) : fallbackRoll;
  let threshold = roll * total;

  for (const ad of weighted) {
    threshold -= ad.ratio;
    if (threshold < 0) return ad;
  }
  return weighted[weighted.length - 1];
}
