import assert from "node:assert/strict";
import test from "node:test";
import { resolveRegistrationMode } from "./registrationMode.ts";

test("resolveRegistrationMode accepts explicit supported modes", () => {
  assert.equal(resolveRegistrationMode(" OPEN ", false), "open");
  assert.equal(resolveRegistrationMode("invite_only", true), "invite_only");
  assert.equal(resolveRegistrationMode("closed", true), "closed");
});

test("resolveRegistrationMode only uses the legacy flag when mode is empty", () => {
  assert.equal(resolveRegistrationMode("", true), "open");
  assert.equal(resolveRegistrationMode(undefined, false), "closed");
});

test("resolveRegistrationMode fails closed for an invalid explicit mode", () => {
  assert.equal(resolveRegistrationMode("unexpected", true), "closed");
});
