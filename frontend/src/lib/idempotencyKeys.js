const CHECKOUT_ATTEMPT_STORAGE_PREFIX = "bbs.mall.checkout-attempt.v1";
const fallbackCheckoutAttempts = new WeakMap();
const unavailableStorageAttempts = new Map();

export function paymentAttemptKey(scope, orderId) {
  const prefix = `${scope || "pay"}-${orderId || "order"}`;
  return `${prefix}-${randomKeySuffix()}`;
}

export function checkoutAttemptKey({ userId, intent, storage, createKey } = {}) {
  const fingerprint = checkoutIntentFingerprint(intent);
  const session = resolveSessionStorage(storage);
  const key = checkoutAttemptStorageKey(userId);
  const attempts = checkoutAttemptsFor(session, key);
  const stored = attempts.find((attempt) => attempt.fingerprint === fingerprint && attempt.idempotencyKey);
  if (stored) {
    return stored.idempotencyKey;
  }

  const idempotencyKey = typeof createKey === "function" ? createKey() : `web-order-${normalizedText(intent?.mode || "single") || "single"}-${randomKeySuffix()}`;
  if (idempotencyKey) {
    attempts.push({ fingerprint, idempotencyKey });
    persistCheckoutAttempts(session, key, attempts);
  }
  return idempotencyKey;
}

export function clearCheckoutAttemptKey({ userId, intent, storage } = {}) {
  const session = resolveSessionStorage(storage);
  const key = checkoutAttemptStorageKey(userId);
  const fingerprint = intent ? checkoutIntentFingerprint(intent) : "";
  if (!fingerprint) {
    persistCheckoutAttempts(session, key, []);
    return;
  }
  const attempts = checkoutAttemptsFor(session, key).filter((attempt) => attempt.fingerprint !== fingerprint);
  persistCheckoutAttempts(session, key, attempts);
}

export function recordCheckoutAttemptOrder({ userId, intent, orderId, storage } = {}) {
  const fingerprint = checkoutIntentFingerprint(intent);
  const normalizedOrderId = normalizedText(orderId);
  if (!fingerprint || !normalizedOrderId) return;
  const session = resolveSessionStorage(storage);
  const key = checkoutAttemptStorageKey(userId);
  const attempts = checkoutAttemptsFor(session, key);
  const nextAttempts = attempts.map((attempt) =>
    attempt.fingerprint === fingerprint ? { ...attempt, orderId: normalizedOrderId } : attempt
  );
  if (nextAttempts.some((attempt, index) => attempt !== attempts[index])) {
    persistCheckoutAttempts(session, key, nextAttempts);
  }
}

export function checkoutAttemptOrderIds({ userId, storage } = {}) {
  const session = resolveSessionStorage(storage);
  const key = checkoutAttemptStorageKey(userId);
  const ids = new Set();
  checkoutAttemptsFor(session, key).forEach((attempt) => {
    const orderId = normalizedText(attempt?.orderId ?? attempt?.order_id);
    if (orderId) ids.add(orderId);
  });
  return [...ids];
}

export function clearCheckoutAttemptForOrder({ userId, orderId, storage } = {}) {
  const normalizedOrderId = normalizedText(orderId);
  if (!normalizedOrderId) return;
  const session = resolveSessionStorage(storage);
  const key = checkoutAttemptStorageKey(userId);
  const attempts = checkoutAttemptsFor(session, key).filter(
    (attempt) => normalizedText(attempt?.orderId ?? attempt?.order_id) !== normalizedOrderId
  );
  persistCheckoutAttempts(session, key, attempts);
}

export function checkoutIntentFingerprint(intent = {}) {
  const items = Array.isArray(intent.items)
    ? intent.items
        .map((item) => ({
          productId: normalizedText(item?.productId ?? item?.product_id ?? item?.product?.id),
          quantity: normalizedQuantity(item?.quantity)
        }))
        .filter((item) => item.productId)
        .sort((left, right) => left.productId.localeCompare(right.productId) || left.quantity - right.quantity)
    : [];
  return JSON.stringify({
    mode: normalizedText(intent.mode || "single"),
    items,
    couponCode: normalizedText(intent.couponCode ?? intent.coupon_code).toUpperCase(),
    receiver: normalizedText(intent.receiver),
    phone: normalizedText(intent.phone),
    address: normalizedText(intent.address)
  });
}

function checkoutAttemptStorageKey(userId) {
  return `${CHECKOUT_ATTEMPT_STORAGE_PREFIX}:${encodeURIComponent(normalizedText(userId) || "current")}`;
}

function readCheckoutAttempts(storage, key) {
  if (!storage) return [];
  try {
    const raw = storage.getItem(key);
    if (!raw) return [];
    const record = JSON.parse(raw);
    if (Array.isArray(record?.attempts)) return record.attempts.filter(validCheckoutAttempt);
    return validCheckoutAttempt(record) ? [record] : [];
  } catch {
    return [];
  }
}

function persistCheckoutAttempts(storage, key, attempts) {
  setFallbackCheckoutAttempts(storage, key, attempts);
  if (attempts.length === 0) {
    if (!storage) return;
    try {
      storage.removeItem(key);
    } catch {
      try {
        storage.setItem(key, JSON.stringify({ attempts: [] }));
      } catch {
        // Storage cleanup is best effort only.
      }
    }
    return;
  }
  if (!storage) {
    return;
  }
  try {
    storage.setItem(key, JSON.stringify({ attempts }));
  } catch {
    // The in-memory copy remains available for retries in this page session.
  }
}

function checkoutAttemptsFor(storage, key) {
  const fallback = fallbackCheckoutAttemptsFor(storage, key);
  if (fallback) {
    return fallback;
  }
  return readCheckoutAttempts(storage, key);
}

function fallbackCheckoutAttemptsFor(storage, key) {
  const attempts = storage ? fallbackCheckoutAttempts.get(storage)?.get(key) : unavailableStorageAttempts.get(key);
  return attempts || null;
}

function setFallbackCheckoutAttempts(storage, key, attempts) {
  if (!storage) {
    unavailableStorageAttempts.set(key, attempts);
    return;
  }
  let records = fallbackCheckoutAttempts.get(storage);
  if (!records) {
    records = new Map();
    fallbackCheckoutAttempts.set(storage, records);
  }
  records.set(key, attempts);
}

function validCheckoutAttempt(attempt) {
  return typeof attempt?.fingerprint === "string" && typeof attempt?.idempotencyKey === "string" && Boolean(attempt.idempotencyKey);
}

function resolveSessionStorage(storage) {
  if (storage) return storage;
  try {
    return globalThis.sessionStorage || null;
  } catch {
    return null;
  }
}

function normalizedQuantity(value) {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? Math.trunc(numeric) : 0;
}

function normalizedText(value) {
  return String(value ?? "").trim();
}

function randomKeySuffix() {
  return globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}
