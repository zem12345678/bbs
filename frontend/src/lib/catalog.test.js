import assert from "node:assert/strict";
import test from "node:test";
import { normalizeHashtagsResponse } from "./catalog.js";

test("normalizes hashtag trend responses and strips hash prefixes", () => {
  assert.deepEqual(
    normalizeHashtagsResponse({
      items: [
        { tag: "#Go", count: 4 },
        { name: "react", mentionedUsersCount: 3 },
        { tag: "", count: 9 }
      ]
    }),
    [
      { name: "Go", count: 4 },
      { name: "react", count: 3 }
    ]
  );
});
