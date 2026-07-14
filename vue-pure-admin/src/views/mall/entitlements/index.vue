<script setup lang="ts">
import dayjs from "dayjs";
import { ElMessageBox } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { normalizeEntityId } from "@/utils/entityId";
import { downloadCsv, type CsvColumn } from "@/utils/csvExport";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  listAdminMallDigitalEntitlements,
  revokeAdminMallDigitalEntitlement,
  type AdminMallDigitalEntitlement
} from "@/api/admin";

defineOptions({
  name: "MallEntitlements"
});

type EntitlementRow = Partial<AdminMallDigitalEntitlement> & Record<string, any>;

const EXPORT_LIMIT = 1000;
const loading = ref(false);
const exporting = ref(false);
const entitlements = ref<AdminMallDigitalEntitlement[]>([]);

const query = reactive({
  keyword: "",
  userId: "",
  status: "",
  grantType: "",
  grantKey: "",
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canList = computed(() => hasPerms("mall:list_digital_entitlements"));
const canRevoke = computed(() => hasPerms("mall:revoke_digital_entitlement"));

const statusOptions = [
  { label: "全部", value: "" },
  { label: "可用", value: "ACTIVE" },
  { label: "已过期", value: "EXPIRED" },
  { label: "已撤销", value: "REVOKED" }
];

const grantTypeOptions = [
  { label: "全部", value: "" },
  { label: "会员", value: "membership" },
  { label: "主题", value: "theme" },
  { label: "徽章", value: "badge" },
  { label: "数字", value: "digital" }
];

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "用户/订单", minWidth: 180, slot: "userOrder" },
  { label: "权益", minWidth: 260, slot: "entitlement" },
  { label: "授权", minWidth: 180, slot: "grant" },
  { label: "状态", width: 130, slot: "status" },
  { label: "发放时间", width: 170, slot: "issuedAt" },
  { label: "有效期", width: 170, slot: "expiresAt" },
  { label: "售后", minWidth: 240, slot: "refund" },
  { label: "操作", width: 110, slot: "actions", fixed: "right" }
];

const exportColumns: CsvColumn<EntitlementRow>[] = [
  { header: "权益ID", value: row => row.id ?? "" },
  { header: "用户ID", value: userIdOf },
  { header: "订单ID", value: orderIdOf },
  { header: "订单号", value: orderNoOf },
  { header: "商品ID", value: productIdOf },
  { header: "SKU", value: row => row.sku ?? "" },
  { header: "权益名称", value: row => row.title ?? "" },
  { header: "交付码", value: entitlementCode },
  { header: "授权类型", value: row => grantLabel(row) },
  { header: "授权Key", value: grantKeyOf },
  { header: "状态", value: row => statusLabel(row) },
  { header: "发放时间", value: row => formatTime(issuedAt(row)) },
  { header: "有效期", value: row => expiryText(row) },
  { header: "撤销时间", value: row => formatTime(revokedAt(row)) },
  { header: "撤销人", value: revokedByOf },
  { header: "撤销原因", value: revokeReasonOf },
  { header: "退款ID", value: refundIdOf }
];

function userIdOf(row: EntitlementRow) {
  return row.user_id ?? row.userId;
}

function orderIdOf(row: EntitlementRow) {
  return row.order_id ?? row.orderId;
}

function orderNoOf(row: EntitlementRow) {
  return row.order_no ?? row.orderNo ?? "";
}

function productIdOf(row: EntitlementRow) {
  return row.product_id ?? row.productId;
}

function entitlementCode(row: EntitlementRow) {
  return row.fulfillment_code ?? row.fulfillmentCode ?? "";
}

function grantTypeOf(row: EntitlementRow) {
  return String(row.grant_type ?? row.grantType ?? "").trim().toLowerCase();
}

function grantKeyOf(row: EntitlementRow) {
  return String(row.grant_key ?? row.grantKey ?? "").trim();
}

function grantLabel(row: EntitlementRow) {
  const labels: Record<string, string> = {
    membership: "会员权益",
    theme: "主题权益",
    badge: "徽章权益",
    digital: "数字权益"
  };
  return labels[grantTypeOf(row)] ?? "数字权益";
}

function issuedAt(row: EntitlementRow) {
  return Number(row.issued_at ?? row.issuedAt ?? 0);
}

function expiresAt(row: EntitlementRow) {
  return Number(row.expires_at ?? row.expiresAt ?? 0);
}

function revokedAt(row: EntitlementRow) {
  return Number(row.revoked_at ?? row.revokedAt ?? 0);
}

function refundIdOf(row: EntitlementRow) {
  return row.refund_id ?? row.refundId ?? "";
}

