import assert from "node:assert/strict";
import { test } from "node:test";

import { safeExternalURL } from "./externalLinks.js";

test("safeExternalURL returns normalized HTTP(S) links", () => {
  assert.equal(safeExternalURL("  https://docs.example.com/guides?lang=zh  "), "https://docs.example.com/guides?lang=zh");
  assert.equal(safeExternalURL("HTTP://EXAMPLE.COM"), "http://example.com/");
});

test("safeExternalURL rejects unsafe or incomplete links", () => {
  [
    "javascript:alert(1)",
    "data:text/html,unsafe",
    "ftp://files.example.com/manual.pdf",
    "https:docs.example.com",
    "/docs/getting-started",
    "https:///docs",
    "https://admin:secret@example.com/private"
  ].forEach((value) => assert.equal(safeExternalURL(value), "", value));
});
