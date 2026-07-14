const DEFAULT_ENTITLEMENT_LIMIT = 50;
const FOCUSED_ENTITLEMENT_LIMIT = 100;

export async function loadEntitlementsForFocus(fetchEntitlements, params = {}, token, focusedEntitlementId, options = {}) {
  const focusedId = normalizeEntitlementId(focusedEntitlementId);
  if (!focusedId) {
    const data = await fetchEntitlements(params, token);
    const items = listItemsOf(data);
    return { items, total: listTotalOf(data, items) };
  }

  const pageLimit = normalizePositiveInt(options.focusLimit, FOCUSED_ENTITLEMENT_LIMIT);
  const baseParams = {
    ...params,
    limit: Math.max(pageLimit, normalizePositiveInt(params.limit, DEFAULT_ENTITLEMENT_LIMIT)),
    offset: normalizeOffset(params.offset)
  };
  const items = [];
  let total = 0;
  let offset = baseParams.offset;

  for (;;) {
    const data = await fetchEntitlements({ ...baseParams, offset }, token);
    const pageItems = listItemsOf(data);
    total = Math.max(total, listTotalOf(data, pageItems));
    items.push(...pageItems);

    if (pageItems.some((item) => entitlementMatchesFocus(item, focusedId))) {
      break;
    }
    if (pageItems.length === 0 || offset + pageItems.length >= total) {
      break;
    }
    offset += pageItems.length;
  }

  return { items: sortFocusedEntitlements(items, focusedId), total };
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

function normalizeOffset(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? Math.floor(number) : 0;
}

function normalizePositiveInt(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? Math.floor(number) : fallback;
}

function listItemsOf(data) {
  if (Array.isArray(data?.items)) return data.items;
  if (Array.isArray(data?.list)) return data.list;
  return [];
}

function listTotalOf(data, fallbackItems) {
  const number = Number(data?.total ?? data?.count);
  return Number.isFinite(number) ? number : fallbackItems.length;
}
