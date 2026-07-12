import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const API_BASE = (process.env.API_BASE || process.env.VITE_API_BASE || "http://127.0.0.1:18080/api/v1").replace(/\/$/, "");
const FRONTEND_BASE = (process.env.FRONTEND_BASE || "http://127.0.0.1:8850").replace(/\/$/, "");
const AUTH_STORAGE_KEY = "bbs.community.auth";
const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.dirname(SCRIPT_DIR);
const FRONTEND_DIR = path.join(REPO_ROOT, "frontend");
const VITE_BIN = path.join(FRONTEND_DIR, "node_modules", "vite", "bin", "vite.js");

const CHECKOUT_PRICE = 20;
const COUPON_DISCOUNT = 5;
const CREDIT_TOP_UP = 200;

async function main() {
  await assertHttpReachable(`${API_BASE}/mall/products?limit=1&offset=0`, "api-gateway");

  const frontendServer = await ensureFrontendServer();
  try {
    const fixture = await createCommercialFixture();
    const chromePath = await findChromeExecutable();
    const result = await runBrowserCheckout(chromePath, fixture);

    console.log(
      JSON.stringify(
        {
          ok: true,
          productId: fixture.product.id,
          cartProductId: fixture.cartProduct.id,
          refundProductId: fixture.refundProduct.id,
          couponCode: fixture.coupon.code,
          userId: fixture.auth.user.id,
          orderId: result.orderId,
          orderNo: result.orderNo,
          paidText: result.paidText,
          fulfillmentText: result.fulfillmentText,
          reviewText: result.reviewText,
          cartOrderId: result.cartOrderId,
          cartText: result.cartText,
          cartNotificationTitles: result.cartNotificationTitles,
          refundOrderId: result.refundOrderId,
          refundText: result.refundText,
          refundNotificationTitles: result.refundNotificationTitles,
          notificationTitles: result.notificationTitles
        },
        null,
        2
      )
    );
  } finally {
    await frontendServer?.stop();
  }
}

