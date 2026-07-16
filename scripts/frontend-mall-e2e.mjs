import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { friendlyMallReviewError } from "../frontend/src/lib/mallErrors.js";

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
const INSUFFICIENT_CHECKOUT_PRICE = 260;
const PAYMENT_RECOVERY_TOP_UP = 300;
const MEMBERSHIP_DURATION_MS = 30 * 24 * 60 * 60 * 1000;

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
          rejectedRefundProductId: fixture.rejectedRefundProduct.id,
          digitalProductId: fixture.digitalProduct.id,
          themeProductId: fixture.themeProduct.id,
          membershipProductId: fixture.membershipProduct.id,
          couponCode: fixture.coupon.code,
          directCouponCode: fixture.directCoupon.code,
          cancelCouponCode: fixture.cancelCoupon.code,
          userId: fixture.auth.user.id,
          orderId: result.orderId,
          orderNo: result.orderNo,
          directCouponOrderId: result.directCouponOrderId,
          directCouponText: result.directCouponText,
          directCouponReuseHttpStatus: result.directCouponReuseHttpStatus,
          directCouponReuseText: result.directCouponReuseText,
          dashboardPayOrderId: result.dashboardPayOrderId,
          dashboardPayText: result.dashboardPayText,
          dashboardPayLockedStock: result.dashboardPayLockedStock,
          dashboardPayNotificationTitles: result.dashboardPayNotificationTitles,
          insufficientPaymentOrderId: result.insufficientPaymentOrderId,
          insufficientPaymentText: result.insufficientPaymentText,
          insufficientPaymentLockedStock: result.insufficientPaymentLockedStock,
          insufficientPaymentBalanceBeforeFailure: result.insufficientPaymentBalanceBeforeFailure,
          insufficientPaymentBalanceAfterTopUp: result.insufficientPaymentBalanceAfterTopUp,
          insufficientPaymentFailedPaymentId: result.insufficientPaymentFailedPaymentId,
          insufficientPaymentRecoveredPaymentId: result.insufficientPaymentRecoveredPaymentId,
          insufficientPaymentNotificationTitles: result.insufficientPaymentNotificationTitles,
          cancelCouponOrderId: result.cancelCouponOrderId,
          cancelCouponText: result.cancelCouponText,
          cancelCouponLockedStock: result.cancelCouponLockedStock,
          cancelCouponRestoredStock: result.cancelCouponRestoredStock,
          cancelCouponReleasedUsageId: result.cancelCouponReleasedUsageId,
          cancelCouponReclaimedUsageId: result.cancelCouponReclaimedUsageId,
          paidText: result.paidText,
          fulfillmentText: result.fulfillmentText,
          promotedAddressText: result.promotedAddressText,
          reviewText: result.reviewText,
          publicReviewText: result.publicReviewText,
          reviewDuplicateHttpStatus: result.reviewDuplicateHttpStatus,
          reviewDuplicateLegacyCode: result.reviewDuplicateLegacyCode,
          reviewDuplicateText: result.reviewDuplicateText,
          reviewHiddenNotificationTitles: result.reviewHiddenNotificationTitles,
          publicReviewHiddenText: result.publicReviewHiddenText,
          reviewNotificationTitles: result.reviewNotificationTitles,
          cartOrderId: result.cartOrderId,
          cartText: result.cartText,
          cartNotificationTitles: result.cartNotificationTitles,
          refundOrderId: result.refundOrderId,
          refundText: result.refundText,
          refundLockedStock: result.refundLockedStock,
          refundRestoredStock: result.refundRestoredStock,
          refundNotificationTitles: result.refundNotificationTitles,
          rejectedRefundOrderId: result.rejectedRefundOrderId,
          rejectedRefundText: result.rejectedRefundText,
          rejectedRefundNotificationTitles: result.rejectedRefundNotificationTitles,
          digitalOrderId: result.digitalOrderId,
          digitalOrderNo: result.digitalOrderNo,
          digitalGrantKey: fixture.digitalGrantKey,
          digitalEntitlementCode: result.digitalEntitlementCode,
          digitalEntitlementCodes: result.digitalEntitlementCodes,
          digitalEntitlementCount: result.digitalEntitlementCount,
          digitalRefundId: result.digitalRefundId,
          digitalText: result.digitalText,
          digitalRevokedText: result.digitalRevokedText,
          publicBadgeText: result.publicBadgeText,
          publicBadgeRevokedText: result.publicBadgeRevokedText,
          themeOrderId: result.themeOrderId,
          themeOrderNo: result.themeOrderNo,
          themeGrantKey: fixture.themeGrantKey,
          themeEntitlementCode: result.themeEntitlementCode,
          themeProfileClass: result.themeProfileClass,
          themeRevokedProfileClass: result.themeRevokedProfileClass,
          themeRevocationReason: result.themeRevocationReason,
          membershipOrderId: result.membershipOrderId,
          membershipOrderNo: result.membershipOrderNo,
          membershipGrantKey: fixture.membershipGrantKey,
          membershipEntitlementCode: result.membershipEntitlementCode,
          membershipExpiresAt: result.membershipExpiresAt,
          membershipRenewalExpiresAt: result.membershipRenewalExpiresAt,
          membershipRefundApiStatus: result.membershipRefundApiStatus,
          membershipRefundApiMessage: result.membershipRefundApiMessage,
          membershipBackgroundUrl: result.membershipBackgroundUrl,
          membershipProfileBackgroundStyle: result.membershipProfileBackgroundStyle,
          membershipRevokedProfileBackgroundStyle: result.membershipRevokedProfileBackgroundStyle,
          membershipText: result.membershipText,
          membershipRevocationReason: result.membershipRevocationReason,
          membershipRevokedBackgroundApiStatus: result.membershipRevokedBackgroundApiStatus,
          membershipRevokedBackgroundApiMessage: result.membershipRevokedBackgroundApiMessage,
          membershipRevokedApiStatus: result.membershipRevokedApiStatus,
          membershipRevokedApiMessage: result.membershipRevokedApiMessage,
          membershipRevokedDraftPublishApiStatus: result.membershipRevokedDraftPublishApiStatus,
          membershipRevokedDraftPublishApiMessage: result.membershipRevokedDraftPublishApiMessage,
          membershipRevokedEditApiStatus: result.membershipRevokedEditApiStatus,
          membershipRevokedEditApiMessage: result.membershipRevokedEditApiMessage,
          membershipRevokedText: result.membershipRevokedText,
          bountyDraftTopicId: result.bountyDraftTopicId,
          bountyDraftTopicTitle: result.bountyDraftTopicTitle,
          bountyTopicId: result.bountyTopicId,
          bountyTopicTitle: result.bountyTopicTitle,
          bountyAcceptedCommentId: result.bountyAcceptedCommentId,
          bountyAcceptedTopicStatus: result.bountyAcceptedTopicStatus,
          bountyAnswererId: fixture.answererAuth.user.id,
          bountyQuestionerLedgerId: result.bountyQuestionerLedgerId,
          bountyAnswererLedgerId: result.bountyAnswererLedgerId,
          bountyInsufficientCreditBalance: result.bountyInsufficientCreditBalance,
          bountyInsufficientCreditText: result.bountyInsufficientCreditText,
          bountyText: result.bountyText,
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
  const directCouponCode = `DIRECT${stamp}`;
  const cancelCouponCode = `CANCEL${stamp}`;
  const productTitle = `E2E Browser Product ${stamp}`;
  const directCouponProductTitle = `E2E Direct Coupon Product ${stamp}`;
  const dashboardPayProductTitle = `E2E Dashboard Pay Product ${stamp}`;
  const insufficientCreditProductTitle = `E2E Insufficient Credit Product ${stamp}`;
  const cancelCouponProductTitle = `E2E Cancel Coupon Product ${stamp}`;
  const cartProductTitle = `E2E Cart Product ${stamp}`;
  const refundProductTitle = `E2E Refund Product ${stamp}`;
  const rejectedRefundProductTitle = `E2E Rejected Refund Product ${stamp}`;
  const digitalGrantKey = `badge-e2e-${stamp}`;
  const digitalProductTitle = `E2E Badge Entitlement ${stamp}`;
  const themeGrantKey = "theme-pro";
  const themeProductTitle = `E2E Theme Pro Entitlement ${stamp}`;
  const membershipGrantKey = `vip-e2e-${stamp}`;
  const membershipProductTitle = `E2E Membership Month ${stamp}`;

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

  const directCouponProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-DIRECT`,
      title: directCouponProductTitle,
      description: "Browser E2E direct coupon product",
      category: "digital",
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const dashboardPayProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-DASHBOARD-PAY`,
      title: dashboardPayProductTitle,
      description: "Browser E2E dashboard payment product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const insufficientCreditProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-INSUFFICIENT-CREDIT`,
      title: insufficientCreditProductTitle,
      description: "Browser E2E insufficient credit recovery product",
      category: slug,
      cover_url: "",
      price_credits: INSUFFICIENT_CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const cancelCouponProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-CANCEL-COUPON`,
      title: cancelCouponProductTitle,
      description: "Browser E2E coupon cancellation product",
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

  const rejectedRefundProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-REJECTED`,
      title: rejectedRefundProductTitle,
      description: "Browser E2E rejected mall refund product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9996
    }
  });

  const digitalProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: digitalGrantKey,
      title: digitalProductTitle,
      description: "Browser E2E digital badge entitlement",
      category: "badge",
      cover_url: "",
      grant_type: "badge",
      grant_key: digitalGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9995
    }
  });

  const membershipProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: membershipGrantKey,
      title: membershipProductTitle,
      description: "Browser E2E membership month entitlement",
      category: "digital",
      cover_url: "",
      grant_type: "membership",
      grant_key: membershipGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9994
    }
  });

  const themeProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-THEME-PRO`,
      title: themeProductTitle,
      description: "Browser E2E profile theme entitlement",
      category: "digital",
      cover_url: "",
      grant_type: "theme",
      grant_key: themeGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9993
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

  const directCoupon = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: directCouponCode,
      name: `${directCouponCode} Direct Checkout Coupon`,
      description: "Browser E2E direct checkout coupon",
      discount_credits: COUPON_DISCOUNT,
      min_order_credits: CHECKOUT_PRICE,
      total_quota: 10,
      per_user_limit: 1,
      status: 2,
      starts_at: 0,
      ends_at: 0
    }
  });

  const cancelCoupon = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: cancelCouponCode,
      name: `${cancelCouponCode} Cancel Recovery Coupon`,
      description: "Browser E2E coupon cancellation recovery",
      discount_credits: COUPON_DISCOUNT,
      min_order_credits: CHECKOUT_PRICE,
      total_quota: 1,
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

  const answererRegistered = await apiRequest("/auth/register", {
    method: "POST",
    body: {
      username: `e2eanswer${stamp}`,
      email: `e2eanswer${stamp}@example.com`,
      password,
      nickname: `E2E Answer ${stamp}`
    }
  });
  const answererAuth = {
    accessToken: answererRegistered.access_token || answererRegistered.accessToken,
    expiresAt: answererRegistered.expires_at || answererRegistered.expiresAt,
    user: answererRegistered.user
  };
  if (!answererAuth.accessToken || !answererAuth.user?.id) {
    throw new Error("Answerer registration did not return auth payload");
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
    answererAuth,
    adminToken,
    category: category.category,
    product: product.product,
    directCouponProduct: directCouponProduct.product,
    dashboardPayProduct: dashboardPayProduct.product,
    insufficientCreditProduct: insufficientCreditProduct.product,
    cancelCouponProduct: cancelCouponProduct.product,
    cartProduct: cartProduct.product,
    refundProduct: refundProduct.product,
    rejectedRefundProduct: rejectedRefundProduct.product,
    digitalProduct: digitalProduct.product,
    digitalGrantKey,
    themeProduct: themeProduct.product,
    themeGrantKey,
    membershipProduct: membershipProduct.product,
    membershipGrantKey,
    coupon: coupon.coupon,
    directCoupon: directCoupon.coupon,
    cancelCoupon: cancelCoupon.coupon,
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
    const expectedBrowserIssues = [];
    const issues = collectBrowserIssues(page, expectedBrowserIssues);
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
    await clickButton(page, "^收藏商品$");
    await waitForText(page, "商品已收藏", "product favorited");
    await clickButtonInArticle(page, fixture.product.title, "^查看详情$");
    await waitForText(page, "商品详情", "favorite product detail target");
    await waitForText(page, fixture.product.title, "favorite product detail target title");
    await clickButton(page, "^取消收藏$");
    await waitForText(page, "已取消收藏", "product unfavorited");

    const directCouponResult = await runBrowserDirectCouponCheckout(page, fixture);
    const dashboardPayResult = await runBrowserDashboardPaymentFlow(page, fixture);
    const insufficientPaymentResult = await runBrowserInsufficientCreditRecoveryFlow(page, fixture, expectedBrowserIssues);
    const cancelCouponResult = await runBrowserCouponCancellationFlow(page, fixture);
    await navigate(page, shopUrl);
    await waitForText(page, fixture.product.title, "product detail after direct coupon checkout");

    await clickButtonInArticle(page, fixture.coupon.code, "^领取$");
    await waitForText(page, "优惠券已领取|已经在你的券包里", "coupon claimed");

    await navigate(page, `${FRONTEND_BASE}/dashboard/coupons`);
    await waitForText(page, "优惠券|个人工作台", "dashboard coupons panel");
    await waitForText(page, fixture.coupon.code, "claimed coupon in dashboard");
    await clickButtonInArticle(page, fixture.coupon.code, "^去使用$");
    await waitForText(page, "优惠券使用引导", "coupon usage guide");
    await waitForText(page, "带券兑换", "coupon usage guide action");
    await clickButton(page, "^带券兑换$");
    await waitForText(page, "确认兑换", "coupon guide checkout preview");
    await waitForText(page, `已预估优惠 ${COUPON_DISCOUNT} 积分`, "coupon guide checkout discount preview");
    await clickButton(page, "^取消$");

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

    const temporaryAddressReceiver = "浏览器联调备用";
    const temporaryAddressDetail = "金科路 9 号";
    await fillByLabel(page, "收件人", temporaryAddressReceiver);
    await fillByLabel(page, "联系电话", "13900000000");
    await fillByLabel(page, "省份", "上海");
    await fillByLabel(page, "城市", "上海");
    await fillByLabel(page, "区县", "浦东新区");
    await fillByLabel(page, "详细地址", temporaryAddressDetail);
    await clickButton(page, "^保存地址$");
    await waitForText(page, "收货地址已新增", "dashboard secondary address created");
    await waitForText(page, temporaryAddressDetail, "dashboard secondary address detail");
    await clickButtonInArticle(page, temporaryAddressReceiver, "^设默认$");
    await waitForText(page, "默认收货地址已更新", "dashboard secondary address set default");
    await waitForText(page, `${temporaryAddressReceiver} · 默认地址`, "dashboard secondary address default");
    await clickButtonInArticle(page, temporaryAddressReceiver, "^删除$");
    await waitForText(page, "收货地址已删除", "dashboard secondary address deleted");
    await waitForText(page, "浏览器联调 · 默认地址", "dashboard primary address promoted default");
    await waitForAddressDeleted(page, temporaryAddressReceiver, "dashboard secondary address row removed");
    const addressesAfterDelete = listItems(await apiRequest("/mall/addresses?limit=20&offset=0", { token: fixture.auth.accessToken }));
    if (addressesAfterDelete.some((address) => String(address?.receiver || "") === temporaryAddressReceiver)) {
      throw new Error(`Deleted secondary address still returned by API: ${JSON.stringify(addressesAfterDelete)}`);
    }
    const promotedAddress = addressesAfterDelete.find((address) => String(address?.receiver || "") === "浏览器联调");
    if (!promotedAddress || !(promotedAddress.is_default || promotedAddress.isDefault) || String(promotedAddress.detail || "") !== updatedAddressDetail) {
      throw new Error(`Primary address was not promoted after deleting default address: ${JSON.stringify(addressesAfterDelete)}`);
    }
    const promotedAddressText = `${promotedAddress.receiver} · ${promotedAddress.detail}`;

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
    await waitForText(page, "再次兑换", "repeat purchase action");
    await clickButton(page, "^再次兑换$");
    await waitForText(page, "商品详情", "repeat purchase product detail");
    await waitForText(page, fixture.product.title, "repeat purchase product title");

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
    await waitForText(page, "查看商品", "order detail product target action");
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

    const createdReview = await latestProductReviewForOrder(fixture, order.id, fixture.product.id);
    await publishMallReview(fixture, createdReview.id);
    const reviewNotifications = await waitForMallReviewNotifications(fixture, createdReview.id, ["商品评价已展示"]);
    const reviewNotificationTitles = reviewNotifications.map((item) => item.title || item.type || "").filter(Boolean);
    await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
    await waitForText(page, "站内通知|站内消息", "messages panel after review publish");
    await waitForText(page, "商品评价已展示", "product review notification title");
    await clickButtonInArticle(page, "商品评价已展示", "查看评价");
    await waitForText(page, "个人工作台", "dashboard shell after review notification");
    await waitForText(page, "个人列表|评价", "review notification target");
    await waitForText(page, "当前定位", "focused review marker");
    await waitForText(page, reviewContent, "focused review content");

    const reviewText = await bodyText(page);
    await navigate(page, `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.product.id)}`);
    await waitForText(page, "商品详情", "review product detail after publish");
    await waitForText(page, fixture.product.title, "review product title after publish");
    const publicReviewText = await waitForPublicProductReview(page, reviewContent, "published review in public product reviews");
    const duplicateReview = await assertDuplicateProductReviewRejected(fixture, order.id, fixture.product.id, reviewContent);
    await hideMallReview(fixture, createdReview.id);
    const hiddenReviewNotifications = await waitForMallReviewNotifications(fixture, createdReview.id, ["商品评价未展示"]);
    const reviewHiddenNotificationTitles = hiddenReviewNotifications.map((item) => item.title || item.type || "").filter(Boolean);
    await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
    await waitForText(page, "商品评价未展示", "product review hidden notification title");
    await clickButtonInArticle(page, "商品评价未展示", "查看评价");
    await waitForText(page, "个人列表|评价", "hidden review notification target");
    await waitForText(page, "当前定位", "hidden review focused marker");
    await waitForText(page, "已隐藏", "hidden review status badge");
    await waitForText(page, reviewContent, "hidden review still visible to owner");
    await navigate(page, `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.product.id)}&review_hidden=${Date.now()}`);
    await waitForText(page, "商品详情", "review product detail after hide");
    await waitForText(page, fixture.product.title, "review product title after hide");
    const publicReviewHiddenText = await waitForPublicProductReviewHidden(page, reviewContent, "hidden review removed from public product reviews");
    const cartResult = await runBrowserCartCheckout(page, fixture);
    const refundResult = await runBrowserRefundFlow(page, fixture);
    const rejectedRefundResult = await runBrowserRejectedRefundFlow(page, fixture);
    const digitalResult = await runBrowserDigitalEntitlementFlow(page, fixture);
    const themeResult = await runBrowserThemeEntitlementFlow(page, fixture);
    const membershipResult = await runBrowserMembershipBountyFlow(page, fixture, expectedBrowserIssues);
    const seriousIssues = issues.filter(isSeriousBrowserIssue);
    if (seriousIssues.length > 0) {
      throw new Error(`Browser reported ${seriousIssues.length} serious issue(s): ${JSON.stringify(seriousIssues.slice(0, 5), null, 2)}`);
    }
    return {
      orderId: String(order.id),
      orderNo: order.order_no || order.orderNo || "",
      directCouponOrderId: directCouponResult.orderId,
      directCouponText: directCouponResult.text,
      directCouponReuseHttpStatus: directCouponResult.reuseStatus,
      directCouponReuseText: directCouponResult.reuseText,
      dashboardPayOrderId: dashboardPayResult.orderId,
      dashboardPayText: dashboardPayResult.text,
      dashboardPayLockedStock: dashboardPayResult.lockedStock,
      dashboardPayNotificationTitles: dashboardPayResult.notificationTitles,
      insufficientPaymentOrderId: insufficientPaymentResult.orderId,
      insufficientPaymentText: insufficientPaymentResult.text,
      insufficientPaymentLockedStock: insufficientPaymentResult.lockedStock,
      insufficientPaymentBalanceBeforeFailure: insufficientPaymentResult.balanceBeforeFailure,
      insufficientPaymentBalanceAfterTopUp: insufficientPaymentResult.balanceAfterTopUp,
      insufficientPaymentFailedPaymentId: insufficientPaymentResult.failedPaymentId,
      insufficientPaymentRecoveredPaymentId: insufficientPaymentResult.recoveredPaymentId,
      insufficientPaymentNotificationTitles: insufficientPaymentResult.notificationTitles,
      cancelCouponOrderId: cancelCouponResult.orderId,
      cancelCouponText: cancelCouponResult.text,
      cancelCouponLockedStock: cancelCouponResult.lockedStock,
      cancelCouponRestoredStock: cancelCouponResult.restoredStock,
      cancelCouponReleasedUsageId: cancelCouponResult.releasedUsageId,
      cancelCouponReclaimedUsageId: cancelCouponResult.reclaimedUsageId,
      paidText: summarizeCheckoutText(paidText),
      fulfillmentText: summarizeOrderLifecycleText(fulfillmentText),
      promotedAddressText,
      reviewText: summarizeReviewText(reviewText),
      publicReviewText: summarizePublicProductReviewText(publicReviewText, reviewContent),
      reviewDuplicateHttpStatus: duplicateReview.status,
      reviewDuplicateLegacyCode: duplicateReview.legacyCode,
      reviewDuplicateText: duplicateReview.frontendText,
      reviewHiddenNotificationTitles,
      publicReviewHiddenText,
      reviewNotificationTitles,
      cartOrderId: cartResult.orderId,
      cartText: cartResult.cartText,
      cartNotificationTitles: cartResult.notificationTitles,
      refundOrderId: refundResult.orderId,
      refundText: refundResult.refundText,
      refundLockedStock: refundResult.lockedStock,
      refundRestoredStock: refundResult.restoredStock,
      refundNotificationTitles: refundResult.notificationTitles,
      rejectedRefundOrderId: rejectedRefundResult.orderId,
      rejectedRefundText: rejectedRefundResult.refundText,
      rejectedRefundNotificationTitles: rejectedRefundResult.notificationTitles,
      digitalOrderId: digitalResult.orderId,
      digitalOrderNo: digitalResult.orderNo,
      digitalEntitlementCode: digitalResult.entitlementCode,
      digitalEntitlementCodes: digitalResult.entitlementCodes,
      digitalEntitlementCount: digitalResult.entitlementCount,
      digitalRefundId: digitalResult.refundId,
      digitalText: digitalResult.digitalText,
      digitalRevokedText: digitalResult.revokedText,
      publicBadgeText: digitalResult.publicBadgeText,
      publicBadgeRevokedText: digitalResult.publicBadgeRevokedText,
      themeOrderId: themeResult.orderId,
      themeOrderNo: themeResult.orderNo,
      themeEntitlementCode: themeResult.entitlementCode,
      themeProfileClass: themeResult.profileClass,
      themeRevokedProfileClass: themeResult.revokedProfileClass,
      themeRevocationReason: themeResult.revocationReason,
      membershipOrderId: membershipResult.orderId,
      membershipOrderNo: membershipResult.orderNo,
      membershipEntitlementCode: membershipResult.entitlementCode,
      membershipExpiresAt: membershipResult.expiresAt,
      membershipRenewalExpiresAt: membershipResult.renewalExpiresAt,
      membershipRefundApiStatus: membershipResult.refundApiStatus,
      membershipRefundApiMessage: membershipResult.refundApiMessage,
      membershipBackgroundUrl: membershipResult.membershipBackgroundUrl,
      membershipProfileBackgroundStyle: membershipResult.membershipProfileBackgroundStyle,
      membershipRevokedProfileBackgroundStyle: membershipResult.membershipRevokedProfileBackgroundStyle,
      membershipText: membershipResult.membershipText,
      membershipRevocationReason: membershipResult.revocationReason,
      membershipRevokedBackgroundApiStatus: membershipResult.revokedBackgroundApiStatus,
      membershipRevokedBackgroundApiMessage: membershipResult.revokedBackgroundApiMessage,
      membershipRevokedApiStatus: membershipResult.revokedApiStatus,
      membershipRevokedApiMessage: membershipResult.revokedApiMessage,
      membershipRevokedDraftPublishApiStatus: membershipResult.revokedDraftPublishApiStatus,
      membershipRevokedDraftPublishApiMessage: membershipResult.revokedDraftPublishApiMessage,
      membershipRevokedEditApiStatus: membershipResult.revokedEditApiStatus,
      membershipRevokedEditApiMessage: membershipResult.revokedEditApiMessage,
      membershipRevokedText: membershipResult.revokedText,
      bountyDraftTopicId: membershipResult.draftTopicId,
      bountyDraftTopicTitle: membershipResult.draftTopicTitle,
      bountyTopicId: membershipResult.topicId,
      bountyTopicTitle: membershipResult.topicTitle,
      bountyAcceptedCommentId: membershipResult.bountyAcceptedCommentId,
      bountyAcceptedTopicStatus: membershipResult.bountyAcceptedTopicStatus,
      bountyQuestionerLedgerId: membershipResult.bountyQuestionerLedgerId,
      bountyAnswererLedgerId: membershipResult.bountyAnswererLedgerId,
      bountyInsufficientCreditBalance: membershipResult.bountyInsufficientCreditBalance,
      bountyInsufficientCreditText: membershipResult.bountyInsufficientCreditText,
      bountyText: membershipResult.bountyText,
      notificationTitles
    };
  } finally {
    await page?.close().catch(() => {});
    chrome.kill();
    await delay(250);
    await rm(userDataDir, { recursive: true, force: true }).catch(() => {});
  }
}

function collectBrowserIssues(page, expectedBrowserIssues = []) {
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
      const issue = { type: `log:${entry.level}`, text: entry.text || "", url: entry.url || "" };
      issue.expected = isExpectedBrowserIssue(expectedBrowserIssues, issue);
      issues.push(issue);
    }
  });
  page.on("Network.responseReceived", (event) => {
    const response = event.response || {};
    if (response.status >= 400 && (response.url?.startsWith(API_BASE) || response.url?.startsWith(FRONTEND_BASE))) {
      const issue = { type: "http", status: response.status, url: response.url };
      issue.expected = isExpectedBrowserIssue(expectedBrowserIssues, issue);
      issues.push(issue);
    }
  });
  return issues;
}

function expectBrowserHttpFailure(expectedBrowserIssues, url, status) {
  expectedBrowserIssues.push({ url, status });
}

function isExpectedBrowserIssue(expectedBrowserIssues, issue) {
  const text = `${issue.url || ""} ${issue.text || ""}`;
  return expectedBrowserIssues.some((expected) => {
    if (expected.url && !text.includes(expected.url)) return false;
    if (expected.status && issue.status && Number(issue.status) !== Number(expected.status)) return false;
    if (expected.status && !issue.status && !text.includes(String(expected.status))) return false;
    return true;
  });
}

function isSeriousBrowserIssue(issue) {
  if (issue.expected) return false;
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
  return lines.find((line) => line.includes("当前定位")) ||
    lines.find((line) => line.includes("已展示")) ||
    lines.find((line) => line.includes("评价已提交")) ||
    "";
}

function summarizePublicProductReviewText(text, reviewContent) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(reviewContent)) ||
    lines.find((line) => line.includes("星评价")) ||
    "";
}

function summarizeCartText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("兑换成功") || line.includes("订单已创建")) || "";
}

function summarizeDashboardPaymentText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("订单已支付")) ||
    lines.find((line) => line.includes("支付成功")) ||
    "";
}

function summarizeInsufficientPaymentRecoveryText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("支付成功")) ||
    lines.find((line) => line.includes("订单已支付")) ||
    lines.find((line) => line.includes("支付失败")) ||
    "";
}

function summarizeCouponCancellationText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("优惠券已领取")) ||
    lines.find((line) => line.includes("已释放")) ||
    lines.find((line) => line.includes("订单已取消")) ||
    "";
}

function summarizeRefundText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("已退款")) || lines.find((line) => line.includes("售后退款已通过")) || "";
}

function summarizeRejectedRefundText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("售后已拒绝")) || lines.find((line) => line.includes("售后申请已拒绝")) || "";
}

function summarizeDigitalEntitlementText(text, grantKey, entitlementCode) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(grantKey)) ||
    (entitlementCode ? lines.find((line) => line.includes(entitlementCode)) : "") ||
    lines.find((line) => line.includes("可用")) ||
    "";
}

function summarizePublicBadgeText(text, badgeTitle) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(badgeTitle)) ||
    lines.find((line) => line.includes("通过商城数字权益获得")) ||
    lines.find((line) => line.includes("暂无公开徽章")) ||
    "";
}

function summarizeMembershipBountyText(text, topicTitle) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(topicTitle)) ||
    lines.find((line) => line.includes("会员权益可用")) ||
    lines.find((line) => line.includes("积分悬赏")) ||
    "";
}

async function runBrowserDirectCouponCheckout(page, fixture) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.directCouponProduct.id)}&coupon_code=${encodeURIComponent(fixture.directCoupon.code)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.directCouponProduct.title, "direct coupon product detail");
  await waitForText(page, "商品详情", "direct coupon product detail panel");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "direct coupon checkout panel");
  await waitForText(page, "优惠码将提交给系统校验", "direct coupon backend validation hint");
  await waitForButtonEnabled(page, "^确认兑换$", "direct coupon checkout enabled");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, `已优惠 ${COUPON_DISCOUNT} 积分|兑换成功|订单已创建`, "direct coupon order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.directCouponProduct.id);
  if (!order?.id) {
    throw new Error("Direct coupon mall order was not returned by user order API");
  }
  const couponCode = order.coupon_code || order.couponCode || "";
  const discountCredits = Number(order.discount_credits ?? order.discountCredits ?? 0);
  if (couponCode !== fixture.directCoupon.code || discountCredits !== COUPON_DISCOUNT) {
    throw new Error(`Direct coupon order did not apply expected discount: ${JSON.stringify({ couponCode, discountCredits })}`);
  }
  const reuseRejection = await assertDirectCouponReuseRejected(fixture);

  return {
    orderId: String(order.id),
    orderNo: order.order_no || order.orderNo || "",
    text: summarizeCheckoutText(await bodyText(page)),
    reuseStatus: reuseRejection.status,
    reuseText: reuseRejection.message
  };
}

async function assertDirectCouponReuseRejected(fixture) {
  const failure = await apiRequestFailure("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "direct coupon reuse",
    body: {
      idempotency_key: `direct-coupon-reuse-${Date.now()}`,
      items: [{ product_id: fixture.directCouponProduct.id, quantity: 1 }],
      coupon_code: fixture.directCoupon.code
    }
  });
  const legacyCode = String(failure.meta?.legacy_code || failure.meta?.legacyCode || "");
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  if (legacyCode !== "FailedPrecondition" || !combined.includes("coupon unavailable")) {
    throw new Error(`Direct coupon reuse rejection mismatch: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "coupon unavailable"
  };
}

