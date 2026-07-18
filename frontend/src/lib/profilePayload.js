import { authToPerson, normalizeProfileTheme } from "./postMappers.js";

export function profileFormFromUser(user = {}) {
  return {
    nickname: user?.nickname || "",
    avatar_url: user?.avatar_url || user?.avatarUrl || "",
    background_url: user?.background_url || user?.backgroundUrl || "",
    profile_theme: normalizeProfileTheme(user?.profile_theme || user?.profileTheme || "default"),
    bio: user?.bio || ""
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
    bio: form.bio
  };
  if (nextTheme !== currentTheme) {
    payload.profile_theme = nextTheme;
  }
  return payload;
}
