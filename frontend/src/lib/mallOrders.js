import { mallGrantTypeOf } from "./mallProducts.js";

const ORDER_STATUS_LABELS = {
  PENDING_PAYMENT: "待支付",
  PAYING: "支付中",
  PAID: "已支付",
  CANCELED: "已取消",
  SHIPPED: "已发货",
  COMPLETED: "已完成",
  CLOSED: "已关闭",
  REFUNDED: "已退款"
};
const ORDER_STATUS_ALIASES = new Map([
  ["1", "PENDING_PAYMENT"],
  ["PENDING_PAYMENT", "PENDING_PAYMENT"],
  ["ORDER_STATUS_PENDING_PAYMENT", "PENDING_PAYMENT"],
  ["2", "PAYING"],
  ["PAYING", "PAYING"],
  ["ORDER_STATUS_PAYING", "PAYING"],
  ["3", "PAID"],
  ["PAID", "PAID"],
  ["ORDER_STATUS_PAID", "PAID"],
  ["4", "CANCELED"],
  ["CANCELED", "CANCELED"],
  ["ORDER_STATUS_CANCELED", "CANCELED"],
  ["5", "SHIPPED"],
  ["SHIPPED", "SHIPPED"],
  ["ORDER_STATUS_SHIPPED", "SHIPPED"],
  ["6", "COMPLETED"],
  ["COMPLETED", "COMPLETED"],
  ["ORDER_STATUS_COMPLETED", "COMPLETED"],
  ["7", "CLOSED"],
  ["CLOSED", "CLOSED"],
  ["ORDER_STATUS_CLOSED", "CLOSED"],
  ["8", "REFUNDED"],
  ["REFUNDED", "REFUNDED"],
  ["ORDER_STATUS_REFUNDED", "REFUNDED"]
]);
const PAYABLE_ORDER_STATUSES = new Set(["PENDING_PAYMENT", "PAYING"]);
const REFUNDABLE_ORDER_STATUSES = new Set(["PAID", "SHIPPED", "COMPLETED"]);
const CANCELABLE_ORDER_STATUSES = new Set(["PENDING_PAYMENT"]);
const PAYMENT_SETTLED_ORDER_STATUSES = new Set(["PAID", "SHIPPED", "COMPLETED"]);
const REPEATABLE_ORDER_STATUSES = new Set(["PAID", "CANCELED", "SHIPPED", "COMPLETED", "CLOSED", "REFUNDED"]);

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
  return REFUNDABLE_ORDER_STATUSES.has(mallOrderStatusKey(order?.status)) && !mallOrderContainsMembershipGrant(order);
}

export function mallOrderCanCancel(order = {}) {
  return CANCELABLE_ORDER_STATUSES.has(mallOrderStatusKey(order?.status));
}

export function mallOrderCanPay(order = {}) {
  return PAYABLE_ORDER_STATUSES.has(mallOrderStatusKey(order?.status));
}

export function mallOrderCanConfirm(order = {}) {
  return mallOrderStatusKey(order?.status) === "SHIPPED";
}

export function mallOrderCanReview(order = {}) {
  return mallOrderStatusKey(order?.status) === "COMPLETED";
}

export function mallOrderCanRepeat(order = {}) {
  return REPEATABLE_ORDER_STATUSES.has(mallOrderStatusKey(order?.status));
}

export function mallOrderPaymentSettled(order = {}) {
  return PAYMENT_SETTLED_ORDER_STATUSES.has(mallOrderStatusKey(order?.status));
}

export function mallOrderStatusKey(status) {
  const normalized = String(status ?? "").trim().toUpperCase();
  return ORDER_STATUS_ALIASES.get(normalized) || normalized.replace(/^ORDER_STATUS_/, "");
}

export function mallOrderStatusLabel(status) {
  return ORDER_STATUS_LABELS[mallOrderStatusKey(status)] || "未知";
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
