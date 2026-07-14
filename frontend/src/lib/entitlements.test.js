import assert from "node:assert/strict";
import test from "node:test";

import { loadEntitlementsForFocus, sortFocusedEntitlements } from "./entitlements.js";

test("sortFocusedEntitlements moves the focused entitlement to the front", () => {
  const items = [{ id: 101 }, { id: "503" }, { id: 204 }];

  assert.deepEqual(
    sortFocusedEntitlements(items, 503).map((item) => String(item.id)),
    ["503", "101", "204"]
  );
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
