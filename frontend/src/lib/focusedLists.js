const DEFAULT_FOCUSED_LIMIT = 50;
const MAX_FOCUSED_LIMIT = 100;

export async function loadAllListPages(fetchList, params = {}, token, options = {}) {
  const pageLimit = normalizePageLimit(options.pageLimit ?? params.limit, DEFAULT_FOCUSED_LIMIT);
  const baseParams = {
    ...params,
    limit: pageLimit,
    offset: normalizeOffset(params.offset)
  };
  const items = [];
  let total = 0;
  let offset = baseParams.offset;

  for (;;) {
    const data = await fetchList({ ...baseParams, offset }, token);
    const pageItems = listItemsOf(data);
    items.push(...pageItems);
    total = Math.max(total, listTotalOf(data, pageItems), items.length);

    if (pageItems.length === 0 || offset + pageItems.length >= total) {
      break;
    }
    offset += pageItems.length;
  }

  return { items, total };
}

export async function loadListForFocus(fetchList, params = {}, token, focus, matchesFocus, sortFocusedItems, options = {}) {
  if (!hasFocus(focus)) {
    const data = await fetchList(params, token);
    const items = listItemsOf(data);
    return { items, total: listTotalOf(data, items) };
  }

  const pageLimit = normalizePositiveInt(options.focusLimit, MAX_FOCUSED_LIMIT);
  const baseParams = {
    ...params,
    limit: Math.max(pageLimit, normalizePositiveInt(params.limit, DEFAULT_FOCUSED_LIMIT)),
    offset: normalizeOffset(params.offset)
  };
  const items = [];
  let total = 0;
  let offset = baseParams.offset;

  for (;;) {
    const data = await fetchList({ ...baseParams, offset }, token);
    const pageItems = listItemsOf(data);
    total = Math.max(total, listTotalOf(data, pageItems));
    items.push(...pageItems);

    if (pageItems.some((item) => matchesFocus(item, focus))) {
      break;
    }
    if (pageItems.length === 0 || offset + pageItems.length >= total) {
      break;
    }
    offset += pageItems.length;
  }

  return { items: sortFocusedItems ? sortFocusedItems(items, focus) : items, total };
}

function hasFocus(value) {
  if (value === null || value === undefined) return false;
  if (Array.isArray(value)) return value.some(hasFocus);
  if (typeof value === "object") return Object.values(value).some(hasFocus);
  return String(value).trim() !== "";
}

function normalizeOffset(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? Math.floor(number) : 0;
}

function normalizePositiveInt(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? Math.floor(number) : fallback;
}

function normalizePageLimit(value, fallback) {
  return Math.min(MAX_FOCUSED_LIMIT, normalizePositiveInt(value, fallback));
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
