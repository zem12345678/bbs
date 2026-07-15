import assert from "node:assert/strict";
import test from "node:test";

import { membershipBountyGateState } from "./membershipBountyGate.js";

test("membershipBountyGateState does not block non-bounty questions", () => {
  assert.deepEqual(membershipBountyGateState(false, { loading: true }), { blocked: false, reason: "" });
});

test("membershipBountyGateState waits while membership precheck is unresolved", () => {
  assert.deepEqual(membershipBountyGateState(true, { loading: true }), { blocked: true, reason: "checking" });
  assert.deepEqual(membershipBountyGateState(true, { checked: false, loading: false, error: "" }), {
    blocked: true,
    reason: "checking"
  });
});

test("membershipBountyGateState blocks confirmed missing membership", () => {
  assert.deepEqual(membershipBountyGateState(true, { checked: true, loading: false, active: false, error: "" }), {
    blocked: true,
    reason: "missing"
  });
});

test("membershipBountyGateState lets the server decide after precheck failures", () => {
  assert.deepEqual(membershipBountyGateState(true, { checked: false, loading: false, active: false, error: "network" }), {
    blocked: false,
    reason: "precheck_failed"
  });
});

test("membershipBountyGateState allows confirmed active membership", () => {
  assert.deepEqual(membershipBountyGateState(true, { checked: true, loading: false, active: true, error: "" }), {
    blocked: false,
    reason: "active"
  });
});