function revokedByOf(row: EntitlementRow) {
  return row.revoked_by ?? row.revokedBy ?? "";
}

function revokeReasonOf(row: EntitlementRow) {
  return row.revoke_reason ?? row.revokeReason ?? "";
}

function statusValue(row: EntitlementRow) {
  return String(row.status ?? "").trim().toUpperCase();
}

function revoked(row: EntitlementRow) {
  return statusValue(row) === "REVOKED" || Boolean(revokedAt(row));
}

function expired(row: EntitlementRow) {
  const expiry = expiresAt(row);
  return statusValue(row) === "EXPIRED" || (!revoked(row) && expiry > 0 && expiry <= Date.now());
}

function statusLabel(row: EntitlementRow) {
  if (revoked(row)) return "已撤销";
  if (expired(row)) return "已过期";
  return "可用";
}

function statusTagType(row: EntitlementRow) {
  if (revoked(row)) return "danger";
  return expired(row) ? "warning" : "success";
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function expiryText(row: EntitlementRow) {
  const expiry = expiresAt(row);
  if (!expiry) return "长期有效";
  return `${expired(row) ? "已过期" : "有效至"} ${formatTime(expiry)}`;
}

function entitlementIdOf(row: EntitlementRow) {
  return normalizeEntityId(row.id ?? row.entitlement_id ?? row.entitlementId);
}

function currentParams(
  limit = query.pageSize,
  offset = (query.currentPage - 1) * query.pageSize
) {
  return {
    user_id: normalizeEntityId(query.userId),
    keyword: query.keyword.trim() || undefined,
    status: query.status || undefined,
    grant_type: query.grantType || undefined,
    grant_key: query.grantKey.trim() || undefined,
    limit,
    offset
  };
}

async function loadEntitlements() {
  if (!canList.value) {
    entitlements.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const { code, data, message: msg } =
      await listAdminMallDigitalEntitlements(currentParams());
    if (code !== 0) {
      message(msg || "加载权益台账失败", { type: "error" });
      return;
    }
    entitlements.value = data.items ?? [];
    query.total = data.total ?? entitlements.value.length;
  } catch (error: any) {
    message(error?.message || "加载权益台账失败", { type: "error" });
  } finally {
    loading.value = false;
  }
}

async function handleRevoke(row: EntitlementRow) {
  if (!canRevoke.value) {
    message("没有撤销权益权限", { type: "warning" });
    return;
  }
  const entitlementId = entitlementIdOf(row);
  if (!entitlementId) {
    message("权益 ID 无效", { type: "warning" });
    return;
  }
  if (revoked(row)) {
    message("该权益已撤销", { type: "warning" });
    return;
  }
  const title = row.title || row.sku || `权益 #${entitlementId}`;
  let value = "";
  try {
    ({ value } = await ElMessageBox.prompt(`确认撤销 ${title}？`, "撤销权益", {
      type: "warning",
      inputType: "textarea",
      inputPlaceholder: "填写撤销原因",
      confirmButtonText: "确认撤销",
      cancelButtonText: "取消",
      inputValidator: (value: string) =>
        String(value ?? "").trim().length > 0 || "撤销原因不能为空"
    }));
  } catch {
    return;
  }
  const reason = String(value ?? "").trim();
  loading.value = true;
  try {
    const { code, message: msg } = await revokeAdminMallDigitalEntitlement(
      entitlementId,
      { reason }
    );
    if (code !== 0) {
      message(msg || "撤销失败", { type: "error" });
      return;
    }
    message("撤销已提交", { type: "success" });
    await loadEntitlements();
  } catch (error: any) {
    message(error?.message || "撤销失败", { type: "error" });
  } finally {
    loading.value = false;
  }
}

async function exportEntitlements() {
  if (!canList.value) {
    message("没有导出权益台账权限", { type: "warning" });
    return;
  }
  exporting.value = true;
  try {
    const { code, data, message: msg } =
      await listAdminMallDigitalEntitlements(currentParams(EXPORT_LIMIT, 0));
    if (code !== 0) {
      message(msg || "导出权益台账失败", { type: "error" });
      return;
    }
    const items = data.items ?? [];
    if (items.length === 0) {
      message("当前筛选条件下没有可导出的权益", { type: "warning" });
      return;
    }
    downloadCsv(
      `mall-digital-entitlements-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      exportColumns,
      items
    );
    const total = data.total ?? items.length;
    message(
      total > items.length
        ? `已导出前 ${items.length} 条权益，当前筛选共 ${total} 条`
        : `已导出 ${items.length} 条权益`,
      { type: "success" }
    );
  } catch (error: any) {
    message(error?.message || "导出权益台账失败", { type: "error" });
  } finally {
    exporting.value = false;
  }
}

function resetQuery() {
  query.keyword = "";
  query.userId = "";
  query.status = "";
  query.grantType = "";
  query.grantKey = "";
  query.currentPage = 1;
  loadEntitlements();
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadEntitlements();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadEntitlements();
}

onMounted(loadEntitlements);
</script>

<template>
  <div class="mall-entitlements-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>权益台账</h2>
          <p>查询数字权益发放、有效期、撤销和售后追踪状态</p>
        </div>
        <div class="panel-actions">
          <el-button
            :icon="useRenderIcon('ep/refresh')"
            :loading="loading"
            :disabled="!canList"
            @click="loadEntitlements"
          >
            刷新
          </el-button>
          <el-button
            type="success"
            plain
            :icon="useRenderIcon('ri/download-2-line')"
            :loading="exporting"
            :disabled="!canList"
            @click="exportEntitlements"
          >
            导出台账
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 mall:list_digital_entitlements 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            class="w-56!"
            clearable
            placeholder="订单号/交付码/SKU/授权"
            @keyup.enter="loadEntitlements"
          />
        </el-form-item>
        <el-form-item label="用户ID">
          <el-input
            v-model="query.userId"
            class="w-36!"
            clearable
            placeholder="用户ID"
            @keyup.enter="loadEntitlements"
          />
        </el-form-item>
        <el-form-item label="类型">
          <el-select
            v-model="query.grantType"
            class="w-32!"
            @change="loadEntitlements"
          >
            <el-option
              v-for="item in grantTypeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="授权Key">
          <el-input
            v-model="query.grantKey"
            class="w-40!"
            clearable
            placeholder="theme-pro"
            @keyup.enter="loadEntitlements"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-32!"
            @change="loadEntitlements"
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
            @click="loadEntitlements"
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
        :data="entitlements"
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
        <template #userOrder="{ row }">
          <div class="entity-cell">
            <strong>用户 #{{ userIdOf(row) || "-" }}</strong>
            <span>{{ orderNoOf(row) || `订单 #${orderIdOf(row) || "-"}` }}</span>
          </div>
        </template>
        <template #entitlement="{ row }">
          <div class="entitlement-cell">
            <strong>{{ row.title || row.sku || `商品 #${productIdOf(row) || "-"}` }}</strong>
            <span>{{ entitlementCode(row) || "无交付码" }}</span>
            <small>商品 #{{ productIdOf(row) || "-" }} · SKU {{ row.sku || "-" }}</small>
          </div>
        </template>
        <template #grant="{ row }">
          <div class="tag-cell">
            <el-tag effect="plain">{{ grantLabel(row) }}</el-tag>
            <span>{{ grantKeyOf(row) || "-" }}</span>
          </div>
        </template>
        <template #status="{ row }">
          <el-tag :type="statusTagType(row)" effect="plain">
            {{ statusLabel(row) }}
          </el-tag>
        </template>
        <template #issuedAt="{ row }">
          {{ formatTime(issuedAt(row)) }}
        </template>
        <template #expiresAt="{ row }">
          {{ expiryText(row) }}
        </template>
        <template #refund="{ row }">
          <div class="entity-cell">
            <strong>{{ refundIdOf(row) ? `退款 #${refundIdOf(row)}` : "-" }}</strong>
            <span v-if="revokedAt(row)">撤销 {{ formatTime(revokedAt(row)) }}</span>
            <span v-else>未撤销</span>
            <small v-if="revokedByOf(row)">操作人 {{ revokedByOf(row) }}</small>
            <small v-if="revokeReasonOf(row)">{{ revokeReasonOf(row) }}</small>
          </div>
        </template>
        <template #actions="{ row }">
          <el-button
            v-if="canRevoke && !revoked(row)"
            size="small"
            type="danger"
            plain
            :icon="useRenderIcon('ri/forbid-2-line')"
            :disabled="loading"
            @click="handleRevoke(row)"
          >
            撤销
          </el-button>
          <span v-else class="muted-action">-</span>
        </template>
      </pure-table>
    </section>
  </div>
</template>

<style scoped>
.mall-entitlements-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.mall-panel {
  padding: 18px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
}

.panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}

.panel-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.panel-header p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.panel-actions,
.search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.permission-alert,
.search-form {
  margin-bottom: 16px;
}

.entity-cell,
.entitlement-cell,
.tag-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  text-align: left;
}

.entity-cell strong,
.entitlement-cell strong {
  font-weight: 650;
  color: var(--el-text-color-primary);
}

.entity-cell span,
.entity-cell small,
.entitlement-cell span,
.entitlement-cell small,
.tag-cell span {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.tag-cell {
  align-items: center;
}

.muted-action {
  color: var(--el-text-color-placeholder);
}

@media (max-width: 900px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
