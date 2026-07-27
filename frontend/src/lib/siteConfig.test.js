import assert from "node:assert/strict";
import test from "node:test";

import { defaultSiteConfig, normalizeLogoUrl, normalizeSiteConfig } from "./siteConfig.js";

test("normalizes public site config with fixed local navigation paths", () => {
  const config = normalizeSiteConfig({
    site_name: "  示例社区  ",
    site_description: "  面向开发者的交流社区。  ",
    seo_keywords: "  开发者,社区  ",
    logo_url: "https://cdn.example.com/logo.png",
    navigation: [
      { key: "shop", label: "积分商城" },
      { key: "plaza", label: "发现" },
      { key: "shop", label: "重复项" },
      { key: "external", label: "外部链接" }
    ]
  });

  assert.equal(config.siteName, "示例社区");
  assert.equal(config.siteDescription, "面向开发者的交流社区。");
  assert.equal(config.seoKeywords, "开发者,社区");
  assert.equal(config.logoUrl, "https://cdn.example.com/logo.png");
  assert.deepEqual(config.navigation, [
    { key: "shop", label: "积分商城", path: "/shop" },
    { key: "plaza", label: "发现", path: "/plaza" },
    { key: "chat", label: "聊天室", path: "/chat" }
  ]);
});

test("falls back to safe defaults for invalid public site config", () => {
  const config = normalizeSiteConfig({
    site_name: " ",
    logo_url: "javascript:alert(1)",
    navigation: [{ key: "external", label: "外部链接" }]
  });

  assert.equal(config.siteName, defaultSiteConfig.siteName);
  assert.equal(config.siteDescription, defaultSiteConfig.siteDescription);
  assert.equal(config.seoKeywords, defaultSiteConfig.seoKeywords);
  assert.equal(config.logoUrl, "");
  assert.deepEqual(config.navigation, defaultSiteConfig.navigation);
  assert.equal(normalizeLogoUrl("/uploads/logo.png"), "/uploads/logo.png");
  assert.equal(normalizeLogoUrl("//cdn.example.com/logo.png"), "");
});

test("adds the chat entry to the unchanged legacy default navigation", () => {
  const config = normalizeSiteConfig({
    navigation: [
      { key: "home", label: "首页" },
      { key: "plaza", label: "广场" },
      { key: "circles", label: "圈子" },
      { key: "help", label: "求助" },
      { key: "resources", label: "资源" },
      { key: "shop", label: "商城" },
      { key: "member", label: "会员" },
      { key: "more", label: "更多" }
    ]
  });

  assert.deepEqual(config.navigation.map((item) => item.key), [
    "home",
    "plaza",
    "circles",
    "chat",
    "help",
    "resources",
    "shop",
    "member",
    "more"
  ]);
});

test("adds the chat entry to custom navigation before more when no plaza exists", () => {
  const config = normalizeSiteConfig({
    navigation: [
      { key: "shop", label: "商城" },
      { key: "more", label: "更多" }
    ]
  });

  assert.deepEqual(config.navigation.map((item) => item.key), ["shop", "chat", "more"]);
});
