import assert from "node:assert/strict";
import test from "node:test";

import {
  MALL_COUPON_CHECKOUT_STATUS,
  MALL_COUPON_USAGE_STATUS,
  mallCouponCheckoutMessage,
  mallCouponCheckoutState,
  mallCouponIsAvailable,
  mallCouponUsageMatchesFocus,
  normalizeMallCouponUsageStatusFilter,
  sortMallCouponUsagesForFocus,
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

test("mallCouponCheckoutState blocks an archived claimed coupon before checkout", () => {
  const state = mallCouponCheckoutState({
    couponCode: "SPRING10",
    selectedCoupon: { id: 1, code: "SPRING10" },
    selectedCouponAvailable: false,
    selectedCouponUsable: false
  });

  assert.equal(state.status, MALL_COUPON_CHECKOUT_STATUS.UNAVAILABLE);
  assert.equal(state.canSubmit, false);
});

test("mallCouponIsAvailable requires an active coupon inside the valid time window", () => {
  const now = Date.UTC(2026, 6, 19, 12, 0, 0);
  const active = {
    coupon: {
      status: "ACTIVE",
      discount_credits: 10,
      starts_at: now - 1,
      ends_at: now + 1
    }
  };

  assert.equal(mallCouponIsAvailable(active, now), true);
  assert.equal(mallCouponIsAvailable({ coupon: { ...active.coupon, status: "ARCHIVED" } }, now), false);
  assert.equal(mallCouponIsAvailable({ coupon: { ...active.coupon, ends_at: now } }, now), false);
  assert.equal(mallCouponIsAvailable({ coupon: { ...active.coupon, starts_at: now + 1 } }, now), false);
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
  assert.equal(
    mallCouponCheckoutMessage({ couponState: { status: MALL_COUPON_CHECKOUT_STATUS.UNAVAILABLE } }),
    "该优惠券当前不可用，请选择其他优惠券。"
  );
  assert.equal(mallCouponCheckoutMessage({ couponState: { status: MALL_COUPON_CHECKOUT_STATUS.NONE } }), "");
});

test("normalizeMallCouponUsageStatusFilter defaults to claimed unless focusing a coupon", () => {
  assert.equal(normalizeMallCouponUsageStatusFilter(null), MALL_COUPON_USAGE_STATUS.CLAIMED);
  assert.equal(normalizeMallCouponUsageStatusFilter("", { hasFocus: true }), MALL_COUPON_USAGE_STATUS.ALL);
  assert.equal(normalizeMallCouponUsageStatusFilter("2"), MALL_COUPON_USAGE_STATUS.USED);
  assert.equal(normalizeMallCouponUsageStatusFilter("9"), MALL_COUPON_USAGE_STATUS.CLAIMED);
});

test("mallCouponUsageMatchesFocus supports usage, coupon, order, and code focus", () => {
  const usage = {
    id: "501",
    coupon_id: "77",
    code: "VIP20",
    order_id: "8801",
    coupon: { id: "77", code: "VIP20" }
  };

  assert.equal(mallCouponUsageMatchesFocus(usage, { usageId: "501" }), true);
  assert.equal(mallCouponUsageMatchesFocus(usage, { couponId: "77" }), true);
  assert.equal(mallCouponUsageMatchesFocus(usage, { orderId: "8801" }), true);
  assert.equal(mallCouponUsageMatchesFocus(usage, { code: "vip20" }), true);
  assert.equal(mallCouponUsageMatchesFocus(usage, { couponId: "88", code: "SAVE10" }), false);
});

test("sortMallCouponUsagesForFocus moves matched coupons to the front", () => {
  const items = [
    { id: 1, coupon_id: 11, code: "OLD" },
    { id: 2, coupon_id: 12, code: "VIP20" },
    { id: 3, coupon_id: 13, code: "SAVE10" }
  ];

  assert.deepEqual(
    sortMallCouponUsagesForFocus(items, { code: "save10" }).map((item) => item.id),
    [3, 1, 2]
  );
});
