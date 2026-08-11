import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import {
  buildEmojiLookup,
  captureEmojiSelection,
  clearPublicEmojiCache,
  emojiTextParts,
  insertEmojiToken,
  loadPublicEmojis,
  normalizeEmojiHighlightMarkup,
  PUBLIC_EMOJI_CACHE_TTL_MS,
  normalizePublicEmojisResponse
} from "./emojis.js";

const realDateNow = Date.now;

afterEach(() => {
  Date.now = realDateNow;
  clearPublicEmojiCache();
});

test("normalizes public emoji responses and drops unsafe entries", () => {
  const emojis = normalizePublicEmojisResponse({
    emojis: [
      { name: "party", url: "https://cdn.example.test/party.webp", aliases: ["celebrate", "celebrate", ""] },
      { name: "local", publicUrl: "/uploads/emojis/local.png", category: "常用" },
      { name: "unsafe", url: "javascript:alert(1)" },
      { name: "protocol-relative", url: "//example.test/emoji.png" },
      { name: "backslash-relative", url: "/\\evil.example/emoji.png" },
      { name: "credentials", url: "https://user:password@example.test/emoji.png" },
      { name: "missing-url" }
    ]
  });

  assert.deepEqual(emojis.map((emoji) => emoji.name), ["party", "local"]);
  assert.deepEqual(emojis[0].aliases, ["celebrate"]);
  assert.equal(emojis[1].url, "/uploads/emojis/local.png");
  assert.equal(emojis[1].category, "常用");
});

test("replaces only known emoji names and aliases", () => {
  const emojis = normalizePublicEmojisResponse([
    { name: "party", url: "https://cdn.example.test/party.webp", aliases: ["celebrate"] },
    { name: "早安", url: "https://cdn.example.test/morning.png" }
  ]);
  const lookup = buildEmojiLookup(emojis);
  const parts = emojiTextParts("Hi :PARTY: :celebrate: :早安: :unknown:", lookup);

  assert.deepEqual(parts.map((part) => [part.type, part.value]), [
    ["text", "Hi "],
    ["emoji", ":PARTY:"],
    ["text", " "],
    ["emoji", ":celebrate:"],
    ["text", " "],
    ["emoji", ":早安:"],
    ["text", " :unknown:"]
  ]);
  assert.equal(parts[1].emoji.name, "party");
  assert.equal(parts[3].emoji.name, "party");
});

test("inserts an emoji token at the current selection", () => {
  assert.deepEqual(insertEmojiToken("hello world", "party", 6, 11), {
    value: "hello :party:",
    selection: 13
  });
  assert.deepEqual(insertEmojiToken("hello", "party"), {
    value: "hello:party:",
    selection: 12
  });
  assert.equal(insertEmojiToken("x".repeat(999), "party", 999, 999, 1000), null);
  assert.deepEqual(insertEmojiToken("x".repeat(999), "party", 992, 999, 1000), {
    value: `${"x".repeat(992)}:party:`,
    selection: 999
  });
});

test("keeps highlighted emoji tokens intact for rendering", () => {
  assert.equal(
    normalizeEmojiHighlightMarkup("before :<mark>party</mark>: after <mark>term</mark>"),
    "before <mark>:party:</mark> after <mark>term</mark>"
  );
  assert.equal(normalizeEmojiHighlightMarkup("<mark>:party:</mark>"), "<mark>:party:</mark>");
});

test("keeps the selection captured before the picker rerenders", () => {
  const input = { selectionStart: 1, selectionEnd: 1 };
  const selection = captureEmojiSelection(input);
  input.selectionStart = 2;
  input.selectionEnd = 2;

  assert.deepEqual(insertEmojiToken("AB", "party", selection.start, selection.end), {
    value: "A:party:B",
    selection: 8
  });
});

test("coalesces concurrent public emoji loads and reuses the result", async () => {
  let calls = 0;
  const loader = async () => {
    calls += 1;
    return { emojis: [{ name: "party", url: "https://cdn.example.test/party.webp" }] };
  };

  const [first, second] = await Promise.all([loadPublicEmojis(loader), loadPublicEmojis(loader)]);
  const third = await loadPublicEmojis(loader);

  assert.equal(calls, 1);
  assert.strictEqual(first, second);
  assert.strictEqual(second, third);
});

test("refreshes the public emoji cache after its TTL", async () => {
  let now = 1_000;
  let calls = 0;
  Date.now = () => now;
  const loader = async () => {
    calls += 1;
    return { emojis: [{ name: `emoji-${calls}`, url: `https://cdn.example.test/${calls}.webp` }] };
  };

  const first = await loadPublicEmojis(loader);
  const cached = await loadPublicEmojis(loader);
  now += PUBLIC_EMOJI_CACHE_TTL_MS + 1;
  const refreshed = await loadPublicEmojis(loader);

  assert.equal(calls, 2);
  assert.strictEqual(first, cached);
  assert.equal(refreshed[0].name, "emoji-2");
});
