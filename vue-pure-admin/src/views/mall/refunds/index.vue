<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { normalizeEntityId, type EntityId } from "@/utils/entityId";
import { downloadCsv, type CsvColumn } from "@/utils/csvExport";
import {
  loadAllOffsetPages,
  OFFSET_LIST_PAGE_CONCURRENCY
} from "@/utils/offsetPages";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  listAdminMallDigitalEntitlements,
  listAdminMallOrderLogs,
  listAdminMallOrderPayments,
  listAdminMallOrders,
  listAdminMallRefunds,
  reviewAdminMallRefund,
  type AdminMallOrder,
  type AdminMallDigitalEntitlement,
  type AdminMallOrderItem,
  type AdminMallOrderStatusLog,
  type AdminMallPayment,
  type AdminMallRefund
} from "@/api/admin";

defineOptions({
  name: "MallRefunds"
});

type RefundRow = Partial<AdminMallRefund> & Record<string, any>;
type OrderRow = Partial<AdminMallOrder> & Record<string, any>;
type EntitlementRow = Partial<AdminMallDigitalEntitlement> & Record<string, any>;
type OrderItemRow = Partial<AdminMallOrderItem> & Record<string, any>;
type LogRow = Partial<AdminMallOrderStatusLog> & Record<string, any>;
type PaymentRow = Partial<AdminMallPayment> & Record<string, any>;
type RefundExportRow = RefundRow & {
  __digitalEntitlements?: EntitlementRow[];
};

const refundEntitlementOrderBatchSize = 100;
const refundEntitlementBatchConcurrency = OFFSET_LIST_PAGE_CONCURRENCY;

const route = useRoute();
const loading = ref(false);
const saving = ref(false);
const detailLoading = ref(false);
const exporting = ref(false);
const refunds = ref<AdminMallRefund[]>([]);
const detailRefund = ref<RefundRow>();
const detailOrder = ref<AdminMallOrder | null>(null);
const detailLogs = ref<AdminMallOrderStatusLog[]>([]);
const detailPayments = ref<AdminMallPayment[]>([]);
const reviewDialogVisible = ref(false);
const detailDrawerVisible = ref(false);
const reviewFormRef = ref<FormInstance>();
let refundListRequestVersion = 0;
let refundDetailRequestVersion = 0;

function errorMessage(error: unknown) {
  const response = (error as any)?.response?.data;
  return response?.message ?? response?.reason ?? (error as Error)?.message ?? "";
}

const query = reactive({
  keyword: "",
  userId: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const reviewForm = reactive({
  id: 0 as EntityId,
  orderNo: "",
  amountCredits: 0,
  status: 0,
  approved: true,
  adminNote: "",
  restoreStock: false
});

const canList = computed(() => hasPerms("mall:list_refunds"));
const canReview = computed(() => hasPerms("mall:review_refunds"));
const canListOrders = computed(() => hasPerms("mall:list_orders"));
const canListEntitlements = computed(() =>
  hasPerms("mall:list_digital_entitlements")
);
const canListOrderLogs = computed(() => hasPerms("mall:list_order_logs"));
const canListPayments = computed(() => hasPerms("mall:list_order_payments"));
const detailEntitlements = computed(() => digitalEntitlementsOf(detailOrder.value));

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "订单号", minWidth: 180, slot: "orderNo" },
  { label: "用户 ID", width: 120, slot: "userId" },
  { label: "退款积分", width: 110, slot: "amountCredits" },
  { label: "状态", width: 110, slot: "status" },
  { prop: "reason", label: "原因", minWidth: 140, showOverflowTooltip: true },
  { label: "用户备注", minWidth: 180, slot: "userNote" },
  { label: "审核备注", minWidth: 180, slot: "adminNote" },
  { label: "恢复库存", width: 100, slot: "restoreStock" },
  { label: "申请时间", width: 170, slot: "requestedAt" },
  { label: "审核时间", width: 170, slot: "reviewedAt" },
  { label: "退款时间", width: 170, slot: "refundedAt" },
  { label: "撤回时间", width: 170, slot: "canceledAt" },
  { label: "操作", fixed: "right", width: 210, slot: "operation" }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "待审核", value: 1 },
  { label: "处理中", value: 2 },
  { label: "已退款", value: 3 },
  { label: "已拒绝", value: 4 },
  { label: "用户已撤回", value: 5 }
];

