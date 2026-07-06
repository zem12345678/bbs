<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  auditAdminReport,
  listAdminReports,
  muteAdminUser,
  unmuteAdminUser,
  type AdminReport
} from "@/api/admin";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import GovernanceDetailDrawer from "../components/GovernanceDetailDrawer.vue";

type ReportRow = Partial<AdminReport> & Record<string, any>;

defineOptions({
  name: "GovernanceReports"
});

const loading = ref(false);
const reports = ref<AdminReport[]>([]);
const detailVisible = ref(false);
const selectedReport = ref<ReportRow | null>(null);
const userActionLoading = ref(false);
const userForm = reactive({
  userId: undefined as number | undefined
});
const query = reactive({
  status: 1,
  entityType: "",
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canList = computed(() => hasPerms("governance:list_reports"));
const canAudit = computed(() => hasPerms("governance:audit_report"));
const canMute = computed(() => hasPerms("governance:mute_user"));
const canUnmute = computed(() => hasPerms("governance:unmute_user"));

const columns: TableColumnList = [
  { prop: "id", label: "举报 ID", width: 96 },
  { label: "对象", minWidth: 130, slot: "entity" },
  { label: "举报人", minWidth: 110, slot: "reporter" },
  { prop: "reason", label: "原因", minWidth: 150 },
  {
    prop: "description",
    label: "说明",
    minWidth: 220,
    showOverflowTooltip: true
  },
  { label: "状态", width: 110, slot: "status" },
  { label: "处理人", width: 110, slot: "handler" },
  { label: "提交时间", width: 170, slot: "createdAt" },
  { label: "处理时间", width: 170, slot: "handledAt" },
  { label: "操作", fixed: "right", width: 230, slot: "operation" }
];

const reportDetailFields = computed(() => {
  const report = selectedReport.value;
  if (!report) return [];
  return [
    { label: "举报 ID", value: `#${report.id ?? "-"}` },
    { label: "状态", status: statusMeta(report.status ?? 0) },
    { label: "对象类型", value: reportEntityType(report) },
    { label: "对象 ID", value: `#${reportEntityId(report)}` },
    { label: "举报人", value: `#${reporterId(report)}` },
    {
      label: "处理人",
      value: handledBy(report) ? `#${handledBy(report)}` : "-"
    },
    { label: "提交时间", value: formatTime(reportCreatedAt(report)) },
    { label: "更新时间", value: formatTime(reportUpdatedAt(report)) },
    { label: "处理时间", value: formatTime(reportHandledAt(report)) }
  ];
});

const reportDetailSections = computed(() => {
  const report = selectedReport.value;
  if (!report) return [];
  return [
    { title: "举报原因", content: report.reason },
    { title: "举报说明", content: report.description }
  ];
});

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "待处理", value: 1 },
  { label: "已处理", value: 2 },
  { label: "已驳回", value: 3 }
];

const entityTypeOptions = [
  { label: "全部", value: "" },
  { label: "文章", value: "article" },
  { label: "话题", value: "topic" },
  { label: "评论", value: "comment" }
];

function statusMeta(status: number) {
  switch (status) {
    case 1:
      return { label: "待处理", type: "warning" as const };
    case 2:
      return { label: "已处理", type: "success" as const };
    case 3:
      return { label: "已驳回", type: "info" as const };
    default:
      return { label: `未知(${status})`, type: "danger" as const };
  }
}

function reportEntity(report: ReportRow) {
  return `${reportEntityType(report)} #${reportEntityId(report)}`;
}

function reportEntityType(report: ReportRow) {
  return entityTypeLabel(reportEntityTypeValue(report));
}

function reportEntityTypeValue(report: ReportRow) {
  const entity = report.entity ?? {};
  return entity.entity_type ?? entity.entityType ?? "-";
}

function reportEntityId(report: ReportRow) {
  const entity = report.entity ?? {};
  return entity.entity_id ?? entity.entityId ?? "-";
}

function reporterId(report: ReportRow) {
  return report.reporter_id ?? report.reporterId ?? 0;
}

function handledBy(report: ReportRow) {
  return report.handled_by ?? report.handledBy ?? 0;
}

