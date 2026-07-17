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
const ADMIN_POSTGRES_DSN =
  process.env.ADMIN_POSTGRES_DSN ||
  process.env.BBS_ADMIN_POSTGRES_DSN ||
  process.env.BBS_ADMIN_TEST_DSN ||
  "postgres://bbs_admin_app:local_admin_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_admin";
const PSQL_BIN = process.env.PSQL_BIN || "psql";
const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.dirname(SCRIPT_DIR);
const ADMIN_DIR = path.join(REPO_ROOT, "vue-pure-admin");
const VITE_BIN = path.join(ADMIN_DIR, "node_modules", "vite", "bin", "vite.js");
const CHECKOUT_PRICE = 18;
const CREDIT_TOP_UP = 100;
const REQUIRED_ADMIN_PERMISSIONS = [
  "mall:list_product_categories",
  "mall:create_product_category",
  "mall:update_product_category",
  "mall:list_products",
  "mall:create_product",
  "mall:update_product",
  "mall:list_product_reviews",
  "mall:update_product_review",
  "mall:list_coupons",
  "mall:list_coupon_usages",
  "mall:create_coupon",
  "mall:update_coupon",
  "mall:list_orders",
  "mall:close_expired_orders",
  "mall:recover_paying_orders",
  "mall:requeue_outbox_events",
  "mall:update_order_status",
  "mall:list_order_logs",
  "mall:list_order_payments",
  "mall:list_digital_entitlements",
  "mall:revoke_digital_entitlement",
  "mall:list_refunds",
  "mall:review_refunds",
  "governance:adjust_user_credits",
];
const ADMIN_MALL_E2E_MENU_SEEDS = [
  {
    name: "mall",
    title: "商城管理",
    icon: "ri/store-2-line",
    path: "/mall",
    type: "M",
    sort: 1200,
    status: "0",
    visible: "0",
    is_hide: "0",
    remark: "admin mall e2e mall root",
  },
  {
    name: "mall.entitlements",
    title: "权益台账",
    icon: "ri/shield-check-line",
    path: "/mall/entitlements",
    component: "mall/entitlements/index",
    type: "C",
    permission: "mall:list_digital_entitlements",
    parentName: "mall",
    sort: 1235,
    status: "0",
    visible: "0",
    is_hide: "0",
    remark: "admin mall e2e digital entitlements",
  },
  {
    name: "mall.entitlements.query",
    title: "查询",
    type: "F",
    permission: "mall:list_digital_entitlements",
    parentName: "mall.entitlements",
    sort: 1236,
    status: "0",
    visible: "1",
    is_hide: "1",
    remark: "admin mall e2e digital entitlement query button",
  },
  {
    name: "mall.entitlements.revoke",
    title: "撤销",
    type: "F",
    permission: "mall:revoke_digital_entitlement",
    parentName: "mall.entitlements",
    sort: 1237,
    status: "0",
    visible: "1",
    is_hide: "1",
    remark: "admin mall e2e digital entitlement revoke button",
  },
];

async function main() {
  const adminSession = await assertAdminApiReady();
  let fixture;
  fixture = await prepareAdminMallFixture(adminSession.token);
  const adminServer = await ensureAdminServer();
  await assertAdminFrontendProxyRevokeRouteReady(adminSession.token);
  try {
    const chromePath = await findChromeExecutable();
    const result = await runBrowserAdminMall(chromePath, fixture, adminSession);
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
          fixtureExpiringProductId: fixture.expiringProductId,
          fixtureExpiringOrderId: fixture.expiringOrderId,
          fixtureExpiringOrderNo: fixture.expiringOrderNo,
          fixtureRecoveringProductId: fixture.recoveringProductId,
          fixtureRecoveringOrderId: fixture.recoveringOrderId,
          fixtureRecoveringOrderNo: fixture.recoveringOrderNo,
          fixtureRecoveringPaymentIdempotencyKey:
            fixture.recoveringPaymentIdempotencyKey,
          fixtureRecoverPayingRecovered: fixture.recoverPayingRecovered,
          fixtureRecoverPayingFailed: fixture.recoverPayingFailed,
          fixtureMembershipProductId: fixture.membershipProductId,
          fixtureMembershipOrderId: fixture.membershipOrderId,
          fixtureMembershipGrantKey: fixture.membershipGrantKey,
          fixtureMembershipEntitlementCode: fixture.membershipEntitlementCode,
          fixtureMembershipExpiresAt: fixture.membershipExpiresAt,
          fixtureFinanceAnomalyOrderId: fixture.financeAnomalyOrderId,
          fixtureFinanceAnomalyOrderNo: fixture.financeAnomalyOrderNo,
          issuedCouponTermsUpdateRejected:
            result.issuedCouponTermsUpdateRejected,
          soldProductFulfillmentUpdateRejected:
            result.soldProductFulfillmentUpdateRejected,
          soldProductGrantUpdateRejected: result.soldProductGrantUpdateRejected,
          fixtureMembershipRevokeReason: result.membershipRevokeReason,
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
  let data = await adminLogin();
  const repaired = await ensureAdminMallE2ePermissions(data);
  if (repaired) {
    data = await adminLogin();
  }
  const permissions = data?.permissions || [];
  for (const permission of REQUIRED_ADMIN_PERMISSIONS) {
    if (!hasAdminPermission(permissions, permission)) {
      throw new Error(
        `Admin account is missing required permission: ${permission}`,
      );
    }
  }
  const token = data?.access_token || data?.accessToken;
  await assertAdminMallRevokeRouteReady(token);
  return {
    token,
    userId: String(data?.user?.id ?? data?.user?.Id ?? ""),
  };
}

async function adminLogin() {
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
  return data;
}

async function ensureAdminMallE2ePermissions(adminData) {
  const permissions = adminData?.permissions || [];
  const missing = REQUIRED_ADMIN_PERMISSIONS.filter(
    (permission) => !hasAdminPermission(permissions, permission),
  );
  const missingMallPermissions = missing.filter((permission) =>
    permission.startsWith("mall:"),
  );
  if (missingMallPermissions.length === 0) {
    return false;
  }
  const token = adminData?.access_token || adminData?.accessToken;
  if (!token) {
    return false;
  }

  let menus = await listSystemMenuFlatItems(token);
  const byName = new Map(menus.map((item) => [String(item.name || ""), item]));
  const ensuredMenuIDs = [];
  for (const seed of ADMIN_MALL_E2E_MENU_SEEDS) {
    const parentID = seed.parentName
      ? Number(byName.get(seed.parentName)?.id || 0)
      : 0;
    if (seed.parentName && !parentID) {
      throw new Error(
        `Cannot ensure admin mall e2e menu ${seed.name}: parent ${seed.parentName} was not found`,
      );
    }
    const payload = {
      parent_id: parentID,
      name: seed.name,
      title: seed.title,
      icon: seed.icon || "",
      path: seed.path || "",
      component: seed.component || "",
      type: seed.type,
      permission: seed.permission || "",
      auths: seed.permission || "",
      status: seed.status || "0",
      visible: seed.visible || "0",
      is_hide: seed.is_hide || "0",
      sort: seed.sort,
      rank: seed.sort,
      remark: seed.remark || "admin mall e2e menu",
    };
    const existing = byName.get(seed.name);
    const response = await apiRequest(
      existing?.id
        ? `/admin/system/menus/${encodeURIComponent(existing.id)}`
        : "/admin/system/menus",
      {
        method: existing?.id ? "PUT" : "POST",
        token,
        body: payload,
      },
    );
    const menu = response?.menu || response;
    if (!menu?.id) {
      throw new Error(
        `Admin system menu upsert for ${seed.name} did not return menu.id`,
      );
    }
    byName.set(seed.name, menu);
    ensuredMenuIDs.push(Number(menu.id));
  }

  const roles = await listSystemRoles(token);
  const protectedRoles = roles.filter((item) =>
    ["admin", "superadmin"].includes(String(item.key || item.roleKey || "")),
  );
  const protectedRoleKeys = new Set(
    protectedRoles
      .map((role) => String(role.key || role.roleKey || ""))
      .filter(Boolean),
  );
  for (const role of protectedRoles) {
    const roleID = Number(role.id || 0);
    if (!roleID) continue;
    const menuIDs = new Set(
      (await systemRoleMenuIDs(token, role)).map((id) => Number(id)),
    );
    let changed = false;
    for (const menuID of ensuredMenuIDs) {
      if (menuID > 0 && !menuIDs.has(menuID)) {
        menuIDs.add(menuID);
        changed = true;
      }
    }
    if (changed) {
      try {
        await apiRequest(
          `/admin/system/roles/${encodeURIComponent(roleID)}/menus`,
          {
            method: "PUT",
            token,
            body: { menu_ids: Array.from(menuIDs).filter((id) => id > 0) },
          },
        );
      } catch (error) {
        if (!isProtectedSystemRoleError(error)) {
          throw error;
        }
      }
    }
  }
  if (protectedRoleKeys.size > 0) {
    await ensureAdminE2eCasbinPolicies(
      Array.from(protectedRoleKeys),
      missingMallPermissions,
    );
  }
  return true;
}

function hasAdminPermission(permissions, permission) {
  return permissions.includes(permission) || permissions.includes("*:*:*");
}

async function assertAdminMallRevokeRouteReady(token) {
  const response = await fetch(
    `${API_BASE}/admin/mall/digital-entitlements/0/revoke`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ reason: "admin mall e2e route probe" }),
    },
  );
  const text = await response.text();
  if (response.status === 404) {
    throw new Error(
      "Admin mall digital entitlement revoke route returned 404. Restart api-gateway from the current source before running admin mall e2e.",
    );
  }
  if (response.status === 401 || response.status === 403) {
    throw new Error(
      `Admin mall digital entitlement revoke route rejected the admin token (${response.status}): ${text.slice(0, 500)}`,
    );
  }
  if (response.status >= 500) {
    throw new Error(
      `Admin mall digital entitlement revoke route probe failed (${response.status}): ${text.slice(0, 500)}`,
    );
  }
}

