import assert from "node:assert/strict";
import test from "node:test";

import { bbsApi } from "../api.js";
import { normalizeCategoriesResponse } from "./catalog.js";
import { authProfileAppearanceNeedsVerification, authProfileThemeNeedsVerification, authToPerson, hydratePostsMeta, normalizeProfileTheme, profileThemeClass, tagSearchHitToPost, topicToPost, userToPerson } from "./postMappers.js";

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
  assert.equal(post.acceptedCommentId, "");
});

test("content mappers preserve Snowflake category ids as strings", () => {
  const categoryId = "339000000000000013";
  const acceptedCommentId = "339000000000000015";
  const [category] = normalizeCategoriesResponse({
    items: [{ id: categoryId, name: "新分类", sort: 10 }]
  });
  const post = topicToPost({
    id: "339000000000000099",
    title: "分类精度",
    category_id: categoryId,
    accepted_comment_id: acceptedCommentId
  });

  assert.equal(category.id, categoryId);
  assert.equal(post.categoryId, categoryId);
  assert.equal(post.acceptedCommentId, acceptedCommentId);
});

test("tag search maps mixed article and topic projections", () => {
  const article = tagSearchHitToPost({
    kind: "article",
    article: { id: "9223372036854775807", title: "Long form", author_id: "7", tag_names: ["go"] }
  });
  const topic = tagSearchHitToPost({
    kind: "topic",
    topic: { id: "9223372036854775806", title: "Discussion", author_id: "8", tag_names: ["go"] }
  });

  assert.equal(article.id, "9223372036854775807");
  assert.equal(article.kind, "article");
  assert.deepEqual(article.tags, ["go"]);
  assert.equal(topic.id, "9223372036854775806");
  assert.equal(topic.kind, "topic");
  assert.deepEqual(topic.tags, ["go"]);
});

test("userToPerson preserves supported profile themes", () => {
  const person = userToPerson({
    id: 42,
    username: "alice",
    nickname: "Alice",
    profile_theme: "theme-pro"
  });

  assert.equal(person.username, "alice");
  assert.equal(person.profileTheme, "theme-pro");
  assert.equal(normalizeProfileTheme("unknown-theme"), "default");
  assert.equal(profileThemeClass(person.profileTheme), "profile-theme-pro");
});

test("userToPerson preserves private account approval state", () => {
  const person = userToPerson({
    id: 42,
    username: "alice",
    follow_approval_required: true
  });

  assert.equal(person.followApprovalRequired, true);
});

test("authToPerson hides cached protected profile appearance until gateway verification", () => {
  const auth = {
    user: {
      id: 42,
      username: "alice",
      nickname: "Alice",
      background_url: "https://example.test/member-background.webp",
      profile_theme: "theme-pro"
    }
  };

  assert.equal(authProfileThemeNeedsVerification(auth), true);
  assert.equal(authProfileAppearanceNeedsVerification(auth), true);
  assert.equal(authToPerson(auth).profileTheme, "default");
  assert.equal(authToPerson(auth).background, "");
  assert.equal(authToPerson(auth).backgroundUrl, "");
  assert.equal(authToPerson(auth, { trustAppearance: true }).profileTheme, "theme-pro");
  assert.equal(authToPerson(auth, { trustAppearance: true }).background, "https://example.test/member-background.webp");
});

test("hydratePostsMeta revalidates cached current-user pro themes", async () => {
  const originalGetUsers = bbsApi.getUsers;
  const calls = [];
  bbsApi.getUsers = async (userIds) => {
    calls.push(userIds);
    return {
      items: [{
        id: 42,
        username: "alice",
        nickname: "Alice",
        profile_theme: "default"
      }]
    };
  };

  try {
    const auth = { user: { id: 42, username: "alice", nickname: "Alice", profile_theme: "theme-pro" } };
    const post = topicToPost({ id: 101, title: "已撤销主题缓存", author_id: 42, body: "content", created_at: 1783896000000 }, auth);

    assert.equal(post.author.profileTheme, "default");

    const [hydrated] = await hydratePostsMeta([post], auth, { skipCounts: true });
    assert.deepEqual(calls, [["42"]]);
    assert.equal(hydrated.author.profileTheme, "default");
  } finally {
    bbsApi.getUsers = originalGetUsers;
  }
});

test("hydratePostsMeta revalidates cached current-user membership backgrounds", async () => {
  const originalGetUsers = bbsApi.getUsers;
  const calls = [];
  bbsApi.getUsers = async (userIds) => {
    calls.push(userIds);
    return {
      items: [{
        id: 42,
        username: "alice",
        nickname: "Alice",
        profile_theme: "default",
        background_url: ""
      }]
    };
  };

  try {
    const auth = {
      user: {
        id: 42,
        username: "alice",
        nickname: "Alice",
        profile_theme: "default",
        background_url: "https://example.test/revoked-membership-background.webp"
      }
    };
    const post = topicToPost({ id: 102, title: "已撤销会员背景缓存", author_id: 42, body: "content", created_at: 1783896000000 }, auth);

    assert.equal(post.author.background, "");

    const [hydrated] = await hydratePostsMeta([post], auth, { skipCounts: true });
    assert.deepEqual(calls, [["42"]]);
    assert.equal(hydrated.author.background, "");
  } finally {
    bbsApi.getUsers = originalGetUsers;
  }
});

test("hydratePostsMeta batches deduplicated post authors", async () => {
  const originalGetUser = bbsApi.getUser;
  const originalGetUsers = bbsApi.getUsers;
  const batchCalls = [];
  let detailCalls = 0;
  bbsApi.getUser = async () => {
    detailCalls += 1;
    throw new Error("unexpected user detail request");
  };
  bbsApi.getUsers = async (userIds) => {
    batchCalls.push(userIds);
    return {
      items: userIds.map((id) => ({ id, username: `user-${id}`, nickname: `User ${id}` }))
    };
  };

  try {
    const posts = Array.from({ length: 20 }, (_, index) =>
      topicToPost({
        id: index + 1,
        title: `Topic ${index + 1}`,
        author_id: index === 19 ? 7 : 42,
        body: "content",
        created_at: 1783896000000 + index
      })
    );

    const hydrated = await hydratePostsMeta(posts, null, { skipCounts: true });

    assert.deepEqual(batchCalls, [["42", "7"]]);
    assert.equal(detailCalls, 0);
    assert.equal(hydrated.filter((post) => post.author.name === "User 42").length, 19);
    assert.equal(hydrated[19].author.name, "User 7");
  } finally {
    bbsApi.getUser = originalGetUser;
    bbsApi.getUsers = originalGetUsers;
  }
});
