import assert from "node:assert/strict";
import test from "node:test";
import {
  announcementDismissalKey,
  normalizeAnnouncementsResponse,
  visibleAnnouncements
} from "./announcements.js";

test("normalizes public announcements and drops incomplete entries", () => {
  const items = normalizeAnnouncementsResponse({
    items: [
      { id: "launch", title: "上线", content: "欢迎", display: "dialog", icon: "warning", updated_at: 12 },
      { id: "missing-title", text: "忽略" },
      null
    ]
  });

  assert.deepEqual(items[0], {
    id: "launch",
    title: "上线",
    content: "欢迎",
    display: "dialog",
    icon: "warning",
    updated_at: 12,
    text: "欢迎",
    imageUrl: "",
    needConfirmationToRead: false,
    silence: false,
    confetti: false,
    forYou: false,
    isRead: false,
    active: true,
    startsAt: 0,
    endsAt: 0,
    updatedAt: 12
  });
  assert.equal(items.length, 1);
});

test("maps compatibility booleans and RFC3339 timestamps", () => {
  const [item] = normalizeAnnouncementsResponse([
    {
      id: "targeted",
      title: "定向公告",
      text: "正文",
      imageUrl: null,
      needConfirmationToRead: true,
      silence: true,
      confetti: true,
      forYou: true,
      isRead: true,
      isActive: false,
      updatedAt: "2026-08-10T12:00:00Z"
    }
  ]);

  assert.equal(item.needConfirmationToRead, true);
  assert.equal(item.silence, true);
  assert.equal(item.confetti, true);
  assert.equal(item.forYou, true);
  assert.equal(item.isRead, true);
  assert.equal(item.active, false);
  assert.equal(item.updatedAt, Date.parse("2026-08-10T12:00:00Z"));
});

test("filters inactive and scheduled announcements", () => {
  const items = normalizeAnnouncementsResponse([
    { id: "active", title: "A", text: "a" },
    { id: "disabled", title: "B", text: "b", active: false },
    { id: "future", title: "C", text: "c", starts_at: 2000 },
    { id: "expired", title: "D", text: "d", ends_at: 1000 }
  ]);

  assert.deepEqual(visibleAnnouncements(items, 1500).map((item) => item.id), ["active"]);
});

test("dismissal keys change when an announcement is updated", () => {
  assert.equal(announcementDismissalKey({ id: "launch", updatedAt: 42 }), "launch:42");
  assert.notEqual(
    announcementDismissalKey({ id: "launch", updatedAt: 42 }),
    announcementDismissalKey({ id: "launch", updatedAt: 43 })
  );
  assert.notEqual(
    announcementDismissalKey({ id: "launch", title: "旧标题", text: "旧内容" }),
    announcementDismissalKey({ id: "launch", title: "新标题", text: "新内容" })
  );
});
