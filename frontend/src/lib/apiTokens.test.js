import assert from "node:assert/strict";
import test from "node:test";

import { apiTokenScopeLabel, apiTokenStatus, apiTokenStatusLabel, apiTokenTime, normalizeAPITokenCreation, normalizeAPITokenList } from "./apiTokens.js";

test("normalizes API token metadata and creation responses", () => {
  assert.deepEqual(normalizeAPITokenList({ items: [{ id: "tok-1", name: "Deploy", scopes: ["write", "read", "other"], created_at: 1_800_000_000, expires_at: 1_800_086_400, active: true }], total: 1 }), {
    items: [{ id: "tok-1", name: "Deploy", scopes: ["write", "read"], createdAt: 1_800_000_000, expiresAt: 1_800_086_400, revokedAt: 0, credentialValid: true, active: true }], total: 1
  });
  assert.equal(normalizeAPITokenCreation({ token: "secret", id: "tok-2", api_token: { id: "tok-2", name: "CLI", scopes: ["read"] } }).token, "secret");
  assert.equal(normalizeAPITokenList(null).total, 0);
});

test("derives API token status and labels", () => {
  const now = 1_800_000_000;
  assert.equal(apiTokenStatus({ expiresAt: now + 1 }, now), "active");
  assert.equal(apiTokenStatus({ expiresAt: now - 1 }, now), "expired");
  assert.equal(apiTokenStatus({ revokedAt: now }, now), "revoked");
  assert.equal(apiTokenStatus({ credentialValid: false }, now), "invalid");
  assert.equal(apiTokenStatusLabel("active"), "有效");
  assert.equal(apiTokenScopeLabel("write"), "写入");
  assert.match(apiTokenTime(now), /2027/);
});