const refundExportColumns: CsvColumn<RefundExportRow>[] = [
  { header: "售后ID", value: row => row.id ?? "" },
  { header: "订单ID", value: orderIdOf },
  { header: "订单号", value: orderNoOf },
  { header: "用户ID", value: userIdOf },
  { header: "退款积分", value: amountCreditsOf },
  { header: "状态", value: row => statusMeta(Number(row.status ?? 0)).label },
  { header: "原因", value: row => row.reason ?? "" },
  { header: "用户备注", value: userNoteOf },
  { header: "审核备注", value: adminNoteOf },
  { header: "数字权益", value: refundDigitalEntitlementExportText },
  { header: "恢复库存", value: row => (restoreStockOf(row) ? "是" : "否") },
  { header: "操作人ID", value: operatorIdOf },
  { header: "申请时间", value: row => formatTime(requestedAt(row)) },
  { header: "审核时间", value: row => formatTime(reviewedAt(row)) },
  { header: "退款时间", value: row => formatTime(refundedAt(row)) },
  { header: "撤回时间", value: row => formatTime(canceledAt(row)) }
];

function routeQueryText(...keys: string[]) {
  for (const key of keys) {
    const value = route.query[key];
    const firstValue = Array.isArray(value) ? value[0] : value;
    const text = String(firstValue ?? "").trim();
    if (text) return text;
  }
  return "";
}

function applyRouteQuery() {
  const routeStatus = Number(routeQueryText("status"));
  query.keyword = routeQueryText(
    "refund_id",
    "refundId",
    "order_id",
    "orderId",
    "order_no",
    "orderNo",
    "keyword"
  );
  query.userId = routeQueryText("user_id", "userId");
  query.status = statusOptions.some(item => item.value === routeStatus)
    ? routeStatus
    : 0;
  query.currentPage = 1;
}

const rules: FormRules = {
  adminNote: [
    {
      validator: (_rule, value, callback) => {
        if (!reviewForm.approved && !String(value || "").trim()) {
          callback(new Error("拒绝售后时必须填写审核备注"));
          return;
        }
        callback();
      },
      trigger: "blur"
    }
  ]
};

function statusMeta(status?: number) {
  switch (status) {
    case 1:
      return { label: "待审核", type: "warning" as const };
    case 2:
      return { label: "处理中", type: "primary" as const };
    case 3:
      return { label: "已退款", type: "success" as const };
    case 4:
      return { label: "已拒绝", type: "danger" as const };
    case 5:
      return { label: "用户已撤回", type: "info" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "info" as const };
  }
}

function orderStatusMeta(status?: number) {
  switch (Number(status ?? 0)) {
    case 0:
      return { label: "创建", type: "info" as const };
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
      return { label: `未知(${status ?? "-"})`, type: "info" as const };
  }
}

