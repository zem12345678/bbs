import { toNumber } from "./formatters.js";
import { mallGrantLabel } from "./mallProducts.js";

export function userBadgeRows(items = [], options = {}) {
  return items.map((item, index) => ({
    key: badgeRowKey(item, index),
    title: badgeTitle(item, index),
    description: badgeDescription(item),
    meta: badgeMeta(item, options)
  }));
}

export function badgeMeta(item, options = {}) {
  const parts = [];
  const grantType = badgeGrantType(item);
  if (badgeSource(item) === "digital_entitlement" || grantType) {
    parts.push(mallGrantLabel(grantType || "digital"));
  }
  const awardedAt = badgeAwardedAt(item);
  if (awardedAt) {
    parts.push(`获得于 ${relativeTimeText(awardedAt, options.now)}`);
  }
  const expiry = badgeExpiryText(item, options.now);
  if (expiry) {
    parts.push(expiry);
  }
  const orderNo = badgeOrderNo(item);
  if (orderNo) {
    parts.push(`订单 ${orderNo}`);
  }
  if (parts.length > 0) {
    return parts.join(" · ");
  }
  return badgeStatusText(item) || "已获得";
}

export function badgeExpiryText(item, now = Date.now()) {
  const expiresAt = toNumber(item?.expires_at ?? item?.expiresAt);
  if (!expiresAt) return "";
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return "";
  return `${expiresAt <= now ? "已过期" : "有效至"} ${date.toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" })}`;
}

function badgeRowKey(item, index) {
  return item?.id || item?.key || item?.entitlement_id || item?.entitlementId || index;
}

function badgeTitle(item, index) {
  return item?.name || item?.title || `徽章 #${index + 1}`;
}

function badgeDescription(item) {
  return item?.description || item?.reason || "社区成就徽章";
}

function badgeAwardedAt(item) {
  return toNumber(item?.awarded_at ?? item?.awardedAt);
}

function badgeGrantType(item) {
  return String(item?.grant_type || item?.grantType || "").trim().toLowerCase();
}

function badgeSource(item) {
  return String(item?.source || item?.Source || "").trim().toLowerCase();
}

function badgeOrderNo(item) {
  return String(item?.order_no || item?.orderNo || "").trim();
}

function badgeStatusText(item) {
  const status = String(item?.status || item?.Status || "").trim().toLowerCase();
  if (status === "awarded") return "已获得";
  if (status === "active") return "可用";
  if (status === "revoked") return "已撤销";
  return status;
}

function relativeTimeText(value, now = Date.now()) {
  const timestamp = toNumber(value);
  if (!timestamp) {
    return "刚刚";
  }
  const millis = timestamp > 100000000000 ? timestamp : timestamp * 1000;
  const diff = Math.max(1, Math.floor((now - millis) / 1000));
  if (diff < 60) return "刚刚";
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
  if (diff < 2592000) return `${Math.floor(diff / 86400)}天前`;
  return new Date(millis).toLocaleDateString("zh-CN");
}
