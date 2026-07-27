import assert from "node:assert/strict";
import test from "node:test";

import {
  attachmentPriceCredits,
  authorizedAttachmentIDsFromDownloads,
  markPaidAttachmentAcquired,
  needsPaidAttachmentConfirmation,
  paidAttachmentAcquisitionsStorageKey,
  persistPaidAttachmentAcquisitionIDs,
  readPaidAttachmentAcquisitionIDs
} from "./attachmentPurchase.js";

test("server-authorized download history is the cross-device acquisition source", () => {
  assert.deepEqual(
    authorizedAttachmentIDsFromDownloads([
      { status: "AUTHORIZED", attachment: { id: "42" } },
      { status: "authorized", attachment: { id: 7 } },
      { status: "PENDING", attachment: { id: "8" } },
      { status: "AUTHORIZED", attachment: { id: "not-an-id" } },
      { status: "AUTHORIZED", attachment: null }
    ]),
    { 7: true, 42: true }
  );
});

test("paid attachments require confirmation until acquired by a non-owner", () => {
  const attachment = { id: "42", price_credits: 8 };

  assert.equal(needsPaidAttachmentConfirmation(attachment, false, false), true);
  assert.equal(needsPaidAttachmentConfirmation(attachment, false, true), false);
  assert.equal(needsPaidAttachmentConfirmation(attachment, true, false), false);
});

test("free and invalid attachment prices do not require payment confirmation", () => {
  assert.equal(needsPaidAttachmentConfirmation({ priceCredits: "0" }, false, false), false);
  assert.equal(needsPaidAttachmentConfirmation({ price_credits: -1 }, false, false), false);
  assert.equal(needsPaidAttachmentConfirmation({ price_credits: "not-a-number" }, false, false), false);
  assert.equal(attachmentPriceCredits({ priceCredits: "7" }), 7);
});

test("a successful paid download marks only that attachment as acquired", () => {
  const acquired = markPaidAttachmentAcquired({}, { id: "42", price_credits: 8 }, false);

  assert.deepEqual(acquired, { 42: true });
  assert.equal(needsPaidAttachmentConfirmation({ id: "42", price_credits: 8 }, false, acquired["42"]), false);
  assert.deepEqual(markPaidAttachmentAcquired(acquired, { id: "7", price_credits: 0 }, false), acquired);
  assert.deepEqual(markPaidAttachmentAcquired(acquired, { id: "8", price_credits: 8 }, true), acquired);
});

test("paid attachment acquisition hints persist per user", () => {
  const values = new Map();
  const storage = {
    getItem: (key) => values.get(key) || null,
    setItem: (key, value) => values.set(key, value)
  };

  const userId = "336853987166789633";
  persistPaidAttachmentAcquisitionIDs(userId, { 7: true, 8: false }, storage);

  assert.equal(paidAttachmentAcquisitionsStorageKey(userId), "bbs.community.paid-attachment-acquisitions.v1:336853987166789633");
  assert.deepEqual(readPaidAttachmentAcquisitionIDs(userId, storage), { 7: true });
  assert.deepEqual(readPaidAttachmentAcquisitionIDs("336853987166789634", storage), {});
  assert.equal(paidAttachmentAcquisitionsStorageKey(Number(userId)), "");
});

test("paid attachment acquisition hints tolerate damaged or unavailable storage", () => {
  const userId = "42";
  const values = new Map([[paidAttachmentAcquisitionsStorageKey(userId), "not-json"]]);
  const damagedStorage = {
    getItem: (key) => values.get(key) || null,
    setItem: (key, value) => values.set(key, value)
  };
  const unavailableStorage = {
    getItem: () => {
      throw new Error("storage unavailable");
    },
    setItem: () => {
      throw new Error("storage unavailable");
    }
  };

  assert.deepEqual(readPaidAttachmentAcquisitionIDs(userId, damagedStorage), {});
  values.set(paidAttachmentAcquisitionsStorageKey(userId), '{"__proto__":true,"7":false,"8":1}');
  assert.deepEqual(readPaidAttachmentAcquisitionIDs(userId, damagedStorage), {});
  assert.deepEqual(readPaidAttachmentAcquisitionIDs(userId, unavailableStorage), {});
  assert.doesNotThrow(() => persistPaidAttachmentAcquisitionIDs(userId, { 7: true }, unavailableStorage));
});