async function assertAdminFrontendProxyRevokeRouteReady(token) {
  const response = await fetch(
    `${ADMIN_BASE}/api/v1/admin/mall/digital-entitlements/0/revoke`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ reason: "admin mall e2e frontend proxy probe" }),
    },
  );
  const text = await response.text();
  if (response.status === 404) {
    throw new Error(
      `Admin frontend API proxy returned 404 for the digital entitlement revoke route. Restart the admin frontend with VITE_API_PROXY_TARGET=${API_BASE.replace(/\/api\/v1$/, "")}, or set ADMIN_BASE to an unused port so this script can start Vite with the correct proxy target.`,
    );
  }
  if (response.status === 401 || response.status === 403) {
    throw new Error(
      `Admin frontend API proxy rejected the admin token (${response.status}): ${text.slice(0, 500)}`,
    );
  }
  if (response.status >= 500) {
    throw new Error(
      `Admin frontend API proxy route probe failed (${response.status}): ${text.slice(0, 500)}`,
    );
  }
}

async function listSystemMenuFlatItems(token) {
  const data = await apiRequest("/admin/system/menus", { token });
  return data?.flatList || data?.flat_items || data?.items || data?.list || [];
}

async function listSystemRoles(token) {
  const data = await apiRequest("/admin/system/roles?page_size=100&pageSize=100", {
    token,
  });
  return data?.items || data?.list || [];
}

async function systemRoleMenuIDs(token, role) {
  const inlineIDs = role?.menu_ids || role?.menuIds || role?.ids;
  if (Array.isArray(inlineIDs) && inlineIDs.length > 0) {
    return inlineIDs;
  }
  const roleID = role?.id;
  const data = await apiRequest(
    `/admin/system/roles/${encodeURIComponent(roleID)}/menu-ids`,
    { token },
  );
  return data?.menu_ids || data?.menuIds || data?.ids || [];
}

function isProtectedSystemRoleError(error) {
  const message = String(error?.message || "");
  return (
    message.includes("内置管理员角色不能修改") ||
    message.includes("PermissionDenied")
  );
}

async function ensureAdminE2eCasbinPolicies(roleKeys, permissions) {
  const rows = [];
  for (const roleKey of roleKeys) {
    for (const permission of permissions) {
      const [resource, action] = String(permission).split(":");
      if (!roleKey || !resource || !action) continue;
      rows.push(
        `(${pgLiteral("p")}, ${pgLiteral(roleKey)}, ${pgLiteral(resource)}, ${pgLiteral(action)}, '', '', '')`,
      );
    }
  }
  if (rows.length === 0) {
    return;
  }
  await runAdminPsql(`
    SET search_path TO bbs_admin;
    INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4, v5)
    VALUES ${rows.join(",\n      ")}
    ON CONFLICT (ptype, v0, v1, v2, v3, v4, v5) DO NOTHING;
  `);
}

function runAdminPsql(sql) {
  return runPsql(adminPsqlDsn(), "admin RBAC E2E fixture", sql);
}

