import assert from "node:assert/strict";
import test from "node:test";

import { mfaChallengeFromResponse, normalizeMFAStatus, recoveryCodesFromResponse, recoveryCodesText } from "./mfa.js";

test("recognizes MFA login challenges without treating ordinary auth as a challenge", () => {
  assert.deepEqual(
    mfaChallengeFromResponse({
      mfa_required: true,
      mfa_challenge: " challenge-token ",
      mfa_expires_at: 1_800_000_000_000,
      user: { id: "42" }
    }),
    { challenge: "challenge-token", expiresAt: 1_800_000_000_000, user: { id: "42" } }
  );
  assert.equal(mfaChallengeFromResponse({ mfa_required: true, mfa_challenge: "" }), null);
  assert.equal(mfaChallengeFromResponse({ access_token: "token" }), null);
});

test("normalizes MFA status and one-time recovery code responses", () => {
  assert.deepEqual(normalizeMFAStatus({ enabled: true, recovery_codes_remaining: 7, enabled_at: 123 }), {
    enabled: true,
    recoveryCodesRemaining: 7,
    enabledAt: 123
  });
  assert.deepEqual(recoveryCodesFromResponse({ recovery_codes: [" first ", "", null, "second"] }), ["first", "second"]);
  assert.equal(recoveryCodesText(["first", "second"]), "first\nsecond");
});
