import assert from "node:assert/strict";
import { test } from "node:test";

import { shareLink } from "./share.js";

test("shareLink uses the native share sheet when available", async () => {
  const calls = [];
  const result = await shareLink("https://community.test/topic/42", {
    title: "Topic title",
    navigator: {
      async share(payload) {
        calls.push(payload);
      }
    }
  });

  assert.deepEqual(result, { status: "shared", message: "已打开系统分享。" });
  assert.deepEqual(calls, [{ title: "Topic title", url: "https://community.test/topic/42" }]);
});

test("shareLink copies the link after a non-cancellation native share failure", async () => {
  const copied = [];
  const result = await shareLink("https://community.test/article/7", {
    navigator: {
      async share() {
        throw new Error("share blocked");
      },
      clipboard: {
        async writeText(value) {
          copied.push(value);
        }
      }
    }
  });

  assert.deepEqual(result, { status: "copied", message: "链接已复制。" });
  assert.deepEqual(copied, ["https://community.test/article/7"]);
});

test("shareLink leaves the page unchanged when the native share sheet is cancelled", async () => {
  let copied = false;
  const result = await shareLink("https://community.test/topic/42", {
    navigator: {
      async share() {
        const error = new Error("cancelled");
        error.name = "AbortError";
        throw error;
      },
      clipboard: {
        async writeText() {
          copied = true;
        }
      }
    }
  });

  assert.deepEqual(result, { status: "cancelled", message: "" });
  assert.equal(copied, false);
});

test("shareLink returns a manual URL when copying is unavailable", async () => {
  const result = await shareLink("https://community.test/topic/42", { navigator: {} });

  assert.deepEqual(result, { status: "manual", message: "https://community.test/topic/42" });
});
