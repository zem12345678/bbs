import assert from "node:assert/strict";
import { test } from "node:test";

import {
  friendlyMallCheckoutError,
  friendlyMallOrderActionError,
  friendlyMallReviewError,
  shouldRefreshMallCouponsAfterError,
  shouldRefreshMallInventoryAfterError
} from "./mallErrors.js";

test("maps checkout stock and credit failures to actionable messages", () => {
  assert.equal(
    friendlyMallCheckoutError({ message: "insufficient stock", meta: { legacy_code: "FailedPrecondition" }, httpCode: 412 }),
    "库存不足，请刷新商品或调整数量后重试。"
  );
  assert.equal(friendlyMallCheckoutError({ message: "积分不足" }), "积分不足，请确认余额后再兑换。");
});

test("keeps backend failed-precondition messages when no specific checkout mapping exists", () => {
  assert.equal(
    friendlyMallCheckoutError({ message: "订单暂不可支付", meta: { legacy_code: "FailedPrecondition" }, httpCode: 412 }),
    "订单暂不可支付"
  );
});

test("maps duplicate active theme entitlement to profile action", () => {
  assert.equal(
    friendlyMallCheckoutError({ message: "active theme entitlement already exists", meta: { legacy_code: "FailedPrecondition" }, httpCode: 412 }),
    "该主题权益已解锁，请直接前往个人资料启用。"
  );
});

test("maps order action errors with operation-specific fallback", () => {
  assert.equal(friendlyMallOrderActionError({ message: "" }, "取消订单失败，请刷新订单后重试。"), "取消订单失败，请刷新订单后重试。");
  assert.equal(friendlyMallOrderActionError({ message: "unsupported payment" }), "当前支付方式暂不支持，请选择积分支付。");
});

test("maps duplicate and ownership review failures to user-facing copy", () => {
  assert.equal(friendlyMallReviewError({ message: "duplicate reference", meta: { legacy_code: "AlreadyExists" } }), "该订单已评价过该商品，请勿重复提交。");
  assert.equal(friendlyMallReviewError({ message: "order does not belong to user" }), "该订单不属于当前账号，无法评价。");
});

test("detects checkout resources that should be refreshed after errors", () => {
  assert.equal(shouldRefreshMallInventoryAfterError({ message: "product unavailable" }), true);
  assert.equal(shouldRefreshMallInventoryAfterError({ message: "积分不足" }), false);
  assert.equal(shouldRefreshMallCouponsAfterError({ message: "coupon unavailable" }), true);
  assert.equal(shouldRefreshMallCouponsAfterError({ message: "库存不足" }), false);
});