async function createCommercialFixture() {
  const stamp = Date.now();
  const admin = await apiRequest("/admin/auth/login", {
    method: "POST",
    body: {
      account: process.env.ADMIN_ACCOUNT || "admin",
      password: process.env.ADMIN_PASSWORD || "Admin123!"
    }
  });
  const adminToken = admin.access_token || admin.accessToken;
  if (!adminToken) {
    throw new Error("Admin login did not return access_token");
  }

  const slug = `e2e-${stamp}`;
  const sku = `E2E-${stamp}`;
  const couponCode = `E2E${stamp}`;
  const productTitle = `E2E Browser Product ${stamp}`;
  const cartProductTitle = `E2E Cart Product ${stamp}`;
  const refundProductTitle = `E2E Refund Product ${stamp}`;

  const category = await apiRequest("/admin/mall/categories", {
    method: "POST",
    token: adminToken,
    body: {
      slug,
      name: `E2E Mall ${stamp}`,
      description: "Browser E2E mall category",
      status: 2,
      sort: 999
    }
  });

  const product = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku,
      title: productTitle,
      description: "Browser E2E mall product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const refundProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-REFUND`,
      title: refundProductTitle,
      description: "Browser E2E mall refund product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9998
    }
  });

  const cartProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-CART`,
      title: cartProductTitle,
      description: "Browser E2E mall cart product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9997
    }
  });

  const coupon = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: couponCode,
      name: `${couponCode} Browser E2E Coupon`,
      description: "Browser E2E coupon",
      discount_credits: COUPON_DISCOUNT,
      min_order_credits: CHECKOUT_PRICE,
      total_quota: 10,
      per_user_limit: 1,
      status: 2,
      starts_at: 0,
      ends_at: 0
    }
  });

  const password = `Passw0rd!${stamp}`;
  const registered = await apiRequest("/auth/register", {
    method: "POST",
    body: {
      username: `e2e${stamp}`,
      email: `e2e${stamp}@example.com`,
      password,
      nickname: `E2E ${stamp}`
    }
  });

  const auth = {
    accessToken: registered.access_token || registered.accessToken,
    expiresAt: registered.expires_at || registered.expiresAt,
    user: registered.user
  };
  if (!auth.accessToken || !auth.user?.id) {
    throw new Error("User registration did not return auth payload");
  }

  await apiRequest(`/admin/credits/users/${encodeURIComponent(auth.user.id)}/adjust`, {
    method: "POST",
    token: adminToken,
    body: {
      delta: CREDIT_TOP_UP,
      reason: "browser_mall_topup",
      description: "Browser mall checkout top-up",
      source_event_id: `browser-mall-credit-${stamp}`
    }
  });

  return {
    auth,
    adminToken,
    category: category.category,
    product: product.product,
    cartProduct: cartProduct.product,
    refundProduct: refundProduct.product,
    coupon: coupon.coupon,
    password
  };
}

async function runBrowserCheckout(chromePath, fixture) {
  const port = await getFreePort();
  const userDataDir = await mkdtemp(path.join(os.tmpdir(), "bbs-mall-e2e-"));
  const chrome = spawn(
    chromePath,
    [
      "--headless=new",
      "--disable-gpu",
      "--no-first-run",
      "--no-default-browser-check",
      "--disable-background-networking",
      "--disable-extensions",
      "--disable-sync",
      "--no-sandbox",
      `--remote-debugging-port=${port}`,
      `--user-data-dir=${userDataDir}`,
      "about:blank"
    ],
    { stdio: ["ignore", "pipe", "pipe"], windowsHide: true }
  );

  const stderr = [];
  chrome.stderr?.on("data", (chunk) => stderr.push(String(chunk)));

  let page;
  try {
    const browserWs = await waitForBrowserWebSocket(port, chrome, stderr);
    const pageWs = await createPageWebSocket(port, browserWs);
    page = new CDPClient(pageWs);
    await page.connect();
    const issues = collectBrowserIssues(page);
    await page.send("Page.enable");
    await page.send("Runtime.enable");
    await page.send("Network.enable");
    await page.send("Log.enable");
    await page.send("Page.addScriptToEvaluateOnNewDocument", {
      source: `window.localStorage.setItem(${JSON.stringify(AUTH_STORAGE_KEY)}, ${JSON.stringify(JSON.stringify(fixture.auth))});`
    });

    const campaignUrl = `${FRONTEND_BASE}/shop?category=${encodeURIComponent(fixture.category.slug)}&keyword=${encodeURIComponent("Browser Product")}`;
    await navigate(page, campaignUrl);
    await waitForText(page, fixture.product.title, "campaign filtered product");

    const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.product.id)}&coupon_code=${encodeURIComponent(fixture.coupon.code)}`;
    await navigate(page, shopUrl);
    await waitForText(page, fixture.product.title, "product detail");
    await waitForText(page, "商品详情", "product detail panel");

    await clickButtonInArticle(page, fixture.coupon.code, "^领取$");
    await waitForText(page, "优惠券已领取|已经在你的券包里", "coupon claimed");

    await fillByLabel(page, "收件人", "浏览器联调");
    await fillByLabel(page, "联系电话", "13800000000");
    await fillByLabel(page, "省份", "上海");
    await fillByLabel(page, "城市", "上海");
    await fillByLabel(page, "区县", "浦东新区");
    await fillByLabel(page, "详细地址", "张江路 1 号");
    await clickButton(page, "保存为收货地址|保存修改");
    await waitForText(page, "收货地址已保存|收货地址已更新", "address saved");

    const updatedAddressDetail = "张江路 2 号";
    await navigate(page, `${FRONTEND_BASE}/dashboard/addresses`);
    await waitForText(page, "新增收货地址|编辑收货地址", "dashboard address book");
    await waitForText(page, "浏览器联调", "dashboard saved address receiver");
    await waitForText(page, "默认地址|默认", "dashboard default address");
    await clickButtonInArticle(page, "浏览器联调", "^编辑$");
    await waitForText(page, "编辑收货地址", "dashboard address edit form");
    await fillByLabel(page, "详细地址", updatedAddressDetail);
    await clickButton(page, "^保存修改$");
    await waitForText(page, "收货地址已更新", "dashboard address updated");
    await waitForText(page, updatedAddressDetail, "dashboard updated address detail");

    await navigate(page, shopUrl);
    await waitForText(page, fixture.product.title, "product detail after address edit");
    await waitForText(page, updatedAddressDetail, "updated default address in shop");

    await clickButton(page, "^立即兑换$");
    await waitForText(page, "确认兑换", "checkout panel");
    await waitForText(page, `已预估优惠 ${COUPON_DISCOUNT} 积分`, "coupon applied");
    await clickButton(page, "^确认兑换$");
    await waitForText(page, "兑换成功|订单已创建", "order paid");

    const paidText = await bodyText(page);
    const order = await latestMallOrderForProduct(fixture, fixture.product.id);
    if (!order?.id) {
      throw new Error("Paid mall order was not returned by user order API");
    }

    await clickButton(page, "查看订单");
    await waitForText(page, "个人工作台", "dashboard shell");
    await waitForText(page, "已支付", "paid order row");
    await waitForText(page, fixture.product.title, "order item title");
    await waitForText(page, "支付记录|支付成功", "payment evidence");

    await shipMallOrder(fixture, order.id);
    await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
    await waitForText(page, "已发货", "shipped order row");
    await waitForText(page, "E2E Express|E2E", "shipping evidence");
    await clickButton(page, "^确认收货$");
    await waitForText(page, "已确认收货，订单已完成。|已完成", "order completed");

    const notifications = await waitForMallOrderNotifications(fixture, order.id, ["订单已支付", "订单已发货", "订单已完成"]);
    const notificationTitles = notifications.map((item) => item.title || item.type || "").filter(Boolean);
    await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
    await waitForText(page, "站内通知|站内消息", "messages panel");
    await waitForText(page, order.order_no || order.orderNo || String(order.id), "mall order notification content");
    await waitForText(page, "订单已支付|订单已发货|订单已完成", "mall order notification titles");
    await clickButtonInArticle(page, order.order_no || order.orderNo || String(order.id), "查看订单");
    await waitForText(page, "订单详情", "notification order target detail");
    await waitForText(page, "评价商品", "review action");

    const fulfillmentText = await bodyText(page);
    await clickButton(page, "^评价商品$");
    await waitForText(page, "商品评价", "product review panel");
    await waitForText(page, "发布评价", "review form");
    const reviewContent = `浏览器联调评价 ${Date.now()}：兑换、履约和确认收货链路顺畅。`;
    await fillByLabel(page, "评价内容", reviewContent);
    await waitForButtonEnabled(page, "^发布评价$", "review submit enabled");
    await clickButton(page, "^发布评价$");
    await waitForText(page, "评价已提交", "review submitted");

    const reviewText = await bodyText(page);
    const cartResult = await runBrowserCartCheckout(page, fixture);
    const refundResult = await runBrowserRefundFlow(page, fixture);
    const seriousIssues = issues.filter(isSeriousBrowserIssue);
    if (seriousIssues.length > 0) {
      throw new Error(`Browser reported ${seriousIssues.length} serious issue(s): ${JSON.stringify(seriousIssues.slice(0, 5), null, 2)}`);
    }
    return {
      orderId: String(order.id),
      orderNo: order.order_no || order.orderNo || "",
      paidText: summarizeCheckoutText(paidText),
      fulfillmentText: summarizeOrderLifecycleText(fulfillmentText),
      reviewText: summarizeReviewText(reviewText),
      cartOrderId: cartResult.orderId,
      cartText: cartResult.cartText,
      cartNotificationTitles: cartResult.notificationTitles,
      refundOrderId: refundResult.orderId,
      refundText: refundResult.refundText,
      refundNotificationTitles: refundResult.notificationTitles,
      notificationTitles
    };
  } finally {
    await page?.close().catch(() => {});
    chrome.kill();
    await delay(250);
    await rm(userDataDir, { recursive: true, force: true }).catch(() => {});
  }
}

