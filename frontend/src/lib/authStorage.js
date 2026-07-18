const AUTH_STORAGE_KEY = "bbs.community.auth";

export function readStoredAuth() {
  try {
    const raw = window.localStorage.getItem(AUTH_STORAGE_KEY);
    return raw ? normalizeAuthResponse(JSON.parse(raw)) : null;
  } catch {
    return null;
  }
}

export function persistAuth(auth) {
  if (!auth) {
    window.localStorage.removeItem(AUTH_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(auth));
}

export function normalizeAuthResponse(data) {
  const accessToken = data?.access_token || data?.accessToken || "";
  const userId = jwtSubject(accessToken);

  return {
    accessToken,
    expiresAt: data?.expires_at || data?.expiresAt || 0,
    user: data?.user ? { ...data.user, ...(userId ? { id: userId } : {}) } : null
  };
}

function jwtSubject(accessToken) {
  const payload = typeof accessToken === "string" ? accessToken.split(".")[1] : "";
  if (!payload || typeof atob !== "function") return "";

  try {
    const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
    const decoded = atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, "="));
    const bytes = Uint8Array.from(decoded, (character) => character.charCodeAt(0));
    const subject = JSON.parse(new TextDecoder().decode(bytes)).sub;
    return typeof subject === "string" ? subject : "";
  } catch {
    return "";
  }
}
