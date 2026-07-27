import assert from "node:assert/strict";
import test from "node:test";

import { checkoutAttemptKey, checkoutAttemptOrderIds, clearCheckoutAttemptForOrder, clearCheckoutAttemptKey, recordCheckoutAttemptOrder } from "./idempotencyKeys.js";

test("checkout attempt retries reuse one key for the same order intent", () => {
  const storage = new MemoryStorage();
  const first = checkoutAttemptKey({
    userId: "42",
    intent: {
      mode: "single",
      items: [{ product_id: "200", quantity: 1 }, { product_id: "100", quantity: 2 }],
      coupon_code: " save10 ",
      receiver: "Alice",
      phone: "13800000000",
      address: "Shanghai"
    },
    storage,
    createKey: () => "order-attempt-1"
  });
  const retry = checkoutAttemptKey({
    userId: "42",
    intent: {
      mode: "single",
      items: [{ product_id: "100", quantity: 2 }, { product_id: "200", quantity: 1 }],
      coupon_code: "SAVE10",
      receiver: " Alice ",
      phone: "13800000000",
      address: "Shanghai"
    },
    storage,
    createKey: () => "order-attempt-2"
  });

  assert.equal(first, "order-attempt-1");
  assert.equal(retry, first);
});

test("checkout attempt key changes when the order intent changes", () => {
  const storage = new MemoryStorage();
  let created = 0;
  const createKey = () => `order-attempt-${++created}`;
  const first = checkoutAttemptKey({
    userId: "42",
    intent: { mode: "single", items: [{ product_id: "100", quantity: 1 }] },
    storage,
    createKey
  });
  const changed = checkoutAttemptKey({
    userId: "42",
    intent: { mode: "single", items: [{ product_id: "100", quantity: 2 }] },
    storage,
    createKey
  });
  const otherUser = checkoutAttemptKey({
    userId: "7",
    intent: { mode: "single", items: [{ product_id: "100", quantity: 2 }] },
    storage,
    createKey
  });

  assert.equal(first, "order-attempt-1");
  assert.equal(changed, "order-attempt-2");
  assert.equal(otherUser, "order-attempt-3");
});

test("checkout retries keep each unresolved order intent's key", () => {
  const storage = new MemoryStorage();
  let created = 0;
  const createKey = () => `order-attempt-${++created}`;
  const firstIntent = { mode: "single", items: [{ product_id: "100", quantity: 1 }] };
  const secondIntent = { mode: "single", items: [{ product_id: "200", quantity: 1 }] };

  const first = checkoutAttemptKey({ userId: "42", intent: firstIntent, storage, createKey });
  const second = checkoutAttemptKey({ userId: "42", intent: secondIntent, storage, createKey });
  const firstRetry = checkoutAttemptKey({ userId: "42", intent: firstIntent, storage, createKey });

  assert.equal(first, "order-attempt-1");
  assert.equal(second, "order-attempt-2");
  assert.equal(firstRetry, first);
});

test("clearing a completed checkout allows a new attempt", () => {
  const storage = new MemoryStorage();
  const intent = { mode: "cart", items: [{ product_id: "100", quantity: 1 }] };
  const otherIntent = { mode: "cart", items: [{ product_id: "200", quantity: 1 }] };
  const first = checkoutAttemptKey({ userId: "42", intent, storage, createKey: () => "order-attempt-1" });
  const other = checkoutAttemptKey({ userId: "42", intent: otherIntent, storage, createKey: () => "order-attempt-2" });

  clearCheckoutAttemptKey({ userId: "42", intent, storage });
  const next = checkoutAttemptKey({ userId: "42", intent, storage, createKey: () => "order-attempt-3" });
  const otherRetry = checkoutAttemptKey({ userId: "42", intent: otherIntent, storage, createKey: () => "order-attempt-4" });

  assert.equal(first, "order-attempt-1");
  assert.equal(other, "order-attempt-2");
  assert.equal(next, "order-attempt-3");
  assert.equal(otherRetry, other);
});

test("canceling after an unknown checkout result keeps the key for a retry", () => {
  const storage = new MemoryStorage();
  const intent = { mode: "cart", items: [{ product_id: "100", quantity: 1 }] };
  const first = checkoutAttemptKey({ userId: "42", intent, storage, createKey: () => "order-attempt-1" });

  // A timeout leaves the server-side result unknown. Closing the checkout UI must
  // not clear this key, so a later retry remains idempotent.
  const retryAfterCancel = checkoutAttemptKey({ userId: "42", intent, storage, createKey: () => "order-attempt-2" });

  assert.equal(first, "order-attempt-1");
  assert.equal(retryAfterCancel, first);
});

