import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, readFile, readdir, rm } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const API_BASE = (process.env.API_BASE || process.env.VITE_API_BASE || "http://127.0.0.1:18080/api/v1").replace(/\/$/, "");
const ADMIN_BASE = (process.env.ADMIN_BASE || process.env.VITE_ADMIN_BASE || "http://127.0.0.1:8849").replace(/\/$/, "");
const ADMIN_ACCOUNT = process.env.ADMIN_ACCOUNT || "admin";
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "Admin123!";
const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.dirname(SCRIPT_DIR);
const ADMIN_DIR = path.join(REPO_ROOT, "vue-pure-admin");
const VITE_BIN = path.join(ADMIN_DIR, "node_modules", "vite", "bin", "vite.js");
const CHECKOUT_PRICE = 18;
const CREDIT_TOP_UP = 100;

async function main() {
  const adminSession = await assertAdminApiReady();
  const fixture = await prepareAdminMallFixture(adminSession.token);
  const adminServer = await ensureAdminServer();
  try {
    const chromePath = await findChromeExecutable();
    const result = await runBrowserAdminMall(chromePath, fixture);
    console.log(
      JSON.stringify(
        {
          ok: true,
          adminUserId: adminSession.userId,
          fixtureOrderId: fixture.orderId,
          fixtureRefundId: fixture.refundId,
          fixtureCouponId: fixture.couponId,
          fixtureReviewId: fixture.reviewId,
          overviewText: result.overviewText,
          visited: result.visited,
          exports: result.exports
        },
        null,
        2
      )
    );
  } finally {
    await adminServer?.stop();
  }
}

async function assertAdminApiReady() {
  const data = await apiRequest("/admin/auth/login", {
    method: "POST",
    body: {
      account: ADMIN_ACCOUNT,
      password: ADMIN_PASSWORD
    }
  });
  const token = data?.access_token || data?.accessToken;
  if (!token) {
    throw new Error("Admin login API did not return access_token");
  }
  const permissions = data?.permissions || [];
  for (const permission of [
    "mall:list_product_categories",
    "mall:list_product_reviews",
    "mall:update_product_review",
    "mall:list_orders",
    "mall:list_order_payments",
    "mall:update_order_status",
    "mall:list_products",
    "mall:list_coupons",
    "mall:list_coupon_usages",
    "mall:list_refunds",
    "mall:create_product_category",
    "mall:create_product",
    "mall:create_coupon",
    "governance:adjust_user_credits"
  ]) {
    if (!permissions.includes(permission) && !permissions.includes("*:*:*")) {
      throw new Error(`Admin account is missing required permission: ${permission}`);
    }
  }
  return {
    token,
    userId: String(data?.user?.id ?? data?.user?.Id ?? "")
  };
}

