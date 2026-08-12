import assert from "node:assert/strict";
import test from "node:test";

import {
  COMMENT_AUTHOR_BATCH_SIZE,
  collectMissingCommentAuthorIDs,
  loadCommentAuthors
} from "./commentAuthors.js";

test("collectMissingCommentAuthorIDs excludes self and cached authors", () => {
  const ids = collectMissingCommentAuthorIDs({
    comments: [{ author_id: 1 }, { authorId: 2 }, { author_id: 2 }, { author_id: 0 }, { author_id: "invalid" }],
    replyState: {
      "10": { items: [{ author_id: 3 }, { author_id: 4 }] }
    },
    conversationState: {
      "12": { items: [{ author_id: 5 }, { author_id: 4 }] }
    },
    knownAuthors: { "3": { id: 3 } },
    currentUserID: 1
  });

  assert.deepEqual(ids, ["2", "4", "5"]);
});

test("loadCommentAuthors batches IDs and bounds concurrent requests", async () => {
  const ids = Array.from({ length: COMMENT_AUTHOR_BATCH_SIZE * 4 + 1 }, (_, index) => String(index + 1));
  const calls = [];
  let active = 0;
  let maxActive = 0;
  const users = await loadCommentAuthors(ids, async (batch) => {
    calls.push(batch);
    active += 1;
    maxActive = Math.max(maxActive, active);
    await new Promise((resolve) => setTimeout(resolve, 5));
    active -= 1;
    return { items: batch.map((id) => ({ id })) };
  });

  assert.deepEqual(calls.map((batch) => batch.length), [100, 100, 100, 100, 1]);
  assert.equal(maxActive, 4);
  assert.deepEqual(users.map((user) => user.id), ids);
});
