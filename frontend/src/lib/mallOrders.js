export function mallOrderReviewableProductIds(order = {}) {
  const items = Array.isArray(order?.items) ? order.items : [];
  const seen = new Set();
  const ids = [];
  for (const item of items) {
    const id = orderItemProductId(item);
    if (!id || seen.has(id)) {
      continue;
    }
    seen.add(id);
    ids.push(id);
  }
  return ids;
}

function orderItemProductId(item) {
  return String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "").trim();
}
