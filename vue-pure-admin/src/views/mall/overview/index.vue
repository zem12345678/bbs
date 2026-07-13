<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  getAdminMallOverview,
  listAdminMallOutboxRequeueAudits,
  requeueAdminMallOutboxEvents,
  type AdminMallOutboxRequeueAudit,
  type AdminMallFinanceAnomaly,
  type AdminMallOverview,
  type AdminMallProduct,
  type AdminMallStatusCount
} from "@/api/admin";

defineOptions({
  name: "MallOverview"
});

type ProductRow = Partial<AdminMallProduct> & Record<string, any>;
type OutboxAuditRow = Partial<AdminMallOutboxRequeueAudit> &
  Record<string, any>;
type FinanceAnomalyRow = Partial<AdminMallFinanceAnomaly> & Record<string, any>;
type FinanceTagType = "primary" | "success" | "warning" | "info" | "danger";

const router = useRouter();
const loading = ref(false);
const outboxRequeueing = ref(false);
const outboxAuditLoading = ref(false);
const overview = ref<AdminMallOverview | null>(null);
const outboxRequeueAudits = ref<AdminMallOutboxRequeueAudit[]>([]);
const outboxRequeueAuditTotal = ref(0);
const lowStockThreshold = ref(10);

const canViewOverview = computed(() => hasPerms("mall:list_orders"));
const canListProducts = computed(() => hasPerms("mall:list_products"));
const canListOrders = computed(() => hasPerms("mall:list_orders"));
const canListRefunds = computed(() => hasPerms("mall:list_refunds"));
const canListCoupons = computed(() => hasPerms("mall:list_coupons"));
const canRequeueOutbox = computed(() => hasPerms("mall:requeue_outbox_events"));

const orderStatusCounts = computed(
  () =>
    overview.value?.order_status_counts ??
    overview.value?.orderStatusCounts ??
    []
);
const refundStatusCounts = computed(
  () =>
    overview.value?.refund_status_counts ??
    overview.value?.refundStatusCounts ??
    []
);
const lowStockProducts = computed<AdminMallProduct[]>(
  () =>
    overview.value?.low_stock_products ?? overview.value?.lowStockProducts ?? []
);
const topSellingProducts = computed<AdminMallProduct[]>(
  () =>
    overview.value?.top_selling_products ??
    overview.value?.topSellingProducts ??
    []
);
const pendingOutboxTotal = computed(() =>
  overviewNumber("pending_outbox_total", "pendingOutboxTotal")
);
const outboxStatusCounts = computed(
  () =>
    overview.value?.outbox_status_counts ??
    overview.value?.outboxStatusCounts ??
    []
);
const outboxFailedTotal = computed(() => outboxStatusCount("failed"));
const outboxDeadLetterTotal = computed(() => outboxStatusCount("dead_letter"));
const outboxPublishingTotal = computed(() => outboxStatusCount("publishing"));
const outboxLastError = computed(() =>
  overviewText("outbox_last_error", "outboxLastError")
);
const outboxLastErrorAt = computed(() =>
  overviewNumber("outbox_last_error_at", "outboxLastErrorAt")
);
const outboxNextAttemptAt = computed(() =>
  overviewNumber("outbox_next_attempt_at", "outboxNextAttemptAt")
);
const netRevenueCreditsTotal = computed(() =>
  overviewNumber("net_revenue_credits_total", "netRevenueCreditsTotal")
);
const succeededPaymentCreditsTotal = computed(() =>
  overviewNumber(
    "succeeded_payment_credits_total",
    "succeededPaymentCreditsTotal"
  )
);
const failedPaymentTotal = computed(() =>
  overviewNumber("failed_payment_total", "failedPaymentTotal")
);
const failedPaymentCreditsTotal = computed(() =>
  overviewNumber("failed_payment_credits_total", "failedPaymentCreditsTotal")
);
const pendingRefundCreditsTotal = computed(() =>
  overviewNumber("pending_refund_credits_total", "pendingRefundCreditsTotal")
);
const financeAnomalyTotal = computed(() =>
  overviewNumber("finance_anomaly_total", "financeAnomalyTotal")
);
const financeAnomalies = computed<AdminMallFinanceAnomaly[]>(
  () =>
    overview.value?.finance_anomalies ?? overview.value?.financeAnomalies ?? []
);

