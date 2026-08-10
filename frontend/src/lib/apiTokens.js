const VALID_SCOPES = new Set(["read", "write"]);

export function normalizeAPITokenList(data) {
  const rawItems = Array.isArray(data?.items) ? data.items : [];
  return {
    items: rawItems.map(normalizeAPIToken).filter((item) => item.id),
    total: Math.max(0, finiteNumber(data?.total))
  };
}

export function normalizeAPIToken(data) {
  const scopes = Array.isArray(data?.scopes) ? data.scopes : Array.isArray(data?.permission) ? data.permission : [];
  return {
    id: String(data?.id || ""),
    name: String(data?.name || "").trim(),
    scopes: [...new Set(scopes.map((scope) => String(scope || "").trim().toLowerCase()).filter((scope) => VALID_SCOPES.has(scope)))],
    createdAt: finiteNumber(data?.created_at ?? data?.createdAt),
    expiresAt: finiteNumber(data?.expires_at ?? data?.expiresAt),
    revokedAt: finiteNumber(data?.revoked_at ?? data?.revokedAt),
    credentialValid: data?.credential_valid !== false && data?.credentialValid !== false,
    active: data?.active !== false
  };
}

export function normalizeAPITokenCreation(data) {
  return {
    token: String(data?.token || ""),
    id: String(data?.id || data?.api_token?.id || ""),
    apiToken: normalizeAPIToken(data?.api_token || data)
  };
}

export function apiTokenStatus(token, nowSeconds = Math.floor(Date.now() / 1000)) {
  if (token?.revokedAt) return "revoked";
  if (token?.credentialValid === false) return "invalid";
  if (token?.expiresAt && token.expiresAt <= nowSeconds) return "expired";
  return token?.active === false ? "inactive" : "active";
}

export function apiTokenStatusLabel(status) {
  return { active: "有效", revoked: "已撤销", expired: "已过期", invalid: "已失效", inactive: "不可用" }[status] || "未知";
}

export function apiTokenScopeLabel(scope) {
  return { read: "读取", write: "写入" }[String(scope || "").toLowerCase()] || String(scope || "未知");
}

export function apiTokenTime(value) {
  const seconds = finiteNumber(value);
  if (!seconds) return "未设置";
  return new Date(seconds * 1000).toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function finiteNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : 0;
}