async function prepareAdminMallFixture(adminToken) {
  const stamp = Date.now();
  const slug = `admin-e2e-${stamp}`;
  const sku = `ADMIN-E2E-${stamp}`;
  const expiringSku = `ADMIN-EXPIRE-${stamp}`;
  const recoveringSku = `ADMIN-RECOVER-${stamp}`;
  const couponCode = `ADMINE2E${stamp}`;
  const productTitle = `Admin E2E Export Product ${stamp}`;
  const expiringProductTitle = `Admin E2E Expiring Order ${stamp}`;
  const recoveringProductTitle = `Admin E2E Recover Paying ${stamp}`;
  const digitalSku = `ADMIN-BADGE-${stamp}`;
  const digitalGrantKey = `badge-admin-e2e-${stamp}`;
  const digitalProductTitle = `Admin E2E Badge Entitlement ${stamp}`;
  const membershipSku = `ADMIN-VIP-${stamp}`;
  const membershipGrantKey = `vip-admin-e2e-${stamp}`;
  const membershipProductTitle = `Admin E2E Membership Entitlement ${stamp}`;
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

  const expiringProductResp = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: expiringSku,
      title: expiringProductTitle,
      description: "Admin browser E2E expiring order fixture",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 1,
      status: 2,
      sort: 991,
    },
  });
  const expiringProduct = expiringProductResp.product;
  if (!expiringProduct?.id) {
    throw new Error(
      "Admin mall fixture expiring product creation did not return product.id",
    );
  }

  const recoveringProductResp = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: recoveringSku,
      title: recoveringProductTitle,
      description: "Admin browser E2E stale paying recovery fixture",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 2,
      status: 2,
      sort: 992,
    },
  });
  const recoveringProduct = recoveringProductResp.product;
  if (!recoveringProduct?.id) {
    throw new Error(
      "Admin mall fixture recovering product creation did not return product.id",
    );
  }

  const digitalProductResp = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: digitalSku,
      title: digitalProductTitle,
      description: "Admin browser E2E digital badge entitlement",
      category: "badge",
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

  const membershipProductResp = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: membershipSku,
      title: membershipProductTitle,
      description: "Admin browser E2E membership entitlement",
      category: "digital",
      cover_url: "",
      grant_type: "membership",
      grant_key: membershipGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 988,
    },
  });
  const membershipProduct = membershipProductResp.product;
  if (!membershipProduct?.id) {
    throw new Error(
      "Admin mall fixture membership product creation did not return product.id",
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

  const financeAnomalyOrderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-finance-anomaly-order-${stamp}`,
      items: [{ product_id: product.id, quantity: 1 }],
      receiver: "管理端对账异常联调",
      phone: "13800000000",
      address: "上海市浦东新区对账路 1 号",
    },
  });
  const financeAnomalyOrder = financeAnomalyOrderResp.order;
  if (!financeAnomalyOrder?.id) {
    throw new Error(
      "Admin mall fixture finance anomaly order creation did not return order.id",
    );
  }
  const financeAnomalyPaymentIdempotencyKey = `admin-finance-anomaly-extra-pay-${financeAnomalyOrder.id}-${stamp}`;
  await apiRequest(
    `/mall/orders/${encodeURIComponent(financeAnomalyOrder.id)}/pay`,
    {
      method: "POST",
      token: userToken,
      body: {
        payment_method: "credits",
        idempotency_key: `admin-finance-anomaly-pay-${financeAnomalyOrder.id}-${stamp}`,
      },
    },
  );
  await createFinanceAnomalyPaymentFixture(
    financeAnomalyOrder,
    userId,
    financeAnomalyPaymentIdempotencyKey,
  );
  await waitForFinanceAnomaly(adminToken, financeAnomalyOrder);

  const expiringOrderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-expire-order-${stamp}`,
      items: [{ product_id: expiringProduct.id, quantity: 1 }],
      receiver: "管理端超时关闭联调",
      phone: "13800000000",
      address: "上海市浦东新区超时路 1 号",
    },
  });
  const expiringOrder = expiringOrderResp.order;
  if (!expiringOrder?.id) {
    throw new Error(
      "Admin mall fixture expiring order creation did not return order.id",
    );
  }
  if (Number(expiringOrder.status ?? 0) !== 1) {
    throw new Error(
      `Admin mall expiring order did not create pending payment order: ${JSON.stringify(expiringOrder)}`,
    );
  }
  const expiringProductAfterOrder = await apiRequest(
    `/mall/products/${encodeURIComponent(expiringProduct.id)}`,
  );
  if (productStock(expiringProductAfterOrder?.product, 0) !== 0) {
    throw new Error(
      `Admin mall expiring product stock was not locked after order creation: ${JSON.stringify(expiringProductAfterOrder)}`,
    );
  }

  const recoveringOrderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-recover-order-${stamp}`,
      items: [{ product_id: recoveringProduct.id, quantity: 1 }],
      receiver: "管理端支付补偿联调",
      phone: "13800000000",
      address: "上海市浦东新区补偿路 1 号",
    },
  });
  const recoveringOrder = recoveringOrderResp.order;
  if (!recoveringOrder?.id) {
    throw new Error(
      "Admin mall fixture recovering order creation did not return order.id",
    );
  }
  if (Number(recoveringOrder.status ?? 0) !== 1) {
    throw new Error(
      `Admin mall recovering order did not create pending payment order: ${JSON.stringify(recoveringOrder)}`,
    );
  }
  const recoveringPaymentIdempotencyKey = `admin-recover-pay-${recoveringOrder.id}-${stamp}`;
  await markOrderStalePaying(
    recoveringOrder,
    userId,
    recoveringPaymentIdempotencyKey,
  );
  const recoveringOrderPaying = await apiRequest(
    `/mall/orders/${encodeURIComponent(recoveringOrder.id)}`,
    { token: userToken },
  );
  if (Number(recoveringOrderPaying?.order?.status ?? 0) !== 2) {
    throw new Error(
      `Admin mall recovering order did not move to PAYING fixture state: ${JSON.stringify(recoveringOrderPaying)}`,
    );
  }
  const recoveringPendingPayments = await apiRequest(
    `/admin/mall/orders/${encodeURIComponent(recoveringOrder.id)}/payments`,
    { token: adminToken },
  );
  const recoveringPendingPayment = (Array.isArray(
    recoveringPendingPayments?.items,
  )
    ? recoveringPendingPayments.items
    : []
  ).find(
    (item) =>
      paymentIdempotencyKeyOf(item) === recoveringPaymentIdempotencyKey &&
      Number(item.status ?? 0) === 1,
  );
  if (!recoveringPendingPayment) {
    throw new Error(
      `Admin mall recovering order did not expose pending payment fixture: ${JSON.stringify(recoveringPendingPayments)}`,
    );
  }

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
  if (
    String(digitalOrder.receiver || "").trim() ||
    String(digitalOrder.phone || "").trim() ||
    String(digitalOrder.address || "").trim()
  ) {
    throw new Error(
      `Admin granted order unexpectedly stored shipping information: ${JSON.stringify({ receiver: digitalOrder.receiver, phone: digitalOrder.phone, address: digitalOrder.address })}`,
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

  const membershipOrderResp = await apiRequest("/mall/orders", {
    method: "POST",
    token: userToken,
    body: {
      idempotency_key: `admin-membership-order-${stamp}`,
      items: [{ product_id: membershipProduct.id, quantity: 1 }],
    },
  });
  const membershipOrder = membershipOrderResp.order;
  if (!membershipOrder?.id) {
    throw new Error(
      "Admin mall fixture membership order creation did not return order.id",
    );
  }
  await apiRequest(`/mall/orders/${encodeURIComponent(membershipOrder.id)}/pay`, {
    method: "POST",
    token: userToken,
    body: {
      payment_method: "credits",
      idempotency_key: `admin-membership-pay-${membershipOrder.id}-${stamp}`,
    },
  });
  const membershipEntitlement = await waitForDigitalEntitlement(
    userToken,
    membershipOrder.id,
    membershipProduct.id,
    membershipGrantKey,
  );

  await delay(1500);
  const expiredOrderResp = await apiRequest("/admin/mall/orders/expire", {
    method: "POST",
    token: adminToken,
    body: {
      expire_after_seconds: 1,
      limit: 100,
    },
  });
  const expiredOrderItems = Array.isArray(expiredOrderResp?.items)
    ? expiredOrderResp.items
    : [];
  const expiredOrderClosed = expiredOrderItems.find(
    (item) =>
      String(item.id ?? item.order_id ?? item.orderId ?? "") ===
        String(expiringOrder.id) &&
      Number(item.status ?? item.order_status ?? item.orderStatus ?? 0) === 7,
  );
  if (!expiredOrderClosed) {
    throw new Error(
      `Admin mall close expired orders did not close fixture order ${expiringOrder.id}: ${JSON.stringify(expiredOrderResp)}`,
    );
  }
  const expiringOrderAfterClose = await apiRequest(
    `/mall/orders/${encodeURIComponent(expiringOrder.id)}`,
    { token: userToken },
  );
  if (Number(expiringOrderAfterClose?.order?.status ?? 0) !== 7) {
    throw new Error(
      `Admin mall expiring order did not move to closed: ${JSON.stringify(expiringOrderAfterClose)}`,
    );
  }
  const expiringProductAfterClose = await apiRequest(
    `/mall/products/${encodeURIComponent(expiringProduct.id)}`,
  );
  if (productStock(expiringProductAfterClose?.product, -1) !== 1) {
    throw new Error(
      `Admin mall expiring order close did not release product stock: ${JSON.stringify(expiringProductAfterClose)}`,
    );
  }
  const expiringOrderLogs = await apiRequest(
    `/mall/orders/${encodeURIComponent(expiringOrder.id)}/logs`,
    {
      token: userToken,
    },
  );
  const expiredOrderLog = (Array.isArray(expiringOrderLogs?.items)
    ? expiringOrderLogs.items
    : []
  ).find((item) => String(item.reason ?? "") === "expired");
  if (
    !expiredOrderLog ||
    !String(expiredOrderLog.note ?? "").includes("订单超时未支付，系统自动关闭")
  ) {
    throw new Error(
      `Admin mall expiring order logs did not include expired status log: ${JSON.stringify(expiringOrderLogs)}`,
    );
  }
  const expiredStockLogs = await apiRequest(
    `/admin/mall/products/${encodeURIComponent(expiringProduct.id)}/stock-logs?reason=order_expired&limit=20&offset=0`,
    {
      token: adminToken,
    },
  );
  const expiredStockLog = (Array.isArray(expiredStockLogs?.items)
    ? expiredStockLogs.items
    : []
  ).find(
    (item) =>
      String(item.reference_id ?? item.referenceId ?? "") ===
        String(expiringOrder.id) &&
      Number(item.after_stock ?? item.afterStock ?? -1) === 1 &&
      String(item.reason ?? "") === "order_expired",
  );
  if (!expiredStockLog) {
    throw new Error(
      `Admin mall stock logs did not include expired order stock release: ${JSON.stringify(expiredStockLogs)}`,
    );
  }

  const recoverPayingResp = await apiRequest(
    "/admin/mall/orders/recover-paying",
    {
      method: "POST",
      token: adminToken,
      body: {
        stale_after_seconds: 1,
        limit: 100,
      },
    },
  );
  if (Number(recoverPayingResp?.recovered ?? 0) < 1) {
    throw new Error(
      `Admin mall recover paying orders did not recover any order: ${JSON.stringify(recoverPayingResp)}`,
    );
  }
  const recoveringOrderAfterRecovery = await apiRequest(
    `/mall/orders/${encodeURIComponent(recoveringOrder.id)}`,
    { token: userToken },
  );
  if (Number(recoveringOrderAfterRecovery?.order?.status ?? 0) !== 3) {
    throw new Error(
      `Admin mall recovering order did not move to PAID after recovery: ${JSON.stringify(recoveringOrderAfterRecovery)}`,
    );
  }
  const recoveringOrderLogs = await apiRequest(
    `/mall/orders/${encodeURIComponent(recoveringOrder.id)}/logs`,
    {
      token: userToken,
    },
  );
  const recoveringPaidLog = (Array.isArray(recoveringOrderLogs?.items)
    ? recoveringOrderLogs.items
    : []
  ).find((item) => String(item.reason ?? "") === "paid");
  if (!recoveringPaidLog) {
    throw new Error(
      `Admin mall recovering order logs did not include paid status log: ${JSON.stringify(recoveringOrderLogs)}`,
    );
  }
  const recoveringSucceededPayments = await apiRequest(
    `/admin/mall/orders/${encodeURIComponent(recoveringOrder.id)}/payments`,
    { token: adminToken },
  );
  const recoveringSucceededPayment = (Array.isArray(
    recoveringSucceededPayments?.items,
  )
    ? recoveringSucceededPayments.items
    : []
  ).find(
    (item) =>
      paymentIdempotencyKeyOf(item) === recoveringPaymentIdempotencyKey &&
      Number(item.status ?? 0) === 2 &&
      String(item.provider_trade_no ?? item.providerTradeNo ?? "").startsWith(
        "credit-",
      ),
  );
  if (!recoveringSucceededPayment) {
    throw new Error(
      `Admin mall recovering order did not expose succeeded payment after recovery: ${JSON.stringify(recoveringSucceededPayments)}`,
    );
  }

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
    categorySlug: slug,
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
    expiringProductId: String(expiringProduct.id),
    expiringProductTitle,
    expiringSku,
    expiringOrderId: String(expiringOrder.id),
    expiringOrderNo:
      expiringOrder.order_no || expiringOrder.orderNo || String(expiringOrder.id),
    recoveringProductId: String(recoveringProduct.id),
    recoveringProductTitle,
    recoveringSku,
    recoveringOrderId: String(recoveringOrder.id),
    recoveringOrderNo:
      recoveringOrder.order_no ||
      recoveringOrder.orderNo ||
      String(recoveringOrder.id),
    recoveringPaymentIdempotencyKey,
    recoverPayingRecovered: Number(recoverPayingResp?.recovered ?? 0),
    recoverPayingFailed: Number(recoverPayingResp?.failed ?? 0),
    membershipProductId: String(membershipProduct.id),
    membershipProductTitle,
    membershipSku,
    membershipOrderId: String(membershipOrder.id),
    membershipOrderNo:
      membershipOrder.order_no || membershipOrder.orderNo || String(membershipOrder.id),
    membershipGrantKey,
    membershipEntitlementCode:
      membershipEntitlement.fulfillment_code ||
      membershipEntitlement.fulfillmentCode ||
      "",
    membershipExpiresAt:
      membershipEntitlement.expires_at ||
      membershipEntitlement.expiresAt ||
      "",
    couponId: String(coupon.id),
    couponCode,
    userId: String(userId),
    orderId: String(order.id),
    orderNo: order.order_no || order.orderNo || String(order.id),
    paymentIdempotencyKey,
    financeAnomalyOrderId: String(financeAnomalyOrder.id),
    financeAnomalyOrderNo:
      financeAnomalyOrder.order_no ||
      financeAnomalyOrder.orderNo ||
      String(financeAnomalyOrder.id),
    financeAnomalyPaymentIdempotencyKey,
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

async function runBrowserAdminMall(chromePath, fixture, adminSession) {
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
    await fillFirstInput(
      page,
      ".login-form input:not([type='password']):not([type='checkbox'])",
      ADMIN_ACCOUNT,
    );
    await fillFirstInput(page, ".login-form input[type='password']", ADMIN_PASSWORD);
    await clickButtonInContainer(page, ".login-form", "^登录$");
    try {
      await waitFor(
        page,
        `location.hash.includes("/welcome") || /登录成功|首页|商城管理/.test(document.body?.innerText || "")`,
        "admin login success",
        30000,
      );
    } catch (error) {
      const diagnostics = await loginDiagnostics(page, issues).catch(
        (diagnosticError) => ({ error: diagnosticError.message }),
      );
      throw new Error(
        `${error.message}\nLogin diagnostics: ${JSON.stringify(diagnostics, null, 2)}`,
      );
    }

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
        fixture.financeAnomalyOrderNo,
        "收款与订单不一致",
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
    await clickButtonInBlock(
      page,
      ".finance-anomaly-list > div",
      fixture.financeAnomalyOrderNo,
      "^查看订单$",
    );
    await waitForText(page, "订单管理", "finance anomaly order deep link target");
    await waitForText(
      page,
      fixture.financeAnomalyOrderNo,
      "finance anomaly linked order visible",
    );
    await waitFor(
      page,
      `decodeURIComponent(location.hash || "").includes(${JSON.stringify(`keyword=${fixture.financeAnomalyOrderNo}`)})`,
      "finance anomaly order keyword applied",
      10000,
    );
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
    const soldProductFulfillmentUpdateRejected =
      await assertSoldProductFulfillmentUpdateRejected(page, fixture);
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
    await closeDrawer(page);
    await fillFirstInput(
      page,
      'input[placeholder="SKU / 商品名称"]',
      fixture.expiringSku,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.expiringProductTitle,
      "fixture expiring product visible in admin products",
    );
    await clickButtonInRow(page, fixture.expiringProductTitle, "^库存流水$");
    await waitForText(page, "库存流水", "expiring stock log drawer");
    await waitForText(
      page,
      fixture.expiringOrderId,
      "expired order reference visible in stock logs",
    );
    await waitForText(
      page,
      "超时释放",
      "expired stock release visible in stock logs",
    );
    await closeDrawer(page);
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
    const soldProductGrantUpdateRejected =
      await assertSoldProductGrantUpdateRejected(page, fixture);
    await fillFirstInput(
      page,
      'input[placeholder="SKU / 商品名称"]',
      fixture.membershipSku,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.membershipProductTitle,
      "fixture membership product visible in admin products",
    );
    await waitForText(
      page,
      fixture.membershipGrantKey,
      "fixture membership grant visible in admin products",
    );
    await waitForText(
      page,
      "会员",
      "fixture membership grant type visible in admin products",
    );
    await visitAdminMallPage(
      page,
      `/#/mall/reviews?product_id=${encodeURIComponent(fixture.productId)}&status=1`,
      ["评价管理", "商品ID", "用户ID", "评价内容", "导出评价"],
      visited,
    );
    await waitForText(
      page,
      fixture.reviewContent,
      "fixture review visible from admin review deep link",
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
    await visitAdminMallPage(
      page,
      `/#/mall/reviews?product_id=${encodeURIComponent(fixture.productId)}&status=2`,
      ["评价管理", "商品ID", "用户ID", "评价内容", "导出评价"],
      visited,
    );
    await waitForText(
      page,
      fixture.reviewContent,
      "published fixture review visible before export",
    );
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
      `/#/mall/coupons?coupon_id=${encodeURIComponent(fixture.couponId)}&usage_user_id=${encodeURIComponent(fixture.userId)}&usage_status=2`,
      ["优惠券管理", "新增优惠券", "使用记录", "复制链接", "预览"],
      visited,
    );
    await waitForText(
      page,
      "优惠券使用记录",
      "coupon usage drawer from admin coupon deep link",
    );
    await waitForText(page, fixture.couponCode, "deep linked coupon usage code");
    await waitForText(page, fixture.orderId, "deep linked coupon usage order id");
    await waitForText(page, fixture.userId, "deep linked coupon usage user id");
    await waitForText(page, "已使用", "deep linked coupon usage used status");
    await closeDrawer(page);
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
    const issuedCouponTermsUpdateRejected =
      await assertIssuedCouponTermsUpdateRejected(page, fixture);
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
      [
        "订单管理",
        "关闭超时订单",
        "补偿支付中",
        "导出订单",
        "导出支付",
        "订单号",
        "用户 ID",
      ],
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
        fixture.membershipGrantKey,
        "有效至",
      ],
    });
    await fillFirstInput(
      page,
      'input[placeholder="订单号 / 商品"]',
      fixture.orderNo,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.orderNo,
      "fixture order re-filtered for payment export",
    );
    const paymentExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出支付$",
      filenamePrefix: "mall-payments-",
      successPattern: "已导出 1 条支付记录",
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
      fixture.expiringOrderNo,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.expiringOrderNo,
      "fixture expiring order visible in admin orders",
    );
    await waitForText(
      page,
      "已关闭",
      "fixture expiring order closed status visible in admin orders",
    );
    await clickButtonInRow(page, fixture.expiringOrderNo, "^日志$");
    await waitForText(page, "订单记录", "expired order records drawer");
    await waitForText(
      page,
      "订单超时未支付，系统自动关闭",
      "expired order log note visible in admin records",
    );
    await waitForText(
      page,
      "已关闭",
      "expired order status visible in admin records",
    );
    await closeDrawer(page);
    await fillFirstInput(
      page,
      'input[placeholder="订单号 / 商品"]',
      fixture.recoveringOrderNo,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.recoveringOrderNo,
      "fixture recovering order visible in admin orders",
    );
    await waitForText(
      page,
      fixture.recoveringProductTitle,
      "fixture recovering order product visible in admin orders",
    );
    await waitForText(
      page,
      "已支付",
      "fixture recovering order paid status visible in admin orders",
    );
    await clickButtonInRow(page, fixture.recoveringOrderNo, "^支付$");
    await waitForText(page, "支付记录", "recovering order payments drawer");
    await waitForText(
      page,
      fixture.recoveringPaymentIdempotencyKey,
      "recovering order payment idempotency key visible in admin records",
    );
    await waitForText(
      page,
      "成功",
      "recovering order succeeded payment visible in admin records",
    );
    await closeDrawer(page);
    await clickButtonInRow(page, fixture.recoveringOrderNo, "^日志$");
    await waitForText(page, "订单记录", "recovering order records drawer");
    await waitForText(
      page,
      "支付中",
      "recovering order paying status log visible in admin records",
    );
    await waitForText(
      page,
      "已支付",
      "recovering order paid status log visible in admin records",
    );
    await closeDrawer(page);
    await fillFirstInput(
      page,
      'input[placeholder="订单号 / 商品"]',
      fixture.membershipOrderNo,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.membershipOrderNo,
      "fixture membership order visible in admin orders",
    );
    await waitForText(
      page,
      fixture.membershipProductTitle,
      "fixture membership order product visible in admin orders",
    );
    await waitForText(
      page,
      "数字权益 可用 1 项",
      "fixture membership fulfillment summary visible in admin orders",
    );
    await clickButtonInRow(page, fixture.membershipOrderNo, "^日志$");
    await waitForText(
      page,
      fixture.membershipGrantKey,
      "membership order grant key visible in admin records",
    );
    await waitForText(page, "有效至", "membership expiry visible in admin records");
    if (fixture.membershipEntitlementCode) {
      await waitForText(
        page,
        fixture.membershipEntitlementCode,
        "membership entitlement code visible in admin records",
      );
    }
    await closeDrawer(page);
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
    await clickButtonInRow(page, fixture.digitalGrantKey, "^台账$");
    await waitForText(page, "权益台账", "order detail entitlement ledger link target");
    await waitForText(
      page,
      fixture.digitalProductTitle,
      "order detail linked entitlement visible in admin entitlements",
    );
    await waitForText(
      page,
      fixture.digitalGrantKey,
      "order detail linked entitlement grant visible in admin entitlements",
    );
    await waitForText(
      page,
      "已撤销",
      "order detail linked entitlement status visible in admin entitlements",
    );
    await visitAdminMallPage(
      page,
      `/#/mall/orders?keyword=${encodeURIComponent(fixture.digitalOrderNo)}`,
      ["订单管理", fixture.digitalOrderNo],
      visited,
    );
    await clickButtonInRow(page, fixture.digitalOrderNo, "^日志$");
    await waitForText(
      page,
      fixture.digitalGrantKey,
      "digital order records reopened before refund link",
    );
    await clickButtonInRow(page, fixture.digitalGrantKey, "^售后$");
    await waitForText(page, "售后管理", "order detail entitlement refund link target");
    await waitForText(
      page,
      fixture.digitalOrderNo,
      "order detail linked refund visible in admin refunds",
    );
    await waitForText(
      page,
      fixture.digitalRefundReason,
      "order detail linked refund reason visible in admin refunds",
    );
    await visitAdminMallPage(
      page,
      `/#/mall/entitlements?grant_type=membership&grant_key=${encodeURIComponent(fixture.membershipGrantKey)}&status=ACTIVE`,
      ["权益台账", fixture.membershipGrantKey, "会员权益"],
      visited,
    );
    await waitForText(
      page,
      fixture.membershipProductTitle,
      "fixture membership entitlement visible from admin entitlement deep link",
    );
    await waitForText(
      page,
      "可用",
      "fixture active membership entitlement status visible from deep link",
    );
    await visitAdminMallPage(
      page,
      "/#/mall/entitlements",
      ["权益台账", "导出台账", "关键词", "授权Key", "售后"],
      visited,
    );
    await waitForText(
      page,
      fixture.membershipProductTitle,
      "fixture membership entitlement visible in admin entitlements",
    );
    await waitForText(
      page,
      fixture.membershipGrantKey,
      "fixture membership grant visible in admin entitlements",
    );
    await waitForText(
      page,
      "会员权益",
      "fixture membership grant label visible in admin entitlements",
    );
    await waitForText(
      page,
      fixture.digitalProductTitle,
      "fixture digital entitlement visible in admin entitlements",
    );
    await waitForText(
      page,
      fixture.digitalGrantKey,
      "fixture digital grant visible in admin entitlements",
    );
    await waitForText(
      page,
      "已撤销",
      "fixture revoked entitlement visible in admin entitlements",
    );
    await waitForText(
      page,
      fixture.digitalRefundId,
      "fixture entitlement refund id visible in admin entitlements",
    );
    await clickButtonInRow(page, fixture.digitalGrantKey, "^订单$");
    await waitForText(page, "订单管理", "entitlement ledger order link target");
    await waitForText(
      page,
      fixture.digitalOrderNo,
      "entitlement ledger linked order visible",
    );
    await visitAdminMallPage(
      page,
      "/#/mall/entitlements",
      ["权益台账", fixture.digitalGrantKey, fixture.digitalRefundId],
      visited,
    );
    await clickButtonInRow(page, fixture.digitalGrantKey, "^售后$");
    await waitForText(page, "售后管理", "entitlement ledger refund link target");
    await waitForText(
      page,
      fixture.digitalOrderNo,
      "entitlement ledger linked refund visible",
    );
    await waitForText(
      page,
      fixture.digitalRefundReason,
      "entitlement ledger linked refund reason visible",
    );
    await visitAdminMallPage(
      page,
      "/#/mall/entitlements",
      ["权益台账", fixture.membershipGrantKey, fixture.digitalGrantKey],
      visited,
    );
    const entitlementExportExpectedTexts = [
      "权益ID",
      "用户ID",
      "授权类型",
      "授权Key",
      fixture.membershipProductTitle,
      fixture.membershipGrantKey,
      fixture.digitalProductTitle,
      fixture.digitalGrantKey,
      fixture.digitalRefundId,
      "已撤销",
      "会员权益",
      "徽章权益",
    ];
    if (fixture.membershipEntitlementCode) {
      entitlementExportExpectedTexts.push(fixture.membershipEntitlementCode);
    }
    if (fixture.digitalEntitlementCode) {
      entitlementExportExpectedTexts.push(fixture.digitalEntitlementCode);
    }
    const entitlementExport = await assertCsvExport(page, downloadDir, {
      buttonPattern: "^导出台账$",
      filenamePrefix: "mall-digital-entitlements-",
      successPattern: "已导出",
      expectedTexts: entitlementExportExpectedTexts,
    });
    const membershipRevokeReason = `admin e2e revoke ${randomUUID()}`;
    await fillFirstInput(
      page,
      'input[placeholder="订单号/交付码/SKU/授权"]',
      fixture.membershipEntitlementCode || fixture.membershipGrantKey,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.membershipProductTitle,
      "fixture membership entitlement filtered before revoke",
    );
    await clickButtonInRow(page, fixture.membershipProductTitle, "^撤销$");
    await submitMessageBoxPrompt(
      page,
      "撤销权益",
      membershipRevokeReason,
      "^确认撤销$",
    );
    await waitForText(page, "撤销已提交", "manual membership entitlement revoke submitted");
    await waitForText(
      page,
      membershipRevokeReason,
      "manual membership entitlement revoke reason visible",
    );
    await waitForText(
      page,
      "已撤销",
      "manual membership entitlement revoked state visible",
    );
    await assertAdminMembershipEntitlementRevoked(
      adminSession.token,
      fixture,
      membershipRevokeReason,
      adminSession.userId,
    );
    await fillFirstInput(
      page,
      'input[placeholder="订单号/交付码/SKU/授权"]',
      fixture.digitalEntitlementCode || fixture.digitalGrantKey,
    );
    await clickButton(page, "^查询$");
    await waitForText(
      page,
      fixture.digitalProductTitle,
      "fixture digital entitlement filtered in admin entitlements",
    );
    await waitForText(
      page,
      fixture.digitalGrantKey,
      "fixture digital grant filtered in admin entitlements",
    );
    await waitForText(
      page,
      "已撤销",
      "fixture digital revoked state filtered in admin entitlements",
    );
    await visitAdminMallPage(
      page,
      `/#/mall/refunds?refund_id=${encodeURIComponent(fixture.digitalRefundId)}&status=3`,
      ["售后管理", "导出售后", "订单号", "退款积分", "状态"],
      visited,
    );
    await waitForText(
      page,
      fixture.digitalOrderNo,
      "fixture digital refund visible from admin refund deep link",
    );
    await waitForText(
      page,
      fixture.digitalRefundReason,
      "fixture digital refund reason visible from admin refund deep link",
    );
    await waitForText(
      page,
      "已退款",
      "fixture digital refund status visible from admin refund deep link",
    );
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
      'input[placeholder="退款ID / 订单号 / 原因"]',
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
        entitlements: entitlementExport,
        refunds: refundExport,
      },
      membershipRevokeReason,
      issuedCouponTermsUpdateRejected,
      soldProductFulfillmentUpdateRejected,
      soldProductGrantUpdateRejected,
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

