import { toNumber } from "./formatters.js";

const PRODUCT_ID_PARAMS = ["product_id", "productId", "product", "p"];
const COUPON_CODE_PARAMS = ["coupon_code", "couponCode", "coupon", "code"];
const REVIEW_ORDER_ID_PARAMS = ["review_order_id", "reviewOrderId", "order_id", "orderId"];
const CATEGORY_PARAMS = ["category", "category_slug", "categorySlug", "cat"];
const KEYWORD_PARAMS = ["keyword", "q", "search"];
const GRANT_TYPE_LABELS = {
  badge: "徽章权益",
  theme: "主题权益",
  membership: "会员权益",
  digital: "数字权益"
};

export function sortProductsForStorefront(products = []) {
  return [...products].sort((left, right) => {
    const leftAvailable = productIsAvailable(left);
    const rightAvailable = productIsAvailable(right);
    if (leftAvailable !== rightAvailable) {
      return rightAvailable - leftAvailable;
    }
    return 0;
  });
}

export function parseShopDeepLink(input = "") {
  const params = toSearchParams(input);
  return {
    productId: normalizeParamValue(firstParam(params, PRODUCT_ID_PARAMS)),
    couponCode: normalizeCouponCode(firstParam(params, COUPON_CODE_PARAMS)),
    reviewOrderId: normalizeParamValue(firstParam(params, REVIEW_ORDER_ID_PARAMS)),
    category: normalizeParamValue(firstParam(params, CATEGORY_PARAMS)),
    keyword: normalizeParamValue(firstParam(params, KEYWORD_PARAMS))
  };
}

export function normalizeCouponCode(value) {
  return normalizeParamValue(value).toUpperCase();
}

export function mallGrantTypeOf(source) {
  const explicit = normalizeParamValue(source?.grant_type ?? source?.grantType).toLowerCase();
  return explicit || grantTypeFromKey(mallGrantKeyOf(source));
}

export function mallGrantKeyOf(source) {
  return normalizeParamValue(source?.grant_key ?? source?.grantKey);
}

export function mallGrantLabel(type) {
  const normalized = normalizeParamValue(type).toLowerCase();
  return GRANT_TYPE_LABELS[normalized] || (normalized ? normalized : GRANT_TYPE_LABELS.digital);
}

export function mallGrantSnapshotText(source) {
  const grantType = mallGrantTypeOf(source);
  const grantKey = mallGrantKeyOf(source);
  if (!grantType && !grantKey) return "";
  return `${mallGrantLabel(grantType || "digital")}${grantKey ? ` · ${grantKey}` : ""}`;
}

function productIsAvailable(product) {
  return toNumber(product?.stock) > 0;
}

function grantTypeFromKey(value) {
  const normalized = normalizeParamValue(value).toLowerCase();
  if (!normalized) return "";
  if (normalized.startsWith("badge-")) return "badge";
  if (normalized.startsWith("theme-")) return "theme";
  if (normalized.startsWith("vip-") || normalized.startsWith("member-") || normalized.includes("membership")) return "membership";
  return "digital";
}

function firstParam(params, names) {
  return names.map((name) => params.get(name)).find((value) => normalizeParamValue(value)) || "";
}

function normalizeParamValue(value) {
  return String(value ?? "").trim();
}

function toSearchParams(input) {
  if (input instanceof URLSearchParams) return input;
  if (typeof input === "string") {
    const text = input.trim();
    if (!text) return new URLSearchParams();
    if (text.startsWith("?") || /^[^/:?#=]+=/u.test(text)) {
      return new URLSearchParams(text.replace(/^\?/, ""));
    }
    try {
      return new URL(text, "http://bbs.local").searchParams;
    } catch {
      return new URLSearchParams(text.replace(/^\?/, ""));
    }
  }
  return new URLSearchParams(input);
}
