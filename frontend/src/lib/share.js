export async function shareLink(url, options = {}) {
  const navigatorLike = options.navigator ?? globalThis.navigator;
  const payload = { url };
  if (options.title) {
    payload.title = options.title;
  }
  if (options.text) {
    payload.text = options.text;
  }

  if (typeof navigatorLike?.share === "function") {
    try {
      await navigatorLike.share(payload);
      return { status: "shared", message: "已打开系统分享。" };
    } catch (error) {
      if (error?.name === "AbortError") {
        return { status: "cancelled", message: "" };
      }
    }
  }

  if (typeof navigatorLike?.clipboard?.writeText === "function") {
    try {
      await navigatorLike.clipboard.writeText(url);
      return { status: "copied", message: "链接已复制。" };
    } catch {
      // Fall through to a selectable URL when the clipboard is unavailable.
    }
  }

  return { status: "manual", message: url };
}
