import assert from "node:assert/strict";
import test from "node:test";

import { normalizeAuthResponse, persistAuth, readStoredAuth } from "./authStorage.js";

const preciseUserId = "336853987166789633";

function tokenWithSubject(subject) {
  const payload = Buffer.from(JSON.stringify({ sub: subject, nickname: "\u6d4b\u8bd5" })).toString("base64url");
  return `header.${payload}.signature`;
}

test("normalizeAuthResponse preserves a Snowflake user ID from the JWT subject", () => {
  const responseUser = { id: Number(preciseUserId), nickname: "\u6d4b\u8bd5" };
  const auth = normalizeAuthResponse({
    access_token: tokenWithSubject(preciseUserId),
    expires_at: 1234,
    user: responseUser
  });

  assert.equal(auth.user.id, preciseUserId);
  assert.notEqual(auth.user.id, String(responseUser.id));
  assert.equal(typeof auth.user.id, "string");
  assert.equal(responseUser.id, Number(preciseUserId));
});

test("readStoredAuth repairs a legacy rounded user ID from its access token", () => {
  const values = new Map();
  const originalWindow = globalThis.window;
  globalThis.window = {
    localStorage: {
      getItem: (key) => values.get(key) || null,
      removeItem: (key) => values.delete(key),
      setItem: (key, value) => values.set(key, value)
    }
  };

  try {
    persistAuth({
      accessToken: tokenWithSubject(preciseUserId),
      expiresAt: 1234,
      user: { id: Number(preciseUserId) }
    });

    assert.equal(readStoredAuth().user.id, preciseUserId);
  } finally {
    globalThis.window = originalWindow;
  }
});

test("persistAuth removes an invalidated session", () => {
  const values = new Map();
  const originalWindow = globalThis.window;
  globalThis.window = {
    localStorage: {
      getItem: (key) => values.get(key) || null,
      removeItem: (key) => values.delete(key),
      setItem: (key, value) => values.set(key, value)
    }
  };

  try {
    persistAuth({ accessToken: tokenWithSubject(preciseUserId), user: { id: preciseUserId } });
    persistAuth(null);

    assert.equal(readStoredAuth(), null);
    assert.equal(values.size, 0);
  } finally {
    globalThis.window = originalWindow;
  }
});

test("drops a stored session without an access token", () => {
  const values = new Map([["bbs.community.auth", JSON.stringify({ user: { id: preciseUserId } })]]);
  const originalWindow = globalThis.window;
  globalThis.window = {
    localStorage: {
      getItem: (key) => values.get(key) || null,
      removeItem: (key) => values.delete(key),
      setItem: (key, value) => values.set(key, value)
    }
  };

  try {
    assert.equal(readStoredAuth(), null);
    assert.equal(values.has("bbs.community.auth"), false);
  } finally {
    globalThis.window = originalWindow;
  }
});

test("tolerates unavailable local storage", () => {
  const originalWindow = globalThis.window;
  globalThis.window = {
    localStorage: {
      getItem: () => {
        throw new Error("unavailable");
      },
      removeItem: () => {
        throw new Error("unavailable");
      },
      setItem: () => {
        throw new Error("unavailable");
      }
    }
  };

  try {
    assert.doesNotThrow(() => persistAuth({ accessToken: tokenWithSubject(preciseUserId), user: { id: preciseUserId } }));
    assert.doesNotThrow(() => persistAuth(null));
    assert.equal(readStoredAuth(), null);
  } finally {
    globalThis.window = originalWindow;
  }
});

test("normalizeAuthResponse keeps the response ID when the access token cannot be decoded", () => {
  const auth = normalizeAuthResponse({ access_token: "not-a-jwt", user: { id: 42 } });

  assert.equal(auth.user.id, 42);
});
