<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { normalizeEntityId } from "@/utils/entityId";
import { downloadCsv, type CsvColumn } from "@/utils/csvExport";
import { loadAllOffsetPages } from "@/utils/offsetPages";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  closeAdminMallExpiredOrders,
  getAdminMallOverview,
  listAdminMallOrderLogs,
  listAdminMallPayments,
  listAdminMallOrderPayments,
  listAdminMallOrders,
  recoverAdminMallStalePayingOrders,
  updateAdminMallOrderStatus,
  type AdminMallOverview,
  type AdminMallOrder,
  type AdminMallOrderItem,
  type AdminMallDigitalEntitlement,
  type AdminMallProduct,
  type AdminMallOrderStatusLog,
  type AdminMallPayment
} from "@/api/admin";

defineOptions({
  name: "MallOrders"
});

type OrderRow = Partial<AdminMallOrder> & Record<string, any>;
type OrderItemRow = Partial<AdminMallOrderItem> & Record<string, any>;
type EntitlementRow = Partial<AdminMallDigitalEntitlement> & Record<string, any>;
type LogRow = Partial<AdminMallOrderStatusLog> & Record<string, any>;
type PaymentRow = Partial<AdminMallPayment> & Record<string, any>;

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const overviewLoading = ref(false);
const expiring = ref(false);
const recoveringPaying = ref(false);
const exporting = ref(false);
const exportingPayments = ref(false);
const statusSaving = ref(false);
const recordsLoading = ref(false);
const orders = ref<AdminMallOrder[]>([]);
const overview = ref<AdminMallOverview | null>(null);
const currentOrder = ref<AdminMallOrder | null>(null);
const logs = ref<AdminMallOrderStatusLog[]>([]);
const payments = ref<AdminMallPayment[]>([]);
const statusDialogVisible = ref(false);
const recordsDrawerVisible = ref(false);
const recordTab = ref("logs");
const statusFormRef = ref<FormInstance>();

function errorMessage(error: unknown) {
  const response = (error as any)?.response?.data;
  return response?.message ?? response?.reason ?? (error as Error)?.message ?? "";
}

const operationSettings = reactive({
  lowStockThreshold: 10,
  closeExpireMinutes: 30,
  closeLimit: 100,
  recoverStaleMinutes: 30,
  recoverLimit: 100
});

const query = reactive({
  keyword: "",
  userId: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const statusForm = reactive({
  id: 0,
  currentStatus: 0,
  orderNo: "",
  status: 3,
  requiresShipping: false,
  shippingCarrier: "",
  trackingNo: "",
  note: ""
});

const canList = computed(() => hasPerms("mall:list_orders"));
const canViewOverview = computed(() => hasPerms("mall:list_orders"));
const canUpdateStatus = computed(() =>
  hasPerms("mall:update_order_status")
);
const canCloseExpired = computed(() => hasPerms("mall:close_expired_orders"));
const canRecoverPaying = computed(() =>
  hasPerms("mall:recover_paying_orders")
);
const canListLogs = computed(() => hasPerms("mall:list_order_logs"));
const canListPayments = computed(() => hasPerms("mall:list_order_payments"));
const canListEntitlements = computed(() =>
  hasPerms("mall:list_digital_entitlements")
);
const canListRefunds = computed(() => hasPerms("mall:list_refunds"));

const currentItems = computed(() => currentOrder.value?.items ?? []);
const currentEntitlements = computed(() => digitalEntitlementsOf(currentOrder.value));
const overviewMetrics = computed(() => [
  {
    label: "累计收入",
    value: overviewNumber("revenue_credits_total", "revenueCreditsTotal"),
    unit: "积分",
    icon: "ri/copper-coin-line"
  },
  {
    label: "今日收入",
    value: overviewNumber("today_revenue_credits", "todayRevenueCredits"),
    unit: "积分",
    icon: "ri/sun-line"
  },
  {
    label: "待发货",
    value: overviewNumber("pending_shipment_total", "pendingShipmentTotal"),
    unit: "单",
    icon: "ri/truck-line"
  },
  {
    label: "待售后",
    value: overviewNumber("pending_refund_total", "pendingRefundTotal"),
    unit: "单",
    icon: "ri/refund-2-line"
  },
  {
    label: "低库存",
    value: overviewNumber("low_stock_total", "lowStockTotal"),
    unit: "个商品",
    icon: "ri/alarm-warning-line"
  }
]);
const overviewOrderStatusCounts = computed(() =>
  overview.value?.order_status_counts ?? overview.value?.orderStatusCounts ?? []
);
const payingOrderTotal = computed(() => orderStatusCount("PAYING"));
const overviewLowStockProducts = computed<AdminMallProduct[]>(() =>
  overview.value?.low_stock_products ?? overview.value?.lowStockProducts ?? []
);
const lowStockThresholdValue = computed(() =>
  positiveInt(operationSettings.lowStockThreshold, 10)
);
const overviewTopSellingProducts = computed<AdminMallProduct[]>(() =>
  overview.value?.top_selling_products ?? overview.value?.topSellingProducts ?? []
);

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "订单号", minWidth: 180, slot: "orderNo" },
  { label: "用户 ID", width: 110, slot: "userId" },
  { label: "商品", minWidth: 240, slot: "items" },
  { label: "实付/优惠", width: 140, slot: "totalCredits" },
  { label: "状态", width: 110, slot: "status" },
  { label: "交付", minWidth: 180, slot: "fulfillment" },
  { prop: "receiver", label: "收货人", minWidth: 120, showOverflowTooltip: true },
  { prop: "phone", label: "手机号", minWidth: 130, showOverflowTooltip: true },
  { label: "支付时间", width: 170, slot: "paidAt" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 220, slot: "operation" }
];

const logColumns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "原状态", width: 120, slot: "fromStatus" },
  { label: "新状态", width: 120, slot: "toStatus" },
  { prop: "operator_id", label: "操作人", minWidth: 120, showOverflowTooltip: true },
  { prop: "note", label: "备注", minWidth: 220, showOverflowTooltip: true },
  { label: "时间", width: 170, slot: "createdAt" }
];

const paymentColumns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "用户 ID", width: 110, slot: "paymentUserId" },
  { label: "金额", width: 110, slot: "amount" },
  { prop: "provider", label: "渠道", minWidth: 110, showOverflowTooltip: true },
  { label: "状态", width: 110, slot: "paymentStatus" },
  {
    label: "渠道流水",
    minWidth: 180,
    showOverflowTooltip: true,
    slot: "providerTradeNo"
  },
  {
    label: "支付幂等键",
    minWidth: 220,
    showOverflowTooltip: true,
    slot: "idempotencyKey"
  },
  {
    label: "失败原因",
    minWidth: 220,
    showOverflowTooltip: true,
    slot: "failureReason"
  },
  { label: "支付时间", width: 170, slot: "paymentPaidAt" },
  { label: "更新时间", width: 170, slot: "paymentUpdatedAt" }
];