async function assertIssuedCouponTermsUpdateRejected(page, fixture) {
  await clickButtonInRow(page, fixture.couponCode, "^编辑$");
  await waitForText(page, "编辑优惠券", "coupon edit dialog");
  await fillFormInputByLabel(page, "优惠积分", "4");
  await clickButtonInContainer(page, ".el-dialog", "^保存$");
  await waitForText(
    page,
    "coupon terms cannot be changed after coupon usages",
    "issued coupon terms update rejected",
    10000,
  );
  await waitForText(
    page,
    "编辑优惠券",
    "coupon edit dialog remains open after rejection",
    5000,
  );
  await clickButtonInContainer(page, ".el-dialog", "^取消$");
  await waitFor(
    page,
    `!Array.from(document.querySelectorAll(".el-dialog")).some((item) => {
      const style = window.getComputedStyle(item);
      const rect = item.getBoundingClientRect();
      return style.display !== "none" && style.visibility !== "hidden" && rect.width > 0 && rect.height > 0;
    })`,
    "coupon edit dialog closed after rejection",
    10000,
  );
  await waitForText(
    page,
    fixture.couponCode,
    "original coupon remains visible after rejected terms edit",
  );
  return true;
}

async function assertSoldProductFulfillmentUpdateRejected(page, fixture) {
  await clickButtonInRow(page, fixture.productTitle, "^编辑$");
  await waitForText(page, "编辑商品", "physical product edit dialog");
  await selectOptionByFormLabel(page, "分类", "digital");
  await waitForText(
    page,
    "选择授权类型时需要同时填写授权 Key",
    "digital fulfillment hint after category change",
    5000,
  );
  await clickButtonInContainer(page, ".el-dialog", "^保存$");
  await waitForText(
    page,
    "product fulfillment cannot be changed after paid orders",
    "sold product fulfillment update rejected",
    10000,
  );
  await waitForText(
    page,
    "编辑商品",
    "physical product edit dialog remains open after rejection",
    5000,
  );
  await clickButtonInContainer(page, ".el-dialog", "^取消$");
  await waitFor(
    page,
    `!Array.from(document.querySelectorAll(".el-dialog")).some((item) => {
      const style = window.getComputedStyle(item);
      const rect = item.getBoundingClientRect();
      return style.display !== "none" && style.visibility !== "hidden" && rect.width > 0 && rect.height > 0;
    })`,
    "physical product edit dialog closed after rejection",
    10000,
  );
  await waitForText(
    page,
    fixture.categorySlug,
    "original product category remains visible after rejected fulfillment edit",
  );
  return true;
}