async function runBrowserDashboardPaymentFlow(page, fixture) {
  const product = fixture.dashboardPayProduct;
  const initialStock = await currentMallProductStock(product.id);
  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `web-dashboard-pay-${Date.now()}`,
      receiver: "浏览器联调补付",
      phone: "13500000000",
      address: "上海 上海 浦东新区 补付路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error("Dashboard payment mall order was not returned by order API");
  }
  const orderStatus = mallOrderStatusValue(order.status);
  if (orderStatus !== 1 || Number(order.total_credits ?? order.totalCredits ?? 0) !== CHECKOUT_PRICE) {
    throw new Error(`Dashboard payment order snapshot mismatch: ${JSON.stringify({ orderStatus, totalCredits: order.total_credits ?? order.totalCredits })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const lockedStock = await waitForMallProductStock(product.id, initialStock - 1, "dashboard payment product stock locked by pending order");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "dashboard payment shell");
  await waitForText(page, orderNo, "dashboard payment order number");
  await waitForText(page, product.title, "dashboard payment order item title");
  await waitForText(page, "待支付", "dashboard payment pending order status");
  await waitForText(page, "继续支付", "dashboard payment action");
  await clickButtonInArticle(page, orderNo, "^继续支付$");
  await waitForText(page, "订单已支付，积分流水已同步。|已支付", "dashboard payment success");

  const paidOrder = await waitForMallOrderStatus(fixture, order.id, 3, "dashboard payment order paid in API");
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["订单已支付"]);
  await waitForMallProductStock(product.id, initialStock - 1, "dashboard payment product stock remains locked after paid");
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "支付记录|支付成功", "dashboard payment records");
  await waitForText(page, product.title, "paid dashboard payment item title");
  await waitForText(page, "再次兑换", "paid dashboard payment repeat action");

  return {
    orderId: String(order.id),
    orderNo: paidOrder.order_no || paidOrder.orderNo || orderNo,
    lockedStock,
    text: summarizeDashboardPaymentText(await bodyText(page)),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserInsufficientCreditRecoveryFlow(page, fixture, expectedBrowserIssues = []) {
  const product = fixture.insufficientCreditProduct;
  const initialStock = await currentMallProductStock(product.id);
  const balanceBeforeFailure = await currentCreditBalance(fixture);
  if (balanceBeforeFailure >= INSUFFICIENT_CHECKOUT_PRICE) {
    throw new Error(`Insufficient-credit fixture balance=${balanceBeforeFailure}, want below ${INSUFFICIENT_CHECKOUT_PRICE}`);
  }

  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `web-insufficient-credit-${Date.now()}`,
      receiver: "浏览器联调余额不足",
      phone: "13700000000",
      address: "上海 上海 浦东新区 余额路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error("Insufficient-credit mall order was not returned by order API");
  }
  const orderStatus = mallOrderStatusValue(order.status);
  const totalCredits = Number(order.total_credits ?? order.totalCredits ?? 0);
  if (orderStatus !== 1 || totalCredits !== INSUFFICIENT_CHECKOUT_PRICE) {
    throw new Error(`Insufficient-credit order snapshot mismatch: ${JSON.stringify({ orderStatus, totalCredits })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const lockedStock = await waitForMallProductStock(product.id, initialStock - 1, "insufficient-credit product stock locked by pending order");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "insufficient-credit dashboard shell");
  await waitForText(page, orderNo, "insufficient-credit order number");
  await waitForText(page, product.title, "insufficient-credit order item title");
  await waitForText(page, "待支付", "insufficient-credit pending order status");
  expectBrowserHttpFailure(expectedBrowserIssues, `${API_BASE}/mall/orders/${encodeURIComponent(order.id)}/pay`, 412);
  await clickButtonInArticle(page, orderNo, "^继续支付$");
  await waitForText(page, "积分不足|订单支付失败", "insufficient-credit payment failure notice");

  const failedPayment = await waitForMallOrderPaymentStatus(fixture, order.id, 3, "insufficient-credit failed payment");
  await waitForMallOrderStatus(fixture, order.id, 1, "insufficient-credit order remains pending after failed payment");
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}&payment_failed=${Date.now()}`);
  await waitForText(page, "支付记录|支付失败", "insufficient-credit failed payment record");
  await waitForText(page, "支付失败", "insufficient-credit failed payment status");
  await waitForText(page, "继续支付", "insufficient-credit retry action");

  const balanceAfterTopUp = await topUpUserCredits(fixture, PAYMENT_RECOVERY_TOP_UP, `browser-mall-payment-recovery-${Date.now()}`);
  if (balanceAfterTopUp < INSUFFICIENT_CHECKOUT_PRICE) {
    throw new Error(`Payment recovery top-up balance=${balanceAfterTopUp}, want at least ${INSUFFICIENT_CHECKOUT_PRICE}`);
  }

  await clickButtonInArticle(page, orderNo, "^继续支付$");
  await waitForText(page, "订单已支付，积分流水已同步。|已支付", "insufficient-credit recovered payment success");
  const paidOrder = await waitForMallOrderStatus(fixture, order.id, 3, "insufficient-credit order paid after recovery");
  const recoveredPayment = await waitForMallOrderPaymentStatus(fixture, order.id, 2, "insufficient-credit recovered payment");
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["订单已支付"]);
  await waitForMallProductStock(product.id, initialStock - 1, "insufficient-credit product stock remains locked after recovered payment");
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}&payment_recovered=${Date.now()}`);
  await waitForText(page, "支付记录|支付成功", "insufficient-credit recovered payment record");
  await waitForText(page, product.title, "recovered insufficient-credit item title");

  return {
    orderId: String(order.id),
    orderNo: paidOrder.order_no || paidOrder.orderNo || orderNo,
    lockedStock,
    balanceBeforeFailure,
    balanceAfterTopUp,
    failedPaymentId: String(failedPayment.id || failedPayment.ID || ""),
    recoveredPaymentId: String(recoveredPayment.id || recoveredPayment.ID || ""),
    text: summarizeInsufficientPaymentRecoveryText(await bodyText(page)),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserCouponCancellationFlow(page, fixture) {
  const product = fixture.cancelCouponProduct;
  const coupon = fixture.cancelCoupon;
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(product.id)}&coupon_code=${encodeURIComponent(coupon.code)}`;
  const initialStock = await currentMallProductStock(product.id);

  await navigate(page, shopUrl);
  await waitForText(page, product.title, "cancel coupon product detail");
  await waitForText(page, coupon.code, "cancel coupon visible in shop");
  await clickButtonInArticle(page, coupon.code, "^领取$");
  await waitForText(page, "优惠券已领取|已经在你的券包里", "cancel coupon claimed");
  await waitForCouponUsageStatus(fixture, coupon.id, 4, "", "cancel coupon claimed usage");

  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `web-cancel-coupon-${Date.now()}`,
      coupon_code: coupon.code,
      receiver: "浏览器联调取消",
      phone: "13600000000",
      address: "上海 上海 浦东新区 取消路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error("Cancel coupon mall order was not returned by order API");
  }
  const orderStatus = mallOrderStatusValue(order.status);
  const discountCredits = Number(order.discount_credits ?? order.discountCredits ?? 0);
  const couponCode = order.coupon_code || order.couponCode || "";
  if (orderStatus !== 1 || discountCredits !== COUPON_DISCOUNT || couponCode !== coupon.code) {
    throw new Error(`Cancel coupon order snapshot mismatch: ${JSON.stringify({ orderStatus, discountCredits, couponCode })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  await waitForCouponUsageStatus(fixture, coupon.id, 1, order.id, "cancel coupon reserved usage");
  const lockedStock = await waitForMallProductStock(product.id, initialStock - 1, "cancel coupon product stock locked by unpaid order");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "cancel coupon dashboard shell");
  await waitForText(page, orderNo, "cancel coupon order number");
  await waitForText(page, product.title, "cancel coupon order item title");
  await waitForText(page, "待支付", "cancel coupon pending order status");
  await waitForText(page, coupon.code, "cancel coupon order discount");
  await clickButtonInArticle(page, orderNo, "^取消订单$");
  await waitForText(page, "订单已取消|已取消", "cancel coupon order canceled");

  await waitForMallOrderStatus(fixture, order.id, 4, "cancel coupon order canceled in API");
  const restoredStock = await waitForMallProductStock(product.id, initialStock, "cancel coupon product stock restored after cancel");
  const releasedUsage = await waitForCouponUsageStatus(fixture, coupon.id, 3, order.id, "cancel coupon released usage");

  await navigate(page, `${FRONTEND_BASE}/dashboard/coupons?status=3&order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "优惠券|个人工作台", "released coupon dashboard");
  await waitForText(page, coupon.code, "released coupon code in dashboard");
  await waitForText(page, "已释放", "released coupon status in dashboard");
  await waitForText(page, `订单 #${order.id}`, "released coupon order reference");

  await navigate(page, `${shopUrl}&reclaim=${Date.now()}`);
  await waitForText(page, product.title, "cancel coupon product detail after release");
  await waitForText(page, coupon.code, "cancel coupon visible for reclaim");
  await clickButtonInArticle(page, coupon.code, "^领取$");
  await waitForText(page, "优惠券已领取|已经在你的券包里", "cancel coupon reclaimed after release");
  const reclaimedUsage = await waitForCouponUsageStatus(fixture, coupon.id, 4, "", "cancel coupon reclaimed usage");

  return {
    orderId: String(order.id),
    orderNo,
    lockedStock,
    restoredStock,
    releasedUsageId: String(releasedUsage.id || releasedUsage.ID || ""),
    reclaimedUsageId: String(reclaimedUsage.id || reclaimedUsage.ID || ""),
    text: summarizeCouponCancellationText(await bodyText(page))
  };
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
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(cartOrder.id)}`);
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
  const initialStock = await currentMallProductStock(fixture.refundProduct.id);
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
  const orderedQuantity = orderQuantityForProduct(order, fixture.refundProduct.id);
  if (orderedQuantity !== 1) {
    throw new Error(`Refund mall order quantity = ${orderedQuantity}, want 1`);
  }
  const lockedStock = await waitForMallProductStock(fixture.refundProduct.id, initialStock - orderedQuantity, "refund product stock locked by order");

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
  const restoredStock = await waitForMallProductStock(fixture.refundProduct.id, initialStock, "refund product stock restored after refund");
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
  await navigate(page, shopUrl);
  await waitForText(page, fixture.refundProduct.title, "refund product detail after stock restore");
  await waitForText(page, `库存\\s*${restoredStock}`, "restored refund product stock in storefront");

  return {
    orderId: String(order.id),
    orderNo,
    lockedStock,
    restoredStock,
    refundText: summarizeRefundText(refundText),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserRejectedRefundFlow(page, fixture) {
  const refundNote = `浏览器联调拒绝售后 ${Date.now()}：验证运营拒绝后用户能看到审核原因。`;
  const adminNote = `Browser E2E refund rejected ${Date.now()} - duplicate or unsupported reason`;
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.rejectedRefundProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.rejectedRefundProduct.title, "rejected refund product detail");
  await waitForText(page, "商品详情", "rejected refund product detail panel");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "rejected refund checkout panel");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "rejected refund order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.rejectedRefundProduct.id);
  if (!order?.id) {
    throw new Error("Rejected refund mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);

  await clickButton(page, "查看订单");
  await waitForText(page, "个人工作台", "rejected refund dashboard shell");
  await waitForText(page, "已支付", "rejected refund paid order row");
  await waitForText(page, fixture.rejectedRefundProduct.title, "rejected refund order item title");
  await waitForText(page, "申请售后", "rejected refund action");
  await clickButton(page, "^申请售后$");
  await waitForText(page, "售后原因", "rejected refund request form");
  await fillByLabel(page, "问题说明", refundNote);
  await waitForButtonEnabled(page, "^提交申请$", "rejected refund submit enabled");
  await clickButton(page, "^提交申请$");
  await waitForText(page, "售后申请已提交|售后待审核", "rejected refund request submitted");

  const refund = await latestRefundForOrder(fixture, order.id);
  if (!refund?.id) {
    throw new Error(`Rejected refund request was not returned for order ${order.id}`);
  }

  await rejectMallRefund(fixture, refund.id, adminNote);
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["售后申请已拒绝"]);

  await navigate(page, `${FRONTEND_BASE}/dashboard/refunds`);
  await waitForText(page, "待审核|处理中|已拒绝", "refunds panel for rejected refund");
  await waitForText(page, orderNo, "rejected refund order number");
  await waitForText(page, "售后已拒绝", "rejected refund status");
  await waitForText(page, adminNote, "rejected refund admin note");
  const refundText = await bodyText(page);

  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "售后申请已拒绝", "rejected refund notification title");
  await waitForText(page, orderNo, "rejected refund notification content");
  await waitForText(page, adminNote, "rejected refund notification admin note");
  await clickButtonInArticle(page, "售后申请已拒绝", "查看售后");
  await waitForText(page, "个人工作台", "rejected refund notification target shell");
  await waitForText(page, "个人列表|售后", "rejected refund notification target");
  await waitForText(page, "当前定位", "rejected refund focused marker");
  await waitForText(page, orderNo, "focused rejected refund order number");
  await waitForText(page, "售后已拒绝", "focused rejected refund status");
  await waitForText(page, adminNote, "focused rejected refund admin note");

  return {
    orderId: String(order.id),
    orderNo,
    refundId: String(refund.id),
    refundText: summarizeRejectedRefundText(refundText),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserDigitalEntitlementFlow(page, fixture) {
  const digitalQuantity = 2;
  const refundNote = `浏览器联调数字权益售后 ${Date.now()}：验证退款后权益撤销。`;
  const adminNote = `Browser E2E digital refund approved ${Date.now()}`;
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.digitalProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.digitalProduct.title, "digital product detail");
  await waitForText(page, "商品详情", "digital product detail panel");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "digital checkout panel");
  await waitForText(page, "徽章权益在线发放，无需收货地址|数字权益在线发放，无需收货地址", "digital checkout fulfillment hint");
  await fillByLabel(page, "数量", String(digitalQuantity));
  await waitForText(page, `${CHECKOUT_PRICE * digitalQuantity} 积分`, "digital checkout quantity total");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "digital order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.digitalProduct.id);
  if (!order?.id) {
    throw new Error("Digital mall order was not returned by user order API");
  }
  const orderedQuantity = orderQuantityForProduct(order, fixture.digitalProduct.id);
  if (orderedQuantity !== digitalQuantity) {
    throw new Error(`Digital mall order quantity = ${orderedQuantity}, want ${digitalQuantity}`);
  }
  const totalCredits = Number(order.total_credits ?? order.totalCredits ?? 0);
  if (totalCredits !== CHECKOUT_PRICE * digitalQuantity) {
    throw new Error(`Digital mall order total = ${totalCredits}, want ${CHECKOUT_PRICE * digitalQuantity}`);
  }
  if (String(order.receiver || "").trim() || String(order.phone || "").trim() || String(order.address || "").trim()) {
    throw new Error(`Granted digital order unexpectedly stored shipping information: ${JSON.stringify({ receiver: order.receiver, phone: order.phone, address: order.address })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const entitlements = await waitForDigitalEntitlements(fixture, order.id, fixture.digitalProduct.id, fixture.digitalGrantKey, "ACTIVE", digitalQuantity);
  const entitlementCodes = entitlements.map((item) => item?.fulfillment_code || item?.fulfillmentCode || "").filter(Boolean);
  const entitlementCode = entitlementCodes[0] || "";

  await clickButton(page, "查看订单");
  await waitForText(page, "个人工作台", "digital dashboard shell");
  await waitForText(page, orderNo, "digital order number");
  await waitForText(page, fixture.digitalProduct.title, "digital order item title");
  await waitForText(page, `x ${digitalQuantity}|等 ${digitalQuantity} 项`, "digital order quantity or entitlement count");
  await waitForText(page, `授权：徽章权益 · ${fixture.digitalGrantKey}`, "digital order grant snapshot");
  await waitForText(page, "数字权益", "digital order entitlement section");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `digital entitlement fulfillment code ${code} in order`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "entitlements dashboard");
  await waitForText(page, "已过期", "expired entitlement filter tab");
  await waitForText(page, "全部权益|徽章|主题|会员", "entitlement grant type filters");
  await clickButton(page, "^徽章$");
  await waitForText(page, fixture.digitalProduct.title, "entitlement title");
  await waitForText(page, fixture.digitalGrantKey, "entitlement grant key");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `entitlement fulfillment code ${code}`);
  }
  await waitForText(page, "可用", "active entitlement state");
  const activeText = summarizeDigitalEntitlementText(await bodyText(page), fixture.digitalGrantKey, entitlementCode);
  await clickButtonInArticle(page, fixture.digitalProduct.title, "查看商品");
  await waitForText(page, "商品详情", "entitlement product target detail");
  await waitForText(page, fixture.digitalProduct.title, "entitlement product target title");
  const publicBadgesUrl = `${FRONTEND_BASE}/user/${encodeURIComponent(fixture.auth.user.id)}/badges`;
  await navigate(page, publicBadgesUrl);
  await waitForText(page, "用户空间", "public badge profile shell");
  await waitForPublicBadgePanelReady(page, fixture.digitalProduct.title, "public badge panel");
  await waitForText(page, fixture.digitalProduct.title, "public badge title");
  await waitForText(page, "通过商城数字权益获得。|通过商城数字权益获得", "public badge description");
  await waitForText(page, "徽章权益", "public badge grant label");
  const publicBadgeText = summarizePublicBadgeText(await bodyText(page), fixture.digitalProduct.title);

  const refund = await createMallRefund(fixture, order.id, refundNote);
  await approveMallRefund(fixture, refund.id, adminNote);
  await waitForDigitalEntitlements(fixture, order.id, fixture.digitalProduct.id, fixture.digitalGrantKey, "REVOKED", digitalQuantity);
  await waitForMallOrderNotifications(fixture, order.id, ["售后退款已通过"]);
  await navigate(page, publicBadgesUrl);
  await waitForText(page, "用户空间", "public badge profile shell after refund");
  await waitForPublicBadgePanelReady(page, fixture.digitalProduct.title, "public badge panel after refund");
  await waitFor(page, `(() => {
    const text = document.body?.innerText || "";
    return !text.includes(${JSON.stringify(fixture.digitalProduct.title)});
  })()`, "public badge removed after refund");
  const publicBadgeRevokedText = summarizePublicBadgeText(await bodyText(page), fixture.digitalProduct.title);

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "entitlements dashboard after refund");
  await clickButton(page, "^已撤销$");
  await clickButton(page, "^徽章$");
  await waitForText(page, fixture.digitalProduct.title, "revoked entitlement title");
  await waitForText(page, fixture.digitalGrantKey, "revoked entitlement grant key");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `revoked entitlement fulfillment code ${code}`);
  }
  await waitForText(page, "已撤销|退款失效", "revoked entitlement state");
  const revokedText = summarizeDigitalEntitlementText(await bodyText(page), fixture.digitalGrantKey, entitlementCode);
  await clickButtonInArticle(page, fixture.digitalProduct.title, "查看售后");
  await waitForText(page, "个人列表|售后", "revoked entitlement refund target");
  await waitForText(page, "当前定位", "revoked entitlement focused refund marker");
  await waitForText(page, orderNo, "revoked entitlement focused refund order");
  await waitForText(page, "已退款", "revoked entitlement focused refund status");
  await waitForText(page, adminNote, "revoked entitlement focused refund admin note");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, orderNo, "refunded digital order number");
  await waitForText(page, "已退款", "refunded digital order status");
  await waitForText(page, "已撤销|退款失效", "refunded digital order entitlement state");
  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "售后退款已通过", "digital refund notification title");
  await waitForText(page, orderNo, "digital refund notification order number");
  await waitForText(page, "数字权益已撤销", "digital refund notification revocation hint");
  await waitForText(page, fixture.digitalGrantKey, "digital refund notification grant key");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `digital refund notification fulfillment code ${code}`);
  }

  return {
    orderId: String(order.id),
    orderNo,
    entitlementCode,
    entitlementCodes,
    entitlementCount: entitlements.length,
    refundId: String(refund.id),
    digitalText: activeText,
    revokedText,
    publicBadgeText,
    publicBadgeRevokedText
  };
}

