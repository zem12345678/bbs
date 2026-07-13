import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { existsSync } from "node:fs";
import { mkdtemp, readFile, readdir, rm } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const API_BASE = (
  process.env.API_BASE ||
  process.env.VITE_API_BASE ||
  "http://127.0.0.1:18080/api/v1"
).replace(/\/$/, "");
const ADMIN_BASE = (
  process.env.ADMIN_BASE ||
  process.env.VITE_ADMIN_BASE ||
  "http://127.0.0.1:8849"
).replace(/\/$/, "");
const ADMIN_ACCOUNT = process.env.ADMIN_ACCOUNT || "admin";
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "Admin123!";
const MALL_POSTGRES_DSN =
  process.env.MALL_POSTGRES_DSN ||
  process.env.BBS_MALL_POSTGRES_DSN ||
  "postgres://bbs_mall_app:local_mall_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_mall";
const PSQL_BIN = process.env.PSQL_BIN || "psql";
const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.dirname(SCRIPT_DIR);
const ADMIN_DIR = path.join(REPO_ROOT, "vue-pure-admin");
const VITE_BIN = path.join(ADMIN_DIR, "node_modules", "vite", "bin", "vite.js");
const CHECKOUT_PRICE = 18;
const CREDIT_TOP_UP = 100;

async function main() {
  const adminSession = await assertAdminApiReady();
  let fixture;
  fixture = await prepareAdminMallFixture(adminSession.token);
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
          fixtureDigitalProductId: fixture.digitalProductId,
          fixtureDigitalOrderId: fixture.digitalOrderId,
          fixtureDigitalGrantKey: fixture.digitalGrantKey,
          fixtureDigitalEntitlementCode: fixture.digitalEntitlementCode,
          fixtureDigitalRefundId: fixture.digitalRefundId,
          fixtureOutboxRequeued: fixture.outboxRequeued,
          fixtureOutboxAuditEventId: fixture.outboxAuditEventId,
          fixtureOutboxAuditOperatorId: fixture.outboxAuditOperatorId,
          overviewText: result.overviewText,
          visited: result.visited,
          exports: result.exports,
        },
        null,
        2,
      ),
    );
  } finally {
    await adminServer?.stop();
    await cleanupOutboxRequeueFixture(fixture?.outboxAuditEventId);
  }
}

async function assertAdminApiReady() {
  const data = await apiRequest("/admin/auth/login", {
    method: "POST",
    body: {
      account: ADMIN_ACCOUNT,
      password: ADMIN_PASSWORD,
    },
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
    "mall:review_refunds",
    "mall:requeue_outbox_events",
    "mall:create_product_category",
    "mall:create_product",
    "mall:create_coupon",
    "governance:adjust_user_credits",
  ]) {
    if (!permissions.includes(permission) && !permissions.includes("*:*:*")) {
      throw new Error(
        `Admin account is missing required permission: ${permission}`,
      );
    }
  }
  return {
    token,
    userId: String(data?.user?.id ?? data?.user?.Id ?? ""),
  };
}

