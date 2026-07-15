import assert from "node:assert/strict";
import test from "node:test";

import {
  MALL_COUPON_CHECKOUT_STATUS,
  mallCouponCheckoutMessage,
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

test("mallCouponCheckoutMessage formats checkout coupon states", () => {
  assert.equal(
    mallCouponCheckoutMessage({
      couponState: { status: MALL_COUPON_CHECKOUT_STATUS.ESTIMATED },
      couponName: "新人券",
      couponCode: "NEW10",
      discountCredits: 10
    }),
    "新人券 已预估优惠 10 积分"
  );
  assert.equal(
    mallCouponCheckoutMessage({
      couponState: { status: MALL_COUPON_CHECKOUT_STATUS.THRESHOLD_UNMET },
      minOrderCredits: 100
    }),
    "该优惠券需满 100 积分可用"
  );
  assert.equal(
    mallCouponCheckoutMessage({
      couponState: { status: MALL_COUPON_CHECKOUT_STATUS.UNVERIFIED },
      couponCode: "PROMO"
    }),
    "优惠码将提交给系统校验，实际优惠和应付积分以订单结果为准。"
  );
  assert.equal(mallCouponCheckoutMessage({ couponState: { status: MALL_COUPON_CHECKOUT_STATUS.NONE } }), "");
});
