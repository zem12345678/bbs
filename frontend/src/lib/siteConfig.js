import { pageRoutes } from "../routes.js";

const DEFAULT_SITE_NAME = "云栖社区广场";
const DEFAULT_SITE_DESCRIPTION = "一个面向内容沉淀、圈子协作和技术交流的社区论坛。";
const DEFAULT_SEO_KEYWORDS = "bbs,community,forum";
const MAX_SITE_NAME_LENGTH = 80;
const MAX_SITE_DESCRIPTION_LENGTH = 500;
const MAX_SEO_KEYWORDS_LENGTH = 500;
const MAX_NAVIGATION_LABEL_LENGTH = 24;
export const defaultSiteConfig = {
  siteName: DEFAULT_SITE_NAME,
  siteDescription: DEFAULT_SITE_DESCRIPTION,
  seoKeywords: DEFAULT_SEO_KEYWORDS,
  logoUrl: "",
  navigation: defaultNavigation()
};

const navigationByKey = new Map(pageRoutes.map((route) => [route.key, route]));

export function normalizeSiteConfig(data) {
  return {
    siteName: normalizedText(data?.site_name, DEFAULT_SITE_NAME, MAX_SITE_NAME_LENGTH),
    siteDescription: normalizedText(
      data?.site_description,
      DEFAULT_SITE_DESCRIPTION,
      MAX_SITE_DESCRIPTION_LENGTH
    ),
    seoKeywords: normalizedText(data?.seo_keywords, DEFAULT_SEO_KEYWORDS, MAX_SEO_KEYWORDS_LENGTH),
    logoUrl: normalizeLogoUrl(data?.logo_url),
    navigation: normalizeNavigation(data?.navigation)
  };
}

export function normalizeNavigation(items) {
  if (!Array.isArray(items)) {
    return defaultNavigation();
  }
  const navigation = [];
  const seen = new Set();
  for (const item of items) {
    const key = typeof item?.key === "string" ? item.key.trim().toLowerCase() : "";
    const route = navigationByKey.get(key);
    if (!route || seen.has(key)) {
      continue;
    }
    seen.add(key);
    navigation.push({
      key,
      label: normalizedText(item?.label, route.label, MAX_NAVIGATION_LABEL_LENGTH),
      path: route.path
    });
  }
  if (navigation.length === 0) {
    return defaultNavigation();
  }
  ensureChatNavigation(navigation);
  return navigation;
}

function ensureChatNavigation(navigation) {
  if (navigation.some((item) => item.key === "chat")) {
    return;
  }
  const chat = navigationByKey.get("chat");
  if (!chat) {
    return;
  }
  const insertAfterKey = navigation.some((item) => item.key === "circles") ? "circles" : "plaza";
  const insertAfterIndex = navigation.findIndex((item) => item.key === insertAfterKey);
  const moreIndex = navigation.findIndex((item) => item.key === "more");
  const insertIndex = insertAfterIndex >= 0 ? insertAfterIndex + 1 : moreIndex >= 0 ? moreIndex : navigation.length;
  navigation.splice(insertIndex, 0, { key: chat.key, label: chat.label, path: chat.path });
}

export function normalizeLogoUrl(value) {
  if (typeof value !== "string") {
    return "";
  }
  const url = value.trim();
  if (!url) {
    return "";
  }
  if (url.startsWith("/") && !url.startsWith("//")) {
    return url;
  }
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" || parsed.protocol === "http:" ? url : "";
  } catch {
    return "";
  }
}

function defaultNavigation() {
  return pageRoutes.map(({ key, label, path }) => ({ key, label, path }));
}

function normalizedText(value, fallback, maxLength) {
  if (typeof value !== "string") {
    return fallback;
  }
  const text = value.trim();
  return text ? text.slice(0, maxLength) : fallback;
}