const orderExportColumns: CsvColumn<OrderRow>[] = [
  { header: "订单ID", value: row => row.id ?? "" },
  { header: "订单号", value: orderNoOf },
  { header: "用户ID", value: userIdOf },
  { header: "商品", value: itemSummary },
  { header: "状态", value: row => statusMeta(Number(row.status ?? 0)).label },
  { header: "实付积分", value: totalCreditsOf },
  { header: "原价积分", value: originalCreditsOf },
  { header: "优惠积分", value: discountCreditsOf },
  { header: "优惠码", value: couponCodeOf },
  { header: "交付", value: fulfillmentSummary },
  { header: "数字权益", value: digitalEntitlementExportText },
  { header: "收货人", value: row => row.receiver ?? "" },
  { header: "手机号", value: row => row.phone ?? "" },
  { header: "收货地址", value: row => row.address ?? "" },
  { header: "支付时间", value: row => formatTime(paidAt(row)) },
  { header: "发货时间", value: row => formatTime(shippedAt(row)) },
  { header: "完成时间", value: row => formatTime(completedAt(row)) },
  { header: "更新时间", value: row => formatTime(updatedAt(row)) }
];

const paymentExportColumns: CsvColumn<PaymentRow>[] = [
  { header: "支付ID", value: row => row.id ?? "" },
  { header: "订单ID", value: row => row.order_id ?? row.orderId ?? "" },
  { header: "订单号", value: row => row.order_no ?? row.orderNo ?? "" },
  { header: "用户ID", value: paymentUserId },
  { header: "金额积分", value: paymentAmount },
  { header: "渠道", value: row => row.provider ?? "" },
  { header: "状态", value: row => paymentStatusMeta(Number(row.status ?? 0)).label },
  { header: "渠道流水", value: providerTradeNo },
  { header: "支付幂等键", value: paymentIdempotencyKey },
  { header: "失败原因", value: paymentFailureReason },
  { header: "支付时间", value: row => formatTime(paymentPaidAt(row)) },
  { header: "创建时间", value: row => formatTime(paymentCreatedAt(row)) },
  { header: "更新时间", value: row => formatTime(paymentUpdatedAt(row)) }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "待支付", value: 1 },
  { label: "支付中", value: 2 },
  { label: "已支付", value: 3 },
  { label: "已取消", value: 4 },
  { label: "已发货", value: 5 },
  { label: "已完成", value: 6 },
  { label: "已关闭", value: 7 },
  { label: "已退款", value: 8 }
];

function applyRouteQuery() {
  const routeStatus = Number(route.query.status ?? 0);
  const routeKeyword = String(
    route.query.keyword ??
      route.query.order_id ??
      route.query.orderId ??
      route.query.order_no ??
      route.query.orderNo ??
      ""
  ).trim();
  const routeUserId = String(route.query.user_id ?? route.query.userId ?? "").trim();
  query.keyword = routeKeyword;
  query.userId = routeUserId;
  query.status = statusOptions.some(item => item.value === routeStatus)
    ? routeStatus
    : 0;
  query.currentPage = 1;
}

const allowedStatusOptions = computed(() =>
  statusOptions.filter(item => allowedNextStatuses(statusForm.currentStatus).includes(item.value))
);

const showFulfillmentFields = computed(() =>
  statusForm.requiresShipping && [5, 6].includes(Number(statusForm.status))
);

const rules: FormRules = {
  status: [{ required: true, message: "请选择订单状态", trigger: "change" }]
};

function statusMeta(status?: number) {
  switch (status) {
    case 1:
      return { label: "待支付", type: "info" as const };
    case 2:
      return { label: "支付中", type: "warning" as const };
    case 3:
      return { label: "已支付", type: "success" as const };
    case 4:
      return { label: "已取消", type: "danger" as const };
    case 5:
      return { label: "已发货", type: "primary" as const };
    case 6:
      return { label: "已完成", type: "success" as const };
    case 7:
      return { label: "已关闭", type: "info" as const };
    case 8:
      return { label: "已退款", type: "success" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "danger" as const };
  }
}

function allowedNextStatuses(status?: number) {
  switch (Number(status ?? 0)) {
    case 3:
      return [5, 6];
    case 5:
      return [6];
    default:
      return [];
  }
}

function canTransitionOrder(row: OrderRow) {
  return allowedNextStatuses(row.status).length > 0;
}

function orderRequiresShipping(row: OrderRow) {
  return Boolean(shippingCarrierOf(row) || trackingNoOf(row) || row.receiver || row.phone || row.address);
}

function hasCarrierOrTracking(carrier: string, trackingNo: string) {
  return Boolean(carrier.trim() || trackingNo.trim());
}

function hasFulfillmentEvidence(carrier: string, trackingNo: string, note: string) {
  return hasCarrierOrTracking(carrier, trackingNo) || Boolean(note.trim());
}

function paymentStatusMeta(status?: number) {
  switch (status) {
    case 1:
      return { label: "待支付", type: "info" as const };
    case 2:
      return { label: "成功", type: "success" as const };
    case 3:
      return { label: "失败", type: "danger" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "warning" as const };
  }
}

function orderNoOf(row: OrderRow) {
  return row.order_no ?? row.orderNo ?? "-";
}

function userIdOf(row: OrderRow) {
  return row.user_id ?? row.userId ?? "-";
}

function totalCreditsOf(row: OrderRow) {
  return Number(row.total_credits ?? row.totalCredits ?? 0);
}

function originalCreditsOf(row: OrderRow) {
  const original = Number(row.original_credits ?? row.originalCredits ?? 0);
  return original > 0 ? original : totalCreditsOf(row);
}

function discountCreditsOf(row: OrderRow) {
  return Number(row.discount_credits ?? row.discountCredits ?? 0);
}

function couponCodeOf(row: OrderRow) {
  return row.coupon_code ?? row.couponCode ?? "";
}

function paidAt(row: OrderRow) {
  return row.paid_at ?? row.paidAt;
}

function updatedAt(row: OrderRow) {
  return row.updated_at ?? row.updatedAt;
}

function shippingCarrierOf(row: OrderRow) {
  return row.shipping_carrier ?? row.shippingCarrier ?? "";
}

function trackingNoOf(row: OrderRow) {
  return row.tracking_no ?? row.trackingNo ?? "";
}

function shippedAt(row: OrderRow) {
  return row.shipped_at ?? row.shippedAt;
}

function completedAt(row: OrderRow) {
  return row.completed_at ?? row.completedAt;
}

function digitalEntitlementsOf(row?: OrderRow | null): EntitlementRow[] {
  const items = row?.digital_entitlements ?? row?.digitalEntitlements ?? [];
  return Array.isArray(items) ? items : [];
}

function entitlementCode(row: EntitlementRow) {
  return row.fulfillment_code ?? row.fulfillmentCode ?? "";
}

function entitlementGrantType(row: EntitlementRow) {
  return String(row.grant_type ?? row.grantType ?? "").trim().toLowerCase();
}

