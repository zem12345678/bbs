import assert from "node:assert/strict";
import test from "node:test";

import {
  digitalEntitlementGrantKey,
  digitalEntitlementGrantType,
  isActiveMembershipEntitlement,
  isActiveThemeEntitlement,
  loadEntitlementsForFocus,
  sortFocusedEntitlements
} from "./entitlements.js";
import { loadListForFocus } from "./focusedLists.js";

test("sortFocusedEntitlements moves the focused entitlement to the front", () => {
  const items = [{ id: 101 }, { id: "503" }, { id: 204 }];

  assert.deepEqual(
    sortFocusedEntitlements(items, 503).map((item) => String(item.id)),
    ["503", "101", "204"]
  );
});

test("isActiveThemeEntitlement requires an explicit active theme grant", () => {
  const now = 2000;

  assert.equal(isActiveThemeEntitlement({ status: "ACTIVE", grant_type: "theme", grant_key: "theme-pro" }, "theme-pro", now), true);
  assert.equal(isActiveThemeEntitlement({ status: "ACTIVE", grant_type: "theme", sku: "theme-pro" }, "theme-pro", now), false);
  assert.equal(isActiveThemeEntitlement({ status: "ACTIVE", grant_type: "digital", grant_key: "theme-pro" }, "theme-pro", now), false);
  assert.equal(isActiveThemeEntitlement({ status: "ACTIVE", grant_type: "theme", grant_key: "theme-pro", revoked_at: 1500 }, "theme-pro", now), false);
  assert.equal(isActiveThemeEntitlement({ status: "ACTIVE", grant_type: "theme", grant_key: "theme-pro", expires_at: 1999 }, "theme-pro", now), false);
  assert.equal(isActiveThemeEntitlement({ grant_type: "theme", grant_key: "theme-pro" }, "theme-pro", now), false);
});

test("digital entitlement grant helpers do not infer grants from SKU", () => {
  const entitlement = {
    status: "ACTIVE",
    sku: "VIP-MONTH",
    grant_type: "",
    grant_key: ""
  };

  assert.equal(digitalEntitlementGrantType(entitlement), "");
  assert.equal(digitalEntitlementGrantKey(entitlement), "");
});

test("isActiveMembershipEntitlement requires keyed expiring membership grants", () => {
  const now = 2000;

  assert.equal(isActiveMembershipEntitlement({ status: "ACTIVE", grant_type: "membership", grant_key: "vip-month", expires_at: 3000 }, now), true);
  assert.equal(isActiveMembershipEntitlement({ status: "ACTIVE", grant_type: "membership", expires_at: 3000 }, now), false);
  assert.equal(isActiveMembershipEntitlement({ status: "ACTIVE", grant_type: "membership", grant_key: "vip-month" }, now), false);
  assert.equal(isActiveMembershipEntitlement({ status: "ACTIVE", grant_type: "membership", grant_key: "vip-month", expires_at: 1999 }, now), false);
  assert.equal(isActiveMembershipEntitlement({ status: "ACTIVE", grant_type: "digital", grant_key: "vip-month", expires_at: 3000 }, now), false);
  assert.equal(isActiveMembershipEntitlement({ grant_type: "membership", grant_key: "vip-month", expires_at: 3000 }, now), false);
});

test("loadEntitlementsForFocus fetches later pages until the focused entitlement is found", async () => {
  const calls = [];
  const pages = {
    0: { items: [{ id: 1 }, { id: 2 }], total: 5 },
    2: { items: [{ id: 3 }, { id: 4 }], total: 5 },
    4: { items: [{ id: 5 }], total: 5 }
  };
  const result = await loadEntitlementsForFocus(
    async (params, token) => {
      calls.push({ ...params, token });
      return pages[params.offset];
    },
    { limit: 2, offset: 0, status: "" },
    "token-1",
    "5",
    { focusLimit: 2 }
  );

  assert.deepEqual(
    calls.map((call) => [call.offset, call.limit, call.token]),
    [
      [0, 2, "token-1"],
      [2, 2, "token-1"],
      [4, 2, "token-1"]
    ]
  );
  assert.equal(result.total, 5);
  assert.deepEqual(
    result.items.map((item) => item.id),
    [5, 1, 2, 3, 4]
  );
});

test("loadEntitlementsForFocus keeps ordinary entitlement lists to one request", async () => {
  const calls = [];
  const result = await loadEntitlementsForFocus(
    async (params) => {
      calls.push(params);
      return { items: [{ id: 11 }, { id: 12 }], total: 12 };
    },
    { limit: 50, offset: 0, status: "ACTIVE" },
    "token-2",
    ""
  );

  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0], { limit: 50, offset: 0, status: "ACTIVE" });
  assert.equal(result.total, 12);
  assert.deepEqual(
    result.items.map((item) => item.id),
    [11, 12]
  );
});

test("loadListForFocus treats empty compound focus as an ordinary one-page list", async () => {
  const calls = [];
  const result = await loadListForFocus(
    async (params) => {
      calls.push(params);
      return { items: [{ id: 21 }], total: 7 };
    },
    { limit: 50, offset: 0, status: 0 },
    "token-3",
    { refundId: "", orderId: "" },
    (item, focus) => item.id === focus.refundId,
    (items) => items
  );

  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0], { limit: 50, offset: 0, status: 0 });
  assert.equal(result.total, 7);
  assert.deepEqual(result.items, [{ id: 21 }]);
});

test("loadListForFocus pages compound focus lists until a matcher succeeds", async () => {
  const calls = [];
  const pages = {
    0: { items: [{ id: 31, order_id: 1001 }], total: 3 },
    1: { items: [{ id: 32, order_id: 1002 }], total: 3 },
    2: { items: [{ id: 33, order_id: 1003 }], total: 3 }
  };
  const result = await loadListForFocus(
    async (params) => {
      calls.push(params);
      return pages[params.offset];
    },
    { limit: 1, offset: 0, status: 0 },
    "token-4",
    { refundId: "", orderId: "1003" },
    (refund, focus) => String(refund.order_id) === String(focus.orderId),
    (items, focus) => [
      ...items.filter((refund) => String(refund.order_id) === String(focus.orderId)),
      ...items.filter((refund) => String(refund.order_id) !== String(focus.orderId))
    ],
    { focusLimit: 1 }
  );

  assert.deepEqual(
    calls.map((call) => call.offset),
    [0, 1, 2]
  );
  assert.equal(result.total, 3);
  assert.deepEqual(
    result.items.map((item) => item.id),
    [33, 31, 32]
  );
});
