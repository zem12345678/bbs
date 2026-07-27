import assert from "node:assert/strict";
import test from "node:test";

import { pageRoutes, pathToPage } from "./routes.js";

test("maps chat workspace routes to the chat navigation section", () => {
  assert.equal(pageRoutes.find((route) => route.key === "chat")?.path, "/chat");
  assert.equal(pathToPage("/chat"), "聊天室");
  assert.equal(pathToPage("/room/AB12CD3E"), "聊天室");
});

test("maps username profile routes to the member navigation section", () => {
  assert.equal(pathToPage("/u/alice"), "会员");
  assert.equal(pathToPage("/u/alice/articles"), "会员");
});
