import assert from "node:assert/strict";
import test from "node:test";

import {
  base64URLToBytes,
  bytesToBase64URL,
  creationOptionsFromResponse,
  normalizePasskeyList,
  publicKeyCredentialJSON,
  requestOptionsFromResponse
} from "./passkeys.js";

test("converts WebAuthn option byte fields without changing other server policy", () => {
  const creation = creationOptionsFromResponse({ options: { publicKey: {
    challenge: "AQID", user: { id: "BAUG", name: "alice" },
    excludeCredentials: [{ type: "public-key", id: "BwgJ", transports: ["internal"] }],
    authenticatorSelection: { residentKey: "required", userVerification: "required" }
  } } });
  assert.deepEqual([...creation.challenge], [1, 2, 3]);
  assert.deepEqual([...creation.user.id], [4, 5, 6]);
  assert.deepEqual([...creation.excludeCredentials[0].id], [7, 8, 9]);
  assert.equal(creation.authenticatorSelection.residentKey, "required");

  const request = requestOptionsFromResponse({ options: { publicKey: { challenge: "AQID", allowCredentials: [{ id: "BAUG", type: "public-key" }] } } });
  assert.deepEqual([...request.allowCredentials[0].id], [4, 5, 6]);
});

test("serializes fallback assertion credentials as base64url JSON", () => {
  const credential = publicKeyCredentialJSON({
    id: "credential",
    rawId: Uint8Array.from([1, 2, 3]).buffer,
    type: "public-key",
    getClientExtensionResults: () => ({ appid: false }),
    response: {
      clientDataJSON: Uint8Array.from([4]).buffer,
      authenticatorData: Uint8Array.from([5]).buffer,
      signature: Uint8Array.from([6]).buffer,
      userHandle: Uint8Array.from([7]).buffer
    }
  });
  assert.equal(credential.rawId, "AQID");
  assert.deepEqual(credential.response, { clientDataJSON: "BA", authenticatorData: "BQ", signature: "Bg", userHandle: "Bw" });
  assert.deepEqual([...base64URLToBytes(bytesToBase64URL(Uint8Array.from([250, 251, 252])))], [250, 251, 252]);
});

test("normalizes passkey list metadata and quoted credential IDs", () => {
  assert.deepEqual(normalizePasskeyList({
    items: [{ credential_id: "cred-id", name: "Phone", backup_eligible: true, created_at: 123 }],
    passwordless_enabled: true,
    limit: 20
  }), {
    items: [{ credentialId: "cred-id", name: "Phone", backupEligible: true, backupState: false, createdAt: 123, updatedAt: 0, lastUsedAt: 0 }],
    passwordlessEnabled: true,
    limit: 20
  });
});