async function prepareAdminMallFixture(adminToken) {
  const stamp = Date.now();
  const slug = `admin-e2e-${stamp}`;
  const sku = `ADMIN-E2E-${stamp}`;
  const couponCode = `ADMINE2E${stamp}`;
  const productTitle = `Admin E2E Export Product ${stamp}`;
  await apiRequest("/admin/mall/categories", {
    method: "POST",
    token: adminToken,
    body: {
      slug,
      name: `Admin E2E ${stamp}`,
      description: "Admin browser E2E export category",
      status: 2,
      sort: 990
    }
  });

  const productResp = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku,
      title: productTitle,
      description: "Admin browser E2E export product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 990
    }
  });
  const product = productResp.product;
  if (!product?.id) {
    throw new Error("Admin mall fixture product creation did not return product.id");
  }

  const couponResp = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: couponCode,
      name: `${couponCode} Admin E2E Coupon`,
      description: "Admin browser E2E coupon usage export",
      discount_credits: 3,
      min_order_credits: CHECKOUT_PRICE,
      total_quota: 20,
      per_user_limit: 1,
      status: 2,
      starts_at: 0,
      ends_at: 0
    }
  });
  const coupon = couponResp.coupon;
  if (!coupon?.id) {
    throw new Error("Admin mall fixture coupon creation did not return coupon.id");
  }

  const password = `Passw0rd!${stamp}`;
  const registered = await apiRequest("/auth/register", {
    method: "POST",
    body: {
      username: `admine2e${stamp}`,
      email: `admine2e${stamp}@example.com`,
      password,
      nickname: `Admin E2E ${stamp}`
    }
  });
  const userToken = registered?.access_token || registered?.accessToken;
  const userId = registered?.user?.id;
  if (!userToken || !userId) {
    throw new Error("Admin mall fixture user registration did not return auth payload");
  }

  await apiRequest(`/admin/credits/users/${encodeURIComponent(userId)}/adjust`, {
    method: "POST",
    token: adminToken,
    body: {
      delta: CREDIT_TOP_UP,
      reason: "admin_browser_mall_export_topup",
      description: "Admin browser mall export fixture top-up",
      source_event_id: `admin-browser-mall-export-credit-${stamp}`
    }
  });

  const orderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-export-order-${stamp}`,
      coupon_code: couponCode,
      items: [{ product_id: product.id, quantity: 1 }],
      receiver: "管理端导出联调",
      phone: "13800000000",
      address: "上海市浦东新区导出路 1 号"
    }
  });
  const order = orderResp.order;
  if (!order?.id) {
    throw new Error("Admin mall fixture order creation did not return order.id");
  }
  const retryOrderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-export-order-${stamp}`,
      coupon_code: couponCode,
      items: [{ product_id: product.id, quantity: 1 }],
      receiver: "管理端导出联调",
      phone: "13800000000",
      address: "上海市浦东新区导出路 1 号"
    }
  });
  if (!retryOrderResp.duplicate || String(retryOrderResp.order?.id) !== String(order.id)) {
    throw new Error("Coupon order idempotency retry did not return the original order");
  }

  const paymentIdempotencyKey = `admin-export-pay-${order.id}-${stamp}`;
  await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/pay`, {
    method: "POST",
    token: userToken,
    body: {
      payment_method: "credits",
      idempotency_key: paymentIdempotencyKey
    }
  });

  await apiRequest(`/admin/mall/orders/${encodeURIComponent(order.id)}/status`, {
    method: "PUT",
    token: adminToken,
    body: {
      status: 5,
      shipping_carrier: "Admin E2E Express",
      tracking_no: `ADM${stamp}`,
      note: "管理端评价导出联调发货"
    }
  });

  await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/confirm`, {
    method: "POST",
    token: userToken
  });

  const reviewContent = `管理端评价导出联调 ${stamp}：后台审核和 CSV 留档可用。`;
  const reviewResp = await apiRequest(`/mall/products/${encodeURIComponent(product.id)}/reviews`, {
    method: "POST",
    token: userToken,
    body: {
      order_id: order.id,
      rating: 5,
      content: reviewContent
    }
  });
  const review = reviewResp.review;
  if (!review?.id) {
    throw new Error("Admin mall fixture review creation did not return review.id");
  }

  const refundReason = `管理端导出售后联调 ${stamp}`;
  const refundResp = await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/refunds`, {
    method: "POST",
    token: userToken,
    body: {
      reason: refundReason,
      note: "验证管理端售后 CSV 导出内容"
    }
  });
  const refund = refundResp.refund;
  if (!refund?.id) {
    throw new Error("Admin mall fixture refund creation did not return refund.id");
  }

  return {
    productId: String(product.id),
    productTitle,
    sku,
    couponId: String(coupon.id),
    couponCode,
    userId: String(userId),
    orderId: String(order.id),
    orderNo: order.order_no || order.orderNo || String(order.id),
    paymentIdempotencyKey,
    reviewId: String(review.id),
    reviewContent,
    refundId: String(refund.id),
    refundReason
  };
}

async function runBrowserAdminMall(chromePath, fixture) {
  const port = await getFreePort();
  const userDataDir = await mkdtemp(path.join(os.tmpdir(), "bbs-admin-mall-e2e-"));
  const downloadDir = await mkdtemp(path.join(os.tmpdir(), "bbs-admin-mall-downloads-"));
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
    await page.send("Page.setDownloadBehavior", { behavior: "allow", downloadPath: downloadDir });

    await navigate(page, `${ADMIN_BASE}/#/login`);
    await waitForText(page, "登录", "admin login page");
    await fillFirstInput(page, "input:not([type='password'])", ADMIN_ACCOUNT);
    await fillFirstInput(page, "input[type='password']", ADMIN_PASSWORD);
    await waitForButtonEnabled(page, "^登录$", "admin login button");
    await clickButton(page, "^登录$");
    await waitFor(page, `location.hash.includes("/welcome") || /登录成功|首页|商城管理/.test(document.body?.innerText || "")`, "admin login success", 30000);

    const visited = [];
    await visitAdminMallPage(page, "/#/mall/overview", ["商城概览", "待投递事件", "事件投递健康", "累计收入"], visited);
    const overviewText = summarizeBody(await bodyText(page), ["待投递事件", "事件投递健康", "累计收入", "待售后"]);
    await visitAdminMallPage(
      page,
      "/#/mall/categories",
      ["商品分类", "新增分类", "复制链接", "预览"],
      visited
    );
    await assertPromotionCopy(page, "分类推广链接已复制");
    await visitAdminMallPage(
      page,
      "/#/mall/products",
      ["商品管理", "新增商品", "库存流水", "复制链接", "预览"],
      visited
    );
    await fillFirstInput(page, 'input[placeholder="SKU / 商品名称"]', fixture.sku);
    await clickButton(page, "^查询$");
    await waitForText(page, fixture.productTitle, "fixture product visible in admin products");
    await assertPromotionCopy(page, "商品推广链接已复制");
    await clickButtonInRow(page, fixture.productTitle, "^库存流水$");
    await waitForText(page, "库存流水", "stock log drawer");
    await waitForText(page, "初始库存|下单锁定", "stock log entries");
    const stockLogExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出流水$",
      filenamePrefix: "mall-stock-logs-",
      successPattern: "已导出",
      expectedTexts: ["流水ID", "SKU", fixture.sku, fixture.productTitle, "初始库存", "下单锁定"]
    });
    await visitAdminMallPage(page, "/#/mall/reviews", ["评价管理", "商品ID", "用户ID", "评价内容", "导出评价"], visited);
    await fillFirstInput(page, 'input[placeholder="商品ID"]', fixture.productId);
    await clickButton(page, "^查询$");
    await waitForText(page, fixture.reviewContent, "fixture review visible in admin reviews");
    await clickButtonInRow(page, fixture.reviewContent, "^公开$");
    await waitForText(page, "评价已公开", "fixture review published");
    await waitForText(page, "已公开", "fixture review published status");
    const reviewExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出评价$",
      filenamePrefix: "mall-product-reviews-",
      successPattern: "已导出",
      expectedTexts: ["评价ID", "商品名称", "评分", "状态", fixture.productTitle, fixture.reviewContent, fixture.userId, "已公开"]
    });
    await visitAdminMallPage(
      page,
      "/#/mall/coupons",
      ["优惠券管理", "新增优惠券", "使用记录", "复制链接", "预览"],
      visited
    );
    await fillFirstInput(page, 'input[placeholder="优惠码 / 名称"]', fixture.couponCode);
    await clickButton(page, "^查询$");
    await waitForText(page, fixture.couponCode, "fixture coupon visible in admin coupons");
    await assertPromotionCopy(page, "优惠券推广链接已复制");
    await clickButtonInRow(page, fixture.couponCode, "^使用记录$");
    await waitForText(page, "优惠券使用记录", "coupon usage drawer");
    await waitForText(page, fixture.couponCode, "fixture coupon usage code");
    await waitForText(page, fixture.orderId, "fixture coupon usage order id");
    await waitForText(page, "已使用", "fixture coupon usage used status");
    const couponUsageExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出记录$",
      filenamePrefix: "mall-coupon-usages-",
      successPattern: "已导出",
      expectedTexts: ["使用记录ID", "优惠码", "订单ID", fixture.couponCode, fixture.orderId, fixture.userId, "已使用"]
    });
    await visitAdminMallPage(
      page,
      "/#/mall/orders",
      ["订单管理", "导出订单", "导出支付", "订单号", "用户 ID"],
      visited
    );
    await waitForText(page, fixture.orderNo, "fixture order visible in admin orders");
    const orderExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出订单$",
      filenamePrefix: "mall-orders-",
      successPattern: "已导出",
      expectedTexts: ["订单ID", "订单号", fixture.orderNo, fixture.productTitle]
    });
    const paymentExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出支付$",
      filenamePrefix: "mall-payments-",
      successPattern: "已导出",
      expectedTexts: ["支付ID", "订单号", "支付幂等键", fixture.orderNo, fixture.paymentIdempotencyKey]
    });
    await visitAdminMallPage(
      page,
      "/#/mall/refunds",
      ["售后管理", "导出售后", "订单号", "退款积分", "状态"],
      visited
    );
    await waitForText(page, fixture.orderNo, "fixture refund visible in admin refunds");
    const refundExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出售后$",
      filenamePrefix: "mall-refunds-",
      successPattern: "已导出",
      expectedTexts: ["售后ID", "订单号", fixture.orderNo, fixture.refundReason]
    });

    const seriousIssues = issues.filter(isSeriousBrowserIssue);
    if (seriousIssues.length > 0) {
      throw new Error(`Admin browser reported ${seriousIssues.length} serious issue(s): ${JSON.stringify(seriousIssues.slice(0, 5), null, 2)}`);
    }
    return { overviewText, visited, exports: { stockLogs: stockLogExport, productReviews: reviewExport, couponUsages: couponUsageExport, orders: orderExport, payments: paymentExport, refunds: refundExport } };
  } finally {
    await page?.close().catch(() => {});
    chrome.kill();
    await delay(250);
    await rm(userDataDir, { recursive: true, force: true }).catch(() => {});
    await rm(downloadDir, { recursive: true, force: true }).catch(() => {});
  }
}

