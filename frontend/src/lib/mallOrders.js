import { toNumber } from "./formatters.js";
import { mallGrantTypeOf } from "./mallProducts.js";

const REFUNDABLE_ORDER_STATUSES = new Set([3, 5, 6]);
const CANCELABLE_ORDER_STATUSES = new Set([1]);
const PAYMENT_SETTLED_ORDER_STATUSES = new Set(["3", "5", "6", "PAID", "SHIPPED", "COMPLETED"]);

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

export function mallOrderCanApplyRefund(order = {}) {
  return REFUNDABLE_ORDER_STATUSES.has(toNumber(order?.status)) && !mallOrderContainsMembershipGrant(order);
}

export function mallOrderCanCancel(order = {}) {
  return CANCELABLE_ORDER_STATUSES.has(toNumber(order?.status));
}

export function mallOrderPaymentSettled(order = {}) {
  return PAYMENT_SETTLED_ORDER_STATUSES.has(String(order?.status ?? "").trim().toUpperCase());
}

export function mallOrderContainsMembershipGrant(order = {}) {
  const items = Array.isArray(order?.items) ? order.items : [];
  const entitlements = Array.isArray(order?.digital_entitlements)
    ? order.digital_entitlements
    : Array.isArray(order?.digitalEntitlements)
      ? order.digitalEntitlements
      : [];
  return [...items, ...entitlements].some((item) => mallGrantTypeOf(item) === "membership");
}

function orderItemProductId(item) {
  return String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "").trim();
}
