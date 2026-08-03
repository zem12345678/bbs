import assert from "node:assert/strict";
import test from "node:test";

import { appendUniqueUserLists, normalizeUserList, userListOwnedBy, validateUserListName } from "./userLists.js";

test("normalizes user-list API fields without losing int64 ids", () => {
  const list = normalizeUserList({
    id: "9223372036854775000",
    owner_id: "9223372036854774000",
    name: "  Editors  ",
    is_public: true,
    member_count: 4,
    favorite_count: 2,
    is_favorited: true
  });

  assert.equal(list.id, "9223372036854775000");
  assert.equal(list.ownerId, "9223372036854774000");
  assert.equal(list.name, "Editors");
  assert.equal(list.memberCount, 4);
  assert.equal(list.favoriteCount, 2);
  assert.equal(list.isFavorited, true);
});

test("validates trimmed list names by Unicode character count", () => {
  assert.deepEqual(validateUserListName("  Team  "), { name: "Team", error: "" });
  assert.ok(validateUserListName("").error);
  assert.ok(validateUserListName("列".repeat(101)).error);
});

test("detects list ownership and appends pages without duplicates", () => {
  assert.equal(userListOwnedBy({ ownerId: "42" }, 42), true);
  assert.deepEqual(appendUniqueUserLists([{ id: "1" }], [{ id: 1 }, { id: "2" }]).map((item) => String(item.id)), ["1", "2"]);
});