function entitlementGrantKey(row: EntitlementRow) {
  return String(row.grant_key ?? row.grantKey ?? row.sku ?? "").trim();
}

function entitlementGrantLabel(row: EntitlementRow) {
  const labels: Record<string, string> = {
    badge: "徽章",
    theme: "主题",
    membership: "会员",
    digital: "数字权益"
  };
  return labels[entitlementGrantType(row)] ?? "数字权益";
}

function entitlementIssuedAt(row: EntitlementRow) {
  return row.issued_at ?? row.issuedAt;
}

function entitlementExpiresAt(row: EntitlementRow) {
  return row.expires_at ?? row.expiresAt;
}

function entitlementExpired(row: EntitlementRow) {
  const expiresAt = Number(entitlementExpiresAt(row) ?? 0);
  return expiresAt > 0 && expiresAt <= Date.now();
}

function entitlementExpiryText(row: EntitlementRow) {
  const expiresAt = Number(entitlementExpiresAt(row) ?? 0);
  if (!expiresAt) return "";
  return `${entitlementExpired(row) ? "已过期" : "有效至"} ${formatTime(expiresAt)}`;
}

function entitlementStatus(row: EntitlementRow) {
  return String(row.status ?? row.Status ?? "").trim().toUpperCase();
}

function entitlementRevokedAt(row: EntitlementRow) {
  return row.revoked_at ?? row.revokedAt;
}

function entitlementRefundId(row: EntitlementRow) {
  return row.refund_id ?? row.refundId ?? "";
}

function hasEntitlementRefund(row: EntitlementRow) {
  return normalizeEntityId(entitlementRefundId(row)) !== undefined;
}

function entitlementId(row: EntitlementRow) {
  return normalizeEntityId(row.id ?? row.entitlement_id ?? row.entitlementId);
}

function entitlementRevoked(row: EntitlementRow) {
  return entitlementStatus(row) === "REVOKED" || Boolean(entitlementRevokedAt(row));
}

function entitlementStatusLabel(row: EntitlementRow) {
  if (entitlementRevoked(row)) return "已撤销";
  return entitlementExpired(row) ? "已过期" : "可用";
}

function entitlementStatusTagType(row: EntitlementRow) {
  if (entitlementRevoked(row)) return "danger";
  return entitlementExpired(row) ? "warning" : "success";
}

function entitlementProductId(row: EntitlementRow) {
  return row.product_id ?? row.productId ?? "-";
}

function entitlementRouteStatus(row: EntitlementRow) {
  if (entitlementRevoked(row)) return "REVOKED";
  if (entitlementExpired(row)) return "EXPIRED";
  return "ACTIVE";
}

function openEntitlementLedger(row: EntitlementRow) {
  const queryParams: Record<string, string> = {
    status: entitlementRouteStatus(row)
  };
  const entitlementIdValue = entitlementId(row);
  const code = entitlementCode(row);
  const orderId = normalizeEntityId(currentOrder.value?.id);
  if (entitlementIdValue !== undefined) {
    queryParams.entitlement_id = String(entitlementIdValue);
  } else if (code) {
    queryParams.fulfillment_code = String(code);
  } else if (orderId !== undefined) {
    queryParams.order_id = String(orderId);
  }
  const grantType = entitlementGrantType(row);
  const grantKey = entitlementGrantKey(row);
  if (grantType) queryParams.grant_type = grantType;
  if (grantKey) queryParams.grant_key = grantKey;
  recordsDrawerVisible.value = false;
  router.push({ path: "/mall/entitlements", query: queryParams });
}

function openEntitlementRefund(row: EntitlementRow) {
  const refundId = normalizeEntityId(entitlementRefundId(row));
  if (refundId === undefined) {
    message("该权益暂无关联售后单", { type: "warning" });
    return;
  }
  recordsDrawerVisible.value = false;
  router.push({
    path: "/mall/refunds",
    query: { refund_id: String(refundId) }
  });
}

function entitlementSummary(row: EntitlementRow) {
  const title = row.title || row.sku || `商品 ${entitlementProductId(row)}`;
  const code = entitlementCode(row);
  const quantity = Number(row.quantity ?? 0);
  const revokedAt = entitlementRevokedAt(row);
  const refundId = entitlementRefundId(row);
  const grantKey = entitlementGrantKey(row);
  const expiry = entitlementExpiryText(row);
  return `${title}${quantity > 0 ? ` x${quantity}` : ""}${code ? ` / ${code}` : ""} / ${entitlementGrantLabel(row)}${grantKey ? `:${grantKey}` : ""} / ${entitlementStatusLabel(row)}${expiry ? ` / ${expiry}` : ""}${revokedAt ? ` / 撤销 ${formatTime(Number(revokedAt))}` : ""}${refundId ? ` / 退款 ${refundId}` : ""}`;
}

function digitalEntitlementExportText(row: OrderRow) {
  const entitlements = digitalEntitlementsOf(row);
  if (entitlements.length === 0) return "";
  return entitlements.map(entitlementSummary).join("；");
}

function digitalEntitlementStatusSummary(row: OrderRow) {
  const entitlements = digitalEntitlementsOf(row);
  if (entitlements.length === 0) return "-";
  const revokedCount = entitlements.filter(entitlementRevoked).length;
  const expiredCount = entitlements.filter(
    item => !entitlementRevoked(item) && entitlementExpired(item)
  ).length;
  const activeCount = entitlements.length - revokedCount - expiredCount;
  if (revokedCount === entitlements.length) {
    return `已撤销 ${entitlements.length} 项`;
  }
  if (expiredCount === entitlements.length) {
    return `已过期 ${entitlements.length} 项`;
  }
  return [
    activeCount > 0 ? `可用 ${activeCount} 项` : "",
    expiredCount > 0 ? `已过期 ${expiredCount} 项` : "",
    revokedCount > 0 ? `已撤销 ${revokedCount} 项` : ""
  ]
    .filter(Boolean)
    .join("，");
}

function itemProductId(row: OrderItemRow) {
  return row.product_id ?? row.productId ?? "-";
}

function itemUnitPrice(row: OrderItemRow) {
  return Number(row.unit_price_credits ?? row.unitPriceCredits ?? 0);
}

function itemSubtotal(row: OrderItemRow) {
  return Number(row.subtotal_credits ?? row.subtotalCredits ?? 0);
}

function itemGrantType(row: OrderItemRow) {
  return String(row.grant_type ?? row.grantType ?? "").trim().toLowerCase();
}

function itemGrantKey(row: OrderItemRow) {
  return String(row.grant_key ?? row.grantKey ?? "").trim();
}

function itemGrantLabel(row: OrderItemRow) {
  const labels: Record<string, string> = {
    badge: "徽章",
    theme: "主题",
    membership: "会员",
    digital: "数字权益"
  };
  const grantType = itemGrantType(row);
  return labels[grantType] ?? (grantType || "数字权益");
}

