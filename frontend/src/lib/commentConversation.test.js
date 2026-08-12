import assert from "node:assert/strict";
import test from "node:test";

import {
  commentReplyTargets,
  commentRootId,
  conversationItems,
  isNestedReply
} from "./commentConversation.js";

test("keeps the selected reply as parent while grouping updates under its root", () => {
  const nestedReply = { id: "303", root_id: "101", parent_id: "202" };

  assert.deepEqual(commentReplyTargets(nestedReply), { parentId: "303", rootId: "101" });
  assert.equal(commentRootId({ id: "101", root_id: 0 }), "101");
});

test("only treats replies whose direct parent differs from the root as nested", () => {
  assert.equal(isNestedReply({ id: "202", root_id: "101", parent_id: "101" }), false);
  assert.equal(isNestedReply({ id: "303", root_id: "101", parent_id: "202" }), true);
  assert.equal(isNestedReply({ id: "101", root_id: 0, parent_id: 0 }), false);
});

test("displays a direct-parent-first conversation from root to direct parent without mutating it", () => {
  const directParentFirst = [{ id: "202" }, { id: "101" }];

  assert.deepEqual(conversationItems({ items: directParentFirst }).map((item) => item.id), ["101", "202"]);
  assert.deepEqual(directParentFirst.map((item) => item.id), ["202", "101"]);
});
