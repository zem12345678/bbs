import { loadListForFocus } from "./focusedLists.js";
import { toNumber } from "./formatters.js";

export const digitalEntitlementLookupLimit = 20;
export const digitalEntitlementStatusActive = "ACTIVE";
const entitlementDashboardStatuses = new Set(["ACTIVE", "EXPIRED", "REVOKED"]);
const entitlementDashboardGrantTypes = new Set(["badge", "theme", "membership", "digital"]);

export async function loadEntitlementsForFocus(fetchEntitlements, params = {}, token, focusedEntitlementId, options = {}) {
  return loadListForFocus(fetchEntitlements, params, token, focusedEntitlementId, entitlementMatchesFocus, sortFocusedEntitlements, options);
}

export function sortFocusedEntitlements(items = [], focusedEntitlementId) {
  const focusedId = normalizeEntitlementId(focusedEntitlementId);
  if (!focusedId) return items;
  return [...items].sort((left, right) => {
    const leftFocused = entitlementMatchesFocus(left, focusedId);
    const rightFocused = entitlementMatchesFocus(right, focusedId);
    return Number(rightFocused) - Number(leftFocused);
  });
}

export function entitlementMatchesFocus(entitlement, focusedEntitlementId) {
  const focusedId = normalizeEntitlementId(focusedEntitlementId);
  return Boolean(focusedId) && entitlementId(entitlement) === focusedId;
}

export function entitlementId(entitlement) {
  return normalizeEntitlementId(entitlement?.id ?? entitlement?.entitlement_id ?? entitlement?.entitlementId);
}

export function normalizeEntitlementStatusFilter(value, options = {}) {
  if (value !== null && value !== undefined) {
    const normalized = String(value).trim().toUpperCase();
    if (!normalized) return "";
    if (entitlementDashboardStatuses.has(normalized)) return normalized;
  }
  return normalizeEntitlementId(options.focusedEntitlementId) ? "" : digitalEntitlementStatusActive;
}

export function normalizeEntitlementGrantTypeFilter(value) {
  const normalized = String(value ?? "").trim().toLowerCase();
  return entitlementDashboardGrantTypes.has(normalized) ? normalized : "";
}

export function entitlementDashboardTarget(entitlement, options = {}) {
  const params = new URLSearchParams();
  const focusedId = entitlementId(entitlement);
  if (focusedId) params.set("entitlement_id", focusedId);
  const hasStatus = Object.prototype.hasOwnProperty.call(options, "status");
  const status = hasStatus
    ? normalizeEntitlementStatusFilter(options.status, {
        focusedEntitlementId: focusedId
      })
    : "";
  const grantType = normalizeEntitlementGrantTypeFilter(options.grantType);
  if (status) params.set("status", status);
  if (grantType) params.set("grant_type", grantType);
  const query = params.toString();
  return query ? `/dashboard/entitlements?${query}` : "/dashboard/entitlements";
}

export function entitlementUsageTarget(entitlement, now = Date.now()) {
  if (isActiveThemeEntitlement(entitlement, "theme-pro", now)) {
    return { label: "启用主题", path: "/dashboard/profile" };
  }
  if (isActiveMembershipEntitlement(entitlement, now)) {
    return { label: "设置背景", path: "/dashboard/profile" };
  }
  return null;
}

export function digitalEntitlementStatus(entitlement) {
  return String(entitlement?.status || entitlement?.Status || "").trim().toUpperCase();
}

export function digitalEntitlementGrantType(entitlement) {
  return String(entitlement?.grant_type || entitlement?.grantType || "").trim().toLowerCase();
}

export function digitalEntitlementGrantKey(entitlement) {
  return String(entitlement?.grant_key || entitlement?.grantKey || "").trim().toLowerCase();
}

export function digitalEntitlementRevoked(entitlement) {
  return digitalEntitlementStatus(entitlement) === "REVOKED" || Boolean(entitlement?.revoked_at || entitlement?.revokedAt);
}

export function digitalEntitlementExpiresAt(entitlement) {
  return toNumber(entitlement?.expires_at ?? entitlement?.expiresAt);
}

export function digitalEntitlementExpired(entitlement, now = Date.now()) {
  const expiresAt = digitalEntitlementExpiresAt(entitlement);
  return expiresAt > 0 && expiresAt <= now;
}

export function isActiveDigitalEntitlement(entitlement, options = {}) {
  const {
    grantType = "",
    grantKey = "",
    now = Date.now(),
    requireGrantKey = false,
    requireFutureExpiry = false
  } = options;
  if (digitalEntitlementStatus(entitlement) !== digitalEntitlementStatusActive) {
    return false;
  }
  if (digitalEntitlementRevoked(entitlement)) {
    return false;
  }
  const normalizedGrantType = String(grantType || "").trim().toLowerCase();
  if (normalizedGrantType && digitalEntitlementGrantType(entitlement) !== normalizedGrantType) {
    return false;
  }
  const actualGrantKey = digitalEntitlementGrantKey(entitlement);
  const normalizedGrantKey = String(grantKey || "").trim().toLowerCase();
  if (requireGrantKey && !actualGrantKey) {
    return false;
  }
  if (normalizedGrantKey && actualGrantKey !== normalizedGrantKey) {
    return false;
  }
  const expiresAt = digitalEntitlementExpiresAt(entitlement);
  if (requireFutureExpiry) {
    return expiresAt > now;
  }
  return expiresAt <= 0 || expiresAt > now;
}

export function isActiveMembershipEntitlement(entitlement, now = Date.now()) {
  return isActiveDigitalEntitlement(entitlement, {
    grantType: "membership",
    now,
    requireGrantKey: true,
    requireFutureExpiry: true
  });
}

export function isActiveThemeEntitlement(entitlement, theme = "theme-pro", now = Date.now()) {
  return isActiveDigitalEntitlement(entitlement, {
    grantType: "theme",
    grantKey: theme,
    now,
    requireGrantKey: true
  });
}

function normalizeEntitlementId(value) {
  return String(value ?? "").trim();
}
