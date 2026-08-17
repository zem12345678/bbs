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
    { id: 4, type: "mall_digital_entitlement_revoked", entity_type: "mall_order", entity_id: 8801, source_id: 503 },
    { id: 5, type: "comment", entity_type: "topic", entity_id: 1001 }
  ];

  assert.equal(isMallNotification(items[0]), true);
  assert.equal(mallNotificationGroup(items[0]), "order");
  assert.equal(mallNotificationGroup(items[1]), "refund");
  assert.equal(mallNotificationGroup(items[2]), "review");
  assert.equal(mallNotificationGroup(items[3]), "entitlement");
  assert.deepEqual(filterNotifications(items, "mall").map((item) => item.id), [1, 2, 3, 4]);

  const summary = summarizeNotifications(items);
  assert.equal(summary.total, 5);
  assert.equal(summary.unread, 4);
  assert.equal(summary.mall.total, 4);
  assert.equal(summary.mall.unread, 3);
  assert.deepEqual(
    ["order", "refund", "review", "entitlement"].map((group) => summary.mall.groups[group].total),
    [1, 1, 1, 1]
  );
});

test("keeps mall notification targets and labels actionable", () => {
  const refundNotification = { type: "mall_refund_rejected", entity_type: "mall_order", entity_id: "8801", source_id: "9902" };
  const reviewNotification = { type: "mall_review_published", entity_type: "mall_product", entity_id: "9901", source_id: "7701" };
  const entitlementNotification = { type: "mall_digital_entitlement_revoked", entity_type: "mall_order", entity_id: "8801", source_id: "503" };

  assert.equal(notificationTarget(refundNotification), "/dashboard/refunds?refund_id=9902&order_id=8801");
  assert.equal(notificationTargetLabel(refundNotification), "查看售后");
  assert.equal(notificationGroupLabel(refundNotification), "售后");
  assert.equal(notificationToneClass(refundNotification), "type-mall-refund");

  assert.equal(notificationTarget(entitlementNotification), "/dashboard/entitlements?entitlement_id=503");
  assert.equal(notificationTargetLabel(entitlementNotification), "查看权益");
  assert.equal(notificationGroupLabel(entitlementNotification), "权益");
  assert.equal(notificationToneClass(entitlementNotification), "type-mall-entitlement");

  assert.equal(notificationTarget(reviewNotification), "/dashboard/reviews?review_id=7701&product_id=9901");
  assert.equal(notificationTargetLabel(reviewNotification), "查看评价");
  assert.equal(notificationGroupLabel(reviewNotification), "评价");
});

test("labels directed system notifications distinctly", () => {
  const notification = { type: "system", entity_type: "system" };

  assert.equal(notificationGroupLabel(notification), "系统");
  assert.equal(notificationTarget(notification), "");
  assert.equal(notificationTargetLabel(notification), "查看");
});

test("routes export completion notifications to the generated file", () => {
  const notification = { type: "export_completed", entity_type: "file", entity_id: "9223372036854775807" };

  assert.equal(notificationTarget(notification), "/dashboard/files?file_id=9223372036854775807");
  assert.equal(notificationTargetLabel(notification), "查看文件");
  assert.equal(notificationGroupLabel(notification), "数据导出");
});

test("routes follow request notifications to their actionable destination", () => {
  const received = { type: "follow_request_received", entity_type: "user", entity_id: 22, actor_id: 22 };
  const accepted = { type: "follow_request_accepted", entity_type: "user", entity_id: 33, actor_id: 33 };

  assert.equal(notificationTarget(received), "/dashboard/interactions?mode=follow-received");
  assert.equal(notificationTargetLabel(received), "处理申请");
  assert.equal(notificationGroupLabel(received), "关注");
  assert.equal(notificationToneClass(received), "type-follow");

  assert.equal(notificationTarget(accepted), "/user/33");
  assert.equal(notificationTargetLabel(accepted), "查看用户");
  assert.equal(notificationGroupLabel(accepted), "关注");
  assert.equal(notificationToneClass(accepted), "type-follow");
});
