import assert from "node:assert/strict";
import test from "node:test";
import { buildFrontendUrl } from "./frontendUrl.ts";

test("buildFrontendUrl targets the configured user-facing origin", () => {
  assert.equal(
    buildFrontendUrl(
      "/topic/9007199254740993#comment-7",
      "http://127.0.0.1:8850/"
    ),
    "http://127.0.0.1:8850/topic/9007199254740993#comment-7"
  );
  assert.equal(buildFrontendUrl("article/1", ""), "/article/1");
});