async function prepareAdminMallFixture(adminToken) {
  const stamp = Date.now();
  const slug = `admin-e2e-${stamp}`;
  const sku = `ADMIN-E2E-${stamp}`;
  const couponCode = `ADMINE2E${stamp}`;
  const productTitle = `Admin E2E Export Product ${stamp}`;
  const digitalSku = `ADMIN-BADGE-${stamp}`;
  const digitalGrantKey = `badge-admin-e2e-${stamp}`;
  const digitalProductTitle = `Admin E2E Badge Entitlement ${stamp}`;
  await apiRequest("/admin/mall/categories", {
    method: "POST",
    token: adminToken,
    body: {
      slug,
      name: `Admin E2E ${stamp}`,
      description: "Admin browser E2E export category",
      status: 2,
      sort: 990,
    },
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
      sort: 990,
    },
  });
  const product = productResp.product;
  if (!product?.id) {
    throw new Error(
      "Admin mall fixture product creation did not return product.id",
    );
  }

  const digitalProductResp = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: digitalSku,
      title: digitalProductTitle,
      description: "Admin browser E2E digital badge entitlement",
      category: "digital",
      cover_url: "",
      grant_type: "badge",
      grant_key: digitalGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 989,
    },
  });
  const digitalProduct = digitalProductResp.product;
  if (!digitalProduct?.id) {
    throw new Error(
      "Admin mall fixture digital product creation did not return product.id",
    );
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
      ends_at: 0,
    },
  });
  const coupon = couponResp.coupon;
  if (!coupon?.id) {
    throw new Error(
      "Admin mall fixture coupon creation did not return coupon.id",
    );
  }

  const password = `Passw0rd!${stamp}`;
  const registered = await apiRequest("/auth/register", {
    method: "POST",
    body: {
      username: `admine2e${stamp}`,
      email: `admine2e${stamp}@example.com`,
      password,
      nickname: `Admin E2E ${stamp}`,
    },
  });
  const userToken = registered?.access_token || registered?.accessToken;
  const userId = registered?.user?.id;
  if (!userToken || !userId) {
    throw new Error(
      "Admin mall fixture user registration did not return auth payload",
    );
  }

  await apiRequest(
    `/admin/credits/users/${encodeURIComponent(userId)}/adjust`,
    {
      method: "POST",
      token: adminToken,
      body: {
        delta: CREDIT_TOP_UP,
        reason: "admin_browser_mall_export_topup",
        description: "Admin browser mall export fixture top-up",
        source_event_id: `admin-browser-mall-export-credit-${stamp}`,
      },
    },
  );

  const orderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-export-order-${stamp}`,
      coupon_code: couponCode,
      items: [{ product_id: product.id, quantity: 1 }],
      receiver: "管理端导出联调",
      phone: "13800000000",
      address: "上海市浦东新区导出路 1 号",
    },
  });
  const order = orderResp.order;
  if (!order?.id) {
    throw new Error(
      "Admin mall fixture order creation did not return order.id",
    );
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
      address: "上海市浦东新区导出路 1 号",
    },
  });
  if (
    !retryOrderResp.duplicate ||
    String(retryOrderResp.order?.id) !== String(order.id)
  ) {
    throw new Error(
      "Coupon order idempotency retry did not return the original order",
    );
  }

  const paymentIdempotencyKey = `admin-export-pay-${order.id}-${stamp}`;
  await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/pay`, {
    method: "POST",
    token: userToken,
    body: {
      payment_method: "credits",
      idempotency_key: paymentIdempotencyKey,
    },
  });

  await apiRequest(
    `/admin/mall/orders/${encodeURIComponent(order.id)}/status`,
    {
      method: "PUT",
      token: adminToken,
      body: {
        status: 5,
        shipping_carrier: "Admin E2E Express",
        tracking_no: `ADM${stamp}`,
        note: "管理端评价导出联调发货",
      },
    },
  );

  await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/confirm`, {
    method: "POST",
    token: userToken,
  });

  const reviewContent = `管理端评价导出联调 ${stamp}：后台审核和 CSV 留档可用。`;
  const reviewResp = await apiRequest(
    `/mall/products/${encodeURIComponent(product.id)}/reviews`,
    {
      method: "POST",
      token: userToken,
      body: {
        order_id: order.id,
        rating: 5,
        content: reviewContent,
      },
    },
  );
  const review = reviewResp.review;
  if (!review?.id) {
    throw new Error(
      "Admin mall fixture review creation did not return review.id",
    );
  }

  const refundReason = `管理端导出售后联调 ${stamp}`;
  const refundResp = await apiRequest(
    `/mall/orders/${encodeURIComponent(order.id)}/refunds`,
    {
      method: "POST",
      token: userToken,
      body: {
        reason: refundReason,
        note: "验证管理端售后 CSV 导出内容",
      },
    },
  );
  const refund = refundResp.refund;
  if (!refund?.id) {
    throw new Error(
      "Admin mall fixture refund creation did not return refund.id",
    );
  }

  const digitalOrderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-digital-order-${stamp}`,
      items: [{ product_id: digitalProduct.id, quantity: 1 }],
    },
  });
  const digitalOrder = digitalOrderResp.order;
  if (!digitalOrder?.id) {
    throw new Error(
      "Admin mall fixture digital order creation did not return order.id",
    );
  }
  await apiRequest(`/mall/orders/${encodeURIComponent(digitalOrder.id)}/pay`, {
    method: "POST",
    token: userToken,
    body: {
      payment_method: "credits",
      idempotency_key: `admin-digital-pay-${digitalOrder.id}-${stamp}`,
    },
  });
  const digitalEntitlement = await waitForDigitalEntitlement(
    userToken,
    digitalOrder.id,
    digitalProduct.id,
    digitalGrantKey,
  );
  await waitForDigitalDeliveryNotification(
    userToken,
    digitalOrder.id,
    digitalEntitlement.fulfillment_code || digitalEntitlement.fulfillmentCode,
  );
  const digitalRefundReason = `管理端数字权益售后联调 ${stamp}`;
  const digitalRefundResp = await apiRequest(
    `/mall/orders/${encodeURIComponent(digitalOrder.id)}/refunds`,
    {
      method: "POST",
      token: userToken,
      body: {
        reason: digitalRefundReason,
        note: "验证管理端数字权益撤销展示",
      },
    },
  );
  const digitalRefund = digitalRefundResp.refund;
  if (!digitalRefund?.id) {
    throw new Error(
      "Admin mall fixture digital refund creation did not return refund.id",
    );
  }
  await apiRequest(
    `/admin/mall/refunds/${encodeURIComponent(digitalRefund.id)}/review`,
    {
      method: "POST",
      token: adminToken,
      body: {
        approved: true,
        admin_note:
          "Admin digital refund approved for entitlement revoke visibility",
        restore_stock: true,
      },
    },
  );
  await waitForDigitalEntitlement(
    userToken,
    digitalOrder.id,
    digitalProduct.id,
    digitalGrantKey,
    "REVOKED",
  );

  const outboxAuditFixture = await createOutboxRequeueFixture(stamp);
  let outboxRequeueResp;
  let outboxAudit;
  try {
    outboxRequeueResp = await apiRequest("/admin/mall/outbox/requeue", {
      method: "POST",
      token: adminToken,
      body: {
        statuses: ["failed", "dead_letter"],
        limit: 1,
      },
    });
    if (typeof outboxRequeueResp?.requeued !== "number") {
      throw new Error(
        `Admin mall outbox requeue did not return numeric requeued count: ${JSON.stringify(outboxRequeueResp)}`,
      );
    }
    if (
      outboxRequeueResp.requeued < 1 ||
      !Array.isArray(outboxRequeueResp.event_ids) ||
      !outboxRequeueResp.event_ids.includes(outboxAuditFixture.eventId)
    ) {
      throw new Error(
        `Admin mall outbox requeue did not requeue controlled event ${outboxAuditFixture.eventId}: ${JSON.stringify(outboxRequeueResp)}`,
      );
    }
    outboxAudit = await waitForOutboxRequeueAudit(
      adminToken,
      outboxAuditFixture,
    );
  } catch (error) {
    await cleanupOutboxRequeueFixture(outboxAuditFixture.eventId).catch(
      (cleanupError) => {
        console.warn(
          `Failed to clean admin mall outbox audit fixture after error: ${cleanupError.message || cleanupError}`,
        );
      },
    );
    throw error;
  }

  return {
    productId: String(product.id),
    productTitle,
    sku,
    digitalProductId: String(digitalProduct.id),
    digitalProductTitle,
    digitalSku,
    digitalGrantKey,
    digitalOrderId: String(digitalOrder.id),
    digitalOrderNo:
      digitalOrder.order_no || digitalOrder.orderNo || String(digitalOrder.id),
    digitalEntitlementCode:
      digitalEntitlement.fulfillment_code ||
      digitalEntitlement.fulfillmentCode ||
      "",
    digitalRefundId: String(digitalRefund.id),
    digitalRefundReason,
    couponId: String(coupon.id),
    couponCode,
    userId: String(userId),
    orderId: String(order.id),
    orderNo: order.order_no || order.orderNo || String(order.id),
    paymentIdempotencyKey,
    reviewId: String(review.id),
    reviewContent,
    refundId: String(refund.id),
    refundReason,
    outboxRequeued: outboxRequeueResp.requeued,
    outboxAuditEventId: outboxAuditFixture.eventId,
    outboxAuditPreviousError: outboxAuditFixture.previousError,
    outboxAuditPreviousAttempts: outboxAuditFixture.previousAttempts,
    outboxAuditOperatorId:
      outboxAudit.operator_id || outboxAudit.operatorId || "",
  };
}