const metricCards = computed(() => [
  {
    label: "累计收入",
    value: overviewNumber("revenue_credits_total", "revenueCreditsTotal"),
    unit: "积分",
    icon: "ri/copper-coin-line",
    action: "查看订单",
    disabled: !canListOrders.value,
    onClick: () => goOrders()
  },
  {
    label: "净收入",
    value: netRevenueCreditsTotal.value,
    unit: "积分",
    icon: "ri/funds-line",
    action: "财务对账",
    disabled: !canListOrders.value,
    onClick: () => goOrders()
  },
  {
    label: "今日收入",
    value: overviewNumber("today_revenue_credits", "todayRevenueCredits"),
    unit: "积分",
    icon: "ri/sun-line",
    action: "今日订单",
    disabled: !canListOrders.value,
    onClick: () => goOrders()
  },
  {
    label: "待发货",
    value: overviewNumber("pending_shipment_total", "pendingShipmentTotal"),
    unit: "单",
    icon: "ri/truck-line",
    action: "处理发货",
    disabled: !canListOrders.value,
    onClick: () => goOrders(3)
  },
  {
    label: "待售后",
    value: overviewNumber("pending_refund_total", "pendingRefundTotal"),
    unit: "单",
    icon: "ri/refund-2-line",
    action: "审核售后",
    disabled: !canListRefunds.value,
    onClick: () => goRefunds(1)
  },
  {
    label: "失败支付",
    value: failedPaymentTotal.value,
    unit: "笔",
    icon: "ri/error-warning-line",
    action: "查看支付",
    disabled: !canListOrders.value,
    onClick: () => goOrders()
  },
  {
    label: "待投递事件",
    value: pendingOutboxTotal.value,
    unit: "条",
    icon: "ri/broadcast-line",
    action: "刷新状态",
    disabled: false,
    onClick: () => loadOverview()
  },
  {
    label: "低库存",
    value: overviewNumber("low_stock_total", "lowStockTotal"),
    unit: "个商品",
    icon: "ri/alarm-warning-line",
    action: "补库存",
    disabled: !canListProducts.value,
    onClick: () => goProducts()
  },
  {
    label: "在售商品",
    value: overviewNumber("active_product_total", "activeProductTotal"),
    unit: "个",
    icon: "ri/shopping-bag-3-line",
    action: "商品管理",
    disabled: !canListProducts.value,
    onClick: () => goProducts()
  }
]);

const financeRows = computed<
  Array<{ label: string; value: number; unit: string; type: FinanceTagType }>
>(() => [
  {
    label: "成功收款",
    value: succeededPaymentCreditsTotal.value,
    unit: "积分",
    type: "success"
  },
  {
    label: "订单收入",
    value: overviewNumber("revenue_credits_total", "revenueCreditsTotal"),
    unit: "积分",
    type: "primary"
  },
  {
    label: "已退款",
    value: overviewNumber("refunded_credits_total", "refundedCreditsTotal"),
    unit: "积分",
    type: "warning"
  },
  {
    label: "待退款",
    value: pendingRefundCreditsTotal.value,
    unit: "积分",
    type: "warning"
  },
  {
    label: "失败支付金额",
    value: failedPaymentCreditsTotal.value,
    unit: "积分",
    type: failedPaymentTotal.value > 0 ? "danger" : "info"
  },
  {
    label: "净收入",
    value: netRevenueCreditsTotal.value,
    unit: "积分",
    type: netRevenueCreditsTotal.value >= 0 ? "success" : "danger"
  }
]);

const operationCards = computed(() => [
  {
    title: "商品与库存",
    desc: "维护商品、售价、库存和上下架状态",
    icon: "ri/shopping-bag-3-line",
    disabled: !canListProducts.value,
    onClick: () => goProducts()
  },
  {
    title: "订单履约",
    desc: "处理已支付订单、发货信息和订单日志",
    icon: "ri/bill-line",
    disabled: !canListOrders.value,
    onClick: () => goOrders(3)
  },
  {
    title: "售后审核",
    desc: "审核退款申请，必要时恢复库存",
    icon: "ri/refund-2-line",
    disabled: !canListRefunds.value,
    onClick: () => goRefunds(1)
  },
  {
    title: "优惠投放",
    desc: "配置优惠码、门槛、额度和投放周期",
    icon: "ri/coupon-3-line",
    disabled: !canListCoupons.value,
    onClick: () => router.push("/mall/coupons")
  }
]);