function paymentStatusMeta(status?: number) {
  switch (Number(status ?? 0)) {
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

function orderNoOf(row: RefundRow) {
  return row.order_no ?? row.orderNo ?? "-";
}

function orderIdOf(row: RefundRow) {
  return row.order_id ?? row.orderId ?? "-";
}

function userIdOf(row: RefundRow) {
  return row.user_id ?? row.userId ?? "-";
}

function amountCreditsOf(row: RefundRow) {
  return Number(row.amount_credits ?? row.amountCredits ?? 0);
}

function userNoteOf(row: RefundRow) {
  return row.user_note ?? row.userNote ?? "-";
}

function adminNoteOf(row: RefundRow) {
  return row.admin_note ?? row.adminNote ?? "-";
}

function restoreStockOf(row: RefundRow) {
  return Boolean(row.restore_stock ?? row.restoreStock ?? false);
}

function requestedAt(row: RefundRow) {
  return row.requested_at ?? row.requestedAt;
}

function reviewedAt(row: RefundRow) {
  return row.reviewed_at ?? row.reviewedAt;
}

function refundedAt(row: RefundRow) {
  return row.refunded_at ?? row.refundedAt;
}

function canceledAt(row: RefundRow) {
  return row.canceled_at ?? row.canceledAt;
}

function operatorIdOf(row: RefundRow) {
  return row.operator_id ?? row.operatorId ?? "-";
}

function orderTotalCredits(row?: OrderRow | null) {
  return Number(row?.total_credits ?? row?.totalCredits ?? 0);
}

function orderOriginalCredits(row?: OrderRow | null) {
  const original = Number(row?.original_credits ?? row?.originalCredits ?? 0);
  return original > 0 ? original : orderTotalCredits(row);
}

function orderDiscountCredits(row?: OrderRow | null) {
  return Number(row?.discount_credits ?? row?.discountCredits ?? 0);
}

function orderCouponCode(row?: OrderRow | null) {
  return row?.coupon_code ?? row?.couponCode ?? "";
}

function shippingCarrierOf(row?: OrderRow | null) {
  return row?.shipping_carrier ?? row?.shippingCarrier ?? "";
}

function trackingNoOf(row?: OrderRow | null) {
  return row?.tracking_no ?? row?.trackingNo ?? "";
}

function paidAtOf(row?: OrderRow | null) {
  return row?.paid_at ?? row?.paidAt;
}

function shippedAtOf(row?: OrderRow | null) {
  return row?.shipped_at ?? row?.shippedAt;
}

function completedAtOf(row?: OrderRow | null) {
  return row?.completed_at ?? row?.completedAt;
}

function orderUpdatedAt(row?: OrderRow | null) {
  return row?.updated_at ?? row?.updatedAt;
}

function orderItems(row?: OrderRow | null) {
  return (row?.items ?? []) as OrderItemRow[];
}

function digitalEntitlementsOf(row?: OrderRow | null): EntitlementRow[] {
  const items = row?.digital_entitlements ?? row?.digitalEntitlements ?? [];
  return Array.isArray(items) ? items : [];
}

function entitlementProductId(row: EntitlementRow) {
  return row.product_id ?? row.productId ?? "-";
}

function entitlementQuantity(row: EntitlementRow) {
  return Number(row.quantity ?? 0);
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

function entitlementSummary(row: EntitlementRow) {
  const title = row.title || row.sku || `商品 ${entitlementProductId(row)}`;
  const code = entitlementCode(row);
  const quantity = entitlementQuantity(row);
  const grantKey = entitlementGrantKey(row);
  const revokedAt = entitlementRevokedAt(row);
  const refundId = entitlementRefundId(row);
  const expiry = entitlementExpiryText(row);
  return `${title}${quantity > 0 ? ` x${quantity}` : ""}${code ? ` / ${code}` : ""} / ${entitlementGrantLabel(row)}${grantKey ? `:${grantKey}` : ""} / ${entitlementStatusLabel(row)}${expiry ? ` / ${expiry}` : ""}${revokedAt ? ` / 撤销 ${formatTime(Number(revokedAt))}` : ""}${refundId ? ` / 退款 ${refundId}` : ""}`;
}

function refundDigitalEntitlementExportText(row: RefundExportRow) {
  const entitlements = row.__digitalEntitlements ?? [];
  if (entitlements.length === 0) return "";
  return entitlements.map(entitlementSummary).join("；");
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

function paymentAmount(row: PaymentRow) {
  return Number(row.amount_credits ?? row.amountCredits ?? 0);
}

function paymentPaidAt(row: PaymentRow) {
  return row.paid_at ?? row.paidAt;
}

function providerTradeNo(row: PaymentRow) {
  return row.provider_trade_no ?? row.providerTradeNo ?? "-";
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function fulfillmentText(row?: OrderRow | null) {
  const carrier = shippingCarrierOf(row);
  const trackingNo = trackingNoOf(row);
  if (carrier || trackingNo) {
    return [carrier, trackingNo].filter(Boolean).join(" / ");
  }
  return "-";
}

function refundStatusValue(row: RefundRow) {
  return Number(row.status ?? 0);
}

function canApproveRefundRow(row: RefundRow) {
  const status = refundStatusValue(row);
  return canReview.value && (status === 1 || status === 2);
}

function canRejectRefundRow(row: RefundRow) {
  return canReview.value && refundStatusValue(row) === 1;
}

function currentRefundListParams(
  limit = query.pageSize,
  offset = (query.currentPage - 1) * query.pageSize
) {
  const userID = query.userId.trim();
  return {
    user_id: userID || undefined,
    keyword: query.keyword.trim(),
    status: Number(query.status || 0),
    limit,
    offset
  };
}

async function loadRefunds() {
  const requestVersion = ++refundListRequestVersion;
  if (!canList.value) {
    refunds.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallRefunds(currentRefundListParams());
    if (requestVersion !== refundListRequestVersion) return;
    if (code !== 0) {
      message(msg || "加载售后列表失败", { type: "error" });
      return;
    }
    refunds.value = data.items ?? [];
    query.total = data.total ?? refunds.value.length;
  } finally {
    if (requestVersion === refundListRequestVersion) {
      loading.value = false;
    }
  }
}

async function exportRefunds() {
  if (!canList.value) {
    message("没有导出售后权限", { type: "warning" });
    return;
  }
  if (!canListEntitlements.value) {
    message("没有查询数字权益权限，无法导出售后数字权益留档", {
      type: "warning"
    });
    return;
  }
  exporting.value = true;
  try {
    const { code, items, message: msg } = await loadAllOffsetPages(
      ({ limit, offset }) =>
        listAdminMallRefunds(currentRefundListParams(limit, offset))
    );
    if (code !== 0) {
      message(msg || "导出售后失败", { type: "error" });
      return;
    }
    if (items.length === 0) {
      message("当前筛选条件下没有可导出的售后单", { type: "warning" });
      return;
    }
    const exportRows = await enrichRefundExportRows(items);
    downloadCsv(
      `mall-refunds-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      refundExportColumns,
      exportRows
    );
    message(`已导出 ${items.length} 条售后单`, { type: "success" });
  } catch (error: any) {
    message(error?.message || "导出售后失败", { type: "error" });
  } finally {
    exporting.value = false;
  }
}

async function enrichRefundExportRows(items: RefundRow[]): Promise<RefundExportRow[]> {
  const orderIDs = refundOrderIDs(items);
  if (orderIDs.length === 0) {
    return items.map(row => ({ ...row, __digitalEntitlements: [] }));
  }
  const entitlements = await loadRefundDigitalEntitlements(orderIDs);
  const orderIDSet = new Set(orderIDs);
  const entitlementsByOrderID = new Map<string, EntitlementRow[]>();
  for (const entitlement of entitlements) {
    const orderID = String(
      entitlement.order_id ?? entitlement.orderId ?? ""
    ).trim();
    if (!orderIDSet.has(orderID)) continue;
    const entries = entitlementsByOrderID.get(orderID) ?? [];
    entries.push(entitlement);
    entitlementsByOrderID.set(orderID, entries);
  }
  return items.map(row => ({
    ...row,
    __digitalEntitlements:
      entitlementsByOrderID.get(String(orderIdOf(row))) ?? []
  }));
}

function refundOrderIDs(items: RefundRow[]) {
  const orderIDs: string[] = [];
  const seen = new Set<string>();
  for (const item of items) {
    const orderID = String(orderIdOf(item)).trim();
    if (!/^[1-9]\d*$/.test(orderID) || seen.has(orderID)) continue;
    seen.add(orderID);
    orderIDs.push(orderID);
  }
  return orderIDs;
}

async function loadRefundDigitalEntitlements(orderIDs: string[]) {
  const batches: string[][] = [];
  for (
    let start = 0;
    start < orderIDs.length;
    start += refundEntitlementOrderBatchSize
  ) {
    batches.push(
      orderIDs.slice(start, start + refundEntitlementOrderBatchSize)
    );
  }
  const results = new Array<EntitlementRow[]>(batches.length);
  const workerCount = Math.min(
    refundEntitlementBatchConcurrency,
    batches.length
  );
  const pageConcurrency = Math.max(
    1,
    Math.floor(OFFSET_LIST_PAGE_CONCURRENCY / workerCount)
  );
  let nextBatchIndex = 0;
  let stopped = false;
  const workers = Array.from(
    { length: workerCount },
    async () => {
      while (!stopped) {
        const batchIndex = nextBatchIndex;
        nextBatchIndex += 1;
        const batch = batches[batchIndex];
        if (!batch) return;
        try {
          results[batchIndex] = await loadRefundDigitalEntitlementBatch(
            batch,
            pageConcurrency
          );
        } catch (error) {
          stopped = true;
          throw error;
        }
      }
    }
  );
  await Promise.all(workers);
  return results.flat();
}

async function loadRefundDigitalEntitlementBatch(
  orderIDs: string[],
  concurrency: number
) {
  const { code, items, message: msg } = await loadAllOffsetPages(
    ({ limit, offset }) =>
      listAdminMallDigitalEntitlements({
        order_ids: orderIDs.join(","),
        limit,
        offset
      }),
    { concurrency }
  );
  if (code !== 0) {
    throw new Error(msg || "加载数字权益台账失败");
  }
  return items;
}

function resetQuery() {
  query.keyword = "";
  query.userId = "";
  query.status = 0;
  query.currentPage = 1;
  loadRefunds();
}

function openReviewDialog(row: RefundRow, approved: boolean) {
  if (approved ? !canApproveRefundRow(row) : !canRejectRefundRow(row)) {
    message("当前售后单不可审核", { type: "warning" });
    return;
  }
  const status = refundStatusValue(row);
  reviewForm.id = normalizeEntityId(row.id) ?? 0;
  reviewForm.orderNo = orderNoOf(row);
  reviewForm.amountCredits = amountCreditsOf(row);
  reviewForm.status = status;
  reviewForm.approved = status === 2 ? true : approved;
  reviewForm.adminNote = "";
  reviewForm.restoreStock = false;
  reviewFormRef.value?.clearValidate();
  reviewDialogVisible.value = true;
}

async function openDetailDrawer(row: RefundRow) {
  const requestVersion = ++refundDetailRequestVersion;
  const isCurrentRequest = () => requestVersion === refundDetailRequestVersion;
  detailRefund.value = row;
  detailOrder.value = null;
  detailLogs.value = [];
  detailPayments.value = [];
  detailLoading.value = false;
  detailDrawerVisible.value = true;
  if (!canListOrders.value) {
    return;
  }
  detailLoading.value = true;
  try {
    const { code, data, message: msg } = await listAdminMallOrders({
      keyword: orderNoOf(row),
      status: 0,
      limit: 1,
      offset: 0
    });
    if (!isCurrentRequest()) return;
    if (code !== 0) {
      throw new Error(msg || "关联订单加载失败");
    }
    const order = (data.items ?? [])[0] ?? null;
    detailOrder.value = order;
    const orderId = normalizeEntityId(order?.id);
    const tasks: Promise<void>[] = [];
    if (orderId && canListOrderLogs.value) {
      tasks.push(
        listAdminMallOrderLogs(orderId).then(({ code, data, message: msg }) => {
          if (!isCurrentRequest()) return;
          if (code !== 0) throw new Error(msg || "订单日志加载失败");
          detailLogs.value = data.items ?? [];
        })
      );
    }
    if (orderId && canListPayments.value) {
      tasks.push(
        listAdminMallOrderPayments(orderId).then(({ code, data, message: msg }) => {
          if (!isCurrentRequest()) return;
          if (code !== 0) throw new Error(msg || "支付记录加载失败");
          detailPayments.value = data.items ?? [];
        })
      );
    }
    await Promise.all(tasks);
  } catch (error: any) {
    if (!isCurrentRequest()) return;
    message(error?.message || "售后详情加载失败", { type: "error" });
  } finally {
    if (isCurrentRequest()) {
      detailLoading.value = false;
    }
  }
}

async function saveReview() {
  if (!canReview.value) {
    message("没有售后审核权限", { type: "warning" });
    return;
  }
  if (reviewForm.status === 2 && !reviewForm.approved) {
    message("处理中售后只能重试退款流程", { type: "warning" });
    return;
  }
  const valid = await reviewFormRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    const { code, message: msg } = await reviewAdminMallRefund(
      normalizeEntityId(reviewForm.id),
      {
        approved: reviewForm.approved,
        admin_note: reviewForm.adminNote.trim(),
        restore_stock: reviewForm.approved && reviewForm.restoreStock
      }
    );
    if (code !== 0) {
      message(msg || "售后审核失败", { type: "error" });
      return;
    }
    message(reviewForm.approved ? "售后已通过并触发退款" : "售后已拒绝", {
      type: "success"
    });
    reviewDialogVisible.value = false;
    await loadRefunds();
  } catch (error) {
    message(errorMessage(error) || "售后审核失败", { type: "error" });
  } finally {
    saving.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadRefunds();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadRefunds();
}

watch(
  () => [
    route.query.refund_id,
    route.query.refundId,
    route.query.order_id,
    route.query.orderId,
    route.query.order_no,
    route.query.orderNo,
    route.query.keyword,
    route.query.user_id,
    route.query.userId,
    route.query.status
  ],
  () => {
    applyRouteQuery();
    loadRefunds();
  }
);

watch(canList, () => {
  loadRefunds();
});

onMounted(() => {
  applyRouteQuery();
  loadRefunds();
});
</script>

<template>
  <div class="mall-refunds-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>售后管理</h2>
          <p>审核积分商城售后申请，退款通过后自动写入积分流水</p>
        </div>
        <el-button
          type="success"
          plain
          :icon="useRenderIcon('ri/download-2-line')"
          :disabled="!canList"
          :loading="exporting"
          @click="exportRefunds"
        >
          导出售后
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 mall:list_refunds 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            class="w-52!"
            clearable
            placeholder="退款ID / 订单号 / 原因"
            @keyup.enter="loadRefunds"
          />
        </el-form-item>
        <el-form-item label="用户 ID">
          <el-input
            v-model="query.userId"
            class="w-36!"
            clearable
            placeholder="用户 ID"
            @keyup.enter="loadRefunds"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-32!"
            @change="loadRefunds"
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
            @click="loadRefunds"
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
        :data="refunds"
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
        <template #amountCredits="{ row }">
          <span class="credit-price">{{ amountCreditsOf(row) }}</span>
        </template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #userNote="{ row }">
          <span>{{ userNoteOf(row) }}</span>
        </template>
        <template #adminNote="{ row }">
          <span>{{ adminNoteOf(row) }}</span>
        </template>
        <template #restoreStock="{ row }">
          <el-tag :type="restoreStockOf(row) ? 'success' : 'info'" effect="plain">
            {{ restoreStockOf(row) ? "是" : "否" }}
          </el-tag>
        </template>
        <template #requestedAt="{ row }">
          {{ formatTime(requestedAt(row)) }}
        </template>
        <template #reviewedAt="{ row }">
          {{ formatTime(reviewedAt(row)) }}
        </template>
        <template #refundedAt="{ row }">
          {{ formatTime(refundedAt(row)) }}
        </template>
        <template #canceledAt="{ row }">
          {{ formatTime(canceledAt(row)) }}
        </template>
        <template #operation="{ row }">
          <div class="operation-cell">
            <el-button
              link
              type="primary"
              :icon="useRenderIcon('ri/file-list-3-line')"
              @click="openDetailDrawer(row)"
            >
              详情
            </el-button>
            <template v-if="canApproveRefundRow(row) || canRejectRefundRow(row)">
              <el-button
                v-if="canApproveRefundRow(row)"
                link
                type="success"
                :icon="useRenderIcon('ri/check-line')"
                @click="openReviewDialog(row, true)"
              >
                {{ refundStatusValue(row) === 2 ? "重试退款" : "通过" }}
              </el-button>
              <el-button
                v-if="canRejectRefundRow(row)"
                link
                type="danger"
                :icon="useRenderIcon('ri/close-line')"
                @click="openReviewDialog(row, false)"
              >
                拒绝
              </el-button>
            </template>
          </div>
        </template>
      </pure-table>
    </section>

    <el-dialog
      v-model="reviewDialogVisible"
      title="售后审核"
      width="520px"
      destroy-on-close
    >
      <el-alert
        :title="`订单 ${reviewForm.orderNo}，退款 ${reviewForm.amountCredits} 积分`"
        type="info"
        show-icon
        :closable="false"
        class="permission-alert"
      />
      <el-form
        ref="reviewFormRef"
        :model="reviewForm"
        :rules="rules"
        label-width="92px"
      >
        <el-form-item label="审核结果">
          <el-radio-group v-model="reviewForm.approved">
            <el-radio-button :label="true">通过</el-radio-button>
            <el-radio-button :label="false" :disabled="reviewForm.status === 2">
              拒绝
            </el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="reviewForm.approved" label="库存处理">
          <el-checkbox v-model="reviewForm.restoreStock">
            退款后恢复订单商品库存
          </el-checkbox>
        </el-form-item>
        <el-form-item label="审核备注" prop="adminNote">
          <el-input
            v-model="reviewForm.adminNote"
            type="textarea"
            :rows="4"
            maxlength="300"
            show-word-limit
            placeholder="填写通过说明、拒绝原因或线下处理记录"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveReview">
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="detailDrawerVisible"
      size="920px"
      destroy-on-close
      class="refund-detail-drawer"
    >
      <template #header>
        <div class="drawer-title">
          <h3>售后详情</h3>
          <span>
            {{ orderNoOf(detailRefund || {}) }}
            <em v-if="detailRefund">售后单 #{{ detailRefund.id }}</em>
          </span>
        </div>
      </template>

      <div v-loading="detailLoading" class="refund-detail">
        <el-alert
          v-if="!canListOrders"
          title="当前账号没有 mall:list_orders 权限，无法加载关联订单"
          type="warning"
          show-icon
          :closable="false"
          class="permission-alert"
        />

        <section class="detail-section">
          <div class="detail-section-header">
            <h4>售后信息</h4>
            <el-tag
              v-if="detailRefund"
              :type="statusMeta(detailRefund.status).type"
              effect="plain"
            >
              {{ statusMeta(detailRefund.status).label }}
            </el-tag>
          </div>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="售后单 ID">
              {{ detailRefund?.id || "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="订单 ID">
              {{ detailRefund ? orderIdOf(detailRefund) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="用户 ID">
              {{ detailRefund ? userIdOf(detailRefund) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="退款积分">
              <span class="credit-price">
                {{ detailRefund ? amountCreditsOf(detailRefund) : 0 }}
              </span>
            </el-descriptions-item>
            <el-descriptions-item label="申请原因">
              {{ detailRefund?.reason || "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="恢复库存">
              {{ detailRefund && restoreStockOf(detailRefund) ? "是" : "否" }}
            </el-descriptions-item>
            <el-descriptions-item label="用户备注" :span="2">
              {{ detailRefund ? userNoteOf(detailRefund) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="审核备注" :span="2">
              {{ detailRefund ? adminNoteOf(detailRefund) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="审核人">
              {{ detailRefund ? operatorIdOf(detailRefund) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="申请时间">
              {{ detailRefund ? formatTime(requestedAt(detailRefund)) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="审核时间">
              {{ detailRefund ? formatTime(reviewedAt(detailRefund)) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="退款时间">
              {{ detailRefund ? formatTime(refundedAt(detailRefund)) : "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="撤回时间">
              {{ detailRefund ? formatTime(canceledAt(detailRefund)) : "-" }}
            </el-descriptions-item>
          </el-descriptions>
        </section>

        <section class="detail-section">
          <div class="detail-section-header">
            <h4>关联订单</h4>
            <el-tag
              v-if="detailOrder"
              :type="orderStatusMeta(detailOrder.status).type"
              effect="plain"
            >
              {{ orderStatusMeta(detailOrder.status).label }}
            </el-tag>
          </div>
          <el-empty
            v-if="canListOrders && !detailOrder && !detailLoading"
            description="未找到关联订单"
          />
          <template v-else-if="detailOrder">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="订单号">
                <span class="order-no">{{ orderNoOf(detailOrder) }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="用户 ID">
                {{ userIdOf(detailOrder) }}
              </el-descriptions-item>
              <el-descriptions-item label="实付积分">
                <span class="credit-price">{{ orderTotalCredits(detailOrder) }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="原价/优惠">
                原价 {{ orderOriginalCredits(detailOrder) }}，优惠
                {{ orderDiscountCredits(detailOrder) }}
              </el-descriptions-item>
              <el-descriptions-item label="优惠码">
                {{ orderCouponCode(detailOrder) || "未使用" }}
              </el-descriptions-item>
              <el-descriptions-item label="物流">
                {{ fulfillmentText(detailOrder) }}
              </el-descriptions-item>
              <el-descriptions-item label="支付时间">
                {{ formatTime(paidAtOf(detailOrder)) }}
              </el-descriptions-item>
              <el-descriptions-item label="发货时间">
                {{ formatTime(shippedAtOf(detailOrder)) }}
              </el-descriptions-item>
              <el-descriptions-item label="完成时间">
                {{ formatTime(completedAtOf(detailOrder)) }}
              </el-descriptions-item>
              <el-descriptions-item label="更新时间">
                {{ formatTime(orderUpdatedAt(detailOrder)) }}
              </el-descriptions-item>
            </el-descriptions>

            <el-table
              :data="orderItems(detailOrder)"
              border
              class="detail-table"
              empty-text="暂无订单商品"
            >
              <el-table-column label="商品" min-width="220">
                <template #default="{ row }">
                  <div class="item-cell">
                    <strong>{{ row.title || row.sku || "-" }}</strong>
                    <small>商品 ID {{ itemProductId(row) }} · SKU {{ row.sku || "-" }}</small>
                    <small v-if="itemGrantText(row)">授权 {{ itemGrantText(row) }}</small>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="quantity" label="数量" width="90" align="center" />
              <el-table-column label="单价" width="110" align="center">
                <template #default="{ row }">{{ itemUnitPrice(row) }}</template>
              </el-table-column>
              <el-table-column label="小计" width="110" align="center">
                <template #default="{ row }">{{ itemSubtotal(row) }}</template>
              </el-table-column>
            </el-table>

            <section v-if="detailEntitlements.length > 0" class="detail-subsection">
              <div class="detail-section-header">
                <h4>数字权益</h4>
              </div>
              <el-table :data="detailEntitlements" border class="detail-table">
                <el-table-column label="商品 ID" width="100">
                  <template #default="{ row }">{{ entitlementProductId(row) }}</template>
                </el-table-column>
                <el-table-column prop="sku" label="SKU" min-width="120" />
                <el-table-column prop="title" label="权益名称" min-width="180" />
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
                <el-table-column label="有效期" width="190">
                  <template #default="{ row }">
                    {{ entitlementExpiryText(row) || "长期有效" }}
                  </template>
                </el-table-column>
                <el-table-column label="状态" width="110">
                  <template #default="{ row }">
                    <el-tag :type="entitlementStatusTagType(row)" effect="plain">
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
              </el-table>
            </section>
          </template>
        </section>

        <section v-if="detailPayments.length > 0" class="detail-section">
          <div class="detail-section-header">
            <h4>支付记录</h4>
          </div>
          <el-table :data="detailPayments" border class="detail-table">
            <el-table-column prop="id" label="ID" width="90" align="center" />
            <el-table-column label="金额" width="110" align="center">
              <template #default="{ row }">{{ paymentAmount(row) }}</template>
            </el-table-column>
            <el-table-column prop="provider" label="渠道" min-width="120" />
            <el-table-column label="状态" width="110" align="center">
              <template #default="{ row }">
                <el-tag :type="paymentStatusMeta(row.status).type" effect="plain">
                  {{ paymentStatusMeta(row.status).label }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="流水号" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">{{ providerTradeNo(row) }}</template>
            </el-table-column>
            <el-table-column label="支付时间" width="170">
              <template #default="{ row }">{{ formatTime(paymentPaidAt(row)) }}</template>
            </el-table-column>
          </el-table>
        </section>

        <section v-if="detailLogs.length > 0" class="detail-section">
          <div class="detail-section-header">
            <h4>订单时间线</h4>
          </div>
          <el-timeline>
            <el-timeline-item
              v-for="log in detailLogs"
              :key="log.id"
              :timestamp="formatTime(logCreatedAt(log))"
              placement="top"
            >
              <div class="timeline-entry">
                <strong>
                  {{ orderStatusMeta(logFromStatus(log)).label }}
                  ->
                  {{ orderStatusMeta(logToStatus(log)).label }}
                </strong>
                <span>{{ log.reason || "-" }}</span>
                <small>
                  操作人 {{ log.operator_id || log.operatorId || "-" }}
                  <template v-if="log.note"> · {{ log.note }}</template>
                </small>
              </div>
            </el-timeline-item>
          </el-timeline>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.mall-refunds-page {
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

.permission-alert {
  margin-bottom: 8px;
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

.operation-cell {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  white-space: nowrap;
}

.drawer-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.drawer-title h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.drawer-title span {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.drawer-title em {
  margin-left: 8px;
  font-style: normal;
  color: var(--el-text-color-placeholder);
}

.refund-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.detail-subsection {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 12px;
}

.detail-section-header {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: space-between;
}

.detail-section-header h4 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.detail-table {
  width: 100%;
}

.item-cell,
.timeline-entry {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-start;
}

.item-cell strong,
.timeline-entry strong {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.item-cell small,
.timeline-entry small,
.timeline-entry span {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }

  .operation-cell {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
