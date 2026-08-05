export function normalizeSessionList(data) {
  return {
    items: Array.isArray(data?.items) ? data.items.map(normalizeSession).filter((item) => item.sessionId) : [],
    total: Math.max(0, finiteNumber(data?.total))
  };
}

export function normalizeSession(data) {
  return {
    sessionId: String(data?.session_id || ""),
    ipAddress: String(data?.ip_address || ""),
    userAgent: String(data?.user_agent || ""),
    loginMethod: String(data?.login_method || ""),
    createdAt: finiteNumber(data?.created_at),
    expiresAt: finiteNumber(data?.expires_at),
    revokedAt: finiteNumber(data?.revoked_at),
    current: data?.current === true
  };
}

export function normalizeLoginEventList(data) {
  return {
    items: Array.isArray(data?.items) ? data.items.map(normalizeLoginEvent).filter((item) => item.id) : [],
    total: Math.max(0, finiteNumber(data?.total))
  };
}

export function normalizeLoginEvent(data) {
  return {
    id: String(data?.id || ""),
    sessionId: String(data?.session_id || ""),
    ipAddress: String(data?.ip_address || ""),
    userAgent: String(data?.user_agent || ""),
    success: data?.success === true,
    failureReason: String(data?.failure_reason || ""),
    createdAt: finiteNumber(data?.created_at)
  };
}

// Sessions carry no server-side status field: an entry is revoked when
// revoked_at is set, otherwise it stays active until expires_at passes.
export function sessionStatus(session, nowSeconds = Math.floor(Date.now() / 1000)) {
  if (finiteNumber(session?.revokedAt)) return "revoked";
  const expiresAt = finiteNumber(session?.expiresAt);
  if (expiresAt && expiresAt <= nowSeconds) return "expired";
  return "active";
}

export function sessionStatusLabel(status) {
  const labels = { active: "活跃", revoked: "已退出", expired: "已过期" };
  return labels[status] || "未知";
}

export function loginMethodLabel(method) {
  const labels = {
    password: "密码登录",
    mfa: "两步验证",
    passkey: "Passkey",
    passwordless_passkey: "Passkey 免密登录",
    oauth: "第三方登录",
    register: "注册登录",
    webmaster: "站长登录"
  };
  const key = String(method || "").trim().toLowerCase();
  return labels[key] || key || "未知方式";
}

export function loginFailureLabel(reason) {
  const labels = {
    invalid_password: "密码错误",
    invalid_account: "账号不存在",
    invalid_code: "验证码错误",
    mfa_required: "缺少两步验证",
    account_locked: "账号被锁定",
    rate_limited: "尝试过于频繁"
  };
  const key = String(reason || "").trim().toLowerCase();
  return labels[key] || key || "登录失败";
}

// Raw user agents are too long to display, so reduce them to a
// "platform · browser" summary and fall back to the raw prefix.
export function describeUserAgent(userAgent) {
  const text = String(userAgent || "").trim();
  if (!text) return "未知设备";
  const platform = matchFirst(text, [
    [/Windows NT/i, "Windows"],
    [/iPhone|iPad|iPod/i, "iOS"],
    [/Android/i, "Android"],
    [/Mac OS X|Macintosh/i, "macOS"],
    [/Linux/i, "Linux"]
  ]);
  const browser = matchFirst(text, [
    [/Edg[e\/]/i, "Edge"],
    [/OPR\/|Opera/i, "Opera"],
    [/Firefox/i, "Firefox"],
    [/Chrome|CriOS/i, "Chrome"],
    [/Safari/i, "Safari"]
  ]);
  const parts = [platform, browser].filter(Boolean);
  return parts.length > 0 ? parts.join(" · ") : text.slice(0, 40);
}

export function ipAddressLabel(value) {
  const text = String(value || "").trim();
  return text || "未知地址";
}

function matchFirst(text, patterns) {
  for (const [pattern, label] of patterns) {
    if (pattern.test(text)) return label;
  }
  return "";
}

function finiteNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : 0;
}