function collectBrowserIssues(page) {
  const issues = [];
  page.on("Runtime.exceptionThrown", (event) => {
    issues.push({
      type: "pageerror",
      text: event.exceptionDetails?.exception?.description || event.exceptionDetails?.text || "Runtime exception"
    });
  });
  page.on("Runtime.consoleAPICalled", (event) => {
    if (event.type === "error" || event.type === "warning") {
      issues.push({
        type: `console:${event.type}`,
        text: (event.args || []).map((arg) => arg.value || arg.description || "").join(" ").slice(0, 800)
      });
    }
  });
  page.on("Log.entryAdded", (event) => {
    const entry = event.entry || {};
    if (entry.level === "error" || entry.level === "warning") {
      issues.push({ type: `log:${entry.level}`, text: entry.text || "", url: entry.url || "" });
    }
  });
  page.on("Network.responseReceived", (event) => {
    const response = event.response || {};
    if (response.status >= 400 && (response.url?.startsWith(API_BASE) || response.url?.startsWith(FRONTEND_BASE))) {
      issues.push({ type: "http", status: response.status, url: response.url });
    }
  });
  return issues;
}

function isSeriousBrowserIssue(issue) {
  const text = `${issue.text || ""} ${issue.url || ""}`;
  if (/favicon|manifest|websocket|ws:\/\//i.test(text)) return false;
  if (/Download the React DevTools/i.test(text)) return false;
  return true;
}

function summarizeCheckoutText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("兑换成功") || line.includes("订单已创建")) || "";
}

