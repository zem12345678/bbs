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
