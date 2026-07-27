import assert from "node:assert/strict";
import test from "node:test";

import { authInvalidationRedirect, authRedirectFromSearch, oauthCallbackURL, safeAuthRedirect } from "./authRedirect.js";

test("keeps a local chat invitation path including its query and hash", () => {
  assert.equal(safeAuthRedirect("/room/AB12CD3E?source=invite#latest"), "/room/AB12CD3E?source=invite#latest");
  assert.equal(authRedirectFromSearch("?redirect=%2Fchat"), "/chat");
});

test("falls back for absent and external authentication redirects", () => {
  for (const value of ["", "room/AB12CD3E", "https://example.com", "//example.com", "/\\example.com"]) {
    assert.equal(safeAuthRedirect(value), "/user/profile");
  }
});

test("returns to a safe page after session invalidation without leaking auth credentials", () => {
  assert.equal(
    authInvalidationRedirect("/room/AB12CD3E?source=invite#latest"),
    "/user/signin?redirect=%2Froom%2FAB12CD3E%3Fsource%3Dinvite%23latest"
  );
  assert.equal(authInvalidationRedirect("/auth/callback#access_token=secret"), "/user/signin");
  assert.equal(authInvalidationRedirect("/user/password/reset?token=secret"), "/user/signin");
});

test("builds OAuth callbacks from the server-configured endpoint only", () => {
  assert.equal(
    oauthCallbackURL("https://bbs.example.com/auth/callback", "/room/AB12CD3E"),
    "https://bbs.example.com/auth/callback?redirect=%2Froom%2FAB12CD3E"
  );
  assert.equal(oauthCallbackURL("javascript:alert(1)", "/chat"), "");
  assert.equal(oauthCallbackURL("https://user:password@bbs.example.com/auth/callback", "/chat"), "");
});