function overviewNumber(snakeKey: string, camelKey: string) {
  const data = (overview.value ?? {}) as Record<string, unknown>;
  return Number(data[snakeKey] ?? data[camelKey] ?? 0);
}

function overviewText(snakeKey: string, camelKey: string) {
  const data = (overview.value ?? {}) as Record<string, unknown>;
  return String(data[snakeKey] ?? data[camelKey] ?? "").trim();
}

function auditText(row: OutboxAuditRow, snakeKey: string, camelKey: string) {
  return String(row[snakeKey] ?? row[camelKey] ?? "").trim();
}

function auditNumber(row: OutboxAuditRow, snakeKey: string, camelKey: string) {
  return Number(row[snakeKey] ?? row[camelKey] ?? 0);
}

function financeText(
  row: FinanceAnomalyRow,
  snakeKey: string,
  camelKey: string
) {
  return String(row[snakeKey] ?? row[camelKey] ?? "").trim();
}

function financeNumber(
  row: FinanceAnomalyRow,
  snakeKey: string,
  camelKey: string
) {
  return Number(row[snakeKey] ?? row[camelKey] ?? 0);
}

function financeIssueLabel(issueType: string) {
  switch (issueType) {
    case "PAYMENT_MISMATCH":
      return "收款与订单不一致";
    case "REFUND_EXCEEDS_PAYMENT":
      return "退款超过收款";
    default:
      return issueType || "未知异常";
  }
}

function productPrice(row: ProductRow) {
  return Number(row.price_credits ?? row.priceCredits ?? 0);
}

function productSales(row: ProductRow) {
  return Number(row.sales_count ?? row.salesCount ?? 0);
}

