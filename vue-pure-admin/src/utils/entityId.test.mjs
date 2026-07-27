import assert from "node:assert/strict";
import test from "node:test";
import { normalizeDecimalEntityId, normalizeEntityId } from "./entityId.ts";

test("normalizeEntityId preserves Snowflake IDs as strings", () => {
  const id = "339000000000000013";

  assert.equal(normalizeEntityId(id), id);
});

test("normalizeDecimalEntityId keeps precise int64 query values", () => {
  const id = "9007199254740993";

  assert.equal(normalizeDecimalEntityId(`  ${id}  `), id);
  assert.equal(normalizeDecimalEntityId("0"), undefined);
  assert.equal(normalizeDecimalEntityId("9223372036854775808"), undefined);
});
