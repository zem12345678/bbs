import { toNumber } from "./formatters.js";

export const MALL_COUPON_CHECKOUT_STATUS = Object.freeze({
  NONE: "none",
  ESTIMATED: "estimated",
  THRESHOLD_UNMET: "threshold_unmet",
  UNVERIFIED: "unverified"
});

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

export function shouldBlockMallCheckoutForBalance({ balanceShortfall, couponState } = {}) {
  if (toNumber(balanceShortfall) <= 0) return false;
  return couponState?.status !== MALL_COUPON_CHECKOUT_STATUS.UNVERIFIED;
}