function orderStatusLabel(status: string) {
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

function refundStatusLabel(status: string) {
  switch (String(status || "").toUpperCase()) {
    case "REQUESTED":
      return "待审核";
    case "PROCESSING":
      return "处理中";
    case "APPROVED":
      return "已退款";
    case "REJECTED":
      return "已拒绝";
    default:
      return status || "-";
  }
}

function statusCountTotal(items: AdminMallStatusCount[]) {
  return items.reduce((sum, item) => sum + Number(item.count || 0), 0);
}

function outboxStatusCount(status: string) {
  const target = status.toLowerCase();
  const match = outboxStatusCounts.value.find(
    item => String(item.status || "").toLowerCase() === target
  );
  return Number(match?.count || 0);
}

function outboxStatusLabel(status: string) {
  switch (String(status || "").toLowerCase()) {
    case "pending":
      return "待投递";
    case "publishing":
      return "投递中";
    case "failed":
      return "失败待重试";
    case "dead_letter":
      return "死信";
    default:
      return status || "-";
  }
}

function statusPercent(
  item: AdminMallStatusCount,
  items: AdminMallStatusCount[]
) {
  const total = statusCountTotal(items);
  if (total <= 0) return 0;
  return Math.round((Number(item.count || 0) / total) * 100);
}

function outboxHealthTitle() {
  if (outboxDeadLetterTotal.value > 0) return "存在死信商城事件";
  if (outboxFailedTotal.value > 0) return "存在失败待重试事件";
  if (outboxPublishingTotal.value > 0) return "商城事件正在投递";
  return pendingOutboxTotal.value > 0
    ? "存在待投递商城事件"
    : "商城事件投递正常";
}

function outboxHealthDescription() {
  if (outboxDeadLetterTotal.value > 0) {
    return "部分商城 outbox 事件已进入死信，需要人工检查 Kafka、通知服务或事件负载后再处理。";
  }
  if (outboxFailedTotal.value > 0) {
    return "部分商城 outbox 事件投递失败，系统会按重试时间继续投递；如果持续失败，需要检查下游服务。";
  }
  return pendingOutboxTotal.value > 0
    ? "订单支付、发货、完成、评价或售后通知可能仍在排队投递，请关注 Kafka、通知服务或 mall-service outbox worker。"
    : "当前没有待投递的商城 outbox 事件，订单与售后通知投影处于健康状态。";
}

function outboxHealthType() {
  if (outboxDeadLetterTotal.value > 0) return "error";
  if (outboxFailedTotal.value > 0 || pendingOutboxTotal.value > 0) {
    return "warning";
  }
  if (outboxPublishingTotal.value > 0) return "info";
  return "success";
}

function outboxAuditAggregate(row: OutboxAuditRow) {
  const aggregateType = auditText(row, "aggregate_type", "aggregateType");
  const aggregateID = auditNumber(row, "aggregate_id", "aggregateId");
  if (!aggregateType && !aggregateID) return "-";
  return `${aggregateType || "聚合"} #${aggregateID || "-"}`;
}

function outboxAuditPreviousState(row: OutboxAuditRow) {
  const status = auditText(row, "previous_status", "previousStatus");
  const attempts = auditNumber(row, "previous_attempts", "previousAttempts");
  return `${outboxStatusLabel(status)} · 已尝试 ${attempts} 次`;
}

async function handleRequeueOutboxEvents() {
  if (!canRequeueOutbox.value) {
    message("没有重试商城事件权限", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    "系统会将 failed / dead_letter 商城 outbox 事件重新置为待投递，并重置本轮投递次数。原失败信息、次数和操作人会留存审计记录。请先确认 Kafka、通知服务或消费端故障已经恢复。",
    "重试商城事件",
    {
      type: "warning",
      confirmButtonText: "执行重试",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  outboxRequeueing.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await requeueAdminMallOutboxEvents({
      statuses: ["failed", "dead_letter"],
      limit: 100
    });
    if (code !== 0) {
      message(msg || "重试商城事件失败", { type: "error" });
      return;
    }
    const requeued = Number(data?.requeued ?? 0);
    const eventIDs = data?.event_ids ?? [];
    message(
      requeued > 0
        ? `已重新排队 ${requeued} 条商城事件${eventIDs.length ? `（${eventIDs.join(", ")}）` : ""}`
        : "没有需要重试的商城事件",
      { type: "success" }
    );
    await loadOverview();
    await loadOutboxRequeueAudits();
  } finally {
    outboxRequeueing.value = false;
  }
}

function formatTime(value?: number) {
  if (!value) return "-";
  return dayjs(value).format("YYYY-MM-DD HH:mm");
}

function goOrders(status?: number) {
  router.push({
    path: "/mall/orders",
    query: status ? { status: String(status) } : undefined
  });
}

function goRefunds(status?: number) {
  router.push({
    path: "/mall/refunds",
    query: status ? { status: String(status) } : undefined
  });
}

function goProducts() {
  router.push("/mall/products");
}

async function loadOverview() {
  if (!canViewOverview.value) {
    overview.value = null;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await getAdminMallOverview({
      low_stock_threshold: lowStockThreshold.value
    });
    if (code !== 0) {
      message(msg || "加载商城概览失败", { type: "error" });
      return;
    }
    overview.value = data.overview ?? null;
  } finally {
    loading.value = false;
  }
}

async function loadOutboxRequeueAudits() {
  if (!canRequeueOutbox.value) {
    outboxRequeueAudits.value = [];
    outboxRequeueAuditTotal.value = 0;
    return;
  }
  outboxAuditLoading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallOutboxRequeueAudits({
      limit: 5,
      offset: 0
    });
    if (code !== 0) {
      message(msg || "加载商城事件重试审计失败", { type: "error" });
      return;
    }
    outboxRequeueAudits.value = data?.items ?? [];
    outboxRequeueAuditTotal.value = Number(data?.total ?? 0);
  } finally {
    outboxAuditLoading.value = false;
  }
}

onMounted(() => {
  loadOverview();
  loadOutboxRequeueAudits();
});
</script>

<template>
  <div class="mall-overview-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>商城概览</h2>
          <p>集中查看收入、履约、售后和库存风险，快速进入运营处理页</p>
        </div>
        <div class="panel-actions">
          <el-input-number
            v-model="lowStockThreshold"
            :min="1"
            :max="9999"
            controls-position="right"
            size="small"
            @change="loadOverview"
          />
          <el-button
            type="primary"
            :icon="useRenderIcon('ep/refresh')"
            :disabled="!canViewOverview"
            :loading="loading"
            @click="loadOverview"
          >
            刷新
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="!canViewOverview"
        title="当前账号没有 mall:list_orders 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <div v-if="canViewOverview" v-loading="loading" class="overview-content">
        <div class="metric-grid">
          <article v-for="item in metricCards" :key="item.label">
            <span class="metric-icon">
              <component :is="useRenderIcon(item.icon)" />
            </span>
            <div>
              <strong>{{ item.value }}</strong>
              <p>{{ item.label }} · {{ item.unit }}</p>
            </div>
            <el-button
              link
              type="primary"
              :disabled="item.disabled"
              @click="item.onClick"
            >
              {{ item.action }}
            </el-button>
          </article>
        </div>

        <div class="operation-grid">
          <article v-for="item in operationCards" :key="item.title">
            <span>
              <component :is="useRenderIcon(item.icon)" />
            </span>
            <div>
              <strong>{{ item.title }}</strong>
              <p>{{ item.desc }}</p>
            </div>
            <el-button
              type="primary"
              plain
              :disabled="item.disabled"
              @click="item.onClick"
            >
              进入
            </el-button>
          </article>
        </div>

        <div class="overview-detail-grid">
          <section>
            <header>
              <h3>财务对账</h3>
              <span>净收入 {{ netRevenueCreditsTotal }} 积分</span>
            </header>
            <div class="finance-list">
              <div v-for="item in financeRows" :key="item.label">
                <span>{{ item.label }}</span>
                <el-tag :type="item.type" effect="light">
                  {{ item.value }} {{ item.unit }}
                </el-tag>
              </div>
            </div>
            <p class="finance-hint">
              净收入 = 订单收入 -
              已退款；成功收款来自支付流水，可用于和订单收入交叉核对。
            </p>
            <div class="finance-anomaly-heading">
              <strong>待处理异常</strong>
              <el-tag :type="financeAnomalyTotal > 0 ? 'danger' : 'success'">
                {{ financeAnomalyTotal }} 条
              </el-tag>
            </div>
            <div
              v-if="financeAnomalies.length > 0"
              class="finance-anomaly-list"
            >
              <div
                v-for="item in financeAnomalies"
                :key="item.order_id ?? item.orderId"
              >
                <div>
                  <strong>{{
                    financeIssueLabel(
                      financeText(item, "issue_type", "issueType")
                    )
                  }}</strong>
                  <span>
                    {{
                      financeText(item, "order_no", "orderNo") || "订单号缺失"
                    }}
                    ·
                    {{
                      orderStatusLabel(
                        financeText(item, "order_status", "orderStatus")
                      )
                    }}
                  </span>
                </div>
                <el-tag type="danger" effect="light">
                  差额
                  {{
                    financeNumber(
                      item,
                      "difference_credits",
                      "differenceCredits"
                    )
                  }}
                  积分
                </el-tag>
              </div>
            </div>
            <p v-else class="finance-anomaly-empty">当前没有财务异常</p>
          </section>

          <section>
            <header>
              <h3>订单状态分布</h3>
              <span>{{ statusCountTotal(orderStatusCounts) }} 单</span>
            </header>
            <div v-if="orderStatusCounts.length > 0" class="status-list">
              <div v-for="item in orderStatusCounts" :key="item.status">
                <div>
                  <strong>{{ orderStatusLabel(item.status) }}</strong>
                  <span>{{ item.count }} 单</span>
                </div>
                <el-progress
                  :percentage="statusPercent(item, orderStatusCounts)"
                  :show-text="false"
                />
              </div>
            </div>
            <el-empty v-else description="暂无订单状态" />
          </section>

          <section>
            <header>
              <h3>售后状态分布</h3>
              <span>{{ statusCountTotal(refundStatusCounts) }} 单</span>
            </header>
            <div v-if="refundStatusCounts.length > 0" class="status-list">
              <div v-for="item in refundStatusCounts" :key="item.status">
                <div>
                  <strong>{{ refundStatusLabel(item.status) }}</strong>
                  <span>{{ item.count }} 单</span>
                </div>
                <el-progress
                  :percentage="statusPercent(item, refundStatusCounts)"
                  :show-text="false"
                  status="warning"
                />
              </div>
            </div>
            <el-empty v-else description="暂无售后状态" />
          </section>

          <section>
            <header class="outbox-section-header">
              <div class="outbox-section-title">
                <h3>事件投递健康</h3>
                <span>{{ pendingOutboxTotal }} 条待投递</span>
              </div>
              <el-button
                size="small"
                type="danger"
                plain
                :loading="outboxRequeueing"
                :disabled="
                  !canRequeueOutbox ||
                  outboxFailedTotal + outboxDeadLetterTotal <= 0
                "
                @click="handleRequeueOutboxEvents"
              >
                重试失败/死信
              </el-button>
            </header>
            <el-alert
              :title="outboxHealthTitle()"
              :description="outboxHealthDescription()"
              :type="outboxHealthType()"
              show-icon
              :closable="false"
              class="outbox-health-alert"
            />
            <div class="outbox-status-list">
              <div v-for="item in outboxStatusCounts" :key="item.status">
                <span>{{ outboxStatusLabel(item.status) }}</span>
                <strong>{{ item.count }}</strong>
              </div>
              <el-empty
                v-if="outboxStatusCounts.length === 0"
                description="暂无待处理事件"
              />
            </div>
            <div v-if="outboxLastError" class="outbox-error">
              <strong>最近错误</strong>
              <span>{{ outboxLastError }}</span>
              <small v-if="outboxLastErrorAt">
                {{ formatTime(outboxLastErrorAt) }}
              </small>
            </div>
            <p v-if="outboxNextAttemptAt" class="outbox-retry">
              下次重试：{{ formatTime(outboxNextAttemptAt) }}
            </p>
            <div
              v-if="canRequeueOutbox"
              v-loading="outboxAuditLoading"
              class="outbox-audit-list"
            >
              <div class="outbox-audit-heading">
                <strong>最近人工重试</strong>
                <span>{{ outboxRequeueAuditTotal }} 条记录</span>
              </div>
              <div
                v-for="item in outboxRequeueAudits"
                :key="item.id"
                class="outbox-audit-item"
              >
                <div>
                  <strong>{{ auditText(item, "event_id", "eventId") }}</strong>
                  <el-tag size="small" type="warning" effect="light">
                    {{ outboxAuditPreviousState(item) }}
                  </el-tag>
                </div>
                <p>
                  {{ outboxAuditAggregate(item) }} · 操作人
                  {{ auditText(item, "operator_id", "operatorId") || "-" }} ·
                  {{
                    formatTime(auditNumber(item, "requeued_at", "requeuedAt"))
                  }}
                </p>
                <small
                  v-if="auditText(item, 'previous_error', 'previousError')"
                >
                  {{ auditText(item, "previous_error", "previousError") }}
                </small>
              </div>
              <el-empty
                v-if="outboxRequeueAudits.length === 0"
                description="暂无人工重试记录"
              />
            </div>
          </section>

          <section>
            <header>
              <h3>低库存预警</h3>
              <span>阈值 {{ lowStockThreshold }}</span>
            </header>
            <div v-if="lowStockProducts.length > 0" class="product-list">
              <button
                v-for="item in lowStockProducts"
                :key="item.id"
                type="button"
                :disabled="!canListProducts"
                @click="goProducts"
              >
                <strong>{{ item.title }}</strong>
                <span
                  >{{ item.stock }} 库存 · {{ productPrice(item) }} 积分</span
                >
              </button>
            </div>
            <el-empty v-else description="库存健康" />
          </section>

          <section>
            <header>
              <h3>热销商品</h3>
              <span>Top 5</span>
            </header>
            <div v-if="topSellingProducts.length > 0" class="product-list">
              <button
                v-for="item in topSellingProducts"
                :key="item.id"
                type="button"
                :disabled="!canListProducts"
                @click="goProducts"
              >
                <strong>{{ item.title }}</strong>
                <span
                  >{{ productSales(item) }} 销量 · {{ item.stock }} 库存</span
                >
              </button>
            </div>
            <el-empty v-else description="暂无销量" />
          </section>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.mall-overview-page {
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
  gap: 8px;
  align-items: center;
}

.permission-alert {
  margin-bottom: 8px;
}

.overview-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.metric-grid article,
.operation-grid article,
.overview-detail-grid section {
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  background: var(--el-fill-color-extra-light);
}

.metric-grid article {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 14px;
}

.metric-icon,
.operation-grid article > span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-radius: 6px;
}

.metric-grid strong {
  font-size: 24px;
  line-height: 1;
  color: var(--el-text-color-primary);
}

.metric-grid p,
.operation-grid p,
.product-list span {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.operation-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.operation-grid article {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  align-items: center;
  padding: 14px;
}

.operation-grid article .el-button {
  grid-column: 1 / -1;
  justify-self: start;
}

.overview-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.overview-detail-grid section {
  min-height: 220px;
  padding: 14px;
}

.overview-detail-grid header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.overview-detail-grid h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.overview-detail-grid header > span,
.outbox-section-title span {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.outbox-section-header {
  gap: 10px;
}

.outbox-section-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.status-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.finance-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.finance-list > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  font-size: 13px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.finance-list span,
.finance-hint {
  color: var(--el-text-color-secondary);
}

.finance-anomaly-heading,
.finance-anomaly-list > div {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
}

.finance-anomaly-heading {
  margin-top: 14px;
  font-size: 13px;
}

.finance-anomaly-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.finance-anomaly-list > div {
  padding: 8px 10px;
  font-size: 12px;
  background: var(--el-color-danger-light-9);
  border: 1px solid var(--el-color-danger-light-7);
  border-radius: 6px;
}

.finance-anomaly-list > div > div {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.finance-anomaly-list span,
.finance-anomaly-empty {
  color: var(--el-text-color-secondary);
}

.finance-anomaly-empty {
  margin: 8px 0 0;
  font-size: 12px;
}

.finance-hint {
  margin: 12px 0 0;
  font-size: 12px;
  line-height: 1.6;
}

.outbox-health-alert {
  margin-top: 8px;
}

.outbox-status-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.outbox-status-list > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  font-size: 13px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.outbox-status-list span,
.outbox-error small,
.outbox-retry {
  color: var(--el-text-color-secondary);
}

.outbox-status-list strong {
  color: var(--el-text-color-primary);
}

.outbox-status-list :deep(.el-empty) {
  grid-column: 1 / -1;
  padding: 8px 0;
}

.outbox-error {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  margin-top: 12px;
  font-size: 13px;
  background: var(--el-color-danger-light-9);
  border: 1px solid var(--el-color-danger-light-7);
  border-radius: 6px;
}

.outbox-error span {
  color: var(--el-color-danger);
  word-break: break-word;
}

.outbox-retry {
  margin: 8px 0 0;
  font-size: 12px;
}

.outbox-audit-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 72px;
  margin-top: 12px;
}

.outbox-audit-heading,
.outbox-audit-item > div {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
}

.outbox-audit-heading strong,
.outbox-audit-item strong {
  color: var(--el-text-color-primary);
}

.outbox-audit-heading span,
.outbox-audit-item p,
.outbox-audit-item small {
  color: var(--el-text-color-secondary);
}

.outbox-audit-item {
  padding: 10px 12px;
  font-size: 12px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.outbox-audit-item strong,
.outbox-audit-item small {
  word-break: break-all;
}

.outbox-audit-item p {
  margin: 6px 0 0;
}

.outbox-audit-item small {
  display: block;
  margin-top: 4px;
}

.outbox-audit-list :deep(.el-empty) {
  padding: 8px 0;
}

.status-list > div > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  font-size: 13px;
}

.product-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.product-list button {
  display: flex;
  flex-direction: column;
  gap: 2px;
  align-items: flex-start;
  width: 100%;
  padding: 10px 12px;
  text-align: left;
  cursor: pointer;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  transition:
    color 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.product-list button:hover:not(:disabled) {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-color: var(--el-color-primary-light-5);
}

.product-list button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

@media (max-width: 1200px) {
  .metric-grid,
  .operation-grid,
  .overview-detail-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .panel-header,
  .panel-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .metric-grid,
  .operation-grid,
  .overview-detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
