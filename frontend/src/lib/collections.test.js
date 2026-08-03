import assert from "node:assert/strict";
import test from "node:test";

import { collectionEntityKey, collectionItemEntity, collectionItemKey, collectionMembership, collectionPostKey } from "./collections.js";

test("collection entity helpers preserve browser-unsafe ids as strings", () => {
  const item = { entity: { entity_type: "ARTICLE", entity_id: "9007199254740999" } };

  assert.deepEqual(collectionItemEntity(item), { entityType: "article", entityId: "9007199254740999" });
  assert.equal(collectionItemKey(item), "article:9007199254740999");
  assert.equal(collectionPostKey({ kind: "article", id: "9007199254740999" }), "article:9007199254740999");
});

test("collection membership normalizes camel-case entities and removes duplicates", () => {
  const membership = collectionMembership([
    { entity: { entityType: "topic", entityId: "42" } },
    { entity: { entity_type: "TOPIC", entity_id: 42 } },
    { entity: { entity_type: "comment", entity_id: 0 } }
  ]);

  assert.deepEqual([...membership], ["topic:42"]);
  assert.equal(collectionEntityKey("", "42"), "");
});
