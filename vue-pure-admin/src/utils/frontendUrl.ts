function configuredFrontendBase() {
  return String(import.meta.env.VITE_MALL_FRONTEND_BASE || "");
}

export function buildFrontendUrl(
  path: string,
  base = configuredFrontendBase()
) {
  const normalizedPath = `/${String(path || "")
    .trim()
    .replace(/^\/+/, "")}`;
  const normalizedBase = String(base || "")
    .trim()
    .replace(/\/+$/, "");
  return normalizedBase ? `${normalizedBase}${normalizedPath}` : normalizedPath;
}
