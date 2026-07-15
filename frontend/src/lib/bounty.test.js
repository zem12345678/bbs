import assert from "node:assert/strict";
import test from "node:test";

import { clampBountyScore, publishedBountyMinimum } from "./bounty.js";

test("publishedBountyMinimum locks positive bounty on published questions", () => {
  assert.equal(publishedBountyMinimum({ isQuestion: true, status: 2, bountyScore: 50 }), 50);
});

test("publishedBountyMinimum does not lock drafts or non-questions", () => {
  assert.equal(publishedBountyMinimum({ isQuestion: true, status: 1, bountyScore: 50 }), 0);
  assert.equal(publishedBountyMinimum({ isQuestion: false, status: 2, bountyScore: 50 }), 0);
  assert.equal(publishedBountyMinimum({ isQuestion: true, status: 2, bountyScore: 0 }), 0);
});

test("clampBountyScore preserves published bounty floor", () => {
  assert.equal(clampBountyScore(0, 50), 50);
  assert.equal(clampBountyScore(80, 50), 80);
  assert.equal(clampBountyScore(-10, 50), 50);
});