function summarizeOrderLifecycleText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("已确认收货") || line.includes("已完成")) || "";
}

function summarizeReviewText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("评价已提交")) || "";
}

function summarizeCartText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("兑换成功") || line.includes("订单已创建")) || "";
}

function summarizeRefundText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("已退款")) || lines.find((line) => line.includes("售后退款已通过")) || "";
}

async function runBrowserCartCheckout(page, fixture) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.cartProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.cartProduct.title, "cart product detail");
  await waitForText(page, "商品详情", "cart product detail panel");
  await clickButtonNearText(page, fixture.cartProduct.title, "^加购物车$");
  await waitForText(page, "商品已加入购物车|购物车", "cart item added");
  await waitForText(page, fixture.cartProduct.title, "cart item title");
  await waitForButtonEnabled(page, "^结算购物车$", "cart checkout enabled");
  await clickButton(page, "^结算购物车$");
  await waitForText(page, "确认兑换", "cart checkout panel");
  await waitForText(page, "x 1", "cart checkout quantity");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "cart order paid");
  const paidText = await bodyText(page);

  const cartOrder = await latestMallOrderForProduct(fixture, fixture.cartProduct.id);
  if (!cartOrder?.id) {
    throw new Error("Cart mall order was not returned by user order API");
  }
  const cartItems = await apiRequest("/mall/cart", {
    token: fixture.auth.accessToken
  });
  if (listItems(cartItems).some((item) => String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "") === String(fixture.cartProduct.id))) {
    throw new Error("Cart checkout did not clear purchased cart item");
  }

  const notifications = await waitForMallOrderNotifications(fixture, cartOrder.id, ["订单已支付"]);
  await clickButton(page, "查看订单");
  await waitForText(page, "个人工作台", "cart dashboard shell");
  await waitForText(page, fixture.cartProduct.title, "cart order item title");
  await waitForText(page, "已支付", "cart paid order row");
  await waitForText(page, "支付记录|支付成功", "cart payment evidence");

  return {
    orderId: String(cartOrder.id),
    orderNo: cartOrder.order_no || cartOrder.orderNo || "",
    cartText: summarizeCartText(paidText),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserRefundFlow(page, fixture) {
  const refundNote = `浏览器联调售后 ${Date.now()}：验证用户申请、运营审核和退款通知链路。`;
  const adminNote = `Browser E2E refund approved ${Date.now()}`;
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.refundProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.refundProduct.title, "refund product detail");
  await waitForText(page, "商品详情", "refund product detail panel");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "refund checkout panel");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "refund order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.refundProduct.id);
  if (!order?.id) {
    throw new Error("Refund mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);

  await clickButton(page, "查看订单");
  await waitForText(page, "个人工作台", "refund dashboard shell");
  await waitForText(page, "已支付", "refund paid order row");
  await waitForText(page, fixture.refundProduct.title, "refund order item title");
  await waitForText(page, "申请售后", "refund action");
  await clickButton(page, "^申请售后$");
  await waitForText(page, "售后原因", "refund request form");
  await fillByLabel(page, "问题说明", refundNote);
  await waitForButtonEnabled(page, "^提交申请$", "refund submit enabled");
  await clickButton(page, "^提交申请$");
  await waitForText(page, "售后申请已提交|售后待审核", "refund request submitted");

  const refund = await latestRefundForOrder(fixture, order.id);
  if (!refund?.id) {
    throw new Error(`Refund request was not returned for order ${order.id}`);
  }

  await approveMallRefund(fixture, refund.id, adminNote);
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["售后退款已通过"]);

  await navigate(page, `${FRONTEND_BASE}/dashboard/refunds`);
  await waitForText(page, "待审核|处理中|已拒绝", "refunds panel");
  await waitForText(page, orderNo, "approved refund order number");
  await waitForText(page, "已退款", "approved refund status");
  await waitForText(page, adminNote, "approved refund admin note");
  const refundText = await bodyText(page);

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "已退款", "refunded order status");
  await waitForText(page, "售后进度", "refunded order detail");
  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "售后退款已通过", "refund notification title");
  await waitForText(page, orderNo, "refund notification content");

  return {
    orderId: String(order.id),
    orderNo,
    refundText: summarizeRefundText(refundText),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function latestMallOrderForProduct(fixture, productId) {
  const data = await apiRequest("/mall/orders?limit=20&offset=0", {
    token: fixture.auth.accessToken
  });
  return listItems(data).find((order) => orderContainsProduct(order, productId));
}

async function latestRefundForOrder(fixture, orderId) {
  const data = await apiRequest("/mall/refunds?limit=50&offset=0", {
    token: fixture.auth.accessToken
  });
  return listItems(data).find((refund) => String(refund?.order_id ?? refund?.orderId ?? "") === String(orderId));
}

async function approveMallRefund(fixture, refundId, adminNote) {
  const data = await apiRequest(`/admin/mall/refunds/${encodeURIComponent(refundId)}/review`, {
    method: "POST",
    token: fixture.adminToken,
    body: {
      approved: true,
      admin_note: adminNote,
      restore_stock: true
    }
  });
  const status = Number(data?.refund?.status);
  if (status !== 3) {
    throw new Error(`Admin refund approval did not approve refund ${refundId}, status=${data?.refund?.status ?? "unknown"}`);
  }
  return data.refund;
}

async function waitForMallOrderNotifications(fixture, orderId, expectedTitles) {
  const deadline = Date.now() + 30000;
  const expected = expectedTitles.map((title) => String(title));
  let lastNotifications = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/notifications?limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastNotifications = listItems(data).filter((item) => notificationBelongsToOrder(item, orderId));
    const presentTitles = new Set(lastNotifications.map((item) => item.title || ""));
    if (expected.every((title) => presentTitles.has(title))) {
      return expected
        .map((title) => lastNotifications.find((item) => item.title === title))
        .filter(Boolean);
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for mall order notifications ${expected.join(", ")} for order ${orderId}. Last notifications: ${JSON.stringify(lastNotifications.slice(0, 10), null, 2)}`);
}

function notificationBelongsToOrder(item, orderId) {
  const entityType = item?.entity_type ?? item?.entityType;
  const entityId = item?.entity_id ?? item?.entityId;
  return entityType === "mall_order" && String(entityId) === String(orderId);
}

async function shipMallOrder(fixture, orderId) {
  const data = await apiRequest(`/admin/mall/orders/${encodeURIComponent(orderId)}/status`, {
    method: "PUT",
    token: fixture.adminToken,
    body: {
      status: 5,
      shipping_carrier: "E2E Express",
      tracking_no: `E2E${Date.now()}`,
      note: "Browser E2E order shipped"
    }
  });
  const status = Number(data?.order?.status);
  if (status !== 5) {
    throw new Error(`Admin ship did not move order to shipped, status=${data?.order?.status ?? "unknown"}`);
  }
  return data.order;
}

function listItems(data) {
  if (Array.isArray(data?.items)) return data.items;
  if (Array.isArray(data?.list)) return data.list;
  return [];
}

function orderContainsProduct(order, productId) {
  const normalizedProductId = String(productId);
  return Array.isArray(order?.items) && order.items.some((item) => String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "") === normalizedProductId);
}

async function apiRequest(pathname, { method = "GET", body, token } = {}) {
  const response = await fetch(`${API_BASE}${pathname}`, {
    method,
    headers: {
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: body === undefined ? undefined : JSON.stringify(body)
  });
  const text = await response.text();
  const data = parseResponseBody(text);
  if (!response.ok || (data && typeof data === "object" && "code" in data && data.code !== 0)) {
    throw new Error(`${method} ${pathname} failed (${response.status}): ${text.slice(0, 800)}`);
  }
  return data?.data ?? data;
}

function parseResponseBody(text) {
  if (!text) return null;
  try {
    return JSON.parse(text.replace(/(:\s*)(-?\d{16,})(?=[,}\]])/g, '$1"$2"'));
  } catch {
    return null;
  }
}

async function ensureFrontendServer() {
  const shopUrl = `${FRONTEND_BASE}/shop`;
  if (await httpReachable(shopUrl)) {
    return null;
  }
  if (truthyEnv("MALL_E2E_NO_AUTO_FRONTEND")) {
    await assertHttpReachable(shopUrl, "frontend");
  }
  if (!existsSync(VITE_BIN)) {
    throw new Error(`frontend is not reachable at ${shopUrl}, and Vite was not found at ${VITE_BIN}. Run npm install in frontend or start the frontend manually.`);
  }

  const frontendUrl = new URL(FRONTEND_BASE);
  const host = frontendUrl.hostname || "127.0.0.1";
  const port = frontendUrl.port || (frontendUrl.protocol === "https:" ? "443" : "80");
  const logs = [];
  const server = spawn(process.execPath, [VITE_BIN, "--host", host, "--port", port, "--strictPort"], {
    cwd: FRONTEND_DIR,
    env: {
      ...process.env,
      VITE_API_BASE: API_BASE
    },
    stdio: ["ignore", "pipe", "pipe"],
    windowsHide: true
  });

  server.stdout?.on("data", (chunk) => pushLog(logs, chunk));
  server.stderr?.on("data", (chunk) => pushLog(logs, chunk));
  try {
    await waitForHttpReachable(shopUrl, "frontend", 45000, server, logs);
  } catch (error) {
    await stopProcess(server);
    throw error;
  }
  console.log(`Started frontend dev server for mall e2e at ${FRONTEND_BASE}`);
  return {
    stop: () => stopProcess(server)
  };
}

async function assertHttpReachable(url, label) {
  try {
    const response = await fetch(url);
    if (response.status >= 500) {
      throw new Error(`HTTP ${response.status}`);
    }
  } catch (error) {
    throw new Error(`${label} is not reachable at ${url}: ${error.message}`);
  }
}

async function httpReachable(url) {
  try {
    const response = await fetch(url);
    return response.status < 500;
  } catch {
    return false;
  }
}

async function waitForHttpReachable(url, label, timeoutMs, child, logs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`${label} dev server exited early with code ${child.exitCode}. Logs:\n${logs.join("").slice(-3000)}`);
    }
    if (await httpReachable(url)) return;
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label} at ${url}. Logs:\n${logs.join("").slice(-3000)}`);
}

function pushLog(logs, chunk) {
  logs.push(String(chunk));
  while (logs.join("").length > 5000) {
    logs.shift();
  }
}

function truthyEnv(name) {
  return /^(1|true|yes|on)$/i.test(String(process.env[name] || ""));
}

async function findChromeExecutable() {
  const explicit = process.env.CHROME_EXECUTABLE || process.env.MALL_E2E_CHROME || process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE;
  const candidates = [
    explicit,
    ...(await puppeteerChromeCandidates()),
    "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
    "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
    path.join(os.homedir(), "AppData", "Local", "Google", "Chrome", "Application", "chrome.exe"),
    "C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
    "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser"
  ].filter(Boolean);
  const match = candidates.find((candidate) => existsSync(candidate));
  if (!match) {
    throw new Error("Chrome/Chromium executable not found. Set CHROME_EXECUTABLE to run frontend mall e2e.");
  }
  return match;
}

async function puppeteerChromeCandidates() {
  const base = path.join(os.homedir(), ".cache", "puppeteer", "chrome");
  if (!existsSync(base)) return [];
  const entries = await readdir(base, { withFileTypes: true }).catch(() => []);
  return entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => path.join(base, entry.name, "chrome-win64", "chrome.exe"))
    .filter((candidate) => existsSync(candidate))
    .sort()
    .reverse();
}

