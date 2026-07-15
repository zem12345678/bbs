import { normalizeProfileTheme } from "./postMappers.js";

export function buildProfileUpdatePayload(form = {}, currentUser = {}) {
  const nextTheme = normalizeProfileTheme(form.profile_theme);
  const currentTheme = normalizeProfileTheme(currentUser.profile_theme || currentUser.profileTheme);
  const payload = {
    nickname: form.nickname,
    avatar_url: form.avatar_url,
    background_url: form.background_url,
    bio: form.bio
  };
  if (nextTheme !== currentTheme) {
    payload.profile_theme = nextTheme;
  }
  return payload;
}
