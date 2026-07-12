import assert from "node:assert/strict";
import { test } from "node:test";

import {
  filterNotifications,
  isMallNotification,
  mallNotificationGroup,
  notificationGroupLabel,
  notificationTarget,
  notificationTargetLabel,
  notificationToneClass,
  summarizeNotifications
} from "./notificationTargets.js";

test("identifies and groups mall notifications", () => {
  const items = [
    { id: 1, type: "mall_order_paid", entity_type: "mall_order", entity_id: 8801 },
    { id: 2, type: "mall_refund_approved", entity_type: "mall_order", entity_id: 8801, read_at: 1783000000000 },
    { id: 3, type: "mall_review_published", entity_type: "mall_product", entity_id: 9901 },
    { id: 4, type: "comment", entity_type: "topic", entity_id: 1001 }
  ];

  assert.equal(isMallNotification(items[0]), true);
  assert.equal(mallNotificationGroup(items[0]), "order");
  assert.equal(mallNotificationGroup(items[1]), "refund");
  assert.equal(mallNotificationGroup(items[2]), "review");
  assert.deepEqual(filterNotifications(items, "mall").map((item) => item.id), [1, 2, 3]);

  const summary = summarizeNotifications(items);
  assert.equal(summary.total, 4);
  assert.equal(summary.unread, 3);
  assert.equal(summary.mall.total, 3);
  assert.equal(summary.mall.unread, 2);
  assert.deepEqual(
    ["order", "refund", "review"].map((group) => summary.mall.groups[group].total),
    [1, 1, 1]
  );
});

test("keeps mall notification targets and labels actionable", () => {
  const refundNotification = { type: "mall_refund_rejected", entity_type: "mall_order", entity_id: "8801" };
  const reviewNotification = { type: "mall_review_published", entity_type: "mall_product", entity_id: "9901", source_id: "7701" };

  assert.equal(notificationTarget(refundNotification), "/dashboard/orders?order_id=8801");
  assert.equal(notificationTargetLabel(refundNotification), "查看售后");
  assert.equal(notificationGroupLabel(refundNotification), "售后");
  assert.equal(notificationToneClass(refundNotification), "type-mall-refund");

  assert.equal(notificationTarget(reviewNotification), "/dashboard/reviews?review_id=7701&product_id=9901");
  assert.equal(notificationTargetLabel(reviewNotification), "查看评价");
  assert.equal(notificationGroupLabel(reviewNotification), "评价");
});
