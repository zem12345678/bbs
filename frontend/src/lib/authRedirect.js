const AUTH_REDIRECT_ORIGIN = "https://bbs.local";

export function safeAuthRedirect(value, fallback = "/user/profile") {
  const candidate = typeof value === "string" ? value.trim() : "";
  if (!candidate.startsWith("/") || candidate.startsWith("//")) return fallback;
  try {
    const target = new URL(candidate, AUTH_REDIRECT_ORIGIN);
    if (target.origin !== AUTH_REDIRECT_ORIGIN) return fallback;
    return `${target.pathname}${target.search}${target.hash}`;
  } catch {
    return fallback;
  }
}

export function authRedirectFromSearch(search, fallback = "/user/profile") {
  return safeAuthRedirect(new URLSearchParams(search).get("redirect"), fallback);
}

export function authInvalidationRedirect(value, fallback = "/user/profile") {
  const target = safeAuthRedirect(value, fallback);
  const pathname = target.split(/[?#]/, 1)[0];
  if (pathname.startsWith("/auth/") || pathname === "/user/signin" || pathname === "/user/signup" || pathname.startsWith("/user/password/")) {
    return "/user/signin";
  }
  return `/user/signin?redirect=${encodeURIComponent(target)}`;
}

export function oauthCallbackURL(callbackHint, redirectTarget) {
  try {
    const callback = new URL(String(callbackHint || "").trim());
    if (!callback.host || callback.username || callback.password || !["http:", "https:"].includes(callback.protocol)) return "";
    callback.searchParams.set("redirect", safeAuthRedirect(redirectTarget));
    return callback.toString();
  } catch {
    return "";
  }
}
