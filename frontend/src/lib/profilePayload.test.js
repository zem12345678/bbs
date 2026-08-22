import assert from "node:assert/strict";
import test from "node:test";

import { buildProfileUpdatePayload, profileFormFromAuth, profileFormFromUser } from "./profilePayload.js";

test("profileFormFromAuth hides cached protected appearance until it is revalidated", () => {
  const auth = {
    user: {
      id: 42,
      username: "alice",
      nickname: "Alice",
      avatar_url: "https://example.com/avatar.png",
      background_url: "https://example.com/revoked-background.webp",
      profile_theme: "theme-pro",
      bio: "Building"
    }
  };

  assert.deepEqual(profileFormFromAuth(auth), {
    nickname: "Alice",
    avatar_url: "https://example.com/avatar.png",
      background_url: "",
      profile_theme: "default",
      bio: "Building",
      birthday: "",
      following_visibility: "public",
      followers_visibility: "public"
  });
  assert.deepEqual(profileFormFromUser(auth.user), {
    nickname: "Alice",
    avatar_url: "https://example.com/avatar.png",
      background_url: "https://example.com/revoked-background.webp",
      profile_theme: "theme-pro",
      bio: "Building",
      birthday: "",
      following_visibility: "public",
      followers_visibility: "public"
  });
});

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
      bio: "Building",
      birthday: null,
      following_visibility: "public",
      followers_visibility: "public"
    }
  );
});

test("buildProfileUpdatePayload includes profile theme when switching to pro", () => {
  assert.equal(buildProfileUpdatePayload({ profile_theme: " theme-pro " }, { profileTheme: "default" }).profile_theme, "theme-pro");
});

test("buildProfileUpdatePayload includes profile theme when switching back to default", () => {
  assert.equal(buildProfileUpdatePayload({ profile_theme: "default" }, { profile_theme: "theme-pro" }).profile_theme, "default");
});
