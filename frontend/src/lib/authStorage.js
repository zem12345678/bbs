const AUTH_STORAGE_KEY = "bbs.community.auth";

export function readStoredAuth() {
  try {
    const raw = window.localStorage.getItem(AUTH_STORAGE_KEY);
    return raw ? JSON.parse(raw) : null;
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
  return {
    accessToken: data?.access_token || data?.accessToken || "",
    expiresAt: data?.expires_at || data?.expiresAt || 0,
    user: data?.user || null
  };
}