async function runBrowserThemeEntitlementFlow(page, fixture) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.themeProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.themeProduct.title, "theme product detail");
  await waitForText(page, "商品详情", "theme product detail panel");
  await waitForText(page, "主题权益", "theme product grant label");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "theme checkout panel");
  await waitForText(page, "主题权益在线发放，无需收货地址|数字权益在线发放，无需收货地址", "theme checkout fulfillment hint");
  await waitForButtonEnabled(page, "^确认兑换$", "theme checkout enabled");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "theme order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.themeProduct.id);
  if (!order?.id) {
    throw new Error("Theme mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const entitlement = await waitForDigitalEntitlement(fixture, order.id, fixture.themeProduct.id, fixture.themeGrantKey, "ACTIVE");
  const entitlementCode = entitlement?.fulfillment_code || entitlement?.fulfillmentCode || "";

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "theme entitlements dashboard");
  await clickButton(page, "^主题$");
  await waitForText(page, fixture.themeProduct.title, "theme dashboard entitlement title");
  await waitForText(page, fixture.themeGrantKey, "theme dashboard entitlement grant key");
  await waitForText(page, "可用", "theme dashboard entitlement active state");

  await navigate(page, `${FRONTEND_BASE}/dashboard/profile`);
  await waitForText(page, "个人资料", "profile settings panel");
  await waitForText(page, "高级主题已解锁", "theme access available");
  await fillByLabel(page, "主题", fixture.themeGrantKey);
  const selectedTheme = await fieldValueByLabel(page, "主题");
  if (selectedTheme !== fixture.themeGrantKey) {
    throw new Error(`Profile theme select value = ${selectedTheme}, want ${fixture.themeGrantKey}`);
  }
  await clickButton(page, "^保存资料$");
  await waitForText(page, "资料已保存", "profile theme saved");

  const publicProfileUrl = `${FRONTEND_BASE}/user/${encodeURIComponent(fixture.auth.user.id)}`;
  await navigate(page, publicProfileUrl);
  await waitForText(page, "用户空间", "public profile after theme save");
  const profileClass = await waitForProfileThemeClass(page, "profile-theme-pro", "public profile theme pro class");

  const revocationReason = `Browser E2E theme revoke ${Date.now()}`;
  const revokedEntitlement = await revokeMallDigitalEntitlement(fixture, entitlement.id, revocationReason);
  if (String(revokedEntitlement?.status || "").toUpperCase() !== "REVOKED") {
    throw new Error(`Theme entitlement revoke status = ${revokedEntitlement?.status ?? "unknown"}, want REVOKED`);
  }
  await waitForDigitalEntitlement(fixture, order.id, fixture.themeProduct.id, fixture.themeGrantKey, "REVOKED");

  await navigate(page, `${publicProfileUrl}?theme_revoked=${Date.now()}`);
  await waitForText(page, "用户空间", "public profile after theme revoke");
  const revokedProfileClass = await waitForProfileThemeClass(page, "profile-theme-default", "public profile default class after theme revoke");

  await navigate(page, `${FRONTEND_BASE}/dashboard/profile`);
  await waitForText(page, "个人资料", "profile settings panel after theme revoke");
  await waitForText(page, "购买 theme-pro 后可使用高级主题", "theme access unavailable after revoke");

  return {
    orderId: String(order.id),
    orderNo,
    entitlementCode,
    profileClass,
    revokedProfileClass,
    revocationReason
  };
}

