import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

import { pageRoutes, pathToPage } from "./routes.js";

test("maps chat workspace routes to the chat navigation section", () => {
  assert.equal(pageRoutes.find((route) => route.key === "chat")?.path, "/chat");
  assert.equal(pathToPage("/chat"), "聊天室");
  assert.equal(pathToPage("/room/AB12CD3E"), "聊天室");
});

test("maps username profile routes to the member navigation section", () => {
  assert.equal(pathToPage("/u/alice"), "会员");
  assert.equal(pathToPage("/u/alice/articles"), "会员");
});

test("keeps explicit chat entries on user message surfaces", () => {
  const messageSurfaces = [
    fs.readFileSync(new URL("./pages/UserRoutes.jsx", import.meta.url), "utf8"),
    fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8")
  ];

  for (const source of messageSurfaces) {
    assert.match(source, /进入聊天室/);
    assert.match(source, /navigate\("\/chat"\)/);
  }
});

test("keeps a desktop floating chat entry", () => {
  const source = fs.readFileSync(new URL("./components/layout/FloatingRail.jsx", import.meta.url), "utf8");

  assert.match(source, /label:\s*"聊天室"/);
  assert.match(source, /path:\s*"\/chat"/);
});

test("shop page uses shared mall order status helpers", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");

  assert.match(source, /mallOrderCanPay/);
  assert.match(source, /mallOrderStatusLabel/);
  assert.doesNotMatch(source, /function formatOrderStatus/);
  assert.doesNotMatch(source, /function orderAwaitingPayment/);
});

test("order history ignores superseded list responses", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");

  assert.match(source, /orderLoadRequestVersionRef/);
  assert.match(source, /const requestVersion = \+\+orderLoadRequestVersionRef\.current/);
  assert.match(source, /requestVersion === orderLoadRequestVersionRef\.current/);
  assert.match(source, /if \(!isCurrent\(\)\) return/);
});

test("dashboard serializes order mutations before button state rerenders", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const ordersPanel = source.slice(source.indexOf("function OrdersPanel"), source.indexOf("function EntitlementsPanel"));
  const loadOrders = ordersPanel.slice(ordersPanel.indexOf("const loadOrders"), ordersPanel.indexOf("function loadMoreOrders"));

  assert.match(ordersPanel, /const orderActionSubmittingRef = React\.useRef\(false\)/);
  assert.match(ordersPanel, /orderActionSubmittingRef\.current = true/);
  assert.match(ordersPanel, /finally \{\s*orderActionSubmittingRef\.current = false/);
  assert.match(ordersPanel, /disabled=\{orderActionBusy\}/);
  assert.doesNotMatch(loadOrders, /action:\s*""/);
});

test("shop serializes review submission and image upload before button state rerenders", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const actions = ["submitProductReview", "uploadReviewImage"].map((name) => {
    const start = source.indexOf(`async function ${name}`);
    const end = source.indexOf("\n\n  async function ", start + 1);
    assert.ok(start >= 0, `${name} is present`);
    return source.slice(start, end === -1 ? undefined : end);
  });

  assert.match(source, /const reviewActionSubmittingRef = React\.useRef\(false\)/);
  assert.match(source, /const \[reviewActionBusy, setReviewActionBusy\] = React\.useState\(false\)/);
  assert.match(source, /if \(!token \|\| !detailProduct\?\.id \|\| reviewActionSubmittingRef\.current\) return/);
  assert.match(source, /if \(!file \|\| reviewActionSubmittingRef\.current\) return/);
  assert.match(source, /disabled=\{reviewActionBusy\}/);
  for (const action of actions) {
    assert.match(action, /reviewActionSubmittingRef\.current = true;\s*setReviewActionBusy\(true\)/);
    assert.match(action, /finally \{\s*reviewActionSubmittingRef\.current = false;\s*setReviewActionBusy\(false\)/);
  }
});

test("shop ignores stale product-detail review responses", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");

  assert.match(source, /const detailReviewSessionRef = React\.useRef\(0\)/);
  assert.match(source, /React\.useLayoutEffect\(\(\) => \{\s*detailReviewSessionRef\.current \+= 1;\s*\}, \[detailProduct\?\.id, token\]\)/);
  assert.match(source, /function isCurrentDetailReviewRequest\(productId, session\)/);

  const reviewActions = new Map();
  for (const name of [
    "loadMoreProductReviews",
    "loadMoreMyProductReviews",
    "loadMoreProductReviewOrders",
    "submitProductReview",
    "uploadReviewImage"
  ]) {
    const start = source.indexOf(`async function ${name}`);
    const end = source.indexOf("\n\n  async function ", start + 1);
    const action = source.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(action, /(?:const|let) reviewSession = detailReviewSessionRef\.current/);
    reviewActions.set(name, action);
  }

  for (const name of ["loadMoreProductReviews", "loadMoreMyProductReviews", "loadMoreProductReviewOrders", "uploadReviewImage"]) {
    const action = reviewActions.get(name);
    assert.match(action, /await bbsApi[\s\S]*?;\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
  }

  const submit = reviewActions.get("submitProductReview");
  assert.match(submit, /await bbsApi\.createMallProductReview\([\s\S]*?\);\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(submit, /reviewSession = \+\+detailReviewSessionRef\.current;\s*setProductReviewOrders/);
  assert.match(submit, /await Promise\.allSettled\([\s\S]*?\);\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(submit, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
});

test("shop ignores stale catalog pages after a query refresh", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const catalogLoad = source.slice(source.indexOf("async function reloadProducts"), source.indexOf("function applyCartData"));

  assert.match(source, /const productLoadRequestVersionRef = React\.useRef\(0\)/);
  assert.match(source, /const productQueryRef = React\.useRef\(\{ keyword: filters\.keyword, category: filters\.category \}\)/);
  assert.match(source, /function isCurrentProductRequest\(query, requestVersion\)/);
  assert.match(source, /React\.useLayoutEffect\(\(\) => \{\s*productLoadRequestVersionRef\.current \+= 1;\s*\}, \[filters\.category, filters\.keyword\]\)/);
  assert.match(source, /const requestVersion = \+\+productLoadRequestVersionRef\.current;\s*const isCurrentRequest = \(\) => alive && isCurrentProductRequest/);

  for (const name of ["reloadProducts", "loadMoreProducts"]) {
    const start = catalogLoad.indexOf(`async function ${name}`);
    const end = catalogLoad.indexOf("\n\n  async function ", start + 1);
    const action = catalogLoad.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(
      action,
      name === "reloadProducts"
        ? /const query = \{ \.\.\.productQueryRef\.current \}/
        : /const query = \{ keyword: filters\.keyword, category: filters\.category \}/
    );
    assert.match(action, /const requestVersion = \+\+productLoadRequestVersionRef\.current/);
    assert.match(action, /await bbsApi\.mallProducts\([\s\S]*?\);\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
  }
});
