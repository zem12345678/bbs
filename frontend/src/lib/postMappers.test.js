import assert from "node:assert/strict";
import test from "node:test";

import { bbsApi } from "../api.js";
import { authProfileAppearanceNeedsVerification, authProfileThemeNeedsVerification, authToPerson, hydratePostsMeta, normalizeProfileTheme, profileThemeClass, topicToPost, userToPerson } from "./postMappers.js";

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

test("userToPerson preserves supported profile themes", () => {
  const person = userToPerson({
    id: 42,
    username: "alice",
    nickname: "Alice",
    profile_theme: "theme-pro"
  });

  assert.equal(person.profileTheme, "theme-pro");
  assert.equal(normalizeProfileTheme("unknown-theme"), "default");
  assert.equal(profileThemeClass(person.profileTheme), "profile-theme-pro");
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
  const originalGetUser = bbsApi.getUser;
  const calls = [];
  bbsApi.getUser = async (userId) => {
    calls.push(userId);
    return {
      user: {
        id: 42,
        username: "alice",
        nickname: "Alice",
        profile_theme: "default"
      }
    };
  };

  try {
    const auth = { user: { id: 42, username: "alice", nickname: "Alice", profile_theme: "theme-pro" } };
    const post = topicToPost({ id: 101, title: "已撤销主题缓存", author_id: 42, body: "content", created_at: 1783896000000 }, auth);

    assert.equal(post.author.profileTheme, "default");

    const [hydrated] = await hydratePostsMeta([post], auth, { skipCounts: true });
    assert.deepEqual(calls.map(String), ["42"]);
    assert.equal(hydrated.author.profileTheme, "default");
  } finally {
    bbsApi.getUser = originalGetUser;
  }
});

test("hydratePostsMeta revalidates cached current-user membership backgrounds", async () => {
  const originalGetUser = bbsApi.getUser;
  const calls = [];
  bbsApi.getUser = async (userId) => {
    calls.push(userId);
    return {
      user: {
        id: 42,
        username: "alice",
        nickname: "Alice",
        profile_theme: "default",
        background_url: ""
      }
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
    assert.deepEqual(calls.map(String), ["42"]);
    assert.equal(hydrated.author.background, "");
  } finally {
    bbsApi.getUser = originalGetUser;
  }
});