async function runBrowserMembershipBountyFlow(page, fixture, expectedBrowserIssues = []) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.membershipProduct.id)}`;
  const bountyScore = 7;
  const topicTitle = `E2E Membership Bounty ${Date.now()}`;
  const topicBody = `浏览器联调会员悬赏问答：先保存草稿，再兑换 ${fixture.membershipGrantKey} 后发布带悬赏问题。`;

  await navigate(page, `${FRONTEND_BASE}/question/create`);
  await waitForText(page, "发布求助|创作中心", "question draft editor");
  await fillBySelector(page, ".compose-title", topicTitle);
  await fillBySelector(page, ".editor-body", topicBody);
  await fillBySelector(page, ".tag-assist input", "会员悬赏 联调");
  await fillByLabel(page, "悬赏积分", String(bountyScore));
  await setCheckboxByLabel(page, "立即发布", false);
  await waitForText(page, "悬赏需会员权益", "question editor membership gate before membership");
  await waitForButtonEnabled(page, "^保存草稿$", "bounty question draft submit enabled");
  await clickButton(page, "^保存草稿$");
  await waitForText(page, "编辑求助|创作中心", "bounty question draft edit page");
  const loadedDraftTitle = await fieldValueBySelector(page, ".compose-title");
  if (loadedDraftTitle !== topicTitle) {
    throw new Error(`Loaded bounty draft title = ${loadedDraftTitle}, want ${topicTitle}`);
  }
  const loadedDraftBody = await fieldValueBySelector(page, ".editor-body");
  if (loadedDraftBody !== topicBody) {
    throw new Error(`Loaded bounty draft body = ${loadedDraftBody}, want ${topicBody}`);
  }
  const loadedDraftBounty = await fieldValueByLabel(page, "悬赏积分");
  if (loadedDraftBounty !== String(bountyScore)) {
    throw new Error(`Loaded bounty draft score = ${loadedDraftBounty}, want ${bountyScore}`);
  }

  const draftTopic = await latestMyTopicForTitle(fixture, topicTitle);
  if (!draftTopic?.id) {
    throw new Error(`Membership bounty draft was not returned by user topic API: ${topicTitle}`);
  }
  const draftStatus = Number(draftTopic.status ?? 0);
  if (draftStatus !== 1) {
    throw new Error(`Membership bounty draft status = ${draftStatus}, want 1`);
  }
  const draftType = String(draftTopic.type || draftTopic.topic_type || draftTopic.topicType || "").toLowerCase();
  if (draftType !== "qa") {
    throw new Error(`Membership bounty draft type = ${draftType || "unknown"}, want qa`);
  }
  const draftBounty = Number(draftTopic.bounty_score ?? draftTopic.bountyScore ?? 0);
  if (draftBounty !== bountyScore) {
    throw new Error(`Membership bounty draft score = ${draftBounty}, want ${bountyScore}`);
  }

  await navigate(page, shopUrl);
  await waitForText(page, fixture.membershipProduct.title, "membership product detail");
  await waitForText(page, "商品详情", "membership product detail panel");
  await waitForText(page, "会员权益", "membership product grant label");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "membership checkout panel");
  await waitForText(page, "会员权益在线发放，无需收货地址", "membership checkout fulfillment hint");
  await waitForButtonEnabled(page, "^确认兑换$", "membership checkout enabled");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "membership order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.membershipProduct.id);
  if (!order?.id) {
    throw new Error("Membership mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const entitlement = await waitForDigitalEntitlement(fixture, order.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "ACTIVE");
  const entitlementCode = entitlement?.fulfillment_code || entitlement?.fulfillmentCode || "";
  const entitlementExpiresAt = Number(entitlement?.expires_at ?? entitlement?.expiresAt ?? 0);
  const entitlementGrantType = String(entitlement?.grant_type || entitlement?.grantType || "").toLowerCase();
  if (entitlementGrantType && entitlementGrantType !== "membership") {
    throw new Error(`Membership entitlement grant_type = ${entitlementGrantType}, want membership`);
  }
  if (!entitlementExpiresAt || entitlementExpiresAt <= Date.now()) {
    throw new Error(`Membership entitlement expires_at = ${entitlementExpiresAt}, want future timestamp`);
  }
  const membershipRefundRejection = await assertMembershipOrderRejectsRefund(fixture, order.id);

  await navigate(page, shopUrl);
  await waitForText(page, fixture.membershipProduct.title, "membership renewal product detail");
  await waitForText(page, "商品详情", "membership renewal product detail panel");
  await waitForButtonEnabled(page, "^立即兑换$", "membership renewal checkout action");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "membership renewal checkout panel");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "membership renewal order paid");
  const renewalOrder = await latestMallOrderForProduct(fixture, fixture.membershipProduct.id);
  if (!renewalOrder?.id || String(renewalOrder.id) === String(order.id)) {
    throw new Error("Membership renewal did not create a distinct order");
  }
  const renewalEntitlement = await waitForDigitalEntitlement(fixture, renewalOrder.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "ACTIVE");
  const renewalExpiresAt = Number(renewalEntitlement?.expires_at ?? renewalEntitlement?.expiresAt ?? 0);
  if (renewalExpiresAt < entitlementExpiresAt + MEMBERSHIP_DURATION_MS - 60000) {
    throw new Error(`Membership renewal expires_at = ${renewalExpiresAt}, want at least ${entitlementExpiresAt + MEMBERSHIP_DURATION_MS - 60000}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "membership dashboard shell");
  await waitForText(page, orderNo, "membership order number");
  await waitForText(page, fixture.membershipProduct.title, "membership order item title");
  await waitForText(page, `授权：会员权益 · ${fixture.membershipGrantKey}|会员权益`, "membership order grant snapshot");
  if (entitlementCode) {
    await waitForText(page, entitlementCode, "membership order entitlement code");
  }

  await navigate(page, `${FRONTEND_BASE}/member`);
  await waitForText(page, "已购会员权益", "member page entitlements panel");
  await waitForText(page, fixture.membershipProduct.title, "member page membership entitlement title");
  await waitForText(page, "会员权益", "member page active membership count");
  await waitForText(page, "有效至", "member page membership expiry");
  const membershipText = summarizeDigitalEntitlementText(await bodyText(page), fixture.membershipGrantKey, entitlementCode);

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "membership entitlements dashboard");
  await clickButton(page, "^会员$");
  await waitForText(page, fixture.membershipProduct.title, "membership dashboard entitlement title");
  await waitForText(page, fixture.membershipGrantKey, "membership dashboard entitlement grant key");
  await waitForText(page, "有效至", "membership dashboard entitlement expiry");

  const membershipBackgroundUrl = `${FRONTEND_BASE}/uploads/e2e-membership-bg-${Date.now()}.webp`;
  await navigate(page, `${FRONTEND_BASE}/dashboard/profile`);
  await waitForText(page, "个人资料", "profile settings panel for membership background");
  await waitForText(page, "会员背景已解锁", "profile background membership access available");
  await fillByLabel(page, "背景图 URL", membershipBackgroundUrl);
  await clickButton(page, "^保存资料$");
  await waitForText(page, "资料已保存", "membership profile background saved");
  const publicProfileUrl = `${FRONTEND_BASE}/user/${encodeURIComponent(fixture.auth.user.id)}`;
  await navigate(page, publicProfileUrl);
  await waitForText(page, "用户空间", "public profile after membership background save");
  const membershipProfileBackgroundStyle = await waitForProfileBackgroundStyle(page, membershipBackgroundUrl, "public profile membership background");

  await navigate(page, `${FRONTEND_BASE}/question/edit/${encodeURIComponent(draftTopic.id)}`);
  await waitForText(page, "编辑求助|创作中心", "bounty question draft editor after membership");
  await waitForText(page, "会员权益可用", "question editor membership gate active");
  const loadedPublishTitle = await fieldValueBySelector(page, ".compose-title");
  if (loadedPublishTitle !== topicTitle) {
    throw new Error(`Loaded bounty draft title = ${loadedPublishTitle}, want ${topicTitle}`);
  }
  const bountyInsufficientCreditBalance = await currentCreditBalance(fixture);
  const oversizedBountyScore = bountyInsufficientCreditBalance + 100;
  await fillByLabel(page, "悬赏积分", String(oversizedBountyScore));
  await setCheckboxByLabel(page, "立即发布", true);
  await waitForButtonEnabled(page, "^发布$", "bounty question submit enabled");
  expectBrowserHttpFailure(expectedBrowserIssues, `${API_BASE}/topics/${encodeURIComponent(draftTopic.id)}/publish`, 412);
  await clickButton(page, "^发布$");
  await waitForText(page, "悬赏积分不足", "bounty insufficient credit publish error");
  const failedPublishDraft = await latestMyTopicForTitle(fixture, topicTitle);
  const failedPublishStatus = Number(failedPublishDraft.status ?? 0);
  if (failedPublishStatus !== 1) {
    throw new Error(`Insufficient-credit bounty draft status = ${failedPublishStatus}, want 1`);
  }
  const failedPublishBounty = Number(failedPublishDraft.bounty_score ?? failedPublishDraft.bountyScore ?? 0);
  if (failedPublishBounty !== oversizedBountyScore) {
    throw new Error(`Insufficient-credit bounty draft score = ${failedPublishBounty}, want ${oversizedBountyScore}`);
  }
  await fillByLabel(page, "悬赏积分", String(bountyScore));
  await waitForButtonEnabled(page, "^发布$", "bounty question submit enabled after balance-sized bounty");
  await clickButton(page, "^发布$");
  await waitForText(page, topicTitle, "bounty topic detail");
  await waitForText(page, `${bountyScore} 积分悬赏`, "bounty topic score");
  const bountyText = summarizeMembershipBountyText(await bodyText(page), topicTitle);

  const topic = await latestTopicForTitle(fixture, topicTitle);
  if (!topic?.id) {
    throw new Error(`Membership bounty topic was not returned by topic list API: ${topicTitle}`);
  }
  if (String(topic.id) !== String(draftTopic.id)) {
    throw new Error(`Published bounty topic id = ${topic.id}, want draft topic id ${draftTopic.id}`);
  }
  const actualBounty = Number(topic.bounty_score ?? topic.bountyScore ?? 0);
  if (actualBounty !== bountyScore) {
    throw new Error(`Membership bounty topic score = ${actualBounty}, want ${bountyScore}`);
  }
  const topicType = String(topic.type || topic.topic_type || topic.topicType || "").toLowerCase();
  if (topicType && topicType !== "qa" && topicType !== "question") {
    throw new Error(`Membership bounty topic type = ${topicType}, want qa`);
  }

  const answerNeedle = `浏览器联调悬赏答案 ${Date.now()}`;
  const answerContent = `${answerNeedle}：采纳后应结算 ${bountyScore} 积分。`;
  const answerResp = await apiRequest(`/topics/${encodeURIComponent(topic.id)}/comments`, {
    method: "POST",
    token: fixture.answererAuth.accessToken,
    body: {
      content: answerContent,
      parent_id: 0
    }
  });
  const answerComment = answerResp?.comment || answerResp;
  if (!answerComment?.id) {
    throw new Error(`Bounty answer comment creation did not return comment.id: ${JSON.stringify(answerResp)}`);
  }
  const questionerBalanceBeforeAccept = await currentCreditBalance(fixture);
  const answererBalanceBeforeAccept = await currentCreditBalance({
    ...fixture,
    auth: fixture.answererAuth
  });

  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(topic.id)}?accepted_e2e=${Date.now()}`);
  await waitForText(page, topicTitle, "bounty topic detail before answer acceptance");
  await waitForText(page, answerNeedle, "bounty answer visible before acceptance");
  await clickButtonWhenEnabled(page, "^采纳答案$", "bounty answer accept button enabled");
  await waitForText(page, "已采纳", "accepted answer badge");
  await waitForText(page, "已解决", "bounty topic resolved state");

  const acceptedTopic = await waitForTopicAccepted(
    topic.id,
    answerComment.id,
    "membership bounty accepted topic",
  );
  const questionerLedger = await waitForCreditLedgerEntry(
    fixture.auth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_bounty_paid" &&
      String(item.source_type ?? item.sourceType ?? "") === "topic" &&
      String(item.source_id ?? item.sourceId ?? "") === String(topic.id) &&
      Number(item.delta ?? 0) === -bountyScore,
    "questioner bounty paid ledger",
  );
  const answererLedger = await waitForCreditLedgerEntry(
    fixture.answererAuth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_answer_accepted" &&
      String(item.source_type ?? item.sourceType ?? "") === "comment" &&
      String(item.source_id ?? item.sourceId ?? "") === String(answerComment.id) &&
      Number(item.delta ?? 0) === bountyScore,
    "answerer accepted answer reward ledger",
  );
  const questionerBalanceAfterAccept = await currentCreditBalance(fixture);
  const answererBalanceAfterAccept = await currentCreditBalance({
    ...fixture,
    auth: fixture.answererAuth
  });
  if (questionerBalanceAfterAccept !== questionerBalanceBeforeAccept - bountyScore) {
    throw new Error(
      `Questioner balance after bounty acceptance = ${questionerBalanceAfterAccept}, want ${questionerBalanceBeforeAccept - bountyScore}`,
    );
  }
  if (answererBalanceAfterAccept !== answererBalanceBeforeAccept + bountyScore) {
    throw new Error(
      `Answerer balance after bounty acceptance = ${answererBalanceAfterAccept}, want ${answererBalanceBeforeAccept + bountyScore}`,
    );
  }

  await revokeMallDigitalEntitlement(fixture, entitlement.id, `Browser E2E first membership revoke ${Date.now()}`);
  await waitForDigitalEntitlement(fixture, order.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "REVOKED");

  const revocationReason = `Browser E2E membership revoke ${Date.now()}`;
  const revokedEntitlement = await revokeMallDigitalEntitlement(fixture, renewalEntitlement.id, revocationReason);
  if (String(revokedEntitlement?.status || "").toUpperCase() !== "REVOKED") {
    throw new Error(`Admin entitlement revoke status = ${revokedEntitlement?.status ?? "unknown"}, want REVOKED`);
  }
  await waitForDigitalEntitlement(fixture, renewalOrder.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "REVOKED");
  await navigate(page, `${publicProfileUrl}?membership_revoked=${Date.now()}`);
  await waitForText(page, "用户空间", "public profile after membership revoke");
  const membershipRevokedProfileBackgroundStyle = await waitForProfileBackgroundCleared(page, "public profile background hidden after membership revoke");
  const revokedBackgroundRejection = await assertRevokedMembershipRejectsProfileBackground(fixture, membershipBackgroundUrl);

  const revocationNotifications = await waitForMallOrderNotifications(fixture, renewalOrder.id, ["数字权益已撤销"]);
  const revocationNotification = revocationNotifications[0];
  const notificationSourceID = revocationNotification?.source_id ?? revocationNotification?.sourceId;
  if (String(notificationSourceID) !== String(renewalEntitlement.id)) {
    throw new Error(`Membership revoke notification source_id = ${notificationSourceID ?? "unknown"}, want ${renewalEntitlement.id}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "数字权益已撤销", "membership revocation notification title");
  await waitForText(page, revocationReason, "membership revocation notification reason");
  await clickButtonInArticle(page, renewalOrder.order_no || renewalOrder.orderNo || String(renewalOrder.id), "查看权益");
  await waitForText(page, "个人列表|数字权益|权益", "membership revocation entitlement target");
  await waitForText(page, fixture.membershipProduct.title, "revoked membership entitlement title");
  await waitForText(page, "当前权益", "revoked membership focused marker");
  await waitForText(page, "已撤销|管理员撤销", "revoked membership entitlement state");
  await waitForText(page, revocationReason, "revoked membership entitlement reason");
  const revokedText = summarizeDigitalEntitlementText(await bodyText(page), fixture.membershipGrantKey, entitlementCode);

  await navigate(page, `${FRONTEND_BASE}/question/create`);
  await waitForText(page, "发布求助|创作中心", "question editor after membership revoke");
  await fillByLabel(page, "悬赏积分", String(bountyScore));
  await waitForText(page, "悬赏需会员权益", "question editor membership gate after revoke");
  const revokedBountyRejection = await assertRevokedMembershipRejectsBountyPublish(fixture, bountyScore);
  const revokedBountyDraftPublishRejection = await assertRevokedMembershipRejectsBountyDraftPublish(fixture, bountyScore);
  const revokedBountyEditRejection = await assertRevokedMembershipRejectsBountyEdit(fixture, topic, bountyScore, topicBody);

  return {
    orderId: String(order.id),
    orderNo,
    entitlementCode,
    expiresAt: entitlementExpiresAt,
    renewalExpiresAt,
    refundApiStatus: membershipRefundRejection.status,
    refundApiMessage: membershipRefundRejection.message,
    membershipBackgroundUrl,
    membershipProfileBackgroundStyle,
    membershipRevokedProfileBackgroundStyle,
    membershipText,
    revocationReason,
    revokedBackgroundApiStatus: revokedBackgroundRejection.status,
    revokedBackgroundApiMessage: revokedBackgroundRejection.message,
    revokedApiStatus: revokedBountyRejection.status,
    revokedApiMessage: revokedBountyRejection.message,
    revokedDraftPublishApiStatus: revokedBountyDraftPublishRejection.status,
    revokedDraftPublishApiMessage: revokedBountyDraftPublishRejection.message,
    revokedEditApiStatus: revokedBountyEditRejection.status,
    revokedEditApiMessage: revokedBountyEditRejection.message,
    revokedText,
    draftTopicId: String(draftTopic.id),
    draftTopicTitle: topicTitle,
    topicId: String(topic.id),
    topicTitle,
    bountyAcceptedCommentId: String(answerComment.id),
    bountyAcceptedTopicStatus: acceptedTopic.qa_status || acceptedTopic.qaStatus || "",
    bountyQuestionerLedgerId: String(questionerLedger.id ?? questionerLedger.ID ?? ""),
    bountyAnswererLedgerId: String(answererLedger.id ?? answererLedger.ID ?? ""),
    bountyInsufficientCreditBalance,
    bountyInsufficientCreditText: "悬赏积分不足，请先补足积分余额。",
    bountyText
  };
}