async function visitAdminMallPage(page, route, expectedTexts, visited) {
  await navigate(page, `${ADMIN_BASE}${route}`);
  for (const text of expectedTexts) {
    await waitForText(page, text, `${route} ${text}`);
  }
  visited.push(route.replace(/^#?\/?/, ""));
}

async function assertPromotionCopy(page, successText) {
  await clickButton(page, "^复制链接$");
  await waitForText(page, successText, successText, 5000);
}

async function assertCsvExport(page, downloadDir, { buttonPattern, filenamePrefix, successPattern, expectedTexts }) {
  const before = new Set(await safeReadDir(downloadDir));
  await clickButton(page, buttonPattern);
  await waitForText(page, successPattern, `${filenamePrefix} export success`, 10000);
  const filename = await waitForDownloadedFile(downloadDir, before, filenamePrefix);
  const filePath = path.join(downloadDir, filename);
  const content = await readFile(filePath, "utf8");
  for (const expected of expectedTexts) {
    if (!content.includes(expected)) {
      throw new Error(`Exported CSV ${filename} did not include ${JSON.stringify(expected)}. Preview: ${content.slice(0, 500)}`);
    }
  }
  return {
    filename,
    rows: content.split(/\r?\n/).filter(Boolean).length
  };
}

async function waitForDownloadedFile(downloadDir, before, filenamePrefix) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    const entries = await safeReadDir(downloadDir);
    const match = entries
      .filter((name) => !before.has(name))
      .filter((name) => name.startsWith(filenamePrefix))
      .filter((name) => !name.endsWith(".crdownload"))
      .sort()
      .pop();
    if (match) {
      return match;
    }
    await delay(200);
  }
  throw new Error(`Timed out waiting for downloaded ${filenamePrefix} CSV. Current files: ${(await safeReadDir(downloadDir)).join(", ")}`);
}

