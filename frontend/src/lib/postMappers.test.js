import assert from "node:assert/strict";
import test from "node:test";

import { topicToPost } from "./postMappers.js";

test("topicToPost preserves QA metadata", () => {
  const post = topicToPost({
    id: 101,
    type: "qa",
    title: "如何定位支付回调问题？",
    body: "已经检查了网关日志，还需要确认消息投递。",
    author_id: 10,
    created_at: 1783896000000,
    bounty_score: 50,
    qa_status: "open",
    accepted_comment_id: 0
  });

  assert.equal(post.topicType, "qa");
  assert.equal(post.level, "问答");
  assert.equal(post.bountyScore, 50);
  assert.equal(post.qaStatus, "open");
  assert.equal(post.acceptedCommentId, 0);
});
