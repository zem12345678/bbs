const THEME_STORAGE_KEY = "bbs:theme";
export const THEME_CHANGED_EVENT = "bbs:theme-changed";

export function currentTheme() {
  if (typeof document === "undefined") {
    return "light";
  }
  return document.documentElement.dataset.theme === "dark" ? "dark" : "light";
}

export function applyTheme(theme) {
  if (typeof document === "undefined") {
    return "light";
  }
  const next = theme === "dark" ? "dark" : "light";
  document.documentElement.dataset.theme = next;
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, next);
  } catch (error) {
    // localStorage 不可用时仅保持本次会话主题
  }
  window.dispatchEvent(new CustomEvent(THEME_CHANGED_EVENT, { detail: { theme: next } }));
  return next;
}

export function toggleTheme() {
  return applyTheme(currentTheme() === "dark" ? "light" : "dark");
}

/** 订阅主题变化，返回取消订阅函数 */
export function subscribeTheme(listener) {
  if (typeof window === "undefined") {
    return () => {};
  }
  const handler = (event) => listener(event?.detail?.theme === "dark" ? "dark" : "light");
  window.addEventListener(THEME_CHANGED_EVENT, handler);
  return () => window.removeEventListener(THEME_CHANGED_EVENT, handler);
}