async function runBrowserAdminMall(chromePath, fixture) {
  const port = await getFreePort();
  const userDataDir = await mkdtemp(
    path.join(os.tmpdir(), "bbs-admin-mall-e2e-"),
  );
  const downloadDir = await mkdtemp(
    path.join(os.tmpdir(), "bbs-admin-mall-downloads-"),
  );
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
      "about:blank",
    ],
    { stdio: ["ignore", "pipe", "pipe"], windowsHide: true },
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
    await page.send("Page.setDownloadBehavior", {
      behavior: "allow",
      downloadPath: downloadDir,
    });

    await navigate(page, `${ADMIN_BASE}/#/login`);
    await waitForText(page, "登录", "admin login page");
    await fillFirstInput(page, "input:not([type='password'])", ADMIN_ACCOUNT);
    await fillFirstInput(page, "input[type='password']", ADMIN_PASSWORD);
    await waitForButtonEnabled(page, "^登录$", "admin login button");
    await clickButton(page, "^登录$");
    await waitFor(
      page,
      `location.hash.includes("/welcome") || /登录成功|首页|商城管理/.test(document.body?.innerText || "")`,
      "admin login success",
      30000,
    );

    const visited = [];
    await visitAdminMallPage(
      page,
      "/#/mall/overview",
      [
        "商城概览",
        "财务对账",
        "净收入",
        "成功收款",
        "失败支付",
        "待投递事件",
        "事件投递健康",
        "重试失败/死信",
        "最近人工重试",
        fixture.outboxAuditEventId,
        fixture.outboxAuditPreviousError,
        "累计收入",
      ],
      visited,
    );
    const overviewText = summarizeBody(await bodyText(page), [
      "财务对账",
      "净收入",
      "成功收款",
      "失败支付",
      "待投递事件",
      "事件投递健康",
      "最近人工重试",
      "累计收入",
      "待售后",
    ]);
    await visitAdminMallPage(
      page,
      "/#/mall/categories",
      ["商品分类", "新增分类", "复制链接", "预览"],
      visited,
    );
    await assertPromotionCopy(page, "分类推广链接已复制");
    await visitAdminMallPage(
      page,
      "/#/mall/products",
      ["商品管理", "新增商品", "库存流水", "复制链接", "预览"],
      visited,
    );
    await fillFirstInput(
      page,
      'input[placeholder="SKU / 商品名称"]',
      fixture.sku,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.productTitle,
      "fixture product visible in admin products",
    );
    await assertPromotionCopy(page, "商品推广链接已复制");
    await clickButtonInRow(page, fixture.productTitle, "^库存流水$");
    await waitForText(page, "库存流水", "stock log drawer");
    await waitForText(page, "初始库存|下单锁定", "stock log entries");
    const stockLogExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出流水$",
      filenamePrefix: "mall-stock-logs-",
      successPattern: "已导出",
      expectedTexts: [
        "流水ID",
        "SKU",
        fixture.sku,
        fixture.productTitle,
        "初始库存",
        "下单锁定",
      ],
    });
    await fillFirstInput(
      page,
      'input[placeholder="SKU / 商品名称"]',
      fixture.digitalSku,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.digitalProductTitle,
      "fixture digital product visible in admin products",
    );
    await waitForText(
      page,
      fixture.digitalGrantKey,
      "fixture digital grant visible in admin products",
    );
    await waitForText(
      page,
      "徽章",
      "fixture digital grant type visible in admin products",
    );
    await visitAdminMallPage(
      page,
      "/#/mall/reviews",
      ["评价管理", "商品ID", "用户ID", "评价内容", "导出评价"],
      visited,
    );
    await fillFirstInput(
      page,
      'input[placeholder="商品ID"]',
      fixture.productId,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.reviewContent,
      "fixture review visible in admin reviews",
    );
    await clickButtonInRow(page, fixture.reviewContent, "^公开$");
    await waitForText(page, "评价已公开", "fixture review published");
    await waitForText(page, "已公开", "fixture review published status");
    const reviewExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出评价$",
      filenamePrefix: "mall-product-reviews-",
      successPattern: "已导出",
      expectedTexts: [
        "评价ID",
        "商品名称",
        "评分",
        "状态",
        fixture.productTitle,
        fixture.reviewContent,
        fixture.userId,
        "已公开",
      ],
    });
    await visitAdminMallPage(
      page,
      "/#/mall/coupons",
      ["优惠券管理", "新增优惠券", "使用记录", "复制链接", "预览"],
      visited,
    );
    await fillFirstInput(
      page,
      'input[placeholder="优惠码 / 名称"]',
      fixture.couponCode,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.couponCode,
      "fixture coupon visible in admin coupons",
    );
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
      expectedTexts: [
        "使用记录ID",
        "优惠码",
        "订单ID",
        fixture.couponCode,
        fixture.orderId,
        fixture.userId,
        "已使用",
      ],
    });
    await clickButtonInRow(page, fixture.orderId, "^查看订单$");
    await waitForText(page, "订单管理", "coupon usage order deep link target");
    await waitForText(
      page,
      fixture.orderNo,
      "coupon usage linked order visible",
    );
    await visitAdminMallPage(
      page,
      "/#/mall/orders",
      ["订单管理", "导出订单", "导出支付", "订单号", "用户 ID"],
      visited,
    );
    await waitForText(
      page,
      fixture.orderNo,
      "fixture order visible in admin orders",
    );
    const orderExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出订单$",
      filenamePrefix: "mall-orders-",
      successPattern: "已导出",
      expectedTexts: [
        "订单ID",
        "订单号",
        fixture.orderNo,
        fixture.productTitle,
      ],
    });
    const paymentExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出支付$",
      filenamePrefix: "mall-payments-",
      successPattern: "已导出",
      expectedTexts: [
        "支付ID",
        "订单号",
        "支付幂等键",
        fixture.orderNo,
        fixture.paymentIdempotencyKey,
      ],
    });
    await fillFirstInput(
      page,
      'input[placeholder="订单号 / 商品"]',
      fixture.digitalOrderNo,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.digitalOrderNo,
      "fixture digital order visible in admin orders",
    );
    await waitForText(
      page,
      fixture.digitalProductTitle,
      "fixture digital order product visible in admin orders",
    );
    await waitForText(
      page,
      "数字权益 已撤销",
      "fixture digital fulfillment revoked visible in admin orders",
    );
    await clickButtonInRow(page, fixture.digitalOrderNo, "^日志$");
    await waitForText(page, "商品明细", "digital order records item detail");
    await waitForText(
      page,
      fixture.digitalGrantKey,
      "digital order grant snapshot visible in admin records",
    );
    await waitForText(
      page,
      "已撤销",
      "digital entitlement revoked state visible in admin records",
    );
    await waitForText(
      page,
      fixture.digitalRefundId,
      "digital entitlement refund id visible in admin records",
    );
    if (fixture.digitalEntitlementCode) {
      await waitForText(
        page,
        fixture.digitalEntitlementCode,
        "digital entitlement code visible in admin records",
      );
    }
    await visitAdminMallPage(
      page,
      "/#/mall/refunds",
      ["售后管理", "导出售后", "订单号", "退款积分", "状态"],
      visited,
    );
    await waitForText(
      page,
      fixture.orderNo,
      "fixture refund visible in admin refunds",
    );
    const refundExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出售后$",
      filenamePrefix: "mall-refunds-",
      successPattern: "已导出",
      expectedTexts: [
        "售后ID",
        "订单号",
        "数字权益",
        fixture.orderNo,
        fixture.refundReason,
        fixture.digitalOrderNo,
        fixture.digitalRefundReason,
        fixture.digitalGrantKey,
        fixture.digitalEntitlementCode,
        "已撤销",
        `退款 ${fixture.digitalRefundId}`,
      ],
    });
    await fillFirstInput(
      page,
      'input[placeholder="订单号 / 原因"]',
      fixture.digitalOrderNo,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.digitalOrderNo,
      "fixture digital refund visible in admin refunds",
    );
    await waitForText(
      page,
      fixture.digitalRefundReason,
      "fixture digital refund reason visible in admin refunds",
    );
    await clickButtonInRow(page, fixture.digitalOrderNo, "^详情$");
    await waitForText(page, "关联订单", "digital refund detail related order");
    await waitForText(
      page,
      fixture.digitalProductTitle,
      "digital refund detail product title",
    );
    await waitForText(
      page,
      fixture.digitalGrantKey,
      "digital refund detail grant key",
    );
    await waitForText(
      page,
      "已撤销",
      "digital refund detail revoked entitlement status",
    );
    await waitForText(
      page,
      fixture.digitalRefundId,
      "digital refund detail entitlement refund id",
    );

    const seriousIssues = issues.filter(isSeriousBrowserIssue);
    if (seriousIssues.length > 0) {
      throw new Error(
        `Admin browser reported ${seriousIssues.length} serious issue(s): ${JSON.stringify(seriousIssues.slice(0, 5), null, 2)}`,
      );
    }
    return {
      overviewText,
      visited,
      exports: {
        stockLogs: stockLogExport,
        productReviews: reviewExport,
        couponUsages: couponUsageExport,
        orders: orderExport,
        payments: paymentExport,
        refunds: refundExport,
      },
    };
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

