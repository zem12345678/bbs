const SECURITY_EMAIL_UNAVAILABLE = "security email delivery unavailable";

export function friendlySecurityEmailError(error, fallback) {
  const message = String(error?.message || "").trim();
  if (message.toLowerCase().includes(SECURITY_EMAIL_UNAVAILABLE)) {
    return "邮件服务暂不可用，请稍后重试。";
  }
  return message || fallback;
}