async function assertMembershipOrderRejectsRefund(fixture, orderId) {
  const failure = await apiRequestFailure(`/mall/orders/${encodeURIComponent(orderId)}/refunds`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "membership order refund",
    body: {
      reason: "membership_refund",
      note: `会员订单普通售后应被拒绝 ${Date.now()}`
    }
  });
  const legacyCode = String(failure.meta?.legacy_code || failure.meta?.legacyCode || "");
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  if (legacyCode !== "FailedPrecondition" || !combined.includes("membership order refund unavailable")) {
    throw new Error(`Membership order refund rejection mismatch: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership order refund unavailable"
  };
}

async function assertRevokedMembershipRejectsBountyDraftPublish(fixture, bountyScore) {
  const stamp = Date.now();
  const title = `E2E Revoked Membership Draft Publish ${stamp}`;
  const created = await apiRequest("/topics", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      slug: `e2e-revoked-membership-draft-${stamp}`,
      type: "qa",
      title,
      body: "Draft creation is allowed after membership revocation, but publishing must stay server-gated.",
      tags: ["membership", "e2e"],
      category_id: 1,
      bounty_score: bountyScore,
      publish: false
    }
  });
  const draft = created?.topic || created;
  const topicID = draft?.id ?? draft?.ID;
  if (!topicID) {
    throw new Error(`Revoked membership bounty draft create did not return topic id: ${JSON.stringify(created)}`);
  }

  const failure = await apiRequestFailure(`/topics/${encodeURIComponent(topicID)}/publish`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership bounty draft publish"
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipReason =
    combined.includes("membership entitlement required") ||
    combined.includes("topic_membership_entitlement_required") ||
    combined.includes("topic_membership");
  if (!hasMembershipReason) {
    throw new Error(`Revoked membership bounty draft publish did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }

  const draftAfterFailure = await latestMyTopicForTitle(fixture, title);
  const draftStatus = Number(draftAfterFailure.status ?? 0);
  if (draftStatus !== 1) {
    throw new Error(`Revoked membership bounty draft status after failed publish = ${draftStatus}, want 1`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership entitlement required for bounty QA draft publish"
  };
}

async function assertRevokedMembershipRejectsProfileBackground(fixture, backgroundUrl) {
  const failure = await apiRequestFailure("/users/me", {
    method: "PUT",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership profile background",
    body: {
      nickname: fixture.auth.user.nickname || fixture.auth.user.username || "E2E User",
      avatar_url: fixture.auth.user.avatar_url || fixture.auth.user.avatarUrl || "",
      background_url: `${backgroundUrl}?revoked=${Date.now()}`,
      bio: fixture.auth.user.bio || ""
    }
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipBackgroundReason =
    combined.includes("profile background membership entitlement required") ||
    combined.includes("background membership") ||
    combined.includes("membership entitlement required");
  if (!hasMembershipBackgroundReason) {
    throw new Error(`Revoked membership profile background did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "profile background membership entitlement required"
  };
}

async function assertRevokedMembershipRejectsBountyPublish(fixture, bountyScore) {
  const stamp = Date.now();
  const failure = await apiRequestFailure("/topics", {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership bounty publish",
    body: {
      slug: `e2e-revoked-membership-bounty-${stamp}`,
      type: "qa",
      title: `E2E Revoked Membership Bounty API ${stamp}`,
      body: "Direct API publish after membership revocation should be rejected by the server.",
      tags: ["membership", "e2e"],
      category_id: 1,
      bounty_score: bountyScore,
      publish: true
    }
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipReason =
    combined.includes("membership entitlement required") ||
    combined.includes("topic_membership_entitlement_required") ||
    combined.includes("topic_membership");
  if (!hasMembershipReason) {
    throw new Error(`Revoked membership bounty publish did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership entitlement required for bounty QA topics"
  };
}

async function assertRevokedMembershipRejectsBountyEdit(fixture, topic, bountyScore, originalBody) {
  const topicID = topic?.id;
  if (!topicID) {
    throw new Error("Cannot verify revoked membership bounty edit without a topic id");
  }
  const failure = await apiRequestFailure(`/topics/${encodeURIComponent(topicID)}`, {
    method: "PUT",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership bounty edit",
    body: {
      title: topic.title || `E2E Revoked Membership Bounty Edit ${Date.now()}`,
      body: `${originalBody}\n\nRevoked membership edit attempt ${Date.now()}`,
      tags: ["membership", "e2e"],
      category_id: Number(topic.category_id ?? topic.categoryId ?? 1),
      bounty_score: bountyScore
    }
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipReason =
    combined.includes("membership entitlement required") ||
    combined.includes("topic_membership_entitlement_required") ||
    combined.includes("topic_membership");
  if (!hasMembershipReason) {
    throw new Error(`Revoked membership bounty edit did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership entitlement required for bounty QA topics"
  };
}

async function latestMallOrderForProduct(fixture, productId) {
  const data = await apiRequest("/mall/orders?limit=20&offset=0", {
    token: fixture.auth.accessToken
  });
  return listItems(data)
    .filter((order) => orderContainsProduct(order, productId))
    .sort((left, right) => Number(right?.id || 0) - Number(left?.id || 0))[0];
}

async function waitForMallOrderStatus(fixture, orderId, expectedStatus, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastOrder = null;
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}`, {
      token: fixture.auth.accessToken
    });
    lastOrder = data?.order || data;
    if (mallOrderStatusValue(lastOrder?.status) === expectedStatus) {
      return lastOrder;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}: status=${lastOrder?.status ?? "unknown"}, want ${expectedStatus}`);
}

async function waitForMallOrderPaymentStatus(fixture, orderId, expectedStatus, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastPayments = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}/payments`, {
      token: fixture.auth.accessToken
    });
    lastPayments = listItems(data);
    const payment = lastPayments.find((item) => mallPaymentStatusValue(item?.status ?? item?.Status) === expectedStatus);
    if (payment?.id || payment?.ID) {
      return payment;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}. Last payments: ${JSON.stringify(lastPayments.slice(0, 10), null, 2)}`);
}

async function waitForCouponUsageStatus(fixture, couponId, expectedStatus, orderId = "", label = "coupon usage", timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastUsages = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/coupons/mine?limit=50&offset=0&status=${encodeURIComponent(expectedStatus)}`, {
      token: fixture.auth.accessToken
    });
    lastUsages = listItems(data);
    const usage = lastUsages.find((item) => {
      const source = item?.coupon || item?.Coupon || {};
      const itemCouponId = item?.coupon_id ?? item?.couponId ?? source?.id ?? source?.Id;
      const itemOrderId = item?.order_id ?? item?.orderId;
      return (
        String(itemCouponId) === String(couponId) &&
        couponUsageStatusValue(item?.status ?? item?.Status) === expectedStatus &&
        (!orderId || String(itemOrderId) === String(orderId))
      );
    });
    if (usage?.id || usage?.ID) {
      return usage;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}. Last usages: ${JSON.stringify(lastUsages.slice(0, 10), null, 2)}`);
}

async function currentCreditBalance(fixture) {
  const data = await apiRequest("/credits/balance", {
    token: fixture.auth.accessToken
  });
  const total = Number(data?.total ?? data?.balance?.total ?? data?.credits ?? 0);
  if (!Number.isFinite(total)) {
    throw new Error(`Credit balance response did not include numeric total: ${JSON.stringify(data)}`);
  }
  return total;
}

async function topUpUserCredits(fixture, delta, sourceEventId) {
  await apiRequest(`/admin/credits/users/${encodeURIComponent(fixture.auth.user.id)}/adjust`, {
    method: "POST",
    token: fixture.adminToken,
    body: {
      delta,
      reason: "browser_mall_payment_recovery",
      description: "Browser mall insufficient payment recovery top-up",
      source_event_id: sourceEventId
    }
  });
  return currentCreditBalance(fixture);
}

async function currentMallProductStock(productId) {
  const data = await apiRequest(`/mall/products/${encodeURIComponent(productId)}`);
  const product = data?.product || data;
  const stock = Number(product?.stock ?? product?.Stock);
  if (!Number.isFinite(stock)) {
    throw new Error(`Mall product ${productId} did not return numeric stock: ${JSON.stringify(data)}`);
  }
  return stock;
}

async function waitForMallProductStock(productId, expectedStock, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastStock = null;
  while (Date.now() < deadline) {
    lastStock = await currentMallProductStock(productId);
    if (lastStock === expectedStock) {
      return lastStock;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}: stock=${lastStock}, want ${expectedStock}`);
}

async function latestTopicForTitle(_fixture, title) {
  const deadline = Date.now() + 10000;
  let lastTopics = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/topics?limit=50&offset=0&type=qa");
    lastTopics = listItems(data);
    const topic = lastTopics.find((item) => String(item?.title || "") === String(title));
    if (topic?.id) {
      return topic;
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for topic title ${title}. Last topics: ${JSON.stringify(lastTopics.slice(0, 10), null, 2)}`);
}

async function waitForTopicAccepted(topicId, commentId, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastTopic = null;
  while (Date.now() < deadline) {
    const data = await apiRequest(`/topics/${encodeURIComponent(topicId)}`);
    lastTopic = data?.topic || data;
    const acceptedCommentId = lastTopic?.accepted_comment_id ?? lastTopic?.acceptedCommentId;
    const qaStatus = String(lastTopic?.qa_status ?? lastTopic?.qaStatus ?? "").toLowerCase();
    if (String(acceptedCommentId) === String(commentId) && qaStatus === "resolved") {
      return lastTopic;
    }
    await delay(300);
  }
  throw new Error(
    `Timed out waiting for ${label}: accepted=${lastTopic?.accepted_comment_id ?? lastTopic?.acceptedCommentId ?? "unknown"}, qa_status=${lastTopic?.qa_status ?? lastTopic?.qaStatus ?? "unknown"}`,
  );
}

async function waitForCreditLedgerEntry(token, predicate, label, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  let lastItems = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/credits/ledger?limit=50&offset=0", {
      token
    });
    lastItems = listItems(data);
    const entry = lastItems.find(predicate);
    if (entry?.id || entry?.ID) {
      return entry;
    }
    await delay(1000);
  }
  throw new Error(
    `Timed out waiting for ${label}. Last ledger entries: ${JSON.stringify(lastItems.slice(0, 10), null, 2)}`,
  );
}

async function latestMyTopicForTitle(fixture, title) {
  const deadline = Date.now() + 10000;
  let lastTopics = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/users/me/topics?status=0&limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastTopics = listItems(data);
    const topic = lastTopics.find((item) => String(item?.title || "") === String(title));
    if (topic?.id) {
      return topic;
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for user topic title ${title}. Last topics: ${JSON.stringify(lastTopics.slice(0, 10), null, 2)}`);
}

async function waitForDigitalEntitlement(fixture, orderId, productId, grantKey, status = "ACTIVE") {
  const entitlements = await waitForDigitalEntitlements(fixture, orderId, productId, grantKey, status, 1);
  return entitlements[0];
}

async function waitForDigitalEntitlements(fixture, orderId, productId, grantKey, status = "ACTIVE", expectedCount = 1) {
  const deadline = Date.now() + 15000;
  let lastEntitlements = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/digital-entitlements?limit=50&offset=0&status=${encodeURIComponent(status)}`, {
      token: fixture.auth.accessToken
    });
    lastEntitlements = listItems(data);
    const entitlements = lastEntitlements.filter((item) => {
      const itemOrderId = item?.order_id ?? item?.orderId;
      const itemProductId = item?.product_id ?? item?.productId;
      const itemGrantKey = item?.grant_key ?? item?.grantKey;
      return (
        String(itemOrderId) === String(orderId) &&
        String(itemProductId) === String(productId) &&
        String(itemGrantKey) === String(grantKey)
      );
    });
    if (entitlements.length >= expectedCount) {
      return entitlements.slice(0, expectedCount);
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for ${expectedCount} ${status} digital entitlements order=${orderId} product=${productId} grant=${grantKey}. Last entitlements: ${JSON.stringify(lastEntitlements.slice(0, 10), null, 2)}`);
}

async function latestRefundForOrder(fixture, orderId) {
  const data = await apiRequest("/mall/refunds?limit=50&offset=0", {
    token: fixture.auth.accessToken
  });
  return listItems(data).find((refund) => String(refund?.order_id ?? refund?.orderId ?? "") === String(orderId));
}

async function createMallRefund(fixture, orderId, note) {
  const data = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}/refunds`, {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      reason: "digital_entitlement_refund",
      note
    }
  });
  const status = Number(data?.refund?.status);
  if (!data?.refund?.id || status !== 1) {
    throw new Error(`Mall digital refund request did not create pending refund for order ${orderId}, status=${data?.refund?.status ?? "unknown"}`);
  }
  return data.refund;
}

async function latestProductReviewForOrder(fixture, orderId, productId) {
  const deadline = Date.now() + 10000;
  let lastReviews = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/reviews?product_id=${encodeURIComponent(productId)}&limit=20&offset=0`, {
      token: fixture.auth.accessToken
    });
    lastReviews = listItems(data);
    const review = lastReviews.find((item) =>
      String(item?.order_id ?? item?.orderId ?? "") === String(orderId) &&
      String(item?.product_id ?? item?.productId ?? "") === String(productId)
    );
    if (review?.id) {
      return review;
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for product review for order ${orderId}, product ${productId}. Last reviews: ${JSON.stringify(lastReviews.slice(0, 10), null, 2)}`);
}

async function assertDuplicateProductReviewRejected(fixture, orderId, productId, reviewContent) {
  const failure = await apiRequestFailure(`/mall/products/${encodeURIComponent(productId)}/reviews`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 409,
    label: "duplicate product review",
    body: {
      order_id: Number(orderId),
      rating: 5,
      content: `${reviewContent}（重复提交校验）`
    }
  });
  const legacyCode = String(failure.meta?.legacy_code || failure.meta?.legacyCode || "");
  if (legacyCode !== "AlreadyExists") {
    throw new Error(`Duplicate product review legacy code = ${legacyCode || "missing"}, want AlreadyExists. Raw: ${failure.rawBody.slice(0, 800)}`);
  }
  const frontendText = friendlyMallReviewError({
    message: failure.message,
    meta: failure.meta,
    httpCode: failure.httpCode,
    status: failure.status
  });
  if (frontendText !== "该订单已评价过该商品，请勿重复提交。") {
    throw new Error(`Duplicate product review frontend text = ${frontendText}, want duplicate review copy`);
  }
  return {
    status: failure.status,
    legacyCode,
    frontendText
  };
}

async function hideMallReview(fixture, reviewId) {
  const data = await apiRequest(`/admin/mall/reviews/${encodeURIComponent(reviewId)}/status`, {
    method: "PUT",
    token: fixture.adminToken,
    body: {
      status: 3
    }
  });
  const status = data?.review?.status;
  if (Number(status) !== 3 && !String(status).includes("HIDDEN")) {
    throw new Error(`Admin review hide did not hide review ${reviewId}, status=${status ?? "unknown"}`);
  }
  return data.review;
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

async function rejectMallRefund(fixture, refundId, adminNote) {
  const data = await apiRequest(`/admin/mall/refunds/${encodeURIComponent(refundId)}/review`, {
    method: "POST",
    token: fixture.adminToken,
    body: {
      approved: false,
      admin_note: adminNote,
      restore_stock: false
    }
  });
  const status = Number(data?.refund?.status);
  if (status !== 4) {
    throw new Error(`Admin refund rejection did not reject refund ${refundId}, status=${data?.refund?.status ?? "unknown"}`);
  }
  return data.refund;
}

async function revokeMallDigitalEntitlement(fixture, entitlementId, reason) {
  const data = await apiRequest(`/admin/mall/digital-entitlements/${encodeURIComponent(entitlementId)}/revoke`, {
    method: "POST",
    token: fixture.adminToken,
    body: { reason }
  });
  if (!data?.entitlement?.id) {
    throw new Error(`Admin entitlement revoke did not return entitlement ${entitlementId}`);
  }
  return data.entitlement;
}

async function publishMallReview(fixture, reviewId) {
  const data = await apiRequest(`/admin/mall/reviews/${encodeURIComponent(reviewId)}/status`, {
    method: "PUT",
    token: fixture.adminToken,
    body: {
      status: 2
    }
  });
  const status = data?.review?.status;
  if (Number(status) !== 2 && !String(status).includes("PUBLISHED")) {
    throw new Error(`Admin review publish did not publish review ${reviewId}, status=${status ?? "unknown"}`);
  }
  return data.review;
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

async function waitForMallReviewNotifications(fixture, reviewId, expectedTitles) {
  const deadline = Date.now() + 30000;
  const expected = expectedTitles.map((title) => String(title));
  let lastNotifications = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/notifications?limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastNotifications = listItems(data).filter((item) => notificationBelongsToReview(item, reviewId));
    const presentTitles = new Set(lastNotifications.map((item) => item.title || ""));
    if (expected.every((title) => presentTitles.has(title))) {
      return expected
        .map((title) => lastNotifications.find((item) => item.title === title))
        .filter(Boolean);
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for mall review notifications ${expected.join(", ")} for review ${reviewId}. Last notifications: ${JSON.stringify(lastNotifications.slice(0, 10), null, 2)}`);
}

function notificationBelongsToOrder(item, orderId) {
  const entityType = item?.entity_type ?? item?.entityType;
  const entityId = item?.entity_id ?? item?.entityId;
  return entityType === "mall_order" && String(entityId) === String(orderId);
}

function notificationBelongsToReview(item, reviewId) {
  const entityType = item?.entity_type ?? item?.entityType;
  const sourceId = item?.source_id ?? item?.sourceId;
  return entityType === "mall_product" && String(sourceId) === String(reviewId);
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

function orderQuantityForProduct(order, productId) {
  const normalizedProductId = String(productId);
  return (Array.isArray(order?.items) ? order.items : [])
    .filter((item) => String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "") === normalizedProductId)
    .reduce((sum, item) => sum + Number(item?.quantity ?? item?.Quantity ?? 0), 0);
}

function mallOrderStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const labels = {
    PENDING_PAYMENT: 1,
    ORDER_STATUS_PENDING_PAYMENT: 1,
    PAYING: 2,
    ORDER_STATUS_PAYING: 2,
    PAID: 3,
    ORDER_STATUS_PAID: 3,
    CANCELED: 4,
    ORDER_STATUS_CANCELED: 4,
    SHIPPED: 5,
    ORDER_STATUS_SHIPPED: 5,
    COMPLETED: 6,
    ORDER_STATUS_COMPLETED: 6,
    CLOSED: 7,
    ORDER_STATUS_CLOSED: 7,
    REFUNDED: 8,
    ORDER_STATUS_REFUNDED: 8
  };
  return labels[String(status).trim().toUpperCase()] || 0;
}

function mallPaymentStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const labels = {
    PENDING: 1,
    PAYMENT_STATUS_PENDING: 1,
    SUCCEEDED: 2,
    PAYMENT_STATUS_SUCCEEDED: 2,
    FAILED: 3,
    PAYMENT_STATUS_FAILED: 3
  };
  return labels[String(status).trim().toUpperCase()] || 0;
}

function couponUsageStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const labels = {
    RESERVED: 1,
    COUPON_USAGE_STATUS_RESERVED: 1,
    USED: 2,
    COUPON_USAGE_STATUS_USED: 2,
    RELEASED: 3,
    COUPON_USAGE_STATUS_RELEASED: 3,
    CLAIMED: 4,
    COUPON_USAGE_STATUS_CLAIMED: 4
  };
  return labels[String(status).trim().toUpperCase()] || 0;
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

async function apiRequestFailure(pathname, { method = "GET", body, token, expectedStatus, label = "api failure" } = {}) {
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
  const failed = !response.ok || (data && typeof data === "object" && "code" in data && data.code !== 0);
  if (!failed) {
    throw new Error(`${label} unexpectedly succeeded (${response.status}): ${text.slice(0, 800)}`);
  }
  if (expectedStatus && response.status !== expectedStatus) {
    throw new Error(`${label} failed with HTTP ${response.status}, want ${expectedStatus}: ${text.slice(0, 800)}`);
  }
  return {
    status: response.status,
    httpCode: Number(data?.http_code || response.status),
    message: String(data?.message || data?.reason || data?.error?.message || data?.error || ""),
    meta: data?.meta || {},
    rawBody: text
  };
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

async function waitForPublicProductReview(page, reviewContent, label = "public product review", timeoutMs = 20000) {
  await waitFor(page, `Array.from(document.querySelectorAll(".product-review-block > .product-review-item"))
    .some((item) => (item.innerText || "").includes(${JSON.stringify(reviewContent)}))`, label, timeoutMs);
  return evaluate(page, `(() => {
    const items = Array.from(document.querySelectorAll(".product-review-block > .product-review-item"));
    const item = items.find((node) => (node.innerText || "").includes(${JSON.stringify(reviewContent)}));
    return item?.innerText || "";
  })()`);
}

async function waitForPublicProductReviewHidden(page, reviewContent, label = "hidden public product review", timeoutMs = 20000) {
  await waitFor(page, `(() => {
    const block = document.querySelector(".product-review-block");
    if (!block) return false;
    if ((block.innerText || "").includes("加载中")) return false;
    return Array.from(document.querySelectorAll(".product-review-block > .product-review-item"))
      .every((item) => !(item.innerText || "").includes(${JSON.stringify(reviewContent)}));
  })()`, label, timeoutMs);
  const text = await evaluate(page, `(() => {
    const items = Array.from(document.querySelectorAll(".product-review-block > .product-review-item"));
    return items.map((item) => (item.innerText || "").trim()).filter(Boolean).join("\\n");
  })()`);
  return text || "公开评价列表已移除隐藏评价";
}

async function waitForProfileThemeClass(page, expectedClass, label = "profile theme class", timeoutMs = 20000) {
  await waitFor(page, `document.querySelector(".user-profile-card")?.classList.contains(${JSON.stringify(expectedClass)})`, label, timeoutMs);
  return evaluate(page, `document.querySelector(".user-profile-card")?.className || ""`);
}

async function waitForProfileBackgroundStyle(page, expectedUrl, label = "profile background", timeoutMs = 20000) {
  await waitFor(
    page,
    `document.querySelector(".user-profile-cover")?.style.backgroundImage.includes(${JSON.stringify(expectedUrl)})`,
    label,
    timeoutMs
  );
  return evaluate(page, `document.querySelector(".user-profile-cover")?.style.backgroundImage || ""`);
}

async function waitForProfileBackgroundCleared(page, label = "profile background cleared", timeoutMs = 20000) {
  await waitFor(
    page,
    `(() => {
      const value = document.querySelector(".user-profile-cover")?.style.backgroundImage || "";
      return value === "" || value === "none";
    })()`,
    label,
    timeoutMs
  );
  return evaluate(page, `document.querySelector(".user-profile-cover")?.style.backgroundImage || ""`);
}

async function waitForAddressDeleted(page, receiver, label = "address deleted", timeoutMs = 20000) {
  await waitFor(page, `!Array.from(document.querySelectorAll(".address-manager-panel article"))
    .some((item) => (item.innerText || "").includes(${JSON.stringify(receiver)}))`, label, timeoutMs);
}

async function waitForPublicBadgePanelReady(page, badgeTitle, label = "public badge panel", timeoutMs = 20000) {
  await waitFor(page, `(() => {
    const text = document.body?.innerText || "";
    if (!text.includes("用户空间")) return false;
    if (text.includes("正在加载用户资料") || text.includes("正在加载用户徽章")) return false;
    return text.includes(${JSON.stringify(badgeTitle)}) ||
      text.includes("暂无公开徽章") ||
      document.querySelector(".data-row") !== null;
  })()`, label, timeoutMs);
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

async function clickButtonWhenEnabled(page, buttonPattern, label = buttonPattern, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      return await clickButton(page, buttonPattern);
    } catch (error) {
      lastError = error.message || String(error);
      if (!lastError.includes("Button not found") && !lastError.includes("Button disabled")) {
        throw error;
      }
    }
    await delay(150);
  }
  const text = await bodyText(page).catch(() => "");
  throw new Error(`Timed out waiting to click ${label}${lastError ? ` (${lastError})` : ""}. Body: ${text.slice(0, 1200)}`);
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
      const prototype = field instanceof HTMLTextAreaElement
        ? HTMLTextAreaElement.prototype
        : field instanceof HTMLSelectElement
          ? HTMLSelectElement.prototype
          : HTMLInputElement.prototype;
      const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
      descriptor.set.call(field, ${JSON.stringify(value)});
      field.dispatchEvent(new Event("input", { bubbles: true }));
      field.dispatchEvent(new Event("change", { bubbles: true }));
      return field.value;
    })()`
  );
}

async function fieldValueByLabel(page, labelText) {
  return evaluate(
    page,
    `(() => {
      const labels = Array.from(document.querySelectorAll("label"));
      const label = labels.find((item) => (item.innerText || "").includes(${JSON.stringify(labelText)}));
      if (!label) throw new Error("Label not found: ${escapeForScript(labelText)}");
      const field = label.querySelector("input, textarea, select");
      if (!field) throw new Error("Field not found for label: ${escapeForScript(labelText)}");
      return field.value;
    })()`
  );
}

async function fillBySelector(page, selector, value) {
  await waitForSelector(page, selector, `field ${selector}`);
  return evaluate(
    page,
    `(() => {
      const field = document.querySelector(${JSON.stringify(selector)});
      if (!field) throw new Error("Field not found: ${escapeForScript(selector)}");
      const prototype = field instanceof HTMLTextAreaElement
        ? HTMLTextAreaElement.prototype
        : field instanceof HTMLSelectElement
          ? HTMLSelectElement.prototype
          : HTMLInputElement.prototype;
      const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
      descriptor.set.call(field, ${JSON.stringify(value)});
      field.dispatchEvent(new Event("input", { bubbles: true }));
      field.dispatchEvent(new Event("change", { bubbles: true }));
      return field.value;
    })()`
  );
}

async function fieldValueBySelector(page, selector) {
  await waitForSelector(page, selector, `field ${selector}`);
  return evaluate(
    page,
    `(() => {
      const field = document.querySelector(${JSON.stringify(selector)});
      if (!field) throw new Error("Field not found: ${escapeForScript(selector)}");
      return field.value;
    })()`
  );
}

async function waitForSelector(page, selector, label = selector, timeoutMs = 20000) {
  await waitFor(page, `document.querySelector(${JSON.stringify(selector)}) !== null`, label, timeoutMs);
}

async function setCheckboxByLabel(page, labelText, checked) {
  return evaluate(
    page,
    `(() => {
      const labels = Array.from(document.querySelectorAll("label"));
      const label = labels.find((item) => (item.innerText || "").includes(${JSON.stringify(labelText)}));
      if (!label) throw new Error("Label not found: ${escapeForScript(labelText)}");
      const field = label.querySelector("input[type='checkbox']");
      if (!field) throw new Error("Checkbox not found for label: ${escapeForScript(labelText)}");
      if (field.disabled) throw new Error("Checkbox disabled for label: ${escapeForScript(labelText)}");
      if (field.checked !== Boolean(${JSON.stringify(checked)})) {
        field.click();
      }
      return field.checked;
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

try {
  await main();
} catch (error) {
  console.error(error.stack || error.message || error);
  process.exitCode = 1;
}