function entityTypeLabel(value?: string) {
  switch (value) {
    case "article":
      return "文章";
    case "topic":
      return "话题";
    case "comment":
      return "评论";
    default:
      return value || "-";
  }
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function reportCreatedAt(report: ReportRow) {
  return report.created_at ?? report.createdAt;
}

function reportHandledAt(report: ReportRow) {
  return report.handled_at ?? report.handledAt;
}

function reportUpdatedAt(report: ReportRow) {
  return report.updated_at ?? report.updatedAt;
}

async function loadReports() {
  if (!canList.value) {
    reports.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminReports({
      status: query.status,
      entity_type: query.entityType || undefined,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载举报列表失败", { type: "error" });
      return;
    }
    reports.value = data.items ?? [];
    query.total = data.total ?? 0;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 1;
  query.entityType = "";
  query.currentPage = 1;
  loadReports();
}

function openDetail(row: ReportRow) {
  selectedReport.value = row;
  detailVisible.value = true;
}

async function handleAudit(row: ReportRow, nextStatus: number) {
  if (!canAudit.value) {
    message("没有审核举报权限", { type: "warning" });
    return;
  }
  const reportId = Number(row.id);
  if (!reportId) {
    message("举报 ID 无效", { type: "warning" });
    return;
  }
  const meta = statusMeta(nextStatus);
  await ElMessageBox.confirm(`确认将举报 #${reportId} 标记为${meta.label}？`, {
    title: "审核确认",
    type: "warning",
    confirmButtonText: "确认",
    cancelButtonText: "取消"
  });
  loading.value = true;
  try {
    const { code, message: msg } = await auditAdminReport(reportId, nextStatus);
    if (code !== 0) {
      message(msg || "审核失败", { type: "error" });
      return;
    }
    message("审核已提交", { type: "success" });
    await loadReports();
  } finally {
    loading.value = false;
  }
}

async function updateUserStatus(muted: boolean) {
  if (muted && !canMute.value) {
    message("没有禁言用户权限", { type: "warning" });
    return;
  }
  if (!muted && !canUnmute.value) {
    message("没有解禁用户权限", { type: "warning" });
    return;
  }
  if (!userForm.userId || userForm.userId <= 0) {
    message("请输入有效用户 ID", { type: "warning" });
    return;
  }
  const actionText = muted ? "禁言" : "解禁";
  await ElMessageBox.confirm(
    `确认${actionText}用户 #${userForm.userId}？`,
    "用户状态确认",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  );
  userActionLoading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = muted
      ? await muteAdminUser(userForm.userId)
      : await unmuteAdminUser(userForm.userId);
    if (code !== 0) {
      message(msg || `${actionText}失败`, { type: "error" });
      return;
    }
    const statusLabel = data.user.status === 2 ? "已禁言" : "正常";
    message(
      `用户 ${data.user.username || data.user.id} 当前状态：${statusLabel}`,
      {
        type: "success"
      }
    );
  } finally {
    userActionLoading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadReports();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadReports();
}

onMounted(loadReports);
</script>

<template>
  <div class="governance-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>举报审核</h2>
          <p>处理用户提交的内容举报</p>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有举报列表查询权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-40!" @change="loadReports">
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="对象类型">
          <el-select
            v-model="query.entityType"
            class="w-40!"
            @change="loadReports"
          >
            <el-option
              v-for="item in entityTypeOptions"
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
            @click="loadReports"
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
        adaptive
        :adaptiveConfig="{ offsetBottom: 156 }"
        align-whole="center"
        table-layout="auto"
        :loading="loading"
        :data="reports"
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
        <template #entity="{ row }">
          <el-tag effect="plain">{{ reportEntity(row) }}</el-tag>
        </template>
        <template #reporter="{ row }">#{{ reporterId(row) }}</template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #handler="{ row }">
          {{ handledBy(row) ? `#${handledBy(row)}` : "-" }}
        </template>
        <template #createdAt="{ row }">
          {{ formatTime(reportCreatedAt(row)) }}
        </template>
        <template #handledAt="{ row }">
          {{ formatTime(reportHandledAt(row)) }}
        </template>
        <template #operation="{ row }">
          <el-button
            link
            type="primary"
            :icon="useRenderIcon('ri/eye-line')"
            @click="openDetail(row)"
          >
            查看
          </el-button>
          <el-button
            v-if="row.status === 1 && canAudit"
            link
            type="primary"
            @click="handleAudit(row, 2)"
          >
            通过
          </el-button>
          <el-button
            v-if="row.status === 1 && canAudit"
            link
            type="danger"
            @click="handleAudit(row, 3)"
          >
            驳回
          </el-button>
        </template>
      </pure-table>

      <GovernanceDetailDrawer
        v-model="detailVisible"
        title="举报详情"
        :fields="reportDetailFields"
        :sections="reportDetailSections"
      />
    </section>

    <section class="governance-panel user-panel">
      <div class="panel-header compact">
        <div>
          <h2>用户状态</h2>
          <p>对社区用户执行禁言或解禁</p>
        </div>
      </div>
      <el-form :inline="true" class="search-form">
        <el-form-item label="用户 ID">
          <el-input-number
            v-model="userForm.userId"
            :min="1"
            :step="1"
            :controls="false"
            class="w-48!"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="danger"
            :disabled="!canMute"
            :loading="userActionLoading"
            @click="updateUserStatus(true)"
          >
            禁言
          </el-button>
          <el-button
            type="primary"
            :disabled="!canUnmute"
            :loading="userActionLoading"
            @click="updateUserStatus(false)"
          >
            解禁
          </el-button>
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>

<style scoped>
.governance-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.governance-panel {
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

.panel-header.compact {
  margin-bottom: 8px;
}

.search-form {
  padding: 12px 0 4px;
}

.permission-alert {
  margin-bottom: 8px;
}

.muted-text {
  color: var(--el-text-color-placeholder);
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
