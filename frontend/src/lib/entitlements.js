import { loadListForFocus } from "./focusedLists.js";

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
  return Boolean(focusedId) && normalizeEntitlementId(entitlement?.id) === focusedId;
}

function normalizeEntitlementId(value) {
  return String(value ?? "").trim();
}
