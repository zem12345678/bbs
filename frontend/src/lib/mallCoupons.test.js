import assert from "node:assert/strict";
import test from "node:test";

import {
  MALL_COUPON_CHECKOUT_STATUS,
  mallCouponCheckoutState,
  shouldBlockMallCheckoutForBalance
} from "./mallCoupons.js";

test("mallCouponCheckoutState allows unverified campaign coupon codes to reach backend validation", () => {
  const state = mallCouponCheckoutState({
    couponCode: "SPRING10",
    selectedCoupon: null,
    selectedCouponUsable: false
  });

  assert.equal(state.status, MALL_COUPON_CHECKOUT_STATUS.UNVERIFIED);
  assert.equal(state.canSubmit, true);
  assert.equal(state.requiresBackendValidation, true);
});

test("mallCouponCheckoutState still blocks claimed coupons that do not meet the order threshold", () => {
  const state = mallCouponCheckoutState({
    couponCode: "SPRING10",
    selectedCoupon: { id: 1, code: "SPRING10" },
    selectedCouponUsable: false
  });

  assert.equal(state.status, MALL_COUPON_CHECKOUT_STATUS.THRESHOLD_UNMET);
  assert.equal(state.canSubmit, false);
});

test("balance precheck does not block unverified coupon codes before backend applies discount", () => {
  const unverified = mallCouponCheckoutState({ couponCode: "PROMO" });
  const empty = mallCouponCheckoutState({ couponCode: "" });
  const usable = mallCouponCheckoutState({
    couponCode: "CLAIMED",
    selectedCoupon: { id: 2 },
    selectedCouponUsable: true
  });

  assert.equal(shouldBlockMallCheckoutForBalance({ balanceShortfall: 15, couponState: unverified }), false);
  assert.equal(shouldBlockMallCheckoutForBalance({ balanceShortfall: 15, couponState: empty }), true);
  assert.equal(shouldBlockMallCheckoutForBalance({ balanceShortfall: 15, couponState: usable }), true);
  assert.equal(shouldBlockMallCheckoutForBalance({ balanceShortfall: 0, couponState: empty }), false);
});
