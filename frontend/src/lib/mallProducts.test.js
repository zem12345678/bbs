import assert from "node:assert/strict";
import test from "node:test";
import { mallGrantLabel, mallGrantSnapshotText, mallProductRequiresShipping, parseShopDeepLink, sortProductsForStorefront } from "./mallProducts.js";

test("sortProductsForStorefront keeps in-stock products before unavailable products", () => {
  const products = [
    { id: "sold-out", stock: 0 },
    { id: "missing-stock" },
    { id: "available", stock: 3 },
    { id: "available-string-stock", stock: "2" }
  ];

  assert.deepEqual(
    sortProductsForStorefront(products).map((product) => product.id),
    ["available", "available-string-stock", "sold-out", "missing-stock"]
  );
});

test("sortProductsForStorefront preserves API order within the same availability group", () => {
  const products = [
    { id: "first", stock: 5 },
    { id: "second", stock: 1 },
    { id: "third", stock: 0 },
    { id: "fourth", stock: 0 }
  ];

  assert.deepEqual(
    sortProductsForStorefront(products).map((product) => product.id),
    ["first", "second", "third", "fourth"]
  );
});

test("parseShopDeepLink normalizes canonical storefront link params", () => {
  const params = new URLSearchParams({
    product_id: " 12345 ",
    coupon_code: " e2e-save-5 ",
    review_order_id: " 98765 "
  });

  assert.deepEqual(parseShopDeepLink(params), {
    productId: "12345",
    couponCode: "E2E-SAVE-5",
    reviewOrderId: "98765",
    category: "",
    keyword: ""
  });
});

test("parseShopDeepLink accepts common product and coupon aliases for campaign links", () => {
  assert.deepEqual(parseShopDeepLink("?product=sku-100&coupon=spring10&orderId=200"), {
    productId: "sku-100",
    couponCode: "SPRING10",
    reviewOrderId: "200",
    category: "",
    keyword: ""
  });
});

test("parseShopDeepLink accepts relative storefront campaign URLs", () => {
  assert.deepEqual(parseShopDeepLink("/shop?productId=sku-200&couponCode=vip20"), {
    productId: "sku-200",
    couponCode: "VIP20",
    reviewOrderId: "",
    category: "",
    keyword: ""
  });
});

test("parseShopDeepLink ignores blank alias values and falls back to the first useful value", () => {
  assert.deepEqual(parseShopDeepLink({ product_id: "", productId: "42", code: " vip " }), {
    productId: "42",
    couponCode: "VIP",
    reviewOrderId: "",
    category: "",
    keyword: ""
  });
});

test("parseShopDeepLink accepts category and keyword campaign filters", () => {
  assert.deepEqual(parseShopDeepLink("/shop?category=digital&keyword=vip%20rights"), {
    productId: "",
    couponCode: "",
    reviewOrderId: "",
    category: "digital",
    keyword: "vip rights"
  });
});

test("parseShopDeepLink accepts common category and keyword aliases", () => {
  assert.deepEqual(parseShopDeepLink("?cat=perks&q=badge"), {
    productId: "",
    couponCode: "",
    reviewOrderId: "",
    category: "perks",
    keyword: "badge"
  });
});

test("mallGrantSnapshotText formats order item grant snapshots", () => {
  assert.equal(mallGrantSnapshotText({ grant_type: "badge", grant_key: "badge-founder" }), "徽章权益 · badge-founder");
  assert.equal(mallGrantSnapshotText({ grantType: "theme", grantKey: "theme-pro" }), "主题权益 · theme-pro");
});

test("mallGrantSnapshotText infers membership grants from bare grant keys", () => {
  assert.equal(mallGrantSnapshotText({ grant_key: "vip-month" }), "会员权益 · vip-month");
  assert.equal(mallGrantSnapshotText({}), "");
});

test("mallProductRequiresShipping treats granted products as online fulfillment", () => {
  assert.equal(mallProductRequiresShipping({ category: "digital" }), false);
  assert.equal(mallProductRequiresShipping({ category: "membership", grant_type: "membership", grant_key: "vip-month" }), false);
  assert.equal(mallProductRequiresShipping({ category: "badge", grant_key: "badge-founder" }), false);
  assert.equal(mallProductRequiresShipping({ category: "merch" }), true);
});

test("mallGrantLabel keeps unknown configured grant types visible", () => {
  assert.equal(mallGrantLabel("custom-right"), "custom-right");
  assert.equal(mallGrantLabel(""), "数字权益");
});