function itemGrantText(row: OrderItemRow) {
  const grantType = itemGrantType(row);
  const grantKey = itemGrantKey(row);
  if (!grantType && !grantKey) return "";
  return `${itemGrantLabel(row)}${grantKey ? ` / ${grantKey}` : ""}`;
}

function logFromStatus(row: LogRow) {
  return Number(row.from_status ?? row.fromStatus ?? 0);
}

function logToStatus(row: LogRow) {
  return Number(row.to_status ?? row.toStatus ?? 0);
}

function logCreatedAt(row: LogRow) {
  return row.created_at ?? row.createdAt;
}

function paymentUserId(row: PaymentRow) {
  return row.user_id ?? row.userId ?? "-";
}

function paymentAmount(row: PaymentRow) {
  return Number(row.amount_credits ?? row.amountCredits ?? 0);
}

function paymentPaidAt(row: PaymentRow) {
  return row.paid_at ?? row.paidAt;
}

function paymentCreatedAt(row: PaymentRow) {
  return row.created_at ?? row.createdAt;
}

function paymentUpdatedAt(row: PaymentRow) {
  return row.updated_at ?? row.updatedAt;
}

function providerTradeNo(row: PaymentRow) {
  return row.provider_trade_no ?? row.providerTradeNo ?? "-";
}

function paymentIdempotencyKey(row: PaymentRow) {
  return row.idempotency_key ?? row.idempotencyKey ?? "-";
}

function paymentFailureReason(row: PaymentRow) {
  return row.failure_reason ?? row.failureReason ?? "";
}

function overviewNumber(snakeKey: string, camelKey: string) {
  const data = (overview.value ?? {}) as Record<string, unknown>;
  return Number(data[snakeKey] ?? data[camelKey] ?? 0);
}

function positiveInt(value: unknown, fallback: number) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0
    ? Math.floor(number)
    : fallback;
}

function minutesToSeconds(value: unknown, fallback: number) {
  return positiveInt(value, fallback) * 60;
}

function orderStatusCount(status: string) {
  const item = overviewOrderStatusCounts.value.find(
    entry => String(entry.status || "").toUpperCase() === status
  );
  return Number(item?.count ?? 0);
}

function productPrice(row: Partial<AdminMallProduct> & Record<string, any>) {
  return Number(row.price_credits ?? row.priceCredits ?? 0);
}

function productSales(row: Partial<AdminMallProduct> & Record<string, any>) {
  return Number(row.sales_count ?? row.salesCount ?? 0);
}

function statusLabel(status: string) {
  switch (String(status || "").toUpperCase()) {
    case "PENDING_PAYMENT":
      return "待支付";
    case "PAYING":
      return "支付中";
    case "PAID":
      return "已支付";
    case "CANCELED":
      return "已取消";
    case "SHIPPED":
      return "已发货";
    case "COMPLETED":
      return "已完成";
    case "CLOSED":
      return "已关闭";
    case "REFUNDED":
      return "已退款";
    default:
      return status || "-";
  }
}

