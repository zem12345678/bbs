export function safeExternalURL(value) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!/^https?:\/\/[^/?#\\\s]+/i.test(raw)) return "";

  try {
    const parsed = new URL(raw);
    if ((parsed.protocol !== "http:" && parsed.protocol !== "https:") || !parsed.hostname || parsed.username || parsed.password) {
      return "";
    }
    return parsed.href;
  } catch {
    return "";
  }
}