async function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      server.close(() => resolve(address.port));
    });
  });
}

async function waitForBrowserWebSocket(port, chrome, stderr) {
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    if (chrome.exitCode !== null) {
      throw new Error(`Chrome exited early with code ${chrome.exitCode}: ${stderr.join("").slice(0, 1000)}`);
    }
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/version`);
      if (response.ok) {
        const data = await response.json();
        if (data.webSocketDebuggerUrl) return data.webSocketDebuggerUrl;
      }
    } catch {
      // Chrome is still starting.
    }
    await delay(100);
  }
  throw new Error(`Timed out waiting for Chrome DevTools endpoint: ${stderr.join("").slice(0, 1000)}`);
}

async function createPageWebSocket(port, browserWs) {
  const response = await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: "PUT" }).catch(() => null);
  if (response?.ok) {
    const data = await response.json();
    if (data.webSocketDebuggerUrl) return data.webSocketDebuggerUrl;
  }

  const browser = new CDPClient(browserWs);
  await browser.connect();
  try {
    const { targetId } = await browser.send("Target.createTarget", { url: "about:blank" });
    const list = await fetch(`http://127.0.0.1:${port}/json/list`).then((item) => item.json());
    const target = list.find((item) => item.id === targetId);
    if (!target?.webSocketDebuggerUrl) {
      throw new Error("Created Chrome target did not expose websocket URL");
    }
    return target.webSocketDebuggerUrl;
  } finally {
    await browser.close();
  }
}

