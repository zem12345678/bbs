import assert from "node:assert/strict";
import test from "node:test";

import { badgeExpiryText, badgeMeta, userBadgeRows } from "./badges.js";

test("userBadgeRows formats synthetic membership entitlement badges", () => {
  const expiresAt = 1783934400000;
  const rows = userBadgeRows(
    [
      {
        id: "digital-membership-vip-month",
        name: "会员",
        description: "已开通会员权益，当前有效至 2026-07-13。",
        source: "digital_entitlement",
        awarded_at: 1783848000000,
        grant_type: "membership",
        grant_key: "vip-month",
        order_no: "O-99",
        expires_at: expiresAt
      }
    ],
    { now: 1783848000000 }
  );

  assert.equal(rows[0].key, "digital-membership-vip-month");
  assert.equal(rows[0].title, "会员");
  assert.equal(rows[0].description, "已开通会员权益，当前有效至 2026-07-13。");
  assert.match(rows[0].meta, /^会员权益 · 获得于 /u);
  assert.match(rows[0].meta, /有效至 2026\/07\/13/u);
  assert.match(rows[0].meta, /订单 O-99$/u);
});

test("badgeMeta keeps ordinary rule badges readable", () => {
  assert.equal(badgeMeta({ status: "awarded" }), "已获得");
  assert.equal(badgeMeta({ awarded_at: 1783848000000 }, { now: 1783848000000 }), "获得于 刚刚");
});

test("badgeExpiryText marks expired membership badges", () => {
  assert.equal(badgeExpiryText({ expires_at: 1783848000000 }, 1783934400000), "已过期 2026/07/12");
  assert.equal(badgeExpiryText({ expires_at: 0 }, 1783934400000), "");
});
