import assert from "node:assert/strict";
import test from "node:test";

import { emptyPollDraft, pollChoicePercent, pollDraftFromApi, pollPayloadFromDraft } from "./topicPoll.js";

test("normalizes API poll data into an editor draft", () => {
  const draft = pollDraftFromApi({ multiple: true, choices: [{ text: "A" }, { text: "B" }], expires_at: 1893456000000 });

  assert.equal(draft.enabled, true);
  assert.equal(draft.multiple, true);
  assert.deepEqual(draft.choices, ["A", "B"]);
  assert.match(draft.expires_at, /^2030-01-01T\d{2}:\d{2}$/);
  assert.equal(draft.dirty, false);
});

test("builds a trimmed poll payload", () => {
  const result = pollPayloadFromDraft({ ...emptyPollDraft(), enabled: true, multiple: true, choices: [" First ", "Second"] }, 1000);

  assert.equal(result.error, "");
  assert.deepEqual(result.payload, { enabled: true, multiple: true, choices: ["First", "Second"], expires_at: undefined });
});

test("rejects duplicate and expired poll settings", () => {
  assert.match(pollPayloadFromDraft({ enabled: true, choices: ["One", " one "] }).error, /不能重复/);
  assert.match(pollPayloadFromDraft({ enabled: true, choices: ["One", "Two"], expires_at: "2020-01-01T00:00" }, Date.now()).error, /截止时间/);
});

test("calculates voter-based option percentages", () => {
  assert.equal(pollChoicePercent(3, 4), 75);
  assert.equal(pollChoicePercent(1, 0), 0);
});
