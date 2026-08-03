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
    active: true,
    startsAt: 0,
    endsAt: 0,
    updatedAt: 12
  });
  assert.equal(items.length, 1);
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
