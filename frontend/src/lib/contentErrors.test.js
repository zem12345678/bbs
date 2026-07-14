import assert from "node:assert/strict";
import test from "node:test";

import { isMembershipBountyError } from "./contentErrors.js";

test("isMembershipBountyError recognizes gateway membership bounty message", () => {
  assert.equal(
    isMembershipBountyError({
      status: 403,
      message: "membership entitlement required for bounty QA topics"
    }),
    true
  );
});

test("isMembershipBountyError recognizes content-service domain code", () => {
  assert.equal(
    isMembershipBountyError({
      httpCode: 403,
      message: "TOPIC_MEMBERSHIP_ENTITLEMENT_REQUIRED"
    }),
    true
  );
});

test("isMembershipBountyError ignores unrelated permission errors", () => {
  assert.equal(
    isMembershipBountyError({
      status: 403,
      message: "profile theme entitlement required"
    }),
    false
  );
  assert.equal(
    isMembershipBountyError({
      status: 412,
      message: "TOPIC_MEMBERSHIP_ENTITLEMENT_REQUIRED"
    }),
    false
  );
});
