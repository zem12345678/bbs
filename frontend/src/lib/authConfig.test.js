import assert from "node:assert/strict";
import test from "node:test";
import { normalizeAuthConfig } from "./authConfig.js";

test("auth config keeps legacy registration settings compatible", () => {
  assert.equal(normalizeAuthConfig({ register_enabled: true }).register_mode, "open");
  assert.equal(normalizeAuthConfig({ register_enabled: false }).register_mode, "closed");
});

test("auth config exposes invite-only registration without treating it as closed", () => {
  const config = normalizeAuthConfig({
    password_enabled: true,
    register_enabled: false,
    register_mode: "invite_only"
  });

  assert.equal(config.register_enabled, true);
  assert.equal(config.invite_required, true);
});

test("auth config fails closed for invalid modes and disabled password auth", () => {
  assert.equal(
    normalizeAuthConfig({ register_enabled: true, register_mode: "invalid" })
      .register_enabled,
    false
  );
  assert.equal(
    normalizeAuthConfig({ password_enabled: false, register_mode: "open" })
      .register_enabled,
    false
  );
});
