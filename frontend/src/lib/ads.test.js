import assert from "node:assert/strict";
import { test } from "node:test";

import { normalizeAd, selectSidebarAd, stableAdRoll } from "./ads.js";

function ad(overrides = {}) {
  return {
    id: "10",
    url: "https://example.com/campaign",
    imageUrl: "https://cdn.example.com/ad.png",
    place: "vertical",
    ratio: 1,
    dayOfWeek: 0,
    ...overrides
  };
}

test("normalizes public ad fields and rejects unsafe creatives", () => {
  assert.deepEqual(normalizeAd(ad({ id: 9223372036854775807n })), {
    id: "9223372036854775807",
    url: "https://example.com/campaign",
    imageUrl: "https://cdn.example.com/ad.png",
    place: "vertical",
    ratio: 1,
    dayOfWeek: 0
  });
  assert.equal(normalizeAd(ad({ url: "javascript:alert(1)" })), null);
  assert.equal(normalizeAd(ad({ imageUrl: "data:image/png;base64,unsafe" })), null);
  assert.equal(normalizeAd(ad({ ratio: -1 })), null);
  assert.equal(normalizeAd(ad({ dayOfWeek: 128 })), null);
});

test("filters sidebar ads by vertical placement and UTC weekday", () => {
  const monday = new Date("2026-08-10T00:30:00.000Z");
  const selected = selectSidebarAd(
    [
      ad({ id: "1", place: "horizontal", dayOfWeek: 1 << 1 }),
      ad({ id: "2", dayOfWeek: 1 << 0 }),
      ad({ id: "3", dayOfWeek: 1 << 1 })
    ],
    { now: monday, random: () => 0 }
  );

  assert.equal(selected?.id, "3");
});

test("selects positive-ratio ads by injected weighted roll", () => {
  const values = [ad({ id: "1", ratio: 1 }), ad({ id: "2", ratio: 3 }), ad({ id: "3", ratio: 0 })];

  assert.equal(selectSidebarAd(values, { random: () => 0 })?.id, "1");
  assert.equal(selectSidebarAd(values, { random: () => 0.25 })?.id, "2");
  assert.equal(selectSidebarAd(values, { random: () => 0.99 })?.id, "2");
});

test("uses a stable daily path roll and falls back when all ratios are zero", () => {
  const now = new Date("2026-08-10T12:00:00.000Z");
  const first = stableAdRoll(now, "/plaza");

  assert.equal(first, stableAdRoll(new Date("2026-08-10T23:59:59.000Z"), "/plaza"));
  assert.notEqual(first, stableAdRoll(now, "/topics"));
  assert.equal(selectSidebarAd([ad({ id: "20", ratio: 0 }), ad({ id: "10", ratio: 0 })], { now, pathname: "/plaza" })?.id, "10");
});