async function safeReadDir(dir) {
  return readdir(dir).catch(() => []);
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
    if (event.type === "error") {
      issues.push({
        type: "console:error",
        text: (event.args || []).map((arg) => arg.value || arg.description || "").join(" ").slice(0, 800)
      });
    }
  });
  page.on("Log.entryAdded", (event) => {
    const entry = event.entry || {};
    if (entry.level === "error") {
      issues.push({ type: "log:error", text: entry.text || "", url: entry.url || "" });
    }
  });
  page.on("Network.responseReceived", (event) => {
    const response = event.response || {};
    if (response.status >= 400 && isAdminApiUrl(response.url || "")) {
      issues.push({ type: "http", status: response.status, url: response.url });
    }
  });
  return issues;
}

function isAdminApiUrl(url) {
  return url.startsWith(API_BASE) || url.startsWith(`${ADMIN_BASE}/api/`) || url.includes("/api/v1/admin/");
}

function isSeriousBrowserIssue(issue) {
  const text = `${issue.text || ""} ${issue.url || ""}`;
  if (/favicon|manifest|websocket|ws:\/\//i.test(text)) return false;
  if (/Download the Vue Devtools|DevTools/i.test(text)) return false;
  return true;
}

function summarizeBody(text, needles) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return needles.map((needle) => lines.find((line) => line.includes(needle)) || needle).join(" · ");
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

async function ensureAdminServer() {
  if (await httpReachable(ADMIN_BASE)) {
    return null;
  }
  if (truthyEnv("ADMIN_MALL_E2E_NO_AUTO_FRONTEND")) {
    await assertHttpReachable(ADMIN_BASE, "admin frontend");
  }
  if (!existsSync(VITE_BIN)) {
    throw new Error(`admin frontend is not reachable at ${ADMIN_BASE}, and Vite was not found at ${VITE_BIN}. Run pnpm install in vue-pure-admin or start the admin frontend manually.`);
  }

  const adminUrl = new URL(ADMIN_BASE);
  const host = adminUrl.hostname || "127.0.0.1";
  const port = adminUrl.port || (adminUrl.protocol === "https:" ? "443" : "80");
  const logs = [];
  const server = spawn(process.execPath, [VITE_BIN, "--host", host, "--port", port, "--strictPort"], {
    cwd: ADMIN_DIR,
    env: {
      ...process.env,
      VITE_API_PROXY_TARGET: API_BASE.replace(/\/api\/v1$/, "")
    },
    stdio: ["ignore", "pipe", "pipe"],
    windowsHide: true
  });

  server.stdout?.on("data", (chunk) => pushLog(logs, chunk));
  server.stderr?.on("data", (chunk) => pushLog(logs, chunk));
  try {
    await waitForHttpReachable(ADMIN_BASE, "admin frontend", 60000, server, logs);
  } catch (error) {
    await stopProcess(server);
    throw error;
  }
  console.log(`Started admin dev server for mall e2e at ${ADMIN_BASE}`);
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
  const explicit = process.env.CHROME_EXECUTABLE || process.env.ADMIN_MALL_E2E_CHROME || process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE;
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
    throw new Error("Chrome/Chromium executable not found. Set CHROME_EXECUTABLE to run admin mall e2e.");
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
  await waitFor(page, `document.readyState === "complete" || document.readyState === "interactive"`, `navigate ${url}`, 30000);
  await delay(700);
}

async function waitForText(page, pattern, label = pattern, timeoutMs = 30000) {
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
  throw new Error(`Timed out waiting for ${label}${lastError ? ` (${lastError})` : ""}. Body: ${text.slice(0, 1400)}`);
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

async function clickButtonInRow(page, rowText, buttonPattern) {
  return evaluate(
    page,
    `(() => {
      const rowNeedle = ${JSON.stringify(rowText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const rows = Array.from(document.querySelectorAll("tr, .el-table__row"));
      const row = rows.find((item) => (item.innerText || "").includes(rowNeedle) && Array.from(item.querySelectorAll("button")).some((button) => pattern.test((button.innerText || button.textContent || "").trim())));
      if (!row) throw new Error("Row not found: ${escapeForScript(rowText)}");
      const button = Array.from(row.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found in row: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled in row: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`
  );
}

async function waitForButtonEnabled(page, buttonPattern, label = buttonPattern, timeoutMs = 30000) {
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

async function fillFirstInput(page, selector, value) {
  return evaluate(
    page,
    `(() => {
      const field = document.querySelector(${JSON.stringify(selector)});
      if (!field) throw new Error("Input not found: ${escapeForScript(selector)}");
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