async function navigate(page, url) {
  await page.send("Page.navigate", { url });
  await waitFor(page, `document.readyState === "complete" || document.readyState === "interactive"`, `navigate ${url}`, 20000);
  await delay(500);
}

async function waitForText(page, pattern, label = pattern, timeoutMs = 20000) {
  const source = pattern instanceof RegExp ? pattern.source : String(pattern);
  await waitFor(page, `new RegExp(${JSON.stringify(source)}, "i").test(document.body?.innerText || "")`, label, timeoutMs);
}

async function waitFor(page, expression, label, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      if (await evaluate(page, `Boolean(${expression})`)) return;
    } catch (error) {
      lastError = error.message;
    }
    await delay(150);
  }
  const text = await bodyText(page).catch(() => "");
  throw new Error(`Timed out waiting for ${label}${lastError ? ` (${lastError})` : ""}. Body: ${text.slice(0, 1200)}`);
}

async function bodyText(page) {
  return evaluate(page, `document.body?.innerText || ""`);
}

async function clickButton(page, buttonPattern) {
  return evaluate(
    page,
    `(() => {
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const buttons = Array.from(document.querySelectorAll("button"));
      const button = buttons.find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`
  );
}

async function waitForButtonEnabled(page, buttonPattern, label = buttonPattern, timeoutMs = 20000) {
  await waitFor(
    page,
    `(() => {
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const button = Array.from(document.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      return Boolean(button && !button.disabled);
    })()`,
    label,
    timeoutMs
  );
}