function orderStatusValue(status: string) {
  switch (String(status || "").toUpperCase()) {
    case "PENDING_PAYMENT":
      return 1;
    case "PAYING":
      return 2;
    case "PAID":
      return 3;
    case "CANCELED":
      return 4;
    case "SHIPPED":
      return 5;
    case "COMPLETED":
      return 6;
    case "CLOSED":
      return 7;
    case "REFUNDED":
      return 8;
    default:
      return 0;
  }
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function itemSummary(row: OrderRow) {
  const items = row.items ?? [];
  if (items.length === 0) return "无商品";
  const first = items[0] as OrderItemRow;
  const title = first.title || first.sku || `商品 ${itemProductId(first)}`;
  const suffix = items.length > 1 ? ` 等 ${items.length} 件` : "";
  return `${title} x${first.quantity ?? 0}${suffix}`;
}

function fulfillmentSummary(row: OrderRow) {
  const entitlements = digitalEntitlementsOf(row);
  if (entitlements.length > 0) {
    return `数字权益 ${digitalEntitlementStatusSummary(row)}`;
  }
  const carrier = shippingCarrierOf(row);
  const trackingNo = trackingNoOf(row);
  if (carrier || trackingNo) {
    return [carrier, trackingNo].filter(Boolean).join(" / ");
  }
  if (shippedAt(row)) return "已发货";
  if (completedAt(row)) return "已完成";
  return "-";
}

function currentOrderListParams(
  limit = query.pageSize,
  offset = (query.currentPage - 1) * query.pageSize
) {
  const userID = query.userId.trim();
  return {
    user_id: userID || undefined,
    keyword: query.keyword.trim(),
    status: query.status,
    limit,
    offset
  };
}

async function loadOrders() {
  if (!canList.value) {
    orders.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallOrders(currentOrderListParams());
    if (code !== 0) {
      message(msg || "加载订单列表失败", { type: "error" });
      return;
    }
    orders.value = data.items ?? [];
    query.total = data.total ?? orders.value.length;
  } finally {
    loading.value = false;
  }
}

async function exportOrders() {
  if (!canList.value) {
    message("没有导出订单权限", { type: "warning" });
    return;
  }
  exporting.value = true;
  try {
    const { code, items, message: msg } = await loadAllOffsetPages(
      ({ limit, offset }) =>
        listAdminMallOrders(currentOrderListParams(limit, offset))
    );
    if (code !== 0) {
      message(msg || "导出订单失败", { type: "error" });
      return;
    }
    if (items.length === 0) {
      message("当前筛选条件下没有可导出的订单", { type: "warning" });
      return;
    }
    downloadCsv(
      `mall-orders-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      orderExportColumns,
      items
    );
    message(`已导出 ${items.length} 条订单`, { type: "success" });
  } finally {
    exporting.value = false;
  }
}

async function exportPayments() {
  if (!canList.value) {
    message("没有查询订单权限，无法导出支付记录", { type: "warning" });
    return;
  }
  if (!canListPayments.value) {
    message("没有导出支付记录权限", { type: "warning" });
    return;
  }
  exportingPayments.value = true;
  try {
    const { code, items, message: msg } = await loadAllOffsetPages(
      ({ limit, offset }) =>
        listAdminMallPayments(currentOrderListParams(limit, offset))
    );
    if (code !== 0) {
      message(msg || "导出支付记录失败", { type: "error" });
      return;
    }
    if (items.length === 0) {
      message("当前筛选条件下没有可导出的支付记录", { type: "warning" });
      return;
    }
    downloadCsv(
      `mall-payments-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      paymentExportColumns,
      items
    );
    message(`已导出 ${items.length} 条支付记录`, { type: "success" });
  } catch (error: any) {
    message(error?.message || "导出支付记录失败", { type: "error" });
  } finally {
    exportingPayments.value = false;
  }
}

async function loadOverview() {
  if (!canViewOverview.value) {
    overview.value = null;
    return;
  }
  overviewLoading.value = true;
  try {
    const { code, data, message: msg } = await getAdminMallOverview({
      low_stock_threshold: lowStockThresholdValue.value
    });
    if (code !== 0) {
      message(msg || "加载商城概览失败", { type: "error" });
      return;
    }
    overview.value = data.overview ?? null;
  } finally {
    overviewLoading.value = false;
  }
}

function resetQuery() {
  query.keyword = "";
  query.userId = "";
  query.status = 0;
  query.currentPage = 1;
  loadOrders();
}

function filterOrdersByStatus(status: number) {
  if (!status) return;
  query.status = status;
  query.currentPage = 1;
  loadOrders();
}

function openStatusDialog(row: OrderRow) {
  if (!canUpdateStatus.value) {
    message("没有修改订单状态权限", { type: "warning" });
    return;
  }
  const nextStatuses = allowedNextStatuses(row.status);
  if (nextStatuses.length === 0) {
    message("当前订单状态不能由运营端继续流转", { type: "warning" });
    return;
  }
  statusForm.id = Number(row.id ?? 0);
  statusForm.currentStatus = Number(row.status ?? 0);
  statusForm.orderNo = orderNoOf(row);
  statusForm.status = nextStatuses[0];
  statusForm.requiresShipping = orderRequiresShipping(row);
  statusForm.shippingCarrier = shippingCarrierOf(row);
  statusForm.trackingNo = trackingNoOf(row);
  statusForm.note = "";
  statusFormRef.value?.clearValidate();
  statusDialogVisible.value = true;
}

async function saveOrderStatus() {
  if (!canUpdateStatus.value) {
    message("没有修改订单状态权限", { type: "warning" });
    return;
  }
  const valid = await statusFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  const id = normalizeEntityId(statusForm.id);
  if (!allowedNextStatuses(statusForm.currentStatus).includes(Number(statusForm.status))) {
    message("请选择合法的目标状态", { type: "warning" });
    return;
  }
  if (statusForm.requiresShipping && Number(statusForm.status) === 5) {
    if (!hasCarrierOrTracking(statusForm.shippingCarrier, statusForm.trackingNo)) {
      message("实体订单发货时请填写物流公司或物流单号", { type: "warning" });
      return;
    }
  }
  if (statusForm.requiresShipping && Number(statusForm.status) === 6 && Number(statusForm.currentStatus) === 3) {
    if (!hasFulfillmentEvidence(statusForm.shippingCarrier, statusForm.trackingNo, statusForm.note)) {
      message("实体订单直接完成时请填写物流信息或履约备注", { type: "warning" });
      return;
    }
  }
  statusSaving.value = true;
  try {
    const { code, message: msg } = await updateAdminMallOrderStatus(id, {
      status: Number(statusForm.status),
      shipping_carrier: statusForm.shippingCarrier.trim(),
      tracking_no: statusForm.trackingNo.trim(),
      note: statusForm.note.trim()
    });
    if (code !== 0) {
      message(msg || "更新订单状态失败", { type: "error" });
      return;
    }
    message("订单状态已更新", { type: "success" });
    statusDialogVisible.value = false;
    await loadOrders();
    await loadOverview();
  } catch (error) {
    message(errorMessage(error) || "更新订单状态失败", { type: "error" });
  } finally {
    statusSaving.value = false;
  }
}

async function handleCloseExpiredOrders() {
  if (!canCloseExpired.value) {
    message("没有关闭超时订单权限", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `系统会关闭超过 ${positiveInt(operationSettings.closeExpireMinutes, 30)} 分钟仍未支付的订单，并释放对应库存。本次最多处理 ${positiveInt(operationSettings.closeLimit, 100)} 单。`,
    "关闭超时订单",
    {
      type: "warning",
      confirmButtonText: "执行关闭",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  expiring.value = true;
  try {
    const { code, data, message: msg } = await closeAdminMallExpiredOrders({
      expire_after_seconds: minutesToSeconds(operationSettings.closeExpireMinutes, 30),
      limit: positiveInt(operationSettings.closeLimit, 100)
    });
    if (code !== 0) {
      message(msg || "关闭超时订单失败", { type: "error" });
      return;
    }
    const total = data?.total ?? data?.items?.length ?? 0;
    message(total > 0 ? `已关闭 ${total} 个超时订单` : "没有需要关闭的超时订单", {
      type: "success"
    });
    await loadOrders();
    await loadOverview();
  } catch (error) {
    message(errorMessage(error) || "关闭超时订单失败", { type: "error" });
  } finally {
    expiring.value = false;
  }
}

async function handleRecoverStalePayingOrders() {
  if (!canRecoverPaying.value) {
    message("没有补偿支付中订单权限", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `系统会重试超过 ${positiveInt(operationSettings.recoverStaleMinutes, 30)} 分钟仍处于支付中的订单，并使用原支付幂等键完成积分支付落库。本次最多处理 ${positiveInt(operationSettings.recoverLimit, 100)} 单。失败订单会保留支付中状态，等待下次补偿或人工处理。`,
    "补偿支付中订单",
    {
      type: "warning",
      confirmButtonText: "执行补偿",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  recoveringPaying.value = true;
  try {
    const { code, data, message: msg } =
      await recoverAdminMallStalePayingOrders({
        stale_after_seconds: minutesToSeconds(operationSettings.recoverStaleMinutes, 30),
        limit: positiveInt(operationSettings.recoverLimit, 100)
      });
    if (code !== 0) {
      message(msg || "补偿支付中订单失败", { type: "error" });
      return;
    }
    const recovered = Number(data?.recovered ?? 0);
    const failed = Number(data?.failed ?? 0);
    message(
      recovered > 0 || failed > 0
        ? `补偿完成：成功 ${recovered} 单，失败 ${failed} 单`
        : "没有需要补偿的支付中订单",
      { type: failed > 0 ? "warning" : "success" }
    );
    await loadOrders();
    await loadOverview();
  } catch (error) {
    message(errorMessage(error) || "补偿支付中订单失败", { type: "error" });
  } finally {
    recoveringPaying.value = false;
  }
}

async function openRecords(row: OrderRow, tab: "logs" | "payments") {
  currentOrder.value = row as AdminMallOrder;
  recordTab.value = tab;
  recordsDrawerVisible.value = true;
  await loadRecords(row);
}

async function loadRecords(row: OrderRow) {
  const id = normalizeEntityId(row.id);
  recordsLoading.value = true;
  try {
    const tasks: Promise<void>[] = [];
    if (canListLogs.value) {
      tasks.push(
        listAdminMallOrderLogs(id).then(({ code, data, message: msg }) => {
          if (code !== 0) {
            message(msg || "加载订单日志失败", { type: "error" });
            return;
          }
          logs.value = data.items ?? [];
        })
      );
    } else {
      logs.value = [];
    }
    if (canListPayments.value) {
      tasks.push(
        listAdminMallOrderPayments(id).then(({ code, data, message: msg }) => {
          if (code !== 0) {
            message(msg || "加载支付记录失败", { type: "error" });
            return;
          }
          payments.value = data.items ?? [];
        })
      );
    } else {
      payments.value = [];
    }
    await Promise.all(tasks);
  } finally {
    recordsLoading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadOrders();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadOrders();
}

watch(
  () => [
    route.query.status,
    route.query.keyword,
    route.query.order_id,
    route.query.orderId,
    route.query.order_no,
    route.query.orderNo,
    route.query.user_id,
    route.query.userId
  ],
  () => {
    applyRouteQuery();
    loadOrders();
  }
);

onMounted(() => {
  applyRouteQuery();
  loadOverview();
  loadOrders();
});
</script>

<template>
  <div class="mall-orders-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>订单管理</h2>
          <p>跟踪积分商城订单、支付结果和履约状态</p>
        </div>
        <div class="panel-actions">
          <el-button
            type="success"
            plain
            :icon="useRenderIcon('ri/download-2-line')"
            :disabled="!canList"
            :loading="exporting"
            @click="exportOrders"
          >
            导出订单
          </el-button>
          <el-button
            type="success"
            plain
            :icon="useRenderIcon('ri/bank-card-line')"
            :disabled="!canList || !canListPayments"
            :loading="exportingPayments"
            @click="exportPayments"
          >
            导出支付
          </el-button>
          <el-button
            type="primary"
            plain
            :icon="useRenderIcon('ri/refresh-line')"
            :disabled="!canRecoverPaying"
            :loading="recoveringPaying"
            @click="handleRecoverStalePayingOrders"
          >
            补偿支付中
          </el-button>
          <el-button
            type="warning"
            plain
            :icon="useRenderIcon('ri/time-line')"
            :disabled="!canCloseExpired"
            :loading="expiring"
            @click="handleCloseExpiredOrders"
          >
            关闭超时订单
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 mall:list_orders 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form
        v-if="canViewOverview || canCloseExpired || canRecoverPaying"
        :inline="true"
        class="maintenance-form"
      >
        <el-form-item v-if="canViewOverview" label="低库存阈值">
          <el-input-number
            v-model="operationSettings.lowStockThreshold"
            :min="1"
            :max="9999"
            :step="1"
            :precision="0"
            size="small"
            controls-position="right"
            @change="loadOverview"
          />
        </el-form-item>
        <el-form-item v-if="canCloseExpired" label="关闭超时(分钟)">
          <el-input-number
            v-model="operationSettings.closeExpireMinutes"
            :min="1"
            :max="1440"
            :step="5"
            :precision="0"
            size="small"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item v-if="canCloseExpired" label="关闭批量">
          <el-input-number
            v-model="operationSettings.closeLimit"
            :min="1"
            :max="1000"
            :step="10"
            :precision="0"
            size="small"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item v-if="canRecoverPaying" label="补偿超时(分钟)">
          <el-input-number
            v-model="operationSettings.recoverStaleMinutes"
            :min="1"
            :max="1440"
            :step="5"
            :precision="0"
            size="small"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item v-if="canRecoverPaying" label="补偿批量">
          <el-input-number
            v-model="operationSettings.recoverLimit"
            :min="1"
            :max="1000"
            :step="10"
            :precision="0"
            size="small"
            controls-position="right"
          />
        </el-form-item>
      </el-form>

      <div v-if="canViewOverview" v-loading="overviewLoading" class="overview-area">
        <div class="overview-grid">
          <article v-for="item in overviewMetrics" :key="item.label">
            <span class="overview-icon">
              <component :is="useRenderIcon(item.icon)" />
            </span>
            <div>
              <strong>{{ item.value }}</strong>
              <p>{{ item.label }} · {{ item.unit }}</p>
            </div>
          </article>
        </div>
        <div class="overview-detail-grid">
          <section>
            <header>
              <h3>订单状态</h3>
              <el-button link type="primary" @click="loadOverview">刷新</el-button>
            </header>
            <div class="status-chip-row">
              <el-tag
                v-for="item in overviewOrderStatusCounts"
                :key="item.status"
                effect="plain"
                class="status-chip"
                :type="statusMeta(orderStatusValue(item.status)).type"
                @click="filterOrdersByStatus(orderStatusValue(item.status))"
              >
                {{ statusLabel(item.status) }} {{ item.count }}
              </el-tag>
              <el-text v-if="overviewOrderStatusCounts.length === 0" type="info">
                暂无订单状态
              </el-text>
            </div>
            <el-alert
              v-if="payingOrderTotal > 0"
              type="warning"
              show-icon
              :closable="false"
              class="paying-alert"
            >
              <template #title>
                当前有 {{ payingOrderTotal }} 单处于支付中，可点击状态筛选后执行补偿。
              </template>
            </el-alert>
          </section>
          <section>
            <header>
              <h3>低库存预警</h3>
              <span>阈值 {{ lowStockThresholdValue }}</span>
            </header>
            <div class="compact-product-list">
              <div v-for="item in overviewLowStockProducts" :key="item.id">
                <strong>{{ item.title }}</strong>
                <span>{{ item.stock }} 库存 · {{ productPrice(item) }} 积分</span>
              </div>
              <el-text v-if="overviewLowStockProducts.length === 0" type="success">
                库存健康
              </el-text>
            </div>
          </section>
          <section>
            <header>
              <h3>热销商品</h3>
              <span>Top 5</span>
            </header>
            <div class="compact-product-list">
              <div v-for="item in overviewTopSellingProducts" :key="item.id">
                <strong>{{ item.title }}</strong>
                <span>{{ productSales(item) }} 销量 · {{ item.stock }} 库存</span>
              </div>
              <el-text v-if="overviewTopSellingProducts.length === 0" type="info">
                暂无销量
              </el-text>
            </div>
          </section>
        </div>
      </div>

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            class="w-52!"
            clearable
            placeholder="订单号 / 商品"
            @keyup.enter="loadOrders"
          />
        </el-form-item>
        <el-form-item label="用户 ID">
          <el-input
            v-model="query.userId"
            class="w-36!"
            clearable
            placeholder="用户 ID"
            @keyup.enter="loadOrders"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-32!"
            @change="loadOrders"
          >
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canList"
            :loading="loading"
            @click="loadOrders"
          >
            查询
          </el-button>
          <el-button :icon="useRenderIcon('ep/refresh')" @click="resetQuery">
            重置
          </el-button>
        </el-form-item>
      </el-form>

      <pure-table
        row-key="id"
        align-whole="center"
        table-layout="auto"
        :loading="loading"
        :data="orders"
        :columns="columns"
        :pagination="{
          total: query.total,
          pageSize: query.pageSize,
          currentPage: query.currentPage,
          background: true
        }"
        :header-cell-style="{
          background: 'var(--el-fill-color-light)',
          color: 'var(--el-text-color-primary)'
        }"
        @page-size-change="onPageSizeChange"
        @page-current-change="onCurrentPageChange"
      >
        <template #orderNo="{ row }">
          <span class="order-no">{{ orderNoOf(row) }}</span>
        </template>
        <template #userId="{ row }">
          <el-tag effect="plain">{{ userIdOf(row) }}</el-tag>
        </template>
        <template #items="{ row }">
          <span>{{ itemSummary(row) }}</span>
        </template>
        <template #totalCredits="{ row }">
          <div class="order-amount-cell">
            <span class="credit-price">{{ totalCreditsOf(row) }}</span>
            <small v-if="discountCreditsOf(row) > 0">
              原 {{ originalCreditsOf(row) }} / 优惠 {{ discountCreditsOf(row) }}
            </small>
            <el-tag
              v-if="couponCodeOf(row)"
              effect="plain"
              type="warning"
              size="small"
            >
              {{ couponCodeOf(row) }}
            </el-tag>
          </div>
        </template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #fulfillment="{ row }">
          <span>{{ fulfillmentSummary(row) }}</span>
        </template>
        <template #paidAt="{ row }">
          {{ formatTime(paidAt(row)) }}
        </template>
        <template #updatedAt="{ row }">
          {{ formatTime(updatedAt(row)) }}
        </template>
        <template #operation="{ row }">
          <el-button
            link
            type="primary"
            :disabled="!canUpdateStatus || !canTransitionOrder(row)"
            :icon="useRenderIcon('ri/exchange-line')"
            @click="openStatusDialog(row)"
          >
            履约
          </el-button>
          <el-button
            link
            type="primary"
            :disabled="!canListLogs"
            :icon="useRenderIcon('ri/file-list-3-line')"
            @click="openRecords(row, 'logs')"
          >
            日志
          </el-button>
          <el-button
            link
            type="primary"
            :disabled="!canListPayments"
            :icon="useRenderIcon('ri/bank-card-line')"
            @click="openRecords(row, 'payments')"
          >
            支付
          </el-button>
        </template>
      </pure-table>
    </section>

    <el-dialog
      v-model="statusDialogVisible"
      title="订单履约处理"
      width="520px"
      destroy-on-close
    >
      <el-alert
        :title="`订单 ${statusForm.orderNo}：${statusMeta(statusForm.currentStatus).label}`"
        type="info"
        show-icon
        :closable="false"
        class="permission-alert"
      />
      <el-form
        ref="statusFormRef"
        :model="statusForm"
        :rules="rules"
        label-width="92px"
      >
        <el-form-item label="目标状态" prop="status">
          <el-select v-model="statusForm.status" class="w-full!">
            <el-option
              v-for="item in allowedStatusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <template v-if="showFulfillmentFields">
          <el-form-item label="物流公司">
            <el-input
              v-model="statusForm.shippingCarrier"
              maxlength="80"
              placeholder="顺丰 / 圆通 / 电子权益等"
              show-word-limit
            />
          </el-form-item>
          <el-form-item label="物流单号">
            <el-input
              v-model="statusForm.trackingNo"
              maxlength="120"
              placeholder="物流单号、兑换码或履约凭证"
              show-word-limit
            />
          </el-form-item>
        </template>
        <el-form-item label="备注">
          <el-input
            v-model="statusForm.note"
            type="textarea"
            :rows="4"
            maxlength="300"
            show-word-limit
            placeholder="填写物流公司、单号、交付说明或人工履约备注"
          />
        </el-form-item>
        <el-alert
          v-if="statusForm.requiresShipping && Number(statusForm.status) === 6 && Number(statusForm.currentStatus) === 3"
          title="实体订单直接完成时请填写履约备注或物流信息"
          type="warning"
          show-icon
          :closable="false"
        />
      </el-form>
      <template #footer>
        <el-button @click="statusDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="statusSaving"
          @click="saveOrderStatus"
        >
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="recordsDrawerVisible"
      title="订单记录"
      size="720px"
      destroy-on-close
    >
      <template v-if="currentOrder">
        <el-descriptions :column="2" border class="order-summary">
          <el-descriptions-item label="订单号">
            {{ orderNoOf(currentOrder) }}
          </el-descriptions-item>
          <el-descriptions-item label="用户 ID">
            {{ userIdOf(currentOrder) }}
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusMeta(currentOrder.status).type">
              {{ statusMeta(currentOrder.status).label }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="实付积分">
            {{ totalCreditsOf(currentOrder) }}
          </el-descriptions-item>
          <el-descriptions-item label="商品原价">
            {{ originalCreditsOf(currentOrder) }}
          </el-descriptions-item>
          <el-descriptions-item label="优惠金额">
            {{ discountCreditsOf(currentOrder) }}
          </el-descriptions-item>
          <el-descriptions-item label="优惠券码">
            {{ couponCodeOf(currentOrder) || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="收货人">
            {{ currentOrder.receiver || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="手机号">
            {{ currentOrder.phone || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="地址" :span="2">
            {{ currentOrder.address || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="物流公司">
            {{ shippingCarrierOf(currentOrder) || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="物流单号">
            {{ trackingNoOf(currentOrder) || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="数字权益" :span="2">
            {{ digitalEntitlementStatusSummary(currentOrder) }}
          </el-descriptions-item>
          <el-descriptions-item label="发货时间">
            {{ formatTime(shippedAt(currentOrder)) }}
          </el-descriptions-item>
          <el-descriptions-item label="完成时间">
            {{ formatTime(completedAt(currentOrder)) }}
          </el-descriptions-item>
        </el-descriptions>

        <section class="record-section">
          <h3>商品明细</h3>
          <el-table :data="currentItems" border>
            <el-table-column label="商品 ID" width="100">
              <template #default="{ row }">{{ itemProductId(row) }}</template>
            </el-table-column>
            <el-table-column prop="sku" label="SKU" min-width="120" />
            <el-table-column prop="title" label="商品名称" min-width="180" />
            <el-table-column label="授权" min-width="160">
              <template #default="{ row }">
                <el-tag v-if="itemGrantText(row)" effect="plain" type="success">
                  {{ itemGrantText(row) }}
                </el-tag>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column prop="quantity" label="数量" width="80" />
            <el-table-column label="单价" width="100">
              <template #default="{ row }">{{ itemUnitPrice(row) }}</template>
            </el-table-column>
            <el-table-column label="小计" width="100">
              <template #default="{ row }">{{ itemSubtotal(row) }}</template>
            </el-table-column>
          </el-table>
        </section>

        <section v-if="currentEntitlements.length > 0" class="record-section">
          <h3>数字权益</h3>
          <el-table :data="currentEntitlements" border>
            <el-table-column label="商品 ID" width="100">
              <template #default="{ row }">{{ entitlementProductId(row) }}</template>
            </el-table-column>
            <el-table-column prop="sku" label="SKU" min-width="120" />
            <el-table-column prop="title" label="权益名称" min-width="180" />
            <el-table-column prop="quantity" label="数量" width="80" />
            <el-table-column label="交付码" min-width="220">
              <template #default="{ row }">
                <span class="order-no">{{ entitlementCode(row) || "-" }}</span>
              </template>
            </el-table-column>
            <el-table-column label="授权" min-width="170">
              <template #default="{ row }">
                {{ entitlementGrantLabel(row) }}{{ entitlementGrantKey(row) ? ` / ${entitlementGrantKey(row)}` : "" }}
              </template>
            </el-table-column>
            <el-table-column label="发放时间" width="170">
              <template #default="{ row }">
                {{ formatTime(entitlementIssuedAt(row)) }}
              </template>
            </el-table-column>
            <el-table-column label="有效期" width="190">
              <template #default="{ row }">
                {{ entitlementExpiryText(row) || "长期有效" }}
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="entitlementStatusTagType(row)">
                  {{ entitlementStatusLabel(row) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="撤销时间" width="170">
              <template #default="{ row }">
                {{ formatTime(entitlementRevokedAt(row)) }}
              </template>
            </el-table-column>
            <el-table-column label="退款 ID" width="110">
              <template #default="{ row }">
                {{ entitlementRefundId(row) || "-" }}
              </template>
            </el-table-column>
            <el-table-column
              v-if="canListEntitlements || canListRefunds"
              label="操作"
              width="120"
              fixed="right"
            >
              <template #default="{ row }">
                <el-button
                  v-if="canListEntitlements"
                  link
                  type="primary"
                  size="small"
                  @click="openEntitlementLedger(row)"
                >
                  台账
                </el-button>
                <el-button
                  v-if="canListRefunds"
                  link
                  type="primary"
                  size="small"
                  :disabled="!hasEntitlementRefund(row)"
                  :data-refund-id="entitlementRefundId(row)"
                  @click="openEntitlementRefund(row)"
                >
                  售后
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </section>

        <el-tabs v-model="recordTab" class="record-tabs">
          <el-tab-pane label="状态日志" name="logs">
            <el-alert
              v-if="!canListLogs"
              title="当前账号没有 mall:list_order_logs 权限"
              type="warning"
              show-icon
              :closable="false"
              class="permission-alert"
            />
            <pure-table
              v-else
              row-key="id"
              align-whole="center"
              table-layout="auto"
              :loading="recordsLoading"
              :data="logs"
              :columns="logColumns"
              :header-cell-style="{
                background: 'var(--el-fill-color-light)',
                color: 'var(--el-text-color-primary)'
              }"
            >
              <template #fromStatus="{ row }">
                <el-tag :type="statusMeta(logFromStatus(row)).type">
                  {{ statusMeta(logFromStatus(row)).label }}
                </el-tag>
              </template>
              <template #toStatus="{ row }">
                <el-tag :type="statusMeta(logToStatus(row)).type">
                  {{ statusMeta(logToStatus(row)).label }}
                </el-tag>
              </template>
              <template #createdAt="{ row }">
                {{ formatTime(logCreatedAt(row)) }}
              </template>
            </pure-table>
          </el-tab-pane>
          <el-tab-pane label="支付记录" name="payments">
            <el-alert
              v-if="!canListPayments"
              title="当前账号没有 mall:list_order_payments 权限"
              type="warning"
              show-icon
              :closable="false"
              class="permission-alert"
            />
            <pure-table
              v-else
              row-key="id"
              align-whole="center"
              table-layout="auto"
              :loading="recordsLoading"
              :data="payments"
              :columns="paymentColumns"
              :header-cell-style="{
                background: 'var(--el-fill-color-light)',
                color: 'var(--el-text-color-primary)'
              }"
            >
              <template #paymentUserId="{ row }">
                <el-tag effect="plain">{{ paymentUserId(row) }}</el-tag>
              </template>
              <template #amount="{ row }">
                <span class="credit-price">{{ paymentAmount(row) }}</span>
              </template>
              <template #paymentStatus="{ row }">
                <el-tag :type="paymentStatusMeta(row.status).type">
                  {{ paymentStatusMeta(row.status).label }}
                </el-tag>
              </template>
              <template #providerTradeNo="{ row }">
                {{ providerTradeNo(row) }}
              </template>
              <template #idempotencyKey="{ row }">
                <span class="order-no">{{ paymentIdempotencyKey(row) }}</span>
              </template>
              <template #failureReason="{ row }">
                <el-text :type="paymentFailureReason(row) ? 'danger' : 'info'">
                  {{ paymentFailureReason(row) || "-" }}
                </el-text>
              </template>
              <template #paymentPaidAt="{ row }">
                {{ formatTime(paymentPaidAt(row)) }}
              </template>
              <template #paymentUpdatedAt="{ row }">
                {{ formatTime(paymentUpdatedAt(row)) }}
              </template>
            </pure-table>
          </el-tab-pane>
        </el-tabs>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped>
.mall-orders-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.mall-panel {
  width: 100%;
  padding: 16px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.panel-header {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 14px;
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.panel-header p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.panel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.permission-alert {
  margin-bottom: 8px;
}

.maintenance-form {
  padding: 12px 12px 0;
  margin-bottom: 12px;
  background: var(--el-fill-color-extra-light);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.overview-area {
  margin: 12px 0 4px;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.overview-grid article {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
  padding: 14px;
  background: var(--el-fill-color-extra-light);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.overview-icon {
  display: inline-flex;
  width: 38px;
  height: 38px;
  align-items: center;
  justify-content: center;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-radius: 6px;
}

.overview-grid strong {
  display: block;
  color: var(--el-text-color-primary);
  font-size: 20px;
  line-height: 1.2;
}

.overview-grid p {
  margin: 5px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.overview-detail-grid {
  display: grid;
  grid-template-columns: 1.1fr 1fr 1fr;
  gap: 12px;
  margin-top: 12px;
}

.overview-detail-grid section {
  min-width: 0;
  padding: 14px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.overview-detail-grid header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
}

.overview-detail-grid h3 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 15px;
  font-weight: 600;
}

.overview-detail-grid header span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.status-chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.status-chip {
  cursor: pointer;
}

.paying-alert {
  margin-top: 10px;
}

.compact-product-list {
  display: grid;
  gap: 8px;
}

.compact-product-list div {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
}

.compact-product-list strong,
.compact-product-list span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact-product-list strong {
  color: var(--el-text-color-primary);
  font-size: 13px;
}

.compact-product-list span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.search-form {
  padding: 12px 0 4px;
}

.order-no {
  font-family:
    ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono",
    "Courier New", monospace;
  font-size: 12px;
}

.credit-price {
  font-weight: 600;
  color: var(--el-color-warning);
}

.order-amount-cell {
  display: inline-flex;
  flex-direction: column;
  gap: 4px;
  align-items: center;
  line-height: 1.2;
}

.order-amount-cell small {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.order-summary {
  margin-bottom: 18px;
}

.record-section {
  margin-bottom: 18px;
}

.record-section h3 {
  margin: 0 0 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.record-tabs {
  margin-top: 4px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }

  .overview-grid,
  .overview-detail-grid {
    grid-template-columns: 1fr;
  }

  .compact-product-list div {
    grid-template-columns: 1fr;
  }
}
</style>
