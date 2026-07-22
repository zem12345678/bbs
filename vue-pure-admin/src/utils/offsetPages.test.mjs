import assert from "node:assert/strict";
import test from "node:test";
import {
  loadAllOffsetPages,
  OFFSET_LIST_PAGE_CONCURRENCY
} from "./offsetPages.ts";

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
    return {
      code: 503,
      message: "temporary failure",
      data: { items: [], total: 0 }
    };
  });

  assert.deepEqual(result, {
    code: 503,
    message: "temporary failure",
    items: [],
    total: 0
  });
  assert.equal(calls, 2);
});

test("loadAllOffsetPages bounds concurrent page requests and preserves order", async () => {
  const source = Array.from({ length: 501 }, (_, index) => index + 1);
  let inFlight = 0;
  let maxInFlight = 0;
  const result = await loadAllOffsetPages(async params => {
    inFlight += 1;
    maxInFlight = Math.max(maxInFlight, inFlight);
    await new Promise(resolve =>
      setTimeout(resolve, Math.max(1, 6 - params.offset / 100))
    );
    inFlight -= 1;
    return {
      code: 0,
      data: {
        items: source.slice(params.offset, params.offset + params.limit),
        total: source.length
      }
    };
  });

  assert.equal(result.code, 0);
  assert.equal(maxInFlight, OFFSET_LIST_PAGE_CONCURRENCY);
  assert.deepEqual(result.items, source);
});

test("loadAllOffsetPages honors a lower page concurrency limit", async () => {
  const source = Array.from({ length: 501 }, (_, index) => index + 1);
  let inFlight = 0;
  let maxInFlight = 0;
  const result = await loadAllOffsetPages(
    async params => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      await new Promise(resolve => setTimeout(resolve, 1));
      inFlight -= 1;
      return {
        code: 0,
        data: {
          items: source.slice(params.offset, params.offset + params.limit),
          total: source.length
        }
      };
    },
    { concurrency: 2 }
  );

  assert.equal(result.code, 0);
  assert.equal(maxInFlight, 2);
  assert.deepEqual(result.items, source);
});