async function assertCsvExport(
  page,
  downloadDir,
  { buttonPattern, filenamePrefix, successPattern, expectedTexts },
) {
  const before = new Set(await safeReadDir(downloadDir));
  await clickButton(page, buttonPattern);
  await waitForText(
    page,
    successPattern,
    `${filenamePrefix} export success`,
    10000,
  );
  const filename = await waitForDownloadedFile(
    downloadDir,
    before,
    filenamePrefix,
  );
  const filePath = path.join(downloadDir, filename);
  const content = await readFile(filePath, "utf8");
  for (const expected of expectedTexts) {
    if (!content.includes(expected)) {
      throw new Error(
        `Exported CSV ${filename} did not include ${JSON.stringify(expected)}. Preview: ${content.slice(0, 500)}`,
      );
    }
  }
  return {
    filename,
    rows: content.split(/\r?\n/).filter(Boolean).length,
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
  throw new Error(
    `Timed out waiting for downloaded ${filenamePrefix} CSV. Current files: ${(await safeReadDir(downloadDir)).join(", ")}`,
  );
}

async function safeReadDir(dir) {
  return readdir(dir).catch(() => []);
}

function collectBrowserIssues(page) {
  const issues = [];
  page.on("Runtime.exceptionThrown", (event) => {
    issues.push({
      type: "pageerror",
      text:
        event.exceptionDetails?.exception?.description ||
        event.exceptionDetails?.text ||
        "Runtime exception",
    });
  });
  page.on("Runtime.consoleAPICalled", (event) => {
    if (event.type === "error") {
      issues.push({
        type: "console:error",
        text: (event.args || [])
          .map((arg) => arg.value || arg.description || "")
          .join(" ")
          .slice(0, 800),
      });
    }
  });
  page.on("Log.entryAdded", (event) => {
    const entry = event.entry || {};
    if (entry.level === "error") {
      issues.push({
        type: "log:error",
        text: entry.text || "",
        url: entry.url || "",
      });
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
  return (
    url.startsWith(API_BASE) ||
    url.startsWith(`${ADMIN_BASE}/api/`) ||
    url.includes("/api/v1/admin/")
  );
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
  return needles
    .map((needle) => lines.find((line) => line.includes(needle)) || needle)
    .join(" · ");
}

async function createOutboxRequeueFixture(stamp) {
  const eventId = `admin-e2e-outbox-requeue-${stamp}-${randomUUID()}`;
  const aggregateId = Number(stamp);
  const previousAttempts = 7;
  const previousError = `admin mall e2e simulated publisher failure ${stamp}`;
  const payload = {
    event_id: eventId,
    event_type: "mall.e2e.requeue_audit.v1",
    occurred_at_unix_ms: stamp,
    aggregate_type: "admin_e2e",
    aggregate_id: aggregateId,
  };
  await runMallPsql(`
    SET search_path TO bbs_mall;
    INSERT INTO mall_outbox_events (
      event_id, aggregate_type, aggregate_id, event_type, message_key, payload_json,
      status, attempts, lease_owner, last_error, created_at, updated_at
    ) VALUES (
      ${pgLiteral(eventId)}, 'admin_e2e', ${aggregateId}, 'mall.e2e.requeue_audit.v1',
      ${pgLiteral(eventId)}, ${pgLiteral(JSON.stringify(payload))}::jsonb,
      'dead_letter', ${previousAttempts}, '', ${pgLiteral(previousError)},
      TIMESTAMPTZ '1970-01-01 00:00:00+00',
      TIMESTAMPTZ '1970-01-01 00:00:00+00'
    );
  `);
  return { eventId, aggregateId, previousAttempts, previousError };
}

async function waitForOutboxRequeueAudit(adminToken, fixture) {
  const deadline = Date.now() + 10000;
  let lastAudits = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(
      `/admin/mall/outbox/requeue-audits?limit=5&offset=0&event_id=${encodeURIComponent(fixture.eventId)}`,
      { token: adminToken },
    );
    lastAudits = Array.isArray(data?.items) ? data.items : [];
    const audit = lastAudits.find(
      (item) => String(item?.event_id ?? item?.eventId) === fixture.eventId,
    );
    if (audit?.id) {
      const previousStatus = audit.previous_status ?? audit.previousStatus;
      const previousAttempts = Number(
        audit.previous_attempts ?? audit.previousAttempts ?? 0,
      );
      const previousError = audit.previous_error ?? audit.previousError ?? "";
      if (
        previousStatus !== "dead_letter" ||
        previousAttempts !== fixture.previousAttempts ||
        previousError !== fixture.previousError
      ) {
        throw new Error(
          `Outbox requeue audit fields mismatch for ${fixture.eventId}: ${JSON.stringify(audit)}`,
        );
      }
      return audit;
    }
    await delay(300);
  }
  throw new Error(
    `Timed out waiting for outbox requeue audit ${fixture.eventId}. Last audits: ${JSON.stringify(lastAudits, null, 2)}`,
  );
}

async function cleanupOutboxRequeueFixture(eventId) {
  if (!eventId) return;
  await runMallPsql(`
    SET search_path TO bbs_mall;
    DELETE FROM mall_outbox_requeue_audits WHERE event_id = ${pgLiteral(eventId)};
    DELETE FROM mall_outbox_events WHERE event_id = ${pgLiteral(eventId)};
  `);
}

function runMallPsql(sql) {
  return new Promise((resolve, reject) => {
    const child = spawn(
      PSQL_BIN,
      [
        "--dbname",
        mallPsqlDsn(),
        "--no-align",
        "--tuples-only",
        "--set",
        "ON_ERROR_STOP=1",
        "--command",
        sql,
      ],
      { stdio: ["ignore", "pipe", "pipe"], windowsHide: true },
    );
    const stdout = [];
    const stderr = [];
    child.stdout?.on("data", (chunk) => stdout.push(String(chunk)));
    child.stderr?.on("data", (chunk) => stderr.push(String(chunk)));
    child.on("error", (error) => {
      reject(
        new Error(
          `Failed to run psql for mall outbox E2E fixture. Set PSQL_BIN if psql is not in PATH. ${error.message}`,
        ),
      );
    });
    child.on("close", (code) => {
      if (code === 0) {
        resolve(stdout.join(""));
        return;
      }
      reject(
        new Error(
          `psql failed while preparing mall outbox E2E fixture (${code}): ${stderr.join("").slice(0, 800)}`,
        ),
      );
    });
  });
}

function mallPsqlDsn() {
  try {
    const url = new URL(MALL_POSTGRES_DSN);
    url.searchParams.delete("search_path");
    return url.toString();
  } catch {
    return MALL_POSTGRES_DSN;
  }
}

function pgLiteral(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

async function waitForDigitalEntitlement(
  userToken,
  orderId,
  productId,
  grantKey,
  status = "ACTIVE",
) {
  const deadline = Date.now() + 15000;
  let lastEntitlements = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(
      `/mall/digital-entitlements?limit=50&offset=0&status=${encodeURIComponent(status)}`,
      {
        token: userToken,
      },
    );
    lastEntitlements = Array.isArray(data?.items) ? data.items : [];
    const entitlement = lastEntitlements.find((item) => {
      const itemOrderId = item?.order_id ?? item?.orderId;
      const itemProductId = item?.product_id ?? item?.productId;
      const itemGrantKey = item?.grant_key ?? item?.grantKey;
      return (
        String(itemOrderId) === String(orderId) &&
        String(itemProductId) === String(productId) &&
        String(itemGrantKey) === String(grantKey)
      );
    });
    if (entitlement?.id) {
      return entitlement;
    }
    await delay(500);
  }
  throw new Error(
    `Timed out waiting for ${status} digital entitlement order=${orderId} product=${productId} grant=${grantKey}. Last entitlements: ${JSON.stringify(lastEntitlements.slice(0, 10), null, 2)}`,
  );
}

async function waitForDigitalDeliveryNotification(userToken, orderId, code) {
  const deadline = Date.now() + 15000;
  let lastNotifications = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/notifications?limit=100&offset=0", {
      token: userToken,
    });
    lastNotifications = Array.isArray(data?.items) ? data.items : [];
    const notification = lastNotifications.find((item) => {
      const entityId = item?.entity_id ?? item?.entityId;
      const type = item?.type || "";
      const content = item?.content || "";
      return (
        type === "mall_order_paid" &&
        String(entityId) === String(orderId) &&
        content.includes("数字权益已发放") &&
        content.includes(code)
      );
    });
    if (notification?.id) {
      return notification;
    }
    await delay(500);
  }
  throw new Error(
    `Timed out waiting for digital delivery notification order=${orderId} code=${code}. Last notifications: ${JSON.stringify(lastNotifications.slice(0, 10), null, 2)}`,
  );
}

