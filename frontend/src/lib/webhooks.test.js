import assert from "node:assert/strict";
import test from "node:test";

import { normalizeWebhook, normalizeWebhookList, validWebhookURL, webhookEventLabel, webhookTime } from "./webhooks.js";

test("normalizes wrapped and direct webhook responses without retaining secrets", () => {
  const normalized = normalizeWebhook({ webhook: {
    id: "9223372036854775807",
    name: "  发布通知  ",
    url: " https://hooks.example.test/bbs ",
    secret: "must-not-reach-page-state",
    on: ["reply", "note", "reply", "unknown"],
    active: false,
    latest_sent_at: 1_800_000_000,
    latest_status: 204,
    created_at: 1_700_000_000,
    updated_at: 1_800_000_001
  } });

  assert.deepEqual(normalized, {
    id: "9223372036854775807",
    name: "发布通知",
    url: "https://hooks.example.test/bbs",
    events: ["reply", "note"],
    active: false,
    latestSentAt: 1_800_000_000,
    latestStatus: 204,
    createdAt: 1_700_000_000,
    updatedAt: 1_800_000_001
  });
  assert.equal("secret" in normalized, false);

  assert.deepEqual(normalizeWebhookList([normalized]), { items: [normalized], total: 1 });
  assert.equal(normalizeWebhookList({ items: [{ id: "hook-1", events: ["followed"] }], total: 4 }).total, 4);
  assert.deepEqual(normalizeWebhookList(null), { items: [], total: 0 });
});

test("labels webhook events and delivery times", () => {
  assert.equal(webhookEventLabel("followed"), "被关注");
  assert.equal(webhookEventLabel("custom"), "custom");
  assert.match(webhookTime(1_800_000_000), /2027/);
  assert.match(webhookTime("2027-01-15T08:00:00Z"), /2027/);
  assert.equal(normalizeWebhook({ id: "hook-2", latestSentAt: "2027-01-15T08:00:00Z" }).latestSentAt, 1_800_000_000);
  assert.equal(webhookTime(0), "暂无");
});

test("accepts HTTPS and limits plain HTTP to loopback development receivers", () => {
  assert.equal(validWebhookURL("https://hooks.example.test/bbs"), true);
  assert.equal(validWebhookURL("http://127.0.0.1:39090/hook"), true);
  assert.equal(validWebhookURL("http://localhost:39090/hook"), true);
  assert.equal(validWebhookURL("http://hooks.example.test/bbs"), false);
  assert.equal(validWebhookURL("file:///tmp/hook"), false);
});
