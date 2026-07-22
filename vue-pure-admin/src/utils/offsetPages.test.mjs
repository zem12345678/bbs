import assert from "node:assert/strict";
import test from "node:test";
import { loadAllOffsetPages } from "./offsetPages.ts";

test("loadAllOffsetPages loads every offset page", async () => {
  const source = Array.from({ length: 205 }, (_, index) => index + 1);
  const calls = [];
  const result = await loadAllOffsetPages(async params => {
    calls.push(params);
    return {
      code: 0,
      message: "",
      data: {
        items: source.slice(params.offset, params.offset + params.limit),
        total: source.length
      }
    };
  });

  assert.equal(result.code, 0);
  assert.equal(result.total, source.length);
  assert.deepEqual(result.items, source);
  assert.deepEqual(calls, [
    { limit: 100, offset: 0 },
    { limit: 100, offset: 100 },
    { limit: 100, offset: 200 }
  ]);
});

test("loadAllOffsetPages stops when a later page fails", async () => {
  let calls = 0;
  const result = await loadAllOffsetPages(async ({ offset }) => {
    calls += 1;
    if (offset === 0) {
      return {
        code: 0,
        message: "",
        data: { items: [1], total: 2 }
      };
    }
    return { code: 503, message: "temporary failure", data: { items: [], total: 0 } };
  });

  assert.deepEqual(result, {
    code: 503,
    message: "temporary failure",
    items: [],
    total: 0
  });
  assert.equal(calls, 2);
});
