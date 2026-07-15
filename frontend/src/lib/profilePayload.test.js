import assert from "node:assert/strict";
import test from "node:test";

import { buildProfileUpdatePayload } from "./profilePayload.js";

test("buildProfileUpdatePayload omits unchanged profile theme", () => {
  assert.deepEqual(
    buildProfileUpdatePayload(
      {
        nickname: "Alice Dev",
        avatar_url: "https://example.com/avatar.png",
        background_url: "https://example.com/bg.webp",
        profile_theme: "theme-pro",
        bio: "Building"
      },
      { profile_theme: "theme-pro" }
    ),
    {
      nickname: "Alice Dev",
      avatar_url: "https://example.com/avatar.png",
      background_url: "https://example.com/bg.webp",
      bio: "Building"
    }
  );
});

test("buildProfileUpdatePayload includes profile theme when switching to pro", () => {
  assert.equal(buildProfileUpdatePayload({ profile_theme: " theme-pro " }, { profileTheme: "default" }).profile_theme, "theme-pro");
});

test("buildProfileUpdatePayload includes profile theme when switching back to default", () => {
  assert.equal(buildProfileUpdatePayload({ profile_theme: "default" }, { profile_theme: "theme-pro" }).profile_theme, "default");
});
