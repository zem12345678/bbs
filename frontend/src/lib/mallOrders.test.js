import assert from "node:assert/strict";
import test from "node:test";

import {
  mallOrderCanApplyRefund,
  mallOrderCanCancel,
  mallOrderCanConfirm,
  mallOrderCanPay,
  mallOrderCanRepeat,
  mallOrderCanReview,
  mallOrderContainsMembershipGrant,
  mallOrderPaymentSettled,
  mallOrderReviewableProductIds,
  mallOrderStatusKey,
  mallOrderStatusLabel
} from "./mallOrders.js";

test("mallOrderReviewableProductIds preserves unique order item product ids", () => {
  assert.deepEqual(
    mallOrderReviewableProductIds({
      items: [
        { product_id: 101, title: "会员月卡" },
        { productId: "102", title: "高级主题" },
        { product: { id: 103 }, title: "创始徽章" },
        { product_id: 101, title: "会员月卡加购" },
        { title: "缺少商品 ID" }
      ]
    }),
    ["101", "102", "103"]
  );
});

test("mallOrderReviewableProductIds handles empty orders", () => {
  assert.deepEqual(mallOrderReviewableProductIds({}), []);
  assert.deepEqual(mallOrderReviewableProductIds({ items: null }), []);
});

test("mallOrderCanApplyRefund blocks membership entitlement orders", () => {
  assert.equal(mallOrderCanApplyRefund({ status: 3, items: [{ grant_type: "membership", grant_key: "vip-month" }] }), false);
  assert.equal(mallOrderCanApplyRefund({ status: 5, items: [{ grant_key: "vip-month" }] }), false);
  assert.equal(
    mallOrderCanApplyRefund({
      status: 6,
      digital_entitlements: [{ grant_type: "membership", grant_key: "vip-month", status: "ACTIVE" }]
    }),
    false
  );
});

test("mallOrderCanApplyRefund still allows refundable non-membership orders", () => {
  assert.equal(mallOrderCanApplyRefund({ status: 3, items: [{ title: "实体商品" }] }), true);
  assert.equal(mallOrderCanApplyRefund({ status: "PAID", items: [{ title: "实体商品" }] }), true);
  assert.equal(mallOrderCanApplyRefund({ status: "ORDER_STATUS_SHIPPED", items: [{ title: "实体商品" }] }), true);
  assert.equal(mallOrderCanApplyRefund({ status: 6, items: [{ grant_type: "badge", grant_key: "badge-founder" }] }), true);
  assert.equal(mallOrderCanApplyRefund({ status: 1, items: [{ title: "待支付商品" }] }), false);
});

test("mallOrderCanCancel only allows pending payment orders across status shapes", () => {
  assert.equal(mallOrderCanCancel({ status: 1 }), true);
  assert.equal(mallOrderCanCancel({ status: "PENDING_PAYMENT" }), true);
  assert.equal(mallOrderCanCancel({ status: "ORDER_STATUS_PENDING_PAYMENT" }), true);
  assert.equal(mallOrderCanCancel({ status: 2 }), false);
  assert.equal(mallOrderCanCancel({ status: 3 }), false);
});

test("mall order action guards handle string and proto enum status names", () => {
  assert.equal(mallOrderCanPay({ status: "PAYING" }), true);
  assert.equal(mallOrderCanConfirm({ status: "ORDER_STATUS_SHIPPED" }), true);
  assert.equal(mallOrderCanReview({ status: "COMPLETED" }), true);
  assert.equal(mallOrderCanRepeat({ status: "ORDER_STATUS_REFUNDED" }), true);
  assert.equal(mallOrderCanPay({ status: "PAID" }), false);
  assert.equal(mallOrderCanConfirm({ status: "PAID" }), false);
});

test("mallOrderPaymentSettled recognizes paid, shipped, and completed orders", () => {
  assert.equal(mallOrderPaymentSettled({ status: 3 }), true);
  assert.equal(mallOrderPaymentSettled({ status: "PAID" }), true);
  assert.equal(mallOrderPaymentSettled({ status: "ORDER_STATUS_PAID" }), true);
  assert.equal(mallOrderPaymentSettled({ status: 5 }), true);
  assert.equal(mallOrderPaymentSettled({ status: "SHIPPED" }), true);
  assert.equal(mallOrderPaymentSettled({ status: 6 }), true);
  assert.equal(mallOrderPaymentSettled({ status: "COMPLETED" }), true);
  assert.equal(mallOrderPaymentSettled({ status: "PENDING_PAYMENT" }), false);
  assert.equal(mallOrderPaymentSettled({ status: "REFUNDED" }), false);
  assert.equal(mallOrderPaymentSettled({ status: "CANCELED" }), false);
});

test("mallOrderContainsMembershipGrant detects item and entitlement grants", () => {
  assert.equal(mallOrderContainsMembershipGrant({ items: [{ grant_key: "member-year" }] }), true);
  assert.equal(mallOrderContainsMembershipGrant({ digitalEntitlements: [{ grant_type: "membership", grant_key: "vip-month" }] }), true);
  assert.equal(mallOrderContainsMembershipGrant({ items: [{ grant_type: "theme", grant_key: "theme-pro" }] }), false);
});

test("mall order status helpers normalize gateway and proto status names", () => {
  assert.equal(mallOrderStatusKey(6), "COMPLETED");
  assert.equal(mallOrderStatusKey("completed"), "COMPLETED");
  assert.equal(mallOrderStatusKey("ORDER_STATUS_COMPLETED"), "COMPLETED");
  assert.equal(mallOrderStatusLabel("ORDER_STATUS_PENDING_PAYMENT"), "待支付");
  assert.equal(mallOrderStatusLabel("unexpected"), "未知");
});
