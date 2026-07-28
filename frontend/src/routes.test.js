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
  const loadOrdersStart = ordersPanel.indexOf("const loadOrders");
  const loadOrders = ordersPanel.slice(loadOrdersStart, ordersPanel.indexOf("React.useEffect(() => ()", loadOrdersStart));

  assert.match(ordersPanel, /const orderActionSubmittingRef = React\.useRef\(false\)/);
  assert.match(ordersPanel, /orderActionSubmittingRef\.current = true/);
  assert.match(ordersPanel, /finally \{\s*if \(isCurrentRequest\(\)\) orderActionSubmittingRef\.current = false/);
  assert.match(ordersPanel, /disabled=\{orderActionBusy\}/);
  assert.doesNotMatch(loadOrders, /action:\s*""/);
});

test("dashboard order actions ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const ordersPanel = source.slice(source.indexOf("function OrdersPanel"), source.indexOf("function EntitlementsPanel"));

  assert.match(ordersPanel, /const orderSessionRef = React\.useRef\(0\)/);
  assert.match(ordersPanel, /const orderTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(ordersPanel, /orderTokenRef\.current = auth\.accessToken/);
  assert.match(ordersPanel, /function isCurrentOrderSessionRequest\(requestToken, session\)/);
  assert.match(ordersPanel, /React\.useLayoutEffect\(\(\) => \{\s*orderSessionRef\.current \+= 1;\s*orderActionSubmittingRef\.current = false/);

  for (const name of ["payOrder", "cancelOrder", "confirmOrder", "submitRefund", "cancelRefund"]) {
    const start = ordersPanel.indexOf(`async function ${name}`);
    const end = ordersPanel.indexOf("\n\n  ", start + 1);
    const action = ordersPanel.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(action, /const requestToken = auth\.accessToken/);
    assert.match(action, /const orderSession = orderSessionRef\.current/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentOrderSessionRequest\(requestToken, orderSession\)/);
    assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\)/);
    assert.match(action, /await bbsApi[\s\S]*?requestToken/);
    assert.match(action, /await bbsApi[\s\S]*?;\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /finally \{\s*if \(isCurrentRequest\(\)\) orderActionSubmittingRef\.current = false/);
    assert.doesNotMatch(action, /auth\.accessToken\)/);
  }

  for (const name of ["payOrder", "cancelOrder"]) {
    const start = ordersPanel.indexOf(`async function ${name}`);
    const end = ordersPanel.indexOf("\n\n  ", start + 1);
    const action = ordersPanel.slice(start, end === -1 ? undefined : end);

    assert.match(action, /const requestUserId = auth\?\.user\?\.id/);
    assert.match(action, /clearCheckoutAttemptForOrder\(\{ userId: requestUserId/);
  }
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

test("shop owns checkout completion across auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const start = source.indexOf("async function redeemProduct");
  const end = source.indexOf("\n\n  function cancelCheckout", start);
  const redeem = source.slice(start, end);

  assert.ok(start >= 0, "redeemProduct is present");
  assert.ok(end > start, "redeemProduct has a bounded body");
  assert.match(source, /const \[checkoutActionBusy, setCheckoutActionBusy\] = React\.useState\(false\)/);
  assert.match(source, /const checkoutSubmittingRef = React\.useRef\(0\)/);
  assert.match(source, /const checkoutRequestIdRef = React\.useRef\(0\)/);
  assert.match(source, /const shopSessionRef = React\.useRef\(0\)/);
  assert.match(source, /const shopTokenRef = React\.useRef\(token\)/);
  assert.match(source, /shopTokenRef\.current = token/);
  assert.match(source, /function isCurrentShopSessionRequest\(requestToken, session\)/);
  assert.match(source, /React\.useLayoutEffect\(\(\) => \{\s*shopSessionRef\.current \+= 1;\s*\}, \[token\]\)/);
  assert.match(source, /checkoutSubmittingRef\.current = 0;\s*setCheckoutActionBusy\(false\);\s*setBusyProductId\(null\)/);
  assert.match(source, /const checkoutBusy = checkoutActionBusy \|\|/);

  assert.match(redeem, /const requestToken = token/);
  assert.match(redeem, /const requestUserId = auth\?\.user\?\.id/);
  assert.match(redeem, /const shopSession = shopSessionRef\.current/);
  assert.match(redeem, /const isCurrentRequest = \(\) => isCurrentShopSessionRequest\(requestToken, shopSession\)/);
  assert.match(redeem, /const requestID = \+\+checkoutRequestIdRef\.current;\s*checkoutSubmittingRef\.current = requestID/);
  assert.match(redeem, /setCheckoutActionBusy\(true\)/);
  assert.match(redeem, /checkoutAttemptKey\(\{\s*userId: requestUserId/);
  assert.match(redeem, /recordCheckoutAttemptOrder\(\{ userId: requestUserId/);
  assert.match(redeem, /clearCheckoutAttemptKey\(\{ userId: requestUserId/);
  assert.match(redeem, /checkoutMallCart\(orderPayload, requestToken\)/);
  assert.match(redeem, /await bbsApi\.payMallOrder\([\s\S]*?requestToken\s*\)/);
  assert.match(redeem, /if \(!isCurrentRequest\(\)\) return;\s*if \(checkout\.mode === "cart"\) applyCartData/);
  assert.match(redeem, /if \(checkoutSubmittingRef\.current !== requestID\) return;\s*checkoutSubmittingRef\.current = 0;\s*setCheckoutActionBusy\(false\)/);
  assert.doesNotMatch(redeem, /checkoutMallCart\(orderPayload, token\)/);
  assert.doesNotMatch(redeem, /payMallOrder\([\s\S]*?,\s*token\s*\)/);
});

test("shop guards authenticated storefront side effects across auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const shopAction = (name) => {
    const start = source.indexOf(`async function ${name}`);
    const nextAsync = source.indexOf("\n\n  async function ", start + 1);
    const nextFunction = source.indexOf("\n\n  function ", start + 1);
    const end = Math.min(...[nextAsync, nextFunction].filter((index) => index > start));

    assert.ok(start >= 0, `${name} is present`);
    assert.ok(end > start, `${name} has a bounded body`);
    return source.slice(start, end);
  };

  for (const name of [
    "reloadFavorites",
    "loadMoreFavorites",
    "addToCart",
    "toggleProductFavorite",
    "refreshCheckoutProduct",
    "updateCartQuantity",
    "removeCartItem",
    "clearCart",
    "reloadAddresses",
    "loadMoreAddresses",
    "loadMoreMyCoupons",
    "claimCoupon",
    "saveAddress",
    "setDefaultAddress",
    "deleteAddress"
  ]) {
    const action = shopAction(name);

    assert.match(action, /const requestToken = token/);
    assert.match(action, /const session = shopSessionRef\.current/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentShopSessionRequest\(requestToken, session\)/);
    assert.match(action, /if \(!isCurrentRequest\(\)\) return/);
    assert.doesNotMatch(action, /bbsApi\.[^(]+\([\s\S]*?, token\)/);
  }

  assert.match(shopAction("refreshCoupons"), /async function refreshCoupons\(isCurrentRequest = \(\) => true\)/);
  assert.match(shopAction("refreshCoupons"), /if \(!isCurrentRequest\(\)\) return \[\];\s*const data = await bbsApi\.mallCoupons/);
  assert.match(shopAction("syncCheckoutAfterMallError"), /refreshCoupons\(isCurrentRequest\)/);
  assert.match(shopAction("claimCoupon"), /Promise\.allSettled\(\[refreshCoupons\(isCurrentRequest\), refreshMyCoupons\(\)\]\)/);

  for (const name of ["claimCoupon", "saveAddress", "setDefaultAddress", "deleteAddress"]) {
    assert.match(shopAction(name), /finally \{\s*if \(isCurrentRequest\(\)\)/);
  }
});
