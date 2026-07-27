import assert from "node:assert/strict";
import test from "node:test";

import { hasSearchResults } from "./searchResults.js";

test("user-only search results remain pageable", () => {
  assert.equal(hasSearchResults([], [{ id: "9007199254740993" }]), true);
  assert.equal(hasSearchResults([{ id: "1" }], []), true);
  assert.equal(hasSearchResults([], []), false);
});