async function apiRequest(pathname, { method = "GET", body, token } = {}) {
  const response = await fetch(`${API_BASE}${pathname}`, {
    method,
    headers: {
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  const data = parseResponseBody(text);
  if (
    !response.ok ||
    (data && typeof data === "object" && "code" in data && data.code !== 0)
  ) {
    throw new Error(
      `${method} ${pathname} failed (${response.status}): ${text.slice(0, 800)}`,
    );
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
    throw new Error(
      `admin frontend is not reachable at ${ADMIN_BASE}, and Vite was not found at ${VITE_BIN}. Run pnpm install in vue-pure-admin or start the admin frontend manually.`,
    );
  }

  const adminUrl = new URL(ADMIN_BASE);
  const host = adminUrl.hostname || "127.0.0.1";
  const port = adminUrl.port || (adminUrl.protocol === "https:" ? "443" : "80");
  const logs = [];
  const server = spawn(
    process.execPath,
    [VITE_BIN, "--host", host, "--port", port, "--strictPort"],
    {
      cwd: ADMIN_DIR,
      env: {
        ...process.env,
        VITE_API_PROXY_TARGET: API_BASE.replace(/\/api\/v1$/, ""),
      },
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    },
  );

  server.stdout?.on("data", (chunk) => pushLog(logs, chunk));
  server.stderr?.on("data", (chunk) => pushLog(logs, chunk));
  try {
    await waitForHttpReachable(
      ADMIN_BASE,
      "admin frontend",
      60000,
      server,
      logs,
    );
  } catch (error) {
    await stopProcess(server);
    throw error;
  }
  console.log(`Started admin dev server for mall e2e at ${ADMIN_BASE}`);
  return {
    stop: () => stopProcess(server),
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
      throw new Error(
        `${label} dev server exited early with code ${child.exitCode}. Logs:\n${logs.join("").slice(-3000)}`,
      );
    }
    if (await httpReachable(url)) return;
    await delay(250);
  }
  throw new Error(
    `Timed out waiting for ${label} at ${url}. Logs:\n${logs.join("").slice(-3000)}`,
  );
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
  const explicit =
    process.env.CHROME_EXECUTABLE ||
    process.env.ADMIN_MALL_E2E_CHROME ||
    process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE;
  const candidates = [
    explicit,
    ...(await puppeteerChromeCandidates()),
    "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
    "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
    path.join(
      os.homedir(),
      "AppData",
      "Local",
      "Google",
      "Chrome",
      "Application",
      "chrome.exe",
    ),
    "C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
    "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
  ].filter(Boolean);
  const match = candidates.find((candidate) => existsSync(candidate));
  if (!match) {
    throw new Error(
      "Chrome/Chromium executable not found. Set CHROME_EXECUTABLE to run admin mall e2e.",
    );
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
      throw new Error(
        `Chrome exited early with code ${chrome.exitCode}: ${stderr.join("").slice(0, 1000)}`,
      );
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
  throw new Error(
    `Timed out waiting for Chrome DevTools endpoint: ${stderr.join("").slice(0, 1000)}`,
  );
}

async function createPageWebSocket(port, browserWs) {
  const response = await fetch(
    `http://127.0.0.1:${port}/json/new?about:blank`,
    { method: "PUT" },
  ).catch(() => null);
  if (response?.ok) {
    const data = await response.json();
    if (data.webSocketDebuggerUrl) return data.webSocketDebuggerUrl;
  }

  const browser = new CDPClient(browserWs);
  await browser.connect();
  try {
    const { targetId } = await browser.send("Target.createTarget", {
      url: "about:blank",
    });
    const list = await fetch(`http://127.0.0.1:${port}/json/list`).then(
      (item) => item.json(),
    );
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
  await waitFor(
    page,
    `document.readyState === "complete" || document.readyState === "interactive"`,
    `navigate ${url}`,
    30000,
  );
  await delay(700);
}

async function waitForText(page, pattern, label = pattern, timeoutMs = 30000) {
  const source = pattern instanceof RegExp ? pattern.source : String(pattern);
  await waitFor(
    page,
    `new RegExp(${JSON.stringify(source)}, "i").test(document.body?.innerText || "")`,
    label,
    timeoutMs,
  );
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
  throw new Error(
    `Timed out waiting for ${label}${lastError ? ` (${lastError})` : ""}. Body: ${text.slice(0, 1400)}`,
  );
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
    })()`,
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
    })()`,
  );
}

async function waitForButtonEnabled(
  page,
  buttonPattern,
  label = buttonPattern,
  timeoutMs = 30000,
) {
  await waitFor(
    page,
    `(() => {
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const button = Array.from(document.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      return Boolean(button && !button.disabled);
    })()`,
    label,
    timeoutMs,
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
    })()`,
  );
}

async function evaluate(page, expression) {
  const response = await page.send("Runtime.evaluate", {
    expression,
    awaitPromise: true,
    returnByValue: true,
    userGesture: true,
  });
  if (response.exceptionDetails) {
    const detail = response.exceptionDetails;
    throw new Error(
      detail.exception?.description || detail.text || "Runtime.evaluate failed",
    );
  }
  return response.result?.value;
}

function escapeForScript(value) {
  return String(value)
    .replace(/\\/g, "\\\\")
    .replace(/"/g, '\\"')
    .replace(/\n/g, "\\n");
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
      this.ws.addEventListener(
        "error",
        () => reject(new Error(`Could not connect to ${this.url}`)),
        { once: true },
      );
      this.ws.addEventListener("message", (event) =>
        this.handleMessage(event.data),
      );
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
        pending.reject(
          new Error(`${pending.method} failed: ${message.error.message}`),
        );
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
