const PROFILE_THEME_ENTITLEMENT_ERROR = "profile theme entitlement required";
const PROFILE_BACKGROUND_ENTITLEMENT_ERROR = "profile background membership entitlement required";

function errorStatus(error) {
  return Number(error?.status || error?.httpCode || error?.responseStatus || 0);
}

function errorText(error) {
  return [
    error?.message,
    error?.reason,
    error?.rawBody,
    error?.code,
    error?.meta?.legacy_code,
    error?.meta?.legacyCode
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

export function isProfileThemeEntitlementError(error) {
  if (errorStatus(error) !== 403) return false;
  return errorText(error).includes(PROFILE_THEME_ENTITLEMENT_ERROR);
}

export function isProfileBackgroundEntitlementError(error) {
  if (errorStatus(error) !== 403) return false;
  return errorText(error).includes(PROFILE_BACKGROUND_ENTITLEMENT_ERROR);
}

export function friendlyProfileUpdateError(error, fallback = "资料保存失败") {
  if (isProfileThemeEntitlementError(error)) {
    return "高级主题需要 theme-pro 权益，请先购买或切回默认主题。";
  }
  if (isProfileBackgroundEntitlementError(error)) {
    return "自定义背景图需要有效会员权益，请先购买会员或清空背景。";
  }
  return error?.message || fallback;
}