async function assertSoldProductGrantUpdateRejected(page, fixture) {
  await clickButtonInRow(page, fixture.digitalProductTitle, "^编辑$");
  await waitForText(page, "编辑商品", "digital product edit dialog");
  await fillFirstInput(
    page,
    '.el-dialog input[placeholder="例如 badge-founder / theme-pro / vip-month"]',
    `${fixture.digitalGrantKey}-mutated`,
  );
  await clickButtonInContainer(page, ".el-dialog", "^保存$");
  await waitForText(
    page,
    "product grant cannot be changed after paid orders",
    "sold product grant update rejected",
    10000,
  );
  await waitForText(
    page,
    "编辑商品",
    "product edit dialog remains open after rejection",
    5000,
  );
  await clickButtonInContainer(page, ".el-dialog", "^取消$");
  await waitFor(
    page,
    `!Array.from(document.querySelectorAll(".el-dialog")).some((item) => {
      const style = window.getComputedStyle(item);
      const rect = item.getBoundingClientRect();
      return style.display !== "none" && style.visibility !== "hidden" && rect.width > 0 && rect.height > 0;
    })`,
    "product edit dialog closed after rejection",
    10000,
  );
  await waitForText(
    page,
    fixture.digitalGrantKey,
    "original digital grant remains visible after rejected edit",
  );
  await waitFor(
    page,
    `!(document.body?.innerText || "").includes(${JSON.stringify(`${fixture.digitalGrantKey}-mutated`)})`,
    "mutated digital grant key not visible after rejected edit",
    10000,
  );
  return true;
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

async function loginDiagnostics(page, issues) {
  const form = await evaluate(
    page,
    `(() => {
      const inputs = Array.from(document.querySelectorAll(".login-form input")).map((input) => ({
        type: input.type,
        placeholder: input.getAttribute("placeholder") || "",
        valueLength: String(input.value || "").length,
        disabled: input.disabled,
      }));
      const buttons = Array.from(document.querySelectorAll(".login-form button")).map((button) => ({
        text: (button.innerText || button.textContent || "").trim(),
        disabled: button.disabled,
      }));
      return {
        href: location.href,
        hash: location.hash,
        inputs,
        buttons,
        localStorageKeys: Object.keys(localStorage).filter((key) => /token|user|permission|router|admin/i.test(key)).sort(),
      };
    })()`,
  );
  return {
    form,
    issues: issues.filter(isSeriousBrowserIssue).slice(-10),
  };
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
  if (isExpectedSoldProductGrantLockIssue(issue)) return false;
  if (isExpectedIssuedCouponTermsLockIssue(issue)) return false;
  if (isExpectedElementPlusDetachedQueryIssue(issue)) return false;
  if (isExpectedExternalStaticAssetNetworkIssue(issue)) return false;
  if (/favicon|manifest|websocket|ws:\/\//i.test(text)) return false;
  if (/Download the Vue Devtools|DevTools/i.test(text)) return false;
  return true;
}

function isExpectedExternalStaticAssetNetworkIssue(issue) {
  const url = String(issue.url || "");
  const text = String(issue.text || "");
  return (
    issue.type === "log:error" &&
    /Failed to load resource: net::ERR_NETWORK_ACCESS_DENIED/i.test(text) &&
    (/^https:\/\/xiaoxian521\.github\.io\/hyperlink\/svg\/smile[125]\.svg(?:[?#]|$)/i.test(
      url,
    ) ||
      /^https:\/\/images\.unsplash\.com\/photo-1516321318423-f06f85e504b3(?:[?#]|$)/i.test(
        url,
      ))
  );
}

function isExpectedElementPlusDetachedQueryIssue(issue) {
  const text = String(issue.text || "");
  return (
    issue.type === "pageerror" &&
    text.includes("Cannot read properties of null (reading 'querySelector')") &&
    /\/node_modules\/\.vite\/deps\/es-[^/]+\.js\?v=/i.test(text) &&
    text.includes("vue.runtime.esm-bundler")
  );
}

function isExpectedSoldProductGrantLockIssue(issue) {
  const url = String(issue.url || "");
  const text = String(issue.text || "");
  return (
    /\/api\/v1\/admin\/mall\/products\/\d+/.test(url) &&
    (Number(issue.status || 0) === 412 || /412|Precondition Failed/i.test(text))
  );
}

function isExpectedIssuedCouponTermsLockIssue(issue) {
  const url = String(issue.url || "");
  const text = String(issue.text || "");
  return (
    /\/api\/v1\/admin\/mall\/coupons\/\d+/.test(url) &&
    (Number(issue.status || 0) === 412 || /412|Precondition Failed/i.test(text))
  );
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
  return runPsql(mallPsqlDsn(), "mall outbox E2E fixture", sql);
}

function runPsql(dsn, label, sql) {
  return new Promise((resolve, reject) => {
    const child = spawn(
      PSQL_BIN,
      [
        "--dbname",
        dsn,
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
          `Failed to run psql for ${label}. Set PSQL_BIN if psql is not in PATH. ${error.message}`,
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
          `psql failed while preparing ${label} (${code}): ${stderr.join("").slice(0, 800)}`,
        ),
      );
    });
  });
}

function mallPsqlDsn() {
  return psqlDsnWithoutSearchPath(MALL_POSTGRES_DSN);
}

function adminPsqlDsn() {
  return psqlDsnWithoutSearchPath(ADMIN_POSTGRES_DSN);
}

function psqlDsnWithoutSearchPath(dsn) {
  try {
    const url = new URL(dsn);
    url.searchParams.delete("search_path");
    return url.toString();
  } catch {
    return dsn;
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

async function assertAdminMembershipEntitlementRevoked(
  adminToken,
  fixture,
  revokeReason,
  expectedOperatorId = "",
) {
  const data = await apiRequest(
    `/admin/mall/digital-entitlements?user_id=${encodeURIComponent(fixture.userId)}&status=REVOKED&grant_type=membership&grant_key=${encodeURIComponent(fixture.membershipGrantKey)}&limit=50&offset=0`,
    {
      token: adminToken,
    },
  );
  const items = Array.isArray(data?.items) ? data.items : [];
  const entitlement = items.find((item) => {
    const itemOrderId = item?.order_id ?? item?.orderId;
    const itemProductId = item?.product_id ?? item?.productId;
    const itemGrantKey = item?.grant_key ?? item?.grantKey;
    const itemRevokeReason = item?.revoke_reason ?? item?.revokeReason;
    return (
      String(itemOrderId) === String(fixture.membershipOrderId) &&
      String(itemProductId) === String(fixture.membershipProductId) &&
      String(itemGrantKey) === String(fixture.membershipGrantKey) &&
      String(itemRevokeReason || "") === String(revokeReason)
    );
  });
  if (!entitlement?.id) {
    throw new Error(
      `Timed out waiting for revoked membership entitlement order=${fixture.membershipOrderId} product=${fixture.membershipProductId} grant=${fixture.membershipGrantKey}. Last entitlements: ${JSON.stringify(items.slice(0, 10), null, 2)}`,
    );
  }
  const revokedBy = entitlement.revoked_by ?? entitlement.revokedBy ?? "";
  if (!revokedBy) {
    throw new Error(
      `Revoked membership entitlement missing revoked_by: ${JSON.stringify(entitlement, null, 2)}`,
    );
  }
  if (expectedOperatorId && String(revokedBy) !== String(expectedOperatorId)) {
    throw new Error(
      `Revoked membership entitlement operator mismatch: got ${revokedBy}, want ${expectedOperatorId}`,
    );
  }
  return entitlement;
}

async function submitMessageBoxPrompt(
  page,
  titlePattern,
  value,
  confirmButtonPattern = "^确认$",
) {
  await waitForText(page, titlePattern, titlePattern, 10000);
  await fillFirstInput(page, ".el-message-box textarea", value);
  await clickButton(page, confirmButtonPattern);
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

function productStock(product, missingDefault = -1) {
  return Number(
    product?.stock ?? product?.stock_count ?? product?.stockCount ?? missingDefault,
  );
}

function paymentIdempotencyKeyOf(payment) {
  return String(payment?.idempotency_key ?? payment?.idempotencyKey ?? "");
}

async function waitForFinanceAnomaly(adminToken, order) {
  const orderId = String(order?.id ?? "").trim();
  const orderNo = String(order?.order_no ?? order?.orderNo ?? orderId).trim();
  const deadline = Date.now() + 10000;
  let lastAnomalies = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/admin/mall/overview?low_stock_threshold=10", {
      token: adminToken,
    });
    const overview = data?.overview ?? {};
    lastAnomalies = Array.isArray(overview.finance_anomalies)
      ? overview.finance_anomalies
      : Array.isArray(overview.financeAnomalies)
        ? overview.financeAnomalies
        : [];
    const anomaly = lastAnomalies.find((item) => {
      const itemOrderId = item?.order_id ?? item?.orderId;
      const itemOrderNo = item?.order_no ?? item?.orderNo;
      return (
        String(itemOrderId) === orderId ||
        (orderNo && String(itemOrderNo) === orderNo)
      );
    });
    if (anomaly) {
      const issueType = String(anomaly.issue_type ?? anomaly.issueType ?? "");
      const difference = Number(
        anomaly.difference_credits ?? anomaly.differenceCredits ?? 0,
      );
      if (issueType !== "PAYMENT_MISMATCH" || difference <= 0) {
        throw new Error(
          `Finance anomaly fixture fields mismatch for order=${orderId}: ${JSON.stringify(anomaly)}`,
        );
      }
      return anomaly;
    }
    await delay(300);
  }
  throw new Error(
    `Timed out waiting for finance anomaly order=${orderId}. Last anomalies: ${JSON.stringify(lastAnomalies, null, 2)}`,
  );
}

async function createFinanceAnomalyPaymentFixture(
  order,
  userId,
  idempotencyKey,
) {
  const orderId = String(order?.id ?? "").trim();
  const normalizedUserId = String(userId ?? "").trim();
  const key = String(idempotencyKey ?? "").trim();
  if (!orderId || !normalizedUserId || !key) {
    throw new Error(
      `Cannot create finance anomaly payment fixture without orderId/userId/idempotencyKey: ${JSON.stringify({ orderId, userId: normalizedUserId, key })}`,
    );
  }
  const stdout = await runMallPsql(`
    SET search_path TO bbs_mall;
    WITH target AS (
      SELECT id, user_id
      FROM mall_orders
      WHERE id = ${pgLiteral(orderId)}::BIGINT
        AND user_id = ${pgLiteral(normalizedUserId)}::BIGINT
        AND status IN ('PAID', 'SHIPPED', 'COMPLETED', 'REFUNDED')
    ),
    inserted_payment AS (
      INSERT INTO mall_payments (
        order_id, user_id, amount_credits, provider, idempotency_key, status,
        provider_trade_no, failure_reason, paid_at, created_at, updated_at
      )
      SELECT
        target.id,
        target.user_id,
        1,
        'credits',
        ${pgLiteral(key)},
        'SUCCEEDED',
        ${pgLiteral(`credit-anomaly-${orderId}`)},
        '',
        NOW(),
        NOW(),
        NOW()
      FROM target
      ON CONFLICT (provider, idempotency_key) DO UPDATE
      SET amount_credits = EXCLUDED.amount_credits,
          status = 'SUCCEEDED',
          provider_trade_no = EXCLUDED.provider_trade_no,
          failure_reason = '',
          paid_at = EXCLUDED.paid_at,
          updated_at = EXCLUDED.updated_at
      RETURNING order_id
    )
    SELECT order_id FROM inserted_payment;
  `);
  if (!stdout.split(/\s+/).includes(orderId)) {
    throw new Error(
      `Failed to create finance anomaly payment fixture for ${orderId}. psql output: ${stdout.slice(0, 500)}`,
    );
  }
}

async function markOrderStalePaying(order, userId, idempotencyKey) {
  const orderId = String(order?.id ?? "").trim();
  const normalizedUserId = String(userId ?? "").trim();
  const key = String(idempotencyKey ?? "").trim();
  if (!orderId || !normalizedUserId || !key) {
    throw new Error(
      `Cannot create stale PAYING order fixture without orderId/userId/idempotencyKey: ${JSON.stringify({ orderId, userId: normalizedUserId, key })}`,
    );
  }
  const stdout = await runMallPsql(`
    SET search_path TO bbs_mall;
    WITH stale AS (
      SELECT NOW() - INTERVAL '5 minutes' AS at
    ),
    updated_order AS (
      UPDATE mall_orders
      SET status = 'PAYING',
          payment_method = 'credits',
          updated_at = (SELECT at FROM stale)
      WHERE id = ${pgLiteral(orderId)}::BIGINT
        AND user_id = ${pgLiteral(normalizedUserId)}::BIGINT
        AND status = 'PENDING_PAYMENT'
      RETURNING id, user_id, total_credits
    ),
    inserted_payment AS (
      INSERT INTO mall_payments (
        order_id, user_id, amount_credits, provider, idempotency_key, status,
        provider_trade_no, failure_reason, paid_at, created_at, updated_at
      )
      SELECT
        updated_order.id,
        updated_order.user_id,
        updated_order.total_credits,
        'credits',
        ${pgLiteral(key)},
        'PENDING',
        '',
        '',
        NULL,
        stale.at,
        stale.at
      FROM updated_order
      CROSS JOIN stale
      ON CONFLICT (provider, idempotency_key) DO UPDATE
      SET status = 'PENDING',
          provider_trade_no = '',
          failure_reason = '',
          paid_at = NULL,
          updated_at = EXCLUDED.updated_at
      RETURNING id
    ),
    inserted_log AS (
      INSERT INTO mall_order_status_logs (
        order_id, from_status, to_status, reason, operator_type, operator_id,
        note, created_at
      )
      SELECT
        updated_order.id,
        'PENDING_PAYMENT',
        'PAYING',
        'paying',
        'user',
        ${pgLiteral(normalizedUserId)},
        'admin e2e stale paying fixture',
        stale.at
      FROM updated_order
      CROSS JOIN stale
      CROSS JOIN inserted_payment
      RETURNING order_id
    )
    SELECT order_id FROM inserted_log;
  `);
  if (!stdout.split(/\s+/).includes(orderId)) {
    throw new Error(
      `Failed to create stale PAYING order fixture for ${orderId}. psql output: ${stdout.slice(0, 500)}`,
    );
  }
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
      const visible = (el) => {
        const rect = el.getBoundingClientRect();
        const style = window.getComputedStyle(el);
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
      };
      const buttons = Array.from(document.querySelectorAll("button"));
      const button =
        buttons.find((item) => visible(item) && !item.disabled && pattern.test((item.innerText || item.textContent || "").trim())) ||
        buttons.find((item) => visible(item) && pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`,
  );
}

async function clickButtonInContainer(page, containerSelector, buttonPattern) {
  await waitFor(
    page,
    scopedButtonEnabledExpression(containerSelector, buttonPattern),
    `${containerSelector} ${buttonPattern} button enabled`,
    10000,
  );
  return evaluate(
    page,
    `(() => {
      const container = document.querySelector(${JSON.stringify(containerSelector)});
      if (!container) throw new Error("Container not found: ${escapeForScript(containerSelector)}");
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const visible = (el) => {
        const rect = el.getBoundingClientRect();
        const style = window.getComputedStyle(el);
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
      };
      const button = Array.from(container.querySelectorAll("button")).find((item) => visible(item) && !item.disabled && pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found in container: ${escapeForScript(buttonPattern)}");
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`,
  );
}

async function clickButtonInBlock(page, blockSelector, blockText, buttonPattern) {
  await waitFor(
    page,
    blockButtonEnabledExpression(blockSelector, blockText, buttonPattern),
    `${blockText} ${buttonPattern} button enabled`,
    10000,
  );
  return evaluate(
    page,
    `(() => {
      const selector = ${JSON.stringify(blockSelector)};
      const blockNeedle = ${JSON.stringify(blockText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const visible = (el) => {
        const rect = el.getBoundingClientRect();
        const style = window.getComputedStyle(el);
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
      };
      const block = Array.from(document.querySelectorAll(selector)).find((item) =>
        visible(item) &&
        (item.innerText || "").includes(blockNeedle) &&
        Array.from(item.querySelectorAll("button")).some((button) => visible(button) && !button.disabled && pattern.test((button.innerText || button.textContent || "").trim())),
      );
      if (!block) throw new Error("Block not found: ${escapeForScript(blockText)}");
      const button = Array.from(block.querySelectorAll("button")).find((item) =>
        visible(item) && !item.disabled && pattern.test((item.innerText || item.textContent || "").trim())
      );
      if (!button) throw new Error("Button not found in block: ${escapeForScript(buttonPattern)}");
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`,
  );
}

function blockButtonEnabledExpression(blockSelector, blockText, buttonPattern) {
  return `(() => {
    const selector = ${JSON.stringify(blockSelector)};
    const blockNeedle = ${JSON.stringify(blockText)};
    const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
    const visible = (el) => {
      const rect = el.getBoundingClientRect();
      const style = window.getComputedStyle(el);
      return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
    };
    return Array.from(document.querySelectorAll(selector)).some((block) =>
      visible(block) &&
      (block.innerText || "").includes(blockNeedle) &&
      Array.from(block.querySelectorAll("button")).some((button) =>
        visible(button) &&
        !button.disabled &&
        pattern.test((button.innerText || button.textContent || "").trim()),
      ),
    );
  })()`;
}

function scopedButtonEnabledExpression(containerSelector, buttonPattern) {
  return `(() => {
    const container = document.querySelector(${JSON.stringify(containerSelector)});
    if (!container) return false;
    const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
    const visible = (el) => {
      const rect = el.getBoundingClientRect();
      const style = window.getComputedStyle(el);
      return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
    };
    return Array.from(container.querySelectorAll("button")).some((button) =>
      visible(button) &&
      !button.disabled &&
      pattern.test((button.innerText || button.textContent || "").trim()),
    );
  })()`;
}

async function clickButtonInRow(page, rowText, buttonPattern) {
  await waitFor(
    page,
    rowButtonEnabledExpression(rowText, buttonPattern),
    `${rowText} ${buttonPattern} button enabled`,
    10000,
  );
  return evaluate(
    page,
    `(() => {
      const rowNeedle = ${JSON.stringify(rowText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const visible = (el) => {
        const rect = el.getBoundingClientRect();
        const style = window.getComputedStyle(el);
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
      };
      const rows = Array.from(document.querySelectorAll("tr, .el-table__row"));
      const candidateRows = rows.filter((item) => (item.innerText || "").includes(rowNeedle));
      const row =
        candidateRows.find((item) =>
          Array.from(item.querySelectorAll("button")).some((button) => visible(button) && !button.disabled && pattern.test((button.innerText || button.textContent || "").trim())),
        ) ||
        candidateRows.find((item) =>
          Array.from(item.querySelectorAll("button")).some((button) => visible(button) && pattern.test((button.innerText || button.textContent || "").trim())),
        );
      if (!row) throw new Error("Row not found: ${escapeForScript(rowText)}");
      let button =
        Array.from(row.querySelectorAll("button")).find((item) => visible(item) && !item.disabled && pattern.test((item.innerText || item.textContent || "").trim())) ||
        Array.from(row.querySelectorAll("button")).find((item) => visible(item) && pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) {
        const scope = row.closest(".el-table") || document;
        button =
          Array.from(scope.querySelectorAll("button")).find((item) => visible(item) && !item.disabled && pattern.test((item.innerText || item.textContent || "").trim())) ||
          Array.from(scope.querySelectorAll("button")).find((item) => visible(item) && pattern.test((item.innerText || item.textContent || "").trim()));
      }
      if (!button) throw new Error("Button not found in row: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled in row: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`,
  );
}

function rowButtonEnabledExpression(rowText, buttonPattern) {
  return `(() => {
    const rowNeedle = ${JSON.stringify(rowText)};
    const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
    const visible = (el) => {
      const rect = el.getBoundingClientRect();
      const style = window.getComputedStyle(el);
      return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
    };
    const rows = Array.from(document.querySelectorAll("tr, .el-table__row"));
    const candidateRows = rows.filter((item) => (item.innerText || "").includes(rowNeedle));
    if (
      candidateRows.some((row) =>
        Array.from(row.querySelectorAll("button")).some((button) =>
          visible(button) &&
          !button.disabled &&
          pattern.test((button.innerText || button.textContent || "").trim()),
        ),
      )
    ) {
      return true;
    }
    return candidateRows.some((row) => {
      const scope = row.closest(".el-table") || document;
      return Array.from(scope.querySelectorAll("button")).some((button) =>
        visible(button) &&
        !button.disabled &&
        pattern.test((button.innerText || button.textContent || "").trim()),
      );
    });
  })()`;
}

async function closeDrawer(page) {
  await evaluate(
    page,
    `(() => {
      const button = document.querySelector(".el-drawer__close-btn");
      if (!button) return false;
      button.click();
      return true;
    })()`,
  );
  await waitFor(
    page,
    `!Array.from(document.querySelectorAll(".el-drawer")).some((item) => {
      const style = window.getComputedStyle(item);
      return style.display !== "none" && style.visibility !== "hidden" && item.getBoundingClientRect().width > 0 && item.getBoundingClientRect().height > 0;
    })`,
    "drawer closed",
    5000,
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

async function fillFormInputByLabel(page, label, value) {
  await waitFor(
    page,
    `(() => {
      const labelText = ${JSON.stringify(label)};
      return Array.from(document.querySelectorAll(".el-dialog .el-form-item"))
        .some((item) => {
          const itemLabel = (item.querySelector(".el-form-item__label")?.innerText || "").trim();
          return itemLabel === labelText && item.querySelector("input");
        });
    })()`,
    `form input ${label}`,
    10000,
  );
  return evaluate(
    page,
    `(() => {
      const labelText = ${JSON.stringify(label)};
      const item = Array.from(document.querySelectorAll(".el-dialog .el-form-item"))
        .find((candidate) => {
          const itemLabel = (candidate.querySelector(".el-form-item__label")?.innerText || "").trim();
          return itemLabel === labelText && candidate.querySelector("input");
        });
      if (!item) throw new Error("Form item not found: ${escapeForScript(label)}");
      const input = item.querySelector("input");
      input.scrollIntoView({ block: "center", inline: "center" });
      input.focus();
      const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value");
      descriptor.set.call(input, ${JSON.stringify(value)});
      input.dispatchEvent(new Event("input", { bubbles: true }));
      input.dispatchEvent(new Event("change", { bubbles: true }));
      input.blur();
      return input.value;
    })()`,
  );
}

async function selectOptionByFormLabel(page, label, value) {
  await waitFor(
    page,
    `(() => {
      const labelText = ${JSON.stringify(label)};
      return Array.from(document.querySelectorAll(".el-dialog .el-form-item"))
        .some((item) => {
          const itemLabel = (item.querySelector(".el-form-item__label")?.innerText || "").trim();
          return itemLabel === labelText && item.querySelector("input");
        });
    })()`,
    `form select ${label}`,
    10000,
  );
  await evaluate(
    page,
    `(() => {
      const labelText = ${JSON.stringify(label)};
      const item = Array.from(document.querySelectorAll(".el-dialog .el-form-item"))
        .find((candidate) => {
          const itemLabel = (candidate.querySelector(".el-form-item__label")?.innerText || "").trim();
          return itemLabel === labelText && candidate.querySelector("input");
        });
      if (!item) throw new Error("Form item not found: ${escapeForScript(label)}");
      const input = item.querySelector("input");
      input.scrollIntoView({ block: "center", inline: "center" });
      input.click();
      input.focus();
      const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value");
      descriptor.set.call(input, ${JSON.stringify(value)});
      input.dispatchEvent(new Event("input", { bubbles: true }));
      input.dispatchEvent(new Event("change", { bubbles: true }));
      return input.value;
    })()`,
  );
  await delay(200);
  await waitFor(
    page,
    `(() => {
      const expected = ${JSON.stringify(value)};
      const visible = (el) => {
        const rect = el.getBoundingClientRect();
        const style = window.getComputedStyle(el);
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
      };
      return Array.from(document.querySelectorAll(".el-select-dropdown__item"))
        .some((item) => {
          const text = (item.innerText || item.textContent || "").trim();
          return visible(item) && (text === expected || text.endsWith("(" + expected + ")"));
        });
    })()`,
    `select option ${value}`,
    5000,
  );
  await evaluate(
    page,
    `(() => {
      const expected = ${JSON.stringify(value)};
      const visible = (el) => {
        const rect = el.getBoundingClientRect();
        const style = window.getComputedStyle(el);
        return rect.width > 0 && rect.height > 0 && style.display !== "none" && style.visibility !== "hidden";
      };
      const option = Array.from(document.querySelectorAll(".el-select-dropdown__item"))
        .find((item) => {
          const text = (item.innerText || item.textContent || "").trim();
          return visible(item) && (text === expected || text.endsWith("(" + expected + ")"));
        });
      if (!option) throw new Error("Select option not found: ${escapeForScript(value)}");
      option.scrollIntoView({ block: "center", inline: "center" });
      option.click();
      return (option.innerText || option.textContent || "").trim();
    })()`,
  );
  await delay(200);
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