async function clickButtonInArticle(page, articleText, buttonPattern) {
  return evaluate(
    page,
    `(() => {
      const articleNeedle = ${JSON.stringify(articleText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const article = Array.from(document.querySelectorAll("article")).find((item) => (item.innerText || "").includes(articleNeedle));
      if (!article) throw new Error("Article not found: ${escapeForScript(articleText)}");
      const button = Array.from(article.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found in article: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled in article: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`
  );
}

async function clickButtonNearText(page, containerText, buttonPattern) {
  return evaluate(
    page,
    `(() => {
      const containerNeedle = ${JSON.stringify(containerText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const containers = Array.from(document.querySelectorAll("article, section.product-detail-panel, section.cart-panel, section.checkout-panel, section.panel"));
      const container = containers.find((item) => (item.innerText || "").includes(containerNeedle) && Array.from(item.querySelectorAll("button")).some((button) => pattern.test((button.innerText || button.textContent || "").trim())));
      if (!container) throw new Error("Container not found: ${escapeForScript(containerText)}");
      const button = Array.from(container.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found near text: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled near text: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`
  );
}

async function fillByLabel(page, labelText, value) {
  return evaluate(
    page,
    `(() => {
      const labels = Array.from(document.querySelectorAll("label"));
      const label = labels.find((item) => (item.innerText || "").includes(${JSON.stringify(labelText)}));
      if (!label) throw new Error("Label not found: ${escapeForScript(labelText)}");
      const field = label.querySelector("input, textarea, select");
      if (!field) throw new Error("Field not found for label: ${escapeForScript(labelText)}");
      const prototype = field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
      const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
      descriptor.set.call(field, ${JSON.stringify(value)});
      field.dispatchEvent(new Event("input", { bubbles: true }));
      field.dispatchEvent(new Event("change", { bubbles: true }));
      return field.value;
    })()`
  );
}

