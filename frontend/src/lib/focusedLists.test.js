import assert from "node:assert/strict";
import test from "node:test";

import { loadAllListPages } from "./focusedLists.js";

test("loads every reported page and preserves the request token", async () => {
  const calls = [];
  const result = await loadAllListPages(
    async (params, token) => {
      calls.push({ params, token });
      const items = Array.from({ length: Math.min(20, 21 - params.offset) }, (_, index) => ({ id: params.offset + index + 1 }));
      return { items, total: 21 };
    },
    { limit: 20, offset: 0 },
    "access-token"
  );

  assert.deepEqual(
    calls.map(({ params, token }) => ({ limit: params.limit, offset: params.offset, token })),
    [
      { limit: 20, offset: 0, token: "access-token" },
      { limit: 20, offset: 20, token: "access-token" }
    ]
  );
  assert.equal(result.total, 21);
  assert.deepEqual(
    result.items.map((item) => item.id),
    Array.from({ length: 21 }, (_, index) => index + 1)
  );
});

test("does not request another page after reaching the reported total", async () => {
  let calls = 0;
  const result = await loadAllListPages(async () => {
    calls += 1;
    return { items: [{ id: 1 }, { id: 2 }], total: 2 };
  });

  assert.equal(calls, 1);
  assert.equal(result.total, 2);
  assert.deepEqual(result.items, [{ id: 1 }, { id: 2 }]);
});