test("a created order remains associated with its checkout key across a page reload", () => {
  const storage = new MemoryStorage();
  const intent = { mode: "single", items: [{ product_id: "100", quantity: 1 }] };
  const first = checkoutAttemptKey({ userId: "42", intent, storage, createKey: () => "order-attempt-1" });
  recordCheckoutAttemptOrder({ userId: "42", intent, orderId: "700", storage });

  const reloadedStorage = new MemoryStorage();
  reloadedStorage.values = new Map(storage.values);
  const retry = checkoutAttemptKey({ userId: "42", intent, storage: reloadedStorage, createKey: () => "order-attempt-2" });

  assert.equal(first, "order-attempt-1");
  assert.equal(retry, first);
  assert.deepEqual(checkoutAttemptOrderIds({ userId: "42", storage: reloadedStorage }), ["700"]);
});

test("clearing a completed or canceled order keeps unrelated checkout attempts", () => {
  const storage = new MemoryStorage();
  const firstIntent = { mode: "single", items: [{ product_id: "100", quantity: 1 }] };
  const secondIntent = { mode: "single", items: [{ product_id: "200", quantity: 1 }] };
  checkoutAttemptKey({ userId: "42", intent: firstIntent, storage, createKey: () => "order-attempt-1" });
  checkoutAttemptKey({ userId: "42", intent: secondIntent, storage, createKey: () => "order-attempt-2" });
  recordCheckoutAttemptOrder({ userId: "42", intent: firstIntent, orderId: "700", storage });
  recordCheckoutAttemptOrder({ userId: "42", intent: secondIntent, orderId: "701", storage });

  clearCheckoutAttemptForOrder({ userId: "42", orderId: "700", storage });

  assert.deepEqual(checkoutAttemptOrderIds({ userId: "42", storage }), ["701"]);
  assert.equal(checkoutAttemptKey({ userId: "42", intent: firstIntent, storage, createKey: () => "order-attempt-3" }), "order-attempt-3");
  assert.equal(checkoutAttemptKey({ userId: "42", intent: secondIntent, storage, createKey: () => "order-attempt-4" }), "order-attempt-2");
});

test("checkout retries still reuse a key when session storage is unavailable", () => {
  const storage = new FailingStorage();
  const intent = { mode: "single", items: [{ product_id: "100", quantity: 1 }] };
  const first = checkoutAttemptKey({ userId: "fallback", intent, storage, createKey: () => "order-attempt-1" });
  const retry = checkoutAttemptKey({ userId: "fallback", intent, storage, createKey: () => "order-attempt-2" });

  assert.equal(first, "order-attempt-1");
  assert.equal(retry, first);
  clearCheckoutAttemptKey({ userId: "fallback", storage });
});

test("checkout retries keep an in-memory key when session storage cannot be read", () => {
  const storage = new ReadFailingStorage();
  const intent = { mode: "single", items: [{ product_id: "100", quantity: 1 }] };
  const first = checkoutAttemptKey({ userId: "read-fallback", intent, storage, createKey: () => "order-attempt-1" });
  const retry = checkoutAttemptKey({ userId: "read-fallback", intent, storage, createKey: () => "order-attempt-2" });

  assert.equal(first, "order-attempt-1");
  assert.equal(retry, first);
  clearCheckoutAttemptKey({ userId: "read-fallback", storage });
});

test("clearing an attempt persists a tombstone when removal is blocked", () => {
  const storage = new RemoveFailingStorage();
  const intent = { mode: "single", items: [{ product_id: "100", quantity: 1 }] };
  const first = checkoutAttemptKey({ userId: "remove-fallback", intent, storage, createKey: () => "order-attempt-1" });

  clearCheckoutAttemptKey({ userId: "remove-fallback", intent, storage });
  const reloadedStorage = new MemoryStorage();
  reloadedStorage.values = new Map(storage.values);
  const next = checkoutAttemptKey({ userId: "remove-fallback", intent, storage: reloadedStorage, createKey: () => "order-attempt-2" });

  assert.equal(first, "order-attempt-1");
  assert.equal(next, "order-attempt-2");
  clearCheckoutAttemptKey({ userId: "remove-fallback", storage });
});

class MemoryStorage {
  values = new Map();

  getItem(key) {
    return this.values.get(key) || null;
  }

  setItem(key, value) {
    this.values.set(key, String(value));
  }

  removeItem(key) {
    this.values.delete(key);
  }
}

class FailingStorage extends MemoryStorage {
  setItem() {
    throw new Error("storage blocked");
  }
}

class ReadFailingStorage extends MemoryStorage {
  getItem() {
    throw new Error("storage blocked");
  }
}

class RemoveFailingStorage extends MemoryStorage {
  removeItem() {
    throw new Error("storage blocked");
  }
}
