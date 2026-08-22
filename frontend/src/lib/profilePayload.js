import { authToPerson, normalizeProfileTheme } from "./postMappers.js";

export function profileFormFromUser(user = {}) {
  return {
    nickname: user?.nickname || "",
    avatar_url: user?.avatar_url || user?.avatarUrl || "",
    background_url: user?.background_url || user?.backgroundUrl || "",
    profile_theme: normalizeProfileTheme(user?.profile_theme || user?.profileTheme || "default"),
    bio: user?.bio || "",
    birthday: user?.birthday || "",
    following_visibility: user?.following_visibility || user?.followingVisibility || "public",
    followers_visibility: user?.followers_visibility || user?.followersVisibility || "public"
  };
}

export function profileFormFromAuth(auth = {}) {
  const form = profileFormFromUser(auth?.user);
  const appearance = authToPerson(auth);
  return {
    ...form,
    background_url: appearance.backgroundUrl,
    profile_theme: appearance.profileTheme
  };
}

export function buildProfileUpdatePayload(form = {}, currentUser = {}) {
  const nextTheme = normalizeProfileTheme(form.profile_theme);
  const currentTheme = normalizeProfileTheme(currentUser.profile_theme || currentUser.profileTheme);
  const payload = {
    nickname: form.nickname,
    avatar_url: form.avatar_url,
    background_url: form.background_url,
    bio: form.bio,
    birthday: String(form.birthday || "").trim() || null,
    following_visibility: form.following_visibility || "public",
    followers_visibility: form.followers_visibility || "public"
  };
  if (nextTheme !== currentTheme) {
    payload.profile_theme = nextTheme;
  }
  return payload;
}