async function evaluate(page, expression) {
  const response = await page.send("Runtime.evaluate", {
    expression,
    awaitPromise: true,
    returnByValue: true,
    userGesture: true
  });
  if (response.exceptionDetails) {
    const detail = response.exceptionDetails;
    throw new Error(detail.exception?.description || detail.text || "Runtime.evaluate failed");
  }
  return response.result?.value;
}

function escapeForScript(value) {
  return String(value).replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\n");
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function stopProcess(child) {
  return new Promise((resolve) => {
    if (!child || child.exitCode !== null) {
      resolve();
      return;
    }
    const timeout = setTimeout(() => resolve(), 3000);
    child.once("exit", () => {
      clearTimeout(timeout);
      resolve();
    });
    child.kill();
  });
}

class CDPClient {
  constructor(url) {
    this.url = url;
    this.nextId = 1;
    this.pending = new Map();
    this.listeners = new Map();
  }

  connect() {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url);
      this.ws.addEventListener("open", () => resolve());
      this.ws.addEventListener("error", () => reject(new Error(`Could not connect to ${this.url}`)), { once: true });
      this.ws.addEventListener("message", (event) => this.handleMessage(event.data));
      this.ws.addEventListener("close", () => {
        for (const { reject: rejectPending } of this.pending.values()) {
          rejectPending(new Error("CDP websocket closed"));
        }
        this.pending.clear();
      });
    });
  }

  send(method, params = {}) {
    const id = this.nextId++;
    this.ws.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject, method });
    });
  }

  on(method, listener) {
    const listeners = this.listeners.get(method) || [];
    listeners.push(listener);
    this.listeners.set(method, listeners);
  }

  handleMessage(raw) {
    const message = JSON.parse(raw);
    if (message.id) {
      const pending = this.pending.get(message.id);
      if (!pending) return;
      this.pending.delete(message.id);
      if (message.error) {
        pending.reject(new Error(`${pending.method} failed: ${message.error.message}`));
      } else {
        pending.resolve(message.result || {});
      }
      return;
    }
    const listeners = this.listeners.get(message.method) || [];
    for (const listener of listeners) {
      listener(message.params || {});
    }
  }

  close() {
    return new Promise((resolve) => {
      if (!this.ws || this.ws.readyState === WebSocket.CLOSED) {
        resolve();
        return;
      }
      this.ws.addEventListener("close", () => resolve(), { once: true });
      this.ws.close();
    });
  }
}

main().catch((error) => {
  console.error(error.stack || error.message || error);
  process.exitCode = 1;
});
