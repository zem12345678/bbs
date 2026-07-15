import { toNumber } from "./formatters.js";

export const MALL_COUPON_USAGE_STATUS = Object.freeze({
  ALL: 0,
  RESERVED: 1,
  USED: 2,
  RELEASED: 3,
  CLAIMED: 4
});

export const MALL_COUPON_CHECKOUT_STATUS = Object.freeze({
  NONE: "none",
  ESTIMATED: "estimated",
  THRESHOLD_UNMET: "threshold_unmet",
  UNVERIFIED: "unverified"
});

const mallCouponUsageStatuses = new Set(Object.values(MALL_COUPON_USAGE_STATUS));

export function mallCouponCheckoutState({ couponCode, selectedCoupon, selectedCouponUsable } = {}) {
  const hasCouponCode = Boolean(String(couponCode || "").trim());
  if (!hasCouponCode) {
    return {
      status: MALL_COUPON_CHECKOUT_STATUS.NONE,
      canSubmit: true,
      requiresBackendValidation: false
    };
  }
  if (!selectedCoupon) {
    return {
      status: MALL_COUPON_CHECKOUT_STATUS.UNVERIFIED,
      canSubmit: true,
      requiresBackendValidation: true
    };
  }
  if (selectedCouponUsable) {
    return {
      status: MALL_COUPON_CHECKOUT_STATUS.ESTIMATED,
      canSubmit: true,
      requiresBackendValidation: false
    };
  }
  return {
    status: MALL_COUPON_CHECKOUT_STATUS.THRESHOLD_UNMET,
    canSubmit: false,
    requiresBackendValidation: false
  };
}

export function mallCouponUsageId(usage) {
  return normalizeCouponFocusValue(usage?.id ?? usage?.Id ?? usage?.usage_id ?? usage?.usageId);
}

export function mallCouponId(usage) {
  const coupon = couponSource(usage);
  return normalizeCouponFocusValue(usage?.coupon_id ?? usage?.couponId ?? coupon?.id ?? coupon?.Id);
}

export function mallCouponCode(usage) {
  const coupon = couponSource(usage);
  return String(usage?.code ?? usage?.Code ?? coupon?.code ?? coupon?.Code ?? "")
    .trim()
    .toUpperCase();
}

export function mallCouponOrderId(usage) {
  return normalizeCouponFocusValue(usage?.order_id ?? usage?.orderId);
}

export function normalizeMallCouponUsageStatusFilter(value, options = {}) {
  if (value !== null && value !== undefined && String(value).trim() !== "") {
    const status = toNumber(value);
    if (mallCouponUsageStatuses.has(status)) return status;
  }
  return options.hasFocus ? MALL_COUPON_USAGE_STATUS.ALL : MALL_COUPON_USAGE_STATUS.CLAIMED;
}

export function mallCouponUsageMatchesFocus(usage, focus = {}) {
  if (!usage || !hasMallCouponFocus(focus)) return false;
  if (focus.usageId && mallCouponUsageId(usage) === normalizeCouponFocusValue(focus.usageId)) return true;
  if (focus.couponId && mallCouponId(usage) === normalizeCouponFocusValue(focus.couponId)) return true;
  if (focus.orderId && mallCouponOrderId(usage) === normalizeCouponFocusValue(focus.orderId)) return true;
  if (focus.code && mallCouponCode(usage) === String(focus.code).trim().toUpperCase()) return true;
  return false;
}

export function sortMallCouponUsagesForFocus(items = [], focus = {}) {
  if (!hasMallCouponFocus(focus)) return items;
  return [...items].sort((left, right) => {
    const leftFocused = mallCouponUsageMatchesFocus(left, focus);
    const rightFocused = mallCouponUsageMatchesFocus(right, focus);
    if (leftFocused !== rightFocused) return leftFocused ? -1 : 1;
    return 0;
  });
}

export function mallCouponCheckoutMessage({ couponState, couponCode, couponName, discountCredits, minOrderCredits } = {}) {
  const status = couponState?.status;
  if (status === MALL_COUPON_CHECKOUT_STATUS.ESTIMATED) {
    const label = String(couponName || couponCode || "优惠券").trim();
    return `${label} 已预估优惠 ${toNumber(discountCredits)} 积分`;
  }
  if (status === MALL_COUPON_CHECKOUT_STATUS.THRESHOLD_UNMET) {
    return `该优惠券需满 ${toNumber(minOrderCredits)} 积分可用`;
  }
  if (status === MALL_COUPON_CHECKOUT_STATUS.UNVERIFIED) {
    return "优惠码将提交给系统校验，实际优惠和应付积分以订单结果为准。";
  }
  return "";
}

export function shouldBlockMallCheckoutForBalance({ balanceShortfall, couponState } = {}) {
  if (toNumber(balanceShortfall) <= 0) return false;
  return couponState?.status !== MALL_COUPON_CHECKOUT_STATUS.UNVERIFIED;
}

function couponSource(usage) {
  return usage?.coupon ?? usage?.Coupon ?? {};
}

function normalizeCouponFocusValue(value) {
  return String(value ?? "").trim();
}

function hasMallCouponFocus(focus = {}) {
  return Boolean(
    normalizeCouponFocusValue(focus.usageId) ||
      normalizeCouponFocusValue(focus.couponId) ||
      normalizeCouponFocusValue(focus.orderId) ||
      normalizeCouponFocusValue(focus.code)
  );
}
